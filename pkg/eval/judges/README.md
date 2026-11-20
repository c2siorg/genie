# LLM-as-Judge Evaluators for Genie

This package implements 4 specialized LLM-based judges with calibration for evaluating critical components of the Genie financial system. Each judge is designed to achieve **TPR ≥ 0.90** and **TNR ≥ 0.90** via bootstrapping with 95% confidence intervals.

## Overview

The judges evaluate:

| Judge | Rubric | Focus | Domain |
|-------|--------|-------|--------|
| **SettlementJudge** | RB-SE-003 | Amount correctness, netting, CBDC ledger alignment, reconciliation | Settlement |
| **ComplianceJudge** | RB-CO-001 | KYC status, AML risk scoring, velocity thresholds, audit trails | Compliance |
| **OrchestrationJudge** | RB-OR-001 | State machine validity, workflow sequences, idempotency | Orchestration |
| **LineageJudge** | RB-LA-001 | Hash chain integrity, timestamp monotonicity, audit completeness | Audit Trail |

## Architecture

### Judge Interface

All judges implement a common interface:

```go
type Judge interface {
    // Evaluate runs judgment on the input and returns a verdict
    Evaluate(ctx context.Context, input interface{}) (Verdict, error)
    
    // Calibrate runs calibration on ground truth samples
    Calibrate(ctx context.Context, samples []interface{}, groundTruth []bool) (CalibrationResult, error)
    
    // Name returns the judge's human-readable name
    Name() string
}
```

### Verdict Structure

Each evaluation returns:

```go
type Verdict struct {
    Pass        bool      // Pass/fail decision
    Score       float64   // Confidence score (0.0-1.0)
    RubricID    string    // Reference to evaluation rubric
    RubricName  string    // Human-readable rubric name
    Reason      string    // Brief explanation of verdict
    Evidence    string    // Supporting facts (JSON)
    EvaluatedAt time.Time // When evaluation occurred
}
```

### Calibration Results

Calibration returns:

```go
type CalibrationResult struct {
    NumPositives      int            // Number of positive examples
    NumNegatives      int            // Number of negative examples
    TPR               float64        // True Positive Rate (recall)
    TNR               float64        // True Negative Rate (specificity)
    TPRLowerBound     float64        // 95% CI lower bound
    TPRUpperBound     float64        // 95% CI upper bound
    TNRLowerBound     float64        // 95% CI lower bound
    TNRUpperBound     float64        // 95% CI upper bound
    OptimalThreshold  float64        // Threshold maximizing sensitivity + specificity
    Samples           []CalibrationSample
    CalibratedAt      time.Time
}
```

## Settlement Judge

Evaluates settlement transactions for:
- **SettlementAmountCorrectness**: Does settlement amount match expected?
- **NettingApplication**: If netting applied, is netted amount correct?
- **CBDCAlignment**: Did CBDC ledger commit succeed?
- **ReconciliationIntegrity**: Do order-ledger-lineage match?

### Usage

```go
judge := NewSettlementJudge(JudgeConfig{
    Provider: "ollama",
    Model:    "llama3.1",
})

input := SettlementJudgeInput{
    OrderID:                  "order-001",
    SettlementID:             "settlement-001",
    SettlementAmount:         100000,
    ExpectedAmount:           100000,
    OrderAmount:              100000,
    NettingApplied:           false,
    CBDCCommitted:            true,
    ReconciliationPassed:     true,
    AuditTrail:               `[{"step": "settlement_completed"}]`,
}

ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

verdict, err := judge.Evaluate(ctx, input)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Verdict: %v (score: %.2f)\n", verdict.Pass, verdict.Score)
fmt.Printf("Reason: %s\n", verdict.Reason)
```

### Evaluation Rubric (RB-SE-003)

**Scoring:**
- **Excellent (0.8-1.0)**: Settlement amount correct, netting properly applied, CBDC committed, reconciliation passed
- **Good (0.6-0.8)**: Settlement amount correct, netting unclear, reconciliation passed
- **Fair (0.4-0.6)**: Settlement amount questionable, reconciliation issues, CBDC not committed
- **Poor (0.0-0.4)**: Settlement amount wrong, netting misapplied, reconciliation failed

## Compliance Judge

Evaluates compliance enforcement for:
- **KYCJustification**: Is customer KYC verified and not expired?
- **AMLThresholds**: Is AML risk score acceptable (<70)?
- **VelocityEnforcement**: Is transaction velocity within threshold?
- **AuditTrails**: Are compliance decisions fully logged with evidence?

### Usage

```go
judge := NewComplianceJudge(JudgeConfig{
    Provider: "ollama",
    Model:    "llama3.1",
})

input := ComplianceJudgeInput{
    CustomerID:             "customer-001",
    OrderID:                "order-001",
    KYCStatus:              "verified",
    KYCExpiryDate:          time.Now().AddDate(0, 6, 0),
    VelocityWindowSeconds:  3600,
    VelocityThresholdPaise: 5000000,
    CurrentVelocityPaise:   1000000,
    SanctionsListAge:       3600,
    MatchingThreshold:      0.95,
    AMLRiskScore:           15.0,
    VelocityExceeded:       false,
    ConsentProvided:        true,
    AuditTrail:             `[{"check": "compliant"}]`,
}

verdict, err := judge.Evaluate(ctx, input)
if err != nil {
    log.Fatal(err)
}
```

### Evaluation Rubric (RB-CO-001)

**Scoring:**
- **Pass (0.8-1.0)**: KYC verified, velocity within threshold, AML risk acceptable, audit complete
- **Pass (0.6-0.8)**: KYC verified, velocity acceptable, audit mostly complete
- **Fail (0.4-0.6)**: KYC pending/expired, velocity exceeded, AML risk high, audit gaps
- **Fail (0.0-0.4)**: KYC failed, velocity severely exceeded, AML risk critical, audit missing

## Orchestration Judge

Evaluates workflow orchestration for:
- **StateValidation**: Are state transitions allowed by the state machine?
- **WorkflowSequences**: Do steps execute in correct order (payment before settlement)?
- **MultiAgentConsistency**: Are payment/settlement agents called in proper sequence?
- **IdempotencyEnforcement**: Are retry requests protected by idempotency keys?

### Valid State Transitions

```
OrderCreated → PaymentInitiated → PaymentConfirmed → SettlementInitiated → SettlementCompleted → Fulfilled
    ↓                ↓                  ↓                    ↓                     ↓
  Cancelled      Cancelled         Cancelled           Cancelled             Cancelled
```

### Usage

```go
judge := NewOrchestrationJudge(JudgeConfig{
    Provider: "ollama",
    Model:    "llama3.1",
})

input := OrchestrationJudgeInput{
    OrderID:                "order-001",
    StepSequence:           []string{"order_created", "payment_initiated", "payment_confirmed"},
    CurrentStep:            "payment_confirmed",
    PreviousStep:           "payment_initiated",
    TransitionValid:        true,
    IdempotencyKey:         "idempotency-001",
    PaymentConfirmed:       true,
    SettlementInitiated:    false,
    WorkflowAuditTrail:     `[{"step": "payment_initiated"}, {"step": "payment_confirmed"}]`,
}

verdict, err := judge.Evaluate(ctx, input)
if err != nil {
    log.Fatal(err)
}
```

### Evaluation Rubric (RB-OR-001)

**Scoring:**
- **Pass (0.8-1.0)**: Valid transition, correct step order, idempotency key present
- **Pass (0.6-0.8)**: Valid transition, correct order, minor idempotency concerns
- **Fail (0.4-0.6)**: Invalid transition or wrong order, idempotency unclear
- **Fail (0.0-0.4)**: Invalid transition allowed, settlement before payment, no idempotency

## Lineage Judge

Evaluates audit trail integrity for:
- **HashChainIntegrity**: Are lineage entries immutable and linked by hash chain?
- **DecisionTraceability**: Does audit provide full decision context?
- **TimestampMonotonicity**: Are timestamps strictly increasing within the order?
- **CompleteLogs**: Are all required audit fields present?

### Deterministic Checks

The LineageJudge performs deterministic verification before calling the LLM:

1. **Hash Chain Verification**: Each entry commits the hash of the previous entry
2. **Timestamp Monotonicity**: Timestamps must be strictly increasing (no equal values)
3. **Required Fields**: All entries must have entryID, orderID, step, timestamp, data, hash

### Usage

```go
judge := NewLineageJudge(JudgeConfig{
    Provider: "ollama",
    Model:    "llama3.1",
})

// Build valid hash chain
entry1 := LineageEntry{
    EntryID:      "entry-001",
    OrderID:      "order-001",
    Step:         "order_created",
    Timestamp:    time.Now(),
    PreviousHash: "",
    Data:         `{"amount": 100000}`,
}
entry1.Hash = computeHash(entry1.OrderID, entry1.Step, entry1.Timestamp.String(), entry1.Data)

entry2 := LineageEntry{
    EntryID:      "entry-002",
    OrderID:      "order-001",
    Step:         "payment_confirmed",
    Timestamp:    time.Now().Add(1*time.Second),
    PreviousHash: entry1.Hash,
    Data:         `{"confirmed": true}`,
}
entry2.Hash = computeHash(entry2.OrderID, entry2.Step, entry2.Timestamp.String(), entry2.Data)

input := LineageJudgeInput{
    OrderID:                      "order-001",
    LineageEntries:               []LineageEntry{entry1, entry2},
    HashChainValid:               true,
    TimestampsMonotonic:          true,
    AllRequiredFieldsPresent:     true,
    DecisionTraceability:         `{"decision": "payment approved"}`,
}

verdict, err := judge.Evaluate(ctx, input)
if err != nil {
    log.Fatal(err)
}
```

### Hash Chain Algorithm

Each entry's hash is computed as:

```
H(n) = SHA256(orderID || step || timestamp || data)
```

And linked by:

```
entry[n].PreviousHash = entry[n-1].Hash
```

### Evaluation Rubric (RB-LA-001)

**Scoring:**
- **Pass (0.8-1.0)**: Hash chain valid, timestamps monotonic, all required fields present, full decision context
- **Pass (0.6-0.8)**: Hash chain valid, timestamps monotonic, mostly complete
- **Fail (0.4-0.6)**: Hash chain issues, timestamp gaps, missing fields
- **Fail (0.0-0.4)**: Hash chain broken, timestamps non-monotonic, critical fields missing

## Calibration

Each judge can be calibrated on ground truth data to compute:
- **TPR**: True Positive Rate (sensitivity/recall)
- **TNR**: True Negative Rate (specificity)
- **95% Confidence Intervals**: Via percentile bootstrap
- **Optimal Threshold**: Maximizing sensitivity + specificity

### Example Calibration

```go
judge := NewSettlementJudge(JudgeConfig{
    Provider: "ollama",
    Model:    "llama3.1",
})

// Prepare calibration samples
samples := []SettlementJudgeInput{
    // ... 10-20 representative samples ...
}

// Provide ground truth labels
groundTruth := []bool{true, false, true, ...}  // true = pass, false = fail

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

calibration, err := judge.Calibrate(ctx, samples, groundTruth)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Calibration Results:\n")
fmt.Printf("  TPR: %.3f (CI: [%.3f, %.3f])\n", 
    calibration.TPR, calibration.TPRLowerBound, calibration.TPRUpperBound)
fmt.Printf("  TNR: %.3f (CI: [%.3f, %.3f])\n",
    calibration.TNR, calibration.TNRLowerBound, calibration.TNRUpperBound)
fmt.Printf("  Optimal Threshold: %.3f\n", calibration.OptimalThreshold)

// Judge will now use optimal threshold for future evaluations
```

### Calibration Quality Requirements

For production use, judges should achieve:

| Metric | Target | Confidence |
|--------|--------|-----------|
| TPR | ≥0.90 | 95% |
| TNR | ≥0.90 | 95% |
| Optimal Threshold | 0.5-0.8 | - |

## LLM Backend Configuration

The judges support multiple LLM backends:

```go
cfg := JudgeConfig{
    Provider:      "anthropic",  // "ollama" | "openai" | "anthropic"
    BaseURL:       "https://api.anthropic.com",  // Optional, uses provider default
    Model:         "claude-opus-4",  // Model name
    APIKey:        os.Getenv("ANTHROPIC_API_KEY"),  // For auth providers
    Temperature:   0.3,  // 0.0-1.0, default 0.3
    MaxTokens:     2000,  // Response token limit
    TimeoutSeconds: 60,   // Timeout in seconds
}
```

### Supported Providers

| Provider | BaseURL | Model | Notes |
|----------|---------|-------|-------|
| **ollama** | http://localhost:11434 | llama3.1 | Local, no API key needed |
| **openai** | https://api.openai.com | gpt-4 | Requires OPENAI_API_KEY |
| **anthropic** | https://api.anthropic.com | claude-opus-4 | Requires ANTHROPIC_API_KEY |

## Running Tests

Run all judge tests:

```bash
go test ./pkg/eval/judges -v
```

Run specific judge tests:

```bash
go test ./pkg/eval/judges -v -run SettlementJudge
go test ./pkg/eval/judges -v -run ComplianceJudge
go test ./pkg/eval/judges -v -run OrchestrationJudge
go test ./pkg/eval/judges -v -run LineageJudge
```

Run deterministic lineage tests only (no LLM needed):

```bash
go test ./pkg/eval/judges -v -run "TestVerify"
```

## Benchmarking

Benchmark evaluation performance:

```bash
go test ./pkg/eval/judges -bench=Evaluate -benchtime=5s
```

Example results (with Ollama backend):
- Settlement evaluation: ~2-5s per request
- Compliance evaluation: ~2-5s per request
- Orchestration evaluation: ~2-5s per request
- Lineage evaluation: ~1-2s per request (much faster due to deterministic checks)

## Integration with Genie

The judges are designed to integrate with:

1. **pkg/eval**: Main evaluation framework
2. **pkg/commerce**: Settlement and order workflows
3. **pkg/compliance**: KYC/AML checks
4. **pkg/lineage**: Audit trail tracking

### Example Integration

```go
import "github.com/c2siorg/genie/pkg/eval/judges"

func evaluateSettlement(settlement *Settlement) error {
    judge := judges.NewSettlementJudge(config)
    
    input := judges.SettlementJudgeInput{
        OrderID:              settlement.OrderID,
        SettlementID:         settlement.ID,
        SettlementAmount:     settlement.Amount,
        ExpectedAmount:       settlement.Order.Amount,
        CBDCCommitted:        settlement.LedgerCommitted,
        ReconciliationPassed: settlement.Reconciled,
        AuditTrail:           settlement.AuditJSON(),
    }
    
    verdict, err := judge.Evaluate(ctx, input)
    if err != nil {
        return err
    }
    
    if !verdict.Pass {
        log.Printf("Settlement %s failed evaluation: %s", settlement.ID, verdict.Reason)
        return fmt.Errorf("settlement evaluation failed")
    }
    
    return nil
}
```

## References

- Rubrics: `/pkg/eval/rubrics.go`
- Failure Modes: `/pkg/eval/failure_modes.go`
- Architecture: `/docs/architecture.md`
- Compliance: `/docs/ai-governance-security.md`

## Performance & Scalability

### Performance Notes

- LLM-based evaluations add latency (1-5s per request)
- Use deterministic checks when possible (e.g., LineageJudge)
- Batch calibration samples to amortize LLM costs
- Cache calibration results for reuse across deployments

### Scalability Recommendations

1. **For development**: Use local Ollama backend (free, no API costs)
2. **For testing**: Use mock/stub judges returning fixed scores
3. **For production**: Use Claude/GPT-4 with request batching
4. **For high throughput**: Pre-calibrate judges and use optimal threshold without re-evaluation

## License

All judges are licensed under MIT (same as Genie project).
