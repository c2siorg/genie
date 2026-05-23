package workflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkflow_RunsInTopoOrder(t *testing.T) {
	var order []string
	w := New(nil)
	w.Add(Step{ID: "c", DependsOn: []string{"a", "b"}, Run: func(_ context.Context, _ State) error { order = append(order, "c"); return nil }})
	w.Add(Step{ID: "a", Run: func(_ context.Context, _ State) error { order = append(order, "a"); return nil }})
	w.Add(Step{ID: "b", DependsOn: []string{"a"}, Run: func(_ context.Context, _ State) error { order = append(order, "b"); return nil }})
	if err := w.Run(context.Background(), State{}); err != nil {
		t.Fatal(err)
	}
	if order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("unexpected order: %v", order)
	}
}

func TestWorkflow_CompensatesOnFailure(t *testing.T) {
	var compensated []string
	w := New(nil)
	w.Add(Step{ID: "a",
		Run:        func(_ context.Context, _ State) error { return nil },
		Compensate: func(_ context.Context, _ State) error { compensated = append(compensated, "a"); return nil },
	})
	w.Add(Step{ID: "b", DependsOn: []string{"a"},
		Run: func(_ context.Context, _ State) error { return errors.New("boom") },
	})
	err := w.Run(context.Background(), State{})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(compensated) != 1 || compensated[0] != "a" {
		t.Fatalf("expected compensation on a, got %v", compensated)
	}
}

func TestWorkflow_AwaitsApproval(t *testing.T) {
	w := New(nil)
	w.Add(Step{ID: "a", RequireApproval: true, Run: func(_ context.Context, _ State) error { return nil }})

	done := make(chan error, 1)
	go func() { done <- w.Run(context.Background(), State{}) }()

	// Brief wait to ensure Run blocks at approval.
	time.Sleep(20 * time.Millisecond)
	if !w.ApproveStep("a") {
		t.Fatal("expected approval to be pending")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestWorkflow_CycleDetected(t *testing.T) {
	w := New(nil)
	w.Add(Step{ID: "a", DependsOn: []string{"b"}, Run: func(_ context.Context, _ State) error { return nil }})
	w.Add(Step{ID: "b", DependsOn: []string{"a"}, Run: func(_ context.Context, _ State) error { return nil }})
	if err := w.Run(context.Background(), State{}); err == nil {
		t.Fatal("expected cycle error")
	}
}
