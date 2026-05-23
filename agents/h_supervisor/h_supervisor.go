// Package h_supervisor is the hierarchical supervisor. It receives a high-
// level request, picks the right sub-domain (finance / portfolio / tax)
// using a semantic router, and dispatches to the corresponding specialist
// supervisor.
//
// Lets Genie scale beyond a single monolithic supervisor without forcing
// the consumer to know which sub-system to talk to.
package h_supervisor

import (
	"context"
	"errors"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/reasoning"
)

const (
	ID         = "h_supervisor"
	Capability = "hierarchical_supervise"
	TypeIn     = "h_supervise_request"
)

// Agent dispatches to a sub-supervisor based on semantic similarity.
type Agent struct {
	Router *reasoning.SemanticRouter
	// Routes maps route id → (agent id, message type to dispatch).
	Routes map[string]Route
}

// Route is one (agent id, message Type) target.
type Route struct {
	AgentID     string
	MessageType string
}

// New constructs the hierarchical supervisor.
func New(router *reasoning.SemanticRouter, routes map[string]Route) *Agent {
	return &Agent{Router: router, Routes: routes}
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Hierarchical Supervisor" }
func (a *Agent) Capabilities() []string { return []string{Capability} }

// RiskLevel — supervisors that decide routing influence every downstream
// agent; classify as Medium.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	if a.Router == nil {
		return nil, errors.New("h_supervisor: no router")
	}
	routes, err := a.Router.Classify(ctx, msg.Content)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
	if len(routes) == 0 {
		return nil, errors.New("h_supervisor: no routes configured")
	}
	winner := routes[0]
	target, ok := a.Routes[winner.ID]
	if !ok {
		return nil, fmt.Errorf("h_supervisor: route %q has no target", winner.ID)
	}
	env.Logf("[h_supervisor] %q → %s (score=%.3f)", msg.Content, target.AgentID, winner.Score)

	md := cloneMetadata(msg.Metadata)
	md["h_route_id"] = winner.ID
	md[protocol.MetaKeyClassification] = string(protocol.ClassInternal)
	return []agent.Message{
		agent.NewMessage(ID, target.AgentID, agent.RoleAgent, target.MessageType, msg.Content, md),
	}, nil
}

func cloneMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
