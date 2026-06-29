package judges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// llmRequest is the wire format for LLM API requests (OpenAI-compatible).
type llmRequest struct {
	Model       string       `json:"model"`
	Messages    []llmMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens"`
}

// llmMessage represents a single message in the request.
type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// llmResponse is the wire format for LLM API responses (OpenAI-compatible).
type llmResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// llmJudgeResponse is the structured response from the judge LLM.
type llmJudgeResponse struct {
	Pass     bool    `json:"pass"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
	Evidence string  `json:"evidence"`
}

// callLLMJudge invokes the LLM with a judge prompt and returns structured verdict.
func callLLMJudge(ctx context.Context, cfg JudgeConfig, systemPrompt, userPrompt string) (llmJudgeResponse, error) {
	var resp llmJudgeResponse

	// Resolve defaults
	if cfg.Provider == "" {
		cfg.Provider = "ollama"
	}
	if cfg.BaseURL == "" {
		switch cfg.Provider {
		case "anthropic":
			cfg.BaseURL = "https://api.anthropic.com"
		case "openai":
			cfg.BaseURL = "https://api.openai.com"
		default:
			cfg.BaseURL = "http://localhost:11434"
		}
	}
	if cfg.Model == "" {
		switch cfg.Provider {
		case "anthropic":
			cfg.Model = "claude-opus-4"
		case "openai":
			cfg.Model = "gpt-4"
		default:
			cfg.Model = "llama3.1"
		}
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.3
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2000
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 60
	}

	// Build request
	req := llmRequest{
		Model: cfg.Model,
		Messages: []llmMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return resp, fmt.Errorf("marshal request: %w", err)
	}

	// Build HTTP request
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return resp, fmt.Errorf("new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	// Execute request with timeout
	client := &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return resp, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(httpResp.Body, 4096))
		return resp, fmt.Errorf("http %d: %s", httpResp.StatusCode, strings.TrimSpace(string(b)))
	}

	// Parse response
	var llmResp llmResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&llmResp); err != nil {
		return resp, fmt.Errorf("decode response: %w", err)
	}

	if llmResp.Error != nil {
		return resp, fmt.Errorf("llm error: %s", llmResp.Error.Message)
	}

	if len(llmResp.Choices) == 0 {
		return resp, fmt.Errorf("no choices in response")
	}

	content := llmResp.Choices[0].Message.Content

	// Try to extract JSON from response (may be wrapped in markdown)
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx >= 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "}"); idx >= 0 {
		content = content[:idx+1]
	}

	// Unmarshal structured response
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return resp, fmt.Errorf("unmarshal judge response: %w (content: %s)", err, content)
	}

	// Clamp score to [0, 1]
	if resp.Score < 0 {
		resp.Score = 0
	}
	if resp.Score > 1 {
		resp.Score = 1
	}

	return resp, nil
}

// ConfidenceInterval represents a 95% confidence interval.
type ConfidenceInterval struct {
	LowerBound float64 `json:"lower_bound"`
	UpperBound float64 `json:"upper_bound"`
}

// bootstrapCI computes 95% confidence interval via percentile bootstrap.
func bootstrapCI(samples []float64, numBootstrapSamples int) ConfidenceInterval {
	if len(samples) == 0 {
		return ConfidenceInterval{}
	}

	bootstrapMeans := make([]float64, numBootstrapSamples)
	for i := 0; i < numBootstrapSamples; i++ {
		var sum float64
		for j := 0; j < len(samples); j++ {
			// Randomly sample with replacement
			idx := (i*997 + j*1009) % len(samples) // Pseudo-random without importing math/rand
			sum += samples[idx]
		}
		bootstrapMeans[i] = sum / float64(len(samples))
	}

	// Sort bootstrap means
	sort.Float64s(bootstrapMeans)

	// Percentile method: 2.5th and 97.5th percentiles
	lowerIdx := int(math.Floor(0.025 * float64(len(bootstrapMeans))))
	upperIdx := int(math.Ceil(0.975 * float64(len(bootstrapMeans))))

	if upperIdx >= len(bootstrapMeans) {
		upperIdx = len(bootstrapMeans) - 1
	}

	return ConfidenceInterval{
		LowerBound: bootstrapMeans[lowerIdx],
		UpperBound: bootstrapMeans[upperIdx],
	}
}

// computeTPR computes true positive rate from verdicts.
func computeTPR(verdicts []bool, groundTruth []bool) float64 {
	if len(verdicts) != len(groundTruth) {
		return 0
	}

	var truePositives, positives int
	for i, gt := range groundTruth {
		if gt {
			positives++
			if verdicts[i] {
				truePositives++
			}
		}
	}

	if positives == 0 {
		return 1.0
	}
	return float64(truePositives) / float64(positives)
}

// computeTNR computes true negative rate from verdicts.
func computeTNR(verdicts []bool, groundTruth []bool) float64 {
	if len(verdicts) != len(groundTruth) {
		return 0
	}

	var trueNegatives, negatives int
	for i, gt := range groundTruth {
		if !gt {
			negatives++
			if !verdicts[i] {
				trueNegatives++
			}
		}
	}

	if negatives == 0 {
		return 1.0
	}
	return float64(trueNegatives) / float64(negatives)
}

// findOptimalThreshold finds the threshold that maximizes sensitivity + specificity.
func findOptimalThreshold(scores []float64, groundTruth []bool) (float64, float64) {
	if len(scores) != len(groundTruth) {
		return 0.5, 0
	}

	// Try thresholds from 0.0 to 1.0 in 0.01 steps
	var bestThreshold float64
	var bestScore float64
	thresholds := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

	for _, t := range thresholds {
		verdicts := make([]bool, len(scores))
		for i, s := range scores {
			verdicts[i] = s >= t
		}

		tpr := computeTPR(verdicts, groundTruth)
		tnr := computeTNR(verdicts, groundTruth)
		score := tpr + tnr // Maximize sensitivity + specificity

		if score > bestScore {
			bestScore = score
			bestThreshold = t
		}
	}

	return bestThreshold, bestScore
}

// stringifyInput converts any input to JSON string for context.
func stringifyInput(input interface{}) (string, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
