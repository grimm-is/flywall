package alerting

import (
	"context"
	"fmt"
	"log"
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
	// TODO: Implement actual notification sending (webhook, email, etc.)
	fmt.Printf("[NOTIFY] Sending alert to %s (%s): %s\n", ch.Name, ch.Type, event.Message)
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
