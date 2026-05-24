// Package google_trends surfaces consumer-search-interest signals that
// feed macro_research (e.g. surging interest in "EV loans" → flag for the
// macro brief) and mf_screener (e.g. surging interest in a fund family
// → flag for the screener).
//
// Inspired by Google ADK samples → google-trends-agent. The agent is
// transport-agnostic: it takes a TrendFetcher implementation (host wires
// the real google-trends client or a BigQuery dataset query) and returns
// a normalised TrendSignal that downstream agents consume.
package google_trends

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "google_trends"
	Capability = "search_trend_signal"
	TypeIn     = "trend_request"
	TypeOut    = "trend_signal"
	NextAgent  = "financial_supervisor"

	// "Surging" — interest in the latest window is ≥ this multiple of the prior window's mean.
	surgeMultiple = 1.5
	// "Fading" — latest window interest ≤ this multiple of prior mean.
	fadeMultiple = 0.6
)

// Series is one keyword's historical interest values, oldest first.
// Interest is the Google Trends 0..100 normalised score.
type Series struct {
	Keyword string `json:"keyword"`
	Geo     string `json:"geo"`     // "IN", "IN-MH", etc.
	Points  []int  `json:"points"`  // 0..100, oldest first
}

// Request asks for an analysis across a set of keyword series.
type Request struct {
	Geo      string   `json:"geo"`
	Keywords []string `json:"keywords"`
	WindowN  int      `json:"window_points"` // "latest" window size; rest is baseline
}

// Signal is one keyword's classified trend.
type Signal struct {
	Keyword           string  `json:"keyword"`
	Direction         string  `json:"direction"` // "surging" | "fading" | "steady"
	LatestMean        float64 `json:"latest_mean"`
	BaselineMean      float64 `json:"baseline_mean"`
	ChangeMultiple    float64 `json:"change_multiple"`
	NoteToDownstream  string  `json:"note_to_downstream"`
}

// Response is the structured output.
type Response struct {
	Geo        string   `json:"geo"`
	Signals    []Signal `json:"signals"`
	Hints      []string `json:"downstream_hints"`
	Disclaimer string   `json:"disclaimer"`
}

// TrendFetcher is the pluggable adapter. Production wires the real
// Google Trends API; tests use a stub.
type TrendFetcher interface {
	Fetch(ctx context.Context, geo string, keywords []string) ([]Series, error)
}

type Agent struct {
	Fetcher TrendFetcher
}

func New(fetcher TrendFetcher) *Agent { return &Agent{Fetcher: fetcher} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Search-Trend Signal" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskLow }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	resp, err := a.Analyse(ctx, req)
	if err != nil {
		return nil, err
	}
	env.Logf("[google_trends] geo=%s signals=%d", resp.Geo, len(resp.Signals))
	body, _ := json.Marshal(resp)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Analyse fetches series + classifies each.
func (a *Agent) Analyse(ctx context.Context, req Request) (Response, error) {
	if a.Fetcher == nil {
		return Response{}, &noFetcherErr{}
	}
	series, err := a.Fetcher.Fetch(ctx, req.Geo, req.Keywords)
	if err != nil {
		return Response{}, err
	}
	window := req.WindowN
	if window <= 0 {
		window = 4
	}

	signals := []Signal{}
	for _, s := range series {
		sig := Classify(s, window)
		signals = append(signals, sig)
	}
	sort.Slice(signals, func(i, j int) bool { return signals[i].ChangeMultiple > signals[j].ChangeMultiple })

	hints := []string{}
	for _, s := range signals {
		switch s.Direction {
		case "surging":
			hints = append(hints, "macro_research: flag surge — "+s.Keyword)
			hints = append(hints, "mf_screener: rescore theme — "+s.Keyword)
		case "fading":
			hints = append(hints, "macro_research: flag fade — "+s.Keyword)
		}
	}

	return Response{
		Geo:     req.Geo,
		Signals: signals,
		Hints:   hints,
		Disclaimer: "Google-Trends-derived interest signal. Not investment advice; correlation " +
			"with actual market moves is variable.",
	}, nil
}

// Classify is pure; useful for unit tests of the math.
func Classify(s Series, windowN int) Signal {
	if windowN <= 0 || len(s.Points) <= windowN {
		return Signal{Keyword: s.Keyword, Direction: "steady"}
	}
	split := len(s.Points) - windowN
	baseline := s.Points[:split]
	latest := s.Points[split:]
	baseMean := meanInt(baseline)
	latestMean := meanInt(latest)
	mult := 0.0
	if baseMean > 0 {
		mult = latestMean / baseMean
	}
	dir := "steady"
	switch {
	case mult >= surgeMultiple:
		dir = "surging"
	case mult > 0 && mult <= fadeMultiple:
		dir = "fading"
	}
	note := ""
	switch dir {
	case "surging":
		note = "Search interest in this term has surged versus baseline; investigate macro driver"
	case "fading":
		note = "Search interest has cooled materially versus baseline"
	}
	return Signal{
		Keyword:          s.Keyword,
		Direction:        dir,
		LatestMean:       round2(latestMean),
		BaselineMean:     round2(baseMean),
		ChangeMultiple:   round2(mult),
		NoteToDownstream: note,
	}
}

func meanInt(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

type noFetcherErr struct{}

func (noFetcherErr) Error() string { return "google_trends: no fetcher configured" }
