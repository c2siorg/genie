package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/opa"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// OPAHandler exposes OPA policy introspection and ad-hoc evaluation endpoints.
// All routes require RoleAdmin (enforced at the router level).
type OPAHandler struct {
	Engine *opa.Engine
}

// -------------------------------------------------------------------
// GET /v1/governance/opa/config
// -------------------------------------------------------------------

// GetConfig returns the PolicyConfig the OPA engine is running with.
// This lets operators verify that the YAML-driven config was loaded correctly
// without having to read Rego source.
func (h *OPAHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.Engine.Config())
}

// -------------------------------------------------------------------
// POST /v1/governance/opa/evaluate
// -------------------------------------------------------------------

// evalRequest is the body for the ad-hoc evaluation endpoint.
type evalRequest struct {
	// Message is the message to evaluate. All fields are optional; missing
	// fields are left as zero values so partial inputs can be tested.
	Message evalMessage `json:"message"`
}

type evalMessage struct {
	ID       string         `json:"id"`
	From     string         `json:"from"`
	To       string         `json:"to"`
	Role     string         `json:"role"`
	Type     string         `json:"type"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata"`
}

// evalResponse is the body returned by the ad-hoc evaluation endpoint.
type evalResponse struct {
	Decision    string   `json:"decision"`
	Reason      string   `json:"reason,omitempty"`
	DenyReasons []string `json:"deny_reasons,omitempty"`
	CheckedByID string   `json:"checked_by_id"`
}

// Evaluate handles POST /v1/governance/opa/evaluate.
// It accepts a partial Message, evaluates it against the running OPA policy,
// and returns the decision with all denial reasons.
// Useful for testing policy changes before deploying, auditing edge cases,
// and building admin tooling.
func (h *OPAHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	var req evalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	msg := protocol.NewMessage(
		req.Message.From,
		req.Message.To,
		protocol.MessageRole(req.Message.Role),
		req.Message.Type,
		req.Message.Content,
		req.Message.Metadata,
	)
	if req.Message.ID != "" {
		msg.ID = req.Message.ID
	}

	result, err := h.Engine.Evaluate(r.Context(), msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var denyReasons []string
	if result.Decision == governance.DecisionDeny {
		// Split the reason back into individual reasons for the response.
		denyReasons = splitReasons(result.Reason)
	}

	respondJSON(w, http.StatusOK, evalResponse{
		Decision:    string(result.Decision),
		Reason:      result.Reason,
		DenyReasons: denyReasons,
		CheckedByID: result.CheckedByID,
	})
}

// -------------------------------------------------------------------
// POST /v1/governance/opa/check-http
// -------------------------------------------------------------------

// HTTPCheckRequest is the body for the HTTP authz check endpoint.
type HTTPCheckRequest struct {
	Method  string            `json:"method"`
	Path    []string          `json:"path"`
	Headers map[string]string `json:"headers"`
	User    opa.HTTPAuthzUser `json:"user"`
}

// CheckHTTP handles POST /v1/governance/opa/check-http.
// Evaluates the data.genie.http_authz policy against a synthetic request.
// Useful for verifying HTTP-level policy rules without making a real HTTP call.
func (h *OPAHandler) CheckHTTP(w http.ResponseWriter, r *http.Request) {
	var req HTTPCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	input := opa.HTTPAuthzInput{
		Method:  req.Method,
		Path:    req.Path,
		Headers: req.Headers,
		User:    req.User,
	}

	allowed, reason, err := h.Engine.EvaluateHTTP(r.Context(), input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"allow":  allowed,
		"reason": reason,
	})
}

// -------------------------------------------------------------------
// GET /v1/governance/opa/health
// -------------------------------------------------------------------

// Health handles GET /v1/governance/opa/health.
// Evaluates a trivial allow-all message to verify the OPA engine is up.
func (h *OPAHandler) Health(w http.ResponseWriter, r *http.Request) {
	// Use a system-role message so tenant checks don't fire.
	probe := protocol.NewMessage(
		"opa-health-probe", "supervisor",
		protocol.RoleSystem,
		"health_check", "ping",
		map[string]any{
			"classification": "internal",
			"user_roles":     []string{"admin"},
			"tenant_id":      "probe",
			"region":         "in",
		},
	)

	_, err := h.Engine.Evaluate(r.Context(), probe)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "degraded",
			"error":  err.Error(),
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

// splitReasons splits the semicolon-joined reason string back into a slice.
// This keeps the handler self-contained without needing access to OPA internals.
func splitReasons(reason string) []string {
	if reason == "" {
		return nil
	}
	// Split on "; " — the separator used by Engine.Evaluate.
	var out []string
	start := 0
	for i := 0; i+2 <= len(reason); i++ {
		if reason[i] == ';' && reason[i+1] == ' ' {
			out = append(out, reason[start:i])
			start = i + 2
		}
	}
	out = append(out, reason[start:])
	return out
}

// respondOPA satisfies the governance.Policy contract for testing.
// Exported for use in cmd/api wiring.
type OPAEvalFunc func(ctx context.Context, msg protocol.Message) (governance.PolicyResult, error)
