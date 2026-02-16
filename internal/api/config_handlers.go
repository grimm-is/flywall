// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/vpn"
)

// --- Config CRUD Handlers ---

// checkPendingStatus compares staged config with running config
func (s *Server) checkPendingStatus() bool {
	s.configMu.RLock()
	staged := s.Config
	s.configMu.RUnlock()

	if staged == nil || s.client == nil {
		return false
	}

	running, err := s.client.GetRunningConfig()
	if err != nil {
		// If we can't reach control plane, assume no pending changes to avoid UI noise
		return false
	}

	// Compare serialized versions to detect deep differences
	stagedJSON, _ := json.Marshal(staged)
	runningJSON, _ := json.Marshal(running)

	return string(stagedJSON) != string(runningJSON)
}

// broadcastPendingStatus notifies subscribers of pending status change
func (s *Server) broadcastPendingStatus() {
	if s.wsManager == nil {
		return
	}
	s.wsManager.TriggerStatusUpdate()

	// Deprecated: For backwards compatibility, still publish the old format for now.
	// We can remove this once ui/src/lib/stores/app.ts fully relies on status topic.
	hasPending := s.checkPendingStatus()
	s.wsManager.Publish("pending_status", map[string]bool{
		"has_pending": hasPending,
	})
}

// applyConfigUpdate safely applies a config modification:
// 1. Clones current config
// 2. Applies the update function to the clone
// 3. Validates the modified config
// 4. Sends to control plane via RPC
// 5. Only updates local s.Config on RPC success
//
// Returns true if the update was successful, false otherwise.
// On validation errors, writes 400 Bad Request.
// On RPC errors, writes 500 Internal Server Error.
func (s *Server) applyConfigUpdate(w http.ResponseWriter, r *http.Request, updateFn func(cfg *config.Config)) bool {
	s.configMu.RLock()
	// Deep clone via JSON (safe for nested structs)
	data, err := json.Marshal(s.Config)
	s.configMu.RUnlock()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to clone config")
		return false
	}

	var cloned config.Config
	if err := json.Unmarshal(data, &cloned); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to clone config")
		return false
	}

	// Apply the update to the clone
	updateFn(&cloned)

	// Validate the modified config
	if errs := cloned.Validate(); errs.HasErrors() {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Validation failed: "+errs.Error())
		return false
	}

	// Staging Logic:
	// We do NOT call s.client.ApplyConfig here. We only update the local s.Config (Staged).
	// The user must explicitly hit POST /config/apply to commit changes to the Control Plane.

	// Update local config (Staged)
	s.configMu.Lock()
	updateFn(s.Config)
	s.configMu.Unlock()

	// Notify UI of pending changes
	go s.broadcastPendingStatus()

	return true
}

// handleUpdateConfig updates the entire configuration (Staged)
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newCfg config.Config
	if !BindJSONLenient(w, r, &newCfg) {
		return
	}

	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		*cfg = newCfg
	}) {
		SuccessResponse(w)
	}
}

// handleGetPolicies returns firewall policies
func (s *Server) handleGetPolicies(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		projected := projectZonePolicies(cfg)
		HandleGetData(w, projected)
	}
}

// projectZonePolicies merges implicit zone configuration into the policy list
func projectZonePolicies(cfg *config.Config) []config.Policy {
	// Start with a shallow copy of explicit policies
	// We need to copy the slice to safely append new policies
	// Usage of pointers in the slice means we must be careful not to mutate existing rules in-place
	// if we want to preserve the original config state (though this is just for JSON output).
	policies := make([]config.Policy, len(cfg.Policies))
	copy(policies, cfg.Policies)

	// Index existing policies by From->To for merging
	// Using pointer to element in the new slice to allow appending rules
	policyMap := make(map[string]*config.Policy)
	for i := range policies {
		key := policies[i].From + "->" + policies[i].To
		policyMap[key] = &policies[i]
	}

	for _, zone := range cfg.Zones {
		var rules []config.PolicyRule

		if zone.Management != nil {
			// 1. Management Access (Input Chain: Zone -> Firewall)
			mgmt := []struct {
				Service string
				Enabled bool
				Name    string
			}{
				{"ssh", zone.Management.SSH, "SSH Access"},
				{"web", zone.Management.Web, "Web Access"},
				{"api", zone.Management.API, "API Access"},
				{"icmp", zone.Management.ICMP, "ICMP (Ping)"},
				{"snmp", zone.Management.SNMP, "SNMP Queries"},
				{"syslog", zone.Management.Syslog, "Syslog"},
			}

			for _, m := range mgmt {
				if m.Enabled {
					rules = append(rules, config.PolicyRule{
						Name:    fmt.Sprintf("Implicit %s", m.Name),
						Service: m.Service,
						Action:  "accept",
						Comment: "Managed via Zone Management",
						Origin:  "implicit_zone_config",
					})
				}
			}
		}

		// 2. Zone Services (Struct fields)
		if zone.Services != nil {
			svcs := []struct {
				Service string
				Enabled bool
				Name    string
			}{
				{"dhcp", zone.Services.DHCP, "DHCP Server Access"},
				{"dns", zone.Services.DNS, "DNS Server Access"},
				{"ntp", zone.Services.NTP, "NTP Server Access"},
			}

			for _, s := range svcs {
				if s.Enabled {
					rules = append(rules, config.PolicyRule{
						Name:    fmt.Sprintf("Implicit %s", s.Name),
						Service: s.Service,
						Action:  "accept",
						Comment: "Managed via Zone Services",
						Origin:  "implicit_zone_config",
					})
				}
			}
		}

		if len(rules) > 0 {
			key := zone.Name + "->firewall"
			if pol, exists := policyMap[key]; exists {
				// Append to existing policy
				pol.Rules = append(pol.Rules, rules...)
			} else {
				// Create new synthetic policy
				newPol := config.Policy{
					From:        zone.Name,
					To:          "firewall",
					Name:        fmt.Sprintf("Zone %s Services", zone.Name),
					Description: "Implicit rules derived from Zone settings",
					Origin:      "implicit_zone_config",
					Rules:       rules,
				}
				policies = append(policies, newPol)
				policyMap[key] = &policies[len(policies)-1]
			}
		}
	}

	return policies
}

// handleUpdatePolicies updates firewall policies
func (s *Server) handleUpdatePolicies(w http.ResponseWriter, r *http.Request) {
	var policies []config.Policy
	if !BindJSONLenient(w, r, &policies) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.Policies = policies
	}) {
		SuccessResponse(w)
	}
}

// handleGetNAT returns NAT configuration
// handleGetNAT returns NAT configuration
func (s *Server) handleGetNAT(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.NAT)
	}
}

// handleUpdateNAT updates NAT configuration
func (s *Server) handleUpdateNAT(w http.ResponseWriter, r *http.Request) {
	var nat []config.NATRule
	if !BindJSONLenient(w, r, &nat) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.NAT = nat
	}) {
		SuccessResponse(w)
	}
}

// handleGetMarkRules returns mark rules
// handleGetMarkRules returns mark rules
func (s *Server) handleGetMarkRules(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.MarkRules)
	}
}

// handleUpdateMarkRules updates mark rules
func (s *Server) handleUpdateMarkRules(w http.ResponseWriter, r *http.Request) {
	var rules []config.MarkRule
	if !BindJSONLenient(w, r, &rules) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.MarkRules = rules
	}) {
		SuccessResponse(w)
	}
}

// handleGetUIDRouting returns UID routing rules
// handleGetUIDRouting returns UID routing rules
func (s *Server) handleGetUIDRouting(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.UIDRouting)
	}
}

// handleUpdateUIDRouting updates UID routing rules
func (s *Server) handleUpdateUIDRouting(w http.ResponseWriter, r *http.Request) {
	var rules []config.UIDRouting
	if !BindJSONLenient(w, r, &rules) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.UIDRouting = rules
	}) {
		SuccessResponse(w)
	}
}

// handleGetPolicyRoutes returns policy routing rules
func (s *Server) handleGetPolicyRoutes(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.PolicyRoutes)
	}
}

// handleUpdatePolicyRoutes updates policy routing rules
func (s *Server) handleUpdatePolicyRoutes(w http.ResponseWriter, r *http.Request) {
	var rules []config.PolicyRoute
	if !BindJSONLenient(w, r, &rules) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.PolicyRoutes = rules
	}) {
		SuccessResponse(w)
	}
}

// handleGetIPSets returns IPSet configuration
// handleGetIPSets returns IPSet configuration
func (s *Server) handleGetIPSets(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.IPSets)
	}
}

// handleUpdateIPSets updates IPSet configuration
func (s *Server) handleUpdateIPSets(w http.ResponseWriter, r *http.Request) {
	var ipsets []config.IPSet
	if !BindJSONLenient(w, r, &ipsets) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.IPSets = ipsets
	}) {
		SuccessResponse(w)
	}
}

// handleGetDHCP returns DHCP server configuration
// handleGetDHCP returns DHCP server configuration
func (s *Server) handleGetDHCP(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.DHCP)
	}
}

// handleUpdateDHCP updates DHCP server configuration
func (s *Server) handleUpdateDHCP(w http.ResponseWriter, r *http.Request) {
	var dhcp config.DHCPServer
	if !BindJSONLenient(w, r, &dhcp) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.DHCP = &dhcp
	}) {
		SuccessResponse(w)
	}
}

// handleGetDNS returns DNS server configuration
func (s *Server) handleGetDNS(w http.ResponseWriter, r *http.Request) {
	cfg := s.GetConfigSnapshot(w, r)
	if cfg == nil {
		return
	}

	// Return both for compatibility
	resp := struct {
		Old *config.DNSServer `json:"dns_server,omitempty"`
		New *config.DNS       `json:"dns,omitempty"`
	}{
		Old: cfg.DNSServer,
		New: cfg.DNS,
	}
	WriteJSON(w, http.StatusOK, resp)
}

// handleUpdateDNS updates DNS server configuration
func (s *Server) handleUpdateDNS(w http.ResponseWriter, r *http.Request) {
	// Try to decode as wrapped format first
	var req struct {
		Old *config.DNSServer `json:"dns_server,omitempty"`
		New *config.DNS       `json:"dns,omitempty"`
	}

	// Limit request body to prevent memory exhaustion (10MB max for config)
	const maxConfigSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxConfigSize))
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	// Determine which format was sent
	var updateFn func(cfg *config.Config)

	if err := json.Unmarshal(body, &req); err == nil && (req.Old != nil || req.New != nil) {
		updateFn = func(cfg *config.Config) {
			if req.Old != nil {
				cfg.DNSServer = req.Old
			}
			if req.New != nil {
				cfg.DNS = req.New
			}
			config.ApplyPostLoadMigrations(cfg)
		}
	} else {
		// Try legacy raw DNSServer format
		var old config.DNSServer
		if err := json.Unmarshal(body, &old); err == nil {
			updateFn = func(cfg *config.Config) {
				cfg.DNSServer = &old
				config.ApplyPostLoadMigrations(cfg)
			}
		} else {
			WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid DNS configuration format")
			return
		}
	}

	if s.applyConfigUpdate(w, r, updateFn) {
		WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}

// handleGetRoutes returns static route configuration
// handleGetRoutes returns static route configuration
func (s *Server) handleGetRoutes(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.Routes)
	}
}

// handleUpdateRoutes updates static route configuration
func (s *Server) handleUpdateRoutes(w http.ResponseWriter, r *http.Request) {
	var routes []config.Route
	if !BindJSONLenient(w, r, &routes) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.Routes = routes
	}) {
		SuccessResponse(w)
	}
}

// handleGetZones returns zone configuration
// handleGetZones returns zone configuration
func (s *Server) handleGetZones(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.Zones)
	}
}

// handleUpdateZones updates zone configuration
func (s *Server) handleUpdateZones(w http.ResponseWriter, r *http.Request) {
	var zones []config.Zone
	if !BindJSONLenient(w, r, &zones) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.Zones = zones
	}) {
		SuccessResponse(w)
	}
}

// handleGetProtections returns per-interface protection settings
// handleGetProtections returns per-interface protection settings
func (s *Server) handleGetProtections(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.Protections)
	}
}

// handleUpdateProtections updates per-interface protection settings
func (s *Server) handleUpdateProtections(w http.ResponseWriter, r *http.Request) {
	// Accept either array directly or wrapped in { protections: [...] }
	var wrapper struct {
		Protections []config.InterfaceProtection `json:"protections"`
	}
	const maxConfigSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxConfigSize))
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	var updateFn func(cfg *config.Config)

	// Try parsing as wrapper
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Protections != nil {
		updateFn = func(cfg *config.Config) {
			cfg.Protections = wrapper.Protections
		}
	} else {
		// Try parsing as array
		var protections []config.InterfaceProtection
		if err := json.Unmarshal(body, &protections); err != nil {
			WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
			return
		}
		updateFn = func(cfg *config.Config) {
			cfg.Protections = protections
		}
	}

	if s.applyConfigUpdate(w, r, updateFn) {
		WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}

// handleGetVPN returns the current VPN configuration
// handleGetVPN returns the current VPN configuration
func (s *Server) handleGetVPN(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.VPN)
	}
}

// handleUpdateVPN updates the VPN configuration
func (s *Server) handleUpdateVPN(w http.ResponseWriter, r *http.Request) {
	var newVPN config.VPNConfig
	if !BindJSONLenient(w, r, &newVPN) {
		return
	}
	if s.applyConfigUpdate(w, r, func(cfg *config.Config) {
		cfg.VPN = &newVPN
	}) {
		SuccessResponse(w)
	}
}

// handleWireGuardGenerateKey generates a new WireGuard key pair.
// This is a stateless operation that doesn't require control plane access.
func (s *Server) handleWireGuardGenerateKey(w http.ResponseWriter, r *http.Request) {
	privKey, pubKey, err := vpn.GenerateKeyPair()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to generate key: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"private_key": privKey,
		"public_key":  pubKey,
	})
}

// handleGetQoS returns QoS policies configuration
// handleGetQoS returns QoS policies configuration
func (s *Server) handleGetQoS(w http.ResponseWriter, r *http.Request) {
	if cfg := s.GetConfigSnapshot(w, r); cfg != nil {
		HandleGetData(w, cfg.QoSPolicies)
	}
}

// handleUpdateQoS updates QoS policies configuration
func (s *Server) handleUpdateQoS(w http.ResponseWriter, r *http.Request) {
	// Accept either array directly or wrapped in { qos_policies: [...] }
	var wrapper struct {
		QoSPolicies []config.QoSPolicy `json:"qos_policies"`
	}
	const maxConfigSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxConfigSize))
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to read request body")
		return
	}

	var updateFn func(cfg *config.Config)

	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.QoSPolicies != nil {
		updateFn = func(cfg *config.Config) {
			cfg.QoSPolicies = wrapper.QoSPolicies
		}
	} else {
		var qos []config.QoSPolicy
		if err := json.Unmarshal(body, &qos); err != nil {
			WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
			return
		}
		updateFn = func(cfg *config.Config) {
			cfg.QoSPolicies = qos
		}
	}

	if s.applyConfigUpdate(w, r, updateFn) {
		WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}

// handleConfig returns the current configuration (staged or running)
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	// Support source=running to get raw running config without status
	if r.URL.Query().Get("source") == "running" {
		if s.client != nil {
			type runConfRes struct {
				cfg *config.Config
				err error
			}
			ch := make(chan runConfRes, 1)
			go func() {
				c, e := s.client.GetRunningConfig()
				ch <- runConfRes{c, e}
			}()

			select {
			case res := <-ch:
				if res.err != nil {
					logging.Error(fmt.Sprintf("GetRunningConfig failed: %v", res.err))
					WriteErrorCtx(w, r, http.StatusInternalServerError, res.err.Error())
					return
				}
				if res.cfg == nil {
					logging.Error("GetRunningConfig returned nil config")
					WriteErrorCtx(w, r, http.StatusInternalServerError, "Running config is nil")
					return
				}
				WriteJSON(w, http.StatusOK, res.cfg)
			case <-time.After(5 * time.Second):
				logging.Error("CRITICAL: GetRunningConfig timed out (running)")
				WriteErrorCtx(w, r, http.StatusGatewayTimeout, "timeout waiting for control plane")
			}
		} else {
			WriteJSON(w, http.StatusOK, s.Config)
		}
		return
	}

	// Default: Return config with _status fields for UI
	s.configMu.RLock()
	staged := s.Config
	s.configMu.RUnlock()

	if s.client != nil {
		// Get running config to compute status

		type runConfRes struct {
			cfg *config.Config
			err error
		}
		ch := make(chan runConfRes, 1)
		go func() {
			c, e := s.client.GetRunningConfig()
			ch <- runConfRes{c, e}
		}()

		var running *config.Config
		var err error
		select {
		case res := <-ch:
			running, err = res.cfg, res.err
		case <-time.After(5 * time.Second):
			fmt.Fprintf(os.Stderr, "CRITICAL: GetRunningConfig timed out\n")
			err = fmt.Errorf("timeout waiting for control plane")
		}
		if err != nil {
			logging.Error("GetRunningConfig failed: " + err.Error())
			// Can't reach control plane - return staged without status
			WriteJSON(w, http.StatusOK, staged)
			return
		}
		if running == nil {
			logging.Error("GetRunningConfig returned nil")
			// Can't reach control plane - return staged without status
			WriteJSON(w, http.StatusOK, staged)
			return
		}
		logging.Info("GetRunningConfig success, building status...")
		// Return config with _status fields for each item
		configWithStatus := BuildConfigWithStatus(staged, running)
		logging.Info("Writing JSON response...")
		WriteJSON(w, http.StatusOK, configWithStatus)
	} else {
		// No client - return raw config
		WriteJSON(w, http.StatusOK, staged)
	}
}

// handleGetConfigDiff returns the diff between saved and in-memory config
func (s *Server) handleGetConfigDiff(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Control plane not connected")
		return
	}

	// 1. Get Running Config
	runningCfg, err := s.client.GetRunningConfig()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to fetch running config: "+err.Error())
		return
	}

	// 2. Get Staged Config
	stagedCfg := s.Config

	// 3. Marshal to JSON for comparison
	runningJSON, _ := json.MarshalIndent(runningCfg, "", "  ")
	stagedJSON, _ := json.MarshalIndent(stagedCfg, "", "  ")

	// 4. Generate Diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(runningJSON)),
		B:        difflib.SplitLines(string(stagedJSON)),
		FromFile: "Running",
		ToFile:   "Staged",
		Context:  3,
	}
	text, _ := difflib.GetUnifiedDiffString(diff)

	if text == "" {
		text = "No changes."
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(text))
}

// handleDiscardConfig discards staged changes by reloading from the control plane
func (s *Server) handleDiscardConfig(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Control plane not connected")
		return
	}

	// 1. Tell control plane to discard its staged changes too
	// Wrap in timeout to prevent RPC hangs
	type discardRes struct {
		err error
	}
	discardCh := make(chan discardRes, 1)
	go func() {
		discardCh <- discardRes{err: s.client.DiscardConfig()}
	}()

	select {
	case res := <-discardCh:
		if res.err != nil {
			msg := "Failed to discard staged config: " + res.err.Error()
			WriteErrorCtx(w, r, http.StatusInternalServerError, msg)
			return
		}
	case <-time.After(10 * time.Second):
		WriteErrorCtx(w, r, http.StatusGatewayTimeout, "Control plane RPC timed out during discard")
		return
	}

	// 2. Refresh local config from control plane (which is now back to running config)
	type configRes struct {
		cfg *config.Config
		err error
	}
	configCh := make(chan configRes, 1)
	go func() {
		cfg, err := s.client.GetConfig()
		configCh <- configRes{cfg, err}
	}()

	var cfg *config.Config
	var err error
	select {
	case res := <-configCh:
		cfg, err = res.cfg, res.err
	case <-time.After(5 * time.Second):
		msg := "Control plane RPC timed out during config reload"
		WriteErrorCtx(w, r, http.StatusGatewayTimeout, msg)
		return
	}

	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to reload config: "+err.Error())
		return
	}

	s.configMu.Lock()
	s.Config = cfg
	s.configMu.Unlock()

	// Notify UI
	go s.broadcastPendingStatus()

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handlePendingStatus returns whether there are pending changes
func (s *Server) handlePendingStatus(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"has_changes": false,
			"reason":      "no control plane",
		})
		return
	}

	// Get Running Config
	runningCfg, err := s.client.GetRunningConfig()
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"has_changes": false,
			"reason":      "failed to get running config",
		})
		return
	}

	// Compare by JSON serialization
	runningJSON, _ := json.Marshal(runningCfg)
	stagedJSON, _ := json.Marshal(s.Config)

	hasChanges := string(runningJSON) != string(stagedJSON)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"has_changes": hasChanges,
	})
}

// handleConfigSalvage returns details about configuration salvage if normal load failed
func (s *Server) handleConfigSalvage(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Control plane not connected")
		return
	}

	res, err := s.client.GetForgivingResult()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to fetch salvage result: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, res)
}
