# Four Rails, One Decision Function: Payment Routing in the NPCI Era

*Building a payment orchestrator that picks UPI vs IMPS vs NEFT vs RTGS — without losing your job to a duplicate ₹50,000 transfer.*

---

## The boring problem

A user taps "Pay ₹3,00,000 to ICICI A/c 00112233."

What should your system do? On the face of it, this is trivial — pick a rail, submit, done. In practice, it's where AI banking platforms either earn their compliance posture or get an uncomfortable letter from the regulator about money movement without proper controls.

The decision matrix:

- **UPI** — instant, free, ≤₹1L per txn, needs VPA. Not available here (amount too high, no VPA given).
- **RTGS** — instant settlement, ≥₹2L, operating window roughly 7am–6pm Mon-Sat. Available now if within window.
- **IMPS** — instant, ≤₹5L, 24×7. Available.
- **NEFT** — batch-settled, 24×7 since 2019, no real ceiling. Available always.

For a ₹3L transfer at 10:30 AM on a Tuesday: RTGS is the right answer (instant, no ceiling concerns, operating window open). At 10:30 PM the same Tuesday: RTGS is closed, IMPS becomes the right answer. On a Sunday afternoon: RTGS is closed all day, IMPS is again the right answer. For amounts below ₹2L: RTGS isn't an option regardless of time.

Now layer in: is this beneficiary trusted? Was a cooling-off period observed? Does the amount need human approval? Is the currency INR? Is there an idempotency key so a retry doesn't duplicate the transfer?

The orchestrator has to get **all of this right**, deterministically, every time, with an audit trail.

---

## What money movement deserves

Three principles I'd put before any "AI" or "agent" feature:

### 1. Determinism

Same inputs, same rail selection, every time. No model temperature deciding which way ₹3L flows. A pure function takes the `Request`, returns an `Instruction`. The instruction can be replayed for any past payment.

### 2. Idempotency

Every payment request requires a client-provided idempotency key. The orchestrator refuses to process without it. **A retry must not duplicate the transfer** — which means the rail layer downstream has to honour the key too.

This sounds like an obvious feature. It is shockingly often missing in v1 of payment systems.

### 3. HITL gate at threshold

Above ₹50,000, the system holds the payment for human approval, regardless of rail. Trusted beneficiary or not. The threshold can move via the board-approved policy, but the gate must exist.

Why ₹50,000? It's the line we picked between low-friction P2P and significant transfers. Your number might be different. The principle: **at some amount, a human's eyes belong on the transaction before it leaves the bank.**

---

## The rail-selection function

Here's the actual function from Genie's `agents/payment_orchestrator`, abbreviated:

```go
func (a *Agent) chooseRail(req Request) (string, []string) {
    now := a.Clock()
    hour := now.Hour()

    // UPI: instant, free, ≤₹1L, requires VPA.
    if req.BeneficiaryVPA != "" && req.AmountRupees <= upiPerTxnLimit {
        return "upi", []string{"Within UPI per-txn limit"}
    }

    // RTGS: instant, ≥₹2L, 7-18 Mon-Sat. For high-value transfers this is
    // the cleanest rail when the window is open, so try it before IMPS.
    if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" &&
        req.AmountRupees >= rtgsMinThreshold &&
        hour >= 7 && hour < 18 && now.Weekday() != time.Sunday {
        return "rtgs", []string{"Amount ≥ ₹2L and within RTGS operating window"}
    }

    // IMPS: instant, 24×7, ≤₹5L, needs IFSC+Account.
    if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" &&
        req.AmountRupees <= impsLimit && strings.ToLower(req.Urgency) != "any" {
        return "imps", []string{"Within IMPS limit and instant credit needed"}
    }

    // NEFT: 24×7 since 2019, batch-settled, no real ceiling.
    if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" {
        return "neft", []string{"Falling back to NEFT batch settlement"}
    }
    return "", nil
}
```

Three things to notice:

1. **The clock is injectable.** `a.Clock()` is a field on the struct, not a call to `time.Now()`. Tests fix the clock and assert "at 10 PM, RTGS is closed → IMPS fires." Production wires `time.Now`.

2. **The function returns reasons.** Every rail selection comes with a list of strings explaining *why* this rail. Those strings end up in the `Instruction.Reasons` field, then in the audit log, then in the dashboard. When something looks wrong six months from now, you can read the trace.

3. **The order matters.** UPI first for small amounts (free, instant), RTGS before IMPS for large amounts in window (real-time finality), NEFT as the catch-all. This ordering is documented and tested.

---

## The five hard rejects

Before rail selection, five conditions hard-reject:

| Condition | Why |
|---|---|
| Currency != INR | The rails this orchestrator knows are NPCI. Cross-border lives elsewhere. |
| Amount ≤ 0 | Pathological input. |
| Missing idempotency key | "Refusing to risk a duplicate transfer." |
| Untrusted beneficiary + amount ≥ ₹50k | HITL gate, regardless of rail availability. |
| No rail matches constraints | E.g. amount > UPI cap, no IFSC+Acct provided. |

Each reject produces an Annexure VI-shaped incident payload (see [Annexure VI as a Query](linkedin-article-annexure-vi.md)). The audit log can be queried six months later: "show me every payment we refused, by reason."

---

## What the orchestrator does NOT do

This is where the design gets opinionated:

### It does NOT submit to the rail

The orchestrator emits an `Instruction`. A separate PSP adapter — the "host concern" in our architecture — picks it up from the bus and submits to NPCI. Why the separation?

Two reasons. First, the rail integration is messy and provider-specific (different PSPs, different APIs, different cert flows); keeping it out of the orchestrator keeps the policy logic clean. Second, **the PSP adapter is where the real money moves** — that's a separate, tightly-controlled service with its own access controls, its own audit, its own approval flow.

The orchestrator is upstream of money movement. The PSP adapter is at money movement. They have different threat models.

### It does NOT check account balance

That's a core-banking call. The orchestrator trusts the upstream UI / business logic to have checked. If the PSP submission fails on insufficient funds, that's an event the orchestrator hears about but doesn't pre-validate.

### It does NOT verify beneficiary cooling-off

The orchestrator trusts the `IsTrustedBeneficiary` flag. The cooling-off — the period during which a newly-added beneficiary can only receive small amounts — is enforced by the customer-onboarding / beneficiary-management service. Setting that flag to true requires an audited admin action upstream.

These three "does NOT" decisions are governance, not laziness. Separation of concerns keeps blast radius small. When the orchestrator misbehaves, balances aren't affected; when the PSP misbehaves, routing logic isn't affected; when the beneficiary service misbehaves, payment math isn't affected.

---

## Tests that matter

Unit tests for a payment orchestrator are deliberately boring. They're also load-bearing:

```go
func TestUPISmallTrustedSubmits(t *testing.T) {
    ins := newAt(midday()).Plan(Request{
        IdempotencyKey: "k1", AmountRupees: 5_000,
        BeneficiaryVPA: "x@upi", IsTrustedBeneficiary: true,
    })
    if ins.Action != "submit" || ins.Rail != "upi" {
        t.Errorf("expected submit via upi; got %s/%s", ins.Action, ins.Rail)
    }
}

func TestRTGSClosedFallsBackToIMPS(t *testing.T) {
    night := time.Date(2026, 5, 14, 22, 0, 0, 0, time.UTC)
    ins := newAt(night).Plan(Request{
        IdempotencyKey: "k4", AmountRupees: 300_000,
        BeneficiaryIFSC: "HDFC0000001", BeneficiaryAcct: "00112233",
        IsTrustedBeneficiary: true,
    })
    if ins.Rail != "imps" {
        t.Errorf("RTGS closed should pick IMPS; got %s", ins.Rail)
    }
}

func TestMissingIdempotencyKeyRejected(t *testing.T) {
    ins := newAt(midday()).Plan(Request{AmountRupees: 100, BeneficiaryVPA: "x@upi"})
    if ins.Action != "reject" {
        t.Errorf("missing idempotency key must reject")
    }
}
```

When a junior engineer comes along and "improves" the rail-selection logic, these tests catch the regression before code review.

---

## What the LLM does (and doesn't)

In this orchestrator: **nothing**. There is no LLM in the rail-selection path. The function is 60 lines of conditional logic over a struct.

The LLM enters elsewhere:

- The customer-facing app uses an LLM to ask "I want to pay rent to Asha" and turn that into a structured `Request` (with VPA/IFSC lookups).
- After the orchestrator emits the `Instruction`, the LLM drafts the SMS / push confirmation in the customer's preferred language.
- After the payment succeeds, the LLM updates the running spend categorisation.

The LLM is on the conversational surface. The decision is in the function. This is the [deterministic-decision pattern](linkedin-article-kyc-deterministic.md), applied to payments.

---

## What this earns under FREE-AI

- **Rec 8 (Graded Liability)** — every payment rejection produces a Medium-grade incident; HITL holds are auditable.
- **Rec 16 (Autonomous Systems)** — declared `RiskHigh`; the orchestrator can only execute on messages with `advisor` or `admin` role on `metadata.user_roles`.
- **Rec 18 (Disclosure)** — every Instruction carries a Disclaimer ("Subject to PSP confirmation, account-balance check, and NPCI / RBI rail availability.")
- **Rec 22 (Annexure VI)** — every reject auto-generates the structured incident payload.

That's four of the 26 recommendations addressed by one 250-line agent. Stacking up.

---

## The thesis

Payment routing is the most boring piece of fintech software, and the easiest to get spectacularly wrong. The patterns that survive:

1. **Determinism over creativity.** Same input, same rail, every time.
2. **Idempotency is non-negotiable.** Refuse to process without a key.
3. **HITL at threshold.** Pick the number, hold the line, audit overrides.
4. **Separation of concerns.** The orchestrator routes; the PSP adapter moves money.
5. **The clock is injectable.** Or your tests for "RTGS at 10 PM" are flaky and you ignore them.

Each is a five-character decision. Together they're the difference between a payment system that survives an audit and one that doesn't.

---

## The repo

Genie is open source under MIT.

- `agents/payment_orchestrator/payment_orchestrator.go` — the worked example
- `docs/agents/payment_orchestrator.md` — the deep-dive doc
- Tests cover every rail × time-of-day × trust × amount combination

```bash
git clone https://github.com/c2siorg/genie.git
go test ./agents/payment_orchestrator/ -v
```

---

If you've shipped a payment orchestrator with a different shape — different rail preferences, different thresholds, async vs sync flow — I'd genuinely like to compare. The most interesting choices in this design are the ones I'd second-guess in a different context.

#FinTech #Payments #NPCI #UPI #RTGS #IMPS #NEFT #ResponsibleAI #RBI #FREEAI #BankingAI #FinTechIndia
