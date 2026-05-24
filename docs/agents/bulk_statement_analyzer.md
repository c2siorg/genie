# agents/bulk_statement_analyzer

> **Risk class:** Medium · **Capability:** `consolidate_statements` · **In:** `bulk_statement_request` · **Out:** `consolidated_cashflow`
> **Inspired by:** Google ADK `high-volume-document-analyzer`, tuned for Account Aggregator multi-statement flows.

---

## Overview

Aggregates multiple bank statements — typically from an Account
Aggregator (AA) fetch covering several accounts — into a single
deduplicated cashflow view.

The canonical `ingestor` → `analyzer` pipeline handles **one** statement
at a time. For SME lending and AA-driven onboarding the applicant
submits **N** statements across **M** accounts; the downstream
`cashflow_underwriter`, `working_capital`, and `goal_planner` agents
need a single consolidated view, not N.

Critical to get right: **inter-account transfer dedup**. A user moving
₹50k from Account A to Account B shows up as a debit on A and a credit
on B — counting both inflates both sides of the cashflow.

---

## Constants

```go
const (
    ID         = "bulk_statement_analyzer"
    Capability = "consolidate_statements"
    TypeIn     = "bulk_statement_request"
    TypeOut    = "consolidated_cashflow"
    NextAgent  = "financial_supervisor"
)
```

---

## Types

### Txn (inbound, repeated)

```go
type Txn struct {
    AccountID   string
    Date        string  // YYYY-MM-DD
    Description string
    Amount      float64
    Type        string  // "credit" | "debit"
}
```

### Request

```go
type Request struct {
    ApplicantID  string
    Transactions []Txn // flat list across all accounts
}
```

### Summary (outbound)

```go
type Summary struct {
    ApplicantID    string
    AccountCount   int
    TxnCountRaw    int
    TxnCountDedup  int
    DurationMonths int
    TotalCredit    float64
    TotalDebit     float64
    NetCashflow    float64
    MonthlyAverage float64
    TopCategories  map[string]float64 // top-5 debit categories
    Disclaimer     string
}
```

---

## Dedup strategy

Two passes:

1. **Exact duplicates**: same `(amount, normalised_description, type)` —
   typical when a statement gets loaded twice. Keep the first occurrence,
   drop the rest.
2. **Inter-account transfers**: same `(amount, normalised_description)`
   with *opposing* types and dates within ±1 day. Drop **both** sides.

`normalised_description` is lowercased, trimmed, double-space collapsed,
truncated to 24 chars. This catches "Transfer to B" matching "transfer
to b" matching "Transfer to B   " etc.

The trade-off: this can over-dedup if the user makes two identical
transactions on the same day (e.g. two Swiggy orders for ₹450). The
order-tracking system would catch this; the dedup heuristic does not.
The `Disclaimer` calls this out.

---

## Categorisation

Heuristic substring matching for top-5 expense categorisation:

| Substring match | Category |
|---|---|
| `rent`, `lease` | `housing:rent` |
| `swiggy`, `zomato` | `food:delivery` |
| `uber`, `ola` | `transport` |
| `electric`, `water`, `gas` | `utilities` |
| `amazon`, `flipkart` | `shopping` |
| `netflix`, `spotify`, `prime` | `entertainment` |
| anything else | `other` |

For production: replace with `agents/enricher`'s richer categorisation
or a learnt classifier. The current shape is enough for the
`TopCategories` headline.

---

## Example

### Request

```json
{
  "applicant_id": "applicant-1",
  "transactions": [
    {"account_id": "A", "date": "2026-01-01", "description": "Transfer to B", "amount_rupees": 5000, "type": "debit"},
    {"account_id": "B", "date": "2026-01-01", "description": "Transfer to B", "amount_rupees": 5000, "type": "credit"},
    {"account_id": "A", "date": "2026-01-03", "description": "Salary", "amount_rupees": 50000, "type": "credit"}
  ]
}
```

### Summary

```json
{
  "applicant_id": "applicant-1",
  "account_count": 2,
  "txn_count_raw": 3,
  "txn_count_dedup": 1,
  "duration_months": 1,
  "total_credit_rupees": 50000,
  "total_debit_rupees": 0,
  "net_cashflow_rupees": 50000,
  "monthly_avg_inflow_rupees": 50000,
  "top_debit_categories": {},
  "disclaimer": "Consolidated cashflow across multiple statements with inter-account transfer dedup. Categorisation is heuristic; verify before underwriting."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — Disclaimer flags the dedup and the categorisation as heuristic.
- **Rec 24 (Audit Framework)** — `TxnCountRaw` vs `TxnCountDedup` is preserved so the auditor can see what was excluded and why.

---

## Integration

### Triggered by

- `agents/aa_fetcher` produces a list of `Txn` across the consented accounts → publishes `bulk_statement_request`.
- An SME loan workflow uploads CSV exports from multiple accounts → host packs them into one Request.

### Hands off to

- `agents/cashflow_underwriter` — consumes the Summary for SME scoring.
- `agents/working_capital` — uses `NetCashflow` and `MonthlyAverage` for the runway projection.
- `agents/goal_planner` — uses `MonthlyAverage` for projection.
- `agents/recommender` — uses `TopCategories` for spending advice.

---

## Anti-patterns

1. **Skipping dedup.** Double-counts inter-account transfers; SME lending decisions get badly inflated cashflow.
2. **Categorising on the dedup'd set only.** Categorisation should be on the dedup'd set (current behaviour). Doing it pre-dedup over-counts a transfer as expense.
3. **Trusting `MonthlyAverage` over a 1-month window.** 1 month is noisy. Production should require ≥3 months.
4. **Hard-coding the ±1-day window.** Some banks settle inter-account transfers same-day; some take a banking day. ±1 day is a sane default but the policy team may want to tune.

---

## Testing

`agents/bulk_statement_analyzer/bulk_statement_analyzer_test.go` covers:

- Inter-account transfer dedup
- Exact duplicate removal
- Account count
- Categorisation hits
- Duration in months
- Monthly average over a 3-month window
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/bulk_statement_analyzer/ -v
```

---

## References

- [Sahamati AA specs](https://sahamati.org.in/) — the source of multi-account fetches
- [RBI Master Direction — Account Aggregator](https://rbi.org.in/) — the regulatory framework
- [Genie agents/aa_fetcher](../../agents/aa_fetcher/) — upstream consumer
