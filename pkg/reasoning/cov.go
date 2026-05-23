package reasoning

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// ChainOfVerification (Dhuliawala et al., 2023) cuts hallucinations by:
//
//  1. Generating an initial answer.
//  2. Listing verifying questions about the answer.
//  3. Answering each question independently.
//  4. Synthesising a final, verified answer.
//
// Useful for the recommender and educator when factual accuracy matters.
type CoVResult struct {
	Initial      string
	Questions    []string
	Verifications []string
	Final        string
}

// CoV runs the four-step verification loop using a single Provider.
func CoV(ctx context.Context, p llm.Provider, model, system, user string) (CoVResult, error) {
	var res CoVResult

	// Step 1 — initial answer.
	initial, err := simpleAnswer(ctx, p, model, []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: user},
	}, 512)
	if err != nil {
		return res, err
	}
	res.Initial = initial

	// Step 2 — list verifying questions.
	questionsText, err := simpleAnswer(ctx, p, model, []llm.Message{
		{Role: llm.RoleSystem, Content: "List independent, factual yes/no questions whose answers would verify the assistant's claims. One per line. No commentary."},
		{Role: llm.RoleUser, Content: "Query: " + user + "\nAnswer:\n" + initial},
	}, 256)
	if err != nil {
		return res, err
	}
	res.Questions = nonEmptyLines(questionsText)

	// Step 3 — answer each question independently. We do this in one call to
	// save round-trips; production should parallelise.
	if len(res.Questions) > 0 {
		var q strings.Builder
		q.WriteString("Answer each question YES or NO with a one-sentence justification. Number them.\n\n")
		for i, qq := range res.Questions {
			q.WriteString(itoa(i + 1))
			q.WriteString(". ")
			q.WriteString(qq)
			q.WriteString("\n")
		}
		verText, err := simpleAnswer(ctx, p, model, []llm.Message{
			{Role: llm.RoleSystem, Content: "You answer factual yes/no questions. Reply numbered, one line each."},
			{Role: llm.RoleUser, Content: q.String()},
		}, 256+24*len(res.Questions))
		if err != nil {
			return res, err
		}
		res.Verifications = nonEmptyLines(verText)
	}

	// Step 4 — synthesise final answer constrained by the verifications.
	final, err := simpleAnswer(ctx, p, model, []llm.Message{
		{Role: llm.RoleSystem, Content: system + "\n\nRevise the assistant's draft using the verifications. If any verification contradicts the draft, correct or remove that claim."},
		{Role: llm.RoleUser, Content: "Query: " + user + "\nDraft: " + res.Initial + "\nVerifications:\n" + strings.Join(res.Verifications, "\n")},
	}, 512)
	if err != nil {
		return res, err
	}
	res.Final = final
	return res, nil
}

// StepBack (Zheng et al., 2023). Generate an abstract "step-back" question
// before the answer; gives the model time to ground itself in higher-level
// principles. Two-step LLM call.
func StepBack(ctx context.Context, p llm.Provider, model, system, user string) (stepback, final string, err error) {
	stepback, err = simpleAnswer(ctx, p, model, []llm.Message{
		{Role: llm.RoleSystem, Content: "Restate the user's question as a more abstract, principle-level question. One sentence."},
		{Role: llm.RoleUser, Content: user},
	}, 64)
	if err != nil {
		return "", "", err
	}
	final, err = simpleAnswer(ctx, p, model, []llm.Message{
		{Role: llm.RoleSystem, Content: system + "\n\nUse the step-back question to ground your answer in higher-level principles before responding to the specific question."},
		{Role: llm.RoleUser, Content: "Specific question: " + user + "\nStep-back question: " + stepback},
	}, 512)
	return stepback, final, err
}

// simpleAnswer is a one-shot Complete + trim helper used by both routines.
func simpleAnswer(ctx context.Context, p llm.Provider, model string, msgs []llm.Message, maxTokens int) (string, error) {
	resp, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model, Messages: msgs, MaxTokens: maxTokens, Temperature: 0.2,
		Residency: llm.Residency{AllowCrossBorder: true},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}

func nonEmptyLines(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// itoa — internal, avoids importing strconv in this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
