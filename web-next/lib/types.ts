// Typed API contract for the Genie console — mirrors the Go wire types the
// backend serves. Keep field names in sync with the JSON tags in:
//   pkg/web/handlers/ask.go        (askRequest / askResponse)
//   pkg/erupeecompliance/types.go  (ComplianceCheck)
//   pkg/eval/judges/types.go       (Verdict)
// The spec-validation CI checks pin this mirroring (scripts/validate-specs.sh).

// AskRequest is the POST /v1/ask body (pkg/web/handlers/ask.go askRequest).
export interface AskRequest {
  question: string;
  document_id: string;
}

// AskResponse is the POST /v1/ask reply (pkg/web/handlers/ask.go askResponse).
export interface AskResponse {
  trace_id: string;
  report: string;
  ai_disclosure?: string;
}

// ComplianceCheck mirrors pkg/erupeecompliance/types.go ComplianceCheck —
// the complete compliance assessment for a payment.
export interface ComplianceCheck {
  payment_id: string;
  aml_result: "pass" | "review" | "block";
  aml_reason: string;
  velocity_result: "ok" | "warning" | "blocked";
  velocity_reason: string;
  fraud_score: number;
  detected_patterns: string[];
  fraud_reason: string;
  decision: "allow" | "review" | "block";
  decision_reason: string;
  checked_at: string;
}

// JudgeVerdict mirrors pkg/eval/judges/types.go Verdict — a judge's decision
// on an evaluation subject.
export interface JudgeVerdict {
  pass: boolean;
  score: number;
  rubric_id: string;
  rubric_name: string;
  reason: string;
  evidence: string;
  evaluated_at: string;
}
