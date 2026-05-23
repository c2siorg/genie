package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCachedProvider_HitsSecondCall(t *testing.T) {
	calls := 0
	wrap := wrap(func(req CompletionRequest) (CompletionResponse, error) {
		calls++
		return CompletionResponse{Text: "ok", Usage: Usage{PromptTokens: 1}}, nil
	})
	c := NewCachedProvider(wrap, 1*time.Minute)
	req := CompletionRequest{Model: "x", Messages: []Message{{Role: RoleUser, Content: "hi"}}, Residency: Residency{AllowCrossBorder: true}}
	_, _ = c.Complete(context.Background(), req)
	_, _ = c.Complete(context.Background(), req)
	if calls != 1 {
		t.Fatalf("expected single underlying call, got %d", calls)
	}
}

func TestChainProvider_FallsBack(t *testing.T) {
	p := wrap(func(_ CompletionRequest) (CompletionResponse, error) { return CompletionResponse{}, errors.New("boom") })
	s := wrap(func(_ CompletionRequest) (CompletionResponse, error) { return CompletionResponse{Text: "ok"}, nil })
	c := NewChain(p, s)
	r, err := c.Complete(context.Background(), CompletionRequest{Residency: Residency{AllowCrossBorder: true}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "ok" {
		t.Fatalf("expected fallback success, got %q", r.Text)
	}
}

func TestCircuit_TripsAfterThreshold(t *testing.T) {
	bad := wrap(func(_ CompletionRequest) (CompletionResponse, error) { return CompletionResponse{}, errors.New("nope") })
	c := NewCircuit(bad, 2, 10*time.Millisecond)
	for i := 0; i < 2; i++ {
		_, _ = c.Complete(context.Background(), CompletionRequest{Residency: Residency{AllowCrossBorder: true}})
	}
	if c.State() != CircuitOpen {
		t.Fatalf("expected breaker open, got %v", c.State())
	}
	_, err := c.Complete(context.Background(), CompletionRequest{Residency: Residency{AllowCrossBorder: true}})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

// wrap is a tiny adapter so wrapper tests don't need llm.Mock's scripting.
type fnProvider struct {
	fn func(req CompletionRequest) (CompletionResponse, error)
}

func wrap(fn func(req CompletionRequest) (CompletionResponse, error)) *fnProvider {
	return &fnProvider{fn: fn}
}

func (p *fnProvider) Name() string                                              { return "fn" }
func (p *fnProvider) Region() string                                            { return "on-prem" }
func (p *fnProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	return p.fn(req)
}
