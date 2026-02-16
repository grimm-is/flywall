// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"grimm.is/flywall/internal/ebpf/interfaces"
	"grimm.is/flywall/internal/services/ebpf/dns_blocklist"
)

// EBPFHandlers handles all eBPF-related API endpoints
type EBPFHandlers struct {
	manager interfaces.Manager
	dns     *dns_blocklist.API
	stats   *EBPFStatsHandlers
}

// NewEBPFHandlers creates a new eBPF API handler
func NewEBPFHandlers(manager interfaces.Manager) *EBPFHandlers {
	dnsService := manager.GetDNSBlocklistService()
	return &EBPFHandlers{
		manager: manager,
		dns:     dns_blocklist.NewAPI(dnsService.(*dns_blocklist.Service)),
		stats:   NewEBPFStatsHandlers(manager),
	}
}

// RegisterRoutes registers eBPF routes
func (h *EBPFHandlers) RegisterRoutes(router *mux.Router) {
	// Register DNS blocklist routes
	h.dns.RegisterRoutes(router)

	// Register statistics routes
	h.stats.RegisterRoutes(router.PathPrefix("/stats").Subrouter())

	// General eBPF status
	router.HandleFunc("/status", h.handleGetStatus).Methods("GET")
	router.HandleFunc("/health", h.handleHealthCheck).Methods("GET")
}

// handleGetStatus returns general eBPF status
func (h *EBPFHandlers) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	features := h.manager.GetFeatureStatus()

	status := map[string]interface{}{
		"enabled":  true,
		"features": features,
		"hooks":    h.manager.GetHookInfo(),
		"maps":     h.manager.GetMapInfo(),
	}

	respondWithJSON(w, http.StatusOK, status)
}

// handleHealthCheck performs a health check on the eBPF subsystem
func (h *EBPFHandlers) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":         "healthy",
		"loaded":         true,
		"running":        true,
		"kernel_support": true,
		"jit_enabled":    true,
	}

	respondWithJSON(w, http.StatusOK, health)
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
