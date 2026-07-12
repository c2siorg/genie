package catalog

import (
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// OptionsExplainerSpec ports agents/options_explainer. It computes the
// Black-Scholes theoretical price and greeks (delta, gamma, theta-per-day,
// vega-per-1%, rho-per-1%) for a single European equity option, along with the
// breakeven at expiry and a 21-point payoff curve (from 0.7K to 1.3K of strike).
// India equity options are physically settled since Oct 2019 — payoff at expiry
// is max(0, S-K) for calls and max(0, K-S) for puts. Inputs and outputs mirror
// the legacy wire JSON exactly. Outputs are advisory/informational only, per the
// RBI FREE-AI report — not a recommendation to buy, sell, or write any option.
func OptionsExplainerSpec() afg.Spec {
	return afg.Spec{
		ID:   "options_explainer",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type request struct {
				Side             string  `json:"side"` // "call" | "put"
				UnderlyingPrice  float64 `json:"underlying_price"`
				Strike           float64 `json:"strike"`
				DaysToExpiry     int     `json:"days_to_expiry"`
				ImpliedVolPct    float64 `json:"implied_volatility_pct"` // 25 -> 25%
				RiskFreeRatePct  float64 `json:"risk_free_rate_pct"`
				DividendYieldPct float64 `json:"dividend_yield_pct"`
				LotSize          int     `json:"lot_size"`
			}
			type greeks struct {
				Delta float64 `json:"delta"`
				Gamma float64 `json:"gamma"`
				Theta float64 `json:"theta_per_day"`
				Vega  float64 `json:"vega_per_1pct"`
				Rho   float64 `json:"rho_per_1pct"`
			}
			type payoffPoint struct {
				Price float64 `json:"price"`
				PNL   float64 `json:"pnl_per_lot"`
			}
			type result struct {
				TheoreticalPrice  float64       `json:"theoretical_price_per_share"`
				Greeks            greeks        `json:"greeks_per_share"`
				BreakevenAtExpiry float64       `json:"breakeven_at_expiry"`
				PayoffCurve       []payoffPoint `json:"payoff_curve"`
				Narrative         string        `json:"narrative"`
				Disclaimer        string        `json:"disclaimer"`
			}

			// N is the cumulative standard-normal distribution.
			N := func(x float64) float64 {
				return 0.5 * (1 + math.Erf(x/math.Sqrt2))
			}
			// phi is the standard-normal PDF.
			phi := func(x float64) float64 {
				return math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			S := req.UnderlyingPrice
			K := req.Strike
			T := float64(req.DaysToExpiry) / 365.0
			sigma := req.ImpliedVolPct / 100
			rfr := req.RiskFreeRatePct / 100
			q := req.DividendYieldPct / 100

			var out result
			if T <= 0 || sigma <= 0 || S <= 0 || K <= 0 {
				out = result{Disclaimer: "Inputs must be positive — cannot price."}
				body, err := json.Marshal(out)
				if err != nil {
					return "", err
				}
				return string(body), nil
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
			payoff := make([]payoffPoint, 0, 21)
			for i := 0; i <= 20; i++ {
				p := K * (0.7 + 0.03*float64(i))
				pnl := 0.0
				if req.Side == "call" {
					pnl = (math.Max(0, p-K) - price) * lot
				} else {
					pnl = (math.Max(0, K-p) - price) * lot
				}
				payoff = append(payoff, payoffPoint{Price: round2(p), PNL: round2(pnl)})
			}
			be := K + price
			if req.Side == "put" {
				be = K - price
			}
			narrative := "Long " + req.Side + " — bought premium; max loss = premium, theta works against you."

			out = result{
				TheoreticalPrice: round2(price),
				Greeks: greeks{
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
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
