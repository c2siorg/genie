// graphrag.go — GraphRAG tools (Issue #25)
//
// Wraps pkg/graphrag as agenttools so agents can traverse the financial
// entity graph (User → Account → Transaction → Merchant → Category) and
// answer relational questions that plain vector search can't handle.
//
// Usage:
//
//	graph := graphrag.New()
//	graph.IngestTransactions("user-123", txns)
//
//	runner := &agentic.Runner{
//	    Registry: agenttools.GraphRAGTools(graph),
//	}
package agenttools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/graphrag"
)

// GraphQuery returns a tool that traverses the entity graph from a seed node
// and returns the connected subgraph as readable text.
//
// Example agent call:
//
//	{"seed": "merchant:Swiggy", "hops": 2}
//	→ "Node user:alice OWNS account:hdfc-savings
//	   Node account:hdfc-savings HAS_TXN txn:t1
//	   Node txn:t1 PAID_TO merchant:Swiggy ..."
func GraphQuery(graph *graphrag.Graph) Tool {
	return &ToolDef{
		ToolName:        "graph_query",
		ToolDescription: "Traverse the financial entity graph to answer relational questions. Use for 'which merchants did this user visit?', 'what categories does account X spend on?', 'show me all transactions for merchant Y'. Provide a seed node (format: 'kind:id', e.g. 'user:alice', 'merchant:Swiggy', 'category:Food') and the number of hops to traverse.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"seed": map[string]any{
					"type":        "string",
					"description": "Seed node ID in format 'kind:id' (e.g. 'user:alice', 'merchant:Swiggy', 'category:Food', 'account:hdfc-savings')",
				},
				"hops": map[string]any{
					"type":        "integer",
					"description": "Number of graph hops to traverse (default 2, max 4)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default 20)",
				},
			},
			"required": []string{"seed"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			if graph == nil {
				return "error: graph not initialised — call graph_ingest first", nil
			}
			seed, ok := args["seed"].(string)
			if !ok || strings.TrimSpace(seed) == "" {
				return "error: seed is required (format: 'kind:id')", nil
			}

			hops := 2
			if n, ok := args["hops"].(float64); ok && n > 0 {
				hops = int(n)
				if hops > 4 {
					hops = 4
				}
			}
			limit := 20
			if n, ok := args["limit"].(float64); ok && n > 0 {
				limit = int(n)
			}

			// Verify seed exists.
			if _, found := graph.Get(seed); !found {
				return fmt.Sprintf("no node found for seed %q — try 'user:id', 'merchant:name', 'category:name'", seed), nil
			}

			sub := graph.Neighborhood(seed, hops)
			if len(sub.Nodes) == 0 && len(sub.Edges) == 0 {
				return fmt.Sprintf("no connected nodes found for %q within %d hops", seed, hops), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Graph neighbourhood of %q (%d hops):\n\n", seed, hops)

			// Nodes section.
			sb.WriteString("Nodes:\n")
			count := 0
			for _, n := range sub.Nodes {
				if count >= limit {
					fmt.Fprintf(&sb, "  ... (%d more nodes)\n", len(sub.Nodes)-limit)
					break
				}
				fmt.Fprintf(&sb, "  [%s] %s", n.Kind, n.ID)
				if len(n.Props) > 0 {
					// Format key props concisely.
					var props []string
					for k, v := range n.Props {
						props = append(props, fmt.Sprintf("%s=%v", k, v))
					}
					fmt.Fprintf(&sb, " (%s)", strings.Join(props, ", "))
				}
				sb.WriteString("\n")
				count++
			}

			// Edges section.
			sb.WriteString("\nRelationships:\n")
			count = 0
			for _, e := range sub.Edges {
				if count >= limit {
					fmt.Fprintf(&sb, "  ... (%d more relationships)\n", len(sub.Edges)-limit)
					break
				}
				fmt.Fprintf(&sb, "  %s -[%s]-> %s\n", e.From, e.Kind, e.To)
				count++
			}

			return strings.TrimSpace(sb.String()), nil
		},
	}
}

// GraphIngest returns a tool that loads financial transactions into the graph
// at runtime. The agent can call this when the user provides transaction data.
func GraphIngest(graph *graphrag.Graph) Tool {
	return &ToolDef{
		ToolName:        "graph_ingest",
		ToolDescription: "Load financial transaction records into the entity graph so they can be queried with graph_query. Accepts a user_id and a JSON array of transactions.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{
					"type":        "string",
					"description": "The user identifier to associate with these transactions",
				},
				"transactions": map[string]any{
					"type":        "string",
					"description": "JSON array of transaction objects with fields: transaction_id, account_id, merchant, category, amount_cents, currency, date, description",
				},
			},
			"required": []string{"user_id", "transactions"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			if graph == nil {
				return "error: graph not initialised", nil
			}
			userID, _ := args["user_id"].(string)
			if strings.TrimSpace(userID) == "" {
				return "error: user_id is required", nil
			}
			txnsJSON, _ := args["transactions"].(string)
			if strings.TrimSpace(txnsJSON) == "" {
				return "error: transactions JSON array is required", nil
			}

			var txns []finance.Transaction
			if err := json.Unmarshal([]byte(txnsJSON), &txns); err != nil {
				return fmt.Sprintf("error parsing transactions JSON: %v", err), nil
			}
			if len(txns) == 0 {
				return "no transactions to ingest", nil
			}

			graph.IngestTransactions(userID, txns)
			return fmt.Sprintf("ingested %d transaction(s) for user %q into the entity graph", len(txns), userID), nil
		},
	}
}

// GraphExplainSpending returns a tool that explains where a user spends money
// by traversing the graph from their user node.
func GraphExplainSpending(graph *graphrag.Graph) Tool {
	return &ToolDef{
		ToolName:        "graph_explain_spending",
		ToolDescription: "Show a structured view of where a user spends money — their accounts, top merchants, and categories — by traversing the entity graph.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id": map[string]any{
					"type":        "string",
					"description": "User identifier",
				},
				"hops": map[string]any{
					"type":        "integer",
					"description": "Graph depth (default 3)",
				},
			},
			"required": []string{"user_id"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			if graph == nil {
				return "error: graph not initialised", nil
			}
			userID, _ := args["user_id"].(string)
			if strings.TrimSpace(userID) == "" {
				return "error: user_id is required", nil
			}
			hops := 3
			if n, ok := args["hops"].(float64); ok && n > 0 {
				hops = int(n)
			}

			sub := graph.ExplainSpending(userID, hops)
			if len(sub.Nodes) == 0 {
				return fmt.Sprintf("no graph data found for user %q — call graph_ingest first", userID), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Spending graph for user %q:\n\n", userID)
			for _, n := range sub.Nodes {
				fmt.Fprintf(&sb, "  [%s] %s\n", n.Kind, n.ID)
			}
			sb.WriteString("\nRelationships:\n")
			for _, e := range sub.Edges {
				fmt.Fprintf(&sb, "  %s -[%s]-> %s\n", e.From, e.Kind, e.To)
			}
			return strings.TrimSpace(sb.String()), nil
		},
	}
}

// GraphRAGTools returns a Registry with all three graph tools backed by graph.
func GraphRAGTools(graph *graphrag.Graph) *Registry {
	r := NewRegistry()
	r.Register(GraphQuery(graph))
	r.Register(GraphIngest(graph))
	r.Register(GraphExplainSpending(graph))
	return r
}
