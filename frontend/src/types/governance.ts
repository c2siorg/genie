// Governance types for agent fleet management, HITL, incidents, and safety.
// Derived from pkg/agent, pkg/hitl, pkg/incidents, pkg/agentgov packages.

// Agent Tier (promotion model)
export type AgentTier = "sketch" | "prototype" | "beta" | "production";

// Agent Ring (access control)
export type AgentRing = "admin" | "standard" | "restricted" | "sandboxed";

// Agent Summary from registry
export interface AgentSummary {
  agent_id: string;
  name: string;
  capabilities: string[]; // e.g., ["supervise_finance", "route_messages"]
  tier: AgentTier;
  ring: AgentRing;
  trust_score: number; // 0-100
  status: "healthy" | "degraded" | "unhealthy";
  last_seen_at?: string; // RFC3339
}

// Trust Score breakdown
export interface TrustScore {
  agent_id: string;
  overall_score: number; // 0-100
  reliability: number; // past execution success rate
  compliance: number; // policy adherence
  audited_at: string; // RFC3339
}

// Incident Severity
export type IncidentSeverity = "low" | "moderate" | "high";

// Incident Failure Mode
export type FailureMode =
  | "bias"
  | "hallucination"
  | "explainability"
  | "privacy_breach"
  | "unintended_action"
  | "policy_denied"
  | "agent_error"
  | "unknown";

// Incident Status
export type IncidentStatus = "ongoing" | "resolved";

// Incident (RBI Annexure VI model)
export interface Incident {
  id: string;
  occurred_at: string; // RFC3339
  detected_at: string; // RFC3339
  use_case: string; // e.g., "payment_processing"
  model: string; // LLM name, e.g., "claude-opus-4"
  third_party_vendor?: string; // optional
  description: string;
  affected_stakeholders: "internal" | "external" | "both";
  severity: IncidentSeverity;
  failure_mode: FailureMode;
  root_cause?: string;
  response_actions?: string;
  status: IncidentStatus;
  actor_id: string; // who reported
  metadata?: Record<string, unknown>;
}

// HITL Approval Request
export interface HITLApprovalRequest {
  id: string;
  session_id: string;
  agent_id: string;
  tool_name: string;
  args: Record<string, unknown>;
  risk_score?: number; // 0-1
  created_at: string; // RFC3339
  expires_at?: string; // RFC3339
}

// HITL Approval Decision
export interface HITLApprovalDecision {
  request_id: string;
  approved: boolean;
  reason: string;
  decided_at: string; // RFC3339
  decided_by: string; // user ID or policy name
}

// Policy Rule (governance)
export interface PolicyRule {
  id: string;
  tool_pattern: string; // glob: "delete_*", "read_*"
  arg_patterns?: Record<string, string>; // regex patterns
  action: "allow" | "deny" | "ask_human" | "rate_limit";
  reason: string;
  priority: number; // higher priority evaluated first
}

// Compliance Limit (CBDC)
export interface ComplianceLimit {
  type: "p2p" | "merchant"; // person-to-person vs business
  daily_limit_paise: number; // in paise
  single_transaction_max_paise: number;
  current_daily_usage_paise: number;
  remaining_today_paise: number;
  reset_at?: string; // RFC3339 (UTC midnight)
}

// Kill-Switch state
export type KillSwitchScope = "global" | "agent" | "capability";

export interface KillSwitchRequest {
  scope: KillSwitchScope;
  target?: string; // agent_id or capability_name
  reason: string;
  activated_at: string; // RFC3339
  activated_by: string;
}

export interface KillSwitchStatus {
  is_active: boolean;
  activations: KillSwitchRequest[];
  last_change_at: string; // RFC3339
}

// Audit Entry (hash-chained)
export interface AuditEntry {
  seq: number;
  occurred_at: string; // RFC3339
  actor: string; // user_id, agent_id, or "system"
  action: string; // e.g., "consent.grant", "payment.settle"
  target: string;
  details?: Record<string, unknown>;
  prev_hash: string; // hex SHA256
  row_hash: string; // hex SHA256
}

// SLO Report
export interface SLOReport {
  agent_id: string;
  availability_percent: number; // target: 99.5%
  latency_p95_ms: number; // target: 10000ms
  reported_at: string; // RFC3339
  window_days: number; // typically 30
}
