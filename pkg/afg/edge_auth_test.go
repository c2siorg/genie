package afg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
)

// restrictedType is a governance message Type that RBAC gates to advisor/admin
// only, letting these tests exercise the deny path without depending on any
// production policy YAML.
const restrictedType = "restricted"

// authTestEdge builds a *Registry with one deterministic (currency) agent
// behind a composite gate: length check + RBAC-by-message-Type gating
// restrictedType to advisor/admin. Requests whose Type defaults to the agent
// ID (CurrencyName) sail through RBAC untouched (opt-in gating — a type with
// no entry in RequiredRolesByType is allowed).
func authTestEdge(t *testing.T) (http.Handler, *auth.Issuer) {
	t.Helper()
	issuer := auth.NewIssuer([]byte("edge-auth-test-secret"), "genie-af-serve", []string{"genie-af-serve"}, time.Hour)
	gate := governance.NewComposite(
		governance.MaxContentLengthPolicy{Max: 1 << 20},
		governance.RBACPolicy{RequiredRolesByType: map[string][]string{
			restrictedType: {"advisor", "admin"},
		}},
	)
	reg := NewRegistry()
	reg.Register(AgentInfo{ID: CurrencyName, Provider: ProviderDeterministic, Risk: "low"}, NewCurrencyAgent(gate))
	return NewHandler(reg, issuer), issuer
}

func mintToken(t *testing.T, issuer *auth.Issuer, roles ...auth.Role) string {
	t.Helper()
	tok, _, err := issuer.Issue("edge-test-user", "edge-test@example.com", roles)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

const convertBody = `{"agent":"currency_converter","type":"restricted","input":"{\"amount_minor\":10000,\"from\":\"USD\",\"to\":\"INR\"}","classification":"internal"}`

// (a) No Authorization header at all -> 401, before governance or the agent
// ever run.
func TestEdgeAuth_NoAuthHeaderIs401(t *testing.T) {
	h, _ := authTestEdge(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(convertBody))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 with no Authorization header, got %d: %s", rec.Code, rec.Body.String())
	}
}

// (b) Valid JWT, but the authenticated role ("user") is not in RBAC's
// allow-list for restrictedType -> the governance gate denies -> 403.
func TestEdgeAuth_ValidJWTButRBACDeniedIs403(t *testing.T) {
	h, issuer := authTestEdge(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(convertBody))
	req.Header.Set("Authorization", "Bearer "+mintToken(t, issuer, auth.RoleUser))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for role without RBAC grant, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "denied") {
		t.Fatalf("expected denial body, got: %s", rec.Body.String())
	}
}

// (c) Valid JWT with a privileged role (advisor) satisfies RBAC -> 200, and
// identity comes from the JWT claims, not the request body (AskRequest has no
// user_id/roles fields any more).
func TestEdgeAuth_ValidJWTPrivilegedRoleIs200(t *testing.T) {
	h, issuer := authTestEdge(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(convertBody))
	req.Header.Set("Authorization", "Bearer "+mintToken(t, issuer, auth.RoleAdvisor))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for advisor role, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp AskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad resp: %v (body=%s)", err, rec.Body.String())
	}
	if !strings.Contains(resp.Output, `"amount_minor_to":830000`) {
		t.Fatalf("unexpected output: %q", resp.Output)
	}
}

// A malformed / garbage bearer token is rejected the same way as a missing one.
func TestEdgeAuth_InvalidTokenIs401(t *testing.T) {
	h, _ := authTestEdge(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(convertBody))
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for garbage token, got %d: %s", rec.Code, rec.Body.String())
	}
}
