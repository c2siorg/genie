package commercesettlement

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	cs "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commercesettlement"
)

// MockEnvironment implements agent.Environment for testing.
type MockEnvironment struct {
	logs []string
}

func (m *MockEnvironment) Now() time.Time {
	return time.Now()
}

func (m *MockEnvironment) Logf(format string, args ...any) {
	m.logs = append(m.logs, format)
}

func TestAgentID(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	if ag.ID() != "commerce_settlement" {
		t.Errorf("Expected ID 'commerce_settlement', got '%s'", ag.ID())
	}
}

func TestAgentCapabilities(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	caps := ag.Capabilities()
	if len(caps) != 1 || caps[0] != "settle_commerce" {
		t.Errorf("Expected capability 'settle_commerce', got %v", caps)
	}
}

func TestCreateBatchMessage(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	req := CreateBatchRequest{
		SettlementDate: "2024-06-01",
		MerchantIDs:    []string{"m1", "m2"},
		AmountsPaise: map[string]int64{
			"m1": 10000,
			"m2": 20000,
		},
	}
	reqBody, _ := json.Marshal(req)

	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "create_settlement_batch", string(reqBody), nil)
	responses, err := ag.HandleMessage(context.Background(), msg, env)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
	}

	var resp CreateBatchResponse
	if err := json.Unmarshal([]byte(responses[0].Content), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Status != string(cs.StatusPending) {
		t.Errorf("Expected status '%s', got '%s'", cs.StatusPending, resp.Status)
	}
	if resp.MerchantCount != 2 {
		t.Errorf("Expected 2 merchants, got %d", resp.MerchantCount)
	}
	if resp.TotalAmountPaise != 30000 {
		t.Errorf("Expected total 30000, got %d", resp.TotalAmountPaise)
	}
}

func TestCreateBatchInvalidDate(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	req := CreateBatchRequest{
		SettlementDate: "invalid-date",
		MerchantIDs:    []string{"m1"},
		AmountsPaise:   map[string]int64{"m1": 10000},
	}
	reqBody, _ := json.Marshal(req)

	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "create_settlement_batch", string(reqBody), nil)
	responses, _ := ag.HandleMessage(context.Background(), msg, env)

	if len(responses) != 1 {
		t.Errorf("Expected 1 response (error), got %d", len(responses))
	}

	if responses[0].Type != "error" {
		t.Errorf("Expected error message, got %s", responses[0].Type)
	}
}

func TestSettleBatchMessage(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	// Create batch first.
	batch, _ := bm.CreateBatch(time.Now(), []string{"m1", "m2"})
	bm.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
	})

	// Now settle it.
	settleReq := SettleBatchRequest{BatchID: batch.ID}
	settleBody, _ := json.Marshal(settleReq)

	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "settle_batch", string(settleBody), nil)
	responses, err := ag.HandleMessage(context.Background(), msg, env)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
	}

	var resp SettleBatchResponse
	if err := json.Unmarshal([]byte(responses[0].Content), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Status != string(cs.StatusSettled) {
		t.Errorf("Expected status '%s', got '%s'", cs.StatusSettled, resp.Status)
	}
	if resp.GrossAmountPaise != 180 {
		t.Errorf("Expected gross 180, got %d", resp.GrossAmountPaise)
	}
	// In pool model with all merchants owing to pool, savings = 0
	if resp.NettingSavingsPaise != 0 {
		t.Errorf("Expected netting savings 0 (pool model), got %d", resp.NettingSavingsPaise)
	}
}

func TestGetStatusMessage(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	// Create and settle batch.
	batch, _ := bm.CreateBatch(time.Now(), []string{"m1", "m2"})
	bm.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
	})

	settleReq := SettleBatchRequest{BatchID: batch.ID}
	settleBody, _ := json.Marshal(settleReq)
	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "settle_batch", string(settleBody), nil)
	ag.HandleMessage(context.Background(), msg, env)

	// Now get status.
	statusReq := GetStatusRequest{BatchID: batch.ID}
	statusBody, _ := json.Marshal(statusReq)

	msg = agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "get_settlement_status", string(statusBody), nil)
	responses, err := ag.HandleMessage(context.Background(), msg, env)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
	}

	var resp GetStatusResponse
	if err := json.Unmarshal([]byte(responses[0].Content), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Status != string(cs.StatusSettled) {
		t.Errorf("Expected status '%s', got '%s'", cs.StatusSettled, resp.Status)
	}
	if resp.MerchantCount != 2 {
		t.Errorf("Expected 2 merchants, got %d", resp.MerchantCount)
	}
	if !resp.IsIdempotent {
		t.Error("Expected IsIdempotent to be true after settlement")
	}
}

func TestUnknownMessageType(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "unknown_type", "{}", nil)
	responses, err := ag.HandleMessage(context.Background(), msg, env)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	// Unknown types should be ignored (no response).
	if len(responses) != 0 {
		t.Errorf("Expected no response for unknown type, got %d", len(responses))
	}
}

func TestInvalidJSONRequest(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "create_settlement_batch", "invalid json", nil)
	responses, _ := ag.HandleMessage(context.Background(), msg, env)

	if len(responses) != 1 {
		t.Errorf("Expected 1 error response, got %d", len(responses))
	}
	if responses[0].Type != "error" {
		t.Errorf("Expected error type, got %s", responses[0].Type)
	}
}

func TestAuditTrail(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	// Create batch.
	req := CreateBatchRequest{
		SettlementDate: "2024-06-01",
		MerchantIDs:    []string{"m1"},
		AmountsPaise:   map[string]int64{"m1": 10000},
	}
	reqBody, _ := json.Marshal(req)
	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "create_settlement_batch", string(reqBody), nil)
	responses, _ := ag.HandleMessage(context.Background(), msg, env)

	var resp CreateBatchResponse
	json.Unmarshal([]byte(responses[0].Content), &resp)

	// Check audit trail.
	entries := ag.auditLog.GetEntries(resp.BatchID)
	if len(entries) == 0 {
		t.Error("Expected audit entries for created batch")
	}

	found := false
	for _, e := range entries {
		if e.Action == "batch_created" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'batch_created' audit entry")
	}
}

func TestEndToEndSettlement(t *testing.T) {
	bm := cs.NewInMemoryBatchManager()
	nc := cs.NewSimpleNettingCalculator()
	pb := cs.NewCBDCPaymentAdapter()
	se := cs.NewDefaultSettlementExecutor(bm, pb)
	ag := NewAgent(bm, nc, se)

	env := &MockEnvironment{}

	// Step 1: Create batch.
	// Two merchants with clear bilateral netting:
	// m1 owes 100, m2 owes 80, so after netting m1 pays m2 the difference (20).
	createReq := CreateBatchRequest{
		SettlementDate: "2024-06-01",
		MerchantIDs:    []string{"m1", "m2"},
		AmountsPaise: map[string]int64{
			"m1": 100,
			"m2": 80,
		},
	}
	createBody, _ := json.Marshal(createReq)
	msg := agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "create_settlement_batch", string(createBody), nil)
	responses, _ := ag.HandleMessage(context.Background(), msg, env)

	var createResp CreateBatchResponse
	json.Unmarshal([]byte(responses[0].Content), &createResp)
	batchID := createResp.BatchID

	// Step 2: Settle batch.
	settleReq := SettleBatchRequest{BatchID: batchID}
	settleBody, _ := json.Marshal(settleReq)
	msg = agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "settle_batch", string(settleBody), nil)
	responses, _ = ag.HandleMessage(context.Background(), msg, env)

	var settleResp SettleBatchResponse
	json.Unmarshal([]byte(responses[0].Content), &settleResp)

	if settleResp.Status != string(cs.StatusSettled) {
		t.Errorf("Expected settled status, got %s", settleResp.Status)
	}
	t.Logf("Settlement txn count: %d, gross: %d, net: %d, savings: %d",
		settleResp.SettlementTxnCount, settleResp.GrossAmountPaise, settleResp.NetAmountPaise, settleResp.NettingSavingsPaise)
	if settleResp.SettlementTxnCount == 0 {
		t.Errorf("Expected settlement transactions, got %d (net positions: %v)", settleResp.SettlementTxnCount, settleResp.NetPositions)
	}

	// Step 3: Get status.
	statusReq := GetStatusRequest{BatchID: batchID}
	statusBody, _ := json.Marshal(statusReq)
	msg = agent.NewMessage("test", "commerce_settlement", agent.RoleUser, "get_settlement_status", string(statusBody), nil)
	responses, _ = ag.HandleMessage(context.Background(), msg, env)

	var statusResp GetStatusResponse
	json.Unmarshal([]byte(responses[0].Content), &statusResp)

	if statusResp.Status != string(cs.StatusSettled) {
		t.Errorf("Expected settled status in final status, got %s", statusResp.Status)
	}
	if statusResp.IsIdempotent != true {
		t.Error("Expected idempotent flag to be true")
	}
}
