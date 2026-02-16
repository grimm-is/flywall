// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"grimm.is/flywall/internal/logging"
)

// EventType defines the type of audit event
type EventType string

const (
	// Authentication events
	EventAuthLogin       EventType = "auth_login"
	EventAuthLogout      EventType = "auth_logout"
	EventAuthFailure     EventType = "auth_failure"
	EventAuthAPIKeyCreate EventType = "auth_api_key_create"
	EventAuthAPIKeyDelete EventType = "auth_api_key_delete"
	
	// Configuration events
	EventConfigCreate    EventType = "config_create"
	EventConfigUpdate    EventType = "config_update"
	EventConfigDelete    EventType = "config_delete"
	EventConfigBackup    EventType = "config_backup"
	EventConfigRestore   EventType = "config_restore"
	
	// Firewall events
	EventRuleCreate      EventType = "rule_create"
	EventRuleUpdate      EventType = "rule_update"
	EventRuleDelete      EventType = "rule_delete"
	EventPolicyApply     EventType = "policy_apply"
	
	// System events
	EventSystemStart     EventType = "system_start"
	EventSystemStop      EventType = "system_stop"
	EventSystemRestart   EventType = "system_restart"
	
	// Security events
	EventSecurityBlock   EventType = "security_block"
	EventSecurityUnblock EventType = "security_unblock"
	EventSecurityAlert   EventType = "security_alert"
)

// Severity defines the severity level of an audit event
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
	SeverityFatal Severity = "fatal"
)

// AuditEvent represents a comprehensive audit log entry
type AuditEvent struct {
	Timestamp    time.Time         `json:"timestamp"`
	EventType    EventType         `json:"event_type"`
	Severity     Severity          `json:"severity"`
	UserID       string            `json:"user_id,omitempty"`
	APIKeyID     string            `json:"api_key_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Resource     string            `json:"resource,omitempty"`
	ResourceID   string            `json:"resource_id,omitempty"`
	Action       string            `json:"action"`
	Success      bool              `json:"success"`
	ErrorMessage string            `json:"error_message,omitempty"`
	RequestData  map[string]interface{} `json:"request_data,omitempty"`
	ResponseData map[string]interface{} `json:"response_data,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Duration     int64             `json:"duration_ms,omitempty"`
}

// Logger provides enhanced audit logging capabilities
type Logger struct {
	store  *Store
	logger *logging.Logger
}

// NewLogger creates a new audit logger
func NewLogger(store *Store, logger *logging.Logger) *Logger {
	return &Logger{
		store:  store,
		logger: logger,
	}
}

// LogEvent logs an audit event with full context
func (l *Logger) LogEvent(ctx context.Context, event AuditEvent) error {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// Extract context information if available
	if userID := ctx.Value("user_id"); userID != nil {
		event.UserID = fmt.Sprintf("%v", userID)
	}
	if apiKeyID := ctx.Value("api_key_id"); apiKeyID != nil {
		event.APIKeyID = fmt.Sprintf("%v", apiKeyID)
	}
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		event.SessionID = fmt.Sprintf("%v", sessionID)
	}
	if ipAddress := ctx.Value("ip_address"); ipAddress != nil {
		event.IPAddress = fmt.Sprintf("%v", ipAddress)
	}
	if userAgent := ctx.Value("user_agent"); userAgent != nil {
		event.UserAgent = fmt.Sprintf("%v", userAgent)
	}
	
	// Log to structured logger
	l.logStructured(event)
	
	// Persist to store if available
	if l.store != nil {
		// Convert to legacy Event format for storage
		legacyEvent := Event{
			Timestamp: event.Timestamp,
			User:      event.UserID,
			Session:   event.SessionID,
			Action:    string(event.EventType),
			Resource:  event.Resource,
			Details: map[string]any{
				"severity":      string(event.Severity),
				"api_key_id":    event.APIKeyID,
				"ip_address":    event.IPAddress,
				"user_agent":    event.UserAgent,
				"resource_id":   event.ResourceID,
				"action":        event.Action,
				"success":       event.Success,
				"error_message": event.ErrorMessage,
				"request_data":  event.RequestData,
				"response_data": event.ResponseData,
				"metadata":      event.Metadata,
				"duration_ms":   event.Duration,
			},
			Status: 0,
			IP:     event.IPAddress,
		}
		
		if err := l.store.Write(legacyEvent); err != nil {
			l.logger.Error("Failed to persist audit event", "error", err)
			return err
		}
	}
	
	return nil
}

// logStructured logs the event to the structured logger
func (l *Logger) logStructured(event AuditEvent) {
	// Determine log level based on severity
	switch event.Severity {
	case SeverityInfo:
		l.logger.Info("AUDIT",
			"event_type", event.EventType,
			"user", event.UserID,
			"action", event.Action,
			"resource", event.Resource,
			"success", event.Success,
			"ip", event.IPAddress,
		)
	case SeverityWarn:
		l.logger.Warn("AUDIT",
			"event_type", event.EventType,
			"user", event.UserID,
			"action", event.Action,
			"resource", event.Resource,
			"success", event.Success,
			"ip", event.IPAddress,
		)
	case SeverityError, SeverityFatal:
		l.logger.Error("AUDIT",
			"event_type", event.EventType,
			"user", event.UserID,
			"action", event.Action,
			"resource", event.Resource,
			"success", event.Success,
			"error", event.ErrorMessage,
			"ip", event.IPAddress,
		)
	}
	
	// If there's additional metadata, log it as JSON
	if len(event.Metadata) > 0 || len(event.RequestData) > 0 || len(event.ResponseData) > 0 {
		if data, err := json.Marshal(map[string]interface{}{
			"metadata":     event.Metadata,
			"request_data": event.RequestData,
			"response_data": event.ResponseData,
		}); err == nil {
			l.logger.Debug("AUDIT_DETAIL", "data", string(data))
		}
	}
}

// Convenience methods for common audit events

// LogAuthEvent logs authentication-related events
func (l *Logger) LogAuthEvent(ctx context.Context, eventType EventType, success bool, username string, ip string, details map[string]interface{}) {
	event := AuditEvent{
		EventType:    eventType,
		Severity:     SeverityInfo,
		UserID:       username,
		IPAddress:    ip,
		Action:       string(eventType),
		Success:      success,
		Metadata:     details,
	}
	
	if !success {
		event.Severity = SeverityWarn
		event.ErrorMessage = "Authentication failed"
	}
	
	l.LogEvent(ctx, event)
}

// LogConfigEvent logs configuration-related events
func (l *Logger) LogConfigEvent(ctx context.Context, eventType EventType, resource string, action string, success bool, details map[string]interface{}) {
	event := AuditEvent{
		EventType:    eventType,
		Severity:     SeverityInfo,
		Resource:     resource,
		Action:       action,
		Success:      success,
		RequestData:  details,
	}
	
	if !success {
		event.Severity = SeverityError
		event.ErrorMessage = "Configuration operation failed"
	}
	
	l.LogEvent(ctx, event)
}

// LogSecurityEvent logs security-related events
func (l *Logger) LogSecurityEvent(ctx context.Context, eventType EventType, resource string, ip string, details map[string]interface{}) {
	event := AuditEvent{
		EventType:    eventType,
		Severity:     SeverityWarn,
		Resource:     resource,
		IPAddress:    ip,
		Action:       string(eventType),
		Success:      true,
		Metadata:     details,
	}
	
	if eventType == EventSecurityAlert {
		event.Severity = SeverityError
	}
	
	l.LogEvent(ctx, event)
}

// LogAPIKeyEvent logs API key management events
func (l *Logger) LogAPIKeyEvent(ctx context.Context, eventType EventType, keyID string, keyName string, success bool, details map[string]interface{}) {
	event := AuditEvent{
		EventType:    eventType,
		Severity:     SeverityInfo,
		APIKeyID:     keyID,
		Resource:     "api_key",
		ResourceID:   keyID,
		Action:       string(eventType),
		Success:      success,
		Metadata: map[string]interface{}{
			"key_name": keyName,
		},
		RequestData: details,
	}
	
	if !success {
		event.Severity = SeverityError
		event.ErrorMessage = "API key operation failed"
	}
	
	l.LogEvent(ctx, event)
}
