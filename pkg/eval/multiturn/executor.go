package multiturn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// systemPrompt is prepended to fresh-task conversations when no messages
// are provided. Keep it minimal so the model focuses on tool use.
const systemPrompt = `You are a helpful assistant with access to tools.
Use tools when needed to answer the user's request. When you have gathered
enough information, provide a concise final answer.`

// -------------------------------------------------------------------
// Wire types (OpenAI-compatible tool-calling format, also used by Ollama)
// -------------------------------------------------------------------

type oaiToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type oaiTool struct {
	Type     string          `json:"type"` // "function"
	Function oaiToolFunction `json:"function"`
}

type oaiRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Tools    []oaiTool     `json:"tools,omitempty"`
}

type oaiChoice struct {
	Message      wireMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// -------------------------------------------------------------------
// Tool schema helpers
// -------------------------------------------------------------------

// buildTools converts a MockTools map into the OpenAI tool-definition list.
// Every mock tool accepts a single optional "args" object so the model can
// pass arguments without triggering schema validation failures.
func buildTools(mockTools map[string]MockToolConfig) []oaiTool {
	tools := make([]oaiTool, 0, len(mockTools))
	for name, cfg := range mockTools {
		tools = append(tools, oaiTool{
			Type: "function",
			Function: oaiToolFunction{
				Name:        name,
				Description: cfg.Description,
				Parameters: map[string]any{
					"type":                 "object",
					"properties":           map[string]any{},
					"additionalProperties": true,
				},
			},
		})
	}
	return tools
}

// executeMock returns the canned result for a tool call. If the tool is not
// in the mock registry it returns an error string so the model can recover.
func executeMock(toolName string, mockTools map[string]MockToolConfig) string {
	cfg, ok := mockTools[toolName]
	if !ok {
		return fmt.Sprintf("error: tool %q not found in mock registry", toolName)
	}
	return cfg.Result
}

// -------------------------------------------------------------------
// HTTP helper
// -------------------------------------------------------------------

// chatCompletions calls the OpenAI-compatible /v1/chat/completions endpoint.
func chatCompletions(
	ctx context.Context,
	cfg ExecConfig,
	msgs []wireMessage,
	tools []oaiTool,
	client *http.Client,
) (oaiResponse, error) {
	body := oaiRequest{
		Model:    cfg.Model,
		Messages: msgs,
		Tools:    tools,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return oaiResponse{}, err
	}

	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return oaiResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return oaiResponse{}, fmt.Errorf("chatCompletions request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return oaiResponse{}, fmt.Errorf("chatCompletions http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out oaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return oaiResponse{}, fmt.Errorf("chatCompletions decode: %w", err)
	}
	return out, nil
}

// -------------------------------------------------------------------
// Executor
// -------------------------------------------------------------------

// Executor runs the agent loop for one EvalData test case.
type Executor struct {
	// HTTPClient is used for all LLM calls. Defaults to a 60 s timeout client.
	HTTPClient *http.Client
}

// NewExecutor creates an Executor with a sensible default HTTP client.
func NewExecutor() *Executor {
	return &Executor{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Run executes the agent loop for one test case and returns a Result.
//
// Algorithm:
//  1. Resolve config (provider/model/URL defaults → Ollama).
//  2. Build initial message list (system + user, or use pre-filled history).
//  3. Loop (up to maxSteps):
//     a. Call LLM with current messages + tool definitions.
//     b. If finish_reason == "tool_calls": execute mocks, append results, continue.
//     c. Otherwise: capture final text and stop.
//  4. Return Result with full trace.
func (e *Executor) Run(ctx context.Context, data EvalData) (Result, error) {
	cfg := data.Config.resolved()
	client := e.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	tools := buildTools(data.MockTools)

	// Build initial message history.
	var msgs []wireMessage
	if len(data.Messages) > 0 {
		// Pre-filled history (mid-conversation test).
		for _, m := range data.Messages {
			msgs = append(msgs, wireMessage{Role: m.Role, Content: m.Content})
		}
	} else {
		// Fresh task.
		msgs = []wireMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: data.Prompt},
		}
	}

	var (
		steps         []Step
		allToolCalls  []string
		finalText     string
	)

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 20
	}

	for step := 0; step < maxSteps; step++ {
		resp, err := chatCompletions(ctx, cfg, msgs, tools, client)
		if err != nil {
			return Result{}, fmt.Errorf("step %d: %w", step, err)
		}
		if len(resp.Choices) == 0 {
			return Result{}, fmt.Errorf("step %d: no choices in response", step)
		}

		choice := resp.Choices[0]
		assistantMsg := choice.Message

		// Normalise content: some providers return null for tool-call turns.
		if assistantMsg.Content == nil {
			assistantMsg.Content = ""
		}

		// Always append the assistant turn to message history.
		msgs = append(msgs, assistantMsg)

		if choice.FinishReason == "tool_calls" || len(assistantMsg.ToolCalls) > 0 {
			// Execute each mocked tool and collect results.
			s := Step{}
			for _, tc := range assistantMsg.ToolCalls {
				// Parse args best-effort (mocks ignore them anyway).
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

				s.ToolCalls = append(s.ToolCalls, ToolCallRecord{
					ToolName: tc.Function.Name,
					Args:     args,
				})

				result := executeMock(tc.Function.Name, data.MockTools)
				s.ToolResults = append(s.ToolResults, ToolResultRecord{
					ToolName: tc.Function.Name,
					Result:   result,
				})
				allToolCalls = append(allToolCalls, tc.Function.Name)

				// Append tool result message in OpenAI format.
				msgs = append(msgs, wireMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}
			steps = append(steps, s)
			// Continue the loop so the model can reason over the results.
			continue
		}

		// No tool calls → final response.
		if content, ok := assistantMsg.Content.(string); ok {
			finalText = content
		}
		if finalText != "" {
			steps = append(steps, Step{Text: finalText})
		}
		break
	}

	// Deduplicate tools used, preserving first-seen order.
	seen := make(map[string]bool, len(allToolCalls))
	toolsUsed := make([]string, 0, len(allToolCalls))
	for _, t := range allToolCalls {
		if !seen[t] {
			seen[t] = true
			toolsUsed = append(toolsUsed, t)
		}
	}

	return Result{
		Text:          finalText,
		Steps:         steps,
		ToolsUsed:     toolsUsed,
		ToolCallOrder: allToolCalls,
	}, nil
}

// MultiTurnWithMocks is a convenience wrapper around a default Executor.
func MultiTurnWithMocks(ctx context.Context, data EvalData) (Result, error) {
	return NewExecutor().Run(ctx, data)
}
