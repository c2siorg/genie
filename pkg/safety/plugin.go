// plugin.go — pluggable safety detector chain.
//
// The existing Detector interface is single-implementation per axis
// (jailbreak, topic, toxicity, bias). The pattern adopted here from Google
// ADK samples → safety-plugins is to let a host stack arbitrary detectors,
// including third-party "shields" (Model Armor, AWS Bedrock Guardrails,
// Lakera Guard), behind a single uniform plugin interface.
//
// Three things live here:
//
//  1. Plugin — Detector + a Name and Stage so the chain can be reasoned about.
//  2. Registry — a small map for hosts to register/replace plugins by name
//     (e.g. via the board-approved policy YAML).
//  3. Chain — a Detector that fans out across plugins and aggregates verdicts
//     into a single composite verdict.
//
// All of this is composition, not new behaviour — existing detectors keep
// working unchanged. The plugin interface just gives them a name and a
// stage so the chain can route messages sensibly.
package safety

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Stage labels when in the request lifecycle a plugin runs. A chain can
// filter plugins by stage to keep inbound vs outbound checks separate.
type Stage string

const (
	StageInbound  Stage = "inbound"  // user → system
	StageInternal Stage = "internal" // agent → agent
	StageOutbound Stage = "outbound" // system → user
	StageAny      Stage = "any"
)

// Plugin is a named, stage-aware safety detector.
type Plugin interface {
	Detector
	Name() string
	Stage() Stage
}

// NamedDetector wraps any Detector into a Plugin with a name and stage.
// Useful for adapting the heuristic detectors already in this package.
type NamedDetector struct {
	N string
	S Stage
	D Detector
}

func (n NamedDetector) Name() string                                              { return n.N }
func (n NamedDetector) Stage() Stage                                              { return n.S }
func (n NamedDetector) Inspect(ctx context.Context, text string) (Verdict, error) { return n.D.Inspect(ctx, text) }

// Registry holds plugins by name. Thread-safe; suitable for runtime updates
// when the policy YAML is reloaded.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{plugins: map[string]Plugin{}}
}

// Register adds or replaces a plugin by Name.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return errors.New("safety: nil plugin")
	}
	if p.Name() == "" {
		return errors.New("safety: plugin missing name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[p.Name()] = p
	return nil
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all plugins matching the stage. Pass StageAny to get every plugin.
func (r *Registry) List(stage Stage) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		if stage == StageAny || p.Stage() == StageAny || p.Stage() == stage {
			out = append(out, p)
		}
	}
	return out
}

// Chain is a Detector that runs multiple plugins and aggregates the worst
// verdict. Use Mode to choose:
//   - ModeFirstFlagged: stop at the first plugin that flags Flagged=true
//   - ModeWorstScore:   run all, return the highest-scored verdict
type Chain struct {
	Plugins []Plugin
	Mode    ChainMode
}

// ChainMode selects aggregation behaviour.
type ChainMode int

const (
	ModeFirstFlagged ChainMode = iota
	ModeWorstScore
)

// Inspect runs the chain against text and returns the aggregated verdict.
// The Reason field gets per-plugin attribution so the audit log can see
// which plugin produced which signal.
func (c Chain) Inspect(ctx context.Context, text string) (Verdict, error) {
	worst := Verdict{}
	notes := []string{}
	for _, p := range c.Plugins {
		v, err := p.Inspect(ctx, text)
		if err != nil {
			notes = append(notes, p.Name()+":error:"+err.Error())
			continue
		}
		notes = append(notes, formatPluginNote(p.Name(), v))
		if v.Flagged && c.Mode == ModeFirstFlagged {
			if v.Reason == "" {
				v.Reason = "blocked by " + p.Name()
			} else {
				v.Reason = p.Name() + ": " + v.Reason
			}
			return v, nil
		}
		if v.Score > worst.Score || (v.Flagged && !worst.Flagged) {
			worst = v
		}
	}
	if worst.Reason == "" && len(notes) > 0 {
		worst.Reason = strings.Join(notes, " | ")
	}
	return worst, nil
}

func formatPluginNote(name string, v Verdict) string {
	tag := "ok"
	if v.Flagged {
		tag = "BLOCK"
	}
	return name + ":" + tag
}

// HTTPShield is the template adapter for talking to an external safety
// service (Model Armor, Bedrock Guardrails, Lakera, etc.). The host wires
// up the Caller; this struct just shapes the verdict.
type HTTPShield struct {
	N      string
	S      Stage
	Caller func(ctx context.Context, text string) (Verdict, error)
}

func (h HTTPShield) Name() string                                                { return h.N }
func (h HTTPShield) Stage() Stage                                                { return h.S }
func (h HTTPShield) Inspect(ctx context.Context, text string) (Verdict, error)   {
	if h.Caller == nil {
		return Verdict{}, errors.New("safety: HTTPShield Caller not configured")
	}
	return h.Caller(ctx, text)
}
