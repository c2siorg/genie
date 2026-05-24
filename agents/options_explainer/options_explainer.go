// Package options_explainer computes the Black-Scholes greeks (delta,
// gamma, theta, vega, rho) for a European option and a payoff diagram
// across a range of underlying prices at expiry. India equity options
// are physically settled now (since Oct 2019) — payoff at expiry is
// max(0, S-K) for calls; max(0, K-S) for puts.
package options_explainer

import (
	"context"
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "options_explainer"
	Capability = "explain_option"
	TypeIn     = "option_request"
	TypeOut    = "option_explanation"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload.
type Request struct {
	Side             string  `json:"side"` // "call" | "put"
	UnderlyingPrice  float64 `json:"underlying_price"`
	Strike           float64 `json:"strike"`
	DaysToExpiry     int     `json:"days_to_expiry"`
	ImpliedVolPct    float64 `json:"implied_volatility_pct"` // 25 → 25%
	RiskFreeRatePct  float64 `json:"risk_free_rate_pct"`
	DividendYieldPct float64 `json:"dividend_yield_pct"`
	LotSize          int     `json:"lot_size"`
}

// Greeks is the per-share sensitivity bundle.
type Greeks struct {
	Delta float64 `json:"delta"`
	Gamma float64 `json:"gamma"`
	Theta float64 `json:"theta_per_day"`
	Vega  float64 `json:"vega_per_1pct"`
	Rho   float64 `json:"rho_per_1pct"`
}

// PayoffPoint is one (price-at-expiry, P&L-at-expiry) sample.
type PayoffPoint struct {
	Price float64 `json:"price"`
	PNL   float64 `json:"pnl_per_lot"`
}

// Result is the wire output.
type Result struct {
	TheoreticalPrice float64       `json:"theoretical_price_per_share"`
	Greeks           Greeks        `json:"greeks_per_share"`
	BreakevenAtExpiry float64      `json:"breakeven_at_expiry"`
	PayoffCurve      []PayoffPoint `json:"payoff_curve"`
	Narrative        string        `json:"narrative"`
	Disclaimer       string        `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Options Strategy Explainer" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Compute(req)
	env.Logf("[options_explainer] price=%.2f delta=%.2f", res.TheoreticalPrice, res.Greeks.Delta)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute returns price + greeks + payoff curve.
func (a *Agent) Compute(req Request) Result {
	S := req.UnderlyingPrice
	K := req.Strike
	T := float64(req.DaysToExpiry) / 365.0
	sigma := req.ImpliedVolPct / 100
	rfr := req.RiskFreeRatePct / 100
	q := req.DividendYieldPct / 100
	if T <= 0 || sigma <= 0 || S <= 0 || K <= 0 {
		return Result{Disclaimer: "Inputs must be positive — cannot price."}
	}
	d1 := (math.Log(S/K) + (rfr-q+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))
	d2 := d1 - sigma*math.Sqrt(T)

	var price, delta, theta, rho float64
	gamma := math.Exp(-q*T) * phi(d1) / (S * sigma * math.Sqrt(T))
	vega := S * math.Exp(-q*T) * phi(d1) * math.Sqrt(T) / 100 // per 1% sigma

	if req.Side == "put" {
		price = K*math.Exp(-rfr*T)*N(-d2) - S*math.Exp(-q*T)*N(-d1)
		delta = -math.Exp(-q*T) * N(-d1)
		theta = (-S*math.Exp(-q*T)*phi(d1)*sigma/(2*math.Sqrt(T)) +
			rfr*K*math.Exp(-rfr*T)*N(-d2) -
			q*S*math.Exp(-q*T)*N(-d1)) / 365
		rho = -K * T * math.Exp(-rfr*T) * N(-d2) / 100
	} else {
		price = S*math.Exp(-q*T)*N(d1) - K*math.Exp(-rfr*T)*N(d2)
		delta = math.Exp(-q*T) * N(d1)
		theta = (-S*math.Exp(-q*T)*phi(d1)*sigma/(2*math.Sqrt(T)) -
			rfr*K*math.Exp(-rfr*T)*N(d2) +
			q*S*math.Exp(-q*T)*N(d1)) / 365
		rho = K * T * math.Exp(-rfr*T) * N(d2) / 100
	}

	// Payoff curve at expiry, 21 points from 0.7K to 1.3K.
	lot := float64(req.LotSize)
	if lot <= 0 {
		lot = 1
	}
	payoff := make([]PayoffPoint, 0, 21)
	for i := 0; i <= 20; i++ {
		p := K * (0.7 + 0.03*float64(i))
		pnl := 0.0
		if req.Side == "call" {
			pnl = (math.Max(0, p-K) - price) * lot
		} else {
			pnl = (math.Max(0, K-p) - price) * lot
		}
		payoff = append(payoff, PayoffPoint{Price: round2(p), PNL: round2(pnl)})
	}
	be := K + price
	if req.Side == "put" {
		be = K - price
	}
	narrative := "Long " + req.Side + " — bought premium; max loss = premium, theta works against you."
	return Result{
		TheoreticalPrice: round2(price),
		Greeks: Greeks{
			Delta: round4(delta),
			Gamma: round4(gamma),
			Theta: round4(theta),
			Vega:  round4(vega),
			Rho:   round4(rho),
		},
		BreakevenAtExpiry: round2(be),
		PayoffCurve:       payoff,
		Narrative:         narrative,
		Disclaimer:        "Black-Scholes assumes no early exercise (European). Indian equity options are European except weekly Bank Nifty — close to model.",
	}
}

// N is the cumulative normal distribution.
func N(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// phi is the standard-normal PDF.
func phi(x float64) float64 {
	return math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
