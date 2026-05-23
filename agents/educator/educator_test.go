package educator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestEducator_Known(t *testing.T) {
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "SIP", nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out[0].Content, "Systematic Investment Plan") {
		t.Fatalf("unexpected answer: %s", out[0].Content)
	}
}

func TestEducator_Unknown(t *testing.T) {
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "no such concept", nil)
	out, _ := New().HandleMessage(context.Background(), msg, testEnv{})
	if !strings.Contains(out[0].Content, "No glossary entry") {
		t.Fatalf("expected glossary fallback, got: %s", out[0].Content)
	}
}
