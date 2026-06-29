package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/transports"
	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// fakeLegacy is a pkg/agent.Agent used to exercise the legacy wire path. It
// echoes back a message addressed to the original sender, so the test can verify
// routing fields (From/To/Content) survive the round-trip in both directions.
type fakeLegacy struct{ id string }

func (f fakeLegacy) ID() string             { return f.id }
func (f fakeLegacy) Name() string           { return f.id }
func (f fakeLegacy) Capabilities() []string { return []string{"message.handle"} }
func (f fakeLegacy) HandleMessage(_ context.Context, msg pkgagent.Message, _ pkgagent.Environment) ([]pkgagent.Message, error) {
	return []pkgagent.Message{{
		From:    f.id,
		To:      msg.From, // reply to whoever sent it (preserves sub-orchestration routing)
		Type:    "reply",
		Content: "echo:" + msg.Content,
	}}, nil
}

func TestHTTPRegistryAgent_RoundTrip(t *testing.T) {
	srv := transports.NewLegacyHTTPServer(fakeLegacy{"analyzer"}, transports.WithToken("tok"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	proxy := NewHTTPRegistryAgent(
		AgentDef{ID: "analyzer", Name: "analyzer", Capabilities: []string{"message.handle"}},
		ts.URL, "tok")

	if proxy.ID() != "analyzer" || proxy.Name() != "analyzer" {
		t.Fatalf("identity wrong: id=%q name=%q", proxy.ID(), proxy.Name())
	}
	if len(proxy.Capabilities()) != 1 || proxy.Capabilities()[0] != "message.handle" {
		t.Fatalf("capabilities wrong: %v", proxy.Capabilities())
	}

	// supervisor sends a message to analyzer; analyzer replies to supervisor.
	in := protocol.Message{ID: "m1", From: "supervisor", To: "analyzer", Content: "hello"}
	out, err := proxy.HandleMessage(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d messages, want 1", len(out))
	}
	if out[0].Content != "echo:hello" {
		t.Errorf("content not round-tripped: %q", out[0].Content)
	}
	if out[0].To != "supervisor" {
		t.Errorf("reply routing lost: To=%q want supervisor", out[0].To)
	}
}

func TestHTTPRegistryAgent_AuthEnforced(t *testing.T) {
	srv := transports.NewLegacyHTTPServer(fakeLegacy{"analyzer"}, transports.WithToken("right"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	proxy := NewHTTPRegistryAgent(AgentDef{ID: "analyzer", Name: "analyzer"}, ts.URL, "wrong")
	_, err := proxy.HandleMessage(context.Background(), protocol.Message{From: "x", Content: "hi"}, nil)
	if err == nil {
		t.Fatal("expected auth failure with wrong token")
	}
}

func TestHTTPRegistryAgent_CircuitOpens(t *testing.T) {
	var calls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	proxy := NewHTTPRegistryAgent(AgentDef{ID: "analyzer", Name: "analyzer"}, ts.URL, "")
	ctx := context.Background()
	// default threshold is 5 consecutive failures.
	for i := 0; i < 5; i++ {
		if _, err := proxy.HandleMessage(ctx, protocol.Message{Content: "x"}, nil); err == nil {
			t.Fatalf("call %d: expected failure", i)
		}
	}
	callsAtTrip := calls
	// Breaker now open → next call fails fast without hitting the server.
	if _, err := proxy.HandleMessage(ctx, protocol.Message{Content: "x"}, nil); err == nil {
		t.Fatal("expected circuit-open error")
	}
	if calls != callsAtTrip {
		t.Fatalf("server called while breaker open (%d->%d)", callsAtTrip, calls)
	}
}

func TestAllLegacyAgents_Integrity(t *testing.T) {
	if len(AllLegacyAgents) != 31 {
		t.Fatalf("expected 31 HTTP-migratable agents, got %d", len(AllLegacyAgents))
	}
	seen := map[string]bool{}
	for _, d := range AllLegacyAgents {
		if d.ID == "" || d.Name == "" {
			t.Errorf("agent has empty id/name: %+v", d)
		}
		if len(d.Capabilities) == 0 {
			t.Errorf("agent %s has no capabilities", d.ID)
		}
		if seen[d.ID] {
			t.Errorf("duplicate agent id %q", d.ID)
		}
		seen[d.ID] = true
		if InProcessOnly[d.ID] {
			t.Errorf("in-process-only agent %q must not be in the HTTP list", d.ID)
		}
	}
	// Ring 0 wildcard sanity.
	for _, d := range AllLegacyAgents {
		if d.ID == "supervisor" && (len(d.Capabilities) != 1 || d.Capabilities[0] != "*") {
			t.Errorf("supervisor (ring 0) must have [\"*\"], got %v", d.Capabilities)
		}
	}
}

func TestHTTPRegistryAgent_Non200IsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()
	proxy := NewHTTPRegistryAgent(AgentDef{ID: "a", Name: "a"}, ts.URL, "")
	if _, err := proxy.HandleMessage(context.Background(), protocol.Message{Content: "x"}, nil); err == nil {
		t.Fatal("expected error on 502")
	}
}

func TestHTTPRegistryAgent_BadJSONIsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{not an array`))
	}))
	defer ts.Close()
	proxy := NewHTTPRegistryAgent(AgentDef{ID: "a", Name: "a"}, ts.URL, "")
	if _, err := proxy.HandleMessage(context.Background(), protocol.Message{Content: "x"}, nil); err == nil {
		t.Fatal("expected decode error on malformed body")
	}
}
