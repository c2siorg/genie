package rates

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

func TestRates_USOutlook(t *testing.T) {
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "us", nil)
	out, _ := New().HandleMessage(context.Background(), msg, testEnv{})
	if !strings.Contains(out[0].Content, "Fed") {
		t.Fatalf("expected Fed mention, got: %s", out[0].Content)
	}
}
