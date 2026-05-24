package invoice_processor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func cleanInvoice() Invoice {
	return Invoice{
		IRN:           "abc123",
		SupplierGSTIN: "27ABCDE1234F1Z5",
		BuyerGSTIN:    "29ABCDE1234F2Z6",
		InvoiceNo:     "INV-001",
		InvoiceDate:   "2026-05-01",
		POReference:   "PO-1",
		GRNReference:  "GRN-1",
		Items: []LineItem{
			{HSN: "8523", Quantity: 10, UnitPrice: 100, GSTPct: 18, LineTotal: 1180},
		},
		TotalRupees: 1180,
	}
}

func cleanRef() Reference {
	return Reference{
		POTotalRupees: 1180,
		GRNQtyByHSN:   map[string]float64{"8523": 10},
		VendorActive:  true,
	}
}

func TestHappyPathPosts(t *testing.T) {
	d := New().Process(cleanInvoice(), cleanRef())
	if d.Action != "post" {
		t.Errorf("clean invoice should post, got %s (issues=%v)", d.Action, d.Issues)
	}
}

func TestInvalidGSTINHolds(t *testing.T) {
	inv := cleanInvoice()
	inv.SupplierGSTIN = "BAD"
	d := New().Process(inv, cleanRef())
	if d.Action == "post" {
		t.Errorf("bad GSTIN should not auto-post")
	}
	found := false
	for _, i := range d.Issues {
		if strings.Contains(i, "Supplier GSTIN") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected supplier GSTIN issue, got %+v", d.Issues)
	}
}

func TestInactiveVendorRejects(t *testing.T) {
	ref := cleanRef()
	ref.VendorActive = false
	d := New().Process(cleanInvoice(), ref)
	if d.Action == "post" {
		t.Errorf("inactive vendor should not auto-post")
	}
}

func TestPOMismatchHolds(t *testing.T) {
	ref := cleanRef()
	ref.POTotalRupees = 999 // off by ₹181
	d := New().Process(cleanInvoice(), ref)
	if d.Action == "post" {
		t.Errorf("PO mismatch should not auto-post")
	}
}

func TestGRNQtyMismatch(t *testing.T) {
	ref := cleanRef()
	ref.GRNQtyByHSN = map[string]float64{"8523": 8} // 20% short
	d := New().Process(cleanInvoice(), ref)
	hit := false
	for _, i := range d.Issues {
		if strings.Contains(i, "GRN beyond tolerance") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected GRN qty mismatch reason, got %+v", d.Issues)
	}
}

func TestGSTINShapeValidator(t *testing.T) {
	if !gstinValid("27ABCDE1234F1Z5") {
		t.Errorf("valid GSTIN rejected")
	}
	if gstinValid("123") || gstinValid("XXABCDE1234F1Z5") || gstinValid("27ABCDE1234F1A5") {
		t.Errorf("invalid GSTIN accepted")
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{Invoice: cleanInvoice(), Ref: cleanRef()})
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	d := New().Process(cleanInvoice(), cleanRef())
	if d.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}
