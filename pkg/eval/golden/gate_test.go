package golden

import (
	"context"
	"sort"
	"testing"
)

// TestGoldenGate is the honest replacement for the old tautological
// TestGoldenDataset_AllCases. It loads the real dataset from disk and scores
// every case with the deterministic judges. A case "passes" only when the
// judge's verdict MATCHES the case's asserted verdict — so if a judge's rule
// logic regresses, cases flip and THIS GATE FAILS. That is the property the
// predecessor lacked.
//
// The seed cases are hand-verified to be self-consistent, so the honest floor
// for the seed is 100% of scored cases. Unscored cases (domains without a
// deterministic judge) are reported but excluded from the rate — never counted
// as passes.
func TestGoldenGate(t *testing.T) {
	ctx := context.Background()
	cases, datasets, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("load embedded dataset: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("no golden cases loaded")
	}

	var scored, passed, unscored int
	var failures []string
	for _, c := range cases {
		r := EvaluateCase(ctx, c)
		if !r.Scored {
			unscored++
			continue
		}
		scored++
		if r.Pass {
			passed++
		} else {
			failures = append(failures, c.ID+": expected "+r.ExpectedVerdict+
				" but judge verdict was "+boolVerdict(r.JudgeVerdict)+" ("+r.Reason+")")
		}
	}

	t.Logf("datasets=%d cases=%d scored=%d passed=%d unscored=%d",
		len(datasets), len(cases), scored, passed, unscored)
	for _, f := range failures {
		t.Logf("MISMATCH: %s", f)
	}

	if scored == 0 {
		t.Fatal("no cases were scored — judges are not wired")
	}
	rate := float64(passed) / float64(scored)
	const floor = 1.0 // seed cases are hand-verified; any flip is a real regression
	if rate < floor {
		t.Fatalf("golden gate pass rate %.1f%% (%d/%d scored) below floor %.0f%%",
			rate*100, passed, scored, floor*100)
	}
}

// TestGoldenGate_CoverageReport documents per-failure-mode coverage. It does not
// fail the build (Phase 0 establishes the baseline); Phase 2 will raise this to
// a hard "≥N cases per failure mode" gate.
func TestGoldenGate_CoverageReport(t *testing.T) {
	cases, _, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	byFM := map[string]int{}
	byDomain := map[string]int{}
	for _, c := range cases {
		byFM[c.FailureMode]++
		byDomain[c.Domain]++
	}
	t.Logf("=== per-domain coverage ===")
	for _, d := range sortedKeys(byDomain) {
		t.Logf("  %-14s %d cases", d, byDomain[d])
	}
	t.Logf("=== per-failure-mode coverage (target ≥5 in Phase 2) ===")
	for _, fm := range sortedKeys(byFM) {
		t.Logf("  %-12s %d cases", fm, byFM[fm])
	}
}

func boolVerdict(b bool) string {
	if b {
		return "PASS"
	}
	return "FAIL"
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
