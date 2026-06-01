package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/c2siorg/genie/pkg/erupeecompliance"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ComplianceHandler handles payment compliance checking endpoints.
type ComplianceHandler struct {
	engine          *erupeecompliance.ComplianceEngine
	mu              sync.RWMutex
	checkResults    map[string]*erupeecompliance.ComplianceCheck
	pendingChecks   map[string]bool
	patternHistory  map[string][]*FraudPatternEntry
}

// FraudPatternEntry represents a fraud pattern detection with timestamp.
type FraudPatternEntry struct {
	PatternType erupeecompliance.FraudPattern `json:"pattern_type"`
	DetectedAt  time.Time                     `json:"detected_at"`
	Evidence    string                        `json:"evidence"`
}

// NewComplianceHandler creates a new compliance handler.
func NewComplianceHandler(engine *erupeecompliance.ComplianceEngine) *ComplianceHandler {
	return &ComplianceHandler{
		engine:         engine,
		checkResults:   make(map[string]*erupeecompliance.ComplianceCheck),
		pendingChecks:  make(map[string]bool),
		patternHistory: make(map[string][]*FraudPatternEntry),
	}
}

// checkPaymentRequest is the request body for POST /v1/compliance/check
type checkPaymentRequest struct {
	PaymentID     string `json:"payment_id"`
	FromAccount   string `json:"from_account"`
	ToAccount     string `json:"to_account"`
	ToName        string `json:"to_name,omitempty"`
	AmountPaise   int64  `json:"amount_paise"`
	AccountAgeSec int64  `json:"account_age_seconds,omitempty"`
}

// checkPaymentResponse is the response body for POST /v1/compliance/check
type checkPaymentResponse struct {
	ComplianceCheckID string   `json:"compliance_check_id"`
	AMLResult         string   `json:"aml_result"`
	VelocityResult    string   `json:"velocity_result"`
	FraudScore        float64  `json:"fraud_score"`
	Decision          string   `json:"decision"`
	Reasons           []string `json:"reasons"`
	CheckedAt         time.Time `json:"checked_at,omitempty"`
}

// getCheckResponse is the response body for GET /v1/compliance/check/{id}
type getCheckResponse struct {
	ComplianceCheckID string   `json:"compliance_check_id"`
	PaymentID         string   `json:"payment_id"`
	AMLResult         string   `json:"aml_result"`
	VelocityResult    string   `json:"velocity_result"`
	FraudScore        float64  `json:"fraud_score"`
	Decision          string   `json:"decision"`
	CheckedAt         time.Time `json:"checked_at"`
}

// velocityResponse is the response body for GET /v1/compliance/account/{id}/velocity
type velocityResponse struct {
	AccountID             string                 `json:"account_id"`
	TransactionCountHourly int                   `json:"txn_count_hourly"`
	TotalAmountHourly     int64                  `json:"total_amount_hourly"`
	DailyTotal            int64                  `json:"daily_total"`
	Limits                velocityLimits         `json:"limits"`
	Status                string                 `json:"status"`
}

type velocityLimits struct {
	HourlyTransactions int64 `json:"hourly_txn"`
	HourlyAmount       int64 `json:"hourly_amount"`
	DailyAmount        int64 `json:"daily_amount"`
}

// fraudHistoryResponse is the response body for GET /v1/compliance/account/{id}/fraud-history
type fraudHistoryResponse struct {
	AccountID      string                  `json:"account_id"`
	FraudPatterns  []*FraudPatternEntry    `json:"fraud_patterns"`
	FraudScore     float64                 `json:"fraud_score"`
	RiskLevel      string                  `json:"risk_level"`
}

// resetVelocityRequest is the request body for POST /v1/compliance/admin/reset-velocity
type resetVelocityRequest struct {
	AccountID string `json:"account_id"`
}

// resetVelocityResponse is the response body for POST /v1/compliance/admin/reset-velocity
type resetVelocityResponse struct {
	AccountID string    `json:"account_id"`
	ResetAt   time.Time `json:"reset_at"`
}

// CheckPayment handles POST /v1/compliance/check
// Performs async compliance check and returns immediately with check ID.
func (h *ComplianceHandler) CheckPayment(w http.ResponseWriter, r *http.Request) {
	var req checkPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.PaymentID == "" || req.FromAccount == "" || req.ToAccount == "" || req.AmountPaise <= 0 {
		http.Error(w, "missing required fields: payment_id, from_account, to_account, amount_paise", http.StatusBadRequest)
		return
	}

	// Generate compliance check ID
	checkID := uuid.New().String()

	// Build payment request
	payment := erupeecompliance.PaymentRequest{
		PaymentID:         req.PaymentID,
		FromAccountID:     req.FromAccount,
		ToAccountID:       req.ToAccount,
		ToName:            req.ToName,
		Amount:            req.AmountPaise,
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: req.AccountAgeSec,
	}

	// Mark as pending and run check asynchronously
	h.mu.Lock()
	h.pendingChecks[checkID] = true
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.pendingChecks, checkID)
			h.mu.Unlock()
		}()

		// Run compliance check
		check, err := h.engine.CheckPayment(r.Context(), payment)
		if err != nil {
			// Store error state (simplified: skip on error)
			return
		}

		// Store result
		h.mu.Lock()
		h.checkResults[checkID] = check

		// Update fraud pattern history
		if len(check.DetectedPatterns) > 0 {
			if h.patternHistory[req.FromAccount] == nil {
				h.patternHistory[req.FromAccount] = []*FraudPatternEntry{}
			}
			for _, pattern := range check.DetectedPatterns {
				h.patternHistory[req.FromAccount] = append(h.patternHistory[req.FromAccount], &FraudPatternEntry{
					PatternType: pattern,
					DetectedAt:  check.CheckedAt,
					Evidence:    check.FraudReason,
				})
			}
		}
		h.mu.Unlock()
	}()

	// Build reasons array
	reasons := []string{}
	if req.AmountPaise > 50_00_000 { // High value indicator
		reasons = append(reasons, "High transaction amount")
	}

	// Return check ID immediately (async response)
	resp := checkPaymentResponse{
		ComplianceCheckID: checkID,
		AMLResult:         "pending",
		VelocityResult:    "pending",
		FraudScore:        0,
		Decision:          "pending",
		Reasons:           reasons,
	}

	respondJSON(w, http.StatusAccepted, resp)
}

// GetCheck handles GET /v1/compliance/check/{compliance_check_id}
// Retrieves a previously performed compliance check result.
func (h *ComplianceHandler) GetCheck(w http.ResponseWriter, r *http.Request) {
	checkID := chi.URLParam(r, "compliance_check_id")
	if checkID == "" {
		http.Error(w, "compliance_check_id required", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	check, exists := h.checkResults[checkID]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, fmt.Sprintf("compliance check %s not found", checkID), http.StatusNotFound)
		return
	}

	resp := getCheckResponse{
		ComplianceCheckID: checkID,
		PaymentID:         check.PaymentID,
		AMLResult:         string(check.AMLResult),
		VelocityResult:    string(check.VelocityResult),
		FraudScore:        check.FraudScore,
		Decision:          string(check.Decision),
		CheckedAt:         check.CheckedAt,
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetVelocity handles GET /v1/compliance/account/{account_id}/velocity
// Returns current velocity metrics for an account.
func (h *ComplianceHandler) GetVelocity(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		http.Error(w, "account_id required", http.StatusBadRequest)
		return
	}

	// Get velocity record from engine
	record, err := h.engine.GetVelocityRecord(r.Context(), accountID)
	if err != nil {
		// Return zero record if not found
		record = &erupeecompliance.VelocityRecord{
			AccountID:        accountID,
			TransactionCount: 0,
			TotalAmount:      0,
			Period:           "hourly",
			LastReset:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
	}

	// Get daily total (simplified: would need separate daily tracking)
	dailyTotal := record.TotalAmount
	if record.Period != "daily" {
		// Approximate daily from hourly
		dailyTotal = record.TotalAmount * 24
	}

	// Build limits response
	limits := velocityLimits{
		HourlyTransactions: int64(erupeecompliance.DefaultVelocityConfig().MaxTransactionsPerHour),
		HourlyAmount:       erupeecompliance.DefaultVelocityConfig().MaxAmountPerHour,
		DailyAmount:        erupeecompliance.DefaultVelocityConfig().MaxAmountPerDay,
	}

	// Determine status
	status := "healthy"
	if record.TransactionCount >= 9 {
		status = "warning"
	}
	if record.TransactionCount >= 10 {
		status = "exceeded"
	}

	resp := velocityResponse{
		AccountID:             accountID,
		TransactionCountHourly: record.TransactionCount,
		TotalAmountHourly:     record.TotalAmount,
		DailyTotal:            dailyTotal,
		Limits:                limits,
		Status:                status,
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetFraudHistory handles GET /v1/compliance/account/{account_id}/fraud-history
// Returns fraud pattern detection history for an account.
func (h *ComplianceHandler) GetFraudHistory(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		http.Error(w, "account_id required", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	patterns := h.patternHistory[accountID]
	h.mu.RUnlock()

	if patterns == nil {
		patterns = []*FraudPatternEntry{}
	}

	// Calculate risk level based on pattern count
	riskLevel := "low"
	fraudScore := 0.0

	if len(patterns) > 0 {
		fraudScore = float64(len(patterns)) * 15.0
		if fraudScore > 100.0 {
			fraudScore = 100.0
		}

		if len(patterns) <= 2 {
			riskLevel = "medium"
		} else {
			riskLevel = "high"
		}
	}

	resp := fraudHistoryResponse{
		AccountID:     accountID,
		FraudPatterns: patterns,
		FraudScore:    fraudScore,
		RiskLevel:     riskLevel,
	}

	respondJSON(w, http.StatusOK, resp)
}

// ResetVelocity handles POST /v1/compliance/admin/reset-velocity
// Admin operation to reset velocity counters for an account.
func (h *ComplianceHandler) ResetVelocity(w http.ResponseWriter, r *http.Request) {
	var req resetVelocityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AccountID == "" {
		http.Error(w, "account_id required", http.StatusBadRequest)
		return
	}

	// Reset velocity in engine
	if err := h.engine.ResetVelocity(r.Context(), req.AccountID); err != nil {
		http.Error(w, fmt.Sprintf("reset failed: %v", err), http.StatusInternalServerError)
		return
	}

	resp := resetVelocityResponse{
		AccountID: req.AccountID,
		ResetAt:   time.Now().UTC(),
	}

	respondJSON(w, http.StatusOK, resp)
}
