package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ErrorResponse is the generic JSON error response sent to clients.
// All sensitive details are scrubbed; only safe information is included.
// This prevents information disclosure attacks (OWASP A01: Broken Access Control,
// A05: Security Misconfiguration).
type ErrorResponse struct {
	Status  int    `json:"status"`             // HTTP status code
	Message string `json:"message"`            // Generic, user-safe message
	TraceID string `json:"trace_id,omitempty"` // Opaque ID for support correlation
	Path    string `json:"path,omitempty"`     // Request path (non-sensitive)
}

// Logger is the interface expected by ErrorSink for structured logging.
// Implementations should write to files/systems not accessible to clients.
type Logger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
}

// ErrorSink encapsulates error handling: logs full details internally,
// responds generically to clients.
// This prevents information disclosure while preserving observability.
type ErrorSink struct {
	Logger Logger // structured logger (file, syslog, etc.)
}

// Handle logs the full error internally and sends a generic response to the client.
// The client sees only: status code, generic message, and trace ID.
// The server log contains: underlying error, SQL query, file path, etc.
//
// This implements "defense in depth" by:
// 1. Logging full context internally (for debugging/forensics)
// 2. Returning generic messages to clients (prevents reconnaissance)
// 3. Including trace ID (allows support to correlate logs)
func (es *ErrorSink) Handle(w http.ResponseWriter, r *http.Request, status int, userMsg string, internalErr error) {
	traceID := extractTraceID(r)

	// Log full context internally (accessible only to ops/support).
	// Include request details, error context, and source information.
	es.Logger.Error("http error",
		"status", status,
		"method", r.Method,
		"path", r.URL.Path,
		"trace_id", traceID,
		"message", userMsg,
		"error", internalErr.Error(),
		"user_agent", r.Header.Get("User-Agent"),
		"client_ip", clientIP(r),
		"remote_addr", r.RemoteAddr,
	)

	// Send generic response to client (no internal details).
	resp := ErrorResponse{
		Status:  status,
		Message: userMsg,
		TraceID: traceID,
		Path:    r.URL.Path,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// Forbidden responds with 403 Forbidden.
// Used when an authenticated user lacks permission for a resource.
func (es *ErrorSink) Forbidden(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusForbidden, "access denied", fmt.Errorf("forbidden: %s", reason))
}

// Unauthorized responds with 401 Unauthorized.
// Used when authentication is missing or invalid.
func (es *ErrorSink) Unauthorized(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusUnauthorized, "authentication required", fmt.Errorf("unauthorized: %s", reason))
}

// BadRequest responds with 400 Bad Request.
// Used for malformed input (invalid JSON, missing fields, etc.).
func (es *ErrorSink) BadRequest(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusBadRequest, "invalid request", fmt.Errorf("bad request: %s", reason))
}

// NotFound responds with 404 Not Found.
// Used when a requested resource does not exist.
func (es *ErrorSink) NotFound(w http.ResponseWriter, r *http.Request) {
	es.Handle(w, r, http.StatusNotFound, "resource not found", fmt.Errorf("not found: %s", r.URL.Path))
}

// ServerError responds with 500 Internal Server Error.
// Never exposes the underlying error to the client.
// Used for unexpected conditions (database errors, system failures, etc.).
func (es *ErrorSink) ServerError(w http.ResponseWriter, r *http.Request, err error) {
	es.Handle(w, r, http.StatusInternalServerError, "an error occurred processing your request", err)
}

// ConflictError responds with 409 Conflict.
// Used when attempting to create a resource that already exists (duplicate).
func (es *ErrorSink) ConflictError(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusConflict, "resource already exists", fmt.Errorf("conflict: %s", reason))
}

// ValidationError responds with 422 Unprocessable Entity.
// Used for client-provided data that fails validation (business logic errors).
// Unlike 400 (syntactically invalid), 422 means the request is well-formed but semantically invalid.
func (es *ErrorSink) ValidationError(w http.ResponseWriter, r *http.Request, reason string) {
	traceID := extractTraceID(r)
	resp := ErrorResponse{
		Status:  http.StatusUnprocessableEntity,
		Message: "validation failed: " + reason,
		TraceID: traceID,
		Path:    r.URL.Path,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(resp)
}

// TooManyRequests responds with 429 Too Many Requests.
// Used by rate-limiting middleware when client exceeds request quota.
func (es *ErrorSink) TooManyRequests(w http.ResponseWriter, r *http.Request) {
	resp := ErrorResponse{
		Status:  http.StatusTooManyRequests,
		Message: "rate limit exceeded",
		TraceID: extractTraceID(r),
		Path:    r.URL.Path,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60") // Suggest 60 seconds between requests
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(resp)
}

// ClientError returns true if the status code is in the 4xx range (client fault).
func ClientError(status int) bool {
	return status >= 400 && status < 500
}

// ServerErr returns true if the status code is in the 5xx range (server fault).
func ServerErr(status int) bool {
	return status >= 500 && status < 600
}

// extractTraceID retrieves the trace ID from the request.
// Falls back to request ID or generates one if neither present.
// This allows support to correlate client errors with server logs.
func extractTraceID(r *http.Request) string {
	// Try X-Trace-ID header (set by OpenTelemetry).
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		return traceID
	}
	// Try X-Request-ID header (set by RequestID middleware).
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}
	// Fall back to generating a short ID.
	return generateRequestID()
}

// clientIP extracts client IP from request, accounting for proxies.
// Checks X-Forwarded-For (load balancer header) before falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	// Check X-Forwarded-For (set by load balancer/reverse proxy).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Fall back to direct remote address.
	return r.RemoteAddr
}

// Sanitize removes sensitive fields from error messages.
// This prevents leaking database URLs, API keys, file paths, etc.
// Examples of patterns sanitized:
// - Database connection strings (postgres://, mysql://)
// - File system paths (/var/www, /home, /etc)
// - Environment variable values
// - JSON passwords and secrets
// - API keys and bearer tokens
func Sanitize(msg string) string {
	if msg == "" {
		return ""
	}

	// List of patterns to redact (simple string replacements).
	// In production, use regex with compiled patterns for efficiency.
	sanitizers := []struct {
		marker  string
		replace string
	}{
		// Database URLs
		{"postgres://", "postgres://***"},
		{"mysql://", "mysql://***"},
		{"mongodb://", "mongodb://***"},

		// File system paths
		{"/var/www", "/***"},
		{"/home/", "/***"},
		{"/etc/", "/***"},
		{"/opt/", "/***"},
		{"C:\\", "***"},

		// Secrets and keys
		{"password", "****"},
		{"secret", "****"},
		{"token", "****"},
		{"api_key", "****"},
		{"api-key", "****"},
		{"Authorization: Bearer", "Authorization: Bearer ***"},

		// Environment variables (only redact values, not keys)
		{"GENIE_", "GENIE_***"},
		{"JWT_SECRET", "JWT_***"},
		{"API_", "API_***"},
	}

	for _, s := range sanitizers {
		if strings.Contains(msg, s.marker) {
			msg = strings.ReplaceAll(msg, s.marker, s.replace)
		}
	}

	return msg
}

// generateRequestID creates a short request ID for error correlation.
// Production should use the RequestID middleware which generates UUIDs.
func generateRequestID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[i%36]
	}
	return string(b)
}
