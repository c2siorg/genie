// Package hallucination detects unsupported claims in an answer relative
// to retrieved context. One LLM call grades each sentence as supported /
// contradicted / unsupported.
package hallucination

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// Verdict is the per-sentence label.
type Verdict string

const (
	Supported     Verdict = "supported"
	Unsupported   Verdict = "unsupported"
	Contradicted  Verdict = "contradicted"
)

// SentenceGrade pairs a sentence with its label.
type SentenceGrade struct {
	Sentence string
	Label    Verdict
}

// Report bundles the per-sentence labels plus an overall fraction of
// sentences that were grounded.
type Report struct {
	Grades            []SentenceGrade
	SupportedFraction float64 // 0..1
}

// Detect splits the answer into sentences, asks the LLM to grade each
// against the context, and returns the report.
//
// Heuristic split: on . ! ? followed by whitespace. Good enough for one
// paragraph of recommender output; production should use a proper sentence
// tokeniser.
func Detect(ctx context.Context, p llm.Provider, model, contextText, answer string) (Report, error) {
	sentences := splitSentences(answer)
	if len(sentences) == 0 {
		return Report{}, nil
	}
	var prompt strings.Builder
	prompt.WriteString("Label each sentence as 'supported', 'unsupported', or 'contradicted' given the context. Reply with one label per line in order.\n\nContext:\n")
	prompt.WriteString(contextText)
	prompt.WriteString("\n\nSentences:\n")
	for i, s := range sentences {
		prompt.WriteString(itoa(i + 1))
		prompt.WriteString(". ")
		prompt.WriteString(s)
		prompt.WriteString("\n")
	}
	resp, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Label sentences using the context. One label per line: 'supported', 'unsupported', or 'contradicted'."},
			{Role: llm.RoleUser, Content: prompt.String()},
		},
		MaxTokens:   8 + 12*len(sentences),
		Temperature: 0,
		Residency:   llm.Residency{AllowCrossBorder: true},
	})
	if err != nil {
		return Report{}, err
	}
	labels := parseLabels(resp.Text, len(sentences))
	r := Report{Grades: make([]SentenceGrade, len(sentences))}
	supported := 0
	for i, s := range sentences {
		r.Grades[i] = SentenceGrade{Sentence: s, Label: labels[i]}
		if labels[i] == Supported {
			supported++
		}
	}
	r.SupportedFraction = float64(supported) / float64(len(sentences))
	return r, nil
}

func splitSentences(text string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseLabels(text string, n int) []Verdict {
	out := make([]Verdict, n)
	for i := range out {
		out[i] = Unsupported
	}
	i := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" {
			continue
		}
		switch {
		case strings.Contains(line, "supported") && !strings.Contains(line, "un"):
			out[i] = Supported
		case strings.Contains(line, "contradict"):
			out[i] = Contradicted
		default:
			out[i] = Unsupported
		}
		i++
		if i >= n {
			break
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
