package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/erupeepayment"
	"github.com/go-chi/chi/v5"
)

// Payment handles HTTP requests for e-Rupee payments.
type Payment struct {
	PaymentAgent   *erupeepayment.PaymentAgent
	AccountManager erupeepayment.AccountManager
	TransactionLog erupeepayment.TransactionLog
}

// InitiatePaymentRequest is the HTTP request body for POST /v1/payment/initiate.
type InitiatePaymentRequest struct {
	FromAccount string                 `json:"from_account"`
	ToAccount   string                 `json:"to_account"`
	AmountPaise int64                  `json:"amount_paise"`
	Reference   string                 `json:"reference"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// InitiatePaymentResponse is the HTTP response for POST /v1/payment/initiate.
type InitiatePaymentResponse struct {
	PaymentID  string                  `json:"payment_id"`
	Status     erupeepayment.PaymentStatus `json:"status"`
	Timestamp  time.Time               `json:"timestamp"`
	LedgerID   string                  `json:"ledger_id,omitempty"`
}

// GetPaymentResponse is the HTTP response for GET /v1/payment/{payment_id}.
type GetPaymentResponse struct {
	PaymentID   string                  `json:"payment_id"`
	FromAccount string                  `json:"from_account"`
	ToAccount   string                  `json:"to_account"`
	AmountPaise int64                   `json:"amount_paise"`
	Status      erupeepayment.PaymentStatus `json:"status"`
	Timestamp   time.Time               `json:"timestamp"`
	LedgerID    string                  `json:"ledger_id,omitempty"`
}

// CreateAccountRequest is the HTTP request body for POST /v1/account.
type CreateAccountRequest struct {
	HolderID             string                      `json:"holder_id"`
	AccountType          erupeepayment.AccountType   `json:"account_type"`
	InitialBalancePaise  int64                       `json:"initial_balance_paise"`
}

// CreateAccountResponse is the HTTP response for POST /v1/account.
type CreateAccountResponse struct {
	AccountID       string                      `json:"account_id"`
	HolderID        string                      `json:"holder_id"`
	BalancePaise    int64                       `json:"balance_paise"`
	Status          erupeepayment.AccountStatus `json:"status"`
}

// GetAccountResponse is the HTTP response for GET /v1/account/{account_id}.
type GetAccountResponse struct {
	AccountID    string                      `json:"account_id"`
	HolderID     string                      `json:"holder_id"`
	BalancePaise int64                       `json:"balance_paise"`
	Status       erupeepayment.AccountStatus `json:"status"`
	CreatedAt    time.Time                   `json:"created_at"`
}

// TransactionResponse represents a transaction in the list response.
type TransactionResponse struct {
	ID          string                      `json:"id"`
	PaymentID   string                      `json:"payment_id"`
	FromAccount string                      `json:"from_account"`
	ToAccount   string                      `json:"to_account"`
	Amount      int64                       `json:"amount"`
	Status      erupeepayment.PaymentStatus `json:"status"`
	Timestamp   time.Time                   `json:"timestamp"`
	LedgerID    string                      `json:"ledger_id,omitempty"`
}

// ListTransactionsResponse is the HTTP response for GET /v1/transaction/{account_id}.
type ListTransactionsResponse struct {
	Transactions []*TransactionResponse `json:"transactions"`
}

// InitiatePayment handles POST /v1/payment/initiate
func (h *Payment) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	var req InitiatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate input
	if req.FromAccount == "" || req.ToAccount == "" {
		respondError(w, http.StatusBadRequest, "from_account and to_account are required")
		return
	}
	if req.AmountPaise <= 0 {
		respondError(w, http.StatusBadRequest, "amount_paise must be positive")
		return
	}

	// Create PaymentInitiationRequest for the agent
	paymentReq := erupeepayment.PaymentInitiationRequest{
		FromAccount: req.FromAccount,
		ToAccount:   req.ToAccount,
		AmountPaise: req.AmountPaise,
		Reference:   req.Reference,
		Metadata:    req.Metadata,
	}

	// Call the payment agent
	result := h.PaymentAgent.InitiatePayment(r.Context(), paymentReq, nil)

	// Map to HTTP response
	resp := InitiatePaymentResponse{
		PaymentID: result.PaymentID,
		Status:    result.Status,
		Timestamp: result.Timestamp,
	}

	if result.Status == erupeepayment.StatusFailed {
		respondError(w, http.StatusBadRequest, result.Error)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// GetPayment handles GET /v1/payment/{payment_id}
func (h *Payment) GetPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "payment_id")
	if paymentID == "" {
		respondError(w, http.StatusBadRequest, "payment_id is required")
		return
	}

	// Get transaction records for this payment
	transactions := h.TransactionLog.GetByPaymentID(paymentID)
	if len(transactions) == 0 {
		respondError(w, http.StatusNotFound, "payment not found")
		return
	}

	// Use the first transaction to get basic info
	txn := transactions[0]

	resp := GetPaymentResponse{
		PaymentID:   txn.PaymentID,
		FromAccount: txn.FromAccount,
		ToAccount:   txn.ToAccount,
		AmountPaise: txn.Amount,
		Status:      txn.Status,
		Timestamp:   txn.Timestamp,
		LedgerID:    txn.LedgerID,
	}

	respondJSON(w, http.StatusOK, resp)
}

// CreateAccount handles POST /v1/account
func (h *Payment) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate input
	if req.HolderID == "" {
		respondError(w, http.StatusBadRequest, "holder_id is required")
		return
	}
	if req.AccountType != erupeepayment.TypePersonal && req.AccountType != erupeepayment.TypeMerchant {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid account_type: must be '%s' or '%s'", erupeepayment.TypePersonal, erupeepayment.TypeMerchant))
		return
	}
	if req.InitialBalancePaise < 0 {
		respondError(w, http.StatusBadRequest, "initial_balance_paise cannot be negative")
		return
	}

	// Create the account
	acct, err := h.AccountManager.CreateAccount(req.HolderID, req.AccountType)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Set initial balance if provided
	if req.InitialBalancePaise > 0 {
		if err := h.AccountManager.UpdateBalance(acct.ID, req.InitialBalancePaise); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to set initial balance: "+err.Error())
			return
		}
		// Refresh account to get updated balance
		acct = h.AccountManager.GetAccount(acct.ID)
	}

	resp := CreateAccountResponse{
		AccountID:    acct.ID,
		HolderID:     acct.HolderID,
		BalancePaise: acct.BalancePaise,
		Status:       acct.Status,
	}

	respondJSON(w, http.StatusCreated, resp)
}

// GetAccount handles GET /v1/account/{account_id}
func (h *Payment) GetAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	acct := h.AccountManager.GetAccount(accountID)
	if acct == nil {
		respondError(w, http.StatusNotFound, "account not found")
		return
	}

	resp := GetAccountResponse{
		AccountID:    acct.ID,
		HolderID:     acct.HolderID,
		BalancePaise: acct.BalancePaise,
		Status:       acct.Status,
		CreatedAt:    acct.CreatedAt,
	}

	respondJSON(w, http.StatusOK, resp)
}

// ListTransactions handles GET /v1/transaction/{account_id}
func (h *Payment) ListTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	// Verify account exists
	if acct := h.AccountManager.GetAccount(accountID); acct == nil {
		respondError(w, http.StatusNotFound, "account not found")
		return
	}

	// Parse query parameters
	since := time.Unix(0, 0) // Beginning of Unix epoch
	until := time.Now()
	limit := 100

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		// Try parsing as Unix timestamp (seconds)
		if ts, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			since = time.Unix(ts, 0)
		} else {
			// Try RFC3339 format
			if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
				since = t
			} else {
				respondError(w, http.StatusBadRequest, "invalid 'since' format: use Unix timestamp or RFC3339")
				return
			}
		}
	}

	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		// Try parsing as Unix timestamp (seconds)
		if ts, err := strconv.ParseInt(untilStr, 10, 64); err == nil {
			until = time.Unix(ts, 0)
		} else {
			// Try RFC3339 format
			if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
				until = t
			} else {
				respondError(w, http.StatusBadRequest, "invalid 'until' format: use Unix timestamp or RFC3339")
				return
			}
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		} else {
			respondError(w, http.StatusBadRequest, "invalid 'limit': must be a positive integer")
			return
		}
	}

	// Query transactions
	txns := h.TransactionLog.Query(accountID, since, until)

	// Apply limit
	if len(txns) > limit {
		txns = txns[:limit]
	}

	// Convert to response format
	respTxns := make([]*TransactionResponse, len(txns))
	for i, txn := range txns {
		respTxns[i] = &TransactionResponse{
			ID:          txn.ID,
			PaymentID:   txn.PaymentID,
			FromAccount: txn.FromAccount,
			ToAccount:   txn.ToAccount,
			Amount:      txn.Amount,
			Status:      txn.Status,
			Timestamp:   txn.Timestamp,
			LedgerID:    txn.LedgerID,
		}
	}

	resp := ListTransactionsResponse{
		Transactions: respTxns,
	}

	respondJSON(w, http.StatusOK, resp)
}
