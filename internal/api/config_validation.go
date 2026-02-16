// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/engine"
)

// handleConfigValidate handles comprehensive configuration validation
func (s *Server) handleConfigValidate(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid config format", http.StatusBadRequest)
		return
	}

	// Use the integrated engine for validation
	integratedEngine := engine.NewIntegratedEngine()
	result, err := integratedEngine.ValidateAndSimulate(&cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":              result.IsValid(),
		"errors":             formatErrors(result.Errors),
		"warnings":           result.Warnings,
		"simulation_results": result.SimulationResults,
		"compliance_report":  result.ComplianceReport,
		"dependency_graph":   result.DependencyGraph,
	})
}

// handleConfigSimulate handles network connectivity simulation
func (s *Server) handleConfigSimulate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config    config.Config `json:"config"`
		Scenarios []string      `json:"scenarios"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	integratedEngine := engine.NewIntegratedEngine()
	var results []config.ConnectivityTestResult

	for _, scenario := range req.Scenarios {
		// Parse scenario string like "LAN->WAN:tcp:80"
		parts := strings.Split(scenario, ":")
		if len(parts) >= 3 {
			fromTo := strings.Split(parts[0], "->")
			if len(fromTo) == 2 {
				port, _ := strconv.Atoi(parts[2])
				result, err := integratedEngine.SimulateConnectivity(&req.Config, fromTo[0], fromTo[1], parts[1], port)
				if err == nil {
					results = append(results, *result)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleConfigPipeline handles staged configuration validation pipeline
func (s *Server) handleConfigPipeline(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	// Get timeout from query parameter
	timeout := 30 * time.Second
	if timeoutStr := r.URL.Query().Get("timeout"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		}
	}

	integratedEngine := engine.NewIntegratedEngine()
	pipeline := engine.NewConfigPipeline(integratedEngine)

	result, err := pipeline.ExecuteWithTimeout(&cfg, timeout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleConfigDryRun handles configuration dry run
func (s *Server) handleConfigDryRun(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	integratedEngine := engine.NewIntegratedEngine()
	result, err := integratedEngine.DryRun(&cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleConfigCompliance handles compliance checking
func (s *Server) handleConfigCompliance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config     config.Config `json:"config"`
		PolicyName string        `json:"policy_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.PolicyName == "" {
		req.PolicyName = "basic-security" // Default policy
	}

	integratedEngine := engine.NewIntegratedEngine()
	report, err := integratedEngine.CheckCompliance(&req.Config, req.PolicyName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleConfigDependencies handles dependency analysis
func (s *Server) handleConfigDependencies(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	integratedEngine := engine.NewIntegratedEngine()
	graph, err := integratedEngine.AnalyzeDependencies(&cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(graph)
}

// handleConfigApply handles configuration application with validation
func (s *Server) handleConfigApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config config.Config `json:"config"`
		DryRun bool          `json:"dry_run"`
		Force  bool          `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	integratedEngine := engine.NewIntegratedEngine()

	if req.DryRun {
		result, err := integratedEngine.DryRun(&req.Config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"dry_run": true,
			"result":  result,
		})
		return
	}

	// Validate before applying
	validationResult, err := integratedEngine.ValidateAndSimulate(&req.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !validationResult.IsValid() && !req.Force {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Configuration validation failed",
			"errors":  formatErrors(validationResult.Errors),
			"message": "Set force=true to apply despite validation errors",
		})
		return
	}

	// Apply configuration
	if err := integratedEngine.ApplyWithValidation(&req.Config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration applied successfully",
	})
}

// handleConfigStages returns available pipeline stages
func (s *Server) handleConfigStages(w http.ResponseWriter, r *http.Request) {
	integratedEngine := engine.NewIntegratedEngine()
	pipeline := engine.NewConfigPipeline(integratedEngine)
	stages := pipeline.GetStages()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stages)
}

// handleConfigPolicies returns available compliance policies
func (s *Server) handleConfigPolicies(w http.ResponseWriter, r *http.Request) {
	policies := config.GetStandardCompliancePolicies()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policies)
}

// formatErrors converts error slice to string slice
func formatErrors(errors []error) []string {
	result := make([]string, len(errors))
	for i, err := range errors {
		result[i] = err.Error()
	}
	return result
}

// RegisterConfigValidationRoutes registers all configuration validation routes
func (s *Server) RegisterConfigValidationRoutes() {
	// TODO: Fix router registration when Server structure is known
	// s.router.HandleFunc("/api/v1/config/validate", s.handleConfigValidate).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/simulate", s.handleConfigSimulate).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/pipeline", s.handleConfigPipeline).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/dry-run", s.handleConfigDryRun).Methods("POST")
	//
	// // Analysis endpoints
	// s.router.HandleFunc("/api/v1/config/compliance", s.handleConfigCompliance).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/dependencies", s.handleConfigDependencies).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/traffic-impact", s.handleConfigTrafficImpact).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/traffic-impact/stream", s.handleConfigTrafficImpactStream).Methods("POST")
	// s.router.HandleFunc("/api/v1/config/traffic-preview", s.handleConfigTrafficPreview).Methods("POST")
	//
	// // Application endpoint
	// s.router.HandleFunc("/api/v1/config/apply", s.handleConfigApply).Methods("POST")
	//
	// // Information endpoints
	// s.router.HandleFunc("/api/v1/config/stages", s.handleConfigStages).Methods("GET")
	// s.router.HandleFunc("/api/v1/config/policies", s.handleConfigPolicies).Methods("GET")
}

// ConfigMiddleware adds configuration context to requests
func (s *Server) ConfigMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add configuration context headers
		w.Header().Set("X-Flywall-Config-Version", "1.0")
		w.Header().Set("X-Flywall-API-Version", "v1")

		// Add request ID for tracing
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}

// handleConfigTrafficImpact handles traffic impact analysis
func (s *Server) handleConfigTrafficImpact(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentConfig  config.Config `json:"current_config"`
		ProposedConfig config.Config `json:"proposed_config"`
		WindowMinutes  int           `json:"window_minutes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.WindowMinutes == 0 {
		req.WindowMinutes = 60 // Default to 1 hour
	}

	integratedEngine := engine.NewIntegratedEngine()
	window := time.Duration(req.WindowMinutes) * time.Minute

	analysis, err := integratedEngine.AnalyzeTrafficImpact(&req.CurrentConfig, &req.ProposedConfig, window)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate report if requested
	if r.URL.Query().Get("report") == "true" {
		report := integratedEngine.GenerateTrafficReport(analysis)
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte(report))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}

// handleConfigTrafficImpactStream handles real-time traffic impact monitoring
func (s *Server) handleConfigTrafficImpactStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentConfig  config.Config `json:"current_config"`
		ProposedConfig config.Config `json:"proposed_config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Enable SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	integratedEngine := engine.NewIntegratedEngine()
	monitor, err := integratedEngine.StartRealTimeMonitoring(&req.CurrentConfig, &req.ProposedConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer monitor.Stop()

	// Send initial analysis
	analysis, err := integratedEngine.AnalyzeTrafficImpact(&req.CurrentConfig, &req.ProposedConfig, 5*time.Minute)
	if err == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "initial_analysis",
			"data": analysis,
		})
		flusher.Flush()
	}

	// Stream real-time impacts
	impactChan := monitor.GetImpactChannel()
	for {
		select {
		case impact := <-impactChan:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": "flow_impact",
				"data": impact,
			})
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
			// Send heartbeat
			w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}

// handleConfigTrafficPreview handles configuration preview with traffic impact
func (s *Server) handleConfigTrafficPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseConfig    config.Config `json:"base_config"`
		Changes       ConfigChanges `json:"changes"`
		WindowMinutes int           `json:"window_minutes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Apply changes to create proposed config
	proposedConfig, err := applyConfigChanges(&req.BaseConfig, &req.Changes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.WindowMinutes == 0 {
		req.WindowMinutes = 60
	}

	integratedEngine := engine.NewIntegratedEngine()
	window := time.Duration(req.WindowMinutes) * time.Minute

	// Analyze traffic impact
	analysis, err := integratedEngine.AnalyzeTrafficImpact(&req.BaseConfig, proposedConfig, window)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate proposed config
	validation, err := integratedEngine.ValidateAndSimulate(proposedConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate preview script
	script, err := integratedEngine.GenerateFirewallScript(proposedConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := map[string]interface{}{
		"traffic_impact":  analysis,
		"validation":      validation,
		"script_preview":  script,
		"changes_applied": req.Changes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ConfigChanges represents configuration changes to apply
type ConfigChanges struct {
	AddedInterfaces   []config.Interface   `json:"added_interfaces"`
	RemovedInterfaces []string             `json:"removed_interfaces"`
	AddedZones        []config.Zone        `json:"added_zones"`
	RemovedZones      []string             `json:"removed_zones"`
	AddedPolicies     []config.Policy      `json:"added_policies"`
	RemovedPolicies   []string             `json:"removed_policies"`
	ModifiedPolicies  []PolicyModification `json:"modified_policies"`
}

// PolicyModification represents a policy modification
type PolicyModification struct {
	Name         string              `json:"name"`
	AddedRules   []config.PolicyRule `json:"added_rules"`
	RemovedRules []config.PolicyRule `json:"removed_rules"`
}

// applyConfigChanges applies changes to a base configuration
func applyConfigChanges(base *config.Config, changes *ConfigChanges) (*config.Config, error) {
	// Deep copy base config
	proposed := &config.Config{}
	// TODO: Implement deep copy

	// Apply interface changes
	for _, iface := range changes.AddedInterfaces {
		proposed.Interfaces = append(proposed.Interfaces, iface)
	}

	// Apply zone changes
	for _, zone := range changes.AddedZones {
		proposed.Zones = append(proposed.Zones, zone)
	}

	// Apply policy changes
	for _, policy := range changes.AddedPolicies {
		proposed.Policies = append(proposed.Policies, policy)
	}

	// TODO: Handle removals and modifications

	return proposed, nil
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
