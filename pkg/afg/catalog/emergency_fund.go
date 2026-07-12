package catalog

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// EmergencyFundSpec ports agents/emergency_fund. It sizes the gap between the
// user's current liquid reserves and a target emergency fund. The target is the
// median monthly expense (derived from the debit transactions supplied) multiplied
// by a coverage factor: 3 months for stable salaried income with no dependents,
// 6 months for variable income (freelance/commissioned), and 9 months for a sole
// breadwinner with dependents. It then derives the shortfall, a suggested monthly
// savings amount (10% of the median monthly expense), and the number of months to
// reach the target. Outputs are advisory/informational only, per the RBI FREE-AI
// report — not a directive to save or invest.
func EmergencyFundSpec() afg.Spec {
	return afg.Spec{
		ID:   "emergency_fund_advisor",
		Risk: "low",
		Handle: func(input string) (string, error) {
			// Transaction mirrors finance.Transaction: amounts are integer minor
			// units (paise/cents) with credits positive and debits negative.
			type transaction struct {
				Date        string `json:"date"` // ISO-8601 YYYY-MM-DD
				AmountCents int64  `json:"amount_cents"`
			}
			type request struct {
				Transactions      []transaction `json:"transactions"`
				LiquidReservesINR float64       `json:"liquid_reserves_rupees"`
				IncomeProfile     string        `json:"income_profile"` // "stable" | "variable"
				HasDependents     bool          `json:"has_dependents"`
			}
			type plan struct {
				MedianMonthlyExpense float64 `json:"median_monthly_expense_rupees"`
				CoverageMonths       int     `json:"coverage_months"`
				TargetINR            float64 `json:"target_rupees"`
				GapINR               float64 `json:"gap_rupees"`
				MonthlySaveINR       float64 `json:"suggested_monthly_save_rupees"`
				MonthsToTarget       int     `json:"months_to_target"`
				Rationale            string  `json:"rationale"`
			}

			const (
				coverageStable     = 3
				coverageVariable   = 6
				coverageDependents = 9
			)

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			medianMonthlyExpense := func(txns []transaction) float64 {
				monthly := map[string]float64{}
				for _, t := range txns {
					if t.AmountCents >= 0 {
						continue
					}
					when, err := time.Parse("2006-01-02", t.Date)
					if err != nil {
						continue
					}
					monthly[when.Format("2006-01")] += float64(-t.AmountCents) / 100
				}
				xs := make([]float64, 0, len(monthly))
				for _, v := range monthly {
					xs = append(xs, v)
				}
				if len(xs) == 0 {
					return 0
				}
				sort.Float64s(xs)
				mid := len(xs) / 2
				if len(xs)%2 == 1 {
					return xs[mid]
				}
				return (xs[mid-1] + xs[mid]) / 2
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			monthly := medianMonthlyExpense(req.Transactions)
			cov := coverageStable
			rationale := "3 months of expenses — stable salaried income with no dependents."
			switch {
			case req.HasDependents:
				cov = coverageDependents
				rationale = "9 months — sole breadwinner with dependents; longer runway recommended."
			case req.IncomeProfile == "variable":
				cov = coverageVariable
				rationale = "6 months — variable income (freelance / commissioned) needs deeper buffer."
			}
			target := monthly * float64(cov)
			gap := target - req.LiquidReservesINR
			if gap < 0 {
				gap = 0
				rationale += " You're already above the target — consider redirecting incremental savings to long-term goals."
			}
			monthlySave := monthly * 0.10 // suggest 10% of monthly expense as the savings rate
			months := 0
			if monthlySave > 0 && gap > 0 {
				months = int(gap / monthlySave)
				if int(gap)%int(monthlySave) > 0 {
					months++
				}
			}

			out := plan{
				MedianMonthlyExpense: round2(monthly),
				CoverageMonths:       cov,
				TargetINR:            round2(target),
				GapINR:               round2(gap),
				MonthlySaveINR:       round2(monthlySave),
				MonthsToTarget:       months,
				Rationale:            rationale,
			}
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
