package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/cbdc"
	"github.com/go-chi/chi/v5"
)

// CBDCHandler orchestrates CBDC ledger and payment operations.
type CBDCHandler struct {
	cbdcBridge      cbdc.CBDCBridge
	ledger          cbdc.Ledger
	limitsValidator *cbdc.RBILimitsValidator
}

// NewCBDCHandler creates a CBDC handler with dependencies.
func NewCBDCHandler(bridge cbdc.CBDCBridge, ledger cbdc.Ledger, validator *cbdc.RBILimitsValidator) *CBDCHandler {
	return &CBDCHandler{
		cbdcBridge:      bridge,
		ledger:          ledger,
		limitsValidator: validator,
	}
}

// initTransactionRequest is the request payload for POST /v1/cbdc/transaction.
type initTransactionRequest struct {
	PaymentID   string `json:"payment_id"`
	FromAccount string `json:"from_account"`
	ToAccount   string `json:"to_account"`
	AmountPaise int64  `json:"amount_paise"`
}

// initTransactionResponse is the response payload for successful transaction initiation.
type initTransactionResponse struct {
	LedgerID          string                 `json:"ledger_id"`
	BlockHeight       uint64                 `json:"block_height"`
	Status            cbdc.TransactionStatus `json:"status"`
	FinalityTimestamp string                 `json:"finality_timestamp"`
}

// InitiateTransaction handles POST /v1/cbdc/transaction.
// Initiates a new payment transaction on the CBDC ledger.
func (h *CBDCHandler) InitiateTransaction(w http.ResponseWriter, r *http.Request) {
	var req initTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.PaymentID == "" || req.FromAccount == "" || req.ToAccount == "" || req.AmountPaise <= 0 {
		http.Error(w, "missing or invalid required fields: payment_id, from_account, to_account, amount_paise", http.StatusBadRequest)
		return
	}

	// Validate amount against single transaction limit
	allowed, reason := h.limitsValidator.ValidatePayment(req.FromAccount, req.AmountPaise, cbdc.TypeP2P)
	if !allowed {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": reason})
		return
	}

	// Commit the payment and record usage
	resp, err := h.cbdcBridge.InitiatePayment(req.PaymentID, req.FromAccount, req.ToAccount, req.AmountPaise)
	if err != nil {
		// Check for double-spend or other validation errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "invalid") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": errMsg})
			return
		}
		http.Error(w, "failed to initiate payment: "+errMsg, http.StatusInternalServerError)
		return
	}

	// Record the transaction against daily limits
	if err := h.limitsValidator.CommitTransaction(req.FromAccount, req.AmountPaise, cbdc.TypeP2P); err != nil {
		http.Error(w, "failed to record limit usage: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Format response
	responseBody := initTransactionResponse{
		LedgerID:          resp.LedgerID,
		BlockHeight:       resp.BlockHeight,
		Status:            resp.Status,
		FinalityTimestamp: resp.FinalityTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	respondJSON(w, http.StatusAccepted, responseBody)
}

// getTransactionResponse is the response payload for GET /v1/cbdc/transaction/{payment_id}.
type getTransactionResponse struct {
	PaymentID   string                 `json:"payment_id"`
	LedgerID    string                 `json:"ledger_id"`
	FromAccount string                 `json:"from_account"`
	ToAccount   string                 `json:"to_account"`
	AmountPaise int64                  `json:"amount_paise"`
	Status      cbdc.TransactionStatus `json:"status"`
	Timestamp   string                 `json:"timestamp"`
	BlockHeight uint64                 `json:"block_height"`
}

// GetTransaction handles GET /v1/cbdc/transaction/{payment_id}.
// Retrieves the status and details of a specific transaction.
func (h *CBDCHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "payment_id")
	if paymentID == "" {
		http.Error(w, "payment_id is required", http.StatusBadRequest)
		return
	}

	entry, found, err := h.ledger.GetStatus(paymentID)
	if err != nil {
		http.Error(w, "failed to query transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "transaction not found"})
		return
	}

	response := getTransactionResponse{
		PaymentID:   entry.PaymentID,
		LedgerID:    entry.ID,
		FromAccount: entry.FromAccount,
		ToAccount:   entry.ToAccount,
		AmountPaise: entry.AmountPaise,
		Status:      entry.Status,
		Timestamp:   entry.Timestamp.Format("2006-01-02T15:04:05Z"),
		BlockHeight: entry.BlockHeight,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// blockTransactionResponse represents a transaction within a block response.
type blockTransactionResponse struct {
	PaymentID string                 `json:"payment_id"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Amount    int64                  `json:"amount"`
	Status    cbdc.TransactionStatus `json:"status"`
}

// getBlockResponse is the response payload for GET /v1/cbdc/block/{block_height}.
type getBlockResponse struct {
	Height           uint64                     `json:"height"`
	Timestamp        string                     `json:"timestamp"`
	TransactionCount int                        `json:"transaction_count"`
	Hash             string                     `json:"hash"`
	PreviousHash     string                     `json:"previous_hash"`
	Transactions     []blockTransactionResponse `json:"transactions"`
}

// GetBlock handles GET /v1/cbdc/block/{block_height}.
// Retrieves all transactions in a specific ledger block.
func (h *CBDCHandler) GetBlock(w http.ResponseWriter, r *http.Request) {
	blockHeightStr := chi.URLParam(r, "block_height")
	if blockHeightStr == "" {
		http.Error(w, "block_height is required", http.StatusBadRequest)
		return
	}

	blockHeight, err := strconv.ParseUint(blockHeightStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid block_height: must be a valid unsigned integer", http.StatusBadRequest)
		return
	}

	block, found, err := h.ledger.QueryBlock(blockHeight)
	if err != nil {
		http.Error(w, "failed to query block: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "block not found"})
		return
	}

	// Convert block transactions to response format
	txns := make([]blockTransactionResponse, len(block.Transactions))
	for i, entry := range block.Transactions {
		txns[i] = blockTransactionResponse{
			PaymentID: entry.PaymentID,
			From:      entry.FromAccount,
			To:        entry.ToAccount,
			Amount:    entry.AmountPaise,
			Status:    entry.Status,
		}
	}

	response := getBlockResponse{
		Height:           block.Height,
		Timestamp:        block.Timestamp.Format("2006-01-02T15:04:05Z"),
		TransactionCount: len(block.Transactions),
		Hash:             block.Hash,
		PreviousHash:     block.PreviousHash,
		Transactions:     txns,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// getLimitsResponse is the response payload for GET /v1/cbdc/limits/{account_id}.
type getLimitsResponse struct {
	AccountID               string `json:"account_id"`
	DailyP2PLimitPaise      int64  `json:"daily_p2p_limit_paise"`
	DailyP2PUsedPaise       int64  `json:"daily_p2p_used_paise"`
	DailyMerchantLimitPaise int64  `json:"daily_merchant_limit_paise"`
	DailyMerchantUsedPaise  int64  `json:"daily_merchant_used_paise"`
	SingleTxnLimitPaise     int64  `json:"single_txn_limit_paise"`
	Status                  string `json:"status"`
}

// GetLimits handles GET /v1/cbdc/limits/{account_id}.
// Retrieves the transaction limits and current usage for an account.
func (h *CBDCHandler) GetLimits(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		http.Error(w, "account_id is required", http.StatusBadRequest)
		return
	}

	// Get limit state
	limitState, exists := h.limitsValidator.GetAccountLimits(accountID)
	if !exists {
		// Return default limits for account that hasn't transacted yet
		limitState = &cbdc.AccountLimitState{
			Account:     accountID,
			LastResetAt: cbdc.NewAccountLimitState(accountID).LastResetAt,
		}
	}

	// Get global limits
	limits := h.limitsValidator.GetLimits()

	// Determine status
	status := "active"
	if limitState.DailyP2PUsed >= limits.DailyP2PLimit {
		status = "p2p_limit_reached"
	}
	if limitState.DailyMerchantUsed >= limits.DailyMerchantLimit {
		status = "merchant_limit_reached"
	}

	response := getLimitsResponse{
		AccountID:               accountID,
		DailyP2PLimitPaise:      limits.DailyP2PLimit,
		DailyP2PUsedPaise:       limitState.DailyP2PUsed,
		DailyMerchantLimitPaise: limits.DailyMerchantLimit,
		DailyMerchantUsedPaise:  limitState.DailyMerchantUsed,
		SingleTxnLimitPaise:     limits.SingleTransactionLimit,
		Status:                  status,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// healthResponse is the response payload for GET /v1/cbdc/health.
type healthResponse struct {
	RBIBackendAvailable bool   `json:"rbi_backend_available"`
	LedgerHeight        uint64 `json:"ledger_height"`
	PendingTransactions int    `json:"pending_transactions"`
	FinalizedCount      int    `json:"finalized_count"`
}

// Health handles GET /v1/cbdc/health.
// Returns the health status of the CBDC ledger and bridge.
func (h *CBDCHandler) Health(w http.ResponseWriter, r *http.Request) {
	// Get ledger stats
	latestHeight := h.ledger.GetLatestBlockHeight()

	// Count pending and finalized transactions across all blocks
	pendingCount := 0
	finalizedCount := 0

	for i := uint64(0); i <= latestHeight; i++ {
		block, found, err := h.ledger.QueryBlock(i)
		if err != nil || !found {
			continue
		}
		for _, txn := range block.Transactions {
			switch txn.Status {
			case cbdc.StatusPending:
				pendingCount++
			case cbdc.StatusFinalized:
				finalizedCount++
			}
		}
	}

	// Simulate RBI backend availability (for now, always true; can be enhanced)
	rbiAvailable := true
	if bridge, ok := h.cbdcBridge.(*cbdc.MockCBDCBridge); ok {
		// Access private field indirectly through a method if available
		// For now, assume true since we control the bridge
		_ = bridge
	}

	response := healthResponse{
		RBIBackendAvailable: rbiAvailable,
		LedgerHeight:        latestHeight,
		PendingTransactions: pendingCount,
		FinalizedCount:      finalizedCount,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
