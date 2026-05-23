// Package macro is a stub for the ADK Economic Research Agent: it returns a
// short macro outlook for the requested region. Real implementation would call
// a market-data tool.
package macro

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "macro_research"
	CapContext = "macro_context"
	TypeIn     = "macro_context"
	TypeOut    = "macro_context_result"
)

var outlooks = map[string]string{
	"in": "Indian growth steady ~6.8%; CPI within RBI tolerance; INR moderately weak vs USD.",
	"us": "US growth moderating; core PCE near target; Fed in pause-with-bias-to-cut posture.",
	"eu": "Euro area stagnating; HICP near 2%; ECB easing cycle underway.",
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Macro Research" }
func (a *Agent) Capabilities() []string { return []string{CapContext} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	region := strings.ToLower(strings.TrimSpace(msg.Content))
	if region == "" {
		region = "in"
	}
	outlook, ok := outlooks[region]
	if !ok {
		outlook = "No regional outlook available; defaulting to: " + outlooks["in"]
	}
	env.Logf("[macro] outlook for %s", region)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, outlook, msg.Metadata),
	}, nil
}
