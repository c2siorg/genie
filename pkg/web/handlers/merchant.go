// Package handlers provides HTTP endpoints for merchant onboarding and management.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/merchant"
	"github.com/go-chi/chi/v5"
)

// MerchantHandler orchestrates merchant onboarding and management HTTP endpoints.
type MerchantHandler struct {
	merchantManager    merchant.MerchantManager
	onboardingWorkflow merchant.OnboardingWorkflow
}

// NewMerchantHandler creates a new merchant handler.
func NewMerchantHandler(mm merchant.MerchantManager, ow merchant.OnboardingWorkflow) *MerchantHandler {
	return &MerchantHandler{
		merchantManager:    mm,
		onboardingWorkflow: ow,
	}
}

// ===================== Request/Response types =====================

// OnboardMerchantRequest represents a merchant onboarding submission.
type OnboardMerchantRequest struct {
	BusinessName string          `json:"business_name"`
	OwnerID      string          `json:"owner_id"`
	BusinessType string          `json:"business_type"`
	GSTNumber    string          `json:"gst_number,omitempty"`
	Documents    []DocumentInput `json:"documents"`
}

// DocumentInput represents a document to be uploaded.
type DocumentInput struct {
	Type    string `json:"type"`
	Content string `json:"content"` // base64 or URL for MVP
}

// OnboardMerchantResponse represents the response after onboarding submission.
type OnboardMerchantResponse struct {
	MerchantID          string `json:"merchant_id"`
	Status              string `json:"status"`
	OnboardingRequestID string `json:"onboarding_request_id"`
	ComplianceStatus    string `json:"compliance_status"`
}

// MerchantResponse represents a merchant profile.
type MerchantResponse struct {
	MerchantID        string    `json:"merchant_id"`
	BusinessName      string    `json:"business_name"`
	OwnerID           string    `json:"owner_id"`
	BusinessType      string    `json:"business_type"`
	GSTNumber         string    `json:"gst_number,omitempty"`
	SettlementAccount string    `json:"settlement_account,omitempty"`
	Status            string    `json:"status"`
	DailyLimitPaise   int64     `json:"daily_limit_paise"`
	CreatedAt         time.Time `json:"created_at"`
}

// OnboardingStatusResponse represents the status of an onboarding request.
type OnboardingStatusResponse struct {
	MerchantID      string                  `json:"merchant_id"`
	KYCResult       *KYCResultResponse      `json:"kyc_result,omitempty"`
	ComplianceCheck *ComplianceResponse     `json:"compliance_check,omitempty"`
	ApprovalStatus  *ApprovalStatusResponse `json:"approval_status,omitempty"`
	Reason          string                  `json:"reason,omitempty"`
}

// KYCResultResponse represents KYC verification results.
type KYCResultResponse struct {
	Status                   string    `json:"status"`
	GSTVerified              bool      `json:"gst_verified"`
	PANVerified              bool      `json:"pan_verified"`
	BusinessAddressConfirmed bool      `json:"business_address_confirmed"`
	OwnerIdentityVerified    bool      `json:"owner_identity_verified"`
	VerificationDate         time.Time `json:"verification_date"`
	Issues                   []string  `json:"issues,omitempty"`
}

// ComplianceResponse represents compliance check results.
type ComplianceResponse struct {
	Status    string                   `json:"status"`
	RiskLevel string                   `json:"risk_level"`
	Flags     []ComplianceFlagResponse `json:"flags,omitempty"`
	CheckDate time.Time                `json:"check_date"`
}

// ComplianceFlagResponse represents a single compliance flag.
type ComplianceFlagResponse struct {
	Category           string `json:"category"`
	Severity           string `json:"severity"`
	Description        string `json:"description"`
	ResolutionRequired bool   `json:"resolution_required"`
}

// ApprovalStatusResponse represents approval policy evaluation.
type ApprovalStatusResponse struct {
	Decision    string    `json:"decision"`
	Reason      string    `json:"reason"`
	RiskScore   float64   `json:"risk_score"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// ApproveMerchantRequest represents an admin approval submission.
type ApproveMerchantRequest struct {
	// Empty body for MVP, can be extended with approval notes
}

// ApproveMerchantResponse represents the result of approval.
type ApproveMerchantResponse struct {
	MerchantID        string `json:"merchant_id"`
	Status            string `json:"status"`
	SettlementAccount string `json:"settlement_account,omitempty"`
}

// UpdateLimitsRequest represents a request to update transaction limits.
type UpdateLimitsRequest struct {
	DailyLimitPaise     int64 `json:"daily_limit_paise"`
	SingleTxnLimitPaise int64 `json:"single_txn_limit_paise"`
}

// UpdateLimitsResponse represents the result of updating limits.
type UpdateLimitsResponse struct {
	MerchantID          string `json:"merchant_id"`
	DailyLimitPaise     int64  `json:"daily_limit_paise"`
	SingleTxnLimitPaise int64  `json:"single_txn_limit_paise"`
}

// ===================== Validation functions =====================

// isValidGST validates GST number format (15 alphanumeric).
//
// A GSTIN is a canonical identifier that is always uppercase. We reject
// lowercase input rather than silently upper-casing it, so a malformed
// (lowercase) number is never accepted and stored un-normalized.
func isValidGST(gst string) bool {
	if gst == "" {
		return true // Optional field
	}
	matched, _ := regexp.MatchString(`^[0-9A-Z]{15}$`, gst)
	return matched
}

// isValidPAN validates PAN format (10 alphanumeric, pattern: AAAAP1234A).
//
// Like GSTIN, a PAN is always uppercase; lowercase input is rejected rather
// than coerced.
func isValidPAN(pan string) bool {
	if pan == "" {
		return true // Optional field
	}
	matched, _ := regexp.MatchString(`^[A-Z]{5}[0-9]{4}[A-Z]{1}$`, pan)
	return matched
}

// isValidBusinessType validates business type enum.
func isValidBusinessType(bt string) bool {
	switch merchant.BusinessType(bt) {
	case merchant.BusinessTypeSole, merchant.BusinessTypeLLP, merchant.BusinessTypePvt, merchant.BusinessTypeGST:
		return true
	}
	return false
}

// ===================== HTTP Handlers =====================

// OnboardMerchant handles POST /v1/merchant/onboard
// Initiates merchant onboarding with KYC documents and business info.
func (h *MerchantHandler) OnboardMerchant(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var req OnboardMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	// Validate required fields
	if req.BusinessName == "" {
		respondError(w, http.StatusBadRequest, "business_name is required")
		return
	}
	if req.OwnerID == "" {
		respondError(w, http.StatusBadRequest, "owner_id is required")
		return
	}
	if req.BusinessType == "" {
		respondError(w, http.StatusBadRequest, "business_type is required")
		return
	}
	if !isValidBusinessType(req.BusinessType) {
		respondError(w, http.StatusBadRequest, "invalid business_type: must be sole, llp, pvt, or gst")
		return
	}
	if len(req.Documents) == 0 {
		respondError(w, http.StatusBadRequest, "at least one document is required")
		return
	}

	// Validate GST number if provided
	if req.GSTNumber != "" && !isValidGST(req.GSTNumber) {
		respondError(w, http.StatusBadRequest, "invalid GST number format (expected 15 alphanumeric)")
		return
	}

	// Create merchant profile
	profile, err := h.merchantManager.CreateMerchant(ctx, req.BusinessName, req.OwnerID, merchant.BusinessType(req.BusinessType))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create merchant: "+err.Error())
		return
	}

	// Store GST number if provided
	if req.GSTNumber != "" {
		// Note: In a real implementation, update the merchant with GST number
		// For now, we create documents below
	}

	// Convert input documents to onboarding documents
	onboardingDocs := make([]merchant.OnboardingDocument, len(req.Documents))
	for i, doc := range req.Documents {
		onboardingDocs[i] = merchant.OnboardingDocument{
			ID:         fmt.Sprintf("doc_%s_%d", profile.ID, i),
			Type:       doc.Type,
			URL:        doc.Content, // MVP: treat content as URL
			UploadedAt: time.Now(),
		}
	}

	// Submit KYC with documents
	onboardingReq, err := h.onboardingWorkflow.SubmitKYC(ctx, profile.ID, onboardingDocs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to submit KYC: "+err.Error())
		return
	}

	// Verify KYC
	_, err = h.onboardingWorkflow.VerifyKYC(ctx, onboardingReq.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "KYC verification failed: "+err.Error())
		return
	}

	// Check compliance
	_, err = h.onboardingWorkflow.CheckCompliance(ctx, onboardingReq.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "compliance check failed: "+err.Error())
		return
	}

	// Evaluate approval policy
	approvalStatus, err := h.onboardingWorkflow.EvaluateApproval(ctx, onboardingReq.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "approval evaluation failed: "+err.Error())
		return
	}

	// If auto-approve, finalize approval
	complianceStatus := approvalStatus.Decision
	if approvalStatus.Decision == "auto_approve" {
		_, _, err := h.onboardingWorkflow.ApproveOnboarding(ctx, onboardingReq.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "auto-approval failed: "+err.Error())
			return
		}
	} else if approvalStatus.Decision == "auto_reject" {
		// Auto-reject merchants on sanctions lists
		err := h.onboardingWorkflow.RejectOnboarding(ctx, onboardingReq.ID, approvalStatus.Reason)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "auto-rejection failed: "+err.Error())
			return
		}
		complianceStatus = "auto_reject"
	}

	resp := OnboardMerchantResponse{
		MerchantID:          profile.ID,
		Status:              string(profile.Status),
		OnboardingRequestID: onboardingReq.ID,
		ComplianceStatus:    complianceStatus,
	}

	respondJSON(w, http.StatusCreated, resp)
}

// GetMerchant handles GET /v1/merchant/{merchant_id}
// Retrieves merchant profile and details.
func (h *MerchantHandler) GetMerchant(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	merchantID := chi.URLParam(r, "merchant_id")
	if merchantID == "" {
		respondError(w, http.StatusBadRequest, "merchant_id is required")
		return
	}

	profile, err := h.merchantManager.GetMerchant(ctx, merchantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "merchant not found: "+err.Error())
		return
	}

	resp := MerchantResponse{
		MerchantID:        profile.ID,
		BusinessName:      profile.BusinessName,
		OwnerID:           profile.OwnerID,
		BusinessType:      string(profile.BusinessType),
		GSTNumber:         profile.GSTNumber,
		SettlementAccount: profile.SettlementAccount,
		Status:            string(profile.Status),
		DailyLimitPaise:   profile.DailyLimitPaise,
		CreatedAt:         profile.CreatedAt,
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetOnboardingStatus handles GET /v1/merchant/{merchant_id}/onboarding
// Retrieves the status of an onboarding request.
func (h *MerchantHandler) GetOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	merchantID := chi.URLParam(r, "merchant_id")
	if merchantID == "" {
		respondError(w, http.StatusBadRequest, "merchant_id is required")
		return
	}

	// Get merchant profile to verify existence
	_, err := h.merchantManager.GetMerchant(ctx, merchantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "merchant not found: "+err.Error())
		return
	}

	// In a real implementation, we'd look up the onboarding request by merchant ID
	// For now, we'll return a basic response based on merchant status
	resp := OnboardingStatusResponse{
		MerchantID: merchantID,
		Reason:     "",
	}

	// Note: To properly implement this, we'd need a way to query onboarding requests by merchant ID
	// The current in-memory implementation only supports lookup by request ID
	// In production, add an index in the OnboardingWorkflow implementation

	respondJSON(w, http.StatusOK, resp)
}

// ApproveMerchant handles POST /v1/merchant/{merchant_id}/approve
// Admin endpoint to approve an onboarded merchant.
func (h *MerchantHandler) ApproveMerchant(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	merchantID := chi.URLParam(r, "merchant_id")
	if merchantID == "" {
		respondError(w, http.StatusBadRequest, "merchant_id is required")
		return
	}

	// Get merchant profile
	profile, err := h.merchantManager.GetMerchant(ctx, merchantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "merchant not found: "+err.Error())
		return
	}

	if profile.Status != merchant.StatusPending {
		respondError(w, http.StatusConflict, fmt.Sprintf("cannot approve merchant with status %s", profile.Status))
		return
	}

	// Update status to approved
	if err := h.merchantManager.UpdateStatus(ctx, merchantID, merchant.StatusApproved, ""); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to approve merchant: "+err.Error())
		return
	}

	// Assign settlement account (in MVP, generate a dummy one)
	settlementAccount := fmt.Sprintf("SA_%s_%d", merchantID, time.Now().Unix())
	if err := h.merchantManager.AssignSettlementAccount(ctx, merchantID, settlementAccount); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to assign settlement account: "+err.Error())
		return
	}

	resp := ApproveMerchantResponse{
		MerchantID:        merchantID,
		Status:            "approved",
		SettlementAccount: settlementAccount,
	}

	respondJSON(w, http.StatusOK, resp)
}

// UpdateLimits handles POST /v1/merchant/{merchant_id}/limits
// Updates transaction limits for a merchant.
func (h *MerchantHandler) UpdateLimits(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	merchantID := chi.URLParam(r, "merchant_id")
	if merchantID == "" {
		respondError(w, http.StatusBadRequest, "merchant_id is required")
		return
	}

	var req UpdateLimitsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	// Validate limits
	if req.DailyLimitPaise <= 0 {
		respondError(w, http.StatusBadRequest, "daily_limit_paise must be positive")
		return
	}
	if req.SingleTxnLimitPaise <= 0 {
		respondError(w, http.StatusBadRequest, "single_txn_limit_paise must be positive")
		return
	}
	if req.SingleTxnLimitPaise > req.DailyLimitPaise {
		respondError(w, http.StatusBadRequest, "single_txn_limit_paise cannot exceed daily_limit_paise")
		return
	}

	// Get merchant to verify existence
	_, err := h.merchantManager.GetMerchant(ctx, merchantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "merchant not found: "+err.Error())
		return
	}

	// Create limits object
	limits := merchant.MerchantLimits{
		DailyP2PPaise:        req.DailyLimitPaise,
		DailySettlementPaise: req.DailyLimitPaise,
		SingleTxnPaise:       req.SingleTxnLimitPaise,
		MonthlyTotalPaise:    req.DailyLimitPaise * 30, // 30 days of daily limit
		EffectiveFrom:        time.Now(),
		Notes:                "Updated via API",
	}

	// Update limits
	if err := h.merchantManager.UpdateLimits(ctx, merchantID, limits); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update limits: "+err.Error())
		return
	}

	resp := UpdateLimitsResponse{
		MerchantID:          merchantID,
		DailyLimitPaise:     req.DailyLimitPaise,
		SingleTxnLimitPaise: req.SingleTxnLimitPaise,
	}

	respondJSON(w, http.StatusOK, resp)
}
