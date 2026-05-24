# agents/claim_adjudicator

> **Risk class:** High · **Capability:** `adjudicate_claim` · **In:** `claim_request` · **Out:** `claim_decision`
> **Inspired by:** Google ADK `claim-adjudication-agent`, tuned for Indian bancassurance flows.

---

## Overview

Adjudicates a bancassurance insurance claim against an insurer-supplied
policy rulebook. Every major Indian bank is now a corporate agent for
multiple insurers; bancassurance claims flow through the bank's app but
the adjudication logic is bespoke per insurer (exclusions, sub-limits,
waiting periods, network-only perils).

The agent's contract: insurer rules are *data* (a `Policy` struct),
agent code is the *rule engine* (a pure `Adjudicate` function). Adding
a new insurer is a config change, not a code change.

---

## Constants

```go
const (
    ID         = "claim_adjudicator"
    Capability = "adjudicate_claim"
    TypeIn     = "claim_request"
    TypeOut    = "claim_decision"
    NextAgent  = "financial_supervisor"

    hitlThresholdRupees = 200_000.0 // ≥₹2L → manual claims-officer review
)
```

---

## Types

### Policy (insurer config)

```go
type Policy struct {
    ProductCode         string
    WaitingPeriodDays   int                // claim incurred before this → deny
    SumInsured          float64
    DeductibleRupees    float64
    CoPayPct            float64            // 0..1
    Exclusions          []string           // lowercased keywords matched against Diagnosis
    SubLimits           map[string]float64 // peril -> max payout
    NetworkOnlyPerils   []string           // e.g. "cashless"
}
```

### Claim (inbound)

```go
type Claim struct {
    ClaimID           string
    PolicyCode        string
    IncurredRupees    float64
    Peril             string  // "hospitalization", "theft", "third-party"
    Diagnosis         string
    DaysSinceIssue    int
    HospitalInNetwork bool
}
```

### Decision (outbound)

```go
type Decision struct {
    ClaimID      string
    Action       string   // "approve" | "approve_partial" | "deny" | "hitl"
    PayoutRupees float64
    Reasons      []string
    Disclaimer   string
}
```

---

## Business rules (in order)

| # | Check | Result |
|---|---|---|
| 1 | Within waiting period | Deny |
| 2 | Diagnosis matches an exclusion keyword | Deny |
| 3 | Network-only peril at non-network hospital | Deny |
| 4 | Subtract deductible from `IncurredRupees` (floor 0) | Reduces payout |
| 5 | Apply co-pay percentage | Reduces payout |
| 6 | Cap by per-peril `SubLimits[peril]` if set | Reduces payout |
| 7 | Cap by `SumInsured` | Reduces payout |
| 8 | If `IncurredRupees ≥ ₹2L` → action = `hitl` | Human review |

`approve` vs `approve_partial`: if any deduction fired, action is
`approve_partial`. The Reasons array lists every deduction with its
trigger so a customer-service agent can explain the gap.

---

## Decision logic

`Adjudicate(c Claim, p Policy) Decision` — pure function. No I/O, no
LLM. Insurer rule changes are YAML edits; behaviour changes are policy
team's call, not engineering's.

---

## Example

### Request

```json
{
  "claim": {
    "claim_id": "CL-9001",
    "policy_code": "HOSP-001",
    "incurred_rupees": 50000,
    "peril": "hospitalization",
    "diagnosis": "viral fever",
    "days_since_issue": 90,
    "hospital_in_network": true
  },
  "policy": {
    "product_code": "HOSP-001",
    "waiting_period_days": 30,
    "sum_insured": 500000,
    "deductible_rupees": 5000,
    "copay_pct": 0.10,
    "exclusions": ["cosmetic", "self-inflicted"],
    "sub_limits": {"hospitalization": 300000},
    "network_only_perils": ["cashless"]
  }
}
```

### Decision

```json
{
  "claim_id": "CL-9001",
  "action": "approve_partial",
  "payout_rupees": 40500,
  "reasons": ["Co-pay applied at 10%"],
  "disclaimer": "Adjudication is rule-based per insurer policy. Final settlement subject to documentation, investigation, and TAT prescribed by IRDAI Health Insurance Regulations 2016."
}
```

Computation: 50,000 − 5,000 deductible = 45,000; 45,000 × (1 − 0.10) =
40,500; under sub-limit (300,000) and sum-insured (500,000); below HITL
threshold (200,000) → `approve_partial`.

---

## FREE-AI alignment

- **Rec 6 (Adaptive Policies)** — every Policy is YAML/JSON config, no code change to onboard a new insurer.
- **Rec 16 (Autonomous Systems)** — RiskHigh, HITL gate at ₹2L.
- **Rec 18 (Disclosure)** — every Decision includes a Disclaimer citing IRDAI Health Insurance Regulations 2016.
- **Rec 22 (Annexure VI)** — deny-on-exclusion or deny-on-fraud-indicator could be wired to auto-record (not done by default — most denials are not "incidents" in the regulatory sense).

---

## Integration

### Triggered by

- `POST /v1/claims/preauth` — the customer-facing endpoint.
- A mobile app's "submit claim" flow uploading docs → orchestrator gets the parsed Claim + the looked-up Policy.

### Hands off to

- `financial_supervisor` for the audit and downstream actions (notify customer, push to insurer TPA, schedule disbursal).
- `pkg/incidents` for HITL routes (the human reviewer needs an audit-logged decision trail).

### Does NOT do

- **Document OCR or claim-form parsing** — host concern.
- **Fraud-network detection** — that's `agents/fraud` and `agents/mule`.
- **Settlement** — that's a downstream payment system.

---

## Anti-patterns

1. **Hard-coding insurer rules in Go.** Defeats the design. Use the Policy YAML.
2. **Approving above ₹2L without HITL.** The threshold is regulatory hygiene; raising it requires risk-team sign-off.
3. **Ignoring `network_only_perils`.** Cashless and reimbursement flows have different documentation requirements; the orchestrator needs the gate.
4. **Treating `approve_partial` as a soft success.** The customer sees a gap. Surface the Reasons array prominently.

---

## Testing

`agents/claim_adjudicator/claim_adjudicator_test.go` covers:

- Waiting-period denial
- Exclusion-keyword denial
- Partial approval with deductible + co-pay
- Sub-limit cap
- HITL on large claim
- Network-only denial
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/claim_adjudicator/ -v
```

---

## References

- [IRDAI Health Insurance Regulations 2016](https://www.irdai.gov.in/) — the TAT and co-pay rules
- [IRDAI cashless-claims circular](https://www.irdai.gov.in/) — for PPN (Preferred Provider Network) hospital rules
- [RBI Master Direction on Bancassurance](https://rbi.org.in/) — for the corporate-agent framework
