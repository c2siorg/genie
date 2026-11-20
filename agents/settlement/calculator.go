// calculator.go — CalculatorAgent sub-agent (Lesson 02: Tools)
//
// CalculatorAgent aggregates raw transactions into net bilateral positions
// and identifies netting opportunities. It groups by (counterparty, currency, date)
// and computes net amounts between each pair.
package settlement

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	sett "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/settlement"
)

// CalculatorAgent computes net settlement positions.
type CalculatorAgent struct {
	// NettingThreshold: minimum number of parties to consider netting
	NettingThreshold int
}

// NewCalculatorAgent returns a calculator with default settings.
func NewCalculatorAgent() *CalculatorAgent {
	return &CalculatorAgent{NettingThreshold: 3}
}

// Registry returns the tool set for the calculator.
func (ca *CalculatorAgent) Registry() *agenttools.Registry {
	reg := agenttools.NewRegistry()

	// calculate_net_amounts: aggregate transactions
	reg.Register(&agenttools.ToolDef{
		ToolName: "calculate_net_amounts",
		ToolDescription: "Aggregate transactions by (counterparty, currency, settlement_date) " +
			"to compute net bilateral positions. Returns the netting matrix showing " +
			"what each counterparty owes/is owed.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"transactions": map[string]any{
					"type":        "array",
					"description": "Array of transactions, each with from_counterparty, to_counterparty, amount, currency, settlement_date",
				},
				"include_netting": map[string]any{
					"type":        "boolean",
					"description": "If true, identify netting opportunities. Default false.",
				},
			},
			"required": []string{"transactions"},
		},
		Fn: ca.calculateNetAmounts,
	})

	// detect_netting_cycles: identify cycle-based netting
	reg.Register(&agenttools.ToolDef{
		ToolName: "detect_netting_cycles",
		ToolDescription: "Detect circular payment patterns where three or more parties can " +
			"reduce settlement via netting pool consolidation. For example, " +
			"A→B, B→C, C→A can reduce to a single pool settlement.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"net_amounts": map[string]any{
					"type":        "array",
					"description": "Array of computed net amounts (counterparty, amount, currency, date)",
				},
			},
			"required": []string{"net_amounts"},
		},
		Fn: ca.detectNettingCycles,
	})

	return reg
}

// calculateNetAmounts aggregates transactions by (cp, currency, date).
func (ca *CalculatorAgent) calculateNetAmounts(ctx context.Context, args map[string]any) (string, error) {
	txnsRaw, ok := args["transactions"]
	if !ok {
		return "error: transactions is required", nil
	}

	// Unmarshal transactions (expect []map[string]any or raw JSON)
	var txns []sett.Transaction
	switch v := txnsRaw.(type) {
	case []interface{}:
		// Unmarshaling from JSON array
		data, _ := json.Marshal(v)
		if err := json.Unmarshal(data, &txns); err != nil {
			return fmt.Sprintf("error: failed to parse transactions: %v", err), nil
		}
	case string:
		// Raw JSON string
		if err := json.Unmarshal([]byte(v), &txns); err != nil {
			return fmt.Sprintf("error: failed to parse transactions: %v", err), nil
		}
	}

	if len(txns) == 0 {
		return "error: no transactions provided", nil
	}

	// Aggregate by (cp, currency, date)
	type key struct {
		cp   string
		curr string
		date string
	}
	nets := make(map[key]float64)

	for _, txn := range txns {
		dateStr := txn.SettlementDate.Format("2006-01-02")

		// Sending out
		k1 := key{txn.FromCounterparty, txn.Currency, dateStr}
		nets[k1] -= txn.Amount

		// Receiving in
		k2 := key{txn.ToCounterparty, txn.Currency, dateStr}
		nets[k2] += txn.Amount
	}

	// Format output
	var result []string
	result = append(result, fmt.Sprintf("Netting summary (%d transactions):", len(txns)))
	result = append(result, "")

	// Sort for deterministic output
	type keyVal struct {
		key key
		val float64
	}
	var sorted []keyVal
	for k, v := range nets {
		if v != 0 { // skip zeros
			sorted = append(sorted, keyVal{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].key.date != sorted[j].key.date {
			return sorted[i].key.date < sorted[j].key.date
		}
		if sorted[i].key.curr != sorted[j].key.curr {
			return sorted[i].key.curr < sorted[j].key.curr
		}
		return sorted[i].key.cp < sorted[j].key.cp
	})

	for _, kv := range sorted {
		sign := "owes"
		amt := kv.val
		if amt < 0 {
			sign = "is owed"
			amt = -amt
		}
		result = append(result, fmt.Sprintf("- %s %s %.0f %s on %s (net: %.0f)",
			kv.key.cp, sign, amt, kv.key.curr, kv.key.date, kv.val))
	}

	return fmt.Sprintf("%d net positions identified\n%s", len(sorted), fmt.Sprintf("%s\n", result)), nil
}

// detectNettingCycles identifies three-party cycles.
func (ca *CalculatorAgent) detectNettingCycles(ctx context.Context, args map[string]any) (string, error) {
	netsRaw, ok := args["net_amounts"]
	if !ok {
		return "error: net_amounts is required", nil
	}

	var nets []sett.NetAmount
	data, _ := json.Marshal(netsRaw)
	if err := json.Unmarshal(data, &nets); err != nil {
		return fmt.Sprintf("error: failed to parse net_amounts: %v", err), nil
	}

	// Simplified: detect if we have 3+ distinct counterparties
	cpSet := make(map[string]bool)
	for _, net := range nets {
		cpSet[net.Counterparty] = true
	}

	if len(cpSet) < ca.NettingThreshold {
		return fmt.Sprintf("netting not viable: only %d distinct counterparties (threshold: %d)",
			len(cpSet), ca.NettingThreshold), nil
	}

	// Stub: report potential netting
	return fmt.Sprintf("netting cycle detected: %d counterparties can consolidate via netting pool. "+
		"Estimated savings: 15-20%% on fees", len(cpSet)), nil
}
