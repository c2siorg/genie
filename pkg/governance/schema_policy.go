package governance

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/schema"
)

// SchemaPolicy validates Message.Content as JSON against a schema registered
// for the message Type. Messages whose Type has no registered schema are
// allowed unconditionally (opt-in gating, matching the rest of governance).
type SchemaPolicy struct {
	SchemasByType map[string]*schema.Schema
}

func (p SchemaPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	s, ok := p.SchemasByType[msg.Type]
	if !ok {
		return PolicyResult{Decision: DecisionAllow, Reason: "no schema registered", CheckedAt: time.Now().UTC()}, nil
	}
	if err := s.ValidateJSON([]byte(msg.Content)); err != nil {
		return PolicyResult{
			Decision:  DecisionDeny,
			Reason:    "schema validation failed: " + err.Error(),
			CheckedAt: time.Now().UTC(),
		}, nil
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "schema passed", CheckedAt: time.Now().UTC()}, nil
}
