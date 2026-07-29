package catalog

import (
	"encoding/json"
	"math"
	"math/rand"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// SipVsLumpsumSpec ports agents/sip_vs_lumpsum.
//
// It compares investing a fixed corpus as a one-shot lumpsum vs. a 12-month SIP
// over a horizon, using a Monte-Carlo lognormal-style return model (monthly
// drift = annual return / 12, monthly sigma = annual volatility / sqrt(12)).
// For each simulated path it compounds the lumpsum and the cumulative SIP
// balance, records terminal values, and counts how often lumpsum beats SIP.
// It reports the p50 terminal value of each strategy, the lumpsum-win
// probability, the expected regret (absolute p50 gap in rupees) and a
// behavioural recommendation keyed on the win-probability thresholds
// (>=70 % favours lumpsum, <=30 % favours SIP, else a toss-up). Paths default
// to 1000 and the RNG seed to 42, so results are reproducible. Deterministic
// arithmetic mirroring the legacy simulator; outputs are advisory per RBI
// FREE-AI and ignore transaction costs and taxes.
func SipVsLumpsumSpec() afg.Spec {
	return afg.Spec{
		ID:   "sip_vs_lumpsum",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type request struct {
				Amount               float64 `json:"amount_rupees"`
				HorizonMonths        int     `json:"horizon_months"`
				ExpectedAnnualReturn float64 `json:"expected_annual_return"`
				AnnualVolatility     float64 `json:"annual_volatility"`
				Paths                int     `json:"paths"`
				Seed                 int64   `json:"seed"`
			}
			type result struct {
				LumpsumP50        float64 `json:"lumpsum_p50"`
				SIPP50            float64 `json:"sip_p50"`
				LumpsumWinProb    float64 `json:"lumpsum_win_probability"`
				ExpectedRegretINR float64 `json:"expected_regret_rupees_at_p50"`
				Recommendation    string  `json:"recommendation"`
				Disclaimer        string  `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			paths := req.Paths
			if paths <= 0 {
				paths = 1000
			}
			seed := req.Seed
			if seed == 0 {
				seed = 42
			}
			r := rand.New(rand.NewSource(seed)) //nolint:gosec // G404: Monte Carlo return simulation; math/rand is intended, not security-sensitive
			monthlyMu := req.ExpectedAnnualReturn / 12
			monthlySigma := req.AnnualVolatility / math.Sqrt(12)
			monthlySIP := req.Amount / 12.0

			lump := make([]float64, paths)
			sip := make([]float64, paths)
			lumpsumWins := 0
			for p := 0; p < paths; p++ {
				L := req.Amount
				S := 0.0
				sipMonths := 12
				for m := 0; m < req.HorizonMonths; m++ {
					z := r.NormFloat64()
					ret := monthlyMu + monthlySigma*z
					L *= (1 + ret)
					S *= (1 + ret)
					if m < sipMonths {
						S += monthlySIP
					}
				}
				lump[p] = L
				sip[p] = S
				if L > S {
					lumpsumWins++
				}
			}
			sort.Float64s(lump)
			sort.Float64s(sip)
			lumpsumP50 := lump[paths/2]
			sipP50 := sip[paths/2]

			rec := "Toss-up — pick the strategy you can stick to behaviourally."
			if float64(lumpsumWins)/float64(paths) >= 0.70 {
				rec = "Lumpsum likely wins; consider STP-over-3-months as a compromise if you fear timing."
			} else if float64(lumpsumWins)/float64(paths) <= 0.30 {
				rec = "SIP likely wins; stick to monthly cadence."
			}

			res := result{
				LumpsumP50:        round2(lumpsumP50),
				SIPP50:            round2(sipP50),
				LumpsumWinProb:    round4(float64(lumpsumWins) / float64(paths)),
				ExpectedRegretINR: round2(math.Abs(lumpsumP50 - sipP50)),
				Recommendation:    rec,
				Disclaimer: "Monte-Carlo with lognormal returns; assumes no transaction costs / taxes. " +
					"Backtest-driven decisions vary by historical window — treat as one input, not the answer.",
			}

			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
