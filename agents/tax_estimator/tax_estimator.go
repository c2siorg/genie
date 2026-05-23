// Package tax_estimator computes a back-of-envelope India income tax
// estimate under the new tax regime (FY 2024-25 slabs). Educational; the
// recommender treats this as informational only.
package tax_estimator

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "tax_estimator"
	Capability = "estimate_tax"
	TypeIn     = "tax_estimate_request"
	TypeOut    = "tax_estimate"
	NextAgent  = "financial_supervisor"
)

// Request is the input payload.
type Request struct {
	GrossIncomeRupees float64 `json:"gross_income_rupees"`
	Regime            string  `json:"regime"` // "new" (default) | "old"
}

// Response is the output payload.
type Response struct {
	GrossIncomeRupees float64 `json:"gross_income_rupees"`
	Regime            string  `json:"regime"`
	TaxRupees         float64 `json:"tax_rupees"`
	EffectiveRatePct  float64 `json:"effective_rate_pct"`
	Rationale         string  `json:"rationale"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Tax Estimator (India)" }
func (a *Agent) Capabilities() []string { return []string{Capability} }

// RiskLevel — informational only, medium risk because users might rely on it.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	if req.GrossIncomeRupees < 0 {
		return nil, errors.New("tax_estimator: gross income cannot be negative")
	}
	if req.Regime == "" {
		req.Regime = "new"
	}
	tax, rationale := newRegime(req.GrossIncomeRupees)
	if req.Regime == "old" {
		tax, rationale = oldRegime(req.GrossIncomeRupees)
	}
	resp := Response{
		GrossIncomeRupees: req.GrossIncomeRupees,
		Regime:            req.Regime,
		TaxRupees:         tax,
		EffectiveRatePct:  effectiveRate(req.GrossIncomeRupees, tax),
		Rationale:         rationale,
	}
	body, _ := json.Marshal(resp)
	env.Logf("[tax_estimator] income=%.0f regime=%s tax=%.0f", req.GrossIncomeRupees, req.Regime, tax)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// newRegime — India FY 2024-25 (AY 2025-26) new tax regime slabs.
// 0-3L: 0%, 3-6L: 5%, 6-9L: 10%, 9-12L: 15%, 12-15L: 20%, >15L: 30%.
// Includes 4% cess; rebate u/s 87A for income <= ₹7L gives effective 0 tax.
func newRegime(income float64) (float64, string) {
	if income <= 700_000 {
		return 0, "Income within ₹7L; full rebate under sec 87A (new regime)."
	}
	bands := []struct {
		upper float64
		rate  float64
	}{
		{300_000, 0.00},
		{600_000, 0.05},
		{900_000, 0.10},
		{1_200_000, 0.15},
		{1_500_000, 0.20},
	}
	var tax float64
	var prev float64
	for _, b := range bands {
		if income > b.upper {
			tax += (b.upper - prev) * b.rate
			prev = b.upper
		} else {
			tax += (income - prev) * b.rate
			prev = income
			break
		}
	}
	if income > 1_500_000 {
		tax += (income - 1_500_000) * 0.30
	}
	tax *= 1.04 // 4% cess
	return tax, "Computed using new-regime slabs (FY 2024-25) plus 4% cess; no deductions."
}

// oldRegime — abbreviated old regime; assumes basic ₹2.5L exemption.
func oldRegime(income float64) (float64, string) {
	bands := []struct {
		upper float64
		rate  float64
	}{
		{250_000, 0.00},
		{500_000, 0.05},
		{1_000_000, 0.20},
	}
	var tax float64
	var prev float64
	for _, b := range bands {
		if income > b.upper {
			tax += (b.upper - prev) * b.rate
			prev = b.upper
		} else {
			tax += (income - prev) * b.rate
			prev = income
			break
		}
	}
	if income > 1_000_000 {
		tax += (income - 1_000_000) * 0.30
	}
	tax *= 1.04
	return tax, "Computed using old-regime slabs with ₹2.5L basic exemption; no Chapter VI-A deductions assumed."
}

func effectiveRate(income, tax float64) float64 {
	if income == 0 {
		return 0
	}
	return (tax / income) * 100
}
