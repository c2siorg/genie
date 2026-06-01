// Package commerce defines types and orchestration for order → payment → settlement
// automation. Orders flow through a state machine: created → paid → fulfilled,
// with rollback on payment or settlement failures.
package commerce

import (
	"encoding/json"
	"time"
)

// OrderStatus represents the current state of an order.
type OrderStatus string

const (
	StatusPending      OrderStatus = "pending"      // order created, awaiting payment
	StatusPaid         OrderStatus = "paid"         // payment confirmed
	StatusFulfilled    OrderStatus = "fulfilled"    // order fulfilled to customer
	StatusCancelled    OrderStatus = "cancelled"    // order cancelled or reverted
	StatusPaymentFailed OrderStatus = "payment_failed" // payment failed, awaiting retry/manual intervention
)

// WorkflowStep represents a phase in the order-to-fulfillment pipeline.
type WorkflowStep string

const (
	StepOrderCreated      WorkflowStep = "order_created"
	StepPaymentInitiated  WorkflowStep = "payment_initiated"
	StepPaymentConfirmed  WorkflowStep = "payment_confirmed"
	StepSettlementInitiated WorkflowStep = "settlement_initiated"
	StepSettlementCompleted WorkflowStep = "settlement_completed"
	StepFulfilled         WorkflowStep = "fulfilled"
)

// OrderItem is one line item in an order.
type OrderItem struct {
	SKU             string `json:"sku"`
	Description     string `json:"description"`
	Quantity        int    `json:"quantity"`
	UnitPricePaise  int64  `json:"unit_price_paise"` // price in paise (₹0.01 units)
}

// Order represents a customer order with items and payment status.
type Order struct {
	// ID uniquely identifies this order
	ID string `json:"id"`
	// MerchantID identifies the merchant selling the items
	MerchantID string `json:"merchant_id"`
	// CustomerID identifies the customer placing the order
	CustomerID string `json:"customer_id"`
	// Items line items in the order
	Items []OrderItem `json:"items"`
	// TotalPaise total order amount in paise
	TotalPaise int64 `json:"total_paise"`
	// Status current order status
	Status OrderStatus `json:"status"`
	// CreatedAt when the order was placed
	CreatedAt time.Time `json:"created_at"`
	// PaidAt when payment was confirmed (zero if not yet paid)
	PaidAt time.Time `json:"paid_at,omitempty"`
}

// CommerceWorkflow tracks an order through the state machine.
type CommerceWorkflow struct {
	// OrderID references the Order being processed
	OrderID string `json:"order_id"`
	// Steps executed so far
	Steps []WorkflowStep `json:"steps"`
	// CurrentStep the step currently being executed
	CurrentStep WorkflowStep `json:"current_step"`
	// Status overall workflow status (maps to OrderStatus)
	Status OrderStatus `json:"status"`
	// CreatedAt when this workflow was instantiated
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt most recent state transition
	UpdatedAt time.Time `json:"updated_at"`
	// ErrorMessage if a step failed, the error detail
	ErrorMessage string `json:"error_message,omitempty"`
	// RequiresHITL if true, escalated for human review (e.g., settlement failure)
	RequiresHITL bool `json:"requires_hitl"`
	// Metadata arbitrary key-value context
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PaymentInitiationRequest is sent to the Payment Agent.
type PaymentInitiationRequest struct {
	OrderID    string `json:"order_id"`
	MerchantID string `json:"merchant_id"`
	CustomerID string `json:"customer_id"`
	AmountPaise int64  `json:"amount_paise"`
	Currency   string `json:"currency"` // e.g., "INR"
}

// PaymentConfirmation is received from Payment Agent / CBDC Bridge polling.
type PaymentConfirmation struct {
	OrderID       string    `json:"order_id"`
	TransactionID string    `json:"transaction_id"`
	Success       bool      `json:"success"`
	ConfirmedAt   time.Time `json:"confirmed_at"`
	ErrorReason   string    `json:"error_reason,omitempty"`
}

// SettlementInitiationRequest is sent to Settlement Coordinator.
type SettlementInitiationRequest struct {
	OrderID       string `json:"order_id"`
	TransactionID string `json:"transaction_id"`
	MerchantID    string `json:"merchant_id"`
	AmountPaise   int64  `json:"amount_paise"`
	Currency      string `json:"currency"`
}

// SettlementResult is received from Settlement Coordinator.
type SettlementResult struct {
	OrderID        string    `json:"order_id"`
	SettlementID   string    `json:"settlement_id"`
	Success        bool      `json:"success"`
	CompletedAt    time.Time `json:"completed_at"`
	ErrorReason    string    `json:"error_reason,omitempty"`
	RequiresReview bool      `json:"requires_review"` // escalate to HITL on settlement failure
}

// AuditEntry records one step or action in the workflow.
type AuditEntry struct {
	// ID uniquely identifies this audit entry
	ID string `json:"id"`
	// OrderID references the parent Order
	OrderID string `json:"order_id"`
	// Step the workflow step that executed
	Step WorkflowStep `json:"step"`
	// Action description (e.g., "payment_initiated", "payment_confirmed")
	Action string `json:"action"`
	// Input serialized input (JSON)
	Input json.RawMessage `json:"input,omitempty"`
	// Output serialized output (JSON)
	Output json.RawMessage `json:"output,omitempty"`
	// Timestamp when the action occurred
	Timestamp time.Time `json:"timestamp"`
	// Error message if the action failed
	Error string `json:"error,omitempty"`
}
