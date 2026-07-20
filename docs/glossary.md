# Glossary

Every load-bearing concept in Genie, grouped by area, with a one- or two-line
definition and where it lives. Terms are grounded in the actual code — package
paths and identifiers are real. Where a term names an aspiration the code does not
yet implement, it says so.

> New here? Read `docs/architecture.md` for the deep dive, then use this as the
> lookup table. Regulator/domain terms are at the bottom.

---

## 1 · Architecture & the core loop

| Term | Definition |
|---|---|
| **Message-driven** | The load-bearing decision: agents never call each other directly — every step is a `protocol.Message` on the bus. One seam each for governance, tracing, fallbacks, and inventory. |
| **`protocol.Message`** | The unit of work: `{ID, From, To, Role, Type, Content, CreatedAt, Metadata}`. `pkg/protocol`; re-exported by `pkg/agent` as a type alias. |
| **`Type`** | A plain routing string on a message (e.g. `finance_question`, `portfolio_request`). Drives RBAC and routing; there are no `MessageType` constants. |
| **`Metadata`** | Per-message map carrying `trace_id`/`traceparent`, `user_id`, `user_roles`, `classification`, `region`, and domain payloads (`MetaKeyUserID`, `MetaKeyUserRoles`, `MetaKeyClassification`, `MetaKeyRegion`). |
| **`trace_id`** | The correlation key. There is **no** `correlation_id` — synchronous request/response is matched on `trace_id`. |
| **Bus** | Fire-and-forget pub/sub with OTel spans (`pkg/comm`, `NewInMemoryBus()`). The single audit/trace/fallback/inventory chokepoint. |
| **Orchestrator** | Subscribes one handler per registered agent at `Start(ctx)` (`pkg/orchestration`): extract trace → evaluate policy → `HandleMessage` → publish outputs, or on error record an incident + route to a fallback. Does **not** enforce a risk ceiling or recover panics. |
| **Correlator** | `pkg/busio.Correlator` layers synchronous request/response over the async bus by waking the waiting HTTP handler when `trace_id` matches. |
| **Registry** | The live capability inventory (`pkg/registry`, `NewInMemory()`); drives `GET /v1/ai-inventory`. |
| **Fan-out / fan-in** | An agent (e.g. `analyzer`) emits parallel messages; the `supervisor` collects them per `trace_id` and fires the `reporter` once all expected named fields are present (structural readiness, not a count). Latency = max(stages), not sum. |
| **Supervisor** | The fan-in agent that assembles a per-`trace_id` session and triggers the final report. `h_supervisor` is the hierarchical variant. |
| **Reporter** | Emits the terminal `final_report` message addressed to `"user"`. |
| **Incident** | A recorded agent failure (`pkg/incidents`); surfaced at `GET /v1/incidents` and feeds the BCP story. |
| **`MessageRole`** | The semantic source of a message: `user` \| `system` \| `agent` \| `tool` \| `observer` \| `evaluator` (`pkg/protocol`). |
| **Handler / broadcast tap** | A bus subscriber `func(ctx, msg)`; a handler subscribed under the **empty** agent id receives every message (used for audit/observability taps). `pkg/comm`. |
| **Fire-and-forget delivery** | `Publish` is non-blocking and spawns a goroutine per matching handler — no ordering, backpressure, retries, or delivery confirmation. `pkg/comm`. |
| **Session** | The supervisor's per-`trace_id` state holder for fan-in; deleted after it finalises exactly once (idempotent under duplicate/out-of-order delivery). `agents/supervisor`. |
| **`SetFallback`** | Orchestrator map `originalID → fallbackID`; on an agent **error** it publishes a `fallback_request` to the mapped agent. `pkg/orchestration`. |
| **Await-before-Publish** | The correlator waiter must be registered before the request is published, or the reply could race ahead of the waiter. `pkg/busio`. |
| **Emergent termination** | The pipeline stops when an agent returns zero messages or targets an unsubscribed recipient — no dispatch loop, no cycle detection, no panic recovery in the core. |

---

## 2 · Governance (the gate)

| Term | Definition |
|---|---|
| **Composite policy** | The governance chain evaluated on every message, **deny-on-first-failure** (`pkg/governance`, `CompositePolicy`). Assembled from board-approved YAML by `pkg/policy`. |
| **Governance is data, not code** | Cross-cutting rules live in `config/ai-policy.example.yaml` + `pkg/governance`, applied to every message — never scattered into agents. |
| **`MaxContentLengthPolicy`** | Rejects over-long content. |
| **`RequiredMetadataPolicy`** | Rejects messages missing mandatory metadata keys. |
| **`RBACPolicy`** | Role gate keyed on message `Type` (shipped YAML gates `finance_question`/`portfolio_request` to `user`/`advisor`/`admin`). |
| **`ClassificationPolicy`** | Enforces a data-classification ceiling (`public` < `internal` < `pii` < `secret`). |
| **`DataResidencyPolicy`** | Enforces the `region` a message may be processed in (`pkg/sovereignty`). `NewResidencyPolicy` is the constructor. |
| **`ConsentPolicy`** | Requires recorded user consent before processing. |
| **`ExplainabilityPolicy`** | Requires an explanation/rationale to accompany certain outputs. |
| **`PIIBlockPolicy`** | Regex block on personally identifiable information. |
| **`PromptInjectionPolicy`** | Blocks known prompt-injection / jailbreak patterns. |
| **`SchemaPolicy`** | Validates message payloads against a schema — **exists but is not wired by the YAML loader.** |
| **Policy DSL** | The rule language the YAML compiles through (`pkg/policy/dsl`, `docs/packages/policy-dsl.md`). |
| **`RiskClass`** | `low` \| `medium` \| `high` (`pkg/agent/risk.go`); `RiskOf` defaults to `low`. Read by **reporting** surfaces (inventory, AIBOM, disclosures), **not** by the dispatcher. |
| **Classification** | `public` \| `internal` \| `pii` \| `secret` (`pkg/protocol`). `secret` is intentionally never selectable in the browser UI. |
| **Deny-on-first-failure** | The composite's evaluation discipline: the first policy that denies short-circuits and returns its single reason; an operational error aborts immediately. |
| **`PolicyResult`** | The audit record of one evaluation: `{Decision (allow\|deny), Reason, CheckedAt, CheckedByID}`. `pkg/governance`. |
| **`BuildComposite`** | Assembles the 9-policy runtime chain from the YAML (`pkg/policy`). Wiring order: MaxContentLength → RequiredMetadata → RBAC → Classification → DataResidency → PIIBlock → PromptInjection → Consent → Explainability. |
| **`OnPolicyDeny` / `OnAgentError`** | Orchestrator hooks that turn a denial or an agent error into an **Incident** (and, for errors, a fallback dispatch). `pkg/orchestration`, wired in `cmd/api`. |
| **Classification ceiling** | The max sensitivity a recipient may receive; note the wired policy hardcodes the ceiling to `internal` (`AllowedTo` is never filled from YAML — see §12). |
| **`RiskAware`** | The optional interface an agent satisfies via `RiskLevel()`; `RiskOf` falls back to `low`. `pkg/agent/risk.go`. |
| **RBAC opt-in / admin bypass** | An unlisted message `Type` is allowed (gating is opt-in); an `admin` role can bypass the RBAC gate. `pkg/governance`. |
| **`pii_acknowledged` bypass** | A human-in-the-loop escape hatch: `metadata["pii_acknowledged"]=="true"` lets a message past `PIIBlockPolicy`. |

---

## 3 · Agents

| Term | Definition |
|---|---|
| **Agent** | One package under `agents/<id>/`: `New()` constructor, exported `ID`/`Capability`/`Type*` constants, `HandleMessage(ctx, msg, env)`, optional `RiskLevel()`. |
| **Capability** | The declared skill an agent advertises to the registry/inventory. |
| **Disclaimer** | Every agent output carries one; high-risk rejects also attach an incident payload. |
| **Wired vs. present** | `cmd/api/main.go` (`run()`) is the source of truth for what's served — 58 specialists + 2 fallbacks of 61 specialist packages are wired. |
| **Pipeline agents** | The deterministic transform/route chain: `ingestor → normalizer → enricher → analyzer → anomaly → forecaster → supervisor → reporter`, plus `recommender`. |
| **Specialist agents** | The 45+ finance experts (`fraud`, `goal_planner`, `portfolio_advisor`, `kyc_orchestrator`, `aml_monitor`, `lcr_projector`, `var_calculator`, …). |
| **`fallback`** | Deterministic (no LLM/network) "a human will follow up" notice, published when a primary agent returns an error (not on panic). Only `portfolio_advisor` and `recommender` have fallbacks wired. |
| **`moa_recommender`** | Mixture-of-agents recommender variant. |
| **Preprocess chain** | The deterministic run-up to analysis: `ingestor` (CSV → raw) → `normalizer` → `enricher` (categorise) → `analyzer` (aggregate + fan-out). `agents/`. |
| **`h_supervisor`** | The hierarchical supervisor variant (present as a package; see wired-vs-present). |
| **Scaffold** | `make scaffold name=… cap=… in=… out=… next=…` generates `agents/<id>/<id>.go` + a passing test; a paired `docs/agents/<id>.md` is part of the contract. `cmd/scaffold`. |

---

## 4 · AFG — the framework-native edge (`pkg/afg`)

| Term | Definition |
|---|---|
| **AFG** | The in-progress port of Genie's orchestration + provider layers onto Microsoft's Agent Framework for Go. Serves `/v1/ask` + `/v1/ai-inventory` from `cmd/af-serve` with **no Postgres and no bus**. |
| **`Spec`** | A framework-free declaration of a governed agent (`pkg/afg/spec.go`): deterministic (`Handle`) or advisory (`Instructions` + `Model`). Lets specialists be authored without importing the framework. |
| **Single-door invariant** | Every agent is built through one of two sanctioned constructors that inject governance as the outermost middleware — reconstructing the bus's single chokepoint per agent. |
| **Governed registry** | `catalog.Registry(gate)` — the AFG source of truth for what's served (4 base + 9 pipeline + 45 specialists = 58 governed agents). |
| **Provider kind** | How an agent is backed in the inventory: `deterministic` \| `mock` \| `ollama` \| `foundry`. |
| **`NewGovernedDeterministic` / `NewGovernedOllama`** | The **only two** sanctioned agent builders (no-LLM and Ollama-backed); both inject the gate as the outermost middleware. `pkg/afg/factory.go`. |
| **`GovMiddleware`** | The deny-on-first-failure gate re-imposed as agent-framework middleware; runs **before** the provider and short-circuits on deny without calling `next`. `pkg/afg/factory.go`. |
| **`DeniedError`** | The typed error a governance denial produces; **must not** be rescued by a fallback (only execution errors are). `pkg/afg`. |
| **`RunWithFallback`** | Runs the primary; routes an **execution** error to a registered fallback, but returns a `DeniedError` as-is for incident recording. `pkg/afg/registry.go`. |
| **`AgentInfo`** | One inventory row: `{ID, Provider, Risk, Governed (always true), HasFallback}`. `pkg/afg/registry.go`. |
| **`RegisterSpecs`** | Batch-builds a list of `Spec`s into governed agents under one gate; used by `catalog.Registry`. `pkg/afg/spec.go`. |
| **`FanOut`** | Concurrent fan-out/fan-in via the framework's concurrent workflow builder; latency = max(stages); every node individually governed. `pkg/afg/orchestrate.go`. |
| **`det()`** | Adapter wrapping a plain `func(string)(string,error)` into the framework's `RunFunc` iterator. `pkg/afg/specialists.go`. |
| **`af-serve`** | The framework-native HTTP edge: loads the board policy, builds the 58-agent registry, serves `POST /v1/ask` (denial → 403) + `GET /v1/ai-inventory` with **no Postgres and no bus**. `cmd/af-serve`, `pkg/afg/httpedge.go`. |
| **`AskRequest` / `AskResponse`** | af-serve wire types: request `{agent, input, type?, user_id, roles, classification, region}`; reply `{agent, output, used_fallback}`; denials return `{error, decision:"denied"}` at 403. `pkg/afg/httpedge.go`. |
| **singledoor test** | The CI guard: it fails if any framework-importing file **outside** `pkg/afg` calls a raw framework constructor — keeping the single door unbypassable. `pkg/afg/singledoor_test.go`. |

---

## 5 · GenAI engineering layer

| Term | Definition |
|---|---|
| **RAG** | Retrieval-augmented generation (`pkg/rag`); vector store via `pkg/rag/pgvector`. |
| **GraphRAG** | Graph-structured retrieval (`pkg/graphrag`) — entities/relations, not just chunks. |
| **Embedder** | Turns text into vectors for retrieval (`rag.Embedder`); uses a **separate** path that bypasses the LLM wrapper chain. |
| **Reasoning** | `pkg/reasoning` — a **ReAct** (reason+act) loop capped at `maxSteps`, plus **Reflexion** (self-critique). Runaway loops are cut off by the Ollama-path wrappers, not the agent. |
| **Memory** | `pkg/memory` — short-term (thread) and long-term (knowledge) stores; see `docs/packages/memory-longterm.md`. |
| **Eval** | `pkg/eval` — offline quality gates: `ragas` (RAG metrics), `elo` (pairwise ranking), `drift`, `hallucination`, `checklist`. |
| **Safety** | `pkg/safety` — output guardrails, constitution-driven **LLM-as-judge**, and pluggable checks (`docs/packages/safety-plugins.md`). |
| **Constitution** | `config/constitution.yaml` + `pkg/constitution` — the rules the LLM-as-judge scores answers against. |
| **Prompt** | `pkg/prompt` — prompt construction/templating. |
| **Synth** | `pkg/synth` — synthetic data generation. |
| **Toolkit** | `pkg/toolkit` — the agent-skill registry (`docs/packages/agent-skill-registry.md`). |
| **MCP** | Model Context Protocol (`pkg/mcp`) — how Genie exposes/consumes external tools. |
| **A2A** | Agent-to-Agent protocol (`pkg/a2a`) — cross-system agent interop. |
| **ADK** | Agent Development Kit extension proposal (`docs/adk-extension-proposal.md`). |
| **CoT / CoV / Step-Back** | Reasoning prompts in `pkg/reasoning`: Chain-of-Thought (step-by-step), Chain-of-Verification (draft → verify questions → synthesise), and Step-Back (abstract the question first). |
| **`MemoryStore` vs pgvector** | `pkg/rag` ships an in-memory O(N) `MemoryStore` (default/tests) and a `pgvector` store (production); both implement the `VectorStore` interface. |
| **BM25 / hybrid retrieval** | Lexical (`bm25.go`) blended with vector search for hybrid ranking. `pkg/rag`. |
| **Self-RAG / CRAG** | Adaptive retrieval: Self-RAG gates whether to retrieve at all; CRAG grades each chunk 0..1 and reports confidence so the caller can fall back. `pkg/rag/selfrag.go`. |
| **Reranker** | Re-scores candidates: `IdentityReranker` (no-op) or `LLMReranker` (model scores 0..10). `pkg/rag/rerank.go`. |
| **`SemanticRouter`** | Routes an input by embedding-similarity to exemplar sets — cheaper/more deterministic than LLM routing. `pkg/reasoning/router.go`. |
| **RAGAS metrics** | LLM-as-judge signals: Faithfulness, AnswerRelevance, ContextPrecision. `pkg/eval/ragas`. |
| **Memory tiers** | `SemanticMemory` (per-user vectors), `EpisodicMemory` (rolling session buffer, consolidated to summaries), `LongTermMemory` (append-only `Fact` store). `pkg/memory`. |
| **Safety detectors** | `Detector`→`Verdict{Flagged, Score, Reason}`: `HeuristicJailbreak` + `LLMJailbreak` (two-gate), `TopicGuardrail`, `ToxicityHeuristic`; stage-aware `Plugin`s (`Inbound/Internal/Outbound`). `pkg/safety`. |
| **`DemographicParity`** | A fairness metric — acceptance-rate gap across subgroups (acceptable ≤ 0.1). `pkg/safety/bias.go`. |

---

## 6 · LLM provider wrapper chain

`cmd/api/llmstack.go` wraps the base provider in layers that bound autonomous
reasoning. On the **Ollama** path the call order (outermost→innermost) is:

| Layer | Definition |
|---|---|
| **Circuit** | Circuit-breaker — trips open after repeated provider failures. |
| **Deadline** | Per-call timeout. |
| **Budget** | Token/cost ceiling per request. |
| **Cache** | Response cache (TTL from `GENIE_LLM_CACHE_TTL`). |
| **Cost** | Cost accounting/metering. |
| **Ollama** | The on-prem default provider (OpenAI-compatible `/v1`). |
| **`mock`** | Deterministic, dependency-free provider — **unwrapped (bare)**; makes everything run offline. |

---

## 7 · Security, identity & privacy

| Term | Definition |
|---|---|
| **Envelope encryption** | Two-key scheme (`pkg/crypto`): a per-document **DEK** encrypts data with **AES-256-GCM**; the DEK is itself encrypted by a **KEK**. |
| **KEK** | Key-Encryption Key — the long-lived master key (from `GENIE_KEK_BASE64`) that wraps DEKs. |
| **DEK** | Data-Encryption Key — a fresh per-payload key, stored only in its KEK-wrapped form. |
| **KMS** | Key Management Service abstraction the KEK can be sourced from. |
| **JWT** | Bearer token auth (`pkg/auth`, `GENIE_JWT_SECRET`); carries user id + roles into message metadata. |
| **RBAC** | Role-based access control — enforced as a governance policy keyed on message `Type`. |
| **OAuth2 / device flow / WebAuthn** | Additional auth methods (`pkg/auth/oauth2`, `pkg/auth/oauth_device`, `pkg/auth/webauthn`). |
| **Identity** | `pkg/identity` — principal/subject resolution. |
| **PII** | Personally Identifiable Information — blocked by `PIIBlockPolicy`; a classification tier. |
| **Privacy / DPO** | `pkg/privacy`, `pkg/dpo` — data-protection-officer controls (retention, subject rights). |
| **Sovereignty** | Data-residency enforcement by `region` (`pkg/sovereignty`). |
| **Federated** | `pkg/federated` — federated (privacy-preserving, cross-node) computation. |
| **`KeyResolver`** | Wraps/unwraps DEKs and names the active KEK for audit; `EnvKeyResolver` (dev) and `KMSKeyResolver` (prod shape). `pkg/crypto/resolver.go`. |
| **What's encrypted** | Only `documents.payload` and `mcp_tokens.payload`; users/accounts/incidents/audit_log/embeddings are plaintext at rest. |
| **`Claims` / `Role` / `User`** | JWT payload `{Subject, Email, Roles}`; roles `user < advisor < admin`; a `User` stores a bcrypt hash, never a raw password. `pkg/auth`. |
| **DID / Verifiable Credential** | W3C `did:key` over an Ed25519 keypair, and a VC 1.1 envelope with an `Ed25519Signature2020` proof (`IssueVC`/`VerifyVC`). `pkg/identity`. |
| **Tokeniser (pseudonymisation)** | Masks sensitive strings via deterministic HMAC-SHA256 keyed per tenant (`tok_…`). `pkg/privacy`. |
| **Differential privacy** | Laplace `(ε,0)` / Gaussian `(ε,δ)` mechanisms; `AggregateWithDP` adds noise scaled by sensitivity/epsilon. `pkg/privacy`. |

---

## 8 · Compliance, transparency & resilience

| Term | Definition |
|---|---|
| **AIBOM** | AI Bill of Materials — a CycloneDX 1.6 inventory of the AI system (`pkg/aibom`); served at `GET /v1/aibom`. |
| **AI inventory** | The live list of served agents + provider/risk/fallback status (`GET /v1/ai-inventory`). |
| **Disclosures** | Public transparency statements (`GET /v1/disclosures`). |
| **Compliance** | `pkg/compliance` — regulatory mapping/checks. |
| **Red-team** | Adversarial probe corpus run against the board policy (`make red-team`, FREE-AI Rec 20). |
| **BCP drill** | Business-Continuity-Plan test: force a `portfolio_advisor` failure to verify the fallback fires (`make bcp-drill`, FREE-AI Rec 21). |
| **Explainability** | Rationale attached to outputs; enforced by `ExplainabilityPolicy`. |
| **Consent ledger** | Persists `Grant/Revoke/HasActive` over categories `transactions / portfolio / recommendations / third_party_share`; the `ConsentPolicy` reads it. `pkg/compliance/consent.go`. |
| **Hash-chained audit log** | Append-only, tamper-evident: each `AuditEntry.RowHash = SHA-256(prev ‖ canonical-json(row))`, verified from a `genesis` seed. `pkg/compliance/audit.go`. |
| **Incident taxonomy** | `Severity` (`Low/Moderate/High`) + `FailureMode` (`bias, hallucination, explainability_gap, privacy_breach, unintended_action, policy_denied, agent_error, unknown`). `pkg/incidents`. |
| **Annexure V / VI** | RBI templates: the board-approved AI **policy** (`config/ai-policy.example.yaml`) and the **incident** report shape (`pkg/incidents`). |
| **Human-in-the-loop** | The pattern behind fallbacks and high-risk gating — a human follows up when automation should not act alone. |

---

## 9 · Observability & protocols

| Term | Definition |
|---|---|
| **OTel** | OpenTelemetry — traces/spans on every bus hop (`pkg/observability`); OTLP export when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, else stdout. |
| **`traceparent`** | W3C trace-context header propagated in message metadata. |
| **CloudEvents** | CNCF event envelope format (`pkg/cloudevents`). |
| **AsyncAPI / OpenAPI** | The async (bus) and sync (HTTP) API contracts (`docs/asyncapi.yaml`, `docs/openapi.yaml`). |
| **observability/bq** | BigQuery sink for telemetry (`docs/packages/observability-bq.md`). |
| **Web edge** | `pkg/web` — chi router, handlers, middleware (`web/mid`): RequestID, Recovery, Log, OTel, JWT, RBAC, rate-limit. SSE streaming at `/v1/ask/stream`. |
| **Console** | The browser UI — a Next.js app statically exported and embedded at `pkg/web/handlers/ui` via `//go:embed all:ui` (`web-next/`, `make ui`). |
| **`/healthz` / `/readyz`** | Liveness (process up) and readiness (Postgres + LLM reachable) probes; `make up` polls `/readyz`. |
| **Trace-context propagation** | `InjectTraceContext`/`ExtractTraceContext` move W3C `traceparent` through `Message.Metadata` (via a `MetadataCarrier`), so `agent.handle` spans re-parent to `bus.publish` across goroutines. `pkg/observability`. |
| **Metrics** | Counters/histograms: `bus.messages_published`, `agent.messages_handled`, `governance.denials`, `agent.errors`, `agent.handle_duration_ms`, plus LLM token/cost/latency (OpenInference conventions). |
| **Compose stack** | `docker compose` brings up genie-api, Postgres, Ollama, OTel Collector, Tempo, Grafana (UI :8080, Grafana :3000). `docker-compose.yaml`, `make up/down`. |
| **smoke / e2e** | `make smoke` (curl signup→upload→/ask→disclosures) and `make e2e` (`-tags=e2e` sim against a live stack). |

---

## 10 · Regulatory framework (the "why")

| Term | Definition |
|---|---|
| **MARA** | Microsoft's **Multi-Agent Reference Architecture** — orchestrator + registry + bus + governance + memory + observability + evaluation. Genie's shape. |
| **RBI FREE-AI** | The Reserve Bank of India's *Framework for Responsible & Ethical Enablement of AI* report (Aug 2025) that Genie aligns to. |
| **Sutras** | The 7 guiding principles of FREE-AI. |
| **Pillars** | The 6 structural pillars of FREE-AI. |
| **Recommendations** | The 26 concrete recommendations; every one maps to a file in `docs/free-ai-mapping.md` (e.g. **Rec 20** → red-team, **Rec 21** → BCP fallback). |

---

## 11 · Indian-finance domain

Money is in integer paise (`*_cents` fields); **₹1L = 100,000**, **₹1cr = 10,000,000**.

| Term | Definition |
|---|---|
| **CRR** | Cash Reserve Ratio — share of deposits a bank parks with the RBI. |
| **Repo rate** | The RBI's key policy lending rate. |
| **MPC** | Monetary Policy Committee — sets the repo rate (`mpc_research`). |
| **LCR** | Liquidity Coverage Ratio — high-quality liquid assets vs. 30-day outflows (`lcr_projector`). |
| **ALM** | Asset-Liability Management — matching maturities/rates (`alm_agent`). |
| **VaR** | Value at Risk — worst expected loss at a confidence level (`var_calculator`). |
| **NPA** | Non-Performing Asset — a loan in default. |
| **KYC** | Know Your Customer — identity verification (`kyc_orchestrator`). |
| **AML** | Anti-Money-Laundering monitoring (`aml_monitor`). |
| **Mule** | A money-mule account used to launder funds (`mule` detector). |
| **AA** | Account Aggregator — India's consented financial-data-sharing rail (`aa_fetcher`). |
| **UPI** | Unified Payments Interface — India's real-time payment rail. |
| **PAN** | Permanent Account Number — the taxpayer ID. |
| **Aadhaar** | India's national identity number. |
| **GSTIN** | Goods & Services Tax Identification Number. |
| **IFSC** | Indian Financial System Code — identifies a bank branch. |
| **IDV** | Insured Declared Value — an insured asset's agreed value (`auto_insurance`). |
| **SIP** | Systematic Investment Plan — fixed periodic mutual-fund investing (`sip_vs_lumpsum`). |
| **Lumpsum** | A one-shot investment (vs. SIP). |
| **FD / sweep FD** | Fixed Deposit; a sweep FD auto-converts idle balance into an FD. |
| **New tax regime** | The lower-rate, fewer-deductions income-tax slabs (`tax_estimator`, `advance_tax_planner`). |
| **V-CIP** | Video-based Customer Identification Procedure — remote liveness KYC (`kyc_orchestrator`). |
| **SDD / EDD** | Simplified vs Enhanced Due Diligence — the KYC tier chosen by risk score (`kyc_orchestrator`). |
| **PEP** | Politically Exposed Person — triggers enhanced screening (`kyc_orchestrator`). |
| **DigiLocker / UIDAI** | Government document repository and the Aadhaar-issuing authority; offline KYC uses UIDAI XML, never the full Aadhaar number on the bus. |
| **Advance tax** | Quarterly estimated tax for the self-employed/high-income (`advance_tax_planner`). |
| **Tax harvesting** | Realising losses to offset capital gains and cut tax (`tax_harvester`). |
| **Prepayment** | Early repayment of a loan before maturity, possibly with a penalty (`prepayment_advisor`). |
| **Dividend** | A profit distribution to shareholders (`dividend_planner`). |

---

## 12 · Known gaps & not-yet-wired (honest scope)

Documented so no one mistakes the config surface for the enforced behaviour.

| Term | Definition |
|---|---|
| **`SchemaPolicy` unwired** | Implemented but never constructed by `BuildComposite` — JSON-schema validation is dead code today. |
| **Classification ceiling hardcoded** | `ClassificationPolicy.AllowedTo` is never populated from YAML; the ceiling is fixed to `internal` for all recipients. |
| **`AllowCrossBorderForPublic` forced** | `BuildComposite` forces this `true`, ignoring the YAML value. |
| **Budget principal = region** | The daily token cap keys on region, not the user — a stand-in for per-principal accounting. |
| **Warehouse sink unwired** | `pkg/observability/bq` exists as an interface + JSONL impl but has no call site. |
| **Cloud LLM providers unwired** | Anthropic/Gemini/OpenAI sit behind the `Provider` interface but aren't in the running stack (mock + Ollama are). |
| **Rate limiter keys on IP** | It runs before auth, so it limits per-`RemoteAddr`, not per-JWT-subject. |
| **af-serve auth** | The AFG edge has no production JWT/RBAC middleware yet, and RAG/wrapper-chain aren't re-hosted onto it — it carries governance context on the request instead. |
| **Representative agents** | A few advisory/pipeline agents in the AFG catalog are representative rather than byte-faithful ports of the bus-edge originals. |

---

*Keep this in sync with the code: identifiers and package paths here are grep-able
against the repo. If you rename a package or policy, update its row.*
