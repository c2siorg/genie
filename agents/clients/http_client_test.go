package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sseTestServer emits a fixed number of SSE "data:" events in the same wire
// format the real StreamHTTPServer uses ("data: <json>\n\n"), so we exercise
// the client's sseScanner against a realistic stream.
func sseTestServer(t *testing.T, events int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}
		for i := 0; i < events; i++ {
			// AgentOutput-shaped event so the client can json.Unmarshal it.
			fmt.Fprintf(w, "data: {\"agent_name\":\"streamer\",\"status\":\"success\",\"trace_id\":\"t-%d\"}\n\n", i)
			flusher.Flush()
		}
	}))
}

// TestExecuteStream_DeliversAllEvents is the regression test for the
// sseScanner.Scan()-always-false bug. Before the fix, the client received zero
// events because the scanner reported no data. We assert all 3 emitted events
// arrive on the channel.
func TestExecuteStream_DeliversAllEvents(t *testing.T) {
	const want = 3
	ts := sseTestServer(t, want)
	defer ts.Close()

	c := NewHTTPClient(ts.URL, "streamer")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := c.ExecuteStream(ctx, map[string]any{"q": "hi"})
	if err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}

	var got int
	for ev := range ch {
		if ev.Status != "success" {
			t.Fatalf("event %d unexpected status %q", got, ev.Status)
		}
		got++
	}
	if got != want {
		t.Fatalf("received %d events, want %d (streaming bug not fixed)", got, want)
	}
}

// TestSSEScanner_LineByLine unit-tests the scanner directly against a raw SSE
// body to confirm it yields each non-empty line.
func TestSSEScanner_LineByLine(t *testing.T) {
	body := "data: one\n\ndata: two\n\ndata: three\n\n"
	sc := newSSEScanner(strings.NewReader(body))

	var lines []string
	for sc.Scan() {
		if b := sc.Bytes(); len(b) > 0 {
			lines = append(lines, string(b))
		}
	}
	if len(lines) != 3 {
		t.Fatalf("got %d data lines, want 3: %v", len(lines), lines)
	}
}

// TestClient_InjectsToken verifies the client sends X-Agent-Token when configured.
func TestClient_InjectsToken(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Agent-Token")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"agent_name":"t","status":"success","result":{"ok":true}}`)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "t", WithClientToken("hunter2"))
	if _, err := c.Execute(context.Background(), map[string]any{}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotToken != "hunter2" {
		t.Fatalf("server saw token %q, want hunter2", gotToken)
	}
}

// TestClient_CircuitOpensAfterFailures verifies the breaker trips and then fails
// fast (the failing upstream stops being called once the breaker is open).
func TestClient_CircuitOpensAfterFailures(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// threshold 3, long cooldown so it stays open during the test.
	c := NewHTTPClient(srv.URL, "t", WithCircuit(3, time.Hour))
	ctx := context.Background()

	// 3 failures trip the breaker.
	for i := 0; i < 3; i++ {
		if _, err := c.Execute(ctx, map[string]any{}); err == nil {
			t.Fatalf("call %d: expected failure", i)
		}
	}
	callsAtTrip := calls

	// Next call should fail fast WITHOUT hitting the server.
	_, err := c.Execute(ctx, map[string]any{})
	if err == nil {
		t.Fatal("expected circuit-open error")
	}
	if calls != callsAtTrip {
		t.Fatalf("server was called while breaker open (calls %d -> %d); should fail fast", callsAtTrip, calls)
	}
}

// TestClient_Health covers the Health path (200 and non-200).
func TestClient_Health(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()
	if err := NewHTTPClient(healthy.URL, "t").Health(context.Background()); err != nil {
		t.Fatalf("healthy: %v", err)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()
	if err := NewHTTPClient(down.URL, "t").Health(context.Background()); err == nil {
		t.Fatal("expected health error on 503")
	}
}

// TestClient_Version covers Version/getInfo (success + fallback to "unknown").
func TestClient_Version(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"t","version":"9.9.9"}`)
	}))
	defer srv.Close()
	if v := NewHTTPClient(srv.URL, "t").Version(); v != "9.9.9" {
		t.Fatalf("version = %q, want 9.9.9", v)
	}
	// Unreachable host → "unknown".
	if v := NewHTTPClient("http://127.0.0.1:0", "t").Version(); v != "unknown" {
		t.Fatalf("version = %q, want unknown", v)
	}
}
