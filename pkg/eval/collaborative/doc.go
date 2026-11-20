// Package collaborative implements inter-rater agreement measurement for collaborative evaluation.
//
// This package enables measurement of agreement between multiple annotators evaluating
// execution traces and failure modes using Cohen's Kappa coefficient (κ). It provides:
//
// 1. **Annotation Capture**: Record individual annotator judgments with confidence levels
// 2. **Agreement Measurement**: Calculate κ for primary failures and per-rubric evaluation
// 3. **Disagreement Analysis**: Identify false positives/negatives and ambiguous criteria
// 4. **Refinement Guidance**: Generate actionable suggestions to improve rubric clarity
// 5. **Roadmap Planning**: Prioritize rubric improvements toward target κ ≥ 0.6
//
// # Core Types
//
// - Annotation: Single annotator's evaluation of a trace
// - AgreementResult: Cohen's Kappa summary with per-rubric and per-annotator stats
// - RubricRefinement: Actionable guidance for improving failing rubrics
// - RubricAlignmentWorkflow: Iterative refinement tracking and history
//
// # Kappa Interpretation
//
// Cohen's Kappa (κ) measures agreement beyond chance:
//
//	κ < 0.0     Poor agreement (worse than random)
//	κ 0.0-0.2   Slight agreement
//	κ 0.2-0.4   Fair agreement
//	κ 0.4-0.6   Moderate agreement
//	κ 0.6-0.8   Substantial agreement (TARGET for Genie)
//	κ 0.8-1.0   Almost perfect agreement
//
// Target for Genie evaluations: κ ≥ 0.6 per rubric, indicating high reliability
// of failure classification and rubric evaluation.
//
// # Usage Example
//
//	// Collect annotations from multiple evaluators
//	annotations := map[string]*Annotation{
//	  "trace-1:alice": {
//	    AnnotatorID:    "alice",
//	    TraceID:        "trace-1",
//	    PrimaryFailure: "FM-SE-001",
//	    RubricsFailed:  []string{"RB-SE-001"},
//	    Confidence:     90,
//	    Notes:          "Clear double-spend violation",
//	  },
//	  "trace-1:bob": {
//	    AnnotatorID:    "bob",
//	    TraceID:        "trace-1",
//	    PrimaryFailure: "FM-SE-001",
//	    RubricsFailed:  []string{"RB-SE-001"},
//	    Confidence:     85,
//	    Notes:          "Confirmed",
//	  },
//	}
//
//	// Calculate agreement
//	result := CohensKappa(annotations)
//
//	// Check for failing rubrics
//	if len(result.RubricFailures) > 0 {
//	  fmt.Printf("Rubrics below target (κ < 0.6):\n")
//	  for _, failure := range result.RubricFailures {
//	    fmt.Printf("  %s: κ=%.3f (priority: %s)\n",
//	      failure.RubricID, failure.KappaScore, failure.Priority)
//	  }
//	}
//
//	// Generate refinement plan
//	refiner := NewRubricRefiner(nil)
//	report := GenerateAlignmentReport(result, refiner)
//	fmt.Printf("Pass rate: %.1f%%\n", report.PassRate)
//	fmt.Printf("Estimated effort: %.1f hours\n", report.EstimatedEffort)
//
// # Workflow
//
// 1. **Collect Annotations**: Multiple evaluators independently annotate traces
// 2. **Calculate Agreement**: CohensKappa() returns κ and identifies problem rubrics
// 3. **Analyze Patterns**: Disagreement types (false positives vs negatives)
// 4. **Refine Rubrics**: NewRubricRefiner generates concrete improvement suggestions
// 5. **Validate Changes**: Re-evaluate with same annotators on disputed cases
// 6. **Iterate**: Repeat until κ ≥ 0.6 for all rubrics
//
// # Performance
//
// Benchmarks on M1 Mac:
//   - 100 annotations: ~55 µs
//   - 1000 annotations: ~1.5 ms
//
// Complexity: O(n²) in annotation count due to pairwise comparisons.
//
// # Integration
//
// Integrates with pkg/eval:
//   - Uses evaluation rubrics from eval.AllRubrics
//   - Tracks failure modes from eval.FailureModeRegistry
//   - Bridges collaborative evaluation to automated testing
//
// MIT License
package collaborative
