// Package sip_vs_lumpsum simulates investing a fixed corpus as a one-shot
// lumpsum vs. a 12-month SIP under a supplied historical-return path or
// a Monte-Carlo lognormal model. Reports terminal-value distribution and
// the "regret" probability — how often SIP loses to lumpsum.
package sip_vs_lumpsum

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "sip_vs_lumpsum"
	Capability = "compare_sip_lumpsum"
	TypeIn     = "sip_lumpsum_request"
	TypeOut    = "sip_lumpsum_result"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload.
type Request struct {
	Amount               float64 `json:"amount_rupees"`
	HorizonMonths        int     `json:"horizon_months"`
	ExpectedAnnualReturn float64 `json:"expected_annual_return"`
	AnnualVolatility     float64 `json:"annual_volatility"`
	Paths                int     `json:"paths"`
	Seed                 int64   `json:"seed"`
}

// Result is the wire output.
type Result struct {
	LumpsumP50           float64 `json:"lumpsum_p50"`
	SIPP50               float64 `json:"sip_p50"`
	LumpsumWinProb       float64 `json:"lumpsum_win_probability"`
	ExpectedRegretINR    float64 `json:"expected_regret_rupees_at_p50"`
	Recommendation       string  `json:"recommendation"`
	Disclaimer           string  `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "SIP vs Lumpsum Comparator" }
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
	res := a.Simulate(req)
	env.Logf("[sip_vs_lumpsum] lumpsum p50=%.0f sip p50=%.0f", res.LumpsumP50, res.SIPP50)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Simulate runs the Monte-Carlo comparison.
func (a *Agent) Simulate(req Request) Result {
	paths := req.Paths
	if paths <= 0 {
		paths = 1000
	}
	seed := req.Seed
	if seed == 0 {
		seed = 42
	}
	r := rand.New(rand.NewSource(seed))
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
			L = L * (1 + ret)
			S = S * (1 + ret)
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
	return Result{
		LumpsumP50:        round2(lumpsumP50),
		SIPP50:            round2(sipP50),
		LumpsumWinProb:    round4(float64(lumpsumWins) / float64(paths)),
		ExpectedRegretINR: round2(math.Abs(lumpsumP50 - sipP50)),
		Recommendation:    rec,
		Disclaimer: "Monte-Carlo with lognormal returns; assumes no transaction costs / taxes. " +
			"Backtest-driven decisions vary by historical window — treat as one input, not the answer.",
	}
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
