package lineage

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestLineageEntry_Serialize(t *testing.T) {
	entry := &LineageEntry{
		ID:           "entry:1",
		Timestamp:    time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		UserID:       "user:alice",
		ResourceID:   "msg:456",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionDenied,
		ReasonCode:   "rbac_mismatch",
		PolicyRule:   "message:rbac",
		AgentID:      "agent:searcher",
		SessionID:    "sess:xyz",
		TraceID:      "trace:abc123",
	}

	serialized := entry.Serialize()

	// Verify it's deterministic and contains all fields
	expected := "entry:1|2026-05-31T12:00:00Z|user:alice|msg:456|message|read|denied|rbac_mismatch|message:rbac|agent:searcher|sess:xyz|trace:abc123"
	if serialized != expected {
		t.Errorf("Serialize() = %q, want %q", serialized, expected)
	}

	// Verify second call produces same result
	serialized2 := entry.Serialize()
	if serialized != serialized2 {
		t.Error("Serialize() not deterministic")
	}
}

func TestLineageEntry_ComputeHash(t *testing.T) {
	entry := &LineageEntry{
		ID:           "entry:1",
		Timestamp:    time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		UserID:       "user:alice",
		ResourceID:   "msg:456",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionDenied,
		ReasonCode:   "rbac_mismatch",
	}

	// Compute hash with empty previous hash
	hash1 := entry.ComputeHash("")
	if hash1 == "" {
		t.Error("ComputeHash returned empty hash")
	}

	// Verify it's a valid SHA256 hash (64 hex chars)
	if len(hash1) != 64 {
		t.Errorf("ComputeHash hash length = %d, want 64", len(hash1))
	}

	// Verify hash is hex-encoded
	_, err := hex.DecodeString(hash1)
	if err != nil {
		t.Errorf("ComputeHash returned invalid hex: %v", err)
	}

	// Verify second call produces same result
	hash2 := entry.ComputeHash("")
	if hash1 != hash2 {
		t.Error("ComputeHash not deterministic")
	}

	// Verify different previous hash produces different result
	hash3 := entry.ComputeHash("differentprevhash")
	if hash1 == hash3 {
		t.Error("ComputeHash independent of previous hash")
	}

	// Verify manual calculation
	data := "" + entry.Serialize()
	h := sha256.Sum256([]byte(data))
	expected := hex.EncodeToString(h[:])
	if hash1 != expected {
		t.Errorf("ComputeHash mismatch: got %s, want %s", hash1, expected)
	}
}

func TestLineageEntry_ComputeHash_Chain(t *testing.T) {
	entry1 := &LineageEntry{
		ID:           "entry:1",
		Timestamp:    time.Now().UTC(),
		UserID:       "user:alice",
		ResourceID:   "msg:1",
		ResourceType: "message",
		Action:       ActionRead,
		Decision:     DecisionAllowed,
		ReasonCode:   "ok",
	}

	entry2 := &LineageEntry{
		ID:           "entry:2",
		Timestamp:    time.Now().UTC().Add(time.Second),
		UserID:       "user:bob",
		ResourceID:   "msg:2",
		ResourceType: "message",
		Action:       ActionWrite,
		Decision:     DecisionDenied,
		ReasonCode:   "denied",
	}

	// Compute hash chain
	hash1 := entry1.ComputeHash("")
	hash2 := entry2.ComputeHash(hash1)

	// Verify they're different
	if hash1 == hash2 {
		t.Error("Hash chain not unique for different entries")
	}

	// Verify hash1 can be recomputed
	hash1Again := entry1.ComputeHash("")
	if hash1 != hash1Again {
		t.Error("Hash not reproducible")
	}

	// Verify swapping order changes hash2
	hash2Alt := entry2.ComputeHash("")
	if hash2 == hash2Alt {
		t.Error("Hash independent of chain position")
	}
}

func TestAction_String(t *testing.T) {
	tests := []struct {
		action   Action
		expected string
	}{
		{ActionRead, "read"},
		{ActionWrite, "write"},
		{ActionDelete, "delete"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if test.action.String() != test.expected {
				t.Errorf("Action.String() = %q, want %q", test.action.String(), test.expected)
			}
		})
	}
}

func TestDecision_String(t *testing.T) {
	tests := []struct {
		decision Decision
		expected string
	}{
		{DecisionAllowed, "allowed"},
		{DecisionDenied, "denied"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if test.decision.String() != test.expected {
				t.Errorf("Decision.String() = %q, want %q", test.decision.String(), test.expected)
			}
		})
	}
}
