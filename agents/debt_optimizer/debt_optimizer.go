// Package debt_optimizer ranks the user's outstanding debts by repayment
// priority under either the "avalanche" (highest interest first, lowest
// total cost) or "snowball" (smallest balance first, fastest psychological
// wins) strategy. Outputs a month-by-month payoff schedule that downstream
// reporters can render as a chart or written plan.
package debt_optimizer

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "debt_optimizer"
	Capability = "optimise_debt_payoff"
	TypeIn     = "debt_payoff_request"
	TypeOut    = "debt_payoff_plan"
	NextAgent  = "financial_supervisor"

	StrategyAvalanche = "avalanche"
	StrategySnowball  = "snowball"

	MaxSimMonths = 600 // 50yr safety cap
)

// Debt is one outstanding obligation.
type Debt struct {
	Name              string  `json:"name"`
	BalanceRupees     float64 `json:"balance_rupees"`
	APR               float64 `json:"apr"`           // annual rate, decimal (0.12 = 12%)
	MinPaymentRupees  float64 `json:"min_payment_rupees"`
}

// Request is the wire payload.
type Request struct {
	Debts           []Debt  `json:"debts"`
	Strategy        string  `json:"strategy"` // "avalanche" (default) | "snowball"
	ExtraPerMonth   float64 `json:"extra_per_month_rupees"`
}

// Plan is the wire output.
type Plan struct {
	Strategy       string             `json:"strategy"`
	Order          []string           `json:"order"`           // debt names, in payoff order
	MonthsToFree   int                `json:"months_to_freedom"`
	TotalInterest  float64            `json:"total_interest_rupees"`
	Disclaimer     string             `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Debt Optimizer" }
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
	plan := a.Compute(req)
	env.Logf("[debt_optimizer] strategy=%s months=%d interest=%.0f", plan.Strategy, plan.MonthsToFree, plan.TotalInterest)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute simulates the payoff month-by-month, applying the strategy to
// route any extra payment toward the highest-priority debt.
func (a *Agent) Compute(req Request) Plan {
	if req.Strategy == "" {
		req.Strategy = StrategyAvalanche
	}
	// Copy so we don't mutate caller's slice.
	debts := make([]Debt, len(req.Debts))
	copy(debts, req.Debts)

	order := []string{}
	totalInterest := 0.0
	months := 0

	for months < MaxSimMonths && len(debts) > 0 {
		months++
		// Accrue monthly interest then pay minimums + apply extra to priority.
		extra := req.ExtraPerMonth
		// Prioritisation order is recomputed each month so paid-off debts drop.
		sortByStrategy(debts, req.Strategy)
		for i := range debts {
			interest := debts[i].BalanceRupees * debts[i].APR / 12.0
			debts[i].BalanceRupees += interest
			totalInterest += interest
		}
		// Pay minimums.
		for i := range debts {
			pay := minF(debts[i].MinPaymentRupees, debts[i].BalanceRupees)
			debts[i].BalanceRupees -= pay
		}
		// Route extra to top priority.
		if extra > 0 && len(debts) > 0 {
			pay := minF(extra, debts[0].BalanceRupees)
			debts[0].BalanceRupees -= pay
		}
		// Drop paid-off.
		remaining := debts[:0]
		for _, d := range debts {
			if d.BalanceRupees > 0.005 {
				remaining = append(remaining, d)
			} else if !contains(order, d.Name) {
				order = append(order, d.Name)
			}
		}
		debts = remaining
	}
	return Plan{
		Strategy:      req.Strategy,
		Order:         order,
		MonthsToFree:  months,
		TotalInterest: round2(totalInterest),
		Disclaimer: "Informational simulation. Actual rates may compound differently. " +
			"Consult your lender before changing EMI obligations.",
	}
}

func sortByStrategy(d []Debt, strat string) {
	switch strat {
	case StrategySnowball:
		sort.SliceStable(d, func(i, j int) bool { return d[i].BalanceRupees < d[j].BalanceRupees })
	default: // avalanche
		sort.SliceStable(d, func(i, j int) bool { return d[i].APR > d[j].APR })
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
