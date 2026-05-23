package rag

import (
	"context"
	"strings"
	"testing"
)

func TestBM25_RetrievesExactMatch(t *testing.T) {
	bm := NewBM25Store()
	bm.Add(Chunk{ID: "a", Text: "red teaming for AI safety"})
	bm.Add(Chunk{ID: "b", Text: "deep learning architectures"})
	hits := bm.Search(context.Background(), "red teaming", 2)
	if len(hits) == 0 || hits[0].ID != "a" {
		t.Fatalf("expected chunk a, got %+v", hits)
	}
}

func TestHybrid_FusesVectorAndBM25(t *testing.T) {
	ctx := context.Background()
	vs := NewMemoryStore()
	emb := NewHashEmbedder(64)
	bm := NewBM25Store()

	chunks := []Chunk{
		{ID: "a", Text: "red teaming for AI safety"},
		{ID: "b", Text: "deep learning architectures"},
		{ID: "c", Text: "AI safety practices include adversarial testing"},
	}
	for _, c := range chunks {
		v, _ := emb.Embed(ctx, c.Text)
		_ = vs.Upsert(ctx, c, v)
		bm.Add(c)
	}
	hits, err := HybridSearch(ctx, vs, emb, bm, "adversarial AI safety", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if !strings.Contains(strings.ToLower(hits[0].Text), "safety") {
		t.Fatalf("top hit not safety chunk: %+v", hits[0])
	}
}

func TestSplitParentChild_SplitsAtTwoLevels(t *testing.T) {
	body := strings.Repeat("Hello world. ", 200)
	out := SplitParentChild(body, 400, 100)
	if len(out) == 0 || len(out[0].Children) == 0 {
		t.Fatalf("expected parent + children, got %+v", out)
	}
	if len(out[0].ParentText) > 500 {
		t.Errorf("parent too large: %d", len(out[0].ParentText))
	}
}
