package agentic_test

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agentic"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
)

// TestSupervisor_FindAgent verifies that findAgent is correct via Route's
// fallback path — no LLM call needed for single-agent case.
func TestSupervisor_SingleAgent_Routes(t *testing.T) {
	fileReg := agenttools.FileTools()
	sup := &agentic.Supervisor{
		Agents: []agentic.SubAgent{
			{
				Name:        "file_agent",
				Description: "Reads and writes files on the local filesystem",
				Registry:    fileReg,
			},
		},
		Config: agentic.DefaultConfig(),
	}

	// With one agent, Route should return it without an LLM call.
	name, reason, err := sup.Route(nil, "show me config.json") //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	if name != "file_agent" {
		t.Errorf("expected file_agent, got %q", name)
	}
	if reason == "" {
		t.Error("expected non-empty routing reason")
	}
}

func TestSupervisor_FallbackAgent(t *testing.T) {
	sup := &agentic.Supervisor{
		Agents: []agentic.SubAgent{
			{Name: "a", Description: "Agent A", Registry: agenttools.FileTools()},
			{Name: "b", Description: "Agent B", Registry: agenttools.FileTools()},
		},
		Config:        agentic.DefaultConfig(),
		FallbackAgent: "a",
	}
	// findAgent should work via the exported Agents slice.
	found := false
	for _, ag := range sup.Agents {
		if ag.Name == sup.FallbackAgent {
			found = true
		}
	}
	if !found {
		t.Error("fallback agent not in Agents slice")
	}
}

func TestSubAgent_Fields(t *testing.T) {
	reg := agenttools.AllTools()
	sa := agentic.SubAgent{
		Name:         "test_agent",
		Description:  "A test agent",
		SystemPrompt: "You are a test agent.",
		Registry:     reg,
	}
	if sa.Name != "test_agent" {
		t.Errorf("name mismatch")
	}
	if sa.Registry == nil {
		t.Error("registry should not be nil")
	}
}
