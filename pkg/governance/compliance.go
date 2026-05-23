package governance

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// ConsentPolicy denies messages that require a specific consent category for
// which the user has not granted (or has revoked) consent.
//
// The mapping from message Type to required ConsentCategory is supplied at
// construction time so this policy is reusable across deployments.
type ConsentPolicy struct {
	Ledger    compliance.Ledger
	TypeToCat map[string]compliance.ConsentCategory
}

func (p ConsentPolicy) Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error) {
	cat, ok := p.TypeToCat[msg.Type]
	if !ok {
		return PolicyResult{Decision: DecisionAllow, Reason: "no consent required", CheckedAt: time.Now().UTC()}, nil
	}
	userID, _ := msg.Metadata[protocol.MetaKeyUserID].(string)
	if userID == "" {
		return PolicyResult{Decision: DecisionDeny, Reason: "consent required but user id missing", CheckedAt: time.Now().UTC()}, nil
	}
	ok, err := p.Ledger.HasActive(ctx, userID, cat)
	if err != nil {
		return PolicyResult{}, err
	}
	if !ok {
		return PolicyResult{Decision: DecisionDeny, Reason: "missing active consent for category " + string(cat), CheckedAt: time.Now().UTC()}, nil
	}
	return PolicyResult{Decision: DecisionAllow, Reason: "consent on file", CheckedAt: time.Now().UTC()}, nil
}

// ExplainabilityPolicy enforces that messages of certain types include a
// non-empty rationale alongside their payload. Keeps recommender and
// advisor outputs accountable: RBI's explainability guidance asks lenders to
// disclose the basis of automated credit decisions; this is the minimum bar.
type ExplainabilityPolicy struct {
	// AppliesTo lists message types that must include rationale.
	AppliesTo []string
	// RationaleField is the JSON key to look for in the payload (default "rationale").
	RationaleField string
}

func (p ExplainabilityPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	if !typeMatches(p.AppliesTo, msg.Type) {
		return PolicyResult{Decision: DecisionAllow, Reason: "type out of scope", CheckedAt: time.Now().UTC()}, nil
	}
	field := p.RationaleField
	if field == "" {
		field = "rationale"
	}
	// Allow either {"rationale":"..."} at root OR a list of objects each with rationale.
	if hasRationale(msg.Content, field) {
		return PolicyResult{Decision: DecisionAllow, Reason: "rationale present", CheckedAt: time.Now().UTC()}, nil
	}
	return PolicyResult{Decision: DecisionDeny, Reason: "missing rationale for output", CheckedAt: time.Now().UTC()}, nil
}

func typeMatches(list []string, t string) bool {
	for _, x := range list {
		if x == t {
			return true
		}
	}
	return false
}

func hasRationale(content, field string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	// Try the simple object form first.
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
		if v, ok := obj[field]; ok {
			if s, isStr := v.(string); isStr && s != "" {
				return true
			}
		}
		// Look one level deep for a list of items that each carry the field.
		for _, v := range obj {
			if items, ok := v.([]any); ok && len(items) > 0 {
				ok := true
				for _, it := range items {
					m, _ := it.(map[string]any)
					if s, isStr := m[field].(string); !isStr || s == "" {
						ok = false
						break
					}
				}
				if ok {
					return true
				}
			}
		}
	}
	return false
}
