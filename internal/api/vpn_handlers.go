// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"grimm.is/flywall/internal/vpn"
)

// handleImportVPNConfig handles importing a VPN configuration file.
// POST /api/vpn/import
func (s *Server) handleImportVPNConfig(w http.ResponseWriter, r *http.Request) {
	// 1. Check content type to decide how to read
	contentType := r.Header.Get("Content-Type")

	var content string

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle file upload
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to read file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Parse directly from file
		config, err := vpn.ParseWireGuardConfig(file)
		if err != nil {
			http.Error(w, "Failed to parse config: "+err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
		return
	}

	// Handle raw text or JSON payload containing text
	if strings.Contains(contentType, "application/json") {
		var req struct {
			Config string `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		content = req.Config
	} else {
		// Assume raw text body
		// Limit size to avoid DoS
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, r.Body); err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		content = buf.String()
	}

	reader := strings.NewReader(content)
	config, err := vpn.ParseWireGuardConfig(reader)
	if err != nil {
		http.Error(w, "Failed to parse config: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
