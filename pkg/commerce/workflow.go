package commerce

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PaymentAgentStub simulates calling the Payment Agent.
// In production, this would send a message via the agent bus.
type PaymentAgentStub interface {
	// InitiatePayment sends a payment request and returns a TransactionID
	InitiatePayment(ctx context.Context, req PaymentInitiationRequest) (transactionID string, err error)
	// PollPaymentStatus checks if payment was confirmed
	PollPaymentStatus(ctx context.Context, transactionID string) (*PaymentConfirmation, error)
}

// SettlementAgentStub simulates calling the Settlement Coordinator.
// In production, this would send a message via the agent bus.
type SettlementAgentStub interface {
	// InitiateSettlement sends a settlement request
	InitiateSettlement(ctx context.Context, req SettlementInitiationRequest) (settlementID string, err error)
	// WaitSettlementCompletion polls for completion
	WaitSettlementCompletion(ctx context.Context, settlementID string) (*SettlementResult, error)
}

// WorkflowOrchestrator orchestrates the order → payment → settlement lifecycle.
type WorkflowOrchestrator interface {
	// ExecuteWorkflow runs the complete order workflow
	ExecuteWorkflow(ctx context.Context, orderID string) (success bool, finalStatus OrderStatus, err error)
	// GetWorkflow retrieves the current workflow state
	GetWorkflow(orderID string) (*CommerceWorkflow, error)
	// GetAuditLog retrieves audit entries for an order
	GetAuditLog(orderID string) ([]*AuditEntry, error)
}

// DefaultWorkflowOrchestrator implements the order automation workflow.
type DefaultWorkflowOrchestrator struct {
	orderMgr          OrderManager
	paymentAgent      PaymentAgentStub
	settlementAgent   SettlementAgentStub
	mu                sync.RWMutex
	workflows         map[string]*CommerceWorkflow
	auditLog          map[string][]*AuditEntry // keyed by orderID
	maxPaymentRetries int
	paymentPollWait   time.Duration
}

// NewDefaultWorkflowOrchestrator creates a new orchestrator.
func NewDefaultWorkflowOrchestrator(
	orderMgr OrderManager,
	paymentAgent PaymentAgentStub,
	settlementAgent SettlementAgentStub,
) *DefaultWorkflowOrchestrator {
	return &DefaultWorkflowOrchestrator{
		orderMgr:          orderMgr,
		paymentAgent:      paymentAgent,
		settlementAgent:   settlementAgent,
		workflows:         make(map[string]*CommerceWorkflow),
		auditLog:          make(map[string][]*AuditEntry),
		maxPaymentRetries: 3,
		paymentPollWait:   500 * time.Millisecond,
	}
}

// ExecuteWorkflow orchestrates the complete workflow for an order.
// Steps:
// 1. Create order (already done before calling this)
// 2. Initiate payment with Payment Agent
// 3. Poll for payment confirmation (up to maxPaymentRetries)
// 4. On payment success, initiate settlement with Settlement Coordinator
// 5. Poll settlement until completion
// 6. Mark order fulfilled
// On failure, logs error and marks for HITL review (settlement) or retry (payment).
func (o *DefaultWorkflowOrchestrator) ExecuteWorkflow(ctx context.Context, orderID string) (bool, OrderStatus, error) {
	order, err := o.orderMgr.GetOrder(orderID)
	if err != nil {
		return false, "", fmt.Errorf("order not found: %w", err)
	}

	// Initialize workflow
	workflow := &CommerceWorkflow{
		OrderID:     orderID,
		Steps:       []WorkflowStep{},
		CurrentStep: StepOrderCreated,
		Status:      StatusPending,
		CreatedAt:   time.Now().UTC(),
		Metadata:    make(map[string]string),
	}

	o.mu.Lock()
	o.workflows[orderID] = workflow
	auditLog := []*AuditEntry{}
	o.auditLog[orderID] = auditLog
	o.mu.Unlock()

	// Step 1: order_created
	o.recordAudit(orderID, StepOrderCreated, "order_created", order, nil, "")

	// Step 2: payment_initiated
	paymentReq := PaymentInitiationRequest{
		OrderID:     orderID,
		MerchantID:  order.MerchantID,
		CustomerID:  order.CustomerID,
		AmountPaise: order.TotalPaise,
		Currency:    "INR",
	}

	transactionID, err := o.paymentAgent.InitiatePayment(ctx, paymentReq)
	if err != nil {
		o.recordAudit(orderID, StepPaymentInitiated, "payment_initiated_failed", paymentReq, nil, err.Error())
		o.updateWorkflow(orderID, StepPaymentInitiated, StatusPaymentFailed, err.Error(), false)
		_, _ = o.orderMgr.UpdateStatus(orderID, StatusPaymentFailed)
		return false, StatusPaymentFailed, fmt.Errorf("payment initiation failed: %w", err)
	}
	o.recordAudit(orderID, StepPaymentInitiated, "payment_initiated", paymentReq, map[string]string{"transaction_id": transactionID}, "")

	// Step 3: poll for payment_confirmed
	var confirmation *PaymentConfirmation
	for attempt := 0; attempt < o.maxPaymentRetries; attempt++ {
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		case <-time.After(o.paymentPollWait):
		}
		confirmation, err = o.paymentAgent.PollPaymentStatus(ctx, transactionID)
		if err == nil && confirmation != nil && confirmation.Success {
			break
		}
		if err != nil {
			o.recordAudit(orderID, StepPaymentConfirmed, fmt.Sprintf("payment_poll_attempt_%d_failed", attempt+1), map[string]string{"transaction_id": transactionID}, nil, err.Error())
		}
	}

	if confirmation == nil || !confirmation.Success {
		reason := "payment confirmation timeout"
		if confirmation != nil {
			reason = confirmation.ErrorReason
		}
		o.recordAudit(orderID, StepPaymentConfirmed, "payment_confirmed_failed", map[string]string{"transaction_id": transactionID}, nil, reason)
		o.updateWorkflow(orderID, StepPaymentConfirmed, StatusPaymentFailed, reason, false)
		_, _ = o.orderMgr.UpdateStatus(orderID, StatusPaymentFailed)
		return false, StatusPaymentFailed, fmt.Errorf("payment confirmation failed: %s", reason)
	}

	o.recordAudit(orderID, StepPaymentConfirmed, "payment_confirmed", paymentReq, confirmation, "")

	// Update order to paid
	_, err = o.orderMgr.UpdateStatus(orderID, StatusPaid)
	if err != nil {
		return false, "", fmt.Errorf("failed to mark order as paid: %w", err)
	}

	// Step 4: settlement_initiated
	settlementReq := SettlementInitiationRequest{
		OrderID:       orderID,
		TransactionID: transactionID,
		MerchantID:    order.MerchantID,
		AmountPaise:   order.TotalPaise,
		Currency:      "INR",
	}

	settlementID, err := o.settlementAgent.InitiateSettlement(ctx, settlementReq)
	if err != nil {
		o.recordAudit(orderID, StepSettlementInitiated, "settlement_initiated_failed", settlementReq, nil, err.Error())
		o.updateWorkflow(orderID, StepSettlementInitiated, StatusPaymentFailed, err.Error(), true) // HITL
		return false, StatusPaid, fmt.Errorf("settlement initiation failed: %w", err)
	}
	o.recordAudit(orderID, StepSettlementInitiated, "settlement_initiated", settlementReq, map[string]string{"settlement_id": settlementID}, "")

	// Step 5: settlement_completed
	settlementResult, err := o.settlementAgent.WaitSettlementCompletion(ctx, settlementID)
	if err != nil || settlementResult == nil || !settlementResult.Success {
		reason := "settlement completion timeout"
		if settlementResult != nil {
			reason = settlementResult.ErrorReason
		}
		if err != nil {
			reason = err.Error()
		}
		o.recordAudit(orderID, StepSettlementCompleted, "settlement_completed_failed", map[string]string{"settlement_id": settlementID}, nil, reason)
		o.updateWorkflow(orderID, StepSettlementCompleted, StatusPaid, reason, true) // Escalate to HITL
		return false, StatusPaid, fmt.Errorf("settlement failed: %s", reason)
	}

	o.recordAudit(orderID, StepSettlementCompleted, "settlement_completed", settlementReq, settlementResult, "")

	// Step 6: fulfilled
	_, err = o.orderMgr.UpdateStatus(orderID, StatusFulfilled)
	if err != nil {
		return false, "", fmt.Errorf("failed to mark order as fulfilled: %w", err)
	}

	o.recordAudit(orderID, StepFulfilled, "fulfilled", order, nil, "")

	o.updateWorkflow(orderID, StepFulfilled, StatusFulfilled, "", false)
	return true, StatusFulfilled, nil
}

// GetWorkflow returns the current workflow state for an order.
func (o *DefaultWorkflowOrchestrator) GetWorkflow(orderID string) (*CommerceWorkflow, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	workflow, ok := o.workflows[orderID]
	if !ok {
		return nil, fmt.Errorf("workflow not found for order: %s", orderID)
	}
	// Return a copy
	copy := *workflow
	return &copy, nil
}

// GetAuditLog returns all audit entries for an order.
func (o *DefaultWorkflowOrchestrator) GetAuditLog(orderID string) ([]*AuditEntry, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	entries, ok := o.auditLog[orderID]
	if !ok {
		return []*AuditEntry{}, nil
	}
	// Return a copy
	entriesCopy := make([]*AuditEntry, len(entries))
	for i, e := range entries {
		copy := *e
		entriesCopy[i] = &copy
	}
	return entriesCopy, nil
}

// recordAudit logs an action with input/output.
func (o *DefaultWorkflowOrchestrator) recordAudit(orderID string, step WorkflowStep, action string, input, output interface{}, errMsg string) {
	entry := &AuditEntry{
		ID:        uuid.New().String(),
		OrderID:   orderID,
		Step:      step,
		Action:    action,
		Timestamp: time.Now().UTC(),
		Error:     errMsg,
	}

	if input != nil {
		if b, err := json.Marshal(input); err == nil {
			entry.Input = b
		}
	}
	if output != nil {
		if b, err := json.Marshal(output); err == nil {
			entry.Output = b
		}
	}

	o.mu.Lock()
	o.auditLog[orderID] = append(o.auditLog[orderID], entry)
	o.mu.Unlock()
}

// updateWorkflow updates the current workflow state.
func (o *DefaultWorkflowOrchestrator) updateWorkflow(orderID string, currentStep WorkflowStep, status OrderStatus, errMsg string, requiresHITL bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if workflow, ok := o.workflows[orderID]; ok {
		workflow.Steps = append(workflow.Steps, currentStep)
		workflow.CurrentStep = currentStep
		workflow.Status = status
		workflow.UpdatedAt = time.Now().UTC()
		workflow.ErrorMessage = errMsg
		workflow.RequiresHITL = requiresHITL
	}
}
