package rag

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// LLMQueryRewriter asks the LLM to produce N rewordings of a query. Useful
// for boosting recall on terse queries.
type LLMQueryRewriter struct {
	Provider llm.Provider
	Model    string
	N        int // number of alternative formulations; defaults to 3
}

// NewLLMQueryRewriter returns a rewriter that asks Provider for N rewrites.
func NewLLMQueryRewriter(p llm.Provider, model string, n int) *LLMQueryRewriter {
	if n <= 0 {
		n = 3
	}
	return &LLMQueryRewriter{Provider: p, Model: model, N: n}
}

// Rewrite asks the model for N alternate phrasings. Returns the original
// plus the rewrites; on error returns just the original.
func (r *LLMQueryRewriter) Rewrite(ctx context.Context, original string) ([]string, error) {
	resp, err := r.Provider.Complete(ctx, llm.CompletionRequest{
		Model: r.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Produce alternative phrasings of the user's query, one per line. Do not number them. Do not preface with anything else."},
			{Role: llm.RoleUser, Content: original},
		},
		MaxTokens:   128,
		Temperature: 0.2,
	})
	if err != nil {
		return []string{original}, nil
	}
	out := []string{original}
	for _, line := range strings.Split(resp.Text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out)-1 >= r.N {
			break
		}
	}
	return out, nil
}

// HyDE — Hypothetical Document Embeddings. Asks the LLM to write a short
// fake answer, then uses *that* as the embedding query. Works well when
// query terms differ from document terms (queries are short, docs are
// dense).
type HyDE struct {
	Provider llm.Provider
	Model    string
}

// NewHyDE wraps a Provider.
func NewHyDE(p llm.Provider, model string) *HyDE { return &HyDE{Provider: p, Model: model} }

// Hypothesise returns a short hypothetical answer to query — the string to
// embed instead of the raw query.
func (h *HyDE) Hypothesise(ctx context.Context, query string) (string, error) {
	resp, err := h.Provider.Complete(ctx, llm.CompletionRequest{
		Model: h.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Write one short paragraph that would answer the user's question. Do not preface with anything; emit the paragraph only."},
			{Role: llm.RoleUser, Content: query},
		},
		MaxTokens:   200,
		Temperature: 0.3,
	})
	if err != nil {
		return query, err
	}
	return strings.TrimSpace(resp.Text), nil
}
