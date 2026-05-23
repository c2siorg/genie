// Package portfolio_advisor fetches the requesting user's portfolio from a
// Zerodha Kite MCP server and publishes a portfolio_snapshot message the
// supervisor merges into the final report.
//
// Token lookup: the user's per-provider MCP session token lives encrypted in
// Postgres (pkg/storage/postgres.MCPToken). The agent decrypts it at call
// time, builds a Client with that bearer token, and discards it after the call.
package portfolio_advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/mcp"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
)

const (
	ID            = "portfolio_advisor"
	CapPortfolio  = "fetch_portfolio"
	TypeQuestion  = "portfolio_request"
	TypeSnapshot  = "portfolio_snapshot"
	NextAgent     = "financial_supervisor"
	ProviderKite  = "zerodha-kite"
)

// ErrNoToken is returned when the user has not yet linked their Kite session.
var ErrNoToken = errors.New("no kite session token on file for user")

// Agent implements the portfolio fetcher.
//
// Tokens and Encryptor are required dependencies; they are passed via the
// constructor instead of pulled from a global so tests can substitute fakes.
type Agent struct {
	Tokens    postgres.MCPTokenRepo
	Encryptor *crypto.Encryptor
	// NewClient is a hook so tests can substitute an in-memory MCP server.
	// Production calls mcp.NewClient(endpoint).
	NewClient func(endpoint string) *mcp.Client
}

// Snapshot is the shape we publish back to the supervisor.
type Snapshot struct {
	Provider     string `json:"provider"`
	Holdings     string `json:"holdings"`      // raw tool text — keep verbatim for the report
	Positions    string `json:"positions"`
	FetchedFor   string `json:"fetched_for"`   // user id
	Classification protocol.Classification `json:"classification"`
}

// New constructs the agent.
func New(tokens postgres.MCPTokenRepo, enc *crypto.Encryptor) *Agent {
	return &Agent{
		Tokens:    tokens,
		Encryptor: enc,
		NewClient: mcp.NewClient,
	}
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Portfolio Advisor (Kite)" }
func (a *Agent) Capabilities() []string { return []string{CapPortfolio} }

// RiskLevel — pulls PII portfolio data from a remote provider; classified
// High per RBI FREE-AI Rec 14.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeQuestion {
		return nil, nil
	}
	userID, _ := msg.Metadata[protocol.MetaKeyUserID].(string)
	if userID == "" {
		return nil, errors.New("portfolio_advisor: user_id required in metadata")
	}

	tok, err := a.Tokens.Get(ctx, userID, ProviderKite)
	if err != nil {
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	plain, err := a.Encryptor.Decrypt(tok.Payload)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}

	client := a.NewClient(tok.Endpoint)
	client.Authorization = "Bearer " + string(plain)
	if err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("kite initialize: %w", err)
	}

	holdings, _ := callText(ctx, client, "get_holdings", nil)
	positions, _ := callText(ctx, client, "get_positions", nil)

	snap := Snapshot{
		Provider:       ProviderKite,
		Holdings:       holdings,
		Positions:      positions,
		FetchedFor:     userID,
		Classification: protocol.ClassPII,
	}
	body, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	env.Logf("[portfolio_advisor] snapshot ready for user=%s", userID)

	// Propagate PII classification so downstream policies make sense.
	md := cloneMetadata(msg.Metadata)
	md[protocol.MetaKeyClassification] = string(protocol.ClassPII)

	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeSnapshot, string(body), md),
	}, nil
}

// callText calls an MCP tool and concatenates the text content blocks.
// Errors are returned as their string for inclusion in the report — the
// supervisor decides whether a missing tool is fatal.
func callText(ctx context.Context, c *mcp.Client, name string, args map[string]any) (string, error) {
	res, err := c.CallTool(ctx, name, args)
	if err != nil {
		return fmt.Sprintf("(%s error: %v)", name, err), err
	}
	out := ""
	for _, blk := range res.Content {
		if blk.Type == "text" {
			out += blk.Text
		}
	}
	return out, nil
}

func cloneMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
