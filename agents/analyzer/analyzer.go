// Package analyzer aggregates enriched transactions: category totals, income vs
// expense, and a flat list of overspend categories (top N by debit volume).
package analyzer

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "analyzer"
	CapAnalyze = "analyze_spending"
	TypeIn     = "enriched_transactions"
	TypeOut    = "analysis_result"
	// fan-out so multiple downstream agents see the analysis.
	FanForecaster = "forecaster"
	FanAnomaly    = "anomaly_detector"
	FanRecommend  = "recommender"
)

type CategoryTotal struct {
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	Count       int    `json:"count"`
}

// Result is the analyzer's structured output. Keep field names stable —
// downstream agents parse it.
type Result struct {
	TotalIncomeCents  int64           `json:"total_income_cents"`
	TotalExpenseCents int64           `json:"total_expense_cents"`
	NetCents          int64           `json:"net_cents"`
	Currency          string          `json:"currency"`
	ByCategory        []CategoryTotal `json:"by_category"`
	TopOverspend      []string        `json:"top_overspend"`
	Transactions      []finance.Transaction `json:"transactions"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Spending Analyzer" }
func (a *Agent) Capabilities() []string { return []string{CapAnalyze} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	txns, err := finance.UnmarshalTransactions(msg.Content)
	if err != nil {
		return nil, err
	}

	res := Result{Transactions: txns}
	totals := map[string]*CategoryTotal{}
	for _, t := range txns {
		if res.Currency == "" {
			res.Currency = t.Currency
		}
		if t.AmountCents >= 0 {
			res.TotalIncomeCents += t.AmountCents
		} else {
			res.TotalExpenseCents += -t.AmountCents
			ct, ok := totals[t.Category]
			if !ok {
				ct = &CategoryTotal{Category: t.Category}
				totals[t.Category] = ct
			}
			ct.AmountCents += -t.AmountCents
			ct.Count++
		}
	}
	res.NetCents = res.TotalIncomeCents - res.TotalExpenseCents
	for _, v := range totals {
		res.ByCategory = append(res.ByCategory, *v)
	}
	sort.Slice(res.ByCategory, func(i, j int) bool {
		return res.ByCategory[i].AmountCents > res.ByCategory[j].AmountCents
	})
	// Top three categories by spend are flagged as overspend candidates.
	for i := 0; i < len(res.ByCategory) && i < 3; i++ {
		res.TopOverspend = append(res.TopOverspend, res.ByCategory[i].Category)
	}

	env.Logf("[analyzer] income=%d expense=%d net=%d top=%v",
		res.TotalIncomeCents, res.TotalExpenseCents, res.NetCents, res.TopOverspend)

	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	content := string(body)

	// Fan out to three downstream specialists so we get forecasts, anomalies,
	// and recommendations in parallel.
	return []agent.Message{
		agent.NewMessage(ID, FanForecaster, agent.RoleAgent, TypeOut, content, msg.Metadata),
		agent.NewMessage(ID, FanAnomaly, agent.RoleAgent, TypeOut, content, msg.Metadata),
		agent.NewMessage(ID, FanRecommend, agent.RoleAgent, TypeOut, content, msg.Metadata),
	}, nil
}
