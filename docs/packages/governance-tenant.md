# pkg/governance.TenantPolicy — bus-layer tenant isolation

> **Where:** `pkg/governance/tenant.go`
> **Lines of code:** ~130 · **Tests:** 7 unit + integration coverage in `tests/security_envelope_test.go`
> **FREE-AI alignment:** Rec 15 (Data Lifecycle Governance), Rec 22 (Tamper-Evident Audit)

---

## Overview

The bus-layer half of Genie's tenant-isolation defence in depth. Every
message that crosses the in-memory bus is evaluated by the
`CompositePolicy`; one of the policies in that composite is
`TenantPolicy`, which denies a message that doesn't carry a
`tenant_id` in its metadata, and denies a message whose
`expected_tenant` doesn't match its `tenant_id`.

The DB-layer half is `pkg/storage/postgres` + the 0005_rls.sql
migration. Together they implement the "two pairs of eyes" rule:
a missed check on either side is caught by the other.

The policy is small on purpose. Cross-tenant message routing is a
boolean question — does the metadata match? — and the cost of a
fancy policy is more places for bugs to hide.

---

## Surface

```go
type TenantPolicy struct {
    // AppliesTo restricts the policy to specific message types. Leave
    // nil to apply to every message that crosses the bus.
    AppliesTo []string
    // AdminBypass allows messages carrying user_roles.admin to skip
    // the tenant check. Used for the audit reader and quarterly export.
    // Default false — must be explicitly opted in.
    AdminBypass bool
}

func (p TenantPolicy) Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error)
```

The policy implements the `governance.Policy` interface, so it slots
into the existing `CompositePolicy` stack:

```go
policy := governance.NewComposite(
    governance.RBACPolicy{...},
    governance.TenantPolicy{
        AppliesTo: []string{"finance_question", "kyc_submit", "payment_request"},
    },
    governance.MaxContentLengthPolicy{Max: 16 * 1024},
)
```

---

## Decision flow

```
msg arrives at orchestrator
    │
    ▼
TenantPolicy.Evaluate(ctx, msg)
    │
    ├─ msg.Type not in AppliesTo (when set)? ─► ALLOW (policy not applicable)
    │
    ├─ msg.Metadata["tenant_id"] missing or empty? ─► DENY (missing tenant_id in metadata)
    │
    ├─ AdminBypass && user_roles contains "admin"? ─► ALLOW (admin tenant bypass)
    │
    ├─ msg.Metadata["expected_tenant"] set AND != tenant_id? ─► DENY (tenant mismatch: …)
    │
    └─ otherwise ─► ALLOW (tenant ok)
```

The denial reason includes the expected/got tenant ids so an on-call
engineer triaging a denial can read the metric tag and tell instantly
whether the bug is a missing claim, a stale token, or an actual
cross-tenant routing attempt.

---

## The AppliesTo filter

Some message types legitimately don't carry a tenant — system-level
events, broadcast notifications, the orchestrator's own
`fallback_request`. Two ways to handle this:

1. **Allow-list every type that does need tenant** (`AppliesTo` is set)
   — explicit, hard-to-bypass; the new-type author must add it
   intentionally.
2. **Deny-list types that don't need tenant** — implicit, easier to
   forget.

Genie picks option 1. The host explicitly lists `finance_question`,
`kyc_submit`, `payment_request`, etc. A new message type that
should carry tenant isn't checked until someone adds it to the list,
which is the same kind of review every other governance change goes
through.

When `AppliesTo` is nil or empty, the policy applies to every
message — the most paranoid setting, suitable for environments where
"every message carries a tenant" is invariant.

---

## The AdminBypass opt-in

Some system-level operations legitimately need cross-tenant reads:

- The audit reader inspects events across tenants.
- The quarterly RBI export aggregates incidents across all tenants.
- The on-call admin investigating a leak needs to read entries that
  don't belong to their own user record.

The naïve approach is "if user has the admin role, skip the tenant
check." That breaks down at policy time: the admin role is held by
multiple humans, and not every admin action should bypass tenant
isolation. The `AdminBypass` flag is **per-policy-instance**: the
host wires one TenantPolicy with `AdminBypass: false` for normal
customer traffic, and a separate stack with `AdminBypass: true` for
the admin-only routes.

```go
// customer-facing route
customerPolicy := governance.NewComposite(
    governance.RBACPolicy{...},
    governance.TenantPolicy{AppliesTo: customerTypes},  // bypass off
    ...
)

// admin-only route, e.g. the audit reader
adminPolicy := governance.NewComposite(
    governance.RBACPolicy{RequiredRolesByType: adminTypes, AdminBypass: true},
    governance.TenantPolicy{AppliesTo: adminTypes, AdminBypass: true},
)
```

The test `TestTenantPolicyAdminBypassRequiresOptIn` pins the
default-off behaviour — a code change that silently flips the
default would fail the test.

---

## metaString and metaStringSlice helpers

Messages crossing the bus go through JSON when they cross process
boundaries (the HTTP intake, the BigQuery sink, the audit log).
After a JSON round-trip, a `[]string` becomes `[]any` — Go's
`encoding/json` can't recover the original element type from the
wire bytes alone.

`metaStringSlice` accepts both:

```go
switch x := v.(type) {
case []string:        return x
case []any:           coerce each element via type-assert to string
}
```

Elements that aren't strings (e.g. a stray int) are dropped silently.
The test `TestMetaStringSliceHandlesAnySlice` pins this behaviour —
a future refactor that "tightens" the type assertion would break
real production messages whose JSON intake stage emits `[]any`.

---

## Defence in depth

| Layer | Catches |
|---|---|
| **HTTP middleware** (`pkg/web/mid`) | Unauthenticated request, missing claims, expired JWT |
| **Bus — TenantPolicy** (this) | Cross-tenant message dispatch, missing tenant_id |
| **DB — RLS** (`pkg/storage/postgres` + `0005_rls.sql`) | Cross-tenant SQL read/write |

A bug in any one layer is contained by the others. The bus layer
runs before the agent's `HandleMessage`, so a denial never reaches
the agent's query path; even if it did, RLS would block the read.

The other Q1 hardening primitives layer alongside:

- **`pkg/auth/tokenexchange`** — even within a single tenant,
  dual-identity tokens record which agent acted on the user's behalf.
- **`pkg/agent.Tier`** — only production-tier agents are eligible
  for the customer-facing dispatch path that TenantPolicy guards.

---

## What this package does *not* do

- **It doesn't validate the JWT.** That's `pkg/web/mid.RequireAuth`.
  The policy assumes the metadata it reads was populated by the
  authenticated middleware.
- **It doesn't compute the expected_tenant.** The orchestrator (or
  the HTTP handler that prepares the message) sets
  `expected_tenant` after resolving the route's intended target.
  Typically: "this is a `payment_request` for user X, so
  expected_tenant = X."
- **It doesn't filter database rows.** That's RLS. The bus check
  refuses to dispatch the message; the DB check refuses to return
  rows.
- **It doesn't enforce multi-tenant org_id semantics.** Today
  `tenant_id` is the user_id in consumer banking. Corporate banking
  (one org = many users) needs the orchestrator to set
  `tenant_id = org_id` on the message and the user-roles claim to
  carry which orgs the user belongs to.

---

## Tests

`pkg/governance/tenant_test.go` covers:

| Test | Asserts |
|---|---|
| `TestTenantPolicyDeniesMissingTenant` | Empty/absent tenant_id → deny |
| `TestTenantPolicyAllowsMatchingTenant` | tenant_id == expected_tenant → allow |
| `TestTenantPolicyDeniesMismatch` | tenant_id != expected_tenant → deny |
| `TestTenantPolicyAppliesToFilter` | Message type not in AppliesTo passes regardless |
| `TestTenantPolicyAdminBypass` | Admin role bypasses cross-tenant when AdminBypass=true |
| `TestTenantPolicyAdminBypassRequiresOptIn` | Admin role does NOT bypass when AdminBypass=false |
| `TestMetaStringSliceHandlesAnySlice` | `[]any` from JSON round-trip is coerced to `[]string` |

Integration:
- `tests/security_envelope_test.go::TestSecurityEnvelope_MissingTenantIsBlocked`
- `tests/security_envelope_test.go::TestSecurityEnvelope_CrossTenantIsBlocked`
- `tests/security_envelope_test.go::TestSecurityEnvelope_HappyPath`
- `tests/security_envelope_test.go::TestSecurityEnvelope_FallbackTriggers` — confirms the fallback path also preserves tenant metadata

---

## FREE-AI mapping

- **Rec 15 — Data Lifecycle Governance.** The bus check is one half
  of the access-control story (the DB-layer RLS is the other half).
- **Rec 22 — Tamper-Evident Audit.** Denial events are emitted via
  the orchestrator's `OnPolicyDeny` hook → metrics + structured log
  → audit chain. A reviewer can answer "did we ever attempt
  cross-tenant routing, and what was the message?"

---

## Pointers

- Implementation: `pkg/governance/tenant.go`
- Tests: `pkg/governance/tenant_test.go`,
  `tests/security_envelope_test.go`
- DB-layer counterpart: [postgres-rls.md](postgres-rls.md)
- Composite policy: `pkg/governance/policy.go`
- RBAC sibling: `pkg/governance/rbac.go`
