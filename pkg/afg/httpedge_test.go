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
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

var testIssuer = auth.NewIssuer([]byte("httpedge-test-secret"), "genie-af-serve", []string{"genie-af-serve"}, time.Hour)

func testToken(t *testing.T, roles ...auth.Role) string {
	t.Helper()
	tok, _, err := testIssuer.Issue("test-user", "test@example.com", roles)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

func testEdge() http.Handler {
	// Board-like gate: classification ceiling denies secret; everything else allows.
	gate := governance.NewComposite(
		governance.MaxContentLengthPolicy{Max: 1 << 20},
		governance.ClassificationPolicy{DefaultCeiling: protocol.ClassInternal},
	)
	reg := NewRegistry()
	reg.Register(AgentInfo{ID: CurrencyName, Provider: ProviderDeterministic, Risk: "low"}, NewCurrencyAgent(gate))
	return NewHandler(reg, testIssuer)
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken(t, auth.RoleUser))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHTTPEdge_AskAllow(t *testing.T) {
	rec := post(t, testEdge(), `{"agent":"currency_converter","input":"{\"amount_minor\":10000,\"from\":\"USD\",\"to\":\"INR\"}","classification":"public"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp AskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad resp: %v", err)
	}
	if !strings.Contains(resp.Output, `"amount_minor_to":830000`) {
		t.Fatalf("unexpected output: %q", resp.Output)
	}
}

func TestHTTPEdge_AskDeniedIs403(t *testing.T) {
	rec := post(t, testEdge(), `{"agent":"currency_converter","input":"{\"amount_minor\":100,\"from\":\"USD\",\"to\":\"INR\"}","classification":"secret"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 on secret classification, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "denied") {
		t.Fatalf("expected denial body, got: %s", rec.Body.String())
	}
}

func TestHTTPEdge_AskNoAuthIs401(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(`{"agent":"currency_converter","input":"{}","classification":"public"}`))
	rec := httptest.NewRecorder()
	testEdge().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 with no Authorization header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPEdge_Inventory(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ai-inventory", nil)
	req.Header.Set("Authorization", "Bearer "+testToken(t, auth.RoleUser))
	rec := httptest.NewRecorder()
	testEdge().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var inv []AgentInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil || len(inv) == 0 {
		t.Fatalf("bad inventory: %v body=%s", err, rec.Body.String())
	}
}

func TestHTTPEdge_Healthz_Public(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		testEdge().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: want 200 with no auth, got %d", path, rec.Code)
		}
	}
}
