// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"grimm.is/flywall/internal/ctlplane"
)

// AnalyticsHandlers provides HTTP handlers for historical flow analytics
type AnalyticsHandlers struct {
	client ctlplane.ControlPlaneClient
}

// NewAnalyticsHandlers creates a new AnalyticsHandlers
func NewAnalyticsHandlers(client ctlplane.ControlPlaneClient) *AnalyticsHandlers {
	return &AnalyticsHandlers{
		client: client,
	}
}

// HandleGetBandwidth returns bandwidth time series
func (h *AnalyticsHandlers) HandleGetBandwidth(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, to := parseTimeRange(fromStr, toStr)

	points, err := h.client.GetAnalyticsBandwidth(&ctlplane.GetAnalyticsBandwidthArgs{
		SrcMAC: mac,
		From:   from,
		To:     to,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"points": points,
	})
}

// HandleGetTopTalkers returns top devices by traffic volume
func (h *AnalyticsHandlers) HandleGetTopTalkers(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")

	from, to := parseTimeRange(fromStr, toStr)
	limit := 10
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	summaries, err := h.client.GetAnalyticsTopTalkers(&ctlplane.GetAnalyticsTopTalkersArgs{
		From:  from,
		To:    to,
		Limit: limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"summaries": summaries,
	})
}

// HandleGetHistoricalFlows returns detailed historical flow records
func (h *AnalyticsHandlers) HandleGetHistoricalFlows(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	from, to := parseTimeRange(fromStr, toStr)
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}
	offset := 0
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			offset = v
		}
	}

	summaries, err := h.client.GetAnalyticsFlows(&ctlplane.GetAnalyticsFlowsArgs{
		SrcMAC: mac,
		From:   from,
		To:     to,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"flows": summaries,
	})
}

func parseTimeRange(fromStr, toStr string) (time.Time, time.Time) {
	to := time.Now()
	if toStr != "" {
		if v, err := strconv.ParseInt(toStr, 10, 64); err == nil {
			to = time.Unix(v, 0)
		} else if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	from := to.Add(-1 * time.Hour)
	if fromStr != "" {
		if v, err := strconv.ParseInt(fromStr, 10, 64); err == nil {
			from = time.Unix(v, 0)
		} else if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}

	return from, to
}
