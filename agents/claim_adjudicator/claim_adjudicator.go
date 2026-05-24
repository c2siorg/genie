// Package claim_adjudicator adjudicates bancassurance insurance claims
// against insurer-supplied policy rules. The rules themselves are data,
// not code — a board-policy-as-YAML style fits insurance product config
// (each insurer ships their schedule of exclusions, sub-limits, waiting
// periods).
//
// Inspired by Google ADK samples → claim-adjudication-agent. Tuned for
// the Indian bancassurance flow where the bank is corporate agent for
// 2-3 insurers and customers expect a single in-app claims experience.
package claim_adjudicator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "claim_adjudicator"
	Capability = "adjudicate_claim"
	TypeIn     = "claim_request"
	TypeOut    = "claim_decision"
	NextAgent  = "financial_supervisor"

	hitlThresholdRupees = 200_000.0
)

// Policy is the insurer-supplied rulebook for one product.
type Policy struct {
	ProductCode         string   `json:"product_code"`
	WaitingPeriodDays   int      `json:"waiting_period_days"`
	SumInsured          float64  `json:"sum_insured"`
	DeductibleRupees    float64  `json:"deductible_rupees"`
	CoPayPct            float64  `json:"copay_pct"`           // 0..1
	Exclusions          []string `json:"exclusions"`          // lowercased keywords
	SubLimits           map[string]float64 `json:"sub_limits"` // peril -> max payout
	NetworkOnlyPerils   []string `json:"network_only_perils"`
}

// Claim is the inbound packet.
type Claim struct {
	ClaimID         string  `json:"claim_id"`
	PolicyCode      string  `json:"policy_code"`
	IncurredRupees  float64 `json:"incurred_rupees"`
	Peril           string  `json:"peril"`           // e.g. "hospitalization", "theft"
	Diagnosis       string  `json:"diagnosis"`
	DaysSinceIssue  int     `json:"days_since_issue"`
	HospitalInNetwork bool  `json:"hospital_in_network"`
}

// Request bundles claim + policy.
type Request struct {
	Claim  Claim  `json:"claim"`
	Policy Policy `json:"policy"`
}

// Decision is the structured output.
type Decision struct {
	ClaimID      string   `json:"claim_id"`
	Action       string   `json:"action"`  // "approve" | "approve_partial" | "deny" | "hitl"
	PayoutRupees float64  `json:"payout_rupees"`
	Reasons      []string `json:"reasons"`
	Disclaimer   string   `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Claim Adjudicator" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	d := a.Adjudicate(req.Claim, req.Policy)
	env.Logf("[claim_adjudicator] claim=%s action=%s payout=%.0f", d.ClaimID, d.Action, d.PayoutRupees)
	body, _ := json.Marshal(d)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Adjudicate is the pure rule engine.
func (a *Agent) Adjudicate(c Claim, p Policy) Decision {
	reasons := []string{}

	// 1. Waiting period.
	if c.DaysSinceIssue < p.WaitingPeriodDays {
		return Decision{
			ClaimID:    c.ClaimID,
			Action:     "deny",
			Reasons:    []string{"Claim incurred within waiting period of " + itoa(p.WaitingPeriodDays) + " days"},
			Disclaimer: stdDisclaimer(),
		}
	}

	// 2. Exclusions (case-insensitive substring match on diagnosis).
	diag := strings.ToLower(c.Diagnosis)
	for _, ex := range p.Exclusions {
		if ex == "" {
			continue
		}
		if strings.Contains(diag, strings.ToLower(ex)) {
			return Decision{
				ClaimID:    c.ClaimID,
				Action:     "deny",
				Reasons:    []string{"Diagnosis matches policy exclusion: " + ex},
				Disclaimer: stdDisclaimer(),
			}
		}
	}

	// 3. Network-only perils.
	for _, n := range p.NetworkOnlyPerils {
		if strings.EqualFold(n, c.Peril) && !c.HospitalInNetwork {
			return Decision{
				ClaimID:    c.ClaimID,
				Action:     "deny",
				Reasons:    []string{"Peril " + c.Peril + " is covered only at network hospitals"},
				Disclaimer: stdDisclaimer(),
			}
		}
	}

	// 4. Compute payout: incurred - deductible, then × (1 - copay), then capped by sub-limit and sum insured.
	payable := c.IncurredRupees - p.DeductibleRupees
	if payable < 0 {
		payable = 0
		reasons = append(reasons, "Incurred amount below deductible")
	}
	if p.CoPayPct > 0 {
		coPay := payable * p.CoPayPct
		payable -= coPay
		reasons = append(reasons, "Co-pay applied at "+pct(p.CoPayPct))
	}
	if sub, ok := p.SubLimits[c.Peril]; ok && sub > 0 && payable > sub {
		payable = sub
		reasons = append(reasons, "Capped by peril sub-limit ₹"+ftos(sub))
	}
	if p.SumInsured > 0 && payable > p.SumInsured {
		payable = p.SumInsured
		reasons = append(reasons, "Capped by sum insured ₹"+ftos(p.SumInsured))
	}

	action := "approve"
	if payable < c.IncurredRupees {
		action = "approve_partial"
	}
	if c.IncurredRupees >= hitlThresholdRupees {
		action = "hitl"
		reasons = append(reasons, "Claim ≥ ₹2L — routed to claims officer for review")
	}

	return Decision{
		ClaimID:      c.ClaimID,
		Action:       action,
		PayoutRupees: round2(payable),
		Reasons:      reasons,
		Disclaimer:   stdDisclaimer(),
	}
}

func stdDisclaimer() string {
	return "Adjudication is rule-based per insurer policy. Final settlement subject to documentation, " +
		"investigation, and TAT prescribed by IRDAI Health Insurance Regulations 2016."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func ftos(f float64) string {
	// rupee amount, no decimals
	n := int64(f + 0.5)
	return itoa64(n)
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func pct(p float64) string {
	return ftos(p*100) + "%"
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
