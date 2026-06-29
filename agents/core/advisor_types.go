// Package core — Advisor domain types.
//
// These types are now ALIASES to the canonical definitions in
// pkg/contract/v1. Aliasing (rather than re-defining) means core.UserProfile
// and contractv1.UserProfile are the SAME type — there is exactly one
// definition platform-wide, so the two copies can no longer drift and no
// conversion is needed at any boundary. See pkg/contract/v1/advisor.go for the
// authoritative struct definitions and field documentation.
package core

import contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"

// ========== ACTION TYPES ==========

type Action = contractv1.Action
type ActionType = contractv1.ActionType

const (
	ActionReduceCategory  = contractv1.ActionReduceCategory
	ActionIncreaseSavings = contractv1.ActionIncreaseSavings
	ActionMoveFunds       = contractv1.ActionMoveFunds
	ActionInvest          = contractv1.ActionInvest
	ActionRefinance       = contractv1.ActionRefinance
	ActionEnrollProgram   = contractv1.ActionEnrollProgram
)

type Difficulty = contractv1.Difficulty

const (
	DifficultyEasy   = contractv1.DifficultyEasy
	DifficultyMedium = contractv1.DifficultyMedium
	DifficultyHard   = contractv1.DifficultyHard
)

// ========== RISK TYPES ==========

type Risk = contractv1.Risk
type RiskCategory = contractv1.RiskCategory

const (
	RiskMarket       = contractv1.RiskMarket
	RiskLiquidity    = contractv1.RiskLiquidity
	RiskCounterparty = contractv1.RiskCounterparty
	RiskRegulatory   = contractv1.RiskRegulatory
)

type Severity = contractv1.Severity

const (
	SeverityLow    = contractv1.SeverityLow
	SeverityMedium = contractv1.SeverityMedium
	SeverityHigh   = contractv1.SeverityHigh
)

// ========== RECOMMENDATION TYPES ==========

type Recommendation = contractv1.Recommendation
type RecommendationCategory = contractv1.RecommendationCategory

const (
	CategorySpendingOptimization = contractv1.CategorySpendingOptimization
	CategorySavings              = contractv1.CategorySavings
	CategoryInvestment           = contractv1.CategoryInvestment
	CategoryTaxEfficiency        = contractv1.CategoryTaxEfficiency
	CategoryRiskManagement       = contractv1.CategoryRiskManagement
)

type RecommendationStatus = contractv1.RecommendationStatus

const (
	StatusPending     = contractv1.StatusPending
	StatusAccepted    = contractv1.StatusAccepted
	StatusRejected    = contractv1.StatusRejected
	StatusDeferred    = contractv1.StatusDeferred
	StatusImplemented = contractv1.StatusImplemented
)

type ImpactEstimate = contractv1.ImpactEstimate

// ========== USER PROFILE TYPES ==========

type UserProfile = contractv1.UserProfile
type RiskTolerance = contractv1.RiskTolerance

const (
	RiskConservative = contractv1.RiskConservative
	RiskModerate     = contractv1.RiskModerate
	RiskAggressive   = contractv1.RiskAggressive
)

type InvestmentExperience = contractv1.InvestmentExperience

const (
	ExperienceBeginner     = contractv1.ExperienceBeginner
	ExperienceIntermediate = contractv1.ExperienceIntermediate
	ExperienceExpert       = contractv1.ExperienceExpert
)

type SpendingOpportunity = contractv1.SpendingOpportunity

// ========== CALIBRATION / FEEDBACK TYPES ==========

type CalibrationMetrics = contractv1.CalibrationMetrics
type RecommendationFeedback = contractv1.RecommendationFeedback
type FeedbackAction = contractv1.FeedbackAction

const (
	ActionAccept = contractv1.ActionAccept
	ActionReject = contractv1.ActionReject
	ActionDefer  = contractv1.ActionDefer
)
