// Package lineage implements immutable audit trail recording for regulatory compliance.
//
// ─── Purpose ──────────────────────────────────────────────────────────────────
//
// LineageEntry records who did what to which resource and why, with cryptographic
// integrity (hash chain). Entries are immutable (append-only) and designed for
// regulatory queries and export (CSV, JSON).
//
// Every operation that touches governance—policy evaluations, agent decisions,
// HITL approvals—is recorded with:
//
//   - Identity (userID, agentID)
//   - Resource (resourceID, resourceType)
//   - Action (read, write, delete)
//   - Decision (allowed, denied)
//   - Reasoning (policyRule, reasonCode)
//   - Traceability (sessionID, traceID)
//   - Integrity (hash chain)
//
// ─── FREE-AI alignment ───────────────────────────────────────────────────────
//
// Rec 16 (Human oversight of AI decisions) — lineage records every approval.
// Rec 22 (Tamper-evident audit) — hash chain detects record modifications.
//
// ─── Examples ──────────────────────────────────────────────────────────────────
//
// Policy denial:
//
//	entry := &LineageEntry{
//	    UserID:       "user:alice",
//	    ResourceID:   "msg:456",
//	    ResourceType: "message",
//	    Action:       ActionRead,
//	    Decision:     DecisionDenied,
//	    ReasonCode:   "rbac_mismatch",
//	    PolicyRule:   "message:rbac",
//	}
//
// Tool approval:
//
//	entry := &LineageEntry{
//	    UserID:       "user:bob",
//	    AgentID:      "agent:searcher",
//	    ResourceID:   "tool:search_db",
//	    ResourceType: "tool",
//	    Action:       ActionWrite,
//	    Decision:     DecisionAllowed,
//	    ReasonCode:   "policy_allow",
//	    PolicyRule:   "tool:search_whitelist",
//	    SessionID:    "sess:xyz",
//	    TraceID:      "trace:abc123",
//	}
package lineage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ─── Action constants ──────────────────────────────────────────────────────────

// Action describes what happened to the resource.
type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
)

// String implements fmt.Stringer.
func (a Action) String() string {
	return string(a)
}

// ─── Decision constants ────────────────────────────────────────────────────────

// Decision is the outcome of the governance check.
type Decision string

const (
	DecisionAllowed Decision = "allowed"
	DecisionDenied  Decision = "denied"
)

// String implements fmt.Stringer.
func (d Decision) String() string {
	return string(d)
}

// ─── LineageEntry ─────────────────────────────────────────────────────────────

// LineageEntry is an immutable record of a governance event.
//
// Fields:
//   - ID: unique identifier (UUID v4 or similar)
//   - Timestamp: RFC3339 UTC time of the event
//   - UserID: identity of the human user (e.g. "user:alice", "sa:bot")
//   - ResourceID: the resource involved (msg:123, tool:search, agent:xyz)
//   - ResourceType: categorizes the resource (message, tool, agent, policy)
//   - Action: what happened (read, write, delete)
//   - Decision: outcome of governance check (allowed, denied)
//   - ReasonCode: machine-readable reason (rbac_mismatch, policy_allow, etc.)
//   - PolicyRule: which policy rule was applied (e.g., "message:rbac", "tool:whitelist")
//   - AgentID: identity of the agent, if agent-initiated (e.g. "agent:searcher")
//   - SessionID: groups related records from one agent run or user session
//   - TraceID: Laminar trace ID for end-to-end correlation (from pkg/eval)
//   - Hash: SHA256(prevHash + entry.Serialize()) for integrity chain
type LineageEntry struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	UserID       string    `json:"user_id"`
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	Action       Action    `json:"action"`
	Decision     Decision  `json:"decision"`
	ReasonCode   string    `json:"reason_code"`
	PolicyRule   string    `json:"policy_rule,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
	TraceID      string    `json:"trace_id,omitempty"`
	Hash         string    `json:"hash,omitempty"`
	PrevHash     string    `json:"prev_hash,omitempty"`
}

// Serialize returns a canonical string representation for hash chain calculation.
// Fields are ordered for deterministic output.
func (e *LineageEntry) Serialize() string {
	// Deterministic format: id|timestamp|userID|resourceID|resourceType|action|decision|reasonCode|policyRule|agentID|sessionID|traceID
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		e.ID,
		e.Timestamp.Format(time.RFC3339Nano),
		e.UserID,
		e.ResourceID,
		e.ResourceType,
		string(e.Action),
		string(e.Decision),
		e.ReasonCode,
		e.PolicyRule,
		e.AgentID,
		e.SessionID,
		e.TraceID,
	)
}

// ComputeHash returns SHA256(prevHash + entry.Serialize()).
// This creates a cryptographic chain where each entry depends on all previous ones.
func (e *LineageEntry) ComputeHash(prevHash string) string {
	data := prevHash + e.Serialize()
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// ─── LineageQuery ─────────────────────────────────────────────────────────────

// LineageQuery filters entries for retrieval.
type LineageQuery struct {
	// UserID filters by user (empty = all users)
	UserID string
	// ResourceID filters by resource (empty = all resources)
	ResourceID string
	// ResourceType filters by resource type (empty = all types)
	ResourceType string
	// Action filters by action (empty = all actions)
	Action Action
	// Decision filters by decision (empty = all decisions)
	Decision Decision
	// Since filters to entries at or after this time (zero = no lower bound)
	Since time.Time
	// Until filters to entries before this time (zero = no upper bound)
	Until time.Time
	// Limit caps the result count (0 = no limit)
	Limit int
	// Offset skips the first N results (for pagination)
	Offset int
}

// ─── LineageIntegrityResult ───────────────────────────────────────────────────

// LineageIntegrityResult summarizes hash chain verification.
type LineageIntegrityResult struct {
	// Valid is true if the hash chain is unbroken.
	Valid bool
	// TotalEntries is the number of entries checked.
	TotalEntries int
	// BrokenAt is the entry ID where the chain breaks (empty if Valid).
	BrokenAt string
	// Error is the underlying verification error (nil if Valid).
	Error error
}
