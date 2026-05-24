package emergency_fund

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestEmergencyFund_StableProfile3Months(t *testing.T) {
	a := New()
	plan := a.Compute(Request{
		Transactions: mkMonthlyExpense(40_000, 6),
		LiquidReservesINR: 0,
		IncomeProfile:     "stable",
	})
	if plan.CoverageMonths != 3 {
		t.Errorf("stable→3 months; got %d", plan.CoverageMonths)
	}
	if plan.TargetINR != 1_20_000 {
		t.Errorf("target should be 3×40k=1.2L; got %.0f", plan.TargetINR)
	}
}

func TestEmergencyFund_VariableProfile6Months(t *testing.T) {
	plan := New().Compute(Request{
		Transactions:  mkMonthlyExpense(50_000, 4),
		IncomeProfile: "variable",
	})
	if plan.CoverageMonths != 6 {
		t.Errorf("variable→6 months; got %d", plan.CoverageMonths)
	}
}

func TestEmergencyFund_Dependents9Months(t *testing.T) {
	plan := New().Compute(Request{
		Transactions:  mkMonthlyExpense(30_000, 4),
		HasDependents: true,
	})
	if plan.CoverageMonths != 9 {
		t.Errorf("dependents→9 months; got %d", plan.CoverageMonths)
	}
}

func TestEmergencyFund_AlreadyFundedShowsZeroGap(t *testing.T) {
	plan := New().Compute(Request{
		Transactions:      mkMonthlyExpense(20_000, 4),
		LiquidReservesINR: 5_00_000,
	})
	if plan.GapINR != 0 {
		t.Errorf("expected zero gap; got %.0f", plan.GapINR)
	}
	if plan.MonthsToTarget != 0 {
		t.Errorf("zero gap should have 0 months to target")
	}
}

func TestEmergencyFund_RationalePresent(t *testing.T) {
	plan := New().Compute(Request{Transactions: mkMonthlyExpense(10_000, 4)})
	if plan.Rationale == "" {
		t.Errorf("rationale missing")
	}
}

func mkMonthlyExpense(rupees float64, months int) []finance.Transaction {
	out := []finance.Transaction{}
	monthStrs := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06"}
	for i := 0; i < months; i++ {
		out = append(out, finance.Transaction{
			Date: monthStrs[i] + "-15", AmountCents: int64(-rupees * 100),
		})
	}
	return out
}
