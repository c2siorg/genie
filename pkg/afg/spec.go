package afg

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"

	"github.com/microsoft/agent-framework-go/agent"
)

// Spec is a framework-free declaration of a governed agent, so specialist packages
// can be authored without importing the agent framework at all. A deterministic
// agent sets Handle (a pure input->output func); an advisory/LLM agent sets
// Instructions (+ optional Model). The governance gate is injected when the Spec is
// built, so the single-door invariant still holds.
type Spec struct {
	ID           string
	Provider     ProviderKind
	Risk         string // low|medium|high
	Handle       func(input string) (string, error)
	Instructions string
	Model        string
}

// Build turns a Spec into a governed *agent.Agent through the single door.
func (s Spec) Build(gate governance.Policy) *agent.Agent {
	if s.Handle != nil {
		return NewGovernedDeterministic(gate, s.ID, det(s.Handle))
	}
	model := s.Model
	if model == "" {
		model = "llama3.1"
	}
	return NewGovernedOllama(gate, s.ID, s.Instructions, model)
}

func (s Spec) info() AgentInfo {
	prov := s.Provider
	if prov == "" {
		if s.Handle != nil {
			prov = ProviderDeterministic
		} else {
			prov = ProviderOllama
		}
	}
	risk := s.Risk
	if risk == "" {
		risk = "low"
	}
	return AgentInfo{ID: s.ID, Provider: prov, Risk: risk}
}

// RegisterSpecs builds and registers a batch of specs under one governance gate.
func (r *Registry) RegisterSpecs(gate governance.Policy, specs ...Spec) {
	for _, s := range specs {
		r.Register(s.info(), s.Build(gate))
	}
}
