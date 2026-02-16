// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"grimm.is/flywall/internal/alerting"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ebpf"
	"grimm.is/flywall/internal/logging"
)

// ControlPlane manages eBPF integration with the control plane
type ControlPlane struct {
	ebpf       *ebpf.Integration
	logger     logging.Logger
	config     *config.Config
	alerts     *alerting.Engine
	httpServer *http.Server
	router     *mux.Router
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewControlPlane creates a new control plane integration
func NewControlPlane(cfg *config.Config, logger logging.Logger, alerts *alerting.Engine) (*ControlPlane, error) {
	// Create eBPF integration
	ebpfIntegration, err := ebpf.NewIntegration(cfg, logger, alerts)
	if err != nil {
		return nil, fmt.Errorf("failed to create eBPF integration: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	cp := &ControlPlane{
		ebpf:   ebpfIntegration,
		logger: logger,
		config: cfg,
		alerts: alerts,
		router: mux.NewRouter(),
		ctx:    ctx,
		cancel: cancel,
	}

	// Setup HTTP routes
	cp.setupRoutes()

	return cp, nil
}

// Start starts the control plane integration
func (cp *ControlPlane) Start() error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	cp.logger.Info("Starting eBPF control plane integration")

	// Start eBPF integration
	if err := cp.ebpf.Start(); err != nil {
		return fmt.Errorf("failed to start eBPF integration: %w", err)
	}

	// Start HTTP server for control plane API
	if cp.config.API != nil && cp.config.API.Enabled {
		if err := cp.startHTTPServer(); err != nil {
			cp.ebpf.Stop()
			return fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}

	cp.logger.Info("eBPF control plane integration started")
	return nil
}

// Stop stops the control plane integration
func (cp *ControlPlane) Stop() error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	cp.logger.Info("Stopping eBPF control plane integration")

	// Cancel context
	cp.cancel()

	// Stop HTTP server
	if cp.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cp.httpServer.Shutdown(ctx)
	}

	// Stop eBPF integration
	if err := cp.ebpf.Stop(); err != nil {
		cp.logger.Error("Failed to stop eBPF integration", "error", err)
		return err
	}

	cp.logger.Info("eBPF control plane integration stopped")
	return nil
}

// Close closes the control plane integration
func (cp *ControlPlane) Close() error {
	if cp.ebpf != nil {
		return cp.ebpf.Close()
	}
	return nil
}

// setupRoutes sets up HTTP routes for the control plane API
func (cp *ControlPlane) setupRoutes() {
	api := cp.router.PathPrefix("/api/v1/ebpf").Subrouter()

	// Statistics endpoints
	api.HandleFunc("/stats", cp.handleStats).Methods("GET")
	api.HandleFunc("/stats/programs", cp.handleProgramStats).Methods("GET")
	api.HandleFunc("/stats/maps", cp.handleMapStats).Methods("GET")

	// Control endpoints
	api.HandleFunc("/reload", cp.handleReload).Methods("POST")
	api.HandleFunc("/features/{feature}/enable", cp.handleEnableFeature).Methods("POST")
	api.HandleFunc("/features/{feature}/disable", cp.handleDisableFeature).Methods("POST")

	// Health check
	api.HandleFunc("/health", cp.handleHealth).Methods("GET")

	// Configuration
	api.HandleFunc("/config", cp.handleGetConfig).Methods("GET")
	api.HandleFunc("/config", cp.handleUpdateConfig).Methods("PUT")
}

// startHTTPServer starts the HTTP server
func (cp *ControlPlane) startHTTPServer() error {
	// TODO: Get port from config - for now use default
	addr := ":8080"
	if cp.config.API != nil {
		// Check if Listen is available
		if cp.config.API.Listen != "" {
			addr = cp.config.API.Listen
		}
	}

	cp.httpServer = &http.Server{
		Addr:    addr,
		Handler: cp.router,
	}

	go func() {
		cp.logger.Info("Starting eBPF control plane API server", "addr", addr)
		if err := cp.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			cp.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// handleStats handles the /stats endpoint
func (cp *ControlPlane) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := cp.ebpf.GetStatistics()
	if stats == nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleProgramStats handles the /stats/programs endpoint
func (cp *ControlPlane) handleProgramStats(w http.ResponseWriter, r *http.Request) {
	stats := cp.ebpf.GetStatistics()
	if stats == nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"programs": stats.Programs,
		"count":    len(stats.Programs),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMapStats handles the /stats/maps endpoint
func (cp *ControlPlane) handleMapStats(w http.ResponseWriter, r *http.Request) {
	stats := cp.ebpf.GetStatistics()
	if stats == nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"maps":  stats.Maps,
		"count": len(stats.Maps),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReload handles the /reload endpoint
func (cp *ControlPlane) handleReload(w http.ResponseWriter, r *http.Request) {
	cp.logger.Info("Reloading eBPF programs")

	// Stop current integration
	if err := cp.ebpf.Stop(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop eBPF: %v", err), http.StatusInternalServerError)
		return
	}

	// Start again
	if err := cp.ebpf.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start eBPF: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleEnableFeature handles enabling a feature
func (cp *ControlPlane) handleEnableFeature(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	feature := vars["feature"]

	cp.logger.Info("Enabling eBPF feature", "feature", feature)

	if err := cp.ebpf.EnableFeature(feature); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enable feature %s: %v", feature, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"feature": feature,
		"enabled": "true",
	})
}

// handleDisableFeature handles disabling a feature
func (cp *ControlPlane) handleDisableFeature(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	feature := vars["feature"]

	cp.logger.Info("Disabling eBPF feature", "feature", feature)

	if err := cp.ebpf.DisableFeature(feature); err != nil {
		http.Error(w, fmt.Sprintf("Failed to disable feature %s: %v", feature, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"feature": feature,
		"enabled": "false",
	})
}

// handleHealth handles the /health endpoint
func (cp *ControlPlane) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := cp.ebpf.GetStatistics()

	healthy := stats != nil && len(stats.Programs) > 0

	status := map[string]interface{}{
		"healthy":   healthy,
		"timestamp": time.Now().Unix(),
		"programs":  0,
		"maps":      0,
	}

	if stats != nil {
		status["programs"] = len(stats.Programs)
		status["maps"] = len(stats.Maps)
	}

	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleGetConfig handles getting the current configuration
func (cp *ControlPlane) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return eBPF configuration
	ebpfConfig := cp.config.EBPF

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ebpf": ebpfConfig,
	})
}

// handleUpdateConfig handles updating the configuration
func (cp *ControlPlane) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	cp.logger.Info("Updating eBPF configuration", "config", newConfig)

	// In a real implementation, we would validate and apply to cp.config
	// For now, we assume the config matches the expected structure and use UpdateConfig
	if err := cp.ebpf.UpdateConfig(cp.config); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GetEBPFIntegration returns the eBPF integration for direct access
func (cp *ControlPlane) GetEBPFIntegration() *ebpf.Integration {
	return cp.ebpf
}

// UpdateFirewallRules updates firewall rules in eBPF programs
func (cp *ControlPlane) UpdateFirewallRules(rules interface{}) error {
	cp.logger.Info("Updating firewall rules", "rules", rules)

	// Attempt to parse as list of IPs (strings) - most common case for now
	if ips, ok := rules.([]string); ok {
		return cp.UpdateBlocklist(ips)
	}

	// Attempt to parse as []interface{} and convert to []string
	if ruleList, ok := rules.([]interface{}); ok {
		var ips []string
		for _, r := range ruleList {
			if ip, ok := r.(string); ok {
				ips = append(ips, ip)
			}
		}
		if len(ips) > 0 {
			return cp.UpdateBlocklist(ips)
		}
		return nil // No valid string rules found, essentially empty update
	}

	return fmt.Errorf("unsupported rule format: expected []string or []interface{}")
}

// UpdateBlocklist updates the IP blocklist
func (cp *ControlPlane) UpdateBlocklist(ips []string) error {
	cp.logger.Info("Updating blocklist", "count", len(ips))

	return cp.ebpf.UpdateBlocklist(ips)
}
