package multiturn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

// oaiFixture returns a mock HTTP server that returns OpenAI-compatible
// responses from a queue. Each request consumes one response in order;
// the last response repeats if the queue is exhausted.
func oaiFixture(t *testing.T, responses []oaiResponse) *httptest.Server {
	t.Helper()
	idx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp := responses[idx]
		if idx < len(responses)-1 {
			idx++
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// toolCallResponse builds an oaiResponse that instructs the model to call a tool.
func toolCallResponse(id, toolName, args string) oaiResponse {
	return oaiResponse{
		Choices: []oaiChoice{
			{
				FinishReason: "tool_calls",
				Message: wireMessage{
					Role:    "assistant",
					Content: nil,
					ToolCalls: []WireToolCall{
						{
							ID:   id,
							Type: "function",
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{Name: toolName, Arguments: args},
						},
					},
				},
			},
		},
	}
}

// textResponse builds an oaiResponse with a plain text answer.
func textResponse(text string) oaiResponse {
	return oaiResponse{
		Choices: []oaiChoice{
			{
				FinishReason: "stop",
				Message: wireMessage{
					Role:    "assistant",
					Content: text,
				},
			},
		},
	}
}

// -------------------------------------------------------------------
// Evaluator unit tests — no HTTP, fully deterministic
// -------------------------------------------------------------------

func TestToolOrderCorrect_Match(t *testing.T) {
	result := Result{ToolCallOrder: []string{"readFile", "writeFile"}}
	target := Target{ExpectedToolOrder: []string{"readFile", "writeFile"}}
	if got := ToolOrderCorrect(result, target); got != 1.0 {
		t.Errorf("expected 1.0, got %f", got)
	}
}

func TestToolOrderCorrect_Mismatch(t *testing.T) {
	result := Result{ToolCallOrder: []string{"writeFile", "readFile"}}
	target := Target{ExpectedToolOrder: []string{"readFile", "writeFile"}}
	if got := ToolOrderCorrect(result, target); got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestToolOrderCorrect_EmptyTarget(t *testing.T) {
	result := Result{ToolCallOrder: []string{"readFile"}}
	target := Target{}
	if got := ToolOrderCorrect(result, target); got != 1.0 {
		t.Errorf("expected 1.0 when no constraint, got %f", got)
	}
}

func TestToolOrderCorrect_PrefixMatch(t *testing.T) {
	// Agent calls the required sequence plus an extra tool — still pass.
	result := Result{ToolCallOrder: []string{"readFile", "writeFile", "notifyUser"}}
	target := Target{ExpectedToolOrder: []string{"readFile", "writeFile"}}
	if got := ToolOrderCorrect(result, target); got != 1.0 {
		t.Errorf("expected 1.0 for prefix match, got %f", got)
	}
}

func TestToolsAvoided_Pass(t *testing.T) {
	result := Result{ToolCallOrder: []string{"readFile"}}
	target := Target{ForbiddenTools: []string{"deleteFile", "runCommand"}}
	if got := ToolsAvoided(result, target); got != 1.0 {
		t.Errorf("expected 1.0, got %f", got)
	}
}

func TestToolsAvoided_Fail(t *testing.T) {
	result := Result{ToolCallOrder: []string{"readFile", "deleteFile"}}
	target := Target{ForbiddenTools: []string{"deleteFile"}}
	if got := ToolsAvoided(result, target); got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestToolsAvoided_EmptyForbidden(t *testing.T) {
	result := Result{ToolCallOrder: []string{"anything"}}
	target := Target{}
	if got := ToolsAvoided(result, target); got != 1.0 {
		t.Errorf("expected 1.0 when no forbidden list, got %f", got)
	}
}

// -------------------------------------------------------------------
// Executor integration tests with mock HTTP server
// -------------------------------------------------------------------

func TestExecutor_FreshTask_SingleTool(t *testing.T) {
	// Simulate: model calls readFile → gets result → returns final text.
	srv := oaiFixture(t, []oaiResponse{
		toolCallResponse("call_1", "readFile", `{"path":"config.json"}`),
		textResponse(`The API endpoint is https://api.example.com/v1`),
	})
	defer srv.Close()

	data := EvalData{
		Prompt: "Read the config.json file and tell me the API endpoint",
		MockTools: map[string]MockToolConfig{
			"readFile": {
				Description: "Read a file",
				Result:      `{"apiEndpoint":"https://api.example.com/v1"}`,
			},
		},
		Config: &ExecConfig{
			Provider: "ollama",
			BaseURL:  srv.URL,
			Model:    "llama3.1",
			MaxSteps: 10,
		},
	}

	result, err := NewExecutor().Run(context.Background(), data)
	if err != nil {
		t.Fatalf("executor error: %v", err)
	}

	if result.Text == "" {
		t.Error("expected non-empty final text")
	}
	if len(result.ToolCallOrder) != 1 || result.ToolCallOrder[0] != "readFile" {
		t.Errorf("expected [readFile], got %v", result.ToolCallOrder)
	}
	if len(result.ToolsUsed) != 1 {
		t.Errorf("expected 1 unique tool, got %d", len(result.ToolsUsed))
	}
	if len(result.Steps) != 2 { // one tool step + one final-text step
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
}

func TestExecutor_TwoToolSequence(t *testing.T) {
	srv := oaiFixture(t, []oaiResponse{
		toolCallResponse("call_1", "readFile", `{}`),
		toolCallResponse("call_2", "writeFile", `{}`),
		textResponse("Done. I read the file and wrote the updated version."),
	})
	defer srv.Close()

	data := EvalData{
		Prompt: "Read config.json then write an updated version",
		MockTools: map[string]MockToolConfig{
			"readFile":  {Description: "Read", Result: `{"port":8080}`},
			"writeFile": {Description: "Write", Result: "ok"},
		},
		Config: &ExecConfig{BaseURL: srv.URL, Model: "llama3.1", MaxSteps: 10},
	}

	result, err := NewExecutor().Run(context.Background(), data)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}

	want := []string{"readFile", "writeFile"}
	if len(result.ToolCallOrder) != 2 {
		t.Fatalf("expected 2 tool calls, got %v", result.ToolCallOrder)
	}
	for i, w := range want {
		if result.ToolCallOrder[i] != w {
			t.Errorf("call[%d]: want %s got %s", i, w, result.ToolCallOrder[i])
		}
	}
}

func TestExecutor_NoTools_DirectAnswer(t *testing.T) {
	srv := oaiFixture(t, []oaiResponse{
		textResponse("2 + 2 = 4"),
	})
	defer srv.Close()

	data := EvalData{
		Prompt: "What is 2 + 2?",
		MockTools: map[string]MockToolConfig{
			"readFile":   {Description: "Read", Result: ""},
			"runCommand": {Description: "Run", Result: ""},
		},
		Config: &ExecConfig{BaseURL: srv.URL, Model: "llama3.1", MaxSteps: 5},
	}

	result, err := NewExecutor().Run(context.Background(), data)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if len(result.ToolCallOrder) != 0 {
		t.Errorf("expected no tool calls, got %v", result.ToolCallOrder)
	}
	if result.Text != "2 + 2 = 4" {
		t.Errorf("unexpected text: %q", result.Text)
	}
}

func TestExecutor_MaxStepsPreventsLoop(t *testing.T) {
	// Server always returns a tool call → loop must stop at maxSteps.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := toolCallResponse("call_x", "readFile", `{}`)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := EvalData{
		Prompt:    "loop forever",
		MockTools: map[string]MockToolConfig{"readFile": {Description: "read", Result: "data"}},
		Config:    &ExecConfig{BaseURL: srv.URL, Model: "llama3.1", MaxSteps: 3},
	}

	result, err := NewExecutor().Run(context.Background(), data)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if len(result.ToolCallOrder) > 3 {
		t.Errorf("loop exceeded maxSteps: got %d tool calls", len(result.ToolCallOrder))
	}
}

// -------------------------------------------------------------------
// LLM judge tests with mock HTTP
// -------------------------------------------------------------------

func TestLLMJudge_ParsesScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := textResponse(`{"score": 9, "reason": "Response correctly uses tool results"}`)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	result := Result{
		Text:          "The API endpoint is https://api.example.com/v1",
		ToolCallOrder: []string{"readFile"},
	}
	target := Target{
		OriginalTask:    "Read config.json and report the API endpoint",
		MockToolResults: map[string]string{"readFile": `{"apiEndpoint":"https://api.example.com/v1"}`},
	}
	cfg := JudgeConfig{BaseURL: srv.URL, Model: "llama3.1"}

	score, judgeResult, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("LLMJudge error: %v", err)
	}
	if score < 0.8 {
		t.Errorf("expected score ≥ 0.8, got %.2f", score)
	}
	if judgeResult.Score != 9 {
		t.Errorf("expected raw score 9, got %d", judgeResult.Score)
	}
}

func TestLLMJudge_HandlesMarkdownFences(t *testing.T) {
	// Some models wrap JSON in ```json ... ```
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := textResponse("```json\n{\"score\": 7, \"reason\": \"Mostly correct\"}\n```")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	result := Result{Text: "ok", ToolCallOrder: []string{"queryDB"}}
	target := Target{OriginalTask: "query transactions"}
	cfg := JudgeConfig{BaseURL: srv.URL, Model: "llama3.1"}

	score, _, err := LLMJudge(context.Background(), result, target, cfg)
	if err != nil {
		t.Fatalf("LLMJudge error: %v", err)
	}
	if score < 0.5 {
		t.Errorf("expected score ≥ 0.5, got %.2f", score)
	}
}

// -------------------------------------------------------------------
// Runner integration test (LLM judge disabled for determinism)
// -------------------------------------------------------------------

func TestRunner_Dataset(t *testing.T) {
	// Mock executor server: always answers with a readFile call then final text.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var resp oaiResponse
		if calls%2 == 1 {
			resp = toolCallResponse("call_x", "readFile", `{}`)
		} else {
			resp = textResponse("The API endpoint is https://api.example.com/v1")
			calls = 0
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dataset := []TestCase{
		{
			Data: EvalData{
				Prompt:    "Read config.json and tell me the API endpoint",
				MockTools: map[string]MockToolConfig{"readFile": {Description: "Read", Result: `{"apiEndpoint":"https://api.example.com/v1"}`}},
				Config:    &ExecConfig{BaseURL: srv.URL, Model: "llama3.1", MaxSteps: 5},
			},
			Target: Target{
				ExpectedToolOrder: []string{"readFile"},
				ForbiddenTools:    []string{"writeFile"},
				OriginalTask:      "Read config.json and report the API endpoint",
			},
		},
	}

	runner := NewRunner(RunnerConfig{Concurrency: 1, SkipLLMJudge: true})
	results := runner.RunDataset(context.Background(), dataset)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != "" {
		t.Fatalf("unexpected error: %s", r.Err)
	}
	if r.Scores.ToolOrder != 1.0 {
		t.Errorf("tool order score: want 1.0, got %.2f", r.Scores.ToolOrder)
	}
	if r.Scores.ToolsAvoided != 1.0 {
		t.Errorf("tools avoided score: want 1.0, got %.2f", r.Scores.ToolsAvoided)
	}
	if r.Scores.OutputQuality != 1.0 {
		t.Errorf("output quality score (skipped judge): want 1.0, got %.2f", r.Scores.OutputQuality)
	}
}

// -------------------------------------------------------------------
// Dataset loading
// -------------------------------------------------------------------

func TestLoadDataset(t *testing.T) {
	cases, err := LoadDataset("data/agent_multiturn.json")
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if len(cases) < 3 {
		t.Errorf("expected ≥3 test cases in dataset, got %d", len(cases))
	}
	for i, tc := range cases {
		if tc.Data.MockTools == nil {
			t.Errorf("case[%d]: MockTools is nil", i)
		}
	}
}
