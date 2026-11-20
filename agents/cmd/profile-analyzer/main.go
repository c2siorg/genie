// Command profile-analyzer runs the ProfileAnalyzer agent as a standalone HTTP
// service. All operational concerns live in the shared agentmain harness.
package main

import (
	profile_analyzer "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/profile-analyzer"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunAgent("profile-analyzer", profile_analyzer.NewAgent())
}
