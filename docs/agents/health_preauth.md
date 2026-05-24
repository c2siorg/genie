# agents/health_preauth

> **Risk class:** High · **Capability:** `health_preauth` · **In:** `preauth_request` · **Out:** `preauth_decision`
> **Inspired by:** Google ADK `medical-pre-authorization`, tuned for IRDAI cashless flows.

---

## Overview

Cashless-claim pre-authorisation. The workflow that, within minutes,
decides how much of an anticipated bill the insurer will cover — before
the patient is admitted.

The hardest part isn't computing the payout; it's modeling all the
**deduction families** that Indian health-insurance products use:

- **Waiting periods**: PED (pre-existing disease) and specific-condition
- **Sub-limits**: room rent and ICU rent per day
- **Room-rent proportionate deduction**: if the patient picks a room above the policy sub-limit, the *associated charges* (doctor fees, OT, lab) are scaled down — the classic "₹50 room → ₹2L bill cut by 40 %" surprise.
- **Procedure package caps**: many policies cap specific procedures (cataract, dialysis).
- **Co-pay**: percentage of the residual bill the patient bears.
- **Sum-insured cap**: overall ceiling.

Get any of these wrong and the patient gets a surprise bill at
discharge. The agent models them all explicitly.

---

## Constants

```go
const (
    ID         = "health_preauth"
    Capability = "health_preauth"
    TypeIn     = "preauth_request"
    TypeOut    = "preauth_decision"
    NextAgent  = "financial_supervisor"

    // IRDAI-mandated decision TAT for pre-auth is 1 hour.
    DecisionTATHours = 1
)
```

---

## Types

### Plan (insurer config)

```go
type Plan struct {
    ProductCode             string
    SumInsuredRupees        float64
    RoomRentSubLimitRupees  float64
    ICURentSubLimitRupees   float64
    CoPayPct                float64
    PEDWaitingMonths        int
    SpecificWaitingMonths   int
    ExcludedProcedures      []string
    ProcedurePackageRupees  map[string]float64 // procedure → max payout
}
```

### Request (from hospital)

```go
type Request struct {
    PreauthID            string
    PolicyCode           string
    Patient              string
    HospitalCode         string
    NetworkPPN           bool
    Procedure            string
    IsPED                bool
    IsSpecificWaiting    bool
    PolicyMonthsAtAdmit  int
    EstimatedBillRupees  float64
    RoomRentPerDayRupees float64
    ICURentPerDayRupees  float64
    LengthOfStayDays     int
    ICUDays              int
}
```

### Decision (outbound)

```go
type Decision struct {
    PreauthID        string
    Action           string   // "approve_full" | "approve_partial" | "approve_with_deduction" | "deny" | "hitl"
    ApprovedRupees   float64
    DeductionsRupees float64
    DeductionReasons []string
    Reasons          []string
    Disclaimer       string
}
```

---

## Business rules (in order)

1. **Network PPN gate**: non-network → deny (reimbursement path instead).
2. **Permanent exclusions**: procedure substring matches → deny.
3. **PED waiting period**: PED + `PolicyMonthsAtAdmit < PEDWaitingMonths` → deny.
4. **Specific-condition waiting**: same shape for hernia/cataract/etc.
5. **Room-rent proportionate deduction** (the famous gotcha):
   - `excessRoom = (RoomRent − SubLimit) × (LOS − ICUDays)`
   - `proportional = EstimatedBill × 0.60 × ((RoomRent − SubLimit) / RoomRent)` — the 0.60 is the industry rule of thumb for the share of "associated charges" that scale with room category.
   - both added to deductions.
6. **ICU sub-limit**: same pattern but per ICU day.
7. **Procedure package cap**: `gross > cap` → trim to cap.
8. **Co-pay**: applied to the residual.
9. **Sum-insured cap**: overall ceiling.

Final action:

- `approved == 0` → `deny`
- `approved < incurred` AND deductions reasons present → `approve_with_deduction`
- `approved < incurred` (no specific reasons) → `approve_partial`
- equal → `approve_full`
- `IncurredRupees ≥ ₹5L` → `hitl` (medical officer review) regardless

---

## Example

### Request

```json
{
  "request": {
    "preauth_id": "PA-1",
    "network_ppn": true,
    "procedure": "appendectomy",
    "policy_months_at_admit": 60,
    "estimated_bill_rupees": 100000,
    "room_rent_per_day_rupees": 10000,
    "length_of_stay_days": 3
  },
  "plan": {
    "product_code": "HC-001",
    "sum_insured_rupees": 500000,
    "room_rent_sublimit_rupees": 5000,
    "icu_rent_sublimit_rupees": 10000,
    "ped_waiting_months": 36,
    "specific_waiting_months": 24,
    "exclusions": ["cosmetic surgery"],
    "procedure_package_rupees": {"cataract": 30000}
  }
}
```

### Decision

The patient picked a ₹10,000 room (2× the ₹5,000 sub-limit) → proportionate
deduction fires on the 60 % associated-charges share. Action will be
`approve_with_deduction`, with a `DeductionReasons` entry citing
"proportionate deduction."

---

## FREE-AI alignment

- **Rec 8 (Graded Liability)** — large claims auto-route to HITL.
- **Rec 16 (Autonomous Systems)** — RiskHigh; only `advisor`/`admin` roles can trigger.
- **Rec 18 (Disclosure)** — Disclaimer cites IRDAI 1-hour TAT and "estimates may revise on final bill review."

---

## Integration

### Triggered by

- A network hospital's TPA system posting a preauth request → host service packs the `Plan` from the insurer config → publishes `preauth_request`.

### Hands off to

- `financial_supervisor` for routing.
- A medical officer's review UI (for `hitl` actions).
- `agents/reporter` for the customer-facing letter.

---

## Anti-patterns

1. **Skipping the proportionate-deduction logic.** This is the #1 source of customer complaints when discharge bills surprise patients.
2. **Treating `approve_partial` and `approve_with_deduction` the same.** The latter has *named* deductions the customer can challenge; the former is a sum-insured cap.
3. **Hard-coding the 0.60 associated-charges ratio.** It's an industry heuristic; the policy team may want to tune per product.
4. **Approving above ₹5L without HITL.** The threshold is policy hygiene; raising it requires medical-officer sign-off.

---

## Testing

`agents/health_preauth/health_preauth_test.go` covers:

- Non-network denial
- Exclusion denial
- PED waiting denial
- Room-rent proportionate deduction reason
- Procedure-package cap
- HITL on bill ≥ ₹5L
- Approve-full on clean small claim
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/health_preauth/ -v
```

---

## References

- [IRDAI Health Insurance Regulations 2016](https://www.irdai.gov.in/) — TAT, co-pay, room-rent rules
- [IRDAI cashless circular](https://www.irdai.gov.in/)
- [Insurance Bureau of India consumer-grievance reports](https://www.irdai.gov.in/) — for the most-common deduction surprises
