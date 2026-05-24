// Command genie wires every implemented agent onto the platform, kicks the
// finance supervisor with a sample question, and prints the rendered report.
// Traces and metrics go to stdout via OpenTelemetry stdout exporters.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/analyzer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/anomaly"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/auditor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/auto_insurance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/bulk_statement_analyzer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/claim_adjudicator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/currency"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cyber_guardian"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/deep_research"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/educator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/enricher"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/forecaster"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/google_trends"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/health_preauth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/ingestor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/invoice_processor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/kyc_orchestrator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/loan"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/macro"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/mpc_research"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/normalizer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/payment_orchestrator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/rates"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/recommender"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/sme_loan_workflow"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supply_chain_finance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/orchestration"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

const sampleCSV = `date,description,category,amount,type
2026-01-01,Salary,Income,50000,credit
2026-01-03,Swiggy order,Food,450,debit
2026-01-05,Swiggy,Food,350,debit
2026-01-06,Uber ride,Transport,250,debit
2026-01-08,Electricity bill,Utilities,2200,debit
2026-01-10,Netflix,Entertainment,649,debit
2026-01-12,Swiggy,Food,2000,debit
2026-01-14,Rent payment,Housing,25000,debit
2026-01-18,Amazon order,Shopping,1899,debit
2026-01-22,Salary,Income,50000,credit`

func main() {
	ctx := context.Background()

	// OTEL traces are noisy on stdout; route them to a file so the chat-style
	// console log stays readable. Metrics still go to stdout periodically.
	traceFile, err := os.Create("genie-traces.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace file:", err)
		os.Exit(1)
	}
	defer traceFile.Close()

	tel, err := observability.SetupTelemetry(ctx, observability.TelemetryConfig{
		ServiceName:    "genie",
		ServiceVersion: "0.1.0",
		TraceWriter:    traceFile,
		MetricWriter:   io.Discard, // silence periodic metric dumps in the demo
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "telemetry:", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tel.Shutdown(shutdownCtx)
	}()

	logger := observability.NewStdLogger()
	env := &orchestration.SimpleEnvironment{Logger: logger, Clock: observability.SystemClock{}}

	reg := registry.NewInMemory()
	bus := comm.NewInMemoryBus()
	evalStore := eval.NewInMemoryStore()

	// Stack: deny obvious prompt injection, require trace_id on finance_question.
	policy := governance.NewComposite(
		governance.MaxContentLengthPolicy{Max: 64 * 1024},
		governance.RequiredMetadataPolicy{
			AppliesTo: []string{"finance_question"},
			Required:  []string{"trace_id", "account_id"},
		},
		governance.PromptInjectionPolicy{},
	)

	register := func(a agent.Agent) {
		if err := reg.Register(ctx, a); err != nil {
			fmt.Fprintln(os.Stderr, "register:", err)
		}
	}
	register(ingestor.New())
	register(normalizer.New())
	register(enricher.New())
	register(analyzer.New())
	register(forecaster.New())
	register(anomaly.New())
	register(recommender.New())
	register(reporter.New())
	register(supervisor.New())
	register(currency.New())
	register(educator.New())
	register(macro.New())
	register(rates.New())
	register(loan.New())
	register(auditor.New(evalStore))

	// ADK-inspired extension agents (see docs/adk-extension-proposal.md).
	register(kyc_orchestrator.New())
	register(claim_adjudicator.New())
	register(sme_loan_workflow.New())
	register(invoice_processor.New())
	register(deep_research.New(nil, "", nil))
	register(bulk_statement_analyzer.New())
	register(mpc_research.New())
	register(auto_insurance.New(nil))
	register(health_preauth.New())
	register(supply_chain_finance.New())
	register(payment_orchestrator.New())
	register(cyber_guardian.New())
	register(google_trends.New(nil))

	// Catch the final report so we can print it and unblock main.
	done := make(chan agent.Message, 1)
	var once sync.Once
	bus.Subscribe("user", func(ctx context.Context, msg agent.Message) {
		if msg.Type == reporter.TypeOut {
			once.Do(func() { done <- msg })
		}
	})

	// Broadcast auditor records every message that crosses the bus.
	bus.Subscribe("", auditor.NewHandler(evalStore))

	orch := orchestration.NewOrchestrator(reg, bus, policy, env)
	orch.Start(ctx)

	traceID := fmt.Sprintf("tr-%d", time.Now().UnixNano())
	question := agent.NewMessage("user", supervisor.ID, agent.RoleUser, supervisor.TypeQuestion,
		"Where am I overspending vs last month?",
		map[string]any{
			"trace_id":   traceID,
			"account_id": "acct-123",
			"csv":        sampleCSV,
		},
	)
	bus.Publish(ctx, question)

	select {
	case report := <-done:
		fmt.Println("\n=== FINAL REPORT ===")
		fmt.Println(report.Content)
	case <-time.After(5 * time.Second):
		fmt.Fprintln(os.Stderr, "timed out waiting for report")
	}

	fmt.Printf("\nAudit records collected: %d\n", len(evalStore.List()))
}
