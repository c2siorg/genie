# pkg/auth/elevation — Privileged Access Manager analog

> **Where:** `pkg/auth/elevation/elevation.go`
> **Lines of code:** ~430 (Go) + ~430 (tests) · **Tests:** 22 unit + 11 HTTP contract
> **PCSE alignment:** §1.4 (Privileged Access Manager)
> **FREE-AI alignment:** Rec 14 (Board-approved policy), Rec 22 (Tamper-evident audit)

---

## Overview

Time-bound privileged access — the GCP PCSE §1.4 *Privileged Access
Manager* analog for Genie. Solves the problem where some operations
need admin role for a short window (incident investigation, ad-hoc
data migration, rare customer edge case), but granting permanent
admin to every engineer who might ever need that is the bad option.

The pattern: **request → approve → window-of-time → automatic expiry**,
with every transition in the hash-chained audit log. After expiry the
role is gone; no daemon to forget to run, no cron to mis-fire. Lazy
expiry checked on read.

Pairs with `pkg/auth.RoleAdmin` (base role model), `pkg/compliance.AuditLog`
(audit chain), and the HTTP middleware that gates routes by effective
roles.

---

## Surface

```go
// pkg/auth/elevation/elevation.go

type Service struct {
    Audit            compliance.AuditLog   // required
    MaxDuration      time.Duration         // default 4h
    RequireApprovers int                   // default 1 ; 2 for 4-eyes
    ElevatableRoles  []auth.Role           // default [RoleAdmin]
    Now              func() time.Time      // injectable clock
}

func New(audit compliance.AuditLog) *Service

// Lifecycle
func (s *Service) Request(ctx, subject, role, reason, ttl) (*Grant, error)
func (s *Service) Approve(ctx, grantID, approverClaims) error
func (s *Service) Deny(ctx, grantID, denierClaims, reason) error
func (s *Service) Revoke(ctx, grantID, revokerClaims, reason) error

// Read
func (s *Service) Get(ctx, grantID) (Grant, error)
func (s *Service) ActiveFor(ctx, subject) []Grant
func (s *Service) List(ctx, limit) []Grant
func (s *Service) EffectiveRoles(ctx, claims) []auth.Role
```

The `Grant` struct carries the full lifecycle state: subject, role,
reason, requested TTL, approvers, approved/expired/revoked/denied
timestamps, status, and the genesis audit entry id for trace-back.

---

## State machine

```
              Request                  Approve(s reach N)
   (nothing) ─────────► Pending ───────────────────────► Active
                          │                                 │
                          │ Deny                            │ Revoke (admin)
                          ▼                                 ▼
                       Denied                           Revoked

                                  past ExpiresAt
                          Active ───────────────► Expired
                                  (lazy, on read)
```

All five terminal states are durable; nothing transitions back. A
denied request can be re-submitted as a new request (new id).

---

## Lifecycle in detail

### Request

```go
g, err := svc.Request(ctx, "alice", auth.RoleAdmin, "investigate ticket #1234", time.Hour)
```

Validations:
- `subject` must be non-empty
- `reason` must be non-empty (the audit log is useless without it)
- `role` must be in `ElevatableRoles` (default: just `RoleAdmin`)
- `ttl > 0 && ttl ≤ MaxDuration` (default cap: 4h)

Side effects:
- Writes one `elevation.request` entry to the audit log
- Stores the grant in `StatusPending` (or `StatusActive` if
  `RequireApprovers == 0`, the dev/test mode)

### Approve

```go
err := svc.Approve(ctx, grantID, approverClaims)
```

Validations:
- Approver must hold `auth.RoleAdmin`
- Approver must NOT be the subject (4-eyes minimum)
- Grant must be in `StatusPending`
- Same approver cannot approve twice (N-eyes integrity)

Side effects:
- Appends approver subject to `Grant.Approvers`
- Writes one `elevation.approve` entry per call
- When approver count reaches `RequireApprovers`, transitions to
  `StatusActive`, sets `ApprovedAt` and `ExpiresAt = ApprovedAt + RequestedTTL`,
  and writes a separate `elevation.activate` entry (cleaner timeline
  query)

### Deny / Revoke

Symmetric admin-gated termination. Deny terminates a Pending grant
(denier didn't approve); Revoke terminates an Active grant (something
changed). Both require a non-empty reason which flows into the audit
entry as `Details.reason`.

### Lazy expiry

No background goroutine. Every `Get`/`ActiveFor`/`List` checks
`now > ExpiresAt` and transitions to `StatusExpired` on observation,
writing one `elevation.expire` entry. Idempotent — subsequent reads
do not duplicate the entry.

Trade-off: a grant that nobody reads after expiry stays as `Active` in
the in-memory map until someone reads it. That's harmless — `Active`
without an effective-role read changes nothing about authorisation —
and avoids a separate timer.

---

## Effective roles

The read path the HTTP middleware calls:

```go
effective := svc.EffectiveRoles(ctx, claims)
if !hasRoleIn(effective, requiredRole) {
    return 403
}
```

`EffectiveRoles` returns the union of:
- `claims.Roles` (base roles from the JWT)
- The role from every active elevation grant for `claims.Subject`

De-duplicated — a user already holding the role doesn't get it twice.
Expired/Revoked/Denied grants contribute nothing.

---

## HTTP surface

Routes wired by `pkg/web/router.go`:

| Method + path | Auth gate | Purpose |
|---|---|---|
| `POST /v1/elevation/requests` | authenticated user | File a request |
| `GET /v1/elevation/requests` | admin (router) | List recent (limit ≤ 50) |
| `GET /v1/elevation/requests/{id}` | subject OR admin (handler) | Read one |
| `POST /v1/elevation/requests/{id}/approve` | admin (router) | Approve |
| `POST /v1/elevation/requests/{id}/deny` | admin (router) | Deny (reason required) |
| `POST /v1/elevation/requests/{id}/revoke` | admin (router) | Revoke (reason required) |

The handler does sentinel-to-status mapping:

| Service error | HTTP status |
|---|---|
| `ErrSubjectRequired`, `ErrReasonRequired`, `ErrRoleNotElevatable`, `ErrTTLOutOfRange`, `ErrApproverIsSubject`, `ErrDuplicateApprover` | 400 |
| `ErrApproverIneligible` | 403 |
| `ErrGrantNotFound` | 404 |
| `ErrGrantNotPending`, `ErrGrantNotActive` | 409 |
| anything else | 500 |

`Get` returns 404 (not 403) for an unauthorised reader — refusing to
confirm existence is the right move for admin-flavoured resources.

---

## Audit chain integration

Every transition writes one entry to the `compliance.AuditLog`. The
schema:

| Action | Actor | Target | Details keys |
|---|---|---|---|
| `elevation.request` | subject | role | grant_id, reason, requested_ttl |
| `elevation.approve` | approver | grant id | grant_id, subject, role, approver_n, approvers, audit_root |
| `elevation.activate` | last approver | grant id | grant_id, subject, role, expires_at, audit_root |
| `elevation.deny` | denier | grant id | grant_id, subject, role, reason, audit_root |
| `elevation.revoke` | revoker | grant id | grant_id, subject, role, reason, audit_root |
| `elevation.expire` | `"system"` | grant id | grant_id, subject, role, expired_at, audit_root |

Every non-request entry references `audit_root` (the genesis
`AuditEntryID` from the original request). A reviewer can pull the
full per-grant thread by filtering on `audit_root` or by walking
forward from the genesis entry.

---

## Defence in depth at the handler boundary

The router gates `/approve`, `/deny`, `/revoke`, `/list` with
`mid.RequireRole(auth.RoleAdmin)`. The service ALSO checks
`approver.HasRole(auth.RoleAdmin)` on every transition. Either layer
alone is sufficient. Both layers make regression noisy: a refactor
that drops the router gate still gets a 403 from the service; a
refactor that drops the service gate still gets a 403 from the
router.

A UI contract test (in roadmap) would pin that the elevation panel
is admin-only and the request form is open to authenticated users.

---

## What this package does *not* do

- **It doesn't mint new JWTs.** EffectiveRoles is the read path used
  by the HTTP middleware and the bus policy stack on every request.
  A future iteration could mint a short-lived JWT with the elevated
  role baked in (cleaner audit attribution, but loses the revoke-
  before-expiry capability).
- **It doesn't enforce a maximum number of active grants per user.**
  A user with three concurrent active grants for the same role just
  has three audit threads; effective-roles de-duplicates. If a
  per-user cap is needed for a regulatory rule, add a counter check
  in `Request`.
- **It doesn't model role hierarchies.** Today only `RoleAdmin` is in
  the default `ElevatableRoles`. If your role taxonomy grows to
  include super-admin, add the role to the list and add a check that
  the approver holds a strictly-higher tier than the requested one.
- **It doesn't persist across restarts.** In-memory map. A
  `PostgresStore` implementation is the next iteration once the
  product needs grants to survive a deploy.

---

## Tests

`pkg/auth/elevation/elevation_test.go` (22 tests):

| Group | Tests |
|---|---|
| Request | empty subject, empty reason, non-allowlist role, TTL out of range (zero / negative / above cap), happy path with audit |
| Approve | non-admin approver, self-approval, unknown grant, single-approver happy path, N-eyes pending then activate, duplicate approver, wrong-status |
| Deny | non-admin denier, happy path, wrong-status |
| Revoke | non-admin revoker, happy path, not-active |
| Lazy expiry | transitions on read; expire entry written exactly once |
| EffectiveRoles | no elevation, active grant unions, expired does not add, de-dup |
| ActiveFor / List | scope by subject, list respects limit |

`pkg/web/handlers/elevation_test.go` (11 tests):

| Group | Tests |
|---|---|
| POST request | happy 201, empty reason → 400, TTL above max → 400, no auth → 401 |
| Approve | happy 200, non-admin → 403 |
| Deny | reason required → 400, happy → 200 |
| Revoke | reason required → 400 |
| Get | subject can read, admin can read, stranger gets 404 with no subject id in body |

---

## Wiring (cmd/api)

```go
// cmd/api/main.go
auditLog := compliance.NewInMemoryAuditLog()
elevationSvc := elevation.New(auditLog)
// ...
deps := web.Deps{
    // ... existing fields ...
    Elevation: &handlers.Elevation{Service: elevationSvc},
}
```

For a production deployment that wants 4-eyes:

```go
elevationSvc := elevation.New(auditLog)
elevationSvc.RequireApprovers = 2
elevationSvc.MaxDuration = 1 * time.Hour  // tighter cap
```

---

## FREE-AI alignment

- **Rec 14 — Board-approved policy.** The thresholds (`MaxDuration`,
  `RequireApprovers`, `ElevatableRoles`) live in code today; the next
  iteration moves them to the policy YAML so the risk team owns them
  without an engineering release.
- **Rec 22 — Tamper-evident audit.** Every state transition writes to
  the hash-chained audit log. The `audit_root` field on every
  non-request entry threads the per-grant timeline.

---

## PCSE alignment

PCSE §1.4 lists *"Identifying use cases and configuring Privileged
Access Manager"* as a tested skill. This package is the application-
layer analog: time-bound elevation with multi-approver workflow and
audit chain.

The map at [`docs/gcp-pcse-mapping.md`](../gcp-pcse-mapping.md) ties
every PCSE bullet to a Genie file path.

---

## Pointers

- Implementation: `pkg/auth/elevation/elevation.go`
- HTTP handler: `pkg/web/handlers/elevation.go`
- Tests: `pkg/auth/elevation/elevation_test.go`,
  `pkg/web/handlers/elevation_test.go`
- Router wire-up: `pkg/web/router.go` (under `/v1/elevation/*`)
- Audit log: `pkg/compliance/audit.go`
- Related: [`oauth-token-exchange.md`](oauth-token-exchange.md) — RFC
  8693 dual-identity tokens; [`postgres-rls.md`](postgres-rls.md) —
  database-layer admin sentinel
- PCSE map: [`../gcp-pcse-mapping.md`](../gcp-pcse-mapping.md)
- Security ref: [`../ai-governance-security.md`](../ai-governance-security.md)
