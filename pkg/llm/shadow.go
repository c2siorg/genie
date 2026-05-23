package llm

import (
	"context"
	"sync"
	"time"
)

// ShadowProvider returns Primary's response synchronously and fires Secondary
// asynchronously in the background. Use it to compare two prompt versions or
// two model versions without affecting users.
//
// Shadow results are written to OnShadowResult so callers can persist them
// for offline comparison.
type ShadowProvider struct {
	Primary, Secondary Provider
	OnShadowResult     func(req CompletionRequest, primary, shadow CompletionResponse, err error)
	Timeout            time.Duration
}

// NewShadow wraps two providers.
func NewShadow(primary, secondary Provider, timeout time.Duration) *ShadowProvider {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ShadowProvider{Primary: primary, Secondary: secondary, Timeout: timeout}
}

func (s *ShadowProvider) Name() string   { return s.Primary.Name() + "|shadow:" + s.Secondary.Name() }
func (s *ShadowProvider) Region() string { return s.Primary.Region() }

func (s *ShadowProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	resp, err := s.Primary.Complete(ctx, req)
	if s.Secondary == nil {
		return resp, err
	}
	// Fire shadow async; never block the user response.
	go func() {
		// Detach from request context so the shadow call survives the response.
		bg, cancel := context.WithTimeout(context.Background(), s.Timeout)
		defer cancel()
		shadow, sErr := s.Secondary.Complete(bg, req)
		if s.OnShadowResult != nil {
			s.OnShadowResult(req, resp, shadow, sErr)
		}
	}()
	return resp, err
}

// MemoryShadowSink is the simplest place to drop shadow results — used in tests.
type MemoryShadowSink struct {
	mu      sync.Mutex
	Entries []ShadowEntry
}

// ShadowEntry stores one observed (primary, shadow) pair.
type ShadowEntry struct {
	Req      CompletionRequest
	Primary  CompletionResponse
	Shadow   CompletionResponse
	ShadowErr error
}

// Record satisfies ShadowProvider.OnShadowResult.
func (s *MemoryShadowSink) Record(req CompletionRequest, primary, shadow CompletionResponse, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = append(s.Entries, ShadowEntry{Req: req, Primary: primary, Shadow: shadow, ShadowErr: err})
}
