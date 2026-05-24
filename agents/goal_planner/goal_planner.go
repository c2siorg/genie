// Package goal_planner runs a Monte-Carlo simulation of whether the user
// will reach a financial goal (down-payment, FIRE corpus, education fund)
// given their current corpus, monthly contribution, target, horizon, and
// expected return + volatility of the asset mix.
//
// The simulator uses 1000 lognormal-return paths by default; outputs
// success probability + p10/p50/p90 corpus envelope. Deterministic with a
// fixed seed so tests are reproducible.
package goal_planner

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "goal_planner"
	Capability = "plan_goal"
	TypeIn     = "goal_request"
	TypeOut    = "goal_plan"
	NextAgent  = "financial_supervisor"

	DefaultPaths = 1000
	DefaultSeed  = 42
)

// Request is the wire payload. Monetary fields in rupees.
type Request struct {
	CurrentCorpus       float64 `json:"current_corpus_rupees"`
	MonthlyContribution float64 `json:"monthly_contribution_rupees"`
	TargetCorpus        float64 `json:"target_corpus_rupees"`
	HorizonMonths       int     `json:"horizon_months"`
	ExpectedAnnualReturn float64 `json:"expected_annual_return"` // decimal e.g. 0.10
	AnnualVolatility     float64 `json:"annual_volatility"`      // decimal e.g. 0.15
	Paths                int     `json:"paths,omitempty"`        // override 1000
	Seed                 int64   `json:"seed,omitempty"`
}

// Plan is the wire output.
type Plan struct {
	SuccessProbability  float64 `json:"success_probability_0_1"`
	P10Corpus           float64 `json:"p10_corpus_rupees"`
	P50Corpus           float64 `json:"p50_corpus_rupees"`
	P90Corpus           float64 `json:"p90_corpus_rupees"`
	RequiredMonthlyINR  float64 `json:"required_monthly_at_p50_rupees"`
	Disclaimer          string  `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Goal Planner" }
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
	plan := a.Simulate(req)
	env.Logf("[goal_planner] P(success)=%.2f p50=%.0f", plan.SuccessProbability, plan.P50Corpus)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Simulate runs the Monte-Carlo and returns success probability + percentiles.
func (a *Agent) Simulate(req Request) Plan {
	paths := req.Paths
	if paths <= 0 {
		paths = DefaultPaths
	}
	seed := req.Seed
	if seed == 0 {
		seed = DefaultSeed
	}
	r := rand.New(rand.NewSource(seed))
	monthlyMu := req.ExpectedAnnualReturn / 12
	monthlySigma := req.AnnualVolatility / math.Sqrt(12)

	terminals := make([]float64, paths)
	success := 0
	for p := 0; p < paths; p++ {
		corpus := req.CurrentCorpus
		for m := 0; m < req.HorizonMonths; m++ {
			z := r.NormFloat64()
			ret := monthlyMu + monthlySigma*z
			corpus = corpus*(1+ret) + req.MonthlyContribution
		}
		terminals[p] = corpus
		if corpus >= req.TargetCorpus {
			success++
		}
	}
	sort.Float64s(terminals)
	return Plan{
		SuccessProbability: round4(float64(success) / float64(paths)),
		P10Corpus:          round2(terminals[paths/10]),
		P50Corpus:          round2(terminals[paths/2]),
		P90Corpus:          round2(terminals[paths-paths/10-1]),
		RequiredMonthlyINR: round2(requiredMonthly(req)),
		Disclaimer: "Monte-Carlo with lognormal returns; assumes constant volatility. " +
			"Real markets exhibit fat tails — treat probability as directional, not guaranteed.",
	}
}

// requiredMonthly solves for the SIP that hits the target at p50 returns
// (no volatility). Closed-form FV of annuity.
func requiredMonthly(req Request) float64 {
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

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
