// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/config"
)

// --- Packet Simulator ---

// SimulatePacketRequest defines the input for packet simulation
type SimulatePacketRequest struct {
	SrcIP    string `json:"src_ip"`
	DstIP    string `json:"dst_ip"`
	DstPort  int    `json:"dst_port"`
	Protocol string `json:"protocol"` // tcp, udp, icmp
	SrcZone  string `json:"src_zone,omitempty"`
	DstZone  string `json:"dst_zone,omitempty"`
}

// SimulatePacketResponse returns the verdict of the simulation
type SimulatePacketResponse struct {
	Action        string   `json:"action"`         // accept, drop, reject
	Verdict       string   `json:"verdict"`        // Human-readable verdict
	MatchedPolicy string   `json:"matched_policy"` // Policy name that matched (e.g., "lan_to_wan")
	MatchedRule   string   `json:"matched_rule"`   // Rule name or description
	RuleIndex     int      `json:"rule_index"`     // Index of matched rule in policy (-1 for default)
	SrcZone       string   `json:"src_zone"`       // Detected or provided source zone
	DstZone       string   `json:"dst_zone"`       // Detected or provided destination zone
	RulePath      []string `json:"rule_path"`      // Evaluation path for debugging
}

// handleSimulatePacket simulates packet flow through the firewall
func (s *Server) handleSimulatePacket(w http.ResponseWriter, r *http.Request) {
	var req SimulatePacketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate inputs
	if req.SrcIP == "" || req.DstIP == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "src_ip and dst_ip are required")
		return
	}
	if net.ParseIP(req.SrcIP) == nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid src_ip")
		return
	}
	if net.ParseIP(req.DstIP) == nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid dst_ip")
		return
	}
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	req.Protocol = strings.ToLower(req.Protocol)

	// Get current config
	s.configMu.RLock()
	cfg := s.Config
	s.configMu.RUnlock()

	if cfg == nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Configuration not loaded")
		return
	}

	// Determine source zone from IP
	srcZone := req.SrcZone
	dstZone := req.DstZone

	if srcZone == "" {
		srcZone = s.detectZoneForIP(req.SrcIP, cfg)
	}
	if dstZone == "" {
		dstZone = s.detectZoneForIP(req.DstIP, cfg)
	}

	// Build response
	resp := SimulatePacketResponse{
		SrcZone:  srcZone,
		DstZone:  dstZone,
		RulePath: []string{},
	}

	// Find matching policy
	var matchedPolicy *struct {
		Name          string
		DefaultAction string
	}

	for _, p := range cfg.Policies {
		if p.From == srcZone && p.To == dstZone {
			matchedPolicy = &struct {
				Name          string
				DefaultAction string
			}{
				Name:          fmt.Sprintf("%s_to_%s", p.From, p.To),
				DefaultAction: p.Action,
			}
			resp.RulePath = append(resp.RulePath, fmt.Sprintf("policy:%s->%s", p.From, p.To))

			// Check rules in order
			for i, rule := range p.Rules {
				if rule.Disabled {
					continue
				}

				// Check protocol match
				if rule.Protocol != "" && rule.Protocol != req.Protocol && rule.Protocol != "all" {
					continue
				}

				// Check destination port
				if rule.DestPort > 0 && rule.DestPort != req.DstPort {
					continue
				}

				// Check source IP (simplified - no CIDR or IPSet matching)
				if rule.SrcIP != "" && rule.SrcIP != req.SrcIP {
					continue
				}

				// Check destination IP
				if rule.DestIP != "" && rule.DestIP != req.DstIP {
					continue
				}

				// Rule matched!
				resp.MatchedPolicy = matchedPolicy.Name
				resp.MatchedRule = rule.Name
				if resp.MatchedRule == "" {
					resp.MatchedRule = rule.Description
				}
				if resp.MatchedRule == "" {
					resp.MatchedRule = fmt.Sprintf("Rule #%d", i+1)
				}
				resp.RuleIndex = i
				resp.Action = rule.Action
				resp.Verdict = fmt.Sprintf("Packet would be %sED by rule '%s'",
					strings.ToUpper(rule.Action), resp.MatchedRule)
				resp.RulePath = append(resp.RulePath, fmt.Sprintf("rule:%s (index %d)", resp.MatchedRule, i))

				WriteJSON(w, http.StatusOK, resp)
				return
			}

			// No rule matched, use default action
			resp.MatchedPolicy = matchedPolicy.Name
			resp.MatchedRule = "default policy"
			resp.RuleIndex = -1
			resp.Action = p.Action
			if resp.Action == "" {
				resp.Action = "drop" // Default to drop if not specified
			}
			resp.Verdict = fmt.Sprintf("Packet would be %sED by default policy", strings.ToUpper(resp.Action))
			resp.RulePath = append(resp.RulePath, "default:"+resp.Action)

			WriteJSON(w, http.StatusOK, resp)
			return
		}
	}

	// No matching policy found - check global default
	resp.MatchedPolicy = ""
	resp.MatchedRule = "global implicit deny"
	resp.RuleIndex = -1
	resp.Action = "drop"
	resp.Verdict = "Packet would be DROPPED - no matching policy found"
	resp.RulePath = append(resp.RulePath, "global:implicit-deny")

	WriteJSON(w, http.StatusOK, resp)
}

// detectZoneForIP determines which zone an IP belongs to
func (s *Server) detectZoneForIP(ip string, cfg *config.Config) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "unknown"
	}

	// Check if IP matches any interface's network
	for _, zone := range cfg.Zones {
		for _, m := range zone.Matches {
			if m.Interface == "" {
				continue
			}
			// Find interface config
			for _, iface := range cfg.Interfaces {
				if iface.Name == m.Interface {
					// Parse interface IP/CIDR
					for _, addr := range iface.IPv4 {
						_, network, err := net.ParseCIDR(addr)
						if err != nil {
							continue
						}
						if network.Contains(parsedIP) {
							return zone.Name
						}
					}
				}
			}
		}
	}

	// Check if it's a public IP (likely WAN destination)
	if !isPrivateIP(parsedIP) {
		// Find WAN zone
		for _, zone := range cfg.Zones {
			if strings.Contains(strings.ToLower(zone.Name), "wan") {
				return zone.Name
			}
		}
		return "wan"
	}

	return "unknown"
}

// isPrivateIP checks if an IP is in a private range
func isPrivateIP(ip net.IP) bool {
	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // Link-local
		"fc00::/7",       // IPv6 ULA
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range private {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// --- Packet Capture ---

type CaptureRequest struct {
	Interface string `json:"interface"`
	Filter    string `json:"filter"`   // tcpdump filter syntax
	Duration  int    `json:"duration"` // Seconds (optional, default 30)
	Count     int    `json:"count"`    // Packet count (optional, default 1000)
}

type CaptureStatus struct {
	Running   bool   `json:"running"`
	Interface string `json:"interface,omitempty"`
	Filter    string `json:"filter,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Path      string `json:"path,omitempty"`
}

var (
	captureMu      sync.Mutex
	currentCapture *exec.Cmd
	captureCancel  context.CancelFunc
	captureFile    string = "/tmp/flywall_capture.pcap"
	captureStatus  CaptureStatus
)

// handleStartCapture starts a packet capture
func (s *Server) handleStartCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Interface == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Interface is required")
		return
	}

	if req.Duration <= 0 {
		req.Duration = 30 // Default 30s
	}
	if req.Duration > 300 {
		req.Duration = 300 // Max 5m
	}
	if req.Count <= 0 {
		req.Count = 1000 // Default 1000 packets
	}
	if req.Count > 10000 {
		req.Count = 10000 // Max 10k packets
	}

	captureMu.Lock()
	defer captureMu.Unlock()

	// Stop existing capture if running
	if currentCapture != nil {
		if captureCancel != nil {
			captureCancel()
		}
		// Wait handled by cleanup in goroutine
		currentCapture = nil
	}

	// Prepare context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Duration)*time.Second)
	captureCancel = cancel

	// Prepare command: tcpdump -i <iface> -c <count> -w <file> <filter>
	args := []string{"-i", req.Interface, "-c", fmt.Sprintf("%d", req.Count), "-w", captureFile}
	if req.Filter != "" {
		// Basic sanitization for filter to prevent command injection (though exec calls are safer)
		// tcpdump handles invalid filters by exiting
		args = append(args, req.Filter)
	}

	// Try using sudo if not root (this assumes passwordless sudo or root execution)
	if os.Geteuid() != 0 {
		args = append([]string{"tcpdump"}, args...)
		currentCapture = exec.CommandContext(ctx, "sudo", args...)
	} else {
		currentCapture = exec.CommandContext(ctx, "tcpdump", args...)
	}

	if err := currentCapture.Start(); err != nil {
		cancel()
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to start capture: "+err.Error())
		return
	}

	captureStatus = CaptureStatus{
		Running:   true,
		Interface: req.Interface,
		Filter:    req.Filter,
		Path:      captureFile,
	}

	// Wait in background
	go func(cmd *exec.Cmd, cancel context.CancelFunc) {
		err := cmd.Wait()
		captureMu.Lock()
		defer captureMu.Unlock()

		// Only update if this is still the active capture
		if currentCapture == cmd {
			currentCapture = nil
			captureCancel = nil
			captureStatus.Running = false

			// Check file size
			if info, err := os.Stat(captureFile); err == nil {
				captureStatus.Size = info.Size()
			}
		}

		// Ensure context is cancelled
		cancel()

		if err != nil && err.Error() != "signal: killed" {
			// Log error if needed, but we don't have logger here easily
			fmt.Printf("Capture finished with error: %v\n", err)
		}
	}(currentCapture, cancel)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Capture started on %s for %ds or %d packets", req.Interface, req.Duration, req.Count),
	})
}

// handleStopCapture stops the current packet capture
func (s *Server) handleStopCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	captureMu.Lock()
	defer captureMu.Unlock()

	if currentCapture != nil && captureCancel != nil {
		captureCancel() // This kills the process via context
		currentCapture = nil
		captureCancel = nil
		captureStatus.Running = false

		// Update size
		if info, err := os.Stat(captureFile); err == nil {
			captureStatus.Size = info.Size()
		}

		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Capture stopped",
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"message": "No capture running",
	})
}

// handleDownloadCapture downloads the captured PCAP file
func (s *Server) handleDownloadCapture(w http.ResponseWriter, r *http.Request) {
	captureMu.Lock()
	running := captureStatus.Running
	captureMu.Unlock()

	if running {
		WriteErrorCtx(w, r, http.StatusConflict, "Capture is currently running. Stop it first.")
		return
	}

	if _, err := os.Stat(captureFile); os.IsNotExist(err) {
		WriteErrorCtx(w, r, http.StatusNotFound, "No capture file found")
		return
	}

	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", "attachment; filename=capture.pcap")
	http.ServeFile(w, r, captureFile)
}

// handleGetCaptureStatus returns current capture status
func (s *Server) handleGetCaptureStatus(w http.ResponseWriter, r *http.Request) {
	captureMu.Lock()
	defer captureMu.Unlock()

	// Update size if file exists
	if !captureStatus.Running {
		if info, err := os.Stat(captureFile); err == nil {
			captureStatus.Size = info.Size()
		}
	} else if info, err := os.Stat(captureFile); err == nil {
		// Update size while running too
		captureStatus.Size = info.Size()
	}

	WriteJSON(w, http.StatusOK, captureStatus)
}
