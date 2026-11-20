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

// -------------------------------------------------------------------
// Deterministic evaluators
// -------------------------------------------------------------------

// ToolOrderCorrect checks whether the actual tool call sequence matches the
// expected order declared in Target.ExpectedToolOrder.
//
// Returns:
//   - 1.0 if ExpectedToolOrder is empty (no constraint) or if it matches exactly.
//   - 0.0 if the order does not match.
//
// Rationale: a partial match score could mask real ordering bugs. A strict
// binary score forces the test author to declare what they actually need.
func ToolOrderCorrect(result Result, target Target) float64 {
	if len(target.ExpectedToolOrder) == 0 {
		return 1.0
	}
	if len(result.ToolCallOrder) < len(target.ExpectedToolOrder) {
		return 0.0
	}
	// Compare prefix — the agent may call additional tools after the required
	// sequence, which is fine.
	for i, expected := range target.ExpectedToolOrder {
		if result.ToolCallOrder[i] != expected {
			return 0.0
		}
	}
	return 1.0
}

// ToolsAvoided checks that none of the forbidden tools were called.
//
// Returns:
//   - 1.0 if ForbiddenTools is empty or none of them appear in the call order.
//   - 0.0 if any forbidden tool was called.
func ToolsAvoided(result Result, target Target) float64 {
	if len(target.ForbiddenTools) == 0 {
		return 1.0
	}
	forbidden := make(map[string]bool, len(target.ForbiddenTools))
	for _, t := range target.ForbiddenTools {
		forbidden[t] = true
	}
	for _, called := range result.ToolCallOrder {
		if forbidden[called] {
			return 0.0
		}
	}
	return 1.0
}

// -------------------------------------------------------------------
// LLM-as-judge evaluator
// -------------------------------------------------------------------

// judgeSystemPrompt instructs the judge to return structured JSON.
const judgeSystemPrompt = `You are an evaluation judge for AI agents.
Your job is to assess whether an agent's final response correctly and helpfully
addresses the given task, given the tool results that were available.

Respond ONLY with a JSON object in this exact format (no markdown, no extra text):
{"score": <integer 1-10>, "reason": "<brief explanation>"}

Scoring rubric:
  10 : Response fully addresses the task using the tool results correctly.
  7-9: Response is mostly correct with minor gaps or unnecessary content.
  4-6: Response partially addresses the task or misuses tool results.
  1-3: Response is mostly incorrect, irrelevant, or ignores tool results.`

// judgeUserPrompt formats the context the judge receives.
func judgeUserPrompt(result Result, target Target) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Task: %s\n\n", target.OriginalTask)
	fmt.Fprintf(&sb, "Tools called (in order): %s\n\n",
		strings.Join(result.ToolCallOrder, " → "))
	if len(target.MockToolResults) > 0 {
		sb.WriteString("Tool results provided:\n")
		for name, res := range target.MockToolResults {
			fmt.Fprintf(&sb, "  %s → %s\n", name, res)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "Agent's final response:\n%s\n\nEvaluate the response.", result.Text)
	return sb.String()
}

// JudgeConfig controls the LLM judge backend.
// Defaults to the same Ollama defaults as the executor (llama3.1 @ localhost:11434).
type JudgeConfig struct {
	Provider string // "ollama" (default) | "openai"
	BaseURL  string
	Model    string
	APIKey   string
}

func (jc JudgeConfig) resolved() JudgeConfig {
	if jc.Provider == "" {
		jc.Provider = "ollama"
	}
	if jc.BaseURL == "" {
		switch jc.Provider {
		case "openai":
			jc.BaseURL = "https://api.openai.com"
		default:
			jc.BaseURL = "http://localhost:11434"
		}
	}
	if jc.Model == "" {
		switch jc.Provider {
		case "openai":
			jc.Model = "gpt-4.1"
		default:
			jc.Model = "llama3.1"
		}
	}
	return jc
}

// LLMJudge asks an LLM to evaluate Result against Target and returns a
// normalised score in [0, 1].
//
// It uses a structured-output prompt that asks the model to return JSON so
// the score can be parsed reliably even without provider-level schema support.
//
// The judge defaults to Ollama (llama3.1) when cfg is zero-value.
func LLMJudge(ctx context.Context, result Result, target Target, cfg JudgeConfig) (float64, JudgeResult, error) {
	cfg = cfg.resolved()
	client := &http.Client{Timeout: 60 * time.Second}

	msgs := []wireMessage{
		{Role: "system", Content: judgeSystemPrompt},
		{Role: "user", Content: judgeUserPrompt(result, target)},
	}

	reqBody := oaiRequest{
		Model:    cfg.Model,
		Messages: msgs,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return 0, JudgeResult{}, err
	}

	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return 0, JudgeResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, JudgeResult{}, fmt.Errorf("llmJudge request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, JudgeResult{}, fmt.Errorf("llmJudge http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var oaiResp oaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return 0, JudgeResult{}, fmt.Errorf("llmJudge decode: %w", err)
	}
	if len(oaiResp.Choices) == 0 {
		return 0, JudgeResult{}, fmt.Errorf("llmJudge: no choices in response")
	}

	// Extract text content.
	raw, err = io.ReadAll(strings.NewReader(""))
	_ = raw
	contentAny := oaiResp.Choices[0].Message.Content
	var content string
	switch v := contentAny.(type) {
	case string:
		content = v
	default:
		b, _ := json.Marshal(contentAny)
		content = string(b)
	}

	// Parse JSON score from model response.
	// The model might wrap it in markdown fences — strip them.
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx > 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "}"); idx >= 0 && idx < len(content)-1 {
		content = content[:idx+1]
	}

	var judgeOut JudgeResult
	if err := json.Unmarshal([]byte(content), &judgeOut); err != nil {
		// Fallback: if parsing fails, return a mid score with the raw text as reason.
		return 0.5, JudgeResult{Score: 5, Reason: content}, nil
	}
	if judgeOut.Score < 1 {
		judgeOut.Score = 1
	}
	if judgeOut.Score > 10 {
		judgeOut.Score = 10
	}

	// Normalise 1-10 → 0-1.
	return float64(judgeOut.Score) / 10.0, judgeOut, nil
}
