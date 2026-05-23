package tax_harvester

import (
	"math"
	"testing"
)

func TestHarvest_STCLOffsetsSTCG(t *testing.T) {
	a := New()
	req := Request{
		AsOfDate: "2026-03-15",
		Realised: RealisedGain{ShortTermRupees: 100_000}, // ₹100k STCG to offset
		Holdings: []Holding{
			{Symbol: "SHORT1", Quantity: 100, CostBasisRupee: 200_000,
				CurrentPrice: 1500, PurchaseDate: "2026-01-01"}, // 73 days → STCL
		},
	}
	plan := a.Compute(req)
	if len(plan.Opportunities) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(plan.Opportunities))
	}
	op := plan.Opportunities[0]
	if op.GainCategory != "STCL" {
		t.Errorf("category=%s want STCL", op.GainCategory)
	}
	if op.UnrealisedLossINR != 50_000 {
		t.Errorf("loss=%.0f want 50000", op.UnrealisedLossINR)
	}
	// 50k STCL offsets 50k of the 100k STCG → tax saved = 50k × 20% = 10k.
	if math.Abs(op.TaxSavedINR-10_000) > 1 {
		t.Errorf("tax saved=%.2f want ~10000", op.TaxSavedINR)
	}
}

func TestHarvest_LTCLOffsetsLTCGAboveExemption(t *testing.T) {
	a := New()
	req := Request{
		AsOfDate: "2026-03-15",
		Realised: RealisedGain{LongTermRupees: 2_25_000}, // ₹2.25L LTCG → ₹1L taxable
		Holdings: []Holding{
			{Symbol: "OLD1", Quantity: 100, CostBasisRupee: 300_000,
				CurrentPrice: 2000, PurchaseDate: "2023-01-01"}, // held ~3yr → LTCL
		},
	}
	plan := a.Compute(req)
	if len(plan.Opportunities) != 1 {
		t.Fatalf("want 1 opp, got %d", len(plan.Opportunities))
	}
	op := plan.Opportunities[0]
	if op.GainCategory != "LTCL" {
		t.Errorf("category=%s want LTCL", op.GainCategory)
	}
	// Loss=100k, taxable LTCG above exemption=100k → fully soaked.
	// Saved = 100k × 12.5% = 12.5k.
	if math.Abs(op.TaxSavedINR-12_500) > 1 {
		t.Errorf("tax saved=%.2f want 12500", op.TaxSavedINR)
	}
}

func TestHarvest_NoOpportunityWhenNoBookedGains(t *testing.T) {
	a := New()
	req := Request{
		AsOfDate: "2026-03-15",
		Realised: RealisedGain{}, // no booked gains
		Holdings: []Holding{
			{Symbol: "X", Quantity: 10, CostBasisRupee: 1_000,
				CurrentPrice: 50, PurchaseDate: "2026-01-01"},
		},
	}
	plan := a.Compute(req)
	// Loss exists but no gain to offset against → no opp.
	if len(plan.Opportunities) != 0 {
		t.Errorf("unexpected opps without booked gain: %+v", plan.Opportunities)
	}
}

func TestHarvest_RankedByLossSize(t *testing.T) {
	a := New()
	req := Request{
		AsOfDate: "2026-03-15",
		Realised: RealisedGain{ShortTermRupees: 200_000},
		Holdings: []Holding{
			{Symbol: "SMALL", Quantity: 100, CostBasisRupee: 50_000,
				CurrentPrice: 400, PurchaseDate: "2026-01-01"}, // loss=10k
			{Symbol: "BIG", Quantity: 100, CostBasisRupee: 200_000,
				CurrentPrice: 1_000, PurchaseDate: "2026-01-01"}, // loss=100k
		},
	}
	plan := a.Compute(req)
	if len(plan.Opportunities) != 2 {
		t.Fatalf("expected 2 opps, got %d", len(plan.Opportunities))
	}
	if plan.Opportunities[0].Symbol != "BIG" {
		t.Errorf("BIG should rank first; got %s", plan.Opportunities[0].Symbol)
	}
}

func TestHarvest_WashSaleWarningAttached(t *testing.T) {
	a := New()
	req := Request{
		AsOfDate: "2026-03-15",
		Realised: RealisedGain{ShortTermRupees: 100_000},
		Holdings: []Holding{
			{Symbol: "Y", Quantity: 10, CostBasisRupee: 10_000,
				CurrentPrice: 500, PurchaseDate: "2026-01-01"},
		},
	}
	plan := a.Compute(req)
	if len(plan.Opportunities) == 0 {
		t.Fatal("expected an opportunity")
	}
	if plan.Opportunities[0].WashSaleWarning == "" {
		t.Errorf("wash-sale warning should be attached")
	}
}
