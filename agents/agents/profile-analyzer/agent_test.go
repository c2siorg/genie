package profile_analyzer

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

func TestExecute_KYCStatusToRiskTolerance(t *testing.T) {
	a := NewAgent()
	cases := map[string]contractv1.RiskTolerance{
		"verified_high_value": contractv1.RiskAggressive,
		"verified_standard":   contractv1.RiskModerate,
		"pending":             contractv1.RiskConservative,
		"":                    contractv1.RiskConservative,
	}
	for kyc, want := range cases {
		out, err := a.Execute(context.Background(), map[string]any{
			"user_id":    "u-1",
			"kyc_status": kyc,
		})
		if err != nil {
			t.Fatalf("kyc=%q: %v", kyc, err)
		}
		p, ok := out.(*contractv1.UserProfile)
		if !ok {
			t.Fatalf("kyc=%q: output type %T, want *UserProfile", kyc, out)
		}
		if p.RiskTolerance != want {
			t.Errorf("kyc=%q: risk = %q, want %q", kyc, p.RiskTolerance, want)
		}
	}
}

func TestExecute_MissingUserID(t *testing.T) {
	a := NewAgent()
	_, err := a.Execute(context.Background(), map[string]any{"kyc_status": "pending"})
	if err == nil {
		t.Fatal("expected validation error for missing user_id")
	}
	var execErr *core.ExecutionError
	if ok := asExecutionError(err, &execErr); !ok || execErr.Code != core.ErrorCodeValidation {
		t.Fatalf("want validation ExecutionError, got %v", err)
	}
}

func TestExecute_ComplianceConstraintsForPendingKYC(t *testing.T) {
	a := NewAgent()
	out, err := a.Execute(context.Background(), map[string]any{
		"user_id":    "u-1",
		"kyc_status": "pending",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	p := out.(*contractv1.UserProfile)
	if len(p.ComplianceRestrictions) == 0 {
		t.Fatal("pending KYC should carry compliance restrictions")
	}
}

// asExecutionError is a tiny errors.As shim kept local to avoid importing errors
// in the test for one call.
func asExecutionError(err error, target **core.ExecutionError) bool {
	if e, ok := err.(*core.ExecutionError); ok {
		*target = e
		return true
	}
	return false
}
