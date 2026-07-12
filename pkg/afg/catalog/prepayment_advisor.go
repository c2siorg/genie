package catalog

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// PrepaymentAdvisorSpec ports agents/prepayment_advisor. It ranks the user's
// outstanding loans by their effective post-tax APR and simulates applying a
// one-time prepayment to the highest-rate loan first (waterfall), holding the
// EMI constant. For a tax-deductible loan (a home loan typically qualifies for
// 80C principal and 24(b) interest deductions) with a declared borrower slab,
// the effective carry cost shrinks by the slab fraction. Floating-rate retail
// loans carry no foreclosure penalty per the RBI master direction; fixed-rate
// loans are flagged because the lender may levy a 2-5% foreclosure penalty. For
// each suggestion it reports the amount applied, the effective rate, the
// interest saved and the months shortened, plus a total saving figure. Outputs
// are advisory/informational only per the RBI FREE-AI report and do not account
// for the opportunity cost of the alternative investment.
func PrepaymentAdvisorSpec() afg.Spec {
	return afg.Spec{
		ID:   "prepayment_advisor",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type loan struct {
				Name              string  `json:"name"`
				OutstandingRupees float64 `json:"outstanding_rupees"`
				APR               float64 `json:"apr"` // decimal
				MonthsRemaining   int     `json:"months_remaining"`
				IsFixedRate       bool    `json:"is_fixed_rate"`
				TaxDeductible     bool    `json:"tax_deductible"`
			}
			type request struct {
				Loans            []loan  `json:"loans"`
				PrepaymentAmount float64 `json:"prepayment_amount_rupees"`
				BorrowerSlabPct  float64 `json:"borrower_slab_pct"` // optional, 0..100
			}
			type suggestion struct {
				LoanName          string   `json:"loan_name"`
				ApplyAmountRupees float64  `json:"apply_amount_rupees"`
				EffectiveRate     float64  `json:"effective_rate_pct"`
				InterestSavedINR  float64  `json:"interest_saved_rupees"`
				MonthsShortened   int      `json:"months_shortened"`
				Flags             []string `json:"flags,omitempty"`
			}
			type plan struct {
				Suggestions    []suggestion `json:"suggestions"`
				TotalSavingINR float64      `json:"total_saving_rupees"`
				Disclaimer     string       `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			// effectiveRate returns the post-tax effective APR. If a loan is tax
			// deductible and the borrower has a slab, the carry cost shrinks.
			effectiveRate := func(l loan, slabPct float64) float64 {
				if !l.TaxDeductible || slabPct <= 0 {
					return l.APR
				}
				return l.APR * (1 - slabPct/100)
			}

			emiFor := func(principal, monthlyRate float64, months int) float64 {
				if monthlyRate == 0 {
					return principal / float64(months)
				}
				r := monthlyRate
				n := float64(months)
				return principal * r * math.Pow(1+r, n) / (math.Pow(1+r, n) - 1)
			}

			totalInterest := func(principal, mr, emi float64, months int) float64 {
				var interest float64
				bal := principal
				for i := 0; i < months && bal > 0; i++ {
					intM := bal * mr
					bal = bal + intM - emi
					interest += intM
				}
				return interest
			}

			totalInterestPrepaid := func(principal, mr, emi float64) (float64, int) {
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

			// simulateSaving compares total interest with and without the
			// prepayment, holding EMI constant. Returns interest saved + months saved.
			simulateSaving := func(l loan, prepay float64) (float64, int) {
				emi := emiFor(l.OutstandingRupees, l.APR/12, l.MonthsRemaining)
				intWithout := totalInterest(l.OutstandingRupees, l.APR/12, emi, l.MonthsRemaining)
				intWith, mo := totalInterestPrepaid(l.OutstandingRupees-prepay, l.APR/12, emi)
				return intWithout - intWith, l.MonthsRemaining - mo
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			ranked := make([]loan, 0, len(req.Loans))
			for _, l := range req.Loans {
				if l.OutstandingRupees <= 0 {
					continue
				}
				ranked = append(ranked, l)
			}
			sort.SliceStable(ranked, func(i, j int) bool {
				return effectiveRate(ranked[i], req.BorrowerSlabPct) > effectiveRate(ranked[j], req.BorrowerSlabPct)
			})

			remaining := req.PrepaymentAmount
			var out plan
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
				out.Suggestions = append(out.Suggestions, suggestion{
					LoanName:          l.Name,
					ApplyAmountRupees: round2(apply),
					EffectiveRate:     round2(effectiveRate(l, req.BorrowerSlabPct) * 100),
					InterestSavedINR:  round2(saved),
					MonthsShortened:   months,
					Flags:             flags,
				})
				out.TotalSavingINR += saved
				remaining -= apply
			}
			out.TotalSavingINR = round2(out.TotalSavingINR)
			out.Disclaimer = "Effective rate adjusts for declared tax deductibility but does not consider opportunity cost. " +
				"Compare against expected return on the alternative investment before prepaying."

			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
