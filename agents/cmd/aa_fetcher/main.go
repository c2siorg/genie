// Command aa_fetcher runs the Account Aggregator fetch agent as a standalone
// HTTP service (/handle). It wires the in-memory FI client + consent ledger
// fixtures, matching how cmd/api constructs it; swap these for real
// implementations via configuration when integrating a live AA network.
package main

import (
	aa_fetcher "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/aa_fetcher"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
)

func main() {
	agent := aa_fetcher.New(aa_fetcher.NewInMemoryFIClient(), compliance.NewInMemoryLedger())
	agentmain.RunLegacyAgent("aa_fetcher", agent)
}
