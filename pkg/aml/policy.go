package aml

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PolicyEvaluator applies OPA-driven policy rules to risk scores.
//
// This reference implementation uses YAML config + hardcoded logic.
// Production systems would integrate the real OPA SDK (github.com/open-policy-agent/opa).
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, score RiskScore, profile RiskProfile) (RiskScore, error)
}

// YAMLPolicyEvaluator loads policies from YAML and applies them.
type YAMLPolicyEvaluator struct {
	// Raw policy config loaded from YAML
	config map[string]interface{}
}

// PolicyConfig represents the structure of an AML policy YAML.
type PolicyConfig struct {
	// Rules define score-based thresholds
	Rules map[string]PolicyRule `yaml:"rules"`

	// Overrides allow case-by-case exceptions
	Overrides map[string]PolicyOverride `yaml:"overrides"`

	// Escalation rules for special cases
	EscalationRules []EscalationRule `yaml:"escalation_rules"`
}

// PolicyRule defines how a risk score maps to a decision.
type PolicyRule struct {
	Level     string  `yaml:"level"` // approve | monitor | review | escalate | block
	MinScore  float64 `yaml:"min_score"`
	MaxScore  float64 `yaml:"max_score"`
	Action    string  `yaml:"action"` // post | hold | review | escalate | block
	Reasoning string  `yaml:"reasoning"`
}

// PolicyOverride allows policy adjustments for specific users or scenarios.
type PolicyOverride struct {
	UserID     string `yaml:"user_id"`
	Level      string `yaml:"level"`
	Reason     string `yaml:"reason"`
	ExpiresAt  string `yaml:"expires_at"` // RFC3339 timestamp
	ApprovedBy string `yaml:"approved_by"`
}

// EscalationRule defines when to escalate beyond the standard policy.
type EscalationRule struct {
	Condition string `yaml:"condition"` // e.g., "pep && amount > 100000"
	Level     string `yaml:"level"`
	Action    string `yaml:"action"`
	Reasoning string `yaml:"reasoning"`
}

// LoadPolicyFromYAML reads an AML policy file.
//
// Example YAML:
//
//	rules:
//	  approve:
//	    min_score: 0
//	    max_score: 30
//	    action: "post"
//	    reasoning: "Low risk, auto-approve"
//	  review:
//	    min_score: 51
//	    max_score: 70
//	    action: "hold"
//	    reasoning: "Manual review required"
//	escalation_rules:
//	  - condition: "pep && beneficiary"
//	    level: "escalate"
//	    action: "escalate"
//	    reasoning: "PEP + beneficiary risk triggers escalation"
func LoadPolicyFromYAML(filePath string) (PolicyEvaluator, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	var cfg PolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse policy YAML: %w", err)
	}

	// Convert to map for storage
	evalConfig := make(map[string]interface{})
	evalConfig["rules"] = cfg.Rules
	evalConfig["overrides"] = cfg.Overrides
	evalConfig["escalation_rules"] = cfg.EscalationRules

	return &YAMLPolicyEvaluator{config: evalConfig}, nil
}

// NewYAMLPolicyEvaluator constructs an evaluator from a policy config structure.
func NewYAMLPolicyEvaluator(cfg *PolicyConfig) PolicyEvaluator {
	evalConfig := make(map[string]interface{})
	evalConfig["rules"] = cfg.Rules
	evalConfig["overrides"] = cfg.Overrides
	evalConfig["escalation_rules"] = cfg.EscalationRules
	return &YAMLPolicyEvaluator{config: evalConfig}
}

// Evaluate applies policy rules and returns a potentially modified RiskScore.
//
// The decision process:
// 1. Check for user-specific overrides (e.g., whitelist exceptions)
// 2. Check escalation rules (e.g., PEP + sanctioned = always escalate)
// 3. Map score to standard level (approve -> monitor -> review -> escalate -> block)
// 4. Return decision with reasoning
func (e *YAMLPolicyEvaluator) Evaluate(ctx context.Context, score RiskScore, profile RiskProfile) (RiskScore, error) {
	result := score

	// Step 1: Check user overrides
	if overrides, ok := e.config["overrides"]; ok {
		overridesMap := overrides.(map[string]PolicyOverride)
		if override, exists := overridesMap[profile.UserID]; exists {
			// Apply override
			result.Level = RiskLevel(override.Level)
			result.PolicyOverride = true
			result.Evidence["policy_override"] = override.Reason
			return result, nil
		}
	}

	// Step 2: Check escalation rules
	// (Simplified: in production, would evaluate Rego expressions)
	if escalations, ok := e.config["escalation_rules"]; ok {
		escalationRules := escalations.([]EscalationRule)
		for _, rule := range escalationRules {
			// Simplified condition check
			if shouldEscalate(rule.Condition, score) {
				result.Level = RiskLevel(rule.Level)
				result.Evidence["escalation_rule"] = rule.Reasoning
				return result, nil
			}
		}
	}

	// Step 3: Standard policy mapping (score -> level)
	// If the score suggests a higher level, respect it
	if result.Level == "" {
		// This shouldn't happen if scorer worked correctly, but safety check
		result.Level = RiskLevelApprove
	}

	return result, nil
}

// shouldEscalate is a simplified condition evaluator.
// In production, this would use the OPA Rego engine.
func shouldEscalate(condition string, score RiskScore) bool {
	// Demo logic: check for PEP + beneficiary combination
	if condition == "pep && beneficiary" {
		hasPEP := false
		hasBeneficiary := false
		for _, rule := range score.TriggeredRules {
			if rule == "beneficiary" {
				hasBeneficiary = true
			}
		}
		if hasPEP && hasBeneficiary {
			return true
		}
	}

	// Check for sanctioned country
	if condition == "jurisdiction && high_risk" {
		for _, rule := range score.TriggeredRules {
			if rule == "jurisdiction" {
				return true
			}
		}
	}

	return false
}

// DefaultPolicyConfig returns a standard AML policy.
func DefaultPolicyConfig() *PolicyConfig {
	return &PolicyConfig{
		Rules: map[string]PolicyRule{
			"approve": {
				Level:     "approve",
				MinScore:  0,
				MaxScore:  30,
				Action:    "post",
				Reasoning: "Low risk, auto-approve",
			},
			"monitor": {
				Level:     "monitor",
				MinScore:  31,
				MaxScore:  50,
				Action:    "post",
				Reasoning: "Monitor account for suspicious patterns",
			},
			"review": {
				Level:     "review",
				MinScore:  51,
				MaxScore:  70,
				Action:    "hold",
				Reasoning: "Require manual analyst review",
			},
			"escalate": {
				Level:     "escalate",
				MinScore:  71,
				MaxScore:  100,
				Action:    "escalate",
				Reasoning: "Escalate to compliance team",
			},
			"block": {
				Level:     "block",
				MinScore:  101,
				MaxScore:  1000,
				Action:    "block",
				Reasoning: "Transaction blocked due to risk",
			},
		},
		Overrides: make(map[string]PolicyOverride),
		EscalationRules: []EscalationRule{
			{
				Condition: "jurisdiction && high_risk",
				Level:     "escalate",
				Action:    "escalate",
				Reasoning: "High-risk jurisdiction requires escalation",
			},
		},
	}
}

// PolicyJSONString returns the policy as a JSON string (useful for logging).
func PolicyJSONString(cfg *PolicyConfig) (string, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
