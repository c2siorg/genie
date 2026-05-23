package rag

import (
	"context"
	"strings"
	"testing"
)

func TestIndex_RoundTripWithHashEmbedder(t *testing.T) {
	idx := NewIndex(NewHashEmbedder(128), NewMemoryStore())
	ctx := context.Background()

	corpus := []struct{ src, title, body string }{
		{"free-ai#sutra-1", "Sutra 1", "Trust is the foundation. Trust is non-negotiable and should remain uncompromised."},
		{"free-ai#sutra-7", "Sutra 7", "Safety, resilience and sustainability. AI systems should be secure, resilient and energy efficient."},
		{"free-ai#rec-20", "Rec 20 Red Teaming", "REs should establish structured red teaming processes across the AI lifecycle."},
		{"unrelated", "Off-topic", "The capital of France is Paris and the Seine flows through it."},
	}
	for _, c := range corpus {
		if _, err := idx.IngestDocument(ctx, c.src, c.title, c.body, 800); err != nil {
			t.Fatal(err)
		}
	}

	res, err := idx.Search(ctx, "red teaming AI lifecycle", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("expected at least one hit")
	}
	if !strings.Contains(res[0].Text, "red teaming") {
		t.Fatalf("top hit not red-team chunk: %+v", res[0])
	}
	if res[0].Source != "free-ai#rec-20" {
		t.Errorf("expected red-team source, got %q", res[0].Source)
	}
}

func TestMemoryStore_ReplacesById(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	_ = s.Upsert(ctx, Chunk{ID: "a", Text: "v1"}, []float32{1, 0})
	_ = s.Upsert(ctx, Chunk{ID: "a", Text: "v2"}, []float32{1, 0})
	if s.Len() != 1 {
		t.Fatalf("expected dedup, got len %d", s.Len())
	}
}
