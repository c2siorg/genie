package memory

import (
	"context"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

// SemanticMemory is a per-user long-term memory backed by a rag.Index.
//
// Each user's memories live in a separate VectorStore so embeddings never
// mix between users — important for the residency and consent stories.
type SemanticMemory struct {
	Embedder rag.Embedder

	mu     sync.Mutex
	stores map[string]*rag.MemoryStore
}

// NewSemanticMemory builds an in-process semantic memory.
func NewSemanticMemory(e rag.Embedder) *SemanticMemory {
	return &SemanticMemory{Embedder: e, stores: map[string]*rag.MemoryStore{}}
}

func (m *SemanticMemory) storeFor(userID string) *rag.MemoryStore {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.stores[userID]
	if !ok {
		s = rag.NewMemoryStore()
		m.stores[userID] = s
	}
	return s
}

// Add stores text under (userID, key). Embeds and writes synchronously.
func (m *SemanticMemory) Add(ctx context.Context, userID, key, text string, meta map[string]any) error {
	v, err := m.Embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	return m.storeFor(userID).Upsert(ctx, rag.Chunk{
		ID: userID + "#" + key, Source: userID, Text: text, Metadata: meta,
	}, v)
}

// Search returns the user's most relevant memories for a query.
func (m *SemanticMemory) Search(ctx context.Context, userID, query string, topK int) ([]rag.ScoredChunk, error) {
	v, err := m.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return m.storeFor(userID).Search(ctx, v, topK)
}
