package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/consent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

func TestConsentHandler_GrantConsent(t *testing.T) {
	handler := &ConsentHandler{
		Matrix: consent.NewInMemoryConsentMatrix(),
		Log:    consent.NewInMemoryAccessLog(),
	}

	body := grantConsentRequest{
		UserID:       "agent-123",
		ResourceType: "database",
		Permissions:  "read_write",
		TTLSeconds:   86400, // 24 hours
		Reason:       "Data processing for analytics",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/consent/grant", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "admin-user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.GrantConsent(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp grantConsentResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ID == "" {
		t.Error("expected consent ID")
	}
	if resp.UserID != "agent-123" {
		t.Errorf("expected user ID agent-123, got %s", resp.UserID)
	}
	if resp.Permissions != "read_write" {
		t.Errorf("expected permissions read_write, got %s", resp.Permissions)
	}
	if resp.ExpiresAt == "" {
		t.Error("expected expires_at")
	}
}

func TestConsentHandler_GrantConsent_InvalidJSON(t *testing.T) {
	handler := &ConsentHandler{}

	req := httptest.NewRequest("POST", "/v1/consent/grant", bytes.NewReader([]byte("invalid")))
	claims := auth.Claims{Subject: "admin-user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.GrantConsent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestConsentHandler_GrantConsent_MissingUserID(t *testing.T) {
	handler := &ConsentHandler{}

	body := grantConsentRequest{
		UserID:       "",
		ResourceType: "database",
		Permissions:  "read",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/consent/grant", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "admin-user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.GrantConsent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestConsentHandler_GrantConsent_InvalidPermission(t *testing.T) {
	handler := &ConsentHandler{}

	body := grantConsentRequest{
		UserID:       "agent-123",
		ResourceType: "database",
		Permissions:  "invalid_perms",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/consent/grant", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "admin-user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.GrantConsent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestConsentHandler_GrantConsent_Unauthenticated(t *testing.T) {
	handler := &ConsentHandler{}

	body := grantConsentRequest{
		UserID:       "agent-123",
		ResourceType: "database",
		Permissions:  "read",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/consent/grant", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	handler.GrantConsent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestConsentHandler_RevokeConsent(t *testing.T) {
	handler := &ConsentHandler{
		Matrix: consent.NewInMemoryConsentMatrix(),
	}

	// First grant a consent
	handler.Matrix.Grant("agent-123", "database", consent.PermRead, 0, "test", "admin")

	// Then revoke it
	req := httptest.NewRequest("DELETE", "/v1/consent/agent-123?user_id=agent-123&resource_type=database", nil)
	claims := auth.Claims{Subject: "admin-user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.RevokeConsent(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestConsentHandler_RevokeConsent_Unauthenticated(t *testing.T) {
	handler := &ConsentHandler{}

	req := httptest.NewRequest("DELETE", "/v1/consent/agent-123?user_id=agent-123&resource_type=database", nil)
	w := httptest.NewRecorder()
	handler.RevokeConsent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestConsentHandler_ListGrants(t *testing.T) {
	matrix := consent.NewInMemoryConsentMatrix()
	handler := &ConsentHandler{
		Matrix: matrix,
	}

	// Grant some consents
	matrix.Grant("agent-123", "database", consent.PermRead, 0, "test1", "admin")
	matrix.Grant("agent-123", "api", consent.PermReadWrite, 0, "test2", "admin")

	req := httptest.NewRequest("GET", "/v1/consent/user/agent-123", nil)
	claims := auth.Claims{Subject: "user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	ctx := req.Context()
	ctx = setupChiContext(ctx, "user_id", "agent-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ListGrants(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp listGrantsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.UserID != "agent-123" {
		t.Errorf("expected user ID agent-123, got %s", resp.UserID)
	}
	if len(resp.Grants) != 2 {
		t.Errorf("expected 2 grants, got %d", len(resp.Grants))
	}
}

func TestConsentHandler_ListGrants_Unauthenticated(t *testing.T) {
	handler := &ConsentHandler{}

	req := httptest.NewRequest("GET", "/v1/consent/user/agent-123", nil)
	w := httptest.NewRecorder()
	handler.ListGrants(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestConsentHandler_GetAuditDecisions(t *testing.T) {
	log := consent.NewInMemoryAccessLog()
	handler := &ConsentHandler{
		Log: log,
	}

	// Log a decision
	log.Log(consent.AccessDecision{
		Timestamp:    time.Now(),
		UserID:       "agent-123",
		ResourceType: "database",
		Action:       consent.ActionRead,
		Result:       "allowed",
		TraceID:      "trace-123",
	})

	req := httptest.NewRequest("GET", "/v1/audit/decisions?user_id=agent-123", nil)
	claims := auth.Claims{Subject: "user"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.GetAuditDecisions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp auditDecisionsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("expected 1 decision, got %d", resp.Total)
	}
	if len(resp.Decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(resp.Decisions))
	}
}

func TestConsentHandler_GetAuditDecisions_Unauthenticated(t *testing.T) {
	handler := &ConsentHandler{}

	req := httptest.NewRequest("GET", "/v1/audit/decisions", nil)
	w := httptest.NewRecorder()
	handler.GetAuditDecisions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
