package reasoning

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

func TestSplitCoT(t *testing.T) {
	chain, ans := SplitCoT("Reasoning: think think\nFinal Answer: 42")
	if !strings.Contains(chain, "think") {
		t.Errorf("chain: %q", chain)
	}
	if ans != "42" {
		t.Errorf("ans: %q", ans)
	}
}

func TestReAct_FinishesImmediately(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{
		{Text: "Thought: I already know.\nAction: finish\nAction Input: 42"},
	}
	res, err := ReAct(context.Background(), mock, "test", "be helpful", "what is the answer?", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "42" {
		t.Fatalf("answer: %q", res.Answer)
	}
}

func TestReAct_CallsTool(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{
		{Text: "Thought: I need the rate.\nAction: lookup_rate\nAction Input: INR"},
		{Text: "Thought: now I have it.\nAction: finish\nAction Input: 83"},
	}
	called := 0
	tools := []Tool{{
		Name: "lookup_rate",
		Description: "returns FX rate",
		Run: func(_ context.Context, input string) (string, error) {
			called++
			return "83.0", nil
		},
	}}
	res, err := ReAct(context.Background(), mock, "test", "rate finder", "rate?", tools, 3)
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("tool called %d times", called)
	}
	if res.Answer != "83" {
		t.Fatalf("answer: %q", res.Answer)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("steps: %d", len(res.Steps))
	}
}
