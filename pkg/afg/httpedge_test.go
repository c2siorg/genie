package afg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func testEdge() http.Handler {
	// Board-like gate: classification ceiling denies secret; everything else allows.
	gate := governance.NewComposite(
		governance.MaxContentLengthPolicy{Max: 1 << 20},
		governance.ClassificationPolicy{DefaultCeiling: protocol.ClassInternal},
	)
	reg := NewRegistry()
	reg.Register(AgentInfo{ID: CurrencyName, Provider: ProviderDeterministic, Risk: "low"}, NewCurrencyAgent(gate))
	return NewHandler(reg)
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(body))
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

func TestHTTPEdge_Inventory(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ai-inventory", nil)
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
