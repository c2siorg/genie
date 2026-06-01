package aml

import (
	"context"
	"fmt"
	"math"
	"time"
)

// AMLScorer is the interface for computing risk scores.
type AMLScorer interface {
	ScoreTransaction(ctx context.Context, txn Transaction, profile RiskProfile) (RiskScore, error)
}

// DefaultScorer implements AMLScorer with 5 weighted rules.
type DefaultScorer struct {
	config             *AMLConfig
	beneficiaryChecker BeneficiaryChecker
	// In production, would have access to:
	// - velocity cache (recent transaction counts per user)
	// - anomaly model (trained on historical patterns)
	// - jurisdiction database (high-risk country lists)
	velocityStore VelocityStore
}

// VelocityStore tracks transaction counts per user.
// Interface allows testing with mocks; production would use Redis/Memcached.
type VelocityStore interface {
	GetHourlyCount(ctx context.Context, userID string) (int, error)
	IncrementCount(ctx context.Context, userID string) error
}

// InMemoryVelocityStore is a simple demo store (not thread-safe; use with care).
type InMemoryVelocityStore struct {
	counts map[string]int
}

// NewInMemoryVelocityStore creates a demo velocity store.
func NewInMemoryVelocityStore() *InMemoryVelocityStore {
	return &InMemoryVelocityStore{
		counts: make(map[string]int),
	}
}

// GetHourlyCount returns the current transaction count for a user.
func (s *InMemoryVelocityStore) GetHourlyCount(ctx context.Context, userID string) (int, error) {
	// In reality, would check time-windowed counts; this is simplified.
	return s.counts[userID], nil
}

// IncrementCount increments the hourly count. In production, would also
// reset the window hourly or use TTL keys.
func (s *InMemoryVelocityStore) IncrementCount(ctx context.Context, userID string) error {
	s.counts[userID]++
	return nil
}

// NewDefaultScorer constructs a scorer with config and optional beneficiary checker.
func NewDefaultScorer(config *AMLConfig, checker BeneficiaryChecker) *DefaultScorer {
	if config == nil {
		config = DefaultAMLConfig()
	}
	if checker == nil {
		checker = NewMockBeneficiaryChecker()
	}
	return &DefaultScorer{
		config:             config,
		beneficiaryChecker: checker,
		velocityStore:      NewInMemoryVelocityStore(),
	}
}

// ScoreTransaction computes a risk score for a transaction.
//
// The algorithm:
// 1. Evaluate 5 rules independently
// 2. Weight each rule's contribution
// 3. Sum weighted contributions
// 4. Clamp to 0-100 (or allow >100 for escalation)
// 5. Return with evidence and triggered rules
func (s *DefaultScorer) ScoreTransaction(ctx context.Context, txn Transaction, profile RiskProfile) (RiskScore, error) {
	now := time.Now().UTC()
	rs := RiskScore{
		ScoredAt:    now,
		Evidence:    make(map[string]string),
		TriggeredRules: []string{},
	}

	// Rule 1: Threshold breach (30% weight)
	thresholdScore := s.scoreThreshold(txn, profile)
	if thresholdScore > 0 {
		rs.TriggeredRules = append(rs.TriggeredRules, "threshold")
		rs.Evidence["threshold"] = fmt.Sprintf("amount %.2f exceeds limit %.2f",
			float64(txn.Amount)/100.0, float64(profile.DailyLimit)/100.0)
		rs.Score += s.config.WeightThreshold * thresholdScore
	}

	// Rule 2: Velocity (25% weight)
	velocityScore := s.scoreVelocity(ctx, txn, profile)
	if velocityScore > 0 {
		rs.TriggeredRules = append(rs.TriggeredRules, "velocity")
		hourCount, _ := s.velocityStore.GetHourlyCount(ctx, txn.UserID)
		rs.Evidence["velocity"] = fmt.Sprintf("transaction count %d exceeds threshold %d/hour",
			hourCount, s.config.MaxTransactionsPerHour)
		rs.Score += s.config.WeightVelocity * velocityScore
	}
	// Increment velocity counter
	_ = s.velocityStore.IncrementCount(ctx, txn.UserID)

	// Rule 3: Jurisdiction (25% weight)
	jurisdictionScore := s.scoreJurisdiction(txn)
	if jurisdictionScore > 0 {
		rs.TriggeredRules = append(rs.TriggeredRules, "jurisdiction")
		rs.Evidence["jurisdiction"] = fmt.Sprintf("beneficiary country %s is high-risk",
			txn.BeneficiaryCountry)
		rs.Score += s.config.WeightJurisdiction * jurisdictionScore
	}

	// Rule 4: Beneficiary risk (15% weight)
	beneficiaryScore := s.scoreBeneficiary(ctx, txn)
	if beneficiaryScore > 0 {
		rs.TriggeredRules = append(rs.TriggeredRules, "beneficiary")
		rs.Evidence["beneficiary"] = fmt.Sprintf("beneficiary has elevated risk indicators")
		rs.Score += s.config.WeightBeneficiary * beneficiaryScore
	}

	// Rule 5: Anomaly detection (5% weight)
	anomalyScore := s.scoreAnomaly(txn, profile)
	if anomalyScore > 0 {
		rs.TriggeredRules = append(rs.TriggeredRules, "anomaly")
		rs.Evidence["anomaly"] = fmt.Sprintf("transaction pattern deviates from baseline")
		rs.Score += s.config.WeightAnomaly * anomalyScore
	}

	// Derive risk level from score
	rs.Level = s.levelFromScore(rs.Score)

	return rs, nil
}

// scoreThreshold returns 0-100 based on amount breach.
func (s *DefaultScorer) scoreThreshold(txn Transaction, profile RiskProfile) float64 {
	if profile.DailyLimit <= 0 {
		return 0 // no limit set
	}
	if txn.Amount <= profile.DailyLimit {
		return 0 // within limit
	}
	// Breach magnitude: how much over the limit?
	overage := float64(txn.Amount-profile.DailyLimit) / float64(profile.DailyLimit)
	// Cap at 100 so the rule alone doesn't exceed 100
	score := math.Min(100.0, overage*100.0)
	return score
}

// scoreVelocity returns 0-100 based on transaction frequency.
func (s *DefaultScorer) scoreVelocity(ctx context.Context, txn Transaction, profile RiskProfile) float64 {
	limit := s.config.MaxTransactionsPerHour
	if limit <= 0 {
		return 0
	}
	count, _ := s.velocityStore.GetHourlyCount(ctx, txn.UserID)
	if count < limit {
		return 0
	}
	// Excess: how many over the limit?
	excess := count - limit
	// Simple linear: each transaction over the limit adds 10 points
	score := math.Min(100.0, float64(excess)*10.0)
	return score
}

// scoreJurisdiction returns 0-100 based on beneficiary country risk.
func (s *DefaultScorer) scoreJurisdiction(txn Transaction) float64 {
	if HighRiskCountries[txn.BeneficiaryCountry] {
		return 100.0 // maximum risk for sanctioned country
	}
	return 0
}

// scoreBeneficiary returns 0-100 based on beneficiary checks.
func (s *DefaultScorer) scoreBeneficiary(ctx context.Context, txn Transaction) float64 {
	info, err := s.beneficiaryChecker.CheckBeneficiary(ctx, txn)
	if err != nil {
		// On error, don't penalize; log and continue
		return 0
	}
	return info.RiskScore
}

// scoreAnomaly returns 0-100 based on pattern deviation.
//
// Simplified for reference implementation:
// - If profile baseline is empty, no anomaly score
// - In production, would use ML models trained on user history
func (s *DefaultScorer) scoreAnomaly(txn Transaction, profile RiskProfile) float64 {
	if profile.AnomalyBaseline == "" {
		return 0
	}
	// Placeholder: in production, would compare txn characteristics
	// (amount, time of day, beneficiary, jurisdiction, frequency)
	// against the trained profile and compute a deviation score.
	//
	// For now, return 0 (no anomaly detected in reference implementation).
	return 0
}

// levelFromScore maps a numerical score to a risk level.
func (s *DefaultScorer) levelFromScore(score float64) RiskLevel {
	switch {
	case score <= s.config.ThresholdApprove:
		return RiskLevelApprove
	case score <= s.config.ThresholdMonitor:
		return RiskLevelMonitor
	case score <= s.config.ThresholdReview:
		return RiskLevelReview
	case score <= s.config.ThresholdEscalate:
		return RiskLevelEscalate
	default:
		return RiskLevelBlock
	}
}
