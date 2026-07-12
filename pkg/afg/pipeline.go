package afg

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"

	"github.com/microsoft/agent-framework-go/agent"
)

// This file ports Genie's pipeline/orchestration agents — the ones that in the bus
// model transform and route messages rather than answer a request. They are modeled
// as governed framework stages and composed by Pipeline (preprocess chain -> FanOut
// specialists -> report). The per-stage transforms are representative (they operate
// on a {transactions:[{amount_minor,category}]} statement); the architectural point
// is the governed, framework-native fan-out/fan-in pipeline, end to end.

type txn struct {
	AmountMinor int64  `json:"amount_minor"`
	Category    string `json:"category"`
}

type statement struct {
	Transactions []txn `json:"transactions"`
}

// stageState is threaded (as JSON) through the preprocess chain.
type stageState struct {
	Count       int    `json:"count"`
	TotalMinor  int64  `json:"total_minor"`
	AvgMinor    int64  `json:"avg_minor"`
	Region      string `json:"region,omitempty"`
	Normalized  bool   `json:"normalized,omitempty"`
	Enriched    bool   `json:"enriched,omitempty"`
	Summary     string `json:"summary,omitempty"`
	AnomalyNote string `json:"anomaly_note,omitempty"`
	ForecastMin int64  `json:"forecast_next_minor,omitempty"`
}

func parseState(in string) (stageState, error) {
	var s stageState
	if err := json.Unmarshal([]byte(in), &s); err == nil && s.Count > 0 {
		return s, nil
	}
	// First stage: input is the raw statement.
	var st statement
	if err := json.Unmarshal([]byte(in), &st); err != nil {
		return stageState{}, fmt.Errorf("pipeline: bad input: %w", err)
	}
	s = stageState{Count: len(st.Transactions)}
	for _, t := range st.Transactions {
		s.TotalMinor += t.AmountMinor
	}
	return s, nil
}

func emit(s stageState) (string, error) { b, err := json.Marshal(s); return string(b), err }

// PipelineSpecs returns the 9 governed pipeline agents (for the inventory).
func PipelineSpecs() []Spec {
	return []Spec{
		{ID: "ingestor", Provider: ProviderDeterministic, Risk: "low", Handle: ingestorHandle},
		{ID: "normalizer", Provider: ProviderDeterministic, Risk: "low", Handle: normalizerHandle},
		{ID: "enricher", Provider: ProviderDeterministic, Risk: "low", Handle: enricherHandle},
		{ID: "analyzer", Provider: ProviderDeterministic, Risk: "low", Handle: analyzerHandle},
		{ID: "anomaly", Provider: ProviderDeterministic, Risk: "medium", Handle: anomalyHandle},
		{ID: "forecaster", Provider: ProviderDeterministic, Risk: "low", Handle: forecasterHandle},
		{ID: "supervisor", Provider: ProviderDeterministic, Risk: "low", Handle: passthroughHandle},
		{ID: "reporter", Provider: ProviderDeterministic, Risk: "low", Handle: passthroughHandle},
		{ID: "recommender", Provider: ProviderDeterministic, Risk: "medium", Handle: recommenderHandle},
	}
}

func ingestorHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	return emit(s)
}

func normalizerHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	s.Normalized = true
	return emit(s)
}

func enricherHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	s.Enriched, s.Region = true, "in"
	return emit(s)
}

func analyzerHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	if s.Count > 0 {
		s.AvgMinor = s.TotalMinor / int64(s.Count)
	}
	s.Summary = fmt.Sprintf("%d transactions totalling %d paise (avg %d)", s.Count, s.TotalMinor, s.AvgMinor)
	return emit(s)
}

func anomalyHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	if s.AvgMinor > 0 && s.TotalMinor > 50*s.AvgMinor {
		s.AnomalyNote = "unusually high aggregate spend vs per-txn average"
	}
	return emit(s)
}

func forecasterHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	s.ForecastMin = s.TotalMinor // naive carry-forward
	return emit(s)
}

func passthroughHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	return emit(s)
}

func recommenderHandle(in string) (string, error) {
	s, err := parseState(in)
	if err != nil {
		return "", err
	}
	recs := []string{"Track discretionary categories."}
	if s.AnomalyNote != "" {
		recs = append(recs, "Review the flagged high-spend anomaly.")
	}
	b, _ := json.Marshal(map[string]any{"state": s, "recommendations": recs})
	return string(b), nil
}

// Pipeline composes the governed preprocess chain and a specialist fan-out into a
// single final report — the framework port of analyzer -> specialists -> supervisor
// -> reporter. Every stage is individually governed.
type Pipeline struct {
	gate  governance.Policy
	chain []*agent.Agent
}

// NewPipeline builds the preprocess chain (ingestor -> normalizer -> enricher ->
// analyzer), each a governed agent.
func NewPipeline(gate governance.Policy) *Pipeline {
	mk := func(id string, h func(string) (string, error)) *agent.Agent {
		return NewGovernedDeterministic(gate, id, det(h))
	}
	return &Pipeline{
		gate: gate,
		chain: []*agent.Agent{
			mk("ingestor", ingestorHandle),
			mk("normalizer", normalizerHandle),
			mk("enricher", enricherHandle),
			mk("analyzer", analyzerHandle),
		},
	}
}

// Report is the pipeline's final output.
type Report struct {
	Analysis    string             `json:"analysis"`
	Specialists []SpecialistOutput `json:"specialists"`
	Disclosure  string             `json:"disclosure"`
}

// Run threads raw input through the governed preprocess chain (latency = sum of the
// linear stages), fans out the analysis to the specialists concurrently (latency =
// max(specialists)), and assembles a report. Any stage that the gate denies aborts
// the run with the denial.
func (p *Pipeline) Run(ctx context.Context, raw string, specialists ...*agent.Agent) (Report, error) {
	cur := raw
	for _, a := range p.chain {
		resp, err := a.RunText(ctx, cur).Collect()
		if err != nil {
			return Report{}, err
		}
		cur = ResponseText(resp)
	}
	analysis := cur
	specs, err := FanOut(ctx, analysis, specialists...)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Analysis:    analysis,
		Specialists: specs,
		Disclosure:  "AI-generated, informational only; you retain final authority over any financial decision.",
	}, nil
}
