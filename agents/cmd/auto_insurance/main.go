// Command auto_insurance runs the auto_insurance agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/auto_insurance"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunLegacyAgent("auto_insurance", auto_insurance.New(nil))
}
