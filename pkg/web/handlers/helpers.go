// helpers.go — Shared utilities for HTTP handlers.
// This file provides generateID, respondJSON, and respondError for all handlers.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// generateID creates a prefixed unique identifier.
// Examples: "sreq-550e8400-e29b-41d4-a716-446655440000"
func generateID(prefix string) string {
	id := uuid.New().String()
	if prefix != "" {
		return prefix + "-" + id
	}
	return id
}

// respondJSON writes a JSON response with the given status code.
//
// Sets Content-Type header and writes the response body as JSON.
// Used throughout handlers package to return structured responses.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// respondError writes a generic error response in JSON format.
//
// For security, error messages should be generic to avoid leaking information.
// Detailed error reasons should be logged server-side, not returned to clients.
//
// Example:
//
//	respondError(w, http.StatusBadRequest, "invalid request format")
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
