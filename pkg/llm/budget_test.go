package llm

import (
	"context"
	"errors"
	"testing"
)

func TestBudgeted_StopsAtCap(t *testing.T) {
	mock := NewMock()
	mock.Responses = []CompletionResponse{
		{Text: "ok", Usage: Usage{PromptTokens: 30, CompletionTokens: 30}},
		{Text: "ok", Usage: Usage{PromptTokens: 30, CompletionTokens: 30}},
		{Text: "ok", Usage: Usage{PromptTokens: 30, CompletionTokens: 30}},
	}
	bp := NewBudgeted(mock, NewInMemoryBudget(), 100)

	// First call: 60 tokens accumulated, still under cap.
	if _, err := bp.Complete(context.Background(), CompletionRequest{Residency: Residency{Region: "u1", AllowCrossBorder: true}}); err != nil {
		t.Fatal(err)
	}
	// Second call: 120 tokens — already over cap when we check (used=60, cap=100, allowed).
	if _, err := bp.Complete(context.Background(), CompletionRequest{Residency: Residency{Region: "u1", AllowCrossBorder: true}}); err != nil {
		t.Fatal(err)
	}
	// Third call: used=120 >= cap=100, deny.
	_, err := bp.Complete(context.Background(), CompletionRequest{Residency: Residency{Region: "u1", AllowCrossBorder: true}})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("want ErrBudgetExceeded, got %v", err)
	}

	// Different principal has its own bucket.
	if _, err := bp.Complete(context.Background(), CompletionRequest{Residency: Residency{Region: "u2", AllowCrossBorder: true}}); err != nil {
		t.Fatal(err)
	}
}
