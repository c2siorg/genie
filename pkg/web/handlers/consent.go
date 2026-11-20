// consent.go — HTTP surface for Consent Registry (A2A) module.
//
// Routes wired by pkg/web/router.go:
//
//	POST /v1/consent/grant — Grant consent (user, resource, permission, ttl)
//	DELETE /v1/consent/{consent_id} — Revoke consent
//	GET /v1/consent/user/{user_id} — List grants for user
//	GET /v1/audit/decisions — Query audit log (user, resource, since, until)
//
// All endpoints require authentication.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/consent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// ConsentHandler wraps consent registry operations.
type ConsentHandler struct {
	// Matrix stores and manages consent grants.
	Matrix consent.ConsentMatrix
	// Log records authorization decisions.
	Log consent.AccessLog
}

// ─── Request/response types ────────────────────────────────────────────────

// grantConsentRequest is the POST /v1/consent/grant body.
type grantConsentRequest struct {
	// UserID is the agent or principal being granted permission.
	UserID string `json:"user_id"`
	// ResourceType identifies the resource being accessed.
	ResourceType string `json:"resource_type"`
	// Permissions is the level of access (read, read_write, admin).
	Permissions string `json:"permissions"`
	// TTLSeconds is the time-to-live in seconds (0 = no expiry).
	TTLSeconds int `json:"ttl_seconds,omitempty"`
	// Reason explains why this consent was granted.
	Reason string `json:"reason"`
}

// grantConsentResponse wraps the created ConsentRecord.
type grantConsentResponse struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	ResourceType string `json:"resource_type"`
	Permissions  string `json:"permissions"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}

// consentRecordResponse represents a consent grant for HTTP responses.
type consentRecordResponse struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	ResourceType string `json:"resource_type"`
	Permissions  string `json:"permissions"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Reason       string `json:"reason"`
	GrantedBy    string `json:"granted_by"`
	CreatedAt    string `json:"created_at"`
	IsExpired    bool   `json:"is_expired"`
}

// listGrantsResponse wraps consent records for a user.
type listGrantsResponse struct {
	UserID string                  `json:"user_id"`
	Grants []consentRecordResponse `json:"grants"`
	Total  int                     `json:"total"`
}

// accessDecisionResponse represents an authorization decision.
type accessDecisionResponse struct {
	Timestamp    string `json:"timestamp"`
	UserID       string `json:"user_id"`
	ResourceType string `json:"resource_type"`
	Action       string `json:"action"`
	Result       string `json:"result"`
	ReasonCode   string `json:"reason_code"`
	TraceID      string `json:"trace_id"`
}

// auditDecisionsResponse wraps a set of access decisions.
type auditDecisionsResponse struct {
	Total     int                      `json:"total"`
	Decisions []accessDecisionResponse `json:"decisions"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// GrantConsent handles POST /v1/consent/grant.
// Grants a new consent record or updates an existing one.
func (h *ConsentHandler) GrantConsent(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	var body grantConsentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if body.UserID == "" || body.ResourceType == "" || body.Permissions == "" {
		http.Error(w, "user_id, resource_type, and permissions required", http.StatusBadRequest)
		return
	}

	// Validate permission level
	permLevel := consent.PermissionLevel(body.Permissions)
	switch permLevel {
	case consent.PermRead, consent.PermReadWrite, consent.PermAdmin:
		// Valid
	default:
		http.Error(w, "invalid permissions: must be read, read_write, or admin", http.StatusBadRequest)
		return
	}

	// Compute TTL
	var ttl time.Duration
	if body.TTLSeconds > 0 {
		ttl = time.Duration(body.TTLSeconds) * time.Second
	}

	// Grant the consent
	consentID, err := h.Matrix.Grant(
		body.UserID,
		body.ResourceType,
		permLevel,
		ttl,
		body.Reason,
		claims.Subject, // The authenticated user is granting this
	)
	if err != nil {
		http.Error(w, "failed to grant consent: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Compute expires_at if TTL was specified
	expiresAt := ""
	if body.TTLSeconds > 0 {
		expiresAt = time.Now().Add(ttl).Format(time.RFC3339)
	}

	respondJSON(w, http.StatusCreated, grantConsentResponse{
		ID:           consentID,
		UserID:       body.UserID,
		ResourceType: body.ResourceType,
		Permissions:  body.Permissions,
		ExpiresAt:    expiresAt,
		Reason:       body.Reason,
		CreatedAt:    time.Now().Format(time.RFC3339),
	})
}

// RevokeConsent handles DELETE /v1/consent/{consent_id}.
// Revokes a consent grant by user_id and resource_type (extracted from path).
func (h *ConsentHandler) RevokeConsent(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	// In practice, the consent ID would be looked up to get user_id and resource_type
	// For now, we require query params
	userID := r.URL.Query().Get("user_id")
	resourceType := r.URL.Query().Get("resource_type")

	if userID == "" || resourceType == "" {
		http.Error(w, "user_id and resource_type query parameters required", http.StatusBadRequest)
		return
	}

	// Revoke the consent
	err := h.Matrix.Revoke(userID, resourceType)
	if err != nil {
		http.Error(w, "failed to revoke consent: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListGrants handles GET /v1/consent/user/{user_id}.
// Lists all consent grants for a user.
func (h *ConsentHandler) ListGrants(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	// List grants for the user
	records, err := h.Matrix.List(userID)
	if err != nil {
		http.Error(w, "failed to list grants: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	grants := make([]consentRecordResponse, len(records))
	for i, cr := range records {
		expiresAt := ""
		if !cr.ExpiresAt.IsZero() {
			expiresAt = cr.ExpiresAt.Format(time.RFC3339)
		}
		grants[i] = consentRecordResponse{
			ID:           cr.ID,
			UserID:       cr.UserID,
			ResourceType: cr.ResourceType,
			Permissions:  string(cr.Permissions),
			ExpiresAt:    expiresAt,
			Reason:       cr.Reason,
			GrantedBy:    cr.GrantedBy,
			CreatedAt:    cr.CreatedAt.Format(time.RFC3339),
			IsExpired:    cr.IsExpired(),
		}
	}

	respondJSON(w, http.StatusOK, listGrantsResponse{
		UserID: userID,
		Grants: grants,
		Total:  len(grants),
	})
}

// GetAuditDecisions handles GET /v1/audit/decisions.
// Queries the audit log for access decisions.
func (h *ConsentHandler) GetAuditDecisions(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	userID := r.URL.Query().Get("user_id")

	var since, until time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = t
		}
	}

	// Query the audit log
	var decisions []consent.AccessDecision
	var err error
	if h.Log != nil {
		decisions, err = h.Log.Query(userID, since, until)
		if err != nil {
			http.Error(w, "failed to query audit log: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Convert to response format
	responseDecisions := make([]accessDecisionResponse, len(decisions))
	for i, d := range decisions {
		responseDecisions[i] = accessDecisionResponse{
			Timestamp:    d.Timestamp.Format(time.RFC3339),
			UserID:       d.UserID,
			ResourceType: d.ResourceType,
			Action:       string(d.Action),
			Result:       d.Result,
			ReasonCode:   d.ReasonCode,
			TraceID:      d.TraceID,
		}
	}

	respondJSON(w, http.StatusOK, auditDecisionsResponse{
		Total:     len(responseDecisions),
		Decisions: responseDecisions,
	})
}
