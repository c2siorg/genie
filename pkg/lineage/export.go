package lineage

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"
)

// ─── Export format constants ───────────────────────────────────────────────────

const (
	ExportFormatCSV  = "csv"
	ExportFormatJSON = "json"
)

// ─── Export functions ─────────────────────────────────────────────────────────

// ExportOptions controls lineage export behavior.
type ExportOptions struct {
	// Format is "csv" or "json"
	Format string
	// Since filters to entries at or after this time (zero = no lower bound)
	Since time.Time
	// Until filters to entries before this time (zero = no upper bound)
	Until time.Time
	// IncludeHash includes the hash and prev_hash fields (for audit chains)
	IncludeHash bool
}

// Export retrieves lineage entries and exports them in the requested format.
//
// CSV format is ideal for regulatory queries and Excel import.
// JSON format preserves all fields and is suitable for programmatic processing.
//
// Returns (data, error). Data is a string containing the export.
func (m *Manager) Export(ctx context.Context, opts ExportOptions) (string, error) {
	if opts.Format == "" {
		opts.Format = ExportFormatCSV
	}

	// Query entries
	q := LineageQuery{
		Since: opts.Since,
		Until: opts.Until,
		Limit: 100000, // reasonable cap for exports
	}
	entries, err := m.recorder.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("lineage: export query failed: %w", err)
	}

	switch opts.Format {
	case ExportFormatCSV:
		return m.exportCSV(entries, opts)
	case ExportFormatJSON:
		return m.exportJSON(entries)
	default:
		return "", fmt.Errorf("lineage: unsupported format: %q", opts.Format)
	}
}

// exportCSV returns a CSV representation of the entries.
func (m *Manager) exportCSV(entries []*LineageEntry, opts ExportOptions) (string, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	// Write header
	header := []string{
		"ID",
		"Timestamp",
		"UserID",
		"ResourceID",
		"ResourceType",
		"Action",
		"Decision",
		"ReasonCode",
		"PolicyRule",
		"AgentID",
		"SessionID",
		"TraceID",
	}
	if opts.IncludeHash {
		header = append(header, []string{"Hash", "PrevHash"}...)
	}

	if err := w.Write(header); err != nil {
		return "", fmt.Errorf("lineage: csv write header failed: %w", err)
	}

	// Write entries
	for _, entry := range entries {
		row := []string{
			entry.ID,
			entry.Timestamp.Format(time.RFC3339),
			entry.UserID,
			entry.ResourceID,
			entry.ResourceType,
			string(entry.Action),
			string(entry.Decision),
			entry.ReasonCode,
			entry.PolicyRule,
			entry.AgentID,
			entry.SessionID,
			entry.TraceID,
		}
		if opts.IncludeHash {
			row = append(row, []string{entry.Hash, entry.PrevHash}...)
		}

		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("lineage: csv write row failed: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("lineage: csv flush failed: %w", err)
	}

	return buf.String(), nil
}

// exportJSON returns a JSON representation of the entries.
func (m *Manager) exportJSON(entries []*LineageEntry) (string, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("lineage: json marshal failed: %w", err)
	}
	return string(data), nil
}

// ─── Lineage verification and integrity checking ────────────────────────────

// VerifyLineageIntegrity validates the hash chain for a time range.
//
// Computes expected hashes for all entries in the range and reports any breaks.
// Returns LineageIntegrityResult with details.
func (m *Manager) VerifyLineageIntegrity(
	ctx context.Context,
	since time.Time,
	until time.Time,
) LineageIntegrityResult {
	// Query the entries in order
	q := LineageQuery{
		Since: since,
		Until: until,
	}
	entries, err := m.recorder.Query(ctx, q)
	if err != nil {
		return LineageIntegrityResult{
			Valid: false,
			Error: fmt.Errorf("lineage: verify query failed: %w", err),
		}
	}

	result := LineageIntegrityResult{
		Valid:        true,
		TotalEntries: len(entries),
	}

	if len(entries) == 0 {
		return result
	}

	// Verify hash chain
	prevHash := ""
	for _, entry := range entries {
		// Allow gap at the start of the range (we may not have the genesis entry)
		if entry.PrevHash != "" && prevHash == "" {
			prevHash = entry.PrevHash
		}

		expectedHash := entry.ComputeHash(prevHash)
		if entry.Hash != expectedHash {
			result.Valid = false
			result.BrokenAt = entry.ID
			result.Error = fmt.Errorf("lineage: hash mismatch at entry %q: expected %s, got %s",
				entry.ID, expectedHash, entry.Hash)
			return result
		}

		prevHash = entry.Hash
	}

	return result
}

// ─── Compliance reporting ─────────────────────────────────────────────────────

// ComplianceReport summarizes lineage data for regulatory submission.
type ComplianceReport struct {
	// ReportID is a unique identifier for this report
	ReportID string `json:"report_id"`
	// GeneratedAt is when the report was created
	GeneratedAt time.Time `json:"generated_at"`
	// Since is the start of the reporting period
	Since time.Time `json:"since"`
	// Until is the end of the reporting period
	Until time.Time `json:"until"`
	// TotalEntries is the number of lineage entries in the period
	TotalEntries int `json:"total_entries"`
	// AllowedDecisions is the count of approved actions
	AllowedDecisions int `json:"allowed_decisions"`
	// DeniedDecisions is the count of denied actions
	DeniedDecisions int `json:"denied_decisions"`
	// UniqueUsers is the count of distinct users
	UniqueUsers int `json:"unique_users"`
	// UniqueResources is the count of distinct resources
	UniqueResources int `json:"unique_resources"`
	// DeniedByReason breaks down denials by reason code
	DeniedByReason map[string]int `json:"denied_by_reason"`
	// IntegrityValid indicates whether the hash chain is intact
	IntegrityValid bool `json:"integrity_valid"`
	// HighestRiskEvents lists denied decisions (for escalation review)
	HighestRiskEvents []*LineageEntry `json:"highest_risk_events,omitempty"`
}

// GenerateComplianceReport creates a compliance-ready report.
//
// Useful for regulatory submissions (RBI, FATF, etc.).
// HighestRiskEvents includes the N most recent denied decisions.
func (m *Manager) GenerateComplianceReport(
	ctx context.Context,
	reportID string,
	since time.Time,
	until time.Time,
	maxHighRiskEvents int,
) (*ComplianceReport, error) {
	// Query all entries in the period
	q := LineageQuery{
		Since: since,
		Until: until,
		Limit: 100000,
	}
	entries, err := m.recorder.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("lineage: compliance report query failed: %w", err)
	}

	// Aggregate statistics
	uniqueUsers := make(map[string]bool)
	uniqueResources := make(map[string]bool)
	deniedByReason := make(map[string]int)
	allowedCount := 0
	deniedCount := 0
	var highRiskEvents []*LineageEntry

	for _, entry := range entries {
		uniqueUsers[entry.UserID] = true
		uniqueResources[entry.ResourceID] = true

		if entry.Decision == DecisionAllowed {
			allowedCount++
		} else {
			deniedCount++
			deniedByReason[entry.ReasonCode]++
			if len(highRiskEvents) < maxHighRiskEvents {
				highRiskEvents = append(highRiskEvents, entry)
			}
		}
	}

	// Check hash chain integrity
	integrity := m.recorder.Verify(ctx)

	report := &ComplianceReport{
		ReportID:         reportID,
		GeneratedAt:      time.Now().UTC(),
		Since:            since,
		Until:            until,
		TotalEntries:     len(entries),
		AllowedDecisions: allowedCount,
		DeniedDecisions:  deniedCount,
		UniqueUsers:      len(uniqueUsers),
		UniqueResources:  len(uniqueResources),
		DeniedByReason:   deniedByReason,
		IntegrityValid:   integrity.Valid,
		HighestRiskEvents: highRiskEvents,
	}

	return report, nil
}

// ComplianceReportJSON exports the compliance report as JSON.
func (m *Manager) ComplianceReportJSON(report *ComplianceReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("lineage: compliance report json marshal failed: %w", err)
	}
	return string(data), nil
}

// ComplianceReportCSV exports the compliance report's high-risk events as CSV.
func (m *Manager) ComplianceReportCSV(report *ComplianceReport) (string, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	// Write header
	header := []string{
		"ID",
		"Timestamp",
		"UserID",
		"ResourceID",
		"ResourceType",
		"Action",
		"Decision",
		"ReasonCode",
		"PolicyRule",
		"AgentID",
		"SessionID",
		"TraceID",
	}
	if err := w.Write(header); err != nil {
		return "", fmt.Errorf("lineage: compliance report csv write header failed: %w", err)
	}

	// Write high-risk events
	for _, entry := range report.HighestRiskEvents {
		row := []string{
			entry.ID,
			entry.Timestamp.Format(time.RFC3339),
			entry.UserID,
			entry.ResourceID,
			entry.ResourceType,
			string(entry.Action),
			string(entry.Decision),
			entry.ReasonCode,
			entry.PolicyRule,
			entry.AgentID,
			entry.SessionID,
			entry.TraceID,
		}
		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("lineage: compliance report csv write row failed: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("lineage: compliance report csv flush failed: %w", err)
	}

	return buf.String(), nil
}
