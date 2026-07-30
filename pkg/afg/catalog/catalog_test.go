package catalog

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
)

// The full catalog wires into a single governed registry with unique IDs.
func TestCatalog_RegistryComplete(t *testing.T) {
	if got := len(All()); got != 46 {
		t.Fatalf("want 46 ported specialist specs, got %d", got)
	}
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1 << 20})
	reg := Registry(gate)
	inv := reg.Inventory()
	if len(inv) != 59 { // 46 catalog + 4 hand-ported + 9 pipeline = full specialist set
		t.Fatalf("want 59 total governed agents, got %d", len(inv))
	}
	seen := map[string]bool{}
	for _, a := range inv {
		if !a.Governed {
			t.Fatalf("agent %q must be governed", a.ID)
		}
		if seen[a.ID] {
			t.Fatalf("duplicate agent id %q", a.ID)
		}
		seen[a.ID] = true
	}
}

// Every deterministic agent survives a minimal invocation without panicking
// (catches nil-derefs / bad indexing in generated Handle bodies). LLM agents are
// skipped to avoid network; they are covered by the gate-before-network test.
func TestCatalog_DeterministicNoPanic(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1 << 20})
	reg := Registry(gate)
	for _, info := range reg.Inventory() {
		if info.Provider == afg.ProviderOllama {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("deterministic agent %q panicked on {}: %v", info.ID, r)
				}
			}()
			_, _, _ = reg.RunWithFallback(context.Background(), info.ID, "{}")
		}()
	}
}
