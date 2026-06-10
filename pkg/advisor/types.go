// Package advisor implements the Advisor workspace for personalized financial recommendations.
// Phase 7: Personalized recommendations based on user profile, spending patterns, compliance status.
// Real API integration with Commerce, Compliance, and Governance phases.
//
// Architecture: ProfileAnalyzer → FinancialAnalyst → RecommendationGenerator
//
// License: MIT
package advisor

import "time"

// Action represents a recommended action for the user (reduce spending, increase savings, invest, etc.)
type Action struct {
	ID                   string        `json:"id"`
	Type                 ActionType    `json:"type"`
	Description          string        `json:"description"`
	EstimatedImpactPaise int64         `json:"estimated_impact_paise"` // ₹ in paise
	Difficulty           Difficulty    `json:"difficulty"`             // easy, medium, hard
	TimelineDays         int           `json:"timeline_days"`           // how long to implement
	Details              map[string]interface{} `json:"details,omitempty"`
}

// ActionType defines the type of action recommended
type ActionType string

const (
	ActionReduceCategory ActionType = "reduce_category"
	ActionIncreaseSavings ActionType = "increase_savings"
	ActionMoveFunds      ActionType = "move_funds"
	ActionInvest         ActionType = "invest"
	ActionRefinance      ActionType = "refinance"
	ActionEnrollProgram  ActionType = "enroll_program"
)

// Difficulty represents how hard an action is to implement
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

// Risk represents potential risks of a recommendation
type Risk struct {
	ID          string      `json:"id"`
	Category    RiskCategory `json:"category"`
	Description string      `json:"description"`
	Severity    Severity    `json:"severity"` // low, medium, high
	Mitigation  string      `json:"mitigation"`
}

// RiskCategory defines risk domain
type RiskCategory string

const (
	RiskMarket       RiskCategory = "market"
	RiskLiquidity    RiskCategory = "liquidity"
	RiskCounterparty RiskCategory = "counterparty"
	RiskRegulatory   RiskCategory = "regulatory"
)

// Severity represents risk severity
type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// Recommendation represents a personalized financial recommendation
type Recommendation struct {
	RecommendationID string                 `json:"recommendation_id"`
	UserID           string                 `json:"user_id"`
	Category         RecommendationCategory `json:"category"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	Rationale        string                 `json:"rationale"` // Why this recommendation
	Actions          []Action               `json:"actions"`
	EstimatedImpact  ImpactEstimate         `json:"estimated_impact"`
	Risks            []Risk                 `json:"risks"`
	ComplianceConstraints []string          `json:"compliance_constraints"` // Rules that apply
	Status           RecommendationStatus   `json:"status"` // pending, accepted, rejected, etc.
	TraceID          string                 `json:"trace_id"` // For audit trail
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// RecommendationCategory defines recommendation type
type RecommendationCategory string

const (
	CategorySpendingOptimization RecommendationCategory = "spending_optimization"
	CategorySavings              RecommendationCategory = "savings"
	CategoryInvestment           RecommendationCategory = "investment"
	CategoryTaxEfficiency        RecommendationCategory = "tax_efficiency"
	CategoryRiskManagement       RecommendationCategory = "risk_management"
)

// RecommendationStatus defines recommendation lifecycle
type RecommendationStatus string

const (
	StatusPending      RecommendationStatus = "pending"
	StatusAccepted     RecommendationStatus = "accepted"
	StatusRejected     RecommendationStatus = "rejected"
	StatusDeferred     RecommendationStatus = "deferred"
	StatusImplemented  RecommendationStatus = "implemented"
)

// ImpactEstimate represents estimated financial impact
type ImpactEstimate struct {
	MonthlySavingsPaise int64   `json:"monthly_savings_paise"`
	AnnualReturnPaise   int64   `json:"annual_return_paise"`
	Confidence          float64 `json:"confidence"` // 0.0-1.0
}

// UserProfile represents user's financial profile for recommendations
type UserProfile struct {
	UserID                string                `json:"user_id"`
	RiskTolerance         RiskTolerance         `json:"risk_tolerance"`
	AnnualIncomePaise     int64                 `json:"annual_income_paise"`
	SavingsGoalPaise      int64                 `json:"savings_goal_paise"`
	InvestmentExperience  InvestmentExperience  `json:"investment_experience"`
	ComplianceRestrictions []string             `json:"compliance_restrictions"` // e.g., ["cannot_invest_offshore"]
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
}

// RiskTolerance represents user's risk appetite
type RiskTolerance string

const (
	RiskConservative RiskTolerance = "conservative"
	RiskModerate     RiskTolerance = "moderate"
	RiskAggressive   RiskTolerance = "aggressive"
)

// InvestmentExperience represents user's investment knowledge
type InvestmentExperience string

const (
	ExperienceBeginner      InvestmentExperience = "beginner"
	ExperienceIntermediate  InvestmentExperience = "intermediate"
	ExperienceExpert        InvestmentExperience = "expert"
)

// CalibrationMetrics represents recommendation accuracy metrics
type CalibrationMetrics struct {
	UserID               string    `json:"user_id"`
	RecommendationCount  int       `json:"recommendations_count"`
	AcceptanceRate       float64   `json:"acceptance_rate"` // % accepted / total
	AvgImpactRealized    float64   `json:"avg_impact_realized"` // actual / estimated ratio
	AccuracyScore        int       `json:"accuracy_score"` // 0-100
	LastCalibratedAt     time.Time `json:"last_calibrated_at"`
}

// RecommendationFeedback represents user feedback on a recommendation
type RecommendationFeedback struct {
	RecommendationID string         `json:"recommendation_id"`
	Action           FeedbackAction `json:"action"` // accept, reject, defer
	Reason           string         `json:"reason,omitempty"` // Why rejected/deferred
	DeferredUntil    *time.Time     `json:"deferred_until,omitempty"` // When to reconsider
	FeedbackAt       time.Time      `json:"feedback_at"`
}

// FeedbackAction represents user's response to recommendation
type FeedbackAction string

const (
	ActionAccept FeedbackAction = "accept"
	ActionReject FeedbackAction = "reject"
	ActionDefer  FeedbackAction = "defer"
)
