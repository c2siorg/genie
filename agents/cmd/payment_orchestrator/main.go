// Command payment_orchestrator runs the payment_orchestrator agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/payment_orchestrator"
)

func main() {
	agentmain.RunLegacyAgent("payment_orchestrator", payment_orchestrator.New())
}
