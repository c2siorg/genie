package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements Provider against the OpenAI Chat Completions
// API. Compatible with Azure OpenAI (set BaseURL appropriately).
type OpenAIProvider struct {
	APIKey  string
	BaseURL string // default "https://api.openai.com"
	Model   string // default "gpt-4.1-mini"
	Client  *http.Client
}

// NewOpenAI builds the provider.
func NewOpenAI(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &OpenAIProvider{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com",
		Model:   model,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OpenAIProvider) Name() string   { return "openai" }
func (o *OpenAIProvider) Region() string { return "us" }

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openaiResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (o *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if o.APIKey == "" {
		return CompletionResponse{}, errors.New("openai: APIKey is required")
	}
	if !req.Residency.AllowCrossBorder && req.Residency.Region != "" && req.Residency.Region != o.Region() {
		return CompletionResponse{}, ErrResidencyDenied
	}
	msgs := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		role := string(m.Role)
		// OpenAI uses "user" and "assistant" and "system"; map cleanly.
		switch m.Role {
		case RoleAssistant:
			role = "assistant"
		case RoleSystem:
			role = "system"
		case RoleTool:
			role = "tool"
		default:
			role = "user"
		}
		msgs[i] = openaiMessage{Role: role, Content: m.Content}
	}
	body := openaiRequest{
		Model:       nonEmpty(req.Model, o.Model),
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(o.BaseURL, "/")+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.Client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return CompletionResponse{}, fmt.Errorf("openai http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var or openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return CompletionResponse{}, err
	}
	var text string
	if len(or.Choices) > 0 {
		text = or.Choices[0].Message.Content
	}
	return CompletionResponse{
		Text:     text,
		Provider: o.Name(),
		Model:    or.Model,
		Usage:    Usage{PromptTokens: or.Usage.PromptTokens, CompletionTokens: or.Usage.CompletionTokens},
	}, nil
}
