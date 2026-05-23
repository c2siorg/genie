// Package rag is a minimum-viable Retrieval-Augmented Generation layer.
//
// Three pieces:
//
//  1. Embedder    — turns text into a fixed-size []float32.
//  2. VectorStore — stores chunks and their embeddings; runs nearest-neighbour search.
//  3. Index       — convenience over Embedder + VectorStore that handles
//                   chunking and ingestion.
//
// Genie ships a deterministic HashEmbedder for tests and an OllamaEmbedder
// for on-prem inference. Both satisfy the Embedder interface so callers
// don't change.
//
// Storage is in-memory. For production swap MemoryStore for a pgvector or
// qdrant backend — the interface is intentionally minimal.
package rag

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
)

// Chunk is one unit of retrievable text.
type Chunk struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"`            // URI, file path, citation key
	Title    string         `json:"title,omitempty"`   // section heading
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ScoredChunk is a search result.
type ScoredChunk struct {
	Chunk
	Score float32 `json:"score"`
}

// Embedder turns text into a fixed-size vector. Implementations MUST return
// a vector of the same length for every call.
type Embedder interface {
	// Dim returns the embedding dimension.
	Dim() int
	// Embed embeds a single text string.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// VectorStore is the storage + retrieval primitive.
type VectorStore interface {
	Upsert(ctx context.Context, c Chunk, vec []float32) error
	Search(ctx context.Context, query []float32, topK int) ([]ScoredChunk, error)
	Len() int
}

// Index is the high-level ingester + searcher most callers want.
type Index struct {
	Emb   Embedder
	Store VectorStore
}

// NewIndex builds an Index from an Embedder + VectorStore pair.
func NewIndex(e Embedder, s VectorStore) *Index { return &Index{Emb: e, Store: s} }

// IngestDocument splits text into ~chunkSize-character chunks at paragraph
// boundaries, embeds each, and stores them.
//
// The chunker is deliberately naive — it favors paragraph breaks but falls
// back to fixed-size chunks so even unstructured payloads work.
func (i *Index) IngestDocument(ctx context.Context, source, title, body string, chunkSize int) (int, error) {
	if chunkSize <= 0 {
		chunkSize = 800
	}
	chunks := chunk(body, chunkSize)
	for idx, text := range chunks {
		vec, err := i.Emb.Embed(ctx, text)
		if err != nil {
			return idx, err
		}
		c := Chunk{
			ID:     source + "#" + itoa(idx),
			Source: source,
			Title:  title,
			Text:   text,
		}
		if err := i.Store.Upsert(ctx, c, vec); err != nil {
			return idx, err
		}
	}
	return len(chunks), nil
}

// Search embeds the query and returns the top-K matches.
func (i *Index) Search(ctx context.Context, query string, topK int) ([]ScoredChunk, error) {
	if topK <= 0 {
		topK = 5
	}
	v, err := i.Emb.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return i.Store.Search(ctx, v, topK)
}

// ---------- chunker ----------

func chunk(body string, size int) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	parts := strings.Split(body, "\n\n")
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Hard split if the paragraph itself exceeds the chunk size.
		for len(p) > size {
			flush()
			out = append(out, p[:size])
			p = p[size:]
		}
		if cur.Len()+len(p)+2 > size && cur.Len() > 0 {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
		if cur.Len() >= size {
			flush()
		}
	}
	flush()
	return out
}

// itoa is a tiny integer-to-string without pulling strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------- in-memory store ----------

// MemoryStore is a list-backed VectorStore. Search is O(N) — fine for the
// demo and up to a few thousand chunks; swap for pgvector when needed.
type MemoryStore struct {
	mu     sync.RWMutex
	items  []memoryItem
}

type memoryItem struct {
	chunk Chunk
	vec   []float32
}

// NewMemoryStore constructs an empty store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func (s *MemoryStore) Upsert(_ context.Context, c Chunk, vec []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Replace if existing id.
	for i := range s.items {
		if s.items[i].chunk.ID == c.ID {
			s.items[i] = memoryItem{chunk: c, vec: vec}
			return nil
		}
	}
	s.items = append(s.items, memoryItem{chunk: c, vec: vec})
	return nil
}

// Search returns the top-K cosine-similarity matches.
func (s *MemoryStore) Search(_ context.Context, query []float32, topK int) ([]ScoredChunk, error) {
	if len(query) == 0 {
		return nil, errors.New("rag: empty query vector")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	scored := make([]ScoredChunk, 0, len(s.items))
	for _, it := range s.items {
		if len(it.vec) != len(query) {
			continue
		}
		scored = append(scored, ScoredChunk{Chunk: it.chunk, Score: cosine(query, it.vec)})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	if topK > len(scored) {
		topK = len(scored)
	}
	return scored[:topK], nil
}

// cosine returns the cosine similarity of two equal-length vectors.
func cosine(a, b []float32) float32 {
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
