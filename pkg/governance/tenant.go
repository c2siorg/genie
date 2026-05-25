// tenant.go — tenant-isolation policy at the message bus.
//
// Defence in depth pairs:
//   - Application-level: RBAC + this TenantPolicy at the message bus.
//   - Database-level: Postgres Row-Level Security (pkg/storage/postgres).
//
// The bus check denies a message that targets one tenant but carries
// metadata for another. The DB check refuses to return another tenant's
// rows even if the application would query them.
//
// The two together mean a single bug in either layer is contained by
// the other. A bug in the bus check that lets a cross-tenant message
// through still hits RLS at the DB and gets denied there. A bug in RLS
// configuration still has the bus check.
package governance

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// TenantPolicy denies a message whose metadata.tenant_id is missing, or
// (when AppliesTo is set) whose tenant doesn't match metadata.expected_tenant
// for messages of the specified types.
//
// In a single-tenant deployment (most consumer-facing finance apps), the
// tenant is the user_id. In a multi-tenant deployment (corporate-banking
// where one tenant = one corporate customer), the tenant is the org_id,
// and a user can belong to multiple tenants via the user_roles claim.
type TenantPolicy struct {
	// AppliesTo restricts the policy to specific message types. Leave
	// nil to apply to every message that crosses the bus.
	AppliesTo []string
	// AdminBypass allows messages carrying user_roles.admin to skip the
	// tenant check. Useful for system-level operations like the audit
	// reader. Default false — explicit admin tokens override only when
	// the host opts in.
	AdminBypass bool
}

// Evaluate enforces tenant presence and (when AppliesTo lists the type)
// match against the expected tenant.
func (p TenantPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	if !appliesTenant(p.AppliesTo, msg.Type) {
		return PolicyResult{Decision: DecisionAllow, Reason: "tenant policy not applicable", CheckedAt: time.Now().UTC()}, nil
	}

	tenant := metaString(msg.Metadata, "tenant_id")
	if tenant == "" {
		return denyTenant("missing tenant_id in metadata")
	}

	// Optional admin bypass: messages carrying admin in user_roles can
	// reach any tenant. Used for the audit reader and quarterly export.
	if p.AdminBypass {
		for _, r := range metaStringSlice(msg.Metadata, "user_roles") {
			if r == "admin" {
				return PolicyResult{Decision: DecisionAllow, Reason: "admin tenant bypass", CheckedAt: time.Now().UTC()}, nil
			}
		}
	}

	// Cross-tenant check: if the caller specified an expected_tenant
	// (typically set by the orchestrator after RBAC resolves the user),
	// it must match.
	if expected := metaString(msg.Metadata, "expected_tenant"); expected != "" && expected != tenant {
		return denyTenant("tenant mismatch: expected " + expected + " got " + tenant)
	}

	return PolicyResult{Decision: DecisionAllow, Reason: "tenant ok", CheckedAt: time.Now().UTC()}, nil
}

func appliesTenant(list []string, t string) bool {
	if len(list) == 0 {
		return true
	}
	for _, x := range list {
		if x == t {
			return true
		}
	}
	return false
}

func denyTenant(reason string) (PolicyResult, error) {
	return PolicyResult{
		Decision:    DecisionDeny,
		Reason:      reason,
		CheckedAt:   time.Now().UTC(),
		CheckedByID: "TenantPolicy",
	}, nil
}

func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func metaStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
