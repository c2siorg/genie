package lcr_projector

import "testing"

func TestCompliantBalanceSheet(t *testing.T) {
	res := New().Compute(Request{
		HQLA: HQLA{Level1INR: 1000_00_00_000}, // ₹100cr cash + G-sec
		Outflows: Outflows{
			StableRetailINR:     500_00_00_000,
			LessStableRetailINR: 200_00_00_000,
		},
		Inflows: Inflows{ContractualRetailINR: 100_00_00_000},
	})
	if !res.Compliant {
		t.Errorf("bank with deep HQLA should be ≥100%% LCR; got %.0f%%", res.LCRPct)
	}
}

func TestNonCompliantBalanceSheet(t *testing.T) {
	res := New().Compute(Request{
		HQLA: HQLA{Level1INR: 10_00_00_000}, // ₹10cr only
		Outflows: Outflows{
			LessStableRetailINR:   200_00_00_000,
			UnsecuredWholesaleINR: 500_00_00_000,
		},
	})
	if res.Compliant {
		t.Errorf("thin HQLA + heavy wholesale should fail LCR; got %.0f%%", res.LCRPct)
	}
}

func TestLevel2HaircutsApplied(t *testing.T) {
	res := New().Compute(Request{
		HQLA:     HQLA{Level2AINR: 100_00_00_000},
		Outflows: Outflows{LessStableRetailINR: 1_00_00_000},
	})
	// L2A starts at 100cr; cap at 40% of total means TotalHQLA = L2A_post / 0.4.
	// L2A after 15% haircut = 85cr. Cap-adjusted (alone in HQLA): all becomes effective TotalHQLA.
	// Verify haircut applied — total cannot exceed face.
	if res.TotalHQLA >= 100_00_00_000 {
		t.Errorf("L2A haircut not applied; got %.0f", res.TotalHQLA)
	}
}

func TestInflowCappedAt75PctOfOutflow(t *testing.T) {
	res := New().Compute(Request{
		HQLA:     HQLA{Level1INR: 10_00_000},
		Outflows: Outflows{LessStableRetailINR: 100_00_000},                                    // run-off 10% = 10L
		Inflows:  Inflows{ContractualRetailINR: 10_00_000_00},                                    // huge — should be capped
	})
	// Capped inflow should be ≤0.75 × outflow.
	if res.TotalInflow > 0.751*res.TotalOutflow {
		t.Errorf("inflow cap violated; got %.0f > 0.75×%.0f", res.TotalInflow, res.TotalOutflow)
	}
}

func TestNoteAttached(t *testing.T) {
	res := New().Compute(Request{HQLA: HQLA{Level1INR: 1}, Outflows: Outflows{}})
	if res.Note == "" {
		t.Errorf("note missing")
	}
}
