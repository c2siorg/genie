package mf_screener

import "testing"

func TestScreen_FilterByCategory(t *testing.T) {
	a := New()
	res := a.Screen(Request{
		Funds: []Fund{
			{Scheme: "Alpha", Category: "equity-large", AUMCr: 5000, FiveYrCAGR: 0.15, StdDev: 0.18},
			{Scheme: "DebtX", Category: "debt-corporate", AUMCr: 3000, FiveYrCAGR: 0.07, StdDev: 0.04},
		},
		Filter: Filter{Category: "equity-large"},
	})
	if len(res.Ranked) != 1 || res.Ranked[0].Scheme != "Alpha" {
		t.Errorf("expected only Alpha; got %+v", res.Ranked)
	}
	if res.FilteredOut != 1 {
		t.Errorf("expected 1 filtered out; got %d", res.FilteredOut)
	}
}

func TestScreen_HigherCAGRRanksHigher(t *testing.T) {
	a := New()
	res := a.Screen(Request{
		Funds: []Fund{
			{Scheme: "Mid", Category: "equity-large", AUMCr: 1000, FiveYrCAGR: 0.10, StdDev: 0.15, ExpenseRatio: 0.015},
			{Scheme: "Top", Category: "equity-large", AUMCr: 1000, FiveYrCAGR: 0.20, StdDev: 0.18, ExpenseRatio: 0.010},
		},
	})
	if res.Ranked[0].Scheme != "Top" {
		t.Errorf("Top should rank first; got %+v", res.Ranked)
	}
}

func TestScreen_LowExpenseBoostsScore(t *testing.T) {
	a := New()
	cheap := a.Screen(Request{Funds: []Fund{{Scheme: "X", Category: "x", FiveYrCAGR: 0.10, StdDev: 0.15, ExpenseRatio: 0.001}}}).Ranked[0].Score
	dear := a.Screen(Request{Funds: []Fund{{Scheme: "X", Category: "x", FiveYrCAGR: 0.10, StdDev: 0.15, ExpenseRatio: 0.020}}}).Ranked[0].Score
	if cheap <= dear {
		t.Errorf("cheaper fund should score higher; cheap=%.2f dear=%.2f", cheap, dear)
	}
}

func TestScreen_NoFundsEmpty(t *testing.T) {
	res := New().Screen(Request{})
	if len(res.Ranked) != 0 {
		t.Errorf("empty input should yield empty results")
	}
}

func TestScreen_AUMFilter(t *testing.T) {
	res := New().Screen(Request{
		Funds: []Fund{
			{Scheme: "Tiny", Category: "x", AUMCr: 50},
			{Scheme: "Big", Category: "x", AUMCr: 5000},
		},
		Filter: Filter{MinAUMCr: 1000},
	})
	if len(res.Ranked) != 1 || res.Ranked[0].Scheme != "Big" {
		t.Errorf("AUM filter failed; got %+v", res.Ranked)
	}
}
