// cmd/eval-multiturn runs the multi-turn agent evaluation harness.
//
// Usage:
//
//	eval-multiturn [flags]
//
// Flags:
//
//	-dataset   path to JSON test-case file (default: pkg/eval/multiturn/data/agent_multiturn.json)
//	-provider  LLM backend: "ollama" (default) | "openai"
//	-model     model name (default: llama3.1 for Ollama, gpt-4.1 for OpenAI)
//	-base-url  override provider base URL
//	-skip-judge  skip the LLM-as-judge evaluator (fast offline mode)
//	-workers   number of concurrent test runners (default: 4)
//	-json      write results as JSON to stdout instead of human-readable table
//
// Environment:
//
//	OPENAI_API_KEY   required when -provider=openai
//	OLLAMA_BASE_URL  overrides the default http://localhost:11434
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/multiturn"
	evaltrace "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/trace"
)

func main() {
	dataset := flag.String("dataset", "pkg/eval/multiturn/data/agent_multiturn.json", "Path to JSON test-case dataset")
	provider := flag.String("provider", "", "LLM provider: ollama (default) | openai")
	model := flag.String("model", "", "Model name (default: llama3.1 for Ollama)")
	baseURL := flag.String("base-url", "", "Override provider base URL")
	skipJudge := flag.Bool("skip-judge", false, "Skip the LLM-as-judge evaluator")
	workers := flag.Int("workers", 4, "Number of concurrent test runners")
	asJSON := flag.Bool("json", false, "Emit results as JSON")
	withLaminar := flag.Bool("laminar", false,
		"Send eval traces to Laminar (LMNR_PROJECT_API_KEY must be set)")
	flag.Parse()

	// Resolve config from flags + environment.
	cfg := multiturn.RunnerConfig{
		Concurrency:  *workers,
		SkipLLMJudge: *skipJudge,
	}

	// Apply provider config to the judge (judge uses same backend as executor by convention).
	if *provider != "" || *model != "" || *baseURL != "" {
		cfg.JudgeCfg = multiturn.JudgeConfig{
			Provider: *provider,
			Model:    *model,
			BaseURL:  *baseURL,
			APIKey:   os.Getenv("OPENAI_API_KEY"),
		}
	}

	// Load dataset.
	cases, err := multiturn.LoadDataset(*dataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading dataset %q: %v\n", *dataset, err)
		os.Exit(1)
	}

	// Apply flag + env-level overrides to each test case's ExecConfig.
	// Command-line flags take precedence over per-case JSON config.
	ollamaURL := os.Getenv("OLLAMA_BASE_URL")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	for i := range cases {
		if cases[i].Data.Config == nil {
			cases[i].Data.Config = &multiturn.ExecConfig{}
		}
		c := cases[i].Data.Config
		// -provider / -model / -base-url flags override per-case config.
		if *provider != "" {
			c.Provider = *provider
		}
		if *model != "" {
			c.Model = *model
		}
		if *baseURL != "" {
			c.BaseURL = *baseURL
		}
		// Env vars fill in what flags didn't provide.
		if c.BaseURL == "" && ollamaURL != "" {
			c.BaseURL = ollamaURL
		}
		if c.APIKey == "" && openaiKey != "" {
			c.APIKey = openaiKey
		}
	}

	fmt.Fprintf(os.Stderr, "running %d test cases with %d workers (judge=%v)\n",
		len(cases), *workers, !*skipJudge)

	runner := multiturn.NewRunner(cfg)

	// ── Laminar tracing ──────────────────────────────────────────────────────
	if *withLaminar {
		lmnrURL := os.Getenv("LMNR_BASE_URL")
		if lmnrURL == "" {
			lmnrURL = "http://localhost:8000"
		}
		tp, tpErr := evaltrace.NewLaminarProvider(context.Background(), "genie-eval-multiturn")
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

	multiturn.PrintSummary(results)

	// Exit 1 if any test case failed.
	for _, r := range results {
		if r.Err != "" || r.Scores.Overall < 0.6 {
			os.Exit(1)
		}
	}
}
