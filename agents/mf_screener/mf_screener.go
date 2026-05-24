// Package mf_screener ranks a list of mutual-fund candidates against
// user-supplied filters and a composite score blending: 3-yr / 5-yr CAGR,
// Sharpe ratio (excess-return / std-dev), expense ratio (lower better),
// and consistency (negative-return quarters in last 5y).
//
// Inputs come from the user's broker MCP or AMFI scraped data. The agent
// performs no live data fetch — it scores what's handed in.
package mf_screener

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "mf_screener"
	Capability = "screen_mutual_funds"
	TypeIn     = "mf_screen_request"
	TypeOut    = "mf_screen_results"
	NextAgent  = "financial_supervisor"
)

// Fund is one candidate.
type Fund struct {
	Scheme            string  `json:"scheme"`
	Category          string  `json:"category"`     // "equity-large", "debt-corporate", "hybrid", ...
	NAV               float64 `json:"nav"`
	AUMCr             float64 `json:"aum_crore"`
	ExpenseRatio      float64 `json:"expense_ratio"`
	ThreeYrCAGR       float64 `json:"three_year_cagr"`
	FiveYrCAGR        float64 `json:"five_year_cagr"`
	StdDev            float64 `json:"std_dev"`
	NegativeQuartersL5 int    `json:"neg_quarters_last_5yr"`
}

// Filter applies hard cuts.
type Filter struct {
	Category    string  `json:"category,omitempty"`
	MinAUMCr    float64 `json:"min_aum_crore,omitempty"`
	MaxExpense  float64 `json:"max_expense_ratio,omitempty"`
	MinThreeYr  float64 `json:"min_three_yr_cagr,omitempty"`
}

// Request is the wire payload.
type Request struct {
	Funds  []Fund `json:"funds"`
	Filter Filter `json:"filter"`
}

// Ranked is one scored result.
type Ranked struct {
	Scheme   string  `json:"scheme"`
	Score    float64 `json:"score_0_100"`
	Sharpe   float64 `json:"sharpe"`
	Reason   string  `json:"reason"`
}

// Result is the wire output.
type Result struct {
	Ranked      []Ranked `json:"ranked"`
	FilteredOut int      `json:"filtered_out"`
	Note        string   `json:"note"`
}

type Agent struct {
	RiskFreeRate float64 // override default for tests
}

func New() *Agent { return &Agent{RiskFreeRate: 0.07} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Mutual Fund Screener" }
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
	res := a.Screen(req)
	env.Logf("[mf_screener] kept %d filtered_out %d", len(res.Ranked), res.FilteredOut)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Screen applies the filter then scores survivors.
func (a *Agent) Screen(req Request) Result {
	res := Result{Note: "Scores are relative within this candidate set. Past returns are not a guarantee of future performance."}
	for _, f := range req.Funds {
		if !passesFilter(f, req.Filter) {
			res.FilteredOut++
			continue
		}
		sharpe := 0.0
		if f.StdDev > 0 {
			sharpe = (f.FiveYrCAGR - a.RiskFreeRate) / f.StdDev
		}
		// Composite 0..100:
		//   40 % 5yr CAGR (normalised by 0.25 cap)
		//   25 % Sharpe (normalised by 1.5 cap)
		//   20 % expense ratio (inverse normalised by 2 % cap)
		//   15 % consistency (inverse of negative-quarter count, cap 8)
		c := 0.40*normCap(f.FiveYrCAGR, 0.25) +
			0.25*normCap(sharpe, 1.5) +
			0.20*normInverse(f.ExpenseRatio, 0.02) +
			0.15*normInverse(float64(f.NegativeQuartersL5), 8)
		res.Ranked = append(res.Ranked, Ranked{
			Scheme: f.Scheme,
			Score:  round2(c * 100),
			Sharpe: round2(sharpe),
			Reason: "Composite of 5y CAGR (40 %), Sharpe (25 %), expense (20 %), consistency (15 %).",
		})
	}
	sort.SliceStable(res.Ranked, func(i, j int) bool {
		return res.Ranked[i].Score > res.Ranked[j].Score
	})
	return res
}

func passesFilter(f Fund, fi Filter) bool {
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

func normCap(x, cap float64) float64 {
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

func normInverse(x, cap float64) float64 {
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

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
