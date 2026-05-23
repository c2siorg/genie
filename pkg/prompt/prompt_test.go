package prompt

import (
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

func TestPrompt_RenderWithFewShot(t *testing.T) {
	p := &Prompt{
		ID: "recommender", Version: "1.0.0",
		System:       "Recommend with rationale.",
		UserTemplate: "Consider category {{.category}} with spend {{.amount}}.",
		Examples: []Example{
			{User: "food spending high", Assistant: "Cut takeout by 20%."},
		},
	}
	msgs, err := p.Render(map[string]any{"category": "food", "amount": 50000})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (sys + ex user + ex assistant + user), got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Fatalf("first message should be system")
	}
	if !strings.Contains(msgs[3].Content, "food") {
		t.Fatalf("user vars not interpolated: %s", msgs[3].Content)
	}
}

func TestRegistry_LatestByLexicographic(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Prompt{ID: "p", Version: "1.0.0", System: "a", UserTemplate: "{{.x}}"})
	_ = r.Register(&Prompt{ID: "p", Version: "1.1.0", System: "b", UserTemplate: "{{.x}}"})
	got, err := r.Get("p", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.1.0" {
		t.Fatalf("latest: %q", got.Version)
	}
}
