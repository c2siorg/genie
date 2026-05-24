// Package asset_allocator recommends a target equity / debt / gold / cash
// allocation given the user's age, risk tolerance, and horizon.
//
// Heuristic-first: starts from the "100 − age" equity rule then adjusts
// for risk tolerance (±15 %) and horizon (≥10y bumps equity by 5 %). The
// rebalance recommendation diffs the target against the current snapshot
// and outputs Buy / Sell rupee amounts.
package asset_allocator

import (
	"context"
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "asset_allocator"
	Capability = "allocate_assets"
	TypeIn     = "asset_allocation_request"
	TypeOut    = "asset_allocation_plan"
	NextAgent  = "financial_supervisor"

	RiskConservative = "conservative"
	RiskModerate     = "moderate"
	RiskAggressive   = "aggressive"
)

// Current snapshot.
type Current struct {
	EquityINR float64 `json:"equity_rupees"`
	DebtINR   float64 `json:"debt_rupees"`
	GoldINR   float64 `json:"gold_rupees"`
	CashINR   float64 `json:"cash_rupees"`
}

// Request is the wire payload.
type Request struct {
	Age           int     `json:"age"`
	HorizonYears  int     `json:"horizon_years"`
	RiskTolerance string  `json:"risk_tolerance"`
	Current       Current `json:"current"`
}

// Allocation is a percentage split summing to 1.
type Allocation struct {
	Equity float64 `json:"equity"`
	Debt   float64 `json:"debt"`
	Gold   float64 `json:"gold"`
	Cash   float64 `json:"cash"`
}

// Rebalance is one Buy/Sell instruction.
type Rebalance struct {
	Asset     string  `json:"asset"`
	DeltaINR  float64 `json:"delta_rupees"` // +ve = buy, -ve = sell
	Action    string  `json:"action"`
}

// Plan is the wire output.
type Plan struct {
	Target       Allocation  `json:"target_allocation"`
	CurrentAlloc Allocation  `json:"current_allocation"`
	Rebalance    []Rebalance `json:"rebalance"`
	Rationale    string      `json:"rationale"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Asset Allocator" }
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
	env.Logf("[asset_allocator] target equity=%.0f%%", plan.Target.Equity*100)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute returns the target allocation + rebalance steps.
func (a *Agent) Compute(req Request) Plan {
	equity := math.Max(0, math.Min(1, float64(100-req.Age)/100))
	switch req.RiskTolerance {
	case RiskConservative:
		equity -= 0.15
	case RiskAggressive:
		equity += 0.15
	}
	if req.HorizonYears >= 10 {
		equity += 0.05
	}
	equity = math.Max(0.1, math.Min(0.85, equity))

	// Remaining 1-equity split: 70 % debt, 15 % gold, 15 % cash by default.
	rem := 1 - equity
	target := Allocation{
		Equity: round4(equity),
		Debt:   round4(rem * 0.70),
		Gold:   round4(rem * 0.15),
		Cash:   round4(rem * 0.15),
	}
	total := req.Current.EquityINR + req.Current.DebtINR + req.Current.GoldINR + req.Current.CashINR
	var cur Allocation
	if total > 0 {
		cur = Allocation{
			Equity: round4(req.Current.EquityINR / total),
			Debt:   round4(req.Current.DebtINR / total),
			Gold:   round4(req.Current.GoldINR / total),
			Cash:   round4(req.Current.CashINR / total),
		}
	}
	rebalance := []Rebalance{
		makeRebalance("equity", target.Equity*total-req.Current.EquityINR),
		makeRebalance("debt", target.Debt*total-req.Current.DebtINR),
		makeRebalance("gold", target.Gold*total-req.Current.GoldINR),
		makeRebalance("cash", target.Cash*total-req.Current.CashINR),
	}
	return Plan{
		Target:       target,
		CurrentAlloc: cur,
		Rebalance:    rebalance,
		Rationale: "Starts from 100-age equity rule, then adjusts for risk tolerance and horizon. " +
			"Within debt sleeve: split between G-Sec gilt funds + corporate bond funds; review annually.",
	}
}

func makeRebalance(asset string, delta float64) Rebalance {
	r := Rebalance{Asset: asset, DeltaINR: round2(delta)}
	switch {
	case delta > 100:
		r.Action = "buy"
	case delta < -100:
		r.Action = "sell"
	default:
		r.Action = "hold"
	}
	return r
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
