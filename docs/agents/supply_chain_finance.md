# agents/supply_chain_finance

> **Risk class:** Medium · **Capability:** `supply_chain_finance` · **In:** `scf_request` · **Out:** `scf_recommendation`
> **Inspired by:** Google ADK `supply-chain`, tuned for Indian SME / TReDS.

---

## Overview

Layers on top of `agents/invoice_discounter` and `agents/working_capital`.
Where those work transaction-by-transaction, SCF needs a **view** of the
buyer-supplier chain over time: concentration risk, payment-cycle
stability, and timing of **TReDS** auctions.

TReDS (Trade Receivables Discounting System) is RBI's regulated venue
where MSMEs auction their receivables to financiers — RXIL, M1xchange,
and A.TREDS are the three operators. GST e-invoicing is a precondition
for TReDS uploads.

---

## Constants

```go
const (
    ID         = "supply_chain_finance"
    Capability = "supply_chain_finance"
    TypeIn     = "scf_request"
    TypeOut    = "scf_recommendation"
    NextAgent  = "financial_supervisor"

    concentrationWarnPct = 0.40 // top-buyer share above this → warn
    tredsThresholdDays   = 45   // aged > 45d → TReDS candidate (MSMED Act anchor)
)
```

---

## Types

### Invoice (inbound)

```go
type Invoice struct {
    InvoiceID       string
    BuyerID         string
    BuyerRating     string // AAA..D (CRISIL/ICRA)
    AmountRupees    float64
    DaysOutstanding int
    GSTeInvoiced    bool   // required for TReDS
}
```

### Request

```go
type Request struct {
    SupplierID string
    Invoices   []Invoice
}
```

### BuyerSlice (per-buyer aggregate)

```go
type BuyerSlice struct {
    BuyerID     string
    TotalRupees float64
    SharePct    float64
    WorstDays   int
    BuyerRating string
}
```

### Recommendation (outbound)

```go
type Recommendation struct {
    SupplierID       string
    TotalReceivables float64
    ConcentrationOK  bool
    WarnReasons      []string
    BuyerSlices      []BuyerSlice  // sorted by total desc
    TREDSCandidates  []Invoice     // sorted by age desc
    NextSteps        []string
    Disclaimer       string
}
```

---

## Business rules

1. Aggregate invoices by buyer → `BuyerSlices`.
2. Top buyer's share > 40 % → concentration warn.
3. Invoice is a TReDS candidate iff: `GSTeInvoiced == true` AND `DaysOutstanding > 45`.
4. Next steps:
   - If TReDS candidates exist: "Submit aged e-invoiced receivables to a TReDS platform for discounting."
   - If concentration warn: "Diversify buyer mix or insure top-buyer receivables to manage concentration risk."
   - Else: "Chain looks healthy; no immediate SCF action recommended."

---

## Example

### Request

```json
{
  "supplier_id": "sme-1",
  "invoices": [
    {"invoice_id": "i1", "buyer_id": "B1", "amount_rupees": 700000, "days_outstanding": 60, "gst_e_invoiced": true},
    {"invoice_id": "i2", "buyer_id": "B2", "amount_rupees": 300000, "days_outstanding": 30, "gst_e_invoiced": true}
  ]
}
```

### Recommendation

```json
{
  "supplier_id": "sme-1",
  "total_receivables_rupees": 1000000,
  "concentration_ok": false,
  "warn_reasons": ["Top buyer share above 40% — concentration risk"],
  "buyer_slices": [
    {"buyer_id": "B1", "total_rupees": 700000, "share_pct": 70.0, "worst_days_outstanding": 60, "buyer_rating": ""},
    {"buyer_id": "B2", "total_rupees": 300000, "share_pct": 30.0, "worst_days_outstanding": 30, "buyer_rating": ""}
  ],
  "treds_candidates": [
    {"invoice_id": "i1", "buyer_id": "B1", "amount_rupees": 700000, "days_outstanding": 60, "gst_e_invoiced": true}
  ],
  "next_steps": [
    "Submit aged e-invoiced receivables to a TReDS platform for discounting",
    "Diversify buyer mix or insure top-buyer receivables to manage concentration risk"
  ],
  "disclaimer": "Indicative supply-chain view. TReDS auction discount rates depend on buyer credit rating and the platform's live auction; final yield is set at clearing."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — Disclaimer flags that TReDS rates are set at auction clearing, not by the agent.

---

## Integration

### Triggered by

- An SME's accounts-receivable export → host packs `Invoice[]` → publishes `scf_request`.
- A scheduled "morning SCF roundup" job per SME customer.

### Hands off to

- `agents/invoice_discounter` — to compute indicative APR for each TReDS candidate.
- `agents/working_capital` — for the broader cycle view (DSO/DIO/DPO).

---

## Anti-patterns

1. **Treating concentration above 40 % as fatal.** It's a warn, not a block. Many viable SMEs run with a single big-name buyer (Tata, Reliance) — the right action is insurance, not refusal.
2. **TReDS uploads without GST e-invoice.** Platforms reject them. The agent filters correctly; don't override.
3. **Recommending TReDS for very-fresh receivables.** Discount rates eat the margin; only aged receivables clear the cost-benefit threshold.

---

## Testing

`agents/supply_chain_finance/supply_chain_finance_test.go` covers:

- Aggregation by buyer (top-N sort)
- Concentration warn at >40 %
- TReDS candidate filter (e-invoiced AND >45d)
- Healthy chain → no actions
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/supply_chain_finance/ -v
```

---

## References

- [TReDS framework — RBI](https://rbi.org.in/) — the regulatory umbrella
- [RXIL](https://www.rxil.in/), [M1xchange](https://www.m1xchange.com/), [A.TREDS](https://www.invoicemart.com/) — the three operators
- [MSMED Act 2006, §15](https://msme.gov.in/) — the 45-day MSME payment obligation
- [GST e-invoice IRP](https://einvoice1.gst.gov.in/) — the precondition for TReDS uploads
