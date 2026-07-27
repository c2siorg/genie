package afg

// This is the oracle parity test for QAService: it drives the LEGACY bus
// agents by hand (ingestor -> normalizer -> enricher -> analyzer ->
// {forecaster, anomaly, recommender} -> supervisor -> reporter), routing each
// agent's output exactly the way pkg/orchestration's bus would (by msg.To /
// msg.Type), and asserts QAService.Answer reproduces the resulting report
// string byte-for-byte.

import (
	"context"
	"errors"
	"os"
	"testing"

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
)

// qaInlineSampleCSV mirrors data/sample.csv's shape (date,description,category,
// amount,type) in case the repo file is ever unavailable to the test binary;
// qaSampleCSVContent prefers the real file.
const qaInlineSampleCSV = `date,description,category,amount,type
2026-01-01,Salary,Income,50000,credit
2026-01-03,Swiggy order,Food,450,debit
2026-01-05,Swiggy,Food,350,debit
2026-01-06,Uber ride,Transport,250,debit
2026-01-08,Electricity bill,Utilities,2200,debit
2026-01-10,Netflix,Entertainment,649,debit
2026-01-12,Swiggy,Food,2000,debit
2026-01-14,Rent payment,Housing,25000,debit
2026-01-18,Amazon order,Shopping,1899,debit
2026-01-22,Salary,Income,50000,credit
`

const qaSampleQuestion = "Where did my money go this month?"

func qaSampleCSVContent(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../data/sample.csv")
	if err != nil {
		return qaInlineSampleCSV
	}
	return string(b)
}

// qaLegacyOracleReport drives the real legacy agents by hand, the way the bus
// would (routing each returned message by its To/Type), and returns the
// reporter's final report string. This is the parity oracle QAService.Answer
// must reproduce byte-for-byte.
func qaLegacyOracleReport(t *testing.T, csv, question string) string {
	t.Helper()
	ctx := context.Background()
	env := qaEnv{}
	const traceID = "legacy-oracle-trace"

	// Metadata set on the first message flows forward unchanged at every hop
	// (each legacy agent's HandleMessage constructs its follow-up message with
	// msg.Metadata, not fresh metadata), so tagging trace_id here is enough for
	// it to reach the supervisor on the analyzer/forecaster/anomaly/recommender
	// messages automatically.
	ingMeta := map[string]any{"trace_id": traceID}

	ing := legacyingestor.New()
	ingOut, err := ing.HandleMessage(ctx, genieagent.NewMessage("user", legacyingestor.ID, genieagent.RoleUser, legacyingestor.TypeRawCSV, csv, ingMeta), env)
	if err != nil || len(ingOut) != 1 {
		t.Fatalf("oracle ingestor: out=%d err=%v", len(ingOut), err)
	}

	norm := legacynormalizer.New()
	normOut, err := norm.HandleMessage(ctx, ingOut[0], env)
	if err != nil || len(normOut) != 1 {
		t.Fatalf("oracle normalizer: out=%d err=%v", len(normOut), err)
	}

	enr := legacyenricher.New()
	enrOut, err := enr.HandleMessage(ctx, normOut[0], env)
	if err != nil || len(enrOut) != 1 {
		t.Fatalf("oracle enricher: out=%d err=%v", len(enrOut), err)
	}

	ana := legacyanalyzer.New()
	anaOut, err := ana.HandleMessage(ctx, enrOut[0], env)
	if err != nil || len(anaOut) != 4 {
		t.Fatalf("oracle analyzer: out=%d err=%v", len(anaOut), err)
	}

	// Route the analyzer's fan-out by target, exactly as the bus would.
	var toForecaster, toAnomaly, toRecommender, toSupervisorAnalysis genieagent.Message
	for _, m := range anaOut {
		switch m.To {
		case legacyanalyzer.FanForecaster:
			toForecaster = m
		case legacyanalyzer.FanAnomaly:
			toAnomaly = m
		case legacyanalyzer.FanRecommend:
			toRecommender = m
		case legacyanalyzer.FanSupervisor:
			toSupervisorAnalysis = m
		}
	}
	if toForecaster.Content == "" || toAnomaly.Content == "" || toRecommender.Content == "" || toSupervisorAnalysis.Content == "" {
		t.Fatalf("oracle analyzer: fan-out did not cover all 4 targets: %+v", anaOut)
	}

	fc := legacyforecaster.New()
	fcOut, err := fc.HandleMessage(ctx, toForecaster, env)
	if err != nil || len(fcOut) != 1 {
		t.Fatalf("oracle forecaster: out=%d err=%v", len(fcOut), err)
	}

	an := legacyanomaly.New()
	anOut, err := an.HandleMessage(ctx, toAnomaly, env)
	if err != nil || len(anOut) != 1 {
		t.Fatalf("oracle anomaly: out=%d err=%v", len(anOut), err)
	}

	rec := legacyrecommender.New()
	recOut, err := rec.HandleMessage(ctx, toRecommender, env)
	if err != nil || len(recOut) != 1 {
		t.Fatalf("oracle recommender: out=%d err=%v", len(recOut), err)
	}

	sup := legacysupervisor.New()
	qMeta := map[string]any{"trace_id": traceID, "csv": csv}
	questionMsg := genieagent.NewMessage("user", legacysupervisor.ID, genieagent.RoleUser, legacysupervisor.TypeQuestion, question, qMeta)
	if _, err := sup.HandleMessage(ctx, questionMsg, env); err != nil {
		t.Fatalf("oracle supervisor(question): %v", err)
	}
	if _, err := sup.HandleMessage(ctx, toSupervisorAnalysis, env); err != nil {
		t.Fatalf("oracle supervisor(analysis): %v", err)
	}
	if _, err := sup.HandleMessage(ctx, fcOut[0], env); err != nil {
		t.Fatalf("oracle supervisor(forecast): %v", err)
	}
	if _, err := sup.HandleMessage(ctx, anOut[0], env); err != nil {
		t.Fatalf("oracle supervisor(anomalies): %v", err)
	}
	finalOut, err := sup.HandleMessage(ctx, recOut[0], env)
	if err != nil || len(finalOut) != 1 {
		t.Fatalf("oracle supervisor(recommendations): out=%d err=%v", len(finalOut), err)
	}

	rep := legacyreporter.New()
	repOut, err := rep.HandleMessage(ctx, finalOut[0], env)
	if err != nil || len(repOut) != 1 {
		t.Fatalf("oracle reporter: out=%d err=%v", len(repOut), err)
	}
	return repOut[0].Content
}

// TestQAService_AnswerMatchesLegacyOracleParity is the whole point of this
// file: QAService.Answer, running entirely inside afg's governed framework
// agents, must reproduce the legacy bus pipeline's final report string
// byte-for-byte for the same (csv, question).
//
// Parity verdict: TRUE byte-identity, no normalization needed. The reporter's
// output (agents/reporter/reporter.go) never includes a message ID, trace id,
// or timestamp — every field it prints (question/currency/income/expense/net/
// top_overspend/forecast/anomalies/recommendations JSON) is a pure function of
// (csv, question), so there is nothing nondeterministic to exclude.
//
// (Pre-existing, unrelated to afg: agents/analyzer/analyzer.go builds
// ByCategory from a Go map before sort.Slice, which is not a stable sort; if
// two categories ever tied exactly on AmountCents, ByCategory/TopOverspend
// order could vary run-to-run in the LEGACY agent itself, independent of afg.
// data/sample.csv's categories have no ties, so this does not manifest here —
// documented for honesty, not because it affects this test.)
func TestQAService_AnswerMatchesLegacyOracleParity(t *testing.T) {
	csv := qaSampleCSVContent(t)
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1 << 20})
	svc := NewQAService(gate)

	got, err := svc.Answer(context.Background(), csv, qaSampleQuestion)
	if err != nil {
		t.Fatalf("QAService.Answer: %v", err)
	}

	want := qaLegacyOracleReport(t, csv, qaSampleQuestion)

	if got != want {
		t.Fatalf("QAService.Answer diverged from the legacy oracle report.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if got == "" {
		t.Fatal("report must not be empty")
	}
}

// TestQAService_DeniedGovernanceAbortsPipeline proves the governance gate
// still holds across the whole afg pipeline: a deny-everything policy makes
// Answer return a *DeniedError (unwrapped) and no report is produced.
func TestQAService_DeniedGovernanceAbortsPipeline(t *testing.T) {
	csv := qaSampleCSVContent(t)
	deny := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1})
	svc := NewQAService(deny)

	report, err := svc.Answer(context.Background(), csv, qaSampleQuestion)
	if err == nil {
		t.Fatalf("expected governance denial to abort the pipeline, got report: %q", report)
	}
	var denied *DeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected *DeniedError, got %T: %v", err, err)
	}
	if report != "" {
		t.Fatalf("expected no report on denial, got %q", report)
	}
}
