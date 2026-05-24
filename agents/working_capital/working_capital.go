// Package working_capital forecasts SME cashflow from a working-capital
// cycle: Days Sales Outstanding (DSO) + Days Inventory Outstanding (DIO)
// − Days Payable Outstanding (DPO) = Cash Conversion Cycle (CCC).
//
// Output: month-by-month operating cashflow projection and the runway
// (months of cash) before going negative under the current burn rate.
package working_capital

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "working_capital_forecaster"
	Capability = "forecast_working_capital"
	TypeIn     = "working_capital_request"
	TypeOut    = "working_capital_forecast"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload.
type Request struct {
	MonthlyRevenue       float64 `json:"monthly_revenue_rupees"`
	GrossMarginPct       float64 `json:"gross_margin_pct"`
	OperatingCostsMonthly float64 `json:"operating_costs_monthly_rupees"`
	DSO                  int     `json:"dso_days"`
	DIO                  int     `json:"dio_days"`
	DPO                  int     `json:"dpo_days"`
	OpeningCashINR       float64 `json:"opening_cash_rupees"`
	HorizonMonths        int     `json:"horizon_months"`
}

// MonthRow is one period of the forecast.
type MonthRow struct {
	Month         int     `json:"month"`
	CashCollected float64 `json:"cash_collected_rupees"`
	CashPaid      float64 `json:"cash_paid_rupees"`
	NetCashflow   float64 `json:"net_cashflow_rupees"`
	EndCash       float64 `json:"end_cash_rupees"`
}

// Result is the wire output.
type Result struct {
	CCC              int        `json:"cash_conversion_cycle_days"`
	RunwayMonths     int        `json:"runway_months"`
	Forecast         []MonthRow `json:"forecast"`
	WorkingCapitalGap float64   `json:"working_capital_gap_rupees"`
	Recommendation   string     `json:"recommendation"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Working Capital Forecaster" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Forecast(req)
	env.Logf("[working_capital] CCC=%d runway=%d", res.CCC, res.RunwayMonths)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Forecast runs the working-capital simulation.
func (a *Agent) Forecast(req Request) Result {
	ccc := req.DSO + req.DIO - req.DPO

	// Cash collected this month = revenue from ⌈DSO/30⌉ months ago.
	// Cash paid this month = COGS (current month) shifted by DPO + opex (current month).
	cogs := req.MonthlyRevenue * (1 - req.GrossMarginPct/100)
	cash := req.OpeningCashINR
	rows := make([]MonthRow, 0, req.HorizonMonths)
	runway := req.HorizonMonths
	for m := 1; m <= req.HorizonMonths; m++ {
		// Sales originated DSO days ago turn into cash now.
		collected := req.MonthlyRevenue
		if m <= req.DSO/30 {
			collected = 0 // not yet collected for the first DSO/30 months
		}
		// Vendor payment lagged by DPO days.
		paid := cogs
		if m <= req.DPO/30 {
			paid = 0
		}
		paid += req.OperatingCostsMonthly
		net := collected - paid
		cash += net
		rows = append(rows, MonthRow{
			Month: m, CashCollected: round2(collected),
			CashPaid: round2(paid), NetCashflow: round2(net), EndCash: round2(cash),
		})
		if cash < 0 && runway == req.HorizonMonths {
			runway = m - 1
		}
	}
	gap := 0.0
	if rows[0].EndCash < 0 {
		gap = -rows[0].EndCash
	}
	rec := "Healthy cycle — monitor DSO drift quarterly."
	if runway < 6 {
		rec = "Runway <6 months — accelerate collections (early-payment discount) or extend DPO with vendor financing."
	}
	if ccc > 90 {
		rec = "Cash Conversion Cycle >90 days — consider invoice discounting (TReDS) to free up working capital."
	}
	return Result{
		CCC: ccc, RunwayMonths: runway, Forecast: rows,
		WorkingCapitalGap: round2(gap), Recommendation: rec,
	}
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
