# pkg/auth/tokenexchange — OAuth 2.0 Token Exchange (RFC 8693)

> **Where:** `pkg/auth/tokenexchange/exchange.go`
> **Lines of code:** ~250 · **Tests:** 7 unit + 2 integration in `tests/security_envelope_test.go`
> **FREE-AI alignment:** Rec 22 (Tamper-Evident Audit), Rec 14 (Board-Approved Policy — who can act for whom)

---

## Overview

When an agent calls an MCP server (or that MCP server calls an
upstream API), the downstream service has historically had two bad
choices:

1. **See only the user** — receive the user's first-party token.
   Audit trail says "user did X." Loses the fact that an automated
   agent was the actual originator and the user only triggered the
   chain by asking a question. If the agent malfunctioned, the
   forensic record blames the user.
2. **See only the agent** — receive a service-to-service token. Audit
   says "kyc_orchestrator service called X." Loses the user — every
   audit row collapses onto the same service identity and you can't
   answer "which user's request caused this row."

This package implements the third option from RFC 8693:
**dual-identity tokens**. The `Subject` claim stays the user; an
`Actor` (`act`) claim records the agent currently acting on the
user's behalf. Both identities are signed into the same JWT, so the
downstream service can write a single audit row that answers both
"who initiated" and "what was the proximate caller."

Two-hop and N-hop chains are supported via `Actor.Nested`. The
medical-assistant pattern from the FREE-AI report — nurse → agent →
MCP server → upstream API — produces a token with `Subject = nurse`,
`Actor = MCP server`, and `Actor.Nested = agent`. Audit can walk the
chain.

---

## Surface

```go
type Request struct {
    SubjectToken string // user's JWT, or a previously-exchanged token
    ActorID      string // stable id of the currently-acting service
    Audience     string // identifier of the target service the new token is for
}

type Service struct {
    Verifier        interface{ Verify(token string) (auth.Claims, error) }
    Minter          *auth.Issuer
    ServiceIdentity string        // recorded in Actor.Issuer
    SafetyMargin    time.Duration // shaves cache TTL; default 60s
}

func New(issuer *auth.Issuer, serviceIdentity string) *Service

func (s *Service) Exchange(ctx context.Context, req Request) (string, auth.Claims, error)
func (s *Service) Invalidate(userSubject string)
func (s *Service) CacheSize() int   // test helper
```

The `Verifier` interface is satisfied by `*auth.Issuer` (via
`VerifyIgnoringAudience`) wrapped in the package's internal
`looseVerifier`. In a federated deployment, replace the verifier with
one wired to the user's IdP.

---

## Wire flow

```
1. User signs in to Genie ────► issuer.Issue() ─► first-party JWT (Subject = user)
2. Agent runtime gets the JWT, prepares to call MCP server
3. svc.Exchange({SubjectToken: userJWT, ActorID: "kyc_orchestrator", Audience: "mcp://kyc-server"})
       ├─ verify userJWT (audience-skipping verify for nested flows)
       ├─ check cache for (user_sub, actor_id, audience) tuple
       ├─ if cache hit: return cached token
       └─ else: issuer.IssueWithActor(user_sub, email, roles, [audience], &Actor{Subject: actor_id, Issuer: serviceIdentity, Nested: prev.Actor})
              → new JWT with Subject = user, Actor = agent
              → cache it on (user_sub, actor_id, audience)
4. Agent calls MCP server with the new JWT
5. (Optional) MCP server runs its own exchange to extend the chain
```

The exchanged token's expiry is `min(minter_default_ttl, subject_token_remaining_ttl)` — an exchanged token must not outlive the underlying user session.

---

## Nested actor chains

A two-hop scenario (`tests/security_envelope_test.go::TestSecurityEnvelope_TokenExchangeAuditIdentity`):

```go
hop1, _, _ := svc.Exchange(ctx, Request{
    SubjectToken: userJWT,
    ActorID:      "kyc_orchestrator",
    Audience:     "mcp://kyc-server",
})
// hop1.Subject = user-alice
// hop1.Actor   = {Subject: kyc_orchestrator, Issuer: genie-api}

_, claims, _ := svc.Exchange(ctx, Request{
    SubjectToken: hop1,
    ActorID:      "kyc-mcp-server",
    Audience:     "https://api.upstream/records",
})
// claims.Subject              = user-alice          ← unchanged
// claims.Actor.Subject        = kyc-mcp-server      ← outermost
// claims.Actor.Nested.Subject = kyc_orchestrator    ← inner hop
```

Walking the chain from `claims.Actor` outward yields the full audit
trail. A downstream auditor reading the final token can answer "this
request was initiated by user-alice, flowed through kyc_orchestrator,
and the proximate caller was kyc-mcp-server."

---

## Caching

The cache key is the **(user_subject, actor_id, audience) tuple** —
not the input token. Two callers that present different subject tokens
for the same user (e.g. token rotation, multiple devices) share the
cached exchanged token.

| Cache decision | Why |
|---|---|
| Cache TTL = `min(token_exp, subject_exp) − SafetyMargin` | Never serve a stale token. SafetyMargin (default 60s) absorbs clock skew and network latency. |
| Eviction = lazy on read | An expired entry is deleted on the next `get()` call — no background goroutine. |
| Cache key includes audience | Prevents confused-deputy: a token cached for `mcp://kyc-server` cannot be served when the caller asks for `https://api.upstream/records`. |
| Cache key includes actor id | A different service hopping the chain produces a different `Actor.Subject` and therefore a different token. |
| `Invalidate(userSubject)` clears all entries for the user | On logout / password change — every exchanged token derived from the burned session must be unreachable. |

---

## Why a "loose verifier"

A first-party token from the issuer carries the issuer's default
audience (`["genie-api"]`). The first hop's exchanged token carries
the requested audience (e.g. `mcp://kyc-server`). When the MCP server
wants to exchange again for an upstream call, the input is the
first-hop token — its audience is `mcp://kyc-server`, not the
verifier's default.

A strict audience check on the input would reject it. The package
wraps the issuer in `looseVerifier`, which calls
`Issuer.VerifyIgnoringAudience`. The output audience is the new one
the caller asks for, which is the audience the *downstream* service
will then verify strictly.

This is safe because:

- The signature is still checked (only the audience claim is skipped).
- The expiry is still checked.
- The issuer claim is still checked.
- The output audience is set explicitly by the caller — there's no
  "anyone" audience floating around.

---

## Failure modes

| Input | Result |
|---|---|
| Empty `Audience` | `ErrAudienceRequired` |
| Empty `SubjectToken` | `ErrSubjectTokenInvalid` |
| Garbled or signature-mismatched token | `ErrSubjectTokenInvalid` wrapping the underlying parse error |
| Expired subject token | `ErrSubjectTokenInvalid` — the cache rejects expired entries even before this, but a fresh expired token comes here |
| Issuer/audience mismatch beyond the loose-verifier waiver | `ErrSubjectTokenInvalid` |

---

## What this package does *not* do

- **No introspection endpoint** — RFC 8693 over the wire (the standard
  HTTP `/token` exchange endpoint) is out of scope. Today the Service
  is used as a Go-level helper; a future HTTP wrapper can adapt it.
- **No mTLS or proof-of-possession** — exchanged tokens are bearer
  tokens. The `pkg/identity` SPIFFE/mTLS layer handles
  workload-identity assertions at the transport level. Combine them
  for high-risk paths.
- **No revocation list** — invalidation is local to the Service's
  cache. A token already minted will remain accepted by downstream
  verifiers until expiry. Short TTLs are the primary defence.
- **No rotation of the signing key** — handled at the `pkg/auth.Issuer`
  level; this package consumes the issuer's current key.

---

## Tests

`pkg/auth/tokenexchange/exchange_test.go` covers:

| Test | Asserts |
|---|---|
| `TestExchangePreservesUserAddsActor` | Subject unchanged, Actor records agent + issuer, audience scoped |
| `TestExchangeRejectsMissingAudience` | `ErrAudienceRequired` |
| `TestExchangeRejectsInvalidSubjectToken` | Garbage token → error |
| `TestExchangeRejectsExpiredSubjectToken` | Already-expired user token → error |
| `TestExchangeCachesByTuple` | Same (user, actor, aud) returns the cached token; different audience or actor mints fresh |
| `TestInvalidateClearsCacheForUser` | `Invalidate` empties cache, next call re-mints |
| `TestNestedActorChain` | Two-hop chain preserves user as Subject and stacks Actor.Nested |

`tests/security_envelope_test.go` adds:

| Test | Asserts |
|---|---|
| `TestSecurityEnvelope_TokenExchangeAuditIdentity` | End-to-end two-hop audit chain reconstruction |
| `TestSecurityEnvelope_InvalidatePropagates` | Cache is empty after Invalidate, even after a previous Exchange populated it |

---

## FREE-AI mapping

- **Rec 22 — Tamper-Evident Audit.** Dual-identity tokens are the
  source of the audit record's "who" and "via what" columns. The
  hash-chained audit log records the claims; the chain lets the
  reviewer reconstruct N-hop attribution from the final token alone.
- **Rec 14 — Board-Approved Policy.** Policies can be expressed as
  "user has permission X AND actor is authorised for operation Y" —
  fine-grained dual-condition gates that aren't possible with a
  single-identity token.

---

## Pointers

- Implementation: `pkg/auth/tokenexchange/exchange.go`
- Issuer helpers: `pkg/auth/jwt.go` — `IssueWithActor`, `VerifyIgnoringAudience`
- Tests: `pkg/auth/tokenexchange/exchange_test.go`,
  `tests/security_envelope_test.go`
- Related: `pkg/identity` (SPIFFE/mTLS), `pkg/audit` (chain log)
- Operations: `docs/operations.md` — token-exchange wiring section
