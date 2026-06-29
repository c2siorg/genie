// Command reporter runs the reporter agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
)

func main() {
	agentmain.RunLegacyAgent("reporter", reporter.New())
}
