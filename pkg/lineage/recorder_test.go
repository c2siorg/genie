package lineage

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInMemoryRecorder_Record_Single(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	entry := &LineageEntry{
		ID:           "entry:1",
		UserID:       "user:alice",
		ResourceID:   "msg:123",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
	}

	err := rec.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Verify entry was stored with hash computed
	if entry.Hash == "" {
		t.Error("Hash not computed")
	}
	if entry.Hash == entry.PrevHash {
		t.Error("Hash same as PrevHash for first entry")
	}
}

func TestInMemoryRecorder_Record_Duplicate(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	entry := &LineageEntry{
		ID:           "entry:1",
		UserID:       "user:alice",
		ResourceID:   "msg:123",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
	}

	// Record once
	err := rec.Record(ctx, entry)
	if err != nil {
		t.Fatalf("First record failed: %v", err)
	}

	// Try to record again with same ID
	entry2 := &LineageEntry{
		ID:           "entry:1",
		UserID:       "user:bob",
		ResourceID:   "msg:456",
		ResourceType: "message",
		Action:       ActionWrite,
		Decision:     DecisionDenied,
		ReasonCode:   "denied",
	}

	err = rec.Record(ctx, entry2)
	if err == nil {
		t.Error("Expected duplicate error, got nil")
	}
}

func TestInMemoryRecorder_Record_Concurrent(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()
	numGoroutines := 100

	var wg sync.WaitGroup
	var errors atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			entry := &LineageEntry{
				ID:           "entry:" + string(rune(idx)),
				UserID:       "user:alice",
				ResourceID:   "msg:123",
				ResourceType: "message",
				Action:       ActionRead,
				Decision:     DecisionAllowed,
				ReasonCode:   "ok",
			}
			if err := rec.Record(ctx, entry); err != nil {
				errors.Add(1)
				t.Logf("Record failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	if errors.Load() > 0 {
		t.Errorf("Got %d errors during concurrent recording", errors.Load())
	}

	// Verify all entries were recorded
	query := LineageQuery{}
	entries, err := rec.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(entries) != numGoroutines {
		t.Errorf("Expected %d entries, got %d", numGoroutines, len(entries))
	}
}

func TestInMemoryRecorder_Record_HashChain(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	// Record 3 entries
	for i := 1; i <= 3; i++ {
		entry := &LineageEntry{
			ID:           "entry:" + string(rune(48+i)), // entry:1, entry:2, entry:3
			UserID:       "user:alice",
			ResourceID:   "msg:123",
			ResourceType: "message",
			Action:       ActionRead,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		}
		err := rec.Record(ctx, entry)
		if err != nil {
			t.Fatalf("Record %d failed: %v", i, err)
		}
	}

	// Query and verify hash chain
	query := LineageQuery{}
	entries, err := rec.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Verify first entry has empty prevHash
	if entries[0].PrevHash != "" {
		t.Error("First entry should have empty PrevHash")
	}

	// Verify chain: entry[i].PrevHash == entry[i-1].Hash
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].Hash {
			t.Errorf("Hash chain broken at entry %d: PrevHash=%s, expected=%s",
				i, entries[i].PrevHash, entries[i-1].Hash)
		}
	}

	// Verify hashes are unique
	hashes := make(map[string]bool)
	for _, entry := range entries {
		if hashes[entry.Hash] {
			t.Error("Duplicate hash found in chain")
		}
		hashes[entry.Hash] = true
	}
}

func TestInMemoryRecorder_Query_Filters(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	// Record multiple entries
	entries := []*LineageEntry{
		{
			ID:           "entry:1",
			UserID:       "user:alice",
			ResourceID:   "msg:1",
			ResourceType: "message",
			Action:       ActionRead,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		},
		{
			ID:           "entry:2",
			UserID:       "user:bob",
			ResourceID:   "msg:2",
			ResourceType: "message",
			Action:       ActionWrite,
			Decision:     DecisionDenied,
			ReasonCode:   "rbac",
		},
		{
			ID:           "entry:3",
			UserID:       "user:alice",
			ResourceID:   "msg:3",
			ResourceType: "tool",
			Action:       ActionWrite,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		},
	}

	for _, entry := range entries {
		if err := rec.Record(ctx, entry); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// Test user filter
	q := LineageQuery{UserID: "user:alice"}
	results, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("UserID filter: expected 2 results, got %d", len(results))
	}

	// Test decision filter
	q = LineageQuery{Decision: DecisionDenied}
	results, err = rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Decision filter: expected 1 result, got %d", len(results))
	}

	// Test resource type filter
	q = LineageQuery{ResourceType: "tool"}
	results, err = rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("ResourceType filter: expected 1 result, got %d", len(results))
	}

	// Test combined filters
	q = LineageQuery{UserID: "user:alice", ResourceType: "message"}
	results, err = rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Combined filters: expected 1 result, got %d", len(results))
	}
}

func TestInMemoryRecorder_Query_TimeRange(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	now := time.Now().UTC()

	// Record entries with specific timestamps
	entries := []*LineageEntry{
		{
			ID:           "entry:1",
			Timestamp:    now.Add(-2 * time.Hour),
			UserID:       "user:alice",
			ResourceID:   "msg:1",
			ResourceType: "message",
			Action:       ActionRead,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		},
		{
			ID:           "entry:2",
			Timestamp:    now,
			UserID:       "user:bob",
			ResourceID:   "msg:2",
			ResourceType: "message",
			Action:       ActionWrite,
			Decision:     DecisionDenied,
			ReasonCode:   "rbac",
		},
		{
			ID:           "entry:3",
			Timestamp:    now.Add(2 * time.Hour),
			UserID:       "user:alice",
			ResourceID:   "msg:3",
			ResourceType: "tool",
			Action:       ActionWrite,
			Decision:     DecisionAllowed,
			ReasonCode:   "ok",
		},
	}

	for _, entry := range entries {
		if err := rec.Record(ctx, entry); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// Test Since filter
	q := LineageQuery{Since: now.Add(-1 * time.Hour)}
	results, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Since filter: expected 2 results, got %d", len(results))
	}

	// Test Until filter
	q = LineageQuery{Until: now.Add(1 * time.Hour)}
	results, err = rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Until filter: expected 2 results, got %d", len(results))
	}

	// Test Since and Until together
	q = LineageQuery{
		Since: now.Add(-1 * time.Hour),
		Until: now.Add(1 * time.Hour),
	}
	results, err = rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Since+Until filter: expected 1 result, got %d", len(results))
	}
}

func TestInMemoryRecorder_Query_Pagination(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	// Record 10 entries
	for i := 1; i <= 10; i++ {
		entry := &LineageEntry{
			ID:           "entry:" + string(rune(48+i%10)),
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

	// Test Limit
	q := LineageQuery{Limit: 3}
	results, err := rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("Limit: expected 3 results, got %d", len(results))
	}

	// Test Offset
	q = LineageQuery{Offset: 5, Limit: 10}
	results, err = rec.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Offset: expected 5 results, got %d", len(results))
	}
}

func TestInMemoryRecorder_Verify_Valid(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	// Record 3 entries
	for i := 1; i <= 3; i++ {
		entry := &LineageEntry{
			ID:           "entry:" + string(rune(48+i)),
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

	result := rec.Verify(ctx)
	if !result.Valid {
		t.Errorf("Verify failed: %v", result.Error)
	}
	if result.TotalEntries != 3 {
		t.Errorf("TotalEntries: expected 3, got %d", result.TotalEntries)
	}
}

func TestInMemoryRecorder_Verify_EmptyRecorder(t *testing.T) {
	ctx := context.Background()
	rec := NewInMemoryRecorder()

	result := rec.Verify(ctx)
	if !result.Valid {
		t.Error("Empty recorder should be valid")
	}
	if result.TotalEntries != 0 {
		t.Errorf("TotalEntries: expected 0, got %d", result.TotalEntries)
	}
}
