// Package adapter bridges the two agent interfaces used in this codebase.
//
// There are two incompatible agent contracts:
//   - pkg/agent.Agent      — the legacy, message-driven contract used by ~34
//     registered agents: HandleMessage(ctx, Message, Environment) ([]Message, error).
//   - agents/core.Agent    — the decoupled, HTTP-native contract used by the new
//     advisor agents and the HTTP transport: Execute(ctx, interface{}) (interface{}, error).
//
// MessageBridgeAdapter wraps a pkg/agent.Agent so it satisfies agents/core.Agent,
// letting any of the legacy agents be served by the agents/core HTTP transport
// (POST /execute) with ZERO rewrites. It is the agent-side (per-binary) half of
// the decoupling; the backend-side half is pkg/registry.HTTPRegistryAgent.
//
// License: MIT
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agentgov"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// MessageBridgeAdapter adapts a pkg/agent.Agent (HandleMessage) to the
// agents/core.Agent (Execute) interface.
type MessageBridgeAdapter struct {
	inner   pkgagent.Agent
	gov     *agentgov.Bundle // nil-safe; governance recording skipped if nil
	version string

	now func() time.Time

	mu            sync.RWMutex
	lastHealthErr error
}

// compile-time assertion that the adapter satisfies agents/core.Agent.
var _ core.Agent = (*MessageBridgeAdapter)(nil)

// NewMessageBridge wraps a legacy agent. gov may be nil (governance recording is
// then skipped). version is the semantic version reported via Version().
func NewMessageBridge(inner pkgagent.Agent, gov *agentgov.Bundle, version string) *MessageBridgeAdapter {
	if version == "" {
		version = "1.0.0"
	}
	return &MessageBridgeAdapter{
		inner:   inner,
		gov:     gov,
		version: version,
		now:     time.Now,
	}
}

// Name returns the inner agent's name.
func (b *MessageBridgeAdapter) Name() string { return b.inner.Name() }

// Version returns the configured version.
func (b *MessageBridgeAdapter) Version() string { return b.version }

// Health reports the last recorded health error (legacy agents have no Health
// method, so the bridge is healthy unless a prior Execute recorded otherwise).
func (b *MessageBridgeAdapter) Health(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastHealthErr
}

// bridgeEnv is the minimal pkg/agent.Environment the bridge supplies to the
// wrapped agent. It exposes the injectable clock and a no-op logger.
type bridgeEnv struct {
	now func() time.Time
}

func (e bridgeEnv) Now() time.Time                  { return e.now() }
func (e bridgeEnv) Logf(format string, args ...any) { /* transport handles logging */ }

// Execute converts the core AgentInput payload into a protocol.Message, calls
// the wrapped agent's HandleMessage, and converts the returned []Message back
// into a result. Governance success/failure is recorded against the agent's ID.
//
// Wire mapping (core → legacy):
//
//	input payload     → Message.Content (JSON-encoded; a raw string passes through)
//	ctx user_id (meta)→ Message.Metadata[user_id]
//	agent name        → Message.To
//
// The returned []protocol.Message is the result. Callers (HTTP transport /
// backend HTTPRegistryAgent) re-publish those messages to continue the chain.
func (b *MessageBridgeAdapter) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	start := b.now()
	agentID := b.inner.ID()

	msg, err := b.toMessage(input)
	if err != nil {
		b.recordFailure(agentID, start, "encode: "+err.Error())
		return nil, core.NewExecutionError(b.Name(), core.ErrorCodeValidation, "encode input: "+err.Error(), err)
	}

	out, err := b.inner.HandleMessage(ctx, msg, bridgeEnv{now: b.now})
	if err != nil {
		b.recordFailure(agentID, start, err.Error())
		return nil, core.NewExecutionError(b.Name(), core.ErrorCodeInternal, "handle message", err)
	}

	b.recordSuccess(agentID, start)
	return out, nil
}

// toMessage builds a protocol.Message from the Execute input. The input is the
// AgentInput.Payload — it may already be a protocol.Message (in-process reuse),
// a raw JSON string/bytes, or any value to be JSON-encoded into Content.
func (b *MessageBridgeAdapter) toMessage(input interface{}) (protocol.Message, error) {
	switch v := input.(type) {
	case protocol.Message:
		return v, nil
	case *protocol.Message:
		if v == nil {
			return protocol.Message{}, fmt.Errorf("nil message")
		}
		return *v, nil
	}

	content, err := encodeContent(input)
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Message{
		To:        b.inner.ID(),
		Role:      protocol.RoleUser,
		Type:      "message.handle",
		Content:   content,
		CreatedAt: b.now().UTC(),
		Metadata:  map[string]any{},
	}, nil
}

// encodeContent renders an arbitrary payload into the Message.Content string. A
// string or []byte/json.RawMessage is used verbatim (so an agent expecting plain
// text gets plain text); anything else is JSON-encoded.
func encodeContent(input interface{}) (string, error) {
	switch v := input.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case json.RawMessage:
		return string(v), nil
	case []byte:
		return string(v), nil
	default:
		b, err := json.Marshal(input)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func (b *MessageBridgeAdapter) recordSuccess(agentID string, start time.Time) {
	b.mu.Lock()
	b.lastHealthErr = nil
	b.mu.Unlock()
	if b.gov != nil {
		b.gov.RecordSuccess(agentID, b.now().Sub(start))
	}
}

func (b *MessageBridgeAdapter) recordFailure(agentID string, start time.Time, reason string) {
	b.mu.Lock()
	b.lastHealthErr = fmt.Errorf("last execute failed: %s", reason)
	b.mu.Unlock()
	if b.gov != nil {
		b.gov.RecordFailure(agentID, b.now().Sub(start), reason)
	}
}
