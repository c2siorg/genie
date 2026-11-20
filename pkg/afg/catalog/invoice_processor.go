package catalog

import (
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// InvoiceProcessorSpec ports agents/invoice_processor.
//
// It performs a deterministic B2B GST e-invoice 3-way match (PO / GRN /
// invoice) for SME current accounts and TReDS: GSTIN shape validation, vendor
// master check, PO total tolerance, GRN quantity match per HSN, and line-total
// sanity — producing a post/hold/reject decision with a confidence score.
func InvoiceProcessorSpec() afg.Spec {
	return afg.Spec{
		ID:   "invoice_processor",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				maxLineVarianceRupees = 100.0 // ±₹100 tolerance per line for 3-way match
				maxQtyVariancePct     = 0.02  // ±2 % GRN vs invoice qty
			)

			type LineItem struct {
				HSN       string  `json:"hsn"`
				Desc      string  `json:"description"`
				Quantity  float64 `json:"quantity"`
				UnitPrice float64 `json:"unit_price_rupees"`
				GSTPct    float64 `json:"gst_pct"`
				LineTotal float64 `json:"line_total_rupees"`
			}
			type Invoice struct {
				IRN           string     `json:"irn"`
				SupplierGSTIN string     `json:"supplier_gstin"`
				BuyerGSTIN    string     `json:"buyer_gstin"`
				InvoiceNo     string     `json:"invoice_no"`
				InvoiceDate   string     `json:"invoice_date"`
				Items         []LineItem `json:"items"`
				TotalRupees   float64    `json:"total_rupees"`
				POReference   string     `json:"po_reference"`
				GRNReference  string     `json:"grn_reference"`
			}
			type Reference struct {
				POTotalRupees float64            `json:"po_total_rupees"`
				GRNQtyByHSN   map[string]float64 `json:"grn_qty_by_hsn"`
				VendorActive  bool               `json:"vendor_active"`
			}
			type Request struct {
				Invoice Invoice   `json:"invoice"`
				Ref     Reference `json:"reference"`
			}
			type Decision struct {
				IRN          string   `json:"irn"`
				Action       string   `json:"action"`
				Confidence   float64  `json:"confidence_0_1"`
				Issues       []string `json:"issues"`
				PostingHints []string `json:"posting_hints"`
				Disclaimer   string   `json:"disclaimer"`
			}

			absF := func(x float64) float64 {
				if x < 0 {
					return -x
				}
				return x
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			isDigit := func(b byte) bool { return b >= '0' && b <= '9' }
			isAlpha := func(b byte) bool { return b >= 'A' && b <= 'Z' }
			isAlphaDigit := func(b byte) bool { return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') }

			// gstinValid checks a 15-char GSTIN structure: 2-digit state + 10-char
			// PAN + 1-char entity number + 1-char Z + 1-char checksum. The mod-36
			// checksum is not validated here — the shape catches most data-entry errors.
			gstinValid := func(g string) bool {
				g = strings.ToUpper(strings.TrimSpace(g))
				if len(g) != 15 {
					return false
				}
				if !isDigit(g[0]) || !isDigit(g[1]) {
					return false
				}
				for i := 2; i < 7; i++ {
					if !isAlpha(g[i]) {
						return false
					}
				}
				for i := 7; i < 11; i++ {
					if !isDigit(g[i]) {
						return false
					}
				}
				if !isAlpha(g[11]) {
					return false
				}
				if !isAlphaDigit(g[12]) {
					return false
				}
				if g[13] != 'Z' {
					return false
				}
				return isAlphaDigit(g[14])
			}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			inv := req.Invoice
			ref := req.Ref

			issues := []string{}
			confidence := 1.0

			// 1. GSTIN format check.
			if !gstinValid(inv.SupplierGSTIN) {
				issues = append(issues, "Supplier GSTIN format invalid")
				confidence -= 0.30
			}
			if !gstinValid(inv.BuyerGSTIN) {
				issues = append(issues, "Buyer GSTIN format invalid")
				confidence -= 0.30
			}

			// 2. Vendor master active.
			if !ref.VendorActive {
				issues = append(issues, "Vendor not active in master")
				confidence -= 0.25
			}

			// 3. PO total tolerance.
			if ref.POTotalRupees > 0 {
				variance := inv.TotalRupees - ref.POTotalRupees
				if absF(variance) > maxLineVarianceRupees {
					issues = append(issues, "Invoice total deviates from PO beyond tolerance")
					confidence -= 0.25
				}
			} else {
				issues = append(issues, "No PO reference matched")
				confidence -= 0.15
			}

			// 4. GRN quantity match per HSN.
			if len(ref.GRNQtyByHSN) > 0 {
				for _, item := range inv.Items {
					grnQty, ok := ref.GRNQtyByHSN[item.HSN]
					if !ok {
						issues = append(issues, "HSN "+item.HSN+" not present on GRN")
						confidence -= 0.10
						continue
					}
					if grnQty == 0 {
						continue
					}
					deviation := absF(item.Quantity-grnQty) / grnQty
					if deviation > maxQtyVariancePct {
						issues = append(issues, "HSN "+item.HSN+" quantity differs from GRN beyond tolerance")
						confidence -= 0.10
					}
				}
			}

			// 5. Line totals sanity (qty × unit × (1+gst/100)).
			for _, item := range inv.Items {
				expected := item.Quantity * item.UnitPrice * (1 + item.GSTPct/100)
				if absF(expected-item.LineTotal) > maxLineVarianceRupees {
					issues = append(issues, "Line total mismatch for HSN "+item.HSN)
					confidence -= 0.05
				}
			}

			if confidence < 0 {
				confidence = 0
			}

			action := "post"
			hints := []string{"Auto-post to AP ledger; subject to standard reconciliation."}
			switch {
			case confidence < 0.50:
				action = "reject"
				hints = []string{"Return to vendor with the listed issues; do not post."}
			case confidence < 0.80:
				action = "hold"
				hints = []string{"Route to AP analyst for manual review."}
			}

			d := Decision{
				IRN:          inv.IRN,
				Action:       action,
				Confidence:   round2(confidence),
				Issues:       issues,
				PostingHints: hints,
				Disclaimer: "Deterministic 3-way match. IRN authenticity should be verified against the " +
					"IRP API before final posting.",
			}
			body, err := json.Marshal(d)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
