package rag

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// SelfRAG asks the model whether retrieval is needed for a given query
// before paying the cost. Reduces unnecessary RAG calls on
// "thank-you"-style turns and pure-arithmetic questions.
//
// Returns true if the model decides retrieval would help.
type SelfRAG struct {
	Provider llm.Provider
	Model    string
}

// NewSelfRAG wraps a Provider.
func NewSelfRAG(p llm.Provider, model string) *SelfRAG { return &SelfRAG{Provider: p, Model: model} }

// Should asks the model "do you need retrieval to answer this?"
func (s *SelfRAG) Should(ctx context.Context, query string) (bool, error) {
	resp, err := s.Provider.Complete(ctx, llm.CompletionRequest{
		Model: s.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Reply 'YES' if external retrieval would meaningfully improve the answer for the user's query, otherwise 'NO'. One word only."},
			{Role: llm.RoleUser, Content: query},
		},
		MaxTokens:   4,
		Temperature: 0,
	})
	if err != nil {
		// Fail open — retrieve when uncertain.
		return true, err
	}
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(resp.Text)), "Y"), nil
}

// CRAG (Corrective RAG) grades each retrieved chunk for relevance, keeps
// the relevant ones, and reports overall confidence so callers can decide
// whether to fall back (e.g. to web search) when confidence is low.
type CRAG struct {
	Provider          llm.Provider
	Model             string
	RelevanceMin      float64 // 0..1; chunks below this are dropped
}

// NewCRAG wraps a Provider with a relevance threshold (default 0.5).
func NewCRAG(p llm.Provider, model string, threshold float64) *CRAG {
	if threshold <= 0 {
		threshold = 0.5
	}
	return &CRAG{Provider: p, Model: model, RelevanceMin: threshold}
}

// CRAGResult contains the filtered chunks plus a confidence score.
type CRAGResult struct {
	Kept       []ScoredChunk
	Dropped    []ScoredChunk
	Confidence float64 // average kept-chunk score normalised 0..1
}

// Grade asks the model to score each chunk 0..1; keeps those at or above
// RelevanceMin.
func (c *CRAG) Grade(ctx context.Context, query string, chunks []ScoredChunk) (CRAGResult, error) {
	if len(chunks) == 0 {
		return CRAGResult{}, nil
	}
	var prompt strings.Builder
	prompt.WriteString("Score each passage 0.0..1.0 for relevance to the query. Reply with one decimal per line, in the same order.\n\nQuery: ")
	prompt.WriteString(query)
	prompt.WriteString("\n\nPassages:\n")
	for i, ch := range chunks {
		prompt.WriteString(label(i))
		prompt.WriteString(" ")
		prompt.WriteString(truncate(ch.Text, 200))
		prompt.WriteString("\n")
	}
	resp, err := c.Provider.Complete(ctx, llm.CompletionRequest{
		Model: c.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a relevance grader. Reply with one decimal 0.0..1.0 per line. No commentary."},
			{Role: llm.RoleUser, Content: prompt.String()},
		},
		MaxTokens:   16 + 8*len(chunks),
		Temperature: 0,
	})
	if err != nil {
		return CRAGResult{}, err
	}
	scores := parseUnits(resp.Text, len(chunks))
	res := CRAGResult{}
	var sum float64
	for i, ch := range chunks {
		s := scores[i]
		if s >= c.RelevanceMin {
			ch.Score = float32(s)
			res.Kept = append(res.Kept, ch)
			sum += s
		} else {
			res.Dropped = append(res.Dropped, ch)
		}
	}
	if len(res.Kept) > 0 {
		res.Confidence = sum / float64(len(res.Kept))
	}
	return res, nil
}

// LostInMiddleReorder mitigates the "lost in the middle" effect: large
// LLMs attend less to chunks in the middle of long contexts. We reorder so
// the best chunks are at the head and the second-best at the tail.
//
// Algorithm: place even-ranked chunks at the head, odd-ranked at the tail
// (reversed). The single most relevant chunk ends up first.
func LostInMiddleReorder(chunks []ScoredChunk) []ScoredChunk {
	if len(chunks) <= 2 {
		return chunks
	}
	head := make([]ScoredChunk, 0, len(chunks))
	tail := make([]ScoredChunk, 0, len(chunks))
	for i, c := range chunks {
		if i%2 == 0 {
			head = append(head, c)
		} else {
			tail = append(tail, c)
		}
	}
	// reverse tail so highest-ranked odd chunk ends up at the very end.
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return append(head, tail...)
}

// label returns "[i]" — kept tiny so we don't pull strconv for one number.
func label(i int) string {
	if i < 10 {
		return string([]byte{'[', byte('0' + i), ']'})
	}
	return "[" + itoa(i) + "]"
}

// parseUnits — reads up to n decimals (0..1, one per line). Missing → 0.
func parseUnits(text string, n int) []float64 {
	out := make([]float64, n)
	i := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := parseDecimal(line)
		if f < 0 {
			f = 0
		}
		if f > 1 {
			f = 1
		}
		out[i] = f
		i++
		if i >= n {
			break
		}
	}
	return out
}

// parseDecimal is a tiny strconv.ParseFloat shim — no whole imports.
func parseDecimal(s string) float64 {
	var f float64
	var frac float64 = 1
	var afterDot bool
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			d := float64(r - '0')
			if afterDot {
				frac *= 10
				f += d / frac
			} else {
				f = f*10 + d
			}
		case r == '.':
			afterDot = true
		default:
			return f
		}
	}
	return f
}
