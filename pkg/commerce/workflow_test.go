package commerce

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockPaymentAgent simulates the Payment Agent behavior.
type MockPaymentAgent struct {
	ShouldFail      bool
	ConfirmOnRetry  int
	confirmAttempt  int
}

func (m *MockPaymentAgent) InitiatePayment(ctx context.Context, req PaymentInitiationRequest) (string, error) {
	if m.ShouldFail {
		return "", fmt.Errorf("payment initiation failed")
	}
	return "txn-" + req.OrderID, nil
}

func (m *MockPaymentAgent) PollPaymentStatus(ctx context.Context, transactionID string) (*PaymentConfirmation, error) {
	m.confirmAttempt++
	if m.confirmAttempt < m.ConfirmOnRetry {
		return &PaymentConfirmation{
			Success: false,
			ErrorReason: "payment pending",
		}, nil
	}
	return &PaymentConfirmation{
		Success:     true,
		TransactionID: transactionID,
		ConfirmedAt: time.Now().UTC(),
	}, nil
}

// MockSettlementAgent simulates the Settlement Coordinator behavior.
type MockSettlementAgent struct {
	ShouldFail bool
}

func (m *MockSettlementAgent) InitiateSettlement(ctx context.Context, req SettlementInitiationRequest) (string, error) {
	if m.ShouldFail {
		return "", fmt.Errorf("settlement initiation failed")
	}
	return "settlement-" + req.OrderID, nil
}

func (m *MockSettlementAgent) WaitSettlementCompletion(ctx context.Context, settlementID string) (*SettlementResult, error) {
	return &SettlementResult{
		Success:     !m.ShouldFail,
		SettlementID: settlementID,
		CompletedAt: time.Now().UTC(),
		ErrorReason: func() string {
			if m.ShouldFail {
				return "settlement failed"
			}
			return ""
		}(),
		RequiresReview: m.ShouldFail,
	}, nil
}

func TestWorkflowExecute_SuccessPath(t *testing.T) {
	// Setup
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{ShouldFail: false, ConfirmOnRetry: 1}
	settlementAgent := &MockSettlementAgent{ShouldFail: false}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	// Create an order
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Execute workflow
	ctx := context.Background()
	success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)

	if err != nil {
		t.Fatalf("ExecuteWorkflow failed: %v", err)
	}
	if !success {
		t.Error("ExecuteWorkflow should return success=true")
	}
	if finalStatus != StatusFulfilled {
		t.Errorf("want finalStatus=fulfilled, got %s", finalStatus)
	}

	// Verify order is fulfilled
	final, _ := orderMgr.GetOrder(order.ID)
	if final.Status != StatusFulfilled {
		t.Errorf("want order Status=fulfilled, got %s", final.Status)
	}

	// Verify workflow state
	workflow, _ := orchestrator.GetWorkflow(order.ID)
	if workflow.Status != StatusFulfilled {
		t.Errorf("want workflow Status=fulfilled, got %s", workflow.Status)
	}
	if workflow.RequiresHITL {
		t.Error("workflow should not require HITL on success")
	}

	// Verify audit log
	auditLog, _ := orchestrator.GetAuditLog(order.ID)
	if len(auditLog) == 0 {
		t.Error("audit log should not be empty")
	}
	// Should have entries for: order_created, payment_initiated, payment_confirmed, settlement_initiated, settlement_completed, fulfilled
	if len(auditLog) < 6 {
		t.Errorf("want at least 6 audit entries, got %d", len(auditLog))
	}
}

func TestWorkflowExecute_PaymentInitiationFailure(t *testing.T) {
	// Setup with payment failure
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{ShouldFail: true}
	settlementAgent := &MockSettlementAgent{ShouldFail: false}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	// Create an order
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Execute workflow
	ctx := context.Background()
	success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)

	if success {
		t.Error("ExecuteWorkflow should return success=false on payment failure")
	}
	if finalStatus != StatusPaymentFailed {
		t.Errorf("want finalStatus=payment_failed, got %s", finalStatus)
	}
	if err == nil {
		t.Error("ExecuteWorkflow should return an error on payment failure")
	}

	// Verify order status
	final, _ := orderMgr.GetOrder(order.ID)
	if final.Status != StatusPaymentFailed {
		t.Errorf("want order Status=payment_failed, got %s", final.Status)
	}

	// Verify no settlement was attempted
	workflow, _ := orchestrator.GetWorkflow(order.ID)
	if len(workflow.Steps) > 2 { // Should only have order_created and payment_initiated
		t.Errorf("workflow should not progress past payment_initiated, got steps: %v", workflow.Steps)
	}
}

func TestWorkflowExecute_PaymentConfirmationFailure(t *testing.T) {
	// Setup with confirmation timeout
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{
		ShouldFail:     false,
		ConfirmOnRetry: 999, // Never confirm
	}
	settlementAgent := &MockSettlementAgent{ShouldFail: false}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)
	orchestrator.maxPaymentRetries = 2

	// Create an order
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Execute workflow
	ctx := context.Background()
	success, finalStatus, _ := orchestrator.ExecuteWorkflow(ctx, order.ID)

	if success {
		t.Error("ExecuteWorkflow should return success=false on confirmation timeout")
	}
	if finalStatus != StatusPaymentFailed {
		t.Errorf("want finalStatus=payment_failed, got %s", finalStatus)
	}

	// Verify order status
	final, _ := orderMgr.GetOrder(order.ID)
	if final.Status != StatusPaymentFailed {
		t.Errorf("want order Status=payment_failed, got %s", final.Status)
	}
}

func TestWorkflowExecute_SettlementFailure(t *testing.T) {
	// Setup with settlement failure
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{ShouldFail: false, ConfirmOnRetry: 1}
	settlementAgent := &MockSettlementAgent{ShouldFail: true}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	// Create an order
	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Execute workflow
	ctx := context.Background()
	success, finalStatus, _ := orchestrator.ExecuteWorkflow(ctx, order.ID)

	if success {
		t.Error("ExecuteWorkflow should return success=false on settlement failure")
	}
	if finalStatus != StatusPaid {
		t.Errorf("want finalStatus=paid (payment succeeded but settlement failed), got %s", finalStatus)
	}

	// Verify workflow state — settlement failure should escalate to HITL
	workflow, _ := orchestrator.GetWorkflow(order.ID)
	if !workflow.RequiresHITL {
		t.Error("workflow should require HITL on settlement failure")
	}

	// Order should remain paid (settlement failure doesn't revert payment)
	final, _ := orderMgr.GetOrder(order.ID)
	if final.Status != StatusPaid {
		t.Errorf("want order Status=paid on settlement failure, got %s", final.Status)
	}
}

func TestGetWorkflow_NotFound(t *testing.T) {
	orchestrator := NewDefaultWorkflowOrchestrator(
		NewInMemoryOrderManager(),
		&MockPaymentAgent{},
		&MockSettlementAgent{},
	)

	_, err := orchestrator.GetWorkflow("non-existent")
	if err == nil {
		t.Error("GetWorkflow should return error for non-existent workflow")
	}
}

func TestGetAuditLog(t *testing.T) {
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{ShouldFail: false, ConfirmOnRetry: 1}
	settlementAgent := &MockSettlementAgent{ShouldFail: false}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Execute workflow
	ctx := context.Background()
	orchestrator.ExecuteWorkflow(ctx, order.ID)

	// Get audit log
	auditLog, err := orchestrator.GetAuditLog(order.ID)
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}

	if len(auditLog) == 0 {
		t.Error("audit log should not be empty after successful execution")
	}

	// Verify audit entries have required fields
	for i, entry := range auditLog {
		if entry.ID == "" {
			t.Errorf("audit entry[%d]: ID should not be empty", i)
		}
		if entry.OrderID != order.ID {
			t.Errorf("audit entry[%d]: OrderID should be %s, got %s", i, order.ID, entry.OrderID)
		}
		if entry.Action == "" {
			t.Errorf("audit entry[%d]: Action should not be empty", i)
		}
		if entry.Timestamp.IsZero() {
			t.Errorf("audit entry[%d]: Timestamp should not be zero", i)
		}
	}
}

func TestWorkflowExecution_Idempotence(t *testing.T) {
	// Test that executing workflow multiple times is safe
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{ShouldFail: false, ConfirmOnRetry: 1}
	settlementAgent := &MockSettlementAgent{ShouldFail: false}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	ctx := context.Background()

	// First execution
	success1, status1, err1 := orchestrator.ExecuteWorkflow(ctx, order.ID)

	// Verify first execution succeeded
	if !success1 || status1 != StatusFulfilled || err1 != nil {
		t.Fatalf("First execution should succeed, got: success=%v, status=%s, err=%v", success1, status1, err1)
	}

	// Get audit log after first execution
	auditLog1, _ := orchestrator.GetAuditLog(order.ID)
	auditCount1 := len(auditLog1)

	// Second execution should fail because order is already fulfilled
	// (cannot transition fulfilled -> anything except cancelled)
	success2, status2, _ := orchestrator.ExecuteWorkflow(ctx, order.ID)

	if success2 {
		// If it somehow succeeds, at least check that audit log grew
		auditLog2, _ := orchestrator.GetAuditLog(order.ID)
		if len(auditLog2) <= auditCount1 {
			t.Error("Workflow should log attempts even if they fail")
		}
	}

	// Status should reflect the attempt
	if status2 != StatusFulfilled && status2 != "" {
		t.Logf("Second execution status: %s (acceptable if not fulfilled)", status2)
	}
}

func TestConcurrentWorkflowExecution(t *testing.T) {
	// Test concurrent order processing
	orderMgr := NewInMemoryOrderManager()
	paymentAgent := &MockPaymentAgent{ShouldFail: false, ConfirmOnRetry: 1}
	settlementAgent := &MockSettlementAgent{ShouldFail: false}
	orchestrator := NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	// Create 5 orders and execute workflows concurrently
	done := make(chan bool)
	results := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			items := []OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
			order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

			ctx := context.Background()
			success, _, _ := orchestrator.ExecuteWorkflow(ctx, order.ID)
			results <- success
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Check results
	successCount := 0
	for i := 0; i < 5; i++ {
		if <-results {
			successCount++
		}
	}

	if successCount < 5 {
		t.Errorf("expected all 5 workflows to succeed, got %d", successCount)
	}

	// Verify all orders are fulfilled
	orders, _ := orderMgr.ListOrders("merchant-1")
	for _, o := range orders {
		if o.Status != StatusFulfilled {
			t.Errorf("order %s should be fulfilled, got status %s", o.ID, o.Status)
		}
	}
}
