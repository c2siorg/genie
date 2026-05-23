package loan

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestLoan_EMIClassic(t *testing.T) {
	// 1,000,000 minor units (10,000.00) @ 12% APR over 24 months → ~470.73/mo major.
	body, _ := json.Marshal(Request{PrincipalCents: 1_000_000, APRPct: 12, TermMonths: 24, MonthlyNetCents: 100_000})
	msg := agent.NewMessage("u", ID, agent.RoleAgent, TypeSimulate, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var r Response
	_ = json.Unmarshal([]byte(out[0].Content), &r)
	if r.EMIInCents < 47000 || r.EMIInCents > 48000 {
		t.Errorf("EMI out of expected band, got %d", r.EMIInCents)
	}
}
