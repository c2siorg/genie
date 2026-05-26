# Genie Documentation Index

> Detailed reference for every agent, package, protocol, and operational
> concern in Genie. Start from the section that matches what you're trying
> to do; each page is self-contained and links back here.

## Quick navigation

| You want to… | Read |
|---|---|
| Get the 30-second pitch | [Root README](../README.md) |
| **Review AI governance + security end-to-end** | **[ai-governance-security.md](ai-governance-security.md)** — the canonical CISO/risk-officer reference |
| Understand the architecture pattern | [architecture.md](architecture.md) |
| Map FREE-AI recommendations to code | [free-ai-mapping.md](free-ai-mapping.md) |
| Run the stack locally | [operations.md](operations.md) |
| Wire a new agent | [agents/README.md](agents/README.md) |
| Use a platform package | [packages/README.md](packages/README.md) |
| Open the HTTP API | [api.md](api.md) |
| Hit MCP/A2A protocols | [protocols.md](protocols.md) |
| Read the LinkedIn-ready writeups | [linkedin-article-architecture.md](linkedin-article-architecture.md) · [linkedin-article-compliance.md](linkedin-article-compliance.md) |
| See the ADK extension proposal | [adk-extension-proposal.md](adk-extension-proposal.md) |

---

## Document tree

```
docs/
├── README.md                                  ← this index
├── ai-governance-security.md                  ← CISO/risk reference; threat model + 11 layers + invariants
├── architecture.md                            ← MARA + the 7 load-bearing pieces
├── free-ai-mapping.md                         ← every Rec → file path
├── operations.md                              ← compose-up, env, KEK, observability
├── api.md                                     ← every endpoint, with curl
├── protocols.md                               ← MCP, A2A, CloudEvents, OAuth, WebAuthn
├── openapi.yaml                               ← HTTP spec
├── asyncapi.yaml                              ← bus event spec
├── adk-extension-proposal.md                  ← the design doc behind the 13 new agents
├── linkedin-article-architecture.md
├── linkedin-article-compliance.md
├── linkedin-article-rbi-freeai.md
├── linkedin-post*.md                          ← short-post variants
├── agents/
│   ├── README.md                              ← agent index + contract
│   ├── kyc_orchestrator.md                    ← Tier 1.1
│   ├── claim_adjudicator.md                   ← Tier 1.2
│   ├── sme_loan_workflow.md                   ← Tier 1.3
│   ├── invoice_processor.md                   ← Tier 1.4
│   ├── deep_research.md                       ← Tier 1.5
│   ├── bulk_statement_analyzer.md             ← Tier 1.6
│   ├── mpc_research.md                        ← Tier 1.7
│   ├── auto_insurance.md                      ← Tier 3.14
│   ├── health_preauth.md                      ← Tier 3.15
│   ├── supply_chain_finance.md                ← Tier 3.16
│   ├── payment_orchestrator.md                ← Tier 3.17
│   ├── cyber_guardian.md                      ← Tier 4.18
│   └── google_trends.md                       ← Tier 4.20
└── packages/
    ├── README.md                              ← package index
    ├── policy-dsl.md                          ← pkg/policy/dsl
    ├── memory-longterm.md                     ← pkg/memory LongTermMemory
    ├── loader-xlsx-and-ocr.md                 ← pkg/loader xlsx + scanned-pdf
    ├── safety-plugins.md                      ← pkg/safety plugin chain
    ├── agent-skill-registry.md                ← pkg/agent SkillRegistry
    ├── observability-bq.md                    ← pkg/observability/bq
    ├── voice-streaming.md                     ← agents/voice StreamingAgent
    ├── postgres-rls.md                        ← pkg/storage/postgres + 0005_rls.sql (RLS)
    ├── oauth-token-exchange.md                ← pkg/auth/tokenexchange (RFC 8693)
    ├── agent-tier.md                          ← pkg/agent.Tier promotion model
    └── governance-tenant.md                   ← pkg/governance.TenantPolicy
```

---

## Reading paths by role

### "I'm an engineer who'll add a new agent"

1. [architecture.md](architecture.md) — protocol, registry, bus, governance
2. [agents/README.md](agents/README.md) — the 7-field agent contract
3. Pick a similar existing agent doc (e.g. [agents/kyc_orchestrator.md](agents/kyc_orchestrator.md)) and copy the structure
4. `make scaffold name=<id> cap=<cap> in=<type> out=<type> next=financial_supervisor`

### "I'm a CRO / compliance officer evaluating Genie for FREE-AI alignment"

1. [free-ai-mapping.md](free-ai-mapping.md) — table per recommendation
2. [linkedin-article-compliance.md](linkedin-article-compliance.md) — the long-form
3. Spot-check: pick one Rec → open the linked file → run `go test ./<pkg>/...`

### "I'm a CISO reviewing the security posture"

1. **[ai-governance-security.md](ai-governance-security.md)** — the canonical reference. Threat model, eleven-layer envelope, every claim anchored to a file path. Read this first.
2. [linkedin-article-security-complete.md](linkedin-article-security-complete.md) — the consolidated security deep-dive (long-form narrative)
3. [linkedin-article-agentic-security-operations.md](linkedin-article-agentic-security-operations.md) — runtime operations playbook (SLIs, runbook, drift, drills)
4. [api.md](api.md) — auth, RBAC, rate limits
5. [protocols.md](protocols.md) — WebAuthn, OAuth 2.1+PKCE, Device flow, OAuth 2.0 Token Exchange (RFC 8693)
6. [agents/cyber_guardian.md](agents/cyber_guardian.md) — session anomaly detection
7. [packages/safety-plugins.md](packages/safety-plugins.md) — pluggable shields
8. The four Q1 hardening primitives — read all four together; they're the defence-in-depth envelope:
   - [packages/postgres-rls.md](packages/postgres-rls.md) — DB-level tenant isolation
   - [packages/governance-tenant.md](packages/governance-tenant.md) — bus-level tenant isolation
   - [packages/oauth-token-exchange.md](packages/oauth-token-exchange.md) — dual-identity audit
   - [packages/agent-tier.md](packages/agent-tier.md) — promotion gate

### "I'm a risk officer setting policy"

1. [packages/policy-dsl.md](packages/policy-dsl.md) — the YAML DSL you can edit
2. [free-ai-mapping.md](free-ai-mapping.md) — Rec 6 (Adaptive Policies) and Rec 14 (Board-Approved)
3. The active policy: [`config/ai-policy.example.yaml`](../config/ai-policy.example.yaml)

### "I'm a product manager scoping bancassurance / SME / payments"

1. [agents/sme_loan_workflow.md](agents/sme_loan_workflow.md)
2. [agents/claim_adjudicator.md](agents/claim_adjudicator.md) + [agents/auto_insurance.md](agents/auto_insurance.md) + [agents/health_preauth.md](agents/health_preauth.md)
3. [agents/payment_orchestrator.md](agents/payment_orchestrator.md)
4. [agents/supply_chain_finance.md](agents/supply_chain_finance.md)

### "I'm running the system in production"

1. [operations.md](operations.md) — compose, env vars, KEK, Postgres
2. [api.md](api.md) — `/readyz`, `/healthz`
3. Observability — OTel collector → Tempo → Grafana (compose default)

---

## Conventions used across the docs

- **Code blocks tagged with `go`, `yaml`, `bash`, `json`** match real files in the repo. Anything in a code block can be `grep`'d.
- **File paths are relative to the repo root**, even inside `docs/` — e.g. `pkg/agent/types.go` not `../pkg/agent/types.go`.
- **Risk class** uses the `pkg/agent.RiskClass` levels exactly: `RiskLow`, `RiskMedium`, `RiskHigh`.
- **FREE-AI rec numbers** are the August 2025 report's recommendation IDs (1–26).
- **Indian English** for finance terms (CRR, IDV, NPA, GSTIN, PAN, IFSC, etc.).
- **₹ amounts** are in rupees; ₹1L = ₹100,000; ₹1cr = ₹10,000,000.

---

## How docs stay in sync with code

- The agent registry (`registry.NewInMemory().List()`) drives the live `/v1/ai-inventory` endpoint. If an agent is renamed, the doc should be too — `tests/agents_registry/` enforces ID uniqueness.
- The FREE-AI mapping table in [free-ai-mapping.md](free-ai-mapping.md) cites exact file paths. If a path moves, update the table.
- New agents should land with a paired doc in `docs/agents/<id>.md` — keep the same template (overview → contract → business rules → integration → anti-patterns → tests).
- New packages get a paired doc in `docs/packages/<short-name>.md`.

If you change behaviour in code without updating its doc, the next reader will be misled — the docs are part of the contract, not a postscript.
