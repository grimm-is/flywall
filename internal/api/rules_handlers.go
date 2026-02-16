// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/firewall"
	"grimm.is/flywall/internal/stats"
)

// RulesHandler provides enriched rule data for the ClearPath Policy Editor.
type RulesHandler struct {
	server    *Server
	collector *stats.Collector
	device    DeviceLookup // Optional device lookup for IP resolution
}

// NewRulesHandler creates a new rules handler.
func NewRulesHandler(s *Server, collector *stats.Collector, device DeviceLookup) *RulesHandler {
	return &RulesHandler{
		server:    s,
		collector: collector,
		device:    device,
	}
}

// HandleGetRuleGroups returns a list of unique GroupTag values for filtering.
func (h *RulesHandler) HandleGetRuleGroups(w http.ResponseWriter, r *http.Request) {
	if h.server.Config == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Configuration not loaded")
		return
	}

	groups := make(map[string]int)
	for _, pol := range h.server.Config.Policies {
		for _, rule := range pol.Rules {
			if rule.GroupTag != "" {
				groups[rule.GroupTag]++
			}
		}
	}

	type GroupInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	response := make([]GroupInfo, 0, len(groups))
	for name, count := range groups {
		response = append(response, GroupInfo{Name: name, Count: count})
	}

	WriteJSON(w, http.StatusOK, response)
}

// RegisterRoutes registers the rules API routes.
func (h *RulesHandler) RegisterRoutes(mux *http.ServeMux, require func(perm string, h http.HandlerFunc) http.Handler) {
	mux.Handle("GET /api/rules", require("read:firewall", http.HandlerFunc(h.HandleGetRules)))
	mux.Handle("GET /api/rules/flat", require("read:firewall", http.HandlerFunc(h.HandleGetFlatRules)))
	mux.Handle("GET /api/rules/groups", require("read:firewall", http.HandlerFunc(h.HandleGetRuleGroups)))
}

// RegisterRoutesNoAuth registers routes without authentication (for dev/test mode).
func (h *RulesHandler) RegisterRoutesNoAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/rules", h.HandleGetRules)
	mux.HandleFunc("GET /api/rules/flat", h.HandleGetFlatRules)
	mux.HandleFunc("GET /api/rules/groups", h.HandleGetRuleGroups)
}

// synthesizeImplicitRules generates implicit rules from Zone Management and Services.
func (h *RulesHandler) synthesizeImplicitRules() []RuleWithStats {
	var rules []RuleWithStats
	if h.server.Config == nil {
		return rules
	}

	appendRule := func(zone string, idSuffix, name, service, desc string) {
		rule := config.PolicyRule{
			ID:          "implicit-" + idSuffix + "-" + zone + "-" + service,
			Name:        name,
			Action:      "accept",
			Service:     service,
			Protocol:    "tcp", // Simplified default
			Description: desc,
			Origin:      "implicit_zone_config",
			Disabled:    false,
		}

		rws := RuleWithStats{
			PolicyRule: rule,
			PolicyFrom: zone,
			PolicyTo:   "Firewall (Self)",
			Stats: RuleStats{
				SparklineData: []float64{},
			},
		}
		rules = append(rules, rws)
	}

	for _, zone := range h.server.Config.Zones {
		// Zone Management (Input to Firewall)
		if zone.Management != nil {
			if zone.Management.Web || zone.Management.WebUI {
				appendRule(zone.Name, "mgmt-web", "Allow Web UI", "https", "Zone "+zone.Name+" management: Web UI")
			}
			if zone.Management.SSH {
				appendRule(zone.Name, "mgmt-ssh", "Allow SSH", "ssh", "Zone "+zone.Name+" management: SSH")
			}
			if zone.Management.API {
				appendRule(zone.Name, "mgmt-api", "Allow API", "https", "Zone "+zone.Name+" management: API")
			}
			if zone.Management.ICMP {
				// ICMP is special, "ping" service macro or protocol icmp
				rule := config.PolicyRule{
					ID:          "implicit-mgmt-icmp-" + zone.Name,
					Name:        "Allow Ping",
					Action:      "accept",
					Protocol:    "icmp",
					Description: "Zone " + zone.Name + " management: ICMP Ping",
					Origin:      "implicit_zone_config",
					Disabled:    false,
				}
				rws := RuleWithStats{
					PolicyRule: rule,
					PolicyFrom: zone.Name,
					PolicyTo:   "Firewall (Self)",
					Stats:      RuleStats{SparklineData: []float64{}},
				}
				rules = append(rules, rws)
			}
			if zone.Management.SNMP {
				appendRule(zone.Name, "mgmt-snmp", "Allow SNMP", "snmp", "Zone "+zone.Name+" management: SNMP")
			}
			if zone.Management.Syslog {
				appendRule(zone.Name, "mgmt-syslog", "Allow Syslog", "syslog", "Zone "+zone.Name+" management: Syslog")
			}
		}

		// Zone Services (Input to Firewall)
		if zone.Services != nil {
			if zone.Services.DHCP {
				appendRule(zone.Name, "svc", "Allow DHCP", "dhcp", "Zone "+zone.Name+" services: DHCP")
			}
			if zone.Services.DNS {
				appendRule(zone.Name, "svc", "Allow DNS", "dns", "Zone "+zone.Name+" services: DNS")
			}
			if zone.Services.NTP {
				appendRule(zone.Name, "svc", "Allow NTP", "ntp", "Zone "+zone.Name+" services: NTP")
			}
			if zone.Services.CaptivePortal {
				appendRule(zone.Name, "svc", "Allow Captive Portal", "http", "Zone "+zone.Name+" services: Captive Portal")
			}

			// Custom Ports
			for _, cp := range zone.Services.CustomPorts {
				rule := config.PolicyRule{
					ID:          "implicit-svc-" + zone.Name + "-custom-" + cp.Name,
					Name:        "Allow " + cp.Name,
					Action:      "accept",
					Protocol:    cp.Protocol,
					DestPort:    cp.Port,
					Description: "Zone " + zone.Name + " services: " + cp.Name,
					Origin:      "implicit_zone_config",
					Disabled:    false,
				}
				if cp.EndPort > 0 {
					// We'd need to handle ranges, but for now single port is fine or ignore endport in simple display
				}

				rws := RuleWithStats{
					PolicyRule: rule,
					PolicyFrom: zone.Name,
					PolicyTo:   "Firewall (Self)",
					Stats:      RuleStats{SparklineData: []float64{}},
				}
				rules = append(rules, rws)
			}
		}
	}

	// Implicit Router -> Zone Output Rules
	// (Automatic Default Allow for router egress to all zones)
	for _, zone := range h.server.Config.Zones {
		rule := config.PolicyRule{
			ID:          "implicit-" + brand.LowerName + "-" + zone.Name,
			Name:        fmt.Sprintf("Allow %s Output to %s", brand.Name, zone.Name),
			Action:      "accept",
			Protocol:    "all",
			Description: fmt.Sprintf("Automatically generated: Allow %s to access %s", brand.Name, zone.Name),
			Origin:      "implicit_" + brand.LowerName + "_output",
			Disabled:    false,
		}
		rws := RuleWithStats{
			PolicyRule: rule,
			PolicyFrom: brand.Name,
			PolicyTo:   zone.Name,
			Stats:      RuleStats{SparklineData: []float64{}},
		}
		rules = append(rules, rws)
	}

	return rules
}

// HandleGetRules returns all policies with their rules.
// If ?with_stats=true, rules are enriched with runtime statistics and alias resolution.
func (h *RulesHandler) HandleGetRules(w http.ResponseWriter, r *http.Request) {
	if h.server.Config == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Configuration not loaded")
		return
	}

	withStats := r.URL.Query().Get("with_stats") == "true"
	resolver := NewAliasResolver(h.server.Config, h.device)

	// Collect all policies with enriched rules
	var response []PolicyWithStats

	// 1. Explicit Policies
	for _, pol := range h.server.Config.Policies {
		polWithStats := PolicyWithStats{
			Policy: pol,
			Rules:  make([]RuleWithStats, 0, len(pol.Rules)),
		}

		for _, rule := range pol.Rules {
			enriched := RuleWithStats{
				PolicyRule: rule,
				PolicyFrom: pol.From,
				PolicyTo:   pol.To,
			}

			// Resolve aliases for UI pills
			enriched.ResolvedSrc = resolver.ResolveSource(rule)
			enriched.ResolvedDest = resolver.ResolveDest(rule)

			// Add stats if requested
			if withStats && h.collector != nil {
				ruleID := rule.ID
				if ruleID == "" {
					ruleID = rule.Name
				}

				if ruleID != "" {
					enriched.Stats.SparklineData = h.collector.GetSparkline(ruleID)
					enriched.Stats.Bytes = h.collector.GetTotalBytes(ruleID)
				}
			}

			// Generate nft syntax for power users
			timezone := "UTC"
			if h.server.Config.System != nil && h.server.Config.System.Timezone != "" {
				timezone = h.server.Config.System.Timezone
			}
			if nftSyntax, err := firewall.BuildRuleExpression(rule, timezone); err == nil {
				enriched.GeneratedSyntax = nftSyntax
			}

			polWithStats.Rules = append(polWithStats.Rules, enriched)
		}

		response = append(response, polWithStats)
	}

	// 2. Synthesize Implicit Rules (Virtual Policies)
	implicitRules := h.synthesizeImplicitRules()
	if len(implicitRules) > 0 {
		// Group by From/To to create virtual policies
		virtualPolicies := make(map[string]*PolicyWithStats)

		// Use a stable order map to preserve order if needed, but iteration over map is random.
		// For implicit rules, splitting by zone is key.

		for _, rule := range implicitRules {
			key := rule.PolicyFrom + "->" + rule.PolicyTo
			if _, exists := virtualPolicies[key]; !exists {
				virtualPolicies[key] = &PolicyWithStats{
					Policy: config.Policy{
						From:        rule.PolicyFrom,
						To:          rule.PolicyTo,
						Name:        "Implicit Rules (" + rule.PolicyFrom + ")",
						Description: "Automatically generated from Zone settings",
						Origin:      "implicit_zone_config",
					},
					Rules: []RuleWithStats{},
				}
			}
			virtualPolicies[key].Rules = append(virtualPolicies[key].Rules, rule)
		}

		// Append virtual policies to response
		// To ensure deterministic order, we could sort, but simplistic append is okay for now
		// or sorting by zone name.
		for _, pol := range virtualPolicies {
			response = append(response, *pol)
		}
	}

	WriteJSON(w, http.StatusOK, response)
}

// HandleGetFlatRules returns all rules flattened (without policy grouping).
// Useful for the unified rule table view.
// Query params:
//   - with_stats=true: Include sparkline data
//   - group=string: Filter by rule GroupTag
//   - limit=int: Max rules to return
func (h *RulesHandler) HandleGetFlatRules(w http.ResponseWriter, r *http.Request) {
	if h.server.Config == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Configuration not loaded")
		return
	}

	withStats := r.URL.Query().Get("with_stats") == "true"
	groupFilter := r.URL.Query().Get("group")
	limitStr := r.URL.Query().Get("limit")
	limit := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	resolver := NewAliasResolver(h.server.Config, h.device)
	var response []RuleWithStats
	count := 0

	// Helper to process a rule
	processRule := func(rule config.PolicyRule, from, to string) {
		// Apply group filter
		if groupFilter != "" && rule.GroupTag != groupFilter {
			return
		}

		enriched := RuleWithStats{
			PolicyRule: rule,
			PolicyFrom: from,
			PolicyTo:   to,
		}

		// Resolve aliases
		enriched.ResolvedSrc = resolver.ResolveSource(rule)
		enriched.ResolvedDest = resolver.ResolveDest(rule)

		// Add stats if requested
		if withStats && h.collector != nil {
			ruleID := rule.ID
			if ruleID == "" {
				ruleID = rule.Name
			}

			if ruleID != "" {
				enriched.Stats.SparklineData = h.collector.GetSparkline(ruleID)
				enriched.Stats.Bytes = h.collector.GetTotalBytes(ruleID)
			}
		}

		// Generate nft syntax for power users
		timezone := "UTC"
		if h.server.Config.System != nil && h.server.Config.System.Timezone != "" {
			timezone = h.server.Config.System.Timezone
		}
		if nftSyntax, err := firewall.BuildRuleExpression(rule, timezone); err == nil {
			enriched.GeneratedSyntax = nftSyntax
		}

		response = append(response, enriched)
		count++
	}

	// 1. Explicit Policies
	for _, pol := range h.server.Config.Policies {
		for _, rule := range pol.Rules {
			processRule(rule, pol.From, pol.To)
			if limit > 0 && count >= limit {
				break
			}
		}
		if limit > 0 && count >= limit {
			break
		}
	}

	// 2. Implicit Rules
	if limit == 0 || count < limit {
		implicitRules := h.synthesizeImplicitRules()
		for _, rule := range implicitRules {
			processRule(rule.PolicyRule, rule.PolicyFrom, rule.PolicyTo)
			if limit > 0 && count >= limit {
				break
			}
		}
	}

	WriteJSON(w, http.StatusOK, response)
}

// handlePolicyReorder reorders policies
func (s *Server) handlePolicyReorder(w http.ResponseWriter, r *http.Request) {
	// Method check removed (handled by router)

	var req struct {
		PolicyName string   `json:"policy_name"`         // Policy to move
		Position   string   `json:"position"`            // "before" or "after"
		RelativeTo string   `json:"relative_to"`         // Target policy name
		NewOrder   []string `json:"new_order,omitempty"` // Or provide complete new order
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// If new_order is provided, use it directly
	if len(req.NewOrder) > 0 {
		policyMap := make(map[string]config.Policy)
		for _, p := range s.Config.Policies {
			policyMap[p.Name] = p
		}

		newPolicies := make([]config.Policy, 0, len(req.NewOrder))
		for _, name := range req.NewOrder {
			if p, ok := policyMap[name]; ok {
				newPolicies = append(newPolicies, p)
			}
		}
		s.Config.Policies = newPolicies
		WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Otherwise, move single policy relative to another
	if req.PolicyName == "" || req.RelativeTo == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "policy_name and relative_to are required")
		return
	}

	// Find indices
	var moveIdx, targetIdx int = -1, -1
	for i, p := range s.Config.Policies {
		if p.Name == req.PolicyName {
			moveIdx = i
		}
		if p.Name == req.RelativeTo {
			targetIdx = i
		}
	}

	if moveIdx == -1 || targetIdx == -1 {
		WriteErrorCtx(w, r, http.StatusNotFound, "Policy not found")
		return
	}

	// Remove policy from current position
	policy := s.Config.Policies[moveIdx]
	policies := append(s.Config.Policies[:moveIdx], s.Config.Policies[moveIdx+1:]...)

	// Adjust target index if needed
	if moveIdx < targetIdx {
		targetIdx--
	}

	// Insert at new position
	insertIdx := targetIdx
	if req.Position == "after" {
		insertIdx++
	}

	// Insert
	newPolicies := make([]config.Policy, 0, len(policies)+1)
	newPolicies = append(newPolicies, policies[:insertIdx]...)
	newPolicies = append(newPolicies, policy)
	newPolicies = append(newPolicies, policies[insertIdx:]...)

	s.configMu.Lock()
	s.Config.Policies = newPolicies
	s.configMu.Unlock()

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleRuleReorder reorders rules within a policy
func (s *Server) handleRuleReorder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		PolicyName string   `json:"policy_name"`         // Policy containing the rules
		RuleName   string   `json:"rule_name"`           // Rule to move
		Position   string   `json:"position"`            // "before" or "after"
		RelativeTo string   `json:"relative_to"`         // Target rule name
		NewOrder   []string `json:"new_order,omitempty"` // Or provide complete new order
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PolicyName == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "policy_name is required")
		return
	}

	// Find the policy
	var policyIdx int = -1
	s.configMu.RLock()
	for i, p := range s.Config.Policies {
		if p.Name == req.PolicyName {
			policyIdx = i
			break
		}
	}
	s.configMu.RUnlock()

	if policyIdx == -1 {
		WriteErrorCtx(w, r, http.StatusNotFound, "Policy not found")
		return
	}

	policy := &s.Config.Policies[policyIdx]

	// If new_order is provided, use it directly
	if len(req.NewOrder) > 0 {
		ruleMap := make(map[string]config.PolicyRule)
		for _, r := range policy.Rules {
			ruleMap[r.Name] = r
		}

		newRules := make([]config.PolicyRule, 0, len(req.NewOrder))
		for _, name := range req.NewOrder {
			if r, ok := ruleMap[name]; ok {
				newRules = append(newRules, r)
			}
		}
		policy.Rules = newRules
		WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Otherwise, move single rule relative to another
	if req.RuleName == "" || req.RelativeTo == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "rule_name and relative_to are required")
		return
	}

	// Find indices
	var moveIdx, targetIdx int = -1, -1
	for i, r := range policy.Rules {
		if r.Name == req.RuleName {
			moveIdx = i
		}
		if r.Name == req.RelativeTo {
			targetIdx = i
		}
	}

	if moveIdx == -1 || targetIdx == -1 {
		WriteErrorCtx(w, r, http.StatusNotFound, "Rule not found")
		return
	}

	// Remove rule from current position
	rule := policy.Rules[moveIdx]
	rules := append(policy.Rules[:moveIdx], policy.Rules[moveIdx+1:]...)

	// Adjust target index if needed
	if moveIdx < targetIdx {
		targetIdx--
	}

	// Insert at new position
	insertIdx := targetIdx
	if req.Position == "after" {
		insertIdx++
	}

	// Insert
	newRules := make([]config.PolicyRule, 0, len(rules)+1)
	newRules = append(newRules, rules[:insertIdx]...)
	newRules = append(newRules, rule)
	newRules = append(newRules, rules[insertIdx:]...)
	policy.Rules = newRules

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}
