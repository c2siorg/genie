package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// RequiredMetadataPolicy denies messages of the configured types unless the
// given metadata keys are present and non-empty.
//
// Modeled on MARA's "Security" guidance: PII-bearing messages must always
// carry an account_id, trace_id, etc.
type RequiredMetadataPolicy struct {
	// AppliesTo is the set of message types this policy applies to.
	// If empty, the policy applies to all messages.
	AppliesTo []string
	// Required is the set of metadata keys that must exist and be non-empty.
	Required []string
}

func (p RequiredMetadataPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	if !applies(p.AppliesTo, msg.Type) {
		return PolicyResult{Decision: DecisionAllow, Reason: "type not in scope", CheckedAt: time.Now().UTC()}, nil
	}
	if msg.Metadata == nil {
		return deny("missing metadata"), nil
	}
	for _, k := range p.Required {
		v, ok := msg.Metadata[k]
		if !ok {
			return deny(fmt.Sprintf("missing metadata key %q", k)), nil
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			return deny(fmt.Sprintf("empty metadata key %q", k)), nil
		}
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "all required metadata present", CheckedAt: time.Now().UTC()}, nil
}

func applies(list []string, t string) bool {
	if len(list) == 0 {
		return true
	}
	for _, s := range list {
		if s == t {
			return true
		}
	}
	return false
}

func deny(reason string) PolicyResult {
	return PolicyResult{Decision: DecisionDeny, Reason: reason, CheckedAt: time.Now().UTC()}
}
