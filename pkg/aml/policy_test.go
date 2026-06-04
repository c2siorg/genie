package aml

import (
	"context"
	"testing"
)

func TestPolicyApproveScore(t *testing.T) {
	cfg := DefaultPolicyConfig()
	evaluator := NewYAMLPolicyEvaluator(cfg)

	score := RiskScore{
		Score:          15.0,
		Level:          RiskLevelApprove,
		TriggeredRules: []string{},
		Evidence:       make(map[string]string),
	}

	profile := RiskProfile{
		UserID:          "user-1",
		RiskLevel:       RiskLevelApprove,
		PolicyOverrides: make(map[string]float64),
	}

	result, err := evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelApprove {
		t.Errorf("expected approve level, got %s", result.Level)
	}
	if result.PolicyOverride {
		t.Errorf("expected no policy override")
	}
}

func TestPolicyReviewScore(t *testing.T) {
	cfg := DefaultPolicyConfig()
	evaluator := NewYAMLPolicyEvaluator(cfg)

	score := RiskScore{
		Score:          60.0,
		Level:          RiskLevelReview,
		TriggeredRules: []string{"threshold", "velocity"},
		Evidence: map[string]string{
			"threshold": "amount exceeds limit",
			"velocity":  "high transaction frequency",
		},
	}

	profile := RiskProfile{
		UserID:          "user-2",
		RiskLevel:       RiskLevelApprove,
		PolicyOverrides: make(map[string]float64),
	}

	result, err := evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelReview {
		t.Errorf("expected review level, got %s", result.Level)
	}
}

func TestPolicyEscalationRule(t *testing.T) {
	cfg := DefaultPolicyConfig()

	// Add escalation rule for high-risk scenario
	cfg.EscalationRules = []EscalationRule{
		{
			Condition: "jurisdiction && high_risk",
			Level:     "escalate",
			Action:    "escalate",
			Reasoning: "High-risk jurisdiction requires escalation",
		},
	}

	evaluator := NewYAMLPolicyEvaluator(cfg)

	score := RiskScore{
		Score:          70.0,
		Level:          RiskLevelReview,
		TriggeredRules: []string{"jurisdiction"},
		Evidence: map[string]string{
			"jurisdiction": "high-risk country",
		},
	}

	profile := RiskProfile{
		UserID:          "user-3",
		RiskLevel:       RiskLevelApprove,
		PolicyOverrides: make(map[string]float64),
	}

	result, err := evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelEscalate {
		t.Errorf("expected escalate level from rule, got %s", result.Level)
	}
}

func TestPolicyUserOverride(t *testing.T) {
	cfg := DefaultPolicyConfig()

	// Add override for whitelist user
	cfg.Overrides["user-whitelist"] = PolicyOverride{
		UserID:     "user-whitelist",
		Level:      "approve",
		Reason:     "Trusted partner account",
		ApprovedBy: "compliance-team",
	}

	evaluator := NewYAMLPolicyEvaluator(cfg)

	// Even high-risk score should be approved due to override
	score := RiskScore{
		Score:          80.0,
		Level:          RiskLevelEscalate,
		TriggeredRules: []string{"threshold", "jurisdiction"},
		Evidence: map[string]string{
			"threshold":    "amount exceeds limit",
			"jurisdiction": "sanctioned country",
		},
	}

	profile := RiskProfile{
		UserID:          "user-whitelist",
		RiskLevel:       RiskLevelApprove,
		PolicyOverrides: make(map[string]float64),
	}

	result, err := evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelApprove {
		t.Errorf("expected approve due to override, got %s", result.Level)
	}
	if !result.PolicyOverride {
		t.Errorf("expected policy override flag")
	}
}

func TestPolicySanctionedCountry(t *testing.T) {
	cfg := DefaultPolicyConfig()

	// Add escalation for sanctioned country
	cfg.EscalationRules = append(cfg.EscalationRules, EscalationRule{
		Condition: "jurisdiction && sanctioned",
		Level:     "block",
		Action:    "block",
		Reasoning: "Transaction to sanctioned jurisdiction is prohibited",
	})

	evaluator := NewYAMLPolicyEvaluator(cfg)

	score := RiskScore{
		Score:          75.0,
		Level:          RiskLevelEscalate,
		TriggeredRules: []string{"jurisdiction"},
		Evidence: map[string]string{
			"jurisdiction": "beneficiary country is sanctioned",
		},
	}

	profile := RiskProfile{
		UserID:          "user-4",
		RiskLevel:       RiskLevelApprove,
		PolicyOverrides: make(map[string]float64),
	}

	result, err := evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelEscalate && result.Level != RiskLevelBlock {
		t.Errorf("expected escalate/block for sanctioned country, got %s", result.Level)
	}
}

func TestDefaultPolicyConfig(t *testing.T) {
	cfg := DefaultPolicyConfig()

	// Verify structure
	if len(cfg.Rules) != 5 {
		t.Errorf("expected 5 rule categories, got %d", len(cfg.Rules))
	}

	expectedRules := []string{"approve", "monitor", "review", "escalate", "block"}
	for _, rule := range expectedRules {
		if _, exists := cfg.Rules[rule]; !exists {
			t.Errorf("missing rule: %s", rule)
		}
	}

	// Verify escalation rules
	if len(cfg.EscalationRules) == 0 {
		t.Errorf("expected escalation rules")
	}
}

func TestPolicyEvaluationFlow(t *testing.T) {
	// End-to-end: low score -> approve
	cfg := DefaultPolicyConfig()
	evaluator := NewYAMLPolicyEvaluator(cfg)

	score := RiskScore{
		Score:          20.0,
		Level:          RiskLevelApprove,
		TriggeredRules: []string{},
		Evidence:       make(map[string]string),
	}

	profile := RiskProfile{
		UserID:          "user-clean",
		RiskLevel:       RiskLevelApprove,
		PolicyOverrides: make(map[string]float64),
	}

	result, err := evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelApprove {
		t.Errorf("low score should result in approve, got %s", result.Level)
	}

	// Mid score -> review
	score.Score = 65.0
	score.Level = RiskLevelReview
	score.TriggeredRules = []string{"velocity"}

	result, err = evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelReview {
		t.Errorf("mid score should result in review, got %s", result.Level)
	}

	// High score -> escalate
	score.Score = 90.0
	score.Level = RiskLevelEscalate
	score.TriggeredRules = []string{"threshold", "jurisdiction", "velocity"}

	result, err = evaluator.Evaluate(context.Background(), score, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Level != RiskLevelEscalate {
		t.Errorf("high score should result in escalate, got %s", result.Level)
	}
}
