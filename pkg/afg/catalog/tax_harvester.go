package catalog

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// TaxHarvesterSpec ports agents/tax_harvester. It identifies Indian equity
// holdings whose unrealised losses can be sold to offset realised gains in the
// same financial year (FY 2024-25 onwards): equity STCG taxed @20%, LTCG @12.5%
// above a ₹1.25L exemption; STCL offsets STCG then LTCG, LTCL offsets LTCG only.
// It ranks losers by loss size, sizes each against the available offset budget,
// computes tax saved, and surfaces a cosmetic wash-sale (30-day re-buy) warning.
// Deterministic; outputs are advisory/informational per RBI FREE-AI, not tax
// advice — consult a chartered accountant.
func TaxHarvesterSpec() afg.Spec {
	return afg.Spec{
		ID:   "tax_harvester",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				stcgRate         = 0.20
				ltcgRate         = 0.125
				ltcgExemptRupees = 1_25_000.0
				ltcgHoldingDays  = 365
			)

			// Holding is one equity line item in the user's portfolio.
			type Holding struct {
				Symbol         string  `json:"symbol"`
				Quantity       float64 `json:"quantity"`
				CostBasisRupee float64 `json:"cost_basis_rupees"` // total, not per-unit
				CurrentPrice   float64 `json:"current_price_rupees"`
				PurchaseDate   string  `json:"purchase_date"` // YYYY-MM-DD
			}
			// RealisedGain represents booked gains so far this FY.
			type RealisedGain struct {
				ShortTermRupees float64 `json:"short_term_rupees"`
				LongTermRupees  float64 `json:"long_term_rupees"`
			}
			// Request is the input payload.
			type Request struct {
				Holdings  []Holding    `json:"holdings"`
				Realised  RealisedGain `json:"realised_gains"`
				AsOfDate  string       `json:"as_of_date"`
				HorizonFY string       `json:"horizon_fy"`
			}
			// Opportunity is one harvesting suggestion.
			type Opportunity struct {
				Symbol            string  `json:"symbol"`
				Quantity          float64 `json:"quantity"`
				HoldingDays       int     `json:"holding_days"`
				GainCategory      string  `json:"gain_category"` // "STCL" | "LTCL"
				UnrealisedLossINR float64 `json:"unrealised_loss_rupees"`
				TaxSavedINR       float64 `json:"tax_saved_rupees"`
				Rationale         string  `json:"rationale"`
				WashSaleWarning   string  `json:"wash_sale_warning,omitempty"`
			}
			// Plan is the output payload.
			type Plan struct {
				TotalTaxSavedINR float64       `json:"total_tax_saved_rupees"`
				Opportunities    []Opportunity `json:"opportunities"`
				UnusedSTCLBudget float64       `json:"unused_stcl_offset_rupees"`
				UnusedLTCLBudget float64       `json:"unused_ltcl_offset_rupees"`
				Disclaimer       string        `json:"disclaimer"`
			}

			max0 := func(x float64) float64 {
				if x < 0 {
					return 0
				}
				return x
			}
			minF := func(a, b float64) float64 {
				if a < b {
					return a
				}
				return b
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			holdingDays := func(purchase string, asOf time.Time) int {
				p, err := time.Parse("2006-01-02", purchase)
				if err != nil {
					return 0
				}
				return int(asOf.Sub(p).Hours() / 24)
			}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			asOf := time.Now().UTC()
			if req.AsOfDate != "" {
				if t, err := time.Parse("2006-01-02", req.AsOfDate); err == nil {
					asOf = t
				}
			}

			// Initial offset budget: STCL can wipe STCG + (LTCG above exemption);
			// LTCL can wipe LTCG above exemption only.
			taxableLTCG := max0(req.Realised.LongTermRupees - ltcgExemptRupees)
			stclBudget := req.Realised.ShortTermRupees + taxableLTCG
			ltclBudget := taxableLTCG

			candidates := []Opportunity{}
			for _, h := range req.Holdings {
				loss := h.CostBasisRupee - h.Quantity*h.CurrentPrice
				if loss <= 0 {
					continue // only losers harvestable
				}
				days := holdingDays(h.PurchaseDate, asOf)
				cat := "STCL"
				if days >= ltcgHoldingDays {
					cat = "LTCL"
				}
				candidates = append(candidates, Opportunity{
					Symbol:            h.Symbol,
					Quantity:          h.Quantity,
					HoldingDays:       days,
					GainCategory:      cat,
					UnrealisedLossINR: loss,
				})
			}

			// Rank by loss size — biggest first soaks the most budget.
			sort.SliceStable(candidates, func(i, j int) bool {
				return candidates[i].UnrealisedLossINR > candidates[j].UnrealisedLossINR
			})

			var plan Plan
			for _, c := range candidates {
				var used, saved float64
				switch c.GainCategory {
				case "STCL":
					used = minF(c.UnrealisedLossINR, stclBudget)
					stclBudget -= used
					// STCL first soaks STCG (@20%), then LTCG (@12.5%).
					stOffset := minF(used, req.Realised.ShortTermRupees)
					req.Realised.ShortTermRupees -= stOffset
					ltOffset := used - stOffset
					saved = stOffset*stcgRate + ltOffset*ltcgRate
					c.Rationale = "Short-term loss; offsets STCG @20% before LTCG @12.5%."
				case "LTCL":
					used = minF(c.UnrealisedLossINR, ltclBudget)
					ltclBudget -= used
					saved = used * ltcgRate
					c.Rationale = "Long-term loss; offsets LTCG above ₹1.25L exemption @12.5%."
				}
				if used <= 0 {
					continue
				}
				c.TaxSavedINR = round2(saved)
				c.UnrealisedLossINR = round2(c.UnrealisedLossINR)
				c.WashSaleWarning = "If you re-buy " + c.Symbol +
					" within 30 days the harvested loss may be challenged. India has no formal wash-sale rule but the IT Dept can disallow cosmetic transactions."
				plan.Opportunities = append(plan.Opportunities, c)
				plan.TotalTaxSavedINR += saved
			}

			plan.TotalTaxSavedINR = round2(plan.TotalTaxSavedINR)
			plan.UnusedSTCLBudget = round2(stclBudget)
			plan.UnusedLTCLBudget = round2(ltclBudget)
			plan.Disclaimer = "Informational; not tax advice. Consult a chartered accountant. " +
				"Slabs and rates per FY 2024-25 Union Budget."

			body, err := json.Marshal(plan)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
