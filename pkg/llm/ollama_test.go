package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllama_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path is OpenAI-compatible.
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		var req oaiChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.Model == "" {
			http.Error(w, "model required", http.StatusBadRequest)
			return
		}
		// Return a text completion response.
		resp := oaiChatResponse{}
		resp.Choices = []struct {
			Message struct {
				Role      string        `json:"role"`
				Content   string        `json:"content"`
				ToolCalls []oaiToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				FinishReason: "stop",
			},
		}
		resp.Choices[0].Message.Role = "assistant"
		resp.Choices[0].Message.Content = "test answer"
		resp.Usage.PromptTokens = 10
		resp.Usage.CompletionTokens = 5

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "qwen3.5:latest")
	r, err := p.Complete(context.Background(), CompletionRequest{
		Model:    "qwen3.5:latest",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "test answer" {
		t.Fatalf("text: %q", r.Text)
	}
	if r.Usage.PromptTokens != 10 {
		t.Fatalf("prompt tokens: %d", r.Usage.PromptTokens)
	}
	if p.Region() != "on-prem" {
		t.Fatalf("expected on-prem, got %s", p.Region())
	}
}

func TestOllama_ToolCalling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req oaiChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify tools were sent.
		if len(req.Tools) == 0 {
			http.Error(w, "expected tools in request", http.StatusBadRequest)
			return
		}

		// Return a tool_calls response.
		resp := oaiChatResponse{}
		tc := oaiToolCall{ID: "call-123", Type: "function"}
		tc.Function.Name = req.Tools[0].Function.Name
		tc.Function.Arguments = `{"path":"config.json"}`

		resp.Choices = []struct {
			Message struct {
				Role      string        `json:"role"`
				Content   string        `json:"content"`
				ToolCalls []oaiToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{FinishReason: "tool_calls"},
		}
		resp.Choices[0].Message.Role = "assistant"
		resp.Choices[0].Message.ToolCalls = []oaiToolCall{tc}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "qwen3.5:latest")
	r, err := p.Complete(context.Background(), CompletionRequest{
		Model:    "qwen3.5:latest",
		Messages: []Message{{Role: RoleUser, Content: "Read config.json"}},
		Tools: []ToolDefinition{
			{Name: "read_file", Description: "Read a file", InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ToolCalls) == 0 {
		t.Fatal("expected tool calls in response")
	}
	if r.ToolCalls[0].Name != "read_file" {
		t.Fatalf("tool name: %q", r.ToolCalls[0].Name)
	}
	if r.ToolCalls[0].ID != "call-123" {
		t.Fatalf("tool id: %q", r.ToolCalls[0].ID)
	}
}

func TestOllama_DefaultModel(t *testing.T) {
	p := NewOllamaProvider("", "")
	if p.Model != "qwen3.5:latest" {
		t.Fatalf("default model: %q", p.Model)
	}
	if p.URL != "http://localhost:11434" {
		t.Fatalf("default url: %q", p.URL)
	}
}
