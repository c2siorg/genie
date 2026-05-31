// memory.go — Lesson 10: Persistent Memory Tools
//
// Four tools that give an agent durable memory across turns and sessions:
//
//   remember_fact  — write a named fact to long-term memory
//   recall_fact    — read one fact by key
//   list_facts     — read all current facts
//   forget_fact    — supersede (remove) a fact
//
// Usage:
//
//	store  := memory.NewLongTermMemory()
//	reg    := agenttools.MemoryTools(store, "user-123")
//	runner := agentic.Runner{Registry: reg}
//
// The runner also accepts a Memory field so it can auto-seed the system
// prompt with existing facts before the first LLM call.
package agenttools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/memory"
)

// RememberFact returns a tool that stores a persistent fact about the user.
// Calling it twice with the same key supersedes the old value — the store is
// append-only and keeps full history for audit.
func RememberFact(store *memory.LongTermMemory, userID string) Tool {
	return &ToolDef{
		ToolName: "remember_fact",
		ToolDescription: "Store a persistent fact about the user or task. Use for information that should survive across sessions: preferences, key decisions, account details, risk appetite, etc.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Canonical key, snake_case (e.g. primary_bank, risk_appetite, monthly_income)",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Human-readable fact value (e.g. HDFC, moderate, 1.2L)",
				},
				"source": map[string]any{
					"type":        "string",
					"description": "Optional: where this fact came from (e.g. user statement, document analysis)",
				},
			},
			"required": []string{"key", "value"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			key, _ := args["key"].(string)
			value, _ := args["value"].(string)
			source, _ := args["source"].(string)
			if key == "" || value == "" {
				return "error: key and value are required", nil
			}
			if source == "" {
				source = "agent inference"
			}
			store.Record(userID, memory.Fact{
				Key:        key,
				Value:      value,
				Source:     source,
				Confidence: 1.0,
				RecordedAt: time.Now().UTC(),
			})
			return fmt.Sprintf("remembered: %s = %q  (source: %s)", key, value, source), nil
		},
	}
}

// RecallFact returns a tool that retrieves one fact by its key.
func RecallFact(store *memory.LongTermMemory, userID string) Tool {
	return &ToolDef{
		ToolName: "recall_fact",
		ToolDescription: "Retrieve a specific remembered fact by its key. Returns the current value and when it was recorded.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "The exact fact key to look up",
				},
			},
			"required": []string{"key"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			key, _ := args["key"].(string)
			if key == "" {
				return "error: key is required", nil
			}
			fact, ok := store.Current(userID, key)
			if !ok {
				return fmt.Sprintf("no fact found for key %q", key), nil
			}
			return fmt.Sprintf("%s = %q  (recorded %s, source: %s, confidence: %.0f%%)",
				fact.Key, fact.Value,
				fact.RecordedAt.Format("2006-01-02"),
				fact.Source,
				fact.Confidence*100,
			), nil
		},
	}
}

// ListFacts returns a tool that lists all current (non-superseded) facts.
func ListFacts(store *memory.LongTermMemory, userID string) Tool {
	return &ToolDef{
		ToolName: "list_facts",
		ToolDescription: "List all currently remembered facts about the user. Returns a structured summary of everything stored in long-term memory.",
		ToolSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			facts := store.CurrentAll(userID)
			if len(facts) == 0 {
				return "no facts stored yet", nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%d fact(s) on record:\n", len(facts)))
			for _, f := range facts {
				sb.WriteString(fmt.Sprintf("  • %s: %s  [%s]\n", f.Key, f.Value, f.RecordedAt.Format("2006-01-02")))
			}
			return sb.String(), nil
		},
	}
}

// ForgetFact returns a tool that supersedes (effectively removes) a fact.
// The old value is retained in history for audit — only its current flag is cleared.
func ForgetFact(store *memory.LongTermMemory, userID string) Tool {
	return &ToolDef{
		ToolName: "forget_fact",
		ToolDescription: "Remove (supersede) a remembered fact. The value is cleared from active memory but retained in audit history. Use when information is no longer accurate.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "The fact key to remove",
				},
			},
			"required": []string{"key"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			key, _ := args["key"].(string)
			if key == "" {
				return "error: key is required", nil
			}
			if _, ok := store.Current(userID, key); !ok {
				return fmt.Sprintf("no active fact found for key %q — nothing to forget", key), nil
			}
			// Supersede by writing an empty sentinel fact.
			t := time.Now().UTC()
			store.Record(userID, memory.Fact{
				Key:        key,
				Value:      "",
				Source:     "forgotten by agent",
				Confidence: 1.0,
				RecordedAt: t,
			})
			// Mark old as superseded: the Record() call already handles this.
			return fmt.Sprintf("forgot: %s (cleared at %s)", key, t.Format("2006-01-02")), nil
		},
	}
}

// SearchFacts returns a tool that does a substring search across active facts.
func SearchFacts(store *memory.LongTermMemory, userID string) Tool {
	return &ToolDef{
		ToolName: "search_facts",
		ToolDescription: "Search remembered facts by keyword. Useful when you know something was stored but don't remember the exact key.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Keyword or phrase to search for in keys and values",
				},
			},
			"required": []string{"query"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			if query == "" {
				return "error: query is required", nil
			}
			facts := store.SearchValue(userID, query)
			if len(facts) == 0 {
				return fmt.Sprintf("no facts match %q", query), nil
			}
			var sb strings.Builder
			for _, f := range facts {
				sb.WriteString(fmt.Sprintf("  • %s: %s\n", f.Key, f.Value))
			}
			return sb.String(), nil
		},
	}
}

// MemoryTools returns a Registry pre-loaded with all five memory tools,
// scoped to userID in the given long-term memory store.
//
// Combine with agentic.Runner.Memory to also auto-seed the system prompt:
//
//	runner := agentic.Runner{
//	    Registry: agenttools.MemoryTools(store, userID),
//	    Memory:   store,
//	    UserID:   userID,
//	}
func MemoryTools(store *memory.LongTermMemory, userID string) *Registry {
	r := NewRegistry()
	r.Register(RememberFact(store, userID))
	r.Register(RecallFact(store, userID))
	r.Register(ListFacts(store, userID))
	r.Register(ForgetFact(store, userID))
	r.Register(SearchFacts(store, userID))
	return r
}

// FactsSummary formats current facts as a system-prompt paragraph.
// Used internally by Runner.buildMessages when Memory is set.
func FactsSummary(store *memory.LongTermMemory, userID string) string {
	facts := store.CurrentAll(userID)
	// Skip empty-value sentinel facts (from ForgetFact).
	var active []memory.Fact
	for _, f := range facts {
		if f.Value != "" {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## What you know about this user (long-term memory):\n")
	for _, f := range active {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
	}
	return sb.String()
}
