# GCP Professional Cloud Security Engineer (PCSE) → Genie mapping

> **Audience:** the security reviewer who knows the GCP PCSE blueprint
> and wants to know how a non-GCP, on-prem-first, RBI FREE-AI-aligned
> agentic platform addresses the same concerns.
> **Source:** the official Google PCSE exam guide, sections 1–5 with
> weighted percentages 25/22/23/19/11.
>
> The point of this document is **not** to claim Genie is "GCP PCSE
> compliant." That's a category error — Genie isn't a GCP product.
> The point is to demonstrate, bullet by bullet, that Genie addresses
> the same security concerns the PCSE blueprint codifies, using
> application-layer primitives instead of Google Cloud products
> wherever the concern is portable.
>
> Status legend:
>
>   - ✅ implemented (file path + test name cited)
>   - 🟡 partial (what's done, what isn't)
>   - ⚪ not implemented (with honest reason — by design, roadmap, or N/A)
>
> If a row claims ✅ without a file/test anchor, treat it as a doc bug
> and open an issue.

---

## Table of contents

- [How to read this map](#how-to-read-this-map)
- [Section 1 — Configuring access (25%)](#section-1--configuring-access-25)
- [Section 2 — Securing communications and boundary protection (22%)](#section-2--securing-communications-and-boundary-protection-22)
- [Section 3 — Ensuring data protection (23%)](#section-3--ensuring-data-protection-23)
- [Section 4 — Managing operations (19%)](#section-4--managing-operations-19)
- [Section 5 — Supporting compliance requirements (11%)](#section-5--supporting-compliance-requirements-11)
- [Honest gaps and roadmap](#honest-gaps-and-roadmap)
- [Appendix: PCSE → FREE-AI cross-walk](#appendix-pcse--free-ai-cross-walk)

---

## How to read this map

For each PCSE bullet:

1. **Bullet** quoted verbatim from the exam guide.
2. **Genie analog** — the closest application-layer equivalent, or "out
   of scope: deployment platform" when the concern is GCP-infrastructure-
   specific.
3. **File path / test** — exactly where in the repo.
4. **Why this differs** — when the analog isn't a one-to-one (e.g., we
   ship application-layer perimeter where GCP ships network-layer
   perimeter).

The deployment-platform concerns — VPC, NGFW, Cloud Armor, IAP,
Interconnect — are intentionally out of scope. Genie runs on
whatever Kubernetes / VM / on-prem fabric the bank deploys it on;
the perimeter at that layer is the platform's responsibility, not
the application's.

---

## Section 1 — Configuring access (25%)

### 1.1 Managing Cloud Identity

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Configuring GCDS + SSO with a 3rd-party IdP | Genie *is* an IdP today (OAuth 2.1 + PKCE, OAuth Device Flow, WebAuthn passkeys); federated SSO *into* Genie from an external IdP not yet wired. | 🟡 |
| Managing a super administrator account | The admin role is the highest tier (`auth.RoleAdmin`); no concept of a super-admin tier above admin. The customer-facing routes do not grant admin role bypass on tenant policy. | 🟡 |
| Automating the user lifecycle management process | Signup, profile read/update, password change exposed via `/v1/users`; programmatic bulk lifecycle (deactivate-on-inactivity) is roadmap. | 🟡 |
| Administering user accounts and groups programmatically | `/v1/users` admin endpoints (admin-gated) — `pkg/web/handlers/users.go`. Group concept is the multi-role array on `User.Roles`. | ✅ |
| Configuring Workforce Identity Federation | Not implemented; would slot in as an alternate verifier on the JWT mid. | ⚪ |

**Anchors:** `pkg/auth/types.go` · `pkg/auth/jwt.go` · `pkg/auth/oauth2/` · `pkg/auth/oauth_device/` · `pkg/auth/webauthn/` · `pkg/web/handlers/users.go`

---

### 1.2 Managing service accounts

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Securing and protecting service accounts | `pkg/identity` ships DID (Ed25519) + W3C Verifiable Credential primitives for workload identity. SPIRE/SPIFFE wiring at deploy time is the missing piece. | 🟡 |
| Identifying scenarios requiring service accounts | Documented in `docs/ai-governance-security.md` §3.3 (workload identity) and §7 (dual-identity tokens). | ✅ |
| Creating, disabling, and authorizing service accounts | Agent IDs are stable strings declared in each `agents/<name>/` package; the registry indexes them; authorisation is per-message via the bus governance stack. | ✅ |
| Securing, auditing, and mitigating the usage of service account keys | DID private keys are never serialised; the issued credential carries only the public key. JWT signing key (`GENIE_JWT_SECRET`) is the symmetric analog and is documented in `operations.md` rotation playbook. | ✅ |
| Managing and creating short-lived credentials | JWT TTL defaults to 15 minutes; exchanged tokens (RFC 8693) inherit `min(token_exp, subject_exp) − safety_margin`. | ✅ |
| Configuring Workload Identity Federation | DID/VC primitives ship today; SPIRE server + mTLS at the proxy is roadmap. | 🟡 |
| **Managing service account impersonation** | **This is exactly what `pkg/auth/tokenexchange` implements via RFC 8693 — the exchanged token's Subject stays the user, Actor records the impersonating service. N-hop chains via `Actor.Nested`.** | ✅ |

**Anchors:** `pkg/identity/identity.go` · `pkg/auth/tokenexchange/exchange.go` · `pkg/auth/jwt.go::Issuer.TTL` · `docs/packages/oauth-token-exchange.md`

---

### 1.3 Managing authentication

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Creating a password and session management policy for user accounts | bcrypt hashing (`pkg/auth/password.go`); JWT TTL 60 min (password tokens) / 15 min (passkey tokens, OAuth flows); Invalidate on logout via `tokenexchange.Service.Invalidate`. | ✅ |
| Setting up SAML and OAuth | OAuth 2.1 + PKCE ✅ (`pkg/auth/oauth2`); OAuth Device Flow ✅ (`pkg/auth/oauth_device`); SAML ⚪. | 🟡 |
| Configuring and enforcing 2-step verification | WebAuthn passkeys (Ed25519) ✅ (`pkg/auth/webauthn`). The architectural choice is passkeys-as-MFA (FIDO2) over TOTP-based 2SV — passkeys are phishing-resistant; TOTP is not. | ✅ |

**Anchors:** `pkg/auth/password.go` · `pkg/auth/oauth2/` · `pkg/auth/oauth_device/` · `pkg/auth/webauthn/`

---

### 1.4 Managing and implementing authorization controls

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Privileged roles + separation of duties | Three-tier role model (`user` / `advisor` / `admin`); HTTP gate via `mid.RequireRole`; bus gate via `RBACPolicy`. The two layers catch different mistakes. | ✅ |
| IAM and ACL permissions | Routes use role gates; messages use type+role tables; resources (documents, accounts) use Postgres RLS. Three layers, three independent failure modes. | ✅ |
| IAM conditions / deny policies | `governance.CompositePolicy` returns deny-on-first; the policy DSL (`pkg/policy/dsl`) supports conditional rules in YAML the risk team can author. | ✅ |
| Least privilege at org/folder/project/resource | Application-level: tenant isolation per message type via `TenantPolicy`; per-row via Postgres RLS. Folder/project hierarchy isn't modelled — Genie's tenant model is flat (user_id today; org_id roadmap). | 🟡 |
| Configuring Access Context Manager | Application analog: `pkg/sovereignty.ProviderRegistry.Allowed(provider, classification)` gates which provider can receive which sensitivity tier. The Google product handles network-context conditions; the Genie analog handles data-context conditions. | 🟡 |
| Applying Policy Intelligence | Audit log analytics via `pkg/observability/bq` (warehouse sink) + the BCP drill harness flag agents with excessive denials. Policy-recommendation tooling not implemented. | 🟡 |
| Managing permissions through groups | Roles are stored as `[]Role` on User — a user belongs to multiple roles concurrently. Group abstraction over roles not implemented (would be a separate `groups` table mapping group → roles). | 🟡 |
| Identifying use cases and configuring **Privileged Access Manager** (time-bound elevation) | **Not implemented.** Would slot in as `pkg/auth.ElevationGrant{role, expires_at, audit_id}` — time-bound `WithAdminContext` with automatic revoke. **Listed as a gap below.** | ⚪ |

**Anchors:** `pkg/web/mid/auth.go` · `pkg/governance/rbac.go` · `pkg/governance/policy.go` · `pkg/policy/dsl/` · `pkg/governance/tenant.go` · `pkg/storage/postgres/migrations/0005_rls.sql` · `pkg/sovereignty/sovereignty.go`

---

### 1.5 Defining the resource hierarchy

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Managing folders and projects at scale | N/A — Genie has no folder hierarchy. The tenant model is flat (user_id) with a roadmap to org_id for corporate banking. | ⚪ |
| Managing pre-built or custom organization policies | `config/ai-policy.example.yaml` is the board-approved policy YAML; the DSL (`pkg/policy/dsl`) lets the risk team author rules without code releases. | ✅ |
| Using the resource hierarchy for access control and permissions inheritance | N/A — no hierarchy to inherit through. Permission inheritance is via the role array, not the resource tree. | ⚪ |

**Anchors:** `config/ai-policy.example.yaml` · `pkg/policy/dsl/` · `docs/packages/policy-dsl.md`

---

## Section 2 — Securing communications and boundary protection (22%)

The vast majority of this section is GCP-infrastructure-specific (Cloud
NGFW, IAP, Cloud Armor, VPC peering, HA VPN, Interconnect, Private
Service Connect). The Genie analog is application-layer perimeter and
classification-based segmentation. Out-of-scope items are listed for
completeness.

### 2.1 Designing and configuring perimeter security

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Configuring network perimeter controls (NGFW, IAP, load balancers, CA Service) | Out of scope — deployment platform | ⚪ |
| **Setting up application layer (L7) inspection** | **`PromptInjectionPolicy`** (jailbreak/injection patterns), **`PIIBlockPolicy`** (Aadhaar/PAN/mobile regex), **`ClassificationPolicy`** (sensitivity tier gate), **`SchemaPolicy`** (JSON schema validation per message type), all chained via `CompositePolicy`. | ✅ |
| Differentiating between private and public IP addressing | Out of scope — deployment platform | ⚪ |
| Configuring web application firewalls (Cloud Armor) | Out of scope; application-layer rate limiting via `pkg/web/mid/ratelimit.go`. | ⚪ + ✅ |
| Deploying Secure Web Proxy | Out of scope — deployment platform | ⚪ |
| Configuring Cloud DNS security settings | Out of scope — deployment platform | ⚪ |
| **Continually monitoring and restricting configured APIs** | Per-route role gates (`RequireRole`) + per-message bus RBAC + rate-limit middleware + OTel spans on every span. | ✅ |

**Anchors:** `pkg/governance/prompt_injection.go` · `pkg/governance/pii.go` · `pkg/governance/classification.go` · `pkg/governance/schema_policy.go` · `pkg/governance/policy.go::CompositePolicy` · `pkg/web/mid/ratelimit.go` · `pkg/web/mid/tracing.go`

---

### 2.2 Configuring boundary segmentation

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Configuring security properties of a VPC network | Out of scope — deployment platform | ⚪ |
| Configuring network isolation and data encapsulation for N-tier applications | Out of scope — deployment platform | ⚪ |
| **Identifying use cases and configuring VPC Service Controls** | Application analog: **`pkg/sovereignty.ProviderRegistry.Allowed(provider, classification)`** — classification → provider allowlist. A `pii`-classified message cannot reach a provider whose region isn't in the allowlist for `pii`. The Google product handles data-egress at the network boundary; the Genie analog handles it at the routing decision. | 🟡 (analog, not equivalent) |

**Anchors:** `pkg/sovereignty/sovereignty.go` · `pkg/llm/router.go` (the LLM router consults the registry before dispatching) · `docs/linkedin-article-sovereign-ai.md`

---

### 2.3 Establishing private connectivity

Every bullet in this sub-section is GCP-infrastructure-specific
(Shared VPC, VPC peering, HA VPN, Cloud Interconnect, Private Google
Access, Private Service Connect, Cloud NAT). **Out of scope — deployment
platform.**

For an on-prem deployment, the analog is the bank's existing
network-segmentation policy plus Genie's `pkg/sovereignty` provider
allowlist — which collectively answer the same question ("can this data
class reach this destination?") at the application layer.

---

## Section 3 — Ensuring data protection (23%)

This is Genie's home turf. Every sub-section maps cleanly.

### 3.1 Protecting sensitive data and preventing data loss

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Configuring Sensitive Data Protection (SDP) — discovering and redacting PII | `pkg/governance/pii.go` — `PIIBlockPolicy` denies messages whose content matches Aadhaar / PAN / mobile-number regex. The patterns are India-specific by design (RBI FREE-AI scope); a deployment in another jurisdiction would extend the regex set. | ✅ |
| Configuring pseudonymization | Incident reporting uses `incidents.affected_id` (opaque pseudonymous id) so the regulator gets the report without seeing the customer's name or PAN; the de-pseudonymisation key is held separately. | ✅ |
| Configuring format-preserving encryption | **Not implemented.** Cases that need it (Aadhaar must remain 12 digits, account number must keep its length): `pkg/crypto` ships envelope AES-GCM only. **Listed as a gap.** | ⚪ |
| Restricting access to Google Cloud data services (BigQuery, Cloud Storage, Cloud SQL) | Application analog: **Postgres RLS** (`pkg/storage/postgres/tenant.go` + `migrations/0005_rls.sql`) plus the bus-layer `TenantPolicy`. The combined defence in depth is documented in `ai-governance-security.md` §6. | ✅ |
| Securing secrets with Secret Manager | KEK secrets via `pkg/crypto.KMSKeyResolver` (with a `KMSClient` interface — production hosts implement against AWS KMS, GCP KMS, Vault). JWT secret + DB DSN via env vars; general purpose secret store with rotation is the operational platform's job. | 🟡 |
| Protecting and managing compute instance metadata | Out of scope — deployment platform | ⚪ |

**Anchors:** `pkg/governance/pii.go` · `pkg/incidents/incidents.go` · `pkg/storage/postgres/tenant.go` · `pkg/storage/postgres/migrations/0005_rls.sql` · `pkg/crypto/envelope.go` · `pkg/crypto/resolver.go` · `docs/packages/postgres-rls.md` · `docs/packages/governance-tenant.md`

---

### 3.2 Managing encryption at rest, in transit, and in use

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Default encryption, CMEK, EKM | **Envelope encryption** (`pkg/crypto/envelope.go`): per-document AES-256-GCM DEK wrapped by a KEK. The `KMSKeyResolver` + `KMSClient` interface is the CMEK / EKM equivalent — the host implements `KMSClient` against AWS KMS, GCP KMS, Vault, or an HSM. The `EnvKeyResolver` is the dev/staging fallback that reads the KEK from `GENIE_KEK_BASE64`. | ✅ |
| Software vs hardware keys | Software via `EnvKeyResolver`; hardware via `KMSKeyResolver` pointed at a KMS that's backed by an HSM (AWS KMS HSM, GCP Cloud HSM). The interface is the same shape both ways. | ✅ |
| Key rotation / revocation / import | Per-row `kek_id` is stored alongside every encrypted payload; rotation = mint a new KEK, switch the resolver's active key id; old documents continue to decrypt with their original KEK via the resolver's `Unwrap(kekID, ...)` lookup. **Lazy rewrap background job is roadmap.** | 🟡 |
| Applying encryption methods to various use cases | Documented per-table in `ai-governance-security.md` §10. | ✅ |
| Configuring object lifecycle policies for Cloud Storage | Application analog: `pkg/storage/postgres/retention.go` — periodic purge of rows whose `expires_at` has passed. | ✅ |
| Enabling Confidential Computing | Out of scope — deployment platform | ⚪ |

**Anchors:** `pkg/crypto/envelope.go` · `pkg/crypto/resolver.go` · `pkg/storage/postgres/retention.go` · `docs/ai-governance-security.md` §10 · `docs/operations.md` (KEK rotation runbook)

---

### 3.3 Securing AI workloads — **the bullet Genie was built for**

| PCSE bullet | Genie analog | Status |
|---|---|---|
| **Implementing security and privacy controls for AI/ML systems to protect against unintentional exploitation of data or models** | **This is the entire `ai-governance-security.md` story.** The eleven-layer defence-in-depth envelope is precisely the implementation of this bullet. Headline pieces: prompt-injection policy, safety plugin chain (jailbreak + toxicity + secrets scoring), classification + sovereignty gate on every LLM call, RLS at the DB, RFC 8693 dual-identity audit, hash-chained tamper-evident audit, agent tier promotion gate, fallback agents for BCP, adversarial corpus in CI. | ✅✅✅ |
| Determining security requirements for IaaS-hosted and PaaS-hosted training models | Provider-level: `pkg/sovereignty.ProviderRegistry` declares which models are allowed for which data classes. Operational: agents that train (none in current Genie; all are inference-only) would inherit the same controls. | 🟡 |
| Implementing security controls for Vertex AI | Vertex-specific controls not implemented — Genie ships **on-prem Ollama** by default. The patterns transfer: the safety plugin chain, the classification gate, the audit hooks all work identically with a Vertex-backed provider; only the provider driver changes. | 🟡 |

**Anchors:** `docs/ai-governance-security.md` (entire document) · `pkg/governance/prompt_injection.go` · `pkg/safety/` (plugin chain + bias) · `pkg/governance/classification.go` · `pkg/sovereignty/sovereignty.go` · `pkg/llm/router.go` · `pkg/llm/budget.go` · `pkg/llm/circuit.go` · `pkg/llm/deadline.go` · `pkg/auth/tokenexchange/` · `pkg/storage/postgres/migrations/0005_rls.sql` · `pkg/compliance/audit.go` · `pkg/agent/tier.go` · `cmd/red-team/` · `cmd/bcp/`

---

## Section 4 — Managing operations (19%)

### 4.1 Automating infrastructure and application security

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Automating security scanning for CVEs through CI/CD | Out of scope (container CVE scan = deployment platform). Application-level: `make red-team` runs the adversarial corpus on every PR (`cmd/red-team/`); a failed probe is a build failure. | 🟡 (CVE ⚪ ; adversarial ✅) |
| **Configuring Binary Authorization to secure GKE clusters or Cloud Run** | Application analog: the **agent tier promotion gate** (`pkg/agent.Tier`) is "binary authorisation for agents." Only an agent that declares `TierProduction` (and passes the promotion checklist in `operations.md`) is eligible for customer-facing dispatch. Default-to-Prototype fails closed. | 🟡 (analog) |
| Automating VM / container image creation | Out of scope — deployment platform | ⚪ |
| Managing policy and drift detection at scale | The security envelope tests (`tests/security_envelope_test.go` — 6 end-to-end cases) act as drift detection: a failed invariant = security regression. CI runs them on every PR. The 15 defence-in-depth invariants in `ai-governance-security.md` Appendix C are the policy-drift signals. | ✅ (app-level) |

**Anchors:** `cmd/red-team/` · `pkg/agent/tier.go` · `tests/security_envelope_test.go` · `docs/ai-governance-security.md` Appendix C · `docs/operations.md` (tier promotion checklist) · `docs/packages/agent-tier.md`

---

### 4.2 Configuring logging, monitoring, and detection

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Configuring and analysing network logs (Cloud NGFW, VPC flow, Packet Mirroring, Cloud IDS, Log Analytics) | Out of scope — deployment platform | ⚪ |
| **Designing an effective logging strategy** | OTel spans on every governance decision (`governance.evaluate`), every agent handle (`agent.handle`), every LLM call (`llm.call`). Metrics on bus traffic, policy denials, agent errors, handle duration, LLM tokens/cost/latency. Documented at `ai-governance-security.md` §16. | ✅ |
| Logging, monitoring, responding to, and remediating security incidents | `pkg/incidents/incidents.go` implements the RBI FREE-AI Annexure VI form. Auto-recording sources: policy deny, agent error, LLM budget breach, circuit-breaker trip, safety flag, KYC sanctions hit, payment rejection. Grading via `pkg/incidents/grading.go` (escalates by failure-mode count in a rolling window). Endpoint: `/v1/incidents` (admin-only). | ✅ |
| Designing secure access to logs | Admin-only routes via `mid.RequireRole(RoleAdmin)` on `/v1/incidents`, `/v1/ai-inventory`, `/v1/aibom`, `/v1/audit`. The audit reader runs under `WithAdminContext` — the only legitimate cross-tenant key. | ✅ |
| Exporting logs to external security systems | `pkg/observability/bq` — buffered JSONL sink for BigQuery / Snowflake / Kafka. Documented at `docs/packages/observability-bq.md`. | ✅ |
| **Configuring and analysing Google Cloud Audit Logs and data access logs** | **Hash-chained tamper-evident audit log** (`pkg/compliance/audit.go`). Each entry references the previous row's SHA-256 hash; `Verify()` walks the chain and detects any tampering. Every governance-relevant action writes to it (policy denials, KYC verdicts, payment routing, consent grants, LLM budget breaches, token-exchange calls). | ✅ |
| Configuring log exports (log sinks and aggregated sinks) | Same as the export bullet above — `pkg/observability/bq` provides the buffered async dispatcher with per-event-shape adapter for warehouse landing tables. | ✅ |
| **Configuring and monitoring Security Command Center** | Application analog: **the admin dashboard** — `/v1/ai-inventory` (live agent list with risk_class + tier), `/v1/incidents` (Annexure VI form), `/v1/aibom` (AI bill of materials with model provenance), `/v1/audit` (hash-chained log + Verify endpoint). The UI contract tests pin the admin-only gating so these never leak to non-admin sessions. | ✅ |

**Anchors:** `pkg/observability/` · `pkg/observability/bq/` · `pkg/compliance/audit.go` · `pkg/incidents/` · `pkg/web/handlers/inventory.go` · `pkg/web/handlers/incidents.go` · `pkg/web/handlers/aibom.go` · `pkg/web/handlers/ui_security_test.go` · `docs/ai-governance-security.md` §11, §12, §16

---

## Section 5 — Supporting compliance requirements (11%)

### 5.1 Adhering to regulatory and industry standards

| PCSE bullet | Genie analog | Status |
|---|---|---|
| Determining technical needs relative to compute, data, network, and storage | Documented in `docs/operations.md` (env vars, resource needs, deployment shapes — CLI / API-only / full stack). | ✅ |
| **Evaluating the shared responsibility model** | Documented explicitly in `ai-governance-security.md` §19.2 ("Things that are explicitly NOT Genie's job") — TLS / ingress / DDoS / WAF / kernel CVEs are the deployment platform's responsibility; the model itself is the regulator-approved bake-off's responsibility; the board policy text is the board's responsibility. | ✅ |
| Configuring security controls within cloud environments to support compliance requirements (Assured Workloads, org policies, Access Transparency, Access Approval, regionalisation) | Org policies ✅ (`config/ai-policy.example.yaml` + DSL). Regionalisation of data and services ✅ (`pkg/sovereignty.ProviderRegistry` enforces classification → allowed region/provider on every LLM call). Access Transparency / Access Approval — the application analog is the hash-chained audit log + admin-only audit reader; Google's exact products are GCP-specific. | ✅ (substance) / 🟡 (specific products) |
| Determining the Google Cloud environment in scope for regulatory compliance | Equivalent in Genie: the deployment scope (which agents, which data classes, which regions) is the FREE-AI Annexure V board policy — captured in `config/ai-policy.example.yaml`. | ✅ |
| **Mapping compliance requirements to Google Cloud services and security controls** | **`docs/free-ai-mapping.md`** maps every one of the 26 RBI FREE-AI recommendations to a file path in this repo. This document is the GCP-PCSE equivalent of that map. Together they give a reviewer end-to-end coverage. | ✅ |

**Anchors:** `docs/operations.md` · `docs/ai-governance-security.md` §19.2 · `pkg/sovereignty/sovereignty.go` · `config/ai-policy.example.yaml` · `docs/free-ai-mapping.md`

---

## Honest gaps and roadmap

Gaps where a PCSE-aligned reviewer would expect more than Genie currently
ships. In priority order:

### Gap 1 — Privileged Access Manager analog (PCSE 1.4)

**What's missing.** Time-bound elevation: a workflow where an engineer
requests admin role for a specific operation, gets it for a bounded
window, and the role is automatically revoked at expiry with a full
audit entry.

**Proposed shape.**

```go
// pkg/auth/elevation/elevation.go (proposed)
type Grant struct {
    Subject     string
    Role        auth.Role
    Reason      string
    GrantedBy   string         // who approved
    ExpiresAt   time.Time
    AuditID     string         // hash-chain entry id at grant time
}
func (s *Service) Request(ctx, subject, role, reason string) (*Grant, error)
func (s *Service) Approve(ctx, grantID, approverID string) error
func (s *Service) ActiveFor(ctx, subject string) []Grant
```

**LoC estimate:** ~250 in `pkg/auth/elevation/` + test + doc.

### Gap 2 — Format-preserving encryption (PCSE 3.1)

**What's missing.** Encrypted Aadhaar that's still 12 digits, encrypted
account number that still passes Luhn — for cases where downstream
schemas can't accept ciphertext.

**Proposed shape.**

```go
// pkg/crypto/fpe/fpe.go (proposed)
type Mode int
const (
    ModeDigits Mode = iota   // preserve digit length
    ModeAlpha                 // preserve length within [A-Z0-9]
)
type Cipher struct { Key []byte; Mode Mode }
func (c *Cipher) Encrypt(plaintext string) (string, error)
func (c *Cipher) Decrypt(ciphertext string) (string, error)
```

Implementation: FF3-1 (NIST SP 800-38G) over AES. **LoC estimate:** ~400
with the FF3-1 algorithm + a NIST test vector check.

### Gap 3 — SAML verifier (PCSE 1.3)

**What's missing.** Many banks have a SAML 2.0 IdP they want Genie to
federate against (Okta, ADFS, internal). Today Genie consumes OAuth and
its own JWT; SAML is the missing third format.

**Proposed shape.** New `pkg/auth/saml/` with assertion parsing, XML
signature verification, and an HTTP middleware that exchanges a SAML
assertion for a Genie JWT. **LoC estimate:** ~600 (XML sig is the
bulk).

### Gap 4 — KEK rewrap background job (PCSE 3.2)

**What's missing.** After a KEK rotation, older rows still point at the
old `kek_id`. Today the resolver handles this by keeping the old KEK
reachable; the lazy rewrap job that re-wraps older DEKs with the new
KEK is roadmap.

**Proposed shape.** A periodic job in `pkg/storage/postgres/rewrap.go`
that scans for rows with `kek_id != active_kek_id`, decrypts the DEK,
re-wraps under the active KEK, updates the row, writes one audit entry
per rewrapped row. **LoC estimate:** ~150 + test that exercises a full
rotate-and-rewrap cycle.

### Lower-priority gaps

- **Workload Identity Federation full wiring (PCSE 1.2).** DID/VC
  primitives ship; SPIRE server + mTLS at the proxy is what's missing.
  Mostly a deployment artefact, not Go code — adds an Envoy proxy with
  SPIRE-issued SVIDs in the cluster.
- **Group abstraction over roles (PCSE 1.4).** A `groups` table + a
  group → roles join. Useful in multi-bank deployments where the same
  user model is shared across orgs. ~200 LoC.
- **Custom organisation modules for Security Health Analytics (PCSE 4.1).**
  GCP-specific tooling; the application analog is the existing
  envelope-tests, but a "live posture dashboard" that scores the
  policy stack would be a nice add. ~300 LoC + UI.

---

## Appendix: PCSE → FREE-AI cross-walk

The PCSE blueprint and the RBI FREE-AI report concern themselves with
the same security substance, expressed in different vocabularies. The
table below cross-references the headline overlaps. The full FREE-AI
map is at [`free-ai-mapping.md`](free-ai-mapping.md).

| PCSE section | RBI FREE-AI rec | Shared concern |
|---|---|---|
| 1.4 IAM + least privilege | Rec 14 (Board-approved policy) | Who is allowed to do what; codified |
| 1.4 Privileged Access Manager | Rec 22 (Audit) | Time-bound elevation logged |
| 2.1 L7 inspection | Rec 19 (Cybersecurity) | Application-layer perimeter |
| 2.2 VPC Service Controls | Rec 16 (Data residency) | Data egress constraints by classification |
| 3.1 DLP | Rec 15 (Data lifecycle) + Rec 19 (Cybersecurity) | PII detection + redaction |
| 3.2 Encryption at rest | Rec 15 (Data lifecycle) | Encrypted, key-rotatable storage |
| 3.3 Securing AI workloads | Rec 16, 17, 19, 20, 21, 22, 23, 25 | The entire FREE-AI mandate |
| 4.2 Logging strategy | Rec 22 (Tamper-evident audit) | Hash chain + admin-only reader |
| 4.2 Incident response | Rec 22 (Annexure VI) | Structured incident form + grading |
| 5.1 Shared responsibility | Rec 8 (Graded liability) | Who is liable for what failure |
| 5.1 Compliance mapping | Rec 14 (Board policy) + Rec 23 (Inventory) | Live mapping from rule to artefact |

A reviewer who knows PCSE can read this map → FREE-AI → file path; a
reviewer who knows FREE-AI can read the FREE-AI map → file path → this
map for the PCSE counterpart. The two indexes are mutually reinforcing.

---

## See also

- [`ai-governance-security.md`](ai-governance-security.md) — the canonical
  Genie security reference; every PCSE row above has a deeper section
  there.
- [`free-ai-mapping.md`](free-ai-mapping.md) — RBI FREE-AI 26 recommendations
  → file path map.
- [`operations.md`](operations.md) — runbook including the KEK rotation,
  the tier promotion checklist, and the security-envelope verification
  command.
- [`packages/postgres-rls.md`](packages/postgres-rls.md),
  [`packages/governance-tenant.md`](packages/governance-tenant.md),
  [`packages/oauth-token-exchange.md`](packages/oauth-token-exchange.md),
  [`packages/agent-tier.md`](packages/agent-tier.md) — the four Q1
  security primitives' per-package detail.
