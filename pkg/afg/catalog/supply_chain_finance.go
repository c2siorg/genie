package catalog

import (
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// SupplyChainFinanceSpec ports agents/supply_chain_finance.
//
// It takes a supplier's outstanding receivables and builds a supply-chain view:
// it aggregates invoices per buyer (total rupees and worst days-outstanding),
// computes each buyer's share of total receivables, and flags concentration risk
// when the top buyer's share exceeds 40%. It then selects TReDS (Trade
// Receivables Discounting System) candidates — GST e-invoiced receivables aged
// beyond 45 days (the MSMED Act obligation) — sorted oldest-first, and derives
// next-step guidance. Deterministic arithmetic mirroring the legacy SCF
// recommender; outputs are indicative/advisory per RBI FREE-AI and final TReDS
// auction yields are set at clearing based on buyer credit rating.
func SupplyChainFinanceSpec() afg.Spec {
	return afg.Spec{
		ID:   "supply_chain_finance",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				concentrationWarnPct = 0.40
				tredsThresholdDays   = 45
			)

			type invoice struct {
				InvoiceID       string  `json:"invoice_id"`
				BuyerID         string  `json:"buyer_id"`
				BuyerRating     string  `json:"buyer_rating"`
				AmountRupees    float64 `json:"amount_rupees"`
				DaysOutstanding int     `json:"days_outstanding"`
				GSTeInvoiced    bool    `json:"gst_e_invoiced"`
			}
			type request struct {
				SupplierID string    `json:"supplier_id"`
				Invoices   []invoice `json:"invoices"`
			}
			type buyerSlice struct {
				BuyerID     string  `json:"buyer_id"`
				TotalRupees float64 `json:"total_rupees"`
				SharePct    float64 `json:"share_pct"`
				WorstDays   int     `json:"worst_days_outstanding"`
				BuyerRating string  `json:"buyer_rating"`
			}
			type recommendation struct {
				SupplierID       string       `json:"supplier_id"`
				TotalReceivables float64      `json:"total_receivables_rupees"`
				ConcentrationOK  bool         `json:"concentration_ok"`
				WarnReasons      []string     `json:"warn_reasons"`
				BuyerSlices      []buyerSlice `json:"buyer_slices"`
				TREDSCandidates  []invoice    `json:"treds_candidates"`
				NextSteps        []string     `json:"next_steps"`
				Disclaimer       string       `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			ftos := func(f float64) string {
				n := int64(f + 0.5)
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
			pct := func(p float64) string { return ftos(p*100) + "%" }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			total := 0.0
			byBuyer := map[string]*buyerSlice{}
			for _, inv := range req.Invoices {
				total += inv.AmountRupees
				s, ok := byBuyer[inv.BuyerID]
				if !ok {
					s = &buyerSlice{BuyerID: inv.BuyerID, BuyerRating: inv.BuyerRating}
					byBuyer[inv.BuyerID] = s
				}
				s.TotalRupees += inv.AmountRupees
				if inv.DaysOutstanding > s.WorstDays {
					s.WorstDays = inv.DaysOutstanding
				}
			}

			slices := make([]buyerSlice, 0, len(byBuyer))
			for _, s := range byBuyer {
				if total > 0 {
					s.SharePct = round2((s.TotalRupees / total) * 100)
				}
				slices = append(slices, *s)
			}
			sort.Slice(slices, func(i, j int) bool { return slices[i].TotalRupees > slices[j].TotalRupees })

			warn := []string{}
			concentrationOK := true
			if len(slices) > 0 && total > 0 {
				topShare := slices[0].TotalRupees / total
				if topShare > concentrationWarnPct {
					concentrationOK = false
					warn = append(warn, "Top buyer share above "+pct(concentrationWarnPct)+" — concentration risk")
				}
			}

			candidates := []invoice{}
			for _, inv := range req.Invoices {
				if inv.GSTeInvoiced && inv.DaysOutstanding > tredsThresholdDays {
					candidates = append(candidates, inv)
				}
			}
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].DaysOutstanding > candidates[j].DaysOutstanding
			})

			steps := []string{}
			if len(candidates) > 0 {
				steps = append(steps, "Submit aged e-invoiced receivables to a TReDS platform for discounting")
			}
			if !concentrationOK {
				steps = append(steps,
					"Diversify buyer mix or insure top-buyer receivables to manage concentration risk")
			}
			if len(steps) == 0 {
				steps = []string{"Chain looks healthy; no immediate SCF action recommended"}
			}

			res := recommendation{
				SupplierID:       req.SupplierID,
				TotalReceivables: round2(total),
				ConcentrationOK:  concentrationOK,
				WarnReasons:      warn,
				BuyerSlices:      slices,
				TREDSCandidates:  candidates,
				NextSteps:        steps,
				Disclaimer: "Indicative supply-chain view. TReDS auction discount rates depend on buyer " +
					"credit rating and the platform's live auction; final yield is set at clearing.",
			}

			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
