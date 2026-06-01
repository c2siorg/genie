// Package merchant provides merchant profile management, KYC onboarding,
// and transaction limits enforcement for the Genie e-Rupee commerce platform.
//
// A merchant goes through these states during onboarding:
// pending → documents_uploaded → kyc_verified → compliance_cleared → approved
//
// Key entities:
// - MerchantProfile: Identity, business type, settlement account, limits, status
// - MerchantOnboardingRequest: Documents, KYC results, compliance checks, approval
// - MerchantLimits: Daily P2P, settlement, single transaction, monthly caps
//
// Integration points:
// - KYC agent for identity verification
// - AML agent for sanctions screening and compliance
// - Payment agent for settlement account assignment
package merchant

import (
	"time"
)

// MerchantStatus represents the current status of a merchant account.
type MerchantStatus string

const (
	StatusPending   MerchantStatus = "pending"    // Onboarding in progress
	StatusApproved  MerchantStatus = "approved"   // Fully approved, active
	StatusRejected  MerchantStatus = "rejected"   // Onboarding rejected
	StatusSuspended MerchantStatus = "suspended"  // Active account suspended
)

// BusinessType represents the legal structure of the merchant business.
type BusinessType string

const (
	BusinessTypeSole BusinessType = "sole"  // Sole proprietor
	BusinessTypeLLP  BusinessType = "llp"   // Limited Liability Partnership
	BusinessTypePvt  BusinessType = "pvt"   // Private Limited Company
	BusinessTypeGST  BusinessType = "gst"   // GST-registered business
)

// OnboardingState represents the current phase of KYC onboarding.
type OnboardingState string

const (
	OnboardingPending          OnboardingState = "pending"
	OnboardingDocumentsReady   OnboardingState = "documents_ready"
	OnboardingKYCSubmitted     OnboardingState = "kyc_submitted"
	OnboardingKYCVerified      OnboardingState = "kyc_verified"
	OnboardingComplianceCheck  OnboardingState = "compliance_check"
	OnboardingComplianceClear  OnboardingState = "compliance_clear"
	OnboardingApprovalPending  OnboardingState = "approval_pending"
	OnboardingApproved         OnboardingState = "approved"
	OnboardingRejected         OnboardingState = "rejected"
	OnboardingManualReview     OnboardingState = "manual_review"
)

// MerchantProfile represents a merchant entity in the e-Rupee commerce system.
type MerchantProfile struct {
	// ID uniquely identifies the merchant
	ID string `json:"id"`

	// BusinessName is the registered business name
	BusinessName string `json:"business_name"`

	// OwnerID references the merchant owner (end-user)
	OwnerID string `json:"owner_id"`

	// BusinessType is the legal structure (sole, llp, pvt, gst)
	BusinessType BusinessType `json:"business_type"`

	// GSTNumber is the GST registration number (if applicable)
	GSTNumber string `json:"gst_number,omitempty"`

	// PAN is the Permanent Account Number (if applicable)
	PAN string `json:"pan,omitempty"`

	// BusinessAddress is the registered business address
	BusinessAddress string `json:"business_address,omitempty"`

	// SettlementAccount is the assigned bank account for settlement
	SettlementAccount string `json:"settlement_account,omitempty"`

	// DailyLimitPaise is the daily transaction limit in paise (₹1 = 100 paise)
	DailyLimitPaise int64 `json:"daily_limit_paise"`

	// Status is the current account status (pending, approved, rejected, suspended)
	Status MerchantStatus `json:"status"`

	// CreatedAt records when the merchant was registered
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt records the most recent update
	UpdatedAt time.Time `json:"updated_at"`

	// SuspensionReason records why account was suspended (if applicable)
	SuspensionReason string `json:"suspension_reason,omitempty"`

	// Metadata holds additional context (category, volume, etc.)
	Metadata map[string]string `json:"metadata,omitempty"`
}

// MerchantOnboardingRequest represents the KYC and compliance workflow for a merchant.
type MerchantOnboardingRequest struct {
	// ID uniquely identifies this onboarding request
	ID string `json:"id"`

	// MerchantID references the parent merchant
	MerchantID string `json:"merchant_id"`

	// State is the current phase of onboarding
	State OnboardingState `json:"state"`

	// Documents submitted during onboarding
	Documents []OnboardingDocument `json:"documents"`

	// KYCResult from identity verification (GST, PAN, business registration)
	KYCResult *KYCVerificationResult `json:"kyc_result,omitempty"`

	// ComplianceCheckResult from AML/sanctions screening
	ComplianceCheckResult *ComplianceCheckResult `json:"compliance_check_result,omitempty"`

	// ApprovalStatus indicates auto-approve, manual review, or auto-reject
	ApprovalStatus *ApprovalStatus `json:"approval_status,omitempty"`

	// ApprovalDecision captures final HITL decision if needed
	ApprovalDecision *ApprovalDecision `json:"approval_decision,omitempty"`

	// SubmittedAt records when onboarding was initiated
	SubmittedAt time.Time `json:"submitted_at"`

	// UpdatedAt records the most recent update
	UpdatedAt time.Time `json:"updated_at"`

	// AuditTrail tracks all state transitions and actions
	AuditTrail []AuditEntry `json:"audit_trail"`
}

// OnboardingDocument represents a document submitted during onboarding.
type OnboardingDocument struct {
	// ID uniquely identifies the document
	ID string `json:"id"`

	// Type is the document category (gst_cert, pan, business_reg, bank_stmt)
	Type string `json:"type"`

	// URL is the location of the stored document
	URL string `json:"url"`

	// UploadedAt records when the document was submitted
	UploadedAt time.Time `json:"uploaded_at"`

	// ExtractionResult holds OCR/parsing output
	ExtractionResult map[string]string `json:"extraction_result,omitempty"`

	// VerificationStatus indicates if document was verified
	VerificationStatus string `json:"verification_status,omitempty"` // pending, verified, rejected
}

// KYCVerificationResult holds the outcome of KYC checks.
type KYCVerificationResult struct {
	// Status indicates if KYC passed (verified, rejected, pending)
	Status string `json:"status"`

	// GSTVerified indicates if GST number is valid and matches
	GSTVerified bool `json:"gst_verified"`

	// PANVerified indicates if PAN is valid and matches
	PANVerified bool `json:"pan_verified"`

	// BusinessAddressConfirmed indicates if address was verified
	BusinessAddressConfirmed bool `json:"business_address_confirmed"`

	// OwnerIdentityVerified indicates if owner identity matches documents
	OwnerIdentityVerified bool `json:"owner_identity_verified"`

	// VerificationDate records when KYC was completed
	VerificationDate time.Time `json:"verification_date"`

	// Issues lists any problems found (empty = clean)
	Issues []string `json:"issues,omitempty"`

	// VerifierID records which agent/system performed the check
	VerifierID string `json:"verifier_id,omitempty"`
}

// ComplianceCheckResult holds the outcome of AML and sanctions screening.
type ComplianceCheckResult struct {
	// Status indicates if compliance check passed (clear, flagged, rejected)
	Status string `json:"status"`

	// AMLResult is true if no AML issues detected
	AMLResult bool `json:"aml_result"`

	// SanctionsCheck is true if merchant is not on sanctions lists
	SanctionsCheck bool `json:"sanctions_check"`

	// RiskLevel categorizes the merchant risk (low, medium, high)
	RiskLevel string `json:"risk_level"`

	// Flags lists any compliance issues (PEP, adverse media, etc.)
	Flags []ComplianceFlag `json:"flags,omitempty"`

	// CheckDate records when compliance check was performed
	CheckDate time.Time `json:"check_date"`

	// CheckerID records which agent performed the check
	CheckerID string `json:"checker_id,omitempty"`
}

// ComplianceFlag represents a single compliance issue.
type ComplianceFlag struct {
	// Category is the type of flag (pep, adverse_media, new_account, etc.)
	Category string `json:"category"`

	// Severity is the urgency level (low, medium, high)
	Severity string `json:"severity"`

	// Description explains the flag
	Description string `json:"description"`

	// ResolutionRequired indicates if this must be manually reviewed
	ResolutionRequired bool `json:"resolution_required"`
}

// ApprovalStatus captures the result of automated approval policy evaluation.
type ApprovalStatus struct {
	// Decision is auto_approve, manual_review, or auto_reject
	Decision string `json:"decision"`

	// Reason explains the decision
	Reason string `json:"reason"`

	// RiskScore is the computed merchant risk (0.0 = low, 1.0 = high)
	RiskScore float64 `json:"risk_score"`

	// EvaluatedAt records when policy was evaluated
	EvaluatedAt time.Time `json:"evaluated_at"`

	// EvaluatorID records which agent performed evaluation
	EvaluatorID string `json:"evaluator_id,omitempty"`
}

// ApprovalDecision captures a human approval decision.
type ApprovalDecision struct {
	// Decision is approve, hold, or reject
	Decision string `json:"decision"`

	// Reason explains the decision
	Reason string `json:"reason"`

	// DecidedBy records which human (approver ID) made the decision
	DecidedBy string `json:"decided_by"`

	// DecidedAt records when the decision was made
	DecidedAt time.Time `json:"decided_at"`

	// RequestedDocs lists any documents to request (if decision is hold)
	RequestedDocs []string `json:"requested_docs,omitempty"`
}

// AuditEntry records a state transition or action in the onboarding workflow.
type AuditEntry struct {
	// ID uniquely identifies this audit entry
	ID string `json:"id"`

	// Action describes what happened (kyc_submitted, compliance_check, etc.)
	Action string `json:"action"`

	// Status captures the result (success, failed, pending)
	Status string `json:"status"`

	// Details holds additional context
	Details map[string]string `json:"details,omitempty"`

	// ActorID records who/what performed the action
	ActorID string `json:"actor_id,omitempty"`

	// Timestamp records when the action occurred
	Timestamp time.Time `json:"timestamp"`
}

// MerchantLimits defines transaction limits for a merchant.
type MerchantLimits struct {
	// DailyP2PPaise is the daily limit for peer-to-peer transactions
	DailyP2PPaise int64 `json:"daily_p2p_paise"`

	// DailySettlementPaise is the daily limit for settlement transactions
	DailySettlementPaise int64 `json:"daily_settlement_paise"`

	// SingleTxnPaise is the maximum for a single transaction
	SingleTxnPaise int64 `json:"single_txn_paise"`

	// MonthlyTotalPaise is the maximum for monthly aggregate
	MonthlyTotalPaise int64 `json:"monthly_total_paise"`

	// EffectiveFrom records when these limits became active
	EffectiveFrom time.Time `json:"effective_from"`

	// Notes captures any override reason
	Notes string `json:"notes,omitempty"`
}

// DefaultMerchantLimits returns the default limits for a new merchant.
// Low-risk sole proprietors get:
// - Daily P2P: ₹500,000 (50,000,000 paise)
// - Daily settlement: ₹500,000 (50,000,000 paise)
// - Single transaction: ₹100,000 (10,000,000 paise)
// - Monthly total: ₹5,000,000 (500,000,000 paise)
func DefaultMerchantLimits() MerchantLimits {
	return MerchantLimits{
		DailyP2PPaise:        50_000_000,   // ₹500k
		DailySettlementPaise: 50_000_000,   // ₹500k
		SingleTxnPaise:       10_000_000,   // ₹100k
		MonthlyTotalPaise:    500_000_000,  // ₹5M
		EffectiveFrom:        time.Now(),
	}
}

// RestrictedMerchantLimits returns limits for merchants requiring manual review.
// Moderate risk merchants get:
// - Daily P2P: ₹100,000 (10,000,000 paise)
// - Daily settlement: ₹100,000 (10,000,000 paise)
// - Single transaction: ₹25,000 (2,500,000 paise)
// - Monthly total: ₹500,000 (50,000,000 paise)
func RestrictedMerchantLimits() MerchantLimits {
	return MerchantLimits{
		DailyP2PPaise:        10_000_000,   // ₹100k
		DailySettlementPaise: 10_000_000,   // ₹100k
		SingleTxnPaise:       2_500_000,    // ₹25k
		MonthlyTotalPaise:    50_000_000,   // ₹500k
		EffectiveFrom:        time.Now(),
	}
}
