// Package web is Genie's HTTP layer. It builds a chi-based router with
// JWT + RBAC middleware and routes that translate REST calls onto the
// multi-agent bus.
//
// Boundaries:
//   - This package depends on auth, governance, comm, registry, crypto,
//     storage. It does NOT depend on any specific agent — the agent set is
//     injected via the bus.
//   - HTTP handlers do not reach into protocol.Message; they go through the
//     small "Publish + wait" helper here so retries/correlation stay in one
//     place.
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// respondJSON writes v as JSON with the given status code. Failures are
// logged via the request context's logger if present.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// respondError writes a JSON error envelope.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// decodeJSON decodes a request body into out, returning a 400-shaped error
// to keep handlers terse.
func decodeJSON(r *http.Request, out any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

// withTimeout derives a context with the given timeout, falling back to the
// request's own deadline.
func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}
