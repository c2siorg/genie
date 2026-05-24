package advance_tax_planner

import (
	"testing"
	"time"
)

func fixedDate(s string) func() time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return func() time.Time { return t }
}

func TestSchedule_FourInstalments(t *testing.T) {
	a := &Agent{Now: fixedDate("2026-05-15")}
	plan := a.Compute(Request{ProjectedAnnualTaxINR: 1_00_000})
	if len(plan.Schedule) != 4 {
		t.Fatalf("expected 4 instalments; got %d", len(plan.Schedule))
	}
	pcts := []int{15, 45, 75, 100}
	for i, p := range pcts {
		if plan.Schedule[i].CumulativePct != p {
			t.Errorf("instalment %d cum pct=%d want %d", i, plan.Schedule[i].CumulativePct, p)
		}
	}
}

func TestNextInstalmentIsJune15WhenInMay(t *testing.T) {
	a := &Agent{Now: fixedDate("2026-05-15")}
	plan := a.Compute(Request{ProjectedAnnualTaxINR: 1_00_000})
	if plan.NextInstalment == nil {
		t.Fatal("next instalment missing")
	}
	if plan.NextInstalment.DueDate != "2026-06-15" {
		t.Errorf("next due should be 2026-06-15; got %s", plan.NextInstalment.DueDate)
	}
	if plan.NextInstalment.CumulativeINR != 15_000 {
		t.Errorf("cum req should be 15k; got %.0f", plan.NextInstalment.CumulativeINR)
	}
}

func TestShortfallNoteFiresAfterDeadline(t *testing.T) {
	a := &Agent{Now: fixedDate("2026-07-01")}
	plan := a.Compute(Request{ProjectedAnnualTaxINR: 1_00_000, PaidSoFarINR: 0})
	first := plan.Schedule[0]
	if first.ShortfallNote == "" {
		t.Errorf("missed-deadline with 0 paid should set shortfall note; got %+v", first)
	}
}

func TestNoShortfallWhenFullyPaid(t *testing.T) {
	a := &Agent{Now: fixedDate("2026-07-01")}
	plan := a.Compute(Request{ProjectedAnnualTaxINR: 1_00_000, PaidSoFarINR: 50_000})
	if plan.Schedule[0].ShortfallNote != "" {
		t.Errorf("≥15k paid before 15-Jun should clear shortfall note")
	}
}

func TestNextInstalmentNilAfterMarchDeadline(t *testing.T) {
	a := &Agent{Now: fixedDate("2027-03-31")}
	plan := a.Compute(Request{ProjectedAnnualTaxINR: 1_00_000})
	if plan.NextInstalment != nil {
		t.Errorf("no upcoming after FY end; got %+v", plan.NextInstalment)
	}
}
