package afg

import (
	"context"
	"iter"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

// TestRetrievalMiddleware_InjectsContext proves RetrievalMiddleware grounds a
// run: it queries the retriever with the request text and prepends a system
// message carrying the retrieved chunks before calling next.
func TestRetrievalMiddleware_InjectsContext(t *testing.T) {
	docs := []string{
		"Equity mutual funds held for over 12 months qualify for long-term capital gains (LTCG) tax treatment in India.",
		"An emergency fund should cover at least 6 months of essential expenses in a liquid instrument.",
	}
	r := NewOfflineRetriever(docs)

	var captured []*message.Message
	next := func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		captured = msgs
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			yield(&agent.ResponseUpdate{
				Role:     message.RoleAssistant,
				Contents: message.Contents{&message.TextContent{Text: "ok"}},
			}, nil)
		}
	}

	mw := RetrievalMiddleware{R: r, TopK: 2}
	in := []*message.Message{message.NewText("What is the tax treatment for long-term equity mutual fund gains?")}

	resp, err := agent.ResponseStream(mw.Run(next, context.Background(), in)).Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response")
	}
	if len(captured) != len(in)+1 {
		t.Fatalf("expected one context message prepended to the original %d, got %d messages", len(in), len(captured))
	}
	if captured[0].Role != message.RoleSystem {
		t.Fatalf("expected first message to be the injected system context, got role %q", captured[0].Role)
	}
	got := messagesText(captured[:1])
	if !strings.Contains(got, "long-term capital gains") {
		t.Fatalf("expected known doc snippet in injected context, got: %q", got)
	}
	if !strings.HasPrefix(got, "Context:") {
		t.Fatalf("expected injected context under a Context: header, got: %q", got)
	}
	// The original messages must still reach next, unmodified, after the context.
	if captured[1] != in[0] {
		t.Fatal("expected the original message to be passed through to next")
	}
}

// denyAllPolicy is a test double implementing governance.Policy that denies
// every message, regardless of content.
type denyAllPolicy struct{}

func (denyAllPolicy) Evaluate(_ context.Context, _ protocol.Message) (governance.PolicyResult, error) {
	return governance.PolicyResult{Decision: governance.DecisionDeny, Reason: "test deny-all policy"}, nil
}

// countingRetriever panics if Retrieve is ever called, and records the call in
// *called first so a failing assertion still reports an accurate count.
type countingRetriever struct {
	called *int
}

func (c countingRetriever) Retrieve(_ context.Context, _ string, _ int) ([]string, error) {
	*c.called++
	panic("retrieval must not run on a governance-denied request")
}

// TestGovernedAdvisory_DenyShortCircuitsBeforeRetrieval proves GovMiddleware
// is outermost: a governance denial must short-circuit BEFORE
// RetrievalMiddleware ever calls the retriever. If retrieval ran before the
// gate, a denied request could still trigger (and leak query text to) the
// retrieval backend.
func TestGovernedAdvisory_DenyShortCircuitsBeforeRetrieval(t *testing.T) {
	called := 0
	r := countingRetriever{called: &called}

	a := NewGovernedAdvisory(denyAllPolicy{}, "advisory_denied", "be a careful financial advisor", "llama3.1", r, 3)
	_, err := a.RunText(context.Background(), "Should I switch to the new tax regime this year?").Collect()
	if err == nil {
		t.Fatal("expected governance denial")
	}
	if !strings.Contains(err.Error(), "governance denied") {
		t.Fatalf("expected governance-denied error, got: %v", err)
	}
	if called != 0 {
		t.Fatalf("retriever must not be called when governance denies the request, called %d time(s)", called)
	}
}
