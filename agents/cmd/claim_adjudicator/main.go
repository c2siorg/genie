// Command claim_adjudicator runs the claim_adjudicator agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/claim_adjudicator"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunLegacyAgent("claim_adjudicator", claim_adjudicator.New())
}
