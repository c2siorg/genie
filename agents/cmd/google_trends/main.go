// Command google_trends runs the google_trends agent as a standalone HTTP service
// over POST /handle (the pkg/agent.Agent HandleMessage contract).
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/google_trends"
)

func main() {
	agentmain.RunLegacyAgent("google_trends", google_trends.New(nil))
}
