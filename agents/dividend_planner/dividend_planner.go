// Package dividend_planner simulates a Dividend Reinvestment Plan (DRIP):
// project the dividend stream + reinvested-share growth over N years
// given a starting position, dividend per share, growth rate, and reinv
// share price assumption.
//
// India context: dividends were tax-free up to FY 2019-20 (DDT regime);
// since then dividends are taxed at the recipient's slab. The agent
// accepts a slab rate and computes both gross and net-of-tax projections.
package dividend_planner

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "dividend_planner"
	Capability = "plan_dividend"
	TypeIn     = "dividend_request"
	TypeOut    = "dividend_projection"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload.
type Request struct {
	Shares                int     `json:"shares"`
	CurrentPrice          float64 `json:"current_price_rupees"`
	DividendPerShare      float64 `json:"dividend_per_share_rupees"`
	DividendGrowthAnnual  float64 `json:"dividend_growth_annual"`   // decimal
	PriceAppreciationAnn  float64 `json:"price_appreciation_annual"`
	HorizonYears          int     `json:"horizon_years"`
	TDSAndSlabPct         float64 `json:"tds_and_slab_pct"`
	Reinvest              bool    `json:"reinvest_dividends"`
}

// YearRow is one year of the projection.
type YearRow struct {
	Year          int     `json:"year"`
	Shares        float64 `json:"shares"`
	DPSGross      float64 `json:"dps_gross"`
	DividendNet   float64 `json:"dividend_net_rupees"`
	PriceEOY      float64 `json:"price_eoy"`
	HoldingValue  float64 `json:"holding_value_eoy_rupees"`
}

// Result is the wire output.
type Result struct {
	Schedule       []YearRow `json:"schedule"`
	TotalDividend  float64   `json:"total_dividend_received_net_rupees"`
	TerminalValue  float64   `json:"terminal_holding_value_rupees"`
	YieldOnCostPct float64   `json:"final_yield_on_cost_pct"`
	Disclaimer     string    `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Dividend Planner" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskLow }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Project(req)
	env.Logf("[dividend_planner] terminal value=%.0f", res.TerminalValue)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Project runs the year-by-year DRIP simulation.
func (a *Agent) Project(req Request) Result {
	shares := float64(req.Shares)
	price := req.CurrentPrice
	dps := req.DividendPerShare
	costBasis := shares * price
	totalNet := 0.0
	rows := make([]YearRow, 0, req.HorizonYears)
	for y := 1; y <= req.HorizonYears; y++ {
		grossDividend := shares * dps
		net := grossDividend * (1 - req.TDSAndSlabPct/100)
		totalNet += net
		// Apply price appreciation across the year first.
		price = price * (1 + req.PriceAppreciationAnn)
		if req.Reinvest && price > 0 {
			shares += net / price
		}
		rows = append(rows, YearRow{
			Year:         y,
			Shares:       round4(shares),
			DPSGross:     round4(dps),
			DividendNet:  round2(net),
			PriceEOY:     round2(price),
			HoldingValue: round2(shares * price),
		})
		dps = dps * (1 + req.DividendGrowthAnnual)
	}
	terminal := shares * price
	yoc := 0.0
	if costBasis > 0 && len(rows) > 0 {
		yoc = rows[len(rows)-1].DividendNet / costBasis * 100
	}
	return Result{
		Schedule:       rows,
		TotalDividend:  round2(totalNet),
		TerminalValue:  round2(terminal),
		YieldOnCostPct: round2(yoc),
		Disclaimer: "Projection assumes fixed annual growth + price appreciation. " +
			"Indian dividends are taxed at recipient's slab + applicable TDS. Past dividends are not a guarantee of future payouts.",
	}
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
