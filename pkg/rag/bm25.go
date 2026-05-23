package rag

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
)

// BM25Store is a keyword (sparse) retriever. Pairs with a VectorStore in a
// hybrid setup — vectors handle semantics, BM25 handles exact-term matches.
//
// Implementation: textbook BM25 with k1=1.5, b=0.75. Tokenisation uses the
// same lowercased-alphanumeric tokeniser as HashEmbedder so vector and BM25
// see the same surface forms.
type BM25Store struct {
	k1, b float64
	mu    sync.RWMutex

	docs   []bm25Doc
	df     map[string]int     // document frequency per term
	avgDL  float64
}

type bm25Doc struct {
	chunk Chunk
	tf    map[string]int
	dl    int
}

// NewBM25Store builds an empty store with default k1=1.5, b=0.75.
func NewBM25Store() *BM25Store {
	return &BM25Store{k1: 1.5, b: 0.75, df: map[string]int{}}
}

// Add indexes a chunk.
func (s *BM25Store) Add(c Chunk) {
	tokens := tokenize(c.Text)
	tf := map[string]int{}
	for _, t := range tokens {
		tf[t]++
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for term := range tf {
		s.df[term]++
	}
	s.docs = append(s.docs, bm25Doc{chunk: c, tf: tf, dl: len(tokens)})
	// Recompute average doc length (cheap; <few-thousand chunks).
	var sum int
	for _, d := range s.docs {
		sum += d.dl
	}
	if len(s.docs) > 0 {
		s.avgDL = float64(sum) / float64(len(s.docs))
	}
}

// Len returns the number of indexed chunks.
func (s *BM25Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}

// Search returns the top-K BM25 hits.
func (s *BM25Store) Search(_ context.Context, query string, topK int) []ScoredChunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	N := float64(len(s.docs))
	if N == 0 {
		return nil
	}
	terms := tokenize(query)
	scored := make([]ScoredChunk, 0, len(s.docs))
	for _, d := range s.docs {
		var score float64
		for _, t := range terms {
			n := float64(s.df[t])
			if n == 0 {
				continue
			}
			tf := float64(d.tf[t])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (N-n+0.5)/(n+0.5))
			denom := tf + s.k1*(1-s.b+s.b*float64(d.dl)/s.avgDL)
			score += idf * (tf * (s.k1 + 1) / denom)
		}
		if score > 0 {
			scored = append(scored, ScoredChunk{Chunk: d.chunk, Score: float32(score)})
		}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	if topK > 0 && topK < len(scored) {
		scored = scored[:topK]
	}
	return scored
}

// HybridSearch fuses BM25 + vector results via Reciprocal Rank Fusion.
//
// RRF formula: sum 1/(k + rank). The k constant softens the impact of any
// single retriever. We pick k=60 — the value popularised by Cormack 2009 and
// adopted by every production hybrid retriever since.
func HybridSearch(ctx context.Context, vstore VectorStore, embedder Embedder, bm *BM25Store, query string, topK int) ([]ScoredChunk, error) {
	const rrfK = 60.0
	vec, err := embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	vecHits, err := vstore.Search(ctx, vec, topK*4)
	if err != nil {
		return nil, err
	}
	bmHits := bm.Search(ctx, query, topK*4)

	merged := map[string]*ScoredChunk{}
	for rank, h := range vecHits {
		c := h
		c.Score = float32(1.0 / (rrfK + float64(rank)))
		merged[h.ID] = &c
	}
	for rank, h := range bmHits {
		add := float32(1.0 / (rrfK + float64(rank)))
		if existing, ok := merged[h.ID]; ok {
			existing.Score += add
		} else {
			c := h
			c.Score = add
			merged[h.ID] = &c
		}
	}
	out := make([]ScoredChunk, 0, len(merged))
	for _, c := range merged {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if topK > 0 && topK < len(out) {
		out = out[:topK]
	}
	return out, nil
}

// TimeDecay rescales scored chunks by exp(-lambda * age_days). Use lambda
// around 0.01 to halve relevance ~70 days out — sane for financial context.
func TimeDecay(chunks []ScoredChunk, lambda float64, ageDaysOf func(Chunk) float64) []ScoredChunk {
	out := make([]ScoredChunk, len(chunks))
	for i, c := range chunks {
		age := ageDaysOf(c.Chunk)
		out[i] = c
		out[i].Score = c.Score * float32(math.Exp(-lambda*age))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// ParentChildChunk splits text into small sentence-level child chunks and
// keeps the larger parent context attached to each. Retrieval matches the
// child (precision) but the parent text is what gets handed to the LLM
// (context). Classic LangChain-style split.
type ParentChild struct {
	ParentText string
	Children   []string
}

// SplitParentChild returns parent/child chunks. parentSize is in characters;
// childSize is the per-sentence cap.
func SplitParentChild(text string, parentSize, childSize int) []ParentChild {
	if parentSize <= 0 {
		parentSize = 1200
	}
	if childSize <= 0 {
		childSize = 240
	}
	parents := chunk(text, parentSize)
	out := make([]ParentChild, 0, len(parents))
	for _, p := range parents {
		out = append(out, ParentChild{
			ParentText: p,
			Children:   chunk(p, childSize),
		})
	}
	return out
}

// QueryRewriter is the interface a query rewriter implements. The default
// LLMQueryRewriter uses any llm.Provider; HyDE wraps it.
type QueryRewriter interface {
	Rewrite(ctx context.Context, original string) ([]string, error)
}

// PassthroughRewriter returns the query unchanged. Used as the default.
type PassthroughRewriter struct{}

// Rewrite returns the input as a one-element slice.
func (PassthroughRewriter) Rewrite(_ context.Context, q string) ([]string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	return []string{q}, nil
}
