package moa_recommender

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func makeMock(text string) *llm.Mock {
	m := llm.NewMock()
	m.Responses = []llm.CompletionResponse{{Text: text}}
	return m
}

func TestMoA_PicksMajority(t *testing.T) {
	a := New(
		Panellist{Name: "p1", Provider: makeMock("cut food spend by 20%"), Model: "m"},
		Panellist{Name: "p2", Provider: makeMock("cut food spend by 20%"), Model: "m"},
		Panellist{Name: "p3", Provider: makeMock("eliminate netflix"), Model: "m"},
	)
	out, err := a.HandleMessage(context.Background(), agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "where to cut?", nil), testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var o Outcome
	_ = json.Unmarshal([]byte(out[0].Content), &o)
	if o.WinnerText != "cut food spend by 20%" {
		t.Fatalf("majority winner expected, got %q", o.WinnerText)
	}
}
