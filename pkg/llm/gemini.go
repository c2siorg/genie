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

// GeminiProvider implements Provider against Google AI Studio's
// generateContent endpoint. Compatible with Vertex AI via a swapped BaseURL.
type GeminiProvider struct {
	APIKey  string
	BaseURL string // default "https://generativelanguage.googleapis.com"
	Model   string // default "gemini-2.0-flash"
	Client  *http.Client
}

// NewGemini builds the provider.
func NewGemini(apiKey, model string) *GeminiProvider {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiProvider{
		APIKey:  apiKey,
		BaseURL: "https://generativelanguage.googleapis.com",
		Model:   model,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (g *GeminiProvider) Name() string   { return "gemini" }
func (g *GeminiProvider) Region() string { return "us" }

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  struct {
		Temperature     float64 `json:"temperature,omitempty"`
		MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

func (g *GeminiProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if g.APIKey == "" {
		return CompletionResponse{}, errors.New("gemini: APIKey is required")
	}
	if !req.Residency.AllowCrossBorder && req.Residency.Region != "" && req.Residency.Region != g.Region() {
		return CompletionResponse{}, ErrResidencyDenied
	}

	body := geminiRequest{}
	body.GenerationConfig.Temperature = req.Temperature
	body.GenerationConfig.MaxOutputTokens = req.MaxTokens

	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			body.SystemInstruction = &geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: m.Content}},
			}
		case RoleAssistant:
			body.Contents = append(body.Contents, geminiContent{
				Role:  "model",
				Parts: []geminiPart{{Text: m.Content}},
			})
		default:
			body.Contents = append(body.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, err
	}
	model := nonEmpty(req.Model, g.Model)
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimRight(g.BaseURL, "/"), model, g.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return CompletionResponse{}, fmt.Errorf("gemini http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var gr geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return CompletionResponse{}, err
	}
	var text strings.Builder
	if len(gr.Candidates) > 0 {
		for _, p := range gr.Candidates[0].Content.Parts {
			text.WriteString(p.Text)
		}
	}
	return CompletionResponse{
		Text:     text.String(),
		Provider: g.Name(),
		Model:    nonEmpty(gr.ModelVersion, model),
		Usage: Usage{
			PromptTokens:     gr.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}
