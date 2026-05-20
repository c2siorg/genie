package main

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/orchestration"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// This demo intentionally keeps agent logic trivial so you can focus on the
// architecture and the *message flow*.
//
// High-level storyline
//
// - A "user" submits a goal message to the planner agent.
// - The planner produces a plan and sends it to the executor agent.
// - The executor produces a result and sends it to the coordinator agent.
// - The coordinator logs the final result (end of workflow).
//
// In a real system:
// - planner/executor/coordinator could each be LLM-backed
// - the executor might call tools
// - the coordinator might dynamically route based on capabilities
// - evaluation and memory would be integrated into Environment
// - governance would include tool authorization and content safeguards

type planningAgent struct {
	id string
}

func (p *planningAgent) ID() string            { return p.id }
func (p *planningAgent) Name() string          { return "Planner" }
func (p *planningAgent) Capabilities() []string { return []string{"plan_task"} }

func (p *planningAgent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	// The planner's job is to convert an incoming "goal" message into a "plan".
	//
	// Notice the shape:
	// - Input: a single message (msg) plus access to shared services via env
	// - Output: zero or more follow-up messages
	//
	// The planner does NOT call the executor directly. It *emits a message* addressed
	// to the executor. Routing is handled by the orchestration+bus layers.
	env.Logf("[planner] received: %s", msg.Content)
	plan := fmt.Sprintf("Plan for goal '%s':\n1) Analyze\n2) Execute\n3) Summarize", msg.Content)
	out := agent.NewMessage(p.id, "executor", agent.RoleAgent, "plan", plan, nil)
	return []agent.Message{out}, nil
}

type executorAgent struct {
	id string
}

func (e *executorAgent) ID() string            { return e.id }
func (e *executorAgent) Name() string          { return "Executor" }
func (e *executorAgent) Capabilities() []string { return []string{"execute_plan"} }

func (e *executorAgent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	// The executor's job is to transform a "plan" into an execution "result".
	//
	// In a production system, this is often where tool calls happen:
	// - search/retrieval
	// - code execution
	// - database queries
	// - external API calls
	//
	// Those integrations are deliberately not included here; the point is to show
	// that tools would be invoked from within HandleMessage, and their outputs would
	// typically be emitted as additional messages (RoleTool) or included in Metadata.
	env.Logf("[executor] executing: %s", msg.Content)
	result := fmt.Sprintf("Executed plan derived from: %q", msg.Content)
	out := agent.NewMessage(e.id, "coordinator", agent.RoleAgent, "result", result, nil)
	return []agent.Message{out}, nil
}

type coordinatorAgent struct {
	id string
}

func (c *coordinatorAgent) ID() string            { return c.id }
func (c *coordinatorAgent) Name() string          { return "Coordinator" }
func (c *coordinatorAgent) Capabilities() []string { return []string{"coordinate"} }

func (c *coordinatorAgent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	// The coordinator is the end of this toy workflow.
	//
	// Coordinators are common in multi-agent systems:
	// - They manage task decomposition (what sub-tasks exist?)
	// - They route to specialists (who should handle sub-task X?)
	// - They merge and verify outputs (is the final answer coherent/safe?)
	//
	// In this minimal demo, the "coordination" is simply logging the final result.
	env.Logf("[coordinator] final result: %s", msg.Content)
	return nil, nil
}

func main() {
	// ---------------------------------------------------------------------------
	// 1) Observability and environment
	// ---------------------------------------------------------------------------
	//
	// We build an Environment implementation that agents and orchestration can use.
	// - Logger: where logs go
	// - Clock: where "now" comes from (helps testing if swapped)
	logger := observability.NewStdLogger()
	env := &orchestration.SimpleEnvironment{
		Logger: logger,
		Clock:  observability.SystemClock{},
	}

	// ---------------------------------------------------------------------------
	// 2) Core platform building blocks
	// ---------------------------------------------------------------------------
	//
	// Registry: where agents are registered and discovered.
	// Bus: how messages are routed (publish/subscribe).
	// Policy: governance guardrails applied *before* agents handle messages.
	reg := registry.NewInMemory()
	bus := comm.NewInMemoryBus()
	policy := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 4096})

	// ---------------------------------------------------------------------------
	// 3) Create agents (specialists)
	// ---------------------------------------------------------------------------
	//
	// Each agent has a stable ID. That ID acts as the routing address and is used
	// in protocol.Message.To.
	planner := &planningAgent{id: "planner"}
	executor := &executorAgent{id: "executor"}
	coord := &coordinatorAgent{id: "coordinator"}

	// ---------------------------------------------------------------------------
	// 4) Register agents
	// ---------------------------------------------------------------------------
	//
	// Registration makes agents discoverable to orchestration.
	// In this demo we ignore errors for brevity.
	ctx := context.Background()
	_ = reg.Register(ctx, planner)
	_ = reg.Register(ctx, executor)
	_ = reg.Register(ctx, coord)

	// ---------------------------------------------------------------------------
	// 5) Start orchestration (wire subscriptions)
	// ---------------------------------------------------------------------------
	//
	// Orchestrator.Start does the critical wiring:
	// - reads the current registry snapshot
	// - subscribes the bus for each agent ID
	// - for each incoming message:
	//     * applies governance policy
	//     * invokes agent.HandleMessage
	//     * publishes follow-up messages
	orch := orchestration.NewOrchestrator(reg, bus, policy, env)
	orch.Start(ctx)

	// ---------------------------------------------------------------------------
	// 6) Kick off the workflow with a user "goal" message
	// ---------------------------------------------------------------------------
	//
	// This is the only "external" action in the demo. Everything after this is
	// message-driven coordination.
	start := agent.NewMessage("user", "planner", agent.RoleUser, "goal", "draft a short plan for a new feature launch", nil)
	bus.Publish(ctx, start)

	// ---------------------------------------------------------------------------
	// 7) Wait briefly for asynchronous message handling
	// ---------------------------------------------------------------------------
	//
	// The in-memory bus delivers messages via goroutines. Sleeping is the simplest
	// way to keep main alive long enough for the demo to complete.
	//
	// Production systems would coordinate shutdown and completion using:
	// - WaitGroups
	// - contexts + cancellation
	// - explicit "done" messages / state machines
	time.Sleep(2 * time.Second)
}

