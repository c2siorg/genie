// Command eval-golden runs the deterministic (Track A) golden gate from the
// command line. It loads the golden dataset (embedded by default, or from a
// directory via -data), scores every case with the deterministic judges, prints
// a per-domain / failure-mode report, and exits non-zero if the pass rate is
// below the floor — so it is usable both interactively and in CI.
//
// Usage:
//
//	eval-golden                 # score the embedded dataset
//	eval-golden -data DIR       # score an on-disk dataset directory
//	eval-golden -floor 0.95     # custom pass-rate floor (default 1.0)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/advisoreval"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/golden"
)

func main() {
	dataDir := flag.String("data", "", "load dataset from this directory instead of the embedded one")
	floor := flag.Float64("floor", 1.0, "minimum pass rate over scored cases (0.0-1.0)")
	flag.Parse()

	var (
		cases []golden.Case
		err   error
	)
	if *dataDir != "" {
		cases, _, err = golden.LoadGoldenDir(*dataDir)
	} else {
		cases, _, err = golden.LoadEmbedded()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval-golden: load failed: %v\n", err)
		os.Exit(2)
	}
	if len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "eval-golden: no cases loaded")
		os.Exit(2)
	}

	ctx := context.Background()
	rep := golden.Evaluate(ctx, cases)
	fmt.Println("── Track A: deterministic judge gate ──")
	fmt.Print(rep.String())

	failed := false
	if rep.Scored == 0 {
		fmt.Fprintln(os.Stderr, "eval-golden: FAIL — no cases were scored (judges not wired)")
		failed = true
	} else if rep.PassRate() < *floor {
		fmt.Fprintf(os.Stderr, "eval-golden: FAIL — Track A pass rate %.1f%% below floor %.1f%%\n",
			rep.PassRate()*100, *floor*100)
		failed = true
	}

	// Track B: advisor-agent execution eval (real agents, deterministic, offline).
	// Skipped when -data points at an ad-hoc directory (advisor cases are embedded).
	if *dataDir == "" {
		advCases, _, aerr := advisoreval.LoadEmbedded()
		if aerr != nil {
			fmt.Fprintf(os.Stderr, "eval-golden: advisor load failed: %v\n", aerr)
			os.Exit(2)
		}
		advPass := 0
		for _, c := range advCases {
			r := advisoreval.EvaluateCase(ctx, c)
			if r.Pass {
				advPass++
			} else {
				fmt.Fprintf(os.Stderr, "  advisor FAIL %s: %s\n", r.CaseID, r.Reason)
			}
		}
		fmt.Printf("\n── Track B: advisor-agent execution ──\nadvisor: %d/%d cases passed (real agent execution)\n",
			advPass, len(advCases))
		if advPass != len(advCases) {
			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
	fmt.Printf("\neval-golden: PASS — Track A %.1f%% ≥ floor %.1f%%, Track B advisor OK\n",
		rep.PassRate()*100, *floor*100)
}
