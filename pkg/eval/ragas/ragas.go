// Package ragas implements lightweight RAG quality metrics — faithfulness,
// answer relevance, context precision — using the LLM as an evaluator.
//
// Faithfulness: does the answer only make claims supported by the retrieved
// chunks? Score 0..1.
// AnswerRelevance: does the answer address the question? 0..1.
// ContextPrecision: how many of the retrieved chunks were actually used?
// 0..1.
//
// These are not statistical guarantees — they're cheap signals you can run
// on every prod request and aggregate over time.
package ragas

import (
	"context"
	"strconv"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

// Sample is one (query, answer, context) tuple to evaluate.
type Sample struct {
	Query   string
	Answer  string
	Context []rag.ScoredChunk
}

// Scorecard holds the three RAGAS metrics.
type Scorecard struct {
	Faithfulness     float64 `json:"faithfulness"`
	AnswerRelevance  float64 `json:"answer_relevance"`
	ContextPrecision float64 `json:"context_precision"`
}

// Evaluator runs the three metrics against a sample using an LLM.
type Evaluator struct {
	Provider llm.Provider
	Model    string
}

// NewEvaluator wraps a Provider.
func NewEvaluator(p llm.Provider, model string) *Evaluator {
	return &Evaluator{Provider: p, Model: model}
}

// Score runs all three metrics. Failures collapse to 0 for that metric.
func (e *Evaluator) Score(ctx context.Context, s Sample) (Scorecard, error) {
	var sc Scorecard
	if v, err := e.faithfulness(ctx, s); err == nil {
		sc.Faithfulness = v
	}
	if v, err := e.relevance(ctx, s); err == nil {
		sc.AnswerRelevance = v
	}
	if v, err := e.contextPrecision(ctx, s); err == nil {
		sc.ContextPrecision = v
	}
	return sc, nil
}

func (e *Evaluator) faithfulness(ctx context.Context, s Sample) (float64, error) {
	var ctxText strings.Builder
	for _, c := range s.Context {
		ctxText.WriteString(c.Text)
		ctxText.WriteString("\n---\n")
	}
	resp, err := e.Provider.Complete(ctx, llm.CompletionRequest{
		Model: e.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Return a single number 0.0..1.0 representing the fraction of the answer's factual claims that are supported by the context. No commentary."},
			{Role: llm.RoleUser, Content: "Context:\n" + ctxText.String() + "\nAnswer: " + s.Answer},
		},
		MaxTokens: 8, Temperature: 0,
	})
	if err != nil {
		return 0, err
	}
	return parseUnit(resp.Text), nil
}

func (e *Evaluator) relevance(ctx context.Context, s Sample) (float64, error) {
	resp, err := e.Provider.Complete(ctx, llm.CompletionRequest{
		Model: e.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Return a single number 0.0..1.0: how well does the answer address the query? No commentary."},
			{Role: llm.RoleUser, Content: "Query: " + s.Query + "\nAnswer: " + s.Answer},
		},
		MaxTokens: 8, Temperature: 0,
	})
	if err != nil {
		return 0, err
	}
	return parseUnit(resp.Text), nil
}

func (e *Evaluator) contextPrecision(ctx context.Context, s Sample) (float64, error) {
	if len(s.Context) == 0 {
		return 0, nil
	}
	var sb strings.Builder
	for i, c := range s.Context {
		sb.WriteString("[")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("] ")
		sb.WriteString(c.Text)
		sb.WriteString("\n---\n")
	}
	resp, err := e.Provider.Complete(ctx, llm.CompletionRequest{
		Model: e.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Return a single number 0.0..1.0: what fraction of the context chunks were actually relevant to answering the query? No commentary."},
			{Role: llm.RoleUser, Content: "Query: " + s.Query + "\nContext:\n" + sb.String()},
		},
		MaxTokens: 8, Temperature: 0,
	})
	if err != nil {
		return 0, err
	}
	return parseUnit(resp.Text), nil
}

func parseUnit(text string) float64 {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(strings.Split(text, "\n")[0], 64)
	if err != nil {
		return 0
	}
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
