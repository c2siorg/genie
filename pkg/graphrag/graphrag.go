// Package graphrag is a lightweight entity-graph retriever. Genie builds a
// graph from canonical finance.Transaction records:
//
//	(User)-[:OWNS]->(Account)-[:HAS_TXN]->(Transaction)-[:PAID_TO]->(Merchant)
//	(Transaction)-[:CATEGORISED_AS]->(Category)
//
// Queries traverse k hops from a seed (a category name, a merchant, etc.)
// and return the connected subgraph. The supervisor can include the
// subgraph as structured context for the recommender, replacing pure
// vector lookup with explainable hops.
//
// Storage: in-memory adjacency lists. For production swap with Neo4j,
// Memgraph, or DuckDB; the interface is intentionally tiny.
package graphrag

import (
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

// Kind labels a node category.
type Kind string

const (
	KindUser        Kind = "user"
	KindAccount     Kind = "account"
	KindTransaction Kind = "transaction"
	KindMerchant    Kind = "merchant"
	KindCategory    Kind = "category"
)

// EdgeKind labels a relation.
type EdgeKind string

const (
	EdgeOwns           EdgeKind = "OWNS"
	EdgeHasTxn         EdgeKind = "HAS_TXN"
	EdgePaidTo         EdgeKind = "PAID_TO"
	EdgeCategorisedAs  EdgeKind = "CATEGORISED_AS"
)

// Node is one entity in the graph. ID is unique within a Kind ("merchant:swiggy").
type Node struct {
	ID    string         `json:"id"`
	Kind  Kind           `json:"kind"`
	Props map[string]any `json:"props,omitempty"`
}

// Edge is a directed relation.
type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

// Graph is the in-memory store.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]Node
	out   map[string][]Edge // outgoing edges keyed by from-node id
	in    map[string][]Edge // incoming edges keyed by to-node id
}

// New builds an empty graph.
func New() *Graph {
	return &Graph{
		nodes: map[string]Node{},
		out:   map[string][]Edge{},
		in:    map[string][]Edge{},
	}
}

// UpsertNode inserts or replaces a node (Props are merged into existing).
func (g *Graph) UpsertNode(n Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	existing, ok := g.nodes[n.ID]
	if !ok {
		if n.Props == nil {
			n.Props = map[string]any{}
		}
		g.nodes[n.ID] = n
		return
	}
	for k, v := range n.Props {
		if existing.Props == nil {
			existing.Props = map[string]any{}
		}
		existing.Props[k] = v
	}
	if existing.Kind == "" {
		existing.Kind = n.Kind
	}
	g.nodes[n.ID] = existing
}

// AddEdge inserts a directed edge (no dedup).
func (g *Graph) AddEdge(e Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.out[e.From] = append(g.out[e.From], e)
	g.in[e.To] = append(g.in[e.To], e)
}

// Get returns the node by id.
func (g *Graph) Get(id string) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	return n, ok
}

// Subgraph collects nodes + edges reachable from `seed` within `hops`
// edges (both directions). Useful for "what surrounds Swiggy?".
type Subgraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Neighborhood does a breadth-first walk from `seed`.
func (g *Graph) Neighborhood(seed string, hops int) Subgraph {
	if hops < 0 {
		hops = 0
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := map[string]int{seed: 0}
	frontier := []string{seed}
	var edges []Edge
	for h := 0; h < hops; h++ {
		var next []string
		for _, id := range frontier {
			for _, e := range g.out[id] {
				edges = append(edges, e)
				if _, seen := visited[e.To]; !seen {
					visited[e.To] = h + 1
					next = append(next, e.To)
				}
			}
			for _, e := range g.in[id] {
				edges = append(edges, e)
				if _, seen := visited[e.From]; !seen {
					visited[e.From] = h + 1
					next = append(next, e.From)
				}
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	nodes := make([]Node, 0, len(visited))
	for id := range visited {
		if n, ok := g.nodes[id]; ok {
			nodes = append(nodes, n)
		}
	}
	return Subgraph{Nodes: nodes, Edges: edges}
}

// IngestTransactions builds the standard finance subgraph from a slice of
// canonical transactions. Idempotent — repeated calls upsert.
func (g *Graph) IngestTransactions(userID string, txns []finance.Transaction) {
	if userID != "" {
		g.UpsertNode(Node{ID: "user:" + userID, Kind: KindUser})
	}
	for _, t := range txns {
		txnID := "txn:" + t.TransactionID
		g.UpsertNode(Node{ID: txnID, Kind: KindTransaction, Props: map[string]any{
			"date":         t.Date,
			"amount_cents": t.AmountCents,
			"currency":     t.Currency,
			"description":  t.Description,
		}})
		if t.AccountID != "" {
			accID := "account:" + t.AccountID
			g.UpsertNode(Node{ID: accID, Kind: KindAccount})
			if userID != "" {
				g.AddEdge(Edge{From: "user:" + userID, To: accID, Kind: EdgeOwns})
			}
			g.AddEdge(Edge{From: accID, To: txnID, Kind: EdgeHasTxn})
		}
		if t.Merchant != "" {
			mID := "merchant:" + t.Merchant
			g.UpsertNode(Node{ID: mID, Kind: KindMerchant})
			g.AddEdge(Edge{From: txnID, To: mID, Kind: EdgePaidTo})
		}
		if t.Category != "" {
			cID := "category:" + t.Category
			g.UpsertNode(Node{ID: cID, Kind: KindCategory})
			g.AddEdge(Edge{From: txnID, To: cID, Kind: EdgeCategorisedAs})
		}
	}
}

// ExplainSpending returns a small subgraph for "where does the user spend":
// the user node, their account(s), and the top categories + their merchants.
//
// Designed to be JSON-marshalled directly into a recommender prompt so the
// rationale field can cite specific paths.
func (g *Graph) ExplainSpending(userID string, hops int) Subgraph {
	if hops <= 0 {
		hops = 3
	}
	return g.Neighborhood("user:"+userID, hops)
}
