package rag

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// Reranker re-scores a candidate list. Production setups use a cross-encoder
// model (e.g. BGE-reranker); for the demo we ship a no-op and an LLM-based
// reranker that asks the model to rate each candidate 0..10.
type Reranker interface {
	Rerank(ctx context.Context, query string, candidates []ScoredChunk, topK int) ([]ScoredChunk, error)
}

// IdentityReranker returns the candidates unchanged (modulo topK).
type IdentityReranker struct{}

// Rerank trims to topK; preserves order.
func (IdentityReranker) Rerank(_ context.Context, _ string, c []ScoredChunk, topK int) ([]ScoredChunk, error) {
	if topK > 0 && topK < len(c) {
		return c[:topK], nil
	}
	return c, nil
}

// LLMReranker asks the LLM to score each candidate 0..10 for relevance.
// Single prompt — N candidates in, N scores out. Cheap for ≤20 candidates.
type LLMReranker struct {
	Provider llm.Provider
	Model    string
}

// NewLLMReranker wraps a Provider.
func NewLLMReranker(p llm.Provider, model string) *LLMReranker {
	return &LLMReranker{Provider: p, Model: model}
}

// Rerank scores each candidate by asking the LLM. Falls back to the input
// order if the model returns garbage.
func (r *LLMReranker) Rerank(ctx context.Context, query string, candidates []ScoredChunk, topK int) ([]ScoredChunk, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}
	var sb strings.Builder
	sb.WriteString("Score each passage 0..10 for relevance to the query. Reply with one score per line, in the same order.\n\nQuery: ")
	sb.WriteString(query)
	sb.WriteString("\n\nPassages:\n")
	for i, c := range candidates {
		fmt.Fprintf(&sb, "[%d] %s\n", i, truncate(c.Text, 240))
	}
	resp, err := r.Provider.Complete(ctx, llm.CompletionRequest{
		Model: r.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a relevance grader. Reply with one integer 0..10 per line. No commentary."},
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens:   64 + 4*len(candidates),
		Temperature: 0,
	})
	if err != nil {
		return candidates, err
	}
	scores := parseScores(resp.Text, len(candidates))
	out := make([]ScoredChunk, len(candidates))
	copy(out, candidates)
	for i := range out {
		out[i].Score = float32(scores[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if topK > 0 && topK < len(out) {
		out = out[:topK]
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// parseScores reads up to n integers (one per line). Missing lines default to 0.
func parseScores(text string, n int) []int {
	out := make([]int, n)
	i := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if v, err := strconv.Atoi(line); err == nil {
			if v < 0 {
				v = 0
			}
			if v > 10 {
				v = 10
			}
			out[i] = v
			i++
			if i >= n {
				break
			}
		}
	}
	return out
}
