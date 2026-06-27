// Package core defines the agent abstraction layer.
// Agents are independent, stateless services that can run anywhere.
// No dependencies on HTTP handlers, storage, or framework-specific code.
package core

import (
	"context"
)

// Agent defines the interface all agents must implement.
// Agents are stateless and can be deployed anywhere: backend, frontend, cloud.
type Agent interface {
	// Name returns the agent identifier.
	Name() string

	// Version returns the semantic version.
	Version() string

	// Execute runs the agent with input and returns output.
	// Context carries request metadata (user_id, trace_id, etc).
	Execute(ctx context.Context, input interface{}) (interface{}, error)

	// Health checks if the agent is ready to serve requests.
	Health(ctx context.Context) error
}

// AgentInput wraps input data for agent execution.
type AgentInput struct {
	UserID    string                 `json:"user_id"`
	TraceID   string                 `json:"trace_id"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Payload   interface{}            `json:"payload"` // Agent-specific data
}

// AgentOutput wraps output data from agent execution.
type AgentOutput struct {
	AgentName string                 `json:"agent_name"`
	TraceID   string                 `json:"trace_id"`
	Status    string                 `json:"status"` // "success" or "error"
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionContext carries request-scoped data (without framework deps).
type ExecutionContext struct {
	UserID   string
	TraceID  string
	Metadata map[string]interface{}
}

// FromContext extracts ExecutionContext from context.Context.
func FromContext(ctx context.Context) ExecutionContext {
	if ec, ok := ctx.Value(executionContextKey).(ExecutionContext); ok {
		return ec
	}
	return ExecutionContext{
		Metadata: make(map[string]interface{}),
	}
}

// WithContext embeds ExecutionContext into context.Context.
func WithContext(ctx context.Context, ec ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey, ec)
}

// contextKey prevents collisions with other context keys.
type contextKey string

const executionContextKey contextKey = "execution_context"

// Pipeline defines how multiple agents work together.
// Example: ProfileAnalyzer → FinancialAnalyst → RecommendationGenerator
type Pipeline struct {
	Name   string
	Agents []Agent
}

// Execute runs agents in sequence, passing output as input to the next.
func (p *Pipeline) Execute(ctx context.Context, initialInput interface{}) (interface{}, error) {
	result := initialInput
	for _, agent := range p.Agents {
		output, err := agent.Execute(ctx, result)
		if err != nil {
			return nil, err
		}
		result = output
	}
	return result, nil
}
