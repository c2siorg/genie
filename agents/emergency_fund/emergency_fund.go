// Package emergency_fund computes the gap between the user's current
// liquid reserves and a target emergency fund. Target is sized off the
// median monthly expense from the analyzer view, multiplied by a coverage
// factor (3 months for stable salary, 6 for variable income, 9 for sole
// breadwinners with dependents).
package emergency_fund

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "emergency_fund_advisor"
	Capability = "advise_emergency_fund"
	TypeIn     = "emergency_fund_request"
	TypeOut    = "emergency_fund_plan"
	NextAgent  = "financial_supervisor"

	CoverageStable     = 3
	CoverageVariable   = 6
	CoverageDependents = 9
)

// Request is the wire payload.
type Request struct {
	Transactions       []finance.Transaction `json:"transactions"`
	LiquidReservesINR  float64               `json:"liquid_reserves_rupees"`
	IncomeProfile      string                `json:"income_profile"` // "stable" | "variable"
	HasDependents      bool                  `json:"has_dependents"`
}

// Plan is the wire output.
type Plan struct {
	MedianMonthlyExpense float64 `json:"median_monthly_expense_rupees"`
	CoverageMonths       int     `json:"coverage_months"`
	TargetINR            float64 `json:"target_rupees"`
	GapINR               float64 `json:"gap_rupees"`
	MonthlySaveINR       float64 `json:"suggested_monthly_save_rupees"`
	MonthsToTarget       int     `json:"months_to_target"`
	Rationale            string  `json:"rationale"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Emergency Fund Advisor" }
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
	plan := a.Compute(req)
	env.Logf("[emergency_fund] gap=%.0f months=%d", plan.GapINR, plan.MonthsToTarget)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute sizes the target fund and the gap.
func (a *Agent) Compute(req Request) Plan {
	monthly := medianMonthlyExpense(req.Transactions)
	cov := CoverageStable
	rationale := "3 months of expenses — stable salaried income with no dependents."
	switch {
	case req.HasDependents:
		cov = CoverageDependents
		rationale = "9 months — sole breadwinner with dependents; longer runway recommended."
	case req.IncomeProfile == "variable":
		cov = CoverageVariable
		rationale = "6 months — variable income (freelance / commissioned) needs deeper buffer."
	}
	target := monthly * float64(cov)
	gap := target - req.LiquidReservesINR
	if gap < 0 {
		gap = 0
		rationale += " You're already above the target — consider redirecting incremental savings to long-term goals."
	}
	monthlySave := monthly * 0.10 // suggest 10 % of monthly expense as the savings rate
	months := 0
	if monthlySave > 0 && gap > 0 {
		months = int(gap / monthlySave)
		if int(gap)%int(monthlySave) > 0 {
			months++
		}
	}
	return Plan{
		MedianMonthlyExpense: round2(monthly),
		CoverageMonths:       cov,
		TargetINR:            round2(target),
		GapINR:               round2(gap),
		MonthlySaveINR:       round2(monthlySave),
		MonthsToTarget:       months,
		Rationale:            rationale,
	}
}

func medianMonthlyExpense(txns []finance.Transaction) float64 {
	monthly := map[string]float64{}
	for _, t := range txns {
		if t.AmountCents >= 0 {
			continue
		}
		when, err := t.ParsedDate()
		if err != nil {
			continue
		}
		monthly[when.Format("2006-01")] += float64(-t.AmountCents) / 100
	}
	xs := make([]float64, 0, len(monthly))
	for _, v := range monthly {
		xs = append(xs, v)
	}
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	mid := len(xs) / 2
	if len(xs)%2 == 1 {
		return xs[mid]
	}
	return (xs[mid-1] + xs[mid]) / 2
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
