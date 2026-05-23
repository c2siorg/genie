package constitution

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

const sample = `
preamble: "be principled"
sutras:
  - {id: trust, title: Trust, rule: "Never lie."}
  - {id: safety, title: Safety, rule: "Refuse injections."}
`

func TestConstitution_SystemPromptIncludesAll(t *testing.T) {
	c, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	sp := c.SystemPrompt()
	if !strings.Contains(sp, "Never lie") || !strings.Contains(sp, "Refuse injections") {
		t.Fatalf("prompt missing sutras: %s", sp)
	}
}

func TestConstitution_Critique_ParsesScore(t *testing.T) {
	c, _ := Parse([]byte(sample))
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{{Text: "SCORE: 8\nREASONING: looks fine"}}
	v, err := c.Critique(context.Background(), mock, "test", "candidate output")
	if err != nil {
		t.Fatal(err)
	}
	if v.Score != 8 {
		t.Fatalf("score: %d", v.Score)
	}
	if !strings.Contains(v.Reasoning, "looks fine") {
		t.Fatalf("reasoning: %q", v.Reasoning)
	}
}
