package catalog

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// VarCalculatorSpec ports agents/var_calculator (registered as "var_calculator").
//
// The legacy agent computes Value-at-Risk (VaR) and Expected Shortfall
// (ES / CVaR) for a portfolio of daily returns using two methods:
//
//   - Historical — VaR is the empirical alpha-quantile of the sorted historical
//     returns; ES is the mean of the returns in the tail worse than the cutoff.
//   - Parametric — assumes normally distributed returns; VaR = (z*sigma - mu),
//     and ES = phi(z)/(1-c)*sigma - mu, using a piecewise inverse-normal table.
//
// Both methods are scaled to the requested horizon by sqrt(horizon_days) and
// reported as a percent VaR/ES and a rupee VaR/ES against the portfolio value.
// Because the logic is cleanly deterministic (sorting, percentile lookup and
// closed-form arithmetic), it is ported as a deterministic Spec. The legacy
// agent declares RiskLevel() = RiskHigh, so risk is high. Outputs are advisory
// and informational only, in line with the RBI FREE-AI report.
func VarCalculatorSpec() afg.Spec {
	return afg.Spec{
		ID:   "var_calculator",
		Risk: "high",
		Handle: func(input string) (string, error) {
			type request struct {
				Returns        []float64 `json:"daily_returns"` // decimal
				PortfolioValue float64   `json:"portfolio_value_rupees"`
				ConfidencePct  float64   `json:"confidence_pct"` // e.g. 99
				HorizonDays    int       `json:"horizon_days"`
			}
			type method struct {
				VaRPct float64 `json:"var_pct"`
				VaRINR float64 `json:"var_rupees"`
				ESPct  float64 `json:"es_pct"`
				ESINR  float64 `json:"es_rupees"`
			}
			type result struct {
				Historical method `json:"historical"`
				Parametric method `json:"parametric"`
				Note       string `json:"note"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

			meanStd := func(xs []float64) (float64, float64) {
				if len(xs) == 0 {
					return 0, 0
				}
				var sum float64
				for _, x := range xs {
					sum += x
				}
				mean := sum / float64(len(xs))
				var sumSq float64
				for _, x := range xs {
					d := x - mean
					sumSq += d * d
				}
				return mean, math.Sqrt(sumSq / float64(len(xs)))
			}

			// zScore approximates the one-tail inverse normal for common
			// confidence levels via a piecewise table (0.95->1.645, 0.99->2.326).
			zScore := func(p float64) float64 {
				switch {
				case p >= 0.999:
					return 3.090
				case p >= 0.995:
					return 2.576
				case p >= 0.99:
					return 2.326
				case p >= 0.975:
					return 1.960
				case p >= 0.95:
					return 1.645
				case p >= 0.90:
					return 1.282
				default:
					return 0
				}
			}

			phi := func(x float64) float64 {
				return math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			if len(req.Returns) == 0 {
				res := result{Note: "No returns supplied — cannot compute VaR."}
				body, err := json.Marshal(res)
				if err != nil {
					return "", err
				}
				return string(body), nil
			}

			conf := req.ConfidencePct
			if conf <= 0 {
				conf = 95
			}
			h := req.HorizonDays
			if h <= 0 {
				h = 1
			}
			sqrtH := math.Sqrt(float64(h))

			// Historical: empirical alpha-quantile of returns. A small epsilon
			// dodges floating-point precision (e.g. 100*0.01 = 1.0000...009).
			sorted := make([]float64, len(req.Returns))
			copy(sorted, req.Returns)
			sort.Float64s(sorted)
			alpha := 1 - conf/100
			idx := int(math.Floor(float64(len(sorted))*alpha - 1e-9))
			if idx >= len(sorted) {
				idx = len(sorted) - 1
			}
			if idx < 0 {
				idx = 0
			}
			histVaR := -sorted[idx] * sqrtH
			// ES: mean of returns worse than the cutoff.
			var sumTail float64
			tailN := idx + 1
			for i := 0; i <= idx; i++ {
				sumTail += sorted[i]
			}
			histES := -(sumTail / float64(tailN)) * sqrtH
			hist := method{
				VaRPct: round4(histVaR * 100),
				VaRINR: round2(histVaR * req.PortfolioValue),
				ESPct:  round4(histES * 100),
				ESINR:  round2(histES * req.PortfolioValue),
			}

			// Parametric: z-score for one tail; normal-distribution ES.
			mean, sd := meanStd(req.Returns)
			z := zScore(conf / 100)
			paraVaR := (z*sd - mean) * sqrtH
			es := (phi(z)/(1-conf/100))*sd*sqrtH - mean*sqrtH
			para := method{
				VaRPct: round4(paraVaR * 100),
				VaRINR: round2(paraVaR * req.PortfolioValue),
				ESPct:  round4(es * 100),
				ESINR:  round2(es * req.PortfolioValue),
			}

			res := result{
				Historical: hist,
				Parametric: para,
				Note:       "Historical uses empirical percentile; parametric assumes normal returns. Compare both — if they diverge significantly, returns are non-normal (fat-tailed).",
			}
			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
