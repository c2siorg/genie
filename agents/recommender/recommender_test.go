package recommender

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

func TestRecommender_NegativeNetTriggersWarning(t *testing.T) {
	body, _ := json.Marshal(analyzerView{
		NetCents: -100000,
		Currency: "INR",
		ByCategory: []CategoryTotal{
			{Category: "food:delivery", AmountCents: 80000, Count: 5},
		},
	})
	msg := agent.NewMessage("analyzer", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var res Result
	_ = json.Unmarshal([]byte(out[0].Content), &res)
	if len(res.Recommendations) < 2 {
		t.Fatalf("want >=2 recs, got %d", len(res.Recommendations))
	}
	if res.Recommendations[0].Action != "review_largest_categories" {
		t.Errorf("first action: %q", res.Recommendations[0].Action)
	}
}
