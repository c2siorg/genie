package golden

import (
	"context"
	"testing"
)

// TestEvaluateCase_GateCanFail is the central regression test for Phase 0: it
// proves the new evaluator is NOT tautological. The predecessor
// (regression_test.go::evaluateTestCase) returned Pass:true unconditionally;
// here, a case whose judge verdict contradicts its asserted expected verdict
// must FAIL.
func TestEvaluateCase_GateCanFail(t *testing.T) {
	ctx := context.Background()

	// A settlement happy-path case: amounts match, CBDC committed, reconciled.
	// The deterministic judge should PASS it, and the case expects PASS → match.
	good := Case{
		ID: "SE-OK", Domain: "settlement", FailureMode: "none",
		Expected: map[string]any{"verdict": "PASS"},
		Input: map[string]any{
			"order_id": "O1", "settlement_amount": float64(50000),
			"expected_amount": float64(50000), "cbdc_committed": true,
			"reconciliation_passed": true,
		},
	}
	if r := EvaluateCase(ctx, good); !r.Scored || !r.Pass {
		t.Fatalf("happy-path settlement should pass: %+v", r)
	}

	// A BROKEN case: it ASSERTS PASS, but the inputs are clearly a failed
	// settlement (amount mismatch, not committed, not reconciled). The judge will
	// return Pass:false, contradicting the asserted PASS → the gate must FAIL.
	// This is exactly the scenario the old tautological evaluator let through.
	broken := Case{
		ID: "SE-BROKEN", Domain: "settlement", FailureMode: "none",
		Expected: map[string]any{"verdict": "PASS"},
		Input: map[string]any{
			"order_id": "O2", "settlement_amount": float64(10),
			"expected_amount": float64(50000), "cbdc_committed": false,
			"reconciliation_passed": false,
		},
	}
	r := EvaluateCase(ctx, broken)
	if !r.Scored {
		t.Fatal("settlement case should be scored")
	}
	if r.Pass {
		t.Fatal("GATE FAILURE EXPECTED: a failed settlement asserting PASS must NOT pass the gate (evaluator is tautological again)")
	}
}

// TestEvaluateCase_FailureModeDetection verifies the FAIL-verdict convention: a
// case tied to a failure mode passes the gate only when the judge DETECTS the
// failure (judge verdict = fail).
func TestEvaluateCase_FailureModeDetection(t *testing.T) {
	ctx := context.Background()
	// Compliance case: velocity exceeded → judge should flag (verdict fail).
	// The case asserts FAIL (the system should reject), so judge-fail == match.
	c := Case{
		ID: "CO-VELOCITY", Domain: "compliance", FailureMode: "FM-CO-002",
		Expected: map[string]any{"verdict": "FAIL"},
		Input: map[string]any{
			"customer_id": "C1", "kyc_status": "verified",
			"velocity_threshold_paise": float64(100000),
			"current_velocity_paise":   float64(500000),
			"velocity_exceeded":        true,
		},
	}
	r := EvaluateCase(ctx, c)
	if !r.Scored {
		t.Fatal("compliance case should be scored")
	}
	if !r.Pass {
		t.Fatalf("velocity-exceeded case should pass the gate (judge correctly detected the breach): %+v", r)
	}
}

// TestEvaluateCase_UnscoredDomainExcluded confirms domains without a judge are
// marked Scored=false (excluded from the gate) rather than fake-passed.
func TestEvaluateCase_UnscoredDomainExcluded(t *testing.T) {
	ctx := context.Background()
	// advisor + agent_behavior are Track B (real-agent + LLM judge), so they have
	// no deterministic judge; an empty domain also has none. (settlement,
	// compliance, orchestration, lineage, merchant DO have deterministic judges.)
	for _, dom := range []string{"", "advisor", "agent_behavior"} {
		r := EvaluateCase(ctx, Case{ID: "X", Domain: dom, Expected: map[string]any{"verdict": "PASS"}})
		if r.Scored {
			t.Errorf("domain %q has no judge yet; must be Scored=false, got %+v", dom, r)
		}
		if r.Pass {
			t.Errorf("domain %q must not report Pass when unscored (no fake green)", dom)
		}
	}
}

// TestExpectedVerdict covers the verdict-resolution convention.
func TestExpectedVerdict(t *testing.T) {
	cases := []struct {
		c    Case
		want string
	}{
		{Case{Expected: map[string]any{"verdict": "PASS"}}, "PASS"},
		{Case{Expected: map[string]any{"verdict": "fail"}}, "FAIL"},
		{Case{FailureMode: "none"}, "PASS"},
		{Case{FailureMode: ""}, "PASS"},
		{Case{FailureMode: "FM-SE-001"}, "FAIL"},
	}
	for _, tc := range cases {
		if got := tc.c.ExpectedVerdict(); got != tc.want {
			t.Errorf("ExpectedVerdict(%+v) = %q, want %q", tc.c, got, tc.want)
		}
	}
}
