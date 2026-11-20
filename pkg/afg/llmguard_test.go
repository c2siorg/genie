package afg

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

// guardTextResponse builds a one-update RunFunc that returns text as the sole
// assistant message, ignoring ctx entirely (so tests can prove GuardMiddleware
// itself enforces the deadline rather than relying on provider cooperation).
func guardTextRunFunc(text string, sleep time.Duration, calls *int, mu *sync.Mutex) agent.RunFunc {
	return func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			if mu != nil {
				mu.Lock()
				*calls++
				mu.Unlock()
			}
			if sleep > 0 {
				time.Sleep(sleep)
			}
			yield(&agent.ResponseUpdate{
				Role:     message.RoleAssistant,
				Contents: message.Contents{&message.TextContent{Text: text}},
			}, nil)
		}
	}
}

// (a) A next RunFunc that sleeps well past Timeout — and ignores ctx entirely —
// must still be cut off by GuardMiddleware's deadline: Collect returns a
// deadline error, and it returns fast (long before the mock's sleep finishes).
func TestGuard_DeadlineCutsOffSlowProvider(t *testing.T) {
	guard := NewGuardMiddleware("slow-agent", GuardConfig{
		Timeout:          20 * time.Millisecond,
		CircuitThreshold: 5,
		CircuitCooldown:  30 * time.Second,
		BudgetPerDay:     0,
		CacheTTL:         time.Minute,
	})

	var calls int
	var mu sync.Mutex
	next := guardTextRunFunc("too-late", 300*time.Millisecond, &calls, &mu)

	msgs := []*message.Message{message.NewText("slow please")}

	start := time.Now()
	stream := guard.Run(next, context.Background(), msgs)
	_, err := agent.ResponseStream(stream).Collect()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected a deadline/timeout error, got: %v", err)
	}
	// The mock sleeps 300ms; a correct cutoff returns well before that.
	if elapsed >= 150*time.Millisecond {
		t.Fatalf("guard did not cut the slow provider off promptly: took %s", elapsed)
	}
}

// (b) Two identical calls: the second must be served from cache WITHOUT
// invoking next again.
func TestGuard_CacheHitSkipsProvider(t *testing.T) {
	guard := NewGuardMiddleware("cache-agent", GuardConfig{
		Timeout:          2 * time.Second,
		CircuitThreshold: 5,
		CircuitCooldown:  30 * time.Second,
		BudgetPerDay:     0,
		CacheTTL:         time.Minute,
	})

	var calls int
	var mu sync.Mutex
	next := guardTextRunFunc("hello-once", 0, &calls, &mu)

	msgs := []*message.Message{message.NewText("same input every time")}

	resp1, err := agent.ResponseStream(guard.Run(next, context.Background(), msgs)).Collect()
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if got := ResponseText(resp1); got != "hello-once" {
		t.Fatalf("first call: unexpected text %q", got)
	}

	resp2, err := agent.ResponseStream(guard.Run(next, context.Background(), msgs)).Collect()
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if got := ResponseText(resp2); got != "hello-once" {
		t.Fatalf("second call: unexpected text %q", got)
	}

	mu.Lock()
	n := calls
	mu.Unlock()
	if n != 1 {
		t.Fatalf("expected next to be invoked exactly once (second call served from cache), got %d calls", n)
	}
}

// denyAllGate is a trivial governance.Policy test double that always denies —
// used to prove ordering: composing []agent.Middleware{GovMiddleware{deny}, guard}
// must short-circuit at the gate BEFORE guard (and therefore before the
// provider) ever runs.
type denyAllGate struct{}

func (denyAllGate) Evaluate(_ context.Context, _ protocol.Message) (governance.PolicyResult, error) {
	return governance.PolicyResult{Decision: governance.DecisionDeny, Reason: "test: deny everything"}, nil
}

// (c) Composing GovMiddleware (outermost) with GuardMiddleware (innermost) on a
// raw agent.New(...): a governance denial must short-circuit BEFORE the guard
// (and therefore the provider) ever runs.
func TestGuard_GovernanceDenialShortCircuitsBeforeGuard(t *testing.T) {
	guard := NewGuardMiddleware("gated-agent", DefaultGuardConfig())

	var calls int
	var mu sync.Mutex
	next := guardTextRunFunc("SHOULD-NOT-APPEAR", 0, &calls, &mu)

	a := agent.New(
		agent.ProviderConfig{ProviderName: "test-provider", Run: next},
		agent.Config{
			Name:                "gated-agent",
			Middlewares:         []agent.Middleware{GovMiddleware{Gate: denyAllGate{}}, guard},
			DisableFuncAutoCall: true,
		},
	)

	_, err := a.RunText(context.Background(), "anything at all").Collect()
	if err == nil {
		t.Fatal("expected a governance denial error")
	}
	if !strings.Contains(err.Error(), "governance denied") {
		t.Fatalf("expected a governance-denied error, got: %v", err)
	}

	mu.Lock()
	n := calls
	mu.Unlock()
	if n != 0 {
		t.Fatalf("provider RunFunc must not run when the gate denies (guard must not reach next either), got %d calls", n)
	}
}
