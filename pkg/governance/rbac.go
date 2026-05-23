package governance

import (
	"context"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// RBACPolicy denies messages whose authenticated roles do not satisfy the
// requirements for the message type. It also denies when an unauthenticated
// message attempts to reach a protected message type.
//
// Reads roles from msg.Metadata["user_roles"] (set by HTTP auth middleware
// when forwarding requests onto the bus). Values may be a comma-separated
// string ("user,admin") or a []string.
type RBACPolicy struct {
	// RequiredRolesByType maps message type -> any-of role list.
	// Empty value means "no roles required" (still allowed for unauthenticated).
	// If a type is not in the map, it is allowed by default — opt-in gating.
	RequiredRolesByType map[string][]string

	// AdminBypass: if true, any user holding "admin" role bypasses gating.
	AdminBypass bool
}

func (p RBACPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	required, ok := p.RequiredRolesByType[msg.Type]
	if !ok || len(required) == 0 {
		return PolicyResult{Decision: DecisionAllow, Reason: "no rbac requirement", CheckedAt: time.Now().UTC()}, nil
	}
	roles := extractRoles(msg.Metadata)
	if p.AdminBypass && containsRole(roles, "admin") {
		return PolicyResult{Decision: DecisionAllow, Reason: "admin bypass", CheckedAt: time.Now().UTC()}, nil
	}
	for _, r := range required {
		if containsRole(roles, r) {
			return PolicyResult{Decision: DecisionAllow, Reason: "role satisfied", CheckedAt: time.Now().UTC()}, nil
		}
	}
	return PolicyResult{Decision: DecisionDeny, Reason: "missing required role for message type", CheckedAt: time.Now().UTC()}, nil
}

func extractRoles(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	switch v := metadata[protocol.MetaKeyUserRoles].(type) {
	case string:
		if v == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func containsRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}
