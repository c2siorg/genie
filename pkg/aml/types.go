// Package aml provides anti-money laundering (AML) risk scoring, policy evaluation,
// and beneficiary risk checking for transaction monitoring and regulatory compliance.
//
// The scorer implements 5 weighted rules:
// - Threshold breach (30%): amount > limit
// - Velocity (25%): transaction count/hour > threshold
// - Jurisdiction (25%): beneficiary in high-risk country
// - Beneficiary risk (15%): PEP, adverse media, new (first_seen < 7d)
// - Anomaly detection (5%): pattern deviation from profile baseline
//
// Policies are evaluated via OPA (Open Policy Agent) Rego files, allowing
// compliance teams to adjust thresholds without code deployment.
package aml

import "time"

// RiskLevel represents the risk decision level.
type RiskLevel string

const (
	RiskLevelApprove  RiskLevel = "approve"  // 0-30
	RiskLevelMonitor  RiskLevel = "monitor"  // 31-50
	RiskLevelReview   RiskLevel = "review"   // 51-70
	RiskLevelEscalate RiskLevel = "escalate" // 71-100
	RiskLevelBlock    RiskLevel = "block"    // > 100 or explicit block
)

// RiskScore represents the complete risk assessment for a transaction.
type RiskScore struct {
	// Score is the numerical risk value 0-100 (can exceed 100 for escalation).
	Score float64 `json:"score"`

	// Level is the risk decision derived from the score and policy.
	Level RiskLevel `json:"level"`

	// TriggeredRules lists the rule names that contributed to the score.
	TriggeredRules []string `json:"triggered_rules"`

	// Evidence maps rule names to their explanations.
	Evidence map[string]string `json:"evidence"`

	// PolicyOverride indicates if this score was overridden by policy.
	PolicyOverride bool `json:"policy_override"`

	// ScoredAt is the timestamp when the score was computed.
	ScoredAt time.Time `json:"scored_at"`
}

// RiskProfile describes a user's AML risk context.
type RiskProfile struct {
	// UserID is the customer's unique identifier.
	UserID string `json:"user_id"`

	// RiskLevel is the user's baseline risk classification.
	RiskLevel RiskLevel `json:"risk_level"`

	// DailyLimit is the approved transaction amount per day (in cents/minor units).
	DailyLimit int64 `json:"daily_limit"`

	// VelocityLimit is max transactions per hour before raising velocity flag.
	VelocityLimit int `json:"velocity_limit"`

	// FirstSeen is when the account was created; used for newness flags.
	FirstSeen time.Time `json:"first_seen"`

	// AnomalyBaseline is a hash or fingerprint of the user's normal transaction pattern.
	// Production systems would store behavioral profiles; for now it's a placeholder.
	AnomalyBaseline string `json:"anomaly_baseline"`

	// PolicyOverrides is a map of rule names to override values (0.0 = ignore, >0 = force weight).
	PolicyOverrides map[string]float64 `json:"policy_overrides"`
}

// AMLConfig holds thresholds and rule weights.
type AMLConfig struct {
	// Risk score thresholds
	ThresholdApprove  float64 `json:"threshold_approve"`  // 0-30
	ThresholdMonitor  float64 `json:"threshold_monitor"`  // 31-50
	ThresholdReview   float64 `json:"threshold_review"`   // 51-70
	ThresholdEscalate float64 `json:"threshold_escalate"` // 71-100

	// Rule weights (must sum to 1.0)
	WeightThreshold    float64 `json:"weight_threshold"`    // 0.30
	WeightVelocity     float64 `json:"weight_velocity"`     // 0.25
	WeightJurisdiction float64 `json:"weight_jurisdiction"` // 0.25
	WeightBeneficiary  float64 `json:"weight_beneficiary"`  // 0.15
	WeightAnomaly      float64 `json:"weight_anomaly"`      // 0.05

	// Velocity config
	MaxTransactionsPerHour int `json:"max_transactions_per_hour"`

	// Beneficiary risk flags
	CheckPEP            bool `json:"check_pep"`           // Check Politically Exposed Person lists
	CheckSanctions      bool `json:"check_sanctions"`     // Check OFAC/UN sanctions
	CheckAdverseMedia   bool `json:"check_adverse_media"` // Check adverse media
	NewAccountDaysLimit int  `json:"new_account_days"`    // Days to flag new accounts (default 7)
}

// Transaction represents a financial transaction for AML scoring.
type Transaction struct {
	// ID is the unique transaction identifier.
	ID string `json:"id"`

	// UserID is the originator of the transaction.
	UserID string `json:"user_id"`

	// Amount is the transaction amount (in cents/minor units).
	Amount int64 `json:"amount"`

	// Beneficiary info
	BeneficiaryID      string `json:"beneficiary_id"`
	BeneficiaryName    string `json:"beneficiary_name"`
	BeneficiaryCountry string `json:"beneficiary_country"` // ISO-2 code

	// Timestamp
	Timestamp time.Time `json:"timestamp"`

	// Metadata
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

// BeneficiaryRiskInfo holds risk indicators for a beneficiary.
type BeneficiaryRiskInfo struct {
	// Flags
	IsPEP           bool `json:"is_pep"`            // Politically Exposed Person
	IsSanctioned    bool `json:"is_sanctioned"`     // OFAC/UN list
	HasAdverseMedia bool `json:"has_adverse_media"` // Adverse media flag

	// Risk score component (0-100)
	RiskScore float64 `json:"risk_score"`

	// Rationale for the score
	Rationale string `json:"rationale"`

	// Last checked
	CheckedAt time.Time `json:"checked_at"`
}

// HighRiskCountries is a list of jurisdictions flagged for enhanced monitoring.
// This is a demonstration list; production systems would fetch from regulatory sources.
var HighRiskCountries = map[string]bool{
	"KP": true, // North Korea
	"IR": true, // Iran
	"SY": true, // Syria
	"CU": true, // Cuba
	"MM": true, // Myanmar
}

// DefaultAMLConfig returns a production-ready configuration.
func DefaultAMLConfig() *AMLConfig {
	return &AMLConfig{
		ThresholdApprove:       30.0,
		ThresholdMonitor:       50.0,
		ThresholdReview:        70.0,
		ThresholdEscalate:      100.0,
		WeightThreshold:        0.30,
		WeightVelocity:         0.25,
		WeightJurisdiction:     0.25,
		WeightBeneficiary:      0.15,
		WeightAnomaly:          0.05,
		MaxTransactionsPerHour: 10,
		CheckPEP:               true,
		CheckSanctions:         true,
		CheckAdverseMedia:      true,
		NewAccountDaysLimit:    7,
	}
}
