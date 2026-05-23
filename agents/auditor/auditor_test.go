package auditor

import (
	"context"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestAuditor_FlagsEmptyContent(t *testing.T) {
	store := eval.NewInMemoryStore()
	a := New(store)
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "", nil)
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Content == "ok" {
		t.Errorf("expected flag, got ok")
	}
	if got := store.List(); len(got) != 1 || got[0].Success {
		t.Errorf("expected one failure record, got %+v", got)
	}
}

func TestAuditor_BroadcastHandler(t *testing.T) {
	store := eval.NewInMemoryStore()
	h := NewHandler(store)
	h(context.Background(), agent.NewMessage("a", "b", agent.RoleAgent, "anything", "hello", nil))
	if got := store.List(); len(got) != 1 || !got[0].Success {
		t.Errorf("expected one successful record, got %+v", got)
	}
}
