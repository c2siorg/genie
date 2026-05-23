package governance

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// ClassificationPolicy denies messages whose classification exceeds the
// configured ceiling for the recipient agent. It is the data-handling
// counterpart to RBACPolicy: where RBAC controls *who* can dispatch a message
// type, ClassificationPolicy controls *which* sensitivity levels each agent
// may receive.
//
// Defaults: agents not listed in AllowedTo are limited to ClassInternal.
type ClassificationPolicy struct {
	// AllowedTo maps recipient agent id -> highest classification it may
	// receive. Order is: public < internal < pii < secret.
	AllowedTo map[string]protocol.Classification

	// DefaultCeiling applies to any recipient not present in AllowedTo.
	DefaultCeiling protocol.Classification
}

var classRank = map[protocol.Classification]int{
	protocol.ClassPublic:   0,
	protocol.ClassInternal: 1,
	protocol.ClassPII:      2,
	protocol.ClassSecret:   3,
}

func (p ClassificationPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	msgClass := protocol.ClassificationOf(msg.Metadata, protocol.ClassInternal)
	ceiling := p.DefaultCeiling
	if ceiling == "" {
		ceiling = protocol.ClassInternal
	}
	if c, ok := p.AllowedTo[msg.To]; ok {
		ceiling = c
	}
	if classRank[msgClass] > classRank[ceiling] {
		return PolicyResult{Decision: DecisionDeny, Reason: "classification exceeds recipient ceiling", CheckedAt: time.Now().UTC()}, nil
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "classification within ceiling", CheckedAt: time.Now().UTC()}, nil
}
