// Package reasoning is the helper layer for Chain-of-Thought, ReAct, and
// Reflection patterns. It sits between LLM-driven agents and pkg/llm.
//
// The patterns are intentionally lightweight — Genie keeps the prompt
// engineering in one place so refinements stay reviewable.
package reasoning

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// CoTPrompt wraps a user prompt in a Chain-of-Thought system instruction.
// The trailing "Final Answer:" anchor makes parsing deterministic.
func CoTPrompt(system, user string) []llm.Message {
	return []llm.Message{
		{Role: llm.RoleSystem, Content: strings.TrimSpace(system) + "\n\nThink step by step. End your response with a line beginning 'Final Answer:'."},
		{Role: llm.RoleUser, Content: user},
	}
}

// SplitCoT separates the chain of thought from the final answer. If no
// "Final Answer:" marker exists, the whole text is treated as the answer.
func SplitCoT(text string) (chain, answer string) {
	const marker = "Final Answer:"
	idx := strings.LastIndex(text, marker)
	if idx < 0 {
		return "", strings.TrimSpace(text)
	}
	return strings.TrimSpace(text[:idx]), strings.TrimSpace(text[idx+len(marker):])
}

// Reflect asks the model to critique an earlier output and produce a refined
// answer. Useful for the recommender or supervisor at the end of a run.
func Reflect(ctx context.Context, p llm.Provider, model, system, original, criticism string) (string, error) {
	resp, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system},
			{Role: llm.RoleAssistant, Content: original},
			{Role: llm.RoleUser, Content: "Reflect on the previous answer. Critique: " + criticism + "\nProduce a refined version."},
		},
		MaxTokens: 512,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}

// Tool is one action available to a ReAct agent.
type Tool struct {
	Name        string
	Description string
	Run         func(ctx context.Context, input string) (string, error)
}

// ReActResult holds the outcome of a ReAct loop.
type ReActResult struct {
	Steps  []ReActStep
	Answer string
}

// ReActStep mirrors the classic "Thought / Action / Observation" trio.
type ReActStep struct {
	Thought     string
	Action      string
	ActionInput string
	Observation string
}

// ReAct runs the Reason-Act loop until the model emits a Final Answer or
// maxSteps is reached. The prompt is the textbook ReAct format; production
// would replace the parser with a structured tool-calling provider.
func ReAct(ctx context.Context, p llm.Provider, model, system, user string, tools []Tool, maxSteps int) (ReActResult, error) {
	if maxSteps <= 0 {
		maxSteps = 5
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(system))
	sb.WriteString("\n\nYou may call tools. Always emit:\n")
	sb.WriteString("Thought: <one line>\nAction: <tool name or 'finish'>\nAction Input: <one line>\n\n")
	sb.WriteString("Tools available:\n")
	for _, t := range tools {
		fmt.Fprintf(&sb, "- %s: %s\n", t.Name, t.Description)
	}
	systemPrompt := sb.String()

	history := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: user},
	}
	var res ReActResult
	for i := 0; i < maxSteps; i++ {
		resp, err := p.Complete(ctx, llm.CompletionRequest{
			Model: model, Messages: history, MaxTokens: 256, Temperature: 0.2,
		})
		if err != nil {
			return res, err
		}
		step := parseReActStep(resp.Text)
		history = append(history, llm.Message{Role: llm.RoleAssistant, Content: resp.Text})
		if step.Action == "finish" || step.Action == "" {
			res.Steps = append(res.Steps, step)
			res.Answer = step.ActionInput
			return res, nil
		}
		// Dispatch.
		var obs string
		var found bool
		for _, t := range tools {
			if t.Name == step.Action {
				found = true
				out, err := t.Run(ctx, step.ActionInput)
				if err != nil {
					obs = "error: " + err.Error()
				} else {
					obs = out
				}
				break
			}
		}
		if !found {
			obs = "unknown tool: " + step.Action
		}
		step.Observation = obs
		res.Steps = append(res.Steps, step)
		history = append(history, llm.Message{Role: llm.RoleUser, Content: "Observation: " + obs})
	}
	return res, errors.New("react: max steps reached without final answer")
}

func parseReActStep(text string) ReActStep {
	var s ReActStep
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Thought:"):
			s.Thought = strings.TrimSpace(strings.TrimPrefix(line, "Thought:"))
		case strings.HasPrefix(line, "Action:"):
			s.Action = strings.TrimSpace(strings.TrimPrefix(line, "Action:"))
		case strings.HasPrefix(line, "Action Input:"):
			s.ActionInput = strings.TrimSpace(strings.TrimPrefix(line, "Action Input:"))
		}
	}
	return s
}
