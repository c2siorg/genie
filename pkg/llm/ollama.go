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

// OllamaProvider is the on-prem LLM provider Genie uses when residency
// requires that PII not leave the perimeter. Region is fixed to "on-prem"
// so the sovereignty machinery treats it as inside any home region.
//
// Wire format follows Ollama's /api/chat endpoint.
type OllamaProvider struct {
	URL    string        // e.g. "http://localhost:11434"
	Model  string        // e.g. "llama3.1"
	Client *http.Client  // optional; default 30s timeout
}

// NewOllamaProvider builds a provider; URL defaults to localhost:11434.
func NewOllamaProvider(url, model string) *OllamaProvider {
	if url == "" {
		url = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.1"
	}
	return &OllamaProvider{URL: url, Model: model, Client: &http.Client{Timeout: 30 * time.Second}}
}

func (p *OllamaProvider) Name() string   { return "ollama" }
func (p *OllamaProvider) Region() string { return "on-prem" }

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // base64 strings — Ollama vision shape
}

// SupportsVision reports true. Ollama vision models (llava, gemma3-vision,
// minicpm-v) accept base64 images directly via the chat API; the request
// builder passes them through.
func (p *OllamaProvider) SupportsVision() bool { return true }

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done             bool   `json:"done"`
	TotalDuration    int64  `json:"total_duration"`
	PromptEvalCount  int    `json:"prompt_eval_count"`
	EvalCount        int    `json:"eval_count"`
	DoneReason       string `json:"done_reason"`
}

func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Residency: Ollama is on-prem so PII is fine; refuse if the caller marks
	// the request as cross-border disallowed AND the home region isn't on-prem.
	if !req.Residency.AllowCrossBorder && req.Residency.Region != "" &&
		req.Residency.Region != "on-prem" {
		// On-prem can serve any home region — only refuse if the caller
		// explicitly named a region we don't satisfy. We satisfy any region
		// because we run locally; this branch is here as a placeholder.
	}

	body := ollamaRequest{
		Model: nonEmpty(req.Model, p.Model),
		Messages: make([]ollamaMessage, 0, len(req.Messages)),
		Stream: false,
	}
	for _, m := range req.Messages {
		om := ollamaMessage{Role: string(m.Role), Content: m.Content}
		if len(m.Images) > 0 {
			om.Images = make([]string, len(m.Images))
			for i, img := range m.Images {
				om.Images[i] = img.Base64
			}
		}
		body.Messages = append(body.Messages, om)
	}
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(p.URL, "/")+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return CompletionResponse{}, fmt.Errorf("ollama http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var or ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return CompletionResponse{}, err
	}
	if or.Message.Content == "" {
		return CompletionResponse{}, errors.New("ollama: empty response")
	}
	return CompletionResponse{
		Text:     or.Message.Content,
		Provider: p.Name(),
		Model:    body.Model,
		Usage: Usage{
			PromptTokens:     or.PromptEvalCount,
			CompletionTokens: or.EvalCount,
		},
	}, nil
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
