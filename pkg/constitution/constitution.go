// Package constitution loads the YAML "constitution" — the 7 Sutras of the
// RBI FREE-AI report rendered as a system prompt — and exposes helpers for
// LLM-driven agents to prepend it and for the auditor to score outputs
// against it.
//
// Constitutional AI in this codebase is intentionally minimal: the LLM sees
// the principles, and the auditor asks the LLM to score each output against
// them. No fine-tuning, no separate reward model.
package constitution

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"gopkg.in/yaml.v3"
)

// Sutra is one principle.
type Sutra struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	Rule  string `yaml:"rule"`
}

// Constitution holds the preamble and the principles.
type Constitution struct {
	Preamble string  `yaml:"preamble"`
	Sutras   []Sutra `yaml:"sutras"`
}

// Load reads + parses a YAML file from disk.
func Load(path string) (*Constitution, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("constitution: %w", err)
	}
	return Parse(body)
}

// Parse decodes YAML bytes.
func Parse(body []byte) (*Constitution, error) {
	var c Constitution
	if err := yaml.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("constitution parse: %w", err)
	}
	if len(c.Sutras) == 0 {
		return nil, fmt.Errorf("constitution: no sutras loaded")
	}
	return &c, nil
}

// SystemPrompt renders the constitution as a single string suitable for the
// "system" role of an llm.Message.
func (c *Constitution) SystemPrompt() string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(c.Preamble))
	sb.WriteString("\n\nPrinciples:\n")
	for i, s := range c.Sutras {
		fmt.Fprintf(&sb, "%d. %s — %s\n", i+1, s.Title, s.Rule)
	}
	return sb.String()
}

// SystemMessage builds an llm.Message with the constitutional preamble.
// Drop-in for LLM-driven agents at the start of any conversation.
func (c *Constitution) SystemMessage() llm.Message {
	return llm.Message{Role: llm.RoleSystem, Content: c.SystemPrompt()}
}

// Critique asks the LLM to judge a candidate output against the
// constitution. Returns the LLM's verdict + a numeric score 0..10.
//
// The output format is enforced by the prompt; the model returns:
//
//	SCORE: <int 0..10>
//	REASONING: <short>
//
// The parser is forgiving — any unparseable response yields score 0 and the
// raw text in Reasoning.
func (c *Constitution) Critique(ctx context.Context, p llm.Provider, modelHint, candidate string) (Verdict, error) {
	req := llm.CompletionRequest{
		Model: modelHint,
		Messages: []llm.Message{
			c.SystemMessage(),
			{Role: llm.RoleUser, Content: "Score the following output 0..10 against the principles. Reply with:\nSCORE: <int>\nREASONING: <one line>\n\n--- BEGIN OUTPUT ---\n" + candidate + "\n--- END OUTPUT ---"},
		},
		MaxTokens:   128,
		Temperature: 0,
	}
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return Verdict{}, err
	}
	return parseVerdict(resp.Text), nil
}

// Verdict is the structured output of Critique.
type Verdict struct {
	Score     int    `json:"score"`
	Reasoning string `json:"reasoning"`
}

func parseVerdict(text string) Verdict {
	var v Verdict
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "SCORE:") {
			var n int
			fmt.Sscanf(line, "SCORE: %d", &n)
			if n < 0 {
				n = 0
			}
			if n > 10 {
				n = 10
			}
			v.Score = n
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "REASONING:") {
			v.Reasoning = strings.TrimSpace(strings.TrimPrefix(line, "REASONING:"))
		}
	}
	if v.Reasoning == "" {
		v.Reasoning = strings.TrimSpace(text)
	}
	return v
}
