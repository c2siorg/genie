package eval

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryAnnotationStore_Save_GeneratesID(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	ann := &Annotation{
		AnnotatorID:    "eval-1",
		TraceID:        "trace-001",
		Label:          LabelFail,
		PrimaryFailure: FailureModeDoubleSpend,
		Confidence:     0.95,
		Notes:          "Ledger allowed concurrent payments",
	}

	err := store.Save(ctx, ann)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if ann.ID == "" {
		t.Error("ID not generated")
	}
	if !ann.Timestamp.Before(time.Now().Add(1 * time.Second)) {
		t.Error("Timestamp not set")
	}
}

func TestInMemoryAnnotationStore_Get(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	ann := &Annotation{
		AnnotatorID:   "eval-1",
		TraceID:       "trace-001",
		Label:         LabelPass,
		Confidence:    0.99,
	}
	_ = store.Save(ctx, ann)

	retrieved, err := store.Get(ctx, ann.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != ann.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, ann.ID)
	}
	if retrieved.AnnotatorID != "eval-1" {
		t.Errorf("AnnotatorID mismatch: got %s, want eval-1", retrieved.AnnotatorID)
	}
}

func TestInMemoryAnnotationStore_GetByTraceID(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	// Create multiple annotations for the same trace
	traceID := "trace-001"
	for i := 0; i < 3; i++ {
		ann := &Annotation{
			AnnotatorID:   "eval-" + string(rune(i)),
			TraceID:       traceID,
			Label:         LabelFail,
			PrimaryFailure: FailureModeAMLBypass,
			Confidence:    0.85,
		}
		_ = store.Save(ctx, ann)
	}

	result, err := store.GetByTraceID(ctx, traceID)
	if err != nil {
		t.Fatalf("GetByTraceID failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 annotations, got %d", len(result))
	}

	for _, ann := range result {
		if ann.TraceID != traceID {
			t.Errorf("TraceID mismatch: got %s, want %s", ann.TraceID, traceID)
		}
	}
}

func TestInMemoryAnnotationStore_ListByLabel(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	// Create annotations with different labels
	labels := []AnnotationLabel{LabelPass, LabelFail, LabelPass, LabelUncertain}
	for i, label := range labels {
		ann := &Annotation{
			AnnotatorID: "eval-1",
			TraceID:     "trace-" + string(rune(i)),
			Label:       label,
			Confidence:  0.8,
		}
		_ = store.Save(ctx, ann)
	}

	passes, _ := store.ListByLabel(ctx, LabelPass)
	if len(passes) != 2 {
		t.Errorf("Expected 2 passes, got %d", len(passes))
	}

	fails, _ := store.ListByLabel(ctx, LabelFail)
	if len(fails) != 1 {
		t.Errorf("Expected 1 fail, got %d", len(fails))
	}
}

func TestInMemoryAnnotationStore_ListByFailureMode(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	// Create annotations with different failure mode codes
	modes := []FailureModeCode{
		FailureModeDoubleSpend,
		FailureModeDoubleSpend,
		FailureModeAMLBypass,
	}

	for i, fm := range modes {
		ann := &Annotation{
			AnnotatorID:    "eval-1",
			TraceID:        "trace-" + string(rune(i)),
			Label:          LabelFail,
			PrimaryFailure: fm,
			Confidence:     0.9,
		}
		_ = store.Save(ctx, ann)
	}

	doubleSpends, _ := store.ListByFailureMode(ctx, FailureModeDoubleSpend)
	if len(doubleSpends) != 2 {
		t.Errorf("Expected 2 double-spend failures, got %d", len(doubleSpends))
	}
}

func TestInMemoryAnnotationStore_List_Pagination(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	// Create 10 annotations
	for i := 0; i < 10; i++ {
		ann := &Annotation{
			AnnotatorID: "eval-1",
			TraceID:     "trace-" + string(rune(i)),
			Label:       LabelPass,
			Confidence:  0.8,
		}
		_ = store.Save(ctx, ann)
	}

	// Test pagination
	page1, _ := store.List(ctx, 0, 5)
	if len(page1) != 5 {
		t.Errorf("Page 1: expected 5, got %d", len(page1))
	}

	page2, _ := store.List(ctx, 5, 5)
	if len(page2) != 5 {
		t.Errorf("Page 2: expected 5, got %d", len(page2))
	}

	page3, _ := store.List(ctx, 10, 5)
	if len(page3) != 0 {
		t.Errorf("Page 3 (out of bounds): expected 0, got %d", len(page3))
	}
}

func TestInMemoryAnnotationStore_Delete(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	ann := &Annotation{
		AnnotatorID: "eval-1",
		TraceID:     "trace-001",
		Label:       LabelPass,
		Confidence:  0.8,
	}
	_ = store.Save(ctx, ann)

	_ = store.Delete(ctx, ann.ID)

	_, err := store.Get(ctx, ann.ID)
	if err == nil {
		t.Error("Expected error when getting deleted annotation, got nil")
	}
}

func TestAnnotation_SecondaryFailures(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	ann := &Annotation{
		AnnotatorID:       "eval-1",
		TraceID:           "trace-001",
		Label:             LabelFail,
		PrimaryFailure:    FailureModeSettlementAmountWrong,
		SecondaryFailures: []FailureModeCode{FailureModeTimeout, FailureModeLogicError},
		Confidence:        0.92,
		Notes:             "Amount calculation affected by retry logic during timeout",
	}

	_ = store.Save(ctx, ann)

	retrieved, _ := store.Get(ctx, ann.ID)
	if len(retrieved.SecondaryFailures) != 2 {
		t.Errorf("Expected 2 secondary failures, got %d", len(retrieved.SecondaryFailures))
	}
}

func TestAnnotation_WithRubricScores(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	// Use existing types from rubrics.go
	rubricScores := map[string]*RubricScore{
		"RB-SE-001": {
			Dimension: "idempotency",
			Level:     "good",
			Score:     75,
			Evidence:  "Ledger enforced duplicate detection",
			Timestamp: time.Now(),
		},
		"RB-SE-002": {
			Dimension: "consistency",
			Level:     "excellent",
			Score:     95,
			Evidence:  "All reads show consistent settlement amount",
			Timestamp: time.Now(),
		},
	}

	ann := &Annotation{
		AnnotatorID:   "eval-1",
		TraceID:       "trace-001",
		Label:         LabelPass,
		Confidence:    0.98,
		RubricScores:  rubricScores,
	}

	_ = store.Save(ctx, ann)

	retrieved, _ := store.Get(ctx, ann.ID)
	if len(retrieved.RubricScores) != 2 {
		t.Errorf("Expected 2 rubric scores, got %d", len(retrieved.RubricScores))
	}

	se001 := retrieved.RubricScores["RB-SE-001"]
	if se001.Level != "good" {
		t.Errorf("RB-SE-001 level: got %v, want %v", se001.Level, "good")
	}
}

func TestAnnotation_Deferred(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	ann := &Annotation{
		AnnotatorID:    "eval-1",
		TraceID:        "trace-001",
		Label:          LabelUncertain,
		Confidence:     0.35,
		Deferred:       true,
		DeferredReason: "Requires domain expertise to classify failure mode",
		Notes:          "Edge case: concurrent AML and payment processing",
	}

	_ = store.Save(ctx, ann)

	retrieved, _ := store.Get(ctx, ann.ID)
	if !retrieved.Deferred {
		t.Error("Expected Deferred=true")
	}
	if retrieved.DeferredReason == "" {
		t.Error("DeferredReason not set")
	}
}

func TestAnnotation_Confidence_Range(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	tests := []struct {
		name       string
		confidence float64
		wantErr    bool
	}{
		{"0.0 (guessing)", 0.0, false},
		{"0.5 (uncertain)", 0.5, false},
		{"1.0 (certain)", 1.0, false},
		{"Valid intermediate", 0.75, false},
	}

	for i, tt := range tests {
		ann := &Annotation{
			AnnotatorID: "eval-1",
			TraceID:     "trace-" + string(rune(i)),
			Label:       LabelPass,
			Confidence:  tt.confidence,
		}

		err := store.Save(ctx, ann)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: Save error = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}

		if !tt.wantErr {
			retrieved, _ := store.Get(ctx, ann.ID)
			if retrieved.Confidence != tt.confidence {
				t.Errorf("%s: got confidence %v, want %v", tt.name, retrieved.Confidence, tt.confidence)
			}
		}
	}
}

func TestAnnotation_Timestamp_Updates(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	ann := &Annotation{
		AnnotatorID: "eval-1",
		TraceID:     "trace-001",
		Label:       LabelPass,
		Confidence:  0.8,
	}

	_ = store.Save(ctx, ann)
	originalTimestamp := ann.Timestamp

	// Modify and re-save
	time.Sleep(10 * time.Millisecond)
	ann.Label = LabelFail
	ann.PrimaryFailure = FailureModeAMLBypass
	_ = store.Save(ctx, ann)

	retrieved, _ := store.Get(ctx, ann.ID)

	// Timestamp should remain the same (not updated on re-save)
	if !retrieved.Timestamp.Equal(originalTimestamp) {
		t.Errorf("Timestamp unexpectedly changed: original %v, after save %v", originalTimestamp, retrieved.Timestamp)
	}

	// UpdatedAt should be newer
	if !retrieved.UpdatedAt.After(originalTimestamp) {
		t.Error("UpdatedAt not updated")
	}
}

func TestInMemoryAnnotationStore_GetByAnnotatorID(t *testing.T) {
	store := NewInMemoryAnnotationStore()
	ctx := context.Background()

	// Create annotations from different evaluators
	evaluators := []string{"alice", "bob", "alice"}
	for i, evaluator := range evaluators {
		ann := &Annotation{
			AnnotatorID: evaluator,
			TraceID:     "trace-" + string(rune(i)),
			Label:       LabelPass,
			Confidence:  0.8,
		}
		_ = store.Save(ctx, ann)
	}

	aliceAnnotations, _ := store.GetByAnnotatorID(ctx, "alice")
	if len(aliceAnnotations) != 2 {
		t.Errorf("Expected 2 annotations from alice, got %d", len(aliceAnnotations))
	}

	bobAnnotations, _ := store.GetByAnnotatorID(ctx, "bob")
	if len(bobAnnotations) != 1 {
		t.Errorf("Expected 1 annotation from bob, got %d", len(bobAnnotations))
	}
}
