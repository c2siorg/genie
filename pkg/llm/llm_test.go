package llm

import (
	"context"
	"errors"
	"testing"
)

func TestMock_ReturnsQueuedResponses(t *testing.T) {
	m := NewMock()
	m.Responses = []CompletionResponse{{Text: "first"}, {Text: "second"}}
	r1, _ := m.Complete(context.Background(), CompletionRequest{Model: "x"})
	r2, _ := m.Complete(context.Background(), CompletionRequest{Model: "x"})
	r3, _ := m.Complete(context.Background(), CompletionRequest{Model: "x"})
	if r1.Text != "first" || r2.Text != "second" || r3.Text != "second" {
		t.Fatalf("unexpected sequence: %q, %q, %q", r1.Text, r2.Text, r3.Text)
	}
}

func TestMock_DeniesCrossBorder(t *testing.T) {
	m := NewMock()
	m.RegionVal = "us"
	_, err := m.Complete(context.Background(), CompletionRequest{
		Residency: Residency{Region: "in", AllowCrossBorder: false, Classification: "pii"},
	})
	if !errors.Is(err, ErrResidencyDenied) {
		t.Fatalf("want residency denied, got %v", err)
	}
}
