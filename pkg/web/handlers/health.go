package handlers

import (
	"net/http"
)

// Health is the liveness/readiness pair.
type Health struct {
	Ready func() error // optional readiness probe
}

func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Health) Readiness(w http.ResponseWriter, r *http.Request) {
	if h.Ready != nil {
		if err := h.Ready(); err != nil {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unready", "error": err.Error()})
			return
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
