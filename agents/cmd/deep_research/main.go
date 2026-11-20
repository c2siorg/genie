// Command deep_research runs the deep_research agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/deep_research"
)

func main() {
	agentmain.RunLegacyAgent("deep_research", deep_research.New(nil, "", nil))
}
