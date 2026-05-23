// Package enricher attaches merchant-category labels to normalized transactions.
// It uses a small built-in lookup table; production would back this with a
// merchant database or RAG over a knowledge layer.
package enricher

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "enricher"
	CapEnrich  = "enrich_merchant"
	NextAgent  = "analyzer"
	TypeIn     = "normalized_transactions"
	TypeOut    = "enriched_transactions"
)

// defaultCategories is intentionally small for the demo. Extend via NewWithCategories.
var defaultCategories = map[string]string{
	"swiggy":   "food:delivery",
	"zomato":   "food:delivery",
	"uber":     "transport:ride",
	"ola":      "transport:ride",
	"amazon":   "shopping:ecom",
	"flipkart": "shopping:ecom",
	"netflix":  "entertainment:streaming",
	"spotify":  "entertainment:streaming",
	"salary":   "income:salary",
	"rent":     "housing:rent",
	"electric": "utilities:power",
}

type Agent struct {
	Categories map[string]string
}

func New() *Agent {
	cp := make(map[string]string, len(defaultCategories))
	for k, v := range defaultCategories {
		cp[k] = v
	}
	return &Agent{Categories: cp}
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Merchant Enricher" }
func (a *Agent) Capabilities() []string { return []string{CapEnrich} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	txns, err := finance.UnmarshalTransactions(msg.Content)
	if err != nil {
		return nil, err
	}
	for i := range txns {
		t := &txns[i]
		key := strings.ToLower(t.Merchant)
		if key == "" {
			key = finance.NormalizeMerchant(t.Description)
			t.Merchant = key
		}
		if cat, ok := a.Categories[key]; ok {
			// Known merchant always wins so canonical labels like
			// "housing:rent" replace whatever the CSV supplied.
			t.Category = cat
		} else if t.Category == "" {
			t.Category = "uncategorized"
		}
	}
	env.Logf("[enricher] enriched %d txns", len(txns))
	payload, err := finance.MarshalTransactions(txns)
	if err != nil {
		return nil, err
	}
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, payload, msg.Metadata),
	}, nil
}
