package agenttools_test

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/memory"
)

const testUser = "user-test-001"

func newMemStore() *memory.LongTermMemory { return memory.NewLongTermMemory() }

// ─── RememberFact ─────────────────────────────────────────────────────────

func TestRememberFact_Stores(t *testing.T) {
	store := newMemStore()
	tool := agenttools.RememberFact(store, testUser)
	result, err := tool.Execute(context.Background(), map[string]any{
		"key": "primary_bank", "value": "HDFC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected confirmation message")
	}
	f, ok := store.Current(testUser, "primary_bank")
	if !ok {
		t.Fatal("fact not stored")
	}
	if f.Value != "HDFC" {
		t.Errorf("got %q want %q", f.Value, "HDFC")
	}
}

func TestRememberFact_MissingKey(t *testing.T) {
	tool := agenttools.RememberFact(newMemStore(), testUser)
	result, _ := tool.Execute(context.Background(), map[string]any{"value": "HDFC"})
	if result == "" {
		t.Error("expected error string for missing key")
	}
}

// ─── RecallFact ───────────────────────────────────────────────────────────

func TestRecallFact_Found(t *testing.T) {
	store := newMemStore()
	agenttools.RememberFact(store, testUser).Execute(context.Background(), map[string]any{
		"key": "risk_appetite", "value": "moderate",
	})
	result, err := agenttools.RecallFact(store, testUser).Execute(
		context.Background(), map[string]any{"key": "risk_appetite"})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result, "moderate") {
		t.Errorf("expected 'moderate' in %q", result)
	}
}

func TestRecallFact_NotFound(t *testing.T) {
	result, _ := agenttools.RecallFact(newMemStore(), testUser).Execute(
		context.Background(), map[string]any{"key": "nonexistent"})
	if result == "" {
		t.Error("expected error string for missing key")
	}
}

// ─── ListFacts ────────────────────────────────────────────────────────────

func TestListFacts_Empty(t *testing.T) {
	result, _ := agenttools.ListFacts(newMemStore(), testUser).Execute(
		context.Background(), nil)
	if result == "" {
		t.Error("expected 'no facts' message")
	}
}

func TestListFacts_WithFacts(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	agenttools.RememberFact(store, testUser).Execute(ctx, map[string]any{"key": "k1", "value": "v1"})
	agenttools.RememberFact(store, testUser).Execute(ctx, map[string]any{"key": "k2", "value": "v2"})
	result, _ := agenttools.ListFacts(store, testUser).Execute(ctx, nil)
	if !contains(result, "k1") || !contains(result, "k2") {
		t.Errorf("expected both keys in %q", result)
	}
}

// ─── ForgetFact ───────────────────────────────────────────────────────────

func TestForgetFact_Clears(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	agenttools.RememberFact(store, testUser).Execute(ctx, map[string]any{"key": "old_bank", "value": "SBI"})
	agenttools.ForgetFact(store, testUser).Execute(ctx, map[string]any{"key": "old_bank"})
	// After forget, list_facts should show no active non-empty fact.
	result, _ := agenttools.ListFacts(store, testUser).Execute(ctx, nil)
	// The empty sentinel shouldn't surface as a useful fact.
	if contains(result, "SBI") {
		t.Errorf("forgotten fact should not appear in listing: %q", result)
	}
}

func TestForgetFact_NotFound(t *testing.T) {
	result, _ := agenttools.ForgetFact(newMemStore(), testUser).Execute(
		context.Background(), map[string]any{"key": "ghost"})
	if result == "" {
		t.Error("expected message for missing key")
	}
}

// ─── SearchFacts ──────────────────────────────────────────────────────────

func TestSearchFacts_Found(t *testing.T) {
	store := newMemStore()
	ctx := context.Background()
	agenttools.RememberFact(store, testUser).Execute(ctx, map[string]any{"key": "employer", "value": "Infosys"})
	result, _ := agenttools.SearchFacts(store, testUser).Execute(ctx, map[string]any{"query": "infosys"})
	if !contains(result, "employer") {
		t.Errorf("expected 'employer' in %q", result)
	}
}

// ─── MemoryTools registry ─────────────────────────────────────────────────

func TestMemoryTools_Registry(t *testing.T) {
	reg := agenttools.MemoryTools(newMemStore(), testUser)
	defs := reg.Definitions()
	if len(defs) != 5 {
		t.Errorf("expected 5 memory tools, got %d", len(defs))
	}
}

// ─── FactsSummary ─────────────────────────────────────────────────────────

func TestFactsSummary_Empty(t *testing.T) {
	summary := agenttools.FactsSummary(newMemStore(), testUser)
	if summary != "" {
		t.Errorf("expected empty string for no facts, got %q", summary)
	}
}

func TestFactsSummary_WithFacts(t *testing.T) {
	store := newMemStore()
	agenttools.RememberFact(store, testUser).Execute(context.Background(), map[string]any{
		"key": "city", "value": "Mumbai",
	})
	summary := agenttools.FactsSummary(store, testUser)
	if !contains(summary, "city") || !contains(summary, "Mumbai") {
		t.Errorf("expected city/Mumbai in summary: %q", summary)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (s == sub || len(s) >= len(sub) &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
