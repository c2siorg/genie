// Command anomaly runs the anomaly agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/anomaly"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunLegacyAgent("anomaly", anomaly.New())
}
