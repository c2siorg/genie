// Package tests holds end-to-end checks that exercise the full multi-agent
// pipeline through the in-memory bus.
package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/analyzer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/anomaly"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/enricher"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/forecaster"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/ingestor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/normalizer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/recommender"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/orchestration"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

const csvSample = `date,description,category,amount,type
2026-01-01,Salary,Income,50000,credit
2026-01-03,Swiggy order,Food,450,debit
2026-01-05,Uber ride,Transport,250,debit
2026-01-14,Rent payment,Housing,25000,debit
`

func TestEndToEnd_FinanceFlow(t *testing.T) {
	ctx := context.Background()
	logger := observability.NewStdLogger()
	env := &orchestration.SimpleEnvironment{Logger: logger, Clock: observability.SystemClock{}}
	reg := registry.NewInMemory()
	bus := comm.NewInMemoryBus()
	policy := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 16 * 1024})

	mustReg := func(a agent.Agent) {
		if err := reg.Register(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	mustReg(ingestor.New())
	mustReg(normalizer.New())
	mustReg(enricher.New())
	mustReg(analyzer.New())
	mustReg(forecaster.New())
	mustReg(anomaly.New())
	mustReg(recommender.New())
	mustReg(reporter.New())
	mustReg(supervisor.New())

	done := make(chan agent.Message, 1)
	var once sync.Once
	bus.Subscribe("user", func(ctx context.Context, msg agent.Message) {
		if msg.Type == reporter.TypeOut {
			once.Do(func() { done <- msg })
		}
	})

	orch := orchestration.NewOrchestrator(reg, bus, policy, env)
	orch.Start(ctx)

	bus.Publish(ctx, agent.NewMessage("user", supervisor.ID, agent.RoleUser, supervisor.TypeQuestion,
		"Where am I overspending?",
		map[string]any{"trace_id": "tr-test", "account_id": "acct-it", "csv": csvSample},
	))

	select {
	case msg := <-done:
		if !strings.Contains(msg.Content, "Genie Financial Report") {
			t.Fatalf("unexpected report:\n%s", msg.Content)
		}
		if !strings.Contains(msg.Content, "housing:rent") {
			t.Fatalf("report missing top category:\n%s", msg.Content)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for final report")
	}
}
