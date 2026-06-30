package commerce

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/cbdc"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/lineage"
)

// fulfilledOrder creates an order and walks it to StatusFulfilled (the order
// manager only allows pending→paid→fulfilled, so we cannot jump directly).
func fulfilledOrder(t *testing.T, mgr OrderManager, amountPaise int64) *Order {
	t.Helper()
	order, err := mgr.CreateOrder("merchant-recon", "customer-recon",
		[]OrderItem{{SKU: "X", Quantity: 1, UnitPricePaise: amountPaise}})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := mgr.UpdateStatus(order.ID, StatusPaid); err != nil {
		t.Fatalf("->paid: %v", err)
	}
	if _, err := mgr.UpdateStatus(order.ID, StatusFulfilled); err != nil {
		t.Fatalf("->fulfilled: %v", err)
	}
	return order
}

// A correctly settled order — ledger entry present with the right amount, intact
// lineage — must reconcile. This proves the check passes for a REAL entry, not
// because the flags were hard-coded.
func TestVerifyOrderSettlement_Reconciles(t *testing.T) {
	ctx := context.Background()
	mgr := NewInMemoryOrderManager()
	ledger := cbdc.NewInMemoryLedger()
	lin := lineage.NewInMemoryRecorder()

	order := fulfilledOrder(t, mgr, 5_000_000) // ₹50k
	settlementID := "settle-" + order.ID
	if _, err := ledger.CommitTransaction(settlementID, "merchant-recon", "settlement-pool", order.TotalPaise); err != nil {
		t.Fatalf("commit settlement: %v", err)
	}

	check, err := VerifyOrderSettlement(ctx, order.ID, settlementID, ledger, lin, mgr)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !check.IsReconciled() {
		t.Fatalf("expected reconciled, got issues=%v amountMatched=%v noDup=%v lineage=%v",
			check.Issues, check.AmountMatched, check.NoDuplicateSpend, check.LineageComplete)
	}
	if check.SettlementStatus != "settled" {
		t.Fatalf("settlement status: want settled, got %q", check.SettlementStatus)
	}
}

// The whole point: a ledger amount that disagrees with the order MUST be caught.
// The old hard-coded AmountMatched=true would have passed this silently.
func TestVerifyOrderSettlement_CatchesAmountMismatch(t *testing.T) {
	ctx := context.Background()
	mgr := NewInMemoryOrderManager()
	ledger := cbdc.NewInMemoryLedger()
	lin := lineage.NewInMemoryRecorder()

	order := fulfilledOrder(t, mgr, 5_000_000) // order says ₹50k
	settlementID := "settle-" + order.ID
	// Ledger committed the WRONG amount (₹40k).
	if _, err := ledger.CommitTransaction(settlementID, "merchant-recon", "settlement-pool", 4_000_000); err != nil {
		t.Fatalf("commit settlement: %v", err)
	}

	check, err := VerifyOrderSettlement(ctx, order.ID, settlementID, ledger, lin, mgr)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if check.AmountMatched {
		t.Fatal("amount mismatch must NOT be reported as matched")
	}
	if check.IsReconciled() {
		t.Fatal("a mismatched order must not reconcile")
	}
	if len(check.Issues) == 0 {
		t.Fatal("expected an amount-mismatch issue to be recorded")
	}
}

// A settlement that was never committed to the ledger must be flagged, not assumed.
func TestVerifyOrderSettlement_CatchesMissingLedgerEntry(t *testing.T) {
	ctx := context.Background()
	mgr := NewInMemoryOrderManager()
	ledger := cbdc.NewInMemoryLedger()
	lin := lineage.NewInMemoryRecorder()

	order := fulfilledOrder(t, mgr, 5_000_000)

	check, err := VerifyOrderSettlement(ctx, order.ID, "settle-"+order.ID, ledger, lin, mgr)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if check.NoDuplicateSpend || check.AmountMatched {
		t.Fatal("no ledger entry exists, yet ledger flags were set")
	}
	if check.IsReconciled() {
		t.Fatal("order with no ledger entry must not reconcile")
	}
}

func TestVerifyOrderSettlement_NilLedgerIsAnIssue(t *testing.T) {
	ctx := context.Background()
	mgr := NewInMemoryOrderManager()
	order := fulfilledOrder(t, mgr, 1_000_000)

	check, err := VerifyOrderSettlement(ctx, order.ID, "settle-x", nil, lineage.NewInMemoryRecorder(), mgr)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if check.IsReconciled() {
		t.Fatal("nil ledger must not reconcile")
	}
}
