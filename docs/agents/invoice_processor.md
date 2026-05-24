# agents/invoice_processor

> **Risk class:** Medium · **Capability:** `process_b2b_invoice` · **In:** `invoice_packet` · **Out:** `invoice_decision`
> **Inspired by:** Google ADK `invoice-processing`, tuned for the Indian GST e-invoice format.

---

## Overview

Handles B2B invoice intake for SME current accounts and TReDS. Where
`agents/receipt_ocr` targets consumer receipts, this agent targets the
**GST e-invoice** shape with HSN line items, vendor master matching, and
the classic AP-side **3-way match**: Purchase Order × Goods Received
Note × Invoice.

Most Indian banks' SME AP-automation is bespoke per integration partner
(Tally, Zoho Books, custom ERP). This agent isolates the *rule logic*
from the *connector logic* — the connector hands the agent a structured
`Invoice` and `Reference` packet, the agent returns a `post / hold /
reject` decision with attribution.

---

## Constants

```go
const (
    ID         = "invoice_processor"
    Capability = "process_b2b_invoice"
    TypeIn     = "invoice_packet"
    TypeOut    = "invoice_decision"
    NextAgent  = "financial_supervisor"

    maxLineVarianceRupees = 100.0  // ±₹100 per line tolerance
    maxQtyVariancePct     = 0.02   // ±2% GRN vs invoice qty tolerance
)
```

---

## Types

### LineItem, Invoice, Reference

```go
type LineItem struct {
    HSN        string
    Desc       string
    Quantity   float64
    UnitPrice  float64
    GSTPct     float64
    LineTotal  float64  // qty × unit × (1 + gst/100)
}

type Invoice struct {
    IRN           string  // 64-char IRP-issued hash
    SupplierGSTIN string  // 15 chars
    BuyerGSTIN    string  // 15 chars
    InvoiceNo     string
    InvoiceDate   string  // YYYY-MM-DD
    Items         []LineItem
    TotalRupees   float64
    POReference   string
    GRNReference  string
}

type Reference struct {
    POTotalRupees float64
    GRNQtyByHSN   map[string]float64
    VendorActive  bool
}
```

### Decision

```go
type Decision struct {
    IRN          string
    Action       string  // "post" | "hold" | "reject"
    Confidence   float64 // 0..1
    Issues       []string
    PostingHints []string
    Disclaimer   string
}
```

---

## Business rules

Confidence starts at 1.0 and is reduced by each failed check:

| Check | Penalty | Notes |
|---|---:|---|
| Supplier GSTIN format invalid | −0.30 | 15-char structure (2-digit state + 10-char PAN + 1-char entity + 1-char Z + 1-char check) |
| Buyer GSTIN format invalid | −0.30 | Same shape |
| Vendor not active in master | −0.25 | Master is host data |
| Invoice total deviates from PO beyond ±₹100 | −0.25 | The 3-way match |
| No PO reference matched | −0.15 | Should be on-system before invoice arrives |
| HSN not present on GRN | −0.10 per HSN | GRN line missing |
| HSN qty differs from GRN beyond ±2 % | −0.10 per HSN | Quantity mismatch |
| Line total mismatch (qty × unit × (1 + gst/100) vs LineTotal) | −0.05 per line | Arithmetic sanity |

Final action:

- `confidence ≥ 0.80` → `post` (auto)
- `0.50 ≤ confidence < 0.80` → `hold` (AP analyst review)
- `confidence < 0.50` → `reject` (return to vendor)

---

## GSTIN structural validation

The 15-char GSTIN format is well-defined and trivially checkable without
hitting the GST portal:

| Position | Content |
|---|---|
| 1–2 | State code (digits) |
| 3–7 | PAN entity letters (uppercase A–Z) |
| 8–11 | PAN sequence digits |
| 12 | PAN check letter (A–Z) |
| 13 | Entity number (alphanumeric) |
| 14 | Static `Z` |
| 15 | Checksum (alphanumeric) |

The agent validates shape only — the mod-36 checksum is a separate
library call (not done here to keep dependencies minimal). For
production confidence, the host can layer a live GST portal lookup.

---

## Example

### Request

```json
{
  "invoice": {
    "irn": "...64-char hash...",
    "supplier_gstin": "27ABCDE1234F1Z5",
    "buyer_gstin": "29ABCDE1234F2Z6",
    "invoice_no": "INV-001",
    "invoice_date": "2026-05-01",
    "items": [{"hsn":"8523","quantity":10,"unit_price_rupees":100,"gst_pct":18,"line_total_rupees":1180}],
    "total_rupees": 1180,
    "po_reference": "PO-1",
    "grn_reference": "GRN-1"
  },
  "reference": {
    "po_total_rupees": 1180,
    "grn_qty_by_hsn": {"8523": 10},
    "vendor_active": true
  }
}
```

### Decision

```json
{
  "irn": "...",
  "action": "post",
  "confidence_0_1": 1.0,
  "issues": [],
  "posting_hints": ["Auto-post to AP ledger; subject to standard reconciliation."],
  "disclaimer": "Deterministic 3-way match. IRN authenticity should be verified against the IRP API before final posting."
}
```

---

## FREE-AI alignment

- **Rec 18 (Disclosure)** — Disclaimer flags that IRN must be verified against the IRP API.
- **Rec 22 (Annexure VI)** — `reject` decisions could be wired to auto-record if combined with a vendor-fraud signal (not done by default).

---

## Integration

### Triggered by

- A Tally / Zoho / ERP webhook posts the invoice → host service packs the `Reference` from its master data → publishes `invoice_packet`.

### Hands off to

- `financial_supervisor` → posts the final decision and routes to the AP team.
- AP ledger update (host concern).

### Does NOT do

- **OCR of paper invoices** — that's `agents/receipt_ocr` (consumer) or a separate B2B OCR agent.
- **IRP authenticity check** — host concern; the Disclaimer reminds the operator.
- **Vendor onboarding** — host concern.

---

## Anti-patterns

1. **Loose tolerances.** ±₹100 is intentional. Raising it lets fraud through.
2. **Skipping the GRN match for "trusted vendors".** GRN is the goods-receipt evidence; skipping it makes invoice-only fraud easy.
3. **Treating `hold` as a slow `post`.** Hold means an analyst eyeballed the issues; auto-resolving them defeats the gate.
4. **Posting before verifying the IRN against the IRP.** The Disclaimer is mandatory wording in the audit log.

---

## Testing

`agents/invoice_processor/invoice_processor_test.go` covers:

- Happy-path posts
- Bad GSTIN holds
- Inactive vendor rejects
- PO mismatch holds
- GRN qty mismatch reason fires
- GSTIN shape validator unit-tested in isolation
- HandleMessage dispatch
- Disclaimer presence

Run:

```bash
go test ./agents/invoice_processor/ -v
```

---

## References

- [GST e-invoice IRP](https://einvoice1.gst.gov.in/) — IRN, QR code, schema
- [HSN code lookup](https://services.gst.gov.in/services/searchhsnsac) — for the line-item taxonomy
- [TReDS RXIL](https://www.rxil.in/) — the discounting venue these invoices feed
- [RBI Master Direction — KYC + KYV (Know Your Vendor)](https://rbi.org.in/) — for vendor master discipline
