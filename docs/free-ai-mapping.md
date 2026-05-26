# RBI FREE-AI Mapping — Recommendation × Code

> The exhaustive cross-walk from each of the 26 FREE-AI recommendations
> to the Genie file paths, agents, packages, endpoints, and tests that
> implement it. Companion to the table in the root README.

Source report: [RBI Framework for Responsible and Ethical Enablement of AI (Aug 2025)](https://rbidocs.rbi.org.in/rdocs/PublicationReport/Pdfs/FREEAIR130820250A24FF2D4578453F824C72ED9F5D5851.PDF)

---

## Legend

- ✅ — implemented, covered by tests
- 🟡 — partial: scaffolding exists, work in progress
- ⚪ — regulator / SRO action, outside Regulated Entity scope

---

## Pillar 1 — Foundations

### Rec 1 — Establish a National Centre of Excellence ⚪

Regulator scope. Genie contributes by being open-source and citation-friendly.

### Rec 2 — AI Innovation Sandbox ✅

The sandbox **is** the production code, with mocked I/O.

| Artefact | Where |
|---|---|
| In-process pipeline | `cmd/genie/main.go` |
| Sample CSV | `data/sample.csv` |
| Run command | `go run ./cmd/genie` (no Postgres, no network, ~30s) |
| Test | `tests/` end-to-end integration |

A risk analyst can clone the repo and see the full pipeline produce a
financial report in 30 seconds, offline.

### Rec 3 — Build Indigenous Datasets ⚪

Regulator + sector action. Genie consumes whatever indigenous datasets
become available (federated learning support in `pkg/federated`).

### Rec 4 — Indigenous AI Models ✅

Every LLM provider declares `Region()`. Residency policy denies PII
bound for a non-home region before the LLM call.

| Artefact | Where |
|---|---|
| Ollama provider (on-prem default) | `pkg/llm/ollama.go` |
| Hosted providers with region tags | `pkg/llm/anthropic.go`, `pkg/llm/openai.go`, `pkg/llm/gemini.go` |
| Residency policy | `pkg/governance/sovereignty.go` |
| Provider registry | `pkg/sovereignty/` |

Hot-path traffic (PAN, account, transaction, holding) routes to Ollama;
cold-path (macro research, generic education) can route to hosted.

### Rec 5 — Capacity Building ⚪

External / educational programs. Genie's docs (`docs/agents/`,
`docs/packages/`, this file) double as training material.

---

## Pillar 2 — Policy & Governance

### Rec 6 — Adaptive Policies ✅

Two forms:

| Mechanism | Where |
|---|---|
| YAML loader | `pkg/policy/` + `config/ai-policy.example.yaml` |
| CEL-style DSL for rule authoring | `pkg/policy/dsl/` (see [packages/policy-dsl.md](packages/policy-dsl.md)) |

Risk team edits YAML; engineering doesn't ship code.

### Rec 7 — Inter-regulatory Coordination ⚪

Regulator scope.

### Rec 8 — Graded Liability ✅

| Artefact | Where |
|---|---|
| Severity grading function | `pkg/incidents.Grade()` |
| Auto-incident on policy deny | `pkg/orchestration/orchestrator.go` |
| Annexure VI form generator | `pkg/incidents/annexure_vi.go` |
| High-grade examples | `agents/kyc_orchestrator` (sanctions → high), `agents/payment_orchestrator` (rejects → medium) |

### Rec 9 — Sectoral AI Code of Conduct ⚪

SRO action.

### Rec 10 — Embed AI in Annual Policy Reviews ⚪

RE governance process.

### Rec 11 — AI in RBI Supervisory Toolkit ⚪

Regulator scope.

### Rec 12 — Industry-Regulator Forums ⚪

External forum.

### Rec 13 — Reportable Incidents Framework ⚪

Regulator definition. Genie implements the reporting side via Rec 22.

---

## Pillar 3 — Capacity

### Rec 14 — Board-Approved AI Policy (Annexure V) ✅

| Artefact | Where |
|---|---|
| Policy template (Annexure V shape) | `config/ai-policy.example.yaml` |
| Loader | `pkg/policy/loader.go` |
| Hash-logged at boot | `cmd/api/main.go` |
| `board_approved_on`, `owner` fields | inside the YAML |

The board signs off on the file the system loads. No translation layer.

---

## Pillar 4 — Adoption

### Rec 15 — Data Lifecycle Governance ✅

| Artefact | Where |
|---|---|
| Envelope AES-256-GCM | `pkg/crypto/envelope.go` |
| KMS-pluggable KEK resolver | `pkg/crypto.KMSKeyResolver` |
| Per-row `kek_id` (supports rotation) | `pkg/storage/postgres/migrations/` |
| Retention job (`expires_at` purge every 6h) | `pkg/storage/postgres/retention.go` |
| Document lifecycle | `pkg/web/handlers/documents.go` |
| **Postgres Row-Level Security (DB-enforced tenant isolation)** | `pkg/storage/postgres/migrations/0005_rls.sql` + `pkg/storage/postgres/tenant.go` (`WithTenant` / `WithAdminContext`) |
| **Bus-layer TenantPolicy (defence in depth with RLS)** | `pkg/governance/tenant.go` |

The RLS + bus pairing is the Q1 hardening's two-pair defence: a bug
in either layer is caught by the other. See
[packages/postgres-rls.md](packages/postgres-rls.md) and
[packages/governance-tenant.md](packages/governance-tenant.md).

### Rec 16 — AI System Governance + Autonomous Systems ✅

| Artefact | Where |
|---|---|
| `RiskLevel()` per agent | `pkg/agent/risk.go` |
| Orchestrator ceiling enforcement | `pkg/orchestration/orchestrator.go` |
| Deadline wrapper | `pkg/llm/deadline.go` |
| Circuit-breaker wrapper | `pkg/llm/circuit.go` |
| Budget wrapper | `pkg/llm/budget.go` |
| Bounded ReAct/Reflexion loops | `pkg/reasoning/` |

A high-risk autonomous loop can't burn through your LLM budget — the
wrapper cuts it off.

### Rec 17 — Product Approval ✅

The Q1 hardening promoted this from 🟡 to ✅ via the four-tier
promotion model (`pkg/agent.Tier`). Each promotion (Sketch → Prototype
→ Beta → Production) is a documented decision, and the dispatch gate
refuses customer-facing traffic to anything below Production. The
risk team reads the tier off `/v1/ai-inventory`.

| Artefact | Where |
|---|---|
| Four-tier promotion model | `pkg/agent/tier.go` (`TierSketch`, `TierPrototype`, `TierBeta`, `TierProduction`) |
| Default-to-Prototype dispatch gate (fail closed) | `pkg/agent.TierOf` |
| Tier column on inventory | `pkg/web/handlers/inventory.go::InventoryItem.Tier` |
| End-to-end gate test | `tests/security_envelope_test.go::TestSecurityEnvelope_SketchTierIsBlocked` |
| Promotion checklist | [`docs/operations.md`](operations.md) — Q1 hardening section |
| Risk class per agent | `pkg/agent.RiskOf(a)` |
| AIBOM with audit timestamps | `pkg/aibom/` |

See [packages/agent-tier.md](packages/agent-tier.md) for the model in
detail.

### Rec 18 — Consumer Protection ✅

| Artefact | Where |
|---|---|
| AI disclosure banner | `config/ai-policy.example.yaml` → `consumer.ai_disclosure_banner` |
| Public disclosure endpoint | `GET /v1/disclosures` |
| First SSE event on `/v1/ask/stream` | `pkg/web/handlers/ask.go` |
| Disclaimer on every agent output | every `Decision` / `Verdict` / `Response` type has a `Disclaimer` field |

### Rec 19 — Cybersecurity ✅

| Concern | Implementation |
|---|---|
| Authn | `pkg/auth/jwt.go` (HS256 stdlib) + `pkg/auth/webauthn` (passkeys) |
| Authz | `pkg/web/mid.RequireRole` + `pkg/governance.RBACPolicy` (defence in depth) |
| Rate limit | `pkg/web/mid.RateLimit` |
| Prompt injection | `pkg/governance/prompt_injection.go` |
| PII regex | `pkg/governance/pii.go` |
| Session anomaly | `agents/cyber_guardian` |
| OAuth 2.1 + PKCE | `pkg/auth/oauth2` |
| OAuth Device Flow | `pkg/auth/oauth_device` |

### Rec 20 — Red Teaming ✅

| Artefact | Where |
|---|---|
| Probe corpus | `cmd/red-team/` |
| CI gate | `make red-team` in CI |
| Live policy — not mocked | `cmd/red-team/main.go` reads `GENIE_AI_POLICY` |

```bash
make red-team
# OK: all probes denied / allowed as expected.
```

### Rec 21 — BCP for AI ✅

| Artefact | Where |
|---|---|
| Fallback agents | `agents/fallback/` |
| Orchestrator hook | `Orchestrator.SetFallback(primaryID, fallbackAgent)` |
| CI drill | `make bcp-drill` (forces `portfolio_advisor` failure) |
| Pattern doc | `docs/agents/README.md` + each agent doc's "anti-patterns" |

### Rec 22 — AI Incident Reporting (Annexure VI) ✅

| Trigger | Where it auto-records |
|---|---|
| Governance policy deny | `pkg/orchestration/orchestrator.go` |
| Agent panic / error above grade threshold | `pkg/orchestration/orchestrator.go` |
| LLM budget breach | `pkg/llm/budget.go` |
| Circuit-breaker trip | `pkg/llm/circuit.go` |
| Safety scorer flag above threshold | `pkg/safety/` |
| KYC sanctions hit | `agents/kyc_orchestrator` `rejectVerdict` |
| Payment rejection | `agents/payment_orchestrator` `reject()` |
| **Dual-identity attribution on every audited call** | `pkg/auth/tokenexchange/exchange.go` — `Subject = user`, `Actor = agent`, N-hop chain via `Actor.Nested` |

Output: Annexure VI-shaped JSON in the incident log, hash-chained into
the audit log. With RFC 8693 dual-identity tokens, a reviewer reading
a single audit entry can answer both "which user initiated" and "what
was the proximate agent caller, and which services touched the request
in between." See [packages/oauth-token-exchange.md](packages/oauth-token-exchange.md).

### Rec 23 — AI Inventory ✅

Live, generated on demand from the registry.

```bash
curl -H "Authorization: Bearer $ADMIN" /v1/ai-inventory | jq .
```

`pkg/web/handlers/inventory.go` reads `registry.List(ctx)` →
`InventoryItem[]` with id, name, capabilities, risk_class, **tier**,
has_fallback. The `tier` column is what the risk team scans to spot
non-production agents serving customer traffic (Rec 17). The UI
contract test (`pkg/web/handlers/ui_security_test.go::TestInventory_ListIncludesTier`)
pins the JSON field name against accidental drift.

### Rec 24 — AI Audit Framework 🟡

Today:

- LLM-as-judge auditor (`agents/auditor`) scores every outbound message against the 7 Sutras
- OTel distributed trace per request (every hop, every LLM call)
- AIBOM (`pkg/aibom/`) with CycloneDX 1.6 ML-BOM + Ed25519 signing
- Warehouse sink (`pkg/observability/bq/`) for long-horizon analytics

Roadmap:

- Signed JSONL export of audit log per quarter
- External auditor portal (RBAC-controlled subset of `/v1/audit/log`)

### Rec 25 — Disclosures ✅

`GET /v1/disclosures` is the public, unauthenticated endpoint.

```bash
curl https://api.example.in/v1/disclosures | jq .
```

Returns active policy version, FREE-AI principles, agent counts by risk
class, AI disclosure banner.

### Rec 26 — AI Toolkit ✅

| Artefact | Where |
|---|---|
| 7-Sutra Scorecard | `pkg/toolkit/scorecard.go` |
| Safety plugin chain | `pkg/safety/plugin.go` (see [packages/safety-plugins.md](packages/safety-plugins.md)) |
| Adversarial probe runner | `cmd/red-team/` |
| BCP drill | `make bcp-drill` |

---

## Things that are NOT Genie's job

Recommendations 1, 3, 5, 7, 9–13 are regulator / SRO actions. Genie
adopts them as they're published — e.g. when the sectoral AI code of
conduct (Rec 9) lands, the relevant clauses become entries in the
policy YAML.

---

## "Show me the file" — quick spot-check

```bash
# Rec 4 — Indigenous models, residency
grep -n "Region()" pkg/llm/*.go
grep -n "DataResidencyPolicy" pkg/governance/*.go

# Rec 14 — Board policy
cat config/ai-policy.example.yaml | head -5

# Rec 20 — Red team
make red-team

# Rec 21 — BCP
make bcp-drill

# Rec 22 — Annexure VI
grep -rn "annexure" pkg/incidents/

# Rec 23 — Live inventory
curl -H "Authorization: Bearer $ADMIN" localhost:8080/v1/ai-inventory | jq 'length'

# Rec 25 — Public disclosures
curl localhost:8080/v1/disclosures | jq .
```

Every claim above has a file path or a curl command. That's the
contract: responsible AI is a property of the running system, not a
paragraph in a policy document.
