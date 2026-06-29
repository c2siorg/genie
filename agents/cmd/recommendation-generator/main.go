// Command recommendation-generator runs the RecommendationGenerator agent as a
// standalone HTTP service. All operational concerns live in the agentmain harness.
package main

import (
	recommendation_generator "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/recommendation-generator"
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
)

func main() {
	agentmain.RunAgent("recommendation-generator", recommendation_generator.NewAgent())
}
