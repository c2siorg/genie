package reporter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestReporter_RendersAllSections(t *testing.T) {
	body, _ := json.Marshal(Bundle{
		Question:          "Where am I overspending?",
		Currency:          "INR",
		TotalIncomeCents:  5000000,
		TotalExpenseCents: 2575000,
		NetCents:          2425000,
		TopOverspend:      []string{"housing:rent", "food:delivery"},
		Forecast:          json.RawMessage(`{"horizon_6m_cents":14550000}`),
		Anomalies:         json.RawMessage(`{"anomalies":[]}`),
		Recommendations:   json.RawMessage(`{"recommendations":[]}`),
	})
	msg := agent.NewMessage("supervisor", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, _ := New().HandleMessage(context.Background(), msg, testEnv{})
	if !strings.Contains(out[0].Content, "Top categories") {
		t.Fatalf("expected sections, got: %s", out[0].Content)
	}
}
