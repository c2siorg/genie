// Package supervisor implements the financial_supervisor: kicks off the
// pipeline when it receives a finance question, then merges the forecaster /
// anomaly / recommender fan-out responses into one final report request.
//
// Per-question state is keyed by metadata["trace_id"] so concurrent sessions
// stay isolated.
package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID            = "financial_supervisor"
	CapSupervise  = "supervise_finance"

	TypeQuestion        = "finance_question"
	TypeForecast        = "forecast_result"
	TypeAnomalies       = "anomalies"
	TypeRecommendations = "recommendations"
	TypeReportRequest   = "final_report_request"

	TargetIngestor = "ingestor"
	TargetReporter = "reporter"
)

// analyzerSnapshot mirrors the fields we want to forward to the reporter.
type analyzerSnapshot struct {
	Currency          string   `json:"currency"`
	TotalIncomeCents  int64    `json:"total_income_cents"`
	TotalExpenseCents int64    `json:"total_expense_cents"`
	NetCents          int64    `json:"net_cents"`
	TopOverspend      []string `json:"top_overspend"`
}

// session collects fan-out responses for a single question.
type session struct {
	Question        string
	Analysis        *analyzerSnapshot
	Forecast        json.RawMessage
	Anomalies       json.RawMessage
	Recommendations json.RawMessage
}

func (s *session) isReady() bool {
	return s.Forecast != nil && s.Anomalies != nil && s.Recommendations != nil
}

type Agent struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func New() *Agent { return &Agent{sessions: map[string]*session{}} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Financial Supervisor" }
func (a *Agent) Capabilities() []string { return []string{CapSupervise} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	traceID, _ := msg.Metadata["trace_id"].(string)
	if traceID == "" {
		traceID = msg.ID
	}

	switch msg.Type {
	case TypeQuestion:
		// Start a fresh session and trigger the ingestor with the CSV payload
		// provided in metadata["csv"]. Production: persist sessions in memory.
		a.mu.Lock()
		a.sessions[traceID] = &session{Question: msg.Content}
		a.mu.Unlock()

		csv, _ := msg.Metadata["csv"].(string)
		if csv == "" {
			return nil, fmt.Errorf("supervisor: metadata[csv] required for question")
		}
		env.Logf("[supervisor] starting pipeline for question=%q trace=%s", msg.Content, traceID)
		return []agent.Message{
			agent.NewMessage(ID, TargetIngestor, agent.RoleAgent, "ingest_csv", csv, msg.Metadata),
		}, nil

	case "analysis_result":
		// Cache the analyzer view so we can stitch it into the final report.
		var snap analyzerSnapshot
		if err := json.Unmarshal([]byte(msg.Content), &snap); err != nil {
			return nil, err
		}
		a.update(traceID, func(s *session) { s.Analysis = &snap })

	case TypeForecast:
		a.update(traceID, func(s *session) { s.Forecast = json.RawMessage(msg.Content) })

	case TypeAnomalies:
		a.update(traceID, func(s *session) { s.Anomalies = json.RawMessage(msg.Content) })

	case TypeRecommendations:
		a.update(traceID, func(s *session) { s.Recommendations = json.RawMessage(msg.Content) })

	default:
		return nil, nil
	}

	return a.maybeFinalize(traceID, msg, env)
}

func (a *Agent) update(traceID string, fn func(*session)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[traceID]
	if !ok {
		s = &session{}
		a.sessions[traceID] = s
	}
	fn(s)
}

func (a *Agent) maybeFinalize(traceID string, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	a.mu.Lock()
	s, ok := a.sessions[traceID]
	if !ok || !s.isReady() {
		a.mu.Unlock()
		return nil, nil
	}
	delete(a.sessions, traceID)
	a.mu.Unlock()

	bundle := map[string]any{
		"question":        s.Question,
		"forecast":        s.Forecast,
		"anomalies":       s.Anomalies,
		"recommendations": s.Recommendations,
	}
	if s.Analysis != nil {
		bundle["currency"] = s.Analysis.Currency
		bundle["total_income_cents"] = s.Analysis.TotalIncomeCents
		bundle["total_expense_cents"] = s.Analysis.TotalExpenseCents
		bundle["net_cents"] = s.Analysis.NetCents
		bundle["top_overspend"] = s.Analysis.TopOverspend
	}
	body, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	env.Logf("[supervisor] all fan-outs received for trace=%s; dispatching reporter", traceID)
	return []agent.Message{
		agent.NewMessage(ID, TargetReporter, agent.RoleAgent, TypeReportRequest, string(body), msg.Metadata),
	}, nil
}
