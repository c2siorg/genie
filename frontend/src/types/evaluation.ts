// Evaluation types for trace review, failure mode analysis, and judge calibration.
// Derived from pkg/eval packages and judge implementations.

// Failure Mode Domain
export type FailureModeDomain =
  | "settlement"
  | "compliance"
  | "orchestration"
  | "merchant_customer"
  | "agent_behavior"
  | "lineage_audit";

// Failure Mode Severity
export type FailureModeSeverity = "critical" | "high" | "medium" | "low";

// Failure Mode Category
export type FailureModeCategory = "operational" | "security" | "compliance" | "availability";

// Failure Mode (RB-DOMAIN-NNN structure)
export interface FailureMode {
  id: string; // e.g., "FM-SE-001"
  name: string;
  domain: FailureModeDomain;
  category: FailureModeCategory;
  severity: FailureModeSeverity;
  rbi_concern?: string; // e.g., "RBI-PS-001"
  description: string;
  example_scenario: string;
  root_cause_patterns: string[];
  system_impact: string;
  linked_rubrics: string[]; // RB-DOMAIN-NNN references
  recovery_path: string;
  prevention_control: string;
}

// Interaction Trace (execution record)
export interface InteractionTrace {
  id: string; // trace_id / session_id
  scenario: string; // test scenario name
  success: boolean; // overall pass/fail
  metrics: Record<string, number>; // latency, throughput, cost, etc.
  metadata: Record<string, unknown>; // agent_id, order_id, settlement_amount, etc.
  started_at: string; // RFC3339
  ended_at: string; // RFC3339
  duration_ms?: number; // computed from start/end
}

// Annotation Label
export type AnnotationLabel = "pass" | "fail" | "uncertain";

// Annotation (human evaluation of a trace)
export interface Annotation {
  id: string;
  annotator_id: string; // evaluator email/user
  trace_id: string;
  label: AnnotationLabel;
  primary_failure?: string; // failure mode code, e.g., "FM-SE-001"
  secondary_failures?: string[];
  confidence: number; // [0.0, 1.0]
  notes?: string; // free-form explanation
  rubric_scores?: Record<string, number>; // per-rubric scores
  deferred?: boolean;
  deferred_reason?: string;
  created_at: string; // RFC3339
}

// Judge Type
export type JudgeType = "settlement" | "compliance" | "orchestration" | "lineage";

// Judge Verdict (output of a judge evaluation)
export interface JudgeVerdict {
  judge_type: JudgeType;
  trace_id: string;
  pass: boolean;
  score: number; // confidence [0.0, 1.0]
  reason: string; // explanation
  rubric_id?: string;
  evaluated_at: string; // RFC3339
}

// Calibration Result (judge accuracy metrics)
export interface CalibrationResult {
  judge_type: JudgeType;
  num_positives: number; // ground-truth positive samples
  num_negatives: number; // ground-truth negative samples
  tpr: number; // True Positive Rate (recall)
  tpr_lower_bound: number; // 95% CI lower
  tpr_upper_bound: number; // 95% CI upper
  tnr: number; // True Negative Rate (specificity)
  tnr_lower_bound: number;
  tnr_upper_bound: number;
  optimal_threshold: number;
  calibrated_at: string; // RFC3339
}

// Evaluation Metrics (dashboard summary)
export interface EvaluationMetrics {
  total_traces: number;
  traces_by_domain: Record<FailureModeDomain, number>;
  traces_by_severity: Record<FailureModeSeverity, number>;
  pass_rate: number; // percentage
  failure_rate: number; // percentage
  uncertain_rate: number; // percentage
  judge_metrics: CalibrationResult[]; // one per judge type
  drift_detected: boolean;
  drift_reason?: string;
  most_common_failures: Array<{
    failure_mode: string;
    count: number;
  }>;
  coverage_by_failure_mode: Record<string, number>; // count of test cases
  evaluated_at: string; // RFC3339
  evaluation_latency_ms: number;
}

// Trace with Annotations
export interface TraceWithAnnotations {
  trace: InteractionTrace;
  annotations: Annotation[];
  verdicts: JudgeVerdict[];
  classified_failure_mode?: string; // consensus from annotations
  consensus_confidence?: number;
}

// Failure Mode Trend (for time-series)
export interface FailureModeTrend {
  failure_mode: string; // FM-DOMAIN-NNN
  timestamp: string; // RFC3339
  count: number;
  severity: FailureModeSeverity;
}

// Coverage Status (golden dataset)
export interface CoverageStatus {
  failure_mode: string;
  test_cases_count: number;
  minimum_required: number;
  coverage_percent: number; // (test_cases / minimum) * 100
  status: "ok" | "warning" | "critical"; // ok if >= 100%, warning if 50-100%, critical if < 50%
}

// Rubric Score (evaluation criterion)
export interface RubricScore {
  rubric_id: string; // RB-DOMAIN-NNN
  name: string;
  score: number; // [0, 100]
  passed: boolean;
  reason?: string;
}
