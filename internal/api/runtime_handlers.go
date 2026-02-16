// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"
)

// getContainersHandler returns a list of active containers
func (s *Server) getContainersHandler(w http.ResponseWriter, r *http.Request) {
	if s.runtime == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, "runtime service not available")
		return
	}

	containers, err := s.runtime.ListContainers(r.Context())
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}
