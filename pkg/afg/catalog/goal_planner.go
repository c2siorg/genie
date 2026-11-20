package catalog

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// GoalPlannerSpec ports agents/goal_planner. It runs a Monte-Carlo simulation of
// whether the user will reach a financial goal (down-payment, FIRE corpus,
// education fund) given their current corpus, monthly contribution, target,
// horizon, and the expected return + volatility of the asset mix. It projects
// many lognormal-return paths (1000 by default), reports the success probability
// and the p10/p50/p90 terminal-corpus envelope, and solves the closed-form SIP
// that would hit the target at the expected return with no volatility. The
// simulation is seeded so results are reproducible; outputs are advisory and
// directional per RBI FREE-AI, not a guarantee.
func GoalPlannerSpec() afg.Spec {
	return afg.Spec{
		ID:   "goal_planner",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				defaultPaths = 1000
				defaultSeed  = 42
			)

			// Request is the wire payload. Monetary fields in rupees.
			type Request struct {
				CurrentCorpus        float64 `json:"current_corpus_rupees"`
				MonthlyContribution  float64 `json:"monthly_contribution_rupees"`
				TargetCorpus         float64 `json:"target_corpus_rupees"`
				HorizonMonths        int     `json:"horizon_months"`
				ExpectedAnnualReturn float64 `json:"expected_annual_return"` // decimal e.g. 0.10
				AnnualVolatility     float64 `json:"annual_volatility"`      // decimal e.g. 0.15
				Paths                int     `json:"paths,omitempty"`        // override 1000
				Seed                 int64   `json:"seed,omitempty"`
			}
			// Plan is the wire output.
			type Plan struct {
				SuccessProbability float64 `json:"success_probability_0_1"`
				P10Corpus          float64 `json:"p10_corpus_rupees"`
				P50Corpus          float64 `json:"p50_corpus_rupees"`
				P90Corpus          float64 `json:"p90_corpus_rupees"`
				RequiredMonthlyINR float64 `json:"required_monthly_at_p50_rupees"`
				Disclaimer         string  `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

			// requiredMonthly solves for the SIP that hits the target at the
			// expected return with no volatility. Closed-form FV of annuity.
			requiredMonthly := func(req Request) float64 {
				r := req.ExpectedAnnualReturn / 12
				n := float64(req.HorizonMonths)
				if r == 0 {
					return (req.TargetCorpus - req.CurrentCorpus) / n
				}
				fvCurrent := req.CurrentCorpus * math.Pow(1+r, n)
				needed := req.TargetCorpus - fvCurrent
				if needed <= 0 {
					return 0
				}
				// FV of SIP = PMT * ((1+r)^n - 1) / r
				return needed * r / (math.Pow(1+r, n) - 1)
			}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			paths := req.Paths
			if paths <= 0 {
				paths = defaultPaths
			}
			seed := req.Seed
			if seed == 0 {
				seed = defaultSeed
			}

			// Self-contained deterministic PRNG (SplitMix64) + Box-Muller so the
			// simulation is reproducible with a fixed seed without importing
			// math/rand. Same seed -> identical results.
			state := uint64(seed)
			nextU64 := func() uint64 {
				state += 0x9E3779B97F4A7C15
				z := state
				z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
				z = (z ^ (z >> 27)) * 0x94D049BB133111EB
				return z ^ (z >> 31)
			}
			// uniform in (0,1)
			nextUnit := func() float64 {
				// 53-bit mantissa, shifted to avoid an exact 0.
				return (float64(nextU64()>>11) + 0.5) * (1.0 / 9007199254740992.0)
			}
			// standard normal via Box-Muller (cache the second draw).
			var haveSpare bool
			var spare float64
			normFloat64 := func() float64 {
				if haveSpare {
					haveSpare = false
					return spare
				}
				u1 := nextUnit()
				u2 := nextUnit()
				mag := math.Sqrt(-2.0 * math.Log(u1))
				z0 := mag * math.Cos(2*math.Pi*u2)
				z1 := mag * math.Sin(2*math.Pi*u2)
				spare = z1
				haveSpare = true
				return z0
			}

			monthlyMu := req.ExpectedAnnualReturn / 12
			monthlySigma := req.AnnualVolatility / math.Sqrt(12)

			terminals := make([]float64, paths)
			success := 0
			for p := 0; p < paths; p++ {
				corpus := req.CurrentCorpus
				for m := 0; m < req.HorizonMonths; m++ {
					z := normFloat64()
					ret := monthlyMu + monthlySigma*z
					corpus = corpus*(1+ret) + req.MonthlyContribution
				}
				terminals[p] = corpus
				if corpus >= req.TargetCorpus {
					success++
				}
			}
			sort.Float64s(terminals)

			plan := Plan{
				SuccessProbability: round4(float64(success) / float64(paths)),
				P10Corpus:          round2(terminals[paths/10]),
				P50Corpus:          round2(terminals[paths/2]),
				P90Corpus:          round2(terminals[paths-paths/10-1]),
				RequiredMonthlyINR: round2(requiredMonthly(req)),
				Disclaimer: "Monte-Carlo with lognormal returns; assumes constant volatility. " +
					"Real markets exhibit fat tails — treat probability as directional, not guaranteed.",
			}

			body, err := json.Marshal(plan)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
