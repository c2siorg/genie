// Command health_preauth runs the health_preauth agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/health_preauth"
)

func main() {
	agentmain.RunLegacyAgent("health_preauth", health_preauth.New())
}
