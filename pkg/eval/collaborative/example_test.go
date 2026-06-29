package collaborative

import (
	"fmt"
	"testing"
	"time"
)

// TestExample_SettlementRubricRefinement demonstrates the full collaborative
// evaluation workflow for a settlement rubric that initially has poor agreement.
func TestExample_SettlementRubricRefinement(t *testing.T) {
	// ============================================================================
	// Phase 1: Initial Evaluation (κ < 0.6 — identifies ambiguity)
	// ============================================================================

	fmt.Println("\n=== PHASE 1: Initial Evaluation ===")

	// Three evaluators annotate the same traces
	initialAnnotations := map[string]*Annotation{
		// Trace 1: Clear double-spend case (high agreement expected)
		"trace-1:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001", // Double-Spend
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     95,
			Notes:          "Two concurrent payment requests both committed to ledger",
			Timestamp:      time.Now(),
		},
		"trace-1:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     90,
			Notes:          "Concurrent idempotency key handling failure",
			Timestamp:      time.Now(),
		},
		"trace-1:charlie": {
			AnnotatorID:    "charlie",
			TraceID:        "trace-1",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     88,
			Notes:          "Both payments settled; idempotency violated",
			Timestamp:      time.Now(),
		},

		// Trace 2: Ambiguous settlement case (low agreement expected)
		"trace-2:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-SE-002", // Settlement Atomicity
			RubricsFailed:  []string{"RB-SE-002"},
			Confidence:     65,
			Notes:          "Partial settlement: payment committed but clearing pending",
			Timestamp:      time.Now(),
		},
		"trace-2:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-2",
			PrimaryFailure: "",
			RubricsFailed:  []string{},
			Confidence:     60,
			Notes:          "Settlement eventually completed; atomicity not strictly required?",
			Timestamp:      time.Now(),
		},
		"trace-2:charlie": {
			AnnotatorID:    "charlie",
			TraceID:        "trace-2",
			PrimaryFailure: "FM-SE-003", // Settlement Consistency
			RubricsFailed:  []string{"RB-SE-003"},
			Confidence:     70,
			Notes:          "Ledger balance inconsistent with order total",
			Timestamp:      time.Now(),
		},

		// Trace 3: Moderate ambiguity
		"trace-3:alice": {
			AnnotatorID:    "alice",
			TraceID:        "trace-3",
			PrimaryFailure: "FM-SE-004",
			RubricsFailed:  []string{"RB-SE-001", "RB-SE-004"},
			Confidence:     75,
			Notes:          "Multiple issues: duplicates + audit trail gaps",
			Timestamp:      time.Now(),
		},
		"trace-3:bob": {
			AnnotatorID:    "bob",
			TraceID:        "trace-3",
			PrimaryFailure: "FM-SE-004",
			RubricsFailed:  []string{"RB-SE-004"},
			Confidence:     72,
			Notes:          "Audit trail incomplete; duplicates may be acceptable",
			Timestamp:      time.Now(),
		},
		"trace-3:charlie": {
			AnnotatorID:    "charlie",
			TraceID:        "trace-3",
			PrimaryFailure: "FM-SE-001",
			RubricsFailed:  []string{"RB-SE-001"},
			Confidence:     78,
			Notes:          "Clear duplicate issue; audit trail separate concern",
			Timestamp:      time.Now(),
		},
	}

	result := CohensKappa(initialAnnotations)

	fmt.Printf("Initial Evaluation Results:\n")
	fmt.Printf("  Overall κ: %.3f (%s)\n", result.KappaOverall, result.InterpretationOverall)
	fmt.Printf("  Total annotations: %d\n", result.TotalAnnotations)
	fmt.Printf("  Total traces: %d\n", result.TotalTraces)
	fmt.Printf("  Annotators: %d\n", len(result.AnnotatorStats))
	fmt.Printf("\n  Per-Annotator Stats:\n")
	for annotator, stats := range result.AnnotatorStats {
		fmt.Printf("    %s: %d annotations, %.1f%% agreement, %.0f%% avg confidence\n",
			annotator, stats.AnnotationCount, stats.AgreementRate*100, stats.AverageConfidence)
	}

	fmt.Printf("\n  Rubric Results:\n")
	for rubricID, agreement := range result.RubricResults {
		status := "PASS"
		if agreement.KappaScore < 0.6 {
			status = "FAIL"
		}
		fmt.Printf("    %s: κ=%.3f [%s] (disagreement: %.1f%%)\n",
			rubricID, agreement.KappaScore, status, agreement.DisagreementRate*100)
	}

	// ============================================================================
	// Phase 2: Identify Problem Rubrics
	// ============================================================================

	fmt.Println("\n=== PHASE 2: Identify Problem Rubrics ===")

	if len(result.RubricFailures) > 0 {
		fmt.Printf("Rubrics Below Target (κ < 0.6):\n")
		for i, failure := range result.RubricFailures {
			fmt.Printf("  %d. %s\n", i+1, failure.RubricID)
			fmt.Printf("     κ=%.3f | Priority: %s\n", failure.KappaScore, failure.Priority)
			if len(failure.Issues) > 0 {
				fmt.Printf("     Issues:\n")
				for _, issue := range failure.Issues {
					fmt.Printf("       - %s\n", issue)
				}
			}
			if len(failure.ExamplePairs) > 0 {
				fmt.Printf("     Example Disagreements:\n")
				for _, pair := range failure.ExamplePairs[:1] { // Show first example
					fmt.Printf("       Trace: %s\n", pair.TraceID)
					fmt.Printf("         %s (conf: %s): %s\n", pair.AnnotatorA, pair.ResponseA, pair.JustificationA)
					fmt.Printf("         %s (conf: %s): %s\n", pair.AnnotatorB, pair.ResponseB, pair.JustificationB)
				}
			}
		}
	} else {
		fmt.Println("All rubrics passed (κ ≥ 0.6)")
	}

	// ============================================================================
	// Phase 3: Generate Refinement Plan
	// ============================================================================

	fmt.Println("\n=== PHASE 3: Refinement Plan ===")

	refiner := NewRubricRefiner(nil)
	report := GenerateAlignmentReport(result, refiner)

	fmt.Printf("Overall Status:\n")
	fmt.Printf("  Total rubrics: %d\n", report.RubricsEvaluated)
	fmt.Printf("  Passing (κ ≥ 0.6): %d (%.1f%%)\n", report.RubricsPassing, report.PassRate)
	fmt.Printf("  Critical (κ < 0.4): %d\n", report.RubricsCritical)
	fmt.Printf("  Average κ: %.3f\n", report.AverageKappa)
	fmt.Printf("  Estimated effort: %.1f person-hours\n", report.EstimatedEffort)

	if report.MostProblematicRubric != nil {
		fmt.Printf("\nMost Problematic:\n")
		fmt.Printf("  %s: κ=%.3f (Priority: %s)\n",
			report.MostProblematicRubric.RubricID,
			report.MostProblematicRubric.KappaScore,
			report.MostProblematicRubric.Priority)
	}

	// ============================================================================
	// Phase 4: Detailed Refinement Worksheet for Worst Rubric
	// ============================================================================

	if len(report.RefinementPlan) > 0 {
		fmt.Println("\n=== PHASE 4: Detailed Refinement (Worst Rubric) ===")

		ws := report.RefinementPlan[0] // Most critical first
		fmt.Printf("Rubric: %s\n", ws.RubricID)
		fmt.Printf("Current κ: %.3f (Priority: %s)\n", ws.CurrentKappa, ws.Priority)
		fmt.Printf("Disagreement cases: %d\n", ws.DisagreementCount)

		fmt.Printf("\nRecommendations:\n")
		for i, rec := range ws.Recommendations {
			fmt.Printf("  %d. %s\n", i+1, rec)
		}

		fmt.Printf("\nProposed Criteria Improvements:\n")
		for i, crit := range ws.ProposedCriteria {
			fmt.Printf("  %d. %s\n", i+1, crit)
		}

		fmt.Printf("\nEstimated Impact: %s\n", ws.EstimatedImpact)

		fmt.Printf("\nValidation Strategy:\n%s\n", ws.ValidationStrategy)
	}

	// ============================================================================
	// Phase 5: Roadmap
	// ============================================================================

	if len(report.RoadmapMilestones) > 0 {
		fmt.Println("\n=== PHASE 5: Refinement Roadmap ===")

		for i, milestone := range report.RoadmapMilestones {
			fmt.Printf("  %s (Weeks: %d)\n", milestone.Phase, milestone.EstimatedWeeks)
			fmt.Printf("    Goal: κ ≥ %.1f\n", milestone.TargetKappa)
			fmt.Printf("    Rubrics: %d in scope\n", len(milestone.RubricsInScope))
			fmt.Printf("    Success Criteria:\n")
			for _, criterion := range milestone.SuccessCriteria {
				fmt.Printf("      - %s\n", criterion)
			}
			if i < len(report.RoadmapMilestones)-1 {
				fmt.Println()
			}
		}
	}

	// ============================================================================
	// Summary
	// ============================================================================

	fmt.Println("\n=== SUMMARY ===")
	fmt.Printf("Initial state: κ=%.3f (Interpretation: %s)\n", result.KappaOverall, result.InterpretationOverall)
	fmt.Printf("Target state: κ ≥ 0.6 (Substantial agreement)\n")
	fmt.Printf("Effort required: %.1f hours\n", report.EstimatedEffort)
	fmt.Printf("Timeline: %d weeks to complete\n", sumWeeks(report.RoadmapMilestones))
}

func sumWeeks(milestones []*RoadmapMilestone) int {
	total := 0
	for _, m := range milestones {
		total += m.EstimatedWeeks
	}
	return total
}
