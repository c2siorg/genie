// Compliance types derived from pkg/erupeecompliance, pkg/kyc, and HITL endpoints.
// Covers KYC onboarding, payment compliance, AML, velocity, fraud detection.

// KYC Onboarding States
export type OnboardingState =
  | "pending"
  | "documents_uploaded"
  | "identity_verified"
  | "sanctions_checked"
  | "approval_pending"
  | "approved"
  | "flagged"
  | "rejected";

// KYC Application (customer onboarding flow)
export interface KYCApplication {
  application_id: string;
  customer_id: string;
  state: OnboardingState;
  pan: string;
  aadhaar: string;
  address: string;
  occupation: string;
  jurisdiction: string;
  pep_flagged: boolean;
  sanctions_flagged: boolean;
  risk_score: number; // 0-1
  approval_reason?: string;
  rejection_reason?: string;
  created_at: string; // RFC3339
  updated_at: string;
}

// AML Result (from ComplianceEngine.CheckPayment)
export type AMLResult = "pass" | "review" | "block";

// Velocity Result
export type VelocityResult = "ok" | "warning" | "blocked";

// Fraud Patterns detected
export type FraudPattern =
  | "structuring"
  | "round_tripping"
  | "velocity_spike"
  | "new_account_high";

// Compliance Decision (final outcome of a payment check)
export type ComplianceDecision = "allow" | "review" | "block";

// Compliance Check (result of POST /v1/compliance/check)
export interface ComplianceCheck {
  check_id: string;
  payment_id: string;
  aml_result: AMLResult;
  aml_reason?: string;
  velocity_result: VelocityResult;
  velocity_reason?: string;
  fraud_score: number; // 0-100
  detected_patterns: FraudPattern[];
  fraud_reason?: string;
  decision: ComplianceDecision;
  decision_reason: string;
  checked_at: string; // RFC3339
}

// Velocity Metrics (from GET /v1/compliance/account/{id}/velocity)
export interface VelocityMetrics {
  account_id: string;
  txn_count_hourly: number;
  total_amount_hourly: number; // paise
  daily_total: number; // paise
  max_txn_count_per_hour: number;
  max_amount_per_hour: number; // paise
  max_amount_per_day: number; // paise
  status: VelocityResult;
  last_transaction_at?: string;
}

// Fraud History (from GET /v1/compliance/account/{id}/fraud-history)
export interface FraudHistory {
  account_id: string;
  patterns: FraudPattern[];
  fraud_score: number; // 0-100
  risk_level: "low" | "medium" | "high" | "critical";
  recent_alerts: Array<{
    timestamp: string;
    pattern: FraudPattern;
    evidence: string;
  }>;
}

// AML Risk Score (from POST /v1/aml/score)
export interface AMLRiskScore {
  score_id: string;
  user_id: string;
  amount_paise: number;
  beneficiary_id: string;
  beneficiary_country?: string;
  score: number; // 0-100
  level: "low" | "medium" | "high" | "critical";
  triggered_rules: string[];
  evidence: Record<string, unknown>;
  scored_at: string; // RFC3339
}

// HITL Approval Request (from GET /v1/hitl/approvals)
export interface HITLApprovalRequest {
  id: string;
  request_type: "kyc" | "payment" | "str"; // Type of approval being requested
  entity_id: string; // application_id, payment_id, or str_id
  reason: string; // Why human review is needed
  created_at: string; // RFC3339
  risk_factors?: string[]; // e.g., "aml_review", "fraud_score_high", "new_account"
  details?: Record<string, unknown>; // Full context (KYCApplication, ComplianceCheck, etc.)
}

// HITL Decision (POST to /v1/hitl/approvals/{id}/approve or deny)
export interface HITLDecision {
  id: string;
  status: "approved" | "denied";
  reason: string;
  decided_by: string; // User ID of approver
  decided_at: string; // RFC3339
}
