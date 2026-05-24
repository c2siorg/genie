package mule

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestMule_PassThrough(t *testing.T) {
	a := New()
	// One large credit, followed by debits within 24h totalling ≥90% of it.
	txns := []finance.Transaction{
		{TransactionID: "c1", AccountID: "A1", Date: "2026-01-01", AmountCents: 1_000_00, Merchant: "alice"},
		{TransactionID: "d1", AccountID: "A1", Date: "2026-01-01", AmountCents: -400_00, Merchant: "bob"},
		{TransactionID: "d2", AccountID: "A1", Date: "2026-01-01", AmountCents: -500_00, Merchant: "carol"},
	}
	sigs := a.Detect(txns)
	if !hasPattern(sigs, "pass_through", "A1") {
		t.Fatalf("expected pass_through on A1, got %+v", sigs)
	}
}

func TestMule_NoPassThroughBelowFraction(t *testing.T) {
	a := New()
	// Only ~30% forwarded — not pass-through.
	txns := []finance.Transaction{
		{TransactionID: "c1", AccountID: "A1", Date: "2026-01-01", AmountCents: 1_000_00, Merchant: "alice"},
		{TransactionID: "d1", AccountID: "A1", Date: "2026-01-01", AmountCents: -300_00, Merchant: "bob"},
	}
	if hasPattern(a.Detect(txns), "pass_through", "A1") {
		t.Fatalf("did not expect pass_through with 30%% forwarding")
	}
}

func TestMule_FanInFanOut(t *testing.T) {
	a := New()
	txns := []finance.Transaction{}
	// 5 distinct senders → A1
	for _, m := range []string{"s1", "s2", "s3", "s4", "s5"} {
		txns = append(txns, finance.Transaction{
			AccountID: "A1", Date: "2026-01-01", AmountCents: 1000_00, Merchant: m,
		})
	}
	// A1 → 5 distinct receivers
	for _, m := range []string{"r1", "r2", "r3", "r4", "r5"} {
		txns = append(txns, finance.Transaction{
			AccountID: "A1", Date: "2026-01-02", AmountCents: -800_00, Merchant: m,
		})
	}
	if !hasPattern(a.Detect(txns), "fan_in_fan_out", "A1") {
		t.Fatalf("expected fan_in_fan_out on A1")
	}
}

func TestMule_NoFanInFanOutForRegularUser(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		// Salary in once, four distinct merchants out.
		{AccountID: "A1", Date: "2026-01-01", AmountCents: 50_000_00, Merchant: "employer"},
		{AccountID: "A1", Date: "2026-01-02", AmountCents: -350_00, Merchant: "swiggy"},
		{AccountID: "A1", Date: "2026-01-03", AmountCents: -250_00, Merchant: "uber"},
		{AccountID: "A1", Date: "2026-01-04", AmountCents: -1_500_00, Merchant: "amazon"},
		{AccountID: "A1", Date: "2026-01-05", AmountCents: -2_200_00, Merchant: "electricity"},
	}
	for _, s := range a.Detect(txns) {
		if s.Pattern == "fan_in_fan_out" {
			t.Fatalf("normal user incorrectly flagged: %+v", s)
		}
	}
}

func TestMule_PostCreditBurst(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{AccountID: "A1", Date: "2026-01-01", AmountCents: 100_000_00, Merchant: "unknown"},
	}
	// 10 debits same day (within 1h since all parse to midnight)
	for i := 0; i < 10; i++ {
		txns = append(txns, finance.Transaction{
			AccountID: "A1", Date: "2026-01-01", AmountCents: -5_000_00, Merchant: "merchant",
		})
	}
	if !hasPattern(a.Detect(txns), "post_credit_burst", "A1") {
		t.Fatalf("expected post_credit_burst on A1")
	}
}

func TestMule_NormalAccountClean(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{AccountID: "A1", Date: "2026-01-01", AmountCents: 50_000_00, Merchant: "employer"},
		{AccountID: "A1", Date: "2026-01-02", AmountCents: -350_00, Merchant: "swiggy"},
		{AccountID: "A1", Date: "2026-01-15", AmountCents: -15_000_00, Merchant: "rent"},
	}
	if got := a.Detect(txns); len(got) != 0 {
		t.Fatalf("normal account flagged: %+v", got)
	}
}

func hasPattern(signals []Signal, pattern, account string) bool {
	for _, s := range signals {
		if s.Pattern == pattern && s.AccountID == account {
			return true
		}
	}
	return false
}
