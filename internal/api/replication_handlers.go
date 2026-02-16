// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"net/http"
)

// handleReplicationStatus returns the current replication status
// GET /api/replication/status
func (s *Server) handleReplicationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorCtx(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status, err := s.client.GetReplicationStatus()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, "Failed to get replication status: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": status.Status,
	})
}
