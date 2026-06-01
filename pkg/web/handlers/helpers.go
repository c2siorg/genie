// helpers.go — Shared utilities for HTTP handlers.
// This file provides generateID for Finance Module handlers.
package handlers

import (
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
