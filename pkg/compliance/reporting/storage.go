package reporting

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AuditEntry is a minimal definition matching the parent compliance package.
// It represents one row in the audit log with hash chaining for integrity.
type AuditEntry struct {
	Seq        int64          `json:"seq"`
	OccurredAt time.Time      `json:"occurred_at"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Target     string         `json:"target"`
	Details    map[string]any `json:"details,omitempty"`
	PrevHash   string         `json:"prev_hash"`
	RowHash    string         `json:"row_hash"`
}

// AuditLog is an interface for audit log access.
type AuditLog interface {
	List(ctx context.Context) ([]AuditEntry, error)
}

// ReportRepository provides immutable storage for compliance reports.
//
// Design principle: reports are append-only. Once filed, a report is never modified.
// This ensures audit integrity: changes are always new reports, never edits to old ones.
// All implementations must be thread-safe.
type ReportRepository interface {
	// SaveRBIReport appends a new RBI report (immutable). Returns the report ID.
	SaveRBIReport(ctx context.Context, report *RBIQuarterlyReport) (string, error)

	// SaveSTRReport appends a new STR (immutable). Returns the report ID.
	SaveSTRReport(ctx context.Context, report *FATPSTRReport) (string, error)

	// GetRBIReport retrieves a single RBI report by ID.
	GetRBIReport(ctx context.Context, reportID string) (*RBIQuarterlyReport, error)

	// GetSTRReport retrieves a single STR by ID.
	GetSTRReport(ctx context.Context, strID string) (*FATPSTRReport, error)

	// QueryRBIReports returns all RBI reports in a date range [since, until].
	QueryRBIReports(ctx context.Context, since, until time.Time) ([]*RBIQuarterlyReport, error)

	// QuerySTRReports returns all STRs in a date range [since, until].
	QuerySTRReports(ctx context.Context, since, until time.Time) ([]*FATPSTRReport, error)

	// QueryByTransactionID finds all reports (RBI and STR) related to a transaction.
	// Used for audit trails and investigating patterns.
	QueryByTransactionID(ctx context.Context, txnID string) ([]interface{}, error)
}

// InMemoryReportRepository is a thread-safe in-memory implementation for testing.
// All operations are O(n) since there's no indexing. For production, use PostgreSQL.
type InMemoryReportRepository struct {
	mu          sync.RWMutex
	rbiReports  map[string]*RBIQuarterlyReport
	strReports  map[string]*FATPSTRReport
	rbiByTime   []*RBIQuarterlyReport // unsorted; used for range queries
	strByTime   []*FATPSTRReport      // unsorted; used for range queries
}

// NewInMemoryReportRepository returns an empty in-memory repository.
func NewInMemoryReportRepository() *InMemoryReportRepository {
	return &InMemoryReportRepository{
		rbiReports: make(map[string]*RBIQuarterlyReport),
		strReports: make(map[string]*FATPSTRReport),
	}
}

// SaveRBIReport appends the report and returns its ID.
func (r *InMemoryReportRepository) SaveRBIReport(ctx context.Context, report *RBIQuarterlyReport) (string, error) {
	if report == nil {
		return "", fmt.Errorf("storage: cannot save nil RBI report")
	}
	if report.ReportID == "" {
		return "", fmt.Errorf("storage: RBI report must have ReportID set")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure immutability: reject duplicate IDs.
	if _, exists := r.rbiReports[report.ReportID]; exists {
		return "", fmt.Errorf("storage: RBI report ID %q already exists", report.ReportID)
	}

	// Deep copy to prevent mutations through retained references.
	copied := *report
	r.rbiReports[report.ReportID] = &copied
	r.rbiByTime = append(r.rbiByTime, &copied)

	return report.ReportID, nil
}

// SaveSTRReport appends the STR and returns its ID.
func (r *InMemoryReportRepository) SaveSTRReport(ctx context.Context, report *FATPSTRReport) (string, error) {
	if report == nil {
		return "", fmt.Errorf("storage: cannot save nil STR report")
	}
	if report.STRID == "" {
		return "", fmt.Errorf("storage: STR report must have STRID set")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure immutability: reject duplicate IDs.
	if _, exists := r.strReports[report.STRID]; exists {
		return "", fmt.Errorf("storage: STR report ID %q already exists", report.STRID)
	}

	// Deep copy to prevent mutations through retained references.
	copied := *report
	r.strReports[report.STRID] = &copied
	r.strByTime = append(r.strByTime, &copied)

	return report.STRID, nil
}

// GetRBIReport retrieves a report by ID.
func (r *InMemoryReportRepository) GetRBIReport(ctx context.Context, reportID string) (*RBIQuarterlyReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report, ok := r.rbiReports[reportID]
	if !ok {
		return nil, fmt.Errorf("storage: RBI report %q not found", reportID)
	}

	// Return a copy to prevent mutation.
	copied := *report
	return &copied, nil
}

// GetSTRReport retrieves an STR by ID.
func (r *InMemoryReportRepository) GetSTRReport(ctx context.Context, strID string) (*FATPSTRReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report, ok := r.strReports[strID]
	if !ok {
		return nil, fmt.Errorf("storage: STR report %q not found", strID)
	}

	// Return a copy to prevent mutation.
	copied := *report
	return &copied, nil
}

// QueryRBIReports returns all RBI reports in [since, until].
func (r *InMemoryReportRepository) QueryRBIReports(ctx context.Context, since, until time.Time) ([]*RBIQuarterlyReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*RBIQuarterlyReport
	for _, report := range r.rbiByTime {
		if report.GeneratedAt.After(since) && report.GeneratedAt.Before(until) {
			copied := *report
			result = append(result, &copied)
		}
	}
	return result, nil
}

// QuerySTRReports returns all STRs in [since, until].
func (r *InMemoryReportRepository) QuerySTRReports(ctx context.Context, since, until time.Time) ([]*FATPSTRReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*FATPSTRReport
	for _, report := range r.strByTime {
		if report.FilingDate.After(since) && report.FilingDate.Before(until) {
			copied := *report
			result = append(result, &copied)
		}
	}
	return result, nil
}

// QueryByTransactionID finds all reports mentioning a transaction ID.
func (r *InMemoryReportRepository) QueryByTransactionID(ctx context.Context, txnID string) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []interface{}

	// Check STRs (they have explicit TransactionID).
	for _, str := range r.strByTime {
		if str.TransactionID == txnID {
			copied := *str
			result = append(result, &copied)
		}
	}

	// RBI reports don't have a direct txn ID, but could cross-reference via merchant.
	// For now, STRs are the primary transaction-level reports.

	return result, nil
}

// All returns a copy of all RBI reports.
func (r *InMemoryReportRepository) All() []*RBIQuarterlyReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*RBIQuarterlyReport, len(r.rbiReports))
	i := 0
	for _, report := range r.rbiReports {
		copied := *report
		out[i] = &copied
		i++
	}
	return out
}

// Stats returns basic statistics about the repository (for monitoring).
func (r *InMemoryReportRepository) Stats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]int{
		"rbi_reports": len(r.rbiReports),
		"str_reports": len(r.strReports),
	}
}
