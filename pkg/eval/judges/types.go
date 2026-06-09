package judges

import (
	"time"
)

// JudgeConfig controls LLM judge backend settings.
type JudgeConfig struct {
	// Provider: "anthropic" | "openai" | "ollama" (default)
	Provider string
	// BaseURL for the LLM provider (optional, uses provider default if empty)
	BaseURL string
	// Model name (e.g., "claude-opus-4", "gpt-4", "llama3.1")
	Model string
	// APIKey for authentication (required for anthropic, openai)
	APIKey string
	// Temperature for model (0.0-1.0, default 0.3)
	Temperature float64
	// MaxTokens for response (default 2000)
	MaxTokens int
	// TimeoutSeconds for LLM call (default 60)
	TimeoutSeconds int
}

// Verdict is the judge's decision on the subject.
type Verdict struct {
	// Pass is true if the subject passes evaluation, false otherwise
	Pass bool
	// Score is the confidence score (0.0-1.0) of the verdict
	Score float64
	// RubricID which rubric was evaluated
	RubricID string
	// RubricName human-readable rubric name
	RubricName string
	// Reason brief explanation of the verdict
	Reason string
	// Evidence supporting the verdict (JSON)
	Evidence string
	// EvaluatedAt timestamp when evaluation occurred
	EvaluatedAt time.Time
}

// CalibrationResult contains metrics from judge calibration.
type CalibrationResult struct {
	// NumPositives is the number of positive ground-truth examples
	NumPositives int
	// NumNegatives is the number of negative ground-truth examples
	NumNegatives int
	// TPR is true positive rate (recall)
	TPR float64
	// TNR is true negative rate (specificity)
	TNR float64
	// TPRLowerBound is the lower bound of 95% confidence interval for TPR
	TPRLowerBound float64
	// TPRUpperBound is the upper bound of 95% confidence interval for TPR
	TPRUpperBound float64
	// TNRLowerBound is the lower bound of 95% confidence interval for TNR
	TNRLowerBound float64
	// TNRUpperBound is the upper bound of 95% confidence interval for TNR
	TNRUpperBound float64
	// OptimalThreshold is the score threshold that maximizes sensitivity + specificity
	OptimalThreshold float64
	// Samples contains individual calibration sample results
	Samples []CalibrationSample
	// CalibratedAt timestamp when calibration completed
	CalibratedAt time.Time
}

// CalibrationSample is one data point from calibration.
type CalibrationSample struct {
	// ID uniquely identifies this sample
	ID string
	// GroundTruth is the expected verdict (true = pass, false = fail)
	GroundTruth bool
	// Verdict is the judge's actual verdict
	Verdict bool
	// Score is the judge's confidence score (0.0-1.0)
	Score float64
	// Reason is the judge's reasoning
	Reason string
}

// SettlementJudgeInput is the input to settlement judge evaluation.
type SettlementJudgeInput struct {
	// OrderID uniquely identifies the order
	OrderID string
	// SettlementID uniquely identifies the settlement
	SettlementID string
	// SettlementAmount in paise
	SettlementAmount int64
	// ExpectedAmount in paise (for correctness check)
	ExpectedAmount int64
	// OrderAmount in paise (original order total)
	OrderAmount int64
	// NettingApplied true if netting was applied
	NettingApplied bool
	// NettedAmount in paise after netting (if applicable)
	NettedAmount int64
	// CBDCCommitted true if ledger commit succeeded
	CBDCCommitted bool
	// ReconciliationPassed true if reconciliation check passed
	ReconciliationPassed bool
	// AuditTrail contains relevant audit entries
	AuditTrail string // JSON serialized
	// Metadata additional context
	Metadata map[string]string
}

// ComplianceJudgeInput is the input to compliance judge evaluation.
type ComplianceJudgeInput struct {
	// CustomerID uniquely identifies the customer
	CustomerID string
	// OrderID uniquely identifies the order
	OrderID string
	// KYCStatus of the customer ("verified", "pending", "failed")
	KYCStatus string
	// KYCExpiryDate when KYC expires
	KYCExpiryDate time.Time
	// VelocityWindowSeconds the lookback window for velocity checks
	VelocityWindowSeconds int
	// VelocityThresholdPaise max amount in window (in paise)
	VelocityThresholdPaise int64
	// CurrentVelocityPaise current sum of transactions in window
	CurrentVelocityPaise int64
	// SanctionsListAge how old is the sanctions list (in seconds)
	SanctionsListAge int
	// MatchingThreshold for sanctions name matching (0.0-1.0)
	MatchingThreshold float64
	// AMLRiskScore current customer risk score (0.0-100.0)
	AMLRiskScore float64
	// VelocityExceeded true if current velocity exceeds threshold
	VelocityExceeded bool
	// ConsentProvided true if consent obtained for data access
	ConsentProvided bool
	// AuditTrail audit entries (JSON)
	AuditTrail string
	// Metadata additional context
	Metadata map[string]string
}

// OrchestrationJudgeInput is the input to orchestration judge evaluation.
type OrchestrationJudgeInput struct {
	// OrderID uniquely identifies the order
	OrderID string
	// StepSequence is the ordered list of workflow steps executed
	StepSequence []string
	// CurrentStep the current step being evaluated
	CurrentStep string
	// PreviousStep the previous step (for transition validation)
	PreviousStep string
	// TransitionValid true if the transition is allowed
	TransitionValid bool
	// IdempotencyKey used for retry deduplication (if applicable)
	IdempotencyKey string
	// PaymentConfirmed true if payment step completed successfully
	PaymentConfirmed bool
	// SettlementInitiated true if settlement was initiated after payment
	SettlementInitiated bool
	// WorkflowAuditTrail audit entries (JSON)
	WorkflowAuditTrail string
	// Metadata additional context
	Metadata map[string]string
}

// LineageJudgeInput is the input to lineage judge evaluation.
type LineageJudgeInput struct {
	// OrderID uniquely identifies the order
	OrderID string
	// LineageEntries the sequence of audit/lineage entries
	LineageEntries []LineageEntry
	// HashChainValid true if hash chain verification passed
	HashChainValid bool
	// TimestampsMonotonic true if timestamps are strictly increasing
	TimestampsMonotonic bool
	// AllRequiredFieldsPresent true if all required audit fields present
	AllRequiredFieldsPresent bool
	// DecisionTraceability the lineage provides full decision context
	DecisionTraceability string // JSON
	// Metadata additional context
	Metadata map[string]string
}

// LineageEntry represents one audit log entry.
type LineageEntry struct {
	// EntryID uniquely identifies the entry
	EntryID string
	// OrderID references the parent order
	OrderID string
	// Step the workflow step (e.g., "payment_confirmed")
	Step string
	// Timestamp when the step occurred
	Timestamp time.Time
	// PreviousHash the SHA256 hash of the previous entry (for chain validation)
	PreviousHash string
	// Hash the SHA256 hash of this entry
	Hash string
	// Data the entry data (JSON)
	Data string
}

// Judge is the common interface for all judge types.
type Judge interface {
	// Evaluate runs judgment on the input and returns a verdict.
	Evaluate(input interface{}) (Verdict, error)
	// Calibrate runs calibration on ground truth samples.
	Calibrate(samples []interface{}, groundTruth []bool) (CalibrationResult, error)
	// Name returns the judge's human-readable name
	Name() string
}
