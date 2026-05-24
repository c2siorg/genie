package deductions_optimizer

import "testing"

func TestFullHeadroomFresh30pctSlab(t *testing.T) {
	plan := New().Compute(Request{
		BorrowerSlabPct:  30,
		ParentsSeniorCit: true,
	})
	// 80C=150k + 80CCD1B=50k + 80D-self=25k + 80D-parents=50k + 80TTA=10k = 285k
	// 30% of 285k = 85.5k
	if plan.TotalSavingINR < 85_000 || plan.TotalSavingINR > 86_000 {
		t.Errorf("expected ~85.5k saving; got %.0f", plan.TotalSavingINR)
	}
	if len(plan.Suggestions) != 5 {
		t.Errorf("want 5 suggestions; got %d", len(plan.Suggestions))
	}
}

func TestNoSuggestionsWhenAllUsed(t *testing.T) {
	plan := New().Compute(Request{
		BorrowerSlabPct: 30,
		Used: Used{
			Sec80C: 1_50_000, Sec80CCD1B: 50_000,
			Sec80DSelf: 25_000, Sec80DParents: 50_000, Sec80TTA: 10_000,
		},
		ParentsSeniorCit: true,
	})
	if len(plan.Suggestions) != 0 {
		t.Errorf("fully-used ceilings should produce no suggestions; got %+v", plan.Suggestions)
	}
}

func TestRankedByTaxSaved(t *testing.T) {
	plan := New().Compute(Request{BorrowerSlabPct: 30, ParentsSeniorCit: true})
	for i := 1; i < len(plan.Suggestions); i++ {
		if plan.Suggestions[i-1].TaxSavedINR < plan.Suggestions[i].TaxSavedINR {
			t.Errorf("suggestions not sorted by tax saved descending: %+v", plan.Suggestions)
		}
	}
}

func TestParentsNonSenior25kCap(t *testing.T) {
	plan := New().Compute(Request{BorrowerSlabPct: 30, ParentsSeniorCit: false})
	for _, s := range plan.Suggestions {
		if s.Section == "80D-parents" && s.HeadroomINR != 25_000 {
			t.Errorf("non-senior parents cap should be 25k; got %.0f", s.HeadroomINR)
		}
	}
}

func TestRegimeNotePresent(t *testing.T) {
	plan := New().Compute(Request{BorrowerSlabPct: 30})
	if plan.RegimeNote == "" {
		t.Errorf("regime note missing")
	}
}
