// tenant.go — tenant-isolation policy at the message bus.
//
// ─── What this file is ──────────────────────────────────────────────────────
//
// The bus-layer half of Genie's tenant-isolation defence in depth. Every
// message that crosses the in-memory bus is evaluated by a
// CompositePolicy; one of the policies in that composite is TenantPolicy,
// which:
//
//   1. Denies a message that doesn't carry metadata.tenant_id.
//   2. Denies a message whose metadata.expected_tenant doesn't match its
//      tenant_id (cross-tenant routing attempt).
//   3. Optionally allows admin role to bypass (off by default).
//
// The DB-layer half is pkg/storage/postgres + migrations/0005_rls.sql.
//
// ─── Defence in depth ──────────────────────────────────────────────────────
//
// Two layers, independent failure modes:
//
//   - This file (bus): runs inside the orchestrator before HandleMessage.
//     A bug here would let a bad message reach the agent.
//   - Postgres RLS: runs inside the database engine on every SELECT/
//     INSERT/UPDATE/DELETE.
//     A bug here would let a query return another tenant's rows.
//
// The two layers share no code and live in different processes (Go vs
// Postgres). A bug in either is contained by the other. The combined
// probability of both failing on the same vector is the product of the
// independent failure probabilities — much smaller than either alone.
//
// ─── In a single-tenant deployment ─────────────────────────────────────────
//
// Consumer-facing financial apps typically have one tenant = one user
// (Pratik's account is Pratik's tenant, regardless of how many devices
// he's logged in from). The tenant_id is just the user_id.
//
// ─── In a multi-tenant deployment ──────────────────────────────────────────
//
// Corporate banking has one tenant = one corporate customer (Acme Corp
// is a tenant; its 50 employees who can each log in are users within
// that tenant). The tenant_id is org_id; a user can belong to multiple
// orgs via the user_roles claim, and the orchestrator picks the right
// tenant per request.
//
// Today's data model uses the single-tenant shape (tenant_id = user_id).
// The org_id shape is the roadmap; the policy code already accepts a
// tenant id of any string shape, so the only thing the corporate path
// needs is for the orchestrator to set tenant_id = org_id in metadata.
//
// ─── FREE-AI alignment ─────────────────────────────────────────────────────
//
// Rec 15 (Data Lifecycle Governance) — the bus check is one half of the
// access-control story (the DB-layer RLS is the other half).
// Rec 22 (Tamper-Evident Audit) — denial events flow through the
// orchestrator's OnPolicyDeny hook into the audit chain; a reviewer can
// answer "did we ever attempt cross-tenant routing, and what was the
// message?"
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
// Two configuration knobs:
//
//   - AppliesTo: which message types this policy applies to. nil/empty
//     means "every message that crosses the bus." Most deployments name
//     the customer-facing message types (finance_question, kyc_submit,
//     payment_request) and leave system-level types (heartbeat, fallback)
//     unchecked.
//   - AdminBypass: whether the admin role is allowed to skip the tenant
//     check. Default false. Set true only for policy instances installed
//     on admin-only routes (audit reader, quarterly export).
//
// In a single-tenant deployment (most consumer-facing finance apps), the
// tenant is the user_id. In a multi-tenant deployment (corporate-banking
// where one tenant = one corporate customer), the tenant is the org_id,
// and a user can belong to multiple tenants via the user_roles claim.
type TenantPolicy struct {
	// AppliesTo restricts the policy to specific message types. Leave
	// nil to apply to every message that crosses the bus.
	//
	// Allow-list (this design) vs deny-list trade-off: allow-list means
	// the author of a new message type must explicitly opt in. They
	// might forget. Deny-list means the author of a new system type
	// must explicitly opt out. They might forget. We pick allow-list
	// because the failure mode of "new type forgot to opt in" is
	// "rejects all messages" (loud), and the failure mode of "new type
	// forgot to opt out" with deny-list would be "denies legitimate
	// internal traffic" (also loud). Either way you find out fast;
	// the choice favours the explicit-deny side as more conservative.
	AppliesTo []string

	// AdminBypass allows messages carrying user_roles.admin to skip the
	// tenant check. Useful for system-level operations like the audit
	// reader. Default false — explicit admin tokens override only when
	// the host opts in.
	//
	// Per-policy-instance, not global. The customer-facing route's
	// TenantPolicy{} has AdminBypass=false; the admin audit reader has
	// its own TenantPolicy{AdminBypass: true}. There is no global
	// "admins bypass everything" mode by design.
	AdminBypass bool
}

// Evaluate enforces tenant presence and (when AppliesTo lists the type)
// match against the expected tenant.
//
// The decision flow:
//
//   if type ∉ AppliesTo (when set)     → ALLOW (policy not applicable)
//   if tenant_id missing / empty       → DENY (missing tenant_id in metadata)
//   if AdminBypass && role.admin       → ALLOW (admin tenant bypass)
//   if expected_tenant ≠ tenant_id     → DENY (tenant mismatch: a got b)
//   otherwise                          → ALLOW (tenant ok)
//
// The denial reason includes both the expected and the got tenant ids so
// the on-call operator can tell at a glance whether the bug is a missing
// claim, a stale token, or an actual cross-tenant attack.
//
// The ctx is currently unused — there's nothing in the request context
// the policy decision depends on. It's kept in the signature because the
// governance.Policy interface requires it, and a future policy might
// need it (e.g. to query a remote consent service).
func (p TenantPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	// Short-circuit: if the policy has an AppliesTo allowlist and this
	// message type isn't on it, we allow. This is how system-level
	// types (fallback_request, heartbeat) avoid being denied for not
	// carrying a tenant — they wouldn't have one.
	if !appliesTenant(p.AppliesTo, msg.Type) {
		return PolicyResult{Decision: DecisionAllow, Reason: "tenant policy not applicable", CheckedAt: time.Now().UTC()}, nil
	}

	// The tenant id must be present. Read from metadata.tenant_id — the
	// HTTP intake (pkg/web/handlers/ask.go etc.) populates this from
	// claims.Subject before publishing onto the bus. If it's missing,
	// either the handler forgot or the message was injected by some
	// other code path; either way, deny.
	tenant := metaString(msg.Metadata, "tenant_id")
	if tenant == "" {
		return denyTenant("missing tenant_id in metadata")
	}

	// Optional admin bypass: messages carrying admin in user_roles can
	// reach any tenant. Used for the audit reader and quarterly export.
	//
	// This branch is opt-in per policy instance. The customer-facing
	// route has AdminBypass=false; even a token with the admin role
	// cannot cross tenants on that route. Only the admin-only routes
	// install a policy instance with AdminBypass=true.
	if p.AdminBypass {
		// metaStringSlice handles both []string and []any so a JSON
		// round-trip through the bus doesn't break the role lookup.
		for _, r := range metaStringSlice(msg.Metadata, "user_roles") {
			if r == "admin" {
				return PolicyResult{Decision: DecisionAllow, Reason: "admin tenant bypass", CheckedAt: time.Now().UTC()}, nil
			}
		}
	}

	// Cross-tenant check: if the caller specified an expected_tenant
	// (typically set by the orchestrator after RBAC resolves the user),
	// it must match.
	//
	// expected_tenant is the "what the routing layer thinks this message
	// should be for." If it's empty (not set), we don't have an opinion
	// to enforce — the message carries a tenant_id and is internally
	// consistent. If it's set and doesn't match the tenant_id, that's a
	// confused-deputy attempt: the caller has authenticated as one
	// tenant but is trying to act on another.
	if expected := metaString(msg.Metadata, "expected_tenant"); expected != "" && expected != tenant {
		// The reason includes both ids so the on-call engineer triaging
		// a metric tag can see at a glance: was it a stale token? A bug
		// in the orchestrator? An actual attack? The exact strings help
		// the diagnosis.
		return denyTenant("tenant mismatch: expected " + expected + " got " + tenant)
	}

	// All checks passed. Allow.
	return PolicyResult{Decision: DecisionAllow, Reason: "tenant ok", CheckedAt: time.Now().UTC()}, nil
}

// appliesTenant reports whether the policy applies to a given message
// type. Empty list means "applies to every message" — that's the most
// paranoid setting and the right default for environments where every
// message must carry tenant.
func appliesTenant(list []string, t string) bool {
	// nil or empty list → applies to everything.
	if len(list) == 0 {
		return true
	}
	// Otherwise linear scan. The list is short (typically 3-5 types) so
	// a map would be overkill; a linear scan with early return is
	// faster in practice for these sizes.
	for _, x := range list {
		if x == t {
			return true
		}
	}
	return false
}

// denyTenant builds a deny result with the TenantPolicy attribution.
// Stamping CheckedByID = "TenantPolicy" so the audit log can attribute
// the denial to this specific policy rather than the composite.
func denyTenant(reason string) (PolicyResult, error) {
	return PolicyResult{
		Decision:    DecisionDeny,
		Reason:      reason,
		CheckedAt:   time.Now().UTC(),
		CheckedByID: "TenantPolicy",
	}, nil
}

// metaString safely reads a string value out of message metadata.
// Returns the empty string if the metadata is nil, the key is missing,
// or the value isn't a string.
//
// Why "empty string" instead of "(value, ok)": every caller in this file
// treats empty as "not present" semantically, so the two-return-value
// signature would force every caller to write the same `if !ok || s == ""`
// boilerplate. Hoist it into the helper.
func metaString(m map[string]any, key string) string {
	// Nil-safe: a message with no metadata at all gives us empty.
	if m == nil {
		return ""
	}
	// Missing key → empty.
	v, ok := m[key]
	if !ok {
		return ""
	}
	// Type-assert. If the metadata value happens to be a non-string
	// (e.g. an int because some handler set it wrong), we return empty
	// rather than coerce. Coercion would hide the bug; returning empty
	// fails the tenant check, which surfaces the bug as a policy denial.
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// metaStringSlice safely reads a string slice out of message metadata.
// Handles three shapes:
//
//   - []string — the native Go shape when metadata was constructed in
//     this process.
//   - []any — the shape after a JSON round-trip. encoding/json cannot
//     recover the original element type from the wire bytes alone, so
//     any list becomes []any with each element typed individually.
//   - anything else — returns nil (no roles → no bypass).
//
// Elements of []any that aren't strings (e.g. a stray int) are silently
// dropped. The test TestMetaStringSliceHandlesAnySlice pins this
// behaviour — a future refactor that "tightens" the type assertion
// would break real production messages whose JSON intake stage emits
// []any.
//
// Why coerce instead of error: an attacker-controlled JSON could inject
// non-string elements into the user_roles array. Dropping them is the
// safe choice — the worst case is "admin role is dropped, fewer
// bypasses" which is fail-closed.
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
		// Native Go shape — pass through directly.
		return x
	case []any:
		// JSON round-trip shape — coerce element by element.
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
			// Silent drop of non-string elements (see doc comment).
		}
		return out
	}
	// Some other shape (single string, map, number) — we don't know
	// how to coerce, return nil. Fail-closed: no roles means no bypass.
	return nil
}
