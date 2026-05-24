package deep_research

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestOfflineBriefDefaultCorpora(t *testing.T) {
	b := New(nil, "", nil).Research(context.Background(), Request{Question: "STR reporting timeline?"})
	if b.Mode != "offline" {
		t.Errorf("expected offline mode without LLM, got %s", b.Mode)
	}
	if len(b.Citations) != 3 {
		t.Errorf("expected 3 default-corpus citations, got %d", len(b.Citations))
	}
	if !strings.Contains(b.Summary, "rbi") {
		t.Errorf("expected RBI in summary, got %q", b.Summary)
	}
}

func TestOfflineBriefFilteredCorpora(t *testing.T) {
	b := New(nil, "", nil).Research(context.Background(),
		Request{Question: "AA flow?", Sources: []string{"sahamati"}})
	if len(b.Citations) != 1 {
		t.Errorf("expected 1 citation when corpus filtered, got %d", len(b.Citations))
	}
	if b.Citations[0].Title != "Sahamati AA specs" {
		t.Errorf("unexpected citation title: %s", b.Citations[0].Title)
	}
}

func TestHandleMessageDispatches(t *testing.T) {
	body, _ := json.Marshal(Request{Question: "STR timeline?"})
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New(nil, "", nil).HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected single dispatch of %s; got %+v", TypeOut, out)
	}
}

type recordingResolver struct {
	called []string
}

func (r *recordingResolver) Resolve(_ context.Context, corpus, q string) (string, Citation, error) {
	r.called = append(r.called, corpus)
	return "snippet for " + corpus, Citation{Title: corpus + "-source"}, nil
}

func TestCustomResolverInvoked(t *testing.T) {
	r := &recordingResolver{}
	b := New(nil, "", r).Research(context.Background(),
		Request{Question: "x", Sources: []string{"rbi", "sahamati"}})
	if len(r.called) != 2 {
		t.Errorf("expected resolver called twice, got %d", len(r.called))
	}
	if len(b.Citations) != 2 || b.Citations[0].Title != "rbi-source" {
		t.Errorf("unexpected citations from custom resolver: %+v", b.Citations)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	b := New(nil, "", nil).Research(context.Background(), Request{Question: "x"})
	if b.Disclaimer == "" {
		t.Errorf("disclaimer must be present")
	}
}

func TestRiskClassMedium(t *testing.T) {
	if New(nil, "", nil).RiskLevel() != agent.RiskMedium {
		t.Errorf("deep_research must be RiskMedium")
	}
}
