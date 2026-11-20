package consent

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInMemoryAccessLogLog(t *testing.T) {
	log := NewInMemoryAccessLog()

	decision := AccessDecision{
		Timestamp:    time.Now().UTC(),
		UserID:       "user1",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
		ReasonCode:   "",
		TraceID:      "trace-123",
	}

	err := log.Log(decision)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	if log.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", log.Len())
	}
}

func TestInMemoryAccessLogQuery(t *testing.T) {
	log := NewInMemoryAccessLog()

	now := time.Now().UTC()

	// Log some decisions
	log.Log(AccessDecision{
		Timestamp:    now.Add(-2 * time.Hour),
		UserID:       "user1",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
	})
	log.Log(AccessDecision{
		Timestamp:    now.Add(-1 * time.Hour),
		UserID:       "user2",
		ResourceType: "api",
		Action:       ActionWrite,
		Result:       "denied",
	})
	log.Log(AccessDecision{
		Timestamp:    now,
		UserID:       "user1",
		ResourceType: "cache",
		Action:       ActionRead,
		Result:       "allowed",
	})

	// Query all
	results, err := log.Query("", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("query all: got %d results, want 3", len(results))
	}

	// Query by userID
	results, _ = log.Query("user1", time.Time{}, time.Time{})
	if len(results) != 2 {
		t.Errorf("query user1: got %d results, want 2", len(results))
	}

	// Query by time range
	since := now.Add(-90 * time.Minute)
	until := now.Add(-30 * time.Minute)
	results, _ = log.Query("", since, until)
	if len(results) != 1 {
		t.Errorf("query by time range: got %d results, want 1", len(results))
	}
	if results[0].UserID != "user2" {
		t.Errorf("query by time range: got userID=%q, want user2", results[0].UserID)
	}

	// Query specific user in time range
	results, _ = log.Query("user1", since, until)
	if len(results) != 0 {
		t.Errorf("query user1 in time range: got %d results, want 0", len(results))
	}
}

func TestInMemoryAccessLogExportJSON(t *testing.T) {
	log := NewInMemoryAccessLog()

	now := time.Now().UTC()
	log.Log(AccessDecision{
		Timestamp:    now,
		UserID:       "user1",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
		ReasonCode:   "",
		TraceID:      "trace-1",
	})

	data, err := log.Export("json")
	if err != nil {
		t.Fatalf("Export JSON failed: %v", err)
	}

	var results []AccessDecision
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("exported: got %d entries, want 1", len(results))
	}
	if results[0].UserID != "user1" {
		t.Errorf("exported: got userID=%q, want user1", results[0].UserID)
	}
}

func TestInMemoryAccessLogExportCSV(t *testing.T) {
	log := NewInMemoryAccessLog()

	now := time.Now().UTC()
	log.Log(AccessDecision{
		Timestamp:    now,
		UserID:       "user1",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
		ReasonCode:   "",
		TraceID:      "trace-1",
	})
	log.Log(AccessDecision{
		Timestamp:    now.Add(1 * time.Second),
		UserID:       "user2",
		ResourceType: "api",
		Action:       ActionWrite,
		Result:       "denied",
		ReasonCode:   "no_grant",
		TraceID:      "trace-2",
	})

	data, err := log.Export("csv")
	if err != nil {
		t.Fatalf("Export CSV failed: %v", err)
	}

	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse failed: %v", err)
	}

	// Should have header + 2 data rows
	if len(records) != 3 {
		t.Errorf("exported: got %d rows, want 3 (header + 2 data)", len(records))
	}

	// Check header
	expectedHeader := []string{"timestamp", "user_id", "resource_type", "action", "result", "reason_code", "trace_id"}
	for i, h := range expectedHeader {
		if records[0][i] != h {
			t.Errorf("header[%d]: got %q, want %q", i, records[0][i], h)
		}
	}

	// Check first data row
	if records[1][1] != "user1" {
		t.Errorf("row 1 user_id: got %q, want user1", records[1][1])
	}
	if records[1][4] != "allowed" {
		t.Errorf("row 1 result: got %q, want allowed", records[1][4])
	}

	// Check second data row
	if records[2][1] != "user2" {
		t.Errorf("row 2 user_id: got %q, want user2", records[2][1])
	}
	if records[2][4] != "denied" {
		t.Errorf("row 2 result: got %q, want denied", records[2][4])
	}
	if records[2][5] != "no_grant" {
		t.Errorf("row 2 reason_code: got %q, want no_grant", records[2][5])
	}
}

func TestInMemoryAccessLogExportInvalidFormat(t *testing.T) {
	log := NewInMemoryAccessLog()

	_, err := log.Export("xml")
	if err == nil {
		t.Errorf("export invalid format: expected error, got nil")
	}
}

func TestInMemoryAccessLogGetStats(t *testing.T) {
	log := NewInMemoryAccessLog()

	now := time.Now().UTC()

	// Log various decisions
	log.Log(AccessDecision{
		Timestamp:    now,
		UserID:       "user1",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
	})
	log.Log(AccessDecision{
		Timestamp:    now.Add(1 * time.Second),
		UserID:       "user1",
		ResourceType: "api",
		Action:       ActionWrite,
		Result:       "denied",
	})
	log.Log(AccessDecision{
		Timestamp:    now.Add(2 * time.Second),
		UserID:       "user2",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
	})

	stats := log.GetStats()

	if stats.TotalEntries != 3 {
		t.Errorf("stats.TotalEntries: got %d, want 3", stats.TotalEntries)
	}
	if stats.AllowedCount != 2 {
		t.Errorf("stats.AllowedCount: got %d, want 2", stats.AllowedCount)
	}
	if stats.DeniedCount != 1 {
		t.Errorf("stats.DeniedCount: got %d, want 1", stats.DeniedCount)
	}
	if stats.UniqueUsers != 2 {
		t.Errorf("stats.UniqueUsers: got %d, want 2", stats.UniqueUsers)
	}
	if stats.UniqueResources != 2 {
		t.Errorf("stats.UniqueResources: got %d, want 2", stats.UniqueResources)
	}
	if !stats.EarliestTimestamp.Equal(now) {
		t.Errorf("stats.EarliestTimestamp mismatch")
	}
	if !stats.LatestTimestamp.Equal(now.Add(2 * time.Second)) {
		t.Errorf("stats.LatestTimestamp mismatch")
	}
}

func TestInMemoryAccessLogGetStatsEmpty(t *testing.T) {
	log := NewInMemoryAccessLog()

	stats := log.GetStats()

	if stats.TotalEntries != 0 {
		t.Errorf("empty log stats: TotalEntries=%d, want 0", stats.TotalEntries)
	}
	if stats.AllowedCount != 0 {
		t.Errorf("empty log stats: AllowedCount=%d, want 0", stats.AllowedCount)
	}
	if !stats.EarliestTimestamp.IsZero() {
		t.Errorf("empty log stats: EarliestTimestamp should be zero")
	}
}

func TestInMemoryAccessLogClear(t *testing.T) {
	log := NewInMemoryAccessLog()

	log.Log(AccessDecision{
		Timestamp:    time.Now().UTC(),
		UserID:       "user1",
		ResourceType: "database",
		Action:       ActionRead,
		Result:       "allowed",
	})

	if log.Len() != 1 {
		t.Errorf("before clear: Len()=%d, want 1", log.Len())
	}

	log.Clear()

	if log.Len() != 0 {
		t.Errorf("after clear: Len()=%d, want 0", log.Len())
	}
}

func TestInMemoryAccessLogConcurrentLog(t *testing.T) {
	log := NewInMemoryAccessLog()
	var wg sync.WaitGroup
	var successCount int32

	numGoroutines := 100
	entriesPerGoroutine := 50

	now := time.Now().UTC()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < entriesPerGoroutine; j++ {
				decision := AccessDecision{
					Timestamp:    now.Add(time.Duration(j) * time.Millisecond),
					UserID:       "user" + string(rune('0'+id%10)),
					ResourceType: "resource" + string(rune('0'+j%5)),
					Action:       ActionRead,
					Result:       "allowed",
				}
				if err := log.Log(decision); err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	expected := int32(numGoroutines * entriesPerGoroutine)
	if successCount != expected {
		t.Errorf("concurrent log: got %d successes, want %d", successCount, expected)
	}
	if log.Len() != int(expected) {
		t.Errorf("concurrent log: final Len()=%d, want %d", log.Len(), expected)
	}
}

func TestInMemoryAccessLogConcurrentQueryAndLog(t *testing.T) {
	log := NewInMemoryAccessLog()
	var wg sync.WaitGroup
	var logCount, queryCount int32

	now := time.Now().UTC()

	// Goroutines that log decisions
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				log.Log(AccessDecision{
					Timestamp:    now.Add(time.Duration(j) * time.Millisecond),
					UserID:       "user" + string(rune('0'+id%5)),
					ResourceType: "resource",
					Action:       ActionRead,
					Result:       "allowed",
				})
				atomic.AddInt32(&logCount, 1)
			}
		}(i)
	}

	// Goroutines that query
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = log.Query("user0", time.Time{}, time.Time{})
				atomic.AddInt32(&queryCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if logCount != 20*50 {
		t.Errorf("concurrent log/query: logCount=%d, want 1000", logCount)
	}
	if queryCount != 20*50 {
		t.Errorf("concurrent log/query: queryCount=%d, want 1000", queryCount)
	}
}
