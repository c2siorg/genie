package catalog

import (
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// DebtOptimizerSpec ports agents/debt_optimizer. It ranks the user's outstanding
// debts by repayment priority under either the "avalanche" (highest interest
// first, lowest total cost) or "snowball" (smallest balance first, fastest
// psychological wins) strategy, then simulates the payoff month-by-month —
// accruing monthly interest, paying minimums, and routing any extra payment to
// the top-priority debt — to produce the payoff order, months to freedom, and
// total interest paid.
func DebtOptimizerSpec() afg.Spec {
	return afg.Spec{
		ID:   "debt_optimizer",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				strategyAvalanche = "avalanche"
				strategySnowball  = "snowball"
				maxSimMonths      = 600 // 50yr safety cap
			)

			// Debt is one outstanding obligation.
			type Debt struct {
				Name             string  `json:"name"`
				BalanceRupees    float64 `json:"balance_rupees"`
				APR              float64 `json:"apr"` // annual rate, decimal (0.12 = 12%)
				MinPaymentRupees float64 `json:"min_payment_rupees"`
			}
			// Request is the wire payload.
			type Request struct {
				Debts         []Debt  `json:"debts"`
				Strategy      string  `json:"strategy"` // "avalanche" (default) | "snowball"
				ExtraPerMonth float64 `json:"extra_per_month_rupees"`
			}
			// Plan is the wire output.
			type Plan struct {
				Strategy      string   `json:"strategy"`
				Order         []string `json:"order"` // debt names, in payoff order
				MonthsToFree  int      `json:"months_to_freedom"`
				TotalInterest float64  `json:"total_interest_rupees"`
				Disclaimer    string   `json:"disclaimer"`
			}

			minF := func(a, b float64) float64 {
				if a < b {
					return a
				}
				return b
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			contains := func(xs []string, s string) bool {
				for _, x := range xs {
					if x == s {
						return true
					}
				}
				return false
			}
			sortByStrategy := func(d []Debt, strategy string) {
				switch strategy {
				case strategySnowball:
					sort.SliceStable(d, func(i, j int) bool { return d[i].BalanceRupees < d[j].BalanceRupees })
				default: // avalanche
					sort.SliceStable(d, func(i, j int) bool { return d[i].APR > d[j].APR })
				}
			}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			if req.Strategy == "" {
				req.Strategy = strategyAvalanche
			}
			// Copy so we don't mutate caller's slice.
			debts := make([]Debt, len(req.Debts))
			copy(debts, req.Debts)

			order := []string{}
			totalInterest := 0.0
			months := 0

			for months < maxSimMonths && len(debts) > 0 {
				months++
				extra := req.ExtraPerMonth
				// Prioritisation order is recomputed each month so paid-off debts drop.
				sortByStrategy(debts, req.Strategy)
				// Accrue monthly interest.
				for i := range debts {
					interest := debts[i].BalanceRupees * debts[i].APR / 12.0
					debts[i].BalanceRupees += interest
					totalInterest += interest
				}
				// Pay minimums.
				for i := range debts {
					pay := minF(debts[i].MinPaymentRupees, debts[i].BalanceRupees)
					debts[i].BalanceRupees -= pay
				}
				// Route extra to top priority.
				if extra > 0 && len(debts) > 0 {
					pay := minF(extra, debts[0].BalanceRupees)
					debts[0].BalanceRupees -= pay
				}
				// Drop paid-off.
				remaining := debts[:0]
				for _, d := range debts {
					if d.BalanceRupees > 0.005 {
						remaining = append(remaining, d)
					} else if !contains(order, d.Name) {
						order = append(order, d.Name)
					}
				}
				debts = remaining
			}

			plan := Plan{
				Strategy:      req.Strategy,
				Order:         order,
				MonthsToFree:  months,
				TotalInterest: round2(totalInterest),
				Disclaimer: "Informational simulation. Actual rates may compound differently. " +
					"Consult your lender before changing EMI obligations.",
			}

			body, err := json.Marshal(plan)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
