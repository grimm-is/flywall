// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"net/smtp"

	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/logging"
)

// Level constants
const (
	LevelInfo     = "info"
	LevelWarning  = "warning"
	LevelCritical = "critical"
)

// Notification represents a notification event
type Notification struct {
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Level     string                 `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Dispatcher manages notification channels and dispatching
type Dispatcher struct {
	config *config.NotificationsConfig
	logger *logging.Logger
	mu     sync.RWMutex

	// Rate limiting state
	lastSent map[string]time.Time

	// HTTP client with timeout
	httpClient *http.Client

	// Email sender (injectable for testing)
	emailSender func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewDispatcher creates a new notification dispatcher
func NewDispatcher(cfg *config.NotificationsConfig, logger *logging.Logger) *Dispatcher {
	if logger == nil {
		logger = logging.Default().WithComponent("notification")
	}
	return &Dispatcher{
		config:   cfg,
		logger:   logger,
		lastSent: make(map[string]time.Time),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		emailSender: smtp.SendMail,
	}
}

// UpdateConfig updates the dispatcher configuration
func (d *Dispatcher) UpdateConfig(cfg *config.NotificationsConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = cfg
}

// Send dispatches a notification to all enabled and relevant channels
func (d *Dispatcher) Send(n Notification) {
	d.mu.RLock()
	cfg := d.config
	d.mu.RUnlock()

	if cfg == nil || !cfg.Enabled {
		return
	}

	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now()
	}

	var wg sync.WaitGroup

	for _, ch := range cfg.Channels {
		if !ch.Enabled {
			continue
		}

		// check level filtering
		if !shouldSend(n.Level, ch.Level) {
			continue
		}

		// Rate limiting (deduplication)
		// Skip if sent within last 60s for same title on same channel
		// CRITICAL alerts bypass rate limiting? Ideally yes, but duplicating 100 critical alerts is also bad.
		// Let's rate limit per minute to avoid storms.
		if d.isRateLimited(ch.Name, n.Title) {
			d.logger.Debug("notification rate limited", "channel", ch.Name, "title", n.Title)
			continue
		}

		wg.Add(1)
		go func(channel config.NotificationChannel) {
			defer wg.Done()
			if err := d.sendToChannel(channel, n); err != nil {
				d.logger.Error("failed to send notification",
					"channel", channel.Name,
					"type", channel.Type,
					"error", err)
			}
		}(ch)
	}

	wg.Wait()
}

// isRateLimited checks if a notification should be skipped due to rate limiting
func (d *Dispatcher) isRateLimited(channelName, title string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := channelName + ":" + title
	last, ok := d.lastSent[key]
	now := time.Now()

	if ok && now.Sub(last) < 60*time.Second {
		return true
	}

	d.lastSent[key] = now

	// Cleanup old entries occasionally?
	// For now, map grows unbounded but keys are limited by unique titles.
	// We could add a cleanup goroutine or check map size.
	if len(d.lastSent) > 1000 {
		// naive cleanup: clear all
		d.lastSent = make(map[string]time.Time)
		d.lastSent[key] = now
	}

	return false
}

// SendSimple is a helper for simple messages
func (d *Dispatcher) SendSimple(title, message, level string) {
	d.Send(Notification{
		Title:   title,
		Message: message,
		Level:   level,
	})
}

// shouldSend checks if a message level meets the channel's minimum level
func shouldSend(msgLevel, chanLevel string) bool {
	// If channel has no level, accept all
	if chanLevel == "" {
		return true
	}

	levels := map[string]int{
		LevelInfo:     1,
		LevelWarning:  2,
		LevelCritical: 3,
	}

	m := levels[strings.ToLower(msgLevel)]
	c := levels[strings.ToLower(chanLevel)]

	return m >= c
}

func (d *Dispatcher) sendToChannel(ch config.NotificationChannel, n Notification) error {
	switch strings.ToLower(ch.Type) {
	case "webhook", "slack", "discord":
		return d.sendWebhook(ch, n)
	case "ntfy":
		return d.sendNtfy(ch, n)
	case "pushover":
		return d.sendPushover(ch, n)
	case "email":
		return d.sendEmail(ch, n)
	default:
		return fmt.Errorf("unknown channel type: %s", ch.Type)
	}
}

// Channel Implementations

func (d *Dispatcher) sendWebhook(ch config.NotificationChannel, n Notification) error {
	if ch.WebhookURL == "" {
		return fmt.Errorf("missing webhook_url")
	}

	// Payload format depends on type, but for generic webhook we send JSON
	// For Slack/Discord we might want specific mapping, but let's start with generic JSON
	// or a simple text payload if it's slack-compatible

	payload := map[string]interface{}{
		"text": fmt.Sprintf("*%s*\n%s\n_Level: %s_", n.Title, n.Message, n.Level),
	}

	// If specific format is needed, we can check ch.Type further
	if ch.Type == "discord" {
		payload = map[string]interface{}{
			"content": fmt.Sprintf("**%s**\n%s", n.Title, n.Message),
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", ch.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (d *Dispatcher) sendNtfy(ch config.NotificationChannel, n Notification) error {
	// ntfy.sh/topic
	url := ch.Server
	if url == "" {
		url = "https://ntfy.sh"
	}
	if ch.Topic == "" {
		return fmt.Errorf("missing topic for ntfy")
	}

	// Construct URL: server/topic
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += ch.Topic

	req, err := http.NewRequest("POST", url, strings.NewReader(n.Message))
	if err != nil {
		return err
	}

	req.Header.Set("Title", n.Title)

	// Map levels to tags/priorities
	switch n.Level {
	case LevelCritical:
		req.Header.Set("Priority", "high")
		req.Header.Set("Tags", "rotating_light")
	case LevelWarning:
		req.Header.Set("Priority", "default")
		req.Header.Set("Tags", "warning")
	case LevelInfo:
		req.Header.Set("Priority", "low")
		req.Header.Set("Tags", "information_source")
	}

	if ch.Password != "" {
		// Basic auth or token? ntfy supports "Authorization: Bearer <token>"
		// config header says "Password". Let's assume Bearer if it looks like a token,
		// but simple password usually implies Basic Auth with username?
		// config struct has generic 'Password' and 'Username' in 'channel'.
		// Let's implement Basic Auth if Username is present.
	}

	// Add custom headers
	for k, v := range ch.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (d *Dispatcher) sendPushover(ch config.NotificationChannel, n Notification) error {
	if ch.APIToken == "" || ch.UserKey == "" {
		return fmt.Errorf("missing api_token or user_key")
	}

	url := "https://api.pushover.net/1/messages.json"

	payload := map[string]interface{}{
		"token":     ch.APIToken,
		"user":      ch.UserKey,
		"message":   n.Message,
		"title":     n.Title,
		"timestamp": n.Timestamp.Unix(),
	}

	if ch.Sound != "" {
		payload["sound"] = ch.Sound
	}

	// Priority mapping
	if n.Level == LevelCritical {
		payload["priority"] = 1
	} else if ch.Priority != 0 {
		payload["priority"] = ch.Priority
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("pushover failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (d *Dispatcher) sendEmail(ch config.NotificationChannel, n Notification) error {
	if ch.SMTPHost == "" || len(ch.To) == 0 {
		return fmt.Errorf("missing smtp_host or recipients")
	}

	host := ch.SMTPHost
	port := ch.SMTPPort
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	var auth smtp.Auth
	if ch.SMTPUser != "" {
		auth = smtp.PlainAuth("", ch.SMTPUser, string(ch.SMTPPassword), host)
	}

	// Prepare email body
	// Headers
	headers := make(map[string]string)
	headers["From"] = ch.From
	if headers["From"] == "" {
		headers["From"] = "flywall@localhost"
	}
	headers["To"] = strings.Join(ch.To, ",")
	headers["Subject"] = fmt.Sprintf("[%s] %s", n.Level, n.Title)
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""

	headerStr := ""
	for k, v := range headers {
		headerStr += fmt.Sprintf("%s: %s\r\n", k, v)
	}

	msg := []byte(headerStr + "\r\n" + n.Message + "\r\n")

	// Use d.emailSender (allows mocking)
	if d.emailSender != nil {
		return d.emailSender(addr, auth, headers["From"], ch.To, msg)
	}

	// Fallback to real smtp.SendMail (though NewDispatcher sets it)
	return smtp.SendMail(addr, auth, headers["From"], ch.To, msg)
}
