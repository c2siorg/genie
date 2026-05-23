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

// AnthropicProvider implements Provider against the Anthropic Messages API.
// We talk HTTP directly to avoid pulling the SDK; if you outgrow this, swap
// for the official SDK behind the same interface.
type AnthropicProvider struct {
	APIKey    string
	BaseURL   string // default "https://api.anthropic.com"
	Model     string // default "claude-sonnet-4-6"
	APIVersion string // default "2023-06-01"
	Client    *http.Client
}

// NewAnthropic builds the provider.
func NewAnthropic(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &AnthropicProvider{
		APIKey:     apiKey,
		BaseURL:    "https://api.anthropic.com",
		Model:      model,
		APIVersion: "2023-06-01",
		Client:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (a *AnthropicProvider) Name() string   { return "anthropic" }
func (a *AnthropicProvider) Region() string { return "us" }

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Temperature float64          `json:"temperature,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model     string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage     struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (a *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if a.APIKey == "" {
		return CompletionResponse{}, errors.New("anthropic: APIKey is required")
	}
	if !req.Residency.AllowCrossBorder && req.Residency.Region != "" && req.Residency.Region != a.Region() {
		return CompletionResponse{}, ErrResidencyDenied
	}

	// Anthropic separates system prompt from the conversation messages.
	var system string
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
			continue
		}
		role := "user"
		if m.Role == RoleAssistant {
			role = "assistant"
		}
		msgs = append(msgs, anthropicMessage{Role: role, Content: m.Content})
	}

	body := anthropicRequest{
		Model:       nonEmpty(req.Model, a.Model),
		System:      system,
		Messages:    msgs,
		MaxTokens:   nonZero(req.MaxTokens, 1024),
		Temperature: req.Temperature,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(a.BaseURL, "/")+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.APIKey)
	httpReq.Header.Set("anthropic-version", a.APIVersion)

	resp, err := a.Client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return CompletionResponse{}, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var ar anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return CompletionResponse{}, err
	}
	var text strings.Builder
	for _, c := range ar.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return CompletionResponse{
		Text:     text.String(),
		Provider: a.Name(),
		Model:    ar.Model,
		Usage: Usage{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
		},
	}, nil
}

func nonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
