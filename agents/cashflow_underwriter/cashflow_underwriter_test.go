package cashflow_underwriter

import (
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

// stableEarner: steady ₹50k salary monthly, modest spend, 6 months history,
// no bounces, no recurring debt. Should land in A/B grade.
func TestUnderwrite_StableEarner_HighScore(t *testing.T) {
	a := New()
	var txns []finance.Transaction
	for m := 1; m <= 6; m++ {
		date := isoDate(2026, m, 1)
		txns = append(txns,
			finance.Transaction{Date: date, AmountCents: 50_000_00, Description: "salary", Category: "income"},
			finance.Transaction{Date: isoDate(2026, m, 3), AmountCents: -10_000_00, Description: "rent", Category: "rent"},
			finance.Transaction{Date: isoDate(2026, m, 5), AmountCents: -5_000_00, Description: "groceries", Category: "food"},
		)
	}
	res := a.Score(txns)
	if res.Score < 700 {
		t.Errorf("stable earner score=%.0f want ≥700; signals=%+v", res.Score, res.Signals)
	}
	if res.Grade != "A" && res.Grade != "B" {
		t.Errorf("stable earner grade=%q want A or B", res.Grade)
	}
}

// stressedBorrower: erratic inflows, high EMI burden, bounces. Should grade D/C.
func TestUnderwrite_StressedBorrower_LowScore(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		// volatile income
		{Date: "2026-01-01", AmountCents: 80_000_00, Description: "gig", Category: "income"},
		{Date: "2026-02-01", AmountCents: 10_000_00, Description: "gig", Category: "income"},
		{Date: "2026-03-01", AmountCents: 5_000_00, Description: "gig", Category: "income"},
		// EMIs eating most of the inflow
		{Date: "2026-01-05", AmountCents: -30_000_00, Description: "EMI personal loan", Category: "debt"},
		{Date: "2026-02-05", AmountCents: -30_000_00, Description: "EMI personal loan", Category: "debt"},
		{Date: "2026-03-05", AmountCents: -30_000_00, Description: "EMI personal loan", Category: "debt"},
		// bounces
		{Date: "2026-02-06", AmountCents: -5_000_00, Description: "ACH_RETURN reversal"},
		{Date: "2026-03-06", AmountCents: -5_000_00, Description: "ECS RET"},
	}
	res := a.Score(txns)
	if res.Score >= 700 {
		t.Errorf("stressed borrower score=%.0f want <700; signals=%+v", res.Score, res.Signals)
	}
	if res.Grade != "C" && res.Grade != "D" {
		t.Errorf("stressed grade=%q want C or D", res.Grade)
	}
	// Bounce signal must reflect it.
	var bounce float64
	for _, s := range res.Signals {
		if s.Name == "bounce_rate" {
			bounce = s.Raw
		}
	}
	if bounce == 0 {
		t.Error("expected non-zero bounce_rate raw")
	}
}

func TestUnderwrite_EmptyDataYieldsFloor(t *testing.T) {
	a := New()
	res := a.Score(nil)
	if res.Score != MinScore || res.Grade != "D" {
		t.Errorf("empty score=%.0f grade=%q want %.0f/D", res.Score, res.Grade, MinScore)
	}
}

func TestUnderwrite_DisclaimerAttached(t *testing.T) {
	a := New()
	res := a.Score([]finance.Transaction{{Date: "2026-01-01", AmountCents: 1000_00, Description: "test"}})
	if !strings.Contains(res.Disclaimer, "CIBIL") {
		t.Errorf("disclaimer should mention CIBIL distinction; got %q", res.Disclaimer)
	}
}

// helper
func isoDate(y, m, d int) string {
	mm := "0" + itoa(m)
	mm = mm[len(mm)-2:]
	dd := "0" + itoa(d)
	dd = dd[len(dd)-2:]
	return itoa(y) + "-" + mm + "-" + dd
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}
