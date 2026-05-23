package toolkit

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

type echoAgent struct{}

func (echoAgent) ID() string             { return "echo" }
func (echoAgent) Name() string           { return "echo" }
func (echoAgent) Capabilities() []string { return []string{"echo"} }
func (echoAgent) HandleMessage(_ context.Context, msg agent.Message, _ agent.Environment) ([]agent.Message, error) {
	return []agent.Message{agent.NewMessage("echo", msg.From, agent.RoleAgent, "echo", msg.Content, msg.Metadata)}, nil
}

func TestToolkit_PassesEchoAgent(t *testing.T) {
	pol := governance.NewComposite(
		governance.PIIBlockPolicy{},
	)
	sc := Run(context.Background(), Subject{
		Agent:  echoAgent{},
		Policy: pol,
		Sample: protocol.Message{Type: "ping", Content: "hello"},
	}, Defaults())

	// Every default check should pass for an honest echo agent.
	if sc.Failed != 0 {
		t.Fatalf("expected 0 failures, got %+v", sc)
	}
}
