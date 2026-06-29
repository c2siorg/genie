package lineage

import (
	"context"
	csvpkg "encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestExport_CSV_Format(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record some entries
	now := time.Now().UTC()
	entry := &LineageEntry{
		ID:           "entry:1",
		Timestamp:    now,
		UserID:       "user:alice",
		ResourceID:   "msg:123",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
		PolicyRule:   "msg:allow",
		SessionID:    "sess:abc",
		TraceID:      "trace:xyz",
	}
	if err := rec.Record(ctx, entry); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	opts := ExportOptions{Format: ExportFormatCSV}
	csv, err := mgr.Export(ctx, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Parse CSV to verify structure
	r := csvpkg.NewReader(strings.NewReader(csv))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 rows (header + 1 entry), got %d", len(records))
	}

	// Verify header
	expectedHeader := []string{
		"ID", "Timestamp", "UserID", "ResourceID", "ResourceType",
		"Action", "Decision", "ReasonCode", "PolicyRule", "AgentID",
		"SessionID", "TraceID",
	}
	if len(records[0]) != len(expectedHeader) {
		t.Errorf("Header columns: expected %d, got %d", len(expectedHeader), len(records[0]))
	}
	for i, h := range expectedHeader {
		if i < len(records[0]) && records[0][i] != h {
			t.Errorf("Header[%d]: expected %q, got %q", i, h, records[0][i])
		}
	}

	// Verify data row
	if len(records[1]) < 12 {
		t.Errorf("Data row: expected at least 12 fields, got %d", len(records[1]))
	}
	if records[1][0] != "entry:1" {
		t.Errorf("ID: expected entry:1, got %q", records[1][0])
	}
	if records[1][2] != "user:alice" {
		t.Errorf("UserID: expected user:alice, got %q", records[1][2])
	}
}

func TestExport_JSON_Format(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record some entries
	now := time.Now().UTC()
	entry := &LineageEntry{
		ID:           "entry:1",
		Timestamp:    now,
		UserID:       "user:alice",
		ResourceID:   "msg:123",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
		PolicyRule:   "msg:allow",
	}
	if err := rec.Record(ctx, entry); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	opts := ExportOptions{Format: ExportFormatJSON}
	jsonStr, err := mgr.Export(ctx, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Parse JSON to verify structure
	var entries []*LineageEntry
	if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
		t.Fatalf("JSON parse failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "entry:1" {
		t.Errorf("ID: expected entry:1, got %q", entries[0].ID)
	}
	if entries[0].UserID != "user:alice" {
		t.Errorf("UserID: expected user:alice, got %q", entries[0].UserID)
	}
}

func TestExport_WithHash(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record some entries
	entry := &LineageEntry{
		ID:           "entry:1",
		UserID:       "user:alice",
		ResourceID:   "msg:123",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
	}
	if err := rec.Record(ctx, entry); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	opts := ExportOptions{
		Format:      ExportFormatCSV,
		IncludeHash: true,
	}
	csvStr, err := mgr.Export(ctx, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Parse CSV to verify hash columns
	r := csvpkg.NewReader(strings.NewReader(csvStr))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse failed: %v", err)
	}

	// Verify header includes Hash and PrevHash
	headerHasHash := false
	headerHasPrevHash := false
	for _, h := range records[0] {
		if h == "Hash" {
			headerHasHash = true
		}
		if h == "PrevHash" {
			headerHasPrevHash = true
		}
	}
	if !headerHasHash || !headerHasPrevHash {
		t.Error("Hash columns not included in header")
	}
}

func TestVerifyLineageIntegrity_Valid(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record 3 entries
	now := time.Now().UTC()
	for i := 1; i <= 3; i++ {
		entry := &LineageEntry{
			ID:           "entry:" + string(rune(48+i)),
			Timestamp:    now.Add(time.Duration(i) * time.Second),
			UserID:       "user:alice",
			ResourceID:   "msg:123",
			ResourceType: "message",
			Action:       ActionRead,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		}
		if err := rec.Record(ctx, entry); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	result := mgr.VerifyLineageIntegrity(ctx, now, now.Add(5*time.Second))
	if !result.Valid {
		t.Errorf("Integrity check failed: %v", result.Error)
	}
	if result.TotalEntries != 3 {
		t.Errorf("TotalEntries: expected 3, got %d", result.TotalEntries)
	}
}

func TestComplianceReport_Generation(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record mixed decisions
	now := time.Now().UTC()
	entries := []*LineageEntry{
		{
			ID:           "entry:1",
			Timestamp:    now,
			UserID:       "user:alice",
			ResourceID:   "msg:1",
			ResourceType: "message",
			Action:       ActionRead,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		},
		{
			ID:           "entry:2",
			Timestamp:    now.Add(time.Second),
			UserID:       "user:bob",
			ResourceID:   "msg:2",
			ResourceType: "message",
			Action:       ActionWrite,
			Decision:     DecisionDenied,
			ReasonCode:   "rbac_mismatch",
		},
		{
			ID:           "entry:3",
			Timestamp:    now.Add(2 * time.Second),
			UserID:       "user:alice",
			ResourceID:   "msg:3",
			ResourceType: "tool",
			Action:       ActionWrite,
			Decision:     DecisionDenied,
			ReasonCode:   "rbac_mismatch",
		},
		{
			ID:           "entry:4",
			Timestamp:    now.Add(3 * time.Second),
			UserID:       "user:bob",
			ResourceID:   "msg:4",
			ResourceType: "message",
			Action:       ActionRead,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		},
	}

	for _, entry := range entries {
		if err := rec.Record(ctx, entry); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	report, err := mgr.GenerateComplianceReport(
		ctx, "report:1", now, now.Add(5*time.Second), 5,
	)
	if err != nil {
		t.Fatalf("GenerateComplianceReport failed: %v", err)
	}

	if report.TotalEntries != 4 {
		t.Errorf("TotalEntries: expected 4, got %d", report.TotalEntries)
	}
	if report.AllowedDecisions != 2 {
		t.Errorf("AllowedDecisions: expected 2, got %d", report.AllowedDecisions)
	}
	if report.DeniedDecisions != 2 {
		t.Errorf("DeniedDecisions: expected 2, got %d", report.DeniedDecisions)
	}
	if report.UniqueUsers != 2 {
		t.Errorf("UniqueUsers: expected 2, got %d", report.UniqueUsers)
	}
	if report.UniqueResources != 4 {
		t.Errorf("UniqueResources: expected 4, got %d", report.UniqueResources)
	}

	// Verify DeniedByReason
	if report.DeniedByReason["rbac_mismatch"] != 2 {
		t.Errorf("DeniedByReason[rbac_mismatch]: expected 2, got %d",
			report.DeniedByReason["rbac_mismatch"])
	}

	// Verify HighestRiskEvents
	if len(report.HighestRiskEvents) != 2 {
		t.Errorf("HighestRiskEvents: expected 2, got %d", len(report.HighestRiskEvents))
	}
}

func TestComplianceReport_JSON(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record a simple entry
	entry := &LineageEntry{
		ID:           "entry:1",
		Timestamp:    time.Now().UTC(),
		UserID:       "user:alice",
		ResourceID:   "msg:123",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
	}
	if err := rec.Record(ctx, entry); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	report, err := mgr.GenerateComplianceReport(
		ctx, "report:1", time.Now().UTC().Add(-1*time.Hour),
		time.Now().UTC().Add(1*time.Hour), 10,
	)
	if err != nil {
		t.Fatalf("GenerateComplianceReport failed: %v", err)
	}

	jsonStr, err := mgr.ComplianceReportJSON(report)
	if err != nil {
		t.Fatalf("ComplianceReportJSON failed: %v", err)
	}

	// Verify JSON parses
	var report2 ComplianceReport
	if err := json.Unmarshal([]byte(jsonStr), &report2); err != nil {
		t.Fatalf("JSON parse failed: %v", err)
	}

	if report2.ReportID != "report:1" {
		t.Errorf("ReportID: expected report:1, got %q", report2.ReportID)
	}
	if report2.TotalEntries != 1 {
		t.Errorf("TotalEntries: expected 1, got %d", report2.TotalEntries)
	}
}

func TestComplianceReport_CSV(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	mgr := NewManager(rec)

	// Record some entries
	now := time.Now().UTC()
	for i := 1; i <= 2; i++ {
		entry := &LineageEntry{
			ID:           "entry:" + string(rune(48+i)),
			Timestamp:    now.Add(time.Duration(i) * time.Second),
			UserID:       "user:alice",
			ResourceID:   "msg:" + string(rune(48+i)),
			ResourceType: "message",
			Action:       ActionRead,
			Decision: func() Decision {
				if i == 1 {
					return DecisionAllowed
				}
				return DecisionDenied
			}(),
			ReasonCode: func() string {
				if i == 1 {
					return "ok"
				}
				return "rbac_mismatch"
			}(),
		}
		if err := rec.Record(ctx, entry); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	report, err := mgr.GenerateComplianceReport(
		ctx, "report:1", now, now.Add(5*time.Second), 10,
	)
	if err != nil {
		t.Fatalf("GenerateComplianceReport failed: %v", err)
	}

	csvStr, err := mgr.ComplianceReportCSV(report)
	if err != nil {
		t.Fatalf("ComplianceReportCSV failed: %v", err)
	}

	// Verify CSV parses
	r := csvpkg.NewReader(strings.NewReader(csvStr))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse failed: %v", err)
	}

	// Header + denied entry
	if len(records) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(records))
	}
}
