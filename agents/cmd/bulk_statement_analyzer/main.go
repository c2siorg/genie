// Command bulk_statement_analyzer runs the bulk_statement_analyzer agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/bulk_statement_analyzer"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunLegacyAgent("bulk_statement_analyzer", bulk_statement_analyzer.New())
}
