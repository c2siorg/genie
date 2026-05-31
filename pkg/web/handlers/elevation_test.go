// elevation_test.go — HTTP contract tests for the elevation handler.
//
// What's pinned:
//   - POST /v1/elevation/requests with valid body → 201, returns the grant
//   - POST with empty reason → 400 (sentinel mapping check)
//   - POST with TTL > MaxDuration → 400
//   - POST without authentication → 401
//   - Approve happy path → 200, status becomes Active
//   - Approve by non-admin (route-bypass simulation) → 403 from service
//   - Deny without reason → 400 (handler-level validation)
//   - Revoke without reason → 400 (handler-level validation)
//   - Get by subject → 200; by stranger → 404 (no existence confirmation)
//
// These tests exercise the handlers directly via httptest.NewRecorder.
// The router-level RequireRole gate is tested separately under pkg/web —
// here we focus on the handler's input validation and the
// service-error-to-HTTP-status mapping in writeElevationError.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth/elevation"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// newElevationHandler builds a fresh handler against an in-memory
// audit log + service. Returns the handler and the service so tests
// can pre-seed grants via direct service calls.
func newElevationHandler(t *testing.T) (*Elevation, *elevation.Service) {
	t.Helper()
	audit := compliance.NewInMemoryAuditLog()
	svc := elevation.New(audit)
	return &Elevation{Service: svc}, svc
}

// ctxWithClaims injects authenticated claims into the request context
// the way mid.Auth would in production. The handler reads claims via
// mid.ClaimsFrom; this helper avoids spinning up the real JWT middleware.
func ctxWithClaims(c auth.Claims) context.Context {
	return mid.WithClaims(context.Background(), c)
}

// makeReq wraps the boilerplate of building a JSON httptest request
// with claims context. Returns (req, recorder).
func makeReq(t *testing.T, method, path string, body any, claims auth.Claims) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	var rdr *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewBuffer(b)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req = req.WithContext(ctxWithClaims(claims))
	return req, httptest.NewRecorder()
}

// withURLParam attaches a chi route param to the request — needed for
// handlers that call chi.URLParam.
func withURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ─── POST /v1/elevation/requests ──────────────────────────────────────────

func TestRequest_HappyPath(t *testing.T) {
	h, _ := newElevationHandler(t)
	req, rec := makeReq(t, "POST", "/v1/elevation/requests", elevationRequestBody{
		Role: "admin", Reason: "investigate ticket #1234", TTLSeconds: 1800,
	}, auth.Claims{Subject: "alice", Roles: []auth.Role{auth.RoleUser}})
	h.Request(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201; got %d (%s)", rec.Code, rec.Body.String())
	}
	var g elevation.Grant
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if g.Subject != "alice" {
		t.Errorf("subject should be alice; got %s", g.Subject)
	}
	if g.Status != elevation.StatusPending {
		t.Errorf("status should be Pending; got %s", g.Status)
	}
}

// TestRequest_EmptyReason — handler should pass the empty-reason error
// through to writeElevationError which maps it to 400.
func TestRequest_EmptyReason(t *testing.T) {
	h, _ := newElevationHandler(t)
	req, rec := makeReq(t, "POST", "/v1/elevation/requests", elevationRequestBody{
		Role: "admin", Reason: "", TTLSeconds: 60,
	}, auth.Claims{Subject: "alice", Roles: []auth.Role{auth.RoleUser}})
	h.Request(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty reason; got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestRequest_TTLAboveMax — sentinel maps to 400.
func TestRequest_TTLAboveMax(t *testing.T) {
	h, _ := newElevationHandler(t)
	// Default MaxDuration is 4h = 14400s; ask for 5h.
	req, rec := makeReq(t, "POST", "/v1/elevation/requests", elevationRequestBody{
		Role: "admin", Reason: "x", TTLSeconds: 5 * 60 * 60,
	}, auth.Claims{Subject: "alice", Roles: []auth.Role{auth.RoleUser}})
	h.Request(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for TTL > MaxDuration; got %d", rec.Code)
	}
}

// TestRequest_Unauthenticated — no claims in context → 401.
func TestRequest_Unauthenticated(t *testing.T) {
	h, _ := newElevationHandler(t)
	// Build the req WITHOUT injecting claims.
	body, _ := json.Marshal(elevationRequestBody{Role: "admin", Reason: "x", TTLSeconds: 60})
	req := httptest.NewRequest("POST", "/v1/elevation/requests", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Request(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401; got %d", rec.Code)
	}
}

// ─── POST .../approve ─────────────────────────────────────────────────────

func TestApprove_HappyPath(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "POST", "/v1/elevation/requests/"+g.ID+"/approve", nil,
		auth.Claims{Subject: "bob", Roles: []auth.Role{auth.RoleAdmin}})
	req = withURLParam(req, "id", g.ID)
	h.Approve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d (%s)", rec.Code, rec.Body.String())
	}
	var g2 elevation.Grant
	_ = json.Unmarshal(rec.Body.Bytes(), &g2)
	if g2.Status != elevation.StatusActive {
		t.Errorf("status should be Active; got %s", g2.Status)
	}
}

// TestApprove_NonAdmin — the service sentinel maps to 403. The router
// gate would normally reject this too; the test verifies the service-
// level defence works in isolation.
func TestApprove_NonAdmin(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "POST", "/v1/elevation/requests/"+g.ID+"/approve", nil,
		auth.Claims{Subject: "eve", Roles: []auth.Role{auth.RoleUser}})
	req = withURLParam(req, "id", g.ID)
	h.Approve(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403; got %d (%s)", rec.Code, rec.Body.String())
	}
}

// ─── POST .../deny ────────────────────────────────────────────────────────

// TestDeny_RequiresReason — handler-level check, fires before the service.
func TestDeny_RequiresReason(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "POST", "/v1/elevation/requests/"+g.ID+"/deny", elevationActionBody{Reason: ""},
		auth.Claims{Subject: "bob", Roles: []auth.Role{auth.RoleAdmin}})
	req = withURLParam(req, "id", g.ID)
	h.Deny(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400; got %d", rec.Code)
	}
}

func TestDeny_HappyPath(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "POST", "/v1/elevation/requests/"+g.ID+"/deny", elevationActionBody{Reason: "insufficient justification"},
		auth.Claims{Subject: "bob", Roles: []auth.Role{auth.RoleAdmin}})
	req = withURLParam(req, "id", g.ID)
	h.Deny(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d (%s)", rec.Code, rec.Body.String())
	}
	var got elevation.Grant
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Status != elevation.StatusDenied {
		t.Errorf("status should be Denied; got %s", got.Status)
	}
}

// ─── POST .../revoke ──────────────────────────────────────────────────────

func TestRevoke_RequiresReason(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	_ = svc.Approve(context.Background(), g.ID, auth.Claims{Subject: "bob", Roles: []auth.Role{auth.RoleAdmin}})

	req, rec := makeReq(t, "POST", "/v1/elevation/requests/"+g.ID+"/revoke", elevationActionBody{Reason: ""},
		auth.Claims{Subject: "carol", Roles: []auth.Role{auth.RoleAdmin}})
	req = withURLParam(req, "id", g.ID)
	h.Revoke(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400; got %d", rec.Code)
	}
}

// ─── GET /v1/elevation/requests/{id} ──────────────────────────────────────

func TestGet_SubjectCanRead(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "GET", "/v1/elevation/requests/"+g.ID, nil,
		auth.Claims{Subject: "alice", Roles: []auth.Role{auth.RoleUser}})
	req = withURLParam(req, "id", g.ID)
	h.Get(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("subject should be able to read own grant; got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestGet_AdminCanRead — admin reads any grant.
func TestGet_AdminCanRead(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "GET", "/v1/elevation/requests/"+g.ID, nil,
		auth.Claims{Subject: "bob", Roles: []auth.Role{auth.RoleAdmin}})
	req = withURLParam(req, "id", g.ID)
	h.Get(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("admin should be able to read any grant; got %d", rec.Code)
	}
}

// TestGet_StrangerGets404 — a non-admin who's not the subject must get
// 404, not 403. Refusing to confirm existence is the right answer for
// an admin-flavoured resource.
func TestGet_StrangerGets404(t *testing.T) {
	h, svc := newElevationHandler(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)

	req, rec := makeReq(t, "GET", "/v1/elevation/requests/"+g.ID, nil,
		auth.Claims{Subject: "eve", Roles: []auth.Role{auth.RoleUser}})
	req = withURLParam(req, "id", g.ID)
	h.Get(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("stranger should get 404 (not 403); got %d", rec.Code)
	}
	// Body should NOT include the subject id — confirms we're not
	// leaking existence through the response body.
	if strings.Contains(rec.Body.String(), "alice") {
		t.Errorf("404 body should not reveal subject id; got %s", rec.Body.String())
	}
}
