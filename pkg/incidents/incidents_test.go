package incidents

import (
	"context"
	"testing"
	"time"
)

func TestIncident_ValidateDefaults(t *testing.T) {
	i := Incident{UseCase: "credit", Description: "model dropped a class"}
	if err := (&i).Validate(); err != nil {
		t.Fatal(err)
	}
	if i.Severity != SeverityLow {
		t.Fatalf("default severity should be low, got %s", i.Severity)
	}
	if i.FailureMode != FailureUnknown {
		t.Fatalf("default failure mode should be unknown, got %s", i.FailureMode)
	}
	if i.ID == "" {
		t.Fatal("expected generated id")
	}
}

func TestInMemoryStore_AndGrade(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	// First incident of its kind -> first offense.
	_, _ = store.Create(ctx, Incident{
		UseCase: "credit", Description: "x", FailureMode: FailureBias,
	})
	g, err := Grade(ctx, store, FailureBias, 30)
	if err != nil {
		t.Fatal(err)
	}
	if g.IsFirstOffense {
		t.Fatal("after the create, the next-incident grade should not be first offense")
	}
	if g.RecentCount != 1 {
		t.Fatalf("recent count: %d", g.RecentCount)
	}

	// Old incident outside the window should be ignored.
	old := Incident{
		UseCase: "credit", Description: "old", FailureMode: FailureBias,
		OccurredAt: time.Now().AddDate(0, 0, -60),
	}
	_, _ = store.Create(ctx, old)
	g, _ = Grade(ctx, store, FailureBias, 30)
	if g.RecentCount != 1 {
		t.Fatalf("old incident should be outside window, got count %d", g.RecentCount)
	}
}
