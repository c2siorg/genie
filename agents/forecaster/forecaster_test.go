package forecaster

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

func TestForecaster_Horizons(t *testing.T) {
	body, _ := json.Marshal(analyzerView{
		Currency: "INR",
		NetCents: 100000,
	})
	msg := agent.NewMessage("analyzer", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1, got %d", len(out))
	}
	var f Forecast
	if err := json.Unmarshal([]byte(out[0].Content), &f); err != nil {
		t.Fatal(err)
	}
	if f.Horizon6mCents != 600000 {
		t.Errorf("6m: %d", f.Horizon6mCents)
	}
}
