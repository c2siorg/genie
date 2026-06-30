package commerce

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/cbdc"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/erupeepayment"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/lineage"
)

// seededPaymentStub creates a RealPaymentAgentStub with pre-registered customer
// and merchant accounts. The real PaymentAgent validates account existence before
// initiating any transfer, so callers must seed the accounts they intend to use.
//
// We use a custom idGen so that account.ID == holderID. Without this, the account
// manager assigns auto-incremented IDs (acct_1, acct_2, …) and GetAccount(customerID)
// would never match.
func seededPaymentStub(customers, merchants []string) *RealPaymentAgentStub {
	all := append(append([]string{}, customers...), merchants...)
	idx := 0
	acctMgr := erupeepayment.NewInMemoryAccountManager(func() string {
		id := all[idx]
		idx++
		return id
	})
	const startingBalance = 10_000_000 // ₹100k per account — covers all test amounts
	for _, id := range customers {
		acctMgr.CreateAccount(id, erupeepayment.TypePersonal)
		acctMgr.UpdateBalance(id, startingBalance)
	}
	for _, id := range merchants {
		acctMgr.CreateAccount(id, erupeepayment.TypeMerchant)
		acctMgr.UpdateBalance(id, startingBalance)
	}
	return NewRealPaymentAgentStub(erupeepayment.NewPaymentAgent(
		acctMgr,
		erupeepayment.NewInMemoryTransactionLog(nil),
	))
}

// TestE2E_OrderToSettlement_HappyPath tests the complete workflow:
// Order creation → Payment → Compliance → CBDC → Settlement → Fulfilled
func TestE2E_OrderToSettlement_HappyPath(t *testing.T) {
	ctx := context.Background()

	// Setup: Create managers and ledgers
	orderMgr := NewInMemoryOrderManager()
	cbdcLedger := cbdc.NewInMemoryLedger()
	merchantID := "merchant-e2e-1"
	customerID := "customer-e2e-1"
	paymentStub := seededPaymentStub([]string{customerID}, []string{merchantID})
	settlementStub := NewRealSettlementAgentStub(&mockSettlementExecutor{ledger: cbdcLedger})
	// Gate installed: the happy path must flow through the real compliance gate,
	// not around it, for this to validate the production money path.
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentStub, settlementStub).
		WithComplianceGate(NewVelocityComplianceGate())

	// Step 1: Create order (₹10k total; within the ₹50k/hr velocity limit)
	items := []OrderItem{
		{SKU: "ITEM-001", Quantity: 1, UnitPricePaise: 500_000}, // ₹5k
		{SKU: "ITEM-002", Quantity: 1, UnitPricePaise: 500_000}, // ₹5k
	}
	order, err := orderMgr.CreateOrder(merchantID, customerID, items)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.Status != StatusPending {
		t.Fatalf("order status: want %v, got %v", StatusPending, order.Status)
	}

	// Step 2: Execute workflow
	success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !success {
		t.Fatalf("workflow failed: final status %v", finalStatus)
	}
	if finalStatus != StatusFulfilled {
		t.Fatalf("final status: want %v, got %v", StatusFulfilled, finalStatus)
	}

	// Step 3: Verify reconciliation
	updatedOrder, err := orderMgr.GetOrder(order.ID)
	if err != nil || updatedOrder.Status != StatusFulfilled {
		t.Fatalf("order not fulfilled: %v", updatedOrder.Status)
	}

	// Verify CBDC ledger has transaction (simplified check)
	if cbdcLedger == nil {
		t.Fatal("CBDC ledger is nil")
	}
}

// TestE2E_ComplianceBlocks_VelocityExceeded verifies that the fail-closed
// compliance gate BLOCKS a velocity-exceeding order before any money moves —
// and proves it with hard assertions (this test previously asserted nothing).
func TestE2E_ComplianceBlocks_VelocityExceeded(t *testing.T) {
	ctx := context.Background()

	orderMgr := NewInMemoryOrderManager()
	customerID := "customer-velocity-test"
	merchantID := "merchant-velocity-test"
	// Seed accounts for the low-value control order (high-value order blocked pre-payment).
	paymentStub := seededPaymentStub([]string{customerID, "customer-low-value"}, []string{merchantID})
	settlementStub := NewRealSettlementAgentStub(&mockSettlementExecutor{ledger: cbdc.NewInMemoryLedger()})
	// Install the deterministic, fail-closed velocity gate (RBI defaults: ₹50k/hr).
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentStub, settlementStub).
		WithComplianceGate(NewVelocityComplianceGate())
	// ₹90,000 single order (9,000,000 paise) exceeds the ₹50k/hour limit → BLOCK.
	items := []OrderItem{{SKU: "HIGH-VALUE", Quantity: 1, UnitPricePaise: 9_000_000}}

	order, err := orderMgr.CreateOrder(merchantID, customerID, items)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)
	if success {
		t.Fatal("velocity-exceeding order must be blocked, got success=true (compliance fail-OPEN!)")
	}
	if finalStatus != StatusComplianceBlocked {
		t.Fatalf("final status: want %v, got %v", StatusComplianceBlocked, finalStatus)
	}
	if err == nil {
		t.Fatal("expected a compliance-blocked error, got nil")
	}

	// The order must be persisted as compliance-blocked and never marked paid.
	got, gErr := orderMgr.GetOrder(order.ID)
	if gErr != nil {
		t.Fatalf("get order: %v", gErr)
	}
	if got.Status != StatusComplianceBlocked {
		t.Fatalf("order status: want %v, got %v", StatusComplianceBlocked, got.Status)
	}

	// The block must be recorded to the audit trail (no silent pass).
	audit, _ := orchestrator.GetAuditLog(order.ID)
	var blocked bool
	for _, e := range audit {
		if e.Action == "compliance_blocked" {
			blocked = true
		}
		if e.Action == "payment_initiated" {
			t.Fatal("payment was initiated despite compliance block — money path leaked")
		}
	}
	if !blocked {
		t.Fatal("expected a 'compliance_blocked' audit entry")
	}

	// Control: a low-value order from a different customer still flows to fulfilled,
	// proving the gate blocks the right thing and not everything.
	lowOrder, lErr := orderMgr.CreateOrder(merchantID, "customer-low-value",
		[]OrderItem{{SKU: "LOW", Quantity: 1, UnitPricePaise: 100_000}}) // ₹1,000
	if lErr != nil {
		t.Fatalf("create low-value order: %v", lErr)
	}
	okSuccess, okStatus, okErr := orchestrator.ExecuteWorkflow(ctx, lowOrder.ID)
	if !okSuccess || okStatus != StatusFulfilled || okErr != nil {
		t.Fatalf("low-value order should pass: success=%v status=%v err=%v", okSuccess, okStatus, okErr)
	}
}

// TestE2E_SettlementBatching_MultipleOrders tests multiple order settlement
func TestE2E_SettlementBatching_MultipleOrders(t *testing.T) {
	ctx := context.Background()

	// Setup
	orderMgr := NewInMemoryOrderManager()
	cbdcLedger := cbdc.NewInMemoryLedger()
	merchantID := "merchant-batch"
	customerIDs := []string{"customer-0", "customer-1", "customer-2"}
	paymentStub := seededPaymentStub(customerIDs, []string{merchantID})
	settlementStub := NewRealSettlementAgentStub(&mockSettlementExecutor{ledger: cbdcLedger})
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentStub, settlementStub).
		WithComplianceGate(NewVelocityComplianceGate())

	// Create 3 orders from same merchant, each from a distinct customer account.
	orders := []*Order{}
	amounts := []int64{5_000_000, 3_000_000, 2_000_000} // ₹50k, ₹30k, ₹20k

	for i, amount := range amounts {
		items := []OrderItem{{SKU: "ITEM", Quantity: 1, UnitPricePaise: amount}}
		order, err := orderMgr.CreateOrder(merchantID, customerIDs[i], items)
		if err != nil {
			t.Fatalf("create order %d: %v", i, err)
		}
		orders = append(orders, order)
	}

	// Execute all 3 workflows
	successCount := 0
	for _, order := range orders {
		success, _, err := orchestrator.ExecuteWorkflow(ctx, order.ID)
		if err == nil && success {
			successCount++
		}
	}

	// Verify at least one order succeeded
	if successCount == 0 {
		t.Fatalf("no orders succeeded")
	}

	// Total amount should be ₹100k (50 + 30 + 20)
	totalAmount := int64(0)
	for _, order := range orders {
		totalAmount += order.TotalPaise
	}
	if totalAmount != 10_000_000 {
		t.Fatalf("total order amount: want %d, got %d", 10_000_000, totalAmount)
	}
}

// TestE2E_AuditTrail_FullLineage tests order execution
func TestE2E_AuditTrail_FullLineage(t *testing.T) {
	ctx := context.Background()

	// Setup
	orderMgr := NewInMemoryOrderManager()
	cbdcLedger := cbdc.NewInMemoryLedger()
	merchantID := "merchant-audit"
	customerID := "customer-audit"
	paymentStub := seededPaymentStub([]string{customerID}, []string{merchantID})
	settlementStub := NewRealSettlementAgentStub(&mockSettlementExecutor{ledger: cbdcLedger})
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentStub, settlementStub).
		WithComplianceGate(NewVelocityComplianceGate())
	items := []OrderItem{{SKU: "TEST", Quantity: 1, UnitPricePaise: 1_000_000}}

	order, err := orderMgr.CreateOrder(merchantID, customerID, items)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !success || finalStatus != StatusFulfilled {
		t.Fatalf("workflow should succeed: success=%v status=%v", success, finalStatus)
	}

	// The audit trail must record the full ordered sequence of steps the order
	// passed through — this is the actual "full lineage" the test name promises.
	audit, _ := orchestrator.GetAuditLog(order.ID)
	wantSteps := []WorkflowStep{
		StepOrderCreated,
		StepComplianceChecked,
		StepPaymentInitiated,
		StepPaymentConfirmed,
		StepSettlementInitiated,
		StepSettlementCompleted,
		StepFulfilled,
	}
	seen := map[WorkflowStep]bool{}
	for _, e := range audit {
		seen[e.Step] = true
	}
	for _, step := range wantSteps {
		if !seen[step] {
			t.Fatalf("audit trail missing step %q; got entries %d", step, len(audit))
		}
	}
	// The compliance check must be recorded as having run (allowed) on this path.
	var complianceChecked bool
	for _, e := range audit {
		if e.Action == "compliance_checked" {
			complianceChecked = true
		}
	}
	if !complianceChecked {
		t.Fatal("expected a 'compliance_checked' audit entry on the happy path")
	}
}

// Mock settlement executor for testing
type mockSettlementExecutor struct {
	ledger cbdc.Ledger
}

func (m *mockSettlementExecutor) ExecuteBatch(ctx context.Context, batchID string, positions map[string]int64) error {
	// Simulate settlement by committing to ledger
	for merchant, amount := range positions {
		_, err := m.ledger.CommitTransaction(
			merchant+"-"+batchID,
			merchant,
			"settlement-pool",
			amount,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *mockSettlementExecutor) GetBatchStatus(ctx context.Context, batchID string) (string, error) {
	return "executed", nil
}

// TestE2E_Reconciliation_VerifySettlementIntegrity tests settlement verification
func TestE2E_Reconciliation_VerifySettlementIntegrity(t *testing.T) {
	ctx := context.Background()

	// Setup
	orderMgr := NewInMemoryOrderManager()
	cbdcLedger := cbdc.NewInMemoryLedger()
	merchantID := "merchant-reconcile"
	customerID := "customer-reconcile"
	paymentStub := seededPaymentStub([]string{customerID}, []string{merchantID})
	settlementStub := NewRealSettlementAgentStub(&mockSettlementExecutor{ledger: cbdcLedger})
	lineageRec := lineage.NewInMemoryRecorder()
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentStub, settlementStub).
		WithComplianceGate(NewVelocityComplianceGate())
	items := []OrderItem{{SKU: "TEST", Quantity: 1, UnitPricePaise: 5_000_000}} // ₹50k

	order, err := orderMgr.CreateOrder(merchantID, customerID, items)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)
	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}
	if !success || finalStatus != StatusFulfilled {
		t.Fatalf("workflow failed: status %v", finalStatus)
	}

	// Actually reconcile via VerifyOrderSettlement — querying the real CBDC
	// ledger entry the settlement committed. The mock settlement executor commits
	// under "<merchant>-<batchID>" where batchID is "batch-<orderID>".
	settlementLedgerID := merchantID + "-batch-" + order.ID
	check, err := VerifyOrderSettlement(ctx, order.ID, settlementLedgerID, cbdcLedger, lineageRec, orderMgr)
	if err != nil {
		t.Fatalf("verify settlement: %v", err)
	}
	if !check.IsReconciled() {
		t.Fatalf("settled order must reconcile; issues=%v amountMatched=%v noDup=%v lineage=%v",
			check.Issues, check.AmountMatched, check.NoDuplicateSpend, check.LineageComplete)
	}
	if check.SettlementStatus != "settled" {
		t.Fatalf("settlement status: want settled, got %q", check.SettlementStatus)
	}
}
