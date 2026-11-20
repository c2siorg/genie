package afg

// This file reproduces Genie's legacy supervisor question-pipeline
// (ingestor -> normalizer -> enricher -> analyzer -> {forecaster, anomaly,
// recommender} -> supervisor -> reporter) as a governed, framework-native
// QAService, so the legacy bus can eventually be deleted without regressing
// the flagship /v1/ask endpoint.
//
// Design note (why this file imports the LEGACY agent packages directly):
// the legacy forecaster/anomaly/recommender/reporter JSON/text is embedded
// verbatim in the final report, so any hand re-derivation of their logic risks
// silent drift. Instead, every governed stage's RunFunc below drives the real
// legacy agents/<id>.Agent.HandleMessage under Genie's own no-LLM Environment —
// the framework layer contributes governance (via NewGovernedDeterministic,
// this package's single sanctioned construction door) and the sequencing that
// the bus would otherwise have provided; the domain logic is untouched, so
// byte-parity with the legacy oracle is structural, not coincidental.
import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	legacyanalyzer "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/analyzer"
	legacyanomaly "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/anomaly"
	legacyenricher "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/enricher"
	legacyforecaster "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/forecaster"
	legacyingestor "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/ingestor"
	legacynormalizer "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/normalizer"
	legacyrecommender "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/recommender"
	legacyreporter "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
	legacysupervisor "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	genieagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"

	"github.com/microsoft/agent-framework-go/agent"
)

// qaEnv is a minimal no-op agent.Environment (Genie's pkg/agent.Environment,
// not the framework's) good enough to drive the legacy agents deterministically
// — they only use it for Now()/Logf(), never for control flow.
type qaEnv struct{}

func (qaEnv) Now() time.Time                  { return time.Now().UTC() }
func (qaEnv) Logf(format string, args ...any) {}

var _ genieagent.Environment = qaEnv{}

// qaOnly asserts a legacy HandleMessage call produced exactly one follow-up
// message (true for every stage here except the analyzer's fan-out) and
// returns its Content. A count mismatch means a msg.Type guard didn't match
// what this file wired up — loud failure beats a silently empty payload.
func qaOnly(out []genieagent.Message, stage string) (string, error) {
	if len(out) != 1 {
		return "", fmt.Errorf("qa %s: expected exactly 1 output message, got %d (check msg.Type wiring)", stage, len(out))
	}
	return out[0].Content, nil
}

// --- governed stage RunFuncs (each wraps one legacy agent's HandleMessage) ---

func qaIngestorHandle(csv string) (string, error) {
	msg := genieagent.NewMessage("qa-user", legacyingestor.ID, genieagent.RoleUser, legacyingestor.TypeRawCSV, csv, nil)
	out, err := legacyingestor.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "ingestor")
}

func qaNormalizerHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacyingestor.ID, legacynormalizer.ID, genieagent.RoleAgent, legacynormalizer.TypeIn, in, nil)
	out, err := legacynormalizer.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "normalizer")
}

func qaEnricherHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacynormalizer.ID, legacyenricher.ID, genieagent.RoleAgent, legacyenricher.TypeIn, in, nil)
	out, err := legacyenricher.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "enricher")
}

// qaAnalyzerHandle drives the real analyzer, which fans out 4 byte-identical
// copies of its Result JSON (to forecaster/anomaly/recommender/supervisor —
// see agents/analyzer/analyzer.go's final return). We take any one of them and
// defensively confirm they really are identical, since downstream governed
// stages here are fed from this single string.
func qaAnalyzerHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacyenricher.ID, legacyanalyzer.ID, genieagent.RoleAgent, legacyanalyzer.TypeIn, in, nil)
	out, err := legacyanalyzer.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "", fmt.Errorf("qa analyzer: produced no output (check msg.Type wiring)")
	}
	first := out[0].Content
	for _, m := range out[1:] {
		if m.Content != first {
			return "", fmt.Errorf("qa analyzer: fan-out payloads diverged unexpectedly")
		}
	}
	return first, nil
}

func qaForecasterHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacyanalyzer.ID, legacyforecaster.ID, genieagent.RoleAgent, legacyforecaster.TypeIn, in, nil)
	out, err := legacyforecaster.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "forecaster")
}

func qaAnomalyHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacyanalyzer.ID, legacyanomaly.ID, genieagent.RoleAgent, legacyanomaly.TypeIn, in, nil)
	out, err := legacyanomaly.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "anomaly")
}

func qaRecommenderHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacyanalyzer.ID, legacyrecommender.ID, genieagent.RoleAgent, legacyrecommender.TypeIn, in, nil)
	out, err := legacyrecommender.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "recommender")
}

// qaSupervisorPacket is the internal glue payload threaded from Answer into the
// governed supervisor stage. In the legacy bus, the real supervisor.Agent
// collects this same information from FOUR independent message deliveries
// (finance_question, analysis_result, forecast_result, anomalies,
// recommendations landing over time, keyed by metadata["trace_id"]); here they
// are produced sequentially in-process, so we hand them to the stage together
// and replay the exact same sequence of HandleMessage calls a bus would have
// made, in order to reuse the legacy session/bundle-assembly logic unchanged.
type qaSupervisorPacket struct {
	Question        string          `json:"question"`
	Analysis        json.RawMessage `json:"analysis"`
	Forecast        json.RawMessage `json:"forecast"`
	Anomalies       json.RawMessage `json:"anomalies"`
	Recommendations json.RawMessage `json:"recommendations"`
}

// qaSupervisorHandle constructs a brand-new legacysupervisor.Agent for this
// call only, so concurrent Answer() calls never share (or race on) the legacy
// supervisor's internal trace_id-keyed session map. A fixed dummy trace id is
// safe because this instance is scoped to a single Answer() invocation.
func qaSupervisorHandle(in string) (string, error) {
	var pkt qaSupervisorPacket
	if err := json.Unmarshal([]byte(in), &pkt); err != nil {
		return "", fmt.Errorf("qa supervisor: bad packet: %w", err)
	}

	sup := legacysupervisor.New()
	env := qaEnv{}
	const traceID = "qa-single-shot"
	meta := map[string]any{"trace_id": traceID, "csv": "unused-qa-placeholder"}

	send := func(msgType, content string) ([]genieagent.Message, error) {
		msg := genieagent.NewMessage("qa-supervisor-driver", legacysupervisor.ID, genieagent.RoleAgent, msgType, content, meta)
		return sup.HandleMessage(context.Background(), msg, env)
	}

	// finance_question seeds session.Question and (harmlessly) returns an
	// ingest_csv message we discard — ingestion already ran as its own governed
	// stage upstream in Answer().
	if _, err := send(legacysupervisor.TypeQuestion, pkt.Question); err != nil {
		return "", err
	}
	if _, err := send(legacyanalyzer.TypeOut, string(pkt.Analysis)); err != nil {
		return "", err
	}
	if _, err := send(legacysupervisor.TypeForecast, string(pkt.Forecast)); err != nil {
		return "", err
	}
	if _, err := send(legacysupervisor.TypeAnomalies, string(pkt.Anomalies)); err != nil {
		return "", err
	}
	out, err := send(legacysupervisor.TypeRecommendations, string(pkt.Recommendations))
	if err != nil {
		return "", err
	}
	return qaOnly(out, "supervisor")
}

func qaReporterHandle(in string) (string, error) {
	msg := genieagent.NewMessage(legacysupervisor.ID, legacyreporter.ID, genieagent.RoleAgent, legacyreporter.TypeIn, in, nil)
	out, err := legacyreporter.New().HandleMessage(context.Background(), msg, qaEnv{})
	if err != nil {
		return "", err
	}
	return qaOnly(out, "reporter")
}

// QAService is the framework-native, governed reproduction of Genie's legacy
// supervisor question-pipeline. Every stage is a governed agent-framework
// agent built exclusively through NewGovernedDeterministic (this package's
// single sanctioned construction door — see factory.go's package doc), so a
// governance denial at ANY stage aborts the whole run.
type QAService struct {
	gate governance.Policy

	ingestor    *agent.Agent
	normalizer  *agent.Agent
	enricher    *agent.Agent
	analyzer    *agent.Agent
	forecaster  *agent.Agent
	anomaly     *agent.Agent
	recommender *agent.Agent
	supervisor  *agent.Agent
	reporter    *agent.Agent
}

// NewQAService builds every stage of the question-pipeline as a governed
// deterministic agent under gate.
func NewQAService(gate governance.Policy) *QAService {
	return &QAService{
		gate:        gate,
		ingestor:    NewGovernedDeterministic(gate, "qa_ingestor", det(qaIngestorHandle)),
		normalizer:  NewGovernedDeterministic(gate, "qa_normalizer", det(qaNormalizerHandle)),
		enricher:    NewGovernedDeterministic(gate, "qa_enricher", det(qaEnricherHandle)),
		analyzer:    NewGovernedDeterministic(gate, "qa_analyzer", det(qaAnalyzerHandle)),
		forecaster:  NewGovernedDeterministic(gate, "qa_forecaster", det(qaForecasterHandle)),
		anomaly:     NewGovernedDeterministic(gate, "qa_anomaly", det(qaAnomalyHandle)),
		recommender: NewGovernedDeterministic(gate, "qa_recommender", det(qaRecommenderHandle)),
		supervisor:  NewGovernedDeterministic(gate, "qa_supervisor", det(qaSupervisorHandle)),
		reporter:    NewGovernedDeterministic(gate, "qa_reporter", det(qaReporterHandle)),
	}
}

// qaCollect runs a governed stage over in and returns its text output. Errors
// — including a governance *DeniedError raised by GovMiddleware — are returned
// exactly as the framework yields them: ResponseStream.Collect propagates the
// yielded error value unchanged (see agent-framework-go/agent/response.go), so
// a *DeniedError here is the very same value GovMiddleware constructed, not a
// wrapped copy. Callers can errors.As for it.
func qaCollect(ctx context.Context, a *agent.Agent, in string) (string, error) {
	resp, err := a.RunText(ctx, in).Collect()
	if err != nil {
		return "", err
	}
	return ResponseText(resp), nil
}

// Answer runs the full question-pipeline for (csv, question) end to end in
// afg and returns the reporter's final human-readable report string — the
// same string the legacy bus pipeline would have produced for identical
// inputs (see qa_service_test.go's oracle parity test).
//
// Stages run sequentially, including the forecaster/anomaly/recommender trio
// that the legacy analyzer fans out concurrently over the bus. That trades the
// bus's max(stages) latency for sum(stages) here, in exchange for a simpler,
// data-race-free single-shot driver; the assembled bundle's CONTENT is
// identical either way because forecaster/anomaly/recommender are each pure
// functions of the same analyzer output and never interact with one another.
//
// A governance *DeniedError from any stage propagates unwrapped (via
// qaCollect) and aborts the run immediately — no further stage executes, and
// the caller (e.g. the HTTP edge) can distinguish a policy denial (403, no
// fallback) from an ordinary pipeline error.
func (q *QAService) Answer(ctx context.Context, csvContent, question string) (string, error) {
	raw, err := qaCollect(ctx, q.ingestor, csvContent)
	if err != nil {
		return "", err
	}
	normalized, err := qaCollect(ctx, q.normalizer, raw)
	if err != nil {
		return "", err
	}
	enriched, err := qaCollect(ctx, q.enricher, normalized)
	if err != nil {
		return "", err
	}
	analysis, err := qaCollect(ctx, q.analyzer, enriched)
	if err != nil {
		return "", err
	}

	forecast, err := qaCollect(ctx, q.forecaster, analysis)
	if err != nil {
		return "", err
	}
	anomalies, err := qaCollect(ctx, q.anomaly, analysis)
	if err != nil {
		return "", err
	}
	recommendations, err := qaCollect(ctx, q.recommender, analysis)
	if err != nil {
		return "", err
	}

	packet, err := json.Marshal(qaSupervisorPacket{
		Question:        question,
		Analysis:        json.RawMessage(analysis),
		Forecast:        json.RawMessage(forecast),
		Anomalies:       json.RawMessage(anomalies),
		Recommendations: json.RawMessage(recommendations),
	})
	if err != nil {
		return "", fmt.Errorf("qa: marshal supervisor packet: %w", err)
	}

	bundle, err := qaCollect(ctx, q.supervisor, string(packet))
	if err != nil {
		return "", err
	}

	report, err := qaCollect(ctx, q.reporter, bundle)
	if err != nil {
		return "", err
	}
	return report, nil
}
