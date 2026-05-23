// Package aa_fetcher integrates India's Account Aggregator (Sahamati)
// framework — the consented-data-sharing rails for the Indian financial sector.
//
// In production, FIClient hits the user's chosen AA (e.g. Anumati, OneMoney,
// Finvu). For this repo we expose the interface and ship an InMemoryFIClient
// fixture; integrators plug their FIClient and the agent works unchanged.
//
// Pairs with pkg/compliance.Ledger — AA mandates explicit consent per data
// fetch, so the agent first checks that consent is on file.
package aa_fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

const (
	ID         = "aa_fetcher"
	Capability = "aa_fetch"
	TypeIn     = "aa_fetch_request"
	TypeOut    = "aa_fetch_result"
	NextAgent  = "financial_supervisor"
)

// Statement is a normalised account statement returned by the AA layer.
type Statement struct {
	AccountID  string                  `json:"account_id"`
	Currency   string                  `json:"currency"`
	From       string                  `json:"from"`
	To         string                  `json:"to"`
	Transactions []map[string]any      `json:"transactions"`
	Source     string                  `json:"source"` // "sahamati://..."
	Classification protocol.Classification `json:"classification"`
}

// FIClient is the Sahamati FIU contract Genie needs. Real impl talks to
// the chosen AA over HTTPS with the FIU's signed certificate.
type FIClient interface {
	FetchStatement(ctx context.Context, userID, accountID string) (Statement, error)
}

// Agent runs AA fetches, gated by the consent ledger.
type Agent struct {
	Client FIClient
	Ledger compliance.Ledger
}

// New constructs an Agent.
func New(c FIClient, l compliance.Ledger) *Agent { return &Agent{Client: c, Ledger: l} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Account Aggregator Fetcher" }
func (a *Agent) Capabilities() []string { return []string{Capability} }

// RiskLevel — fetches PII via consented data sharing; High risk per RBI FREE-AI Rec 14.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	userID, _ := msg.Metadata[protocol.MetaKeyUserID].(string)
	accountID, _ := msg.Metadata["account_id"].(string)
	if userID == "" || accountID == "" {
		return nil, errors.New("aa_fetcher: user_id and account_id required")
	}
	// Sahamati requires explicit consent per category.
	ok, err := a.Ledger.HasActive(ctx, userID, compliance.CategoryTransactions)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("aa_fetcher: no active consent for transactions")
	}
	stmt, err := a.Client.FetchStatement(ctx, userID, accountID)
	if err != nil {
		return nil, err
	}
	stmt.Classification = protocol.ClassPII
	body, err := json.Marshal(stmt)
	if err != nil {
		return nil, err
	}
	env.Logf("[aa_fetcher] fetched %d txns for user=%s acct=%s", len(stmt.Transactions), userID, accountID)
	md := cloneMetadata(msg.Metadata)
	md[protocol.MetaKeyClassification] = string(protocol.ClassPII)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), md),
	}, nil
}

func cloneMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

// InMemoryFIClient is the test/demo fixture. Pre-seed with Seed().
type InMemoryFIClient struct {
	statements map[string]Statement
}

// NewInMemoryFIClient builds the fixture.
func NewInMemoryFIClient() *InMemoryFIClient { return &InMemoryFIClient{statements: map[string]Statement{}} }

// Seed registers a statement under (userID, accountID).
func (c *InMemoryFIClient) Seed(userID, accountID string, s Statement) {
	c.statements[userID+":"+accountID] = s
}

func (c *InMemoryFIClient) FetchStatement(_ context.Context, userID, accountID string) (Statement, error) {
	s, ok := c.statements[userID+":"+accountID]
	if !ok {
		return Statement{}, errors.New("aa_fetcher: no statement found")
	}
	if s.From == "" {
		s.From = time.Now().AddDate(0, -1, 0).UTC().Format("2006-01-02")
	}
	if s.To == "" {
		s.To = time.Now().UTC().Format("2006-01-02")
	}
	return s, nil
}
