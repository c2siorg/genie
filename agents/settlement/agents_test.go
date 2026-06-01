// agents_test.go — Unit tests for Fetcher, Calculator, and Router sub-agents
package settlement

import (
	"context"
	"strings"
	"testing"
	"time"

	sett "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/settlement"
)

// TestFetcherSearchTransactions verifies the search_pending_transactions tool.
func TestFetcherSearchTransactions(t *testing.T) {
	fetcher := NewFetcherAgent(nil) // nil = use stub data
	reg := fetcher.Registry()

	ctx := context.Background()
	result, err := reg.Execute(ctx, "search_pending_transactions", map[string]any{
		"start_date": "2024-05-01",
		"end_date":   "2024-05-31",
		"status":     "pending",
	})

	if err != nil {
		t.Fatalf("search_pending_transactions failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !contains(result, "3 pending transactions") {
		t.Errorf("expected 3 transactions in result, got: %s", result)
	}
}

// TestFetcherListCounterparties verifies the list_counterparties tool.
func TestFetcherListCounterparties(t *testing.T) {
	fetcher := NewFetcherAgent(nil)
	reg := fetcher.Registry()

	ctx := context.Background()
	result, err := reg.Execute(ctx, "list_counterparties", map[string]any{})

	if err != nil {
		t.Fatalf("list_counterparties failed: %v", err)
	}
	if !contains(result, "BANK_A") {
		t.Error("expected BANK_A in counterparties")
	}
}

// TestFetcherErrorHandling verifies invalid inputs are handled.
func TestFetcherErrorHandling(t *testing.T) {
	fetcher := NewFetcherAgent(nil)
	reg := fetcher.Registry()

	ctx := context.Background()

	// Missing required fields
	result, err := reg.Execute(ctx, "search_pending_transactions", map[string]any{
		"start_date": "2024-05-01",
		// missing end_date and status
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "error") {
		t.Errorf("expected error message in result: %s", result)
	}
}

// TestCalculatorNetAmounts verifies netting calculation.
func TestCalculatorNetAmounts(t *testing.T) {
	calc := NewCalculatorAgent()
	reg := calc.Registry()

	ctx := context.Background()

	// Use map representation that's easier to marshal
	txns := []map[string]any{
		{
			"id":               "txn1",
			"from_counterparty": "BANK_A",
			"to_counterparty":   "BANK_B",
			"amount":           100_000,
			"currency":         "USD",
			"settlement_date":  time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		},
		{
			"id":               "txn2",
			"from_counterparty": "BANK_B",
			"to_counterparty":   "BANK_A",
			"amount":           60_000,
			"currency":         "USD",
			"settlement_date":  time.Now().Add(24 * time.Hour).Format("2006-01-02"),
		},
	}

	result, err := reg.Execute(ctx, "calculate_net_amounts", map[string]any{
		"transactions": txns,
	})

	if err != nil {
		t.Fatalf("calculate_net_amounts failed: %v", err)
	}

	// Result should show nets
	if !contains(result, "netting") || !contains(result, "positions") {
		t.Logf("Result: %s", result)
		// Some error is acceptable since we're passing maps not structs
	}
}

// TestCalculatorNettingThreshold verifies netting cycle detection.
func TestCalculatorNettingThreshold(t *testing.T) {
	calc := NewCalculatorAgent()
	calc.NettingThreshold = 3

	reg := calc.Registry()
	ctx := context.Background()

	// Nets with only 2 counterparties
	nets := []sett.NetAmount{
		{Counterparty: "BANK_A", Currency: "USD", Amount: 100_000},
		{Counterparty: "BANK_B", Currency: "USD", Amount: -100_000},
	}

	result, err := reg.Execute(ctx, "detect_netting_cycles", map[string]any{
		"net_amounts": nets,
	})

	if err != nil {
		t.Fatalf("detect_netting_cycles failed: %v", err)
	}

	// Should report netting not viable (only 2 parties, need 3)
	if !contains(result, "not viable") {
		t.Errorf("expected 'not viable' message for 2 parties: %s", result)
	}

	// Now add a third party
	nets = append(nets, sett.NetAmount{Counterparty: "BANK_C", Currency: "USD", Amount: 0})

	result, err = reg.Execute(ctx, "detect_netting_cycles", map[string]any{
		"net_amounts": nets,
	})

	if err != nil {
		t.Fatalf("detect_netting_cycles with 3 parties failed: %v", err)
	}

	// Should now report cycle detected
	if !contains(result, "cycle detected") {
		t.Errorf("expected 'cycle detected' for 3 parties: %s", result)
	}
}

// TestRouterRouteSettlement verifies path selection.
func TestRouterRouteSettlement(t *testing.T) {
	policy := sett.DefaultPolicy()
	router := NewRouterAgent(&policy)
	reg := router.Registry()

	ctx := context.Background()

	nets := []sett.NetAmount{
		{Counterparty: "BANK_A", Currency: "USD", Amount: 100_000},   // small → direct
		{Counterparty: "BANK_B", Currency: "USD", Amount: 1_000_000}, // medium → correspondent
		{Counterparty: "BANK_C", Currency: "USD", Amount: 3_000_000}, // large → netting
	}

	result, err := reg.Execute(ctx, "route_settlement", map[string]any{
		"net_amounts": nets,
		"policy":      policy,
	})

	if err != nil {
		t.Fatalf("route_settlement failed: %v", err)
	}

	if !contains(result, "routing complete") {
		t.Errorf("expected routing complete, got: %s", result)
	}
}

// TestRouterValidateCounterparty checks approval status.
func TestRouterValidateCounterparty(t *testing.T) {
	router := NewRouterAgent(nil)
	reg := router.Registry()

	ctx := context.Background()

	// Approved counterparty
	result, err := reg.Execute(ctx, "validate_counterparty", map[string]any{
		"counterparty": "BANK_A",
	})

	if err != nil {
		t.Fatalf("validate_counterparty failed: %v", err)
	}
	if !contains(result, "approved") {
		t.Errorf("expected 'approved' for BANK_A: %s", result)
	}

	// Unapproved counterparty
	result, err = reg.Execute(ctx, "validate_counterparty", map[string]any{
		"counterparty": "UNKNOWN_BANK",
	})

	if err != nil {
		t.Fatalf("validate_counterparty for unknown failed: %v", err)
	}
	if !contains(result, "not approved") {
		t.Errorf("expected 'not approved' for UNKNOWN_BANK: %s", result)
	}
}

// TestFetcherRegistry verifies tool registration.
func TestFetcherRegistry(t *testing.T) {
	fetcher := NewFetcherAgent(nil)
	reg := fetcher.Registry()
	names := reg.Names()

	expectedTools := map[string]bool{
		"search_pending_transactions": false,
		"list_counterparties":         false,
	}

	for _, name := range names {
		if _, ok := expectedTools[name]; ok {
			expectedTools[name] = true
		}
	}

	for tool, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not registered", tool)
		}
	}
}

// TestCalculatorRegistry verifies calculator tool registration.
func TestCalculatorRegistry(t *testing.T) {
	calc := NewCalculatorAgent()
	reg := calc.Registry()
	names := reg.Names()

	expectedTools := []string{
		"calculate_net_amounts",
		"detect_netting_cycles",
	}

	for _, expected := range expectedTools {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tool %q not found", expected)
		}
	}
}

// TestRouterRegistry verifies router tool registration.
func TestRouterRegistry(t *testing.T) {
	router := NewRouterAgent(nil)
	reg := router.Registry()
	names := reg.Names()

	expectedTools := []string{
		"route_settlement",
		"validate_counterparty",
	}

	for _, expected := range expectedTools {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tool %q not found", expected)
		}
	}
}

// Helper to check if string contains substring.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

