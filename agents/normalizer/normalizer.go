// Package normalizer canonicalizes raw transactions: trims, lowercases,
// normalizes merchant slugs, defaults currency from account metadata, and
// generates stable transaction IDs.
package normalizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID           = "normalizer"
	CapNormalize = "normalize"
	NextAgent    = "enricher"
	TypeIn       = "raw_transactions"
	TypeOut      = "normalized_transactions"
)

type Agent struct {
	DefaultCurrency string
}

func New() *Agent { return &Agent{DefaultCurrency: "INR"} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Transaction Normalizer" }
func (a *Agent) Capabilities() []string { return []string{CapNormalize} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	txns, err := finance.UnmarshalTransactions(msg.Content)
	if err != nil {
		return nil, err
	}

	currency := a.DefaultCurrency
	if md, ok := msg.Metadata["currency"].(string); ok && md != "" {
		currency = md
	}
	accountID, _ := msg.Metadata["account_id"].(string)

	for i := range txns {
		t := &txns[i]
		if t.Currency == "" {
			t.Currency = currency
		}
		if t.AccountID == "" && accountID != "" {
			t.AccountID = accountID
		}
		t.Description = strings.TrimSpace(t.Description)
		t.Merchant = finance.NormalizeMerchant(t.Description)
		if t.TransactionID == "" {
			t.TransactionID = fmt.Sprintf("txn-%s-%04d", accountID, i+1)
		}
	}

	env.Logf("[normalizer] canonicalized %d txns", len(txns))
	payload, err := finance.MarshalTransactions(txns)
	if err != nil {
		return nil, err
	}
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, payload, msg.Metadata),
	}, nil
}
