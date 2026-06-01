// Package lineage implements immutable audit trail recording for regulatory compliance.
//
// ─── Overview ─────────────────────────────────────────────────────────────────
//
// Lineage tracks governance events (policy decisions, agent actions, human
// approvals) with cryptographic integrity via hash chains. Designed for
// regulatory compliance (RBI, FATF, FREE-AI) and audit queries.
//
// ─── Key features ──────────────────────────────────────────────────────────────
//
// Immutable recording: Entries are append-only; no updates or deletes.
// Hash chain integrity: Each entry hashes the previous entry's hash,
//   detecting tampering or reordering.
// Thread-safe: Concurrent Record calls are serialized and ordered.
// Multiple backends: In-memory (testing) and PostgreSQL (production).
// Regulatory export: CSV and JSON formats for audit submissions.
// Query interface: Filter by user, resource, decision, time range, etc.
//
// ─── Usage ────────────────────────────────────────────────────────────────────
//
// 1. Create a recorder:
//
//	rec := lineage.NewInMemoryRecorder()
//	// or
//	db, _ := sql.Open("postgres", "...")
//	rec := lineage.NewPostgreSQLRecorder(db)
//	rec.EnsureSchema(ctx)
//
// 2. Create a manager and hook it into your systems:
//
//	mgr := lineage.NewManager(rec)
//	policyListener := lineage.NewPolicyListener(mgr)
//	agentListener := lineage.NewAgentListener(mgr)
//	approvalListener := lineage.NewApprovalListener(mgr)
//
// 3. Record decisions (typically from integrations):
//
//	mgr.RecordPolicyDecision(ctx,
//	    "user:alice", "msg:123", "message", lineage.ActionRead,
//	    lineage.DecisionDenied, "rbac_mismatch", "message:rbac", "trace:xyz")
//
// 4. Query lineage:
//
//	entries, _ := mgr.QueryDenials(ctx, since, until)
//	for _, entry := range entries {
//	    fmt.Println(entry.ID, entry.UserID, entry.ReasonCode)
//	}
//
// 5. Export for compliance:
//
//	opts := lineage.ExportOptions{Format: "csv", Since: since, Until: until}
//	csv, _ := mgr.Export(ctx, opts)
//	fmt.Println(csv)
//
// 6. Verify integrity:
//
//	result := mgr.VerifyLineageIntegrity(ctx, since, until)
//	if !result.Valid {
//	    fmt.Printf("Hash chain broken at %s: %v\n", result.BrokenAt, result.Error)
//	}
//
// ─── Integration patterns ──────────────────────────────────────────────────────
//
// Policy evaluation hook (pkg/governance):
//
//	// In your policy evaluation loop:
//	result, _ := policy.Evaluate(ctx, msg)
//	policyListener.OnPolicyDecision(ctx,
//	    userID, msg.ID, msg.Type, action,
//	    decision, reasonCode, policyRule, traceID)
//
// Agent execution hook (pkg/agentic):
//
//	// In Runner.Run, after tool execution approval:
//	if r.Approver != nil {
//	    approved, _ := r.Approver.RequestApproval(ctx, req)
//	    agentListener.OnToolCall(ctx,
//	        r.UserID, agentID, tc.Function.Name,
//	        decision, sessionID, traceID)
//	}
//
// HITL approval hook (pkg/hitl):
//
//	// In async approver decision handler:
//	approvalListener.OnApprovalDecision(ctx,
//	    decision.DecidedBy, toolName, decision.Approved,
//	    decision.Reason, sessionID, traceID)
//
// ─── FREE-AI alignment ─────────────────────────────────────────────────────────
//
// Rec 16 (Human oversight of AI decisions):
//	Every approval is recorded with who decided and why.
//
// Rec 22 (Tamper-evident audit):
//	Hash chain detects modifications; each entry includes previous hash.
//
// ─── Types overview ───────────────────────────────────────────────────────────
//
// LineageEntry: Immutable record of a governance event.
// Recorder: Interface for recording and querying entries.
// InMemoryLineageRecorder: Thread-safe in-memory implementation.
// PostgreSQLLineageRecorder: Durable database implementation with indexes.
// Manager: Coordinates recording from multiple sources; provides helpers.
// *Listener: Convenience hooks for policy, agent, and approval events.
// ExportOptions: Controls export format and time range.
// ComplianceReport: Summary of lineage for regulatory submission.
//
// ─── Testing ───────────────────────────────────────────────────────────────────
//
// The package includes comprehensive unit tests covering:
//
//	- Hash chain integrity (deterministic, ordered, unbroken)
//	- Concurrent recording (no duplicates, thread-safe)
//	- Query filters (user, resource, decision, time range, pagination)
//	- Export formats (CSV with headers, JSON parseable)
//	- Compliance reports (counts, denied-by-reason, high-risk events)
//	- Integration scenarios (policy, agent, approval listeners)
//
// Run: go test ./pkg/lineage/...
package lineage
