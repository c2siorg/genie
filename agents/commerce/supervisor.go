// Package commerce implements the Commerce Agent, which orchestrates
// order placement, payment, settlement, and fulfillment workflows.
package commerce

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commerce"
)

const (
	ID         = "commerce_supervisor"
	Capability = "manage_commerce"
	TypeIn     = "commerce_request"
	TypeOut    = "commerce_response"
)

// CommerceAgent handles place_order, get_order_status, fulfill_order messages.
type CommerceAgent struct {
	orderMgr  commerce.OrderManager
	workflow  commerce.WorkflowOrchestrator
}

// New creates a new CommerceAgent with dependencies.
func New(orderMgr commerce.OrderManager, workflow commerce.WorkflowOrchestrator) *CommerceAgent {
	return &CommerceAgent{
		orderMgr: orderMgr,
		workflow: workflow,
	}
}

func (a *CommerceAgent) ID() string {
	return ID
}

func (a *CommerceAgent) Name() string {
	return "Commerce Agent"
}

func (a *CommerceAgent) Capabilities() []string {
	return []string{Capability}
}

// HandleMessage processes incoming commerce requests.
// Supported message formats:
//   {"action": "place_order", "merchant_id": "...", "customer_id": "...", "items": [...]}
//   {"action": "get_order_status", "order_id": "..."}
//   {"action": "execute_workflow", "order_id": "..."}
func (a *CommerceAgent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}

	var req map[string]interface{}
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return a.errorResponse(msg, "invalid request format: "+err.Error()), nil
	}

	action, ok := req["action"].(string)
	if !ok {
		return a.errorResponse(msg, "action field required"), nil
	}

	var result interface{}
	var err error

	switch action {
	case "place_order":
		result, err = a.handlePlaceOrder(req)
	case "get_order_status":
		result, err = a.handleGetOrderStatus(req)
	case "execute_workflow":
		result, err = a.handleExecuteWorkflow(ctx, req)
	case "get_audit_log":
		result, err = a.handleGetAuditLog(req)
	default:
		return a.errorResponse(msg, "unknown action: "+action), nil
	}

	if err != nil {
		return a.errorResponse(msg, err.Error()), nil
	}

	body, _ := json.Marshal(result)
	env.Logf("[commerce_agent] %s completed", action)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// PlaceOrderRequest is the schema for place_order.
type PlaceOrderRequest struct {
	MerchantID string                `json:"merchant_id"`
	CustomerID string                `json:"customer_id"`
	Items      []commerce.OrderItem `json:"items"`
}

// PlaceOrderResponse is returned after creating an order.
type PlaceOrderResponse struct {
	Success bool          `json:"success"`
	OrderID string        `json:"order_id,omitempty"`
	Order   *commerce.Order `json:"order,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// handlePlaceOrder creates a new order.
func (a *CommerceAgent) handlePlaceOrder(req map[string]interface{}) (*PlaceOrderResponse, error) {
	placeReq := PlaceOrderRequest{}
	body, _ := json.Marshal(req)
	if err := json.Unmarshal(body, &placeReq); err != nil {
		return &PlaceOrderResponse{Success: false, Error: err.Error()}, nil
	}

	order, err := a.orderMgr.CreateOrder(placeReq.MerchantID, placeReq.CustomerID, placeReq.Items)
	if err != nil {
		return &PlaceOrderResponse{Success: false, Error: err.Error()}, nil
	}

	return &PlaceOrderResponse{
		Success: true,
		OrderID: order.ID,
		Order:   order,
	}, nil
}

// GetOrderStatusRequest is the schema for get_order_status.
type GetOrderStatusRequest struct {
	OrderID string `json:"order_id"`
}

// GetOrderStatusResponse returns order details.
type GetOrderStatusResponse struct {
	Success   bool             `json:"success"`
	Order     *commerce.Order    `json:"order,omitempty"`
	Workflow  *commerce.CommerceWorkflow `json:"workflow,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// handleGetOrderStatus retrieves order and workflow status.
func (a *CommerceAgent) handleGetOrderStatus(req map[string]interface{}) (*GetOrderStatusResponse, error) {
	statusReq := GetOrderStatusRequest{}
	body, _ := json.Marshal(req)
	if err := json.Unmarshal(body, &statusReq); err != nil {
		return &GetOrderStatusResponse{Success: false, Error: err.Error()}, nil
	}

	order, err := a.orderMgr.GetOrder(statusReq.OrderID)
	if err != nil {
		return &GetOrderStatusResponse{Success: false, Error: err.Error()}, nil
	}

	workflow, _ := a.workflow.GetWorkflow(statusReq.OrderID)

	return &GetOrderStatusResponse{
		Success:  true,
		Order:    order,
		Workflow: workflow,
	}, nil
}

// ExecuteWorkflowRequest is the schema for execute_workflow.
type ExecuteWorkflowRequest struct {
	OrderID string `json:"order_id"`
}

// ExecuteWorkflowResponse returns the result of workflow execution.
type ExecuteWorkflowResponse struct {
	Success     bool               `json:"success"`
	OrderID     string             `json:"order_id,omitempty"`
	FinalStatus commerce.OrderStatus `json:"final_status,omitempty"`
	Workflow    *commerce.CommerceWorkflow `json:"workflow,omitempty"`
	Error       string             `json:"error,omitempty"`
}

// handleExecuteWorkflow executes the complete order workflow.
func (a *CommerceAgent) handleExecuteWorkflow(ctx context.Context, req map[string]interface{}) (*ExecuteWorkflowResponse, error) {
	execReq := ExecuteWorkflowRequest{}
	body, _ := json.Marshal(req)
	if err := json.Unmarshal(body, &execReq); err != nil {
		return &ExecuteWorkflowResponse{Success: false, Error: err.Error()}, nil
	}

	success, finalStatus, err := a.workflow.ExecuteWorkflow(ctx, execReq.OrderID)
	workflow, _ := a.workflow.GetWorkflow(execReq.OrderID)

	if err != nil {
		return &ExecuteWorkflowResponse{
			Success:     false,
			OrderID:     execReq.OrderID,
			FinalStatus: finalStatus,
			Workflow:    workflow,
			Error:       err.Error(),
		}, nil
	}

	return &ExecuteWorkflowResponse{
		Success:     success,
		OrderID:     execReq.OrderID,
		FinalStatus: finalStatus,
		Workflow:    workflow,
	}, nil
}

// GetAuditLogRequest is the schema for get_audit_log.
type GetAuditLogRequest struct {
	OrderID string `json:"order_id"`
}

// GetAuditLogResponse returns audit entries for an order.
type GetAuditLogResponse struct {
	Success bool                 `json:"success"`
	OrderID string               `json:"order_id,omitempty"`
	Entries []*commerce.AuditEntry `json:"entries,omitempty"`
	Error   string               `json:"error,omitempty"`
}

// handleGetAuditLog retrieves the audit log for an order.
func (a *CommerceAgent) handleGetAuditLog(req map[string]interface{}) (*GetAuditLogResponse, error) {
	auditReq := GetAuditLogRequest{}
	body, _ := json.Marshal(req)
	if err := json.Unmarshal(body, &auditReq); err != nil {
		return &GetAuditLogResponse{Success: false, Error: err.Error()}, nil
	}

	entries, err := a.workflow.GetAuditLog(auditReq.OrderID)
	if err != nil {
		return &GetAuditLogResponse{Success: false, Error: err.Error()}, nil
	}

	return &GetAuditLogResponse{
		Success: true,
		OrderID: auditReq.OrderID,
		Entries: entries,
	}, nil
}

// errorResponse builds an error response.
func (a *CommerceAgent) errorResponse(msg agent.Message, errMsg string) []agent.Message {
	resp := map[string]interface{}{
		"success": false,
		"error":   errMsg,
	}
	body, _ := json.Marshal(resp)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}
}
