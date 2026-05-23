// Package recommender produces ranked, informational actions from the analyzer
// output. Rules-based for now; an LLM-backed implementation slots in by
// changing this file alone.
package recommender

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID            = "recommender"
	CapRecommend  = "recommend"
	CapSimulate   = "simulate_action"
	TypeIn        = "analysis_result"
	TypeOut       = "recommendations"
	NextAgent     = "financial_supervisor"
)

type CategoryTotal struct {
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	Count       int    `json:"count"`
}

type analyzerView struct {
	NetCents          int64           `json:"net_cents"`
	TotalIncomeCents  int64           `json:"total_income_cents"`
	TotalExpenseCents int64           `json:"total_expense_cents"`
	Currency          string          `json:"currency"`
	ByCategory        []CategoryTotal `json:"by_category"`
}

type Recommendation struct {
	Title          string `json:"title"`
	Category       string `json:"category,omitempty"`
	ImpactCents    int64  `json:"impact_cents"`
	Confidence     string `json:"confidence"`
	Action         string `json:"action"`
	Rationale      string `json:"rationale"`
	Informational  bool   `json:"informational"`
}

type Result struct {
	Currency        string           `json:"currency"`
	Recommendations []Recommendation `json:"recommendations"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Spending Recommender" }
func (a *Agent) Capabilities() []string { return []string{CapRecommend, CapSimulate} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	res := Result{Currency: av.Currency}

	if av.NetCents < 0 {
		res.Recommendations = append(res.Recommendations, Recommendation{
			Title:         "Spending exceeds income this period",
			ImpactCents:   -av.NetCents,
			Confidence:    "high",
			Action:        "review_largest_categories",
			Rationale:     fmt.Sprintf("Net cashflow is %d %s (negative).", av.NetCents, av.Currency),
			Informational: true,
		})
	}

	sorted := append([]CategoryTotal(nil), av.ByCategory...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].AmountCents > sorted[j].AmountCents })
	for i := 0; i < len(sorted) && i < 3; i++ {
		ct := sorted[i]
		impact := ct.AmountCents / 10 // assume a realistic 10% cut
		res.Recommendations = append(res.Recommendations, Recommendation{
			Title:         fmt.Sprintf("Consider trimming %q", ct.Category),
			Category:      ct.Category,
			ImpactCents:   impact,
			Confidence:    "medium",
			Action:        "set_budget",
			Rationale:     fmt.Sprintf("%d txns in %q this period.", ct.Count, ct.Category),
			Informational: true,
		})
	}

	env.Logf("[recommender] produced %d items", len(res.Recommendations))
	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}
