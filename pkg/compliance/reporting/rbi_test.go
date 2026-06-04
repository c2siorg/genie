package reporting

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestGenerateRBIReport_QuarterFilter(t *testing.T) {
	// Test that GenerateRBIReport correctly filters by quarter.
	txns := []finance.Transaction{
		{
			TransactionID: "txn1",
			AccountID:     "acct1",
			Date:          "2024-01-15", // Q1
			AmountCents:   100_000,
			Currency:      "INR",
			Merchant:      "Store A",
		},
		{
			TransactionID: "txn2",
			AccountID:     "acct1",
			Date:          "2024-04-20", // Q2
			AmountCents:   200_000,
			Currency:      "INR",
			Merchant:      "Store B",
		},
		{
			TransactionID: "txn3",
			AccountID:     "acct1",
			Date:          "2024-07-10", // Q3
			AmountCents:   150_000,
			Currency:      "INR",
			Merchant:      "Store A",
		},
	}

	auditLog := []AuditEntry{}

	// Generate Q1 report.
	report1, err := GenerateRBIReport(1, 2024, txns, auditLog)
	if err != nil {
		t.Fatalf("GenerateRBIReport Q1 failed: %v", err)
	}
	if report1.TotalTransactions != 1 {
		t.Errorf("Q1 should have 1 transaction, got %d", report1.TotalTransactions)
	}
	if report1.TotalVolumeCents != 100_000 {
		t.Errorf("Q1 volume should be 100000, got %d", report1.TotalVolumeCents)
	}

	// Generate Q2 report.
	report2, err := GenerateRBIReport(2, 2024, txns, auditLog)
	if err != nil {
		t.Fatalf("GenerateRBIReport Q2 failed: %v", err)
	}
	if report2.TotalTransactions != 1 {
		t.Errorf("Q2 should have 1 transaction, got %d", report2.TotalTransactions)
	}

	// Generate Q3 report.
	report3, err := GenerateRBIReport(3, 2024, txns, auditLog)
	if err != nil {
		t.Fatalf("GenerateRBIReport Q3 failed: %v", err)
	}
	if report3.TotalTransactions != 1 {
		t.Errorf("Q3 should have 1 transaction, got %d", report3.TotalTransactions)
	}

	// Q4 should be empty.
	report4, err := GenerateRBIReport(4, 2024, txns, auditLog)
	if err != nil {
		t.Fatalf("GenerateRBIReport Q4 failed: %v", err)
	}
	if report4.TotalTransactions != 0 {
		t.Errorf("Q4 should have 0 transactions, got %d", report4.TotalTransactions)
	}
}

func TestGenerateRBIReport_SuspiciousFlagging(t *testing.T) {
	// Test that high-value transactions are marked as suspicious.
	txns := []finance.Transaction{
		{
			TransactionID: "txn_normal",
			AccountID:     "acct1",
			Date:          "2024-01-15",
			AmountCents:   500_000, // normal
			Currency:      "INR",
			Merchant:      "Store A",
		},
		{
			TransactionID: "txn_high",
			AccountID:     "acct1",
			Date:          "2024-01-16",
			AmountCents:   11_000_000, // high-value threshold > 10M
			Currency:      "INR",
			Merchant:      "Store B",
		},
	}

	auditLog := []AuditEntry{}

	report, err := GenerateRBIReport(1, 2024, txns, auditLog)
	if err != nil {
		t.Fatalf("GenerateRBIReport failed: %v", err)
	}

	if report.TotalTransactions != 2 {
		t.Errorf("expected 2 txns, got %d", report.TotalTransactions)
	}
	if report.SuspiciousCount != 1 {
		t.Errorf("expected 1 suspicious txn, got %d", report.SuspiciousCount)
	}
}

func TestGenerateRBIReport_AuditLogFlagging(t *testing.T) {
	// Test that audit log AML flags are picked up.
	txns := []finance.Transaction{
		{
			TransactionID: "txn_flagged",
			AccountID:     "acct1",
			Date:          "2024-01-15",
			AmountCents:   500_000,
			Currency:      "INR",
			Merchant:      "Store A",
		},
	}

	// Audit log with explicit AML flag.
	auditLog := []AuditEntry{
		{
			Seq:        1,
			OccurredAt: time.Now(),
			Actor:      "aml_agent",
			Action:     "aml.flagged",
			Target:     "txn_flagged",
			Details: map[string]any{
				"transaction_id": "txn_flagged",
				"reason":         "suspicious_pattern",
			},
		},
	}

	report, err := GenerateRBIReport(1, 2024, txns, auditLog)
	if err != nil {
		t.Fatalf("GenerateRBIReport failed: %v", err)
	}

	if report.SuspiciousCount != 1 {
		t.Errorf("expected 1 suspicious txn from audit flag, got %d", report.SuspiciousCount)
	}
}

func TestExportRBIReport_CSV(t *testing.T) {
	report := &RBIQuarterlyReport{
		Quarter:           1,
		Year:              2024,
		ReportID:          "RBI-Q1-2024-123",
		GeneratedAt:       time.Now(),
		TotalTransactions: 100,
		TotalVolumeCents:  50_000_000,
		SuspiciousCount:   5,
		RiskyMerchants:    2,
		ComplianceNotes:   "Test notes",
		MerchantBreakdown: []MerchantVolume{
			{
				Merchant:         "store_a",
				TransactionCount: 50,
				VolumeCents:      30_000_000,
				SuspiciousCount:  2,
			},
		},
	}

	data, err := ExportRBIReport(report, "csv")
	if err != nil {
		t.Fatalf("ExportRBIReport CSV failed: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("CSV export returned empty data")
	}

	csv := string(data)
	if !strings.Contains(csv, "RBI Quarterly Report") {
		t.Errorf("CSV should contain report header")
	}
	if !strings.Contains(csv, "100") {
		t.Errorf("CSV should contain total transaction count")
	}
	if !strings.Contains(csv, "store_a") {
		t.Errorf("CSV should contain merchant breakdown")
	}
}

func TestExportRBIReport_JSON(t *testing.T) {
	report := &RBIQuarterlyReport{
		Quarter:           1,
		Year:              2024,
		ReportID:          "RBI-Q1-2024-456",
		GeneratedAt:       time.Now(),
		TotalTransactions: 100,
		TotalVolumeCents:  50_000_000,
		SuspiciousCount:   5,
		RiskyMerchants:    2,
		ComplianceNotes:   "Test notes",
	}

	data, err := ExportRBIReport(report, "json")
	if err != nil {
		t.Fatalf("ExportRBIReport JSON failed: %v", err)
	}

	// Verify it's valid JSON and can be unmarshaled.
	var parsed RBIQuarterlyReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.ReportID != report.ReportID {
		t.Errorf("JSON roundtrip failed: report ID mismatch")
	}
	if parsed.SuspiciousCount != report.SuspiciousCount {
		t.Errorf("JSON roundtrip failed: suspicious count mismatch")
	}
}

func TestExportRBIReport_InvalidFormat(t *testing.T) {
	report := &RBIQuarterlyReport{Quarter: 1, Year: 2024}

	_, err := ExportRBIReport(report, "invalid_format")
	if err == nil {
		t.Errorf("Expected error for invalid format, got nil")
	}
}

func TestRBIReportWriter_ThreadSafety(t *testing.T) {
	// Basic thread-safety test: concurrent appends should not corrupt the list.
	writer := &ReportWriter{}

	// Append reports concurrently.
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			report := &RBIQuarterlyReport{
				Quarter:           1,
				Year:              2024,
				ReportID:          "report-" + string(rune(idx)),
				TotalTransactions: int64(idx),
			}
			writer.Append(report)
			done <- true
		}(i)
	}

	// Wait for all goroutines.
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all reports are present.
	all := writer.All()
	if len(all) != 10 {
		t.Errorf("Expected 10 reports, got %d", len(all))
	}
}

func TestRBIReportMerchantBreakdown_Ordering(t *testing.T) {
	// Test that merchant breakdown is ordered by volume descending.
	txns := []finance.Transaction{
		{
			TransactionID: "txn1",
			AccountID:     "acct1",
			Date:          "2024-01-15",
			AmountCents:   1_000_000, // Small
			Currency:      "INR",
			Merchant:      "Small Store",
		},
		{
			TransactionID: "txn2",
			AccountID:     "acct1",
			Date:          "2024-01-16",
			AmountCents:   10_000_000, // Large
			Currency:      "INR",
			Merchant:      "Big Store",
		},
		{
			TransactionID: "txn3",
			AccountID:     "acct1",
			Date:          "2024-01-17",
			AmountCents:   5_000_000, // Medium
			Currency:      "INR",
			Merchant:      "Medium Store",
		},
	}

	report, err := GenerateRBIReport(1, 2024, txns, []AuditEntry{})
	if err != nil {
		t.Fatalf("GenerateRBIReport failed: %v", err)
	}

	if len(report.MerchantBreakdown) != 3 {
		t.Fatalf("Expected 3 merchants, got %d", len(report.MerchantBreakdown))
	}

	// Check ordering: largest volume first.
	if report.MerchantBreakdown[0].VolumeCents != 10_000_000 {
		t.Errorf("First merchant should have largest volume (10M), got %d", report.MerchantBreakdown[0].VolumeCents)
	}
	if report.MerchantBreakdown[1].VolumeCents != 5_000_000 {
		t.Errorf("Second merchant should have medium volume (5M), got %d", report.MerchantBreakdown[1].VolumeCents)
	}
	if report.MerchantBreakdown[2].VolumeCents != 1_000_000 {
		t.Errorf("Third merchant should have smallest volume (1M), got %d", report.MerchantBreakdown[2].VolumeCents)
	}
}

func TestGenerateRBIReport_InvalidQuarter(t *testing.T) {
	_, err := GenerateRBIReport(5, 2024, []finance.Transaction{}, []AuditEntry{})
	if err == nil {
		t.Errorf("Expected error for invalid quarter, got nil")
	}

	_, err = GenerateRBIReport(0, 2024, []finance.Transaction{}, []AuditEntry{})
	if err == nil {
		t.Errorf("Expected error for quarter 0, got nil")
	}
}

func TestGenerateRBIReport_InvalidYear(t *testing.T) {
	_, err := GenerateRBIReport(1, 1999, []finance.Transaction{}, []AuditEntry{})
	if err == nil {
		t.Errorf("Expected error for year 1999, got nil")
	}

	_, err = GenerateRBIReport(1, 2101, []finance.Transaction{}, []AuditEntry{})
	if err == nil {
		t.Errorf("Expected error for year 2101, got nil")
	}
}

func TestGenerateRBIReport_ComplianceNotes(t *testing.T) {
	txns := []finance.Transaction{
		{
			TransactionID: "txn1",
			AccountID:     "acct1",
			Date:          "2024-01-15",
			AmountCents:   12_000_000, // suspicious
			Currency:      "INR",
			Merchant:      "Store A",
		},
	}

	report, _ := GenerateRBIReport(1, 2024, txns, []AuditEntry{})

	if !strings.Contains(report.ComplianceNotes, "Q1 2024") {
		t.Errorf("Compliance notes should reference quarter and year")
	}
	if !strings.Contains(report.ComplianceNotes, "RBI") {
		t.Errorf("Compliance notes should reference RBI")
	}
	if !strings.Contains(report.ComplianceNotes, "7 years") {
		t.Errorf("Compliance notes should mention 7-year retention requirement")
	}
}

func TestInMemoryReportRepository_SaveAndRetrieve(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	report := &RBIQuarterlyReport{
		Quarter:           1,
		Year:              2024,
		ReportID:          "test-rbi-001",
		TotalTransactions: 100,
	}

	// Save.
	id, err := repo.SaveRBIReport(ctx, report)
	if err != nil {
		t.Fatalf("SaveRBIReport failed: %v", err)
	}
	if id != "test-rbi-001" {
		t.Errorf("Expected report ID test-rbi-001, got %s", id)
	}

	// Retrieve.
	retrieved, err := repo.GetRBIReport(ctx, id)
	if err != nil {
		t.Fatalf("GetRBIReport failed: %v", err)
	}
	if retrieved.TotalTransactions != 100 {
		t.Errorf("Retrieved report has wrong transaction count")
	}
}

func TestInMemoryReportRepository_ImmutabilityEnforced(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	report := &RBIQuarterlyReport{
		Quarter:  1,
		Year:     2024,
		ReportID: "immutable-001",
	}

	// Save once.
	repo.SaveRBIReport(ctx, report)

	// Try to save again with same ID.
	_, err := repo.SaveRBIReport(ctx, report)
	if err == nil {
		t.Errorf("Expected error when saving duplicate report ID, got nil")
	}
}

func TestInMemoryReportRepository_QueryByDateRange(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	report1 := &RBIQuarterlyReport{
		Quarter:     1,
		Year:        2024,
		ReportID:    "report-1",
		GeneratedAt: now,
	}

	report2 := &RBIQuarterlyReport{
		Quarter:     2,
		Year:        2024,
		ReportID:    "report-2",
		GeneratedAt: yesterday,
	}

	repo.SaveRBIReport(ctx, report1)
	repo.SaveRBIReport(ctx, report2)

	// Query for reports in the last 5 days.
	fiveDaysAgo := now.AddDate(0, 0, -5)
	results, err := repo.QueryRBIReports(ctx, fiveDaysAgo, tomorrow)
	if err != nil {
		t.Fatalf("QueryRBIReports failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 reports in range, got %d", len(results))
	}
}

func TestInMemoryReportRepository_Stats(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	stats := repo.Stats()
	if stats["rbi_reports"] != 0 {
		t.Errorf("Initial RBI report count should be 0")
	}

	repo.SaveRBIReport(ctx, &RBIQuarterlyReport{ReportID: "rbi-1"})
	stats = repo.Stats()
	if stats["rbi_reports"] != 1 {
		t.Errorf("After save, RBI report count should be 1, got %d", stats["rbi_reports"])
	}
}
