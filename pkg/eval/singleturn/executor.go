package singleturn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	otrace "go.opentelemetry.io/otel/trace"
)

// Executor runs a single-turn eval: one LLM call with mocked tools.
// Tools never execute — only tool selection is tested.
// Mirrors singleTurnWithMocks() from lesson 03.
type Executor struct {
	// BaseURL defaults to Ollama (GENIE_OLLAMA_URL env var).
	BaseURL string
	// Model defaults to GENIE_OLLAMA_CHAT env var.
	Model      string
	HTTPClient *http.Client
}

// NewExecutor creates an Executor with Ollama defaults.
func NewExecutor() *Executor {
	model := os.Getenv("GENIE_OLLAMA_CHAT")
	if model == "" {
		model = "llama3.2:1b"
	}
	baseURL := os.Getenv("GENIE_OLLAMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Executor{
		BaseURL:    baseURL,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Run executes a single-turn eval and returns which tools the agent selected.
// Tools have empty execute functions — the model sees schemas but never runs code.
func (e *Executor) Run(ctx context.Context, data EvalData) (SingleTurnResult, error) {
	model := e.Model
	baseURL := e.BaseURL
	if data.Config != nil {
		if data.Config.Model != "" {
			model = data.Config.Model
		}
		if data.Config.BaseURL != "" {
			baseURL = data.Config.BaseURL
		}
	}

	// Build tool definitions from the tool name list.
	// Schemas are minimal — just enough for the model to know the tool exists.
	tools := make([]map[string]any, 0, len(data.Tools))
	for _, name := range data.Tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": toolDescription(name),
				"parameters": map[string]any{
					"type":                 "object",
					"properties":           map[string]any{},
					"additionalProperties": true,
				},
			},
		})
	}

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": data.Prompt},
		},
		"tools": tools,
	}
	raw, _ := json.Marshal(reqBody)

	endpoint := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return SingleTurnResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return SingleTurnResult{}, fmt.Errorf("LLM call: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode/100 != 2 {
		return SingleTurnResult{}, fmt.Errorf("LLM http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return SingleTurnResult{}, fmt.Errorf("decode: %w", err)
	}

	var calls []ToolCall
	var names []string
	if len(out.Choices) > 0 {
		for _, tc := range out.Choices[0].Message.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			calls = append(calls, ToolCall{ToolName: tc.Function.Name, Args: args})
			names = append(names, tc.Function.Name)
		}
	}

	return SingleTurnResult{
		ToolCalls:   calls,
		ToolNames:   names,
		SelectedAny: len(names) > 0,
	}, nil
}

// ─── Dataset runner ────────────────────────────────────────────────────────

// Runner executes a dataset of test cases concurrently.
type Runner struct {
	Executor *Executor
	Workers  int
	// TP is an optional TracerProvider. When non-nil, each test case is wrapped
	// in an OTLP span so scores appear as trace attributes in Laminar.
	// Set via pkg/eval/trace.NewLaminarProvider.
	TP otrace.TracerProvider
}

// NewRunner creates a Runner.
func NewRunner(exec *Executor) *Runner {
	if exec == nil {
		exec = NewExecutor()
	}
	return &Runner{Executor: exec, Workers: 4}
}

// RunDataset evaluates all test cases and returns results.
// Respects ctx cancellation: if the context is cancelled before a goroutine
// can acquire a semaphore slot, the slot-wait is abandoned and the result is
// recorded as a cancellation error instead of leaking the goroutine.
func (r *Runner) RunDataset(ctx context.Context, cases []TestCase) []EvalResult {
	results := make([]EvalResult, len(cases))
	sem := make(chan struct{}, r.Workers)

	type indexedResult struct {
		idx int
		res EvalResult
	}
	ch := make(chan indexedResult, len(cases))

	for i, tc := range cases {
		// Acquire semaphore slot or bail on cancellation — prevents goroutine leak.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			// Record a cancelled result for this case and all remaining ones.
			for j := i; j < len(cases); j++ {
				ch <- indexedResult{idx: j, res: EvalResult{
					TestCase: cases[j],
					Error:    ctx.Err().Error(),
				}}
			}
			goto drain
		}
		go func(idx int, tc TestCase) {
			defer func() { <-sem }()

			runCtx := ctx
			var span otrace.Span

			// ── Laminar tracing ─────────────────────────────────────────────
			if r.TP != nil {
				tracer := r.TP.Tracer("genie/eval/singleturn")
				spanName := spanLabel(tc.Data.Prompt)
				runCtx, span = tracer.Start(ctx, spanName)
				defer span.End()
			}

			result, err := r.Executor.Run(runCtx, tc.Data)
			er := EvalResult{TestCase: tc, Result: result}
			if err != nil {
				er.Error = err.Error()
				if span != nil {
					span.SetStatus(otelcodes.Error, err.Error())
					span.SetAttributes(attribute.String("eval.error", err.Error()))
				}
			} else {
				er.Scores = Score(result, tc.Target)
				er.Passed = allPassed(er.Scores)
				if span != nil {
					attrs := []attribute.KeyValue{
						attribute.String("eval.prompt", tc.Data.Prompt),
						attribute.String("eval.category", string(tc.Target.Category)),
						attribute.Bool("eval.passed", er.Passed),
						attribute.StringSlice("eval.tools_selected", result.ToolNames),
					}
					for k, v := range er.Scores {
						attrs = append(attrs, attribute.Float64("eval.score."+k, v))
					}
					span.SetAttributes(attrs...)
					if er.Passed {
						span.SetStatus(otelcodes.Ok, "")
					} else {
						span.SetStatus(otelcodes.Error, "eval failed")
					}
				}
			}
			ch <- indexedResult{idx: idx, res: er}
		}(i, tc)
	}

drain:
	for range cases {
		ir := <-ch
		results[ir.idx] = ir.res
	}
	return results
}

// PrintSummary writes a human-readable table of results to stdout.
func PrintSummary(results []EvalResult) {
	passed, total := 0, len(results)
	var sum float64

	for i, r := range results {
		mark := "✓"
		if r.Error != "" || !r.Passed {
			mark = "✗"
		} else {
			passed++
		}
		prompt := r.TestCase.Data.Prompt
		if len(prompt) > 60 {
			prompt = prompt[:57] + "..."
		}
		scores := ""
		for k, v := range r.Scores {
			scores += fmt.Sprintf(" %s=%.2f", k, v)
		}
		fmt.Printf("[%d] %s  cat=%-9s%s  prompt=%q\n",
			i+1, mark, r.TestCase.Target.Category, scores, prompt)
		if r.Error != "" {
			fmt.Printf("    error: %s\n", r.Error)
		}
		for _, v := range r.Scores {
			sum += v
		}
	}

	avg := 0.0
	if total > 0 {
		avg = sum / float64(total)
	}
	fmt.Printf("\n--- %d/%d passed  avg_score=%.2f ---\n", passed, total, avg)
}

// spanLabel returns a short span name from a prompt (≤ 60 chars).
func spanLabel(prompt string) string {
	if len(prompt) <= 60 {
		return "eval " + prompt
	}
	return "eval " + prompt[:57] + "..."
}

// LoadDataset reads test cases from a JSON file.
func LoadDataset(path string) ([]TestCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("singleturn: load dataset %q: %w", path, err)
	}
	var cases []TestCase
	if err := json.Unmarshal(b, &cases); err != nil {
		return nil, fmt.Errorf("singleturn: parse dataset %q: %w", path, err)
	}
	return cases, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func allPassed(scores map[string]float64) bool {
	for _, s := range scores {
		if s < 1.0 {
			return false
		}
	}
	return true
}

// toolDescription returns a generic description for common tool names.
// Used when no registry is available in single-turn eval mode.
func toolDescription(name string) string {
	descriptions := map[string]string{
		"read_file":    "Read the contents of a file at the specified path",
		"write_file":   "Write content to a file at the specified path",
		"list_files":   "List all files and directories in a directory",
		"delete_file":  "Delete a file at the specified path",
		"run_command":  "Execute a shell command and return its output",
		"execute_code": "Execute code in Python, Go, Bash, or JavaScript",
		"web_search":   "Search the web for current information",
	}
	if d, ok := descriptions[name]; ok {
		return d
	}
	return fmt.Sprintf("Tool: %s", name)
}
