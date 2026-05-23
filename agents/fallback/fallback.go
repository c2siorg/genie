// Package fallback provides a generic fallback agent — when a primary
// agent's HandleMessage returns an error, the orchestrator can route to a
// fallback id and this agent emits a "human review required" message back
// to the user.
//
// This is the BCP-for-AI plumbing the RBI FREE-AI report asks for in
// Recommendation 21: an AI model that fails should be able to declare itself
// "unavailable" and trigger a backup process.
package fallback

import (
	"context"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

// Agent is a parameterised fallback. Construct one per primary agent with a
// distinct ID via NewFor (e.g. NewFor("recommender") -> id "recommender_fallback").
type Agent struct {
	id      string
	primary string
	target  string // who to dispatch the human-review notice to (default "user")
}

// NewFor builds a fallback for the given primary agent id.
// The fallback id is `<primary>_fallback`.
func NewFor(primary string) *Agent {
	return &Agent{
		id:      primary + "_fallback",
		primary: primary,
		target:  "user",
	}
}

// WithTarget overrides where the "human review" notice is dispatched.
func (a *Agent) WithTarget(id string) *Agent { a.target = id; return a }

func (a *Agent) ID() string             { return a.id }
func (a *Agent) Name() string           { return "Fallback for " + a.primary }
func (a *Agent) Capabilities() []string { return []string{"fallback"} }

// RiskLevel — fallbacks are low risk by definition: they only emit a notice.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskLow }

func (a *Agent) HandleMessage(_ context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	env.Logf("[fallback:%s] primary failed; emitting human-review notice", a.primary)
	notice := fmt.Sprintf("The %s agent could not complete your request. A human reviewer will follow up. Reference id: %s",
		a.primary, msg.ID)
	out := agent.NewMessage(a.id, a.target, agent.RoleAgent, "fallback_notice", notice, msg.Metadata)
	return []agent.Message{out}, nil
}
