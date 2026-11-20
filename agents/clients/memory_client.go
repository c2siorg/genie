// Package clients - memory client for direct agent calls
package clients

import (
	"context"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
)

// MemoryClient calls agents directly in-process (no network overhead).
// This is how agents are called when embedded in the backend.
// Implements core.Agent interface.
type MemoryClient struct {
	agent core.Agent
}

// NewMemoryClient wraps a direct agent reference.
// Used when agents are embedded in backend.
func NewMemoryClient(agent core.Agent) *MemoryClient {
	return &MemoryClient{agent: agent}
}

// Name returns the agent name.
func (c *MemoryClient) Name() string {
	return c.agent.Name()
}

// Version returns the agent version.
func (c *MemoryClient) Version() string {
	return c.agent.Version()
}

// Health checks agent health.
func (c *MemoryClient) Health(ctx context.Context) error {
	return c.agent.Health(ctx)
}

// Execute calls the agent directly.
func (c *MemoryClient) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	return c.agent.Execute(ctx, input)
}

// AgentRegistry manages multiple agents for pipeline execution.
// Allows backend to orchestrate agents by name without knowing their implementation.
type AgentRegistry struct {
	agents map[string]core.Agent
}

// NewAgentRegistry creates a registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]core.Agent),
	}
}

// Register adds an agent to the registry.
func (r *AgentRegistry) Register(agent core.Agent) {
	r.agents[agent.Name()] = agent
}

// Get retrieves an agent by name.
func (r *AgentRegistry) Get(name string) core.Agent {
	return r.agents[name]
}

// GetAll returns all registered agents.
func (r *AgentRegistry) GetAll() map[string]core.Agent {
	return r.agents
}

// CreatePipeline builds a pipeline from agent names.
func (r *AgentRegistry) CreatePipeline(name string, agentNames ...string) *core.Pipeline {
	pipeline := &core.Pipeline{
		Name:   name,
		Agents: make([]core.Agent, 0, len(agentNames)),
	}

	for _, agentName := range agentNames {
		if agent, ok := r.agents[agentName]; ok {
			pipeline.Agents = append(pipeline.Agents, agent)
		}
	}

	return pipeline
}
