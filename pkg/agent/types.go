package agent

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// Re-export core protocol types for convenience.
//
// Architectural rationale:
// - "Everyone" needs to refer to Message/Role.
// - If comm/governance/orchestration imported pkg/agent just to use Message,
//   we could easily create dependency cycles.
// - Instead, the true definitions live in pkg/protocol, and pkg/agent re-exports
//   them because agents work with messages constantly and the ergonomics matter.
type Message = protocol.Message
type MessageRole = protocol.MessageRole

const (
	RoleUser      = protocol.RoleUser
	RoleSystem    = protocol.RoleSystem
	RoleAgent     = protocol.RoleAgent
	RoleTool      = protocol.RoleTool
	RoleObserver  = protocol.RoleObserver
	RoleEvaluator = protocol.RoleEvaluator
)

// NewMessage constructs a new protocol message.
//
// This is a thin wrapper so agent implementations can do `agent.NewMessage(...)`
// without importing pkg/protocol directly.
func NewMessage(from string, to string, role MessageRole, msgType, content string, metadata map[string]any) Message {
	return protocol.NewMessage(from, to, role, msgType, content, metadata)
}

// Agent describes the minimal interface for an agent in the system.
//
// How to read this interface:
//
// - ID: stable routing identity ("address") for the agent within the system.
// - Name: human-friendly label used in logs/UIs.
// - Capabilities: the agent's "skills" advertised to registries/routers.
// - HandleMessage: the single entry point for work.
//
// In this reference implementation, orchestration is message-driven: agents do
// not call each other directly. They emit messages that are routed by the bus.
// This keeps coupling low and supports swapping implementations.
type Agent interface {
	// ID returns the unique ID of the agent.
	ID() string
	// Name is a human-readable name.
	Name() string
	// Capabilities describes what the agent can do.
	Capabilities() []string
	// HandleMessage processes an incoming message and may emit zero or more messages in response.
	HandleMessage(ctx context.Context, msg Message, env Environment) ([]Message, error)
}

// Environment provides agents with access to shared services like memory, tools, and observability.
//
// In a production platform, Environment often becomes a service-locator that
// exposes things like:
// - Memory (short-term scratchpad, long-term vector store, conversation state)
// - Tools (search, code execution, retrieval, DB access)
// - Identity/security context (who asked, what permissions apply)
// - Observability (structured logging, tracing, metrics)
// - Evaluation hooks (rubrics, test harness, golden datasets)
//
// This repo keeps Environment minimal on purpose and expands by composition
// rather than baking in assumptions.
type Environment interface {
	Now() time.Time
	Logf(format string, args ...any)
	// Additional services (memory, tools, policies, etc.) can be added here as needed.
}

