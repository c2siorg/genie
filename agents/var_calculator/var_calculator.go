// Package var_calculator computes Value-at-Risk for a portfolio of
// returns. Two methods supported:
//
//   * Historical — VaR is the p-th percentile of the historical returns
//   * Parametric — assumes normal returns; VaR = -(μ + zα × σ) × value
//
// Expected Shortfall (ES / CVaR) is included for both methods. Outputs
// rupee VaR and percent VaR at the requested confidence level.
package var_calculator

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "var_calculator"
	Capability = "compute_var"
	TypeIn     = "var_request"
	TypeOut    = "var_result"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload.
type Request struct {
	Returns        []float64 `json:"daily_returns"` // decimal
	PortfolioValue float64   `json:"portfolio_value_rupees"`
	ConfidencePct  float64   `json:"confidence_pct"` // e.g. 99
	HorizonDays    int       `json:"horizon_days"`
}

// Method captures one VaR computation method's output.
type Method struct {
	VaRPct  float64 `json:"var_pct"`
	VaRINR  float64 `json:"var_rupees"`
	ESPct   float64 `json:"es_pct"`
	ESINR   float64 `json:"es_rupees"`
}

// Result is the wire output.
type Result struct {
	Historical Method `json:"historical"`
	Parametric Method `json:"parametric"`
	Note       string `json:"note"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "VaR Calculator" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Compute(req)
	env.Logf("[var_calculator] hist=%.0f para=%.0f", res.Historical.VaRINR, res.Parametric.VaRINR)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute runs both methods.
func (a *Agent) Compute(req Request) Result {
	if len(req.Returns) == 0 {
		return Result{Note: "No returns supplied — cannot compute VaR."}
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

	// Historical
	sorted := make([]float64, len(req.Returns))
	copy(sorted, req.Returns)
	sort.Float64s(sorted)
	// Empirical α-quantile of returns. For N=100, c=99% we want the worst
	// 1% — sorted[floor(N×α - ε)] (clamped). Use a small epsilon to
	// dodge floating-point precision e.g. 100×0.01=1.0000000000000009.
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
	hist := Method{
		VaRPct: round4(histVaR * 100),
		VaRINR: round2(histVaR * req.PortfolioValue),
		ESPct:  round4(histES * 100),
		ESINR:  round2(histES * req.PortfolioValue),
	}

	// Parametric: z-score for one tail.
	mean, sd := meanStd(req.Returns)
	z := zScore(conf / 100)
	paraVaR := (z*sd - mean) * sqrtH
	// ES under normal: φ(z) / (1-c) × σ
	es := (phi(z)/(1-conf/100))*sd*sqrtH - mean*sqrtH
	para := Method{
		VaRPct: round4(paraVaR * 100),
		VaRINR: round2(paraVaR * req.PortfolioValue),
		ESPct:  round4(es * 100),
		ESINR:  round2(es * req.PortfolioValue),
	}

	return Result{
		Historical: hist, Parametric: para,
		Note: "Historical uses empirical percentile; parametric assumes normal returns. Compare both — if they diverge significantly, returns are non-normal (fat-tailed).",
	}
}

func meanStd(xs []float64) (float64, float64) {
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

// zScore approximates the one-tail inverse normal for common confidence.
// For confidence 0.95→1.645, 0.99→2.326. Use a piecewise table for
// clarity; production code can swap in a beta-based inverse normal.
func zScore(p float64) float64 {
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

func phi(x float64) float64 {
	return math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
