package afg

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

// GuardMiddleware re-hosts Genie's legacy LLM wrapper chain
// (cmd/api/llmstack.go: Circuit -> Deadline -> Budget -> Cache -> Cost -> Ollama)
// onto the agent-framework RunFunc path, as an agent.Middleware.
//
// Honest limitation: this bounds the *streaming* RunFunc at the message layer.
// It is NOT the byte-identical legacy pkg/llm.Provider chain (there is no
// CompletionRequest/CompletionResponse, no per-call token Usage from the
// provider, and cost/observability stays out of scope here) — but it enforces
// the same five bounds: a circuit breaker, a per-call deadline, a per-principal
// daily budget, a TTL cache, and (implicitly, via the cache/budget bookkeeping)
// the same cost-shaped accounting the legacy Cost wrapper observed.
//
// Ordering contract: GuardMiddleware is designed to sit INSIDE GovMiddleware
// (gov outermost, guard innermost, provider innermost of all). It does not call
// governance.Policy.Evaluate itself — that is GovMiddleware's job — so composing
// []agent.Middleware{GovMiddleware{...}, guard} gives you both seams without
// duplicating the gate.
type GuardMiddleware struct {
	name  string
	cfg   GuardConfig
	state *guardState
}

// guardState is the mutable core shared by every copy of a GuardMiddleware value
// (the constructor returns GuardMiddleware, not *GuardMiddleware, so the mutable
// bits live behind this pointer to survive being copied into a []agent.Middleware
// slice / interface value).
type guardState struct {
	mu     sync.Mutex
	budget *llm.InMemoryBudget

	// circuit breaker: consecutive failures since the last success/reset.
	failures         int
	circuitOpenUntil time.Time // zero value == closed

	cache map[string]guardCacheEntry
}

type guardCacheEntry struct {
	text      string
	expiresAt time.Time
}

// GuardConfig bounds a single agent's path through GuardMiddleware.
type GuardConfig struct {
	// Timeout bounds a single call to next (the wrapped provider RunFunc).
	Timeout time.Duration
	// CircuitThreshold is the number of consecutive failures that trips the
	// breaker open.
	CircuitThreshold int
	// CircuitCooldown is how long the breaker stays open before allowing a
	// half-open probe.
	CircuitCooldown time.Duration
	// BudgetPerDay is the per-agent daily cap on approximate output tokens.
	// 0 (or negative) means unlimited.
	BudgetPerDay int
	// CacheTTL is how long an identical (name, input) pair may be served from
	// cache without calling next again.
	CacheTTL time.Duration
}

// DefaultGuardConfig reads the SAME env vars as cmd/api/llmstack.go's ollamaStack,
// so the framework path and the legacy bus path are bounded identically by
// default:
//
//	GENIE_LLM_TIMEOUT      per-call timeout seconds, default 30
//	GENIE_LLM_CIRCUIT      consecutive-error threshold, default 5
//	GENIE_LLM_BUDGET       daily approx-token cap per agent, default 1_000_000
//	GENIE_LLM_CACHE_TTL    cache TTL in seconds, default 600
//
// The circuit cooldown is not env-configurable in llmstack.go either — it is a
// fixed 30s there, so DefaultGuardConfig fixes it at 30s too.
func DefaultGuardConfig() GuardConfig {
	return GuardConfig{
		Timeout:          time.Duration(guardEnvInt("GENIE_LLM_TIMEOUT", 30)) * time.Second,
		CircuitThreshold: guardEnvInt("GENIE_LLM_CIRCUIT", 5),
		CircuitCooldown:  30 * time.Second,
		BudgetPerDay:     guardEnvInt("GENIE_LLM_BUDGET", 1_000_000),
		CacheTTL:         time.Duration(guardEnvInt("GENIE_LLM_CACHE_TTL", 600)) * time.Second,
	}
}

func guardEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// NewGuardMiddleware builds a GuardMiddleware for a single agent. name is the
// agent id (e.g. Config.Name passed to agent.New) and scopes both the cache key
// and the budget principal, so two different agents sharing a GuardConfig never
// collide on cache hits or drain a shared budget.
func NewGuardMiddleware(name string, cfg GuardConfig) GuardMiddleware {
	return GuardMiddleware{
		name: name,
		cfg:  cfg,
		state: &guardState{
			budget: llm.NewInMemoryBudget(),
			cache:  map[string]guardCacheEntry{},
		},
	}
}

// ErrGuardCircuitOpen is returned (via the response stream) when the breaker is
// tripped and calls are being short-circuited during the cooldown window.
var ErrGuardCircuitOpen = errors.New("afg guard: circuit breaker open")

// Run implements agent.Middleware. Call order (outermost -> innermost, all
// within this one middleware): circuit-open check -> cache lookup (return on
// hit, next is NOT called) -> budget check -> deadline -> next -> on success,
// cache the collected text and reset the circuit; on error, trip the circuit
// counter.
func (g GuardMiddleware) Run(next agent.RunFunc, ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
	if err := g.circuitCheck(); err != nil {
		return errStream(err)
	}

	key := g.name + "\x00" + messagesText(msgs)
	if text, ok := g.cacheLookup(key); ok {
		return guardCachedStream(text)
	}

	if err := g.budgetCheck(ctx); err != nil {
		return errStream(err)
	}

	timeout := g.cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dctx, cancel := context.WithTimeout(ctx, timeout)

	return func(yield func(*agent.ResponseUpdate, error) bool) {
		defer cancel()

		// next is invoked in its own goroutine so a provider that ignores ctx
		// cancellation (as some mocks/streaming clients do) still gets cut off
		// from the caller's perspective at the deadline — the caller does not
		// block waiting for a run that has already blown its budget of time.
		// This can leak a goroutine if next never returns and never respects
		// ctx; that is the same risk context.WithTimeout always carries against
		// a callee that ignores context, and is the honest limitation noted on
		// GuardMiddleware.
		type result struct {
			updates []*agent.ResponseUpdate
			err     error
		}
		done := make(chan result, 1)
		go func() {
			var updates []*agent.ResponseUpdate
			for update, err := range next(dctx, msgs, opts...) {
				if err != nil {
					done <- result{updates, err}
					return
				}
				updates = append(updates, update)
			}
			done <- result{updates, nil}
		}()

		select {
		case <-dctx.Done():
			g.recordFailure()
			yield(nil, fmt.Errorf("afg guard: deadline exceeded after %s: %w", timeout, dctx.Err()))
			return
		case res := <-done:
			if res.err != nil {
				g.recordFailure()
				yield(nil, res.err)
				return
			}
			g.recordSuccess()
			var text strings.Builder
			for _, u := range res.updates {
				text.WriteString(u.String())
				if !yield(u, nil) {
					return
				}
			}
			full := text.String()
			g.cacheStore(key, full)
			_ = g.state.budget.Add(context.Background(), g.name, guardApproxTokens(full))
		}
	}
}

func guardCachedStream(text string) iter.Seq2[*agent.ResponseUpdate, error] {
	return func(yield func(*agent.ResponseUpdate, error) bool) {
		yield(&agent.ResponseUpdate{
			Role:     message.RoleAssistant,
			Contents: message.Contents{&message.TextContent{Text: text}},
		}, nil)
	}
}

// circuitCheck returns ErrGuardCircuitOpen while the breaker is tripped. Once
// the cooldown elapses it clears the open marker and allows a half-open probe
// through; recordFailure / recordSuccess decide whether it reopens or resets.
func (g GuardMiddleware) circuitCheck() error {
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.circuitOpenUntil.IsZero() {
		return nil
	}
	if time.Now().Before(s.circuitOpenUntil) {
		return ErrGuardCircuitOpen
	}
	s.circuitOpenUntil = time.Time{} // half-open: let the next call probe
	return nil
}

func (g GuardMiddleware) recordFailure() {
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures++
	threshold := g.cfg.CircuitThreshold
	if threshold <= 0 {
		threshold = 5
	}
	if s.failures >= threshold {
		cooldown := g.cfg.CircuitCooldown
		if cooldown <= 0 {
			cooldown = 30 * time.Second
		}
		s.circuitOpenUntil = time.Now().Add(cooldown)
	}
}

func (g GuardMiddleware) recordSuccess() {
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = 0
	s.circuitOpenUntil = time.Time{}
}

func (g GuardMiddleware) cacheLookup(key string) (string, bool) {
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[key]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.text, true
}

func (g GuardMiddleware) cacheStore(key, text string) {
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	ttl := g.cfg.CacheTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	s.cache[key] = guardCacheEntry{text: text, expiresAt: time.Now().Add(ttl)}
}

// budgetCheck consults the daily ledger for this agent's principal (its name).
// It does NOT record spend — that happens once the call actually succeeds, in
// Run, via guardApproxTokens.
func (g GuardMiddleware) budgetCheck(ctx context.Context) error {
	if g.cfg.BudgetPerDay <= 0 {
		return nil
	}
	used, err := g.state.budget.Consumed(ctx, g.name)
	if err != nil {
		return err
	}
	if used >= g.cfg.BudgetPerDay {
		return fmt.Errorf("afg guard: %w: agent=%s used=%d cap=%d", llm.ErrBudgetExceeded, g.name, used, g.cfg.BudgetPerDay)
	}
	return nil
}

// guardApproxTokens approximates "tokens" as output length / 4 (the same rough
// fallback cmd/api/llmstack.go's BudgetedProvider uses when a provider doesn't
// report real Usage). The framework's ResponseUpdate stream has no
// prompt/completion token count at this layer, so this is enforceable-but-approximate
// by design, per the task brief.
func guardApproxTokens(text string) int {
	n := len(text) / 4
	if n == 0 && text != "" {
		n = 1
	}
	return n
}
