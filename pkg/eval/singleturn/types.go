// Package singleturn implements single-turn tool-selection evaluations (lesson 03).
//
// Single-turn evals test one interaction: given a user prompt and a set of
// available tools, did the agent select the right tool(s)?
//
// Three eval categories (from lesson 03):
//   - golden   — must select exactly the expected tools (binary pass/fail)
//   - secondary — partial credit via F1 score (precision/recall)
//   - negative  — must NOT select any forbidden tools
//
// Hill climbing: run evals → get baseline → change prompt/model → run again →
// keep the change if scores improved. Every change is justified by data.
package singleturn

// Category classifies a test case.
type Category string

const (
	CategoryGolden    Category = "golden"
	CategorySecondary Category = "secondary"
	CategoryNegative  Category = "negative"
)

// EvalData is the input to the executor. Mirrors the TypeScript EvalData type.
type EvalData struct {
	// Prompt is the user message. Used when Messages is empty (fresh task).
	Prompt string `json:"prompt"`
	// Tools is the list of tool names available to the agent.
	Tools []string `json:"tools"`
	// Config overrides model/temperature per test case.
	Config *EvalConfig `json:"config,omitempty"`
}

// EvalConfig overrides model or temperature for a specific test case.
type EvalConfig struct {
	Model       string  `json:"model,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// EvalTarget describes what the agent should do.
type EvalTarget struct {
	// ExpectedTools is the list of tools that must all be selected (golden).
	ExpectedTools []string `json:"expected_tools,omitempty"`
	// ForbiddenTools is the list of tools that must not be selected (negative).
	ForbiddenTools []string `json:"forbidden_tools,omitempty"`
	// Category classifies the test case.
	Category Category `json:"category"`
}

// SingleTurnResult is the executor's output for one test case.
type SingleTurnResult struct {
	// ToolCalls is the list of tool calls made by the agent.
	ToolCalls []ToolCall `json:"tool_calls"`
	// ToolNames is a convenience slice of just the tool names.
	ToolNames []string `json:"tool_names"`
	// SelectedAny is true when at least one tool was called.
	SelectedAny bool `json:"selected_any"`
}

// ToolCall holds one tool invocation.
type ToolCall struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args,omitempty"`
}

// TestCase combines input and expected output for dataset loading.
type TestCase struct {
	Data     EvalData   `json:"data"`
	Target   EvalTarget `json:"target"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EvalResult is returned by the Runner for one test case.
type EvalResult struct {
	TestCase TestCase
	Result   SingleTurnResult
	Scores   map[string]float64
	Passed   bool
	Error    string
}
