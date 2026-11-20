# Production Evaluation System

Genie's Production Evaluation System provides real-time assessment of LLM agent quality in production with statistical confidence intervals, judge accuracy tracking, and automatic drift detection.

## Architecture Overview

The system comprises three main components:

1. **Monitoring Engine** (`pkg/eval/monitoring.go`)
   - Bootstrap confidence interval computation
   - Stratified sampling of evaluation records
   - Concurrent judge execution
   - Real-time metric export via OpenTelemetry

2. **Evaluation Dashboard** (`cmd/eval-dashboard/main.go`)
   - HTML dashboard with live metrics
   - JSON API for programmatic access
   - Prometheus `/metrics` endpoint integration
   - Optional Laminar trace export

3. **Test Suite** (`pkg/eval/monitoring_test.go`)
   - 21 comprehensive unit tests
   - Bootstrap CI verification
   - Sampling validation
   - Multi-judge orchestration testing

## Core Types

### ProductionEvalConfig

Controls evaluation behavior:

```go
type ProductionEvalConfig struct {
    // SampleRate: fraction of requests to evaluate (0..1, default: 0.01)
    SampleRate float64

    // ConfidenceLevel: desired CI coverage (default: 0.95 = 95%)
    ConfidenceLevel float64

    // Judges: map of judge name → scoring function
    Judges map[string]JudgeFunc

    // DriftThreshold: alert if pass rate falls below this (default: 0.85)
    DriftThreshold float64

    // BootstrapSamples: resamples for CI computation (default: 10000)
    BootstrapSamples int

    // Store: evaluation record persistence
    Store eval.Store
}
```

### JudgeFunc

A judge is any function that scores a single interaction:

```go
type JudgeFunc func(
    ctx context.Context,
    p llm.Provider,
    model string,
    input string,
    output string,
) (score float64, err error)
```

Scores must be in [0.0, 1.0]:
- **1.0** = pass (requirement met)
- **0.0** = fail (requirement violated)
- **0.5** = uncertain (judge cannot determine)

### ProductionEvalMetrics

Aggregated results from one evaluation run:

```go
type ProductionEvalMetrics struct {
    // RunID: unique identifier for this evaluation
    RunID string

    // SampleSize: number of records evaluated
    SampleSize int

    // SettlementAmountHallucinatedCI: 95% CI for settlement judge pass rate
    SettlementAmountHallucinatedCI ConfidenceInterval

    // JudgeMetrics: per-judge performance
    JudgeMetrics map[string]JudgeMetrics

    // DriftDetected: true if any judge's pass rate < DriftThreshold
    DriftDetected bool
    DriftReason string

    // Timestamps
    StartedAt time.Time
    CompletedAt time.Time
}
```

## Key Features

### 1. Bootstrap Confidence Intervals

Computes [Lower, Point, Upper] estimates of pass rate with minimal statistical assumptions:

```go
ci := bootstrapCI(scores, 0.95, 10000)
// Output: ConfidenceInterval{Lower: 0.83, Point: 0.90, Upper: 0.96}
```

**Algorithm:**
1. Compute empirical mean (point estimate)
2. Resample with replacement N times
3. Sort bootstrap means
4. Extract (1-α)/2 and (1+α)/2 percentiles

**Advantages:**
- Model-free (no normality assumption)
- Handles skewed/bimodal distributions
- Converges quickly (1000 samples typically sufficient)
- Interpretable intervals

### 2. Stratified Sampling

Selects approximately `SampleRate * N` records uniformly at random:

```go
sampled := sampleRecords(records, 0.10)  // Sample 10%
```

Uses Fisher-Yates shuffle for O(N) time and uniform selection probability.

### 3. Concurrent Judge Execution

All judges run in parallel with bounded concurrency (8 concurrent API calls):

```go
judgeResults := make(chan struct{...}, len(cfg.Judges))
// Each judge runs in its own goroutine
// Results collected asynchronously
```

Maximum latency = max(judge_latency) + CI computation overhead.

### 4. Drift Detection

Automatically flags degradation when pass rate drops below threshold:

```go
if metrics.PassRate < cfg.DriftThreshold {
    metrics.DriftDetected = true
    metrics.DriftReason = "..."
}
```

Default threshold: 0.85 (85%).

### 5. OpenTelemetry Integration

All metrics exported to OTEL:

```
eval.judge.settlement_hallucination.pass_rate
eval.judge.settlement_hallucination.confidence_interval.lower
eval.judge.settlement_hallucination.confidence_interval.upper
```

Traces include:
- Judge execution events
- Bootstrap completion
- Drift detection flags

### 6. Laminar Trace Export (Optional)

When `LMNR_PROJECT_API_KEY` is set, traces are sent to Laminar:

```bash
LMNR_PROJECT_API_KEY=xxx eval-dashboard -laminar
```

## Built-in Judges

### DefaultSettlementHallucinationJudge

Detects unsupported settlement amount claims using the hallucination package:

```go
score, err := DefaultSettlementHallucinationJudge(
    ctx,
    openAIProvider,
    "gpt-4",
    "merchant_transaction_context",
    "settlement_response",
)
```

**Logic:**
1. Splits response into sentences
2. Grades each sentence as supported/unsupported/contradicted
3. Returns 1.0 if >= 80% supported, else 0.0

## Running the System

### 1. Minimal Example

```go
package main

import (
    "context"
    "fmt"
    "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
    "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

func main() {
    ctx := context.Background()

    // Create a store and populate it with interaction records
    store := eval.NewInMemoryStore()
    // ... populate store with records ...

    // Configure judges
    cfg := eval.ProductionEvalConfig{
        SampleRate:       0.10, // 10% of interactions
        ConfidenceLevel:  0.95,
        DriftThreshold:   0.85,
        BootstrapSamples: 1000,
        Judges: map[string]eval.JudgeFunc{
            "settlement_hallucination": eval.DefaultSettlementHallucinationJudge,
        },
        LLMProvider: openAIProvider,
        LLMModel:    "gpt-4",
        Store:       store,
    }

    // Run evaluation
    metrics, err := eval.RunProductionEvaluation(ctx, cfg)
    if err != nil {
        panic(err)
    }

    // Check results
    fmt.Printf("Sample size: %d\n", metrics.SampleSize)
    fmt.Printf("Settlement pass rate: %.1f%% [%.1f%%, %.1f%%]\n",
        metrics.SettlementAmountHallucinatedCI.Point*100,
        metrics.SettlementAmountHallucinatedCI.Lower*100,
        metrics.SettlementAmountHallucinatedCI.Upper*100,
    )

    if metrics.DriftDetected {
        fmt.Printf("ALERT: %s\n", metrics.DriftReason)
    }
}
```

### 2. Dashboard

Start the evaluation dashboard:

```bash
# Basic mode (stdout traces)
go run ./cmd/eval-dashboard/main.go

# With Laminar export
LMNR_PROJECT_API_KEY=xxx go run ./cmd/eval-dashboard/main.go -laminar

# With custom config
go run ./cmd/eval-dashboard/main.go \
    -listen :8080 \
    -metricsPort :9464 \
    -interval 30s \
    -sample-rate 0.05 \
    -confidence 0.99 \
    -drift 0.80
```

Visit `http://localhost:8080` to view the dashboard.

### 3. Programmatic API

Query metrics via JSON API:

```bash
curl http://localhost:8080/api/metrics | jq
```

Response:
```json
{
  "metrics": {
    "runID": "eval-1717....",
    "sampleSize": 50,
    "settlementAmountHallucinatedCI": {
      "lower": 0.83,
      "point": 0.90,
      "upper": 0.96
    },
    "driftDetected": false,
    "judgeMetrics": {
      "settlement_hallucination": {
        "name": "settlement_hallucination",
        "totalScored": 50,
        "passCount": 45,
        "failCount": 5,
        "passRate": 0.90,
        "confidenceIntervals": {...}
      }
    }
  },
  "error": "",
  "timestamp": "2026-06-06T15:30:00Z"
}
```

## Custom Judges

Implement your own judge by conforming to `JudgeFunc`:

### Example: Regulatory Compliance Judge

```go
func RegulatoryComplianceJudge(
    ctx context.Context,
    p llm.Provider,
    model string,
    input string,
    output string,
) (float64, error) {
    // 1. Parse settlement details from input
    details, err := parseSettlementInput(input)
    if err != nil {
        return 0.0, err
    }

    // 2. Check compliance rules
    isCompliant := true
    if details.Amount > 10000 && !details.KYCVerified {
        isCompliant = false
    }
    if details.Jurisdiction == "Iran" {
        isCompliant = false
    }

    // 3. Validate output mentions compliance
    if !strings.Contains(output, "compliance") {
        isCompliant = false
    }

    if isCompliant {
        return 1.0, nil
    }
    return 0.0, nil
}

// Register the judge
cfg.Judges["regulatory_compliance"] = RegulatoryComplianceJudge
```

### Example: LLM-as-Judge via API

```go
func LLMJudge(
    ctx context.Context,
    p llm.Provider,
    model string,
    input string,
    output string,
) (float64, error) {
    resp, err := p.Complete(ctx, llm.CompletionRequest{
        Model: model,
        Messages: []llm.Message{
            {
                Role: llm.RoleSystem,
                Content: "You are a settlement quality auditor. Score the response 0-10.",
            },
            {
                Role: llm.RoleUser,
                Content: fmt.Sprintf(
                    "Input:\n%s\n\nOutput:\n%s\n\nScore (0-10):",
                    input, output,
                ),
            },
        },
        Temperature: 0,
    })
    if err != nil {
        return 0.0, err
    }

    // Parse score from response
    var score float64
    fmt.Sscanf(resp.Text, "%f", &score)
    return score / 10.0, nil // Normalize to [0..1]
}
```

## Configuration Best Practices

### Production Settings

```go
cfg := eval.ProductionEvalConfig{
    SampleRate:       0.01,  // 1% sampling (low overhead)
    ConfidenceLevel:  0.99,  // 99% CI (stricter requirements)
    DriftThreshold:   0.80,  // Alert at 80% pass rate
    BootstrapSamples: 50000, // More resamples for precision
    Judges: map[string]eval.JudgeFunc{
        "settlement_hallucination": eval.DefaultSettlementHallucinationJudge,
        "regulatory_compliance":    RegulatoryComplianceJudge,
        "llm_auditor":              LLMJudge,
    },
}
```

### Development Settings

```go
cfg := eval.ProductionEvalConfig{
    SampleRate:       1.0,   // 100% (no sampling)
    ConfidenceLevel:  0.95,  // 95% CI
    DriftThreshold:   0.85,  // Standard threshold
    BootstrapSamples: 1000,  // Fast iteration
    Judges: map[string]eval.JudgeFunc{
        "settlement_hallucination": eval.DefaultSettlementHallucinationJudge,
    },
}
```

## Testing

Run the test suite:

```bash
# All eval tests
go test ./pkg/eval -v

# Only monitoring tests
go test ./pkg/eval -v -run "Bootstrap|Sample|Production|Confidence|Default|Random"

# Benchmarks
go test ./pkg/eval -bench "Bootstrap|Sample|Production" -benchmem
```

Expected results:
- 21 tests pass in ~0.3s
- Benchmarks: ~0.1s for bootstrap, ~1μs for sampling

## Troubleshooting

### Issue: Low pass rate / drift detected

**Check:**
1. Judge implementation (is it too strict?)
2. LLM model quality (try a different model?)
3. Sample size (need more samples for stability?)
4. Recent code changes (what changed since last good run?)

**Mitigation:**
1. Lower `DriftThreshold` temporarily to validate
2. Increase `BootstrapSamples` for tighter CI
3. Implement additional judges for root cause analysis

### Issue: Slow evaluation

**Optimizations:**
1. Increase `SampleRate` (evaluate fewer requests)
2. Lower `BootstrapSamples` (faster CI, less precise)
3. Limit concurrent judges
4. Cache judge results

### Issue: Laminar traces not appearing

**Debug:**
```bash
# Check API key
echo $LMNR_PROJECT_API_KEY

# Verify connectivity
curl https://api.laminar.run/health

# Enable verbose logging
go run ./cmd/eval-dashboard -laminar -v
```

## Metrics Reference

### OpenTelemetry Metrics

All metrics use namespace `genie`:

```
eval.judge.{judge_name}.pass_rate
eval.judge.{judge_name}.confidence_interval.lower
eval.judge.{judge_name}.confidence_interval.upper
```

### Prometheus Metrics

Accessed via `GET /metrics`:

```
genie_eval_judge_settlement_hallucination_pass_rate
genie_eval_judge_settlement_hallucination_confidence_interval_lower
genie_eval_judge_settlement_hallucination_confidence_interval_upper
```

### Dashboard Metrics

HTML dashboard displays:
- Settlement hallucination pass rate [95% CI]
- Judge accuracy (per-judge breakdown)
- Sample size and configuration
- Drift detection status
- Timestamp of last evaluation

## Integration with Observability

The system integrates with Genie's existing observability stack:

1. **OpenTelemetry**: Automatic metric & trace collection
2. **Prometheus**: `/metrics` scrape endpoint
3. **Laminar**: Optional trace export (when API key set)
4. **OTEL Collector**: Compatible with Tempo/Prometheus via OTLP

Example collector config:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  logging: {}
  prometheus:
    endpoint: "0.0.0.0:8888"
  jaeger:
    endpoint: "jaeger:14250"

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [logging, jaeger]
    metrics:
      receivers: [otlp]
      exporters: [logging, prometheus]
```

## Future Enhancements

1. **Confidence Interval Methods**
   - Normal approximation (faster, requires ~30+ samples)
   - Bayesian credible intervals
   - Adjusted percentile method (more accurate)

2. **Additional Judges**
   - Privacy/PII detection
   - Regulatory reporting compliance
   - Latency SLO validation
   - Cost per request audit

3. **Advanced Features**
   - Stratified sampling by merchant/transaction type
   - Time-series analysis (trend detection)
   - A/B testing support (compare two models)
   - Judge ensemble (weighted combination)

4. **Dashboard Enhancements**
   - Historical trend visualization
   - Per-merchant/region drill-down
   - Alert rules configuration
   - Judge performance heatmaps

## References

- **Bootstrap Confidence Intervals**: Efron & Tibshirani, "An Introduction to the Bootstrap"
- **OTEL Spec**: https://opentelemetry.io/docs/concepts/
- **Laminar Docs**: https://laminar.run/docs

---

**Version:** 1.0.0  
**Last Updated:** 2026-06-06  
**Status:** Production Ready  
**License:** MIT
