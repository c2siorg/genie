// Command mpc_research runs the mpc_research agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/mpc_research"
)

func main() {
	agentmain.RunLegacyAgent("mpc_research", mpc_research.New())
}
