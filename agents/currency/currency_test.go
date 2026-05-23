package currency

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

func TestCurrency_ConvertUSDtoINR(t *testing.T) {
	body, _ := json.Marshal(ConvertRequest{AmountMinor: 100, From: "USD", To: "INR"})
	msg := agent.NewMessage("normalizer", ID, agent.RoleAgent, TypeConvert, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var r ConvertResponse
	_ = json.Unmarshal([]byte(out[0].Content), &r)
	if r.AmountMinorTo != 8300 {
		t.Errorf("expected 8300, got %d", r.AmountMinorTo)
	}
}

func TestCurrency_UnknownCurrency(t *testing.T) {
	body, _ := json.Marshal(ConvertRequest{AmountMinor: 1, From: "XXX", To: "USD"})
	msg := agent.NewMessage("u", ID, agent.RoleAgent, TypeConvert, string(body), nil)
	if _, err := New().HandleMessage(context.Background(), msg, testEnv{}); err == nil {
		t.Fatal("expected error")
	}
}
