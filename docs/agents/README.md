# Agents — Detailed Reference

Genie ships 60+ specialist agents organised in three layers:

1. **Canonical MARA pipeline** (ingestor → normalizer → enricher → analyzer → forecaster → anomaly → recommender → reporter + supervisor)
2. **Domain-expansion agents** (fraud, AML, VaR, ALM, LCR, tax-harvester, cashflow-underwriter, mule-detector, complaint-triage, carbon-estimator, etc. — 26 in total)
3. **ADK-inspired extension agents** (the new cohort documented in detail here)

This subtree of the docs covers the third layer. The other two are documented at the package level (see `agents/<name>/<name>.go` headers) and in the root README's tables.

---

## ADK-inspired agents — index

### Tier 1 — KYC, claims, lending, research

| Doc | Risk | What it does |
|---|---|---|
| [kyc_orchestrator.md](kyc_orchestrator.md) | High | Full RBI Master Direction KYC: PAN → Aadhaar offline → DigiLocker → PEP/sanctions → SDD/standard/EDD or auto-reject with Annexure VI |
| [claim_adjudicator.md](claim_adjudicator.md) | High | Bancassurance claims rule engine; insurer policies as YAML; HITL above ₹2L |
| [sme_loan_workflow.md](sme_loan_workflow.md) | High | End-to-end SME lending DAG: GST → cashflow → CGTMSE → indicative offer → HITL → sanction letter |
| [invoice_processor.md](invoice_processor.md) | Medium | B2B invoice OCR + GSTIN validation + 3-way match (PO/GRN/invoice) |
| [deep_research.md](deep_research.md) | Medium | Multi-turn ReAct over RBI/Sahamati/FIU-IND corpora; offline fallback for sandbox + CI |
| [bulk_statement_analyzer.md](bulk_statement_analyzer.md) | Medium | Consolidates N statements across accounts; dedups inter-account transfers |
| [mpc_research.md](mpc_research.md) | Low | RBI MPC event analyser with fan-out hints to loan/prepayment/rate agents |

### Tier 3 — Bancassurance & payments

| Doc | Risk | What it does |
|---|---|---|
| [auto_insurance.md](auto_insurance.md) | Medium | Motor FNOL + total-loss (≥75% IDV) + roadside dispatch + NCB-ladder renewal |
| [health_preauth.md](health_preauth.md) | High | IRDAI cashless pre-auth with PPN gate, PED waiting, room-rent proportionate deduction, HITL ≥₹5L |
| [supply_chain_finance.md](supply_chain_finance.md) | Medium | Buyer concentration + TReDS auction candidate selection |
| [payment_orchestrator.md](payment_orchestrator.md) | High | UPI/IMPS/NEFT/RTGS rail routing with time-of-day + HITL ≥₹50k |

### Tier 4 — Cyber & signals

| Doc | Risk | What it does |
|---|---|---|
| [cyber_guardian.md](cyber_guardian.md) | Medium | Session-level anomaly stack: impossible travel, credential stuffing, device fingerprint churn |
| [google_trends.md](google_trends.md) | Low | Surging/fading/steady classifier; fans out hints to macro_research + mf_screener |

---

## The agent contract

Every agent in Genie implements `pkg/agent.Agent`:

```go
type Agent interface {
    ID() string                  // stable routing identity
    Name() string                // human-readable label
    Capabilities() []string      // skill keywords for the registry
    HandleMessage(ctx, msg, env) ([]Message, error)
}
```

Plus the optional `pkg/agent.RiskAware` interface that the orchestrator
checks before dispatching to high-risk agents:

```go
type RiskAware interface { RiskLevel() RiskClass }
```

Risk levels (`pkg/agent/risk.go`):

- **RiskLow** — internal automation, glossary lookups, document summarisation. Failure is contained.
- **RiskMedium** — customer-facing assistance, fraud signals, basic chatbots. Errors may inconvenience customers; human override expected.
- **RiskHigh** — credit decisioning, autonomous fund movement, autonomous KYC. Errors have material customer or systemic consequences; HITL mandatory.

The orchestrator enforces ceilings: a `RiskHigh` agent cannot execute on
a message that lacks `advisor` or `admin` role on the inbound metadata.

---

## The standard fields on every agent

Each agent doc in this folder follows the same template:

1. **Overview** — one paragraph on the purpose, the ADK source it's inspired by, and the FREE-AI / Indian-banking context it serves.
2. **Constants** — `ID`, `Capability`, `TypeIn`, `TypeOut`, `NextAgent`. These are exported package constants so callers don't need to import string literals.
3. **Risk class** — declared via `RiskLevel()`.
4. **Request / Response types** — the JSON shapes that flow through `msg.Content`.
5. **Business rules** — every threshold, every percentage, every regulatory citation, in plain English.
6. **Decision logic** — pure function, easy to unit-test in isolation.
7. **Example** — a real-shaped JSON request and the verdict you'd see.
8. **HandleMessage** — what comes in, what goes out, with a sequence sketch.
9. **FREE-AI alignment** — recommendation numbers and what they ask for.
10. **Integration** — how the agent fits into the canonical pipeline or stands alone.
11. **Anti-patterns** — common ways teams misuse the agent or break the contract.
12. **Testing** — what the test suite covers + how to extend it.
13. **References** — RBI / IRDAI / FIU-IND / NPCI source documents.

---

## Scaffolding a new ADK-inspired agent

```bash
make scaffold name=auction_bidder cap=auction_bid in=auction_request out=auction_bid next=financial_supervisor
```

That generates `agents/auction_bidder/auction_bidder.go` + a passing test
file. Replace the `TODO` in `HandleMessage` with real logic, decide a
risk class, give every output a `Disclaimer`, and (for high-risk rejects)
an `IncidentPayload` ready for Annexure VI.

When you commit, also add `docs/agents/auction_bidder.md` following the
13-section template above. The doc is part of the contract.

---

## Where to file complaints

- Bug in an agent's logic → file against this repo with the agent ID
- Suggestion for a new agent → open an issue tagged `agent-proposal`
- Compliance escalation → see [free-ai-mapping.md](../free-ai-mapping.md) for the relevant recommendation and the file that implements it
