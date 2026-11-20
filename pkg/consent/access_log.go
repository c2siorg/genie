package consent

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// InMemoryAccessLog is a thread-safe, in-memory implementation of AccessLog.
//
// It maintains a log of all authorization decisions in chronological order.
// Logs are queryable by userID and time range. All reads and writes are
// protected by a single RWMutex. For production deployments, consider
// replacing this with a persistent backend (e.g. TimescaleDB, Cassandra).
type InMemoryAccessLog struct {
	mu      sync.RWMutex
	entries []AccessDecision
}

// NewInMemoryAccessLog creates a new, empty access log.
func NewInMemoryAccessLog() *InMemoryAccessLog {
	return &InMemoryAccessLog{
		entries: []AccessDecision{},
	}
}

// Log records a single authorization decision.
// Always succeeds; failures are not propagated to avoid breaking the
// authorization flow.
func (al *InMemoryAccessLog) Log(decision AccessDecision) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.entries = append(al.entries, decision)
	return nil
}

// Query returns decisions matching the given filters.
// If userID is empty, all users are included.
// If since/until are zero, no time filtering is applied.
// Results are returned in chronological order (earliest first).
func (al *InMemoryAccessLog) Query(userID string, since, until time.Time) ([]AccessDecision, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var results []AccessDecision

	for _, entry := range al.entries {
		// Filter by userID
		if userID != "" && entry.UserID != userID {
			continue
		}

		// Filter by time range
		if !since.IsZero() && entry.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && entry.Timestamp.After(until) {
			continue
		}

		results = append(results, entry)
	}

	return results, nil
}

// Export returns all logged decisions in the specified format.
// Supported formats: "json", "csv".
// Returns an error if the format is unrecognized.
func (al *InMemoryAccessLog) Export(format string) ([]byte, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	switch format {
	case "json":
		return al.exportJSON()
	case "csv":
		return al.exportCSV()
	default:
		return nil, fmt.Errorf("%w: unsupported export format %q", ErrInvalidInput, format)
	}
}

// exportJSON marshals all entries as a JSON array.
func (al *InMemoryAccessLog) exportJSON() ([]byte, error) {
	// Sort by timestamp for consistency
	entries := make([]AccessDecision, len(al.entries))
	copy(entries, al.entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: json marshal: %v", ErrInternalError, err)
	}
	return data, nil
}

// exportCSV writes all entries in CSV format with headers.
func (al *InMemoryAccessLog) exportCSV() ([]byte, error) {
	// Sort by timestamp for consistency
	entries := make([]AccessDecision, len(al.entries))
	copy(entries, al.entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header
	header := []string{"timestamp", "user_id", "resource_type", "action", "result", "reason_code", "trace_id"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("%w: csv write header: %v", ErrInternalError, err)
	}

	// Write records
	for _, entry := range entries {
		record := []string{
			entry.Timestamp.Format(time.RFC3339),
			entry.UserID,
			entry.ResourceType,
			string(entry.Action),
			entry.Result,
			entry.ReasonCode,
			entry.TraceID,
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("%w: csv write record: %v", ErrInternalError, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("%w: csv flush: %v", ErrInternalError, err)
	}

	return buf.Bytes(), nil
}

// Stats returns summary statistics about the access log.
// Useful for monitoring and debugging.
type LogStats struct {
	TotalEntries      int
	AllowedCount      int
	DeniedCount       int
	UniqueUsers       int
	UniqueResources   int
	EarliestTimestamp time.Time
	LatestTimestamp   time.Time
}

// GetStats returns summary statistics about the log.
func (al *InMemoryAccessLog) GetStats() LogStats {
	al.mu.RLock()
	defer al.mu.RUnlock()

	stats := LogStats{
		TotalEntries: len(al.entries),
	}

	if len(al.entries) == 0 {
		return stats
	}

	userSet := make(map[string]bool)
	resourceSet := make(map[string]bool)

	for _, entry := range al.entries {
		if entry.Result == "allowed" {
			stats.AllowedCount++
		} else {
			stats.DeniedCount++
		}
		userSet[entry.UserID] = true
		resourceSet[entry.ResourceType] = true

		if stats.EarliestTimestamp.IsZero() || entry.Timestamp.Before(stats.EarliestTimestamp) {
			stats.EarliestTimestamp = entry.Timestamp
		}
		if entry.Timestamp.After(stats.LatestTimestamp) {
			stats.LatestTimestamp = entry.Timestamp
		}
	}

	stats.UniqueUsers = len(userSet)
	stats.UniqueResources = len(resourceSet)

	return stats
}

// Clear removes all entries from the log.
// Useful for testing; not recommended for production.
func (al *InMemoryAccessLog) Clear() {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.entries = []AccessDecision{}
}

// Len returns the number of entries in the log.
func (al *InMemoryAccessLog) Len() int {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return len(al.entries)
}
