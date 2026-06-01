package reporting

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryReportRepository_SaveNilReport(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	_, err := repo.SaveRBIReport(ctx, nil)
	if err == nil {
		t.Errorf("Expected error when saving nil RBI report")
	}

	_, err = repo.SaveSTRReport(ctx, nil)
	if err == nil {
		t.Errorf("Expected error when saving nil STR report")
	}
}

func TestInMemoryReportRepository_MissingReportID(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	// RBI report without ID.
	report := &RBIQuarterlyReport{Quarter: 1, Year: 2024}
	_, err := repo.SaveRBIReport(ctx, report)
	if err == nil {
		t.Errorf("Expected error when RBI report has no ID")
	}

	// STR without ID.
	str := &FATPSTRReport{TransactionID: "txn1"}
	_, err = repo.SaveSTRReport(ctx, str)
	if err == nil {
		t.Errorf("Expected error when STR has no ID")
	}
}

func TestInMemoryReportRepository_DuplicateRBIReportID(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	report := &RBIQuarterlyReport{
		Quarter:  1,
		Year:     2024,
		ReportID: "duplicate-id",
	}

	// Save first time.
	_, err := repo.SaveRBIReport(ctx, report)
	if err != nil {
		t.Fatalf("First save should succeed: %v", err)
	}

	// Save second time with same ID should fail.
	_, err = repo.SaveRBIReport(ctx, report)
	if err == nil {
		t.Errorf("Expected error when saving duplicate RBI report ID")
	}
}

func TestInMemoryReportRepository_DuplicateSTRID(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	str := &FATPSTRReport{
		STRID:         "duplicate-str",
		TransactionID: "txn1",
		Amount:        1_000_000,
	}

	// Save first time.
	_, err := repo.SaveSTRReport(ctx, str)
	if err != nil {
		t.Fatalf("First save should succeed: %v", err)
	}

	// Save second time with same ID should fail.
	_, err = repo.SaveSTRReport(ctx, str)
	if err == nil {
		t.Errorf("Expected error when saving duplicate STR ID")
	}
}

func TestInMemoryReportRepository_GetMissingReport(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	_, err := repo.GetRBIReport(ctx, "nonexistent-rbi")
	if err == nil {
		t.Errorf("Expected error when getting nonexistent RBI report")
	}

	_, err = repo.GetSTRReport(ctx, "nonexistent-str")
	if err == nil {
		t.Errorf("Expected error when getting nonexistent STR report")
	}
}

func TestInMemoryReportRepository_DeepCopyOnSave(t *testing.T) {
	// Test that saving deep-copies the report to prevent mutations.
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	original := &RBIQuarterlyReport{
		Quarter:           1,
		Year:              2024,
		ReportID:          "copy-test",
		TotalTransactions: 100,
		SuspiciousCount:   5,
	}

	repo.SaveRBIReport(ctx, original)

	// Mutate the original.
	original.TotalTransactions = 999
	original.SuspiciousCount = 888

	// Retrieve should have original values, not mutated.
	retrieved, _ := repo.GetRBIReport(ctx, "copy-test")
	if retrieved.TotalTransactions != 100 {
		t.Errorf("Retrieved report was mutated by original: got %d", retrieved.TotalTransactions)
	}
	if retrieved.SuspiciousCount != 5 {
		t.Errorf("Retrieved report suspicious count mutated: got %d", retrieved.SuspiciousCount)
	}
}

func TestInMemoryReportRepository_DeepCopyOnRetrieve(t *testing.T) {
	// Test that retrieving returns a deep copy.
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	report := &RBIQuarterlyReport{
		Quarter:           2,
		Year:              2024,
		ReportID:          "retrieve-copy",
		TotalTransactions: 200,
	}

	repo.SaveRBIReport(ctx, report)

	// Retrieve and mutate.
	retrieved, _ := repo.GetRBIReport(ctx, "retrieve-copy")
	retrieved.TotalTransactions = 777

	// Retrieve again; should have original value.
	retrieved2, _ := repo.GetRBIReport(ctx, "retrieve-copy")
	if retrieved2.TotalTransactions != 200 {
		t.Errorf("Second retrieve should have original value, got %d", retrieved2.TotalTransactions)
	}
}

func TestInMemoryReportRepository_QueryRBIReports_EmptyResult(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	// Query when repository is empty.
	results, err := repo.QueryRBIReports(ctx, time.Now().AddDate(-1, 0, 0), time.Now())
	if err != nil {
		t.Fatalf("QueryRBIReports failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Empty repository should return 0 results")
	}
}

func TestInMemoryReportRepository_QueryRBIReports_TimeFiltering(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)
	nextWeek := now.AddDate(0, 0, 7)

	// Report generated yesterday.
	reportYesterday := &RBIQuarterlyReport{
		Quarter:     1,
		Year:        2024,
		ReportID:    "rbi-yesterday",
		GeneratedAt: yesterday,
	}

	// Report generated today.
	reportToday := &RBIQuarterlyReport{
		Quarter:     2,
		Year:        2024,
		ReportID:    "rbi-today",
		GeneratedAt: now,
	}

	repo.SaveRBIReport(ctx, reportYesterday)
	repo.SaveRBIReport(ctx, reportToday)

	// Query for last 2 days.
	twoDaysAgo := now.AddDate(0, 0, -2)
	results, _ := repo.QueryRBIReports(ctx, twoDaysAgo, tomorrow)
	if len(results) != 2 {
		t.Errorf("Expected 2 reports in 2-day window, got %d", len(results))
	}

	// Query for next week (should be empty).
	results, _ = repo.QueryRBIReports(ctx, nextWeek, nextWeek.AddDate(0, 0, 1))
	if len(results) != 0 {
		t.Errorf("Query for next week should be empty, got %d", len(results))
	}
}

func TestInMemoryReportRepository_QuerySTRReports_TimeFiltering(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	// STR filed yesterday.
	strYesterday := &FATPSTRReport{
		STRID:       "str-yesterday",
		FilingDate:  yesterday,
		TransactionID: "txn1",
		Amount:      1_000_000,
	}

	// STR filed today.
	strToday := &FATPSTRReport{
		STRID:         "str-today",
		FilingDate:    now,
		TransactionID: "txn2",
		Amount:        1_000_000,
	}

	repo.SaveSTRReport(ctx, strYesterday)
	repo.SaveSTRReport(ctx, strToday)

	// Query for last 3 days.
	threeDaysAgo := now.AddDate(0, 0, -3)
	results, _ := repo.QuerySTRReports(ctx, threeDaysAgo, tomorrow)
	if len(results) != 2 {
		t.Errorf("Expected 2 STRs in 3-day window, got %d", len(results))
	}
}

func TestInMemoryReportRepository_Stats_Growth(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	stats := repo.Stats()
	if stats["rbi_reports"] != 0 || stats["str_reports"] != 0 {
		t.Errorf("Initial stats should be 0")
	}

	// Add RBI report.
	repo.SaveRBIReport(ctx, &RBIQuarterlyReport{ReportID: "rbi-1"})
	stats = repo.Stats()
	if stats["rbi_reports"] != 1 {
		t.Errorf("After adding 1 RBI, count should be 1, got %d", stats["rbi_reports"])
	}
	if stats["str_reports"] != 0 {
		t.Errorf("STR count should still be 0")
	}

	// Add STR report.
	repo.SaveSTRReport(ctx, &FATPSTRReport{STRID: "str-1", TransactionID: "txn1", Amount: 1})
	stats = repo.Stats()
	if stats["rbi_reports"] != 1 {
		t.Errorf("RBI count should still be 1")
	}
	if stats["str_reports"] != 1 {
		t.Errorf("After adding 1 STR, count should be 1, got %d", stats["str_reports"])
	}

	// Add more RBI reports.
	repo.SaveRBIReport(ctx, &RBIQuarterlyReport{ReportID: "rbi-2"})
	repo.SaveRBIReport(ctx, &RBIQuarterlyReport{ReportID: "rbi-3"})
	stats = repo.Stats()
	if stats["rbi_reports"] != 3 {
		t.Errorf("After adding 3 RBI total, count should be 3, got %d", stats["rbi_reports"])
	}
}

func TestInMemoryReportRepository_QueryByTransactionID_Empty(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	// Query for nonexistent transaction.
	results, err := repo.QueryByTransactionID(ctx, "nonexistent-txn")
	if err != nil {
		t.Fatalf("QueryByTransactionID failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Query for nonexistent transaction should return 0 results")
	}
}

func TestInMemoryReportRepository_QueryByTransactionID_Multiple(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	// Two STRs for the same transaction (shouldn't happen in practice, but test robustness).
	str1 := &FATPSTRReport{
		STRID:         "str-1",
		TransactionID: "txn-shared",
		Amount:        1_000_000,
	}
	str2 := &FATPSTRReport{
		STRID:         "str-2",
		TransactionID: "txn-shared",
		Amount:        2_000_000,
	}

	repo.SaveSTRReport(ctx, str1)
	repo.SaveSTRReport(ctx, str2)

	// Query for the shared transaction.
	results, _ := repo.QueryByTransactionID(ctx, "txn-shared")
	if len(results) != 2 {
		t.Errorf("Expected 2 results for shared transaction, got %d", len(results))
	}
}

func TestInMemoryReportRepository_All_Returns_Copies(t *testing.T) {
	// Test that All() returns deep copies.
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	report := &RBIQuarterlyReport{
		Quarter:           3,
		Year:              2024,
		ReportID:          "all-test",
		TotalTransactions: 500,
	}

	repo.SaveRBIReport(ctx, report)

	// Get all and mutate.
	allReports := repo.All()
	if len(allReports) > 0 {
		allReports[0].TotalTransactions = 999
	}

	// Retrieve original again.
	retrieved, _ := repo.GetRBIReport(ctx, "all-test")
	if retrieved.TotalTransactions != 500 {
		t.Errorf("Original should be unchanged after mutating All() results")
	}
}
