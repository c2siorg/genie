// Command ingestor runs the ingestor agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/ingestor"
)

func main() {
	agentmain.RunLegacyAgent("ingestor", ingestor.New())
}
