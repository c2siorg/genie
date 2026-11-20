package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/hitl"
	"github.com/go-chi/chi/v5"
)

// HITLHandler exposes the async approval queue over HTTP.
//
// Routes (all admin-gated at the router level):
//
//	GET  /v1/hitl/approvals           — list pending approvals
//	GET  /v1/hitl/approvals/{id}      — get one pending approval
//	POST /v1/hitl/approvals/{id}/approve — approve
//	POST /v1/hitl/approvals/{id}/deny    — deny
type HITLHandler struct {
	Approver *hitl.AsyncApprover
}

// ─── GET /v1/hitl/approvals ───────────────────────────────────────────────

// List returns all pending approval requests.
func (h *HITLHandler) List(w http.ResponseWriter, r *http.Request) {
	pending := h.Approver.Store().List()
	if pending == nil {
		pending = []hitl.ApprovalRequest{}
	}
	respondJSON(w, http.StatusOK, pending)
}

// ─── GET /v1/hitl/approvals/{id} ─────────────────────────────────────────

// Get returns a single pending approval request by ID.
func (h *HITLHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	req, ok := h.Approver.Store().Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, req)
}

// ─── POST /v1/hitl/approvals/{id}/approve ────────────────────────────────

type decideBody struct {
	Reason    string `json:"reason"`
	DecidedBy string `json:"decided_by"`
}

// Approve delivers an approval decision for the given request.
func (h *HITLHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, true)
}

// Deny delivers a denial decision for the given request.
func (h *HITLHandler) Deny(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, false)
}

func (h *HITLHandler) decide(w http.ResponseWriter, r *http.Request, approved bool) {
	id := chi.URLParam(r, "id")

	var body decideBody
	// body is optional — ignore decode errors
	_ = json.NewDecoder(r.Body).Decode(&body)

	decision := hitl.ApprovalDecision{
		RequestID: id,
		Approved:  approved,
		Reason:    body.Reason,
		DecidedBy: body.DecidedBy,
	}

	if err := h.Approver.Store().Decide(decision); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	status := "approved"
	if !approved {
		status = "denied"
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"id":     id,
		"status": status,
	})
}
