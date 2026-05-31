package multiturn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	otrace "go.opentelemetry.io/otel/trace"
)

// RunnerConfig controls how the Runner executes the dataset.
type RunnerConfig struct {
	// Concurrency is the number of test cases evaluated in parallel (default: 4).
	Concurrency int
	// JudgeCfg configures the LLM judge. Zero value defaults to Ollama.
	JudgeCfg JudgeConfig
	// SkipLLMJudge disables the LLM-as-judge evaluator (useful for fast offline tests).
	SkipLLMJudge bool
}

// Runner evaluates a dataset of TestCases.
type Runner struct {
	executor *Executor
	cfg      RunnerConfig
	// TP is an optional TracerProvider. When non-nil each test case run is
	// wrapped in an OTLP span so scores appear in Laminar under Traces.
	TP otrace.TracerProvider
}

// NewRunner creates a Runner. Pass a zero-value RunnerConfig for all defaults.
func NewRunner(cfg RunnerConfig) *Runner {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	return &Runner{
		executor: NewExecutor(),
		cfg:      cfg,
	}
}

// RunDataset evaluates all test cases and returns one EvalResult per case.
// Cases are processed concurrently (up to RunnerConfig.Concurrency goroutines).
func (r *Runner) RunDataset(ctx context.Context, cases []TestCase) []EvalResult {
	results := make([]EvalResult, len(cases))

	sem := make(chan struct{}, r.cfg.Concurrency)
	var wg sync.WaitGroup

	for i, tc := range cases {
		wg.Add(1)
		go func(idx int, tc TestCase) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = r.runOne(ctx, tc)
		}(i, tc)
	}
	wg.Wait()
	return results
}

// runOne executes a single test case end-to-end.
func (r *Runner) runOne(ctx context.Context, tc TestCase) EvalResult {
	// ── Laminar tracing ──────────────────────────────────────────────────────
	var span otrace.Span
	if r.TP != nil {
		tracer := r.TP.Tracer("genie/eval/multiturn")
		task := tc.Target.OriginalTask
		if task == "" {
			task = tc.Data.Prompt
		}
		if len(task) > 60 {
			task = task[:57] + "..."
		}
		ctx, span = tracer.Start(ctx, "eval "+task)
		defer span.End()
	}

	res, err := r.executor.Run(ctx, tc.Data)
	if err != nil {
		if span != nil {
			span.SetStatus(otelcodes.Error, err.Error())
			span.SetAttributes(attribute.String("eval.error", err.Error()))
		}
		return EvalResult{
			TestCase: tc,
			Result:   res,
			Err:      fmt.Sprintf("executor: %v", err),
			Scores:   EvalScore{},
		}
	}

	// --- Deterministic evaluators (no LLM call needed) ---
	toolOrder := ToolOrderCorrect(res, tc.Target)
	toolsAvoided := ToolsAvoided(res, tc.Target)

	// --- LLM-as-judge ---
	var outputQuality float64
	if r.cfg.SkipLLMJudge || tc.Target.OriginalTask == "" {
		// No task description → skip judge, treat as pass.
		outputQuality = 1.0
	} else {
		score, _, judgeErr := LLMJudge(ctx, res, tc.Target, r.cfg.JudgeCfg)
		if judgeErr != nil {
			// Non-fatal: record the error but continue.
			outputQuality = 0.5
			return EvalResult{
				TestCase: tc,
				Result:   res,
				Err:      fmt.Sprintf("llmJudge: %v", judgeErr),
				Scores: EvalScore{
					ToolOrder:     toolOrder,
					ToolsAvoided:  toolsAvoided,
					OutputQuality: outputQuality,
					Overall:       (toolOrder + toolsAvoided + outputQuality) / 3,
				},
			}
		}
		outputQuality = score
	}

	overall := (toolOrder + toolsAvoided + outputQuality) / 3

	if span != nil {
		passed := overall >= 0.6
		span.SetAttributes(
			attribute.Float64("eval.score.tool_order", toolOrder),
			attribute.Float64("eval.score.tools_avoided", toolsAvoided),
			attribute.Float64("eval.score.output_quality", outputQuality),
			attribute.Float64("eval.score.overall", overall),
			attribute.Bool("eval.passed", passed),
			attribute.StringSlice("eval.tools_used", res.ToolsUsed),
		)
		if passed {
			span.SetStatus(otelcodes.Ok, "")
		} else {
			span.SetStatus(otelcodes.Error, "eval failed")
		}
	}

	return EvalResult{
		TestCase: tc,
		Result:   res,
		Scores: EvalScore{
			ToolOrder:     toolOrder,
			ToolsAvoided:  toolsAvoided,
			OutputQuality: outputQuality,
			Overall:       overall,
		},
	}
}

// -------------------------------------------------------------------
// Dataset helpers
// -------------------------------------------------------------------

// LoadDataset reads a JSON file containing a []TestCase array.
//
// The JSON schema matches the TypeScript reference implementation:
//
//	[{"data": {...}, "target": {...}}, ...]
func LoadDataset(path string) ([]TestCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("LoadDataset: %w", err)
	}
	defer f.Close()

	var cases []TestCase
	if err := json.NewDecoder(f).Decode(&cases); err != nil {
		return nil, fmt.Errorf("LoadDataset decode: %w", err)
	}
	return cases, nil
}

// PrintSummary writes a human-readable summary of evaluation results to stdout.
func PrintSummary(results []EvalResult) {
	passed, total := 0, len(results)
	var sumOverall float64

	for i, r := range results {
		status := "✓"
		if r.Err != "" || r.Scores.Overall < 0.6 {
			status = "✗"
		} else {
			passed++
		}
		task := r.TestCase.Target.OriginalTask
		if task == "" {
			task = r.TestCase.Data.Prompt
		}
		fmt.Printf("[%d] %s  overall=%.2f  toolOrder=%.2f  avoided=%.2f  quality=%.2f  task=%q\n",
			i+1, status, r.Scores.Overall,
			r.Scores.ToolOrder, r.Scores.ToolsAvoided, r.Scores.OutputQuality,
			truncate(task, 60),
		)
		if r.Err != "" {
			fmt.Printf("    error: %s\n", r.Err)
		}
		sumOverall += r.Scores.Overall
	}

	fmt.Printf("\n--- %d/%d passed  avg_overall=%.2f ---\n",
		passed, total, sumOverall/float64(max(total, 1)))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
