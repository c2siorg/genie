package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"
)

// DataResidencyPolicy denies messages whose region tag does not satisfy the
// required home region for the message's classification.
//
// Default behaviour:
//   - PII and Secret messages must stay in HomeRegion (or be on-prem).
//   - Public/Internal messages may cross borders.
//
// HomeRegion is typically "in" for India deployments.
type DataResidencyPolicy struct {
	HomeRegion sovereignty.Region
	// AllowCrossBorderForPublic toggles whether ClassPublic may leave the home
	// region. Default true (less surprising), but flip to false for the
	// strictest deployments.
	AllowCrossBorderForPublic bool
}

// NewResidencyPolicy returns a policy with sensible defaults.
func NewResidencyPolicy(home sovereignty.Region) DataResidencyPolicy {
	return DataResidencyPolicy{HomeRegion: home, AllowCrossBorderForPublic: true}
}

func (p DataResidencyPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	region := messageRegion(msg.Metadata)
	cls := protocol.ClassificationOf(msg.Metadata, protocol.ClassInternal)
	if region == "" || region == p.HomeRegion || region == sovereignty.RegionOnPrem {
		return PolicyResult{Decision: DecisionAllow, Reason: "in-home or on-prem", CheckedAt: time.Now().UTC()}, nil
	}
	// Region mismatches require explicit allowance per classification.
	switch cls {
	case protocol.ClassPII, protocol.ClassSecret:
		return PolicyResult{
			Decision:  DecisionDeny,
			Reason:    fmt.Sprintf("classification %q cannot leave region %q (target=%q)", cls, p.HomeRegion, region),
			CheckedAt: time.Now().UTC(),
		}, nil
	case protocol.ClassInternal:
		return PolicyResult{Decision: DecisionDeny, Reason: "internal data cannot leave home region", CheckedAt: time.Now().UTC()}, nil
	case protocol.ClassPublic:
		if !p.AllowCrossBorderForPublic {
			return PolicyResult{Decision: DecisionDeny, Reason: "public-data cross-border disabled", CheckedAt: time.Now().UTC()}, nil
		}
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "within residency policy", CheckedAt: time.Now().UTC()}, nil
}

func messageRegion(metadata map[string]any) sovereignty.Region {
	if metadata == nil {
		return ""
	}
	if v, ok := metadata[sovereignty.MetaKeyRegion].(string); ok {
		return sovereignty.Region(v)
	}
	return ""
}
