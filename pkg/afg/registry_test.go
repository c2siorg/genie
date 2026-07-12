package afg

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

// failingRun is an agent whose provider always errors (simulates a downstream
// outage) — used to exercise fallback routing (Genie's BCP drill).
func failingRun() agent.RunFunc {
	return func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			yield(nil, errors.New("upstream advisor unavailable"))
		}
	}
}

func TestRegistry_InventoryAndFallback(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 4096})
	reg := NewRegistry()

	reg.Register(AgentInfo{ID: CurrencyName, Provider: ProviderDeterministic, Risk: "low"}, NewCurrencyAgent(gate))
	reg.Register(AgentInfo{ID: "portfolio_advisor", Provider: ProviderOllama, Risk: "high"},
		NewGovernedDeterministic(gate, "portfolio_advisor", failingRun()))
	reg.RegisterFallback("portfolio_advisor",
		NewGovernedDeterministic(gate, "portfolio_advisor-fallback", cannedRun("A human advisor will follow up.")))

	// Inventory is governed, sorted, and reflects fallback wiring.
	inv := reg.Inventory()
	if len(inv) != 2 {
		t.Fatalf("want 2 inventory rows, got %d", len(inv))
	}
	if inv[0].ID != CurrencyName || inv[1].ID != "portfolio_advisor" {
		t.Fatalf("inventory not sorted: %+v", inv)
	}
	for _, row := range inv {
		if !row.Governed {
			t.Fatalf("row %s must be governed", row.ID)
		}
	}
	if inv[1].HasFallback != true || inv[0].HasFallback != false {
		t.Fatalf("fallback flags wrong: %+v", inv)
	}

	// Primary execution error -> fallback fires.
	txt, usedFB, err := reg.RunWithFallback(context.Background(), "portfolio_advisor", "advise me")
	if err != nil {
		t.Fatalf("expected fallback success, got: %v", err)
	}
	if !usedFB || !strings.Contains(txt, "human advisor") {
		t.Fatalf("fallback did not fire correctly: used=%v txt=%q", usedFB, txt)
	}

	// Currency succeeds -> no fallback path.
	txt, usedFB, err = reg.RunWithFallback(context.Background(), CurrencyName, `{"amount_minor":100,"from":"USD","to":"INR"}`)
	if err != nil || usedFB || !strings.Contains(txt, `"rate"`) {
		t.Fatalf("currency happy path wrong: used=%v err=%v txt=%q", usedFB, err, txt)
	}
}

// A governance denial must NOT be rescued by a fallback — it is a policy rejection.
func TestRegistry_DenialNotRescuedByFallback(t *testing.T) {
	deny := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1}) // denies everything non-trivial
	reg := NewRegistry()
	reg.Register(AgentInfo{ID: "gated", Provider: ProviderDeterministic, Risk: "low"},
		NewGovernedDeterministic(deny, "gated", cannedRun("should not appear")))
	reg.RegisterFallback("gated", NewGovernedDeterministic(deny, "gated-fb", cannedRun("fallback answer")))

	txt, usedFB, err := reg.RunWithFallback(context.Background(), "gated", "a long enough input to be denied")
	if err == nil {
		t.Fatalf("expected governance denial, got txt=%q", txt)
	}
	if usedFB {
		t.Fatal("fallback must NOT rescue a governance denial")
	}
	var denied *DeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected *DeniedError, got %T: %v", err, err)
	}
}
