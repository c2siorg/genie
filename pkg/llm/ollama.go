package llm

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

// OllamaProvider is the on-prem LLM provider Genie uses when residency
// requires that PII not leave the perimeter. Region is fixed to "on-prem"
// so the sovereignty machinery treats it as inside any home region.
//
// Wire format: OpenAI-compatible /v1/chat/completions so Ollama tool-calling
// works with any model that supports it (qwen3.5, granite4.1, mistral-nemo,
// llama3.1:8b+, etc.). The smaller llama3.2:1b model does not reliably
// produce structured tool_calls — use a >= 7B parameter model for tool use.
type OllamaProvider struct {
	URL    string        // e.g. "http://localhost:11434"
	Model  string        // e.g. "qwen3.5:latest"
	Client *http.Client  // optional; default 60s timeout
}

// NewOllamaProvider builds a provider; URL defaults to localhost:11434.
func NewOllamaProvider(url, model string) *OllamaProvider {
	if url == "" {
		url = "http://localhost:11434"
	}
	if model == "" {
		model = "qwen3.5:latest"
	}
	return &OllamaProvider{URL: url, Model: model, Client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *OllamaProvider) Name() string   { return "ollama" }
func (p *OllamaProvider) Region() string { return "on-prem" }

// SupportsVision reports true. Ollama vision models (llava, gemma3-vision,
// minicpm-v) accept base64 images directly; the request builder passes them through.
func (p *OllamaProvider) SupportsVision() bool { return true }

// ─── OpenAI-compatible wire types ─────────────────────────────────────────

// oaiMessage is one turn in the OpenAI-compatible chat format.
type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiTool struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type oaiChatRequest struct {
	Model    string        `json:"model"`
	Messages []oaiMessage  `json:"messages"`
	Tools    []oaiTool     `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type oaiChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string        `json:"role"`
			Content   string        `json:"content"`
			ToolCalls []oaiToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// ─── Complete ──────────────────────────────────────────────────────────────

func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Build the OpenAI-compatible request.
	body := oaiChatRequest{
		Model:    nonEmpty(req.Model, p.Model),
		Messages: make([]oaiMessage, 0, len(req.Messages)),
		Stream:   false,
	}

	// Map messages to wire format.
	for _, m := range req.Messages {
		om := oaiMessage{Role: string(m.Role), Content: m.Content}
		if m.ToolName != "" {
			om.Name = m.ToolName
		}
		body.Messages = append(body.Messages, om)
	}

	// Map tools to OpenAI format.
	if len(req.Tools) > 0 {
		body.Tools = make([]oaiTool, len(req.Tools))
		for i, t := range req.Tools {
			body.Tools[i].Type = "function"
			body.Tools[i].Function.Name = t.Name
			body.Tools[i].Function.Description = t.Description
			body.Tools[i].Function.Parameters = t.InputSchema
			if body.Tools[i].Function.Parameters == nil {
				body.Tools[i].Function.Parameters = map[string]any{
					"type": "object", "properties": map[string]any{},
				}
			}
		}
	}

	// Inference options.
	if req.MaxTokens > 0 || req.Temperature > 0 {
		body.Options = map[string]any{}
		if req.MaxTokens > 0 {
			body.Options["num_predict"] = req.MaxTokens
		}
		if req.Temperature > 0 {
			body.Options["temperature"] = req.Temperature
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, err
	}

	endpoint := strings.TrimRight(p.URL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return CompletionResponse{}, fmt.Errorf("ollama http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var or oaiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama decode: %w", err)
	}
	if len(or.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("ollama: no choices in response")
	}

	choice := or.Choices[0]
	out := CompletionResponse{
		Text:     choice.Message.Content,
		Provider: p.Name(),
		Model:    body.Model,
		Usage: Usage{
			PromptTokens:     or.Usage.PromptTokens,
			CompletionTokens: or.Usage.CompletionTokens,
		},
	}

	// Map tool calls from the response.
	for _, tc := range choice.Message.ToolCalls {
		var input map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return out, nil
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
