package rag

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

func TestSelfRAG_RespectsModelDecision(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{{Text: "NO"}}
	s := NewSelfRAG(mock, "test")
	need, err := s.Should(context.Background(), "what is 2+2?")
	if err != nil {
		t.Fatal(err)
	}
	if need {
		t.Fatal("expected NO (no retrieval needed)")
	}
}

func TestCRAG_DropsLowScore(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{{Text: "0.9\n0.2\n0.7"}}
	c := NewCRAG(mock, "test", 0.5)
	chunks := []ScoredChunk{
		{Chunk: Chunk{ID: "a", Text: "relevant"}},
		{Chunk: Chunk{ID: "b", Text: "irrelevant"}},
		{Chunk: Chunk{ID: "c", Text: "ok"}},
	}
	res, err := c.Grade(context.Background(), "query", chunks)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Kept) != 2 || len(res.Dropped) != 1 {
		t.Fatalf("expected 2 kept / 1 dropped, got %+v", res)
	}
	if res.Confidence < 0.5 {
		t.Errorf("confidence: %f", res.Confidence)
	}
}

func TestLostInMiddleReorder(t *testing.T) {
	chunks := []ScoredChunk{
		{Chunk: Chunk{ID: "best"}},
		{Chunk: Chunk{ID: "2nd"}},
		{Chunk: Chunk{ID: "3rd"}},
		{Chunk: Chunk{ID: "4th"}},
	}
	out := LostInMiddleReorder(chunks)
	if out[0].ID != "best" {
		t.Fatalf("first should be best, got %q", out[0].ID)
	}
	if out[len(out)-1].ID != "2nd" {
		t.Fatalf("last should be 2nd (reversed-odd tail), got %q", out[len(out)-1].ID)
	}
}
