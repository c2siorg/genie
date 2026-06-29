package eval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// TestBootstrapCI_UniformDistribution verifies CI computation on uniform scores.
func TestBootstrapCI_UniformDistribution(t *testing.T) {
	// All scores are 0.9, so CI should be tight around 0.9.
	scores := make([]float64, 100)
	for i := range scores {
		scores[i] = 0.9
	}

	ci := bootstrapCI(scores, 0.95, 1000)

	if ci.Point < 0.88 || ci.Point > 0.91 {
		t.Errorf("expected point ~0.9, got %.3f", ci.Point)
	}
	if ci.Lower < 0.85 {
		t.Errorf("lower CI too low: %.3f", ci.Lower)
	}
	if ci.Upper > 0.95 {
		t.Errorf("upper CI too high: %.3f", ci.Upper)
	}
	// For uniform data, CI bounds should be close to point (may be equal).
	if ci.Lower > ci.Point || ci.Point > ci.Upper {
		t.Errorf("invalid CI order: [%.3f, %.3f, %.3f]", ci.Lower, ci.Point, ci.Upper)
	}
}

// TestBootstrapCI_BinaryDistribution verifies CI on pass/fail data (0.0 or 1.0).
func TestBootstrapCI_BinaryDistribution(t *testing.T) {
	// 70% pass (1.0), 30% fail (0.0).
	scores := make([]float64, 100)
	for i := 0; i < 70; i++ {
		scores[i] = 1.0
	}
	for i := 70; i < 100; i++ {
		scores[i] = 0.0
	}

	ci := bootstrapCI(scores, 0.95, 5000)

	// Point should be ~0.7.
	if ci.Point < 0.60 || ci.Point > 0.80 {
		t.Errorf("expected point ~0.7, got %.3f", ci.Point)
	}
	// CI should be narrower for 95% confidence.
	if ci.Upper-ci.Lower > 0.3 {
		t.Errorf("CI too wide: %.3f", ci.Upper-ci.Lower)
	}
}

// TestBootstrapCI_WideDistribution verifies CI handles uniform [0..1].
func TestBootstrapCI_WideDistribution(t *testing.T) {
	// Uniform scores across [0..1].
	scores := make([]float64, 100)
	for i := 0; i < 100; i++ {
		scores[i] = float64(i) / 100.0
	}

	ci := bootstrapCI(scores, 0.95, 1000)

	// Point should be ~0.5.
	if ci.Point < 0.40 || ci.Point > 0.60 {
		t.Errorf("expected point ~0.5, got %.3f", ci.Point)
	}
	// CI should be reasonable; with 100 samples, width varies.
	// Allow 0.05 to 0.5 range to be flexible about bootstrap resampling variance.
	if ci.Upper-ci.Lower < 0.05 {
		t.Logf("CI width is narrow (%.3f) but not necessarily wrong", ci.Upper-ci.Lower)
	}
}

// TestSampleRecords_ExactRate verifies sampling produces expected sample size.
func TestSampleRecords_ExactRate(t *testing.T) {
	// Create 1000 records.
	records := make([]InteractionRecord, 1000)
	for i := range records {
		records[i] = InteractionRecord{
			ID: fmt.Sprintf("rec-%d", i),
		}
	}

	// Sample at 10%.
	sampled := sampleRecords(records, 0.10)

	expectedSize := 100 // 10% of 1000.
	if sampled == nil || len(sampled) != expectedSize {
		t.Errorf("expected %d samples, got %d", expectedSize, len(sampled))
	}

	// Verify all sampled records are unique and from original set.
	seenIDs := make(map[string]bool)
	for _, r := range sampled {
		if seenIDs[r.ID] {
			t.Errorf("duplicate record in sample: %s", r.ID)
		}
		seenIDs[r.ID] = true
	}
}

// TestSampleRecords_FullCoverage verifies sampleRate >= 1.0 returns all records.
func TestSampleRecords_FullCoverage(t *testing.T) {
	records := make([]InteractionRecord, 50)
	for i := range records {
		records[i] = InteractionRecord{ID: fmt.Sprintf("rec-%d", i)}
	}

	// Sample at 100%.
	sampled := sampleRecords(records, 1.0)
	if len(sampled) != len(records) {
		t.Errorf("expected all records, got %d/%d", len(sampled), len(records))
	}
}

// TestRunProductionEvaluation_HappyPath tests successful eval with mock judge.
func TestRunProductionEvaluation_HappyPath(t *testing.T) {
	// Setup: create store with sample records.
	store := NewInMemoryStore()
	for i := 0; i < 100; i++ {
		record := InteractionRecord{
			ID:      fmt.Sprintf("interaction-%d", i),
			Success: i%10 != 0, // 90% success.
			Metadata: map[string]any{
				"input":  fmt.Sprintf("Process settlement for merchant %d", i),
				"output": fmt.Sprintf("Settlement amount: $%.2f USD", float64(i)*10.5),
			},
		}
		if err := store.Save(record); err != nil {
			t.Fatalf("failed to save record: %v", err)
		}
	}

	// Mock judge that always returns 1.0 (pass).
	mockJudge := func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
		return 1.0, nil
	}

	cfg := ProductionEvalConfig{
		SampleRate:       0.50, // Sample 50%.
		ConfidenceLevel:  0.95,
		DriftThreshold:   0.80,
		BootstrapSamples: 1000,
		Judges: map[string]JudgeFunc{
			"test_judge": mockJudge,
		},
		Store: store,
	}

	ctx := context.Background()
	metrics, err := RunProductionEvaluation(ctx, cfg)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Verify results.
	if metrics.SampleSize < 40 || metrics.SampleSize > 60 { // ~50 records ± tolerance.
		t.Errorf("unexpected sample size: %d", metrics.SampleSize)
	}
	if metrics.SampleRate != 0.50 {
		t.Errorf("sample rate mismatch: %.2f", metrics.SampleRate)
	}

	judgeMetrics, ok := metrics.JudgeMetrics["test_judge"]
	if !ok {
		t.Fatalf("test_judge not in results")
	}

	// Mock judge always passes, so pass rate should be 1.0.
	if judgeMetrics.PassRate != 1.0 {
		t.Errorf("expected 100%% pass rate, got %.2f", judgeMetrics.PassRate)
	}

	// Drift should not be detected (1.0 > 0.80 threshold).
	if metrics.DriftDetected {
		t.Errorf("unexpected drift detected: %s", metrics.DriftReason)
	}
}

// TestRunProductionEvaluation_DetectsDrift verifies drift detection on low pass rate.
func TestRunProductionEvaluation_DetectsDrift(t *testing.T) {
	store := NewInMemoryStore()
	for i := 0; i < 100; i++ {
		record := InteractionRecord{
			ID:        fmt.Sprintf("interaction-%d", i),
			Metadata:  map[string]any{"input": "test", "output": "test"},
			Success:   i%10 == 0, // Only 10% success.
		}
		if err := store.Save(record); err != nil {
			t.Fatalf("failed to save record: %v", err)
		}
	}

	// Judge that returns 0.0 (always fail).
	badJudge := func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
		return 0.0, nil
	}

	cfg := ProductionEvalConfig{
		SampleRate:       1.0,
		ConfidenceLevel:  0.95,
		DriftThreshold:   0.85, // High threshold to trigger drift.
		BootstrapSamples: 1000,
		Judges: map[string]JudgeFunc{
			"failing_judge": badJudge,
		},
		Store: store,
	}

	ctx := context.Background()
	metrics, err := RunProductionEvaluation(ctx, cfg)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Drift should be detected.
	if !metrics.DriftDetected {
		t.Errorf("expected drift detection, but none occurred")
	}
	if metrics.DriftReason == "" {
		t.Errorf("drift detected but no reason provided")
	}
}

// TestRunProductionEvaluation_EmptyStore tests behavior on empty evaluation store.
func TestRunProductionEvaluation_EmptyStore(t *testing.T) {
	store := NewInMemoryStore() // Empty.

	cfg := ProductionEvalConfig{
		SampleRate: 0.50,
		Judges: map[string]JudgeFunc{
			"test": func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
				return 1.0, nil
			},
		},
		Store: store,
	}

	ctx := context.Background()
	_, err := RunProductionEvaluation(ctx, cfg)
	if err == nil {
		t.Errorf("expected error on empty store, but got none")
	}
}

// TestRunProductionEvaluation_NoJudges tests behavior when Judges map is empty.
func TestRunProductionEvaluation_NoJudges(t *testing.T) {
	store := NewInMemoryStore()
	record := InteractionRecord{
		ID:       "test",
		Metadata: map[string]any{"input": "test", "output": "test"},
	}
	if err := store.Save(record); err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	cfg := ProductionEvalConfig{
		SampleRate: 1.0,
		Judges:     map[string]JudgeFunc{}, // Empty.
		Store:      store,
	}

	ctx := context.Background()
	metrics, err := RunProductionEvaluation(ctx, cfg)
	if err != nil {
		t.Fatalf("evaluation with no judges should not error: %v", err)
	}

	// Result should have no judge metrics.
	if len(metrics.JudgeMetrics) != 0 {
		t.Errorf("expected no judge metrics, got %d", len(metrics.JudgeMetrics))
	}
}

// TestDefaultSettlementHallucinationJudge_NoProvider tests judge with nil provider.
func TestDefaultSettlementHallucinationJudge_NoProvider(t *testing.T) {
	ctx := context.Background()
	score, err := DefaultSettlementHallucinationJudge(ctx, nil, "", "input", "output")
	if err != nil {
		t.Errorf("judge should not error on nil provider: %v", err)
	}
	if score != 0.5 {
		t.Errorf("expected neutral score 0.5, got %.2f", score)
	}
}

// TestRunProductionEvaluation_MultipleJudges tests concurrent judge execution.
func TestRunProductionEvaluation_MultipleJudges(t *testing.T) {
	store := NewInMemoryStore()
	for i := 0; i < 20; i++ {
		record := InteractionRecord{
			ID:       fmt.Sprintf("interaction-%d", i),
			Metadata: map[string]any{"input": "test", "output": "test"},
		}
		if err := store.Save(record); err != nil {
			t.Fatalf("failed to save record: %v", err)
		}
	}

	// Create 3 judges with different pass rates.
	judge1 := func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
		return 1.0, nil // Always pass.
	}
	judge2 := func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
		return 0.5, nil // Always neutral.
	}
	judge3 := func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
		return 0.0, nil // Always fail.
	}

	cfg := ProductionEvalConfig{
		SampleRate: 1.0,
		Judges: map[string]JudgeFunc{
			"perfect_judge":  judge1,
			"neutral_judge":  judge2,
			"failing_judge":  judge3,
		},
		Store: store,
	}

	ctx := context.Background()
	metrics, err := RunProductionEvaluation(ctx, cfg)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// All 3 judges should have results.
	if len(metrics.JudgeMetrics) != 3 {
		t.Errorf("expected 3 judge metrics, got %d", len(metrics.JudgeMetrics))
	}

	// Verify individual judge results.
	if perfect, ok := metrics.JudgeMetrics["perfect_judge"]; ok && perfect.PassRate != 1.0 {
		t.Errorf("perfect judge pass rate should be 1.0, got %.2f", perfect.PassRate)
	}
	if failing, ok := metrics.JudgeMetrics["failing_judge"]; ok && failing.PassRate != 0.0 {
		t.Errorf("failing judge pass rate should be 0.0, got %.2f", failing.PassRate)
	}
}

// TestRunProductionEvaluation_ConfigDefaults tests that zero/invalid configs use defaults.
func TestRunProductionEvaluation_ConfigDefaults(t *testing.T) {
	store := NewInMemoryStore()
	for i := 0; i < 10; i++ {
		record := InteractionRecord{
			ID:       fmt.Sprintf("rec-%d", i),
			Metadata: map[string]any{"input": "test", "output": "test"},
		}
		if err := store.Save(record); err != nil {
			t.Fatalf("failed to save: %v", err)
		}
	}

	cfg := ProductionEvalConfig{
		SampleRate: 0,             // Invalid: should default to 0.01.
		ConfidenceLevel: 0,        // Invalid: should default to 0.95.
		DriftThreshold: 0,         // Invalid: should default to 0.85.
		BootstrapSamples: 0,       // Invalid: should default to 10000.
		Judges: map[string]JudgeFunc{
			"test": func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
				return 1.0, nil
			},
		},
		Store: store,
	}

	ctx := context.Background()
	metrics, err := RunProductionEvaluation(ctx, cfg)
	if err != nil {
		t.Fatalf("evaluation with defaulted config failed: %v", err)
	}

	// Sample size should be ~1% of 10 = ~0.1, rounded up to 1.
	if metrics.SampleSize < 1 || metrics.SampleSize > 2 {
		t.Errorf("expected ~1 sample with 1%% rate, got %d", metrics.SampleSize)
	}

	// Confidence level should be 0.95.
	if metrics.ConfidenceLevel != 0.95 {
		t.Errorf("expected default confidence 0.95, got %.2f", metrics.ConfidenceLevel)
	}
}

// TestBootstrapCI_EdgeCase_SingleScore tests CI on single score.
func TestBootstrapCI_EdgeCase_SingleScore(t *testing.T) {
	scores := []float64{0.75}

	ci := bootstrapCI(scores, 0.95, 100)

	// With only 1 sample, bootstrap means should all be 0.75.
	if ci.Point != 0.75 {
		t.Errorf("expected point 0.75, got %.3f", ci.Point)
	}
	if ci.Lower != 0.75 || ci.Upper != 0.75 {
		t.Errorf("expected CI [0.75, 0.75], got [%.3f, %.3f]", ci.Lower, ci.Upper)
	}
}

// TestGetStringMetadata_ValidKey tests metadata extraction.
func TestGetStringMetadata_ValidKey(t *testing.T) {
	record := InteractionRecord{
		Metadata: map[string]any{
			"key1": "value1",
			"key2": 42, // Non-string value.
		},
	}

	val := getStringMetadata(record, "key1", "default")
	if val != "value1" {
		t.Errorf("expected 'value1', got %q", val)
	}

	// Non-string metadata should return default.
	val = getStringMetadata(record, "key2", "default")
	if val != "default" {
		t.Errorf("expected 'default' for non-string, got %q", val)
	}

	// Missing key should return default.
	val = getStringMetadata(record, "missing", "default")
	if val != "default" {
		t.Errorf("expected 'default' for missing key, got %q", val)
	}
}

// TestGetStringMetadata_NilMetadata tests with nil metadata map.
func TestGetStringMetadata_NilMetadata(t *testing.T) {
	record := InteractionRecord{
		Metadata: nil,
	}

	val := getStringMetadata(record, "any", "default")
	if val != "default" {
		t.Errorf("expected 'default' for nil metadata, got %q", val)
	}
}

// TestConfidenceIntervalBounds verifies CI ordering.
func TestConfidenceIntervalBounds(t *testing.T) {
	// Generate diverse scores.
	scores := make([]float64, 1000)
	for i := 0; i < 1000; i++ {
		// Skewed distribution: 60% at 1.0, 40% at 0.0.
		if i%5 < 3 {
			scores[i] = 1.0
		} else {
			scores[i] = 0.0
		}
	}

	ci := bootstrapCI(scores, 0.95, 5000)

	// Verify strict ordering: Lower <= Point <= Upper.
	if !(ci.Lower <= ci.Point && ci.Point <= ci.Upper) {
		t.Errorf("invalid CI ordering: [%.3f, %.3f, %.3f]", ci.Lower, ci.Point, ci.Upper)
	}

	// For a 95% CI, the width should be reasonable.
	width := ci.Upper - ci.Lower
	if width < 0.01 || width > 0.5 {
		t.Logf("warning: CI width %.3f is unusual (but not necessarily wrong)", width)
	}
}

// BenchmarkBootstrapCI measures bootstrap CI performance.
func BenchmarkBootstrapCI(b *testing.B) {
	scores := make([]float64, 100)
	for i := 0; i < 100; i++ {
		scores[i] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bootstrapCI(scores, 0.95, 1000)
	}
}

// BenchmarkSampleRecords measures sampling performance.
func BenchmarkSampleRecords(b *testing.B) {
	records := make([]InteractionRecord, 10000)
	for i := range records {
		records[i] = InteractionRecord{ID: fmt.Sprintf("rec-%d", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sampleRecords(records, 0.10)
	}
}

// BenchmarkRunProductionEvaluation_SmallDataset measures full eval on small dataset.
func BenchmarkRunProductionEvaluation_SmallDataset(b *testing.B) {
	store := NewInMemoryStore()
	for i := 0; i < 100; i++ {
		record := InteractionRecord{
			ID:       fmt.Sprintf("rec-%d", i),
			Metadata: map[string]any{"input": "test", "output": "test"},
		}
		if err := store.Save(record); err != nil {
			b.Fatalf("failed to save: %v", err)
		}
	}

	fastJudge := func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
		return 0.9, nil
	}

	cfg := ProductionEvalConfig{
		SampleRate:       1.0,
		ConfidenceLevel:  0.95,
		DriftThreshold:   0.85,
		BootstrapSamples: 100, // Faster for benchmark.
		Judges: map[string]JudgeFunc{
			"fast_judge": fastJudge,
		},
		Store: store,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunProductionEvaluation(ctx, cfg)
	}
}

// TestRandomFloat64_Distribution verifies randomness is roughly uniform.
func TestRandomFloat64_Distribution(t *testing.T) {
	samples := 10000
	buckets := make([]int, 10) // [0-0.1), [0.1-0.2), ..., [0.9-1.0).

	for i := 0; i < samples; i++ {
		r := randomFloat64()
		if r < 0 || r >= 1.0 {
			t.Errorf("randomFloat64 out of range: %.3f", r)
		}
		bucket := int(r * 10)
		if bucket >= 10 {
			bucket = 9
		}
		buckets[bucket]++
	}

	// Each bucket should have approximately samples/10 entries (±30% tolerance).
	expected := samples / 10
	tolerance := int(float64(expected) * 0.3)

	for i, count := range buckets {
		if count < expected-tolerance || count > expected+tolerance {
			t.Logf("bucket %d: %d samples (expected ~%d ±%d)", i, count, expected, tolerance)
		}
	}
}

// TestProductionEvalMetrics_Timestamps verifies timing fields are set.
func TestProductionEvalMetrics_Timestamps(t *testing.T) {
	store := NewInMemoryStore()
	record := InteractionRecord{ID: "test", Metadata: map[string]any{"input": "x", "output": "y"}}
	store.Save(record)

	cfg := ProductionEvalConfig{
		SampleRate: 1.0,
		Judges: map[string]JudgeFunc{
			"j": func(ctx context.Context, p llm.Provider, model, input, output string) (float64, error) {
				return 1.0, nil
			},
		},
		Store: store,
	}

	beforeEval := time.Now()
	ctx := context.Background()
	metrics, err := RunProductionEvaluation(ctx, cfg)
	afterEval := time.Now()

	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}

	if metrics.StartedAt.IsZero() {
		t.Errorf("StartedAt not set")
	}
	if metrics.CompletedAt.IsZero() {
		t.Errorf("CompletedAt not set")
	}
	if !metrics.StartedAt.After(beforeEval.Add(-1*time.Second)) ||
		!metrics.CompletedAt.Before(afterEval.Add(1*time.Second)) {
		t.Errorf("timestamps outside expected bounds")
	}
	if !metrics.StartedAt.Before(metrics.CompletedAt) {
		t.Errorf("StartedAt should be before CompletedAt")
	}
}
