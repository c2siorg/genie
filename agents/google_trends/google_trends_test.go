package google_trends

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

type stubFetcher struct{ series []Series }

func (s stubFetcher) Fetch(_ context.Context, _ string, _ []string) ([]Series, error) {
	return s.series, nil
}

func TestClassifySurging(t *testing.T) {
	s := Classify(Series{Keyword: "ev loan", Points: []int{10, 10, 10, 30, 30, 30}}, 3)
	if s.Direction != "surging" {
		t.Errorf("3× baseline should be surging; got %s", s.Direction)
	}
}

func TestClassifyFading(t *testing.T) {
	s := Classify(Series{Keyword: "metaverse", Points: []int{80, 80, 80, 20, 20, 20}}, 3)
	if s.Direction != "fading" {
		t.Errorf("0.25× baseline should be fading; got %s", s.Direction)
	}
}

func TestClassifySteady(t *testing.T) {
	s := Classify(Series{Keyword: "hdfc", Points: []int{50, 50, 50, 50, 50, 50}}, 3)
	if s.Direction != "steady" {
		t.Errorf("flat series should be steady; got %s", s.Direction)
	}
}

func TestClassifyShortSeriesIsSteady(t *testing.T) {
	s := Classify(Series{Keyword: "x", Points: []int{10}}, 3)
	if s.Direction != "steady" {
		t.Errorf("too-short series should be steady; got %s", s.Direction)
	}
}

func TestAnalyseProducesHints(t *testing.T) {
	a := New(stubFetcher{series: []Series{
		{Keyword: "ev loan", Points: []int{10, 10, 10, 30, 30, 30}},
		{Keyword: "metaverse", Points: []int{80, 80, 80, 20, 20, 20}},
	}})
	resp, err := a.Analyse(context.Background(), Request{Geo: "IN", WindowN: 3})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(resp.Hints, " | ")
	if !strings.Contains(joined, "ev loan") || !strings.Contains(joined, "metaverse") {
		t.Errorf("expected hints for both signals; got %+v", resp.Hints)
	}
	// Surging should sort above fading because change_multiple is higher.
	if resp.Signals[0].Keyword != "ev loan" {
		t.Errorf("expected ev loan as top signal; got %s", resp.Signals[0].Keyword)
	}
}

func TestAnalyseNoFetcher(t *testing.T) {
	_, err := (&Agent{}).Analyse(context.Background(), Request{})
	if err == nil {
		t.Errorf("expected error without fetcher")
	}
}

func TestHandleMessage(t *testing.T) {
	a := New(stubFetcher{series: []Series{{Keyword: "x", Points: []int{1, 1, 1, 1}}}})
	body, _ := json.Marshal(Request{Geo: "IN", WindowN: 2})
	msg := agent.NewMessage("s", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimer(t *testing.T) {
	a := New(stubFetcher{})
	resp, _ := a.Analyse(context.Background(), Request{Geo: "IN"})
	if resp.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}
