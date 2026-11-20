// Command forecaster runs the forecaster agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/forecaster"
)

func main() {
	agentmain.RunLegacyAgent("forecaster", forecaster.New())
}
