// elevation.go — HTTP surface for the Privileged Access Manager analog.
//
// Routes wired by pkg/web/router.go:
//
//   POST /v1/elevation/requests            — authenticated user files a request
//   POST /v1/elevation/requests/{id}/approve  — admin approves
//   POST /v1/elevation/requests/{id}/deny     — admin denies
//   POST /v1/elevation/requests/{id}/revoke   — admin terminates active grant
//   GET  /v1/elevation/requests            — admin lists (paginated)
//   GET  /v1/elevation/requests/{id}       — admin or subject reads one
//
// The approve/deny/revoke routes require RoleAdmin via the router-level
// gate; the Service double-checks via auth.Claims so a misrouted call
// can't bypass the role check.
//
// Subject can read their own request via Get; approver gets richer
// per-request details. List is admin-only at the router.
//
// ─── Why per-call gates AND router gates ───────────────────────────────────
//
// Defence in depth at the handler boundary. The router gate says "only
// admins reach this URL." The Service-level gate says "only admins are
// recorded as approvers." If a future refactor accidentally drops the
// router gate, the Service still refuses. If a future Service refactor
// drops the check, the router still refuses. Either layer alone is
// sufficient; both layers makes regression noisy.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth/elevation"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// Elevation is the HTTP-level wrapper around elevation.Service.
//
// Construction: cmd/api wires one Service per host, then passes it
// here. The handler holds no state of its own — every call goes
// straight through to the Service.
type Elevation struct {
	Service *elevation.Service
}

// ─── Request bodies ────────────────────────────────────────────────────────

// elevationRequestBody is the POST /v1/elevation/requests payload.
//
// TTLSeconds is the duration in whole seconds rather than a Go
// time.Duration string — easier for browser clients to construct and
// harder to mis-parse than "1h30m" strings.
type elevationRequestBody struct {
	Role       string `json:"role"`        // currently only "admin" elevatable
	Reason     string `json:"reason"`      // required, free-text justification
	TTLSeconds int    `json:"ttl_seconds"` // capped at Service.MaxDuration
}

// elevationActionBody is the shared body for approve/deny/revoke.
// Reason is required for deny/revoke (audit trail); approve treats
// reason as optional context.
type elevationActionBody struct {
	Reason string `json:"reason"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// Request handles POST /v1/elevation/requests.
//
// The authenticated user's Subject is taken from claims — clients do
// NOT pass a subject id (that would let an admin file a request on
// someone else's behalf, which we explicitly don't support). If a
// future "request on behalf of" feature is added, model it as a
// distinct endpoint with a separate admin gate.
func (h *Elevation) Request(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	var body elevationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Translate TTL seconds → time.Duration. Negative or zero is
	// rejected by the service; we don't do an early check here so the
	// service-level sentinel is the single source of truth for the
	// "what counts as a valid TTL" rule.
	ttl := time.Duration(body.TTLSeconds) * time.Second

	g, err := h.Service.Request(r.Context(), claims.Subject, auth.Role(body.Role), body.Reason, ttl)
	if err != nil {
		writeElevationError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, g)
}

// Approve handles POST /v1/elevation/requests/{id}/approve.
func (h *Elevation) Approve(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	grantID := chi.URLParam(r, "id")

	// Body is optional — approve doesn't require a reason (the request
	// itself carries the justification).
	var body elevationActionBody
	_ = json.NewDecoder(r.Body).Decode(&body) // ignore decode error; body may be empty

	if err := h.Service.Approve(r.Context(), grantID, claims); err != nil {
		writeElevationError(w, err)
		return
	}
	// Return the updated grant so the caller can see the new status
	// (Pending if N-eyes not yet satisfied; Active if it is).
	g, _ := h.Service.Get(r.Context(), grantID)
	respondJSON(w, http.StatusOK, g)
}

// Deny handles POST /v1/elevation/requests/{id}/deny.
//
// A non-empty reason is required so the audit log captures *why* the
// request was refused. The Service writes the reason into Details.
func (h *Elevation) Deny(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	grantID := chi.URLParam(r, "id")

	var body elevationActionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Reason == "" {
		http.Error(w, "reason is required for deny", http.StatusBadRequest)
		return
	}

	if err := h.Service.Deny(r.Context(), grantID, claims, body.Reason); err != nil {
		writeElevationError(w, err)
		return
	}
	g, _ := h.Service.Get(r.Context(), grantID)
	respondJSON(w, http.StatusOK, g)
}

// Revoke handles POST /v1/elevation/requests/{id}/revoke.
//
// Same reason-required pattern as Deny. Revoking is the "I changed my
// mind" or "the task is done" exit; the reason explains which.
func (h *Elevation) Revoke(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	grantID := chi.URLParam(r, "id")

	var body elevationActionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Reason == "" {
		http.Error(w, "reason is required for revoke", http.StatusBadRequest)
		return
	}

	if err := h.Service.Revoke(r.Context(), grantID, claims, body.Reason); err != nil {
		writeElevationError(w, err)
		return
	}
	g, _ := h.Service.Get(r.Context(), grantID)
	respondJSON(w, http.StatusOK, g)
}

// Get handles GET /v1/elevation/requests/{id}.
//
// Access rule: the subject of the grant or any admin can read it.
// Other users get 404 (not 403 — refusing to confirm existence is
// the right move for an admin-flavoured resource).
func (h *Elevation) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	grantID := chi.URLParam(r, "id")

	g, err := h.Service.Get(r.Context(), grantID)
	if err != nil {
		// Translate not-found to 404; other errors to 500.
		if errors.Is(err, elevation.ErrGrantNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}

	// Authorisation: admin OR subject. Anyone else gets 404 (don't
	// confirm existence to non-authorised callers).
	if !claims.HasRole(auth.RoleAdmin) && claims.Subject != g.Subject {
		http.NotFound(w, r)
		return
	}

	respondJSON(w, http.StatusOK, g)
}

// List handles GET /v1/elevation/requests — admin-only at the router.
//
// Limit defaults to 50 in the service. Pagination cursor is roadmap
// once the in-memory store is replaced with a SQL backing.
func (h *Elevation) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out := h.Service.List(r.Context(), limit)
	respondJSON(w, http.StatusOK, out)
}

// ─── Internals ─────────────────────────────────────────────────────────────

// writeElevationError maps a service-layer sentinel to the right HTTP
// status. Generic 500 for anything unrecognised — we don't want to
// leak unexpected error detail through HTTP.
func writeElevationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, elevation.ErrSubjectRequired),
		errors.Is(err, elevation.ErrReasonRequired),
		errors.Is(err, elevation.ErrRoleNotElevatable),
		errors.Is(err, elevation.ErrTTLOutOfRange),
		errors.Is(err, elevation.ErrApproverIsSubject),
		errors.Is(err, elevation.ErrDuplicateApprover):
		// Client-side mistakes — 400.
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, elevation.ErrApproverIneligible):
		// Caller lacks the role to perform the action — 403.
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, elevation.ErrGrantNotFound):
		http.NotFound(w, nil)
	case errors.Is(err, elevation.ErrGrantNotPending),
		errors.Is(err, elevation.ErrGrantNotActive):
		// Wrong state for the requested transition — 409 Conflict.
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
