// Command kyc_orchestrator runs the kyc_orchestrator agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/kyc_orchestrator"
)

func main() {
	agentmain.RunLegacyAgent("kyc_orchestrator", kyc_orchestrator.New())
}
