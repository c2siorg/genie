# Annexure VI as a Query

*Building incident reporting that survives an audit — and why the regulator's most uncomfortable question should become a SQL query.*

---

## The Friday-afternoon scramble

A regulator's email lands at 2 PM on Friday. They want a report by Monday on every AI-driven decision rejection in the last 90 days, broken down by severity, with the underlying inputs (with PII redacted) and the policy each rejection cited.

If you're running a typical multi-LLM stack:

- Logs are in five different formats (the LLM provider's, the orchestrator's, the application's, the audit DB's, the SIEM's).
- "Rejection" doesn't have a single source of truth — sometimes a guardrail fires, sometimes an exception bubbles up, sometimes the LLM just declines.
- The "policy each rejection cited" doesn't exist in any structured form — the safety system returned `{flagged: true}` and that's that.
- PII redaction is a one-off script someone wrote in March; you can't remember if it handles the new product lines.

You spend the weekend writing a script. You deliver a 73-page PDF on Monday. The regulator scans it and writes back with three follow-up questions on the same data, sliced differently. You spend another weekend.

This is what **FREE-AI Rec 22 (AI Incident Reporting)** is asking you to avoid. The recommendation references **Annexure VI** of the report — a structured form for reporting AI incidents to the RBI. The goal: a query, not a scramble.

---

## The mental shift

The old model: incidents are *logged*. Each incident is a line of unstructured text or a half-structured JSON blob that the application happened to emit. Reporting them later means re-deriving structure from logs.

The new model: incidents are *records*. Each incident is a structured artefact that conforms to Annexure VI's schema *at the moment of creation*. Reporting them later means filtering a structured table.

In the new model, the regulator's "give me a report" becomes a SQL query against a table whose schema *is* Annexure VI's required fields. The work is the schema — once.

---

## What Annexure VI wants

Without quoting the report verbatim, Annexure VI broadly requires (for each incident):

- An incident identifier
- The date / time of detection
- The AI system or capability involved
- A severity grading
- The nature of the incident (deny, error, breach, data leak, etc.)
- The customer or process affected (count, demographic, financial impact)
- Root cause analysis
- Remediation action taken
- Disclosure status (was the customer notified?)

That's a schema. Not a paragraph. Not a slide. A schema.

---

## How Genie does it

Three pieces work together:

### 1. The grading function

`pkg/incidents.Grade(incident) Severity` is a pure, deterministic function. It takes an incident (a structured event from anywhere in the system) and returns one of `Informational | Low | Medium | High | Critical`.

The function considers:

- Financial impact (₹ involved)
- Customer harm (data leak vs minor inconvenience)
- Reversibility (can the action be undone?)
- Number affected (one customer vs many)

The output drives escalation:

| Grade | Action |
|---|---|
| Informational | Audit log only |
| Low | Audit log + daily digest |
| Medium | Audit log + page on-call |
| High | Audit log + auto-generate Annexure VI form + page on-call |
| Critical | All of the above + BCP drill |

### 2. The structured payload

Every place in the system that could produce an incident emits a structured payload:

```go
type IncidentPayload struct {
    Annexure     string            // "VI"
    IncidentID   string            // uuid
    OccurredAt   time.Time
    System       string            // "kyc_orchestrator" | "payment_orchestrator" | ...
    Capability   string            // the agent's declared capability
    Severity     Severity          // from Grade()
    Nature       string            // "policy_deny" | "agent_panic" | "budget_breach" | ...
    Reason       string            // human-readable why
    AffectedID   string            // customer id, account id, etc.
    Financial    float64           // ₹ impact, if applicable
    Reversible   bool
    PolicyName   string            // which composite policy fired
    PolicyRuleID string            // which DSL rule, if any
    Action       string            // what the system did
    Metadata     map[string]string // free-form, but typed values please
}
```

That payload goes into `incidents` (a Postgres table — or in dev, the in-memory store), and a column-equivalent row goes into the warehouse via `pkg/observability/bq`.

### 3. Auto-generation at the source

Critically, every place that produces an incident produces the structured payload **automatically**, not as a post-hoc reconciliation step. Examples from Genie:

- **KYC orchestrator** — sanctions hit produces `{Severity: High, Nature: "policy_deny", PolicyName: "sanctions_match"}`.
- **Payment orchestrator** — any reject produces `{Severity: Medium, Nature: "payment_reject", Reason: "..."}`.
- **Bus governance** — every denied message produces a payload with the policy name.
- **LLM budget** — when the per-principal budget is exceeded, the wrapper emits `{Severity: Medium, Nature: "budget_breach", Reason: "daily token cap"}`.
- **Circuit breaker** — opening produces `{Severity: Medium, Nature: "circuit_open", Reason: "5 consecutive errors"}`.
- **Safety scorer** — high-score jailbreak detection emits `{Severity: Medium, Nature: "safety_flag"}`.

All of those go through `Grade()` to assign severity, then into the table.

---

## What this buys you when the email lands

Suppose the regulator asks: "Show me all high-grade incidents in the last 90 days affecting customer onboarding, with the policy that fired."

```sql
SELECT incident_id, occurred_at, system, reason, policy_name, action
FROM incidents
WHERE severity = 'High'
  AND occurred_at >= NOW() - INTERVAL '90 days'
  AND system IN ('kyc_orchestrator', 'synthetic_identity', 'cyber_guardian')
ORDER BY occurred_at DESC;
```

That's the response. PII is already redacted (no customer names or full account numbers in the payload — only `AffectedID`, an opaque pseudonym). Reproducible by anyone with read access.

You're done by 2:30 PM on Friday.

---

## The hash-chained audit log

A subtle but important detail: a bank's incident log is one of the most attacked assets in the system. An attacker who can rewrite the incident log can hide everything else.

`pkg/compliance/audit.go` implements a **hash-chained** audit log: each entry includes the SHA-256 of the previous entry. Tampering breaks the chain; the next verification pass detects it.

This is not a blockchain. It's a Merkle-style chain anchored periodically to an external timestamp (S3 + Object Lock, or a notary service). Boring, well-understood, works.

When the regulator asks "can you prove this log hasn't been altered?", the answer is "here's the chain; here's the most-recent anchor timestamp; verify."

---

## Why "at the source" matters

A common anti-pattern: an "incident reconciliation job" that runs nightly, scans application logs, and produces incidents from grep patterns. This is what most teams build first, and it has three failure modes:

1. **Log retention**. If your logs roll off after 7 days, the job can't reconcile beyond that.
2. **Structure drift**. The grep patterns assume log line shapes that change when someone refactors.
3. **Missed signals**. The application knows when something is an incident; the log doesn't necessarily.

Auto-generation at the source avoids all three. The application — the KYC orchestrator, the payment orchestrator, the policy engine — *knows* when it's producing an incident, because it just denied a message or panicked. It emits the structured payload directly into the incident store. No reconciliation, no scanning, no inference.

---

## Disclosure is part of the form

Annexure VI has a "disclosure status" field. Was the customer informed? When? Through what channel?

A common gap: the incident is logged, the customer is notified by email, but the notification is in a different system from the incident log. The disclosure column stays empty because nobody wires it up.

The fix: every place the customer is notified about an AI-driven outcome emits a `Disclosure` event keyed by the incident ID. A nightly join updates the disclosure column. The compliance team can see at a glance how many medium-grade incidents have outstanding disclosures.

In Genie, the customer-facing reports (from `agents/reporter`) include the AI disclosure banner as the first SSE event and the first field in JSON. If a rejection produced an incident, the reporter knows the incident ID and stamps it into the disclosure event. Loop closed.

---

## What "Friday afternoon" looks like in the new model

Same scenario:

- Regulator emails at 2 PM Friday.
- 2:05 PM: open the incidents table, write the SQL.
- 2:15 PM: run it through a CSV export.
- 2:20 PM: open a one-page report template, paste the data, click PDF.
- 2:25 PM: send.

If the regulator asks a follow-up, change the SQL `WHERE` clause and re-send. Five minutes.

This is what *operational excellence in compliance* looks like. It's not a bigger team. It's not a slicker dashboard. It's the right schema, populated at the source, queryable on demand.

---

## The five rules

1. **Schema, not paragraphs.** Annexure VI's required fields become your `incidents` table columns.
2. **Auto-generate at the source.** The application emits the structured payload. No reconciliation jobs.
3. **Grade deterministically.** A pure function maps incident → severity. Liability follows the grade.
4. **Hash-chain the log.** Tamper evidence isn't optional in a regulated system.
5. **Close the disclosure loop.** Every notification updates the disclosure column.

---

## The repo

Genie is open source under MIT.

- `pkg/incidents/` — grading + Annexure VI payload
- `pkg/compliance/audit.go` — hash-chained audit log
- `agents/kyc_orchestrator/` — emits incidents on sanctions match
- `agents/payment_orchestrator/` — emits incidents on reject
- `docs/free-ai-mapping.md` — Rec 22 cross-walk

```bash
git clone https://github.com/c2siorg/genie.git
curl -H "Authorization: Bearer $ADMIN" localhost:8080/v1/incidents | jq .
```

---

The Friday-afternoon scramble is a choice. If you've replaced it with a query in your shop, what did the schema migration look like? Always interested in how others sliced this problem.

#ResponsibleAI #RBI #FREEAI #AuditAndCompliance #FinTechIndia #BankingAI #IncidentReporting #DataGovernance
