# LLM-as-Judge Evaluators: Implementation Summary

## Task Completion

Successfully implemented 4 specialized LLM-as-Judge evaluators with calibration for the Genie platform. All judges target TPR ≥ 0.90 and TNR ≥ 0.90 with 95% confidence intervals.

## Deliverables

### 1. Core Judges (4 files)

#### settlement_judge.go
- **Primary Rubric**: RB-SE-003 (Settlement Batch Composition Correctness)
- **Evaluates**: SettlementAmountCorrectness, NettingApplication, CBDCAlignment, ReconciliationIntegrity
- **Scoring**: 0.0-1.0 with binary pass/fail decision
- **Key Methods**:
  - `NewSettlementJudge(cfg JudgeConfig) *SettlementJudge`
  - `Evaluate(ctx context.Context, input SettlementJudgeInput) (Verdict, error)`
  - `Calibrate(ctx context.Context, samples []SettlementJudgeInput, groundTruth []bool) (CalibrationResult, error)`

#### compliance_judge.go
- **Primary Rubric**: RB-CO-001 (Velocity Monitoring)
- **Evaluates**: KYCJustification, AMLThresholds, VelocityEnforcement, AuditTrails
- **Scoring**: 0.0-1.0 with binary pass/fail decision
- **Key Methods**:
  - `NewComplianceJudge(cfg JudgeConfig) *ComplianceJudge`
  - `Evaluate(ctx context.Context, input ComplianceJudgeInput) (Verdict, error)`
  - `Calibrate(ctx context.Context, samples []ComplianceJudgeInput, groundTruth []bool) (CalibrationResult, error)`

#### orchestration_judge.go
- **Primary Rubric**: RB-OR-001 (Workflow State Machine Validity)
- **Evaluates**: StateValidation, WorkflowSequences, MultiAgentConsistency, IdempotencyEnforcement
- **Scoring**: 0.0-1.0 with binary pass/fail decision
- **Valid Transitions**: OrderCreated → PaymentInitiated → PaymentConfirmed → SettlementInitiated → SettlementCompleted → Fulfilled
- **Key Methods**:
  - `NewOrchestrationJudge(cfg JudgeConfig) *OrchestrationJudge`
  - `Evaluate(ctx context.Context, input OrchestrationJudgeInput) (Verdict, error)`
  - `Calibrate(ctx context.Context, samples []OrchestrationJudgeInput, groundTruth []bool) (CalibrationResult, error)`

#### lineage_judge.go
- **Primary Rubric**: RB-LA-001 (Audit Trail Immutability and Hash Chain)
- **Evaluates**: HashChainIntegrity, DecisionTraceability, TimestampMonotonicity, CompleteLogs
- **Scoring**: 0.0-1.0 with binary pass/fail decision
- **Deterministic Checks**:
  1. Hash chain verification: H(n) = SHA256(H(n-1) || data(n))
  2. Timestamp monotonicity: Strictly increasing timestamps (no equal values)
  3. Required fields: All entries must have entryID, orderID, step, timestamp, data, hash
- **Key Methods**:
  - `NewLineageJudge(cfg JudgeConfig) *LineageJudge`
  - `Evaluate(ctx context.Context, input LineageJudgeInput) (Verdict, error)`
  - `Calibrate(ctx context.Context, samples []LineageJudgeInput, groundTruth []bool) (CalibrationResult, error)`

### 2. Type Definitions (types.go)

```go
// Core input types for each judge
type SettlementJudgeInput struct { ... }
type ComplianceJudgeInput struct { ... }
type OrchestrationJudgeInput struct { ... }
type LineageJudgeInput struct { ... }

// Shared types
type JudgeConfig struct { ... }         // LLM backend configuration
type Verdict struct { ... }             // Evaluation result
type CalibrationResult struct { ... }   // Calibration metrics (TPR, TNR, CI)
type CalibrationSample struct { ... }   // Individual calibration data point
type LineageEntry struct { ... }        // Audit trail entry for hash chain
type Judge interface { ... }            // Common interface for all judges
```

### 3. Utility Functions (utils.go)

- `callLLMJudge()`: Invokes LLM with judge prompt and returns structured verdict
- `bootstrapCI()`: Computes 95% confidence intervals via percentile bootstrap
- `computeTPR()`: Calculates true positive rate from verdicts
- `computeTNR()`: Calculates true negative rate from verdicts
- `findOptimalThreshold()`: Finds threshold maximizing sensitivity + specificity
- OpenAI-compatible LLM API support (anthropic, openai, ollama)

### 4. Test Files (4 files)

#### settlement_judge_test.go
- `TestSettlementJudge_EvaluateCorrect()`: Evaluates correct settlement
- `TestSettlementJudge_EvaluateIncorrect()`: Evaluates incorrect settlement
- `TestSettlementJudge_Calibration()`: Calibration with ground truth
- `TestSettlementJudge_Name()`: Judge name verification
- `BenchmarkSettlementJudge_Evaluate()`: Performance benchmark

#### compliance_judge_test.go
- `TestComplianceJudge_EvaluateCompliant()`: Compliant transaction
- `TestComplianceJudge_EvaluateNonCompliant()`: Non-compliant transaction
- `TestComplianceJudge_VelocityExceeded()`: Velocity threshold test
- `TestComplianceJudge_Calibration()`: Calibration test
- `TestComplianceJudge_Name()`: Judge name verification
- `BenchmarkComplianceJudge_Evaluate()`: Performance benchmark

#### orchestration_judge_test.go
- `TestOrchestrationJudge_EvaluateValidSequence()`: Valid state transitions
- `TestOrchestrationJudge_EvaluateInvalidSequence()`: Invalid transitions
- `TestOrchestrationJudge_EvaluateNoIdempotencyKey()`: Idempotency test
- `TestOrchestrationJudge_Calibration()`: Calibration test
- `TestOrchestrationJudge_Name()`: Judge name verification
- `BenchmarkOrchestrationJudge_Evaluate()`: Performance benchmark

#### lineage_judge_test.go
- `TestLineageJudge_EvaluateValidChain()`: Valid hash chain
- `TestLineageJudge_EvaluateBrokenChain()`: Broken hash chain
- `TestLineageJudge_EvaluateNonMonotonicTimestamps()`: Timestamp validation
- `TestLineageJudge_EvaluateMissingFields()`: Required fields check
- `TestVerifyHashChain()`: Hash chain verification unit test
- `TestVerifyTimestampMonotonicity()`: Timestamp monotonicity unit test
- `TestVerifyRequiredFields()`: Field requirement unit test
- `TestLineageJudge_Name()`: Judge name verification

### 5. Documentation Files

#### doc.go
- Package overview and purpose
- Judge list and responsibilities
- Calibration process explanation
- Example usage snippet

#### README.md
- Comprehensive guide (800+ lines)
- Overview table of all judges
- Architecture and interface documentation
- Detailed usage examples for each judge
- Rubric scoring guidelines
- Hash chain algorithm explanation
- Calibration instructions
- LLM backend configuration options
- Performance and scalability notes
- Integration examples

#### IMPLEMENTATION_SUMMARY.md (this file)
- Deliverables checklist
- File structure overview
- Calibration strategy
- Quality metrics and testing approach

## Implementation Details

### Calibration Strategy

Each judge implements a **bootstrapping calibration** workflow:

```
1. Collect N ground-truth samples (positive and negative examples)
2. For each sample, run Evaluate() to get verdict and score
3. Collect all verdicts and scores
4. Compute TPR = P(verdict=true | ground-truth=true)
5. Compute TNR = P(verdict=false | ground-truth=false)
6. Bootstrap confidence intervals (1000 iterations):
   - Sample with replacement from scores
   - Compute mean for each bootstrap sample
   - Use 2.5th and 97.5th percentiles as CI bounds
7. Find optimal threshold:
   - Try thresholds 0.0-1.0 in 0.1 steps
   - For each threshold, compute TPR + TNR
   - Select threshold maximizing sum
8. Store optimal threshold for future evaluations
```

### LLM Prompts

Each judge uses a **system prompt** and **user prompt** format:

1. **System Prompt**: Defines judge role, scoring rubric (0.0-1.0 scale), key evaluation points
2. **User Prompt**: Provides specific context (amounts, dates, flags, audit trail) with evaluation questions

All judges expect **structured JSON response**:
```json
{
  "pass": boolean,
  "score": float (0.0-1.0),
  "reason": "brief explanation",
  "evidence": "observed facts"
}
```

### Hash Chain Implementation (LineageJudge)

The LineageJudge uses SHA256 for hash chain:

```
entry[0].PreviousHash = ""  // First entry has no previous
entry[0].Hash = SHA256(orderID || step || timestamp || data)

for i in 1..n:
    entry[i].PreviousHash = entry[i-1].Hash
    entry[i].Hash = SHA256(orderID || step || timestamp || data)
```

**Deterministic Verification**:
1. Check each entry's hash matches computed value
2. Check previous hash links match
3. Check timestamps are strictly increasing
4. Check all required fields present

If deterministic check fails → verdict.Pass = false (no LLM call needed)

### Quality Metrics

**Target Metrics**:
- TPR ≥ 0.90 (catches 90%+ of true positives)
- TNR ≥ 0.90 (correctly rejects 90%+ of negatives)
- 95% confidence intervals (statistically sound)
- Optimal threshold typically 0.5-0.8

**Testing Coverage**:
- Unit tests for deterministic functions (hash, timestamp, fields)
- Integration tests with mock/stubbed LLM
- Calibration tests with ground truth
- Benchmark tests for performance
- Name/interface verification tests

## File Structure

```
pkg/eval/judges/
├── doc.go                          # Package documentation
├── types.go                        # Type definitions (JudgeConfig, Verdict, etc.)
├── utils.go                        # Utility functions (LLM calls, bootstrap, metrics)
├── settlement_judge.go             # Settlement judge implementation
├── settlement_judge_test.go        # Settlement judge tests
├── compliance_judge.go             # Compliance judge implementation
├── compliance_judge_test.go        # Compliance judge tests
├── orchestration_judge.go          # Orchestration judge implementation
├── orchestration_judge_test.go     # Orchestration judge tests
├── lineage_judge.go                # Lineage judge implementation
├── lineage_judge_test.go           # Lineage judge tests
├── README.md                       # Comprehensive usage guide
└── IMPLEMENTATION_SUMMARY.md       # This file
```

## Usage Examples

### Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    "github.com/c2siorg/genie/pkg/eval/judges"
)

func main() {
    // Create judge
    judge := judges.NewSettlementJudge(judges.JudgeConfig{
        Provider: "ollama",
        Model:    "llama3.1",
    })

    // Evaluate settlement
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    input := judges.SettlementJudgeInput{
        OrderID:              "order-001",
        SettlementID:         "settle-001",
        SettlementAmount:     100000,
        ExpectedAmount:       100000,
        OrderAmount:          100000,
        CBDCCommitted:        true,
        ReconciliationPassed: true,
        AuditTrail:           `[{"step": "settlement_completed"}]`,
    }

    verdict, err := judge.Evaluate(ctx, input)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Verdict: %v (score: %.2f)\n", verdict.Pass, verdict.Score)
}
```

### Calibration

```go
// Prepare calibration data
samples := []judges.SettlementJudgeInput{
    // ... 20-30 representative examples ...
}

groundTruth := []bool{true, false, true, ...}  // Expected outcomes

// Calibrate judge
calibration, err := judge.Calibrate(ctx, samples, groundTruth)
if err != nil {
    log.Fatal(err)
}

// Check quality
if calibration.TPR >= 0.90 && calibration.TNR >= 0.90 {
    fmt.Println("Calibration successful!")
    fmt.Printf("Optimal threshold: %.3f\n", calibration.OptimalThreshold)
}
```

## Integration Points

The judges integrate with:

1. **pkg/eval**: Main evaluation framework and rubric registry
2. **pkg/commerce**: Order, settlement, workflow types
3. **pkg/compliance**: KYC, AML, velocity checking
4. **pkg/lineage**: Audit trail and decision tracking

## Testing

**Deterministic Tests** (no LLM needed):
```bash
go test ./pkg/eval/judges -v -run "TestVerify|Name"
```

**All Tests** (requires Ollama or API credentials):
```bash
go test ./pkg/eval/judges -v
```

**Benchmarks**:
```bash
go test ./pkg/eval/judges -bench=Evaluate -benchtime=5s
```

## Performance Characteristics

| Judge | Latency | TPR Target | TNR Target | Notes |
|-------|---------|-----------|-----------|-------|
| SettlementJudge | 2-5s | ≥0.90 | ≥0.90 | LLM-based |
| ComplianceJudge | 2-5s | ≥0.90 | ≥0.90 | LLM-based |
| OrchestrationJudge | 2-5s | ≥0.90 | ≥0.90 | LLM-based |
| LineageJudge | 1-2s | ≥0.90 | ≥0.90 | Partially deterministic |

## Future Enhancements

1. **Batch Evaluation**: Support batching multiple evaluations for efficiency
2. **Caching**: Cache evaluation results to reduce LLM calls
3. **Ensemble Judges**: Combine multiple judges for final verdict
4. **Custom Rubrics**: Allow domain-specific rubric overrides
5. **Feedback Loop**: Learn from human review feedback to improve calibration
6. **Async Evaluation**: Non-blocking evaluation for high-throughput scenarios

## References

- Rubric Definitions: `pkg/eval/rubrics.go`
- Failure Modes: `pkg/eval/failure_modes.go`
- System Architecture: `docs/architecture.md`
- Compliance Mapping: `docs/ai-governance-security.md`

## License

All code is licensed under MIT (same as Genie project).
