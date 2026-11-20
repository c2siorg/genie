package multiturn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// ============================================================================
// PHASE 5: Multi-Turn Evaluation 100% Coverage — Error Paths & Edge Cases
// ============================================================================

// TestDefaultExecConfig tests defaultExecConfig
func TestDefaultExecConfig(t *testing.T) {
	config := defaultExecConfig()
	if config.MaxSteps <= 0 {
		t.Error("MaxSteps should be positive")
	}
	if config.Model == "" {
		t.Error("Model should not be empty")
	}
}

// TestExecConfigResolved tests ExecConfig.resolved()
func TestExecConfigResolved(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ExecConfig
	}{
		{"nil_config", nil},
		{"partial_config", &ExecConfig{Model: "custom"}},
		{"provider_openai", &ExecConfig{Provider: "openai"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.resolved()
			if got.Model == "" {
				t.Error("resolved config should have model")
			}
			if got.MaxSteps <= 0 {
				t.Error("resolved config should have max steps")
			}
		})
	}
}

// TestBuildTools_AllTools tests buildTools with multiple tools
func TestBuildTools_AllTools(t *testing.T) {
	mockTools := map[string]MockToolConfig{
		"add":      {Description: "add two numbers", Result: "5"},
		"subtract": {Description: "subtract numbers", Result: "1"},
	}

	builtTools := buildTools(mockTools)
	if len(builtTools) != len(mockTools) {
		t.Errorf("expected %d tools, got %d", len(mockTools), len(builtTools))
	}

	for _, tool := range builtTools {
		if tool.Type != "function" {
			t.Errorf("tool type should be 'function', got %q", tool.Type)
		}
		if tool.Function.Name == "" {
			t.Error("tool name should not be empty")
		}
	}
}

// TestBuildTools_EmptyList tests buildTools with empty list
func TestBuildTools_EmptyList(t *testing.T) {
	mockTools := map[string]MockToolConfig{}
	builtTools := buildTools(mockTools)
	if len(builtTools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(builtTools))
	}
}

// TestExecuteMock_Success tests executeMock with valid tool
func TestExecuteMock_Success(t *testing.T) {
	mockTools := map[string]MockToolConfig{
		"get": {Description: "get value", Result: "42"},
	}

	result := executeMock("get", mockTools)
	if result != "42" {
		t.Errorf("expected '42', got %q", result)
	}
}

// TestExecuteMock_Undefined tests executeMock with undefined tool
func TestExecuteMock_Undefined(t *testing.T) {
	mockTools := map[string]MockToolConfig{
		"get": {Description: "get value", Result: "42"},
	}

	result := executeMock("undefined", mockTools)
	if result == "42" {
		t.Error("should return error for undefined tool")
	}
	if !stringContains(result, "error") && !stringContains(result, "not found") {
		t.Errorf("error result should mention error: %q", result)
	}
}

// TestChatCompletions_Success tests chatCompletions with valid response
func TestChatCompletions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				FinishReason: "stop",
				Message: wireMessage{
					Role:    "assistant",
					Content: "final answer",
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := ExecConfig{
		Provider: "ollama",
		BaseURL:  server.URL,
		Model:    "test",
	}

	resp, err := chatCompletions(context.Background(), cfg, []wireMessage{}, []oaiTool{}, &http.Client{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Error("response should have choices")
	}
}

// TestChatCompletions_InvalidResponse tests chat completions with malformed JSON
func TestChatCompletions_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid}`))
	}))
	defer server.Close()

	cfg := ExecConfig{
		Provider: "ollama",
		BaseURL:  server.URL,
		Model:    "test",
	}

	_, err := chatCompletions(context.Background(), cfg, []wireMessage{}, []oaiTool{}, &http.Client{})
	if err == nil {
		t.Error("invalid response should cause error")
	}
}

// TestChatCompletions_Timeout tests chat completions with context timeout
func TestChatCompletions_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
	}))
	defer server.Close()

	cfg := ExecConfig{
		Provider: "ollama",
		BaseURL:  server.URL,
		Model:    "test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := chatCompletions(ctx, cfg, []wireMessage{}, []oaiTool{}, &http.Client{})
	if err == nil {
		t.Error("timeout should cause error")
	}
}

// TestExecutor_NewExecutor tests NewExecutor
func TestExecutor_NewExecutor(t *testing.T) {
	exec := NewExecutor()
	if exec == nil {
		t.Error("NewExecutor should return non-nil executor")
	}
	if exec.HTTPClient == nil {
		t.Error("executor should have default HTTP client")
	}
}

// TestExecutor_Run_NoChoices tests Run with empty choices response
func TestExecutor_Run_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{Choices: []oaiChoice{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exec := NewExecutor()
	exec.HTTPClient = &http.Client{}

	data := EvalData{
		Prompt: "test",
		MockTools: map[string]MockToolConfig{
			"test": {Description: "test", Result: "ok"},
		},
		Config: &ExecConfig{
			Provider: "ollama",
			BaseURL:  server.URL,
			Model:    "test",
		},
	}

	_, err := exec.Run(context.Background(), data)
	if err == nil {
		t.Error("should error on no choices")
	}
}

// TestExecutor_Run_Success tests Run with valid response
func TestExecutor_Run_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				FinishReason: "stop",
				Message: wireMessage{
					Role:    "assistant",
					Content: "done",
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	exec := NewExecutor()
	exec.HTTPClient = &http.Client{}

	data := EvalData{
		Prompt: "test",
		MockTools: map[string]MockToolConfig{
			"test": {Description: "test", Result: "ok"},
		},
		Config: &ExecConfig{
			Provider: "ollama",
			BaseURL:  server.URL,
			Model:    "test",
			MaxSteps: 1,
		},
	}

	result, err := exec.Run(context.Background(), data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Text == "" {
		t.Error("result should have text")
	}
}

// TestLoadDataset_FileNotFound tests LoadDataset with missing file
func TestLoadDataset_FileNotFound(t *testing.T) {
	_, err := LoadDataset("/nonexistent/path_" + fmt.Sprintf("%d", time.Now().Unix()) + ".json")
	if err == nil {
		t.Error("missing file should cause error")
	}
}

// TestLoadDataset_InvalidJSON tests LoadDataset with invalid JSON
func TestLoadDataset_InvalidJSON(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "invalid.json")
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString("{invalid json}")
	tmpfile.Close()

	_, err := LoadDataset(tmpfile.Name())
	if err == nil {
		t.Error("invalid JSON should cause error")
	}
}

// TestPrintSummary_EmptyResults tests PrintSummary with no results
func TestPrintSummary_EmptyResults(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintSummary([]EvalResult{})

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	if len(output) == 0 {
		t.Error("PrintSummary should produce output even for empty results")
	}
}

// TestPrintSummary_WithResults tests PrintSummary with results
func TestPrintSummary_WithResults(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	results := []EvalResult{
		{
			TestCase: TestCase{
				Data:   EvalData{Prompt: "test1"},
				Target: Target{OriginalTask: "task1"},
			},
			Scores: EvalScore{Overall: 0.8, ToolOrder: 0.8, ToolsAvoided: 0.8, OutputQuality: 0.8},
		},
	}
	PrintSummary(results)

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	if len(output) == 0 {
		t.Error("PrintSummary should produce output")
	}
}

// TestTruncate tests truncate utility function
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{"short", "hello", 10},
		{"exact", "hello", 5},
		{"truncate", "hello world", 8},
		{"empty", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if tt.maxLen > 0 && len(got) > tt.maxLen {
				t.Errorf("truncate result too long: got %d, max %d", len(got), tt.maxLen)
			}
		})
	}
}

// TestMax tests max utility function
func TestMax(t *testing.T) {
	tests := []struct {
		a, b int
		want int
	}{
		{5, 3, 5},
		{3, 5, 5},
		{5, 5, 5},
		{-5, 3, 3},
		{-5, -3, -3},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("max(%d,%d)", tt.a, tt.b), func(t *testing.T) {
			got := max(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// TestRunnerNewRunner_DefaultConfig tests NewRunner with zero config
func TestRunnerNewRunner_DefaultConfig(t *testing.T) {
	runner := NewRunner(RunnerConfig{})
	if runner == nil {
		t.Error("NewRunner should create runner")
	}
	if runner.cfg.Concurrency <= 0 {
		t.Error("NewRunner should set default concurrency")
	}
}

// TestRunnerNewRunner_CustomConfig tests NewRunner with custom config
func TestRunnerNewRunner_CustomConfig(t *testing.T) {
	cfg := RunnerConfig{Concurrency: 8}
	runner := NewRunner(cfg)
	if runner == nil {
		t.Error("NewRunner should create runner")
	}
	if runner.cfg.Concurrency != 8 {
		t.Error("NewRunner should preserve custom concurrency")
	}
}

// TestRunDataset_EmptyDataset tests RunDataset with empty dataset
func TestRunDataset_EmptyDataset(t *testing.T) {
	runner := NewRunner(RunnerConfig{})
	results := runner.RunDataset(context.Background(), []TestCase{})

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestToolOrderCorrect_EmptyBoth tests ToolOrderCorrect with empty arrays
func TestToolOrderCorrect_EmptyBoth(t *testing.T) {
	result := Result{ToolCallOrder: []string{}}
	target := Target{ExpectedToolOrder: []string{}}
	score := ToolOrderCorrect(result, target)
	if score != 1.0 {
		t.Errorf("empty arrays should match: got %f", score)
	}
}

// TestToolsAvoided_AllAvoided tests ToolsAvoided when all forbidden tools are avoided
func TestToolsAvoided_AllAvoided(t *testing.T) {
	result := Result{ToolCallOrder: []string{"readFile", "writeFile"}}
	target := Target{ForbiddenTools: []string{"deleteFile", "formatDisk"}}
	score := ToolsAvoided(result, target)
	if score != 1.0 {
		t.Errorf("should avoid all forbidden tools: got %f", score)
	}
}

// TestToolsAvoided_SomeForbidden tests ToolsAvoided when forbidden tools are used
func TestToolsAvoided_SomeForbidden(t *testing.T) {
	result := Result{ToolCallOrder: []string{"readFile", "deleteFile"}}
	target := Target{ForbiddenTools: []string{"deleteFile", "formatDisk"}}
	score := ToolsAvoided(result, target)
	if score >= 1.0 {
		t.Errorf("should penalize forbidden tools: got %f", score)
	}
}

// stringContains checks if needle is in haystack
func stringContains(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestJudgeConfigResolved tests JudgeConfig.resolved()
func TestJudgeConfigResolved(t *testing.T) {
	tests := []struct {
		name string
		cfg  JudgeConfig
		want JudgeConfig
	}{
		{
			"zero_value",
			JudgeConfig{},
			JudgeConfig{Provider: "ollama", BaseURL: "http://localhost:11434", Model: "llama3.1"},
		},
		{
			"custom_provider_openai",
			JudgeConfig{Provider: "openai"},
			JudgeConfig{Provider: "openai", BaseURL: "https://api.openai.com", Model: "gpt-4.1"},
		},
		{
			"custom_all",
			JudgeConfig{Provider: "ollama", BaseURL: "http://custom:8000", Model: "custom-model"},
			JudgeConfig{Provider: "ollama", BaseURL: "http://custom:8000", Model: "custom-model"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.resolved()
			if got.Provider != tt.want.Provider {
				t.Errorf("provider: got %q, want %q", got.Provider, tt.want.Provider)
			}
			if got.BaseURL != tt.want.BaseURL {
				t.Errorf("baseURL: got %q, want %q", got.BaseURL, tt.want.BaseURL)
			}
			if got.Model != tt.want.Model {
				t.Errorf("model: got %q, want %q", got.Model, tt.want.Model)
			}
		})
	}
}

// TestLLMJudge_ValidScore tests LLMJudge with valid JSON response
func TestLLMJudge_ValidScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				Message: wireMessage{
					Role:    "assistant",
					Content: `{"score": 8, "reason": "good response"}`,
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := Result{
		Text:          "answer",
		ToolCallOrder: []string{"tool1"},
	}
	target := Target{
		OriginalTask:    "find something",
		MockToolResults: map[string]string{"tool1": "result1"},
	}
	cfg := JudgeConfig{Provider: "ollama", BaseURL: server.URL, Model: "test"}

	score, judgeResult, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.8 {
		t.Errorf("expected 0.8, got %f", score)
	}
	if judgeResult.Score != 8 {
		t.Errorf("expected score 8, got %d", judgeResult.Score)
	}
}

// TestLLMJudge_OutOfBoundsScore tests LLMJudge with out-of-bounds score
func TestLLMJudge_OutOfBoundsScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				Message: wireMessage{
					Role:    "assistant",
					Content: `{"score": 15, "reason": "over max"}`,
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := Result{Text: "answer", ToolCallOrder: []string{}}
	target := Target{OriginalTask: "test"}
	cfg := JudgeConfig{Provider: "ollama", BaseURL: server.URL, Model: "test"}

	score, judgeResult, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1.0 {
		t.Errorf("should clamp to max: got %f", score)
	}
	if judgeResult.Score != 10 {
		t.Errorf("should clamp score to 10, got %d", judgeResult.Score)
	}
}

// TestLLMJudge_LowScore tests LLMJudge with score below 1
func TestLLMJudge_LowScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				Message: wireMessage{
					Role:    "assistant",
					Content: `{"score": 0, "reason": "bad"}`,
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := Result{Text: "answer", ToolCallOrder: []string{}}
	target := Target{OriginalTask: "test"}
	cfg := JudgeConfig{Provider: "ollama", BaseURL: server.URL, Model: "test"}

	_, judgeResult, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if judgeResult.Score != 1 {
		t.Errorf("should clamp to min 1, got %d", judgeResult.Score)
	}
}

// TestLLMJudge_JSONInMarkdown tests LLMJudge with JSON in markdown
func TestLLMJudge_JSONInMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				Message: wireMessage{
					Role:    "assistant",
					Content: "Here's the score:\n```json\n{\"score\": 7, \"reason\": \"ok\"}\n```",
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := Result{Text: "answer", ToolCallOrder: []string{}}
	target := Target{OriginalTask: "test"}
	cfg := JudgeConfig{Provider: "ollama", BaseURL: server.URL, Model: "test"}

	score, judgeResult, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.7 {
		t.Errorf("should extract JSON from markdown: got %f", score)
	}
	if judgeResult.Score != 7 {
		t.Errorf("expected score 7, got %d", judgeResult.Score)
	}
}

// TestLLMJudge_InvalidJSON tests LLMJudge with invalid JSON fallback
func TestLLMJudge_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := oaiResponse{
			Choices: []oaiChoice{{
				Message: wireMessage{
					Role:    "assistant",
					Content: "not json at all",
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := Result{Text: "answer", ToolCallOrder: []string{}}
	target := Target{OriginalTask: "test"}
	cfg := JudgeConfig{Provider: "ollama", BaseURL: server.URL, Model: "test"}

	score, judgeResult, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 0.5 {
		t.Errorf("should return fallback score 0.5: got %f", score)
	}
	if judgeResult.Score != 5 {
		t.Errorf("should return fallback score 5, got %d", judgeResult.Score)
	}
}

// TestToolOrderCorrect_ExtraTools tests when agent calls extra tools
func TestToolOrderCorrect_ExtraTools(t *testing.T) {
	result := Result{ToolCallOrder: []string{"tool1", "tool2", "tool3"}}
	target := Target{ExpectedToolOrder: []string{"tool1", "tool2"}}
	score := ToolOrderCorrect(result, target)
	if score != 1.0 {
		t.Errorf("extra tools after sequence should still match: got %f", score)
	}
}

// TestPrintSummary_WithErrors tests PrintSummary with error results
func TestPrintSummary_WithErrors(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	results := []EvalResult{
		{
			TestCase: TestCase{
				Data:   EvalData{Prompt: "test"},
				Target: Target{OriginalTask: "task"},
			},
			Scores: EvalScore{Overall: 0.4, ToolOrder: 0.3},
			Err:    "test error",
		},
	}
	PrintSummary(results)

	w.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)
	if len(output) == 0 {
		t.Error("PrintSummary should include error output")
	}
	// Check that error is printed
	outputStr := string(output)
	if !stringContains(outputStr, "error") {
		t.Error("output should contain error message")
	}
}
