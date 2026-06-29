package lineage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestManager_RecordPolicyDecision(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	err := mgr.RecordPolicyDecision(
		ctx,
		"user:alice",
		"msg:123",
		"message",
		ActionRead,
		DecisionDenied,
		"rbac_mismatch",
		"message:rbac",
		"trace:abc",
	)

	if err != nil {
		t.Fatalf("RecordPolicyDecision failed: %v", err)
	}

	// Verify entry was recorded
	q := LineageQuery{UserID: "user:alice"}
	entries, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.UserID != "user:alice" {
		t.Errorf("UserID: expected user:alice, got %q", entry.UserID)
	}
	if entry.Decision != DecisionDenied {
		t.Errorf("Decision: expected denied, got %v", entry.Decision)
	}
	if entry.TraceID != "trace:abc" {
		t.Errorf("TraceID: expected trace:abc, got %q", entry.TraceID)
	}
}

func TestManager_RecordAgentDecision(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	err := mgr.RecordAgentDecision(
		ctx,
		"user:bob",
		"agent:searcher",
		"search_db",
		DecisionAllowed,
		"policy_allow",
		"sess:xyz",
		"trace:def",
	)

	if err != nil {
		t.Fatalf("RecordAgentDecision failed: %v", err)
	}

	// Verify entry was recorded
	q := LineageQuery{UserID: "user:bob"}
	entries, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.AgentID != "agent:searcher" {
		t.Errorf("AgentID: expected agent:searcher, got %q", entry.AgentID)
	}
	if entry.ResourceID != "tool:search_db" {
		t.Errorf("ResourceID: expected tool:search_db, got %q", entry.ResourceID)
	}
	if entry.Action != ActionWrite {
		t.Errorf("Action: expected write, got %v", entry.Action)
	}
}

func TestManager_RecordApprovalDecision(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	err := mgr.RecordApprovalDecision(
		ctx,
		"user:approver",
		"delete_file",
		true,
		"approved after review",
		"sess:xyz",
		"trace:ghi",
	)

	if err != nil {
		t.Fatalf("RecordApprovalDecision failed: %v", err)
	}

	// Verify entry was recorded
	q := LineageQuery{UserID: "user:approver", Decision: DecisionAllowed}
	entries, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.ReasonCode != "hitl_approved" {
		t.Errorf("ReasonCode: expected hitl_approved, got %q", entry.ReasonCode)
	}
	if entry.PolicyRule != "hitl_approval" {
		t.Errorf("PolicyRule: expected hitl_approval, got %q", entry.PolicyRule)
	}
}

func TestManager_QueryUserActivity(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	now := time.Now().UTC()

	// Record entries from two users at different times
	_ = mgr.RecordPolicyDecision(
		ctx, "user:alice", "msg:1", "message", ActionRead, DecisionAllowed, "ok", "rule1", "trace:1",
	)
	time.Sleep(10 * time.Millisecond)
	_ = mgr.RecordPolicyDecision(
		ctx, "user:bob", "msg:2", "message", ActionRead, DecisionAllowed, "ok", "rule1", "trace:2",
	)
	time.Sleep(10 * time.Millisecond)
	_ = mgr.RecordPolicyDecision(
		ctx, "user:alice", "msg:3", "message", ActionWrite, DecisionDenied, "rbac", "rule2", "trace:3",
	)

	// Query alice's activity
	since := now.Add(-1 * time.Second)
	until := now.Add(1 * time.Second)
	entries, err := mgr.QueryUserActivity(ctx, "user:alice", since, until)
	if err != nil {
		t.Fatalf("QueryUserActivity failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries for alice, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry.UserID != "user:alice" {
			t.Errorf("Expected user:alice, got %q", entry.UserID)
		}
	}
}

func TestManager_QueryDenials(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	now := time.Now().UTC()

	// Record mixed decisions
	_ = mgr.RecordPolicyDecision(
		ctx, "user:alice", "msg:1", "message", ActionRead, DecisionAllowed, "ok", "rule1", "trace:1",
	)
	_ = mgr.RecordPolicyDecision(
		ctx, "user:bob", "msg:2", "message", ActionRead, DecisionDenied, "rbac", "rule1", "trace:2",
	)
	_ = mgr.RecordPolicyDecision(
		ctx, "user:alice", "msg:3", "message", ActionWrite, DecisionDenied, "pii_block", "rule2", "trace:3",
	)

	// Query denials
	since := now.Add(-1 * time.Second)
	until := now.Add(1 * time.Second)
	entries, err := mgr.QueryDenials(ctx, since, until)
	if err != nil {
		t.Fatalf("QueryDenials failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 denials, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry.Decision != DecisionDenied {
			t.Errorf("Expected denied, got %v", entry.Decision)
		}
	}
}

func TestManager_GenerateAuditReport(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	now := time.Now().UTC()

	// Record mixed decisions
	_ = mgr.RecordPolicyDecision(
		ctx, "user:alice", "msg:1", "message", ActionRead, DecisionAllowed, "ok", "rule1", "trace:1",
	)
	_ = mgr.RecordPolicyDecision(
		ctx, "user:bob", "msg:2", "message", ActionRead, DecisionDenied, "rbac", "rule1", "trace:2",
	)

	report, err := mgr.GenerateAuditReport(ctx, now.Add(-1*time.Second), now.Add(1*time.Second))
	if err != nil {
		t.Fatalf("GenerateAuditReport failed: %v", err)
	}

	// Verify report contains expected content
	if !strings.Contains(report, "Total entries: 2") {
		t.Error("Report missing total entries")
	}
	if !strings.Contains(report, "1 allowed") {
		t.Error("Report missing allowed count")
	}
	if !strings.Contains(report, "1 denied") {
		t.Error("Report missing denied count")
	}
	if !strings.Contains(report, "Denied Decisions") {
		t.Error("Report missing denied decisions section")
	}
}

func TestPolicyListener_OnPolicyDecision(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)
	listener := NewPolicyListener(mgr)

	listener.OnPolicyDecision(
		ctx,
		"user:alice",
		"msg:123",
		"message",
		ActionRead,
		DecisionDenied,
		"rbac_mismatch",
		"message:rbac",
		"trace:xyz",
	)

	// Verify entry was recorded
	q := LineageQuery{UserID: "user:alice"}
	entries, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
}

func TestAgentListener_OnToolCall(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)
	listener := NewAgentListener(mgr)

	listener.OnToolCall(
		ctx,
		"user:alice",
		"agent:searcher",
		"search_db",
		DecisionAllowed,
		"sess:xyz",
		"trace:abc",
	)

	// Verify entry was recorded
	q := LineageQuery{UserID: "user:alice"}
	entries, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Decision != DecisionAllowed {
		t.Errorf("Decision: expected allowed, got %v", entry.Decision)
	}
	if entry.ReasonCode != "agent_execute" {
		t.Errorf("ReasonCode: expected agent_execute, got %q", entry.ReasonCode)
	}
}

func TestApprovalListener_OnApprovalDecision(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)
	listener := NewApprovalListener(mgr)

	listener.OnApprovalDecision(
		ctx,
		"user:approver",
		"delete_file",
		true,
		"reviewed and approved",
		"sess:xyz",
		"trace:def",
	)

	// Verify entry was recorded
	q := LineageQuery{UserID: "user:approver"}
	entries, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Decision != DecisionAllowed {
		t.Errorf("Decision: expected allowed, got %v", entry.Decision)
	}
	if entry.ReasonCode != "hitl_approved" {
		t.Errorf("ReasonCode: expected hitl_approved, got %q", entry.ReasonCode)
	}
}

func TestManager_VerifyIntegrity(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record entries
	for i := 1; i <= 3; i++ {
		_ = mgr.RecordPolicyDecision(
			ctx,
			"user:alice",
			"msg:"+string(rune(48+i)),
			"message",
			ActionRead,
			DecisionAllowed,
			"ok",
			"rule",
			"trace",
		)
	}

	result := mgr.VerifyIntegrity(ctx)
	if !result.Valid {
		t.Errorf("VerifyIntegrity failed: %v", result.Error)
	}
	if result.TotalEntries != 3 {
		t.Errorf("TotalEntries: expected 3, got %d", result.TotalEntries)
	}
}
