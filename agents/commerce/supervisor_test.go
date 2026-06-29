package commerce

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commerce"
)

// MockEnvironment implements agent.Environment for testing.
type MockEnvironment struct {
	logged []string
}

func (e *MockEnvironment) Now() time.Time { return time.Now().UTC() }
func (e *MockEnvironment) Logf(fmt string, args ...interface{}) {
	e.logged = append(e.logged, "log")
}

// testPaymentAgent is a test double for PaymentAgentStub.
type testPaymentAgent struct {
	shouldFail     bool
	confirmOnRetry int
	confirmAttempt int
}

func (m *testPaymentAgent) InitiatePayment(ctx context.Context, req commerce.PaymentInitiationRequest) (string, error) {
	if m.shouldFail {
		return "", context.Canceled
	}
	return "txn-" + req.OrderID, nil
}

func (m *testPaymentAgent) PollPaymentStatus(ctx context.Context, transactionID string) (*commerce.PaymentConfirmation, error) {
	m.confirmAttempt++
	if m.confirmAttempt < m.confirmOnRetry {
		return &commerce.PaymentConfirmation{
			Success:     false,
			ErrorReason: "payment pending",
		}, nil
	}
	return &commerce.PaymentConfirmation{
		Success:       true,
		TransactionID: transactionID,
		ConfirmedAt:   time.Now().UTC(),
	}, nil
}

// testSettlementAgent is a test double for SettlementAgentStub.
type testSettlementAgent struct {
	shouldFail bool
}

func (m *testSettlementAgent) InitiateSettlement(ctx context.Context, req commerce.SettlementInitiationRequest) (string, error) {
	if m.shouldFail {
		return "", context.Canceled
	}
	return "settlement-" + req.OrderID, nil
}

func (m *testSettlementAgent) WaitSettlementCompletion(ctx context.Context, settlementID string) (*commerce.SettlementResult, error) {
	return &commerce.SettlementResult{
		Success:      !m.shouldFail,
		SettlementID: settlementID,
		CompletedAt:  time.Now().UTC(),
		ErrorReason: func() string {
			if m.shouldFail {
				return "settlement failed"
			}
			return ""
		}(),
		RequiresReview: m.shouldFail,
	}, nil
}

func TestCommerceAgent_PlaceOrder(t *testing.T) {
	orderMgr := commerce.NewInMemoryOrderManager()
	paymentAgent := &testPaymentAgent{shouldFail: false, confirmOnRetry: 1}
	settlementAgent := &testSettlementAgent{shouldFail: false}
	workflow := commerce.NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	commerceAgent := New(orderMgr, workflow)
	env := &MockEnvironment{}

	// Create place_order message
	req := PlaceOrderRequest{
		MerchantID: "merchant-1",
		CustomerID: "customer-1",
		Items: []commerce.OrderItem{
			{SKU: "ITEM001", Description: "Widget", Quantity: 2, UnitPricePaise: 50_000},
			{SKU: "ITEM002", Description: "Gadget", Quantity: 1, UnitPricePaise: 100_000},
		},
	}
	reqBody, _ := json.Marshal(map[string]interface{}{
		"action":      "place_order",
		"merchant_id": req.MerchantID,
		"customer_id": req.CustomerID,
		"items":       req.Items,
	})

	msg := agent.NewMessage("test-user", commerceAgent.ID(), agent.RoleUser, TypeIn, string(reqBody), nil)

	// Handle message
	responses, err := commerceAgent.HandleMessage(context.Background(), msg, env)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	// Parse response
	var resp PlaceOrderResponse
	json.Unmarshal([]byte(responses[0].Content), &resp)

	if !resp.Success {
		t.Errorf("expected success=true, got %v (error: %s)", resp.Success, resp.Error)
	}
	if resp.OrderID == "" {
		t.Error("OrderID should not be empty")
	}
	if resp.Order == nil {
		t.Error("Order should not be nil")
	} else {
		if resp.Order.TotalPaise != 200_000 {
			t.Errorf("expected TotalPaise=200000, got %d", resp.Order.TotalPaise)
		}
	}
}

func TestCommerceAgent_PlaceOrder_InvalidInput(t *testing.T) {
	orderMgr := commerce.NewInMemoryOrderManager()
	paymentAgent := &testPaymentAgent{}
	settlementAgent := &testSettlementAgent{}
	workflow := commerce.NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	commerceAgent := New(orderMgr, workflow)
	env := &MockEnvironment{}

	// Create request with missing merchant_id
	reqBody, _ := json.Marshal(map[string]interface{}{
		"action":      "place_order",
		"customer_id": "customer-1",
		"items":       []commerce.OrderItem{},
	})

	msg := agent.NewMessage("test-user", commerceAgent.ID(), agent.RoleUser, TypeIn, string(reqBody), nil)
	responses, _ := commerceAgent.HandleMessage(context.Background(), msg, env)

	var resp PlaceOrderResponse
	json.Unmarshal([]byte(responses[0].Content), &resp)

	if resp.Success {
		t.Error("expected success=false for missing merchant_id")
	}
	if resp.Error == "" {
		t.Error("expected error message")
	}
}

func TestCommerceAgent_GetOrderStatus(t *testing.T) {
	orderMgr := commerce.NewInMemoryOrderManager()
	paymentAgent := &testPaymentAgent{shouldFail: false, confirmOnRetry: 1}
	settlementAgent := &testSettlementAgent{shouldFail: false}
	workflow := commerce.NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	commerceAgent := New(orderMgr, workflow)
	env := &MockEnvironment{}

	// Create an order first
	items := []commerce.OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Create get_order_status message
	reqBody, _ := json.Marshal(map[string]interface{}{
		"action":   "get_order_status",
		"order_id": order.ID,
	})

	msg := agent.NewMessage("test-user", commerceAgent.ID(), agent.RoleUser, TypeIn, string(reqBody), nil)
	responses, _ := commerceAgent.HandleMessage(context.Background(), msg, env)

	var resp GetOrderStatusResponse
	json.Unmarshal([]byte(responses[0].Content), &resp)

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
	if resp.Order == nil {
		t.Error("Order should not be nil")
	} else {
		if resp.Order.ID != order.ID {
			t.Errorf("expected OrderID=%s, got %s", order.ID, resp.Order.ID)
		}
	}
}

func TestCommerceAgent_ExecuteWorkflow(t *testing.T) {
	orderMgr := commerce.NewInMemoryOrderManager()
	paymentAgent := &testPaymentAgent{shouldFail: false, confirmOnRetry: 1}
	settlementAgent := &testSettlementAgent{shouldFail: false}
	workflow := commerce.NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	commerceAgent := New(orderMgr, workflow)
	env := &MockEnvironment{}

	// Create an order first
	items := []commerce.OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)

	// Create execute_workflow message
	reqBody, _ := json.Marshal(map[string]interface{}{
		"action":   "execute_workflow",
		"order_id": order.ID,
	})

	msg := agent.NewMessage("test-user", commerceAgent.ID(), agent.RoleUser, TypeIn, string(reqBody), nil)
	responses, _ := commerceAgent.HandleMessage(context.Background(), msg, env)

	var resp ExecuteWorkflowResponse
	json.Unmarshal([]byte(responses[0].Content), &resp)

	if !resp.Success {
		t.Errorf("expected success=true, got %v (error: %s)", resp.Success, resp.Error)
	}
	if resp.FinalStatus != commerce.StatusFulfilled {
		t.Errorf("expected FinalStatus=fulfilled, got %s", resp.FinalStatus)
	}
}

func TestCommerceAgent_GetAuditLog(t *testing.T) {
	orderMgr := commerce.NewInMemoryOrderManager()
	paymentAgent := &testPaymentAgent{shouldFail: false, confirmOnRetry: 1}
	settlementAgent := &testSettlementAgent{shouldFail: false}
	workflow := commerce.NewDefaultWorkflowOrchestrator(orderMgr, paymentAgent, settlementAgent)

	commerceAgent := New(orderMgr, workflow)
	env := &MockEnvironment{}

	// Create an order and execute workflow
	items := []commerce.OrderItem{{SKU: "A", Quantity: 1, UnitPricePaise: 100_000}}
	order, _ := orderMgr.CreateOrder("merchant-1", "customer-1", items)
	workflow.ExecuteWorkflow(context.Background(), order.ID)

	// Create get_audit_log message
	reqBody, _ := json.Marshal(map[string]interface{}{
		"action":   "get_audit_log",
		"order_id": order.ID,
	})

	msg := agent.NewMessage("test-user", commerceAgent.ID(), agent.RoleUser, TypeIn, string(reqBody), nil)
	responses, _ := commerceAgent.HandleMessage(context.Background(), msg, env)

	var resp GetAuditLogResponse
	json.Unmarshal([]byte(responses[0].Content), &resp)

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
	if len(resp.Entries) == 0 {
		t.Error("expected audit entries")
	}
}
