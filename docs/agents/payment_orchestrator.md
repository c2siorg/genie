# agents/payment_orchestrator

> **Risk class:** High · **Capability:** `orchestrate_payment` · **In:** `payment_request` · **Out:** `payment_instruction`
> **Inspired by:** Google ADK `antom-payment`, Indianised to NPCI rails.

---

## Overview

Routes outbound payments across **UPI / IMPS / NEFT / RTGS** based on
amount, urgency, beneficiary status, and time-of-day, with HITL approval
at configurable thresholds.

This is the *bridge* to actual money movement. Genie's other agents can
analyse and recommend; this agent turns an approved recommendation into
a structured payment instruction. The host wires the PSP / NPCI
integration; the agent owns the *routing policy*.

**Important**: the agent emits an `Instruction`. It does **not** submit
to the rail. That separation keeps the policy logic unit-testable and
the PSP integration replaceable.

---

## Constants

```go
const (
    ID         = "payment_orchestrator"
    Capability = "orchestrate_payment"
    TypeIn     = "payment_request"
    TypeOut    = "payment_instruction"
    NextAgent  = "financial_supervisor"

    // NPCI per-rail limits (post-2023 revisions).
    upiPerTxnLimit   = 1_00_000.0  // ₹1L default
    impsLimit        = 5_00_000.0
    rtgsMinThreshold = 2_00_000.0  // ₹2L RTGS floor

    hitlThresholdRupees = 50_000.0 // HITL above this
)
```

---

## Constructor

```go
func New() *Agent
```

Clock is injectable for tests:

```go
&Agent{Clock: func() time.Time { return midday }}
```

---

## Types

### Request

```go
type Request struct {
    IdempotencyKey       string
    PayerID              string
    PayerAccount         string
    BeneficiaryName      string
    BeneficiaryVPA       string // for UPI
    BeneficiaryIFSC      string // for IMPS/NEFT/RTGS
    BeneficiaryAcct      string
    AmountRupees         float64
    Currency             string // INR only
    Purpose              string
    Urgency              string // "now" | "today" | "any"
    IsTrustedBeneficiary bool   // cooling-off cleared
}
```

### Instruction (outbound)

```go
type Instruction struct {
    IdempotencyKey  string
    Action          string  // "submit" | "hold_hitl" | "reject"
    Rail            string  // "upi" | "imps" | "neft" | "rtgs" | ""
    AmountRupees    float64
    Reasons         []string
    IncidentPayload string  // Annexure VI on reject
    Disclaimer      string
}
```

---

## Routing logic

### Hard rejects (before rail selection)

- Currency != INR → reject ("Only INR rails are supported")
- Amount ≤ 0 → reject ("Amount must be positive")
- Missing `IdempotencyKey` → reject ("refusing to risk a duplicate transfer")

### Untrusted-large hold

- `IsTrustedBeneficiary == false` AND `Amount ≥ ₹50k` → `hold_hitl` regardless of rail.

### Rail selection

In order of preference:

1. **UPI** — has VPA, amount ≤ ₹1L → `upi`.
2. **RTGS** — has IFSC+Acct, amount ≥ ₹2L, hour in [7,18), not Sunday → `rtgs`. (Preferred for large transfers when the window is open.)
3. **IMPS** — has IFSC+Acct, amount ≤ ₹5L, urgency not "any" → `imps`. (Fallback when RTGS is closed or amount below RTGS floor.)
4. **NEFT** — has IFSC+Acct → `neft`. (Catch-all 24×7 batch rail.)
5. None match → reject ("No rail satisfies the amount + urgency + IFSC/VPA constraints").

### HITL gate

After rail selection, if `Amount ≥ ₹50k`: action is `hold_hitl`. The
selected rail is still recorded so the human approver knows the
intended path.

---

## Why this ordering matters

- **UPI first for small amounts**: free, instant, the customer's
  default expectation.
- **RTGS before IMPS for large amounts in window**: RTGS settles
  individually with real-time finality; IMPS is also instant but settled
  in batches and has a ₹5L cap.
- **IMPS for medium amounts at night**: when RTGS is closed.
- **NEFT as the catch-all**: 24×7 since 2019, batch-settled, no real
  ceiling. Use when speed isn't critical.

---

## Example

### Request

```json
{
  "idempotency_key": "txn-2026-05-14-001",
  "payer_id": "user-123",
  "payer_account": "00112233",
  "beneficiary_name": "Alice Singh",
  "beneficiary_vpa": "alice@upi",
  "amount_rupees": 5000,
  "currency": "INR",
  "purpose": "Rent",
  "urgency": "now",
  "is_trusted_beneficiary": true
}
```

### Instruction

```json
{
  "idempotency_key": "txn-2026-05-14-001",
  "action": "submit",
  "rail": "upi",
  "amount_rupees": 5000,
  "reasons": ["Routed via upi", "Within UPI per-txn limit"],
  "disclaimer": "AI-generated payment instruction. Subject to PSP confirmation, account-balance check, and NPCI / RBI rail availability."
}
```

---

## FREE-AI alignment

- **Rec 8 (Graded Liability)** — every ≥₹50k payment is HITL-gated regardless of trust.
- **Rec 16 (Autonomous Systems)** — RiskHigh; `advisor`/`admin` role required.
- **Rec 18 (Disclosure)** — Instruction carries a Disclaimer.
- **Rec 22 (Annexure VI)** — every `reject` includes an `IncidentPayload` ready for regulator escalation.

---

## Integration

### Triggered by

- A "Pay" button in the customer app → host service packs `Request` → publishes `payment_request`.
- An approved recommendation from `agents/recommender` (rare; requires explicit user consent).

### Hands off to

- The PSP adapter (host) — picks up `Instruction` from the bus and submits to NPCI.
- HITL UI for `hold_hitl` actions.
- `pkg/incidents` for `reject` actions (auto-records the Annexure VI payload).

### Does NOT do

- **Account-balance check** — PSP/core-banking concern.
- **Beneficiary cooling-off enforcement** — host concern; the agent trusts `IsTrustedBeneficiary`.
- **Settlement confirmation** — PSP callback territory.

---

## Anti-patterns

1. **Skipping the idempotency key.** The agent rejects. Don't suppress that — duplicate transfers in production are very hard to reverse.
2. **Promoting an untrusted beneficiary to trusted mid-flight.** The cooling-off period exists to defend against social-engineering attacks. Override only via an explicit, audited admin action.
3. **Submitting the Instruction without re-reading the audit trail.** The downstream PSP submitter should re-verify the `Reasons` against current rail availability — UPI outages happen.
4. **Raising the ₹50k HITL threshold without policy approval.** This is the line between low-friction P2P and significant transfers.

---

## Testing

`agents/payment_orchestrator/payment_orchestrator_test.go` covers:

- Small trusted UPI → submit
- Large untrusted → hold_hitl
- RTGS in window + amount in range → rtgs (with hold_hitl due to >₹50k)
- RTGS closed at night → falls back to IMPS
- Non-urgent night transfer → NEFT (when urgency = "any")
- Non-INR currency → reject
- Zero amount → reject
- Missing idempotency key → reject
- No rail options → reject
- HandleMessage dispatch
- Disclaimer presence
- RiskHigh

Run:

```bash
go test ./agents/payment_orchestrator/ -v
```

---

## References

- [NPCI UPI](https://www.npci.org.in/what-we-do/upi/product-overview) — per-txn limits
- [NPCI IMPS](https://www.npci.org.in/what-we-do/imps/product-overview)
- [RBI NEFT & RTGS](https://rbi.org.in/scripts/FAQView.aspx?Id=60) — operating windows, settlement
- [RBI Master Direction — Digital Payments Security](https://rbi.org.in/) — for the cooling-off + HITL norms
