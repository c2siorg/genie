// Command financial-analyst runs the FinancialAnalyst agent as a standalone HTTP
// service. All operational concerns live in the shared agentmain harness.
package main

import (
	financial_analyst "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/financial-analyst"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunAgent("financial-analyst", financial_analyst.NewAgent())
}
