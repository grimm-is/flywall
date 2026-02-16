// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"grimm.is/flywall/internal/identity"
)

// handleGetDeviceGroups returns all device groups
func (s *Server) handleGetDeviceGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groups, err := s.client.GetDeviceGroups()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to fetch groups: "+err.Error())
		return
	}

	// Helper to ensure empty list not null
	if groups == nil {
		groups = []identity.DeviceGroup{}
	}

	WriteJSON(w, http.StatusOK, groups)
}

// handleUpdateDeviceGroup updates or creates a group
func (s *Server) handleUpdateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var group identity.DeviceGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if group.Name == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Group name is required")
		return
	}

	if err := s.client.UpdateDeviceGroup(group); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to update group: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, group)
}

// handleDeleteDeviceGroup deletes a group
func (s *Server) handleDeleteDeviceGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// path should be /api/groups/{id}
	// Logic: /api/groups/123 -> ["", "api", "groups", "123"]
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Missing group ID")
		return
	}
	id := parts[3]

	if id == "" {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid group ID")
		return
	}

	if err := s.client.DeleteDeviceGroup(id); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to delete group: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
