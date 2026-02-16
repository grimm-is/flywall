// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"grimm.is/flywall/internal/config"
)

// Engine manages alert rules and handles incoming events.
type Engine struct {
	mu         sync.RWMutex
	rules      map[string]*AlertRule
	channels   map[string]config.NotificationChannel
	history    []AlertEvent
	maxHistory int
	eventChan  chan AlertEvent
	stopChan   chan struct{}
	httpClient *http.Client
}

// NewEngine creates a new Alerting Engine.
func NewEngine() *Engine {
	return &Engine{
		rules:      make(map[string]*AlertRule),
		channels:   make(map[string]config.NotificationChannel),
		history:    make([]AlertEvent, 0),
		maxHistory: 1000,
		eventChan:  make(chan AlertEvent, 100),
		stopChan:   make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// UpdateConfig updates the engine's rules and channels from the configuration.
func (e *Engine) UpdateConfig(cfg *config.NotificationsConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if cfg == nil || !cfg.Enabled {
		e.rules = make(map[string]*AlertRule)
		e.channels = make(map[string]config.NotificationChannel)
		return
	}

	// Update channels
	e.channels = make(map[string]config.NotificationChannel)
	for _, ch := range cfg.Channels {
		e.channels[ch.Name] = ch
	}

	// Update rules, preserving runtime state (LastFired)
	newRules := make(map[string]*AlertRule)
	for _, r := range cfg.Rules {
		existing, ok := e.rules[r.Name]

		cooldown, _ := time.ParseDuration(r.Cooldown)
		if cooldown == 0 {
			cooldown = 15 * time.Minute // Default cooldown
		}

		rule := &AlertRule{
			ID:        r.Name,
			Name:      r.Name,
			Enabled:   r.Enabled,
			Severity:  AlertLevel(r.Severity),
			Condition: r.Condition,
			Channels:  r.Channels,
			Cooldown:  cooldown,
		}

		if ok {
			rule.LastFired = existing.LastFired
		}
		newRules[r.Name] = rule
	}
	e.rules = newRules
}

// Start starts the engine's background processing.
func (e *Engine) Start(ctx context.Context) {
	go e.run(ctx)
}

func (e *Engine) run(ctx context.Context) {
	for {
		select {
		case event := <-e.eventChan:
			e.handleEvent(event)
		case <-e.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// handleEvent processes an incoming alert event.
func (e *Engine) handleEvent(event AlertEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Logic to match event to rules or generic handling
	// For now, let's assume events are pre-matched or we just log them/add to history

	e.history = append(e.history, event)
	if len(e.history) > e.maxHistory {
		e.history = e.history[1:]
	}

	log.Printf("[ALERT] %s: %s (%s)", event.Severity, event.Message, event.RuleName)

	// Send to channels if associated with a rule
	if event.RuleID != "" {
		if rule, ok := e.rules[event.RuleID]; ok && rule.Enabled {
			if time.Since(rule.LastFired) > rule.Cooldown {
				rule.LastFired = time.Now()
				e.notify(rule, event)
			}
		}
	}
}

// notify sends the alert to configured channels.
func (e *Engine) notify(rule *AlertRule, event AlertEvent) {
	for _, chName := range rule.Channels {
		if ch, ok := e.channels[chName]; ok && ch.Enabled {
			go e.sendToChannel(ch, event)
		}
	}
}

func (e *Engine) sendToChannel(ch config.NotificationChannel, event AlertEvent) {
	switch ch.Type {
	case "webhook", "slack", "discord", "ntfy":
		e.sendWebhook(ch, event)
	case "email":
		e.sendEmail(ch, event)
	default:
		log.Printf("[ALERT] Unsupported channel type: %s", ch.Type)
	}
}

func (e *Engine) sendWebhook(ch config.NotificationChannel, event AlertEvent) {
	url := ch.WebhookURL
	if ch.Type == "ntfy" && ch.Server != "" && ch.Topic != "" {
		url = fmt.Sprintf("%s/%s", ch.Server, ch.Topic)
	}

	if url == "" {
		log.Printf("[ALERT] Webhook URL missing for channel %s", ch.Name)
		return
	}

	var payload interface{}
	switch ch.Type {
	case "slack":
		payload = map[string]string{"text": fmt.Sprintf("*%s*: %s", event.Severity, event.Message)}
	case "discord":
		payload = map[string]string{"content": fmt.Sprintf("**%s**: %s", event.Severity, event.Message)}
	default: // generic webhook or ntfy
		payload = event
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ALERT] Failed to marshal webhook payload: %v", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[ALERT] Failed to create webhook request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range ch.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		log.Printf("[ALERT] Webhook delivery failed for %s: %v", ch.Name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ALERT] Webhook returned non-success status for %s: %d", ch.Name, resp.StatusCode)
	}
}

func (e *Engine) sendEmail(ch config.NotificationChannel, event AlertEvent) {
	if ch.SMTPHost == "" || len(ch.To) == 0 {
		log.Printf("[ALERT] SMTP configuration missing for channel %s", ch.Name)
		return
	}

	auth := smtp.PlainAuth("", ch.SMTPUser, string(ch.SMTPPassword), ch.SMTPHost)
	addr := fmt.Sprintf("%s:%d", ch.SMTPHost, ch.SMTPPort)

	subject := fmt.Sprintf("Flywall Alert: %s", event.RuleName)
	body := fmt.Sprintf("Severity: %s\nMessage: %s\nTime: %s\n",
		event.Severity, event.Message, event.Timestamp.Format(time.RFC3339))

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s",
		strings.Join(ch.To, ","), subject, body))

	err := smtp.SendMail(addr, auth, ch.From, ch.To, msg)
	if err != nil {
		log.Printf("[ALERT] Email delivery failed for %s: %v", ch.Name, err)
	}
}


// Trigger triggers a manual alert event.
func (e *Engine) Trigger(event AlertEvent) {
	select {
	case e.eventChan <- event:
	default:
		log.Printf("[ALERT] Event queue full, dropping event: %s", event.Message)
	}
}

// GetHistory returns the alert history.
func (e *Engine) GetHistory() []AlertEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return a copy
	res := make([]AlertEvent, len(e.history))
	copy(res, e.history)
	return res
}
