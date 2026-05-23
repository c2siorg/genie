package reasoning

import (
	"context"
	"errors"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

// SemanticRouter classifies an input query to one of N routes using
// embedding similarity to a small per-route exemplar set. Cheaper than an
// LLM call and deterministic.
type SemanticRouter struct {
	Embedder rag.Embedder
	routes   []routeEntry
}

type routeEntry struct {
	ID         string
	Exemplars  [][]float32
}

// NewSemanticRouter builds a router around an Embedder.
func NewSemanticRouter(e rag.Embedder) *SemanticRouter { return &SemanticRouter{Embedder: e} }

// Register a route with the given id and a list of exemplar phrases.
func (r *SemanticRouter) Register(ctx context.Context, id string, exemplars []string) error {
	if len(exemplars) == 0 {
		return errors.New("router: at least one exemplar required")
	}
	entry := routeEntry{ID: id}
	for _, ex := range exemplars {
		v, err := r.Embedder.Embed(ctx, ex)
		if err != nil {
			return err
		}
		entry.Exemplars = append(entry.Exemplars, v)
	}
	r.routes = append(r.routes, entry)
	return nil
}

// Route is one candidate destination, returned sorted by score (desc).
type Route struct {
	ID    string
	Score float32
}

// Classify returns the routes sorted by best-match cosine similarity.
func (r *SemanticRouter) Classify(ctx context.Context, query string) ([]Route, error) {
	qv, err := r.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]Route, 0, len(r.routes))
	for _, route := range r.routes {
		best := float32(-2.0)
		for _, ex := range route.Exemplars {
			s := cosineFloat32(qv, ex)
			if s > best {
				best = s
			}
		}
		out = append(out, Route{ID: route.ID, Score: best})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

// cosineFloat32 — duplicated here to avoid importing internals of pkg/rag;
// kept short to limit surface area.
func cosineFloat32(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
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
