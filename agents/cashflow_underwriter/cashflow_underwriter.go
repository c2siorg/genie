// Package cashflow_underwriter scores a borrower's creditworthiness from
// their bank transaction history alone — no bureau pull. This is the
// "alt-data" lane the RBI FREE-AI report (Rec 4) and CIBIL-light NBFCs
// are betting on for thin-file customers.
//
// Six signals, each normalised to 0..100, then a weighted average:
//
//   inflow_stability   — coefficient of variation of monthly inflows
//   savings_rate       — (inflow − outflow) / inflow
//   debt_burden        — recurring EMI/SIP outflows / inflow
//   bounce_rate        — share of debits that bounced (ECS/NACH returns)
//   expense_volatility — coefficient of variation of monthly outflows
//   tenure             — months of history present in the data
//
// Output is mapped to a 300–900 score (CIBIL-comparable scale) so a human
// underwriter can compare at a glance — but the breakdown is the
// explainability artefact (Rec 25): every score should be reproducible
// from the signals.
package cashflow_underwriter

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "cashflow_underwriter"
	Capability = "underwrite_cashflow"
	TypeIn     = "underwrite_request"
	TypeOut    = "underwrite_score"
	NextAgent  = "financial_supervisor"

	// Score band (CIBIL-style for at-a-glance).
	MinScore = 300.0
	MaxScore = 900.0
)

// Request is the input payload.
type Request struct {
	Transactions []finance.Transaction `json:"transactions"`
}

// Signal is one component of the score with its raw value + normalised 0..100.
type Signal struct {
	Name       string  `json:"name"`
	Raw        float64 `json:"raw"`
	Normalised float64 `json:"normalised_0_100"`
	Weight     float64 `json:"weight"`
	Reason     string  `json:"reason"`
}

// Result is the message payload.
type Result struct {
	Score       float64  `json:"score_300_900"`
	Grade       string   `json:"grade"`            // A/B/C/D
	Signals     []Signal `json:"signals"`
	MonthsCover int      `json:"months_of_history"`
	Disclaimer  string   `json:"disclaimer"`
}

// Weights — must sum to 1. Exposed so calibration can be done in tests
// without touching package state.
var Weights = map[string]float64{
	"inflow_stability":   0.25,
	"savings_rate":       0.25,
	"debt_burden":        0.20,
	"bounce_rate":        0.15,
	"expense_volatility": 0.10,
	"tenure":             0.05,
}

// Agent implements agent.Agent.
type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Cashflow Underwriter" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh } // credit decisioning

// HandleMessage runs Score and emits the result.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Score(req.Transactions)
	env.Logf("[cashflow_underwriter] score=%.0f grade=%s months=%d",
		res.Score, res.Grade, res.MonthsCover)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Score computes the six signals + final mapped score for a transaction set.
func (a *Agent) Score(txns []finance.Transaction) Result {
	res := Result{
		Disclaimer: "Cashflow-based underwriting signal. Not a CIBIL score. " +
			"For lending decisions combine with KYC + bureau where required.",
	}
	if len(txns) == 0 {
		res.Score = MinScore
		res.Grade = "D"
		return res
	}

	// Bucket by YYYY-MM.
	monthlyIn := map[string]int64{}
	monthlyOut := map[string]int64{}
	totalBounces := 0
	totalDebits := 0
	totalRecurringOut := int64(0)
	earliest, latest := time.Time{}, time.Time{}
	for _, t := range txns {
		when, err := t.ParsedDate()
		if err != nil {
			continue
		}
		if earliest.IsZero() || when.Before(earliest) {
			earliest = when
		}
		if latest.IsZero() || when.After(latest) {
			latest = when
		}
		month := when.Format("2006-01")
		if t.AmountCents > 0 {
			monthlyIn[month] += t.AmountCents
		} else if t.AmountCents < 0 {
			amt := -t.AmountCents
			monthlyOut[month] += amt
			totalDebits++
			if looksLikeBounce(t.Description) {
				totalBounces++
			}
			if looksLikeRecurring(t.Description, t.Category) {
				totalRecurringOut += amt
			}
		}
	}
	monthsCover := monthsBetween(earliest, latest)
	res.MonthsCover = monthsCover

	inflowStabilityCV := cv(values(monthlyIn))
	inflowMean := mean(values(monthlyIn))
	outflowMean := mean(values(monthlyOut))
	expenseVolatilityCV := cv(values(monthlyOut))

	savingsRate := 0.0
	if inflowMean > 0 {
		savingsRate = (inflowMean - outflowMean) / inflowMean
	}
	debtBurden := 0.0
	if inflowMean > 0 && monthsCover > 0 {
		debtBurden = float64(totalRecurringOut) / float64(monthsCover) / inflowMean
	}
	bounceRate := 0.0
	if totalDebits > 0 {
		bounceRate = float64(totalBounces) / float64(totalDebits)
	}

	signals := []Signal{
		{Name: "inflow_stability", Raw: inflowStabilityCV,
			Normalised: normInverse(inflowStabilityCV, 1.0),
			Weight:     Weights["inflow_stability"],
			Reason:     "Lower CV of monthly inflows is better. CV≥1 ⇒ 0."},
		{Name: "savings_rate", Raw: savingsRate,
			Normalised: normClamp01(savingsRate),
			Weight:     Weights["savings_rate"],
			Reason:     "(inflow − outflow) / inflow, clamped 0..1."},
		{Name: "debt_burden", Raw: debtBurden,
			Normalised: normInverse(debtBurden, 0.6),
			Weight:     Weights["debt_burden"],
			Reason:     "Recurring EMI/SIP outflows as fraction of inflow. ≥60 % ⇒ 0."},
		{Name: "bounce_rate", Raw: bounceRate,
			Normalised: normInverse(bounceRate, 0.1),
			Weight:     Weights["bounce_rate"],
			Reason:     "Share of debits flagged as bounce. ≥10 % ⇒ 0."},
		{Name: "expense_volatility", Raw: expenseVolatilityCV,
			Normalised: normInverse(expenseVolatilityCV, 1.0),
			Weight:     Weights["expense_volatility"],
			Reason:     "Lower CV of monthly outflows is better."},
		{Name: "tenure", Raw: float64(monthsCover),
			Normalised: normClamp01(float64(monthsCover) / 12.0),
			Weight:     Weights["tenure"],
			Reason:     "Months of history present; 12 ⇒ 100."},
	}
	sort.Slice(signals, func(i, j int) bool { return signals[i].Name < signals[j].Name })

	var weighted float64
	for _, s := range signals {
		weighted += s.Weight * s.Normalised // 0..100
	}
	score := MinScore + (weighted/100.0)*(MaxScore-MinScore)
	res.Signals = signals
	res.Score = round1(score)
	res.Grade = gradeFor(score)
	return res
}

// looksLikeBounce inspects the description for typical NACH/ECS return strings.
func looksLikeBounce(desc string) bool {
	d := strings.ToLower(desc)
	for _, tok := range []string{"return", "reversal", "rev:", "ach_return", "nft_return", "i/w cheque return", "ecs ret"} {
		if strings.Contains(d, tok) {
			return true
		}
	}
	return false
}

// looksLikeRecurring detects EMIs/SIPs without an explicit flag.
func looksLikeRecurring(desc, cat string) bool {
	d := strings.ToLower(desc + " " + cat)
	for _, tok := range []string{"emi", "sip", "loan", "mortgage", "auto-debit", "nach", "ecs"} {
		if strings.Contains(d, tok) {
			return true
		}
	}
	return false
}

func monthsBetween(a, b time.Time) int {
	if a.IsZero() || b.IsZero() {
		return 0
	}
	y, m := b.Year()-a.Year(), int(b.Month())-int(a.Month())
	total := y*12 + m + 1
	if total < 1 {
		return 1
	}
	return total
}

func values(m map[string]int64) []float64 {
	out := make([]float64, 0, len(m))
	for _, v := range m {
		out = append(out, float64(v))
	}
	return out
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func cv(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	if m == 0 {
		return 0
	}
	var sumSq float64
	for _, x := range xs {
		d := x - m
		sumSq += d * d
	}
	std := math.Sqrt(sumSq / float64(len(xs)))
	return std / m
}

// normInverse maps "lower is better" onto 0..100. Above cap → 0; at 0 → 100.
func normInverse(x, cap float64) float64 {
	if cap <= 0 {
		return 0
	}
	if x <= 0 {
		return 100
	}
	if x >= cap {
		return 0
	}
	return (1 - x/cap) * 100
}

// normClamp01 maps a 0..1 input onto 0..100, clamped.
func normClamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 100
	}
	return x * 100
}

func round1(x float64) float64 {
	return float64(int64(x*10+0.5)) / 10
}

func gradeFor(score float64) string {
	switch {
	case score >= 800:
		return "A"
	case score >= 700:
		return "B"
	case score >= 600:
		return "C"
	default:
		return "D"
	}
}
