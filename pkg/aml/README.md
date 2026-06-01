# AML Risk Scoring Agent

The AML (Anti-Money Laundering) Risk Scoring Agent is a compliance-grade transaction risk assessment engine for the Genie multi-agent platform. It provides:

- **5-Rule Risk Scoring**: Threshold breach, velocity, jurisdiction, beneficiary risk, anomaly detection
- **Policy-Driven Decisions**: YAML-based policy configuration for compliance team governance
- **OPA Integration Ready**: Infrastructure for Open Policy Agent evaluation
- **HITL Support**: Structured output for human-in-the-loop analyst review workflows
- **Production Patterns**: Thread-safe, error-handling, structured logging, Laminar tracing

## Architecture

### Core Components

#### Types (`types.go`)
- **RiskScore**: Numerical score (0-100+), decision level, triggered rules, evidence, policy overrides
- **RiskProfile**: User baseline (risk level, daily limit, velocity limit, first-seen date)
- **Transaction**: The transaction being scored (amount, beneficiary, timestamp, metadata)
- **AMLConfig**: Thresholds, rule weights, velocity limits, beneficiary check flags
- **BeneficiaryRiskInfo**: PEP/sanctioned/adverse-media flags for a beneficiary

#### Scorer (`scorer.go`)
- **AMLScorer interface**: Pluggable scoring implementations
- **DefaultScorer**: Reference implementation with 5 weighted rules
  - Threshold: 30% weight - amount > limit
  - Velocity: 25% weight - transaction frequency > threshold
  - Jurisdiction: 25% weight - high-risk countries (OFAC, UN lists)
  - Beneficiary: 15% weight - PEP, sanctioned, adverse media
  - Anomaly: 5% weight - pattern deviation (ML-ready placeholder)
- **VelocityStore interface**: In-memory and cacheable implementations

#### Policy Evaluator (`policy.go`)
- **PolicyEvaluator interface**: Pluggable policy engine
- **YAMLPolicyEvaluator**: YAML config + hardcoded logic (OPA SDK ready)
  - Load policies from files: `LoadPolicyFromYAML(path)`
  - Evaluate rules: score → decision (approve|monitor|review|escalate|block)
  - User overrides: whitelist/exception handling
  - Escalation rules: conditional logic (PEP + sanctioned = escalate)

#### Agent (`agent.go`)
- **Agent**: Genie agent implementation with tool registration
- **Tool**: `score_transaction` - LLM-invocable transaction scoring
- **Message Types**:
  - `score_transaction` - request scoring
  - `get_config` - retrieve current configuration
  - `get_policy` - retrieve current policy rules

#### Beneficiary Checker (`beneficiary.go`)
- **BeneficiaryChecker interface**: Pluggable external service integration
- **MockBeneficiaryChecker**: Demo implementation with hardcoded PEP/sanctioned lists
  - Production would integrate OFAC CSL, UN lists, adverse media feeds, local watchlists

### Scoring Algorithm

```
RiskScore = WeightThreshold × ThresholdScore
          + WeightVelocity × VelocityScore
          + WeightJurisdiction × JurisdictionScore
          + WeightBeneficiary × BeneficiaryScore
          + WeightAnomaly × AnomalyScore

Level = LevelFromScore(RiskScore)
      = { Approve (0-30), Monitor (31-50), Review (51-70), Escalate (71-100), Block (>100) }
```

Each rule returns a 0-100 score independently, then weighted contributions are summed.

### Policy Evaluation Flow

```
1. Load Policy (YAML or code config)
2. For each transaction:
   a. Score transaction (5 rules, weighted sum)
   b. Evaluate policy:
      - Check user overrides (whitelist exceptions)
      - Check escalation rules (PEP + jurisdiction = escalate)
      - Map score to level
   c. Return decision with evidence + recommendation
3. Return to HITL (analyst review) or auto-action (post/hold/block)
```

## Usage

### Basic Scoring

```go
package main

import (
    "context"
    "github.com/c2siorg/genie/pkg/aml"
)

func main() {
    // Create scorer with default config
    scorer := aml.NewDefaultScorer(nil, nil)
    
    // Define transaction
    txn := aml.Transaction{
        ID:                 "txn-123",
        UserID:             "user-456",
        Amount:             2500000,        // 25,000 units
        BeneficiaryCountry: "IR",           // Iran (high-risk)
        BeneficiaryName:    "Entity X",
        Timestamp:          time.Now(),
    }
    
    // Define user profile
    profile := aml.RiskProfile{
        UserID:          "user-456",
        DailyLimit:      1000000,          // 10,000 units
        VelocityLimit:   10,
        FirstSeen:       time.Now().Add(-30 * 24 * time.Hour),
        PolicyOverrides: make(map[string]float64),
    }
    
    // Score the transaction
    score, err := scorer.ScoreTransaction(context.Background(), txn, profile)
    if err != nil {
        panic(err)
    }
    
    // Apply policy
    policy := aml.NewYAMLPolicyEvaluator(aml.DefaultPolicyConfig())
    result, err := policy.Evaluate(context.Background(), score, profile)
    if err != nil {
        panic(err)
    }
    
    println("Risk Level:", result.Level)
    println("Score:", result.Score)
    println("Rules:", result.TriggeredRules)
}
```

### Agent Integration

```go
import "github.com/c2siorg/genie/pkg/aml"

// Create agent
agent := aml.New()

// Use in message handler
msg := agent.NewMessage(
    "requester",
    aml.AgentID,
    agent.RoleUser,
    "score_transaction",
    `{
        "transaction_id": "txn-123",
        "user_id": "user-456",
        "amount": 2500000,
        "beneficiary": {
            "id": "ben-789",
            "name": "Entity X",
            "country": "IR"
        }
    }`,
    nil,
)

responses, _ := agent.HandleMessage(ctx, msg, env)
// responses[0].Content contains ScoreResponse JSON
```

### Custom Configuration

```go
cfg := &aml.AMLConfig{
    ThresholdApprove:   25.0,   // Adjust from 30
    ThresholdMonitor:   40.0,   // Adjust from 50
    ThresholdReview:    65.0,   // Adjust from 70
    ThresholdEscalate:  95.0,   // Adjust from 100
    WeightThreshold:    0.35,   // Adjust weights
    WeightVelocity:     0.30,
    WeightJurisdiction: 0.20,
    WeightBeneficiary:  0.10,
    WeightAnomaly:      0.05,
    MaxTransactionsPerHour: 5,
    CheckPEP:           true,
    CheckSanctions:     true,
    CheckAdverseMedia:  true,
    NewAccountDaysLimit: 14,
}

scorer := aml.NewDefaultScorer(cfg, checker)
```

### YAML Policy Configuration

Edit `config.example.yaml` and load it:

```go
policy, err := aml.LoadPolicyFromYAML("pkg/aml/config.example.yaml")
if err != nil {
    panic(err)
}

score, _ := scorer.ScoreTransaction(ctx, txn, profile)
result, _ := policy.Evaluate(ctx, score, profile)
```

## Test Scenarios

The test suite covers:

1. **Threshold Breach**: Amount exceeds limit → elevated risk
2. **Velocity**: High transaction frequency → escalation
3. **Jurisdiction**: High-risk country detection → escalation
4. **Beneficiary**: PEP/sanctioned/adverse media flags → escalation
5. **Smurfing**: Multiple small transactions to avoid detection → velocity flag
6. **Sanctions + Velocity**: Combined high-risk indicators
7. **Round-Tripping**: Circular transfers between accounts
8. **Extreme Velocity**: Many rapid transactions

Run tests:
```bash
go test ./pkg/aml/...
```

## Integration with HITL

The AML Agent is designed to feed into human-in-the-loop workflows:

### Output to Analyst Review

```json
{
  "transaction_id": "txn-123",
  "risk_score": 75.5,
  "risk_level": "escalate",
  "triggered_rules": ["threshold", "jurisdiction"],
  "evidence": {
    "threshold": "amount 25,000 exceeds limit 10,000",
    "jurisdiction": "beneficiary country IR is high-risk"
  },
  "recommendation": "Escalate to compliance team. Potential SAR/STR filing may be required.",
  "trace_id": "txn-123-1717175400000000000"
}
```

### HITL Actions

- **Approve (Level ≤ 30)**: Auto-post, monitor account
- **Monitor (31-50)**: Post, add to watchlist
- **Review (51-70)**: Hold, route to analyst for KYC update or SAR decision
- **Escalate (71-100)**: Halt, escalate to compliance officer, prepare SAR filing
- **Block (> 100)**: Reject, flag for enforcement investigation

## Observability & Tracing

Each scored transaction generates a trace ID for Laminar linkage:

```
trace_id: txn-123-1717175400000000000
```

This links:
- Transaction ID
- Risk score computation
- Policy evaluation decision
- Analyst actions (if HITL)
- Regulatory reporting (SAR/STR filing)

## Production Readiness Checklist

- [x] 5-rule scoring engine with weighted aggregation
- [x] YAML policy configuration for compliance teams
- [x] User overrides and escalation rules
- [x] Pluggable BeneficiaryChecker (mock + interface for real services)
- [x] Thread-safe velocity store (in-memory + interface for Redis)
- [x] Structured evidence and reasoning
- [x] Comprehensive test suite (8+ scenarios)
- [x] Agent integration with tool registration
- [x] Error handling and logging
- [x] Laminar tracing support (trace_id in response)
- [ ] OPA Rego policy engine (infrastructure ready, awaiting real SDK integration)
- [ ] ML-based anomaly detection (placeholder, ready for model integration)
- [ ] Real beneficiary checkers (OFAC, UN, PEP databases)
- [ ] Redis velocity store (mock in-memory provided)
- [ ] Audit trail and policy change logging

## Future Enhancements

1. **ML Anomaly Detection**: Train models on user transaction history; replace placeholder
2. **Real Beneficiary Checks**: Integrate OFAC CSL, UN lists, commercial PEP/adverse media feeds
3. **OPA Rego Integration**: Use real OPA SDK for complex policy logic
4. **Redis Cache**: Replace in-memory velocity store for distributed deployment
5. **Webhook Callbacks**: Notify external systems on escalation (SAR filing, customer notification)
6. **Batch Scoring**: Score multiple transactions in one call (compliance reporting)
7. **Analytics Dashboard**: Track rules triggered, levels distributed, policy override rates
8. **A/B Testing**: Compare policy versions, measure impact on false positives

## References

- [FATF AML/CFT Recommendations](https://www.fatf-gafi.org/publications/fatfrecommendations/)
- [OPA Documentation](https://www.openpolicyagent.org/)
- [OFAC Sanctions List](https://sanctionsearch.ofac.treas.gov/)
- [UN Security Council Consolidated List](https://www.un.org/securitycouncil/sanctions/)
- [Transparency International PEP Database](https://www.transparency.org/)

## Support

For questions or contributions, contact the compliance engineering team or open an issue.
