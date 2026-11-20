// Command supply_chain_finance runs the supply_chain_finance agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supply_chain_finance"
)

func main() {
	agentmain.RunLegacyAgent("supply_chain_finance", supply_chain_finance.New())
}
