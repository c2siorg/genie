package afg

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
)

func TestRegisterSpecs_DeterministicAndLLM(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 4096})
	reg := NewRegistry()
	reg.RegisterSpecs(gate,
		Spec{ID: "echo", Risk: "low", Handle: func(in string) (string, error) { return "echo:" + in, nil }},
		Spec{ID: "advisor", Risk: "medium", Instructions: "You are a financial educator."},
	)

	inv := reg.Inventory()
	if len(inv) != 2 {
		t.Fatalf("want 2, got %d", len(inv))
	}
	// Provider is inferred: Handle -> deterministic, Instructions -> ollama.
	byID := map[string]AgentInfo{inv[0].ID: inv[0], inv[1].ID: inv[1]}
	if byID["echo"].Provider != ProviderDeterministic || byID["advisor"].Provider != ProviderOllama {
		t.Fatalf("provider inference wrong: %+v", inv)
	}

	// deterministic spec runs
	if txt, _, err := reg.RunWithFallback(context.Background(), "echo", "hi"); err != nil || txt != "echo:hi" {
		t.Fatalf("echo spec wrong: err=%v txt=%q", err, txt)
	}
	// llm spec is governed too: a denial short-circuits before any network call
	deny := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1})
	reg2 := NewRegistry()
	reg2.RegisterSpecs(deny, Spec{ID: "advisor", Instructions: "x"})
	if _, _, err := reg2.RunWithFallback(context.Background(), "advisor", "long input"); err == nil || !strings.Contains(err.Error(), "governance denied") {
		t.Fatalf("llm spec must be gated pre-network, got: %v", err)
	}
}
