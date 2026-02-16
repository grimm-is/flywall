// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"grimm.is/flywall/internal/audit"
)

// auditMiddlewareEnhanced provides comprehensive audit logging for all API requests
func (s *Server) auditMiddlewareEnhanced(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
		}

		// Create context with audit information
		ctx := context.WithValue(r.Context(), "ip_address", getClientIP(r))
		ctx = context.WithValue(ctx, "user_agent", r.UserAgent())

		// Add API key info if available
		if key := GetAPIKey(r.Context()); key != nil {
			ctx = context.WithValue(ctx, "api_key_id", key.ID)
			ctx = context.WithValue(ctx, "user_id", key.Name)
		}

		// Process request
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Calculate duration
		duration := time.Since(start)

		// Determine event type and severity
		eventType, severity := getAuditEventType(r.Method, wrapped.statusCode)

		// Create audit event
		event := audit.AuditEvent{
			EventType: eventType,
			Severity:  severity,
			Action:    r.Method + " " + r.URL.Path,
			Resource:  getResourceFromPath(r.URL.Path),
			Success:   wrapped.statusCode < 400,
			Duration:  duration.Milliseconds(),
			RequestData: map[string]interface{}{
				"method":  r.Method,
				"path":    r.URL.Path,
				"query":   r.URL.RawQuery,
				"headers": sanitizeHeaders(r.Header),
			},
			ResponseData: map[string]interface{}{
				"status_code": wrapped.statusCode,
			},
		}

		// Add error message for failed requests
		if wrapped.statusCode >= 400 {
			event.ErrorMessage = getErrorMessage(wrapped.statusCode)
		}

		// Log the event
		if s.auditLogger != nil {
			if err := s.auditLogger.LogEvent(ctx, event); err != nil {
				s.logger.Error("Failed to log audit event", "error", err)
			}
		}
	})
}

// getAuditEventType determines the audit event type based on HTTP method and status
func getAuditEventType(method string, statusCode int) (audit.EventType, audit.Severity) {
	// Determine severity based on status code
	var severity audit.Severity
	switch {
	case statusCode >= 500:
		severity = audit.SeverityError
	case statusCode >= 400:
		severity = audit.SeverityWarn
	default:
		severity = audit.SeverityInfo
	}

	// Determine event type based on method and path pattern
	switch method {
	case http.MethodGet, http.MethodHead:
		return audit.EventType("resource_read"), severity
	case http.MethodPost:
		return audit.EventType("resource_create"), severity
	case http.MethodPut, http.MethodPatch:
		return audit.EventType("resource_update"), severity
	case http.MethodDelete:
		return audit.EventType("resource_delete"), severity
	default:
		return audit.EventType("resource_access"), severity
	}
}

// getResourceFromPath extracts resource type from URL path
func getResourceFromPath(path string) string {
	// Simple path parsing - could be enhanced with proper routing
	parts := splitPath(path)
	if len(parts) >= 2 {
		return parts[1]
	}
	return "unknown"
}

// sanitizeHeaders removes sensitive headers from audit logs
func sanitizeHeaders(headers http.Header) map[string]string {
	sanitized := make(map[string]string)

	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"set-cookie":    true,
	}

	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = strings.Join(values, ", ")
		}
	}

	return sanitized
}

// getErrorMessage returns a user-friendly error message for status codes
func getErrorMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 409:
		return "Conflict"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		return http.StatusText(statusCode)
	}
}

// splitPath splits a URL path into components
func splitPath(path string) []string {
	if path == "" || path == "/" {
		return []string{}
	}

	// Remove leading slash and split
	path = strings.TrimPrefix(path, "/")
	return strings.Split(path, "/")
}

// audit helper for manual audit logging
func (s *Server) audit(r *http.Request, eventType string, action string, details map[string]interface{}) {
	if s.auditLogger == nil {
		return
	}

	event := audit.AuditEvent{
		EventType:   audit.EventType(eventType),
		Severity:    audit.SeverityInfo,
		Action:      action,
		Resource:    "auth",
		Success:     true,
		Timestamp:   time.Now(),
		RequestData: details,
	}

	// Add context info
	ctx := r.Context()
	ctx = context.WithValue(ctx, "ip_address", getClientIP(r))

	if err := s.auditLogger.LogEvent(ctx, event); err != nil {
		s.logger.Error("Failed to log audit event", "error", err)
	}
}
