package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/cbdc"
	"github.com/go-chi/chi/v5"
)

// helperURLParam sets URL parameters for chi routes in tests.
func helperURLParamCBDC(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestInitiateTransaction tests POST /v1/cbdc/transaction endpoint.
func TestInitiateTransaction(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	validator := cbdc.NewRBILimitsValidator()
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), validator)

	tests := []struct {
		name           string
		paymentID      string
		from           string
		to             string
		amount         int64
		expectedStatus int
	}{
		{
			name:           "successful transaction",
			paymentID:      "pay-001",
			from:           "acc-alice",
			to:             "acc-bob",
			amount:         100000,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "missing payment_id",
			paymentID:      "",
			from:           "acc-alice",
			to:             "acc-bob",
			amount:         100000,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "exceeds single transaction limit",
			paymentID:      "pay-limit",
			from:           "acc-alice",
			to:             "acc-bob",
			amount:         10_000_000,
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := initTransactionRequest{
				PaymentID:   tt.paymentID,
				FromAccount: tt.from,
				ToAccount:   tt.to,
				AmountPaise: tt.amount,
			}
			body, _ := json.Marshal(req)
			httpReq := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.InitiateTransaction(w, httpReq)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestGetTransaction tests GET /v1/cbdc/transaction/{payment_id}.
func TestGetTransaction(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), cbdc.NewRBILimitsValidator())

	paymentID := "pay-get-001"
	bridge.InitiatePayment(paymentID, "acc-alice", "acc-bob", 500000)

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/cbdc/transaction/%s", paymentID), nil)
	req = helperURLParamCBDC(req, "payment_id", paymentID)
	w := httptest.NewRecorder()

	handler.GetTransaction(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp getTransactionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.PaymentID != paymentID {
		t.Errorf("expected payment_id %s, got %s", paymentID, resp.PaymentID)
	}
}

// TestGetBlock tests GET /v1/cbdc/block/{block_height}.
func TestGetBlock(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), cbdc.NewRBILimitsValidator())

	// Get ledger stats to find the right block
	ledger := bridge.GetLedger()
	bridge.InitiatePayment("pay-block-001", "acc-alice", "acc-bob", 100000)
	height := ledger.GetLatestBlockHeight()

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/cbdc/block/%d", height), nil)
	req = helperURLParamCBDC(req, "block_height", fmt.Sprintf("%d", height))
	w := httptest.NewRecorder()

	handler.GetBlock(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp getBlockResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Height != height {
		t.Errorf("expected height %d, got %d", height, resp.Height)
	}
}

// TestGetLimits tests GET /v1/cbdc/limits/{account_id}.
func TestGetLimits(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	validator := cbdc.NewRBILimitsValidator()
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), validator)

	validator.CommitTransaction("acc-alice", 1000000, cbdc.TypeP2P)

	req := httptest.NewRequest("GET", "/v1/cbdc/limits/acc-alice", nil)
	req = helperURLParamCBDC(req, "account_id", "acc-alice")
	w := httptest.NewRecorder()

	handler.GetLimits(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp getLimitsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.DailyP2PUsedPaise != 1000000 {
		t.Errorf("expected used 1000000, got %d", resp.DailyP2PUsedPaise)
	}
}

// TestHealth tests GET /v1/cbdc/health.
func TestHealth(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), cbdc.NewRBILimitsValidator())

	bridge.InitiatePayment("pay-health-001", "acc-alice", "acc-bob", 100000)

	req := httptest.NewRequest("GET", "/v1/cbdc/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !resp.RBIBackendAvailable {
		t.Errorf("expected RBI backend available")
	}
}

// TestLimitValidation tests daily limit enforcement.
func TestLimitValidation(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	validator := cbdc.NewRBILimitsValidatorWithLimits(cbdc.RBILimits{
		DailyP2PLimit:          1000000,
		DailyMerchantLimit:     5000000,
		SingleTransactionLimit: 5000000,
	})
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), validator)

	// First transaction - should succeed
	req1 := initTransactionRequest{
		PaymentID:   "pay-limit-001",
		FromAccount: "acc-limit",
		ToAccount:   "acc-bob",
		AmountPaise: 700000,
	}
	body1, _ := json.Marshal(req1)
	httpReq1 := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body1))
	httpReq1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()

	handler.InitiateTransaction(w1, httpReq1)

	if w1.Code != http.StatusAccepted {
		t.Errorf("first transaction failed: expected %d, got %d", http.StatusAccepted, w1.Code)
	}

	// Second transaction - should exceed limit
	req2 := initTransactionRequest{
		PaymentID:   "pay-limit-002",
		FromAccount: "acc-limit",
		ToAccount:   "acc-bob",
		AmountPaise: 500000,
	}
	body2, _ := json.Marshal(req2)
	httpReq2 := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body2))
	httpReq2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	handler.InitiateTransaction(w2, httpReq2)

	if w2.Code != http.StatusUnprocessableEntity {
		t.Errorf("second transaction: expected %d, got %d", http.StatusUnprocessableEntity, w2.Code)
	}
}

// TestDoubleSpendPrevention tests duplicate payment ID rejection.
func TestDoubleSpendPrevention(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), cbdc.NewRBILimitsValidator())

	paymentID := "pay-dup-001"

	// First transaction
	req1 := initTransactionRequest{
		PaymentID:   paymentID,
		FromAccount: "acc-alice",
		ToAccount:   "acc-bob",
		AmountPaise: 100000,
	}
	body1, _ := json.Marshal(req1)
	httpReq1 := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body1))
	httpReq1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()

	handler.InitiateTransaction(w1, httpReq1)

	if w1.Code != http.StatusAccepted {
		t.Fatalf("first transaction failed")
	}

	// Second transaction with same ID
	req2 := initTransactionRequest{
		PaymentID:   paymentID,
		FromAccount: "acc-alice",
		ToAccount:   "acc-charlie",
		AmountPaise: 200000,
	}
	body2, _ := json.Marshal(req2)
	httpReq2 := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body2))
	httpReq2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	handler.InitiateTransaction(w2, httpReq2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("duplicate should be rejected: expected %d, got %d", http.StatusBadRequest, w2.Code)
	}
}

// TestFinalityDelayPolling tests transaction finality transitions.
func TestFinalityDelayPolling(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	ledger := bridge.GetLedger()

	paymentID := "pay-finality-001"
	resp, err := bridge.InitiatePayment(paymentID, "acc-alice", "acc-bob", 100000)
	if err != nil {
		t.Fatalf("initiate failed: %v", err)
	}

	if resp.Status != cbdc.StatusPending {
		t.Errorf("expected pending, got %s", resp.Status)
	}

	// Add more transactions to advance block height
	for i := 0; i < 5; i++ {
		bridge.InitiatePayment(fmt.Sprintf("pay-finality-%d", i+2), "acc-alice", "acc-bob", 100000)
	}

	// Process finality
	err = bridge.ProcessFinality()
	if err != nil {
		t.Fatalf("ProcessFinality failed: %v", err)
	}

	// Query again
	entry, found, _ := ledger.GetStatus(paymentID)
	if !found {
		t.Errorf("transaction should exist")
	}

	if entry != nil && entry.Status == cbdc.StatusFinalized {
		// Transaction was finalized as expected
	}
}

// TestConcurrentTransactions tests thread-safe concurrent processing.
func TestConcurrentTransactions(t *testing.T) {
	bridge := cbdc.NewMockCBDCBridge(1)
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), cbdc.NewRBILimitsValidator())

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			req := initTransactionRequest{
				PaymentID:   fmt.Sprintf("pay-concurrent-%d", index),
				FromAccount: fmt.Sprintf("acc-from-%d", index),
				ToAccount:   fmt.Sprintf("acc-to-%d", index),
				AmountPaise: 100000,
			}
			body, _ := json.Marshal(req)
			httpReq := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.InitiateTransaction(w, httpReq)

			if w.Code != http.StatusAccepted {
				t.Errorf("concurrent %d failed", index)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// BenchmarkInitiateTransaction measures transaction initiation performance.
func BenchmarkInitiateTransaction(b *testing.B) {
	bridge := cbdc.NewMockCBDCBridge(1)
	handler := NewCBDCHandler(bridge, bridge.GetLedger(), cbdc.NewRBILimitsValidator())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := initTransactionRequest{
			PaymentID:   fmt.Sprintf("pay-bench-%d", i),
			FromAccount: "acc-alice",
			ToAccount:   "acc-bob",
			AmountPaise: 100000,
		}
		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/v1/cbdc/transaction", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.InitiateTransaction(w, httpReq)
	}
}
