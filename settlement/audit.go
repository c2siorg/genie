// audit.go — Audit trail for settlement state transitions and agent actions.
//
// The AuditLog interface records every step in the settlement process:
// - State transitions (pending → fetched → calculated → routed → approved → executed)
// - Tool executions (agent, action, input, output)
// - Human approvals (HITL decisions)
// - Failures and retries
//
// Implementations can be in-memory, database-backed, or event-streamed.
package settlement

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AuditLog records events in the settlement lifecycle for compliance and debugging.
type AuditLog interface {
	// Log appends an entry to the audit trail.
	Log(ctx context.Context, entry AuditEntry) error
	// GetBySettlementID returns all entries for a given settlement request.
	GetBySettlementID(ctx context.Context, settlementID string) ([]AuditEntry, error)
}

// InMemoryAuditLog is a simple in-memory implementation for testing and development.
type InMemoryAuditLog struct {
	entries []AuditEntry
}

// NewInMemoryAuditLog returns an empty in-memory audit log.
func NewInMemoryAuditLog() *InMemoryAuditLog {
	return &InMemoryAuditLog{entries: []AuditEntry{}}
}

// Log appends an entry.
func (l *InMemoryAuditLog) Log(ctx context.Context, entry AuditEntry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	l.entries = append(l.entries, entry)
	return nil
}

// GetBySettlementID returns all entries for the settlement ID.
func (l *InMemoryAuditLog) GetBySettlementID(ctx context.Context, settlementID string) ([]AuditEntry, error) {
	var result []AuditEntry
	for _, e := range l.entries {
		if e.SettlementID == settlementID {
			result = append(result, e)
		}
	}
	return result, nil
}

// LogStateTransition logs a state change with the agent that executed it.
func LogStateTransition(ctx context.Context, log AuditLog, settlementID string,
	agent string, fromState, toState SettlementState) error {
	if log == nil {
		return nil // no-op if no audit log configured
	}

	input, _ := json.Marshal(map[string]string{"from": string(fromState), "to": string(toState)})
	output, _ := json.Marshal(map[string]string{"state": string(toState)})

	return log.Log(ctx, AuditEntry{
		SettlementID: settlementID,
		Agent:        agent,
		Action:       "state_transition",
		Input:        input,
		Output:       output,
		Timestamp:    time.Now(),
	})
}

// LogToolExecution logs a tool run (fetcher, calculator, router).
func LogToolExecution(ctx context.Context, log AuditLog, settlementID, agent, action string,
	input, output any, err error) error {
	if log == nil {
		return nil
	}

	inputJSON, _ := json.Marshal(input)
	outputJSON, _ := json.Marshal(output)

	entry := AuditEntry{
		SettlementID: settlementID,
		Agent:        agent,
		Action:       action,
		Input:        inputJSON,
		Output:       outputJSON,
		Timestamp:    time.Now(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	return log.Log(ctx, entry)
}

// LogApproval logs a HITL approval or rejection decision.
func LogApproval(ctx context.Context, log AuditLog, settlementID string,
	approverID string, approved bool, reason string) error {
	if log == nil {
		return nil
	}

	output := map[string]any{"approved": approved, "reason": reason}
	outputJSON, _ := json.Marshal(output)

	action := "approve"
	if !approved {
		action = "reject"
	}

	return log.Log(ctx, AuditEntry{
		SettlementID: settlementID,
		Agent:        "human",
		Action:       action,
		Output:       outputJSON,
		Approver:     approverID,
		Timestamp:    time.Now(),
	})
}
