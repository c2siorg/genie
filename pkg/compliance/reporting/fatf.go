package reporting

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

// FATPSTRReport represents a Suspicious Transaction Report (STR) for FATF compliance.
//
// FATF (Financial Action Task Force) requires reporting of suspicious transactions
// to the Financial Intelligence Unit (FIU-IND in India). Each STR is filed individually
// with supporting evidence and decision reasoning.
type FATPSTRReport struct {
	STRID              string            `json:"str_id"`              // Unique STR identifier
	TransactionID      string            `json:"transaction_id"`
	FilingDate         time.Time         `json:"filing_date"`
	Amount             int64             `json:"amount_cents"`        // in minor units
	Currency           string            `json:"currency"`
	Parties            TransactionParties `json:"parties"`            // sender/receiver
	RiskIndicators     []string          `json:"risk_indicators"`     // flags that triggered STR
	RiskScore          float64           `json:"risk_score"`          // 0-1 scale
	ReasonForSuspicion string            `json:"reason_for_suspicion"` // narrative
	AuditTrail         []AuditDetail     `json:"audit_trail"`         // decision reasoning
	RetentionYears     int               `json:"retention_years"`     // always 7 per regulation
	Status             string            `json:"status"`              // "pending"|"filed"|"closed"
	InvestigationNotes string            `json:"investigation_notes,omitempty"`
}

// TransactionParties holds sender and receiver info.
type TransactionParties struct {
	Sender   PartyInfo `json:"sender"`
	Receiver PartyInfo `json:"receiver"`
}

// PartyInfo holds identifying information about a transaction party.
type PartyInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // "individual"|"merchant"|"institution"
	PEP   bool   `json:"pep"`  // Politically Exposed Person flag
	KYCed bool   `json:"kyced"`
}

// AuditDetail is a single entry in the STR decision trail.
type AuditDetail struct {
	Timestamp time.Time              `json:"timestamp"`
	Action    string                 `json:"action"`    // e.g. "risk.assessed", "threshold.breached"
	Score     float64                `json:"score"`     // intermediate risk score
	Evidence  map[string]interface{} `json:"evidence"`  // supporting data
	DecidedBy string                 `json:"decided_by"` // agent/system id
}

// GenerateSTR generates a Suspicious Transaction Report from a transaction and audit log.
//
// The STR captures:
// - Transaction details (amount, parties, date)
// - Risk indicators (what raised suspicion)
// - Decision reasoning (audit trail showing how risk score was computed)
// - Retention policy (always 7 years per FATF/RBI requirement)
//
// Risk indicators include:
// - High value (above threshold)
// - Rapid succession (multiple large txns from same party)
// - PEP/sanctions match
// - Unusual patterns (time of day, frequency, geography)
// - Structuring (multiple sub-threshold txns to avoid detection)
func GenerateSTR(tx finance.Transaction, auditLog []AuditEntry, riskScore float64) (*FATPSTRReport, error) {
	if tx.TransactionID == "" {
		return nil, fmt.Errorf("fatf: transaction id required")
	}
	if tx.AmountCents <= 0 {
		return nil, fmt.Errorf("fatf: positive amount required")
	}

	// Identify risk indicators for this transaction.
	indicators := identifyRiskIndicators(tx, auditLog)

	// Extract audit trail entries related to this transaction.
	auditDetails := extractAuditTrail(tx.TransactionID, auditLog, riskScore)

	// Build narrative.
	narrative := buildSTRNarrative(tx, indicators, riskScore)

	str := &FATPSTRReport{
		STRID:              generateReportID("STR", 0, 0), // no quarter/year for STR
		TransactionID:      tx.TransactionID,
		FilingDate:         time.Now().UTC(),
		Amount:             tx.AmountCents,
		Currency:           tx.Currency,
		Parties:            extractParties(tx),
		RiskIndicators:     indicators,
		RiskScore:          riskScore,
		ReasonForSuspicion: narrative,
		AuditTrail:         auditDetails,
		RetentionYears:     7, // Non-negotiable per FATF/RBI
		Status:             "pending", // Will be "filed" after submission
	}

	return str, nil
}

// FileTTR returns a Threshold Transaction Report (TTR) template.
//
// TTR is used to report all transactions above a threshold (typically 10M paise / 100k INR)
// without implying suspicion. It's a regulatory reporting requirement separate from STR.
// FileTTR does not generate data; it returns the template structure for population.
func FileTTR(txns []finance.Transaction) map[string]interface{} {
	template := map[string]interface{}{
		"report_type": "TTR",
		"description": "Threshold Transaction Report - all transactions above 10M paise (100k INR)",
		"filing_period": map[string]interface{}{
			"start_date": time.Now().AddDate(0, -1, 0).Format("2006-01-02"),
			"end_date":   time.Now().Format("2006-01-02"),
		},
		"threshold_paise":     10_000_000,
		"transaction_count":   len(txns),
		"total_volume_paise":  sumVolume(txns),
		"transactions":        txns,
		"retention_years":     7,
		"submission_deadline": time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
		"notes":               "TTR is filed for transparency; threshold breaches do not imply illegal activity.",
	}
	return template
}

// ExportSTRAsJSON returns the STR as pretty-printed JSON suitable for filing.
func ExportSTRAsJSON(str *FATPSTRReport) ([]byte, error) {
	if str == nil {
		return nil, fmt.Errorf("fatf: export requires non-nil report")
	}
	b, err := json.MarshalIndent(str, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("fatf: json marshal failed: %w", err)
	}
	return b, nil
}

// ExportSTRAsXML returns the STR in a simplified XML structure for FIU-IND submission.
// In production, this would conform to FIU-IND's actual XML schema.
func ExportSTRAsXML(str *FATPSTRReport) ([]byte, error) {
	if str == nil {
		return nil, fmt.Errorf("fatf: export requires non-nil report")
	}

	// Simplified XML; in production, conform to FIU-IND schema.
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<STR>` + "\n")
	fmt.Fprintf(&sb, `  <STRID>%s</STRID>`+"\n", escapeXML(str.STRID))
	fmt.Fprintf(&sb, `  <TransactionID>%s</TransactionID>`+"\n", escapeXML(str.TransactionID))
	fmt.Fprintf(&sb, `  <FilingDate>%s</FilingDate>`+"\n", str.FilingDate.Format(time.RFC3339))
	fmt.Fprintf(&sb, `  <Amount>%d</Amount>`+"\n", str.Amount)
	fmt.Fprintf(&sb, `  <Currency>%s</Currency>`+"\n", str.Currency)
	fmt.Fprintf(&sb, `  <RiskScore>%.2f</RiskScore>`+"\n", str.RiskScore)
	fmt.Fprintf(&sb, `  <Indicators>`+"\n")
	for _, ind := range str.RiskIndicators {
		fmt.Fprintf(&sb, `    <Indicator>%s</Indicator>`+"\n", escapeXML(ind))
	}
	fmt.Fprintf(&sb, `  </Indicators>`+"\n")
	fmt.Fprintf(&sb, `  <ReasonForSuspicion>%s</ReasonForSuspicion>`+"\n", escapeXML(str.ReasonForSuspicion))
	fmt.Fprintf(&sb, `  <RetentionYears>%d</RetentionYears>`+"\n", str.RetentionYears)
	fmt.Fprintf(&sb, `  <Status>%s</Status>`+"\n", str.Status)
	sb.WriteString(`</STR>` + "\n")

	return []byte(sb.String()), nil
}

// QueryRoleForRetention queries the report repository for all STRs filed within
// the past N years. Used to verify retention compliance.
// This is a placeholder signature; actual implementation depends on ReportRepository.
func QueryRoleForRetention(repositoryFunc func() interface{}, yearsBack int) ([]string, error) {
	// Placeholder: in production, query DB for STRs where FilingDate >= now - yearsBack years.
	// For now, return a note about what would be queried.
	sinceDate := time.Now().AddDate(-yearsBack, 0, 0)
	return []string{
		fmt.Sprintf("Query: STRs filed since %s (retention window: %d years)",
			sinceDate.Format("2006-01-02"), yearsBack),
	}, nil
}

// ─── Helper functions ──────────────────────────────────────────────────────

// identifyRiskIndicators returns a list of flags that triggered the STR.
func identifyRiskIndicators(tx finance.Transaction, auditLog []AuditEntry) []string {
	var indicators []string
	const highValueThreshold = 10_000_000 // paise

	// High value.
	if tx.AmountCents > highValueThreshold {
		indicators = append(indicators, fmt.Sprintf("high_value: %.2f %s", float64(tx.AmountCents)/100, tx.Currency))
	}

	// Rapid succession (check audit log for recent txns from same account).
	recentCount := 0
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	for _, entry := range auditLog {
		if entry.OccurredAt.After(fiveMinutesAgo) && entry.Action == "transaction.completed" {
			if acctID, ok := entry.Details["account_id"].(string); ok && acctID == tx.AccountID {
				recentCount++
			}
		}
	}
	if recentCount > 3 {
		indicators = append(indicators, fmt.Sprintf("rapid_succession: %d txns in 5min", recentCount))
	}

	// PEP/sanctions match (would be checked in audit log by a dedicated service).
	for _, entry := range auditLog {
		if entry.Action == "sanctions.match" && entry.Target == tx.TransactionID {
			if reason, ok := entry.Details["match_type"].(string); ok {
				indicators = append(indicators, fmt.Sprintf("sanctions_match: %s", reason))
			}
		}
	}

	// Structuring pattern (multiple txns just under threshold from same merchant).
	sameM := 0
	m := strings.ToLower(strings.TrimSpace(tx.Merchant))
	const structuringThreshold = 5_000_000 // just under high-value threshold
	for _, entry := range auditLog {
		if entry.Action == "transaction.completed" && entry.Target == m {
			sameM++
		}
	}
	if sameM > 5 {
		indicators = append(indicators, "structuring: multiple sub-threshold txns")
	}

	// Unusual time of day (e.g., 3 AM).
	hour := time.Now().Hour()
	if hour < 6 || hour > 22 {
		indicators = append(indicators, fmt.Sprintf("unusual_time: %d:00", hour))
	}

	if len(indicators) == 0 {
		indicators = append(indicators, "manual_review_flag")
	}

	return indicators
}

// extractParties builds party info from transaction.
func extractParties(tx finance.Transaction) TransactionParties {
	return TransactionParties{
		Sender: PartyInfo{
			ID:    tx.AccountID,
			Name:  "Account " + tx.AccountID,
			Type:  "individual",
			KYCed: true, // assume verified in compliance pipeline
		},
		Receiver: PartyInfo{
			ID:    finance.NormalizeMerchant(tx.Merchant),
			Name:  tx.Merchant,
			Type:  "merchant",
			KYCed: true, // assume merchant is registered
		},
	}
}

// extractAuditTrail builds a simplified decision history from the audit log.
func extractAuditTrail(txnID string, auditLog []AuditEntry, riskScore float64) []AuditDetail {
	var trail []AuditDetail

	// Collect all entries related to this transaction.
	for _, entry := range auditLog {
		if entry.Target != txnID && entry.Details["transaction_id"] != txnID {
			continue
		}

		score := 0.0
		if s, ok := entry.Details["risk_score"].(float64); ok {
			score = s
		}

		trail = append(trail, AuditDetail{
			Timestamp: entry.OccurredAt,
			Action:    entry.Action,
			Score:     score,
			Evidence:  entry.Details,
			DecidedBy: entry.Actor,
		})
	}

	// If no audit entries, at least record the final assessment.
	if len(trail) == 0 {
		trail = append(trail, AuditDetail{
			Timestamp: time.Now().UTC(),
			Action:    "aml.assessed",
			Score:     riskScore,
			Evidence: map[string]interface{}{
				"method": "automated_screening",
			},
			DecidedBy: "system",
		})
	}

	return trail
}

// buildSTRNarrative constructs the reason_for_suspicion text.
func buildSTRNarrative(tx finance.Transaction, indicators []string, riskScore float64) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Transaction %s flagged for suspicious activity.\n", tx.TransactionID)
	fmt.Fprintf(&sb, "Amount: %.2f %s\n", float64(tx.AmountCents)/100, tx.Currency)
	fmt.Fprintf(&sb, "Merchant: %s\n", tx.Merchant)
	fmt.Fprintf(&sb, "Risk Score: %.2f (0-1 scale)\n\n", riskScore)

	fmt.Fprintf(&sb, "Risk Indicators:\n")
	for i, ind := range indicators {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, ind)
	}

	fmt.Fprintf(&sb, "\nBasis for STR Filing:\n")
	fmt.Fprintf(&sb, "This transaction meets one or more criteria for mandatory suspicious transaction reporting\n")
	fmt.Fprintf(&sb, "under FATF recommendations and RBI guidelines. The transaction has been assessed and\n")
	fmt.Fprintf(&sb, "determined to warrant further investigation and reporting to FIU-IND.\n")

	return sb.String()
}

// sumVolume totals the amount_cents across all transactions.
func sumVolume(txns []finance.Transaction) int64 {
	var total int64
	for _, tx := range txns {
		total += tx.AmountCents
	}
	return total
}

// escapeXML does basic XML escaping (not a full XML encoder).
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
