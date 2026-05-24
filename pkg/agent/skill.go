// skill.go — SkillToolset / progressive disclosure pattern.
//
// Borrowed from Google ADK samples → agent-skills-tutorial. The problem:
// a supervisor that holds 40 sub-agents and 100 tools surfaces all of
// them to the LLM in every prompt — context bloat, slower routing, worse
// decisions.
//
// The fix: agents declare *Skills*, each with a short description and a
// lazy tool manifest. The supervisor lists the skill *titles* in the
// initial prompt; only when the LLM selects a skill does the toolset for
// that skill get materialised and surfaced.
//
// This file adds the data model + a small registry. Existing agents can
// opt into the pattern by implementing SkillProvider — non-implementers
// keep working unchanged.
package agent

import (
	"context"
	"errors"
	"sort"
	"sync"
)

// Skill is a coarse-grained capability with a lazy toolset.
//
//   - ID:        machine-routable id (e.g. "tax_planning_in")
//   - Title:     short human label shown in the supervisor prompt
//   - Summary:   one-sentence description; <= 200 chars
//   - Tools:     materialised only when the skill is invoked
type Skill struct {
	ID      string
	Title   string
	Summary string
	Tools   func(ctx context.Context) []SkillTool
}

// SkillTool is a tool exposed by a Skill. Mirrors the shape of the tool
// catalogue agents already use, but scoped to its parent skill.
type SkillTool struct {
	Name        string
	Description string
	Run         func(ctx context.Context, input string) (string, error)
}

// SkillProvider is the optional interface an Agent advertises when it
// wants its capabilities consumed progressively.
type SkillProvider interface {
	Skills() []Skill
}

// SkillsOf returns the agent's skills, or nil if the agent does not
// implement SkillProvider. This lets callers stay generic.
func SkillsOf(a Agent) []Skill {
	if sp, ok := a.(SkillProvider); ok {
		return sp.Skills()
	}
	return nil
}

// SkillRegistry collects skills across multiple agents so a supervisor
// can present them in one ranked list. Thread-safe.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]Skill
	owners map[string]string // skillID -> agentID
}

// NewSkillRegistry constructs an empty registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: map[string]Skill{},
		owners: map[string]string{},
	}
}

// RegisterAgent walks the agent's skills and registers each one.
// Returns the number of skills registered. Returns an error if the agent
// declared a skill ID that collides with an existing owner.
func (r *SkillRegistry) RegisterAgent(a Agent) (int, error) {
	skills := SkillsOf(a)
	if len(skills) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range skills {
		if s.ID == "" {
			return 0, errors.New("skill missing id from agent " + a.ID())
		}
		if owner, exists := r.owners[s.ID]; exists && owner != a.ID() {
			return 0, errors.New("skill collision: " + s.ID + " owned by " + owner)
		}
		r.skills[s.ID] = s
		r.owners[s.ID] = a.ID()
	}
	return len(skills), nil
}

// List returns every registered skill sorted by Title (stable for prompts).
func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// Manifest returns the lazy summary line per skill — the "progressive
// disclosure" payload that goes into the supervisor's initial prompt.
// Each line is `ID — Title — Summary`.
func (r *SkillRegistry) Manifest() []string {
	skills := r.List()
	out := make([]string, 0, len(skills))
	for _, s := range skills {
		out = append(out, s.ID+" — "+s.Title+" — "+s.Summary)
	}
	return out
}

// Invoke materialises the skill's tools and returns them. The supervisor
// then surfaces those tools to the LLM only for this turn.
func (r *SkillRegistry) Invoke(ctx context.Context, skillID string) ([]SkillTool, error) {
	r.mu.RLock()
	s, ok := r.skills[skillID]
	r.mu.RUnlock()
	if !ok {
		return nil, errors.New("unknown skill: " + skillID)
	}
	if s.Tools == nil {
		return nil, nil
	}
	return s.Tools(ctx), nil
}

// OwnerOf returns the agentID that registered a given skill.
func (r *SkillRegistry) OwnerOf(skillID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.owners[skillID]
	return o, ok
}
