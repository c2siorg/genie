package supervisor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestSupervisor_FinalizesAfterAllFanOuts(t *testing.T) {
	a := New()
	md := map[string]any{"trace_id": "tr-1", "csv": "date,description,amount,type\n2026-01-01,x,1,credit"}

	// 1) question kicks off the pipeline.
	out, err := a.HandleMessage(context.Background(), agent.NewMessage("user", ID, agent.RoleUser, TypeQuestion, "Where am I overspending?", md), testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].To != TargetIngestor {
		t.Fatalf("expected ingestor kick-off, got %+v", out)
	}

	// 2) feed analysis + the three fan-out results in any order.
	analysisBody, _ := json.Marshal(map[string]any{
		"currency":            "INR",
		"total_income_cents":  100,
		"total_expense_cents": 50,
		"net_cents":           50,
		"top_overspend":       []string{"food:delivery"},
	})
	steps := []agent.Message{
		agent.NewMessage("analyzer", ID, agent.RoleAgent, "analysis_result", string(analysisBody), md),
		agent.NewMessage("forecaster", ID, agent.RoleAgent, TypeForecast, `{"horizon_6m_cents": 600}`, md),
		agent.NewMessage("anomaly_detector", ID, agent.RoleAgent, TypeAnomalies, `{"anomalies":[]}`, md),
	}
	for _, m := range steps {
		out, err = a.HandleMessage(context.Background(), m, testEnv{})
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 0 {
			t.Fatalf("expected no outputs yet, got %+v", out)
		}
	}

	// 3) the final fan-out triggers the report.
	out, err = a.HandleMessage(context.Background(), agent.NewMessage("recommender", ID, agent.RoleAgent, TypeRecommendations, `{"recommendations":[]}`, md), testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].To != TargetReporter {
		t.Fatalf("expected reporter dispatch, got %+v", out)
	}
}
