# Building an RBI FREE-AI-Aligned Financial Assistant in Go: Inside Genie

*An open-source reference for what "Responsible AI in Indian banking" actually looks like in code.*

---

## The gap nobody wants to talk about

In August 2025, the Reserve Bank of India published the **Framework for Responsible and Ethical Enablement of AI** — the FREE-AI report. Seven Sutras. Six Pillars. Twenty-six Recommendations. It is, by some distance, the most concrete piece of AI guidance any major central bank has produced.

Nine months on, most "AI in banking" demos still look the same: a chatbot wrapper around a US-hosted foundation model, an opaque RAG pipeline, no audit trail, no incident form, no board-approved policy, no clue where the customer's PAN just travelled. The compliance team gets handed a 40-slide deck after the fact and asked to bless it.

That gap — between the policy text and a system you can actually point a regulator at — is what **Genie** tries to close.

Genie is an open-source AI financial assistant written in Go, built on Microsoft's **Multi-Agent Reference Architecture (MARA)** and explicitly designed against the RBI FREE-AI checklist. It speaks MCP and A2A, ships Ollama on-prem by default, and every architectural choice has a clause number next to it.

This article is a tour of how the 26 FREE-AI recommendations land in real Go packages — and why "responsible AI" has to be a property of the system, not a paragraph in a policy document.

Repo: <https://github.com/c2siorg/genie>

---

## Why multi-agent, and why Go

Two design decisions up front, because they shape everything that follows.

**Multi-agent over monolithic LLM.** A single prompt-stuffed LLM cannot be audited, cannot be partially upgraded, cannot fail gracefully, and cannot be reasoned about by a risk committee. A pipeline of small, single-purpose agents — ingestor, normalizer, enricher, analyzer, forecaster, anomaly_detector, recommender, reporter — can. Each one has a defined input schema, a defined output schema, a risk class, and its own test suite. When the regulator asks "what does this system do with my customer's bank statement?", the answer is a sequence diagram, not a vibe.

**Go over Python.** Not because Python is bad — because the system-edge of an AI assistant (HTTP, auth, encryption, observability, concurrency, supply-chain) is the boring part that determines whether you survive a real audit. Go's standard library, its first-class concurrency, its statically-linked single-binary deploy story, and its tooling around supply-chain (`govulncheck`, reproducible builds, module proxies) make the compliance surface smaller. The LLM call itself is one provider interface; everything around it is the system.

The architecture, in one diagram:

```
HTTP edge (chi + middleware)
   ├── JWT + RBAC + RateLimit + OTel + Recovery
   └── /v1/ask, /v1/ask/stream (SSE), /v1/chat/ws, /mcp
        │
        ▼
   Orchestrator + Bus + Registry
        │
        ▼
   Governance Composite Policy ← board-approved YAML
        │ (length, RBAC, classification, residency, consent,
        │  PII, prompt-injection, schema, explainability)
        ▼
   Specialist agents (40+)
        │
        ▼
   GenAI layer: LLM · RAG · GraphRAG · Reasoning · Memory · Safety · Eval
        │
        ▼
   Postgres (envelope encrypted) · OTel → Tempo → Grafana
```

Every message that crosses the bus passes through governance **before** any agent's `HandleMessage` runs. This is not a middleware afterthought — it is the load-bearing wall of the entire compliance story.

---

## The 7 Sutras, encoded

The FREE-AI report opens with seven principles — "Sutras" — that frame everything else. In Genie they live as YAML, not as a wall plaque:

```yaml
# config/constitution.yaml
principles:
  - "Trust is the Foundation"
  - "People First"
  - "Innovation over Restraint"
  - "Fairness and Equity"
  - "Accountability"
  - "Understandable by Design"
  - "Safety, Resilience, and Sustainability"
```

These get loaded into `pkg/constitution` and used in two places:

1. As a **system-prompt prefix** on every LLM call (so the model itself sees the principles).
2. As an **LLM-as-judge critique** — a separate agent (`agents/llm_auditor`) subscribes to the bus as a broadcast listener and scores every outgoing message against the constitution. Failed critiques are written to the audit log with the offending text and the principle that was violated.

This is the difference between *claiming* alignment and *measuring* it.

---

## Recommendation-by-recommendation: the compliance map

The FREE-AI report's 26 recommendations split roughly into "things the regulator/SRO does" (1, 3, 5, 7, 9–13) and "things a Regulated Entity must build" (the rest). Genie is the latter. Here's the map.

### Rec 2 — AI Innovation Sandbox

The single most underrated requirement. A bank cannot iterate on AI if it has to convene a change-advisory board for every prompt tweak.

Genie ships `cmd/genie` — a zero-dependency CLI binary that runs the entire pipeline in-process, with stdout OTel exporters. No Postgres, no network, no keys. A risk analyst can clone the repo, run `go test ./...` (57 packages, all green), then `go run ./cmd/genie`, and watch the full agent pipeline produce a financial report against a sample CSV. The sandbox **is** the production code — same orchestrator, same governance, same agents — with mocked I/O.

This is the test that should be table stakes for any AI vendor pitching a bank: *can I run your system on my laptop, offline, in five minutes?*

### Rec 4 — Indigenous AI Models

This is where "Sovereign AI" stops being a buzzword. Genie's default LLM provider is **Ollama**, running on-prem. The provider stack also ships Anthropic, OpenAI, and Gemini implementations — but each provider declares its `Region()`, and the `DataResidencyPolicy` denies any PII-classified message from leaving the home region unless the destination is on-prem.

```go
// pkg/llm/ollama.go
func (p *OllamaProvider) Region() string { return "on-prem" }

// pkg/llm/anthropic.go
func (p *AnthropicProvider) Region() string { return "us" }
```

A bank running Genie in India with `home_region: in` in the policy YAML will see PII traffic routed to the local Ollama model automatically. Public queries (macro research, rate lookups, generic financial education) can be routed to a stronger hosted model. The router is a configuration choice, not a code change.

### Rec 6 — Adaptive Policies

The board approves the policy. Engineers ship the loader.

```yaml
# config/ai-policy.example.yaml
version: "0.1.0"
board_approved_on: "2025-08-13"
owner: "Chief Risk Officer"

governance:
  rbac:
    finance_question:  ["user", "advisor", "admin"]
    portfolio_request: ["user", "advisor", "admin"]

risk:
  max_content_length_bytes: 262144

data:
  retention_days: 180
  block_pii: true
  block_prompt_injection: true

consumer:
  ai_disclosure_banner: |
    This response was produced by an AI pipeline at Genie...

sovereignty:
  home_region: "in"
  allow_cross_border_for_public: true

explainability:
  applies_to: ["recommendations"]
```

`pkg/policy` reads this at boot and wires each clause to a corresponding `governance.Policy`. The board can change a threshold without a code release. Every load is logged with a hash, so an auditor can prove which version was active on a given date.

### Rec 8 — Graded Liability

Not every AI mistake is equal. A wrongly-categorised coffee transaction is not a missed fraud alert.

`pkg/incidents.Grade` classifies incidents by financial impact, customer harm, and reversibility. Low-grade incidents stream to the audit log. Medium-grade incidents trigger a notification. High-grade incidents auto-generate an **Annexure VI** form (Rec 22) ready for FIU / RBI submission.

The grading function is small, deterministic, and unit-tested. Liability follows the grade.

### Rec 14 — Board-Approved AI Policy

`config/ai-policy.example.yaml` is the Annexure V shape. It is a YAML file in the repo, in version control, with a `board_approved_on` field. A regulator can see every change, every approver, every diff.

This is not a Word document on a SharePoint. It is configuration that the running system actually obeys. If the YAML says `block_pii: true`, the `PIIBlockPolicy` is loaded into the composite — and red-team probes (Rec 20) verify it stays denied.

### Rec 15 — Data Lifecycle Governance

Every document uploaded to Genie is encrypted at rest using **envelope AES-256-GCM**:

- Each upload gets a fresh **Data Encryption Key (DEK)**.
- The DEK is wrapped by the active **Key Encryption Key (KEK)**.
- Local dev uses `pkg/crypto.EnvKeyResolver` (KEK from env). Production uses `pkg/crypto.KMSKeyResolver` — pluggable against AWS KMS, GCP KMS, or HashiCorp Vault Transit. The raw KEK never touches Genie's memory in the prod path.
- `documents.expires_at` columns + `db.StartRetentionJob` purge expired rows every 6 hours.

The data lifecycle isn't a slide. It's a column, a job, and a key resolver.

### Rec 16 — AI System Governance + Autonomous Systems

Every agent declares its `RiskLevel()`:

```go
func (a *RecommenderAgent) RiskLevel() agent.RiskClass {
    return agent.RiskMedium
}
```

The orchestrator uses this to enforce policy ceilings — a `RiskHigh` agent (e.g. `agents/aml_monitor`, `agents/var_calculator`, `agents/alm_agent`) cannot execute without an `advisor` or `admin` role on the inbound message. Autonomous loops are bounded by per-call deadlines (`DeadlineProvider`), circuit breakers (`CircuitProvider`), and budget caps (`BudgetedProvider`) on the LLM stack.

### Rec 17 — Product Approval

Partial today. The AI inventory endpoint (Rec 23) lists every agent with its risk class, its model dependencies, and its last-audited timestamp. A product approval workflow that consumes this inventory is on the roadmap.

### Rec 18 — Consumer Protection

Every AI-generated response carries an explicit disclosure banner, configured in the policy YAML and surfaced as:

```
GET /v1/disclosures
```

The banner ships on every SSE event and every JSON response. The user is never in doubt that they are talking to a machine. The disclosure text is board-controlled.

### Rec 19 — Cybersecurity

The HTTP edge is paranoid by default:

- **JWT** HS256, 60-minute TTL, stdlib implementation (small surface).
- **Bcrypt** for passwords.
- **RBAC** at two layers — middleware (`mid.RequireRole`) and bus (`governance.RBACPolicy`). Even a compromised handler cannot bypass the bus check.
- **Classification ceilings** — a `pii`-classified message cannot reach a `public`-cleared agent.
- **Prompt injection** detection on every inbound message.
- **Rate limiting** per-principal at the middleware layer.
- **WebAuthn + Ed25519 passkeys** for passwordless login.
- **OAuth 2.1 + PKCE** for delegated clients (the spec explicitly forbids `plain`).
- **OAuth Device Flow (RFC 8628)** for MCP token onboarding — replaces the "paste your Zerodha session token here" anti-pattern.

### Rec 20 — Red Teaming

```bash
make red-team
# OK: all probes denied / allowed as expected.
```

`cmd/red-team` runs an adversarial probe corpus against the **active** composite policy. Not against a mocked policy. Not against the policy from last quarter. The exact bytes that production is running. If the board tightens a threshold, the red-team output changes on the next run. CI fails if a probe that should be denied gets through.

### Rec 21 — BCP for AI

Every agent has a fallback. `Orchestrator.SetFallback(agentName, fallbackAgent)` wires a degraded-mode handler that runs when the primary fails its deadline or circuit-breaks. `make bcp-drill` forces a `portfolio_advisor` failure and verifies the fallback fires — a continuity test that runs in CI.

The fallback agents are deterministic and don't require the LLM to be reachable. If Ollama is down, if Anthropic rate-limits, if the network partitions — the user gets a degraded but truthful answer, not a 500.

### Rec 22 — AI Incident Reporting (Annexure VI)

`pkg/incidents` auto-generates the Annexure VI form when:

- A governance policy denies a message (with the policy name + reason).
- An agent panics or returns an error above its grade threshold.
- An LLM call exceeds budget or trips the circuit breaker.
- A safety scorer flags toxicity, bias, or jailbreak above threshold.

The form is structured (not free text), tamper-evidently hash-chained into the audit log, and exposed via `GET /v1/incidents` (admin-only). The reporting timeline that FREE-AI prescribes becomes a query, not a Friday-afternoon scramble.

### Rec 23 — AI Inventory

```bash
curl -H "Authorization: Bearer $ADMIN" localhost:8080/v1/ai-inventory | jq .
```

Returns every registered agent: ID, capability, risk class, model dependencies, training-data classification, last-audited timestamp. Built from the live `pkg/registry` — so the inventory cannot drift from what's actually running. There is no separate spreadsheet that goes stale.

### Rec 24 — AI Audit Framework

Partial. `agents/llm_auditor` runs LLM-as-judge critiques on every outbound message and writes scores to the audit log. The next milestone is making those scores feed into a structured audit report consumable by an external auditor.

### Rec 25 — Disclosures

Public, unauthenticated endpoint:

```bash
curl localhost:8080/v1/disclosures | jq .
```

Returns the consumer-facing AI disclosure, the active policy version, the LLM providers in use, the residency posture, and the list of data classifications the system handles. This is the document that should live on the bank's public website.

### Rec 26 — AI Toolkit

`pkg/toolkit` ships a 7-Sutra Scorecard — one default check per Sutra, runnable on any output:

```go
score := toolkit.Scorecard.Run(output)
// score.PerSutra map[string]float64
// score.Overall  float64
```

The bank can extend the checks, but the scaffolding is there. This is the "show me a number, not a vibe" interface for the seven principles.

---

## What FREE-AI doesn't say but every implementer hits

Three things bite you in week two that the report doesn't spell out:

### 1. Trace context across async hops

The orchestrator publishes a message, an agent picks it up in a different goroutine, calls the LLM, the LLM calls back, the result goes to another agent. Without explicit trace propagation, this is six disconnected spans in Tempo and a useless audit trail.

Genie injects the W3C `traceparent` header into `Message.Metadata` on publish, and re-extracts it inside the orchestrator before each agent runs. The result: one distributed trace per user question, spanning HTTP → governance → bus → every agent → every LLM call. Every span carries OpenInference semantic conventions, so Arize Phoenix or Langfuse picks it up unchanged.

When the regulator asks "what happened on this specific request at 14:32:07 on May 14?", the answer is a Tempo URL.

### 2. The audit log has to be tamper-evident

A bank's audit log is the single most attacked asset in the system — an attacker who can rewrite the audit log can hide everything else. `pkg/compliance` ships a **hash-chained** audit log: each entry includes the SHA-256 of the previous entry, so any tampering breaks the chain and is detectable on the next verification pass.

This is not a blockchain. It's a Merkle-style chain anchored to a periodic external timestamp (S3 + Object Lock, or a notary service). It's the boring thing that actually works.

### 3. Consent has to be a ledger

`pkg/compliance.ConsentLedger` is an append-only record of (`user_id`, `data_category`, `purpose`, `granted_at`, `expires_at`, `revoked_at`). Every governance evaluation checks the ledger — a `portfolio_request` cannot proceed without a live `portfolio` consent. Revocation is a single insert. The DPDP Act 2023 alignment falls out of this for free.

---

## The Sovereign AI argument, made concrete

"Data must stay in India" is easy to say and hard to enforce. Genie's version:

- Every LLM provider declares `Region()`.
- Every message carries a `classification` (public / internal / pii / secret).
- `DataResidencyPolicy(HomeRegion: "in")` denies any PII/Secret message destined for a non-on-prem provider.
- The deny is logged, an incident is auto-recorded, and the user gets a degraded-mode response from the fallback agent.

This is not a checkbox. It is a policy that runs on every single message and is verified by the red-team corpus on every commit.

Combined with Ollama for on-prem inference, the deployment shape that satisfies FREE-AI is:

- **Hot path** (anything touching a customer's PAN, account, transaction, holding): Ollama on the bank's own GPUs, region `on-prem`.
- **Cold path** (macro research, generic financial education, public news summaries): hosted frontier model, region `us`, no PII ever sees it.

The router is a 30-line function. The compliance posture is a YAML file.

---

## What this looks like for the customer

A user uploads their bank statement CSV. Asks: *"Where am I overspending vs last month?"*

What happens, in order:

1. CSV is **encrypted at rest** before it touches Postgres (envelope AES-GCM, fresh DEK per upload).
2. `/v1/ask` decrypts in memory, publishes a `finance_question` to the bus, marked `classification=pii`, `region=in`.
3. **Governance composite** evaluates: length OK, metadata complete, role authorised, classification within recipient ceiling, residency satisfied (Ollama is on-prem), consent live, no PII regex hit (the CSV body itself is allowed because it's the payload, not the prompt), no injection markers, schema valid, explainability requirement noted.
4. **Supervisor** kicks off ingestor → normalizer → enricher → analyzer.
5. Analyzer fans out to forecaster, anomaly_detector, recommender in parallel.
6. **LLM-as-judge auditor** subscribes as a broadcast listener and scores every message against the 7 Sutras as it flies past.
7. Recommender produces ranked recommendations *with a `rationale` field* — `ExplainabilityPolicy` denies anything without one.
8. Reporter consolidates. Final report streams back to the user via SSE, with the AI disclosure banner as the first event.
9. Every step is in the Tempo trace. Every denied probe is in the incident log. Every LLM token is in the cost meter. Every consent check is in the ledger.

The customer sees a financial report. The regulator sees an audit trail. The CISO sees encrypted-at-rest data. The board sees a YAML they approved.

That alignment — between what each stakeholder needs to see — is what FREE-AI is asking for. Genie tries to make it the default, not the exception.

---

## What's still 🚧

Honesty is part of the principle list ("Trust is the Foundation"). What's not done:

- **KEK rotation** — schema supports it (`kek_id` per row), rotation job is on the roadmap.
- **Full Sahamati Account Aggregator FIU flow** — fetcher is wired, the consent-artifact round-trip is mocked.
- **Postgres-backed eval / feedback stores** — currently in-memory.
- **Kubernetes manifests (kustomize)** — Docker Compose works, K8s is in progress.
- **Agentic Commerce Protocol (ACP / AP2 / UPI Circle)** — the next protocol surface.
- **Product approval workflow consuming the AI inventory** — Rec 17 is partial.
- **External auditor report from `agents/llm_auditor` scores** — Rec 24 is partial.

These are tracked openly in the roadmap section of the README, and contributions are welcome.

---

## Why this matters

The FREE-AI report is the clearest mandate any major regulator has issued: **AI in finance must be explainable, auditable, resilient, fair, sovereign, and human-centred.** The report does not prescribe how to build it. That gap is where 80% of the work lives — and where most "responsible AI" claims fall apart.

Genie is an attempt to make the implementation as open as the policy. Every architectural decision has a clause number. Every promise has a test. Every claim has a `grep`-able file path. If you disagree with a decision, you can fork it. If you find a gap, you can file an issue. If you build a bank-grade fork, the license (MIT) does not stand in your way.

The argument is simple: *responsible AI is a property of the system, not a property of the press release.* The only way to prove it is to publish the system.

---

## How to look at it yourself

```bash
git clone https://github.com/c2siorg/genie.git
cd genie

# Run the 57-package test suite
go test ./...

# Run the full pipeline in-process (no Postgres, no network)
go run ./cmd/genie

# Run the adversarial probe corpus against the active policy
make red-team

# Bring up the full stack: Postgres + Tempo + Grafana + Ollama + API
make compose-up
# Open http://localhost:8080 — sign up, upload a CSV, ask a question
# Open http://localhost:3000 — Tempo → search service:genie-api
```

The repo is at <https://github.com/c2siorg/genie>. The architecture diagrams, the FREE-AI compliance table, and the API spec live in the README and `docs/`. The board policy template is `config/ai-policy.example.yaml`. The constitution is `config/constitution.yaml`.

If you're building AI in Indian banking — at an RE, an SRO, a fintech, a regulator, or a vendor — clone it, break it, fork it, contribute back.

The next billion bank customers in India deserve AI that's safe, sovereign, and explainable by default. The FREE-AI report says that out loud. Genie tries to ship it.

---

*Genie is open source under the MIT license and is not affiliated with the RBI. The compliance mapping in this article is a good-faith engineering interpretation of the August 2025 FREE-AI report and is not a substitute for a formal regulatory assessment.*

**References**

- [RBI FREE-AI Report (Aug 2025)](https://rbidocs.rbi.org.in/rdocs/PublicationReport/Pdfs/FREEAIR130820250A24FF2D4578453F824C72ED9F5D5851.PDF)
- [Microsoft Multi-Agent Reference Architecture](https://microsoft.github.io/multi-agent-reference-architecture/index.html)
- [Anthropic Model Context Protocol](https://modelcontextprotocol.io/)
- [Google Agent2Agent Protocol](https://github.com/google/a2a)
- [Genie repository](https://github.com/c2siorg/genie)

#ResponsibleAI #RBI #FREEAI #SovereignAI #FinTech #IndiaStack #MultiAgent #Golang #OpenSource #BankingAI
