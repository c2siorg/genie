// Command sme_loan_workflow runs the sme_loan_workflow agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/sme_loan_workflow"
)

func main() {
	agentmain.RunLegacyAgent("sme_loan_workflow", sme_loan_workflow.New())
}
