package afg

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
	"github.com/microsoft/agent-framework-go/workflow"
	"github.com/microsoft/agent-framework-go/workflow/agentworkflow"
	"github.com/microsoft/agent-framework-go/workflow/inproc"
)

// SpecialistOutput is one specialist's contribution collected at the fan-in.
type SpecialistOutput struct {
	Executor string
	Text     string
}

// FanOut runs governed agents concurrently over the same input — the framework
// port of Genie's analyzer -> (specialists) -> supervisor fan-out/fan-in. Latency is
// max(stages), not sum, because the agents run concurrently on the in-process
// engine. Each agent is individually governed (built via NewGoverned*), so the
// governance gate still fires at every node of the graph.
func FanOut(ctx context.Context, input string, agents ...*agent.Agent) ([]SpecialistOutput, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("fanout: no agents")
	}
	wf, err := agentworkflow.NewConcurrentWorkflowBuilder(agents...).
		WithOutputFrom(agents...).
		WithName("genie-fanout").
		Build()
	if err != nil {
		return nil, fmt.Errorf("fanout build: %w", err)
	}
	run, err := inproc.Default.Run(ctx, wf, message.NewText(input))
	if err != nil {
		return nil, fmt.Errorf("fanout run: %w", err)
	}
	var out []SpecialistOutput
	for ev := range run.NewEvents() {
		oe, ok := ev.(workflow.OutputEvent)
		if !ok || oe.IsIntermediate() {
			continue
		}
		if up, ok := oe.Output.(*agent.ResponseUpdate); ok {
			out = append(out, SpecialistOutput{Executor: oe.ExecutorID, Text: updateText(up)})
		}
	}
	return out, nil
}

func updateText(up *agent.ResponseUpdate) string {
	if up == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range up.Contents {
		if tc, ok := c.(*message.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
