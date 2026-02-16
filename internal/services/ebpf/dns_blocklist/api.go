// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dns_blocklist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// API handles HTTP API endpoints for DNS blocklist management
type API struct {
	service *Service
}

// NewAPI creates a new DNS blocklist API handler
func NewAPI(service *Service) *API {
	return &API{
		service: service,
	}
}

// RegisterRoutes registers API routes
func (api *API) RegisterRoutes(router *mux.Router) {
	// DNS blocklist management endpoints
	router.HandleFunc("/api/v1/ebpf/dns/blocklist", api.handleGetBlocklist).Methods("GET")
	router.HandleFunc("/api/v1/ebpf/dns/blocklist/config", api.handleUpdateConfig).Methods("POST")
	router.HandleFunc("/api/v1/ebpf/dns/blocklist", api.handleClearBlocklist).Methods("DELETE")

	// Domain management endpoints
	router.HandleFunc("/api/v1/ebpf/dns/domains", api.handleAddDomains).Methods("POST")
	router.HandleFunc("/api/v1/ebpf/dns/domains/{domain}", api.handleRemoveDomain).Methods("DELETE")
	router.HandleFunc("/api/v1/ebpf/dns/domains/{domain}", api.handleCheckDomain).Methods("GET")

	// Bulk operations
	router.HandleFunc("/api/v1/ebpf/dns/blocklist/import", api.handleImport).Methods("POST")
	router.HandleFunc("/api/v1/ebpf/dns/blocklist/export", api.handleExport).Methods("GET")

	// Statistics and status
	router.HandleFunc("/api/v1/ebpf/dns/stats", api.handleGetStats).Methods("GET")
}

// handleGetBlocklist returns the current blocklist
func (api *API) handleGetBlocklist(w http.ResponseWriter, r *http.Request) {
	domains := api.service.Export()

	response := map[string]interface{}{
		"domains": domains,
		"count":   len(domains),
	}

	api.writeJSON(w, http.StatusOK, response)
}

// handleUpdateConfig updates the blocklist configuration
func (api *API) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		api.writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Update service configuration
	if err := api.service.UpdateConfig(&config); err != nil {
		api.writeError(w, http.StatusInternalServerError, "Failed to update configuration", err)
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "updated",
		"config": config,
	})
}

// handleClearBlocklist clears all domains
func (api *API) handleClearBlocklist(w http.ResponseWriter, r *http.Request) {
	if err := api.service.Clear(); err != nil {
		api.writeError(w, http.StatusInternalServerError, "Failed to clear blocklist", err)
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]string{
		"status": "cleared",
	})
}

// handleAddDomains adds domains to the blocklist
func (api *API) handleAddDomains(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Domains []string `json:"domains"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.writeError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	if len(request.Domains) == 0 {
		api.writeError(w, http.StatusBadRequest, "No domains provided", nil)
		return
	}

	added := 0
	errors := make(map[string]string)

	for _, domain := range request.Domains {
		if err := api.service.AddDomain(domain); err != nil {
			errors[domain] = err.Error()
		} else {
			added++
		}
	}

	response := map[string]interface{}{
		"total":  len(request.Domains),
		"added":  added,
		"failed": len(errors),
		"errors": errors,
	}

	api.writeJSON(w, http.StatusOK, response)
}

// handleRemoveDomain removes a domain from the blocklist
func (api *API) handleRemoveDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	if domain == "" {
		api.writeError(w, http.StatusBadRequest, "Domain required", nil)
		return
	}

	if err := api.service.RemoveDomain(domain); err != nil {
		api.writeError(w, http.StatusInternalServerError, "Failed to remove domain", err)
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]string{
		"domain": domain,
		"status": "removed",
	})
}

// handleCheckDomain checks if a domain is blocked
func (api *API) handleCheckDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	if domain == "" {
		api.writeError(w, http.StatusBadRequest, "Domain required", nil)
		return
	}

	blocked := api.service.IsBlocked(domain)

	response := map[string]interface{}{
		"domain":  domain,
		"blocked": blocked,
	}

	api.writeJSON(w, http.StatusOK, response)
}

// handleImport imports domains from a file or text
func (api *API) handleImport(w http.ResponseWriter, r *http.Request) {
	// Handle both JSON and plain text formats
	contentType := r.Header.Get("Content-Type")

	var domains []string

	if strings.HasPrefix(contentType, "application/json") {
		var request struct {
			Domains []string `json:"domains"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			api.writeError(w, http.StatusBadRequest, "Invalid JSON", err)
			return
		}
		domains = request.Domains
	} else {
		// Treat as plain text - one domain per line
		body, err := io.ReadAll(r.Body)
		if err != nil {
			api.writeError(w, http.StatusBadRequest, "Failed to read body", err)
			return
		}

		lines := strings.Split(string(body), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				domains = append(domains, line)
			}
		}
	}

	if len(domains) == 0 {
		api.writeError(w, http.StatusBadRequest, "No domains provided", nil)
		return
	}

	if err := api.service.Import(domains); err != nil {
		api.writeError(w, http.StatusInternalServerError, "Failed to import domains", err)
		return
	}

	api.writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": len(domains),
		"status":   "success",
	})
}

// handleExport exports the blocklist
func (api *API) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	domains := api.service.Export()

	switch format {
	case "txt":
		w.Header().Set("Content-Type", "text/plain")
		for _, domain := range domains {
			fmt.Fprintln(w, domain)
		}
	case "json":
		fallthrough
	default:
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"domains": domains,
			"count":   len(domains),
			"format":  "json",
		}
		json.NewEncoder(w).Encode(response)
	}
}

// handleGetStats returns DNS blocklist statistics
func (api *API) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := api.service.GetStats()
	api.writeJSON(w, http.StatusOK, stats)
}

// writeJSON writes a JSON response
func (api *API) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (api *API) writeError(w http.ResponseWriter, status int, message string, err error) {
	response := map[string]interface{}{
		"error":  message,
		"status": status,
	}

	if err != nil {
		response["details"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
