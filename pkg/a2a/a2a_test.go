package a2a

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestA2A_GetCardAndSubmit(t *testing.T) {
	srv := NewServer(AgentCard{
		Name: "demo",
		URL:  "http://example/",
		Skills: []Skill{{
			ID:          "echo",
			Description: "echo input",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	srv.Handle("echo", func(_ context.Context, t Task) (TaskResult, error) {
		v, _ := t.Input["text"].(string)
		return TaskResult{Output: "echo: " + v}, nil
	})
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	c := NewClient(httpSrv.URL)
	card, err := c.GetAgentCard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if card.Name != "demo" || len(card.Skills) != 1 {
		t.Fatalf("unexpected card: %+v", card)
	}

	res, err := c.SubmitTask(context.Background(), Task{ID: "t1", SkillID: "echo", Input: map[string]any{"text": "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "echo: hi" || res.Status != "completed" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestA2A_UnknownSkill(t *testing.T) {
	srv := NewServer(AgentCard{Name: "x"})
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	c := NewClient(httpSrv.URL)
	_, err := c.SubmitTask(context.Background(), Task{ID: "t1", SkillID: "nope"})
	if err == nil {
		t.Fatal("expected unknown-skill error")
	}
}
