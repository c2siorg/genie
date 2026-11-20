package afg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/microsoft/agent-framework-go/agent"
)

// ProviderKind labels how an agent is backed. It feeds the AI inventory.
type ProviderKind string

const (
	ProviderDeterministic ProviderKind = "deterministic"
	ProviderOllama        ProviderKind = "ollama"
)

// AgentInfo is one row of the live capability inventory (the framework port of
// Genie's /v1/ai-inventory + AIBOM source of truth). Governed is always true
// because every registered agent was built through the single door.
type AgentInfo struct {
	ID          string       `json:"id"`
	Provider    ProviderKind `json:"provider"`
	Risk        string       `json:"risk"` // low|medium|high
	Governed    bool         `json:"governed"`
	HasFallback bool         `json:"has_fallback"`
}

// Registry is the live agent inventory + fallback wiring. It replaces the bus-era
// pkg/registry as the source of truth for what is actually served.
type Registry struct {
	mu        sync.RWMutex
	agents    map[string]*agent.Agent
	info      map[string]AgentInfo
	fallbacks map[string]*agent.Agent
}

func NewRegistry() *Registry {
	return &Registry{
		agents:    map[string]*agent.Agent{},
		info:      map[string]AgentInfo{},
		fallbacks: map[string]*agent.Agent{},
	}
}

// Register adds a governed agent under info.ID.
func (r *Registry) Register(info AgentInfo, a *agent.Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info.Governed = true // by construction: agents only exist via NewGoverned*
	r.agents[info.ID] = a
	r.info[info.ID] = info
}

// RegisterFallback wires a deterministic fallback for a primary agent (Genie's
// BCP seam, FREE-AI Rec 21). Only agents with a registered fallback are rescued.
func (r *Registry) RegisterFallback(primaryID string, fb *agent.Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallbacks[primaryID] = fb
	if info, ok := r.info[primaryID]; ok {
		info.HasFallback = true
		r.info[primaryID] = info
	}
}

// Get returns a registered agent.
func (r *Registry) Get(id string) (*agent.Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	return a, ok
}

// Inventory returns the AI inventory, sorted by ID (stable output for the API/AIBOM).
func (r *Registry) Inventory() []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AgentInfo, 0, len(r.info))
	for _, v := range r.info {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RunWithFallback runs the primary agent; on an EXECUTION error it routes to a
// registered fallback. A governance DENIAL (DeniedError) is a policy rejection and
// is NOT rescued — it returns as-is for incident recording.
func (r *Registry) RunWithFallback(ctx context.Context, id, input string) (text string, usedFallback bool, err error) {
	a, ok := r.Get(id)
	if !ok {
		return "", false, fmt.Errorf("unknown agent %q", id)
	}
	resp, runErr := a.RunText(ctx, input).Collect()
	if runErr == nil {
		return ResponseText(resp), false, nil
	}
	var denied *DeniedError
	if errors.As(runErr, &denied) {
		return "", false, runErr // policy rejection: do not fall back
	}
	r.mu.RLock()
	fb, hasFB := r.fallbacks[id]
	r.mu.RUnlock()
	if !hasFB {
		return "", false, runErr
	}
	fresp, ferr := fb.RunText(ctx, input).Collect()
	if ferr != nil {
		return "", true, fmt.Errorf("primary and fallback both failed: %w", ferr)
	}
	return ResponseText(fresp), true, nil
}
