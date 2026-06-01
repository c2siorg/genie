package kyc

import (
	"context"
	"time"

	"github.com/c2siorg/genie/pkg/compliance"
)

// OnboardingState defines the finite states of a KYC request.
type OnboardingState string

const (
	StatePending          OnboardingState = "pending"
	StateDocumentsUploaded OnboardingState = "documents_uploaded"
	StateIdentityVerified OnboardingState = "identity_verified"
	StateSanctionsChecked OnboardingState = "sanctions_checked"
	StateApprovalPending  OnboardingState = "approval_pending"
	StateApproved         OnboardingState = "approved"
	StateFlagged          OnboardingState = "flagged"
	StateRejected         OnboardingState = "rejected"
)

// OnboardingRequest is the core entity representing a customer onboarding workflow.
type OnboardingRequest struct {
	ID               string                       `json:"id"`
	UserID           string                       `json:"user_id"`
	RequestedAt      time.Time                    `json:"requested_at"`
	State            OnboardingState              `json:"state"`
	UpdatedAt        time.Time                    `json:"updated_at"`
	ApplicantInfo    ApplicantInfo                `json:"applicant_info"`
	Documents        []Document                   `json:"documents"`
	IdentityResult   *IdentificationResult        `json:"identity_result,omitempty"`
	SanctionsResult  *SanctionsResult             `json:"sanctions_result,omitempty"`
	ApprovalPolicy   ApprovalPolicyResult         `json:"approval_policy"`
	ApprovalDecision *ApprovalDecision            `json:"approval_decision,omitempty"`
	AuditLog         []compliance.AuditEntry      `json:"audit_log"`
	LineageRecords   []string                     `json:"lineage_records"`
}

// RiskScore computes a synthetic risk score for the onboarding request.
// Returns 0.0 (lowest risk) to 1.0 (highest risk).
func (or *OnboardingRequest) RiskScore() float64 {
	// TODO: implement
	panic("not implemented")
}

// ApplicantInfo holds personal and demographic information about the applicant.
type ApplicantInfo struct {
	FullName       string    `json:"full_name"`
	DateOfBirth    time.Time `json:"date_of_birth"`
	Address        string    `json:"address"`
	Jurisdiction   string    `json:"jurisdiction"`   // ISO 3166-1 alpha-2
	Email          string    `json:"email"`
	PhoneNumber    string    `json:"phone_number"`
	OccupationCode string    `json:"occupation_code"`
}

// Document represents an uploaded identity or financial document.
type Document struct {
	ID                  string         `json:"id"`
	OnboardingRequestID string         `json:"request_id"`
	Type                string         `json:"type"` // "passport" | "driving_license" | "bank_statement"
	UploadedAt          time.Time      `json:"uploaded_at"`
	StorageReference    string         `json:"storage_ref"`
	OCRExtractedText    string         `json:"ocr_text"`
	OCRConfidence       float64        `json:"ocr_confidence_0_1"`
	FormattedFields     map[string]any `json:"formatted_fields"`
	VerificationResults map[string]any `json:"verification_results"`
	Hash                string         `json:"hash"` // SHA256 for tamper detection
}

// IdentificationResult holds the output of identity verification.
type IdentificationResult struct {
	RequestID          string
	FieldMatches       FieldMatchDetails
	OverallConfidence  float64
	VerificationMethod string
	VerifiedAt         time.Time
	VerifierID         string
	Issues             []string
}

// FieldMatchDetails breaks down match results per field.
type FieldMatchDetails struct {
	Name     MatchResult
	DOB      MatchResult
	Address  MatchResult
	IDNumber MatchResult
}

// MatchResult represents the result of matching a single field.
type MatchResult struct {
	IsMatch    bool
	Confidence float64
	Extracted  string
	Expected   string
	Note       string
}

// SanctionsResult holds the output of sanctions screening.
type SanctionsResult struct {
	RequestID         string
	IsSanctioned      bool
	Hits              []SanctionsHit
	WatchlistsChecked []string
	RiskLevel         string // "green" | "yellow" | "red"
	LastCheckedAt     time.Time
	CheckerID         string
	Note              string
}

// SanctionsHit represents a match against a watchlist.
type SanctionsHit struct {
	Watchlist     string
	MatchedName   string
	Confidence    float64
	RecordID      string
	DateOfListing time.Time
	Type          string // "individual" | "entity"
}

// ApprovalPolicyResult holds the output of approval policy evaluation.
type ApprovalPolicyResult struct {
	Decision   string // "auto_approve" | "manual_review" | "auto_flag"
	RiskScore  float64
	Rationale  string
	EvaluatedAt time.Time
}

// ApprovalDecision represents a human analyst's decision on a flagged case.
type ApprovalDecision struct {
	OnboardingRequestID string
	Decision            string    // "approve" | "hold" | "reject" | "request_docs"
	Reason              string
	DecidedBy           string
	DecidedAt           time.Time
	RequestedDocs       []string
	Notes               string
}

// OnboardingStore defines the persistence interface for onboarding requests.
type OnboardingStore interface {
	Create(ctx context.Context, request *OnboardingRequest) error
	Get(ctx context.Context, id string) (*OnboardingRequest, error)
	Update(ctx context.Context, request *OnboardingRequest) error
	ListByUserID(ctx context.Context, userID string) ([]*OnboardingRequest, error)
	Archive(ctx context.Context, id string) error
}

// InMemoryOnboardingStore provides in-memory storage for testing.
type InMemoryOnboardingStore struct {
	// TODO: implement
}

// DBOnboardingStore provides persistent database storage.
type DBOnboardingStore struct {
	// TODO: implement
}
