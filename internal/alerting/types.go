package alerting

import (
	"time"
)

// AlertLevel represents the severity of an alert.
type AlertLevel string

const (
	LevelInfo     AlertLevel = "info"
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
)

// AlertRule defines when an alert should be triggered.
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	Severity    AlertLevel    `json:"severity"`
	Condition   string        `json:"condition"` // e.g. "device.anomaly", "bandwidth.wan > 100Mbps"
	Channels    []string      `json:"channels"`  // Names of notification channels
	Cooldown    time.Duration `json:"cooldown"`
	LastFired   time.Time     `json:"last_fired"`
}

// AlertEvent represents a triggered alert occurrence.
type AlertEvent struct {
	ID        string     `json:"id"`
	RuleID    string     `json:"rule_id"`
	RuleName  string     `json:"rule_name"`
	Message   string     `json:"message"`
	Severity  AlertLevel `json:"severity"`
	Timestamp time.Time  `json:"timestamp"`
	Data      any        `json:"data,omitempty"`
}

// Action represents an action to take when an alert fires.
type Action struct {
	Type   string `json:"type"`   // "webhook", "email", "log"
	Target string `json:"target"` // URL, Email address, or channel name
}
