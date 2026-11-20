// Package golden is the single source of truth for the Genie golden test
// dataset. It replaces two disjoint, drifting systems:
//
//   - The Go-literal cases in pkg/eval/regression_test.go, which were scored by
//     a tautological evaluator (always returned Pass:true).
//   - The orphaned JSON files in pkg/eval/golden_datasets/, which were loaded by
//     no code and carried pre-baked actual_output/evaluation_results.
//
// Design principles (see EVAL_EXPANSION_PLAN.md):
//   - A Case carries only input + expected + rubric references. It NEVER carries
//     a pre-baked actual_output — the verdict must be PRODUCED at evaluation time
//     by a real (deterministic or LLM) judge.
//   - One on-disk JSON format, loaded by LoadGoldenDir, scored by EvaluateCase.
//   - The pass-rate gate built on this CAN fail, unlike its predecessor.
package golden

import "time"

// Case is one scenario in the golden dataset. It is a source of truth: input +
// expected outcome + rubric references, with NO pre-baked result.
type Case struct {
	// ID is a unique identifier, e.g. "SE-CORRECT-001" or "TC-FM-CO-002-003".
	ID string `json:"id"`

	// FailureMode maps to a failure_modes.json entry (e.g. "FM-SE-001"), or
	// "none" for a happy-path case.
	FailureMode string `json:"failure_mode"`

	// Domain is the system area: settlement | compliance | orchestration |
	// lineage | merchant | advisor. Drives which judge scores the case. May be
	// empty, in which case EvaluateCase falls back to a direct expected-check.
	Domain string `json:"domain"`

	// RubricRefs lists rubric IDs used to evaluate this case (e.g. ["RB-SE-001"]).
	RubricRefs []string `json:"rubric_refs,omitempty"`

	// Scenario is a one or two sentence human description.
	Scenario string `json:"scenario"`

	// Input holds the parameters fed to the judge/agent. Keys are domain-specific
	// (see the *JudgeInput builders in evaluate.go).
	Input map[string]any `json:"input"`

	// Expected is the outcome a passing execution must satisfy. For deterministic
	// (Track A) scoring it expresses the verdict the judge should reach, e.g.
	// {"verdict":"PASS"} or {"verdict":"FAIL"}. It is NOT a pre-baked actual.
	Expected map[string]any `json:"expected"`

	// Metadata carries tags such as severity and category.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Source records provenance: "regression_test" (migrated Go literal),
	// "json_golden" (migrated JSON), or "" for newly authored cases.
	Source string `json:"source,omitempty"`
}

// ExpectedVerdict returns the verdict the case asserts a judge should reach
// ("PASS" or "FAIL"), derived from Expected. Resolution order:
//   - Expected["verdict"] if present ("PASS"/"FAIL", case-insensitive)
//   - else FailureMode=="none"/"" implies PASS (happy path), otherwise FAIL.
//
// The second clause encodes the convention that a case tied to a failure mode is
// asserting the system DETECTS that failure (verdict FAIL = correctly flagged).
func (c Case) ExpectedVerdict() string {
	if v, ok := c.Expected["verdict"].(string); ok {
		if up := upper(v); up == "PASS" || up == "FAIL" {
			return up
		}
	}
	if c.FailureMode == "" || c.FailureMode == "none" {
		return "PASS"
	}
	return "FAIL"
}

// Dataset is a named collection of cases for one domain/category, matching one
// on-disk JSON file.
type Dataset struct {
	Domain      string `json:"domain"`
	Category    string `json:"category"`
	Description string `json:"description,omitempty"`
	Cases       []Case `json:"cases"`
	// TotalCases is informational; LoadGoldenDir recomputes and verifies it.
	TotalCases int       `json:"total_cases"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

// upper uppercases an ASCII string without importing strings (keeps this file
// dependency-light; the rest of the package uses stdlib freely).
func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
