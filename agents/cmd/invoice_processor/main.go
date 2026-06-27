// Command invoice_processor runs the invoice_processor agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/invoice_processor"
)

func main() {
	agentmain.RunLegacyAgent("invoice_processor", invoice_processor.New())
}
