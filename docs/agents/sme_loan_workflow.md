# agents/sme_loan_workflow

> **Risk class:** High · **Capability:** `orchestrate_sme_loan` · **In:** `sme_loan_request` · **Out:** `sme_loan_offer`
> **Inspired by:** Google ADK `small-business-loan-agent`, tuned for the Indian SME stack.

---

## Overview

End-to-end SME lending journey from application to in-principle sanction,
using `pkg/workflow` as the DAG runtime with a Human-in-the-Loop (HITL)
approval gate at disbursal. Stitches together primitives Genie already
has — `cashflow_underwriter`, `working_capital`, `invoice_discounter` —
into a single regulator-friendly orchestration.

Indian SME context: the **CGTMSE** (Credit Guarantee Fund for Micro &
Small Enterprises) provides collateral-free guarantees up to ₹5cr to
UDYAM-registered MSMEs in covered sectors. The orchestrator knows the
scheme rules and surfaces eligibility automatically.

---

## Constants

```go
const (
    ID         = "sme_loan_workflow"
    Capability = "orchestrate_sme_loan"
    TypeIn     = "sme_loan_request"
    TypeOut    = "sme_loan_offer"
    NextAgent  = "financial_supervisor"

    cgtmseMaxTicketRupees = 50_000_000 // ₹5cr revised ceiling (2023)
    maxLoanMultipleOfRev  = 0.30       // offer capped at 30% of annual turnover
)
```

---

## The DAG

```
gst_fetch
   ↓
cashflow_analysis
   ↓
cgtmse_eligibility
   ↓
indicative_offer
   ↓
human_approval        ← HITL gate (RequireApproval = true)
   ↓
sanction_letter
```

Each step writes to a shared `workflow.State` map. The event-sourced
`workflow.InMemorySink` records every transition (`started`,
`completed`, `failed`, `awaiting_approval`, `approved`,
`compensated`). The state + the event log together are the audit
artefact — replay them and you reconstruct exactly what happened.

---

## Types

### Application (inbound)

```go
type Application struct {
    BorrowerID         string
    UDYAMRegistered    bool
    Sector             string  // "manufacturing" | "services" | "trading"
    AnnualTurnover     float64 // ₹
    GSTFilingRegular   bool    // last 12 returns on time
    RequestedAmount    float64
    RequestedTenorMths int
    CashflowScore0to1  float64 // from cashflow_underwriter
    CollateralRupees   float64 // 0 if collateral-free
}
```

### Offer (outbound)

```go
type Offer struct {
    BorrowerID        string
    Decision          string  // "approved" | "in_principle" | "rejected"
    OfferedAmount     float64
    OfferedTenorMths  int
    IndicativeRatePct float64
    MonthlyEMIRupees  float64
    CGTMSEEligible    bool
    Rationale         []string
    WorkflowEvents    int     // for audit
    Disclaimer        string
}
```

---

## Business rules

- **Floor on cashflow**: score < 0.30 → reject. Below this nothing else matters.
- **Turnover cap**: offer ≤ 30 % of annual turnover. Anything higher signals over-leverage.
- **CGTMSE eligibility**: UDYAM-registered AND ticket ≤ ₹5cr AND sector in {manufacturing, services, trading}.
- **Rate formula** (indicative — production calls a pricing engine):
  - base = 10.5 %
  - + (1 − cashflow_score) × 5 % — weaker file pays more
  - − 0.5 % if CGTMSE eligible (the bank's risk is lower)
- **EMI** computed on reducing-balance basis.

### Decision matrix

| State | Decision |
|---|---|
| cashflow_pass = false | `rejected` |
| cashflow_pass = true, human_approved = false | `in_principle` |
| cashflow_pass = true, human_approved = true | `approved` |

---

## HITL gate

The `human_approval` step has `RequireApproval: true` on its
`workflow.Step`. The workflow blocks here until an external caller
invokes `workflow.ApproveStep("human_approval")`.

The agent has two modes:

- **Synchronous (`HandleMessage`)** — passes `autoApprove=true` so the
  bus-driven path completes deterministically. Useful for testing,
  sandboxing, and the in-process CLI demo.
- **Asynchronous (production)** — caller invokes `Process(ctx, app,
  false)` and a separate HTTP endpoint calls `ApproveStep` when the
  Relationship Manager clicks "approve" in the back-office UI.

To prevent infinite hangs when no approval is forthcoming, `Process`
wraps the run in a derived cancellable context and bails after a short
polling window if `autoApprove=false`.

---

## Example

### Request

```json
{
  "borrower_id": "sme-7891",
  "udyam_registered": true,
  "sector": "manufacturing",
  "annual_turnover_rupees": 20000000,
  "gst_filing_regular": true,
  "requested_amount_rupees": 3000000,
  "requested_tenor_months": 36,
  "cashflow_score_0_1": 0.75
}
```

### Offer (auto-approved)

```json
{
  "borrower_id": "sme-7891",
  "decision": "approved",
  "offered_amount_rupees": 3000000,
  "offered_tenor_months": 36,
  "indicative_rate_pct": 11.25,
  "monthly_emi_rupees": 98345.67,
  "cgtmse_eligible": true,
  "rationale": [
    "All checks cleared; sanction letter drafted",
    "CGTMSE eligible — collateral-free guarantee applicable"
  ],
  "workflow_event_count": 13,
  "disclaimer": "Indicative SME loan offer. Final sanction subject to credit committee, complete documentation, and CGTMSE registration where applicable."
}
```

---

## FREE-AI alignment

- **Rec 16 (Autonomous Systems)** — RiskHigh; HITL gate is mandatory in production paths.
- **Rec 18 (Disclosure)** — Disclaimer cites credit-committee dependency.
- **Rec 22 (Annexure VI)** — would auto-fire if a step throws a hard error.
- **Rec 24 (Audit Framework)** — the workflow event log + state snapshot is the audit artefact; an external auditor can replay any borrower's journey.

---

## Integration

### Triggered by

- `POST /v1/sme/apply` (host route) → packs Application JSON → publishes `sme_loan_request`.

### Hands off to

- `financial_supervisor` for the final report dispatch.
- `agents/reporter` for the sanction-letter language.
- A back-office HITL UI (host responsibility) that calls `ApproveStep`.

### Reuses

- `agents/cashflow_underwriter` — for the cashflow score (the agent expects this to be precomputed; in production the orchestrator could call it inline).
- `pkg/workflow` — DAG runtime with Saga compensation + event sink.

---

## Anti-patterns

1. **Skipping the HITL gate in production.** The DAG enforces it; don't refactor the step out.
2. **Approving via bus message metadata instead of `ApproveStep`.** The latter is the audited path.
3. **Auto-disbursing on `approved` without a credit-committee check.** The Disclaimer says "subject to credit committee" — honour it.
4. **Lowering `maxLoanMultipleOfRev`** without a policy update. It's there to prevent over-lending to thin SMEs.
5. **Storing the full Application JSON in WorkflowState forever.** It contains GST-derived turnover. Use the encrypted document store + a reference id.

---

## Testing

`agents/sme_loan_workflow/sme_loan_workflow_test.go` covers:

- Happy-path approval
- Cashflow-floor rejection
- Turnover-cap enforcement (offer = 30 % of turnover, not requested amount)
- CGTMSE ineligibility for non-covered sectors
- In-principle decision when no human approval is given
- Rate risk premium (weaker cashflow pays more)
- HandleMessage dispatch
- Disclaimer presence
- Rationale mentions CGTMSE when eligible

Run:

```bash
go test ./agents/sme_loan_workflow/ -v
```

---

## References

- [CGTMSE official site](https://www.cgtmse.in/) — scheme rules, ticket size, sector coverage
- [Mudra scheme](https://www.mudra.org.in/) — for sub-₹10L ticket sizes (a similar agent would specialise here)
- [TReDS RXIL / M1xchange / A.TREDS](https://www.rxil.in/) — for invoice discounting (`agents/invoice_discounter`)
- [UDYAM registration](https://udyamregistration.gov.in/) — the MSME definition
- [RBI Master Direction — Lending to MSMEs](https://rbi.org.in/) — the regulatory floor
