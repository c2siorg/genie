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

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/erupeepayment"
	"github.com/go-chi/chi/v5"
)

// TestInitiatePayment_Success tests the happy path for initiating a payment.
func TestInitiatePayment_Success(t *testing.T) {
	// Setup
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	agent := erupeepayment.NewPaymentAgent(accountMgr, txnLog)
	handler := &Payment{
		PaymentAgent:   agent,
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	// Create test accounts with balances
	from, _ := accountMgr.CreateAccount("holder1", erupeepayment.TypePersonal)
	accountMgr.UpdateBalance(from.ID, 100_000)
	to, _ := accountMgr.CreateAccount("holder2", erupeepayment.TypePersonal)

	// Create request
	req := InitiatePaymentRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 50_000,
		Reference:   "INV-001",
	}
	body, _ := json.Marshal(req)

	// Make request
	r := httptest.NewRequest("POST", "/v1/payment/initiate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.InitiatePayment(w, r)

	// Verify response
	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp InitiatePaymentResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.PaymentID == "" {
		t.Error("expected payment_id to be set")
	}
	if resp.Status != erupeepayment.StatusPending {
		t.Errorf("expected status %s, got %s", erupeepayment.StatusPending, resp.Status)
	}
}

// TestInitiatePayment_InvalidAmount tests error handling for invalid amounts.
func TestInitiatePayment_InvalidAmount(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	agent := erupeepayment.NewPaymentAgent(accountMgr, txnLog)
	handler := &Payment{
		PaymentAgent:   agent,
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	req := InitiatePaymentRequest{
		FromAccount: "acct_1",
		ToAccount:   "acct_2",
		AmountPaise: 0,
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest("POST", "/v1/payment/initiate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.InitiatePayment(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestInitiatePayment_MissingAccounts tests error handling for missing accounts.
func TestInitiatePayment_MissingAccounts(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	agent := erupeepayment.NewPaymentAgent(accountMgr, txnLog)
	handler := &Payment{
		PaymentAgent:   agent,
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	req := InitiatePaymentRequest{
		FromAccount: "nonexistent",
		ToAccount:   "nonexistent",
		AmountPaise: 1000,
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest("POST", "/v1/payment/initiate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.InitiatePayment(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestGetPayment_Success tests retrieving a payment.
func TestGetPayment_Success(t *testing.T) {
	// Setup
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	handler := &Payment{
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	// Create a transaction record
	txn := &erupeepayment.TransactionRecord{
		PaymentID:   "pay_123",
		FromAccount: "acct_1",
		ToAccount:   "acct_2",
		Amount:      50_000,
		Timestamp:   time.Now(),
		Status:      erupeepayment.StatusPending,
	}
	txnLog.Record(txn)

	// Make request with chi context
	r := httptest.NewRequest("GET", "/v1/payment/pay_123", nil)
	w := httptest.NewRecorder()

	// Set chi URL parameter in context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("payment_id", "pay_123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.GetPayment(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp GetPaymentResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.PaymentID != "pay_123" {
		t.Errorf("expected payment_id pay_123, got %s", resp.PaymentID)
	}
}

// TestGetPayment_NotFound tests 404 for missing payment.
func TestGetPayment_NotFound(t *testing.T) {
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	handler := &Payment{
		TransactionLog: txnLog,
	}

	r := httptest.NewRequest("GET", "/v1/payment/nonexistent", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("payment_id", "nonexistent")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.GetPayment(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestCreateAccount_Success tests creating an account.
func TestCreateAccount_Success(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	handler := &Payment{
		AccountManager: accountMgr,
	}

	req := CreateAccountRequest{
		HolderID:            "holder123",
		AccountType:         erupeepayment.TypePersonal,
		InitialBalancePaise: 100_000,
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest("POST", "/v1/account", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateAccount(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp CreateAccountResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.AccountID == "" {
		t.Error("expected account_id to be set")
	}
	if resp.HolderID != "holder123" {
		t.Errorf("expected holder_id holder123, got %s", resp.HolderID)
	}
	if resp.BalancePaise != 100_000 {
		t.Errorf("expected balance 100000, got %d", resp.BalancePaise)
	}
}

// TestCreateAccount_InvalidType tests error handling for invalid account type.
func TestCreateAccount_InvalidType(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	handler := &Payment{
		AccountManager: accountMgr,
	}

	req := CreateAccountRequest{
		HolderID:    "holder123",
		AccountType: erupeepayment.AccountType("invalid"),
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest("POST", "/v1/account", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateAccount(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreateAccount_MissingHolderID tests error handling for missing holder_id.
func TestCreateAccount_MissingHolderID(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	handler := &Payment{
		AccountManager: accountMgr,
	}

	req := CreateAccountRequest{
		HolderID:    "",
		AccountType: erupeepayment.TypePersonal,
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest("POST", "/v1/account", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateAccount(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestGetAccount_Success tests retrieving an account.
func TestGetAccount_Success(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	handler := &Payment{
		AccountManager: accountMgr,
	}

	// Create account
	acct, _ := accountMgr.CreateAccount("holder123", erupeepayment.TypeMerchant)

	r := httptest.NewRequest("GET", "/v1/account/"+acct.ID, nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("account_id", acct.ID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.GetAccount(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp GetAccountResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.AccountID != acct.ID {
		t.Errorf("expected account_id %s, got %s", acct.ID, resp.AccountID)
	}
}

// TestGetAccount_NotFound tests 404 for missing account.
func TestGetAccount_NotFound(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	handler := &Payment{
		AccountManager: accountMgr,
	}

	r := httptest.NewRequest("GET", "/v1/account/nonexistent", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("account_id", "nonexistent")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.GetAccount(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestListTransactions_Success tests listing transactions for an account.
func TestListTransactions_Success(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	handler := &Payment{
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	// Create account
	acct, _ := accountMgr.CreateAccount("holder1", erupeepayment.TypePersonal)

	// Create transactions
	now := time.Now()
	future := now.Add(2 * time.Hour) // Far future to ensure it's within range
	txn1 := &erupeepayment.TransactionRecord{
		PaymentID:   "pay_1",
		FromAccount: acct.ID,
		ToAccount:   "acct_2",
		Amount:      10_000,
		Timestamp:   now,
		Status:      erupeepayment.StatusConfirmed,
	}
	txn2 := &erupeepayment.TransactionRecord{
		PaymentID:   "pay_2",
		FromAccount: "acct_3",
		ToAccount:   acct.ID,
		Amount:      20_000,
		Timestamp:   now.Add(time.Hour),
		Status:      erupeepayment.StatusPending,
	}
	txnLog.Record(txn1)
	txnLog.Record(txn2)

	// Query with explicit time range that includes both transactions
	sinceStr := fmt.Sprintf("%d", now.Add(-1*time.Hour).Unix())
	untilStr := fmt.Sprintf("%d", future.Unix())

	r := httptest.NewRequest("GET", "/v1/transaction/"+acct.ID+"?since="+sinceStr+"&until="+untilStr, nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("account_id", acct.ID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.ListTransactions(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ListTransactionsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Transactions) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(resp.Transactions))
	}
}

// TestListTransactions_AccountNotFound tests 404 for missing account.
func TestListTransactions_AccountNotFound(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	handler := &Payment{
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	r := httptest.NewRequest("GET", "/v1/transaction/nonexistent", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("account_id", "nonexistent")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.ListTransactions(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestListTransactions_WithLimit tests the limit query parameter.
func TestListTransactions_WithLimit(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	handler := &Payment{
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	// Create account
	acct, _ := accountMgr.CreateAccount("holder1", erupeepayment.TypePersonal)

	// Create 5 transactions
	now := time.Now()
	for i := 0; i < 5; i++ {
		txn := &erupeepayment.TransactionRecord{
			PaymentID:   "pay_" + fmt.Sprintf("%d", i),
			FromAccount: acct.ID,
			ToAccount:   "acct_2",
			Amount:      10_000,
			Timestamp:   now.Add(time.Duration(i) * time.Hour),
			Status:      erupeepayment.StatusConfirmed,
		}
		txnLog.Record(txn)
	}

	// Query with time range that includes all transactions
	sinceStr := fmt.Sprintf("%d", now.Add(-1*time.Hour).Unix())
	untilStr := fmt.Sprintf("%d", now.Add(5*time.Hour).Unix())

	r := httptest.NewRequest("GET", "/v1/transaction/"+acct.ID+"?limit=2&since="+sinceStr+"&until="+untilStr, nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("account_id", acct.ID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.ListTransactions(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ListTransactionsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Transactions) != 2 {
		t.Errorf("expected 2 transactions (limited), got %d", len(resp.Transactions))
	}
}

// TestListTransactions_WithTimeRange tests the since/until query parameters.
func TestListTransactions_WithTimeRange(t *testing.T) {
	accountMgr := erupeepayment.NewInMemoryAccountManager(nil)
	txnLog := erupeepayment.NewInMemoryTransactionLog(nil)
	handler := &Payment{
		AccountManager: accountMgr,
		TransactionLog: txnLog,
	}

	// Create account
	acct, _ := accountMgr.CreateAccount("holder1", erupeepayment.TypePersonal)

	// Create transactions at different times
	baseTime := time.Now()
	txn1 := &erupeepayment.TransactionRecord{
		PaymentID:   "pay_1",
		FromAccount: acct.ID,
		ToAccount:   "acct_2",
		Amount:      10_000,
		Timestamp:   baseTime.Add(-2 * time.Hour),
		Status:      erupeepayment.StatusConfirmed,
	}
	txn2 := &erupeepayment.TransactionRecord{
		PaymentID:   "pay_2",
		FromAccount: acct.ID,
		ToAccount:   "acct_2",
		Amount:      10_000,
		Timestamp:   baseTime,
		Status:      erupeepayment.StatusConfirmed,
	}
	txn3 := &erupeepayment.TransactionRecord{
		PaymentID:   "pay_3",
		FromAccount: acct.ID,
		ToAccount:   "acct_2",
		Amount:      10_000,
		Timestamp:   baseTime.Add(2 * time.Hour),
		Status:      erupeepayment.StatusConfirmed,
	}
	txnLog.Record(txn1)
	txnLog.Record(txn2)
	txnLog.Record(txn3)

	// Query with time range
	since := baseTime.Add(-1 * time.Hour).Unix()
	until := baseTime.Add(1 * time.Hour).Unix()

	r := httptest.NewRequest(
		"GET",
		"/v1/transaction/"+acct.ID+"?since="+fmt.Sprintf("%d", since)+"&until="+fmt.Sprintf("%d", until),
		nil,
	)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("account_id", acct.ID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.ListTransactions(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ListTransactionsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Should only include txn2 (within the range)
	if len(resp.Transactions) != 1 {
		t.Errorf("expected 1 transaction (within range), got %d", len(resp.Transactions))
	}
}
