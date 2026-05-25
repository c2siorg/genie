# The Decision Is Deterministic. The LLM Just Talks.

*A pattern for high-stakes automation in regulated industries — illustrated with a full RBI KYC orchestrator that an LLM never actually decides.*

---

## The trap

I've sat through enough "AI in banking" demos this year to spot the failure mode at fifteen paces.

A startup founder fires up a chat window. Types: *"Process this KYC application for me."* The LLM thinks for a beat, then announces: "I've reviewed the documents and recommend approval at standard tier."

It's a great demo. It would also be the last demo before the regulator's letter.

What just happened? A model with no specification produced a decision with regulatory consequences. The reasoning is opaque. The audit trail is the prompt and the response. The "I approve / EDD / reject" decision rule lives nowhere — it's diffused through whatever the model felt like that turn. If the same application came in tomorrow with the same data, the answer might be different. If asked to explain, the model would confabulate a rationale that doesn't necessarily match what actually drove the output.

This is the **everything-is-the-model** trap. And it's why every serious AI-in-regulated-industries project converges on the same pattern eventually:

> **The decision is deterministic. The LLM just talks.**

---

## What "deterministic" means here

Two properties:

1. **Same input → same output, every time.** A pure function. No hidden state, no temperature, no model sampling. You can replay any past decision and get the same answer.

2. **Every output is traceable to a named rule.** When the system says "EDD tier, score 0.75, because (a) PEP hit, (b) high-risk jurisdiction, (c) weak address match," each of those reasons is a line of code with a regulatory citation.

The LLM still has a role — but it's not deciding. It's:

- Paraphrasing the decision into something the customer can read
- Generating the regulator's incident form into prose
- Asking clarifying questions if the user's input is ambiguous
- Drafting the next-step instructions

Decisions live in a pure function. Communication lives in the LLM. The audit trail is the function, not the prompt.

---

## A worked example: RBI KYC

The Reserve Bank of India's **Master Direction on KYC** (DBR.AML.BC.No.81/14.01.001/2015-16) is the canonical Indian KYC playbook. It mandates:

- PAN structural validity check
- Aadhaar offline KYC (online OTP is no longer permitted for full KYC since 2018)
- Name match across documents
- Address validation
- Liveness via V-CIP (Video Customer Identification Process)
- Sanctions / PEP / FATF screening
- Tiering into SDD (Simplified) / standard / EDD (Enhanced Due Diligence)

That's a regulated workflow with regulated thresholds. Genie ships an orchestrator for it. Here's what the decision function looks like — abbreviated:

```go
func Decide(app Application) Verdict {
    score := 0.0
    reasons := []string{}

    // PAN structural validity (10 chars, 4th=P, 5th = surname initial)
    if !panLooksValid(app.PANNumber, app.NameOnPAN) {
        score += 0.30
        reasons = append(reasons, "PAN failed structural validation")
    }

    // Aadhaar offline KYC mandate
    if !app.AadhaarOfflineKYC {
        score += 0.20
        reasons = append(reasons, "Aadhaar offline KYC XML missing")
    }

    // Name match across PAN/Aadhaar
    if !nameTokensOverlap(app.NameOnPAN, app.NameOnAadhaar) {
        score += 0.20
        reasons = append(reasons, "Name on PAN does not share a token with Aadhaar")
    }

    // Sanctions list hit → immediate reject
    if app.SanctionsHit {
        return rejectVerdict(app, 1.0, "Sanctions list hit (OFAC/UN/MHA)")
    }

    // PEP, geo, occupation contributions...

    return Verdict{Decision: classifyTier(score), Reasons: reasons, ...}
}
```

That's the entire decision. Every contribution to the score is a named rule. Every named rule traces to an RBI / FATF citation. The function is a 150-line `_test.go` away from being fully exercised.

The LLM never sees this function. The LLM never decides anything. The LLM gets handed the Verdict and asked: *"Write the customer-facing email explaining this outcome in plain English, in their preferred language."*

---

## Why this works for compliance

A regulator's most uncomfortable question for any AI system is: ***"Why did it produce this output?"***

For a one-LLM-call system, the honest answer is "I don't know — here's the prompt, here's the temperature, here are the model weights, good luck." That's not an answer; that's a confession.

For a deterministic-decision system, the answer is:

> "Score = 0.75 because PEP hit (+0.35), high-risk jurisdiction (+0.20), weak address match (+0.10), liveness below threshold (+0.15). Score ≥ 0.70 routes to EDD per board-approved policy v0.1.0 §risk.scoreEDD. The orchestrator code is at `agents/kyc_orchestrator/kyc_orchestrator.go`, line 130. The test that asserts this routing is `TestPEPRoutesToEDD` at line 67."

Notice what just happened in that sentence. Every claim has a file path. Every threshold has a policy version. Every assertion has a test. The conversation went from "trust me" to "show me the file."

This is what **FREE-AI Rec 25 (Disclosures)** is asking for. The deterministic path gives it for free.

---

## What the LLM still does

Three things the LLM is genuinely good at, none of which are decisions:

### 1. Customer-facing prose

The Verdict struct is structured data. Customers don't want JSON. The LLM takes:

```json
{
  "decision": "edd",
  "reasons": ["PEP hit", "weak address match"],
  "next_steps": ["Compliance officer will review within 48 hours"]
}
```

…and produces:

> "Hi Asha, your application is in additional review. Our compliance team will reach out within 48 hours; this is a normal step for accounts where we need a closer look at the supporting documents. Nothing for you to do right now — we'll be in touch."

Better than templated mail-merge, worse at decisions than a 50-line rule engine. Use it where it shines.

### 2. Drafting the Annexure VI form

When a sanctions hit fires, FREE-AI Rec 22 requires a structured incident form. The LLM doesn't *generate* the form — the orchestrator does, as a JSON payload. But the LLM can take the structured form and produce the narrative summary that goes in the "description of event" field, in the compliance team's preferred tone.

### 3. Clarifying ambiguous inputs

If the user's input has missing or conflicting fields, the LLM asks: *"You marked the customer as a politically exposed person but their occupation field is blank. Should we treat them as PEP for this application?"* The orchestrator then runs the deterministic logic on the clarified input.

---

## The pattern, in five rules

1. **The decision function is a pure function.** No I/O. No clock (except via a passed-in argument for testability). No LLM. No randomness.

2. **Every output carries a reason.** Not a probability score — a *named reason*, traceable to a rule, traceable to a regulation.

3. **Every threshold is a constant or a policy-YAML field.** Not a magic number. The risk team can tune via YAML; engineering doesn't ship code.

4. **The LLM gets the structured output, not the raw input.** It never decides; it only paraphrases.

5. **Every reject carries an Annexure VI payload.** The regulator's question becomes a query against your audit log.

---

## What you give up

This pattern has a real cost. You can't ship the deterministic version as fast as you can ship a prompt-and-pray version. Modeling Indian KYC properly took us ~150 lines of Go and 9 tests. A prompt would have been 30 lines.

You also give up the "delightful surprise" of an LLM finding an edge case the rules didn't anticipate. The rule engine doesn't think laterally — it does exactly what's written.

In regulated industries, both of these are features, not bugs. You **don't want** a banking system that's faster to ship than to audit. You **don't want** an AI system that finds surprising edge cases in your KYC tiering.

What you *do* want: a system where the auditor can replay any decision, the compliance team can change a threshold without a release, the on-call can debug a misfire by reading a single function, and the regulator's most uncomfortable question has a confident, file-path-backed answer.

---

## The broader thesis

This pattern isn't only for KYC. It applies wherever a system makes a regulated decision:

- **Claims adjudication** — sub-limits, exclusions, co-pay math are rules; the LLM writes the customer email.
- **Credit scoring** — the score is a function of inputs; the LLM explains the score in plain English.
- **Payment rail routing** — UPI vs IMPS vs NEFT vs RTGS is a deterministic choice on amount, time, beneficiary trust; the LLM writes the confirmation message.
- **Pre-auth in health insurance** — room-rent proportionate deduction is math; the LLM writes the patient explanation.
- **AML/STR triggers** — velocity, structuring, geography are rules; the LLM drafts the STR narrative.

In every case the same line holds: **the decision is deterministic, the LLM just talks.**

---

## How to look at it

The Genie implementation is open source:

- `agents/kyc_orchestrator/` — the worked example
- `docs/agents/kyc_orchestrator.md` — the deep-dive doc
- `tests/` — the test suite that proves the determinism

```bash
git clone https://github.com/c2siorg/genie.git
cd genie
go test ./agents/kyc_orchestrator/ -v
# 9 tests pass; every branch of the decision tree is exercised.
```

MIT licensed.

---

If you're building AI for regulated industries, the question I'd leave you with: **what fraction of your "AI decisions" should actually be AI decisions?** Most teams overshoot. The pattern above is a way to be honest about which decisions belong in the model and which belong in code.

#ResponsibleAI #RBI #FREEAI #KYC #FinTechIndia #BankingAI #SoftwareArchitecture #Determinism #Compliance
