package transports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
)

// stubAgent is a minimal core.Agent for transport tests. It echoes a fixed
// result and reports healthy.
type stubAgent struct {
	name string
}

func (a *stubAgent) Name() string                 { return a.name }
func (a *stubAgent) Version() string              { return "test" }
func (a *stubAgent) Health(context.Context) error { return nil }
func (a *stubAgent) Execute(_ context.Context, _ interface{}) (interface{}, error) {
	return map[string]any{"ok": true}, nil
}

// TestHandleExecute_TraceIDsAreUnique verifies the fix for the hardcoded
// getCurrentTimestamp() bug: two sequential requests that omit a TraceID must
// receive DISTINCT server-generated TraceIDs. Before the fix every TraceID was
// "tr-<agent>-1000", which collided across all requests.
func TestHandleExecute_TraceIDsAreUnique(t *testing.T) {
	srv := NewHTTPServer(&stubAgent{name: "tester"})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	traceIDs := make(map[string]struct{})
	const n = 5
	for i := 0; i < n; i++ {
		resp, err := http.Post(ts.URL+"/execute", "application/json",
			strings.NewReader(`{"user_id":"u-1","payload":{}}`))
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		var out core.AgentOutput
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			t.Fatalf("decode %d: %v", i, err)
		}
		resp.Body.Close()

		if out.TraceID == "" {
			t.Fatalf("request %d: server returned empty TraceID", i)
		}
		if !strings.HasPrefix(out.TraceID, "tr-tester-") {
			t.Fatalf("request %d: unexpected TraceID format %q", i, out.TraceID)
		}
		if _, dup := traceIDs[out.TraceID]; dup {
			t.Fatalf("request %d: duplicate TraceID %q (collision bug not fixed)", i, out.TraceID)
		}
		traceIDs[out.TraceID] = struct{}{}
	}

	if len(traceIDs) != n {
		t.Fatalf("expected %d unique TraceIDs, got %d", n, len(traceIDs))
	}
}

// TestHandleExecute_PreservesCallerTraceID verifies that a caller-supplied
// TraceID is respected and not overwritten by the generator.
func TestHandleExecute_PreservesCallerTraceID(t *testing.T) {
	srv := NewHTTPServer(&stubAgent{name: "tester"})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/execute", "application/json",
		strings.NewReader(`{"user_id":"u-1","trace_id":"caller-supplied-123","payload":{}}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var out core.AgentOutput
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.TraceID != "caller-supplied-123" {
		t.Fatalf("caller TraceID overwritten: got %q", out.TraceID)
	}
}

// TestAuth_RejectsMissingToken verifies that when a token is configured, /execute
// without the header is 401, but /health stays open for K8s probes.
func TestAuth_RejectsMissingToken(t *testing.T) {
	srv := NewHTTPServer(&stubAgent{name: "sec"}, WithToken("s3cr3t"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// /execute without token → 401
	resp, err := http.Post(ts.URL+"/execute", "application/json", strings.NewReader(`{"payload":{}}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", resp.StatusCode)
	}

	// /health without token → 200 (probes must work)
	hresp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		t.Fatalf("health without token: got %d, want 200", hresp.StatusCode)
	}
}

// TestAuth_AcceptsValidToken verifies a correct X-Agent-Token is accepted.
func TestAuth_AcceptsValidToken(t *testing.T) {
	srv := NewHTTPServer(&stubAgent{name: "sec"}, WithToken("s3cr3t"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/execute", strings.NewReader(`{"payload":{}}`))
	req.Header.Set("X-Agent-Token", "s3cr3t")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid token: got %d, want 200", resp.StatusCode)
	}
}

// TestAuth_WrongToken is rejected.
func TestAuth_WrongToken(t *testing.T) {
	srv := NewHTTPServer(&stubAgent{name: "sec"}, WithToken("right"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/execute", strings.NewReader(`{"payload":{}}`))
	req.Header.Set("X-Agent-Token", "wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token: got %d, want 401", resp.StatusCode)
	}
}

// TestMaxBodyBytes rejects an oversized request body.
func TestMaxBodyBytes(t *testing.T) {
	srv := NewHTTPServer(&stubAgent{name: "lim"}, WithMaxBodyBytes(64))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	big := `{"payload":{"x":"` + strings.Repeat("A", 500) + `"}}`
	resp, err := http.Post(ts.URL+"/execute", "application/json", strings.NewReader(big))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	// MaxBytesReader makes the decode fail → handler returns 400.
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("oversized body accepted (got 200); MaxBytesReader not enforced")
	}
}

// TestNoWildcardCORS confirms the SSE handler no longer emits Access-Control-Allow-Origin: *.
func TestNoWildcardCORS(t *testing.T) {
	srv := NewStreamHTTPServer(&stubAgent{name: "sse"})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/execute-stream", "application/json", strings.NewReader(`{"payload":{}}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatal("SSE handler still sets wildcard CORS")
	}
}
