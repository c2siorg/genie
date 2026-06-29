// Package contractv1 holds the canonical, versioned domain types shared across
// the advisor pipeline (ProfileAnalyzer → FinancialAnalyst → RecommendationGenerator).
//
// Why this package exists: the advisor domain types were defined twice — in
// agents/core/advisor_types.go and pkg/advisor — and had already diverged
// (SpendingOpportunity.PrimaryDriver and .AnalyzedAt existed only in the
// pkg/advisor copy; both copies carried BenchmarkPercentile). Two definitions of
// the "same" type silently drift and force lossy conversions at every boundary.
// This package is the single source of truth; other packages alias to it
// (e.g. agents/core uses `type UserProfile = contractv1.UserProfile`), so all
// callers share one identical type with no conversion.
//
// Versioned path (contract/v1) lets the wire/domain contract evolve without
// breaking existing consumers: a future breaking change becomes contract/v2.
//
// License: MIT
package contractv1

import (
	"fmt"
	"time"
)

// ========== ACTION TYPES ==========

// Action represents a recommended action (reduce spending, increase savings, invest, etc.).
type Action struct {
	ID                   string                 `json:"id"`
	Type                 ActionType             `json:"type"`
	Description          string                 `json:"description"`
	EstimatedImpactPaise int64                  `json:"estimated_impact_paise"` // ₹ in paise
	Difficulty           Difficulty             `json:"difficulty"`             // easy, medium, hard
	TimelineDays         int                    `json:"timeline_days"`          // days to implement
	Details              map[string]interface{} `json:"details,omitempty"`
}

// ActionType defines the type of action recommended.
type ActionType string

const (
	ActionReduceCategory  ActionType = "reduce_category"
	ActionIncreaseSavings ActionType = "increase_savings"
	ActionMoveFunds       ActionType = "move_funds"
	ActionInvest          ActionType = "invest"
	ActionRefinance       ActionType = "refinance"
	ActionEnrollProgram   ActionType = "enroll_program"
)

// Difficulty represents how hard an action is to implement.
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

// ========== RISK TYPES ==========

// Risk represents potential risks of a recommendation.
type Risk struct {
	ID          string       `json:"id"`
	Category    RiskCategory `json:"category"`
	Description string       `json:"description"`
	Severity    Severity     `json:"severity"` // low, medium, high
	Mitigation  string       `json:"mitigation"`
}

// RiskCategory defines a risk domain.
type RiskCategory string

const (
	RiskMarket       RiskCategory = "market"
	RiskLiquidity    RiskCategory = "liquidity"
	RiskCounterparty RiskCategory = "counterparty"
	RiskRegulatory   RiskCategory = "regulatory"
)

// Severity represents risk severity.
type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// ========== RECOMMENDATION TYPES ==========

// Recommendation represents a personalized financial recommendation.
type Recommendation struct {
	RecommendationID      string                 `json:"recommendation_id"`
	UserID                string                 `json:"user_id"`
	Category              RecommendationCategory `json:"category"`
	Title                 string                 `json:"title"`
	Description           string                 `json:"description"`
	Rationale             string                 `json:"rationale"` // Why this recommendation
	Actions               []Action               `json:"actions"`
	EstimatedImpact       ImpactEstimate         `json:"estimated_impact"`
	Risks                 []Risk                 `json:"risks"`
	ComplianceConstraints []string               `json:"compliance_constraints"` // Rules that apply
	Status                RecommendationStatus   `json:"status"`                 // pending, accepted, rejected, etc.
	TraceID               string                 `json:"trace_id"`               // For audit trail
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// Validate ensures a Recommendation is well-formed before it is returned to a
// caller or persisted. Mirrors RecommendationGenerator.ValidateRecommendation.
func (r *Recommendation) Validate() error {
	if r == nil {
		return fmt.Errorf("recommendation required")
	}
	if r.RecommendationID == "" {
		return fmt.Errorf("recommendation_id required")
	}
	if r.UserID == "" {
		return fmt.Errorf("user_id required")
	}
	if r.Title == "" {
		return fmt.Errorf("title required")
	}
	if r.EstimatedImpact.Confidence < 0 || r.EstimatedImpact.Confidence > 1 {
		return fmt.Errorf("confidence must be 0.0-1.0")
	}
	return nil
}

// RecommendationCategory defines the recommendation type.
type RecommendationCategory string

const (
	CategorySpendingOptimization RecommendationCategory = "spending_optimization"
	CategorySavings              RecommendationCategory = "savings"
	CategoryInvestment           RecommendationCategory = "investment"
	CategoryTaxEfficiency        RecommendationCategory = "tax_efficiency"
	CategoryRiskManagement       RecommendationCategory = "risk_management"
)

// RecommendationStatus defines the recommendation lifecycle.
type RecommendationStatus string

const (
	StatusPending     RecommendationStatus = "pending"
	StatusAccepted    RecommendationStatus = "accepted"
	StatusRejected    RecommendationStatus = "rejected"
	StatusDeferred    RecommendationStatus = "deferred"
	StatusImplemented RecommendationStatus = "implemented"
)

// ImpactEstimate represents estimated financial impact.
type ImpactEstimate struct {
	MonthlySavingsPaise int64   `json:"monthly_savings_paise"`
	AnnualReturnPaise   int64   `json:"annual_return_paise"`
	Confidence          float64 `json:"confidence"` // 0.0-1.0
}

// ========== USER PROFILE TYPES ==========

// UserProfile represents a user's financial profile for recommendations.
type UserProfile struct {
	UserID                 string               `json:"user_id"`
	RiskTolerance          RiskTolerance        `json:"risk_tolerance"`
	AnnualIncomePaise      int64                `json:"annual_income_paise"`
	SavingsGoalPaise       int64                `json:"savings_goal_paise"`
	InvestmentExperience   InvestmentExperience `json:"investment_experience"`
	ComplianceRestrictions []string             `json:"compliance_restrictions"` // e.g., ["cannot_invest_offshore"]
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}

// RiskTolerance represents a user's risk appetite.
type RiskTolerance string

const (
	RiskConservative RiskTolerance = "conservative"
	RiskModerate     RiskTolerance = "moderate"
	RiskAggressive   RiskTolerance = "aggressive"
)

// InvestmentExperience represents a user's investment knowledge.
type InvestmentExperience string

const (
	ExperienceBeginner     InvestmentExperience = "beginner"
	ExperienceIntermediate InvestmentExperience = "intermediate"
	ExperienceExpert       InvestmentExperience = "expert"
)

// SpendingOpportunity represents a potential area for financial improvement.
//
// This is the MERGED canonical definition: it carries every field from both
// prior copies — PrimaryDriver + AnalyzedAt (previously only in pkg/advisor) and
// BenchmarkPercentile (present in both). PrimaryDriver is omitempty because the
// agents/core copy never set it.
type SpendingOpportunity struct {
	ID                      string    `json:"id"`
	Category                string    `json:"category"` // spending category where opportunity exists
	CurrentMonthlyPaise     int64     `json:"current_monthly_paise"`
	OptimizedMonthlyPaise   int64     `json:"optimized_monthly_paise"`
	MonthlySavingsPotential int64     `json:"monthly_savings_potential"` // current - optimized
	Confidence              float64   `json:"confidence"`                // 0.0-1.0
	PrimaryDriver           string    `json:"primary_driver,omitempty"`  // what's causing overspend?
	BenchmarkPercentile     int       `json:"benchmark_percentile"`      // user vs peers (0-100)
	Trend                   string    `json:"trend"`                     // "increasing", "stable", "decreasing"
	AnalyzedAt              time.Time `json:"analyzed_at"`
}

// ========== CALIBRATION / FEEDBACK TYPES ==========

// CalibrationMetrics represents recommendation accuracy metrics.
type CalibrationMetrics struct {
	UserID              string    `json:"user_id"`
	RecommendationCount int       `json:"recommendations_count"`
	AcceptanceRate      float64   `json:"acceptance_rate"`     // % accepted / total
	AvgImpactRealized   float64   `json:"avg_impact_realized"` // actual / estimated ratio
	AccuracyScore       int       `json:"accuracy_score"`      // 0-100
	LastCalibratedAt    time.Time `json:"last_calibrated_at"`
}

// RecommendationFeedback represents user feedback on a recommendation.
type RecommendationFeedback struct {
	RecommendationID string         `json:"recommendation_id"`
	Action           FeedbackAction `json:"action"`                   // accept, reject, defer
	Reason           string         `json:"reason,omitempty"`         // Why rejected/deferred
	DeferredUntil    *time.Time     `json:"deferred_until,omitempty"` // When to reconsider
	FeedbackAt       time.Time      `json:"feedback_at"`
}

// FeedbackAction represents a user's response to a recommendation.
type FeedbackAction string

const (
	ActionAccept FeedbackAction = "accept"
	ActionReject FeedbackAction = "reject"
	ActionDefer  FeedbackAction = "defer"
)

// ========== PIPELINE INPUT TYPES (with validation) ==========
//
// These are the typed inputs each advisor agent decodes from its request
// payload. Validation lives on the INPUT types so an agent can validate without
// constructing any service object.

// ProfileAnalyzerInput is the input to the ProfileAnalyzer agent.
type ProfileAnalyzerInput struct {
	UserID            string `json:"user_id"`
	KYCStatus         string `json:"kyc_status,omitempty"` // verified_high_value, verified_standard, pending
	AnnualIncomePaise int64  `json:"annual_income_paise,omitempty"`
}

// Validate checks required fields for ProfileAnalyzerInput.
func (in *ProfileAnalyzerInput) Validate() error {
	if in == nil {
		return fmt.Errorf("input required")
	}
	if in.UserID == "" {
		return fmt.Errorf("user_id required")
	}
	if in.AnnualIncomePaise < 0 {
		return fmt.Errorf("annual_income_paise must be >= 0")
	}
	return nil
}

// FinancialAnalystInput is the input to the FinancialAnalyst agent: a user
// profile to analyze for spending opportunities.
type FinancialAnalystInput struct {
	Profile UserProfile `json:"profile"`
}

// Validate checks required fields for FinancialAnalystInput.
func (in *FinancialAnalystInput) Validate() error {
	if in == nil {
		return fmt.Errorf("input required")
	}
	if in.Profile.UserID == "" {
		return fmt.Errorf("profile.user_id required")
	}
	return nil
}

// RecommendationGeneratorInput is the input to the RecommendationGenerator
// agent: a profile plus the opportunities identified by the FinancialAnalyst.
type RecommendationGeneratorInput struct {
	Profile       UserProfile           `json:"profile"`
	Opportunities []SpendingOpportunity `json:"opportunities"`
}

// Validate checks required fields for RecommendationGeneratorInput.
func (in *RecommendationGeneratorInput) Validate() error {
	if in == nil {
		return fmt.Errorf("input required")
	}
	if in.Profile.UserID == "" {
		return fmt.Errorf("profile.user_id required")
	}
	if len(in.Opportunities) == 0 {
		return fmt.Errorf("at least one opportunity required")
	}
	return nil
}
