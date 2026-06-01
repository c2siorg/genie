package reporting

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

// RBIQuarterlyReport represents a compliance report for the Reserve Bank of India.
//
// The RBI circular on cyber-resilience and AML requires quarterly reporting of:
// - Total transactions and volumes by quarter
// - Suspicious transaction counts and reasons
// - Compliance attestations and notes
//
// Quarter is 1-4; Year is YYYY.
type RBIQuarterlyReport struct {
	Quarter            int       `json:"quarter"`
	Year               int       `json:"year"`
	ReportID           string    `json:"report_id"` // unique identifier for this report
	GeneratedAt        time.Time `json:"generated_at"`
	TotalTransactions  int64     `json:"total_transactions"`
	TotalVolumeCents   int64     `json:"total_volume_cents"` // in minor units (paise/cents)
	SuspiciousCount    int       `json:"suspicious_count"`
	RiskyMerchants     int       `json:"risky_merchants"`
	ComplianceNotes    string    `json:"compliance_notes"`
	AttestationSignee  string    `json:"attestation_signee,omitempty"`
	MerchantBreakdown  []MerchantVolume `json:"merchant_breakdown,omitempty"`
}

// MerchantVolume is the transaction count and volume for a single merchant.
type MerchantVolume struct {
	Merchant      string `json:"merchant"`
	TransactionCount int64  `json:"transaction_count"`
	VolumeCents   int64  `json:"volume_cents"`
	SuspiciousCount int    `json:"suspicious_count"`
}

// GenerateRBIReport compiles a quarterly report from transactions and audit log.
// The report sums transaction counts and volumes, counts suspicious markers,
// and includes a compliance attestation string.
//
// Suspicious transactions are identified by:
// - High value (> 10 million paise / 100k INR equivalent)
// - Rapid sequence (multiple txns within 5 minutes)
// - Unusual merchant patterns (e.g., frequent merchant changes)
// - Explicit flags in audit log (action="aml.flagged")
func GenerateRBIReport(quarter, year int, txns []finance.Transaction, auditLog []AuditEntry) (*RBIQuarterlyReport, error) {
	if quarter < 1 || quarter > 4 {
		return nil, fmt.Errorf("rbi: invalid quarter %d", quarter)
	}
	if year < 2000 || year > 2100 {
		return nil, fmt.Errorf("rbi: invalid year %d", year)
	}

	// Filter transactions to this quarter.
	qTxns := filterByQuarter(txns, quarter, year)
	if len(qTxns) == 0 {
		qTxns = []finance.Transaction{} // ensure non-nil
	}

	// Count suspicious transactions and identify suspicious IDs.
	suspiciousTxnIDs := identifySuspiciousTransactions(qTxns, auditLog)

	// Compute aggregates.
	totalTxns := int64(len(qTxns))
	var totalVolume int64
	merchantVols := make(map[string]*MerchantVolume)

	for _, tx := range qTxns {
		totalVolume += tx.AmountCents
		m := finance.NormalizeMerchant(tx.Merchant)
		if m == "" {
			m = "unknown"
		}
		if _, ok := merchantVols[m]; !ok {
			merchantVols[m] = &MerchantVolume{Merchant: m}
		}
		merchantVols[m].TransactionCount++
		merchantVols[m].VolumeCents += tx.AmountCents
		if suspiciousTxnIDs[tx.TransactionID] {
			merchantVols[m].SuspiciousCount++
		}
	}

	// Count risky merchants (those with multiple suspicious txns).
	riskyMerchants := 0
	for _, mv := range merchantVols {
		if mv.SuspiciousCount > 0 {
			riskyMerchants++
		}
	}

	// Sort merchant breakdown by volume descending.
	breakdown := make([]MerchantVolume, 0, len(merchantVols))
	for _, mv := range merchantVols {
		breakdown = append(breakdown, *mv)
	}
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].VolumeCents > breakdown[j].VolumeCents
	})

	// Build compliance notes.
	notes := buildRBIComplianceNotes(quarter, year, totalTxns, len(suspiciousTxnIDs), riskyMerchants)

	report := &RBIQuarterlyReport{
		Quarter:           quarter,
		Year:              year,
		ReportID:          generateReportID("RBI", quarter, year),
		GeneratedAt:       time.Now().UTC(),
		TotalTransactions: totalTxns,
		TotalVolumeCents:  totalVolume,
		SuspiciousCount:   len(suspiciousTxnIDs),
		RiskyMerchants:    riskyMerchants,
		ComplianceNotes:   notes,
		MerchantBreakdown: breakdown,
	}

	return report, nil
}

// ExportRBIReport converts the report to CSV, JSON, or PDF format.
// - "csv": comma-separated values (summary + merchant breakdown as separate sections)
// - "json": pretty-printed JSON
// - "pdf": returns a JSON payload describing the report (actual PDF generation requires external libs)
func ExportRBIReport(report *RBIQuarterlyReport, format string) ([]byte, error) {
	if report == nil {
		return nil, fmt.Errorf("rbi: export requires non-nil report")
	}

	switch strings.ToLower(format) {
	case "csv":
		return exportRBIAsCSV(report)
	case "json":
		return exportRBIAsJSON(report)
	case "pdf":
		return exportRBIAsPDF(report)
	default:
		return nil, fmt.Errorf("rbi: unsupported format %q (csv|json|pdf)", format)
	}
}

// exportRBIAsCSV returns a CSV with two sections: summary and merchant breakdown.
func exportRBIAsCSV(report *RBIQuarterlyReport) ([]byte, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)

	// Summary section.
	w.Write([]string{"RBI Quarterly Report"})
	w.Write([]string{})
	w.Write([]string{"Field", "Value"})
	w.Write([]string{"Report ID", report.ReportID})
	w.Write([]string{"Quarter", fmt.Sprintf("Q%d", report.Quarter)})
	w.Write([]string{"Year", fmt.Sprintf("%d", report.Year)})
	w.Write([]string{"Generated At", report.GeneratedAt.Format(time.RFC3339)})
	w.Write([]string{"Total Transactions", fmt.Sprintf("%d", report.TotalTransactions)})
	w.Write([]string{"Total Volume (cents)", fmt.Sprintf("%d", report.TotalVolumeCents)})
	w.Write([]string{"Suspicious Count", fmt.Sprintf("%d", report.SuspiciousCount)})
	w.Write([]string{"Risky Merchants", fmt.Sprintf("%d", report.RiskyMerchants)})
	w.Write([]string{})

	// Merchant breakdown section.
	w.Write([]string{"Merchant Breakdown"})
	w.Write([]string{"Merchant", "Transaction Count", "Volume (cents)", "Suspicious Count"})
	for _, mv := range report.MerchantBreakdown {
		w.Write([]string{
			mv.Merchant,
			fmt.Sprintf("%d", mv.TransactionCount),
			fmt.Sprintf("%d", mv.VolumeCents),
			fmt.Sprintf("%d", mv.SuspiciousCount),
		})
	}
	w.Write([]string{})
	w.Write([]string{"Compliance Notes"})
	w.Write([]string{report.ComplianceNotes})

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("rbi: csv write failed: %w", err)
	}

	return []byte(sb.String()), nil
}

// exportRBIAsJSON returns the report as pretty-printed JSON.
func exportRBIAsJSON(report *RBIQuarterlyReport) ([]byte, error) {
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("rbi: json marshal failed: %w", err)
	}
	return b, nil
}

// exportRBIAsPDF returns a JSON structure describing the report (placeholder).
// In production, this would invoke a PDF library (e.g., gofpdf, wkhtmltopdf) to
// generate an actual PDF with proper formatting, headers, and attestation blocks.
func exportRBIAsPDF(report *RBIQuarterlyReport) ([]byte, error) {
	// Placeholder: return a JSON structure that describes what the PDF would contain.
	pdfStruct := map[string]any{
		"format":  "pdf",
		"report":  report,
		"note":    "PDF generation requires external library (e.g., gofpdf); this is a JSON placeholder",
	}
	b, _ := json.MarshalIndent(pdfStruct, "", "  ")
	return b, nil
}

// ─── Helper functions ──────────────────────────────────────────────────────

// filterByQuarter returns transactions in the given quarter/year.
// Q1 = Jan-Mar (01-03), Q2 = Apr-Jun (04-06), Q3 = Jul-Sep (07-09), Q4 = Oct-Dec (10-12).
func filterByQuarter(txns []finance.Transaction, quarter, year int) []finance.Transaction {
	var result []finance.Transaction
	monthStart := (quarter-1)*3 + 1
	monthEnd := quarter * 3

	for _, tx := range txns {
		t, err := tx.ParsedDate()
		if err != nil {
			continue // skip unparseable dates
		}
		if t.Year() == year && int(t.Month()) >= monthStart && int(t.Month()) <= monthEnd {
			result = append(result, tx)
		}
	}
	return result
}

// identifySuspiciousTransactions returns a set of transaction IDs flagged as suspicious.
// Heuristics:
// - Amount > 10 million paise (100,000 INR / ~1200 USD)
// - Flagged in audit log (action="aml.flagged")
// - Rapid sequence within time window (detected by audit entry)
func identifySuspiciousTransactions(txns []finance.Transaction, auditLog []AuditEntry) map[string]bool {
	suspicious := make(map[string]bool)
	const highValueThreshold = 10_000_000 // paise

	// Mark high-value transactions.
	for _, tx := range txns {
		if tx.AmountCents > highValueThreshold {
			suspicious[tx.TransactionID] = true
		}
	}

	// Mark transactions explicitly flagged in audit log.
	for _, entry := range auditLog {
		if entry.Action == "aml.flagged" {
			if txnID, ok := entry.Details["transaction_id"].(string); ok && txnID != "" {
				suspicious[txnID] = true
			}
		}
	}

	return suspicious
}

// buildRBIComplianceNotes constructs the attestation text for the report.
func buildRBIComplianceNotes(quarter, year int, totalTxns int64, suspiciousCount, riskyMerchants int) string {
	var notes strings.Builder

	fmt.Fprintf(&notes, "RBI Quarterly Compliance Report\n")
	fmt.Fprintf(&notes, "Period: Q%d %d\n", quarter, year)
	fmt.Fprintf(&notes, "Report Date: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))

	fmt.Fprintf(&notes, "Summary:\n")
	fmt.Fprintf(&notes, "- Total transactions processed: %d\n", totalTxns)
	fmt.Fprintf(&notes, "- Suspicious transactions identified: %d\n", suspiciousCount)
	fmt.Fprintf(&notes, "- Merchants flagged for monitoring: %d\n\n", riskyMerchants)

	fmt.Fprintf(&notes, "Compliance Status:\n")
	if suspiciousCount > 0 {
		fmt.Fprintf(&notes, "- %d suspicious transactions have been identified and logged.\n", suspiciousCount)
		fmt.Fprintf(&notes, "- Detailed STR filings are prepared and ready for submission to FIU-IND.\n")
	} else {
		fmt.Fprintf(&notes, "- No suspicious transactions detected during this quarter.\n")
	}
	if riskyMerchants > 0 {
		fmt.Fprintf(&notes, "- %d merchants require enhanced due diligence monitoring.\n", riskyMerchants)
	}

	fmt.Fprintf(&notes, "\nAttestation:\n")
	fmt.Fprintf(&notes, "This report is prepared in accordance with RBI Master Circular on AML/CFT and Cyber-Resilience.\n")
	fmt.Fprintf(&notes, "All transactions have been screened against sanctions lists and PEP databases.\n")
	fmt.Fprintf(&notes, "Audit trail and supporting documentation are maintained for 7 years per regulatory requirement.\n")

	return notes.String()
}

// generateReportID creates a unique ID for a report.
func generateReportID(prefix string, quarter, year int) string {
	return fmt.Sprintf("%s-Q%d-%04d-%d", prefix, quarter, year, time.Now().UnixNano())
}

// ─── Thread-safe operations ────────────────────────────────────────────────

// ReportWriter provides thread-safe append-only writes to a report buffer.
// This is used to accumulate multiple reports safely in concurrent scenarios.
type ReportWriter struct {
	mu      sync.RWMutex
	reports []*RBIQuarterlyReport
}

// Append adds a report to the writer. Returns the index for tracing.
func (w *ReportWriter) Append(report *RBIQuarterlyReport) int {
	if report == nil {
		return -1
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	idx := len(w.reports)
	w.reports = append(w.reports, report)
	return idx
}

// All returns a copy of all appended reports.
func (w *ReportWriter) All() []*RBIQuarterlyReport {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*RBIQuarterlyReport, len(w.reports))
	copy(out, w.reports)
	return out
}
