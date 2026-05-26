# AI Governance & Security — Genie Reference

> **Audience:** CISO, risk officer, security architect, platform engineer.
> **Purpose:** the single canonical document covering how Genie defends a
> production financial-assistant deployment. Every claim is tied to a
> file path, a function name, or a test name in this repository. If you
> find a paragraph that isn't anchored in real code, treat it as a bug
> and open an issue.

The goal of this document is to let a reviewer read once, then verify
once, then ship. Each section starts with the contract (what the layer
promises), gives the implementation (file paths + function names),
explains the design choice (why this approach over the obvious
alternatives), states the failure modes (what breaks the contract),
and ends with the tests that prove the contract.

---

## Table of contents

1. [The mental model — defence in depth, not defence in line](#1-the-mental-model)
2. [Threat model](#2-threat-model)
3. [Identity layer — who the caller is](#3-identity-layer)
4. [Authentication](#4-authentication)
5. [Authorisation — RBAC, tier, policy stack](#5-authorisation)
6. [Tenant isolation — bus + database](#6-tenant-isolation)
7. [Dual-identity tokens — OAuth 2.0 Token Exchange (RFC 8693)](#7-dual-identity-tokens)
8. [Governance policies as code](#8-governance-policies-as-code)
9. [Safety guardrails — input and output](#9-safety-guardrails)
10. [Data at rest — envelope encryption](#10-data-at-rest)
11. [Audit chain — tamper-evident log](#11-audit-chain)
12. [Incident reporting — Annexure VI as a query](#12-incident-reporting)
13. [Tier promotion — fail-closed AI dispatch](#13-tier-promotion)
14. [Sovereignty — data residency by classification](#14-sovereignty)
15. [BCP — forced-failure drills](#15-bcp)
16. [Observability — every decision is a span](#16-observability)
17. [FREE-AI alignment — recommendation by recommendation](#17-free-ai-alignment)
18. [Operational runbook](#18-operational-runbook)
19. [Open questions and roadmap](#19-open-questions-and-roadmap)

---

## 1. The mental model

Single-line defences fail. A `WHERE user_id = $1` clause that someone
forgot to add is a cross-tenant leak. An `if !isAdmin { return 403 }`
check that someone copied wrong is privilege escalation. A
`sanitize(input)` call that someone removed during a refactor is an
injection.

Defence in depth assumes every layer has bugs. The job of the stack
is to ensure that a bug in any one layer is contained by the next.

Genie's security envelope has eleven layers in the request path. They
are listed below in execution order — top to bottom — for a single
`/v1/ask` request from a user holding a valid JWT.

```
┌─────────────────────────────────────────────────────────────────────┐
│ L1. TLS termination                                                  │
│     — out of scope of this repo; provided by the ingress             │
├─────────────────────────────────────────────────────────────────────┤
│ L2. HTTP middleware: rate limit, request ID, OTel trace start        │
│     pkg/web/mid/ratelimit.go, tracing.go                             │
├─────────────────────────────────────────────────────────────────────┤
│ L3. JWT verify + claims extract                                      │
│     pkg/web/mid/auth.go::RequireAuth → pkg/auth/jwt.go::Verify       │
├─────────────────────────────────────────────────────────────────────┤
│ L4. Role gate (optional per route)                                   │
│     pkg/web/mid/auth.go::RequireRole                                 │
├─────────────────────────────────────────────────────────────────────┤
│ L5. Handler validation: schema, classification, size                 │
│     pkg/web/handlers/*.go                                            │
├─────────────────────────────────────────────────────────────────────┤
│ L6. Message enrichment: tenant_id, expected_tenant, user_roles       │
│     into Metadata, then bus.Publish                                  │
│     pkg/web/handlers/ask.go                                          │
├─────────────────────────────────────────────────────────────────────┤
│ L7. Orchestrator policy stack (CompositePolicy):                     │
│     RBAC → Tenant → Tier → Classification → PromptInjection →        │
│     PII → Schema → Consent → Explainability →                        │
│     MaxContentLength → board-DSL rules                               │
│     pkg/orchestration/orchestrator.go + pkg/governance/*.go          │
├─────────────────────────────────────────────────────────────────────┤
│ L8. Agent HandleMessage (the actual work)                            │
│     agents/<name>/<name>.go                                          │
├─────────────────────────────────────────────────────────────────────┤
│ L9. DB query under WithTenant — RLS enforces tenant column           │
│     pkg/storage/postgres/tenant.go + migrations/0005_rls.sql         │
├─────────────────────────────────────────────────────────────────────┤
│ L10. LLM call wrapper: deadline, circuit, budget, sovereignty        │
│      pkg/llm/{deadline,circuit,budget,router}.go                     │
├─────────────────────────────────────────────────────────────────────┤
│ L11. Audit append + hash chain + Annexure VI incident if applicable  │
│      pkg/compliance/audit.go + pkg/incidents/incidents.go            │
└─────────────────────────────────────────────────────────────────────┘
```

The contract any reviewer should hold us to: **no two adjacent layers
share a single point of failure for the same class of attack.** A
cross-tenant request that bypasses L7 still hits L9; an unauthorised
caller that bypasses L4 still hits L3; a hallucinating LLM that
bypasses L7's classification check still gets the disclaimer added
at L11 and is incident-logged.

---

## 2. Threat model

The threats below are the ones we actively design for. We use
STRIDE-style categories to anchor each one to standard literature,
but the labels matter less than the per-threat mitigations.

| # | Threat | Class | Lives in / mitigated by |
|---|---|---|---|
| T1 | Cross-tenant data leak via missing `WHERE user_id = $1` | Information disclosure | RLS (L9) + bus tenant policy (L7) |
| T2 | Privilege escalation by forging or replaying a JWT | Spoofing | Short TTL + HS256 signature + audience check; passkeys for MFA |
| T3 | Confused-deputy: agent A's token used to read agent B's data | Elevation of privilege | RFC 8693 audience-scoped exchanged tokens |
| T4 | Prompt injection that pivots a tool call | Tampering | `PromptInjectionPolicy`, output schema validation, tool allowlist |
| T5 | Hallucinated payment / KYC verdict | Tampering | Deterministic agent core; LLM is narration only |
| T6 | Audit log tampering by an insider with DB write access | Repudiation | Hash chain (`pkg/compliance/audit.go`) + WORM external sink |
| T7 | Untriaged AI-generated agent in production traffic | Tampering / EoP | Tier promotion gate (L7) defaults to TierPrototype |
| T8 | LLM provider data-residency violation | Compliance | `pkg/sovereignty.ProviderRegistry.Allowed()` |
| T9 | Runaway autonomy: agent loops until budget exhausted | Denial of service | Budget + circuit + deadline wrappers (L10) |
| T10 | Single-vendor outage takes down the customer-facing path | Availability | Fallback agents + BCP drill |
| T11 | KEK compromise | Information disclosure | Per-row `kek_id`, KMS-pluggable resolver, rotation playbook |
| T12 | Service-to-service identity spoofing inside the mesh | Spoofing | SPIFFE/SVID identity (`pkg/identity`) + mTLS (out of repo) |
| T13 | Insider browsing all incidents via the admin UI | Information disclosure | Admin-only routes + audit on every read |
| T14 | Supply-chain attack on a third-party model provider | Tampering | AIBOM with provenance + `pkg/safety` adversarial corpus on every release |
| T15 | Adversarial input designed to evade safety scoring | Tampering | Plugin chain with all-of mode; multi-vendor scoring |

The threats not in scope for this repo (handled by infrastructure):
TLS, DDoS at the edge, container escape, kernel CVEs. These are
mitigated by the deployment platform (the K8s cluster, the ingress,
the CDN), not by Genie code.

---

## 3. Identity layer

Identity in Genie comes in three flavours, each with a different
trust root.

### 3.1 User identity (humans)

| Field | Source |
|---|---|
| `User.ID` | UUID, generated at signup |
| `User.Email` | Unique, lower-cased, the login key |
| `User.Roles` | Multi-role array (`user` / `advisor` / `admin`) |
| `User.PasswordHash` | bcrypt; raw password never stored |

`pkg/auth/types.go`. User records live in `users` in Postgres
(`pkg/storage/postgres/users.go`); the row is itself protected by RLS
so a user can only read their own profile (the policy includes the
`__admin__` OR clause so login by email works — see §6.5).

### 3.2 Agent identity (software within Genie)

Every agent declares a stable string ID at the top of its package:

```go
const (
    ID   = "kyc_orchestrator"
    Name = "KYC Orchestrator"
)
```

The registry indexes by ID; `tests/agents_registry/` enforces
uniqueness across the agent tree via a grep test that runs on every
`go test ./...`. The agent ID is what `Actor.Subject` carries in
the RFC 8693 dual-identity flow (§7).

### 3.3 Workload identity (services across the network)

`pkg/identity` ships SPIFFE-style DID + W3C Verifiable Credential
primitives:

```go
// pkg/identity/identity.go
func NewDIDKey() (*DID, error)                                    // generates Ed25519 pair, returns did:key:z…
func IssueVC(issuer *DID, vc *VerifiableCredential) (*VerifiableCredential, error)
func VerifyVC(vc *VerifiableCredential, pub ed25519.PublicKey) error
```

DIDs let a service prove "I am `kyc-mcp-server`" without needing a
shared secret with every other service. The credential model lets
the issuer attest scoped claims ("this service may call the
sanctions API"). Today this is a developer-facing toolkit; wiring
SPIFFE/SPIRE for full workload identity at deploy time is the
roadmap item that connects this to mTLS at the transport layer.

The MCP token store (`pkg/storage/postgres/mcp_tokens.go`) is the
per-user landing zone for third-party API tokens (e.g. Zerodha Kite).
Each row is tenant-scoped via RLS so one user's broker token cannot
be read by another.

---

## 4. Authentication

Four authentication paths land in the same `auth.Claims` shape, so
downstream code never needs to branch on "how did the user prove
themselves."

### 4.1 Password (bcrypt)

```go
// pkg/auth/password.go
func HashPassword(plain string) (string, error)
func VerifyPassword(hash, plain string) error
```

bcrypt cost is the library default (10) — increase via
`bcrypt.GenerateFromPassword` if your threat model warrants. The
hash is stored in `users.password_hash`; the column is excluded
from the JSON marshal of `auth.User` (the field tag is `-`) so it
never leaks through any handler.

### 4.2 JWT

```go
// pkg/auth/jwt.go
type Issuer struct { Secret []byte; Issuer string; Audience []string; TTL time.Duration }
func NewIssuer(secret, issuer string, audience []string, ttl time.Duration) *Issuer
func (i *Issuer) Issue(userID, email string, roles []Role) (string, Claims, error)
func (i *Issuer) IssueWithActor(userID, email string, roles []Role, audience []string, actor *Actor) (string, Claims, error)
func (i *Issuer) Verify(token string) (Claims, error)
func (i *Issuer) VerifyIgnoringAudience(token string) (Claims, error)   // used by token-exchange
```

HS256 because we control both ends; no third-party JWT library — the
implementation is in the same file as the issuer and uses
`crypto/hmac` + `encoding/base64` from stdlib. The audit risk of
shipping a JWT library is higher than the risk of writing 100 lines
of HMAC-SHA256.

Default TTL: 60 minutes for password tokens; 15 minutes for tokens
returned by passkey assertion; 15 minutes for OAuth Device flow;
the same TTL for token-exchange outputs (with a hard cap of the
user token's remaining TTL — §7).

### 4.3 Passkeys (WebAuthn, Ed25519)

```go
// pkg/auth/webauthn/webauthn.go
func (s *Service) BeginRegistration(userID string) (ceremonyID string, challenge []byte, err error)
func (s *Service) FinishRegistration(ceremonyID, credentialID string, pubKey ed25519.PublicKey, clientChallenge []byte) error
func (s *Service) BeginAssertion(userID string) (ceremonyID string, challenge []byte, err error)
func (s *Service) FinishAssertion(ceremonyID, credentialID string, authenticatorData, clientDataJSON, signature []byte) (string, error)
```

Stdlib `crypto/ed25519` for the verify; the WebAuthn dance is
explicit (challenge, attestation client data, signature) rather than
proxied to a third-party library — same reasoning as JWT.

### 4.4 OAuth 2.1 + PKCE / Device Flow

```go
// pkg/auth/oauth2/oauth2.go
func (s *Server) Authorize(req AuthorizeRequest, subject, email string, roles []auth.Role) (AuthorizeResponse, error)
func (s *Server) Token(req TokenRequest) (TokenResponse, error)
func GenerateVerifier() (verifier, challenge string, err error)
```

```go
// pkg/auth/oauth_device/oauth_device.go
func (s *Service) Begin() (AuthorizationResponse, error)        // user_code + device_code
func (s *Service) Approve(userCode, token string) error
func (s *Service) Deny(userCode string) error
func (s *Service) Poll(deviceCode string) (TokenResponse, error)
```

Use cases:
- **OAuth 2.1 + PKCE** — first-party browser app or mobile app
  authenticating against Genie.
- **OAuth Device Flow** — voice assistants, kiosks, the
  conversational call-centre agent: the device shows a `user_code`
  on screen, the user types it on a phone, the device polls until
  approval.

Both return the same `auth.Claims` shape. The PKCE verifier
prevents authorisation-code interception attacks; the device flow's
short user code is paired with a long device code so a casual
shoulder-surfer cannot replay the device's session.

---

## 5. Authorisation

Authorisation runs at three layers — HTTP middleware, the bus
governance stack, and the database. Each catches a different
mistake.

### 5.1 HTTP-layer role gate

```go
// pkg/web/mid/auth.go
func RequireRole(roles ...auth.Role) func(http.Handler) http.Handler
```

Wired in `pkg/web/router.go`:

```go
r.With(mid.RequireRole(auth.RoleAdmin)).Get("/incidents",     d.Incidents.List)
r.With(mid.RequireRole(auth.RoleAdmin)).Get("/ai-inventory",  d.Inventory.List)
r.With(mid.RequireRole(auth.RoleAdmin)).Get("/aibom",         d.AIBOM.Get)
```

The role gate is "is the bearer of this token allowed to call this
route at all?" — coarse, fast, declarative. It fails closed: an
unknown role or a missing claim returns 403 before the handler runs.

### 5.2 Bus-layer RBAC

```go
// pkg/governance/rbac.go
type RBACPolicy struct {
    RequiredRolesByType map[string][]string  // msg.Type → any-of role list
    AdminBypass         bool
}
```

The HTTP layer protects routes; the bus layer protects message
types. The two are not redundant — many message types arrive via
internal hops (an agent fanning out to a sub-agent), and those
never traverse HTTP. The bus check catches the internal case.

Roles are read from `metadata.user_roles`, which the HTTP intake
sets from `Claims.Roles`. `extractRoles` accepts a `string`
(comma-separated), `[]string`, or `[]any` (JSON round-trip), so a
bus message that crossed the wire still parses.

`AdminBypass` is per-policy-instance. The customer-facing route's
`RBACPolicy{}` instance keeps it `false`; the admin audit reader's
instance flips it `true`. The flag is never global state.

### 5.3 Tier dispatch gate

See §13 for the full tier model. In short: a `Sketch` or
`Prototype` agent cannot serve a customer-facing message. The
predicate is `agent.Production(agent.TierOf(target))`. Default-to-
Prototype means an agent that forgot to declare `Tier()` is denied
rather than allowed.

### 5.4 Database-layer enforcement

RLS (§6) is the third "is this caller allowed?" gate. A user with
the right HTTP role and the right bus role still can't read
another tenant's row, because the database itself refuses.

---

## 6. Tenant isolation

The single most important property in a multi-tenant financial
system: one customer must never see another customer's data. Genie
enforces this at two layers.

### 6.1 Why two layers

A single layer is a single bug away from a breach. The bus layer
runs in Go code; the database layer runs in PostgreSQL's RLS engine.
A logic bug in one is not a logic bug in the other. The combined
probability of both layers failing on the same vector is the product
of their independent failure probabilities — much smaller than
either alone.

### 6.2 Bus layer — `TenantPolicy`

```go
// pkg/governance/tenant.go
type TenantPolicy struct {
    AppliesTo   []string  // message types this policy applies to
    AdminBypass bool      // opt-in admin cross-tenant bypass
}
func (p TenantPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error)
```

Decision flow:

```
if type ∉ AppliesTo (when set)     → allow (not applicable)
if metadata.tenant_id is empty     → deny (missing tenant_id)
if AdminBypass && role.admin       → allow (admin tenant bypass)
if expected_tenant ≠ tenant_id     → deny (tenant mismatch: a got b)
otherwise                          → allow
```

The denial reason includes both ids so the operator can tell at a
glance whether it was a forgotten claim, a stale token, or an actual
attack.

Tests live in `pkg/governance/tenant_test.go` (seven cases). The
helper `metaStringSlice` handles the `[]any` shape that JSON
round-trip produces — pinned by `TestMetaStringSliceHandlesAnySlice`
to prevent a refactor from "tightening" the type assertion and
breaking real production messages.

### 6.3 Database layer — Postgres RLS

```sql
-- pkg/storage/postgres/migrations/0005_rls.sql
ALTER TABLE documents  ENABLE  ROW LEVEL SECURITY;
ALTER TABLE documents  FORCE   ROW LEVEL SECURITY;

CREATE POLICY documents_tenant_isolation ON documents
    USING       (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK  (user_id::text = current_setting('app.current_tenant', true));
```

`FORCE` extends RLS even to table owners — without it, the
migration role and any future superuser would silently bypass.
The same shape is applied to `accounts`, `mcp_tokens`, `incidents`,
and `users` (the latter two with an `__admin__` OR clause for the
audit reader and the login lookup).

### 6.4 Setting the tenant context

```go
// pkg/storage/postgres/tenant.go
func (db *DB) WithTenant(ctx context.Context, tenantID string, fn TenantFunc) error
func (db *DB) WithAdminContext(ctx context.Context, fn TenantFunc) error
```

`WithTenant` opens a txn, runs `SELECT set_config('app.current_tenant',
$1, true)` (the `true` flag scopes the GUC to the transaction), then
runs the closure with the bound `pgx.Tx`. When the txn commits or
rolls back, the GUC dies. The pool may hand the connection to
another request; that request's `WithTenant` sets its own GUC. The
GUC never leaks across requests.

`set_config(name, value, is_local)` is used instead of `SET LOCAL`
because the SQL form rejects bind parameters; `set_config` accepts a
string and respects the local flag.

### 6.5 The `__admin__` sentinel

Some operations legitimately need cross-tenant access: the login
handler (the user's identity isn't known until after the email
lookup), the audit reader (an admin investigates across tenants),
the inventory endpoint, and the retention/reconciliation jobs.

Those use `WithAdminContext`, which sets the GUC to the literal
`__admin__`. The RLS policies on `users` and `incidents` include
an `OR current_setting('app.current_tenant', true) = '__admin__'`
clause. No other policy does. The sentinel is the only legitimate
cross-tenant key, and the only way to set it is via
`WithAdminContext` — which must be reachable only from a route
gated by `RequireRole(RoleAdmin)`.

A UI contract test (`pkg/web/handlers/ui_security_test.go::
TestUI_NoAdminFieldsInPublicHTML`) refuses to let the string
`__admin__`, `app.current_tenant`, `BYPASSRLS`, or `set_config`
appear in any UI asset, so the sentinel is invisible to anyone
reading the page source.

### 6.6 What the two layers catch, side by side

| Failure mode | Bus catches? | DB catches? |
|---|---|---|
| Handler forgot `WHERE user_id = $1` | ✗ (handler never publishes) | ✓ |
| Handler sends wrong `tenant_id` in metadata | ✓ | (handler did set GUC correctly → still safe) |
| Compromised admin token reused in customer route | ✗ (AdminBypass=false on that route) → ✓ deny | ✓ (sentinel not set) |
| Background job runs `Reconcile()` without setting GUC | n/a | ✓ (RLS rejects, query returns 0 rows) |
| Engineer adds new table without RLS policy | ✗ | ✗ — covered by the migration review checklist + the `pg_class` audit query in operations.md |
| Engineer sets GUC to attacker-controlled string | n/a | mitigated — `WithTenant("")` returns `ErrNoTenant`; `WithAdminContext` is the only way to use the sentinel |

The integration test
`tests/security_envelope_test.go::TestSecurityEnvelope_CrossTenantIsBlocked`
exercises the bus layer end-to-end through the orchestrator's
policy stack.

---

## 7. Dual-identity tokens

Two scenarios that look the same to a single-identity token, and
why they're not the same:

- **Scenario A.** A user logs in, navigates to their dashboard, and
  the dashboard reads their transactions. The audit row says
  "user X read transactions for user X." Correct.
- **Scenario B.** A user asks "what was my biggest expense?", an
  agent runtime processes the query, the agent calls an MCP server,
  the MCP server queries the same transaction table. The audit row
  says — *what?* If the agent presents the user's token, the audit
  row blames the user for an automated action. If the agent
  presents its own token, the audit row loses the user entirely.

OAuth 2.0 Token Exchange (RFC 8693) gives us the third option:
**dual-identity**. The exchanged token's `Subject` is the user
(unchanged); the `Actor` (`act`) claim records the agent that's
currently acting on the user's behalf. Both are in the same signed
JWT.

### 7.1 The shape

```go
// pkg/auth/types.go
type Claims struct {
    Subject   string   // sub  — the user
    Email     string
    Roles     []Role
    IssuedAt  int64
    ExpiresAt int64
    Issuer    string
    Audience  []string
    Actor     *Actor   // act  — the service acting on the user's behalf
}
type Actor struct {
    Subject string  // sub — agent / service id
    Issuer  string  // iss — who minted this actor identity
    Nested  *Actor  // act — previous actor in the chain (for N-hop)
}
```

### 7.2 The flow

```go
// pkg/auth/tokenexchange/exchange.go
svc := tokenexchange.New(issuer, "genie-api")
exchanged, claims, _ := svc.Exchange(ctx, tokenexchange.Request{
    SubjectToken: userJWT,
    ActorID:      "kyc_orchestrator",
    Audience:     "mcp://kyc-server",
})
// claims.Subject       = user (unchanged)
// claims.Actor.Subject = kyc_orchestrator
// claims.Actor.Issuer  = genie-api
```

A second hop chains the previous actor under `Actor.Nested`:

```go
final, claims2, _ := svc.Exchange(ctx, tokenexchange.Request{
    SubjectToken: exchanged,
    ActorID:      "kyc-mcp-server",
    Audience:     "https://api.upstream/records",
})
// claims2.Subject              = user
// claims2.Actor.Subject        = kyc-mcp-server
// claims2.Actor.Nested.Subject = kyc_orchestrator
```

A downstream auditor reading the final token's claims can answer
"who initiated, what was the proximate caller, what was in between."

### 7.3 The cache

Cache key: `(user_subject, actor_id, audience)`. **Not** the input
token. Two callers presenting different user tokens for the same
user share the cached exchanged token.

Cache TTL: `min(token_exp, subject_exp) − SafetyMargin` (default
margin: 60s). The exchanged token never outlives the underlying user
session — invariant the contract depends on.

```go
svc.Invalidate(userSubject)   // logout / password change
```

drops every cached token derived from the burned session.

### 7.4 Why a "loose verifier"

The first-hop exchanged token has audience `mcp://kyc-server`, not
the issuer's default. The second-hop exchange has to accept that
input. The verifier wraps the issuer in `looseVerifier`, which
calls `Issuer.VerifyIgnoringAudience` — signature is still checked,
expiry is still checked, issuer is still checked, only the audience
claim is skipped. The output audience is set explicitly by the
caller, so there's no "anyone" audience floating around. The
*downstream* service strictly verifies the audience it expects.

### 7.5 What it integrates with

- **Audit log** (§11) records the full claims on every action; the
  hash chain makes the audit non-repudiable.
- **RBAC policies** can express "user must have permission X AND
  actor must be authorised for operation Y" — a composite that's
  impossible with a single-identity token.
- **Annexure VI** (§12) reports cite both the user and the agent
  chain, which is what the form actually asks for.

Tests: `pkg/auth/tokenexchange/exchange_test.go` (seven cases,
including two-hop) plus
`tests/security_envelope_test.go::TestSecurityEnvelope_TokenExchangeAuditIdentity`.

---

## 8. Governance policies as code

`pkg/governance/policy.go` defines the contract:

```go
type Decision string
const (
    DecisionAllow Decision = "allow"
    DecisionDeny  Decision = "deny"
)
type PolicyResult struct {
    Decision    Decision
    Reason      string
    CheckedAt   time.Time
    CheckedByID string
}
type Policy interface {
    Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error)
}
```

The contract is intentionally narrow: binary decision, human-readable
reason, optional `CheckedByID` for attribution in the audit log. Two
or three policy interfaces would have been more expressive; we chose
one so a new author cannot make policy decisions inconsistent across
the stack.

### 8.1 The shipped policies

| Policy | Catches |
|---|---|
| `RBACPolicy` | Wrong role for message type |
| `TenantPolicy` | Cross-tenant or untenanted message |
| `ClassificationPolicy` | "secret"-classified content reaches a non-cleared agent |
| `PromptInjectionPolicy` | Common jailbreak patterns ("ignore previous", "you are now") |
| `PIIBlockPolicy` | Aadhaar / PAN / mobile number patterns in places they shouldn't be |
| `RequiredMetadataPolicy` | Missing trace_id, missing account_id, missing classification |
| `SchemaPolicy` | Message content fails to parse against a JSON schema for the type |
| `ConsentPolicy` | Action attempted that the user hasn't consented to |
| `ExplainabilityPolicy` | High-risk decision missing a `rationale` field |
| `MaxContentLengthPolicy` | Payload above the per-route ceiling (default 16 KiB) |

### 8.2 Composition

```go
// pkg/governance/policy.go
func NewComposite(policies ...Policy) *CompositePolicy
```

`CompositePolicy` evaluates each in order; first denial wins; reason
of the first denial is propagated. Deny-on-first means a denied
message produces a single clear reason rather than a wall of
overlapping deny messages — easier to operationalise.

### 8.3 Adaptive policies via DSL

The risk team can author new rules without engineering involvement.

```go
// pkg/policy/dsl/
type Rule struct {
    ID       string
    When     string   // boolean expression in the DSL
    Decision string   // "allow" | "deny"
    Reason   string
}
func Compile(rules []Rule) ([]CompiledRule, error)
func AsPolicies(rules []CompiledRule) []governance.Policy
```

The DSL covers comparison, boolean composition, string `contains` /
`startsWith`, and dotted `metadata.*` access. No loops, no
functions, no recursion — the grammar fits on a screen. If a rule
needs more than the DSL provides, write a Go-side `governance.Policy`.

The host loader:

```go
raw, _ := os.ReadFile("config/policies/rules.yaml")
var rules []dsl.Rule
yaml.Unmarshal(raw, &rules)
compiled, _ := dsl.Compile(rules)
composite := governance.NewComposite(append(coreStack, dsl.AsPolicies(compiled)...)...)
```

The board-approved policy YAML is version-controlled in
`config/ai-policy.example.yaml` (replace with the real one in
deployment). The DSL is documented at
[packages/policy-dsl.md](packages/policy-dsl.md).

---

## 9. Safety guardrails

Safety scoring runs as a plugin chain (`pkg/safety/plugin.go`)
keyed by `Stage`:

| Stage | Where it runs |
|---|---|
| `StageInput` | Before the LLM call — prompt injection, jailbreak attempts |
| `StageOutput` | After the LLM call — toxicity, PII leak, secrets-in-output |
| `StagePrePersist` | Before writing model output to durable storage |

```go
type Plugin interface {
    Name() string
    Stage() Stage
    Inspect(ctx context.Context, text string) (Verdict, error)
}
type Chain struct {
    Plugins []Plugin
    Mode    ChainMode  // FirstFail | AllScored | AnyMatch
}
```

Three modes:
- **FirstFail** — stop at the first deny. Cheap; used when scoring
  is expensive.
- **AllScored** — run every plugin and report all verdicts.
  Required when multiple scorers each contribute a dimension.
- **AnyMatch** — deny if any single plugin matches. Used for
  pattern-based hard blocks.

`HTTPShield` is a built-in plugin shape that adapts an external
HTTP scorer (Model Armor, Bedrock Guardrails, Lakera) into the
`Plugin` interface. The host registers the shield with a name and a
URL; the scorer's verdict is normalised before propagation. See
[packages/safety-plugins.md](packages/safety-plugins.md).

### 9.1 Bias scoring

```go
// pkg/safety/bias.go
type DemographicParity struct {
    GroupA, GroupB    string
    PositiveRateA     float64
    PositiveRateB     float64
    Disparity         float64
    ExceedsThreshold  bool
}
func ComputeDemographicParity(pA, nA, pB, nB int, threshold float64) DemographicParity
```

Demographic-parity scoring runs offline against decisioning agents
(KYC, loan, claim). The output flows into a fairness report; if
`ExceedsThreshold`, the offending agent's tier is held at Beta
(see §13) and an incident is opened.

### 9.2 Adversarial corpus

`cmd/red-team/` runs the probe corpus against the live policy
stack. `make red-team` is the CI command; a single probe failure
(an allow where deny was expected, or vice versa) fails the build.
The corpus is the source of "did we test for X" answers when a new
attack class lands in the news cycle.

---

## 10. Data at rest

Document content is **envelope encrypted**: a fresh per-document
DEK (data encryption key) encrypts the content; the DEK itself is
encrypted by a KEK (key encryption key) and stored alongside.

```go
// pkg/crypto/envelope.go
type EncryptedPayload struct {
    KEKID      string   // which KEK encrypted the DEK
    DEKWrapped []byte   // the encrypted DEK
    Nonce      []byte
    Ciphertext []byte
    ...
}
type KeyResolver interface {
    ActiveKEKID() string
    Wrap(dek []byte) ([]byte, error)
    Unwrap(kekID string, wrapped []byte) ([]byte, error)
}
type Encryptor struct { Keys KeyResolver }
func (e *Encryptor) Encrypt(plaintext []byte) (EncryptedPayload, error)
func (e *Encryptor) Decrypt(p EncryptedPayload) ([]byte, error)
```

### 10.1 KEK resolvers

```go
// pkg/crypto/resolver.go
type EnvKeyResolver struct {...}   // reads KEK from GENIE_KEK_BASE64; dev/staging
type KMSKeyResolver struct {...}   // delegates to a KMS via a small interface
type KMSClient interface {
    Encrypt(kekID string, plaintext []byte) ([]byte, error)
    Decrypt(kekID string, ciphertext []byte) ([]byte, error)
}
```

Production hosts implement `KMSClient` against AWS KMS, GCP KMS,
or HashiCorp Vault. The interface is intentionally tiny —
encrypt/decrypt by KEK ID — so any KMS that supports envelope
encryption fits.

### 10.2 Why per-document DEK

Three reasons:
- **Blast radius.** A leaked DEK exposes one document, not the corpus.
- **Rotation.** Re-wrapping a DEK with a new KEK is one
  `kms:Encrypt` call; re-encrypting the document body is not needed.
- **Audit attribution.** Each document's encryption call is one
  audit event; rotating one KEK is N rotation events, not one
  re-encrypt-everything event.

### 10.3 KEK rotation

The schema stores `kek_id` per row (`pkg/storage/postgres/migrations/`).
Rotation:

1. Generate a new KEK in KMS, assign a new `kek_id`.
2. Update `GENIE_KEK_BASE64` (or the KMS pointer) on the API.
3. New uploads use the new KEK; old documents keep decrypting with
   the original KEK (the row's `kek_id` tells the resolver which
   KEK to use).
4. A background rewrap job (roadmap) lazily re-wraps older rows.

Until the rewrap job ships, leave both old and new KEKs reachable
in the resolver. Most KMS systems do multi-key rotation natively
(AWS KMS aliases, GCP KMS key versions).

---

## 11. Audit chain

Every governance-relevant action writes an entry to a hash-chained
append-only log.

```go
// pkg/compliance/audit.go
type AuditEntry struct {
    Seq        int64
    OccurredAt time.Time
    Actor      string         // user id, agent id, "system"
    Action     string         // "policy.deny", "kyc.decide", "consent.grant", ...
    Target     string
    Details    map[string]any
    PrevHash   string
    RowHash    string
}
type AuditLog interface {
    Append(ctx context.Context, actor, action, target string, details map[string]any) (AuditEntry, error)
    List(ctx context.Context) ([]AuditEntry, error)
    Verify(ctx context.Context) error
}
```

`RowHash = SHA-256(prev_hash || canonical-JSON(row-without-hash))`.
`Verify` walks the chain from genesis and refuses if any row's hash
is inconsistent.

### 11.1 What writes to the audit log

- Governance policy denials (via the orchestrator's `OnPolicyDeny` hook).
- Agent errors (via `OnAgentError`).
- KYC verdicts (`agents/kyc_orchestrator`).
- Payment routing decisions (`agents/payment_orchestrator`).
- Claim adjudications (`agents/claim_adjudicator`).
- Consent grants and revocations (`pkg/compliance/consent.go`).
- LLM budget breaches, circuit-breaker trips.
- KEK rotation events (when implemented).
- Every token-exchange call (the actor chain is in `Details`).

### 11.2 Why a hash chain is enough

The RBI cyber-resilience expectation is "logs cannot be tampered with
without detection." A hash chain gives that without requiring WORM
storage. The naïve attack — delete the most recent N entries — is
detected because the chain head changes; a verifier compares the
head to an off-system anchor (e.g. an anchor row published hourly to
a separate database or sent to a customer-controlled email).

The strong attack — rewrite every entry from the modified point
forward — requires control of every byte from then on. That's
detectable by anyone holding a previously-anchored head.

### 11.3 Anchoring (roadmap)

The current implementation ships the in-memory and Postgres-backed
log. The anchoring pipeline that publishes the head to an external
witness (S3 Object Lock, customer email, Sigstore Rekor) is the
remaining piece to declare the log fully tamper-evident in
production. Until that ships, the log is detectable-on-restart but
not detectable-in-flight.

---

## 12. Incident reporting

`pkg/incidents/incidents.go` implements the RBI FREE-AI Annexure VI
form. Fields mirror the form; the structured taxonomy maps to the
form's "failure mode" categorisation.

```go
type Severity    string  // low | moderate | high
type Status      string  // ongoing | resolved
type FailureMode string  // bias | hallucination | explainability_gap | privacy_breach
                          // unintended_action | policy_denied | agent_error | unknown

type Incident struct {
    ID                   string
    OccurredAt           time.Time
    DetectedAt           time.Time
    UseCase              string
    Model                string
    ThirdPartyVendor     string
    Description          string
    AffectedStakeholders string
    Severity             Severity
    FailureMode          FailureMode
    RootCause            string
    ResponseActions      string
    Status               Status
    ActorID              string
    Metadata             map[string]any
}

type Store interface {
    Create(ctx context.Context, i Incident) (Incident, error)
    List(ctx context.Context, limit int) ([]Incident, error)
    CountByModeSince(ctx context.Context, mode FailureMode, since time.Time) (int, error)
}
```

### 12.1 Auto-recording sources

Most incidents are auto-recorded. The Annexure VI form is what the
operator submits, but the *data* is sourced from the runtime.

| Trigger | Source |
|---|---|
| Policy deny | `pkg/orchestration/orchestrator.go::OnPolicyDeny` |
| Agent panic / error above grade threshold | `OnAgentError` |
| LLM budget breach | `pkg/llm/budget.go` |
| Circuit-breaker trip | `pkg/llm/circuit.go` |
| Safety scorer flag above threshold | `pkg/safety/` |
| KYC sanctions hit | `agents/kyc_orchestrator::rejectVerdict` |
| Payment rejection | `agents/payment_orchestrator::reject()` |
| Dual-identity attribution | `pkg/auth/tokenexchange` populates `Actor` chain in `Metadata` |

### 12.2 Grading (FREE-AI Rec 8: graded liability)

```go
// pkg/incidents/grading.go
type Liability struct {
    Mode        FailureMode
    Count       int
    Liability   string  // "low" | "medium" | "high" | "critical"
    WindowDays  int
}
func Grade(ctx context.Context, store Store, mode FailureMode, windowDays int) (Liability, error)
```

Repeated failures in the same mode escalate the liability tier. The
escalation table is owned by the risk team in policy YAML; the
defaults are deliberately conservative.

### 12.3 Endpoint

`GET /v1/incidents` — admin-gated, returns the most recent N
incidents. The handler is `pkg/web/handlers/incidents.go`. The
record never includes the customer's name or PAN in the response
body (only the pseudonymous `affected_id`); the form is downloaded
separately by the regulator with the de-pseudonymisation key.

---

## 13. Tier promotion

```go
// pkg/agent/tier.go
type Tier string
const (
    TierSketch     Tier = "sketch"
    TierPrototype  Tier = "prototype"
    TierBeta       Tier = "beta"
    TierProduction Tier = "production"
)
type TierAware interface { Tier() Tier }
func TierOf(a Agent) Tier            // returns declared tier or TierPrototype
func Production(t Tier) bool
func TierOrdinal(t Tier) int          // unknown → -1
func AtLeast(got, required Tier) bool
```

The four tiers reflect the actual lifecycle of AI-generated code in
a regulated shop:

| Tier | Who writes it | Tests | RBAC | Audit | Dispatch in |
|---|---|---|---|---|---|
| Sketch | AI scaffold | None | None | None | Sandbox |
| Prototype | Engineer owns it | Unit | Dev token | Per-call log | Internal dev |
| Beta | Authored, staged | Unit + integ | Staging RBAC | Per-call audit + metrics | Staged customers |
| Production | Authored, reviewed | Unit + integ + adversarial | Prod RBAC | Hash-chained audit | Prod |

### 13.1 Default-to-Prototype rationale

Undeclared `Tier()` → `TierPrototype`. **Not** `TierProduction`.
Why:

- Default-to-Production means an attacker (or a careless engineer)
  can promote an agent by *not* implementing the interface.
- Default-to-Prototype means an existing agent that hasn't been
  touched is treated as not-promoted — until the human explicitly
  returns `TierProduction`.

### 13.2 Dispatch gate

```go
target := registry.Get(ctx, msg.To)
if !agent.Production(agent.TierOf(target)) && env != "sandbox" {
    return ErrTierNotPermitted
}
```

In Genie's reference, this lives in the policy stack so it composes
with the other governance checks and shares the denial/metric path
(`tests/security_envelope_test.go::tierPolicy`).

### 13.3 Inventory integration

`/v1/ai-inventory` (admin-only) returns the tier per agent:

```json
{
  "id": "kyc_orchestrator",
  "name": "KYC Orchestrator",
  "capabilities": ["kyc.decide"],
  "risk_class": "high",
  "tier": "production",
  "has_fallback": true,
  "fallback_id": "kyc_fallback"
}
```

The risk team reads the tier column to spot non-production agents
serving customer traffic. The UI contract test
(`pkg/web/handlers/ui_security_test.go::TestInventory_ListIncludesTier`)
pins the field so a refactor cannot drop it.

### 13.4 Promotion checklist (Beta → Production)

- [ ] `Tier()` returns `TierProduction` in source.
- [ ] `RiskLevel()` declared.
- [ ] Unit tests cover each branch of `HandleMessage`.
- [ ] At least one integration test in `tests/` exercises it through the bus.
- [ ] Adversarial corpus passes — prompt injection and jailbreak suites at minimum.
- [ ] Fallback wired via `orchestrator.SetFallback(<id>, <fallback>)`.
- [ ] BCP drill (`make bcp-drill`) confirms fallback fires.
- [ ] Audit hooks emit on every output.
- [ ] Appears in `/v1/ai-inventory` with `"tier": "production"`.
- [ ] Risk team signed off in the deployment record.

Single-commit promotion is the rule — the tier change is in the
diff, the deployment record links to the diff.

---

## 14. Sovereignty

Data residency is a **policy**, not a slide. The policy lives in
code; the policy gate runs on every LLM call.

```go
// pkg/sovereignty/sovereignty.go
type Region string
type Provider struct {
    Name             string
    Region           Region
    AllowedClasses   []protocol.Classification
}
type ProviderRegistry struct{...}
func (r *ProviderRegistry) Register(p Provider)
func (r *ProviderRegistry) Get(name string) (Provider, bool)
func (r *ProviderRegistry) Allowed(name string, c protocol.Classification) bool
```

The router (`pkg/llm/router.go`) consults the registry before
dispatching a prompt to a provider. A `pii`-classified message
cannot reach a provider whose region isn't in the allowlist for
`pii`. The classification → provider mapping is owned by the policy
YAML and reviewed by the risk team.

| Classification | Allowed providers (example) |
|---|---|
| `public` | any |
| `internal` | in-region cloud + on-prem |
| `pii` | on-prem only |
| `secret` | on-prem only, with prompt redaction |

A misrouted prompt produces a policy denial logged via the audit
chain and an incident with `FailurePrivacyBreach`.

---

## 15. BCP

The Business Continuity Plan story for AI is two pieces: **fallback
agents** + **forced-failure drills**.

### 15.1 Fallback agents

```go
orch.SetFallback("kyc_orchestrator", "kyc_fallback")
```

When `kyc_orchestrator` errors, the orchestrator re-publishes the
message as a `fallback_request` to `kyc_fallback`. The fallback is
typically a deterministic, rules-only path with no LLM dependency.
A customer asking a KYC question always gets a usable answer.

The integration test
`tests/security_envelope_test.go::TestSecurityEnvelope_FallbackTriggers`
exercises the full hop, including the assertion that **tenant
metadata is preserved on the fallback request** — otherwise the
fallback would itself be a tenant-leak gap.

### 15.2 Forced-failure drills

`make bcp-drill` runs the `cmd/bcp` harness which:
1. Wires the production agent stack against the in-memory bus.
2. Replaces the primary `portfolio_advisor` with a stub that always errors.
3. Publishes a real customer-shaped message.
4. Asserts the fallback produces a usable verdict.

Run on every PR. A drill failure means a fallback is missing or
broken; the PR can't merge until fixed.

---

## 16. Observability

Every governance decision is a span. The schema:

| Span | Attributes |
|---|---|
| `agent.handle` | `genie.agent.id`, `genie.agent.name`, `genie.msg.id`, `genie.msg.type` |
| `governance.evaluate` | `genie.policy.decision`, `genie.policy.reason`, `genie.msg.type` |
| `llm.call` | `model`, `provider`, `tokens.in`, `tokens.out`, `cost.micros`, `latency_ms` |

Metrics (`pkg/observability`):
- `genie.bus.messages_published`
- `genie.agent.messages_handled` (per agent)
- `genie.governance.denials` (per policy)
- `genie.agent.errors` (per agent)
- `genie.agent.handle_duration_ms` (histogram per agent)
- `genie.llm.tokens` / `cost_micros` / `latency_ms`

Trace sink: OTel Collector → Tempo. Long-horizon analytics:
`pkg/observability/bq` dual-writes to BigQuery / Snowflake / Kafka
([packages/observability-bq.md](packages/observability-bq.md)).

OpenInference semantic conventions on LLM spans — Arize Phoenix,
Langfuse, and other LLM-observability platforms pick them up unchanged.

---

## 17. FREE-AI alignment

The August 2025 RBI FREE-AI report's 26 recommendations map to
this repo as follows (✅ done, 🟡 partial, ⚪ not yet).
See `docs/free-ai-mapping.md` for the complete table; this section
re-emphasises the security-relevant subset.

| Rec | What it says | Where in Genie |
|---|---|---|
| 6 | Adaptive policies | `pkg/policy/dsl` — risk team owns YAML |
| 8 | Graded liability | `pkg/incidents/grading.go` |
| 14 | Board-approved AI policy (Annexure V) | `config/ai-policy.example.yaml` |
| 15 | Data lifecycle governance | RLS (§6) + envelope encryption (§10) + retention job |
| 16 | AI system governance + autonomous systems | `pkg/agent.RiskLevel()`; budget/circuit/deadline wrappers |
| 17 | Product approval | Tier model (§13); upgraded 🟡→✅ in Q1 hardening |
| 18 | Consumer protection | Disclosure banner + first SSE event |
| 19 | Cybersecurity | Authn (§4) + RBAC (§5) + PII (§9) + session anomaly |
| 20 | Red teaming | `cmd/red-team/` in CI |
| 21 | BCP for AI | Fallbacks (§15) + drill in CI |
| 22 | AI incident reporting (Annexure VI) | `pkg/incidents` + audit chain + dual-identity tokens |
| 23 | AI inventory | `/v1/ai-inventory` with risk_class + tier |
| 25 | Disclosures | `/v1/disclosures` + first SSE event + per-output `Disclaimer` |

---

## 18. Operational runbook

The operational details (commands, env vars, troubleshooting) live
in [`operations.md`](operations.md). This section names the
on-call-only procedures specific to a security event.

### 18.1 "We may have leaked a tenant's data"

1. **Containment.** Disable the suspected agent: in the registry,
   set its `Tier()` to `TierSketch` and redeploy — the dispatch gate
   refuses production traffic.
2. **Evidence.** Pull the last hour of audit entries for that
   tenant: `curl -H "Authorization: Bearer $ADMIN" /v1/audit?actor=<tenant_id>`.
3. **Verify chain.** `curl /v1/audit/verify` — if the hash chain
   has been tampered with, this returns the offending `seq`. Treat
   tampered chain as evidence of insider compromise.
4. **Tenant scope.** Run a one-off `WithAdminContext` query against
   each tenant column: `SELECT user_id, count(*) FROM documents
   GROUP BY user_id` and compare to the expected per-tenant count.
5. **Annexure VI.** Open an incident in `/v1/incidents` with
   `FailurePrivacyBreach`. Severity `high` triggers the immediate
   regulatory disclosure path.
6. **Token invalidation.** For each affected user:
   `tokenexchange.Service.Invalidate(userSubject)` — burns every
   cached exchanged token.

### 18.2 "An agent is hallucinating"

1. Set the agent's `Tier()` to `TierBeta`; production traffic
   stops, staging continues for diagnosis.
2. Pull the LLM spans for that agent over the affected window:
   filter Tempo by `genie.agent.id`.
3. Replay the offending input through `cmd/red-team/` to confirm
   the hallucination is reproducible.
4. Add the offending prompt to the corpus so the build catches it
   next time.

### 18.3 "A KEK was compromised"

1. Mark the KEK retired in KMS — KMS continues to support `Decrypt`
   but stops supporting `Encrypt`.
2. Update `GENIE_KEK_BASE64` (or the KMS pointer) to a fresh KEK.
3. Background rewrap job re-encrypts older rows.
4. Audit `pkg/storage/postgres` for any row still pointing at the
   compromised `kek_id` after the rewrap has run.

### 18.4 "The audit chain doesn't verify"

1. Stop accepting new audit writes (route to a side-buffer).
2. Walk the chain backward to find the last consistent
   `(seq, row_hash)`. That's the last trustworthy moment.
3. Compare to the most recent off-system anchor (when anchoring
   ships) — if anchored head doesn't match in-DB head, treat the
   intervening rows as suspect.
4. Forensics on who has DB write access since the last good seq.

---

## 19. Open questions and roadmap

### 19.1 Things this doc honestly admits we don't ship yet

- **Live SPIFFE/SPIRE wiring.** `pkg/identity` ships the DID/VC
  primitives; the deployment-time integration (SPIRE server,
  workload attestation, mTLS at the proxy) is the missing piece.
- **HTTP `/v1/auth/token-exchange` endpoint.** The Go-level
  `Service` is feature-complete; an HTTP wrapper that speaks
  RFC 8693 over the wire is the next chunk.
- **Audit anchoring.** Hash chain ships; off-system anchoring
  (S3 Object Lock, Rekor, customer email) is the missing chunk for
  full tamper-evidence in flight.
- **KEK rewrap background job.** Per-row `kek_id` supports
  rotation; the job that lazily re-wraps older rows after a KEK
  rotation is roadmap.
- **Multi-tenant org_id.** Today `tenant_id = user_id` (consumer
  banking). Corporate banking needs an `org_id` column and a policy
  update; the bus + DB design is ready for it but the data model
  isn't yet plumbed end-to-end.

### 19.2 Things that are explicitly *not* Genie's job

- TLS, ingress, DDoS, WAF — the deployment platform.
- The model itself — Genie ships against on-prem Ollama by default;
  the model is whatever the regulator-approved bake-off picked.
- The human review process — Genie surfaces the data; the risk
  team owns the call.
- The board policy text — Genie enforces it; the board writes it.

### 19.3 Reading order for new reviewers

1. This document — the canonical security overview.
2. [`packages/postgres-rls.md`](packages/postgres-rls.md),
   [`packages/governance-tenant.md`](packages/governance-tenant.md),
   [`packages/oauth-token-exchange.md`](packages/oauth-token-exchange.md),
   [`packages/agent-tier.md`](packages/agent-tier.md) — the Q1
   security primitives in detail.
3. [`linkedin-article-security-complete.md`](linkedin-article-security-complete.md)
   — the long-form security narrative for the non-engineer audience.
4. [`linkedin-article-agentic-security-operations.md`](linkedin-article-agentic-security-operations.md)
   — the runtime operations playbook.
5. [`free-ai-mapping.md`](free-ai-mapping.md) — recommendation-by-
   recommendation index.
6. [`operations.md`](operations.md) — the runbook.

### 19.4 If you find a gap

Open an issue. Link the specific section of this document, the
specific file path the gap touches, and the threat from §2 the gap
exposes. Security claims age fast; a doc that hasn't been
challenged in a quarter is probably wrong somewhere.

---

## Appendix A — Quick-reference: every security-related path in the repo

```
pkg/auth/                              authentication primitives
├── jwt.go                             HS256 JWT issue + verify
├── password.go                        bcrypt
├── types.go                           User, Claims, Actor
├── oauth2/                            OAuth 2.1 + PKCE
├── oauth_device/                      RFC 8628 device flow
├── webauthn/                          passkeys (Ed25519)
└── tokenexchange/                     RFC 8693 dual-identity tokens

pkg/web/mid/                           HTTP middleware
├── auth.go                            RequireAuth, RequireRole
├── ratelimit.go                       per-IP throttle
└── tracing.go                         OTel span start

pkg/governance/                        bus-layer policies
├── policy.go                          Policy interface, CompositePolicy
├── rbac.go                            RBACPolicy
├── tenant.go                          TenantPolicy
├── classification.go                  ClassificationPolicy
├── prompt_injection.go                PromptInjectionPolicy
├── pii.go                             PIIBlockPolicy
├── required_metadata.go               RequiredMetadataPolicy
├── schema_policy.go                   SchemaPolicy
├── compliance.go                      ConsentPolicy, ExplainabilityPolicy
└── sovereignty.go                     (re-exports)

pkg/policy/dsl/                        adaptive policies in YAML
pkg/agent/                             agent contract
├── types.go                           Agent, Environment
├── risk.go                            RiskClass
├── tier.go                            Tier (Sketch/Prototype/Beta/Production)
└── skill.go                           SkillRegistry

pkg/crypto/                            envelope encryption
├── envelope.go                        Encryptor, EncryptedPayload
└── resolver.go                        EnvKeyResolver, KMSKeyResolver

pkg/storage/postgres/                  Postgres layer
├── tenant.go                          WithTenant, WithAdminContext, __admin__
└── migrations/0005_rls.sql            Row-Level Security on every tenant table

pkg/identity/                          DID + W3C Verifiable Credentials
pkg/compliance/                        audit chain + consent ledger
pkg/incidents/                         Annexure VI store + grading
pkg/safety/                            plugin chain + bias scorers
pkg/sovereignty/                       provider registry + classification gate
pkg/llm/                               wrappers: deadline, circuit, budget, router

cmd/red-team/                          adversarial corpus runner (CI)
cmd/bcp/                               BCP drill harness (CI)

tests/security_envelope_test.go        end-to-end defence-in-depth integration

pkg/web/handlers/                      HTTP surface
├── inventory.go                       /v1/ai-inventory (admin)
├── incidents.go                       /v1/incidents (admin)
├── aibom.go                           /v1/aibom (admin)
├── disclosures.go                     /v1/disclosures (public)
└── ui_security_test.go                UI contract for security primitives

docs/
├── ai-governance-security.md          THIS DOCUMENT
├── packages/postgres-rls.md
├── packages/governance-tenant.md
├── packages/oauth-token-exchange.md
├── packages/agent-tier.md
├── free-ai-mapping.md
├── operations.md
├── linkedin-article-security-complete.md
└── linkedin-article-agentic-security-operations.md

config/
├── ai-policy.example.yaml             board-approved policy (replace in deploy)
└── constitution.yaml                  7 Sutras
```

---

## Appendix B — Glossary

| Term | Meaning |
|---|---|
| **Annexure V** | RBI FREE-AI's board-approved AI policy template |
| **Annexure VI** | RBI FREE-AI's AI incident reporting form |
| **DEK** | Data Encryption Key — per-document AES key |
| **DID** | Decentralised Identifier (W3C) — agent/workload identity |
| **EDD** | Enhanced Due Diligence — high-risk KYC path |
| **FREE-AI** | "Framework for Responsible & Ethical Enablement of AI" — RBI report, Aug 2025 |
| **GUC** | Postgres Grand Unified Configuration variable — runtime setting |
| **HITL** | Human in the Loop |
| **KEK** | Key Encryption Key — wraps the DEK |
| **MARA** | Microsoft Multi-Agent Reference Architecture |
| **MCP** | Model Context Protocol — Anthropic spec for tool servers |
| **PEP** | Politically Exposed Person — sanctions/AML category |
| **RFC 8693** | OAuth 2.0 Token Exchange |
| **RLS** | Row-Level Security (Postgres) |
| **SDD** | Simplified Due Diligence — low-risk KYC path |
| **SPIFFE** | Secure Production Identity Framework For Everyone |
| **SVID** | SPIFFE Verifiable Identity Document — workload identity assertion |
| **WORM** | Write Once Read Many — for tamper-evident logs |

---

## Appendix C — Defence-in-depth invariants

These are the invariants the test suite enforces. If any breaks,
the security posture has regressed.

| # | Invariant | Enforced by |
|---|---|---|
| I1 | Every customer-facing route requires authentication | `pkg/web/router.go` chains `mid.RequireAuth` on every `/v1/*` group |
| I2 | Admin-only routes require the admin role | `pkg/web/router.go` chains `mid.RequireRole(auth.RoleAdmin)` |
| I3 | Every customer-facing message carries `tenant_id` | `pkg/web/handlers/*.go` set it; `TenantPolicy` denies if missing |
| I4 | `expected_tenant ≠ tenant_id` denies the message | `pkg/governance/tenant_test.go::TestTenantPolicyDeniesMismatch` |
| I5 | Cross-tenant DB read returns zero rows | RLS policy on the tenant table; `relforcerowsecurity = t` |
| I6 | A Sketch-tier agent cannot serve customer traffic | `tests/security_envelope_test.go::TestSecurityEnvelope_SketchTierIsBlocked` |
| I7 | An undeclared-tier agent defaults to Prototype | `pkg/agent/tier_test.go::TestTierOfDefaultsToPrototype` |
| I8 | A two-hop token exchange preserves user as Subject and stacks the actor chain | `tests/security_envelope_test.go::TestSecurityEnvelope_TokenExchangeAuditIdentity` |
| I9 | A fallback request carries the original tenant metadata | `tests/security_envelope_test.go::TestSecurityEnvelope_FallbackTriggers` |
| I10 | The audit chain detects tampering | `pkg/compliance/audit_test.go::TestAudit_ChainAndVerify` |
| I11 | RLS internals never leak into the public UI | `pkg/web/handlers/ui_security_test.go::TestUI_NoAdminFieldsInPublicHTML` |
| I12 | The `/ai-inventory` fetch is admin-gated in the JS | `pkg/web/handlers/ui_security_test.go::TestUI_InventoryFetchGatedByAdmin` |
| I13 | The `tier` field is present and stable in the inventory JSON | `pkg/web/handlers/ui_security_test.go::TestInventory_TierFieldStableJSONName` |
| I14 | An invalidated user has no cached exchanged tokens | `tests/security_envelope_test.go::TestSecurityEnvelope_InvalidatePropagates` |
| I15 | An agent ID is unique across the agent tree | `tests/agents_registry/` grep test |

Run all invariants:

```bash
go test ./... -count=1
```

A failing invariant is a security regression. Treat it the same way
you would treat a failing compile — revert first, debug second.
