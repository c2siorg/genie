package lineage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ─── Lineage Manager ──────────────────────────────────────────────────────────

// Manager coordinates recording lineage events from multiple sources
// (agent decisions, policy evaluations, HITL approvals).
//
// Thread-safe. Wraps a Recorder and provides convenience methods
// for common scenarios.
type Manager struct {
	recorder Recorder
}

// NewManager constructs a new lineage manager.
func NewManager(recorder Recorder) *Manager {
	return &Manager{recorder: recorder}
}

// ─── Recording helpers ────────────────────────────────────────────────────────

// RecordPolicyDecision records a policy evaluation result.
//
// Called by pkg/opa.Engine after evaluating a message-level policy.
// Captures what resource was checked, by whom, with what decision and reason.
func (m *Manager) RecordPolicyDecision(
	ctx context.Context,
	userID string,
	resourceID string,
	resourceType string,
	action Action,
	decision Decision,
	reasonCode string,
	policyRule string,
	traceID string,
) error {
	entry := &LineageEntry{
		ID:           uuid.New().String(),
		Timestamp:    time.Now().UTC(),
		UserID:       userID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Action:       action,
		Decision:     decision,
		ReasonCode:   reasonCode,
		PolicyRule:   policyRule,
		TraceID:      traceID,
	}

	return m.recorder.Record(ctx, entry)
}

// RecordAgentDecision records an agent's action decision.
//
// Called by pkg/agentic.Runner when an agent attempts to execute a tool.
// Captures the agent's identity, the tool being invoked, and whether
// it was approved for execution.
func (m *Manager) RecordAgentDecision(
	ctx context.Context,
	userID string,
	agentID string,
	toolName string,
	decision Decision,
	reasonCode string,
	sessionID string,
	traceID string,
) error {
	entry := &LineageEntry{
		ID:           uuid.New().String(),
		Timestamp:    time.Now().UTC(),
		UserID:       userID,
		AgentID:      agentID,
		ResourceID:   "tool:" + toolName,
		ResourceType: "tool",
		Action:       ActionWrite,
		Decision:     decision,
		ReasonCode:   reasonCode,
		SessionID:    sessionID,
		TraceID:      traceID,
	}

	return m.recorder.Record(ctx, entry)
}

// RecordApprovalDecision records a HITL approval decision.
//
// Called by pkg/hitl when a human approves or denies a tool execution request.
// Captures who approved it, what was approved, and their reasoning.
func (m *Manager) RecordApprovalDecision(
	ctx context.Context,
	deciderID string,
	toolName string,
	approved bool,
	reason string,
	sessionID string,
	traceID string,
) error {
	decision := DecisionAllowed
	if !approved {
		decision = DecisionDenied
	}

	reasonCode := "hitl_approved"
	if !approved {
		reasonCode = "hitl_denied"
	}

	entry := &LineageEntry{
		ID:           uuid.New().String(),
		Timestamp:    time.Now().UTC(),
		UserID:       deciderID,
		ResourceID:   "tool:" + toolName,
		ResourceType: "tool",
		Action:       ActionWrite,
		Decision:     decision,
		ReasonCode:   reasonCode,
		PolicyRule:   "hitl_approval",
		SessionID:    sessionID,
		TraceID:      traceID,
	}

	return m.recorder.Record(ctx, entry)
}

// RecordResourceAccess records a resource read/write/delete event.
//
// General-purpose logging for any resource access (messages, files, configs).
// Useful for capturing compliance-relevant operations beyond agent tools.
func (m *Manager) RecordResourceAccess(
	ctx context.Context,
	userID string,
	resourceID string,
	resourceType string,
	action Action,
	decision Decision,
	reasonCode string,
	policyRule string,
	agentID string,
	sessionID string,
	traceID string,
) error {
	entry := &LineageEntry{
		ID:           uuid.New().String(),
		Timestamp:    time.Now().UTC(),
		UserID:       userID,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		Action:       action,
		Decision:     decision,
		ReasonCode:   reasonCode,
		PolicyRule:   policyRule,
		AgentID:      agentID,
		SessionID:    sessionID,
		TraceID:      traceID,
	}

	return m.recorder.Record(ctx, entry)
}

// ─── Listener interfaces ──────────────────────────────────────────────────────

// PolicyListener hooks into policy evaluation for lineage recording.
// Use with pkg/governance.Policy implementations.
//
// Example:
//
//	listener := lineage.NewPolicyListener(mgr)
//	// Inside policy evaluation loop:
//	listener.OnPolicyDecision(ctx, userID, msg.ID, "message", ..., result)
type PolicyListener struct {
	mgr *Manager
}

// NewPolicyListener constructs a policy listener.
func NewPolicyListener(mgr *Manager) *PolicyListener {
	return &PolicyListener{mgr: mgr}
}

// OnPolicyDecision records a policy evaluation.
func (p *PolicyListener) OnPolicyDecision(
	ctx context.Context,
	userID string,
	resourceID string,
	resourceType string,
	action Action,
	decision Decision,
	reasonCode string,
	policyRule string,
	traceID string,
) {
	_ = p.mgr.RecordPolicyDecision(
		ctx, userID, resourceID, resourceType, action, decision,
		reasonCode, policyRule, traceID,
	)
}

// AgentListener hooks into agent execution for lineage recording.
// Use with pkg/agentic.Runner.Callbacks.
//
// Example:
//
//	listener := lineage.NewAgentListener(mgr)
//	runner.Callbacks.OnToolCallStart = func(toolName string, args map[string]any) {
//	    listener.OnToolCall(ctx, userID, agentID, toolName, sessionID, traceID)
//	}
type AgentListener struct {
	mgr *Manager
}

// NewAgentListener constructs an agent listener.
func NewAgentListener(mgr *Manager) *AgentListener {
	return &AgentListener{mgr: mgr}
}

// OnToolCall records a tool execution attempt.
func (a *AgentListener) OnToolCall(
	ctx context.Context,
	userID string,
	agentID string,
	toolName string,
	decision Decision,
	sessionID string,
	traceID string,
) {
	reasonCode := "agent_execute"
	if decision == DecisionDenied {
		reasonCode = "agent_rejected"
	}

	_ = a.mgr.RecordAgentDecision(
		ctx, userID, agentID, toolName, decision,
		reasonCode, sessionID, traceID,
	)
}

// ApprovalListener hooks into HITL approval decisions for lineage recording.
// Use with pkg/hitl.AsyncApprover.OnApprovalDecision callback.
//
// Example:
//
//	listener := lineage.NewApprovalListener(mgr)
//	approver.OnApprovalDecision = func(ctx context.Context, decision *hitl.ApprovalDecision) {
//	    listener.OnApprovalDecision(ctx, decision.DecidedBy, toolName, decision.Approved, ...)
//	}
type ApprovalListener struct {
	mgr *Manager
}

// NewApprovalListener constructs an approval listener.
func NewApprovalListener(mgr *Manager) *ApprovalListener {
	return &ApprovalListener{mgr: mgr}
}

// OnApprovalDecision records an approval decision.
func (a *ApprovalListener) OnApprovalDecision(
	ctx context.Context,
	deciderID string,
	toolName string,
	approved bool,
	reason string,
	sessionID string,
	traceID string,
) {
	_ = a.mgr.RecordApprovalDecision(
		ctx, deciderID, toolName, approved, reason, sessionID, traceID,
	)
}

// ─── Query helpers ────────────────────────────────────────────────────────────

// QueryUserActivity returns all lineage entries for a specific user within a time range.
func (m *Manager) QueryUserActivity(
	ctx context.Context,
	userID string,
	since time.Time,
	until time.Time,
) ([]*LineageEntry, error) {
	q := LineageQuery{
		UserID: userID,
		Since:  since,
		Until:  until,
	}
	return m.recorder.Query(ctx, q)
}

// QueryResourceAccess returns all lineage entries for a specific resource.
func (m *Manager) QueryResourceAccess(
	ctx context.Context,
	resourceID string,
) ([]*LineageEntry, error) {
	q := LineageQuery{
		ResourceID: resourceID,
	}
	return m.recorder.Query(ctx, q)
}

// QueryDenials returns all denied decisions within a time range.
func (m *Manager) QueryDenials(
	ctx context.Context,
	since time.Time,
	until time.Time,
) ([]*LineageEntry, error) {
	q := LineageQuery{
		Decision: DecisionDenied,
		Since:    since,
		Until:    until,
	}
	return m.recorder.Query(ctx, q)
}

// QueryAgentActions returns all lineage entries for a specific agent.
func (m *Manager) QueryAgentActions(
	ctx context.Context,
	agentID string,
	since time.Time,
	until time.Time,
) ([]*LineageEntry, error) {
	// This is a helper; in a real system, you might want to add AgentID to the LineageQuery filter.
	// For now, we fetch all and filter in-memory.
	q := LineageQuery{
		Since: since,
		Until: until,
	}
	entries, err := m.recorder.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	var results []*LineageEntry
	for _, entry := range entries {
		if entry.AgentID == agentID {
			results = append(results, entry)
		}
	}

	return results, nil
}

// QuerySession returns all lineage entries for a specific session.
func (m *Manager) QuerySession(
	ctx context.Context,
	sessionID string,
) ([]*LineageEntry, error) {
	// Helper to fetch a session's lineage.
	// In a real system, you might add SessionID to the LineageQuery filter.
	q := LineageQuery{
		Limit: 1000, // reasonable default for session queries
	}
	entries, err := m.recorder.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	var results []*LineageEntry
	for _, entry := range entries {
		if entry.SessionID == sessionID {
			results = append(results, entry)
		}
	}

	return results, nil
}

// ─── Audit functions ──────────────────────────────────────────────────────────

// GenerateAuditReport creates a human-readable audit report for a given time range.
func (m *Manager) GenerateAuditReport(
	ctx context.Context,
	since time.Time,
	until time.Time,
) (string, error) {
	q := LineageQuery{
		Since: since,
		Until: until,
	}

	entries, err := m.recorder.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("lineage: audit report query failed: %w", err)
	}

	report := fmt.Sprintf("Audit Report: %s to %s\n", since.Format(time.RFC3339), until.Format(time.RFC3339))
	report += fmt.Sprintf("Total entries: %d\n\n", len(entries))

	// Aggregate decisions
	allowedCount := 0
	deniedCount := 0
	for _, e := range entries {
		if e.Decision == DecisionAllowed {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	report += fmt.Sprintf("Decisions: %d allowed, %d denied\n\n", allowedCount, deniedCount)

	// Denied entries detail
	if deniedCount > 0 {
		report += "Denied Decisions:\n"
		for _, e := range entries {
			if e.Decision == DecisionDenied {
				report += fmt.Sprintf(
					"  - [%s] %s on %s (resource: %s, reason: %s)\n",
					e.Timestamp.Format(time.RFC3339),
					e.UserID,
					e.Action,
					e.ResourceID,
					e.ReasonCode,
				)
			}
		}
	}

	return report, nil
}

// VerifyIntegrity checks the hash chain integrity.
func (m *Manager) VerifyIntegrity(ctx context.Context) LineageIntegrityResult {
	return m.recorder.Verify(ctx)
}

// Close closes the underlying recorder.
func (m *Manager) Close() error {
	return m.recorder.Close()
}
