// Package handlers wires HTTP endpoints for the Commerce Workflow Engine.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commerce"
	"github.com/go-chi/chi/v5"
)

// CommerceHandler wires order and workflow management endpoints.
type CommerceHandler struct {
	orderManager         commerce.OrderManager
	workflowOrchestrator commerce.WorkflowOrchestrator
}

// NewCommerceHandler creates a new commerce handler.
func NewCommerceHandler(
	orderManager commerce.OrderManager,
	workflowOrchestrator commerce.WorkflowOrchestrator,
) *CommerceHandler {
	return &CommerceHandler{
		orderManager:         orderManager,
		workflowOrchestrator: workflowOrchestrator,
	}
}

// ─── Request/Response Types ────────────────────────────────────────────────

// createOrderRequest represents POST /v1/commerce/order body.
type createOrderRequest struct {
	MerchantID string               `json:"merchant_id"`
	CustomerID string               `json:"customer_id"`
	Items      []commerce.OrderItem `json:"items"`
}

// createOrderResponse represents the response to order creation.
type createOrderResponse struct {
	OrderID    string `json:"order_id"`
	TotalPaise int64  `json:"total_paise"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"created_at"` // Unix timestamp
}

// getOrderResponse represents GET /v1/commerce/order/{order_id} response.
type getOrderResponse struct {
	OrderID        string               `json:"order_id"`
	MerchantID     string               `json:"merchant_id"`
	CustomerID     string               `json:"customer_id"`
	Items          []commerce.OrderItem `json:"items"`
	TotalPaise     int64                `json:"total_paise"`
	Status         string               `json:"status"`
	WorkflowStatus string               `json:"workflow_status,omitempty"`
	CreatedAt      int64                `json:"created_at"`
	PaidAt         *int64               `json:"paid_at,omitempty"`
}

// executeWorkflowResponse represents the response to workflow execution.
type executeWorkflowResponse struct {
	OrderID        string       `json:"order_id"`
	WorkflowStatus string       `json:"workflow_status"`
	Steps          []stepStatus `json:"steps"`
	FinalStatus    string       `json:"final_status"`
	Error          string       `json:"error,omitempty"`
}

// stepStatus represents one step in the workflow response.
type stepStatus struct {
	StepName  string `json:"step_name"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"` // Unix timestamp
	Error     string `json:"error,omitempty"`
}

// auditTrailResponse represents GET /v1/commerce/order/{order_id}/audit response.
type auditTrailResponse struct {
	OrderID    string      `json:"order_id"`
	AuditTrail []auditItem `json:"audit_trail"`
}

// auditItem represents one entry in the audit log.
type auditItem struct {
	Step      string          `json:"step"`
	Timestamp int64           `json:"timestamp"` // Unix timestamp
	Input     json.RawMessage `json:"input,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// ─── HTTP Handlers ────────────────────────────────────────────────────────

// CreateOrder handles POST /v1/commerce/order.
// Creates a new order with the provided items and returns the order ID.
func (h *CommerceHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Validate request
	if strings.TrimSpace(req.MerchantID) == "" {
		http.Error(w, "merchant_id required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.CustomerID) == "" {
		http.Error(w, "customer_id required", http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		http.Error(w, "items required (at least one)", http.StatusBadRequest)
		return
	}

	// Create order
	order, err := h.orderManager.CreateOrder(req.MerchantID, req.CustomerID, req.Items)
	if err != nil {
		http.Error(w, "create order failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, createOrderResponse{
		OrderID:    order.ID,
		TotalPaise: order.TotalPaise,
		Status:     string(order.Status),
		CreatedAt:  order.CreatedAt.Unix(),
	})
}

// GetOrder handles GET /v1/commerce/order/{order_id}.
// Retrieves an order and its workflow status.
func (h *CommerceHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	if strings.TrimSpace(orderID) == "" {
		http.Error(w, "order_id required", http.StatusBadRequest)
		return
	}

	order, err := h.orderManager.GetOrder(orderID)
	if err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	resp := getOrderResponse{
		OrderID:    order.ID,
		MerchantID: order.MerchantID,
		CustomerID: order.CustomerID,
		Items:      order.Items,
		TotalPaise: order.TotalPaise,
		Status:     string(order.Status),
		CreatedAt:  order.CreatedAt.Unix(),
	}

	// Attach workflow status if available
	if workflow, err := h.workflowOrchestrator.GetWorkflow(orderID); err == nil {
		resp.WorkflowStatus = string(workflow.Status)
	}

	// Attach paid_at timestamp if order is paid
	if !order.PaidAt.IsZero() {
		paidAt := order.PaidAt.Unix()
		resp.PaidAt = &paidAt
	}

	respondJSON(w, http.StatusOK, resp)
}

// ExecuteWorkflow handles POST /v1/commerce/order/{order_id}/execute.
// Executes the complete workflow (payment + settlement) for an order.
func (h *CommerceHandler) ExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	if strings.TrimSpace(orderID) == "" {
		http.Error(w, "order_id required", http.StatusBadRequest)
		return
	}

	// Verify order exists
	if _, err := h.orderManager.GetOrder(orderID); err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	// Execute workflow with a timeout context
	ctx, cancel := context.WithTimeout(r.Context(), 30*1e9) // 30 seconds in nanoseconds
	defer cancel()

	success, finalStatus, err := h.workflowOrchestrator.ExecuteWorkflow(ctx, orderID)

	// Get the workflow to build the response
	workflow, getErr := h.workflowOrchestrator.GetWorkflow(orderID)
	if getErr != nil {
		http.Error(w, "failed to retrieve workflow status", http.StatusInternalServerError)
		return
	}

	// Build step list from workflow
	steps := make([]stepStatus, len(workflow.Steps))
	for i, step := range workflow.Steps {
		steps[i] = stepStatus{
			StepName:  string(step),
			Status:    "completed",
			Timestamp: workflow.UpdatedAt.Unix(),
		}
	}

	statusCode := http.StatusOK
	if !success {
		statusCode = http.StatusInternalServerError
	}

	resp := executeWorkflowResponse{
		OrderID:        orderID,
		WorkflowStatus: string(workflow.Status),
		Steps:          steps,
		FinalStatus:    string(finalStatus),
	}

	if err != nil {
		resp.Error = err.Error()
	}
	if workflow.ErrorMessage != "" && resp.Error == "" {
		resp.Error = workflow.ErrorMessage
	}

	respondJSON(w, statusCode, resp)
}

// GetAuditLog handles GET /v1/commerce/order/{order_id}/audit.
// Returns the complete audit trail for an order.
func (h *CommerceHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "order_id")
	if strings.TrimSpace(orderID) == "" {
		http.Error(w, "order_id required", http.StatusBadRequest)
		return
	}

	// Verify order exists
	if _, err := h.orderManager.GetOrder(orderID); err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	// Get audit log
	entries, err := h.workflowOrchestrator.GetAuditLog(orderID)
	if err != nil {
		http.Error(w, "failed to retrieve audit log", http.StatusInternalServerError)
		return
	}

	// Build response
	auditItems := make([]auditItem, len(entries))
	for i, entry := range entries {
		auditItems[i] = auditItem{
			Step:      string(entry.Step),
			Timestamp: entry.Timestamp.Unix(),
			Input:     entry.Input,
			Output:    entry.Output,
			Error:     entry.Error,
		}
	}

	respondJSON(w, http.StatusOK, auditTrailResponse{
		OrderID:    orderID,
		AuditTrail: auditItems,
	})
}
