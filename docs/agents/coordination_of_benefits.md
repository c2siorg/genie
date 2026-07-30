# coordination_of_benefits

**Package:** `pkg/afg/catalog` (`CobSpec`) · **Risk:** medium · **Served by:** the governed
agent-framework catalog (`cmd/af-serve` / `catalog.Registry`).

## Overview

A deterministic **Coordination-of-Benefits (COB)** engine for a patient (or family)
insured under two overlapping health plans. It decides which plan pays first, applies the
secondary plan's **non-duplication** rule, honours per-plan deductible / out-of-pocket-max
accumulation across claims, and — in family mode — searches the primary ordering that
**minimises total out-of-pocket cost**. Standard US-style COB, applied in INR.

Every rupee is computed here in **integer paise** — never by a model (`the LLM reasons; the
ledger counts`). This is a genie-native reimplementation of the standard COB domain rules;
it is not a benefits determination — the insurer's adjudication and the policy terms govern.

## Types

### Request

```json
{
  "mode": "individual | family",
  "plans": {
    "A": {"deductible_paise": 5000000, "coinsurance_rate": 0.2, "oop_max_paise": 0},
    "B": {"deductible_paise": 3000000, "coinsurance_rate": 0.1}
  },
  "claims": [
    {"claim_id": "c1", "patient": "P", "billed_paise": 10000000,
     "own_plan_id": "A", "other_plan_id": "B"}
  ]
}
```

- `own_plan_id` — the plan where the patient is the member (primary in standard order).
- `other_plan_id` — the plan where the patient is a dependent (secondary in standard order).
- `oop_max_paise` — `0` (or omitted) means no cap.

### Response

Per-claim primary/secondary payments + patient OOP, the total, the standard-order total,
the savings from the optimal ordering, a recommendation, and a disclaimer.

## Business rules

- **Adjudication (as primary):** patient pays the remaining deductible, then
  `coinsurance_rate` of the remainder (half-to-even rounding), capped at the remaining
  out-of-pocket max; the plan pays the rest.
- **Secondary (non-duplication):** the secondary pays
  `max(0, min(secondary_as_primary − primary_payment, patient_residual))`, so the two plans
  together never pay more than the billed amount.
- **Deductible accumulation:** per `(plan, scope)`, where scope is the patient (individual)
  or the family (family mode → one shared deductible rolls over across claims on a plan).
- **Optimisation:** for ≤ 16 claims, all primary orderings are enumerated and the
  minimum-OOP combination is chosen; beyond that the standard order is reported.

**Invariants (tested):** `0 ≤ patient_oop ≤ billed`, `primary + secondary payout ≤ billed`,
`primary + secondary + patient_oop == billed`.

## FREE-AI alignment

- **Rec 18 (Disclosure):** every response carries a plain-language disclaimer.
- **Deterministic money:** arithmetic is unit-tested integer paise, not model output.

## Integration

Invoke through the governed catalog edge (`cmd/af-serve`): `POST /v1/ask` with
`agent="coordination_of_benefits"` and the request JSON above. The governance gate runs
before the engine (single construction door).

## Anti-patterns

- Do **not** let a model compute any payment — the engine is the only source of rupee math.
- Do **not** treat the output as a claim decision; it is advisory pre-adjudication.
