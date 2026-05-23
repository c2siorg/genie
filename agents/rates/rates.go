// Package rates is a stub for the ADK FOMC Research Agent — returns a
// short rate outlook string.
package rates

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "rate_watcher"
	CapOutlook = "rate_outlook"
	TypeIn     = "rate_outlook"
	TypeOut    = "rate_outlook_result"
)

var outlook = map[string]string{
	"in": "RBI policy repo rate steady; tone neutral with watchful inflation guidance.",
	"us": "Fed funds at 5.25-5.50%; statement signals data-dependent pause.",
	"eu": "ECB deposit rate trending lower; forward guidance tilts dovish.",
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Rate Watcher" }
func (a *Agent) Capabilities() []string { return []string{CapOutlook} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	cb := strings.ToLower(strings.TrimSpace(msg.Content))
	if cb == "" {
		cb = "in"
	}
	body, ok := outlook[cb]
	if !ok {
		body = outlook["in"]
	}
	env.Logf("[rates] outlook %s", cb)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, body, msg.Metadata),
	}, nil
}
