package afg

import (
	"context"
	"iter"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

func cannedRun(text string) agent.RunFunc {
	return func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			yield(&agent.ResponseUpdate{Role: message.RoleAssistant,
				Contents: message.Contents{&message.TextContent{Text: text}}}, nil)
		}
	}
}

// Fan-out over governed specialists (the real currency agent + two deterministic
// notes) runs concurrently and collects every output at the fan-in.
func TestFanOut_GovernedSpecialists(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 4096})
	cur := NewCurrencyAgent(gate)
	disc := NewGovernedDeterministic(gate, "disclaimer", cannedRun("This is informational, not advice."))
	risk := NewGovernedDeterministic(gate, "risk_note", cannedRun("FX rates fluctuate."))

	in := `{"amount_minor":10000,"from":"USD","to":"INR"}`
	outs, err := FanOut(context.Background(), in, cur, disc, risk)
	if err != nil {
		t.Fatalf("fanout: %v", err)
	}
	if len(outs) != 3 {
		t.Fatalf("want 3 specialist outputs, got %d: %+v", len(outs), outs)
	}
	var joined strings.Builder
	for _, o := range outs {
		joined.WriteString(o.Text)
	}
	all := joined.String()
	if !strings.Contains(all, `"rate"`) {
		t.Fatalf("currency specialist output missing from fan-in: %q", all)
	}
	if !strings.Contains(all, "informational") || !strings.Contains(all, "fluctuate") {
		t.Fatalf("deterministic specialist outputs missing from fan-in: %q", all)
	}
}
