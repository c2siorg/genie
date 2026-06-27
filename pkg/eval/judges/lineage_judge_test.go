package judges

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

// computeHash is a helper to compute entry hash for testing.
func computeHash(orderID, step, timestamp, data string) string {
	h := sha256.New()
	h.Write([]byte(orderID + "|" + step + "|" + timestamp + "|" + data))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestLineageJudge_EvaluateValidChain tests evaluation of a valid hash chain.
func TestLineageJudge_EvaluateValidChain(t *testing.T) {
	judge := NewMockLineageJudge()

	// Build a valid hash chain
	now := time.Now()
	entry1 := LineageEntry{
		EntryID:      "entry-001",
		OrderID:      "order-001",
		Step:         "order_created",
		Timestamp:    now,
		PreviousHash: "", // First entry has no previous
		Data:         `{"amount": 100000}`,
	}
	entry1.Hash = computeHash(entry1.OrderID, entry1.Step, entry1.Timestamp.String(), entry1.Data)

	entry2 := LineageEntry{
		EntryID:      "entry-002",
		OrderID:      "order-001",
		Step:         "payment_initiated",
		Timestamp:    now.Add(1 * time.Second),
		PreviousHash: entry1.Hash,
		Data:         `{"transaction_id": "tx-001"}`,
	}
	entry2.Hash = computeHash(entry2.OrderID, entry2.Step, entry2.Timestamp.String(), entry2.Data)

	entry3 := LineageEntry{
		EntryID:      "entry-003",
		OrderID:      "order-001",
		Step:         "payment_confirmed",
		Timestamp:    now.Add(2 * time.Second),
		PreviousHash: entry2.Hash,
		Data:         `{"confirmed": true}`,
	}
	entry3.Hash = computeHash(entry3.OrderID, entry3.Step, entry3.Timestamp.String(), entry3.Data)

	input := LineageJudgeInput{
		OrderID:                  "order-001",
		LineageEntries:           []LineageEntry{entry1, entry2, entry3},
		HashChainValid:           true,
		TimestampsMonotonic:      true,
		AllRequiredFieldsPresent: true,
		DecisionTraceability:     `{"decision": "payment approved", "basis": "kyc verified"}`,
		Metadata:                 map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Valid hash chain should pass
	if verdict.Score < 0.6 {
		t.Logf("Warning: valid hash chain received low score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f (order %s with %d entries)", verdict.Pass, verdict.Score, input.OrderID, len(input.LineageEntries))
}

// TestLineageJudge_EvaluateBrokenChain tests evaluation of a broken hash chain.
func TestLineageJudge_EvaluateBrokenChain(t *testing.T) {
	judge := NewMockLineageJudge()

	// Build a broken hash chain
	now := time.Now()
	entry1 := LineageEntry{
		EntryID:      "entry-001",
		OrderID:      "order-002",
		Step:         "order_created",
		Timestamp:    now,
		PreviousHash: "",
		Data:         `{"amount": 100000}`,
	}
	entry1.Hash = computeHash(entry1.OrderID, entry1.Step, entry1.Timestamp.String(), entry1.Data)

	entry2 := LineageEntry{
		EntryID:      "entry-002",
		OrderID:      "order-002",
		Step:         "payment_initiated",
		Timestamp:    now.Add(1 * time.Second),
		PreviousHash: "wrong_hash_here", // Wrong previous hash!
		Data:         `{"transaction_id": "tx-002"}`,
	}
	entry2.Hash = computeHash(entry2.OrderID, entry2.Step, entry2.Timestamp.String(), entry2.Data)

	input := LineageJudgeInput{
		OrderID:                  "order-002",
		LineageEntries:           []LineageEntry{entry1, entry2},
		HashChainValid:           false, // Broken chain
		TimestampsMonotonic:      true,
		AllRequiredFieldsPresent: true,
		DecisionTraceability:     `{"decision": "payment failed"}`,
		Metadata:                 map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Broken hash chain should result in Pass=false or low score
	if verdict.Score > 0.6 {
		t.Logf("Warning: broken hash chain received high score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f (broken hash chain detected for order %s)", verdict.Pass, verdict.Score, input.OrderID)
}

// TestLineageJudge_EvaluateNonMonotonicTimestamps tests non-monotonic timestamps.
func TestLineageJudge_EvaluateNonMonotonicTimestamps(t *testing.T) {
	judge := NewMockLineageJudge()

	// Build entries with non-monotonic timestamps
	now := time.Now()
	entry1 := LineageEntry{
		EntryID:      "entry-001",
		OrderID:      "order-003",
		Step:         "order_created",
		Timestamp:    now.Add(2 * time.Second), // Later timestamp
		PreviousHash: "",
		Data:         `{"amount": 100000}`,
	}
	entry1.Hash = computeHash(entry1.OrderID, entry1.Step, entry1.Timestamp.String(), entry1.Data)

	entry2 := LineageEntry{
		EntryID:      "entry-002",
		OrderID:      "order-003",
		Step:         "payment_initiated",
		Timestamp:    now, // Earlier timestamp! (violates monotonicity)
		PreviousHash: entry1.Hash,
		Data:         `{"transaction_id": "tx-003"}`,
	}
	entry2.Hash = computeHash(entry2.OrderID, entry2.Step, entry2.Timestamp.String(), entry2.Data)

	input := LineageJudgeInput{
		OrderID:                  "order-003",
		LineageEntries:           []LineageEntry{entry1, entry2},
		HashChainValid:           true,
		TimestampsMonotonic:      false, // Not monotonic
		AllRequiredFieldsPresent: true,
		DecisionTraceability:     `{}`,
		Metadata:                 map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Non-monotonic timestamps should result in lower score
	if verdict.Score > 0.6 {
		t.Logf("Warning: non-monotonic timestamps received high score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f (non-monotonic timestamps detected for order %s)", verdict.Pass, verdict.Score, input.OrderID)
}

// TestLineageJudge_EvaluateMissingFields tests missing required fields.
func TestLineageJudge_EvaluateMissingFields(t *testing.T) {
	judge := NewMockLineageJudge()

	// Build entries with missing fields
	entry1 := LineageEntry{
		EntryID:      "entry-001",
		OrderID:      "order-004",
		Step:         "order_created",
		Timestamp:    time.Now(),
		PreviousHash: "",
		Data:         ``, // Empty data!
		Hash:         "abcd1234",
	}

	input := LineageJudgeInput{
		OrderID:                  "order-004",
		LineageEntries:           []LineageEntry{entry1},
		HashChainValid:           true,
		TimestampsMonotonic:      true,
		AllRequiredFieldsPresent: false, // Missing required field
		DecisionTraceability:     `{}`,
		Metadata:                 map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Missing fields should result in lower score
	if verdict.Score > 0.6 {
		t.Logf("Warning: missing fields received high score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f (missing fields detected for order %s)", verdict.Pass, verdict.Score, input.OrderID)
}

// TestVerifyHashChain tests hash chain verification function.
func TestVerifyHashChain(t *testing.T) {
	now := time.Now()

	// Build valid chain
	entry1 := LineageEntry{
		EntryID:      "entry-001",
		OrderID:      "order-test",
		Step:         "step1",
		Timestamp:    now,
		PreviousHash: "",
		Data:         `{"test": 1}`,
	}
	entry1.Hash = computeHash(entry1.OrderID, entry1.Step, entry1.Timestamp.String(), entry1.Data)

	entry2 := LineageEntry{
		EntryID:      "entry-002",
		OrderID:      "order-test",
		Step:         "step2",
		Timestamp:    now.Add(1 * time.Second),
		PreviousHash: entry1.Hash,
		Data:         `{"test": 2}`,
	}
	entry2.Hash = computeHash(entry2.OrderID, entry2.Step, entry2.Timestamp.String(), entry2.Data)

	// Test valid chain
	validChain := []LineageEntry{entry1, entry2}
	if !verifyHashChain(validChain) {
		t.Error("valid hash chain should pass verification")
	}

	// Test broken chain
	entry2.PreviousHash = "wrong_hash"
	brokenChain := []LineageEntry{entry1, entry2}
	if verifyHashChain(brokenChain) {
		t.Error("broken hash chain should fail verification")
	}
}

// TestVerifyTimestampMonotonicity tests timestamp monotonicity verification.
func TestVerifyTimestampMonotonicity(t *testing.T) {
	now := time.Now()

	// Valid monotonic timestamps
	entries := []LineageEntry{
		{Timestamp: now},
		{Timestamp: now.Add(1 * time.Second)},
		{Timestamp: now.Add(2 * time.Second)},
	}

	if !verifyTimestampMonotonicity(entries) {
		t.Error("monotonic timestamps should pass verification")
	}

	// Non-monotonic timestamps
	entries = []LineageEntry{
		{Timestamp: now},
		{Timestamp: now.Add(1 * time.Second)},
		{Timestamp: now}, // Goes backward
	}

	if verifyTimestampMonotonicity(entries) {
		t.Error("non-monotonic timestamps should fail verification")
	}

	// Equal timestamps (not strictly monotonic)
	entries = []LineageEntry{
		{Timestamp: now},
		{Timestamp: now}, // Equal to previous
	}

	if verifyTimestampMonotonicity(entries) {
		t.Error("equal timestamps should fail (not strictly monotonic) verification")
	}
}

// TestVerifyRequiredFields tests required field verification.
func TestVerifyRequiredFields(t *testing.T) {
	// All fields present
	entries := []LineageEntry{
		{
			EntryID: "entry-001",
			OrderID: "order-001",
			Step:    "step1",
			Data:    `{"test": 1}`,
			Hash:    "abcd1234",
		},
	}

	if !verifyRequiredFields(entries) {
		t.Error("entries with all required fields should pass verification")
	}

	// Missing EntryID
	entries = []LineageEntry{
		{
			OrderID: "order-001",
			Step:    "step1",
			Data:    `{"test": 1}`,
			Hash:    "abcd1234",
		},
	}

	if verifyRequiredFields(entries) {
		t.Error("entries with missing EntryID should fail verification")
	}

	// Missing Data
	entries = []LineageEntry{
		{
			EntryID: "entry-001",
			OrderID: "order-001",
			Step:    "step1",
			Hash:    "abcd1234",
		},
	}

	if verifyRequiredFields(entries) {
		t.Error("entries with missing Data should fail verification")
	}
}

// TestLineageJudge_Name tests judge name.
func TestLineageJudge_Name(t *testing.T) {
	judge := NewMockLineageJudge()
	if judge.Name() != "MockLineageJudge" {
		t.Errorf("expected name MockLineageJudge, got %s", judge.Name())
	}
}
