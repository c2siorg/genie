package catalog

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// CashflowUnderwriterSpec ports agents/cashflow_underwriter. It scores a
// borrower's creditworthiness from their bank transaction history alone — the
// "alt-data" lane for thin-file customers (RBI FREE-AI Rec 4), with no bureau
// pull. Six signals are each normalised to 0..100 — inflow_stability and
// expense_volatility (coefficient of variation of monthly inflows/outflows),
// savings_rate ((inflow−outflow)/inflow), debt_burden (recurring EMI/SIP
// outflows as a fraction of inflow), bounce_rate (share of debits flagged as
// NACH/ECS returns), and tenure (months of history) — then combined as a
// weighted average and mapped onto a CIBIL-comparable 300–900 band with an
// A/B/C/D grade. The logic is pure arithmetic and fully reproducible from the
// signals (the explainability artefact, Rec 25), so it is ported as a
// deterministic Spec. It is High risk because it feeds credit decisioning.
func CashflowUnderwriterSpec() afg.Spec {
	return afg.Spec{
		ID:   "cashflow_underwriter",
		Risk: "high",
		Handle: func(input string) (string, error) {
			// Score band (CIBIL-style for at-a-glance).
			const minScore = 300.0
			const maxScore = 900.0

			// Weights — must sum to 1.
			weights := map[string]float64{
				"inflow_stability":   0.25,
				"savings_rate":       0.25,
				"debt_burden":        0.20,
				"bounce_rate":        0.15,
				"expense_volatility": 0.10,
				"tenure":             0.05,
			}

			type transaction struct {
				Date        string `json:"date"` // ISO-8601 YYYY-MM-DD
				AmountCents int64  `json:"amount_cents"`
				Description string `json:"description"`
				Category    string `json:"category,omitempty"`
			}
			type request struct {
				Transactions []transaction `json:"transactions"`
			}
			type signal struct {
				Name       string  `json:"name"`
				Raw        float64 `json:"raw"`
				Normalised float64 `json:"normalised_0_100"`
				Weight     float64 `json:"weight"`
				Reason     string  `json:"reason"`
			}
			type result struct {
				Score       float64  `json:"score_300_900"`
				Grade       string   `json:"grade"` // A/B/C/D
				Signals     []signal `json:"signals"`
				MonthsCover int      `json:"months_of_history"`
				Disclaimer  string   `json:"disclaimer"`
			}

			parsedDate := func(d string) (time.Time, error) {
				return time.Parse("2006-01-02", d)
			}

			// looksLikeBounce inspects the description for typical NACH/ECS return strings.
			looksLikeBounce := func(desc string) bool {
				d := strings.ToLower(desc)
				for _, tok := range []string{"return", "reversal", "rev:", "ach_return", "nft_return", "i/w cheque return", "ecs ret"} {
					if strings.Contains(d, tok) {
						return true
					}
				}
				return false
			}

			// looksLikeRecurring detects EMIs/SIPs without an explicit flag.
			looksLikeRecurring := func(desc, cat string) bool {
				d := strings.ToLower(desc + " " + cat)
				for _, tok := range []string{"emi", "sip", "loan", "mortgage", "auto-debit", "nach", "ecs"} {
					if strings.Contains(d, tok) {
						return true
					}
				}
				return false
			}

			monthsBetween := func(a, b time.Time) int {
				if a.IsZero() || b.IsZero() {
					return 0
				}
				y, m := b.Year()-a.Year(), int(b.Month())-int(a.Month())
				total := y*12 + m + 1
				if total < 1 {
					return 1
				}
				return total
			}

			values := func(m map[string]int64) []float64 {
				out := make([]float64, 0, len(m))
				for _, v := range m {
					out = append(out, float64(v))
				}
				return out
			}

			mean := func(xs []float64) float64 {
				if len(xs) == 0 {
					return 0
				}
				var sum float64
				for _, x := range xs {
					sum += x
				}
				return sum / float64(len(xs))
			}

			cv := func(xs []float64) float64 {
				if len(xs) < 2 {
					return 0
				}
				m := mean(xs)
				if m == 0 {
					return 0
				}
				var sumSq float64
				for _, x := range xs {
					d := x - m
					sumSq += d * d
				}
				std := math.Sqrt(sumSq / float64(len(xs)))
				return std / m
			}

			// normInverse maps "lower is better" onto 0..100. Above cap → 0; at 0 → 100.
			normInverse := func(x, capVal float64) float64 {
				if capVal <= 0 {
					return 0
				}
				if x <= 0 {
					return 100
				}
				if x >= capVal {
					return 0
				}
				return (1 - x/capVal) * 100
			}

			// normClamp01 maps a 0..1 input onto 0..100, clamped.
			normClamp01 := func(x float64) float64 {
				if x < 0 {
					return 0
				}
				if x > 1 {
					return 100
				}
				return x * 100
			}

			round1 := func(x float64) float64 {
				return float64(int64(x*10+0.5)) / 10
			}

			gradeFor := func(score float64) string {
				switch {
				case score >= 800:
					return "A"
				case score >= 700:
					return "B"
				case score >= 600:
					return "C"
				default:
					return "D"
				}
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			txns := req.Transactions

			res := result{
				Disclaimer: "Cashflow-based underwriting signal. Not a CIBIL score. " +
					"For lending decisions combine with KYC + bureau where required.",
			}
			if len(txns) == 0 {
				res.Score = minScore
				res.Grade = "D"
				body, err := json.Marshal(res)
				if err != nil {
					return "", err
				}
				return string(body), nil
			}

			// Bucket by YYYY-MM.
			monthlyIn := map[string]int64{}
			monthlyOut := map[string]int64{}
			totalBounces := 0
			totalDebits := 0
			totalRecurringOut := int64(0)
			earliest, latest := time.Time{}, time.Time{}
			for _, t := range txns {
				when, err := parsedDate(t.Date)
				if err != nil {
					continue
				}
				if earliest.IsZero() || when.Before(earliest) {
					earliest = when
				}
				if latest.IsZero() || when.After(latest) {
					latest = when
				}
				month := when.Format("2006-01")
				if t.AmountCents > 0 {
					monthlyIn[month] += t.AmountCents
				} else if t.AmountCents < 0 {
					amt := -t.AmountCents
					monthlyOut[month] += amt
					totalDebits++
					if looksLikeBounce(t.Description) {
						totalBounces++
					}
					if looksLikeRecurring(t.Description, t.Category) {
						totalRecurringOut += amt
					}
				}
			}
			monthsCover := monthsBetween(earliest, latest)
			res.MonthsCover = monthsCover

			inflowStabilityCV := cv(values(monthlyIn))
			inflowMean := mean(values(monthlyIn))
			outflowMean := mean(values(monthlyOut))
			expenseVolatilityCV := cv(values(monthlyOut))

			savingsRate := 0.0
			if inflowMean > 0 {
				savingsRate = (inflowMean - outflowMean) / inflowMean
			}
			debtBurden := 0.0
			if inflowMean > 0 && monthsCover > 0 {
				debtBurden = float64(totalRecurringOut) / float64(monthsCover) / inflowMean
			}
			bounceRate := 0.0
			if totalDebits > 0 {
				bounceRate = float64(totalBounces) / float64(totalDebits)
			}

			signals := []signal{
				{Name: "inflow_stability", Raw: inflowStabilityCV,
					Normalised: normInverse(inflowStabilityCV, 1.0),
					Weight:     weights["inflow_stability"],
					Reason:     "Lower CV of monthly inflows is better. CV≥1 ⇒ 0."},
				{Name: "savings_rate", Raw: savingsRate,
					Normalised: normClamp01(savingsRate),
					Weight:     weights["savings_rate"],
					Reason:     "(inflow − outflow) / inflow, clamped 0..1."},
				{Name: "debt_burden", Raw: debtBurden,
					Normalised: normInverse(debtBurden, 0.6),
					Weight:     weights["debt_burden"],
					Reason:     "Recurring EMI/SIP outflows as fraction of inflow. ≥60 % ⇒ 0."},
				{Name: "bounce_rate", Raw: bounceRate,
					Normalised: normInverse(bounceRate, 0.1),
					Weight:     weights["bounce_rate"],
					Reason:     "Share of debits flagged as bounce. ≥10 % ⇒ 0."},
				{Name: "expense_volatility", Raw: expenseVolatilityCV,
					Normalised: normInverse(expenseVolatilityCV, 1.0),
					Weight:     weights["expense_volatility"],
					Reason:     "Lower CV of monthly outflows is better."},
				{Name: "tenure", Raw: float64(monthsCover),
					Normalised: normClamp01(float64(monthsCover) / 12.0),
					Weight:     weights["tenure"],
					Reason:     "Months of history present; 12 ⇒ 100."},
			}
			sort.Slice(signals, func(i, j int) bool { return signals[i].Name < signals[j].Name })

			var weighted float64
			for _, s := range signals {
				weighted += s.Weight * s.Normalised // 0..100
			}
			score := minScore + (weighted/100.0)*(maxScore-minScore)
			res.Signals = signals
			res.Score = round1(score)
			res.Grade = gradeFor(score)

			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
