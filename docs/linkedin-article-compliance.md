# From Policy Text to Production Code: A Field Guide to RBI FREE-AI Compliance

*The August 2025 FREE-AI report has 26 recommendations. This is how the ones that matter most translate from PDF into running systems.*

---

## The 9-month problem

In August 2025, the RBI published the **Framework for Responsible and Ethical Enablement of AI** — FREE-AI. Seven Sutras, six Pillars, 26 Recommendations. The clearest mandate on responsible AI any major central bank has issued.

It is now May 2026. Walk into most Regulated Entities, fintechs, or AI vendors pitching into Indian banking and ask one question: ***"Show me the file."***

Show me the file that says PII can't leave India. Show me the audit log entry that proves which policy was running last Tuesday at 14:32. Show me the red-team probe corpus that fails CI when a guardrail breaks. Show me the Annexure VI form generated when your fraud model misfired in March.

If those files don't exist, the policy is a poster.

This article walks the recommendations that fall on Regulated Entities, the engineering pattern each one requires, and (where useful) a worked example from **Genie**, an open-source Go implementation. If you take the patterns and ignore the code, you've got what you came for.

---

## The Sutras are testable

Two patterns turn the seven principles from a slide into a measurement:

1. **Load them as a system-prompt prefix on every LLM call.** The model sees the principles before it sees the user's question. The prefix is in every prompt; the prompt is in every trace.
2. **Run an LLM-as-judge auditor as a broadcast subscriber on the message bus.** It scores every outbound message against each Sutra (0–1 per principle) and writes the score to the audit log. Anything below threshold triggers an incident.

Now "how aligned were our outputs to People First in Q1?" is a SQL query. That's the difference between *claiming* alignment and *demonstrating* it.

---

## The recommendations, with patterns

### Rec 2 — AI Innovation Sandbox

**The pattern:** the sandbox *is* the production code, with mocked I/O. Same orchestrator, same governance, same agents. One command runs the full pipeline on a sample dataset.

**Anti-pattern:** a "sandbox" that is a separate codebase. The moment it diverges from production — and it always diverges — its tests prove nothing.

### Rec 4 — Indigenous AI Models

What it really asks: sovereign AI. Sensitive data never leaves the country.

**The pattern:** every LLM provider declares `Region()`. Every message carries `Classification`. A residency policy denies PII bound for a non-home region, before the LLM call is made.

```go
func (p *OllamaProvider) Region() string { return "on-prem" }
func (p *AnthropicProvider) Region() string { return "us" }
```

Hot-path traffic (PAN, account, transaction, holdings) → on-prem Ollama. Cold-path (macro research, generic education) → hosted frontier model. The router is 30 lines. The compliance posture is a YAML file.

### Rec 6 — Adaptive Policies

The policy must update without a code release. **The pattern:** YAML in version control, loaded at boot, with `board_approved_on` and `owner` fields. Every load is hash-logged.

```yaml
version: "0.1.0"
board_approved_on: "2025-08-13"
owner: "Chief Risk Officer"
sovereignty: { home_region: "in", allow_cross_border_for_public: true }
explainability: { applies_to: ["recommendations"] }
```

**Anti-pattern:** the policy is a Word doc on SharePoint. Engineers translate it into constants. By Q3, the document and the constants have diverged.

### Rec 8 — Graded Liability

**The pattern:** a small deterministic `Grade(incident) Severity` function considering financial impact, customer harm, reversibility, and number affected. Output maps to action: log, notify, page, auto-generate Annexure VI, trigger BCP. Unit-tested with documented cases.

### Rec 14 — Board-Approved AI Policy

**The pattern:** the YAML from Rec 6 *is* the board-approved policy. Stored in version control. Every change is a PR with a board-resolution reference. The board signs off on the file the system loads. No translation layer, no drift.

### Rec 15 — Data Lifecycle Governance

**The pattern:** envelope encryption. Each document gets a fresh AES-256-GCM **DEK**, wrapped by a **KEK** held in KMS. Raw KEK never touches application memory in prod.

```json
{
  "kek_id": "kms://aws/alias/genie-kek-2025",
  "wrapped_dek": "base64...",
  "nonce": "base64...",
  "ciphertext": "base64..."
}
```

Plus `expires_at` columns, a retention job that purges every 6h, and a KEK rotation schedule (schema accommodates rotation via per-row `kek_id`). The lifecycle is a column, a job, and a key resolver.

### Rec 16 — System Governance + Autonomous Systems

**The pattern:** every agent declares `RiskLevel()`. The orchestrator enforces ceilings — a `RiskHigh` agent (AML, VaR, ALM, fraud) cannot execute without `advisor` or `admin` role. Autonomous loops are bounded by three LLM wrappers: `DeadlineProvider`, `CircuitProvider`, `BudgetedProvider`. Runaway loops self-terminate.

### Rec 18 — Consumer Protection

**The pattern:** AI disclosure banner on every response, text owned by the policy YAML (legal team, not engineering). Ships as the first SSE event in streams, top-level field in JSON. Also exposed publicly:

```bash
curl https://api.example.in/v1/disclosures | jq .
```

So any consumer or journalist can see the active disclosure without authenticating.

### Rec 19 — Cybersecurity

**The pattern:** standard web-app hygiene applied rigorously, plus one AI-specific addition.

Standard: JWT (HS256, short TTL), bcrypt, RBAC at *two* layers (HTTP middleware *and* bus governance — defence in depth), per-principal rate limits, WebAuthn passkeys, OAuth 2.1 + PKCE, OAuth Device Flow for token onboarding.

AI-specific: **prompt injection detection** as a governance policy. Regex on inbound content, optional LLM classifier on top. Denied messages drop, incident is recorded.

### Rec 20 — Red Teaming

**The pattern:** an adversarial probe corpus runs against the **active** composite policy on every commit. Not a mocked policy. The exact bytes production is running.

```bash
make red-team
# OK: all probes denied / allowed as expected.
```

If the board tightens a threshold, the red-team output changes on the next CI run. If a probe that *should* be denied gets through, CI fails. New attack class? Add a probe, fix the policy, the test is permanent.

### Rec 21 — BCP for AI

**The pattern:** every agent has a deterministic fallback that needs neither LLM nor network.

```go
orchestrator.SetFallback("portfolio_advisor", deterministic.PortfolioFallback{})
```

When the primary fails, the user gets a degraded but truthful answer — *"live analytics unavailable, here's your cached snapshot from 14:00 IST"*. Audit log records the fallback. On-call gets paged. CI verifies it: `make bcp-drill` forces a failure and asserts the fallback fires.

### Rec 22 — AI Incident Reporting (Annexure VI)

**The pattern:** auto-generation on any of: governance deny, agent panic, LLM budget breach, circuit trip, safety scorer flag. The form is **structured**, hash-chained into a tamper-evident audit log, exposed via `GET /v1/incidents`. The reporting timeline becomes a query, not a Friday scramble.

### Rec 23 — AI Inventory

**The pattern:** the inventory endpoint is built from the **live registry**. Every registered agent shows up; every deregistered one disappears. Cannot drift from what's actually running because it *is* the same source.

```bash
curl -H "Authorization: Bearer $ADMIN" /v1/ai-inventory | jq .
```

**Anti-pattern:** a spreadsheet maintained by hand. Stale within weeks.

### Rec 24 — AI Audit Framework

**Partial — one of the hardest.** The LLM-as-judge auditor writes Sutra-conformance scores to a structured audit log. Combined with the OpenTelemetry trace (one per user question, every hop), an auditor can pick any request and reconstruct what was asked, which policies fired, which agents ran, what the LLM saw and returned, how the Sutras scored it. Next milestone: signed export in CycloneDX ML-BOM + JSONL for external auditor tooling.

### Rec 25 — Disclosures

**The pattern:** a public unauthenticated endpoint returning active policy version, LLM providers in use, residency posture, data classifications handled, and disclosure text. Linked from every RE's public website. Generated from the live policy; always current.

### Rec 26 — AI Toolkit

**The pattern:** a Sutra Scorecard — one default check per Sutra, runnable on any output. Banks extend with their own checks. The scaffolding is the contribution.

```go
score := toolkit.Scorecard.Run(output)
```

Teams run it locally before submitting for review. Sutras become a feedback loop, not a gate at the end.

---

## What FREE-AI doesn't say but every implementer hits

**1. Trace context across async hops.** Without explicit propagation, your distributed system is six disconnected spans in Tempo. Inject W3C `traceparent` into `Message.Metadata` on publish; extract before each agent runs. One trace per user question, across HTTP → governance → bus → every agent → every LLM call. Use OpenInference semantics so Phoenix/Langfuse pick it up unchanged.

**2. The audit log must be tamper-evident.** Hash-chained entries — each includes SHA-256 of the previous. Anchor periodically to S3 Object Lock or a notary service so an external party can verify the chain. Not a blockchain; a Merkle chain with a trusted timestamp. Boring, works.

**3. Consent must be a ledger.** Append-only `(user_id, data_category, purpose, granted_at, expires_at, revoked_at)`. Every governance evaluation checks it. Revocation is a single insert. DPDP alignment falls out naturally.

---

## The implementation map

| Rec | Pattern | Where (in Genie) |
|---|---|---|
| 2 | Production code with mocked I/O | `cmd/genie` |
| 4 | `Region()` per provider + residency policy | `pkg/llm`, `pkg/governance` |
| 6 | Board-approved YAML loader | `config/ai-policy.example.yaml`, `pkg/policy` |
| 8 | `Grade()` function with documented thresholds | `pkg/incidents` |
| 14 | YAML with `board_approved_on`, hashed at boot | `config/ai-policy.example.yaml` |
| 15 | Envelope AES-GCM + KMS + retention job | `pkg/crypto`, `pkg/storage/postgres` |
| 16 | `RiskLevel()` + budget/circuit/deadline wrappers | `pkg/agent`, `pkg/llm` |
| 18 | AI disclosure as first SSE event | `pkg/web/handlers`, policy YAML |
| 19 | JWT + RBAC × 2 + WebAuthn + injection policy | `pkg/auth`, `pkg/governance` |
| 20 | Probe corpus vs active composite in CI | `cmd/red-team` |
| 21 | Deterministic fallback per agent + CI drill | `agents/fallback`, `make bcp-drill` |
| 22 | Auto-Annexure VI + hash-chained log | `pkg/incidents`, `pkg/compliance` |
| 23 | Live endpoint built from registry | `pkg/registry`, `GET /v1/ai-inventory` |
| 24 | LLM-as-judge + OTel trace export | `agents/llm_auditor`, `pkg/observability` |
| 25 | Public unauthenticated endpoint | `GET /v1/disclosures` |
| 26 | Sutra Scorecard | `pkg/toolkit` |

Recs 1, 3, 5, 7, 9–13, 17 are regulator/SRO actions or partial. Honesty is part of the Sutras.

---

## The takeaway

Three principles hold across all 26:

1. **The policy is the system.** If your governance lives in a Word doc, you have aspirations, not governance. The file the board signs off on must be the file the system loads at boot.
2. **Every claim needs a `grep`-able file path.** "We comply with FREE-AI" is a claim. "We comply because Rec X lives in `pkg/y/z.go`, tested by `pkg/y/z_test.go`, verified in CI by `make red-team`" is a defence.
3. **Verification belongs in CI, not in audit prep.** Red-team probes, BCP drills, conformance tests — they run on every commit. The audit is a query.

Responsible AI is a property of the running system, not the press release. The only way to prove it is to publish the system — or to be able to point a regulator at the equivalent in your own codebase, on demand, with confidence.

Genie is one open-source attempt to make that possible for the FREE-AI era. MIT licensed, honest about what's partial, patterns portable to any stack.

The 26 recommendations are not a wishlist. They are a specification. The longer the industry treats them as the former, the more uncomfortable the next supervisory cycle is going to be.

---

**References**

- [RBI FREE-AI Report (Aug 2025)](https://rbidocs.rbi.org.in/rdocs/PublicationReport/Pdfs/FREEAIR130820250A24FF2D4578453F824C72ED9F5D5851.PDF)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [OpenInference Semantic Conventions](https://github.com/Arize-ai/openinference)
- [Genie — Go reference implementation (MIT)](https://github.com/c2siorg/genie)

*Genie is not affiliated with the RBI. The compliance mapping is a good-faith engineering interpretation of FREE-AI and is not a substitute for a formal regulatory assessment.*

If you're building AI in Indian banking, which recommendations are hardest to implement at your shop? What pattern have you converged on?

#RBI #FREEAI #ResponsibleAI #SovereignAI #FinTechIndia #BankingCompliance #AIGovernance #DPDP #OpenSource
