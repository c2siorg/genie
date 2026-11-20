// Package hitl implements Human-in-the-Loop (HITL) approval flows for
// AI agents. It provides a language-idiomatic Go port of the runtime
// approval pattern described in lesson 09-HITL.
//
// ─── What this solves ──────────────────────────────────────────────────────
//
// Agents acting autonomously can take irreversible actions: deleting files,
// sending messages, calling paid APIs, executing shell commands. HITL inserts
// a human checkpoint before those actions execute, giving users:
//
//   - Trust     — the agent cannot act without explicit consent
//   - Audit     — every decision is logged with who decided and why
//   - Reversal  — rejecting an approval prevents the action entirely
//
// ─── Approval modes ────────────────────────────────────────────────────────
//
// Synchronous (CLIApprover):
//
//	Agent loop blocks on stdin. Best for CLI tools and local development.
//	No persistence needed; the process stays alive.
//
// Async (AsyncApprover + InMemoryStore):
//
//	Agent loop parks the request in a store and blocks on a channel.
//	An HTTP handler delivers the human decision, unblocking the channel.
//	Best for web deployments and multi-user systems.
//
// Policy-only (PolicyApprover):
//
//	Rules evaluate tool name + args and return Allow / Deny / AskHuman.
//	When AskHuman fires, an inner Approver (sync or async) is called.
//	Best for tiered risk: auto-approve safe ops, escalate risky ones.
//
// ─── Executor integration ──────────────────────────────────────────────────
//
// Set Executor.Approver before calling Run. The agent loop calls
// approver.RequestApproval before executing each tool call, and aborts
// the whole step if approval is denied — mirroring the TypeScript reference:
//
//	for _, tc := range toolCalls {
//	    approved, err := approver.RequestApproval(ctx, req)
//	    if !approved { break }
//	    result := executeTool(tc)
//	}
//
// ─── FREE-AI alignment ─────────────────────────────────────────────────────
//
// Rec 16 (Human oversight of AI decisions) — HITL is the runtime embodiment.
// Rec 22 (Tamper-evident audit) — every approval decision carries a timestamp,
// decider identity, and request ID for audit-log insertion.
package hitl

import (
	"context"
	"time"
)

// ─── Core types ────────────────────────────────────────────────────────────

// ApprovalRequest is sent to an Approver when the agent wants to execute a tool.
type ApprovalRequest struct {
	// ID is a unique identifier for this request (UUID or trace-id derived).
	ID string `json:"id"`
	// SessionID groups requests from the same agent run.
	SessionID string `json:"session_id,omitempty"`
	// AgentID identifies the agent making the request.
	AgentID string `json:"agent_id,omitempty"`
	// ToolName is the tool the agent wants to call.
	ToolName string `json:"tool_name"`
	// Args are the arguments the agent wants to pass to the tool.
	Args map[string]any `json:"args"`
	// RiskScore is an optional 0–1 score from a RiskScorer.
	RiskScore float64 `json:"risk_score,omitempty"`
	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is the deadline for a decision (zero = no deadline).
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ApprovalDecision is the outcome of an approval request.
type ApprovalDecision struct {
	// RequestID must match ApprovalRequest.ID.
	RequestID string `json:"request_id"`
	// Approved is true if the action may proceed.
	Approved bool `json:"approved"`
	// Reason is a human-readable explanation (optional).
	Reason string `json:"reason,omitempty"`
	// DecidedAt is when the decision was made.
	DecidedAt time.Time `json:"decided_at"`
	// DecidedBy is the identity of the decision maker (user ID, policy name, …).
	DecidedBy string `json:"decided_by,omitempty"`
}

// ─── Approver interface ────────────────────────────────────────────────────

// Approver is the single interface all approval implementations satisfy.
// Implementations block until a decision is available or ctx is cancelled.
// Returning (false, nil) means "denied by design" (not an infrastructure error).
type Approver interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (bool, error)
}

// ApproverFunc lets a plain function satisfy Approver.
type ApproverFunc func(ctx context.Context, req ApprovalRequest) (bool, error)

// RequestApproval implements Approver.
func (f ApproverFunc) RequestApproval(ctx context.Context, req ApprovalRequest) (bool, error) {
	return f(ctx, req)
}

// ─── Sentinels ─────────────────────────────────────────────────────────────

// Allow is an Approver that approves every request immediately.
// Use in tests and for genuinely low-risk operations.
var Allow Approver = ApproverFunc(func(_ context.Context, _ ApprovalRequest) (bool, error) {
	return true, nil
})

// Deny is an Approver that rejects every request immediately.
// Use in tests and for hard-blocked operations.
var Deny Approver = ApproverFunc(func(_ context.Context, _ ApprovalRequest) (bool, error) {
	return false, nil
})
