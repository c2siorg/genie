# Extending Genie with ADK-Inspired Agents

Source: <https://github.com/google/adk-samples/tree/main/python/agents>

Cross-referenced against Genie's existing 48 agents. The ADK samples cover a wider surface area than just finance — but several patterns translate directly into the Indian banking + FREE-AI context Genie targets. Grouped by strategic fit.

---

## What we already have (no work needed)

| ADK sample | Genie equivalent |
|---|---|
| Currency Agent | `agents/currency` |
| Financial Advisor (educational) | `agents/educator` + `agents/portfolio_advisor` |
| LLM Auditor | `agents/auditor` |
| Hierarchical Workflow Automation | `agents/h_supervisor` |
| RAG (Vertex AI) | `pkg/rag` (hybrid + pgvector + Self-RAG + CRAG) |
| Order Processing (HITL) | `pkg/workflow` HITL gates |
| Safety Guardrail Plugins | `pkg/safety` + `pkg/governance` composite |

Worth noting these in the README's "prior art" section so reviewers see the gaps are intentional, not unexamined.

---

## Tier 1 — Direct fits, fill real gaps (recommended next)

These map cleanly to Indian banking use cases Genie doesn't yet cover. Each one is a single new agent (or small cluster) following the existing `agents/<name>/<name>.go` pattern.

### 1. `agents/kyc_orchestrator` — Full KYC workflow

**ADK source:** `global-kyc-agent`
**Why it fits:** Genie has `synthetic_identity` (anomaly detection on KYC data) but no orchestrated KYC workflow. RBI Master Direction on KYC mandates Aadhaar/PAN/address/PEP/sanctions checks in a defined sequence with documented decisioning.
**What it does:** sequences PAN validity (5th-char rule) → Aadhaar offline KYC → DigiLocker fetch → name-match scoring → PEP/sanctions check → liveness gate → final risk-graded decision. Outputs a structured KYC packet + Annexure VI on rejection.
**Risk class:** High.
**Effort:** ~2 days.

### 2. `agents/claim_adjudicator` — Bancassurance claims

**ADK source:** `claim-adjudication-agent`
**Why it fits:** every major Indian bank is now a corporate agent for insurance. Bancassurance claims flow through the bank's app but adjudication logic is bespoke per insurer. A pluggable adjudicator with the insurer-specific rules as YAML aligns with the board-policy-as-YAML pattern.
**What it does:** parses claim documents → matches against policy clauses → flags exclusions → computes payout estimate → routes high-value claims to HITL.
**Risk class:** High.
**Effort:** ~2 days.

### 3. `agents/sme_loan_workflow` — End-to-end SME lending

**ADK source:** `small-business-loan-agent`
**Why it fits:** Genie has the *primitives* (`cashflow_underwriter`, `invoice_discounter`, `working_capital`) but no orchestrating workflow that takes an SME from application to disbursal. The CGTMSE / Mudra schemes have well-defined eligibility — perfect for a deterministic workflow with LLM-assisted document parsing.
**What it does:** GST data fetch → bank statement analysis (via `cashflow_underwriter`) → CGTMSE eligibility → indicative offer → HITL approval gate → sanction letter draft.
**Risk class:** High.
**Effort:** ~3 days (uses `pkg/workflow` DAG + Saga).

### 4. `agents/invoice_processor` — Invoice OCR + workflow

**ADK source:** `invoice-processing`
**Why it fits:** Genie has `receipt_ocr` for personal use but not B2B invoices (GST line items, HSN codes, vendor master matching). Critical for SME current accounts and TReDS.
**What it does:** vision parse → GSTIN validation → vendor master match → 3-way match (PO/GRN/invoice) → posting recommendation.
**Risk class:** Medium.
**Effort:** ~2 days.

### 5. `agents/deep_research` — Multi-turn research agent

**ADK source:** `deep-search`
**Why it fits:** `macro_research` is one-shot. RBI MPC analysis, sector deep-dives, regulatory circular impact assessments need iterative search → synthesise → re-search → cite. ReAct loop with web/RBI-corpus tools.
**What it does:** wraps `reasoning.ReAct` with a regulator-corpus tool, Sahamati-specs tool, and FRED/macro tool. Returns a cited brief.
**Risk class:** Medium.
**Effort:** ~2 days (most plumbing exists in `pkg/reasoning`).

### 6. `agents/bulk_statement_analyzer` — High-volume document analysis

**ADK source:** `high-volume-document-analyzer`
**Why it fits:** the canonical pipeline (ingestor → analyzer) handles one statement at a time. SME lending and account aggregator flows ingest dozens of statements per applicant. Needs batched, parallelised, deduplicated processing with a single rolled-up report.
**What it does:** fan-out N statements to parallel pipelines, dedupe transactions across statements, produce consolidated cashflow + ratios.
**Risk class:** Medium.
**Effort:** ~2 days (orchestration pattern, no new primitives).

### 7. `agents/fomc_research` — Central-bank event analysis

**ADK source:** `fomc-research`
**Why it fits:** Indian equivalent is RBI MPC + monetary policy statements. Genie has `rate_watcher` for current rates but no event-driven analysis of MPC minutes / governor statements.
**What it does:** fetches MPC minutes → diffs against prior → extracts policy signals → estimates rate-path probabilities → publishes to bus for downstream agents (`loan_advisor`, `prepayment_advisor`) to recompute recommendations.
**Risk class:** Low.
**Effort:** ~1.5 days.

### 8. `pkg/policy/dsl` — Policy-as-code DSL

**ADK source:** `policy-as-code`
**Why it fits:** Genie's board policy is already YAML, but every new rule requires a Go-side policy struct. A small DSL (or Rego/CEL adapter) would let the risk team express rules without engineering involvement. Directly strengthens FREE-AI Rec 6 (Adaptive Policies).
**What it does:** loads `*.rego` or `*.cel` rules from `config/policies/`, compiles at boot, exposes as `governance.Policy` instances.
**Risk class:** Low (infrastructure).
**Effort:** ~3 days (CEL is simpler if going that route — `github.com/google/cel-go`).

---

## Tier 2 — Pattern adoption (refines existing primitives)

These are not new agents — they're techniques worth folding into existing packages.

### 9. SkillToolset progressive disclosure

**ADK source:** `agent-skills-tutorial`
**Where it lands:** `pkg/agent`. The pattern: an agent exposes a *skill manifest* with short descriptions; tools are surfaced to the LLM only when the matching skill is invoked. Reduces context bloat on supervisors with many sub-agents.
**Effort:** ~1 day refactor on `h_supervisor`.

### 10. Memory Bank pattern

**ADK source:** `memory-bank`
**Where it lands:** `pkg/memory`. Adds a third memory tier: **long-term consolidated facts** that survive across sessions, written by an offline consolidation job. Today Genie has semantic + episodic; this adds the "what does the system *know* about Alice's financial life" layer.
**Effort:** ~1.5 days.

### 11. Multi-format hybrid RAG

**ADK source:** `multiformat-hybrid-rag`
**Where it lands:** `pkg/loader`. Today: PDF/HTML/DOCX. Add XLSX (statement Excel exports) and image-rich PDFs (scanned cheques, KYC docs).
**Effort:** ~1 day per format.

### 12. Realtime conversational (streaming voice)

**ADK source:** `realtime-conversational-agent`
**Where it lands:** `agents/voice`. Today: Bhashini-shaped batched transcribe. Upgrade to streaming via WebSocket + a streaming ASR provider interface.
**Effort:** ~2 days.

### 13. Pluggable safety guardrails

**ADK source:** `safety-plugins`
**Where it lands:** `pkg/safety`. Today: heuristic + single-LLM jailbreak detector. Refactor to a plugin interface so `Model Armor` (or any third-party shield) can be dropped in.
**Effort:** ~1 day.

---

## Tier 3 — Adjacent verticals (worth doing if Genie targets bancassurance / SME)

### 14. `agents/auto_insurance` — Bancassurance auto

**ADK source:** `auto-insurance-agent`
**Why:** every bank co-sells motor insurance. A claims-and-roadside agent inside the bank's assistant is a real product play.
**Risk class:** Medium. **Effort:** ~2 days.

### 15. `agents/health_preauth` — Medical pre-authorization

**ADK source:** `medical-pre-authorization`
**Why:** bancassurance for health. Same shape as claim adjudicator but tuned to pre-authorisation cycles (cashless network hospitals).
**Risk class:** High. **Effort:** ~2 days.

### 16. `agents/supply_chain_finance` — SCF for SME

**ADK source:** `supply-chain`
**Why:** layered on `invoice_discounter` + `working_capital`. Tracks buyer-supplier chains, flags concentration risk, recommends TReDS auction timing.
**Risk class:** Medium. **Effort:** ~3 days.

### 17. `agents/payment_orchestrator` — UPI / RTGS / NEFT

**ADK source:** `antom-payment`
**Why:** Genie can analyse and recommend, but cannot *execute* payments. A payment orchestrator with HITL approval at thresholds + AP2/UPI Circle integration is the bridge to agentic commerce.
**Risk class:** High (money movement). **Effort:** ~4 days (requires real PSP integration).

---

## Tier 4 — Infrastructure (lower urgency, real value)

### 18. `agents/cyber_guardian` — Anomalous-access detection

**ADK source:** `cyber-guardian-agent`
**Why:** complements `fraud` (transaction-level) with session/access-level anomaly detection — impossible-travel logins, credential-stuffing patterns, device fingerprint changes.
**Effort:** ~2 days.

### 19. `pkg/observability/bq` — Agent observability sink

**ADK source:** `agent-observability-bq`
**Why:** Genie ships OTel → Tempo. Some banks already standardise on BigQuery / Snowflake for analytics. A pluggable observability sink to dual-write traces + metrics to a warehouse enables long-horizon agent performance analytics.
**Effort:** ~2 days.

### 20. `agents/google_trends` — Sentiment / search-interest signal

**ADK source:** `google-trends-agent`
**Why:** feeds `macro_research` and `mf_screener` with consumer-interest signals (e.g. EV trend → auto-fund recommendation).
**Effort:** ~1 day.

---

## Recommended next sprint

If you want a focused 2-week sprint that materially expands Genie's coverage, my pick:

1. **`kyc_orchestrator`** (Tier 1.1) — closes a major FREE-AI compliance gap
2. **`sme_loan_workflow`** (Tier 1.3) — ties together three existing agents into a real product
3. **`invoice_processor`** (Tier 1.4) — unlocks SME current account + TReDS use cases
4. **`fomc_research`** (Tier 1.7) — small, demonstrates event-driven cross-agent updates
5. **`pkg/policy/dsl` with CEL** (Tier 1.8) — strengthens the FREE-AI Rec 6 story significantly

Total effort estimate: ~11.5 days. Each is independently shippable, each adds a section to the README, and the cluster gives Genie a credible "SME banking + KYC" story to pair with the existing retail-banking-and-tax story.

The Tier 2 pattern adoptions (especially **memory bank** and **SkillToolset**) can be folded into whichever sprint touches the adjacent code.

---

## What I'd skip

- **Marketing Agency, Brand Search Optimization, GenMedia for Commerce, Short Movie Agents, Travel Concierge, Personalized Shopping, YouTube Analyst, Story Teller, Fun Facts** — not aligned with Genie's mission.
- **SDLC agents, SWE Benchmark, Software Bug Assistant** — developer tools, not financial.
- **Data Science Agent / Data Engineering Agent** — Genie's analyzer covers this in the finance domain; the ADK versions are general-purpose BigQuery orchestrators.

---

## Attribution

Any agent ported from the ADK samples should credit the original in the agent's Go file header and in the README's "References" section, consistent with how Genie already credits the ADK-inspired agents (educator, currency, macro, rates, loan, auditor).
