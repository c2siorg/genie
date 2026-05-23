package anomaly

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestAnomaly_DetectsOutlier(t *testing.T) {
	txns := []finance.Transaction{
		{TransactionID: "a", Category: "food:delivery", AmountCents: -30000},
		{TransactionID: "b", Category: "food:delivery", AmountCents: -35000},
		{TransactionID: "c", Category: "food:delivery", AmountCents: -32000},
		{TransactionID: "d", Category: "food:delivery", AmountCents: -200000},
	}
	body, _ := json.Marshal(analyzerView{Transactions: txns})
	msg := agent.NewMessage("analyzer", ID, agent.RoleAgent, TypeIn, string(body), nil)
	a := New()
	// With only 4 points the z-score is bounded by sqrt(n-1) ≈ 1.73, so use a
	// looser threshold for the unit test. Production uses DefaultZThreshold.
	a.ZThreshold = 1.5
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var res Result
	_ = json.Unmarshal([]byte(out[0].Content), &res)
	if len(res.Anomalies) != 1 || res.Anomalies[0].TransactionID != "d" {
		t.Fatalf("want anomaly d, got %+v", res.Anomalies)
	}
}
