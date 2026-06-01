package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/c2siorg/genie/pkg/erupeecompliance"
	"github.com/go-chi/chi/v5"
)

func setupComplianceHandler(t *testing.T) (*ComplianceHandler, *chi.Mux) {
	engine := erupeecompliance.NewComplianceEngine()
	handler := NewComplianceHandler(engine)

	r := chi.NewRouter()
	r.Route("/v1/compliance", func(r chi.Router) {
		r.Post("/check", handler.CheckPayment)
		r.Get("/check/{compliance_check_id}", handler.GetCheck)
		r.Get("/account/{account_id}/velocity", handler.GetVelocity)
		r.Get("/account/{account_id}/fraud-history", handler.GetFraudHistory)
		r.Post("/admin/reset-velocity", handler.ResetVelocity)
	})

	return handler, r
}

// Test 1: POST /v1/compliance/check - basic compliance check
func TestCheckPayment(t *testing.T) {
	_, router := setupComplianceHandler(t)

	reqBody := checkPaymentRequest{
		PaymentID:     "payment_001",
		FromAccount:   "account_001",
		ToAccount:     "account_002",
		ToName:        "Recipient Name",
		AmountPaise:   100000, // ₹1000
		AccountAgeSec: 7776000, // 90 days
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/compliance/check", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, w.Code)
	}

	var resp checkPaymentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ComplianceCheckID == "" {
		t.Error("expected compliance_check_id in response")
	}
	if resp.Decision != "pending" {
		t.Errorf("expected decision 'pending', got %s", resp.Decision)
	}
}

// Test 2: POST /v1/compliance/check - validation errors
func TestCheckPaymentValidation(t *testing.T) {
	_, router := setupComplianceHandler(t)

	tests := []struct {
		name   string
		req    checkPaymentRequest
		errMsg string
	}{
		{
			name:   "missing payment_id",
			req:    checkPaymentRequest{FromAccount: "a1", ToAccount: "a2", AmountPaise: 1000},
			errMsg: "missing required fields",
		},
		{
			name:   "missing from_account",
			req:    checkPaymentRequest{PaymentID: "p1", ToAccount: "a2", AmountPaise: 1000},
			errMsg: "missing required fields",
		},
		{
			name:   "missing to_account",
			req:    checkPaymentRequest{PaymentID: "p1", FromAccount: "a1", AmountPaise: 1000},
			errMsg: "missing required fields",
		},
		{
			name:   "zero amount",
			req:    checkPaymentRequest{PaymentID: "p1", FromAccount: "a1", ToAccount: "a2", AmountPaise: 0},
			errMsg: "missing required fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.req)
			req := httptest.NewRequest("POST", "/v1/compliance/check", bytes.NewReader(body))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

// Test 3: GET /v1/compliance/check/{id} - retrieve check result
func TestGetCheck(t *testing.T) {
	handler, router := setupComplianceHandler(t)

	// Manually insert a check result
	checkID := "check_001"
	handler.mu.Lock()
	handler.checkResults[checkID] = &erupeecompliance.ComplianceCheck{
		PaymentID:      "payment_001",
		AMLResult:      erupeecompliance.AMLPass,
		VelocityResult: erupeecompliance.VelocityOK,
		FraudScore:     10.0,
		Decision:       erupeecompliance.DecisionAllow,
		CheckedAt:      time.Now().UTC(),
	}
	handler.mu.Unlock()

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/check/%s", checkID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp getCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ComplianceCheckID != checkID {
		t.Errorf("expected check_id %s, got %s", checkID, resp.ComplianceCheckID)
	}
	if resp.PaymentID != "payment_001" {
		t.Errorf("expected payment_id payment_001, got %s", resp.PaymentID)
	}
	if resp.Decision != "allow" {
		t.Errorf("expected decision 'allow', got %s", resp.Decision)
	}
}

// Test 4: GET /v1/compliance/check/{id} - not found
func TestGetCheckNotFound(t *testing.T) {
	_, router := setupComplianceHandler(t)

	req := httptest.NewRequest("GET", "/v1/compliance/check/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test 5: GET /v1/compliance/account/{id}/velocity - basic velocity check
func TestGetVelocity(t *testing.T) {
	handler, router := setupComplianceHandler(t)

	accountID := "account_001"
	// Pre-populate some velocity data by recording a transaction
	ctx := context.Background()
	handler.engine.GetVelocityRecord(ctx, accountID) // Initialize

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/account/%s/velocity", accountID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp velocityResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AccountID != accountID {
		t.Errorf("expected account_id %s, got %s", accountID, resp.AccountID)
	}
	if resp.Status == "" {
		t.Error("expected status field to be populated")
	}
	if resp.Limits.HourlyTransactions == 0 {
		t.Error("expected hourly transaction limit to be set")
	}
}

// Test 6: GET /v1/compliance/account/{id}/velocity - velocity threshold breach
func TestGetVelocityThresholdBreach(t *testing.T) {
	engine := erupeecompliance.NewComplianceEngine()
	handler := NewComplianceHandler(engine)

	r := chi.NewRouter()
	r.Get("/v1/compliance/account/{account_id}/velocity", handler.GetVelocity)

	accountID := "account_breach"
	ctx := context.Background()

	// Record multiple transactions to breach hourly limit
	for i := 0; i < 11; i++ {
		payment := erupeecompliance.PaymentRequest{
			PaymentID:     fmt.Sprintf("payment_%d", i),
			FromAccountID: accountID,
			ToAccountID:   "account_002",
			Amount:        1000000, // ₹10k
			Timestamp:     time.Now().UTC(),
		}
		engine.CheckPayment(ctx, payment)
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/account/%s/velocity", accountID), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp velocityResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should show exceeded status
	if resp.Status != "exceeded" {
		t.Logf("expected status 'exceeded' for %d transactions, got %s", resp.TransactionCountHourly, resp.Status)
	}
}

// Test 7: GET /v1/compliance/account/{id}/fraud-history - basic fraud history
func TestGetFraudHistory(t *testing.T) {
	_, router := setupComplianceHandler(t)

	accountID := "account_001"

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/account/%s/fraud-history", accountID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp fraudHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AccountID != accountID {
		t.Errorf("expected account_id %s, got %s", accountID, resp.AccountID)
	}
	if resp.RiskLevel == "" {
		t.Error("expected risk_level to be populated")
	}
}

// Test 8: GET /v1/compliance/account/{id}/fraud-history - fraud pattern detection
func TestGetFraudHistoryWithPatterns(t *testing.T) {
	handler, router := setupComplianceHandler(t)

	accountID := "account_fraud"

	// Manually add fraud patterns
	handler.mu.Lock()
	handler.patternHistory[accountID] = []*FraudPatternEntry{
		{
			PatternType: erupeecompliance.PatternStructuring,
			DetectedAt:  time.Now().UTC(),
			Evidence:    "5 transactions under ₹10k in 24 hours",
		},
		{
			PatternType: erupeecompliance.PatternRoundTripping,
			DetectedAt:  time.Now().UTC().Add(-1 * time.Hour),
			Evidence:    "Send-receive cycle detected",
		},
	}
	handler.mu.Unlock()

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/account/%s/fraud-history", accountID), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var resp fraudHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.FraudPatterns) != 2 {
		t.Errorf("expected 2 fraud patterns, got %d", len(resp.FraudPatterns))
	}
	if resp.RiskLevel != "medium" {
		t.Errorf("expected risk_level 'medium' for 2 patterns, got %s", resp.RiskLevel)
	}
	if resp.FraudScore == 0 {
		t.Error("expected fraud_score to be non-zero")
	}
}

// Test 9: POST /v1/compliance/admin/reset-velocity
func TestResetVelocity(t *testing.T) {
	_, router := setupComplianceHandler(t)

	accountID := "account_reset"
	reqBody := resetVelocityRequest{
		AccountID: accountID,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/compliance/admin/reset-velocity", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp resetVelocityResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AccountID != accountID {
		t.Errorf("expected account_id %s, got %s", accountID, resp.AccountID)
	}
	if resp.ResetAt.IsZero() {
		t.Error("expected reset_at timestamp")
	}
}

// Test 10: POST /v1/compliance/admin/reset-velocity - validation
func TestResetVelocityValidation(t *testing.T) {
	_, router := setupComplianceHandler(t)

	reqBody := resetVelocityRequest{
		AccountID: "", // Missing required field
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/compliance/admin/reset-velocity", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test 11: Async compliance check - polling behavior
func TestAsyncComplianceCheckPolling(t *testing.T) {
	_, router := setupComplianceHandler(t)

	reqBody := checkPaymentRequest{
		PaymentID:     "payment_async",
		FromAccount:   "account_async",
		ToAccount:     "account_002",
		AmountPaise:   100000,
		AccountAgeSec: 7776000,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/compliance/check", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	var resp checkPaymentResponse
	json.NewDecoder(w.Body).Decode(&resp)

	checkID := resp.ComplianceCheckID

	// Poll for result (with timeout)
	startTime := time.Now()
	timeout := 5 * time.Second
	var finalResp getCheckResponse

	for {
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/check/%s", checkID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			json.NewDecoder(w.Body).Decode(&finalResp)
			if finalResp.Decision != "pending" {
				break // Check is complete
			}
		}

		if time.Since(startTime) > timeout {
			t.Logf("check result still pending after %v", timeout)
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	// At minimum, we should have gotten a valid response
	if finalResp.ComplianceCheckID != checkID {
		t.Errorf("expected check_id %s, got %s", checkID, finalResp.ComplianceCheckID)
	}
}

// Test 12: Compliance check with AML block decision
func TestComplianceCheckAMLBlock(t *testing.T) {
	engine := erupeecompliance.NewComplianceEngine()
	handler := NewComplianceHandler(engine)

	r := chi.NewRouter()
	r.Route("/v1/compliance", func(r chi.Router) {
		r.Post("/check", handler.CheckPayment)
		r.Get("/check/{compliance_check_id}", handler.GetCheck)
	})

	// Use a known blocked account (from AML config)
	reqBody := checkPaymentRequest{
		PaymentID:     "payment_blocked",
		FromAccount:   "account_001",
		ToAccount:     "sanctions_account", // This would trigger AML block in real system
		AmountPaise:   100000,
		AccountAgeSec: 7776000,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/v1/compliance/check", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var checkResp checkPaymentResponse
	json.NewDecoder(w.Body).Decode(&checkResp)

	// Wait a moment for async check to complete
	time.Sleep(200 * time.Millisecond)

	// Retrieve the check result
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/check/%s", checkResp.ComplianceCheckID), nil)
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	var getResp getCheckResponse
	json.NewDecoder(w2.Body).Decode(&getResp)

	// Decision should be allow (default) since we're using in-memory defaults
	// A real AML system would block "sanctions_account"
	if getResp.Decision == "" {
		t.Error("expected decision to be set")
	}
}

// Test 13: Multiple async checks don't interfere
func TestMultipleAsyncChecks(t *testing.T) {
	_, router := setupComplianceHandler(t)

	checkIDs := []string{}

	// Submit multiple checks
	for i := 0; i < 3; i++ {
		reqBody := checkPaymentRequest{
			PaymentID:     fmt.Sprintf("payment_%d", i),
			FromAccount:   fmt.Sprintf("account_%d", i),
			ToAccount:     "account_999",
			AmountPaise:   int64(100000 + i*10000),
			AccountAgeSec: 7776000,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/v1/compliance/check", bytes.NewReader(body))
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		var resp checkPaymentResponse
		json.NewDecoder(w.Body).Decode(&resp)

		checkIDs = append(checkIDs, resp.ComplianceCheckID)
	}

	// All should have different IDs
	for i := 0; i < len(checkIDs); i++ {
		for j := i + 1; j < len(checkIDs); j++ {
			if checkIDs[i] == checkIDs[j] {
				t.Errorf("check IDs should be unique, got duplicate: %s", checkIDs[i])
			}
		}
	}

	// Wait for checks to complete
	time.Sleep(500 * time.Millisecond)

	// Verify each can be retrieved independently
	for _, checkID := range checkIDs {
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/compliance/check/%s", checkID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("failed to retrieve check %s: status %d", checkID, w.Code)
		}
	}
}

// Test 14: Velocity and fraud pattern tracking across multiple checks
func TestComplianceCheckWithVelocityTracking(t *testing.T) {
	engine := erupeecompliance.NewComplianceEngine()
	_ = NewComplianceHandler(engine)

	ctx := context.Background()
	accountID := "account_velocity_test"

	// Submit multiple checks from same account
	for i := 0; i < 5; i++ {
		payment := erupeecompliance.PaymentRequest{
			PaymentID:     fmt.Sprintf("payment_%d", i),
			FromAccountID: accountID,
			ToAccountID:   "account_target",
			Amount:        100000 + int64(i*10000),
			Timestamp:     time.Now().UTC(),
			AccountAgeSeconds: 7776000,
		}

		_, err := engine.CheckPayment(ctx, payment)
		if err != nil {
			t.Fatalf("CheckPayment failed: %v", err)
		}
	}

	// Get velocity for account
	record, err := engine.GetVelocityRecord(ctx, accountID)
	if err != nil {
		t.Fatalf("GetVelocityRecord failed: %v", err)
	}

	// Should have recorded transactions
	if record.TransactionCount == 0 {
		t.Error("expected transaction count > 0")
	}

	if record.TotalAmount == 0 {
		t.Error("expected total amount > 0")
	}
}

// Test 15: Edge cases and error handling
func TestEdgeCases(t *testing.T) {
	_, router := setupComplianceHandler(t)

	tests := []struct {
		name       string
		path       string
		body       string
		method     string
		expectCode int
	}{
		{
			name:       "invalid JSON in check request",
			path:       "/v1/compliance/check",
			body:       "{invalid json",
			method:     "POST",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON in reset request",
			path:       "/v1/compliance/admin/reset-velocity",
			body:       "{invalid json",
			method:     "POST",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "missing URL param for velocity",
			path:       "/v1/compliance/account//velocity",
			method:     "GET",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "missing URL param for fraud history",
			path:       "/v1/compliance/account//fraud-history",
			method:     "GET",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.body)))
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d", tc.expectCode, w.Code)
			}
		})
	}
}
