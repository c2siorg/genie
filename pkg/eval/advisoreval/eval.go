// Package advisoreval evaluates the advisor-pipeline agents (profile-analyzer,
// financial-analyst, recommendation-generator) by EXECUTING them for real and
// asserting on their output.
//
// This is "Track B" (real-agent execution) but, because the advisor agents are
// deterministic Go logic — not LLM calls — it runs fully offline and is CI-safe.
// It closes the advisor coverage gap that the deterministic golden judges (Track
// A) could not, without resorting to mocks.
//
// Cases reuse the golden.Case schema. The case's Metadata["agent"] selects which
// agent to run; the case's Input is the agent payload; the case's Expected holds
// the output assertions (keys depend on the agent — see the assert* helpers).
package advisoreval

import (
	"context"
	"fmt"

	financial_analyst "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/financial-analyst"
	profile_analyzer "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/profile-analyzer"
	recommendation_generator "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/recommendation-generator"
	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/golden"
)

// Result is the outcome of running one advisor case.
type Result struct {
	CaseID string
	Pass   bool
	Reason string
}

// EvaluateCase runs the advisor agent named by c.Metadata["agent"] against
// c.Input and checks c.Expected. If Expected["error"] is true, the agent is
// expected to return an error (invalid-input cases); otherwise it must succeed
// and satisfy the per-agent assertions.
func EvaluateCase(ctx context.Context, c golden.Case) Result {
	agent := c.Metadata["agent"]
	wantErr, _ := c.Expected["error"].(bool)

	var out interface{}
	var err error
	switch agent {
	case "profile-analyzer":
		out, err = profile_analyzer.NewAgent().Execute(ctx, c.Input)
	case "financial-analyst":
		out, err = financial_analyst.NewAgent().Execute(ctx, c.Input)
	case "recommendation-generator":
		out, err = recommendation_generator.NewAgent().Execute(ctx, c.Input)
	default:
		return Result{CaseID: c.ID, Pass: false, Reason: "unknown agent " + strconvQuote(agent)}
	}

	if wantErr {
		if err == nil {
			return fail(c, "expected an error but Execute succeeded")
		}
		return Result{CaseID: c.ID, Pass: true, Reason: "error correctly returned"}
	}
	if err != nil {
		return fail(c, "unexpected error: "+err.Error())
	}

	switch agent {
	case "profile-analyzer":
		return assertProfile(c, out)
	case "financial-analyst":
		return assertAnalyst(c, out)
	case "recommendation-generator":
		return assertGenerator(c, out)
	}
	return fail(c, "no assertions ran")
}

// assertProfile checks profile-analyzer output (a *contractv1.UserProfile).
// Supported Expected keys: risk_tolerance (string), min_restrictions (number).
func assertProfile(c golden.Case, out interface{}) Result {
	p, ok := out.(*contractv1.UserProfile)
	if !ok {
		return fail(c, fmt.Sprintf("output %T is not *UserProfile", out))
	}
	if want, ok := c.Expected["risk_tolerance"].(string); ok {
		if string(p.RiskTolerance) != want {
			return fail(c, fmt.Sprintf("risk_tolerance=%q want %q", p.RiskTolerance, want))
		}
	}
	if want, ok := numExpected(c, "min_restrictions"); ok {
		if len(p.ComplianceRestrictions) < want {
			return fail(c, fmt.Sprintf("restrictions=%d want >=%d", len(p.ComplianceRestrictions), want))
		}
	}
	return pass(c)
}

// assertAnalyst checks financial-analyst output ([]contractv1.SpendingOpportunity).
// Supported Expected keys: opportunity_count (number).
func assertAnalyst(c golden.Case, out interface{}) Result {
	opps, ok := out.([]contractv1.SpendingOpportunity)
	if !ok {
		return fail(c, fmt.Sprintf("output %T is not []SpendingOpportunity", out))
	}
	if want, ok := numExpected(c, "opportunity_count"); ok {
		if len(opps) != want {
			return fail(c, fmt.Sprintf("opportunities=%d want %d", len(opps), want))
		}
	}
	return pass(c)
}

// assertGenerator checks recommendation-generator output ([]contractv1.Recommendation).
// Supported Expected keys: recommendation_count (number), confidence (number).
func assertGenerator(c golden.Case, out interface{}) Result {
	recs, ok := out.([]contractv1.Recommendation)
	if !ok {
		return fail(c, fmt.Sprintf("output %T is not []Recommendation", out))
	}
	if want, ok := numExpected(c, "recommendation_count"); ok {
		if len(recs) != want {
			return fail(c, fmt.Sprintf("recommendations=%d want %d", len(recs), want))
		}
	}
	if wantConf, ok := c.Expected["confidence"].(float64); ok {
		if len(recs) == 0 {
			return fail(c, "expected a confidence but got 0 recommendations")
		}
		if recs[0].EstimatedImpact.Confidence != wantConf {
			return fail(c, fmt.Sprintf("confidence=%v want %v", recs[0].EstimatedImpact.Confidence, wantConf))
		}
	}
	return pass(c)
}

func numExpected(c golden.Case, key string) (int, bool) {
	if v, ok := c.Expected[key].(float64); ok {
		return int(v), true
	}
	return 0, false
}

func pass(c golden.Case) Result { return Result{CaseID: c.ID, Pass: true} }
func fail(c golden.Case, reason string) Result {
	return Result{CaseID: c.ID, Pass: false, Reason: reason}
}

func strconvQuote(s string) string {
	if s == "" {
		return `"(empty)"`
	}
	return `"` + s + `"`
}
