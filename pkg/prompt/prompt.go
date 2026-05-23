// Package prompt is Genie's prompt registry. Every LLM call should reference
// a versioned prompt id so production runs can be replayed exactly.
//
// Templates are Go's text/template. Variables come in as map[string]any.
// Few-shot examples are part of the prompt body — the renderer injects them
// before the user message.
package prompt

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/schema"
)

// Example is one few-shot example pair.
type Example struct {
	User      string `yaml:"user" json:"user"`
	Assistant string `yaml:"assistant" json:"assistant"`
}

// Prompt is one versioned prompt entry.
type Prompt struct {
	ID           string         `yaml:"id" json:"id"`
	Version      string         `yaml:"version" json:"version"`
	Description  string         `yaml:"description,omitempty" json:"description,omitempty"`
	System       string         `yaml:"system" json:"system"`
	UserTemplate string         `yaml:"user_template" json:"user_template"`
	Examples     []Example      `yaml:"examples,omitempty" json:"examples,omitempty"`
	OutputSchema *schema.Schema `yaml:"-" json:"-"`
}

// Render produces an []llm.Message ready for Provider.Complete.
func (p *Prompt) Render(vars map[string]any) ([]llm.Message, error) {
	if p.System == "" && p.UserTemplate == "" {
		return nil, errors.New("prompt: empty system and user template")
	}
	user, err := renderTemplate(p.UserTemplate, vars)
	if err != nil {
		return nil, err
	}
	out := make([]llm.Message, 0, 2+2*len(p.Examples))
	if p.System != "" {
		out = append(out, llm.Message{Role: llm.RoleSystem, Content: p.System})
	}
	for _, ex := range p.Examples {
		out = append(out,
			llm.Message{Role: llm.RoleUser, Content: ex.User},
			llm.Message{Role: llm.RoleAssistant, Content: ex.Assistant},
		)
	}
	out = append(out, llm.Message{Role: llm.RoleUser, Content: user})
	return out, nil
}

// ValidateOutput checks the completion against the prompt's OutputSchema.
// No-op if the prompt has no schema attached.
func (p *Prompt) ValidateOutput(text string) error {
	if p.OutputSchema == nil {
		return nil
	}
	return p.OutputSchema.ValidateJSON([]byte(text))
}

func renderTemplate(tmpl string, vars map[string]any) (string, error) {
	if tmpl == "" {
		return "", nil
	}
	t, err := template.New("p").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse: %w", err)
	}
	var sb bytes.Buffer
	if err := t.Execute(&sb, vars); err != nil {
		return "", fmt.Errorf("template execute: %w", err)
	}
	return strings.TrimSpace(sb.String()), nil
}

// Registry is the process-wide collection of named, versioned prompts.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]map[string]*Prompt // id -> version -> Prompt
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{entries: map[string]map[string]*Prompt{}}
}

// Register adds or replaces a prompt version.
func (r *Registry) Register(p *Prompt) error {
	if p == nil || p.ID == "" || p.Version == "" {
		return errors.New("prompt registry: id and version required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries[p.ID] == nil {
		r.entries[p.ID] = map[string]*Prompt{}
	}
	r.entries[p.ID][p.Version] = p
	return nil
}

// Get returns the prompt for (id, version). version="" returns the latest by
// lexicographic order — fine for semver-ish strings.
func (r *Registry) Get(id, version string) (*Prompt, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.entries[id]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("prompt %q: not found", id)
	}
	if version != "" {
		if p, ok := versions[version]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("prompt %q@%q: not found", id, version)
	}
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return versions[keys[len(keys)-1]], nil
}

// List returns the (id, latest-version) pairs for every prompt.
func (r *Registry) List() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := map[string]string{}
	for id, versions := range r.entries {
		keys := make([]string, 0, len(versions))
		for k := range versions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out[id] = keys[len(keys)-1]
	}
	return out
}
