package catalog

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// InvoiceDiscounterSpec ports agents/invoice_discounter.
//
// It ranks outstanding invoices by the cost of factoring them on TReDS (RBI's
// Trade Receivables Discounting System). Each invoice's discount cost is
// face × annualised_factor_rate × (days_to_maturity/365), scaled up by a
// counterparty credit-risk premium. Invoices are ranked cheapest-first (lowest
// effective APR) and greedily selected until the target working-capital
// infusion is met.
func InvoiceDiscounterSpec() afg.Spec {
	return afg.Spec{
		ID:   "invoice_discounter",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type invoice struct {
				ID                 string  `json:"id"`
				Counterparty       string  `json:"counterparty"`
				CounterpartyRating string  `json:"counterparty_rating"`
				FaceValueRupees    float64 `json:"face_value_rupees"`
				IssuedOn           string  `json:"issued_on"`
				DueOn              string  `json:"due_on"`
			}
			type request struct {
				Invoices         []invoice `json:"invoices"`
				TargetCashINR    float64   `json:"target_cash_rupees"`
				ReferenceAsOf    string    `json:"as_of_date"`
				AnnualisedRateBp float64   `json:"annualised_rate_bp"`
			}
			type choice struct {
				InvoiceID       string  `json:"invoice_id"`
				NetCashINR      float64 `json:"net_cash_rupees"`
				DiscountCostINR float64 `json:"discount_cost_rupees"`
				EffectiveAPR    float64 `json:"effective_apr_pct"`
				DaysToMaturity  int     `json:"days_to_maturity"`
				Rating          string  `json:"rating"`
			}
			type plan struct {
				Selected         []choice `json:"selected"`
				TotalNetCashINR  float64  `json:"total_net_cash_rupees"`
				TotalDiscountINR float64  `json:"total_discount_cost_rupees"`
				UnfundedGapINR   float64  `json:"unfunded_gap_rupees"`
				Note             string   `json:"note"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			applyRatingPremium := func(base float64, rating string) float64 {
				switch rating {
				case "AAA":
					return base
				case "AA":
					return base * 1.10
				case "A":
					return base * 1.25
				case "BBB":
					return base * 1.50
				default:
					return base * 2.0
				}
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			now := time.Now()
			if req.ReferenceAsOf != "" {
				if t, err := time.Parse("2006-01-02", req.ReferenceAsOf); err == nil {
					now = t
				}
			}
			rate := req.AnnualisedRateBp / 10_000.0

			candidates := []choice{}
			for _, inv := range req.Invoices {
				due, err := time.Parse("2006-01-02", inv.DueOn)
				if err != nil {
					continue
				}
				days := int(due.Sub(now).Hours() / 24)
				if days <= 0 {
					continue // past due — not discountable here
				}
				discount := inv.FaceValueRupees * rate * float64(days) / 365.0
				discount = applyRatingPremium(discount, inv.CounterpartyRating)
				netCash := inv.FaceValueRupees - discount
				apr := 0.0
				if netCash > 0 && days > 0 {
					apr = (discount / netCash) * (365.0 / float64(days)) * 100
				}
				candidates = append(candidates, choice{
					InvoiceID:       inv.ID,
					NetCashINR:      round2(netCash),
					DiscountCostINR: round2(discount),
					EffectiveAPR:    round2(apr),
					DaysToMaturity:  days,
					Rating:          inv.CounterpartyRating,
				})
			}

			// Cheapest first (lowest effective APR).
			sort.SliceStable(candidates, func(i, j int) bool {
				return candidates[i].EffectiveAPR < candidates[j].EffectiveAPR
			})

			out := plan{Note: "Greedy selection; for binding decision combine with bank's TReDS auction outcome. Advisory/informational only per RBI FREE-AI."}
			for _, c := range candidates {
				if out.TotalNetCashINR >= req.TargetCashINR {
					break
				}
				out.Selected = append(out.Selected, c)
				out.TotalNetCashINR += c.NetCashINR
				out.TotalDiscountINR += c.DiscountCostINR
			}
			out.TotalNetCashINR = round2(out.TotalNetCashINR)
			out.TotalDiscountINR = round2(out.TotalDiscountINR)
			if out.TotalNetCashINR < req.TargetCashINR {
				out.UnfundedGapINR = round2(req.TargetCashINR - out.TotalNetCashINR)
			}

			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
