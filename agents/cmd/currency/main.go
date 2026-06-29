// Command currency runs the currency agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/currency"
)

func main() {
	agentmain.RunLegacyAgent("currency", currency.New())
}
