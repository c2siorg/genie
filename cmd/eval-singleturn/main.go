// cmd/eval-singleturn runs the single-turn tool-selection evaluation harness.
//
// Usage:
//
//	eval-singleturn [flags]
//
// Flags:
//
//	-dataset    path to JSON test-case file (default: pkg/eval/singleturn/data/file-tools.json)
//	-model      model name (overrides env / per-case config)
//	-base-url   override LLM base URL
//	-workers    number of concurrent runners (default: 4)
//	-json       write results as JSON to stdout
//	-laminar    send traces to Laminar (reads LMNR_PROJECT_API_KEY + LMNR_BASE_URL)
//
// Environment:
//
//	GENIE_OLLAMA_CHAT     model name (default: llama3.2:1b)
//	GENIE_OLLAMA_URL      LLM base URL (default: http://localhost:11434)
//	LMNR_PROJECT_API_KEY  Laminar project API key — copy from UI Settings → API Keys
//	LMNR_BASE_URL         Laminar HTTP base URL   (default: http://localhost:8000)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/singleturn"
	evaltrace "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/trace"
)

func main() {
	dataset := flag.String("dataset", "pkg/eval/singleturn/data/file-tools.json",
		"Path to JSON test-case dataset")
	model := flag.String("model", "", "Model name (overrides env/per-case config)")
	baseURL := flag.String("base-url", "", "Override LLM base URL")
	workers := flag.Int("workers", 4, "Number of concurrent test runners")
	asJSON := flag.Bool("json", false, "Emit results as JSON to stdout")
	withLaminar := flag.Bool("laminar", false,
		"Send eval traces to Laminar (LMNR_PROJECT_API_KEY must be set)")
	flag.Parse()

	// Load dataset.
	cases, err := singleturn.LoadDataset(*dataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading dataset %q: %v\n", *dataset, err)
		os.Exit(1)
	}

	// Apply CLI overrides to each test-case config.
	for i := range cases {
		if cases[i].Data.Config == nil {
			cases[i].Data.Config = &singleturn.EvalConfig{}
		}
		if *model != "" {
			cases[i].Data.Config.Model = *model
		}
		if *baseURL != "" {
			cases[i].Data.Config.BaseURL = *baseURL
		}
	}

	exec := singleturn.NewExecutor()
	runner := singleturn.NewRunner(exec)
	runner.Workers = *workers

	// ── Laminar tracing ────────────────────────────────────────────────────
	if *withLaminar {
		lmnrURL := os.Getenv("LMNR_BASE_URL")
		if lmnrURL == "" {
			lmnrURL = "http://localhost:8000"
		}
		tp, tpErr := evaltrace.NewLaminarProvider(context.Background(), "genie-eval-singleturn")
		if tpErr != nil {
			fmt.Fprintf(os.Stderr, "laminar setup error: %v\n", tpErr)
			os.Exit(1)
		}
		defer func() {
			if sErr := tp.Shutdown(context.Background()); sErr != nil {
				fmt.Fprintf(os.Stderr, "laminar shutdown: %v\n", sErr)
			}
		}()
		runner.TP = tp
		fmt.Fprintf(os.Stderr, "laminar tracing enabled → %s  (UI: http://localhost:5667)\n", lmnrURL)
	}

	fmt.Fprintf(os.Stderr, "running %d test cases with %d workers\n", len(cases), *workers)
	results := runner.RunDataset(context.Background(), cases)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
			os.Exit(1)
		}
		return
	}

	singleturn.PrintSummary(results)

	// Exit 1 if any test case failed — useful for CI gating.
	for _, r := range results {
		if !r.Passed {
			os.Exit(1)
		}
	}
}
