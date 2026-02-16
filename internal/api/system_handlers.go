// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"grimm.is/flywall/internal/install"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/firewall"
)

// handleReboot triggers a system reboot
// POST /api/system/reboot
func (s *Server) handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Force bool `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if s.client == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Control plane not connected")
		return
	}

	msg, err := s.client.SystemReboot(req.Force)
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to reboot system: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": msg,
	})
}

// handleRestore imports a configuration from JSON
// POST /api/system/restore
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	// Parse the uploaded config
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid configuration: "+err.Error())
		return
	}

	// Apply the config
	if s.client != nil {
		if err := s.client.ApplyConfig(&cfg); err != nil {
			WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to apply config: "+err.Error())
			return
		}
	} else {
		// Update in-memory config
		s.configMu.Lock()
		*s.Config = cfg
		s.configMu.Unlock()
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleSystemUpgrade triggers a system upgrade via uploaded binary
// POST /api/system/upgrade (multipart/form-data)
// Fields:
//   - binary: the new binary file
//   - checksum: SHA256 hex string for verification
//   - arch: expected architecture (e.g., "linux/arm64")
func (s *Server) handleSystemUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check content type - support both multipart upload and trigger-only JSON
	contentType := r.Header.Get("Content-Type")

	// If no binary is being uploaded (JSON or empty body), just trigger local upgrade
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		// Legacy behavior: trigger upgrade with staged binary
		if err := s.client.Upgrade(""); err != nil {
			WriteErrorCtx(w, r, http.StatusInternalServerError, "Upgrade failed: "+err.Error())
			return
		}
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Upgrade initiated (using staged binary)",
		})
		return
	}

	// Parse multipart form (limit: 100MB)
	const maxUploadSize = 100 << 20 // 100 MB
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Failed to parse multipart form: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Get the binary file
	file, header, err := r.FormFile("binary")
	if err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Missing 'binary' field: "+err.Error())
		return
	}
	defer file.Close()

	// Get expected checksum
	expectedChecksum := r.FormValue("checksum")
	if expectedChecksum == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Missing 'checksum' field")
		return
	}

	// Get expected architecture
	expectedArch := r.FormValue("arch")
	if expectedArch == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Missing 'arch' field")
		return
	}

	// Read binary into memory (for RPC transport)
	binaryData, err := io.ReadAll(file)
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to read binary: "+err.Error())
		return
	}

	// Verify checksum locally before sending to control plane
	hasher := sha256.New()
	hasher.Write(binaryData)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		WriteErrorCtx(w, r, http.StatusBadRequest,
			fmt.Sprintf("Checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum))
		return
	}

	log.Printf("[API] Staging upgrade binary via RPC: %s (%d bytes, arch: %s)",
		header.Filename, len(binaryData), expectedArch)

	// Stage binary via control plane RPC (which runs as root)
	stageReply, err := s.client.StageBinary(binaryData, actualChecksum, expectedArch)
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to stage binary: "+err.Error())
		return
	}

	log.Printf("[API] Binary staged at %s, triggering upgrade...", stageReply.Path)

	// Trigger upgrade via control plane
	if err := s.client.Upgrade(actualChecksum); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Upgrade failed: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Upgrade initiated",
		"checksum": actualChecksum,
		"bytes":    len(binaryData),
	})
}

// handleWakeOnLAN sends a Wake-on-LAN magic packet
// POST /api/system/wol
func (s *Server) handleWakeOnLAN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		MAC       string `json:"mac"`
		Interface string `json:"interface,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MAC == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "MAC address is required")
		return
	}

	if err := s.client.WakeOnLAN(req.MAC, req.Interface); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to send WOL packet: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleSystemStats returns system statistics
// GET /api/system/stats
func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats, err := s.client.GetSystemStats()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to get system stats: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"stats": stats,
	})
}

// handleSystemRoutes returns the kernel routing table
// GET /api/system/routes
func (s *Server) handleSystemRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	routes, err := s.client.GetRoutes()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to get routes: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"routes": routes,
	})
}

// handleSafeModeStatus returns safe mode status
// GET /api/system/safe-mode
func (s *Server) handleSafeModeStatus(w http.ResponseWriter, r *http.Request) {
	inSafeMode, err := s.client.IsInSafeMode()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to check safe mode status: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"in_safe_mode": inSafeMode,
	})
}

// handleEnterSafeMode activates safe mode (emergency lockdown)
// POST /api/system/safe-mode
func (s *Server) handleEnterSafeMode(w http.ResponseWriter, r *http.Request) {
	err := s.client.EnterSafeMode()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to enter safe mode: "+err.Error())
		return
	}

	log.Printf("[API] Safe mode activated by user")
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Safe mode activated - forwarding disabled",
		"in_safe_mode": true,
	})
}

// handleExitSafeMode deactivates safe mode
// DELETE /api/system/safe-mode
func (s *Server) handleExitSafeMode(w http.ResponseWriter, r *http.Request) {
	err := s.client.ExitSafeMode()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to exit safe mode: "+err.Error())
		return
	}

	log.Printf("[API] Safe mode deactivated by user")
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Safe mode deactivated - normal operation resumed",
		"in_safe_mode": false,
	})
}

func (s *Server) handleLeases(w http.ResponseWriter, r *http.Request) {
	if s.client != nil {
		leases, err := s.client.GetDHCPLeases()
		if err != nil {
			leases = []ctlplane.DHCPLease{}
		}
		if leases == nil {
			leases = []ctlplane.DHCPLease{}
		}

		// Sentinel Enrichment
		// We process in-place since we can modify the slice elements
		if s.sentinel != nil {
			for i := range leases {
				// Analyze device
				result := s.sentinel.Analyze(leases[i].MAC, leases[i].Hostname)

				// Enrich Vendor if missing
				if leases[i].Vendor == "" && result.Vendor != "" {
					leases[i].Vendor = result.Vendor
				}

				// Enrich Type if missing
				// We prefer specific detail (e.g. "iPhone") over category ("Mobile") for better icons
				if leases[i].Type == "" {
					if result.Detail != "" {
						leases[i].Type = strings.ToLower(result.Detail)
					} else {
						leases[i].Type = result.Category
					}
				}
			}
		}

		WriteJSON(w, http.StatusOK, leases)
	} else {
		WriteJSON(w, http.StatusOK, []interface{}{})
	}
}

// handlePublicCert serves the root CA certificate publicly
func (s *Server) handlePublicCert(w http.ResponseWriter, r *http.Request) {
	certPath := filepath.Join(install.GetConfigDir(), "certs", "server.crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		data, err = os.ReadFile("local/certs/server.crt")
		if err != nil {
			WriteErrorCtx(w, r, http.StatusNotFound, "Certificate not found")
			return
		}
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=flywall-ca.crt")
	w.Write(data)
}

// handleServices returns the available service definitions
func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	type ServiceInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Ports       []struct {
			Port     int    `json:"port"`
			EndPort  int    `json:"end_port,omitempty"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
	}

	services := make([]ServiceInfo, 0, len(firewall.BuiltinServices))
	for name, svc := range firewall.BuiltinServices {
		info := ServiceInfo{
			Name:        name,
			Description: svc.Description,
		}
		if svc.Protocol&firewall.ProtoTCP != 0 {
			if len(svc.Ports) > 0 {
				for _, p := range svc.Ports {
					info.Ports = append(info.Ports, struct {
						Port     int    `json:"port"`
						EndPort  int    `json:"end_port,omitempty"`
						Protocol string `json:"protocol"`
					}{Port: p, Protocol: "tcp"})
				}
			} else if svc.Port > 0 {
				info.Ports = append(info.Ports, struct {
					Port     int    `json:"port"`
					EndPort  int    `json:"end_port,omitempty"`
					Protocol string `json:"protocol"`
				}{Port: svc.Port, EndPort: svc.EndPort, Protocol: "tcp"})
			}
		}
		if svc.Protocol&firewall.ProtoUDP != 0 {
			if len(svc.Ports) > 0 {
				for _, p := range svc.Ports {
					info.Ports = append(info.Ports, struct {
						Port     int    `json:"port"`
						EndPort  int    `json:"end_port,omitempty"`
						Protocol string `json:"protocol"`
					}{Port: p, Protocol: "udp"})
				}
			} else if svc.Port > 0 {
				info.Ports = append(info.Ports, struct {
					Port     int    `json:"port"`
					EndPort  int    `json:"end_port,omitempty"`
					Protocol string `json:"protocol"`
				}{Port: svc.Port, EndPort: svc.EndPort, Protocol: "udp"})
			}
		}
		if svc.Protocol&firewall.ProtoICMP != 0 {
			info.Ports = append(info.Ports, struct {
				Port     int    `json:"port"`
				EndPort  int    `json:"end_port,omitempty"`
				Protocol string `json:"protocol"`
			}{Protocol: "icmp"})
		}
		services = append(services, info)
	}

	WriteJSON(w, http.StatusOK, services)
}

// handleRestartService restarts a specific service
// POST /api/system/services/restart
func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Service == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Service name is required")
		return
	}

	if s.client == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Control plane not connected")
		return
	}

	if err := s.client.RestartService(req.Service); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to restart service: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}
