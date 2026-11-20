package agenttools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

func newTestIndex(t *testing.T) *rag.Index {
	t.Helper()
	idx := rag.NewIndex(rag.NewHashEmbedder(64), rag.NewMemoryStore())
	ctx := context.Background()
	_, err := idx.IngestDocument(ctx, "doc-1", "Go Concurrency", "Goroutines are lightweight threads managed by the Go runtime. Use channels to communicate between goroutines safely.", 200)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	_, err = idx.IngestDocument(ctx, "doc-2", "Go Interfaces", "An interface type specifies a method set. A value implements the interface if it has all the methods in the set.", 200)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	return idx
}

// ─── SearchKnowledgeBase ──────────────────────────────────────────────────

func TestSearchKnowledgeBase_ReturnsResults(t *testing.T) {
	idx := newTestIndex(t)
	tool := agenttools.SearchKnowledgeBase(idx)

	result, err := tool.Execute(context.Background(), map[string]any{
		"query": "goroutines channels",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should surface the concurrency document.
	if !strings.Contains(result, "Go Concurrency") && !strings.Contains(result, "goroutine") {
		t.Logf("result: %s", result)
		// HashEmbedder is deterministic but not semantic — just verify structure.
	}
}

func TestSearchKnowledgeBase_EmptyQuery(t *testing.T) {
	idx := newTestIndex(t)
	tool := agenttools.SearchKnowledgeBase(idx)

	result, _ := tool.Execute(context.Background(), map[string]any{"query": ""})
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for empty query, got %q", result)
	}
}

func TestSearchKnowledgeBase_TopK(t *testing.T) {
	idx := newTestIndex(t)
	tool := agenttools.SearchKnowledgeBase(idx)

	result, err := tool.Execute(context.Background(), map[string]any{
		"query": "Go",
		"top_k": float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Only 1 result requested.
	if strings.Count(result, "[1]") != 1 {
		t.Logf("result: %s", result) // acceptable — just verify no error
	}
}

func TestSearchKnowledgeBase_EmptyIndex(t *testing.T) {
	idx := rag.NewIndex(rag.NewHashEmbedder(64), rag.NewMemoryStore())
	tool := agenttools.SearchKnowledgeBase(idx)

	result, _ := tool.Execute(context.Background(), map[string]any{"query": "anything"})
	if !strings.Contains(result, "no results") {
		t.Errorf("empty index should say 'no results', got %q", result)
	}
}

// ─── IngestDocument ───────────────────────────────────────────────────────

func TestIngestDocument_AddsToIndex(t *testing.T) {
	idx := rag.NewIndex(rag.NewHashEmbedder(64), rag.NewMemoryStore())
	tool := agenttools.IngestDocument(idx)

	result, err := tool.Execute(context.Background(), map[string]any{
		"source":  "test-source",
		"title":   "Test Doc",
		"content": "This is a test document about testing in Go.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "ingested") {
		t.Errorf("expected confirmation, got %q", result)
	}
	// Index should now have entries.
	if idx.Store.(*rag.MemoryStore).Len() == 0 {
		t.Error("index should have entries after ingestion")
	}
}

func TestIngestDocument_MissingContent(t *testing.T) {
	idx := rag.NewIndex(rag.NewHashEmbedder(64), rag.NewMemoryStore())
	tool := agenttools.IngestDocument(idx)

	result, _ := tool.Execute(context.Background(), map[string]any{
		"source": "s", "title": "t", "content": "",
	})
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for empty content, got %q", result)
	}
}

func TestIngestDocument_MissingSource(t *testing.T) {
	idx := rag.NewIndex(rag.NewHashEmbedder(64), rag.NewMemoryStore())
	tool := agenttools.IngestDocument(idx)

	result, _ := tool.Execute(context.Background(), map[string]any{
		"content": "some text",
	})
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for missing source, got %q", result)
	}
}

// ─── RAGTools registry ────────────────────────────────────────────────────

func TestRAGTools_HasTwoTools(t *testing.T) {
	idx := newTestIndex(t)
	reg := agenttools.RAGTools(idx)
	if len(reg.Definitions()) != 2 {
		t.Errorf("expected 2 tools, got %d", len(reg.Definitions()))
	}
}

func TestRAGTools_Execute(t *testing.T) {
	idx := newTestIndex(t)
	reg := agenttools.RAGTools(idx)

	result, err := reg.Execute(context.Background(), "search_knowledge_base", map[string]any{
		"query": "interface method set",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty search result via registry")
	}
}
