package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/synth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Feedback collects user thumbs-up / -down / edits per report. Recorded
// signals feed the preference-data pipeline used for RLAIF / DPO later.
type Feedback struct {
	Store synth.FeedbackStore
}

// Submit POSTs a feedback entry. UserID is pinned from the JWT.
func (h *Feedback) Submit(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var in synth.Feedback
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	in.UserID = claims.Subject
	out, err := h.Store.Record(r.Context(), in)
	if err != nil {
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, out)
}
