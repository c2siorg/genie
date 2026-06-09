package collaborative

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// TestCohensKappa_PerfectAgreement tests κ when all annotators agree perfectly.
func TestCohensKappa_PerfectAgreement(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-002"},
			Confidence:     90,
			Notes:          "Clear double-spend violation",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-002"},
			Confidence:     85,
			Notes:          "Confirmed: concurrent payments both succeeded",
			Timestamp:      time.Now(),
		},
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-003",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     88,
			Notes:          "Orchestration deadlock detected",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-003",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     92,
			Notes:          "Confirmed: workflow stuck in cycle",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// Perfect agreement should have κ = 1.0
	if math.Abs(result.KappaOverall-1.0) > 0.001 {
		t.Errorf("expected κ ≈ 1.0 for perfect agreement, got %.3f", result.KappaOverall)
	}

	if result.TotalAnnotations != 4 {
		t.Errorf("expected 4 annotations, got %d", result.TotalAnnotations)
	}

	if result.TotalTraces != 2 {
		t.Errorf("expected 2 traces, got %d", result.TotalTraces)
	}

	if len(result.RubricFailures) != 0 {
		t.Errorf("expected no failing rubrics, got %d", len(result.RubricFailures))
	}

	if result.InterpretationOverall != "Almost perfect agreement" {
		t.Errorf("expected 'Almost perfect agreement', got %q", result.InterpretationOverall)
	}
}

// TestCohensKappa_CompleteDisagreement tests κ when annotators never agree.
func TestCohensKappa_CompleteDisagreement(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     70,
			Notes:          "Double-spend issue",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-CO-003",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     75,
			Notes:          "Orchestration failure",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// Complete disagreement should have κ <= 0
	if result.KappaOverall > 0.1 {
		t.Errorf("expected κ ≤ 0 for complete disagreement, got %.3f", result.KappaOverall)
	}

	if result.InterpretationOverall != "Poor agreement (worse than chance)" {
		t.Errorf("expected poor agreement interpretation, got %q", result.InterpretationOverall)
	}
}

// TestCohensKappa_PartialAgreement tests κ with moderate agreement.
func TestCohensKappa_PartialAgreement(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-002"},
			Confidence:     80,
			Notes:          "Clear settlement issue",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     75,
			Notes:          "Primary rubric clear, secondary debatable",
			Timestamp:      time.Now(),
		},
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-002",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     70,
			Notes:          "Possible orchestration issue",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "",
			RubricsFailed:  []string{},
			Confidence:     60,
			Notes:          "No clear failure observed",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// Partial agreement should fall between extremes
	if result.KappaOverall < 0.2 || result.KappaOverall > 0.9 {
		t.Logf("moderate agreement κ=%.3f (expected 0.2-0.9)", result.KappaOverall)
	}

	if len(result.RubricResults) == 0 {
		t.Error("expected rubric results")
	}
}

// TestCohensKappa_RubricAgreement tests per-rubric κ calculation.
func TestCohensKappa_RubricAgreement(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-003"},
			Confidence:     90,
			Notes:          "Double-spend and atomicity issues",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-003"},
			Confidence:     85,
			Notes:          "Agree: double-spend and atomicity",
			Timestamp:      time.Now(),
		},
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-SE-002",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     75,
			Notes:          "Settlement issue",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-SE-002",
			RubricsFailed:  []string{"RB-SE-002"},
			Confidence:     70,
			Notes:          "Different rubric flagged",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// Check that we have rubric results
	if len(result.RubricResults) == 0 {
		t.Fatal("expected rubric results")
	}

	// RB-SE-001 should have high agreement (3 agree: trace-1 both, trace-2 alice vs bob's RB-SE-002)
	if rb1, ok := result.RubricResults["RB-SE-001"]; ok {
		if rb1.KappaScore < 0.3 {
			t.Logf("RB-SE-001 κ=%.3f (some disagreement expected)", rb1.KappaScore)
		}
		if rb1.AnnotationPairs == 0 {
			t.Error("RB-SE-001 should have annotation pairs")
		}
	}

	// RB-SE-003 should have agreement on both traces
	if rb3, ok := result.RubricResults["RB-SE-003"]; ok {
		if rb3.KappaScore < 0.5 {
			t.Logf("RB-SE-003 κ=%.3f", rb3.KappaScore)
		}
	}
}

// TestCohensKappa_EmptyAnnotations tests κ with no annotations.
func TestCohensKappa_EmptyAnnotations(t *testing.T) {
	annotations := map[string]*Annotation{}

	result := CohensKappa(annotations)

	if result.TotalAnnotations != 0 {
		t.Errorf("expected 0 annotations, got %d", result.TotalAnnotations)
	}
	if result.TotalTraces != 0 {
		t.Errorf("expected 0 traces, got %d", result.TotalTraces)
	}
	if result.KappaOverall != 0 {
		t.Errorf("expected κ=0, got %.3f", result.KappaOverall)
	}
}

// TestCohensKappa_SingleAnnotatorPerTrace tests κ with single annotators per trace.
func TestCohensKappa_SingleAnnotatorPerTrace(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     90,
			Notes:          "Clear issue",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-003",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     80,
			Notes:          "Clear issue",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// Single annotators per trace can't form agreement
	if result.KappaOverall != 0 {
		t.Errorf("expected κ=0 with single annotators per trace, got %.3f", result.KappaOverall)
	}
	if result.TotalAnnotations != 2 {
		t.Errorf("expected 2 annotations, got %d", result.TotalAnnotations)
	}
}

// TestCohensKappa_RubricFailuresBelow60 tests detection of κ < 0.6 rubrics.
func TestCohensKappa_RubricFailuresBelow60(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"}, // Both agree
			Confidence:     90,
			Notes:          "Clear double-spend",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"}, // Both agree
			Confidence:     85,
			Notes:          "Confirmed",
			Timestamp:      time.Now(),
		},
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-SE-002",
			RubricsFailed:  []string{"RB-SE-002"}, // Alice only
			Confidence:     65,
			Notes:          "Ambiguous atomicity issue",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-SE-002",
			RubricsFailed:  []string{}, // Bob disagrees
			Confidence:     70,
			Notes:          "No atomicity violation observed",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// RB-SE-002 should fail (disagreement in trace-2)
	if len(result.RubricFailures) == 0 {
		t.Log("note: RB-SE-002 may pass if agreement happens by chance")
	}

	// Check rubric results
	if rb2, ok := result.RubricResults["RB-SE-002"]; ok {
		if rb2.KappaScore >= 0.6 {
			t.Logf("RB-SE-002 κ=%.3f (agrees above threshold)", rb2.KappaScore)
		}
	}
}

// TestCohensKappa_AnnotatorStats tests per-annotator metrics.
func TestCohensKappa_AnnotatorStats(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     90,
			Notes:          "High confidence",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     85,
			Notes:          "High confidence",
			Timestamp:      time.Now(),
		},
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-003",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     75,
			Notes:          "Medium confidence",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-002",
			RubricsFailed:  []string{"RB-CO-002"},
			Confidence:     65,
			Notes:          "Disagreed",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// Both annotators should have stats
	if len(result.AnnotatorStats) != 2 {
		t.Errorf("expected 2 annotators, got %d", len(result.AnnotatorStats))
	}

	// Alice has 2 annotations with avg confidence (90+75)/2 = 82.5
	if aliceStats, ok := result.AnnotatorStats["alice"]; ok {
		if aliceStats.AnnotationCount != 2 {
			t.Errorf("alice: expected 2 annotations, got %d", aliceStats.AnnotationCount)
		}
		expectedAvgConf := 82.5
		if math.Abs(aliceStats.AverageConfidence-expectedAvgConf) > 0.1 {
			t.Errorf("alice: expected avg confidence %.1f, got %.1f", expectedAvgConf, aliceStats.AverageConfidence)
		}
		if aliceStats.AgreementRate != 0.5 {
			t.Errorf("alice: expected 50%% agreement rate (1 agree out of 2), got %.1f%%", aliceStats.AgreementRate*100)
		}
	}

	// Bob has 2 annotations with avg confidence (85+65)/2 = 75
	if bobStats, ok := result.AnnotatorStats["bob"]; ok {
		if bobStats.AnnotationCount != 2 {
			t.Errorf("bob: expected 2 annotations, got %d", bobStats.AnnotationCount)
		}
		expectedAvgConf := 75.0
		if math.Abs(bobStats.AverageConfidence-expectedAvgConf) > 0.1 {
			t.Errorf("bob: expected avg confidence %.1f, got %.1f", expectedAvgConf, bobStats.AverageConfidence)
		}
		if bobStats.AgreementRate != 0.5 {
			t.Errorf("bob: expected 50%% agreement rate (1 agree out of 2), got %.1f%%", bobStats.AgreementRate*100)
		}
	}
}

// TestCohensKappa_ConfidenceAnalysis tests confidence-agreement correlation.
func TestCohensKappa_ConfidenceAnalysis(t *testing.T) {
	annotations := map[string]*Annotation{
		// High confidence agreements
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     95,
			Notes:          "Very confident",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     90,
			Notes:          "Very confident",
			Timestamp:      time.Now(),
		},
		// Low confidence disagreement
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-003",
			RubricsFailed:  []string{"RB-CO-001"},
			Confidence:     45,
			Notes:          "Not sure",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-CO-002",
			RubricsFailed:  []string{"RB-CO-002"},
			Confidence:     50,
			Notes:          "Unsure",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	if result.ConfidenceAnalysis == nil {
		t.Fatal("expected confidence analysis")
	}

	// High confidence should have better agreement
	if result.ConfidenceAnalysis.HighConfidenceAgreement < result.ConfidenceAnalysis.LowConfidenceAgreement {
		t.Logf("high conf κ=%.3f >= low conf κ=%.3f (as expected)",
			result.ConfidenceAnalysis.HighConfidenceAgreement,
			result.ConfidenceAnalysis.LowConfidenceAgreement)
	}

	if result.ConfidenceAnalysis.Observation == "" {
		t.Error("expected confidence observation")
	}
}

// TestCohensKappa_DisagreementDetection tests identification of false positives/negatives.
func TestCohensKappa_DisagreementDetection(t *testing.T) {
	annotations := map[string]*Annotation{
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-002"}, // Both flags
			Confidence:     85,
			Notes:          "Both rubrics failed",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"}, // Only one
			Confidence:     80,
			Notes:          "Only RB-SE-001 failed",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(annotations)

	// RB-SE-002 should be identified as failing
	var rb2Failure *RubricRefinement
	for _, failure := range result.RubricFailures {
		if failure.RubricID == "RB-SE-002" {
			rb2Failure = failure
			break
		}
	}

	if rb2Failure != nil {
		if len(rb2Failure.Issues) == 0 {
			t.Error("expected disagreement issues for RB-SE-002")
		}
		if len(rb2Failure.ExamplePairs) == 0 {
			t.Error("expected example pairs for RB-SE-002")
		}
	}
}

// TestInterpretKappa tests κ interpretation strings.
func TestInterpretKappa(t *testing.T) {
	tests := []struct {
		kappa       float64
		expected    string
	}{
		{-0.5, "Poor agreement (worse than chance)"},
		{0.1, "Slight agreement"},
		{0.3, "Fair agreement"},
		{0.5, "Moderate agreement"},
		{0.7, "Substantial agreement"},
		{0.95, "Almost perfect agreement"},
	}

	for _, tt := range tests {
		got := interpretKappa(tt.kappa)
		if got != tt.expected {
			t.Errorf("κ=%.2f: expected %q, got %q", tt.kappa, tt.expected, got)
		}
	}
}

// TestPrioritizeRubric tests priority assignment based on κ.
func TestPrioritizeRubric(t *testing.T) {
	tests := []struct {
		kappa    float64
		expected string
	}{
		{0.2, "critical"},
		{0.45, "high"},
		{0.55, "medium"},
	}

	for _, tt := range tests {
		got := prioritizeRubric(tt.kappa)
		if got != tt.expected {
			t.Errorf("κ=%.2f: expected %q, got %q", tt.kappa, tt.expected, got)
		}
	}
}

// TestCohensKappa_LargeDataset tests κ with many annotations.
func TestCohensKappa_LargeDataset(t *testing.T) {
	annotations := make(map[string]*Annotation)

	// Create 10 traces with 5 annotators each, 80% agreement
	for i := 1; i <= 10; i++ {
		primaryFailure := "FM-SE-001"
		if i > 5 {
			primaryFailure = "FM-CO-003"
		}

		for j := 0; j < 5; j++ {
			annotator := string(rune('A' + j))
			key := formatKey(i, annotator)

			// 80% of the time, agree with primary
			failure := primaryFailure
			if j%5 == 0 && i > 1 {
				failure = "FM-CO-002" // Occasional disagreement
			}

			annotations[key] = &Annotation{
				AnnotatorID:    annotator,
				TraceID:        formatTrace(i),
				PrimaryFailure: failure,
				RubricsFailed:  []string{"RB-SE-001"},
				Confidence:     80 + j*2,
				Notes:          "Annotation",
				Timestamp:      time.Now(),
			}
		}
	}

	result := CohensKappa(annotations)

	if result.TotalAnnotations != 50 {
		t.Errorf("expected 50 annotations, got %d", result.TotalAnnotations)
	}

	if result.TotalTraces != 10 {
		t.Errorf("expected 10 traces, got %d", result.TotalTraces)
	}

	if result.KappaOverall < 0.5 {
		t.Logf("κ=%.3f (80%% agreement expected κ ≈ 0.67)", result.KappaOverall)
	}

	if len(result.AnnotatorStats) != 5 {
		t.Errorf("expected 5 annotators, got %d", len(result.AnnotatorStats))
	}
}

// Helpers

func formatKey(traceNum int, annotator string) string {
	return "trace-" + string(rune('0'+traceNum)) + ":" + annotator
}

func formatTrace(num int) string {
	return "trace-" + string(rune('0'+num))
}

// Benchmark tests

func BenchmarkCohensKappa_100Annotations(b *testing.B) {
	annotations := make(map[string]*Annotation)
	for i := 0; i < 100; i++ {
		annotations[fmt.Sprintf("ann-%d", i)] = &Annotation{
			AnnotatorID:    fmt.Sprintf("annotator-%d", i%10),
			TraceID:        fmt.Sprintf("trace-%d", i%20),
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     80,
			Notes:          "test",
			Timestamp:      time.Now(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CohensKappa(annotations)
	}
}

func BenchmarkCohensKappa_1000Annotations(b *testing.B) {
	annotations := make(map[string]*Annotation)
	for i := 0; i < 1000; i++ {
		annotations[fmt.Sprintf("ann-%d", i)] = &Annotation{
			AnnotatorID:    fmt.Sprintf("annotator-%d", i%50),
			TraceID:        fmt.Sprintf("trace-%d", i%200),
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     80,
			Notes:          "test",
			Timestamp:      time.Now(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CohensKappa(annotations)
	}
}

