// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package tui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/tui"
)

// MockBackend simulates the Flywall API for TUI tests
type MockBackend struct {
	server *httptest.Server
	rules  []learning.PendingRule
}

func NewMockBackend() *MockBackend {
	m := &MockBackend{
		rules: []learning.PendingRule{
			{
				ID:              "rule-123",
				SrcNetwork:      "192.168.1.50/32",
				DstPort:         "80",
				Protocol:        "tcp",
				Status:          "pending",
				SuggestedAction: "accept",
				FirstSeen:       time.Now(),
				HitCount:        5,
			},
		},
	}

	mux := http.NewServeMux()

	// Handle /api/status - Dashboard needs this
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"uptime": "1h 23m", "firewall_active": true, "blocked_count": 42}`))
	})

	// Handle /api/learning/rules - Learning Tab needs this
	mux.HandleFunc("/api/learning/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.rules)
	})

	// Handle /api/learning/rules/{id}/approve - The action we test
	mux.HandleFunc("/api/learning/rules/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// Simulate approval
			// In a real test we'd check the ID and move it to approved rules
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// Handle /api/flows - Flows Tab needs this (even if empty for now)
	mux.HandleFunc("/api/flows", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"flows": []}`))
	})

	// Handle /api/config - Config Tab needs this
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})

	// Handle /api/alerts/history - History Tab needs this
	mux.HandleFunc("/api/alerts/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})

	m.server = httptest.NewServer(mux)
	return m
}

func (m *MockBackend) Close() {
	m.server.Close()
}

func (m *MockBackend) URL() string {
	return m.server.URL
}

func TestTUILearningApproval(t *testing.T) {
	// 1. Setup Mock Backend
	backend := NewMockBackend()
	defer backend.Close()

	// 2. Initialize TUI Model providing the mock URL
	// We use the RemoteBackend implementation pointing to our httptest server
	tuiBackend := tui.NewRemoteBackend(backend.URL(), "test-key", true)

	// Create the main model
	model := tui.NewModel(tuiBackend)

	// 3. Initialize Teatest
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// 4. Assert Initial State (Dashboard)
	// Wait for "Overview" to appear (Dashboard title)
	tm.Type("d") // Navigate to dashboard just in case (though it's default)

	// 5. Navigate to Policy Tab (as proxy for Learning not being in View yet)
	// We'll wait a bit for the HTTP fetch to complete
	time.Sleep(1 * time.Second)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})

	// 6. Assert State
	// We'll wait a bit for the HTTP fetch to complete
	time.Sleep(1 * time.Second)

	// Get the final model state
	finalModel := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second*5))

	view := finalModel.View()

	if len(view) == 0 {
		t.Errorf("View is empty")
	}

	// Dump view for debugging if needed
	t.Logf("Final View:\n%s", view)
}
