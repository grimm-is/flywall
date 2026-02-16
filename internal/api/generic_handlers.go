// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
)

//go:embed spec/openapi.yaml
var openAPISpec []byte //nolint:typecheck

// ==============================================================================
// Common Error Messages - Deduplication of repeated strings
// ==============================================================================

const (
	ErrInvalidBody      = "Invalid request body"
	ErrControlPlaneDown = "Control plane not connected"
	ErrNotFound         = "Not found"
	ErrVersionRequired  = "Version is required"
	ErrUnauthorized     = "Unauthorized"
	ErrForbidden        = "Forbidden"
)

// ==============================================================================
// Generic JSON Binding Helper
// Reduces: var req T; if err := json.NewDecoder(r.Body).Decode(&req); err != nil { ... }
// ==============================================================================

// BindJSON decodes JSON from request body into the provided pointer.
// Returns true on success, false if decoding failed (error response already sent).
func BindJSON[T any](w http.ResponseWriter, r *http.Request, dest *T) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		WriteError(w, http.StatusBadRequest, ErrInvalidBody)
		return false
	}
	return true
}

// BindJSONCustomErr decodes JSON with a custom error message.
func BindJSONCustomErr[T any](w http.ResponseWriter, r *http.Request, dest *T, errMsg string) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		WriteError(w, http.StatusBadRequest, errMsg)
		return false
	}
	return true
}

// BindJSONLenient decodes JSON but allows unknown fields (like _status from UI).
// Use this for endpoints where UI may send extra metadata fields.
func BindJSONLenient[T any](w http.ResponseWriter, r *http.Request, dest *T) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		WriteError(w, http.StatusBadRequest, ErrInvalidBody)
		return false
	}
	return true
}

// ==============================================================================
// Generic Handler Patterns
// Reduces: cfg := s.getConfigOrWrite(w); if cfg == nil { return }; WriteJSON(w, 200, cfg.Field)
// ==============================================================================

// HandleGet wraps a simple GET handler that returns data.
func HandleGet(w http.ResponseWriter, r *http.Request, dataFn func() (interface{}, error)) {
	data, err := dataFn()
	if err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, data)
}

// HandleGetData wraps a GET handler returning typed data without error.
func HandleGetData(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

// HandleUpdate wraps a POST/PUT handler that updates config.
func HandleUpdate(w http.ResponseWriter, r *http.Request, updateFn func() error) {
	if err := updateFn(); err != nil {
		WriteErrorCtx(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// RequireControlPlane returns true if control plane client is connected.
func (s *Server) RequireControlPlane(w http.ResponseWriter, r *http.Request) bool {
	if s.client == nil {
		WriteErrorCtx(w, r, http.StatusServiceUnavailable, ErrControlPlaneDown)
		return false
	}
	return true
}

// GetConfigSnapshot gets config from control plane or returns a local snapshot.
func (s *Server) GetConfigSnapshot(w http.ResponseWriter, r *http.Request) *config.Config {
	source := r.URL.Query().Get("source")
	if source == "running" && s.client != nil {
		cfg, err := s.client.GetRunningConfig()
		if err != nil {
			WriteErrorCtx(w, r, http.StatusInternalServerError, err.Error())
			return nil
		}
		return cfg
	}
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.Config.Clone()
}

// ==============================================================================
// Success/Error Response Helpers
// ==============================================================================

// SuccessResponse writes a standard success JSON response.
func SuccessResponse(w http.ResponseWriter) {
	WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// SuccessWithData writes success with additional data fields.
func SuccessWithData(w http.ResponseWriter, data map[string]interface{}) {
	data["success"] = true
	WriteJSON(w, http.StatusOK, data)
}

// SendErrorJSON writes a standard error JSON response.
func SendErrorJSON(w http.ResponseWriter, err error) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	})
}

// ErrorMessage writes an error with a message string.
func ErrorMessage(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

// Batch Request/Response types
type BatchRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   any    `json:"body"` // Optional
}

type BatchResponse struct {
	Status int `json:"status"`
	Body   any `json:"body"`
}

type batchResponseWriter struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func (w *batchResponseWriter) Header() http.Header {
	return w.header
}

func (w *batchResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *batchResponseWriter) WriteHeader(code int) {
	w.statusCode = code
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	var requests []BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if len(requests) > 20 {
		WriteErrorCtx(w, r, http.StatusBadRequest, "Too many requests in batch")
		return
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}

	if !s.rateLimiter.AllowN("batch:"+ip, 60, time.Minute, len(requests)) {
		WriteErrorCtx(w, r, http.StatusTooManyRequests, "Rate limit exceeded for batch")
		return
	}

	responses := make([]BatchResponse, len(requests))
	for i, req := range requests {
		var bodyReader io.Reader
		if req.Body != nil {
			b, err := json.Marshal(req.Body)
			if err == nil {
				bodyReader = bytes.NewReader(b)
			}
		}

		subReq, err := http.NewRequest(req.Method, req.Path, bodyReader)
		if err != nil {
			responses[i] = BatchResponse{Status: 500, Body: "Failed to create request: " + err.Error()}
			continue
		}

		subReq.RemoteAddr = r.RemoteAddr
		subReq.Header.Set("Content-Type", "application/json")
		if auth := r.Header.Get("Authorization"); auth != "" {
			subReq.Header.Set("Authorization", auth)
		}
		for _, c := range r.Cookies() {
			subReq.AddCookie(c)
		}

		rr := &batchResponseWriter{header: make(http.Header), statusCode: http.StatusOK}
		s.mux.ServeHTTP(rr, subReq)

		var respBody any
		if rr.body.Len() > 0 {
			if contentType := rr.header.Get("Content-Type"); strings.Contains(contentType, "application/json") {
				_ = json.Unmarshal(rr.body.Bytes(), &respBody)
			} else {
				respBody = rr.body.String()
			}
		}

		responses[i] = BatchResponse{
			Status: rr.statusCode,
			Body:   respBody,
		}
	}

	WriteJSON(w, http.StatusOK, responses)
}

func (s *Server) handleBrand(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, brand.Get())
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(openAPISpec)
}

func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="description" content="SwaggerHTT" />
    <title>` + brand.Name + ` API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" crossorigin></script>
<script>
    window.onload = () => {
        window.ui = SwaggerUIBundle({
            url: '/api/openapi.yaml',
            dom_id: '#swagger-ui',
        });
    };
</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
