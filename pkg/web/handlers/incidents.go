package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/incidents"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Incidents exposes the AI Incident Reporting form (Annexure VI) as HTTP
// endpoints. Reads list incidents (admin only); writes create one.
type Incidents struct {
	Store incidents.Store
}

// Create implements POST /v1/incidents — matches the Annexure VI form fields.
func (h *Incidents) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var in incidents.Incident
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if in.ActorID == "" {
		in.ActorID = claims.Subject
	}
	created, err := h.Store.Create(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

// List returns the most recent incidents. Admin only.
func (h *Incidents) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Store.List(r.Context(), limit)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, out)
}
