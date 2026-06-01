package aml

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	AgentID    = "aml_risk_scorer"
	AgentName  = "AML Risk Scoring Agent"
	Capability = "score_transaction"
)

// Agent implements the AML risk scoring agent for the Genie platform.
type Agent struct {
	scorer AMLScorer
	policy PolicyEvaluator
	config *AMLConfig
}

// ScoreRequest is the input to the score_transaction tool.
type ScoreRequest struct {
	TransactionID string `json:"transaction_id"`
	UserID        string `json:"user_id"`
	Amount        int64  `json:"amount"` // cents/minor units
	Beneficiary   struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Country string `json:"country"` // ISO-2 code
	} `json:"beneficiary"`
	Description string `json:"description"`
}

// ScoreResponse is the output of the score_transaction tool.
type ScoreResponse struct {
	TransactionID  string                 `json:"transaction_id"`
	RiskScore      float64                `json:"risk_score"`
	RiskLevel      RiskLevel              `json:"risk_level"`
	TriggeredRules []string               `json:"triggered_rules"`
	Evidence       map[string]string      `json:"evidence"`
	Recommendation string                 `json:"recommendation"`
	TraceID        string                 `json:"trace_id"` // For linking with Laminar observability
}

// New constructs an AML agent with default configuration.
func New() *Agent {
	cfg := DefaultAMLConfig()
	scorer := NewDefaultScorer(cfg, NewMockBeneficiaryChecker())
	policy := NewYAMLPolicyEvaluator(DefaultPolicyConfig())

	return &Agent{
		scorer: scorer,
		policy: policy,
		config: cfg,
	}
}

// NewWithConfig constructs an AML agent with custom configuration.
func NewWithConfig(cfg *AMLConfig, scorer AMLScorer, policy PolicyEvaluator) *Agent {
	if cfg == nil {
		cfg = DefaultAMLConfig()
	}
	if scorer == nil {
		scorer = NewDefaultScorer(cfg, NewMockBeneficiaryChecker())
	}
	if policy == nil {
		policy = NewYAMLPolicyEvaluator(DefaultPolicyConfig())
	}

	return &Agent{
		scorer: scorer,
		policy: policy,
		config: cfg,
	}
}

// Register implements agent.Agent interface.
func (a *Agent) ID() string {
	return AgentID
}

// Name implements agent.Agent interface.
func (a *Agent) Name() string {
	return AgentName
}

// Capabilities implements agent.Agent interface.
func (a *Agent) Capabilities() []string {
	return []string{Capability}
}

// HandleMessage implements agent.Agent interface.
//
// Expected message types:
// - "score_transaction": requests risk scoring for a transaction
// - "get_config": returns current AML configuration
// - "get_policy": returns current policy rules
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	switch msg.Type {
	case "score_transaction":
		return a.handleScoreTransaction(ctx, msg, env)
	case "get_config":
		return a.handleGetConfig(ctx, msg, env)
	case "get_policy":
		return a.handleGetPolicy(ctx, msg, env)
	default:
		return nil, nil
	}
}

// handleScoreTransaction processes a score_transaction message.
func (a *Agent) handleScoreTransaction(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req ScoreRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		env.Logf("[aml_agent] error decoding score_transaction: %v", err)
		return nil, err
	}

	// Build transaction and profile from request
	// Convert metadata from map[string]any to map[string]string
	metadataStr := make(map[string]string)
	if msg.Metadata != nil {
		for k, v := range msg.Metadata {
			if str, ok := v.(string); ok {
				metadataStr[k] = str
			}
		}
	}

	txn := Transaction{
		ID:                 req.TransactionID,
		UserID:             req.UserID,
		Amount:             req.Amount,
		BeneficiaryID:      req.Beneficiary.ID,
		BeneficiaryName:    req.Beneficiary.Name,
		BeneficiaryCountry: req.Beneficiary.Country,
		Timestamp:          time.Now().UTC(),
		Description:        req.Description,
		Metadata:           metadataStr,
	}

	// Default risk profile (in production, would fetch from customer database)
	profile := RiskProfile{
		UserID:          req.UserID,
		RiskLevel:       RiskLevelApprove,
		DailyLimit:      1000000, // 10,000 units default
		VelocityLimit:   10,
		FirstSeen:       time.Now().UTC().Add(-30 * 24 * time.Hour), // assume 30 days old
		AnomalyBaseline: "",
		PolicyOverrides: make(map[string]float64),
	}

	// Score the transaction
	score, err := a.scorer.ScoreTransaction(ctx, txn, profile)
	if err != nil {
		env.Logf("[aml_agent] error scoring transaction: %v", err)
		return nil, err
	}

	// Apply policy evaluation
	score, err = a.policy.Evaluate(ctx, score, profile)
	if err != nil {
		env.Logf("[aml_agent] error evaluating policy: %v", err)
		return nil, err
	}

	// Build response
	recommendation := a.recommendAction(score.Level)
	resp := ScoreResponse{
		TransactionID:  req.TransactionID,
		RiskScore:      score.Score,
		RiskLevel:      score.Level,
		TriggeredRules: score.TriggeredRules,
		Evidence:       score.Evidence,
		Recommendation: recommendation,
		TraceID:        fmt.Sprintf("%s-%d", req.TransactionID, time.Now().UnixNano()),
	}

	body, _ := json.Marshal(resp)
	env.Logf("[aml_agent] txn=%s score=%.1f level=%s rules=%d",
		req.TransactionID, score.Score, score.Level, len(score.TriggeredRules))

	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "score_response", string(body), msg.Metadata),
	}, nil
}

// handleGetConfig returns the current AML configuration.
func (a *Agent) handleGetConfig(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	body, _ := json.Marshal(a.config)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "config_response", string(body), msg.Metadata),
	}, nil
}

// handleGetPolicy returns the current policy rules.
func (a *Agent) handleGetPolicy(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	policy := DefaultPolicyConfig()
	body, _ := json.Marshal(policy)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "policy_response", string(body), msg.Metadata),
	}, nil
}

// recommendAction provides a human-readable recommendation based on risk level.
func (a *Agent) recommendAction(level RiskLevel) string {
	switch level {
	case RiskLevelApprove:
		return "Approve transaction. No action required."
	case RiskLevelMonitor:
		return "Approve with monitoring. Flag account for continued observation."
	case RiskLevelReview:
		return "Hold transaction pending manual analyst review."
	case RiskLevelEscalate:
		return "Escalate to compliance team. Potential SAR/STR filing may be required."
	case RiskLevelBlock:
		return "Block transaction immediately. Contact law enforcement if warranted."
	default:
		return "Unknown risk level; escalate to compliance."
	}
}

