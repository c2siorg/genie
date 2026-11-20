// Command enricher runs the enricher agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/enricher"
)

func main() {
	agentmain.RunLegacyAgent("enricher", enricher.New())
}
