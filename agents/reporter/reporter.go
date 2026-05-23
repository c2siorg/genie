// Package reporter renders a human-readable final report from the supervisor's
// merged analysis + forecast + anomalies + recommendations.
package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "reporter"
	CapReport  = "render_report"
	TypeIn     = "final_report_request"
	TypeOut    = "final_report"
	NextAgent  = "user"
)

// Bundle is what the supervisor publishes to the reporter.
type Bundle struct {
	Question          string          `json:"question"`
	Currency          string          `json:"currency"`
	TotalIncomeCents  int64           `json:"total_income_cents"`
	TotalExpenseCents int64           `json:"total_expense_cents"`
	NetCents          int64           `json:"net_cents"`
	TopOverspend      []string        `json:"top_overspend"`
	Forecast          json.RawMessage `json:"forecast,omitempty"`
	Anomalies         json.RawMessage `json:"anomalies,omitempty"`
	Recommendations   json.RawMessage `json:"recommendations,omitempty"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Reporter" }
func (a *Agent) Capabilities() []string { return []string{CapReport} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var b Bundle
	if err := json.Unmarshal([]byte(msg.Content), &b); err != nil {
		return nil, err
	}

	var s strings.Builder
	fmt.Fprintf(&s, "Genie Financial Report\n")
	fmt.Fprintf(&s, "Question: %s\n", b.Question)
	fmt.Fprintf(&s, "Currency: %s\n", b.Currency)
	fmt.Fprintf(&s, "Income:  %d (minor units)\n", b.TotalIncomeCents)
	fmt.Fprintf(&s, "Expense: %d (minor units)\n", b.TotalExpenseCents)
	fmt.Fprintf(&s, "Net:     %d (minor units)\n", b.NetCents)
	if len(b.TopOverspend) > 0 {
		fmt.Fprintf(&s, "Top categories: %s\n", strings.Join(b.TopOverspend, ", "))
	}
	if len(b.Forecast) > 0 {
		fmt.Fprintf(&s, "Forecast: %s\n", string(b.Forecast))
	}
	if len(b.Anomalies) > 0 {
		fmt.Fprintf(&s, "Anomalies: %s\n", string(b.Anomalies))
	}
	if len(b.Recommendations) > 0 {
		fmt.Fprintf(&s, "Recommendations: %s\n", string(b.Recommendations))
	}

	env.Logf("[reporter] final report ready (%d bytes)", s.Len())
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, s.String(), msg.Metadata),
	}, nil
}
