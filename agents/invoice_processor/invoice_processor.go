// Package invoice_processor handles B2B invoice intake for SME current
// accounts and TReDS. Where receipt_ocr targets consumer receipts, this
// agent targets the GST e-invoice shape with HSN line items, vendor master
// matching, and 3-way match (PO / GRN / Invoice).
//
// Inspired by Google ADK samples → invoice-processing. Tuned for the GSTIN
// format and IRP (Invoice Registration Portal) e-invoice constraints.
package invoice_processor

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "invoice_processor"
	Capability = "process_b2b_invoice"
	TypeIn     = "invoice_packet"
	TypeOut    = "invoice_decision"
	NextAgent  = "financial_supervisor"

	maxLineVarianceRupees = 100.0  // ±₹100 tolerance per line for 3-way match
	maxQtyVariancePct     = 0.02   // ±2 % GRN vs invoice qty
)

// LineItem is one row on the invoice.
type LineItem struct {
	HSN        string  `json:"hsn"`
	Desc       string  `json:"description"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price_rupees"`
	GSTPct     float64 `json:"gst_pct"`
	LineTotal  float64 `json:"line_total_rupees"` // qty × unit × (1 + gst/100)
}

// Invoice is the inbound packet.
type Invoice struct {
	IRN         string     `json:"irn"`            // 64-char IRP-issued hash
	SupplierGSTIN string   `json:"supplier_gstin"`
	BuyerGSTIN    string   `json:"buyer_gstin"`
	InvoiceNo   string     `json:"invoice_no"`
	InvoiceDate string     `json:"invoice_date"`   // YYYY-MM-DD
	Items       []LineItem `json:"items"`
	TotalRupees float64    `json:"total_rupees"`
	POReference string     `json:"po_reference"`
	GRNReference string    `json:"grn_reference"`
}

// Reference is what we matched against from the buyer's system of record.
type Reference struct {
	POTotalRupees  float64            `json:"po_total_rupees"`
	GRNQtyByHSN    map[string]float64 `json:"grn_qty_by_hsn"`
	VendorActive   bool               `json:"vendor_active"` // vendor master flag
}

// Request bundles both.
type Request struct {
	Invoice Invoice   `json:"invoice"`
	Ref     Reference `json:"reference"`
}

// Decision is the structured output.
type Decision struct {
	IRN          string   `json:"irn"`
	Action       string   `json:"action"` // "post" | "hold" | "reject"
	Confidence   float64  `json:"confidence_0_1"`
	Issues       []string `json:"issues"`
	PostingHints []string `json:"posting_hints"`
	Disclaimer   string   `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Invoice Processor" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	d := a.Process(req.Invoice, req.Ref)
	env.Logf("[invoice_processor] irn=%s action=%s issues=%d", d.IRN, d.Action, len(d.Issues))
	body, _ := json.Marshal(d)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Process runs the full pipeline. Pure function — easy to unit-test.
func (a *Agent) Process(inv Invoice, ref Reference) Decision {
	issues := []string{}
	confidence := 1.0

	// 1. GSTIN format check (15 chars: state + PAN + entity + check).
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

	return Decision{
		IRN:          inv.IRN,
		Action:       action,
		Confidence:   round2(confidence),
		Issues:       issues,
		PostingHints: hints,
		Disclaimer: "Deterministic 3-way match. IRN authenticity should be verified against the " +
			"IRP API before final posting.",
	}
}

// gstinValid checks a 15-char GSTIN structure: 2-digit state + 10-char PAN +
// 1-char entity number + 1-char Z + 1-char checksum. We don't validate the
// checksum (mod-36) here — that's a separate library call — but the shape is
// enough to catch most data-entry errors.
func gstinValid(g string) bool {
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

func isDigit(b byte) bool      { return b >= '0' && b <= '9' }
func isAlpha(b byte) bool      { return b >= 'A' && b <= 'Z' }
func isAlphaDigit(b byte) bool { return isAlpha(b) || isDigit(b) }
func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
