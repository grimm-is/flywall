// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"grimm.is/flywall/internal/ebpf/interfaces"
)

// EBPFStatsHandlers handles eBPF statistics API endpoints
type EBPFStatsHandlers struct {
	manager interfaces.Manager
}

// NewEBPFStatsHandlers creates a new eBPF statistics handler
func NewEBPFStatsHandlers(manager interfaces.Manager) *EBPFStatsHandlers {
	return &EBPFStatsHandlers{
		manager: manager,
	}
}

// RegisterRoutes registers the statistics routes
func (h *EBPFStatsHandlers) RegisterRoutes(router *mux.Router) {
	// General statistics
	router.HandleFunc("/stats", h.handleGetStatistics).Methods("GET")

	// Feature status
	router.HandleFunc("/features", h.handleGetFeatureStatus).Methods("GET")
	router.HandleFunc("/features/{feature}", h.handleGetFeature).Methods("GET")

	// Map information
	router.HandleFunc("/maps", h.handleGetMaps).Methods("GET")
	router.HandleFunc("/maps/{map}", h.handleGetMap).Methods("GET")

	// Hook information
	router.HandleFunc("/hooks", h.handleGetHooks).Methods("GET")

	// Performance metrics
	router.HandleFunc("/performance", h.handleGetPerformance).Methods("GET")
	router.HandleFunc("/performance/history", h.handleGetPerformanceHistory).Methods("GET")
}

// handleGetStatistics returns general eBPF statistics
func (h *EBPFStatsHandlers) handleGetStatistics(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStatistics()

	// Add timestamp and uptime
	response := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"statistics": stats,
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetFeatureStatus returns the status of all eBPF features
func (h *EBPFStatsHandlers) handleGetFeatureStatus(w http.ResponseWriter, r *http.Request) {
	features := h.manager.GetFeatureStatus()

	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"features":  features,
		"count":     len(features),
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetFeature returns the status of a specific feature
func (h *EBPFStatsHandlers) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	featureName := vars["feature"]

	if featureName == "" {
		respondWithErrorStats(w, http.StatusBadRequest, "Feature name required")
		return
	}

	features := h.manager.GetFeatureStatus()
	feature, exists := features[featureName]

	if !exists {
		respondWithErrorStats(w, http.StatusNotFound, "Feature not found")
		return
	}

	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"feature":   featureName,
		"status":    feature,
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetMaps returns information about all eBPF maps
func (h *EBPFStatsHandlers) handleGetMaps(w http.ResponseWriter, r *http.Request) {
	maps := h.manager.GetMapInfo()

	// Calculate total entries across all maps
	totalEntries := 0
	for range maps {
		// TODO: Get actual entry count from map
		totalEntries++
	}

	response := map[string]interface{}{
		"timestamp":     time.Now().UTC(),
		"maps":          maps,
		"count":         len(maps),
		"total_entries": totalEntries,
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetMap returns information about a specific eBPF map
func (h *EBPFStatsHandlers) handleGetMap(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mapName := vars["map"]

	if mapName == "" {
		respondWithErrorStats(w, http.StatusBadRequest, "Map name required")
		return
	}

	maps := h.manager.GetMapInfo()
	mapInfo, exists := maps[mapName]

	if !exists {
		respondWithErrorStats(w, http.StatusNotFound, "Map not found")
		return
	}

	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"map":       mapName,
		"info":      mapInfo,
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetHooks returns information about all eBPF hooks
func (h *EBPFStatsHandlers) handleGetHooks(w http.ResponseWriter, r *http.Request) {
	hooks := h.manager.GetHookInfo()

	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"hooks":     hooks,
		"count":     len(hooks),
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetPerformance returns performance metrics
func (h *EBPFStatsHandlers) handleGetPerformance(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStatistics()

	// Calculate performance metrics
	pps := float64(stats.PacketsProcessed) / time.Since(time.Now().Add(-time.Second)).Seconds()

	performance := map[string]interface{}{
		"packets_per_second":  pps,
		"drop_rate":           float64(stats.PacketsDropped) / float64(stats.PacketsProcessed) * 100,
		"pass_rate":           float64(stats.PacketsPassed) / float64(stats.PacketsProcessed) * 100,
		"average_packet_size": float64(stats.BytesProcessed) / float64(stats.PacketsProcessed),
	}

	response := map[string]interface{}{
		"timestamp":   time.Now().UTC(),
		"performance": performance,
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// handleGetPerformanceHistory returns historical performance data
func (h *EBPFStatsHandlers) handleGetPerformanceHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	duration := r.URL.Query().Get("duration")
	if duration == "" {
		duration = "1h"
	}

	// TODO: Implement historical data collection
	// For now, return empty history
	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"duration":  duration,
		"history":   []interface{}{},
		"message":   "Historical data collection not yet implemented",
	}

	respondWithJSONStats(w, http.StatusOK, response)
}

// Helper functions
func respondWithJSONStats(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondWithErrorStats(w http.ResponseWriter, status int, message string) {
	respondWithJSONStats(w, status, map[string]string{"error": message})
}
