// Package pgvector backs pkg/rag.VectorStore with PostgreSQL's pgvector
// extension. Same interface as MemoryStore — only the wiring changes.
//
// Use a separate namespace per logical corpus (e.g. "free-ai", "user-memory")
// so cosine search stays scoped.
package pgvector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements rag.VectorStore against pgvector.
type Store struct {
	Pool      *pgxpool.Pool
	Namespace string
}

// New constructs a Store. namespace separates corpora ("free-ai", "user-u1").
func New(pool *pgxpool.Pool, namespace string) *Store {
	return &Store{Pool: pool, Namespace: namespace}
}

// Len returns the number of stored chunks in this namespace.
func (s *Store) Len() int {
	var n int
	_ = s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM rag_embeddings WHERE namespace = $1`, s.Namespace).Scan(&n)
	return n
}

// Upsert writes one chunk + its vector.
func (s *Store) Upsert(ctx context.Context, c rag.Chunk, vec []float32) error {
	if len(vec) == 0 {
		return errors.New("pgvector: empty vector")
	}
	var meta []byte
	if len(c.Metadata) > 0 {
		var err error
		meta, err = json.Marshal(c.Metadata)
		if err != nil {
			return err
		}
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO rag_embeddings (namespace, chunk_id, source, title, body, metadata, embedding)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::vector)
		 ON CONFLICT (namespace, chunk_id) DO UPDATE SET
		   source = EXCLUDED.source,
		   title = EXCLUDED.title,
		   body = EXCLUDED.body,
		   metadata = EXCLUDED.metadata,
		   embedding = EXCLUDED.embedding,
		   created_at = now()`,
		s.Namespace, c.ID, c.Source, c.Title, c.Text, meta, vectorLiteral(vec),
	)
	return err
}

// Search returns the top-K nearest neighbours by cosine distance.
// pgvector's `<=>` operator is "cosine distance" — smaller is closer; we
// convert to similarity = 1 - distance so callers compare like cosine sim.
func (s *Store) Search(ctx context.Context, query []float32, topK int) ([]rag.ScoredChunk, error) {
	if topK <= 0 {
		topK = 5
	}
	if len(query) == 0 {
		return nil, errors.New("pgvector: empty query vector")
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT chunk_id, source, title, body, metadata, embedding <=> $1::vector AS dist
		   FROM rag_embeddings
		  WHERE namespace = $2
		  ORDER BY embedding <=> $1::vector ASC
		  LIMIT $3`,
		vectorLiteral(query), s.Namespace, topK,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rag.ScoredChunk
	for rows.Next() {
		var c rag.Chunk
		var meta []byte
		var dist float64
		if err := rows.Scan(&c.ID, &c.Source, &c.Title, &c.Text, &meta, &dist); err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &c.Metadata)
		}
		out = append(out, rag.ScoredChunk{Chunk: c, Score: float32(1.0 - dist)})
	}
	return out, rows.Err()
}

// vectorLiteral formats a Go slice as pgvector's text input format:
// "[1,2,3]". pgvector accepts this when the column is cast to ::vector.
func vectorLiteral(v []float32) string {
	var sb strings.Builder
	sb.Grow(len(v) * 6)
	sb.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(x), 'f', -1, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Compile-time check.
var _ rag.VectorStore = (*Store)(nil)

// PingExtension verifies the pgvector extension is installed. Useful from
// the readiness probe before announcing a Postgres-backed RAG store ready.
func PingExtension(ctx context.Context, pool *pgxpool.Pool) error {
	var name string
	if err := pool.QueryRow(ctx,
		`SELECT extname FROM pg_extension WHERE extname = 'vector'`,
	).Scan(&name); err != nil {
		return fmt.Errorf("pgvector extension not installed: %w", err)
	}
	return nil
}
