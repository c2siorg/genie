// Package forecaster projects 1-, 3-, and 6-month cashflow using a naive
// linear extrapolation from the analyzer's totals. Good enough to demonstrate
// the role; a real implementation would use seasonality and account history.
package forecaster

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "forecaster"
	CapCash    = "forecast_cashflow"
	TypeIn     = "analysis_result"
	TypeOut    = "forecast_result"
	NextAgent  = "financial_supervisor"
)

type analyzerView struct {
	TotalIncomeCents  int64                 `json:"total_income_cents"`
	TotalExpenseCents int64                 `json:"total_expense_cents"`
	NetCents          int64                 `json:"net_cents"`
	Currency          string                `json:"currency"`
	Transactions      []finance.Transaction `json:"transactions"`
}

type Forecast struct {
	Currency        string `json:"currency"`
	MonthlyNetCents int64  `json:"monthly_net_cents"`
	Horizon1mCents  int64  `json:"horizon_1m_cents"`
	Horizon3mCents  int64  `json:"horizon_3m_cents"`
	Horizon6mCents  int64  `json:"horizon_6m_cents"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Cashflow Forecaster" }
func (a *Agent) Capabilities() []string { return []string{CapCash} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	f := Forecast{
		Currency:        av.Currency,
		MonthlyNetCents: av.NetCents,
		Horizon1mCents:  av.NetCents,
		Horizon3mCents:  av.NetCents * 3,
		Horizon6mCents:  av.NetCents * 6,
	}
	env.Logf("[forecaster] 6m projection=%d %s", f.Horizon6mCents, f.Currency)
	body, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}
