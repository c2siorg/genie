package merchant

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/merchant"
)

// TestAgentInstantiation verifies the agent can be created.
func TestAgentInstantiation(t *testing.T) {
	manager := merchant.NewInMemoryMerchantManager()
	workflow := merchant.NewInMemoryOnboardingWorkflow(manager)

	a := New(manager, workflow)
	if a == nil {
		t.Fatal("agent should not be nil")
	}

	if a.ID() != AgentID {
		t.Errorf("expected ID %q, got %q", AgentID, a.ID())
	}

	if a.Name() != AgentName {
		t.Errorf("expected name %q, got %q", AgentName, a.Name())
	}

	caps := a.Capabilities()
	if len(caps) == 0 {
		t.Error("expected at least one capability")
	}

	if caps[0] != Capability {
		t.Errorf("expected capability %q, got %q", Capability, caps[0])
	}
}

// TestNewWithDefaults verifies agent can be created with defaults.
func TestNewWithDefaults(t *testing.T) {
	a := New(nil, nil)
	if a == nil {
		t.Fatal("agent should not be nil with nil dependencies")
	}

	if a.ID() != AgentID {
		t.Errorf("expected ID %q, got %q", AgentID, a.ID())
	}
}
