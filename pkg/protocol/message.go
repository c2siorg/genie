package protocol

import (
	"time"

	"github.com/google/uuid"
)

// MessageRole indicates the semantic role of a message in an interaction.
//
// Roles are deliberately coarse-grained. They help:
// - Observability: filter/aggregate interactions by "who said it"
// - Governance: apply different policies based on origin/type
// - Evaluation: separate user intent from agent/tool outputs
//
// In a production system, roles are typically complemented by stronger identity
// concepts (service identity, user identity, authz context, tenant, etc.).
type MessageRole string

const (
	// RoleUser identifies messages that represent end-user intent or input.
	RoleUser      MessageRole = "user"

	// RoleSystem is reserved for control-plane/system-level instructions
	// (e.g. "you must follow policy X") that typically shouldn't be mixed with
	// end-user content.
	RoleSystem    MessageRole = "system"

	// RoleAgent identifies agent-authored messages (plans, intermediate steps,
	// answers, delegations, etc.).
	RoleAgent     MessageRole = "agent"

	// RoleTool identifies outputs produced by tools invoked by agents
	// (search results, code execution, DB query responses, etc.).
	RoleTool      MessageRole = "tool"

	// RoleObserver identifies passive observation events (telemetry, audit,
	// monitoring) that can be emitted into the same stream for tracing.
	RoleObserver  MessageRole = "observer"

	// RoleEvaluator identifies evaluation events or rubric-based judgments.
	RoleEvaluator MessageRole = "evaluator"
)

// Message represents a unit of communication between components (primarily agents).
//
// Mental model
//
// - A message is the "currency" of coordination.
// - Orchestration routes messages to the appropriate agents.
// - Agents handle a message and emit follow-up messages.
// - Governance can allow/deny messages at boundaries.
// - Observability and evaluation can consume the same message stream.
//
// Addressing
//
// - From: identity of the sender (agent id, "user", tool id, etc.).
// - To: identity of the intended recipient. If empty, the message can be treated
//   as broadcast (depending on the bus implementation).
//
// Classification
//
// - Role: semantic source category (user/agent/tool/system/etc.)
// - Type: short machine-friendly label used by agents to branch logic.
//   Examples: "goal", "plan", "result", "tool:search", "event:audit", etc.
//
// Extensibility
//
// - Metadata is the escape hatch for correlation ids, trace context, domain hints,
//   safety labels, cost accounting, and other structured context.
type Message struct {
	// ID uniquely identifies this message instance.
	// The ID is useful for logging, auditing, and causal graphs.
	ID string `json:"id"`

	// From is the sender identity (agent id, "user", etc.).
	From string `json:"from"`

	// To is the intended recipient identity.
	To string `json:"to,omitempty"`

	// Role is a semantic category of the sender/content.
	Role MessageRole `json:"role"`

	// Type is a short label used for routing/handling decisions.
	Type string `json:"type"`

	// Content is the primary payload. For rich payloads, you can encode JSON and
	// declare a Type that indicates the encoding, or place structured data in Metadata.
	Content string `json:"content"`

	// CreatedAt records when the message was created (UTC).
	CreatedAt time.Time `json:"created_at"`

	// Metadata carries optional structured context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewMessage constructs a new message with sensible defaults.
//
// Defaults:
// - ID: UUID
// - CreatedAt: current time in UTC
//
// This keeps message creation consistent across all packages and reduces the
// chance that downstream systems (observability/evaluation) see partially
// populated messages.
func NewMessage(from string, to string, role MessageRole, msgType, content string, metadata map[string]any) Message {
	return Message{
		ID:        uuid.NewString(),
		From:      from,
		To:        to,
		Role:      role,
		Type:      msgType,
		Content:   content,
		CreatedAt: time.Now().UTC(),
		Metadata:  metadata,
	}
}

