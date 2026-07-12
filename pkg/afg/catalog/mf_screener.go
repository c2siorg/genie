package catalog

import (
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// MfScreenerSpec ports agents/mf_screener.
//
// It ranks a list of mutual-fund candidates against user-supplied hard filters
// (category, minimum AUM in crore, maximum expense ratio, minimum 3-year CAGR)
// and then scores the survivors on a composite 0..100 blend: 40% 5-year CAGR
// (normalised, 0.25 cap), 25% Sharpe ratio (excess return over the risk-free
// rate divided by std-dev, 1.5 cap), 20% expense ratio (inverse, 2% cap), and
// 15% consistency (inverse of negative-return quarters in the last 5 years,
// cap 8). Results are sorted best-first. The agent performs no live data fetch —
// it scores what is handed in. Deterministic arithmetic; scores are relative
// within the candidate set and outputs are advisory per RBI FREE-AI.
func MfScreenerSpec() afg.Spec {
	return afg.Spec{
		ID:   "mf_screener",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type fund struct {
				Scheme             string  `json:"scheme"`
				Category           string  `json:"category"`
				NAV                float64 `json:"nav"`
				AUMCr              float64 `json:"aum_crore"`
				ExpenseRatio       float64 `json:"expense_ratio"`
				ThreeYrCAGR        float64 `json:"three_year_cagr"`
				FiveYrCAGR         float64 `json:"five_year_cagr"`
				StdDev             float64 `json:"std_dev"`
				NegativeQuartersL5 int     `json:"neg_quarters_last_5yr"`
			}
			type filter struct {
				Category   string  `json:"category,omitempty"`
				MinAUMCr   float64 `json:"min_aum_crore,omitempty"`
				MaxExpense float64 `json:"max_expense_ratio,omitempty"`
				MinThreeYr float64 `json:"min_three_yr_cagr,omitempty"`
			}
			type request struct {
				Funds  []fund `json:"funds"`
				Filter filter `json:"filter"`
			}
			type ranked struct {
				Scheme string  `json:"scheme"`
				Score  float64 `json:"score_0_100"`
				Sharpe float64 `json:"sharpe"`
				Reason string  `json:"reason"`
			}
			type result struct {
				Ranked      []ranked `json:"ranked"`
				FilteredOut int      `json:"filtered_out"`
				Note        string   `json:"note"`
			}

			const riskFreeRate = 0.07

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			normCap := func(x, cap float64) float64 {
				if cap <= 0 {
					return 0
				}
				if x <= 0 {
					return 0
				}
				if x >= cap {
					return 1
				}
				return x / cap
			}
			normInverse := func(x, cap float64) float64 {
				if cap <= 0 {
					return 0
				}
				if x <= 0 {
					return 1
				}
				if x >= cap {
					return 0
				}
				return 1 - x/cap
			}
			passesFilter := func(f fund, fi filter) bool {
				if fi.Category != "" && f.Category != fi.Category {
					return false
				}
				if fi.MinAUMCr > 0 && f.AUMCr < fi.MinAUMCr {
					return false
				}
				if fi.MaxExpense > 0 && f.ExpenseRatio > fi.MaxExpense {
					return false
				}
				if fi.MinThreeYr > 0 && f.ThreeYrCAGR < fi.MinThreeYr {
					return false
				}
				return true
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			res := result{Note: "Scores are relative within this candidate set. Past returns are not a guarantee of future performance."}
			for _, f := range req.Funds {
				if !passesFilter(f, req.Filter) {
					res.FilteredOut++
					continue
				}
				sharpe := 0.0
				if f.StdDev > 0 {
					sharpe = (f.FiveYrCAGR - riskFreeRate) / f.StdDev
				}
				// Composite 0..100:
				//   40% 5yr CAGR (normalised by 0.25 cap)
				//   25% Sharpe (normalised by 1.5 cap)
				//   20% expense ratio (inverse normalised by 2% cap)
				//   15% consistency (inverse of negative-quarter count, cap 8)
				c := 0.40*normCap(f.FiveYrCAGR, 0.25) +
					0.25*normCap(sharpe, 1.5) +
					0.20*normInverse(f.ExpenseRatio, 0.02) +
					0.15*normInverse(float64(f.NegativeQuartersL5), 8)
				res.Ranked = append(res.Ranked, ranked{
					Scheme: f.Scheme,
					Score:  round2(c * 100),
					Sharpe: round2(sharpe),
					Reason: "Composite of 5y CAGR (40 %), Sharpe (25 %), expense (20 %), consistency (15 %).",
				})
			}
			sort.SliceStable(res.Ranked, func(i, j int) bool {
				return res.Ranked[i].Score > res.Ranked[j].Score
			})

			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
