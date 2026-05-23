package reasoning

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// ReflexionTrace is the result of a Reflexion loop: the initial answer, the
// critique, and the refined answer. Persisting traces lets the agent
// "remember" its past failure modes — verbal RL without weight updates.
type ReflexionTrace struct {
	Initial   string
	Critique  string
	Refined   string
	Improved  bool
}

// Reflexion runs Initial -> Critique -> Refined in three LLM calls. Self-
// memory pulled from MemoryFetch is prepended to the initial prompt so the
// agent doesn't repeat its prior mistakes.
//
// MemoryFetch returns lines of past critiques to prepend; nil is fine.
func Reflexion(ctx context.Context, p llm.Provider, model, system, user string, memoryFetch func() []string) (ReflexionTrace, error) {
	var memNote string
	if memoryFetch != nil {
		mem := memoryFetch()
		if len(mem) > 0 {
			memNote = "Past critiques to avoid repeating:\n- " + strings.Join(mem, "\n- ") + "\n\n"
		}
	}

	// Step 1: initial answer.
	initialResp, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system + "\n\n" + memNote},
			{Role: llm.RoleUser, Content: user},
		},
		MaxTokens: 512,
	})
	if err != nil {
		return ReflexionTrace{}, err
	}
	trace := ReflexionTrace{Initial: strings.TrimSpace(initialResp.Text)}

	// Step 2: critique.
	critiqueResp, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Critique the assistant's answer for accuracy, completeness, and alignment with the system prompt. Be specific. One paragraph."},
			{Role: llm.RoleUser, Content: "System: " + system + "\n\nUser: " + user + "\n\nAnswer: " + trace.Initial},
		},
		MaxTokens: 256,
	})
	if err != nil {
		return trace, err
	}
	trace.Critique = strings.TrimSpace(critiqueResp.Text)

	// Step 3: refine.
	refineResp, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system},
			{Role: llm.RoleUser, Content: user},
			{Role: llm.RoleAssistant, Content: trace.Initial},
			{Role: llm.RoleUser, Content: "Critique: " + trace.Critique + "\n\nProduce a refined, corrected answer."},
		},
		MaxTokens: 512,
	})
	if err != nil {
		return trace, err
	}
	trace.Refined = strings.TrimSpace(refineResp.Text)
	trace.Improved = trace.Refined != trace.Initial
	return trace, nil
}
