package advisoreval

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/golden"
)

// mustCase builds an in-memory advisor case for regression tests.
func mustCase(t *testing.T, agent string, input, expected map[string]any) golden.Case {
	t.Helper()
	return golden.Case{
		ID:       "INLINE",
		Domain:   "advisor",
		Input:    input,
		Expected: expected,
		Metadata: map[string]string{"agent": agent},
	}
}

// TestAdvisorEval runs every advisor case by EXECUTING the real agent and
// checking its output. Because the advisor agents are deterministic, this gate
// is offline and CI-safe. It fails if any agent's behavior diverges from the
// asserted output — closing the advisor coverage gap with real execution.
func TestAdvisorEval(t *testing.T) {
	ctx := context.Background()
	cases, _, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("load advisor dataset: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("no advisor cases loaded")
	}

	passed := 0
	for _, c := range cases {
		r := EvaluateCase(ctx, c)
		if r.Pass {
			passed++
		} else {
			t.Errorf("FAIL %s [%s]: %s", c.ID, c.Metadata["agent"], r.Reason)
		}
	}
	t.Logf("advisor eval: %d/%d cases passed", passed, len(cases))
}

// TestAdvisorEval_DetectsRegression proves the gate is not vacuous: a case that
// asserts the wrong risk tolerance must fail.
func TestAdvisorEval_DetectsRegression(t *testing.T) {
	ctx := context.Background()
	// verified_high_value maps to "aggressive"; asserting "conservative" must fail.
	bad := mustCase(t, "profile-analyzer",
		map[string]any{"user_id": "x", "kyc_status": "verified_high_value"},
		map[string]any{"risk_tolerance": "conservative"})
	if r := EvaluateCase(ctx, bad); r.Pass {
		t.Fatal("gate is vacuous: a wrong-assertion case passed")
	}
}
