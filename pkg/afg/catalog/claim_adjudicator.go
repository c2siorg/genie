package catalog

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// ClaimAdjudicatorSpec ports agents/claim_adjudicator — the bancassurance
// claim-adjudication engine. It applies the insurer-supplied rulebook (data,
// not code) for one product: waiting-period denial, policy-exclusion match on
// diagnosis, network-only-peril gating, then payout math (incurred less
// deductible, less co-pay, capped by peril sub-limit and sum insured). Claims
// >= ₹2L are routed to a human claims officer (HITL). Deterministic port of the
// legacy pure rule engine; faithfully mirrors the legacy input/output JSON.
func ClaimAdjudicatorSpec() afg.Spec {
	return afg.Spec{
		ID:   "claim_adjudicator",
		Risk: "high",
		Handle: func(input string) (string, error) {
			const hitlThresholdRupees = 200_000.0

			// Policy is the insurer-supplied rulebook for one product.
			type Policy struct {
				ProductCode       string             `json:"product_code"`
				WaitingPeriodDays int                `json:"waiting_period_days"`
				SumInsured        float64            `json:"sum_insured"`
				DeductibleRupees  float64            `json:"deductible_rupees"`
				CoPayPct          float64            `json:"copay_pct"` // 0..1
				Exclusions        []string           `json:"exclusions"`
				SubLimits         map[string]float64 `json:"sub_limits"` // peril -> max payout
				NetworkOnlyPerils []string           `json:"network_only_perils"`
			}

			// Claim is the inbound packet.
			type Claim struct {
				ClaimID           string  `json:"claim_id"`
				PolicyCode        string  `json:"policy_code"`
				IncurredRupees    float64 `json:"incurred_rupees"`
				Peril             string  `json:"peril"`
				Diagnosis         string  `json:"diagnosis"`
				DaysSinceIssue    int     `json:"days_since_issue"`
				HospitalInNetwork bool    `json:"hospital_in_network"`
			}

			// Request bundles claim + policy.
			type Request struct {
				Claim  Claim  `json:"claim"`
				Policy Policy `json:"policy"`
			}

			// Decision is the structured output.
			type Decision struct {
				ClaimID      string   `json:"claim_id"`
				Action       string   `json:"action"` // "approve" | "approve_partial" | "deny" | "hitl"
				PayoutRupees float64  `json:"payout_rupees"`
				Reasons      []string `json:"reasons"`
				Disclaimer   string   `json:"disclaimer"`
			}

			stdDisclaimer := func() string {
				return "Adjudication is rule-based per insurer policy. Final settlement subject to documentation, " +
					"investigation, and TAT prescribed by IRDAI Health Insurance Regulations 2016."
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			ftos := func(f float64) string { return strconv.FormatInt(int64(f+0.5), 10) }
			pct := func(p float64) string { return ftos(p*100) + "%" }

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			c, p := req.Claim, req.Policy

			marshal := func(d Decision) (string, error) {
				body, err := json.Marshal(d)
				if err != nil {
					return "", err
				}
				return string(body), nil
			}

			// 1. Waiting period.
			if c.DaysSinceIssue < p.WaitingPeriodDays {
				return marshal(Decision{
					ClaimID:    c.ClaimID,
					Action:     "deny",
					Reasons:    []string{"Claim incurred within waiting period of " + strconv.Itoa(p.WaitingPeriodDays) + " days"},
					Disclaimer: stdDisclaimer(),
				})
			}

			// 2. Exclusions (case-insensitive substring match on diagnosis).
			diag := strings.ToLower(c.Diagnosis)
			for _, ex := range p.Exclusions {
				if ex == "" {
					continue
				}
				if strings.Contains(diag, strings.ToLower(ex)) {
					return marshal(Decision{
						ClaimID:    c.ClaimID,
						Action:     "deny",
						Reasons:    []string{"Diagnosis matches policy exclusion: " + ex},
						Disclaimer: stdDisclaimer(),
					})
				}
			}

			// 3. Network-only perils.
			for _, n := range p.NetworkOnlyPerils {
				if strings.EqualFold(n, c.Peril) && !c.HospitalInNetwork {
					return marshal(Decision{
						ClaimID:    c.ClaimID,
						Action:     "deny",
						Reasons:    []string{"Peril " + c.Peril + " is covered only at network hospitals"},
						Disclaimer: stdDisclaimer(),
					})
				}
			}

			// 4. Compute payout: incurred - deductible, then × (1 - copay),
			// then capped by sub-limit and sum insured.
			reasons := []string{}
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

			return marshal(Decision{
				ClaimID:      c.ClaimID,
				Action:       action,
				PayoutRupees: round2(payable),
				Reasons:      reasons,
				Disclaimer:   stdDisclaimer(),
			})
		},
	}
}
