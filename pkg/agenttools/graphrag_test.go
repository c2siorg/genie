package agenttools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/graphrag"
)

func seedGraph(t *testing.T) *graphrag.Graph {
	t.Helper()
	g := graphrag.New()
	g.IngestTransactions("alice", []finance.Transaction{
		{TransactionID: "t1", AccountID: "hdfc-1", Merchant: "Swiggy", Category: "Food", AmountCents: 50000, Currency: "INR"},
		{TransactionID: "t2", AccountID: "hdfc-1", Merchant: "Amazon", Category: "Shopping", AmountCents: 120000, Currency: "INR"},
		{TransactionID: "t3", AccountID: "hdfc-1", Merchant: "Swiggy", Category: "Food", AmountCents: 30000, Currency: "INR"},
	})
	return g
}

// ─── GraphQuery ───────────────────────────────────────────────────────────

func TestGraphQuery_ReturnsNeighbourhood(t *testing.T) {
	g := seedGraph(t)
	tool := agenttools.GraphQuery(g)

	result, err := tool.Execute(context.Background(), map[string]any{
		"seed": "user:alice", "hops": float64(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "alice") {
		t.Errorf("expected alice in result, got: %s", result)
	}
	if !strings.Contains(result, "Swiggy") && !strings.Contains(result, "account") {
		t.Errorf("expected connected nodes in result, got: %s", result)
	}
}

func TestGraphQuery_MissingSeed(t *testing.T) {
	g := seedGraph(t)
	tool := agenttools.GraphQuery(g)
	result, _ := tool.Execute(context.Background(), map[string]any{"seed": ""})
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for empty seed, got %q", result)
	}
}

func TestGraphQuery_UnknownSeed(t *testing.T) {
	g := seedGraph(t)
	tool := agenttools.GraphQuery(g)
	result, _ := tool.Execute(context.Background(), map[string]any{"seed": "user:nobody"})
	if !strings.Contains(result, "no node found") {
		t.Errorf("expected 'no node found', got %q", result)
	}
}

func TestGraphQuery_NilGraph(t *testing.T) {
	tool := agenttools.GraphQuery(nil)
	result, _ := tool.Execute(context.Background(), map[string]any{"seed": "user:x"})
	if !strings.Contains(result, "not initialised") {
		t.Errorf("expected 'not initialised', got %q", result)
	}
}

// ─── GraphIngest ──────────────────────────────────────────────────────────

func TestGraphIngest_AddsNodes(t *testing.T) {
	g := graphrag.New()
	tool := agenttools.GraphIngest(g)

	txnsJSON := `[{"transaction_id":"t99","account_id":"acc-1","merchant":"Netflix","category":"Entertainment","amount_cents":50000,"currency":"INR"}]`
	result, err := tool.Execute(context.Background(), map[string]any{
		"user_id":      "bob",
		"transactions": txnsJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "ingested") {
		t.Errorf("expected confirmation, got %q", result)
	}
	// Node should exist now.
	if _, ok := g.Get("merchant:Netflix"); !ok {
		t.Error("merchant:Netflix should be in graph after ingest")
	}
}

func TestGraphIngest_InvalidJSON(t *testing.T) {
	g := graphrag.New()
	tool := agenttools.GraphIngest(g)
	result, _ := tool.Execute(context.Background(), map[string]any{
		"user_id": "bob", "transactions": "not-json",
	})
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for invalid JSON, got %q", result)
	}
}

// ─── GraphExplainSpending ─────────────────────────────────────────────────

func TestGraphExplainSpending_ReturnsSpendGraph(t *testing.T) {
	g := seedGraph(t)
	tool := agenttools.GraphExplainSpending(g)
	result, err := tool.Execute(context.Background(), map[string]any{"user_id": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "alice") {
		t.Errorf("expected alice in result, got %s", result)
	}
}

func TestGraphExplainSpending_UnknownUser(t *testing.T) {
	g := graphrag.New()
	tool := agenttools.GraphExplainSpending(g)
	result, _ := tool.Execute(context.Background(), map[string]any{"user_id": "ghost"})
	if !strings.Contains(result, "no graph data") {
		t.Errorf("expected 'no graph data', got %q", result)
	}
}

// ─── GraphRAGTools registry ───────────────────────────────────────────────

func TestGraphRAGTools_HasThreeTools(t *testing.T) {
	reg := agenttools.GraphRAGTools(graphrag.New())
	if len(reg.Definitions()) != 3 {
		t.Errorf("expected 3 tools, got %d", len(reg.Definitions()))
	}
}

func TestGraphRAGTools_Execute(t *testing.T) {
	g := seedGraph(t)
	reg := agenttools.GraphRAGTools(g)
	result, err := reg.Execute(context.Background(), "graph_query", map[string]any{
		"seed": "merchant:Swiggy", "hops": float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}
