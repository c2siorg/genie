package fallback

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

func TestFallback_EmitsHumanReviewNotice(t *testing.T) {
	a := NewFor("recommender")
	if a.ID() != "recommender_fallback" {
		t.Fatalf("unexpected id %q", a.ID())
	}
	msg := agent.NewMessage("recommender", "recommender_fallback", agent.RoleAgent, "fallback_request", "anything", nil)
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].To != "user" {
		t.Fatalf("expected dispatch to user, got %+v", out)
	}
	if !strings.Contains(out[0].Content, "human reviewer") {
		t.Errorf("notice missing human-review wording: %s", out[0].Content)
	}
}
