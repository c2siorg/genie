package governance

import (
	"context"
	"regexp"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// piiPatterns flag obvious PII shapes in message content. The intent is a
// fail-closed boundary: deny rather than silently redact, because the
// reference Policy interface only returns allow/deny. A "redact" extension
// would change the interface; left as a follow-up.
var piiPatterns = []*regexp.Regexp{
	// 12+ consecutive digits (card / bank account / Aadhaar-shaped).
	regexp.MustCompile(`\d{12,}`),
	// Email addresses.
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	// 10-digit phone numbers with optional country prefix.
	regexp.MustCompile(`\+?\d{10,12}`),
}

// PIIBlockPolicy denies messages whose Content matches any PII pattern.
//
// Allowlist: if msg.Metadata["pii_acknowledged"] is "true" the policy allows
// the message — this is the human-in-the-loop escape hatch used by tool-bound
// agents that intentionally handle PII.
type PIIBlockPolicy struct{}

func (PIIBlockPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	if v, ok := msg.Metadata["pii_acknowledged"].(string); ok && v == "true" {
		return PolicyResult{Decision: DecisionAllow, Reason: "pii acknowledged", CheckedAt: time.Now().UTC()}, nil
	}
	for _, re := range piiPatterns {
		if re.MatchString(msg.Content) {
			return PolicyResult{Decision: DecisionDeny, Reason: "content matched PII pattern", CheckedAt: time.Now().UTC()}, nil
		}
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "no PII detected", CheckedAt: time.Now().UTC()}, nil
}
