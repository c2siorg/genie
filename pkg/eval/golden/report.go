package golden

import (
	"context"
	"fmt"
	"sort"
)

// Report aggregates the outcome of scoring a whole dataset.
type Report struct {
	Total    int
	Scored   int
	Passed   int
	Unscored int
	Failures []Result // scored cases whose verdict did not match
	ByDomain map[string]int
	ByFM     map[string]int
}

// PassRate returns passed/scored (1.0 when nothing was scored, to avoid a
// divide-by-zero; callers should check Scored>0 before trusting it).
func (r Report) PassRate() float64 {
	if r.Scored == 0 {
		return 1.0
	}
	return float64(r.Passed) / float64(r.Scored)
}

// Evaluate scores every case and produces an aggregate Report. This is the
// single scoring path shared by the gate test and the eval-golden CLI.
func Evaluate(ctx context.Context, cases []Case) Report {
	rep := Report{
		Total:    len(cases),
		ByDomain: map[string]int{},
		ByFM:     map[string]int{},
	}
	for _, c := range cases {
		rep.ByDomain[c.Domain]++
		rep.ByFM[c.FailureMode]++
		res := EvaluateCase(ctx, c)
		if !res.Scored {
			rep.Unscored++
			continue
		}
		rep.Scored++
		if res.Pass {
			rep.Passed++
		} else {
			rep.Failures = append(rep.Failures, res)
		}
	}
	return rep
}

// String renders a human-readable summary (used by the CLI).
func (r Report) String() string {
	out := fmt.Sprintf("golden dataset: %d cases (%d scored, %d unscored)\n", r.Total, r.Scored, r.Unscored)
	out += fmt.Sprintf("pass rate: %.1f%% (%d/%d scored)\n", r.PassRate()*100, r.Passed, r.Scored)

	out += "\nper-domain:\n"
	for _, d := range sortedStringKeys(r.ByDomain) {
		out += fmt.Sprintf("  %-14s %d\n", d, r.ByDomain[d])
	}
	out += "\nfailure modes covered: " + fmt.Sprintf("%d\n", len(r.ByFM))

	if len(r.Failures) > 0 {
		out += "\nMISMATCHES:\n"
		for _, f := range r.Failures {
			out += fmt.Sprintf("  %s: expected %s, judge said %s (%s)\n",
				f.CaseID, f.ExpectedVerdict, verdictString(f.JudgeVerdict), f.Reason)
		}
	}
	return out
}

func verdictString(b bool) string {
	if b {
		return "PASS"
	}
	return "FAIL"
}

func sortedStringKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
