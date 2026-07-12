package afg

import (
	"context"
	"encoding/json"
	"iter"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/policy"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

const convertUSDINR = `{"amount_minor":10000,"from":"USD","to":"INR"}`

// Allowed message flows through the gate into the currency RunFunc and converts.
func TestGovernedCurrency_Allow(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1024})
	got, err := NewCurrencyAgent(gate).RunText(context.Background(), convertUSDINR).Collect()
	if err != nil {
		t.Fatalf("expected allow, got error: %v", err)
	}
	var out CurrencyResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(ResponseText(got))), &out); err != nil {
		t.Fatalf("bad response %q: %v", ResponseText(got), err)
	}
	if out.To != "INR" || out.AmountMinorTo != int64(10000*83.0) {
		t.Fatalf("wrong conversion: %+v", out)
	}
}

// The gate denies BEFORE the provider runs: the RunFunc flips `reached`, and on a
// denied message it must stay false. This is the load-bearing proof that the
// single-seam property survives the bus -> middleware move.
func TestGovernedCurrency_DenyShortCircuitsBeforeProvider(t *testing.T) {
	reached := false
	run := func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			reached = true
			yield(&agent.ResponseUpdate{
				Role:     message.RoleAssistant,
				Contents: message.Contents{&message.TextContent{Text: "SHOULD-NOT-APPEAR"}},
			}, nil)
		}
	}
	// Max:4 denies anything longer than 4 bytes -> the ~45-byte request is rejected.
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 4})
	got, err := NewGovernedDeterministic(gate, "deny-probe", run).RunText(context.Background(), convertUSDINR).Collect()
	if err == nil {
		t.Fatalf("expected governance denial, got resp=%q", ResponseText(got))
	}
	if !strings.Contains(err.Error(), "governance denied") {
		t.Fatalf("expected governance-denied error, got: %v", err)
	}
	if reached {
		t.Fatal("provider RunFunc must not run on a denied message")
	}
}

// The real board-approved policy (config/ai-policy.example.yaml) allows a benign
// currency message once its governance-relevant metadata is attached via ctx —
// proving the full composite (RBAC/required-metadata/classification/residency/PII)
// wires in, not just a toy policy.
func TestGovernedCurrency_RealBoardPolicyAllows(t *testing.T) {
	p, err := policy.Load("../../config/ai-policy.example.yaml")
	if err != nil {
		t.Fatalf("load board policy: %v", err)
	}
	gate := p.BuildComposite(nil)

	// amount_minor stays short so the PII policy's 12+/10-digit patterns don't fire.
	const in = `{"amount_minor":5000,"from":"EUR","to":"INR"}`
	pm := protocol.Message{
		From: "user", Role: protocol.RoleUser, Type: "convert_currency", Content: in,
		Metadata: map[string]any{
			protocol.MetaKeyUserRoles:      "user",
			protocol.MetaKeyClassification: string(protocol.ClassPublic),
			protocol.MetaKeyUserID:         "u1",
			"trace_id":                     "t1",
		},
	}
	ctx := WithGovMessage(context.Background(), pm)
	got, err := NewCurrencyAgent(gate).RunText(ctx, in).Collect()
	if err != nil {
		t.Fatalf("real board policy should allow benign currency msg, got: %v", err)
	}
	if !strings.Contains(ResponseText(got), `"rate"`) {
		t.Fatalf("unexpected response: %q", ResponseText(got))
	}
}

// The gate must also short-circuit the OLLAMA path BEFORE any network call: a
// denied message returns a governance error, NOT a connection error to
// localhost:11434. This proves the openaiprovider middleware slot gates pre-provider
// (both provider paths governed) without needing a live Ollama.
func TestGovernedOllama_DeniesBeforeNetwork(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1})
	a := NewGovernedOllama(gate, "fx_explainer", "be terse", "llama3.1")
	_, err := a.RunText(context.Background(), "this input is far longer than one byte").Collect()
	if err == nil {
		t.Fatal("expected governance denial on the Ollama path")
	}
	if !strings.Contains(err.Error(), "governance denied") {
		t.Fatalf("gate must run before the network; got a non-governance error: %v", err)
	}
}
