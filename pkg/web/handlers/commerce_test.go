package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commerce"
	"github.com/go-chi/chi/v5"
)

// ─── Mock Implementations ─────────────────────────────────────────────────

// MockOrderManager is a test double for commerce.OrderManager.
type MockOrderManager struct {
	orders map[string]*commerce.Order
	calls  []string // tracks method calls for verification
}

func NewMockOrderManager() *MockOrderManager {
	return &MockOrderManager{
		orders: make(map[string]*commerce.Order),
		calls:  []string{},
	}
}

func (m *MockOrderManager) CreateOrder(merchantID, customerID string, items []commerce.OrderItem) (*commerce.Order, error) {
	m.calls = append(m.calls, "CreateOrder")
	if merchantID == "" || customerID == "" || len(items) == 0 {
		return nil, fmt.Errorf("invalid input")
	}
	var totalPaise int64
	for _, item := range items {
		totalPaise += int64(item.Quantity) * item.UnitPricePaise
	}
	order := &commerce.Order{
		ID:         "test-order-123",
		MerchantID: merchantID,
		CustomerID: customerID,
		Items:      items,
		TotalPaise: totalPaise,
		Status:     commerce.StatusPending,
		CreatedAt:  time.Now().UTC(),
	}
	m.orders[order.ID] = order
	return order, nil
}

func (m *MockOrderManager) GetOrder(orderID string) (*commerce.Order, error) {
	m.calls = append(m.calls, "GetOrder")
	if orderID == "not-found" {
		return nil, fmt.Errorf("order not found")
	}
	order, ok := m.orders[orderID]
	if !ok {
		// Return a default test order if not found in store
		if orderID == "test-order-123" {
			return &commerce.Order{
				ID:         orderID,
				MerchantID: "merchant-1",
				CustomerID: "customer-1",
				Items: []commerce.OrderItem{
					{SKU: "SKU-1", Description: "Item 1", Quantity: 1, UnitPricePaise: 10000},
				},
				TotalPaise: 10000,
				Status:     commerce.StatusPending,
				CreatedAt:  time.Now().UTC(),
			}, nil
		}
		return nil, fmt.Errorf("order not found: %s", orderID)
	}
	return order, nil
}

func (m *MockOrderManager) UpdateStatus(orderID string, status commerce.OrderStatus) (*commerce.Order, error) {
	m.calls = append(m.calls, "UpdateStatus")
	order, ok := m.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found")
	}
	order.Status = status
	if status == commerce.StatusPaid {
		order.PaidAt = time.Now().UTC()
	}
	return order, nil
}

func (m *MockOrderManager) ListOrders(merchantID string) ([]*commerce.Order, error) {
	m.calls = append(m.calls, "ListOrders")
	var result []*commerce.Order
	for _, order := range m.orders {
		if order.MerchantID == merchantID {
			result = append(result, order)
		}
	}
	return result, nil
}

// MockWorkflowOrchestrator is a test double for commerce.WorkflowOrchestrator.
type MockWorkflowOrchestrator struct {
	workflows       map[string]*commerce.CommerceWorkflow
	auditLogs       map[string][]*commerce.AuditEntry
	calls           []string
	executeFailure  error                // if set, ExecuteWorkflow returns this error
	executeSuccess  bool                 // if set, ExecuteWorkflow returns this success status
	executeFinalSts commerce.OrderStatus // if set, ExecuteWorkflow returns this final status
}

func NewMockWorkflowOrchestrator() *MockWorkflowOrchestrator {
	return &MockWorkflowOrchestrator{
		workflows:       make(map[string]*commerce.CommerceWorkflow),
		auditLogs:       make(map[string][]*commerce.AuditEntry),
		calls:           []string{},
		executeSuccess:  true,
		executeFinalSts: commerce.StatusFulfilled,
	}
}

func (m *MockWorkflowOrchestrator) ExecuteWorkflow(ctx context.Context, orderID string) (bool, commerce.OrderStatus, error) {
	m.calls = append(m.calls, "ExecuteWorkflow")
	if m.executeFailure != nil {
		return false, commerce.StatusPaymentFailed, m.executeFailure
	}

	workflow := &commerce.CommerceWorkflow{
		OrderID:     orderID,
		Steps:       []commerce.WorkflowStep{commerce.StepOrderCreated, commerce.StepPaymentConfirmed},
		CurrentStep: commerce.StepPaymentConfirmed,
		Status:      commerce.StatusPaid,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	m.workflows[orderID] = workflow

	m.auditLogs[orderID] = []*commerce.AuditEntry{
		{
			ID:        "audit-1",
			OrderID:   orderID,
			Step:      commerce.StepOrderCreated,
			Action:    "order_created",
			Timestamp: time.Now().UTC(),
		},
	}

	return m.executeSuccess, m.executeFinalSts, nil
}

func (m *MockWorkflowOrchestrator) GetWorkflow(orderID string) (*commerce.CommerceWorkflow, error) {
	m.calls = append(m.calls, "GetWorkflow")
	workflow, ok := m.workflows[orderID]
	if !ok {
		return nil, fmt.Errorf("workflow not found")
	}
	copy := *workflow
	return &copy, nil
}

func (m *MockWorkflowOrchestrator) GetAuditLog(orderID string) ([]*commerce.AuditEntry, error) {
	m.calls = append(m.calls, "GetAuditLog")
	entries, ok := m.auditLogs[orderID]
	if !ok {
		return []*commerce.AuditEntry{}, nil
	}
	return entries, nil
}

// ─── Test Helpers ─────────────────────────────────────────────────────────

// makeRequest builds an HTTP request and sends it to the handler.
func makeRequest(t *testing.T, method, path string, body interface{}, handler http.Handler) (*http.Response, string) {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	return resp, string(respBody)
}

// ─── Tests ────────────────────────────────────────────────────────────────

// TestCreateOrder_Success tests successful order creation.
func TestCreateOrder_Success(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	req := createOrderRequest{
		MerchantID: "merchant-1",
		CustomerID: "customer-1",
		Items: []commerce.OrderItem{
			{SKU: "SKU-001", Description: "Laptop", Quantity: 1, UnitPricePaise: 100000},
		},
	}

	// Set up router to handle URL params
	router := chi.NewRouter()
	router.Post("/v1/commerce/order", handler.CreateOrder)

	resp, respBody := makeRequest(t, http.MethodPost, "/v1/commerce/order", req, router)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var respData createOrderResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.OrderID != "test-order-123" {
		t.Errorf("expected order_id test-order-123, got %s", respData.OrderID)
	}
	if respData.TotalPaise != 100000 {
		t.Errorf("expected total_paise 100000, got %d", respData.TotalPaise)
	}
	if respData.Status != "pending" {
		t.Errorf("expected status pending, got %s", respData.Status)
	}
}

// TestCreateOrder_MissingMerchantID tests validation of merchant_id.
func TestCreateOrder_MissingMerchantID(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	req := createOrderRequest{
		CustomerID: "customer-1",
		Items: []commerce.OrderItem{
			{SKU: "SKU-001", Description: "Item", Quantity: 1, UnitPricePaise: 10000},
		},
	}

	router := chi.NewRouter()
	router.Post("/v1/commerce/order", handler.CreateOrder)

	resp, _ := makeRequest(t, http.MethodPost, "/v1/commerce/order", req, router)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestCreateOrder_MissingItems tests validation of items.
func TestCreateOrder_MissingItems(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	req := createOrderRequest{
		MerchantID: "merchant-1",
		CustomerID: "customer-1",
		Items:      []commerce.OrderItem{},
	}

	router := chi.NewRouter()
	router.Post("/v1/commerce/order", handler.CreateOrder)

	resp, _ := makeRequest(t, http.MethodPost, "/v1/commerce/order", req, router)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestGetOrder_Success tests retrieving an existing order.
func TestGetOrder_Success(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Get("/v1/commerce/order/{order_id}", handler.GetOrder)

	resp, respBody := makeRequest(t, http.MethodGet, "/v1/commerce/order/test-order-123", nil, router)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var respData getOrderResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.OrderID != "test-order-123" {
		t.Errorf("expected order_id test-order-123, got %s", respData.OrderID)
	}
	if respData.MerchantID != "merchant-1" {
		t.Errorf("expected merchant_id merchant-1, got %s", respData.MerchantID)
	}
	if respData.CustomerID != "customer-1" {
		t.Errorf("expected customer_id customer-1, got %s", respData.CustomerID)
	}
}

// TestGetOrder_NotFound tests 404 for non-existent order.
func TestGetOrder_NotFound(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Get("/v1/commerce/order/{order_id}", handler.GetOrder)

	resp, _ := makeRequest(t, http.MethodGet, "/v1/commerce/order/not-found", nil, router)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

// TestExecuteWorkflow_Success tests successful workflow execution.
func TestExecuteWorkflow_Success(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Post("/v1/commerce/order/{order_id}/execute", handler.ExecuteWorkflow)

	resp, respBody := makeRequest(t, http.MethodPost, "/v1/commerce/order/test-order-123/execute", nil, router)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var respData executeWorkflowResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.OrderID != "test-order-123" {
		t.Errorf("expected order_id test-order-123, got %s", respData.OrderID)
	}
	if respData.FinalStatus != "fulfilled" {
		t.Errorf("expected final_status fulfilled, got %s", respData.FinalStatus)
	}
}

// TestExecuteWorkflow_OrderNotFound tests 404 for non-existent order.
func TestExecuteWorkflow_OrderNotFound(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Post("/v1/commerce/order/{order_id}/execute", handler.ExecuteWorkflow)

	resp, _ := makeRequest(t, http.MethodPost, "/v1/commerce/order/not-found/execute", nil, router)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

// TestExecuteWorkflow_PaymentFailure tests payment failure recovery flow.
func TestExecuteWorkflow_PaymentFailure(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	workflowOrch.executeSuccess = false
	workflowOrch.executeFinalSts = commerce.StatusPaymentFailed
	workflowOrch.executeFailure = fmt.Errorf("payment confirmation timeout")

	// Pre-populate the workflow with failure state
	workflow := &commerce.CommerceWorkflow{
		OrderID:      "test-order-123",
		Steps:        []commerce.WorkflowStep{commerce.StepOrderCreated, commerce.StepPaymentInitiated},
		CurrentStep:  commerce.StepPaymentInitiated,
		Status:       commerce.StatusPaymentFailed,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		ErrorMessage: "payment confirmation timeout",
	}
	workflowOrch.workflows["test-order-123"] = workflow

	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Post("/v1/commerce/order/{order_id}/execute", handler.ExecuteWorkflow)

	resp, respBody := makeRequest(t, http.MethodPost, "/v1/commerce/order/test-order-123/execute", nil, router)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	var respData executeWorkflowResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.FinalStatus != "payment_failed" {
		t.Errorf("expected final_status payment_failed, got %s", respData.FinalStatus)
	}
}

// TestExecuteWorkflow_SettlementFailure tests settlement failure escalation (HITL).
func TestExecuteWorkflow_SettlementFailure(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	workflowOrch.executeSuccess = false
	workflowOrch.executeFinalSts = commerce.StatusPaid
	// Don't set executeFailure so it uses the workflow state instead

	// Set up the workflow to indicate HITL requirement (settlement failure)
	workflow := &commerce.CommerceWorkflow{
		OrderID:      "test-order-123",
		Steps:        []commerce.WorkflowStep{commerce.StepOrderCreated, commerce.StepPaymentConfirmed, commerce.StepSettlementInitiated},
		CurrentStep:  commerce.StepSettlementInitiated,
		Status:       commerce.StatusPaid, // Order is paid but settlement failed
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		RequiresHITL: true,
		ErrorMessage: "settlement initiation failed",
	}
	workflowOrch.workflows["test-order-123"] = workflow

	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Post("/v1/commerce/order/{order_id}/execute", handler.ExecuteWorkflow)

	resp, respBody := makeRequest(t, http.MethodPost, "/v1/commerce/order/test-order-123/execute", nil, router)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	var respData executeWorkflowResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.FinalStatus != "paid" {
		t.Errorf("expected final_status paid (settlement failure escalated to HITL), got %s", respData.FinalStatus)
	}
}

// TestGetAuditLog_Success tests retrieving audit log for an order.
func TestGetAuditLog_Success(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()

	// Pre-populate audit log
	auditEntries := []*commerce.AuditEntry{
		{
			ID:        "audit-1",
			OrderID:   "test-order-123",
			Step:      commerce.StepOrderCreated,
			Action:    "order_created",
			Timestamp: time.Now().UTC(),
		},
		{
			ID:        "audit-2",
			OrderID:   "test-order-123",
			Step:      commerce.StepPaymentInitiated,
			Action:    "payment_initiated",
			Timestamp: time.Now().UTC(),
		},
	}
	workflowOrch.auditLogs["test-order-123"] = auditEntries

	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Get("/v1/commerce/order/{order_id}/audit", handler.GetAuditLog)

	resp, respBody := makeRequest(t, http.MethodGet, "/v1/commerce/order/test-order-123/audit", nil, router)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var respData auditTrailResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.OrderID != "test-order-123" {
		t.Errorf("expected order_id test-order-123, got %s", respData.OrderID)
	}
	if len(respData.AuditTrail) != 2 {
		t.Errorf("expected 2 audit entries, got %d", len(respData.AuditTrail))
	}
}

// TestGetAuditLog_NotFound tests 404 for non-existent order.
func TestGetAuditLog_NotFound(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Get("/v1/commerce/order/{order_id}/audit", handler.GetAuditLog)

	resp, _ := makeRequest(t, http.MethodGet, "/v1/commerce/order/not-found/audit", nil, router)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

// TestGetAuditLog_EmptyLog tests empty audit log response.
func TestGetAuditLog_EmptyLog(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Get("/v1/commerce/order/{order_id}/audit", handler.GetAuditLog)

	resp, respBody := makeRequest(t, http.MethodGet, "/v1/commerce/order/test-order-123/audit", nil, router)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var respData auditTrailResponse
	if err := json.Unmarshal([]byte(respBody), &respData); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if respData.OrderID != "test-order-123" {
		t.Errorf("expected order_id test-order-123, got %s", respData.OrderID)
	}
	if len(respData.AuditTrail) != 0 {
		t.Errorf("expected 0 audit entries, got %d", len(respData.AuditTrail))
	}
}

// TestIntegration_CompleteWorkflow tests a complete happy path workflow.
func TestIntegration_CompleteWorkflow(t *testing.T) {
	orderMgr := NewMockOrderManager()
	workflowOrch := NewMockWorkflowOrchestrator()
	handler := NewCommerceHandler(orderMgr, workflowOrch)

	router := chi.NewRouter()
	router.Post("/v1/commerce/order", handler.CreateOrder)
	router.Get("/v1/commerce/order/{order_id}", handler.GetOrder)
	router.Post("/v1/commerce/order/{order_id}/execute", handler.ExecuteWorkflow)
	router.Get("/v1/commerce/order/{order_id}/audit", handler.GetAuditLog)

	// Step 1: Create order
	createReq := createOrderRequest{
		MerchantID: "merchant-1",
		CustomerID: "customer-1",
		Items: []commerce.OrderItem{
			{SKU: "SKU-001", Description: "Test Item", Quantity: 2, UnitPricePaise: 50000},
		},
	}

	resp, respBody := makeRequest(t, http.MethodPost, "/v1/commerce/order", createReq, router)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create order failed: got status %d", resp.StatusCode)
	}

	var createResp createOrderResponse
	if err := json.Unmarshal([]byte(respBody), &createResp); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}

	orderID := createResp.OrderID

	// Step 2: Get order
	resp, respBody = makeRequest(t, http.MethodGet, "/v1/commerce/order/"+orderID, nil, router)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get order failed: got status %d", resp.StatusCode)
	}

	var getResp getOrderResponse
	if err := json.Unmarshal([]byte(respBody), &getResp); err != nil {
		t.Fatalf("failed to unmarshal get response: %v", err)
	}

	if getResp.TotalPaise != 100000 {
		t.Fatalf("expected total_paise 100000, got %d", getResp.TotalPaise)
	}

	// Step 3: Execute workflow
	resp, _ = makeRequest(t, http.MethodPost, "/v1/commerce/order/"+orderID+"/execute", nil, router)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("execute workflow failed: got status %d", resp.StatusCode)
	}

	// Step 4: Get audit log
	resp, respBody = makeRequest(t, http.MethodGet, "/v1/commerce/order/"+orderID+"/audit", nil, router)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get audit log failed: got status %d", resp.StatusCode)
	}

	var auditResp auditTrailResponse
	if err := json.Unmarshal([]byte(respBody), &auditResp); err != nil {
		t.Fatalf("failed to unmarshal audit response: %v", err)
	}

	if len(auditResp.AuditTrail) == 0 {
		t.Errorf("expected at least 1 audit entry, got %d", len(auditResp.AuditTrail))
	}
}
