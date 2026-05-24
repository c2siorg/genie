// Package deep_research is a multi-turn ReAct research agent for the
// Indian banking corpus — RBI circulars, Sahamati AA specs, FATF and
// FIU-IND guidance, macro releases. Where macro_research is one-shot,
// deep_research iterates: search → synthesise → re-search → cite.
//
// Inspired by Google ADK samples → deep-search.
//
// Architecture:
//   - The agent wraps pkg/reasoning.ReAct with a fixed toolbelt
//     (corpus_search, rbi_circular_fetch, sahamati_spec_lookup).
//   - The tool implementations are pluggable Resolvers so the same
//     agent works offline (deterministic stub) or online (real fetchers).
//   - Returns a Brief — a cited synthesis with reasoning trace.
package deep_research

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/reasoning"
)

const (
	ID         = "deep_research"
	Capability = "deep_research"
	TypeIn     = "research_request"
	TypeOut    = "research_brief"
	NextAgent  = "financial_supervisor"
)

// Request is one research query.
type Request struct {
	Question string   `json:"question"`
	Sources  []string `json:"sources,omitempty"` // optional corpus filter
	MaxSteps int      `json:"max_steps,omitempty"`
}

// Citation pins a claim to a source.
type Citation struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Quote string `json:"quote,omitempty"`
}

// Brief is the structured output.
type Brief struct {
	Question   string     `json:"question"`
	Summary    string     `json:"summary"`
	Citations  []Citation `json:"citations"`
	Steps      int        `json:"steps"`
	Mode       string     `json:"mode"` // "react" | "offline"
	Disclaimer string     `json:"disclaimer"`
}

// Resolver is the pluggable interface a host wires up.
// Returns a short factual snippet for the given query against the named
// corpus. Implementations: in-memory stub, RBI circular fetcher, web search.
type Resolver interface {
	Resolve(ctx context.Context, corpus, query string) (snippet string, source Citation, err error)
}

// StubResolver is the deterministic offline resolver — useful for tests,
// the sandbox, and as a safe fallback when the LLM is unavailable.
type StubResolver struct{}

func (StubResolver) Resolve(_ context.Context, corpus, query string) (string, Citation, error) {
	switch corpus {
	case "rbi":
		return "RBI circulars are indexed under master directions per topic.",
			Citation{Title: "RBI master directions", URL: "https://rbi.org.in/Scripts/BS_ViewMasDirections.aspx"}, nil
	case "sahamati":
		return "Sahamati Account Aggregator framework defines FIP/FIU/AA roles.",
			Citation{Title: "Sahamati AA specs", URL: "https://sahamati.org.in/"}, nil
	case "fiu_ind":
		return "FIU-IND requires STR within 7 days, CTR for cash ≥₹10L, CCR within 7 days.",
			Citation{Title: "FIU-IND reporting", URL: "https://fiuindia.gov.in/"}, nil
	default:
		return fmt.Sprintf("No snippet found for %q in %q.", query, corpus),
			Citation{Title: "no_source"}, nil
	}
}

// Agent is the deep-research worker.
type Agent struct {
	Provider llm.Provider // optional — if nil, falls back to offline mode
	Model    string
	Resolver Resolver
}

// New constructs the agent with the supplied LLM and resolver. Pass
// llm=nil and Resolver=StubResolver{} for the offline path used in
// tests and the in-process sandbox.
func New(provider llm.Provider, model string, resolver Resolver) *Agent {
	if resolver == nil {
		resolver = StubResolver{}
	}
	return &Agent{Provider: provider, Model: model, Resolver: resolver}
}

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Deep Research" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	b := a.Research(ctx, req)
	env.Logf("[deep_research] q=%q mode=%s steps=%d cites=%d", req.Question, b.Mode, b.Steps, len(b.Citations))
	body, _ := json.Marshal(b)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Research is the entry point. Falls back to offline mode if Provider is nil.
func (a *Agent) Research(ctx context.Context, req Request) Brief {
	if a.Provider == nil {
		return a.offlineBrief(ctx, req)
	}
	maxSteps := req.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 4
	}
	tools := a.buildTools(ctx)
	system := "You are a precise financial researcher. Use the available tools to gather " +
		"facts before answering. Cite the source corpus for every claim."
	res, err := reasoning.ReAct(ctx, a.Provider, a.Model, system, req.Question, tools, maxSteps)
	if err != nil {
		// graceful degradation — fall back to offline.
		return a.offlineBrief(ctx, req)
	}
	return Brief{
		Question:  req.Question,
		Summary:   res.Answer,
		Citations: a.scrapeCitations(res),
		Steps:     len(res.Steps),
		Mode:      "react",
		Disclaimer: "Synthesised by an AI research loop. Verify all citations and quotations " +
			"against the original RBI/Sahamati/FIU-IND publications before relying on them.",
	}
}

func (a *Agent) offlineBrief(ctx context.Context, req Request) Brief {
	corpora := req.Sources
	if len(corpora) == 0 {
		corpora = []string{"rbi", "sahamati", "fiu_ind"}
	}
	parts := []string{}
	cites := []Citation{}
	for _, c := range corpora {
		snippet, src, err := a.Resolver.Resolve(ctx, c, req.Question)
		if err != nil {
			continue
		}
		parts = append(parts, "["+c+"] "+snippet)
		cites = append(cites, src)
	}
	return Brief{
		Question:  req.Question,
		Summary:   strings.Join(parts, "\n"),
		Citations: cites,
		Steps:     len(cites),
		Mode:      "offline",
		Disclaimer: "Offline deterministic synthesis (no LLM). Suitable for sandbox and CI; " +
			"for production research enable an LLM provider.",
	}
}

func (a *Agent) buildTools(ctx context.Context) []reasoning.Tool {
	corpora := []string{"rbi", "sahamati", "fiu_ind"}
	tools := make([]reasoning.Tool, 0, len(corpora))
	for _, c := range corpora {
		c := c
		tools = append(tools, reasoning.Tool{
			Name: "search_" + c,
			Run: func(ctx context.Context, input string) (string, error) {
				snippet, src, err := a.Resolver.Resolve(ctx, c, input)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s (source: %s)", snippet, src.Title), nil
			},
		})
	}
	return tools
}

// scrapeCitations is a heuristic that pulls source labels out of the ReAct
// trace's tool-observation steps so the final brief carries citations even
// when the LLM doesn't format them strictly.
func (a *Agent) scrapeCitations(res reasoning.ReActResult) []Citation {
	var cites []Citation
	for _, s := range res.Steps {
		if s.Observation == "" {
			continue
		}
		if idx := strings.Index(s.Observation, "source: "); idx >= 0 {
			title := strings.TrimSpace(s.Observation[idx+len("source: "):])
			title = strings.TrimSuffix(title, ")")
			cites = append(cites, Citation{Title: title})
		}
	}
	return cites
}
