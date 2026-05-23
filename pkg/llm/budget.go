package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrBudgetExceeded is returned by BudgetedProvider when a request would
// push the principal over the per-day token quota.
var ErrBudgetExceeded = errors.New("token budget exceeded")

// BudgetLedger tracks token consumption per principal per day.
//
// Genie ships an in-memory implementation; production deployments should
// implement the same interface against Postgres or Redis.
type BudgetLedger interface {
	// Consumed returns the prompt+completion tokens spent today.
	Consumed(ctx context.Context, principal string) (int, error)
	// Add records additional consumption.
	Add(ctx context.Context, principal string, tokens int) error
}

// InMemoryBudget is a clock-aware in-process ledger. Entries reset at UTC
// midnight (kept simple — no rolling-window logic).
type InMemoryBudget struct {
	mu    sync.Mutex
	day   string
	usage map[string]int
}

// NewInMemoryBudget constructs an empty ledger.
func NewInMemoryBudget() *InMemoryBudget {
	return &InMemoryBudget{day: today(), usage: map[string]int{}}
}

func today() string { return time.Now().UTC().Format("2006-01-02") }

func (b *InMemoryBudget) Consumed(_ context.Context, p string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.day != today() {
		b.day = today()
		b.usage = map[string]int{}
	}
	return b.usage[p], nil
}

func (b *InMemoryBudget) Add(_ context.Context, p string, tokens int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.day != today() {
		b.day = today()
		b.usage = map[string]int{}
	}
	b.usage[p] += tokens
	return nil
}

// BudgetedProvider wraps any Provider with a per-principal daily token cap.
// The principal id comes from CompletionRequest.Residency.Region as a stand-in
// for now; production should add a User-scoped field to CompletionRequest.
type BudgetedProvider struct {
	Inner   Provider
	Ledger  BudgetLedger
	MaxDay  int // hard cap; 0 means no limit
}

// NewBudgeted wraps p with a daily cap. Use 0 for "no limit".
func NewBudgeted(p Provider, ledger BudgetLedger, maxPerDay int) *BudgetedProvider {
	return &BudgetedProvider{Inner: p, Ledger: ledger, MaxDay: maxPerDay}
}

func (b *BudgetedProvider) Name() string   { return b.Inner.Name() + "+budget" }
func (b *BudgetedProvider) Region() string { return b.Inner.Region() }

// Complete checks the ledger, calls the inner provider, then records usage.
//
// principal is read from req.Residency.Region for now (cheap stand-in).
// Production should pass an explicit principal id (user/org) via a new field.
func (b *BudgetedProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	principal := req.Residency.Region
	if principal == "" {
		principal = "default"
	}
	used, err := b.Ledger.Consumed(ctx, principal)
	if err != nil {
		return CompletionResponse{}, err
	}
	if b.MaxDay > 0 && used >= b.MaxDay {
		return CompletionResponse{}, fmt.Errorf("%w: principal=%s used=%d cap=%d", ErrBudgetExceeded, principal, used, b.MaxDay)
	}
	resp, err := b.Inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	spent := resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	if spent == 0 {
		spent = len(resp.Text) / 4 // rough fallback if usage isn't reported
	}
	_ = b.Ledger.Add(ctx, principal, spent)
	return resp, nil
}
