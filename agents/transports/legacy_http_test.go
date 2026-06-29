package transports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// echoLegacy is a pkg/agent.Agent that echoes the message content back.
type echoLegacy struct{}

func (echoLegacy) ID() string             { return "echo" }
func (echoLegacy) Name() string           { return "Echo Agent" }
func (echoLegacy) Capabilities() []string { return []string{"message.handle"} }
func (echoLegacy) HandleMessage(_ context.Context, m pkgagent.Message, _ pkgagent.Environment) ([]pkgagent.Message, error) {
	return []pkgagent.Message{{From: "echo", To: m.From, Content: "echo:" + m.Content}}, nil
}

func TestLegacyHTTPServer_HandleAndInfo(t *testing.T) {
	srv := NewLegacyHTTPServer(echoLegacy{}, WithToken("tok"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// /info (with token) reports id/name/capabilities.
	req, _ := http.NewRequest("GET", ts.URL+"/info", nil)
	req.Header.Set("X-Agent-Token", "tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	var info map[string]any
	json.NewDecoder(resp.Body).Decode(&info)
	resp.Body.Close()
	if info["id"] != "echo" {
		t.Errorf("info id = %v, want echo", info["id"])
	}

	// /handle round-trips a message.
	body, _ := json.Marshal(protocol.Message{ID: "m1", From: "supervisor", Content: "hi"})
	hreq, _ := http.NewRequest("POST", ts.URL+"/handle", strings.NewReader(string(body)))
	hreq.Header.Set("X-Agent-Token", "tok")
	hreq.Header.Set("Content-Type", "application/json")
	hresp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		t.Fatalf("handle status = %d, want 200", hresp.StatusCode)
	}
	var out []protocol.Message
	json.NewDecoder(hresp.Body).Decode(&out)
	if len(out) != 1 || out[0].Content != "echo:hi" || out[0].To != "supervisor" {
		t.Fatalf("unexpected handle output: %+v", out)
	}
}

func TestLegacyHTTPServer_Health_Open_And_Auth(t *testing.T) {
	srv := NewLegacyHTTPServer(echoLegacy{}, WithToken("tok"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// /health is open without a token.
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health = %d, want 200", resp.StatusCode)
	}

	// /handle without a token is rejected.
	r, err := http.Post(ts.URL+"/handle", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("handle no token = %d, want 401", r.StatusCode)
	}
}

func TestLegacyHTTPServer_BadBody(t *testing.T) {
	srv := NewLegacyHTTPServer(echoLegacy{}) // no token
	ts := httptest.NewServer(srv)
	defer ts.Close()

	r, err := http.Post(ts.URL+"/handle", "application/json", strings.NewReader(`{not json`))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", r.StatusCode)
	}
}
