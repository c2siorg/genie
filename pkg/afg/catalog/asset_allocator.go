package catalog

import (
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// AssetAllocatorSpec ports agents/asset_allocator.
//
// It recommends a target equity / debt / gold / cash allocation given the
// user's age, risk tolerance, and horizon. Heuristic-first: it starts from the
// "100 - age" equity rule, then adjusts for risk tolerance (+/- 15%) and horizon
// (>= 10y bumps equity by 5%), clamps equity to [0.10, 0.85], and splits the
// remainder 70% debt / 15% gold / 15% cash. The rebalance recommendation diffs
// the target against the current snapshot and emits Buy / Sell / Hold rupee
// amounts. Deterministic arithmetic; outputs are advisory per RBI FREE-AI.
func AssetAllocatorSpec() afg.Spec {
	return afg.Spec{
		ID:   "asset_allocator",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type current struct {
				EquityINR float64 `json:"equity_rupees"`
				DebtINR   float64 `json:"debt_rupees"`
				GoldINR   float64 `json:"gold_rupees"`
				CashINR   float64 `json:"cash_rupees"`
			}
			type request struct {
				Age           int     `json:"age"`
				HorizonYears  int     `json:"horizon_years"`
				RiskTolerance string  `json:"risk_tolerance"`
				Current       current `json:"current"`
			}
			type allocation struct {
				Equity float64 `json:"equity"`
				Debt   float64 `json:"debt"`
				Gold   float64 `json:"gold"`
				Cash   float64 `json:"cash"`
			}
			type rebalance struct {
				Asset    string  `json:"asset"`
				DeltaINR float64 `json:"delta_rupees"` // +ve = buy, -ve = sell
				Action   string  `json:"action"`
			}
			type plan struct {
				Target       allocation  `json:"target_allocation"`
				CurrentAlloc allocation  `json:"current_allocation"`
				Rebalance    []rebalance `json:"rebalance"`
				Rationale    string      `json:"rationale"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
			makeRebalance := func(asset string, delta float64) rebalance {
				r := rebalance{Asset: asset, DeltaINR: round2(delta)}
				switch {
				case delta > 100:
					r.Action = "buy"
				case delta < -100:
					r.Action = "sell"
				default:
					r.Action = "hold"
				}
				return r
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			equity := math.Max(0, math.Min(1, float64(100-req.Age)/100))
			switch req.RiskTolerance {
			case "conservative":
				equity -= 0.15
			case "aggressive":
				equity += 0.15
			}
			if req.HorizonYears >= 10 {
				equity += 0.05
			}
			equity = math.Max(0.1, math.Min(0.85, equity))

			// Remaining 1-equity split: 70% debt, 15% gold, 15% cash by default.
			rem := 1 - equity
			target := allocation{
				Equity: round4(equity),
				Debt:   round4(rem * 0.70),
				Gold:   round4(rem * 0.15),
				Cash:   round4(rem * 0.15),
			}

			total := req.Current.EquityINR + req.Current.DebtINR + req.Current.GoldINR + req.Current.CashINR
			var cur allocation
			if total > 0 {
				cur = allocation{
					Equity: round4(req.Current.EquityINR / total),
					Debt:   round4(req.Current.DebtINR / total),
					Gold:   round4(req.Current.GoldINR / total),
					Cash:   round4(req.Current.CashINR / total),
				}
			}

			reb := []rebalance{
				makeRebalance("equity", target.Equity*total-req.Current.EquityINR),
				makeRebalance("debt", target.Debt*total-req.Current.DebtINR),
				makeRebalance("gold", target.Gold*total-req.Current.GoldINR),
				makeRebalance("cash", target.Cash*total-req.Current.CashINR),
			}

			out := plan{
				Target:       target,
				CurrentAlloc: cur,
				Rebalance:    reb,
				Rationale: "Starts from 100-age equity rule, then adjusts for risk tolerance and horizon. " +
					"Within debt sleeve: split between G-Sec gilt funds + corporate bond funds; review annually.",
			}

			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
