// Command tax_estimator runs the tax_estimator agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/tax_estimator"
)

func main() {
	agentmain.RunLegacyAgent("tax_estimator", tax_estimator.New())
}
