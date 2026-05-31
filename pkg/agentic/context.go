package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Context management (lesson 07).
//
// Strategy: estimate token usage before each turn; if over the threshold
// (default 80% of context window) summarize the conversation history and
// replace it with a compact seed. This lets agent conversations run
// indefinitely without hitting context window limits.
//
// Token estimation: ~4 chars per token (English rule of thumb).
// Compaction: ask the LLM to summarize, replace history with the summary.

const (
	// defaultContextWindow is a conservative estimate for most models.
	defaultContextWindow = 128_000
	// charsPerToken is the English rule of thumb for token estimation.
	charsPerToken = 4
)

// EstimateTokens returns a rough token count for a slice of messages.
// Uses the 4-chars-per-token heuristic — good enough for threshold checks.
func EstimateTokens(msgs []Message) TokenUsage {
	var inputChars, outputChars int
	for _, m := range msgs {
		chars := len(m.Role) + len(m.Content)
		if m.Role == "assistant" {
			outputChars += chars
		} else {
			inputChars += chars
		}
	}
	return TokenUsage{
		InputTokens:   inputChars / charsPerToken,
		OutputTokens:  outputChars / charsPerToken,
		TotalTokens:   (inputChars + outputChars) / charsPerToken,
		ContextWindow: defaultContextWindow,
		Percentage:    float64(inputChars+outputChars) / float64(defaultContextWindow*charsPerToken) * 100,
	}
}

// IsOverThreshold returns true when estimated usage exceeds the threshold.
func IsOverThreshold(msgs []Message, threshold float64) bool {
	if threshold <= 0 {
		threshold = 0.80
	}
	usage := EstimateTokens(msgs)
	return float64(usage.TotalTokens) > float64(usage.ContextWindow)*threshold
}

// compactMessages summarizes conversation history and returns a 2-message
// "seed" that continues the conversation without the full history.
// System messages are excluded (they're re-added each turn).
//
// Mirrors the TypeScript compactConversation() exactly.
func compactMessages(ctx context.Context, cfg Config, msgs []Message, client *http.Client) ([]Message, error) {
	// Collect non-system messages.
	var history []Message
	for _, m := range msgs {
		if m.Role != "system" {
			history = append(history, m)
		}
	}
	if len(history) == 0 {
		return nil, nil
	}

	// Build a plain-text rendering.
	var sb strings.Builder
	for _, m := range history {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", m.Role, m.Content))
	}

	summarizationPrompt := `You are a conversation summarizer. Create a concise summary that preserves:
1. Key decisions and conclusions reached
2. Important context and facts mentioned
3. Any pending tasks or questions
4. The overall goal of the conversation

Be concise but complete so the conversation can continue naturally.

Conversation to summarize:
` + sb.String()

	summary, err := simpleLLMCall(ctx, cfg, summarizationPrompt, client)
	if err != nil {
		return nil, fmt.Errorf("compaction LLM call: %w", err)
	}

	return []Message{
		{
			Role:    "user",
			Content: "[CONVERSATION SUMMARY]\nThe following is a summary of our conversation so far:\n\n" + summary + "\n\nPlease continue from where we left off.",
		},
		{
			Role:    "assistant",
			Content: "I've reviewed the conversation summary and I'm ready to continue. How can I help you next?",
		},
	}, nil
}

// simpleLLMCall makes a single non-streaming LLM call for summarization.
func simpleLLMCall(ctx context.Context, cfg Config, prompt string, client *http.Client) (string, error) {
	body := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	raw, _ := json.Marshal(body)

	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var out struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("unexpected response: %s", b)
	}
	return out.Choices[0].Message.Content, nil
}
