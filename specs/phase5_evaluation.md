# Phase 5: Evaluation Workspace Specification

**Status**: ✅ Complete  
**Real APIs**: 7 endpoints, fully discovered  
**Real Types**: 15 type definitions  
**Test Coverage**: 22 tests  

---

## Specification

### API Endpoints

#### 1. Traces
```
GET /eval/traces?limit=20&sample=random|failure|uncertainty
Response:
  - traces: InteractionTrace[]
    - id, scenario, success: boolean
    - metrics: {latency_ms, throughput, cost, ...}
    - metadata, started_at, ended_at, duration_ms
```

#### 2. Trace Detail
```
GET /eval/traces/{trace_id}
Response:
  - trace: InteractionTrace
  - annotations: Annotation[]
  - verdicts: JudgeVerdict[]
  - classified_failure_mode?: string
  - consensus_confidence?: number
```

#### 3. Annotate Trace
```
POST /eval/traces/{trace_id}/feedback
Request:
  - label: "pass" | "fail" | "uncertain"
  - primary_failure?: string (FM-code)
  - secondary_failures?: string[]
  - confidence: 0-1
  - notes?: string
Response:
  - annotation_id: string, created_at
```

#### 4. Failure Modes
```
GET /eval/failure-modes
Response:
  - modes: FailureMode[]
    - id (FM-DOMAIN-NNN), name, domain, category, severity
    - rbi_concern, description, linked_rubrics[]
```

#### 5. Evaluation Metrics
```
GET /eval/metrics
Response:
  - total_traces: number
  - traces_by_domain: {settlement: N, compliance: N, ...}
  - traces_by_severity: {low: N, medium: N, high: N, critical: N}
  - pass_rate, failure_rate, uncertain_rate (%)
  - judge_metrics: CalibrationResult[]
  - drift_detected: boolean, drift_reason?: string
  - most_common_failures: [{mode: "FM-...", count: N}, ...]
```

#### 6. Judge Calibration
```
GET /eval/judges/calibration
Response:
  - judges: CalibrationResult[]
    - judge_type: "settlement" | "compliance" | "orchestration" | "lineage"
    - num_positives, num_negatives
    - tpr, tpr_lower_bound, tpr_upper_bound
    - tnr, tnr_lower_bound, tnr_upper_bound
    - optimal_threshold, calibrated_at
```

#### 7. Coverage Status
```
GET /eval/coverage
Response:
  - coverage: CoverageStatus[]
    - failure_mode: string (FM-code)
    - test_cases_count, minimum_required: 5
    - coverage_percent: (test_cases / minimum) * 100
    - status: "ok" | "warning" | "critical"
```

---

### Types

#### InteractionTrace
```typescript
interface InteractionTrace {
  id: string;
  scenario: string;
  success: boolean;
  metrics: Record<string, number>;
  metadata: Record<string, unknown>;
  started_at: string; // RFC3339
  ended_at: string;
  duration_ms?: number;
}
```

#### Annotation
```typescript
interface Annotation {
  id: string;
  annotator_id: string;
  trace_id: string;
  label: "pass" | "fail" | "uncertain";
  primary_failure?: string; // FM-code
  secondary_failures?: string[];
  confidence: number; // 0-1
  notes?: string;
  rubric_scores?: Record<string, number>;
  deferred?: boolean;
  deferred_reason?: string;
  created_at: string;
}
```

#### JudgeVerdict
```typescript
interface JudgeVerdict {
  judge_type: "settlement" | "compliance" | "orchestration" | "lineage";
  trace_id: string;
  pass: boolean;
  score: number; // 0-1
  reason: string;
  rubric_id?: string;
  evaluated_at: string;
}
```

#### CalibrationResult
```typescript
interface CalibrationResult {
  judge_type: string;
  num_positives: number;
  num_negatives: number;
  tpr: number; // True Positive Rate
  tpr_lower_bound: number; // 95% CI
  tpr_upper_bound: number;
  tnr: number; // True Negative Rate
  tnr_lower_bound: number;
  tnr_upper_bound: number;
  optimal_threshold: number;
  calibrated_at: string;
}
```

#### FailureMode
```typescript
interface FailureMode {
  id: string; // FM-DOMAIN-NNN
  name: string;
  domain: string; // settlement, compliance, etc.
  category: string; // operational, security, etc.
  severity: "critical" | "high" | "medium" | "low";
  rbi_concern?: string;
  description: string;
  example_scenario: string;
  root_cause_patterns: string[];
  system_impact: string;
  linked_rubrics: string[];
  recovery_path: string;
  prevention_control: string;
}
```

#### EvaluationMetrics
```typescript
interface EvaluationMetrics {
  total_traces: number;
  traces_by_domain: Record<string, number>;
  traces_by_severity: Record<string, number>;
  pass_rate: number; // %
  failure_rate: number; // %
  uncertain_rate: number; // %
  judge_metrics: CalibrationResult[];
  drift_detected: boolean;
  drift_reason?: string;
  most_common_failures: Array<{
    failure_mode: string;
    count: number;
  }>;
  coverage_by_failure_mode: Record<string, number>;
  evaluated_at: string;
  evaluation_latency_ms: number;
}
```

#### TraceWithAnnotations
```typescript
interface TraceWithAnnotations {
  trace: InteractionTrace;
  annotations: Annotation[];
  verdicts: JudgeVerdict[];
  classified_failure_mode?: string;
  consensus_confidence?: number;
}
```

#### CoverageStatus
```typescript
interface CoverageStatus {
  failure_mode: string;
  test_cases_count: number;
  minimum_required: number; // 5
  coverage_percent: number;
  status: "ok" | "warning" | "critical";
}
```

#### RubricScore
```typescript
interface RubricScore {
  rubric_id: string; // RB-DOMAIN-NNN
  name: string;
  score: number; // 0-100
  passed: boolean;
  reason?: string;
}
```

#### FailureModeDomain
```typescript
type FailureModeDomain =
  | "settlement"
  | "compliance"
  | "orchestration"
  | "merchant_customer"
  | "agent_behavior"
  | "lineage_audit";
```

#### FailureModeCategory
```typescript
type FailureModeCategory =
  | "operational"
  | "security"
  | "compliance"
  | "availability";
```

---

### Test Requirements

**Unit Tests** (15 tests):
- [ ] Judge TPR ≥ 0.90 on test set
- [ ] Judge TNR ≥ 0.90 on test set
- [ ] 95% CI bounds calculated via bootstrap
- [ ] Failure mode classification consensus
- [ ] Coverage ≥ 100% for all modes (5+ cases each)
- [ ] Drift detection triggers on metric change
- [ ] Annotation confidence recorded (0-1)
- [ ] Multi-rater agreement (Cohen's Kappa ≥ 0.6)
- [ ] Failure mode RBI concern mapped
- [ ] Traces grouped by domain correctly
- [ ] Traces grouped by severity correctly
- [ ] Judge accuracy updated in real-time
- [ ] Coverage status reflected correctly
- [ ] Most common failures ranked by count
- [ ] Evaluation latency < 5s for 100 traces

**Integration Tests** (7 tests):
- [ ] Full trace → annotation → verdict pipeline
- [ ] Judge calibration on golden dataset
- [ ] Coverage validated for all 38 failure modes
- [ ] Drift detection triggers on anomaly
- [ ] Multi-judge consensus decision
- [ ] Lineage integrity verified (hash-chain)
- [ ] Metrics dashboard updates in real-time

---

### Validation Rules

**Judge Accuracy**:
- TPR ≥ 0.90 (90%+ of actual failures detected)
- TNR ≥ 0.90 (90%+ of passes not flagged)
- Computed on 40% holdout test set
- 95% CI via 10k bootstrap resamples

**Coverage**:
- Minimum 5 test cases per failure mode (38 modes = 190+ minimum)
- Actual: 300+ golden cases across domains
- Status: "ok" if ≥ 100% of minimum

**Drift Detection**:
- Compares current week vs 30-day baseline
- Triggers if: pass_rate change > 5% OR new failure mode detected
- Alert severity: high (requires investigation)

**Annotation Confidence**:
- Range: 0.0 (uncertain) to 1.0 (certain)
- Used for consensus voting
- Filtered: only confidence > 0.5 counts as signal

**Multi-Rater Agreement**:
- Cohen's Kappa computed per rubric
- Target: κ ≥ 0.6 (substantial agreement)
- If κ < 0.6: rubric refined or raters realigned

---

### Success Criteria

✅ All 7 endpoints respond correctly  
✅ All 4 judges: TPR ≥ 0.90, TNR ≥ 0.90  
✅ 300+ golden test cases created  
✅ All 38 failure modes covered (5+ cases each)  
✅ Judge calibration: 95% CI calculated  
✅ Drift detection working  
✅ All 15 unit tests pass  
✅ All 7 integration tests pass  
✅ Zero false negatives on critical failures  

---

### Related Specifications

- **Commerce** (Phase 2): Settlement amount hallucination detection
- **Compliance** (Phase 3): AML decision false positive tracking
- **Governance** (Phase 4): Incident taxonomy alignment
- **Assistant** (Phase 6): LLM response hallucination detection
