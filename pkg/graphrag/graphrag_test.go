package graphrag

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestIngest_BuildsExpectedSubgraph(t *testing.T) {
	g := New()
	g.IngestTransactions("alice", []finance.Transaction{
		{TransactionID: "txn-1", AccountID: "acct-1", Date: "2026-01-03", AmountCents: -45000, Currency: "INR", Merchant: "swiggy", Category: "food:delivery"},
		{TransactionID: "txn-2", AccountID: "acct-1", Date: "2026-01-12", AmountCents: -2500000, Currency: "INR", Merchant: "rent", Category: "housing:rent"},
	})
	sub := g.ExplainSpending("alice", 3)

	kinds := map[Kind]int{}
	for _, n := range sub.Nodes {
		kinds[n.Kind]++
	}
	if kinds[KindUser] != 1 {
		t.Errorf("expected 1 user, got %d", kinds[KindUser])
	}
	if kinds[KindAccount] != 1 {
		t.Errorf("expected 1 account, got %d", kinds[KindAccount])
	}
	if kinds[KindTransaction] != 2 {
		t.Errorf("expected 2 txns, got %d", kinds[KindTransaction])
	}
	if kinds[KindMerchant] != 2 {
		t.Errorf("expected 2 merchants, got %d", kinds[KindMerchant])
	}
	if kinds[KindCategory] != 2 {
		t.Errorf("expected 2 categories, got %d", kinds[KindCategory])
	}
}

func TestNeighborhood_HopBoundary(t *testing.T) {
	g := New()
	g.UpsertNode(Node{ID: "a", Kind: "x"})
	g.UpsertNode(Node{ID: "b", Kind: "x"})
	g.UpsertNode(Node{ID: "c", Kind: "x"})
	g.AddEdge(Edge{From: "a", To: "b", Kind: "L"})
	g.AddEdge(Edge{From: "b", To: "c", Kind: "L"})

	sub := g.Neighborhood("a", 1)
	if len(sub.Nodes) != 2 {
		t.Fatalf("1-hop should include a + b only, got %+v", sub.Nodes)
	}
	sub = g.Neighborhood("a", 2)
	if len(sub.Nodes) != 3 {
		t.Fatalf("2-hop should include a + b + c, got %+v", sub.Nodes)
	}
}
