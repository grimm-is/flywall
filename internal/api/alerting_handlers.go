// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"grimm.is/flywall/internal/alerting"
)

// HandleGetAlertHistory returns the alert history.
// GET /api/alerts/history?limit=100
func (s *Server) HandleGetAlertHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}

	history, err := s.client.GetAlertHistory(limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, history)
}

// HandleGetAlertRules returns the alert rules.
// GET /api/alerts/rules
func (s *Server) HandleGetAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.client.GetAlertRules()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, rules)
}

// HandleUpdateAlertRule updates or creates an alert rule.
// POST /api/alerts/rules
func (s *Server) HandleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	var rule alerting.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if rule.Name == "" {
		WriteError(w, http.StatusBadRequest, "rule name is required")
		return
	}

	if err := s.client.UpdateAlertRule(rule); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}
