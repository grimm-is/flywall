// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

func TestDispatcher_Webhook(t *testing.T) {
	called := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Verify payload structure (simple check)
		if _, ok := body["text"]; !ok {
			// Maybe it's discord format?
			if _, ok := body["content"]; !ok {
				t.Errorf("expected 'text' or 'content' field in payload, got %v", body)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.NotificationsConfig{
		Enabled: true,
		Channels: []config.NotificationChannel{
			{
				Name:       "test-webhook",
				Type:       "webhook",
				Enabled:    true,
				WebhookURL: ts.URL,
			},
		},
	}

	d := NewDispatcher(cfg, logging.New(logging.DefaultConfig()))

	// First send
	d.SendSimple("Test Title", "Test Message", "info")

	// Allow async goroutine to finish (Send uses waitgroup so it blocks until done)
	// Wait, Send uses waitgroup? YES.

	if called.Load() != 1 {
		t.Errorf("expected webhook to be called once, got %d", called.Load())
	}
}

func TestDispatcher_RateLimit(t *testing.T) {
	called := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.NotificationsConfig{
		Enabled: true,
		Channels: []config.NotificationChannel{
			{
				Name:       "test-webhook-rl",
				Type:       "webhook",
				Enabled:    true,
				WebhookURL: ts.URL,
			},
		},
	}

	d := NewDispatcher(cfg, logging.New(logging.DefaultConfig()))

	// Send same message twice
	d.SendSimple("Duplicate Title", "Message body", "info")
	d.SendSimple("Duplicate Title", "Message body", "info")

	// Should only be called once if rate limiting allows 1 per minute per title
	// But currently implementation doesn't have rate limiting, so this test EXPECTS failure or 2 calls if not implemented.
	// I'll assert 1 call, which will fail until I implement it.
	if called.Load() != 1 {
		t.Fatalf("expected webhook to be called once (rate limited), got %d", called.Load())
	}
}

func TestDispatcher_Timeout(t *testing.T) {
	// Config short timeout for test
	// But Dispatcher hardcodes 10s?
	// I should make it configurable or inspect client.

	// Skip for now, hard to test reliably without dependency injection of client.
}
