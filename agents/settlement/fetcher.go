// fetcher.go — FetcherAgent sub-agent (Lesson 02: Tools)
//
// FetcherAgent specializes in retrieving pending transactions from the source system.
// It exposes a search_pending_transactions tool that the LLM can call to query
// transactions by date range and settlement status. In production, this tool
// would integrate with GraphRAG Query or a database.
package settlement

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
)

// FetcherAgent provides transaction retrieval capabilities.
// It wraps tools that query the source transaction system.
type FetcherAgent struct {
	// DataSource is a stub for database/API access (implement in production)
	DataSource TransactionSource
}

// TransactionSource is the interface for fetching transactions.
type TransactionSource interface {
	// QueryByDateRange fetches transactions in the given date range with status filter
	QueryByDateRange(ctx context.Context, startDate, endDate time.Time, status string) ([]Transaction, error)
}

// Transaction is an alias for pkg/settlement.Transaction
type Transaction = fmt.Stringer // placeholder

// NewFetcherAgent returns a fetcher with optional data source.
// If ds is nil, the agent uses stub data for demo purposes.
func NewFetcherAgent(ds TransactionSource) *FetcherAgent {
	return &FetcherAgent{DataSource: ds}
}

// Registry returns the tool set for the fetcher.
func (fa *FetcherAgent) Registry() *agenttools.Registry {
	reg := agenttools.NewRegistry()

	// search_pending_transactions: query by date and status
	reg.Register(&agenttools.ToolDef{
		ToolName: "search_pending_transactions",
		ToolDescription: "Search for pending transactions by date range and settlement status. " +
			"Returns a list of transactions awaiting settlement. " +
			"Use this to retrieve the raw transaction data before netting.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start_date": map[string]any{
					"type":        "string",
					"description": "Start date (ISO 8601, e.g. 2024-05-01)",
				},
				"end_date": map[string]any{
					"type":        "string",
					"description": "End date (ISO 8601, e.g. 2024-05-31)",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Settlement status: 'pending', 'in_flight', or 'settled'",
				},
			},
			"required": []string{"start_date", "end_date", "status"},
		},
		Fn: fa.searchPendingTransactions,
	})

	// list_counterparties: get list of known counterparties
	reg.Register(&agenttools.ToolDef{
		ToolName: "list_counterparties",
		ToolDescription: "List all known counterparties in the settlement system. " +
			"Use this to validate counterparty names before settlement.",
		ToolSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Fn: fa.listCounterparties,
	})

	return reg
}

// searchPendingTransactions is the tool function for fetching transactions.
func (fa *FetcherAgent) searchPendingTransactions(ctx context.Context, args map[string]any) (string, error) {
	startStr, ok := args["start_date"].(string)
	if !ok || startStr == "" {
		return "error: start_date is required", nil
	}
	endStr, ok := args["end_date"].(string)
	if !ok || endStr == "" {
		return "error: end_date is required", nil
	}
	status, ok := args["status"].(string)
	if !ok || status == "" {
		return "error: status is required", nil
	}

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return fmt.Sprintf("error: invalid start_date %q: %v", startStr, err), nil
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return fmt.Sprintf("error: invalid end_date %q: %v", endStr, err), nil
	}

	// If data source configured, query it; otherwise return stub data
	if fa.DataSource != nil {
		txns, err := fa.DataSource.QueryByDateRange(ctx, start, end, status)
		if err != nil {
			return fmt.Sprintf("error: query failed: %v", err), nil
		}
		return fmt.Sprintf("Found %d transactions between %s and %s with status %q", len(txns), startStr, endStr, status), nil
	}

	// Stub data for demo
	return fmt.Sprintf("Found 3 pending transactions between %s and %s:\n"+
		"1. BANK_A -> BANK_B: USD 100,000 (2024-05-02)\n"+
		"2. BANK_B -> BANK_C: USD 75,000 (2024-05-02)\n"+
		"3. BANK_C -> BANK_A: USD 50,000 (2024-05-02)", startStr, endStr), nil
}

// listCounterparties returns the list of known settlement participants.
func (fa *FetcherAgent) listCounterparties(ctx context.Context, args map[string]any) (string, error) {
	// Stub: return common banking participants
	return "Known counterparties: BANK_A, BANK_B, BANK_C, SWIFT_POOL, CORRESPONDENT_XYZ", nil
}
