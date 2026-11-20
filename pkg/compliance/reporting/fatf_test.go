package reporting

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestGenerateSTR_BasicGeneration(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_001",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   15_000_000, // 150k INR
		Currency:      "INR",
		Description:   "High-value transfer",
		Merchant:      "International Distributor",
		Direction:     finance.DirectionDebit,
	}

	str, err := GenerateSTR(tx, []AuditEntry{}, 0.75)
	if err != nil {
		t.Fatalf("GenerateSTR failed: %v", err)
	}

	if str.TransactionID != "txn_001" {
		t.Errorf("STR should have correct transaction ID")
	}
	if str.Amount != 15_000_000 {
		t.Errorf("STR should preserve amount")
	}
	if str.Currency != "INR" {
		t.Errorf("STR should preserve currency")
	}
	if str.RiskScore != 0.75 {
		t.Errorf("STR should have correct risk score")
	}
	if str.RetentionYears != 7 {
		t.Errorf("STR retention years should always be 7, got %d", str.RetentionYears)
	}
}

func TestGenerateSTR_InvalidTransaction(t *testing.T) {
	// Missing transaction ID.
	tx := finance.Transaction{
		TransactionID: "",
		AmountCents:   1_000_000,
	}

	_, err := GenerateSTR(tx, []AuditEntry{}, 0.5)
	if err == nil {
		t.Errorf("Expected error for missing transaction ID, got nil")
	}

	// Zero or negative amount.
	tx.TransactionID = "txn_001"
	tx.AmountCents = 0
	_, err = GenerateSTR(tx, []AuditEntry{}, 0.5)
	if err == nil {
		t.Errorf("Expected error for zero amount, got nil")
	}
}

func TestGenerateSTR_HighValueIndicator(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_high_value",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   11_000_000, // Above threshold
		Currency:      "INR",
		Merchant:      "Store",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.8)

	hasHighValueIndicator := false
	for _, ind := range str.RiskIndicators {
		if strings.Contains(ind, "high_value") {
			hasHighValueIndicator = true
			break
		}
	}

	if !hasHighValueIndicator {
		t.Errorf("STR should identify high-value transaction")
	}
}

func TestGenerateSTR_AuditTrailExtraction(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_audited",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store",
	}

	auditLog := []AuditEntry{
		{
			Seq:        1,
			OccurredAt: time.Now(),
			Actor:      "aml_agent",
			Action:     "aml.assessed",
			Target:     "txn_audited",
			Details: map[string]any{
				"risk_score": 0.65,
				"reason":     "pattern_match",
			},
		},
		{
			Seq:        2,
			OccurredAt: time.Now().Add(1 * time.Minute),
			Actor:      "compliance_officer",
			Action:     "aml.reviewed",
			Target:     "txn_audited",
			Details: map[string]any{
				"risk_score": 0.72,
				"decision":   "suspicious",
			},
		},
	}

	str, _ := GenerateSTR(tx, auditLog, 0.72)

	if len(str.AuditTrail) < 2 {
		t.Errorf("STR should capture audit trail, got %d entries", len(str.AuditTrail))
	}

	// Verify chronological order.
	if str.AuditTrail[0].Action != "aml.assessed" {
		t.Errorf("First audit entry should be aml.assessed")
	}
	if str.AuditTrail[1].Action != "aml.reviewed" {
		t.Errorf("Second audit entry should be aml.reviewed")
	}
}

func TestGenerateSTR_PartyExtraction(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_parties",
		AccountID:     "acct_john_doe",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Merchant XYZ",
		Direction:     finance.DirectionDebit,
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.5)

	if str.Parties.Sender.ID != "acct_john_doe" {
		t.Errorf("Sender should be the account ID")
	}
	if !strings.Contains(str.Parties.Receiver.Name, "Merchant") {
		t.Errorf("Receiver should be the merchant")
	}
}

func TestFileTTR_TemplateStructure(t *testing.T) {
	txns := []finance.Transaction{
		{
			TransactionID: "txn1",
			AmountCents:   15_000_000,
			Currency:      "INR",
		},
		{
			TransactionID: "txn2",
			AmountCents:   20_000_000,
			Currency:      "INR",
		},
	}

	template := FileTTR(txns)

	if template["report_type"] != "TTR" {
		t.Errorf("Template should have report_type=TTR")
	}
	if template["transaction_count"] != 2 {
		t.Errorf("Template should have correct transaction count")
	}
	if template["retention_years"] != 7 {
		t.Errorf("TTR retention should be 7 years")
	}

	vol := template["total_volume_paise"]
	if vol != int64(35_000_000) {
		t.Errorf("Template should sum volumes correctly, got %v (expected 35000000)", vol)
	}
}

func TestExportSTRAsJSON(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_json",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   2_000_000,
		Currency:      "INR",
		Merchant:      "Test Store",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.6)

	data, err := ExportSTRAsJSON(str)
	if err != nil {
		t.Fatalf("ExportSTRAsJSON failed: %v", err)
	}

	// Verify it's valid JSON.
	var parsed FATPSTRReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.TransactionID != "txn_json" {
		t.Errorf("JSON roundtrip failed: transaction ID mismatch")
	}
}

func TestExportSTRAsXML(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_xml",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   3_000_000,
		Currency:      "INR",
		Merchant:      "Test Store",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.7)

	data, err := ExportSTRAsXML(str)
	if err != nil {
		t.Fatalf("ExportSTRAsXML failed: %v", err)
	}

	xml := string(data)

	// Verify XML structure.
	if !strings.Contains(xml, "<?xml") {
		t.Errorf("XML should have declaration")
	}
	if !strings.Contains(xml, "<STR>") || !strings.Contains(xml, "</STR>") {
		t.Errorf("XML should have STR root element")
	}
	if !strings.Contains(xml, "<TransactionID>") {
		t.Errorf("XML should contain TransactionID element")
	}
	if !strings.Contains(xml, "<RiskScore>") {
		t.Errorf("XML should contain RiskScore element")
	}
}

func TestExportSTRAsXML_Escaping(t *testing.T) {
	// Test that XML special characters are escaped.
	tx := finance.Transaction{
		TransactionID: "txn_special",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store & Co. <Ltd>",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.5)

	data, _ := ExportSTRAsXML(str)
	xml := string(data)

	// Verify escaping.
	if strings.Contains(xml, "<Ltd>") || strings.Contains(xml, "&") && !strings.Contains(xml, "&amp;") {
		t.Errorf("XML should escape special characters")
	}
}

func TestInMemoryReportRepository_SaveAndRetrieveSTR(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	tx := finance.Transaction{
		TransactionID: "txn_repo",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.5)

	// Save.
	strID, err := repo.SaveSTRReport(ctx, str)
	if err != nil {
		t.Fatalf("SaveSTRReport failed: %v", err)
	}

	// Retrieve.
	retrieved, err := repo.GetSTRReport(ctx, strID)
	if err != nil {
		t.Fatalf("GetSTRReport failed: %v", err)
	}

	if retrieved.TransactionID != "txn_repo" {
		t.Errorf("Retrieved STR has wrong transaction ID")
	}
}

func TestInMemoryReportRepository_QueryByTransactionID(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	tx := finance.Transaction{
		TransactionID: "txn_query",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.5)
	repo.SaveSTRReport(ctx, str)

	// Query by transaction ID.
	results, err := repo.QueryByTransactionID(ctx, "txn_query")
	if err != nil {
		t.Fatalf("QueryByTransactionID failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for txn_query, got %d", len(results))
	}
}

func TestInMemoryReportRepository_QuerySTRByDateRange(t *testing.T) {
	repo := NewInMemoryReportRepository()
	ctx := context.Background()

	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	tx1 := finance.Transaction{
		TransactionID: "txn1",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store",
	}

	tx2 := finance.Transaction{
		TransactionID: "txn2",
		AccountID:     "acct_123",
		Date:          "2024-05-30",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store",
	}

	str1, _ := GenerateSTR(tx1, []AuditEntry{}, 0.5)
	str2, _ := GenerateSTR(tx2, []AuditEntry{}, 0.5)

	repo.SaveSTRReport(ctx, str1)
	repo.SaveSTRReport(ctx, str2)

	// Query for last 5 days.
	fiveDaysAgo := now.AddDate(0, 0, -5)
	results, err := repo.QuerySTRReports(ctx, fiveDaysAgo, tomorrow)
	if err != nil {
		t.Fatalf("QuerySTRReports failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 STRs in range, got %d", len(results))
	}
}

func TestSTRNarrative_Content(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_narrative",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   20_000_000,
		Currency:      "INR",
		Merchant:      "International Trader",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.85)

	if !strings.Contains(str.ReasonForSuspicion, "txn_narrative") {
		t.Errorf("Narrative should reference transaction ID")
	}
	if !strings.Contains(str.ReasonForSuspicion, "FATF") {
		t.Errorf("Narrative should reference FATF")
	}
	if !strings.Contains(str.ReasonForSuspicion, "RBI") {
		t.Errorf("Narrative should reference RBI")
	}
}

func TestSTR_Status_Pending(t *testing.T) {
	tx := finance.Transaction{
		TransactionID: "txn_status",
		AccountID:     "acct_123",
		Date:          "2024-05-31",
		AmountCents:   1_000_000,
		Currency:      "INR",
		Merchant:      "Store",
	}

	str, _ := GenerateSTR(tx, []AuditEntry{}, 0.5)

	if str.Status != "pending" {
		t.Errorf("New STR should have status 'pending', got %q", str.Status)
	}
}

func TestQueryRoleForRetention_SevenYears(t *testing.T) {
	// Placeholder test: verify the retention query references 7 years.
	results, err := QueryRoleForRetention(nil, 7)
	if err != nil {
		t.Fatalf("QueryRoleForRetention failed: %v", err)
	}

	if len(results) == 0 {
		t.Errorf("QueryRoleForRetention should return query instructions")
	}

	// Check that the result mentions retention period.
	resultStr := strings.Join(results, " ")
	if !strings.Contains(resultStr, "7") {
		t.Errorf("Query result should mention 7-year retention")
	}
}
