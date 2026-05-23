// Package aibom is Genie's AI Bill of Materials. Each agent (and provider)
// can supply a Manifest describing model id, version, training data class,
// region, and dependencies. The /v1/aibom endpoint emits the full document
// for regulators and auditors.
//
// AIBOMs are an emerging supply-chain primitive — similar in spirit to
// SBOM/CycloneDX but for AI components. Genie's shape borrows from the
// CycloneDX 1.6 ML-BOM extension.
package aibom

import (
	"sort"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

// TrainingDataClass categorises the data the underlying model was trained on.
type TrainingDataClass string

const (
	TrainingPublicCorpora TrainingDataClass = "public_corpora"
	TrainingProprietary   TrainingDataClass = "proprietary"
	TrainingRegulated     TrainingDataClass = "regulated_sensitive"
	TrainingUnknown       TrainingDataClass = "unknown"
)

// Manifest describes one component (agent / model / provider).
type Manifest struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Kind              string            `json:"kind"`              // "agent" | "llm" | "embedder" | "reranker" | "tool"
	Version           string            `json:"version,omitempty"`
	Model             string            `json:"model,omitempty"`
	Provider          string            `json:"provider,omitempty"`
	Region            string            `json:"region,omitempty"`
	TrainingDataClass TrainingDataClass `json:"training_data_class"`
	RiskClass         agent.RiskClass   `json:"risk_class,omitempty"`
	Capabilities      []string          `json:"capabilities,omitempty"`
	Dependencies      []string          `json:"dependencies,omitempty"`
	Notes             string            `json:"notes,omitempty"`
}

// Manifester is the optional interface an agent implements to advertise its
// manifest. Without this the registry falls back to a default Manifest built
// from ID/Name/RiskLevel.
type Manifester interface {
	AIBOM() Manifest
}

// ManifestOf returns the manifest for an agent. If the agent doesn't
// implement Manifester, a default is constructed.
func ManifestOf(a agent.Agent) Manifest {
	if m, ok := a.(Manifester); ok {
		return m.AIBOM()
	}
	return Manifest{
		ID:                a.ID(),
		Name:              a.Name(),
		Kind:              "agent",
		RiskClass:         agent.RiskOf(a),
		Capabilities:      a.Capabilities(),
		TrainingDataClass: TrainingUnknown,
	}
}

// Document is the top-level AIBOM payload.
type Document struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Components  []Manifest `json:"components"`
}

// Builder accumulates manifests from agents + extra components (LLM
// providers, embedders, etc.) and renders the final Document.
type Builder struct {
	mu     sync.Mutex
	extras []Manifest
}

// NewBuilder constructs an empty builder.
func NewBuilder() *Builder { return &Builder{} }

// Add registers an extra (non-agent) manifest.
func (b *Builder) Add(m Manifest) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.extras = append(b.extras, m)
}

// Render returns the AIBOM document for the supplied agents plus all extras.
// Sorted by ID for stable diffs across runs.
func (b *Builder) Render(agents []agent.Agent) Document {
	out := make([]Manifest, 0, len(agents))
	for _, a := range agents {
		out = append(out, ManifestOf(a))
	}
	b.mu.Lock()
	out = append(out, b.extras...)
	b.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return Document{GeneratedAt: time.Now().UTC(), Components: out}
}
