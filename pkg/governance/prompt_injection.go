package governance

import (
	"context"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// PromptInjectionPolicy denies messages whose content contains naive
// prompt-injection markers. Inspired by the ADK "Safety Guardrail Plugins"
// sample (Gemini-as-a-judge / Model Armor). Real systems should route
// through a dedicated LLM classifier.
type PromptInjectionPolicy struct {
	ExtraPhrases []string
}

var defaultInjectionPhrases = []string{
	"ignore previous instructions",
	"ignore prior instructions",
	"disregard the system prompt",
	"reveal your system prompt",
	"act as the admin",
	"system: you are now",
	"begin override",
}

func (p PromptInjectionPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	body := strings.ToLower(msg.Content)
	for _, phrase := range defaultInjectionPhrases {
		if strings.Contains(body, phrase) {
			return PolicyResult{Decision: DecisionDeny, Reason: "prompt-injection phrase detected", CheckedAt: time.Now().UTC()}, nil
		}
	}
	for _, phrase := range p.ExtraPhrases {
		if strings.Contains(body, strings.ToLower(phrase)) {
			return PolicyResult{Decision: DecisionDeny, Reason: "prompt-injection phrase detected", CheckedAt: time.Now().UTC()}, nil
		}
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "no injection markers", CheckedAt: time.Now().UTC()}, nil
}
