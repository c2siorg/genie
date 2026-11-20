// Command cyber_guardian runs the cyber_guardian agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cyber_guardian"
)

func main() {
	agentmain.RunLegacyAgent("cyber_guardian", cyber_guardian.New())
}
