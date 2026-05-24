# LinkedIn post — promoting the RBI FREE-AI compliance article

Three variants. Tone: *we read the report, we tried to map it to running code, sharing it openly in case it helps someone else.*

---

## Option A — "here's what we built" (~1,400 chars)

When the RBI published the FREE-AI report last August — 7 Sutras, 6 Pillars, 26 Recommendations — we sat with it for a while and asked a simple question:

*If we took this seriously, what would the code actually look like?*

Over the last few months we've been building **Genie**, an open-source AI financial assistant in Go, with FREE-AI as the design spec. Every architectural choice has a recommendation number next to it. Today we're sharing what we shipped, what we learned, and what's still in progress — in case it's useful to anyone else building under the same framework.

A few of the mappings that worked well for us:

→ **Rec 4 (Indigenous models)** — every LLM provider declares `Region()`. A residency policy on the message bus denies PII bound for non-home regions before the LLM call is made. On-prem Ollama for hot-path, hosted models only for public queries.

→ **Rec 14 (Board-approved policy)** — the policy is a YAML file in version control with a `board_approved_on` field. The board edits a file; the running system obeys it.

→ **Rec 15 (Data lifecycle)** — envelope AES-256-GCM with KMS-wrapped DEKs. Per-row `kek_id` so rotation is a column, not a migration.

→ **Rec 20 (Red teaming)** — an adversarial probe corpus runs against the *active* policy on every commit. If a guardrail breaks, CI fails.

→ **Rec 22 (Annexure VI)** — auto-generated on policy deny, agent panic, or budget breach. Hash-chained into a tamper-evident audit log.

Full write-up with every recommendation, the engineering pattern, and the file paths ↓

Repo (MIT): https://github.com/c2siorg/genie

If you're working on something similar — at an RE, fintech, regulator, or vendor — would love to compare notes on what's working for you.

#RBI #FREEAI #ResponsibleAI #SovereignAI #FinTechIndia #BankingCompliance #OpenSource

---

## Option B — "sharing our notes" (~1,200 chars)

Sharing some work we've been doing on RBI FREE-AI implementation, in case it's useful.

When the report came out last August — 26 recommendations across 6 Pillars — we started building **Genie**, an open-source AI financial assistant in Go, treating each recommendation as a design requirement rather than a documentation exercise. Nine months in, here's the long-form write-up of how we translated the policy text into running code.

What's in it:

→ A walkthrough of every relevant recommendation (Recs 2, 4, 6, 8, 14–26) with the engineering pattern we landed on and the file path that implements it
→ Three things FREE-AI doesn't say but every implementer hits — async trace propagation, tamper-evident audit logs, consent ledgers
→ A one-page table mapping each Rec to its pattern, so you can compare against your own approach
→ An honest list of what's still partial (Rec 17 and 24 are works in progress)

Worked examples are from Genie (MIT licensed, on GitHub), but the patterns are portable to any stack. We've tried to be specific enough that you could fork the ideas without forking the code.

If you're building under FREE-AI at an RE, a fintech, an SRO, or a regulator, and you've taken different design decisions — we'd genuinely like to hear what's worked for you.

Full article ↓
Repo: https://github.com/c2siorg/genie

#RBI #FREEAI #ResponsibleAI #SovereignAI #FinTechIndia #AIGovernance #OpenSource

---

## Option C — short and direct (~800 chars)

We've spent the last few months building **Genie** — an open-source AI financial assistant in Go — with the RBI FREE-AI report as the design spec. Each of the 26 recommendations is wired to a specific package, with tests and CI gates that verify it stays wired.

Sharing the long-form write-up of how each recommendation translated into code: the residency policy, the board-approved YAML loader, the envelope encryption, the auto-generated Annexure VI, the hash-chained audit log, the fallback agents, the live AI inventory, the public disclosures.

MIT licensed and on GitHub if you'd like to take a look, fork the ideas, or contribute back. If you're building under FREE-AI at an RE, fintech, or regulator — we'd love to compare notes.

Full article ↓
Repo: https://github.com/c2siorg/genie

#RBI #FREEAI #ResponsibleAI #SovereignAI #FinTechIndia #OpenSource
