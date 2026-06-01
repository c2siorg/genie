package merchant

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// OnboardingWorkflow defines the interface for KYC and compliance onboarding.
type OnboardingWorkflow interface {
	// SubmitKYC initiates KYC verification with documents.
	SubmitKYC(ctx context.Context, merchantID string, documents []OnboardingDocument) (*MerchantOnboardingRequest, error)

	// VerifyKYC performs identity and business verification.
	VerifyKYC(ctx context.Context, requestID string) (*KYCVerificationResult, error)

	// CheckCompliance runs AML and sanctions screening.
	CheckCompliance(ctx context.Context, requestID string) (*ComplianceCheckResult, error)

	// EvaluateApproval applies policy rules to determine auto-approve/review/reject.
	EvaluateApproval(ctx context.Context, requestID string) (*ApprovalStatus, error)

	// ApproveOnboarding finalizes approval and transitions merchant to approved.
	ApproveOnboarding(ctx context.Context, requestID string) (bool, string, error)

	// RejectOnboarding rejects the onboarding request.
	RejectOnboarding(ctx context.Context, requestID string, reason string) error

	// GetOnboardingRequest retrieves an onboarding request by ID.
	GetOnboardingRequest(ctx context.Context, requestID string) (*MerchantOnboardingRequest, error)
}

// InMemoryOnboardingWorkflow provides thread-safe in-memory onboarding storage and logic.
type InMemoryOnboardingWorkflow struct {
	mu                  sync.RWMutex
	requests            map[string]*MerchantOnboardingRequest
	nextID              int64
	merchantManager     MerchantManager
	sanctionedMerchants map[string]bool // For testing auto-reject
}

// NewInMemoryOnboardingWorkflow creates a new in-memory onboarding workflow.
func NewInMemoryOnboardingWorkflow(merchantManager MerchantManager) *InMemoryOnboardingWorkflow {
	return &InMemoryOnboardingWorkflow{
		requests:            make(map[string]*MerchantOnboardingRequest),
		merchantManager:     merchantManager,
		sanctionedMerchants: make(map[string]bool),
	}
}

// SubmitKYC initiates KYC verification with documents.
func (w *InMemoryOnboardingWorkflow) SubmitKYC(ctx context.Context, merchantID string, documents []OnboardingDocument) (*MerchantOnboardingRequest, error) {
	if merchantID == "" {
		return nil, fmt.Errorf("merchant_id is required")
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("at least one document is required")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.nextID++
	id := fmt.Sprintf("onboard_%d", w.nextID)
	now := time.Now()

	req := &MerchantOnboardingRequest{
		ID:         id,
		MerchantID: merchantID,
		State:      OnboardingDocumentsReady,
		Documents:  documents,
		SubmittedAt: now,
		UpdatedAt:  now,
		AuditTrail: []AuditEntry{
			{
				ID:        fmt.Sprintf("audit_%d", w.nextID),
				Action:    "submit_kyc",
				Status:    "success",
				ActorID:   "merchant_agent",
				Timestamp: now,
				Details: map[string]string{
					"document_count": fmt.Sprintf("%d", len(documents)),
				},
			},
		},
	}

	w.requests[id] = req
	return req, nil
}

// VerifyKYC performs identity and business verification.
func (w *InMemoryOnboardingWorkflow) VerifyKYC(ctx context.Context, requestID string) (*KYCVerificationResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, exists := w.requests[requestID]
	if !exists {
		return nil, fmt.Errorf("onboarding request not found: %s", requestID)
	}

	// Simulate KYC verification
	result := &KYCVerificationResult{
		Status:                  "verified",
		GSTVerified:             true,
		PANVerified:             true,
		BusinessAddressConfirmed: true,
		OwnerIdentityVerified:   true,
		VerificationDate:        time.Now(),
		VerifierID:              "kyc_agent",
	}

	// Check if any documents are missing critical fields
	for _, doc := range req.Documents {
		if doc.ExtractionResult == nil || len(doc.ExtractionResult) == 0 {
			result.Status = "pending"
			result.Issues = append(result.Issues, fmt.Sprintf("document %s lacks extraction results", doc.Type))
		}
	}

	req.KYCResult = result
	req.State = OnboardingKYCVerified
	req.UpdatedAt = time.Now()
	req.AuditTrail = append(req.AuditTrail, AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    "verify_kyc",
		Status:    "success",
		ActorID:   "kyc_agent",
		Timestamp: time.Now(),
		Details: map[string]string{
			"gst_verified": fmt.Sprintf("%v", result.GSTVerified),
			"pan_verified": fmt.Sprintf("%v", result.PANVerified),
		},
	})

	return result, nil
}

// CheckCompliance runs AML and sanctions screening.
func (w *InMemoryOnboardingWorkflow) CheckCompliance(ctx context.Context, requestID string) (*ComplianceCheckResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, exists := w.requests[requestID]
	if !exists {
		return nil, fmt.Errorf("onboarding request not found: %s", requestID)
	}

	// Simulate compliance check
	result := &ComplianceCheckResult{
		Status:         "clear",
		AMLResult:      true,
		SanctionsCheck: true,
		RiskLevel:      "low",
		CheckDate:      time.Now(),
		CheckerID:      "aml_agent",
	}

	// Check if merchant is in sanctions list (for testing)
	if w.sanctionedMerchants[req.MerchantID] {
		result.Status = "rejected"
		result.SanctionsCheck = false
		result.RiskLevel = "high"
		result.Flags = append(result.Flags, ComplianceFlag{
			Category:           "sanctions",
			Severity:           "high",
			Description:        "Merchant found on sanctions watchlist",
			ResolutionRequired: true,
		})
	}

	req.ComplianceCheckResult = result
	req.State = OnboardingComplianceClear
	req.UpdatedAt = time.Now()
	req.AuditTrail = append(req.AuditTrail, AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    "check_compliance",
		Status:    "success",
		ActorID:   "aml_agent",
		Timestamp: time.Now(),
		Details: map[string]string{
			"risk_level": result.RiskLevel,
			"status":     result.Status,
		},
	})

	return result, nil
}

// EvaluateApproval applies policy rules to determine auto-approve/review/reject.
func (w *InMemoryOnboardingWorkflow) EvaluateApproval(ctx context.Context, requestID string) (*ApprovalStatus, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, exists := w.requests[requestID]
	if !exists {
		return nil, fmt.Errorf("onboarding request not found: %s", requestID)
	}

	// If compliance check shows sanctions, auto-reject
	if req.ComplianceCheckResult != nil && req.ComplianceCheckResult.Status == "rejected" {
		status := &ApprovalStatus{
			Decision:    "auto_reject",
			Reason:      "Merchant found on sanctions watchlist",
			RiskScore:   1.0,
			EvaluatedAt: time.Now(),
			EvaluatorID: "policy_engine",
		}
		req.ApprovalStatus = status
		req.State = OnboardingRejected
		return status, nil
	}

	// If KYC and compliance are both clean, auto-approve low-risk merchants
	if req.KYCResult != nil && req.KYCResult.Status == "verified" &&
		req.ComplianceCheckResult != nil && req.ComplianceCheckResult.Status == "clear" &&
		req.ComplianceCheckResult.RiskLevel == "low" {
		status := &ApprovalStatus{
			Decision:    "auto_approve",
			Reason:      "Low-risk merchant with clean KYC and compliance",
			RiskScore:   0.1,
			EvaluatedAt: time.Now(),
			EvaluatorID: "policy_engine",
		}
		req.ApprovalStatus = status
		req.State = OnboardingApprovalPending
		return status, nil
	}

	// Medium-risk merchants go to manual review
	status := &ApprovalStatus{
		Decision:    "manual_review",
		Reason:      "Merchant requires manual verification",
		RiskScore:   0.5,
		EvaluatedAt: time.Now(),
		EvaluatorID: "policy_engine",
	}
	req.ApprovalStatus = status
	req.State = OnboardingManualReview
	return status, nil
}

// ApproveOnboarding finalizes approval and transitions merchant to approved.
func (w *InMemoryOnboardingWorkflow) ApproveOnboarding(ctx context.Context, requestID string) (bool, string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, exists := w.requests[requestID]
	if !exists {
		return false, "", fmt.Errorf("onboarding request not found: %s", requestID)
	}

	// Prevent approval if policy says auto-reject
	if req.ApprovalStatus != nil && req.ApprovalStatus.Decision == "auto_reject" {
		return false, "Policy requires rejection", nil
	}

	// Transition merchant to approved
	if err := w.merchantManager.UpdateStatus(ctx, req.MerchantID, StatusApproved, ""); err != nil {
		return false, err.Error(), nil
	}

	// Assign default limits if not already set
	if err := w.merchantManager.UpdateLimits(ctx, req.MerchantID, DefaultMerchantLimits()); err != nil {
		return false, err.Error(), nil
	}

	// Record approval
	now := time.Now()
	req.ApprovalDecision = &ApprovalDecision{
		Decision:  "approve",
		Reason:    "Auto-approved by policy engine",
		DecidedBy: "policy_engine",
		DecidedAt: now,
	}
	req.State = OnboardingApproved
	req.UpdatedAt = now
	req.AuditTrail = append(req.AuditTrail, AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    "approve_onboarding",
		Status:    "success",
		ActorID:   "policy_engine",
		Timestamp: now,
	})

	return true, "Merchant approved successfully", nil
}

// RejectOnboarding rejects the onboarding request.
func (w *InMemoryOnboardingWorkflow) RejectOnboarding(ctx context.Context, requestID string, reason string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, exists := w.requests[requestID]
	if !exists {
		return fmt.Errorf("onboarding request not found: %s", requestID)
	}

	// Transition merchant to rejected
	if err := w.merchantManager.UpdateStatus(ctx, req.MerchantID, StatusRejected, reason); err != nil {
		return err
	}

	// Record rejection
	now := time.Now()
	req.ApprovalDecision = &ApprovalDecision{
		Decision:  "reject",
		Reason:    reason,
		DecidedBy: "policy_engine",
		DecidedAt: now,
	}
	req.State = OnboardingRejected
	req.UpdatedAt = now
	req.AuditTrail = append(req.AuditTrail, AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:    "reject_onboarding",
		Status:    "success",
		ActorID:   "policy_engine",
		Timestamp: now,
		Details: map[string]string{
			"reason": reason,
		},
	})

	return nil
}

// GetOnboardingRequest retrieves an onboarding request by ID.
func (w *InMemoryOnboardingWorkflow) GetOnboardingRequest(ctx context.Context, requestID string) (*MerchantOnboardingRequest, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	req, exists := w.requests[requestID]
	if !exists {
		return nil, fmt.Errorf("onboarding request not found: %s", requestID)
	}

	return req, nil
}

// SetSanctionedMerchant marks a merchant as sanctioned (for testing).
func (w *InMemoryOnboardingWorkflow) SetSanctionedMerchant(merchantID string, sanctioned bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if sanctioned {
		w.sanctionedMerchants[merchantID] = true
	} else {
		delete(w.sanctionedMerchants, merchantID)
	}
}
