// Package multiturn implements multi-turn agent evaluation.
//
// Multi-turn evals test the full agent loop: given a task, does the agent
// complete it correctly across multiple tool calls and reasoning steps?
//
// # Why multi-turn evals
//
// Single-turn evals (did the model pick the right tool?) miss failures that
// only appear across a conversation:
//   - Agent picks the right first tool but wrong second tool
//   - Agent gets stuck in loops (caught by maxSteps)
//   - Agent misinterprets tool results and takes the wrong next action
//   - Agent doesn't know when to stop
//
// # Architecture
//
// Each test case is a (EvalData, Target) pair stored in a JSON dataset.
// The Executor runs the agent loop with mocked tools so results are
// deterministic and there are no filesystem/network side effects.
// Three Evaluator functions score the Result:
//
//  1. ToolOrderCorrect — did the tools fire in the expected sequence?
//  2. ToolsAvoided     — were forbidden tools skipped entirely?
//  3. LLMJudge         — does the final response make sense? (LLM-as-judge)
//
// The Runner ties them together over a full dataset and emits EvalResult
// summaries with per-evaluator scores.
//
// # Ollama default
//
// All LLM calls (agent loop + judge) default to Ollama at
// http://localhost:11434 with model "llama3.1" so the entire eval harness
// runs locally without an API key.
package multiturn

// MockToolConfig defines a mock tool's description and its canned response.
// Tools return Result verbatim regardless of what arguments the model passes.
type MockToolConfig struct {
	Description string `json:"description"`
	Result      string `json:"result"`
}

// wireMessage is the OpenAI-format chat message used on the wire.
// Using a separate wire type keeps the public API clean while matching
// exactly what Ollama / OpenAI expect.
type wireMessage struct {
	Role       string       `json:"role"`
	Content    any          `json:"content"` // string or null for tool-call turns
	ToolCalls  []WireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	Name       string       `json:"name,omitempty"`
}

// WireToolCall is the OpenAI function-call shape inside a model response.
type WireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // always "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON-encoded args
	} `json:"function"`
}

// Message is a public conversation turn. It supports both plain text turns
// and tool-call turns. Use this in EvalData.Messages for pre-filled history.
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
	Content string `json:"content"`
}

// ToolCallRecord captures one tool invocation in the step trace.
type ToolCallRecord struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args"`
}

// ToolResultRecord captures the result of one tool invocation in the step trace.
type ToolResultRecord struct {
	ToolName string `json:"tool_name"`
	Result   string `json:"result"`
}

// Step represents one reasoning turn in the agent loop.
// A step may contain tool calls (and their results) followed by optional text.
type Step struct {
	ToolCalls   []ToolCallRecord   `json:"tool_calls,omitempty"`
	ToolResults []ToolResultRecord `json:"tool_results,omitempty"`
	Text        string             `json:"text,omitempty"`
}

// ExecConfig controls agent-loop behaviour for one test case.
// All fields are optional; zero values fall back to sensible defaults.
type ExecConfig struct {
	// Model to use (default: "llama3.1").
	Model string `json:"model,omitempty"`
	// MaxSteps caps the agent loop to prevent infinite tool-call cycles (default: 20).
	MaxSteps int `json:"maxSteps,omitempty"`
	// Provider selects the backend: "ollama" (default), "openai", or "anthropic".
	Provider string `json:"provider,omitempty"`
	// BaseURL overrides the default endpoint.
	// Defaults to "http://localhost:11434" for Ollama, "https://api.openai.com" for OpenAI.
	BaseURL string `json:"baseUrl,omitempty"`
	// APIKey is required for non-Ollama providers.
	APIKey string `json:"apiKey,omitempty"`
}

// EvalData is the input half of a test case.
type EvalData struct {
	// Prompt is a fresh user task. Used when Messages is empty.
	Prompt string `json:"prompt,omitempty"`
	// Messages is a pre-filled conversation history (mid-conversation tests).
	Messages []Message `json:"messages,omitempty"`
	// MockTools maps tool name → mock config. Tools return their Result regardless of args.
	MockTools map[string]MockToolConfig `json:"mockTools"`
	// Config controls execution behaviour. nil means use DefaultExecConfig.
	Config *ExecConfig `json:"config,omitempty"`
}

// Target is the expected-outcome half of a test case.
type Target struct {
	// ExpectedToolOrder lists the exact tool call sequence required.
	// An empty slice means "any order is fine".
	ExpectedToolOrder []string `json:"expectedToolOrder,omitempty"`
	// ForbiddenTools lists tools that must NOT be called at all.
	ForbiddenTools []string `json:"forbiddenTools,omitempty"`
	// OriginalTask is the natural-language task description passed to the LLM judge.
	OriginalTask string `json:"originalTask,omitempty"`
	// MockToolResults is a copy of the tool results provided to the judge for context.
	// Typically mirrors what MockTools return so the judge can verify the agent used them.
	MockToolResults map[string]string `json:"mockToolResults,omitempty"`
}

// TestCase is one entry in a dataset JSON file.
type TestCase struct {
	Data   EvalData `json:"data"`
	Target Target   `json:"target"`
}

// Result is returned by the Executor after a complete agent run.
type Result struct {
	// Text is the agent's final natural-language response.
	Text string `json:"text"`
	// Steps is the full reasoning trace (tool calls + results per turn).
	Steps []Step `json:"steps"`
	// ToolsUsed is the deduplicated set of tools invoked.
	ToolsUsed []string `json:"tools_used"`
	// ToolCallOrder is the full ordered list of tool names called (with repeats).
	ToolCallOrder []string `json:"tool_call_order"`
}

// JudgeResult holds the structured output from the LLM judge.
type JudgeResult struct {
	// Score is 1–10; 10 means the response perfectly addressed the task.
	Score int `json:"score"`
	// Reason briefly explains the score.
	Reason string `json:"reason"`
}

// EvalScore holds the per-evaluator scores for one test case.
// All scores are normalised to [0, 1].
type EvalScore struct {
	ToolOrder     float64 `json:"tool_order"`
	ToolsAvoided  float64 `json:"tools_avoided"`
	OutputQuality float64 `json:"output_quality"`
	// Overall is the unweighted mean of the three scores.
	Overall float64 `json:"overall"`
}

// EvalResult bundles a test case with its execution result and scores.
type EvalResult struct {
	TestCase TestCase  `json:"test_case"`
	Result   Result    `json:"result"`
	Scores   EvalScore `json:"scores"`
	Err      string    `json:"error,omitempty"`
}

// defaultExecConfig returns the base config used when EvalData.Config is nil.
func defaultExecConfig() ExecConfig {
	return ExecConfig{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434",
		Model:    "llama3.1",
		MaxSteps: 20,
	}
}

// resolved merges caller config on top of defaults.
func (c *ExecConfig) resolved() ExecConfig {
	d := defaultExecConfig()
	if c == nil {
		return d
	}
	out := *c
	if out.Provider == "" {
		out.Provider = d.Provider
	}
	if out.BaseURL == "" {
		switch out.Provider {
		case "openai":
			out.BaseURL = "https://api.openai.com"
		default:
			out.BaseURL = d.BaseURL
		}
	}
	if out.Model == "" {
		out.Model = d.Model
	}
	if out.MaxSteps <= 0 {
		out.MaxSteps = d.MaxSteps
	}
	return out
}
