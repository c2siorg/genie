// Package prepayment_advisor ranks the user's loans by how much interest
// a one-time prepayment of a given amount would save. Indian context:
//   - Home loans enjoy 80C principal + 24(b) interest deductions, so the
//     effective post-tax rate is lower than coupon APR.
//   - Floating-rate retail loans have no foreclosure penalty (RBI master
//     direction). Fixed-rate loans typically do — surfaced as a flag.
//   - We do not compute the borrower's slab; the recommender adds the
//     "tax-adjusted" narrative when the user supplies a slab.
package prepayment_advisor

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "prepayment_advisor"
	Capability = "advise_prepayment"
	TypeIn     = "prepayment_request"
	TypeOut    = "prepayment_plan"
	NextAgent  = "financial_supervisor"
)

// Loan is one outstanding loan.
type Loan struct {
	Name              string  `json:"name"`
	OutstandingRupees float64 `json:"outstanding_rupees"`
	APR               float64 `json:"apr"` // decimal
	MonthsRemaining   int     `json:"months_remaining"`
	IsFixedRate       bool    `json:"is_fixed_rate"`
	TaxDeductible     bool    `json:"tax_deductible"` // home loan typically true
}

// Request is the wire payload.
type Request struct {
	Loans              []Loan  `json:"loans"`
	PrepaymentAmount   float64 `json:"prepayment_amount_rupees"`
	BorrowerSlabPct    float64 `json:"borrower_slab_pct"` // optional, 0..100
}

// Suggestion is one ranked recommendation.
type Suggestion struct {
	LoanName          string  `json:"loan_name"`
	ApplyAmountRupees float64 `json:"apply_amount_rupees"`
	EffectiveRate     float64 `json:"effective_rate_pct"`
	InterestSavedINR  float64 `json:"interest_saved_rupees"`
	MonthsShortened   int     `json:"months_shortened"`
	Flags             []string `json:"flags,omitempty"`
}

// Plan is the wire output.
type Plan struct {
	Suggestions     []Suggestion `json:"suggestions"`
	TotalSavingINR  float64      `json:"total_saving_rupees"`
	Disclaimer      string       `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Prepayment Advisor" }
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
	env.Logf("[prepayment_advisor] %d suggestions saving %.0f", len(plan.Suggestions), plan.TotalSavingINR)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute ranks loans by effective post-tax APR and computes the savings
// from applying the prepayment to the top-ranked loan first.
func (a *Agent) Compute(req Request) Plan {
	ranked := make([]Loan, 0, len(req.Loans))
	for _, l := range req.Loans {
		if l.OutstandingRupees <= 0 {
			continue
		}
		ranked = append(ranked, l)
	}
	// Sort by effective rate descending.
	sort.SliceStable(ranked, func(i, j int) bool {
		return effectiveRate(ranked[i], req.BorrowerSlabPct) > effectiveRate(ranked[j], req.BorrowerSlabPct)
	})

	remaining := req.PrepaymentAmount
	var plan Plan
	for _, l := range ranked {
		if remaining <= 0 {
			break
		}
		apply := remaining
		if apply > l.OutstandingRupees {
			apply = l.OutstandingRupees
		}
		saved, months := simulateSaving(l, apply)
		flags := []string{}
		if l.IsFixedRate {
			flags = append(flags, "fixed-rate: lender may charge foreclosure penalty (2-5% typically)")
		}
		plan.Suggestions = append(plan.Suggestions, Suggestion{
			LoanName:          l.Name,
			ApplyAmountRupees: round2(apply),
			EffectiveRate:     round2(effectiveRate(l, req.BorrowerSlabPct) * 100),
			InterestSavedINR:  round2(saved),
			MonthsShortened:   months,
			Flags:             flags,
		})
		plan.TotalSavingINR += saved
		remaining -= apply
	}
	plan.TotalSavingINR = round2(plan.TotalSavingINR)
	plan.Disclaimer = "Effective rate adjusts for declared tax deductibility but does not consider opportunity cost. " +
		"Compare against expected return on the alternative investment before prepaying."
	return plan
}

// effectiveRate returns the post-tax effective APR. If a loan is tax
// deductible and the borrower has a slab, the carry cost shrinks.
func effectiveRate(l Loan, slabPct float64) float64 {
	if !l.TaxDeductible || slabPct <= 0 {
		return l.APR
	}
	return l.APR * (1 - slabPct/100)
}

// simulateSaving — compares total interest with and without the prepayment,
// holding EMI constant. Returns interest saved + months saved.
func simulateSaving(l Loan, prepay float64) (float64, int) {
	emi := emiFor(l.OutstandingRupees, l.APR/12, l.MonthsRemaining)
	intWithout := totalInterest(l.OutstandingRupees, l.APR/12, emi, l.MonthsRemaining)
	intWith, mo := totalInterestPrepaid(l.OutstandingRupees-prepay, l.APR/12, emi)
	return intWithout - intWith, l.MonthsRemaining - mo
}

func emiFor(principal, monthlyRate float64, months int) float64 {
	if monthlyRate == 0 {
		return principal / float64(months)
	}
	r := monthlyRate
	n := float64(months)
	return principal * r * math.Pow(1+r, n) / (math.Pow(1+r, n) - 1)
}

func totalInterest(principal, mr, emi float64, months int) float64 {
	var interest float64
	bal := principal
	for i := 0; i < months && bal > 0; i++ {
		intM := bal * mr
		bal = bal + intM - emi
		interest += intM
	}
	return interest
}

func totalInterestPrepaid(principal, mr, emi float64) (float64, int) {
	var interest float64
	bal := principal
	months := 0
	for bal > 0 && months < 600 {
		intM := bal * mr
		bal = bal + intM - emi
		interest += intM
		months++
	}
	return interest, months
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
