// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"net/http"
	"strconv"
	"time"

	"grimm.is/flywall/internal/logging"
)

func (s *Server) handleGetDNSQueryHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	search := r.URL.Query().Get("search")

	services, err := s.client.GetDNSQueryHistory(limit, offset, search)
	if err != nil {
		logging.Error("Failed to get DNS query history", "error", err)
		WriteError(w, http.StatusInternalServerError, "Failed to get DNS query history")
		return
	}

	WriteJSON(w, http.StatusOK, services)
}

func (s *Server) handleGetDNSStats(w http.ResponseWriter, r *http.Request) {
	// Default to last 24h
	to := time.Now()
	from := to.Add(-24 * time.Hour)

	if f := r.URL.Query().Get("from"); f != "" {
		if val, err := time.Parse(time.RFC3339, f); err == nil {
			from = val
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if val, err := time.Parse(time.RFC3339, t); err == nil {
			to = val
		}
	}

	stats, err := s.client.GetDNSStats(from, to)
	if err != nil {
		logging.Error("Failed to get DNS stats", "error", err)
		WriteError(w, http.StatusInternalServerError, "Failed to get DNS stats")
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}
