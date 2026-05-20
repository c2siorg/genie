package registry

import (
	"context"
	"errors"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

// ErrNotFound is returned when an agent cannot be found.
//
// Registry lookups are a frequent operation in orchestrators/routers.
// Having a sentinel error allows callers to distinguish "not found" from
// infrastructure errors.
var ErrNotFound = errors.New("agent not found")

// Registry stores and exposes agents for discovery.
//
// Conceptually, this is the "agents registry" building block in the reference
// architecture. It is deliberately separate from the comm bus:
//
// - The registry answers: "Which agents exist and what can they do?"
// - The bus answers: "How do messages move between agents?"
//
// That separation makes it easier to swap either implementation.
type Registry interface {
	// Register makes an agent discoverable.
	//
	// Most systems register at startup, but dynamic registration is also common:
	// ephemeral agents, autoscaling workers, or user-scoped agents.
	Register(ctx context.Context, a agent.Agent) error

	// Get fetches an agent by ID (its routing address).
	Get(ctx context.Context, id string) (agent.Agent, error)

	// List returns all registered agents.
	List(ctx context.Context) []agent.Agent

	// FindByCapability returns all agents that claim a given capability.
	// Coordinators/routers can use this for capability-based dispatch.
	FindByCapability(ctx context.Context, capability string) []agent.Agent
}

// InMemoryRegistry is a simple thread-safe implementation of Registry.
//
// Concurrency model:
// - Uses a RWMutex so reads (Get/List) can proceed concurrently.
// - Register takes a write lock because it mutates the map.
//
// This is intentionally minimal: it favors clarity over features like TTLs,
// health checks, or distributed coordination.
type InMemoryRegistry struct {
	mu     sync.RWMutex
	agents map[string]agent.Agent
}

// NewInMemory creates a new in-memory registry.
//
// The returned registry is safe for concurrent use.
func NewInMemory() *InMemoryRegistry {
	return &InMemoryRegistry{
		agents: make(map[string]agent.Agent),
	}
}

// Register adds or replaces an agent in the registry.
//
// Replace behavior:
// If the same ID is registered again, the old implementation is replaced.
// This is useful in toy systems and can support hot-reload patterns in more
// advanced systems, but production registries usually have additional controls.
func (r *InMemoryRegistry) Register(_ context.Context, a agent.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a == nil || a.ID() == "" {
		return errors.New("invalid agent")
	}
	r.agents[a.ID()] = a
	return nil
}

// Get returns an agent by ID.
//
// ID is the agent's routing address; it is what appears in protocol.Message.To.
func (r *InMemoryRegistry) Get(_ context.Context, id string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

// List returns all agents.
//
// Note: The returned slice is a snapshot; it is safe for the caller to iterate
// without holding locks, but the agent values themselves are not copied.
func (r *InMemoryRegistry) List(_ context.Context) []agent.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]agent.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}

// FindByCapability returns agents that advertise the given capability.
//
// Capability strings are an intentionally lightweight mechanism. In production,
// you may want a richer schema (capability name + version + input/output types
// + cost/latency characteristics + security classification).
func (r *InMemoryRegistry) FindByCapability(ctx context.Context, capability string) []agent.Agent {
	all := r.List(ctx)
	out := make([]agent.Agent, 0, len(all))
	for _, a := range all {
		for _, c := range a.Capabilities() {
			if c == capability {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

var _ Registry = (*InMemoryRegistry)(nil)

