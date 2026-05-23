package rag

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

func TestLLMReranker_OrdersByScore(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{
		{Text: "2\n9\n5"}, // candidate index 1 wins.
	}
	r := NewLLMReranker(mock, "test")
	cands := []ScoredChunk{
		{Chunk: Chunk{ID: "a", Text: "irrelevant"}},
		{Chunk: Chunk{ID: "b", Text: "perfect match"}},
		{Chunk: Chunk{ID: "c", Text: "ok"}},
	}
	out, err := r.Rerank(context.Background(), "query", cands, 3)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].ID != "b" {
		t.Fatalf("expected b on top, got %+v", out)
	}
}
