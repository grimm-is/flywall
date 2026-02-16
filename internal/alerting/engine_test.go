// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package alerting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"grimm.is/flywall/internal/config"
)

func TestEngine_Notifications(t *testing.T) {
	// Mock webhook server
	var receivedPayload map[string]interface{}
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Wait for processing with eventual consistency
	assertPayload := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return receivedPayload != nil && receivedPayload["message"] == "Test Alert"
	}

	engine := NewEngine()

	cfg := &config.NotificationsConfig{
		Enabled: true,
		Channels: []config.NotificationChannel{
			{
				Name:       "test-webhook",
				Type:       "webhook",
				Enabled:    true,
				WebhookURL: server.URL,
			},
		},
		Rules: []config.AlertRule{
			{
				Name:     "test-rule",
				Enabled:  true,
				Channels: []string{"test-webhook"},
				Cooldown: "1s",
			},
		},
	}

	engine.UpdateConfig(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	event := AlertEvent{
		RuleID:    "test-rule",
		RuleName:  "Test Rule",
		Message:   "Test Alert",
		Severity:  LevelWarning,
		Timestamp: time.Now(),
	}

	engine.Trigger(event)

	engine.Trigger(event)

	// Wait for processing (up to 5s to avoid flakes)
	assert.Eventually(t, assertPayload, 5*time.Second, 10*time.Millisecond, "Webhook payload not received")
}
