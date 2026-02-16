// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/health"
	"grimm.is/flywall/internal/metrics"
)

// ==============================================================================
// Monitoring Handlers
// ==============================================================================

// MonitoringOverview is the combined monitoring data response.
type MonitoringOverview struct {
	Timestamp  int64                              `json:"timestamp"`
	System     *metrics.SystemStats               `json:"system"`
	Interfaces map[string]*metrics.InterfaceStats `json:"interfaces"`
	Policies   map[string]*metrics.PolicyStats    `json:"policies"`
	Services   *metrics.ServiceStats              `json:"services"`
	Conntrack  *metrics.ConntrackStats            `json:"conntrack"`
}

// handleMonitoringOverview returns all monitoring data in one request.
func (s *Server) handleMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	overview := MonitoringOverview{
		Timestamp:  s.collector.GetLastUpdate().Unix(),
		System:     s.collector.GetSystemStats(),
		Interfaces: s.collector.GetInterfaceStats(),
		Policies:   s.collector.GetPolicyStats(),
		Services:   s.collector.GetServiceStats(),
		Conntrack:  s.collector.GetConntrackStats(),
	}

	WriteJSON(w, http.StatusOK, overview)
}

// handleMonitoringInterfaces returns interface traffic statistics.
func (s *Server) handleMonitoringInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := s.collector.GetInterfaceStats()

	// Convert to sorted slice for consistent ordering
	type ifaceWithStats struct {
		*metrics.InterfaceStats
	}
	result := make([]ifaceWithStats, 0, len(stats))
	for _, stat := range stats {
		result = append(result, ifaceWithStats{stat})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp":  s.collector.GetLastUpdate().Unix(),
		"interfaces": result,
	})
}

// handleMonitoringPolicies returns firewall policy statistics.
func (s *Server) handleMonitoringPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := s.collector.GetPolicyStats()

	// Convert to slice
	result := make([]*metrics.PolicyStats, 0, len(stats))
	for _, stat := range stats {
		result = append(result, stat)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp": s.collector.GetLastUpdate().Unix(),
		"policies":  result,
	})
}

// handleMonitoringServices returns service statistics (DHCP, DNS).
func (s *Server) handleMonitoringServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := s.collector.GetServiceStats()

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp": s.collector.GetLastUpdate().Unix(),
		"services":  stats,
	})
}

// handleMonitoringSystem returns system statistics (CPU, memory, load).
func (s *Server) handleMonitoringSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := s.collector.GetSystemStats()

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp": s.collector.GetLastUpdate().Unix(),
		"system":    stats,
	})
}

// handleMonitoringConntrack returns connection tracking statistics.
func (s *Server) handleMonitoringConntrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := s.collector.GetConntrackStats()

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp": s.collector.GetLastUpdate().Unix(),
		"conntrack": stats,
	})
}

// getServerInfo returns comprehensive server information.
func (s *Server) getServerInfo() ServerInfo {
	uptime := time.Since(s.startTime)

	info := ServerInfo{
		Status:       "online",
		Uptime:       uptime.String(),
		StartTime:    s.startTime.Format(time.RFC3339),
		Version:      brand.Version,
		BuildTime:    brand.BuildTime,
		BuildArch:    brand.BuildArch,
		GitCommit:    brand.GitCommit,
		GitBranch:    brand.GitBranch,
		GitMergeBase: brand.GitMergeBase,
	}

	// Get host uptime from /proc/uptime (Linux)
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if seconds, err := strconv.ParseFloat(fields[0], 64); err == nil {
				info.HostUptime = time.Duration(seconds * float64(time.Second)).String()
			}
		}
	}

	return info
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Build comprehensive server info
	info := s.getServerInfo()

	// Use RPC client if available for control plane status
	if s.client != nil {
		type rpcResult struct {
			status   *ctlplane.Status
			sysStats *ctlplane.SystemStats
			ifaces   []ctlplane.InterfaceStatus
			monitors []ctlplane.MonitorResult
			blocked  int64
			err      error
		}

		// Perform RPC calls in a goroutine with timeout
		ch := make(chan rpcResult, 1)
		go func() {
			res := rpcResult{}
			// 1. Basic Status
			res.status, _ = s.client.GetStatus()
			// 2. System Stats
			res.sysStats, _ = s.client.GetSystemStats()
			// 3. Interfaces (for WAN IP)
			res.ifaces, _ = s.client.GetInterfaces()
			// 4. Blocked Count
			if ips, err := s.client.GetIPSetElements("blocked_ips"); err == nil {
				res.blocked = int64(len(ips))
			}
			// 5. Monitors
			res.monitors, _ = s.client.GetMonitors()
			ch <- res
		}()

		select {
		case res := <-ch:
			if res.status != nil {
				info.Uptime = res.status.Uptime
				info.FirewallActive = res.status.FirewallActive
			}
			if res.sysStats != nil {
				info.CPULoad = res.sysStats.CPUUsage
				if res.sysStats.MemoryTotal > 0 {
					info.MemUsage = float64(res.sysStats.MemoryUsed) / float64(res.sysStats.MemoryTotal) * 100
				}
			}
			if res.monitors != nil {
				info.Monitors = res.monitors
			}
			if res.ifaces != nil {
				var wanIfaceName string
				s.configMu.RLock()
				if s.Config != nil {
					for _, iface := range s.Config.Interfaces {
						if strings.ToUpper(iface.Zone) == "WAN" {
							wanIfaceName = iface.Name
							break
						}
					}
				}
				s.configMu.RUnlock()

				if wanIfaceName != "" {
					for _, iface := range res.ifaces {
						if iface.Name == wanIfaceName {
							if len(iface.IPv4Addrs) > 0 {
								info.WanIP = iface.IPv4Addrs[0]
							}
							break
						}
					}
				} else {
					for _, iface := range res.ifaces {
						if iface.Name == "eth0" || iface.Name == "wan" {
							if len(iface.IPv4Addrs) > 0 {
								info.WanIP = iface.IPv4Addrs[0]
							}
							break
						}
					}
				}
			}
			info.BlockedCount = res.blocked

		case <-time.After(3 * time.Second):
			// Log timeout and return partial info
			fmt.Println("Warning: RPC timeout in handleStatus")
		}
	}

	WriteJSON(w, http.StatusOK, info)
}

// ServerInfo contains comprehensive information about the server.
type ServerInfo struct {
	Status          string  `json:"status"`
	Uptime          string  `json:"uptime"`                     // Router process uptime
	HostUptime      string  `json:"host_uptime,omitempty"`      // System uptime
	StartTime       string  `json:"start_time"`                 // When router started
	Version         string  `json:"version"`                    // Software version
	BuildTime       string  `json:"build_time,omitempty"`       // When binary was built
	BuildArch       string  `json:"build_arch,omitempty"`       // Build architecture
	GitCommit       string  `json:"git_commit,omitempty"`       // Git commit hash
	GitBranch       string  `json:"git_branch,omitempty"`       // Git branch name
	GitMergeBase    string  `json:"git_merge_base,omitempty"`   // Git merge-base with main
	FirewallActive  bool    `json:"firewall_active"`            // Whether firewall is running
	FirewallApplied string  `json:"firewall_applied,omitempty"` // When rules were last applied
	WanIP           string  `json:"wan_ip,omitempty"`           // WAN IP Address
	BlockedCount    int64   `json:"blocked_count"`              // Total blocked packets (last 24h/session)
	CPULoad         float64 `json:"cpu_load"`                   // CPU Usage %
	MemUsage        float64 `json:"mem_usage"`                  // Memory Usage %
	Monitors        []ctlplane.MonitorResult `json:"monitors,omitempty"` // Connectivity monitors
}

// handleHealth returns the overall health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	checker := health.NewChecker()

	checker.Register("nftables", func(ctx context.Context) health.Check {
		if s.healthy.Load() {
			return health.Check{
				Name:    "nftables",
				Status:  health.StatusHealthy,
				Message: "NFTables is responding",
			}
		}
		return health.Check{
			Name:    "nftables",
			Status:  health.StatusUnhealthy,
			Message: "NFTables is unresponsive",
		}
	})

	checker.Register("control-plane", func(ctx context.Context) health.Check {
		if s.client == nil {
			return health.Check{
				Name:    "control-plane",
				Status:  health.StatusDegraded,
				Message: "Control plane client not initialized",
			}
		}

		_, err := s.client.GetStatus()
		if err != nil {
			return health.Check{
				Name:    "control-plane",
				Status:  health.StatusUnhealthy,
				Message: fmt.Sprintf("Control plane unreachable: %v", err),
			}
		}
		return health.Check{
			Name:    "control-plane",
			Status:  health.StatusHealthy,
			Message: "Control plane is responding",
		}
	})

	checker.Register("config", func(ctx context.Context) health.Check {
		if s.Config == nil {
			return health.Check{
				Name:    "config",
				Status:  health.StatusUnhealthy,
				Message: "No configuration loaded",
			}
		}
		return health.Check{
			Name:    "config",
			Status:  health.StatusHealthy,
			Message: "Configuration loaded successfully",
		}
	})

	report := checker.Check(context.Background())

	var statusCode int
	switch report.Status {
	case health.StatusHealthy:
		statusCode = http.StatusOK
	case health.StatusDegraded:
		statusCode = http.StatusOK
	case health.StatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	default:
		statusCode = http.StatusServiceUnavailable
	}

	WriteJSON(w, statusCode, report)
}

// handleReadiness returns readiness status for Kubernetes-style probes.
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ready := true
	message := "Ready"

	if s.Config == nil {
		ready = false
		message = "Configuration not loaded"
	} else if s.client == nil {
		ready = false
		message = "Control plane not connected"
	}

	if ready {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ready",
			"message": message,
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "not ready",
			"message": message,
		})
	}
}

// handleTraffic returns traffic accounting statistics
func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Collector not initialized")
		return
	}
	stats := s.collector.GetInterfaceStats()
	WriteJSON(w, http.StatusOK, stats)
}

// handleVPNStatus returns WireGuard peer statistics
func (s *Server) handleVPNStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.collector.GetVPNStats()
	if stats == nil {
		// Return empty object if nil (e.g. collector not started)
		stats = make(map[string]map[string]*metrics.PeerStats)
	}
	WriteJSON(w, http.StatusOK, stats)
}
