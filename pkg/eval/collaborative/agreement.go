// Package collaborative implements inter-rater agreement measurement for collaborative evaluation.
//
// This package provides Cohen's Kappa coefficient calculation to measure agreement between
// annotators on failure classification and rubric evaluation. It enables detection of rubric
// ambiguity and guides refinement of evaluation criteria toward target κ ≥ 0.6 per rubric.
//
// Key types:
//   - Annotation: individual annotator evaluation of a trace
//   - AgreementResult: Cohen's Kappa results and statistical summary per rubric
//   - RubricRefinement: guidance for improving rubric clarity
//
// Usage:
//
//	annotations := map[string]*Annotation{
//	  "ann-001": {AnnotatorID: "alice", PrimaryFailure: "FM-SE-001", RubricsFailed: []string{"RB-SE-001"}},
//	  "ann-002": {AnnotatorID: "bob", PrimaryFailure: "FM-SE-001", RubricsFailed: []string{"RB-SE-001"}},
//	}
//	result := CohensKappa(annotations)
//	if result.KappaOverall < 0.6 {
//	  for _, rubric := range result.RubricFailures {
//	    fmt.Printf("Fix: %s (κ=%.2f)\n", rubric.RubricID, rubric.KappaScore)
//	  }
//	}
package collaborative

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Annotation represents a single annotator's evaluation of a trace.
//
// Each annotation captures one annotator's judgment about failure modes and rubric violations.
// Multiple annotations for the same TraceID enable inter-rater agreement analysis.
type Annotation struct {
	// AnnotatorID unique identifier for the annotator (e.g., "alice", "bob", "reviewer-001")
	AnnotatorID string `json:"annotator_id"`
	// TraceID the trace/interaction being evaluated
	TraceID string `json:"trace_id"`
	// PrimaryFailure the dominant failure mode identified (FM-DOMAIN-NNN), empty if none
	PrimaryFailure string `json:"primary_failure"`
	// RubricsFailed list of rubric IDs that failed evaluation (RB-DOMAIN-NNN)
	RubricsFailed []string `json:"rubrics_failed"`
	// Confidence confidence in this annotation (0-100)
	Confidence int `json:"confidence"`
	// Notes free-form justification or evidence for the annotation
	Notes string `json:"notes"`
	// Timestamp when the annotation was created
	Timestamp time.Time `json:"timestamp"`
}

// AgreementResult summarizes Cohen's Kappa agreement statistics.
type AgreementResult struct {
	// KappaOverall Cohen's Kappa for primary failure classification (aggregated)
	KappaOverall float64 `json:"kappa_overall"`
	// InterpretationOverall English description of overall agreement level
	InterpretationOverall string `json:"interpretation_overall"`
	// TotalAnnotations number of annotations analyzed
	TotalAnnotations int `json:"total_annotations"`
	// TotalTraces number of unique traces covered
	TotalTraces int `json:"total_traces"`
	// RubricResults Cohen's Kappa for each rubric
	RubricResults map[string]*RubricAgreement `json:"rubric_results"`
	// RubricFailures rubrics below target threshold (κ < 0.6), sorted by impact
	RubricFailures []*RubricRefinement `json:"rubric_failures"`
	// AnnotatorStats statistics per annotator (count, avg confidence, agreement)
	AnnotatorStats map[string]*AnnotatorStat `json:"annotator_stats"`
	// ConfidenceAnalysis relationship between confidence and agreement
	ConfidenceAnalysis *ConfidenceAnalysis `json:"confidence_analysis"`
}

// RubricAgreement summarizes agreement for a single rubric.
type RubricAgreement struct {
	RubricID         string  `json:"rubric_id"`
	KappaScore       float64 `json:"kappa_score"`
	Interpretation   string  `json:"interpretation"`
	PoAgreement      float64 `json:"po_agreement"`      // observed agreement
	PeAgreement      float64 `json:"pe_agreement"`      // expected agreement
	AnnotationPairs  int     `json:"annotation_pairs"`  // pairwise comparisons
	DisagreementRate float64 `json:"disagreement_rate"` // % of pairs that disagree
}

// RubricRefinement provides actionable guidance for improving a rubric.
type RubricRefinement struct {
	RubricID          string   `json:"rubric_id"`
	KappaScore        float64  `json:"kappa_score"`
	Priority          string   `json:"priority"` // "critical" (κ < 0.4), "high" (0.4-0.5), "medium" (0.5-0.6)
	Issues            []string `json:"issues"`
	SuggestedCriteria []string `json:"suggested_criteria"`
	ExamplePairs      []*AnnotationPair `json:"example_pairs"` // cases of disagreement
}

// AnnotationPair captures a disagreement between two annotators for the same trace.
type AnnotationPair struct {
	TraceID        string `json:"trace_id"`
	AnnotatorA     string `json:"annotator_a"`
	AnnotatorB     string `json:"annotator_b"`
	ResponseA      string `json:"response_a"`
	ResponseB      string `json:"response_b"`
	JustificationA string `json:"justification_a"`
	JustificationB string `json:"justification_b"`
}

// AnnotatorStat tracks individual annotator performance.
type AnnotatorStat struct {
	AnnotatorID       string  `json:"annotator_id"`
	AnnotationCount   int     `json:"annotation_count"`
	AverageConfidence float64 `json:"average_confidence"`
	AgreementRate     float64 `json:"agreement_rate"` // % of annotations in agreement with others
}

// ConfidenceAnalysis examines relationship between confidence and agreement.
type ConfidenceAnalysis struct {
	HighConfidenceAgreement   float64 `json:"high_confidence_agreement"`   // κ for annotations >= 80% confidence
	MediumConfidenceAgreement float64 `json:"medium_confidence_agreement"` // κ for 60-80% confidence
	LowConfidenceAgreement    float64 `json:"low_confidence_agreement"`    // κ for < 60% confidence
	Observation               string  `json:"observation"`                 // insight on confidence-agreement link
}

// CohensKappa calculates inter-rater agreement using Cohen's Kappa coefficient.
//
// Kappa formula: κ = (Po - Pe) / (1 - Pe)
// where:
//   Po = observed agreement (proportion of concordant pairs)
//   Pe = expected agreement by chance (marginal probability)
//
// κ interpretation:
//   κ < 0.0: Poor agreement (worse than chance)
//   κ 0.0-0.2: Slight agreement
//   κ 0.2-0.4: Fair agreement
//   κ 0.4-0.6: Moderate agreement
//   κ 0.6-0.8: Substantial agreement (TARGET)
//   κ 0.8-1.0: Almost perfect agreement
//
// Args:
//   annotations: map[string]*Annotation keyed by unique annotation ID (traceID:annotatorID recommended)
//
// Returns:
//   *AgreementResult with overall κ, per-rubric κ, failures, and annotator statistics
func CohensKappa(annotations map[string]*Annotation) *AgreementResult {
	result := &AgreementResult{
		TotalAnnotations: len(annotations),
		RubricResults:    make(map[string]*RubricAgreement),
		RubricFailures:   []*RubricRefinement{},
		AnnotatorStats:   make(map[string]*AnnotatorStat),
	}

	if len(annotations) == 0 {
		return result
	}

	// Step 1: Group annotations by trace
	traceAnnotations := groupAnnotationsByTrace(annotations)
	result.TotalTraces = len(traceAnnotations)

	// Step 2: Calculate agreement on primary failure
	primaryAgreement := calculatePrimaryFailureAgreement(traceAnnotations)
	result.KappaOverall = primaryAgreement.Kappa
	result.InterpretationOverall = interpretKappa(primaryAgreement.Kappa)

	// Step 3: Calculate per-rubric agreement
	rubricAgreements := calculateRubricAgreement(traceAnnotations)
	for rubricID, agreement := range rubricAgreements {
		agreement.Interpretation = interpretKappa(agreement.KappaScore)
		result.RubricResults[rubricID] = agreement

		// Flag rubrics below 0.6 threshold
		if agreement.KappaScore < 0.6 {
			result.RubricFailures = append(result.RubricFailures, &RubricRefinement{
				RubricID:   rubricID,
				KappaScore: agreement.KappaScore,
				Priority:   prioritizeRubric(agreement.KappaScore),
				Issues:     identifyDisagreements(traceAnnotations, rubricID),
				ExamplePairs: findDisagreementExamples(traceAnnotations, rubricID, 3),
			})
		}
	}

	// Sort failures by impact (κ ascending = worst first)
	sort.Slice(result.RubricFailures, func(i, j int) bool {
		return result.RubricFailures[i].KappaScore < result.RubricFailures[j].KappaScore
	})

	// Step 4: Calculate per-annotator statistics
	calculateAnnotatorStats(traceAnnotations, result)

	// Step 5: Analyze confidence-agreement relationship
	result.ConfidenceAnalysis = analyzeConfidenceImpact(annotations, result)

	return result
}

// groupAnnotationsByTrace organizes annotations by the trace they evaluate.
func groupAnnotationsByTrace(annotations map[string]*Annotation) map[string][]*Annotation {
	grouped := make(map[string][]*Annotation)
	for _, ann := range annotations {
		grouped[ann.TraceID] = append(grouped[ann.TraceID], ann)
	}
	return grouped
}

// calculatePrimaryFailureAgreement computes κ for primary failure classification.
func calculatePrimaryFailureAgreement(traceAnnotations map[string][]*Annotation) *AgreementMetrics {
	var pairs []*ComparisonPair
	var poCount, totalPairs float64

	// For each trace with multiple annotators, generate pairwise comparisons
	for traceID, annotations := range traceAnnotations {
		if len(annotations) < 2 {
			continue // Need at least 2 annotators for agreement
		}

		// Generate all pairwise comparisons for this trace
		for i := 0; i < len(annotations); i++ {
			for j := i + 1; j < len(annotations); j++ {
				pair := &ComparisonPair{
					ItemID: traceID,
					Rater1: annotations[i].AnnotatorID,
					Rater2: annotations[j].AnnotatorID,
					Code1:  annotations[i].PrimaryFailure,
					Code2:  annotations[j].PrimaryFailure,
				}
				pairs = append(pairs, pair)
				totalPairs++
				if pair.Code1 == pair.Code2 {
					poCount++
				}
			}
		}
	}

	if totalPairs == 0 {
		return &AgreementMetrics{Kappa: 0, Po: 0, Pe: 0}
	}

	po := poCount / totalPairs
	pe := calculateExpectedAgreement(pairs)
	kappa := (po - pe) / (1 - pe)
	if math.IsNaN(kappa) {
		kappa = 0 // Handle case where pe = 1 (perfect chance agreement)
	}

	return &AgreementMetrics{Kappa: kappa, Po: po, Pe: pe}
}

// calculateRubricAgreement computes κ for each rubric independently.
func calculateRubricAgreement(traceAnnotations map[string][]*Annotation) map[string]*RubricAgreement {
	results := make(map[string]*RubricAgreement)
	rubricCounts := make(map[string]*rubricMetrics)

	// For each trace and pair of annotators, compare rubric flags
	for _, annotations := range traceAnnotations {
		if len(annotations) < 2 {
			continue
		}

		// Collect all unique rubrics mentioned
		allRubrics := make(map[string]bool)
		for _, ann := range annotations {
			for _, rubric := range ann.RubricsFailed {
				allRubrics[rubric] = true
			}
		}

		// For each rubric, compare whether both annotators flagged it
		for rubric := range allRubrics {
			if rubricCounts[rubric] == nil {
				rubricCounts[rubric] = &rubricMetrics{}
			}

			for i := 0; i < len(annotations); i++ {
				for j := i + 1; j < len(annotations); j++ {
					flag1 := contains(annotations[i].RubricsFailed, rubric)
					flag2 := contains(annotations[j].RubricsFailed, rubric)

					rubricCounts[rubric].totalPairs++
					if flag1 == flag2 {
						rubricCounts[rubric].agreePairs++
					}
				}
			}
		}
	}

	// Calculate κ for each rubric
	for rubricID, metrics := range rubricCounts {
		po := float64(metrics.agreePairs) / float64(metrics.totalPairs)
		pe := 0.5 // Binary choice (flagged or not), random chance is 50%
		kappa := (po - pe) / (1 - pe)

		results[rubricID] = &RubricAgreement{
			RubricID:        rubricID,
			KappaScore:      kappa,
			PoAgreement:     po,
			PeAgreement:     pe,
			AnnotationPairs: metrics.totalPairs,
			DisagreementRate: 1.0 - po,
		}
	}

	return results
}

// calculateExpectedAgreement computes Pe given pairwise comparisons.
// Pe = Σ(p_i^2) where p_i is the marginal probability of code i.
func calculateExpectedAgreement(pairs []*ComparisonPair) float64 {
	if len(pairs) == 0 {
		return 0
	}

	// Count occurrences of each code
	codeCounts := make(map[string]int)
	totalCodes := 0
	for _, pair := range pairs {
		codeCounts[pair.Code1]++
		codeCounts[pair.Code2]++
		totalCodes += 2
	}

	// Calculate Pe
	var pe float64
	for _, count := range codeCounts {
		p := float64(count) / float64(totalCodes)
		pe += p * p
	}

	return pe
}

// interpretKappa provides English interpretation of κ value.
func interpretKappa(kappa float64) string {
	switch {
	case kappa < 0.0:
		return "Poor agreement (worse than chance)"
	case kappa <= 0.2:
		return "Slight agreement"
	case kappa <= 0.4:
		return "Fair agreement"
	case kappa <= 0.6:
		return "Moderate agreement"
	case kappa <= 0.8:
		return "Substantial agreement"
	default:
		return "Almost perfect agreement"
	}
}

// prioritizeRubric assigns priority based on κ score.
func prioritizeRubric(kappa float64) string {
	switch {
	case kappa < 0.4:
		return "critical"
	case kappa < 0.5:
		return "high"
	default:
		return "medium"
	}
}

// identifyDisagreements catalogs types of disagreement for a rubric.
func identifyDisagreements(traceAnnotations map[string][]*Annotation, rubricID string) []string {
	issues := []string{}
	disagreementPatterns := make(map[string]int)

	for traceID, annotations := range traceAnnotations {
		_ = traceID // unused in current implementation
		if len(annotations) < 2 {
			continue
		}

		for i := 0; i < len(annotations); i++ {
			for j := i + 1; j < len(annotations); j++ {
				flag1 := contains(annotations[i].RubricsFailed, rubricID)
				flag2 := contains(annotations[j].RubricsFailed, rubricID)

				if flag1 != flag2 {
					if flag1 {
						disagreementPatterns["false_negatives"]++
					} else {
						disagreementPatterns["false_positives"]++
					}
				}
			}
		}
	}

	if count := disagreementPatterns["false_negatives"]; count > 0 {
		issues = append(issues, fmt.Sprintf("False negatives: %d (annotators missed failures)", count))
	}
	if count := disagreementPatterns["false_positives"]; count > 0 {
		issues = append(issues, fmt.Sprintf("False positives: %d (annotators over-flagged)", count))
	}

	return issues
}

// findDisagreementExamples returns examples of annotator disagreement.
func findDisagreementExamples(traceAnnotations map[string][]*Annotation, rubricID string, maxExamples int) []*AnnotationPair {
	var examples []*AnnotationPair

	for traceID, annotations := range traceAnnotations {
		if len(examples) >= maxExamples {
			break
		}
		if len(annotations) < 2 {
			continue
		}

		for i := 0; i < len(annotations) && len(examples) < maxExamples; i++ {
			for j := i + 1; j < len(annotations) && len(examples) < maxExamples; j++ {
				flag1 := contains(annotations[i].RubricsFailed, rubricID)
				flag2 := contains(annotations[j].RubricsFailed, rubricID)

				if flag1 != flag2 {
					examples = append(examples, &AnnotationPair{
						TraceID:        traceID,
						AnnotatorA:     annotations[i].AnnotatorID,
						AnnotatorB:     annotations[j].AnnotatorID,
						ResponseA:      boolToString(flag1),
						ResponseB:      boolToString(flag2),
						JustificationA: annotations[i].Notes,
						JustificationB: annotations[j].Notes,
					})
				}
			}
		}
	}

	return examples
}

// calculateAnnotatorStats computes per-annotator metrics.
func calculateAnnotatorStats(traceAnnotations map[string][]*Annotation, result *AgreementResult) {
	annotatorData := make(map[string]*annotatorMetrics)

	// Collect per-annotator data
	for _, annotations := range traceAnnotations {
		for _, ann := range annotations {
			if annotatorData[ann.AnnotatorID] == nil {
				annotatorData[ann.AnnotatorID] = &annotatorMetrics{}
			}
			annotatorData[ann.AnnotatorID].count++
			annotatorData[ann.AnnotatorID].totalConfidence += float64(ann.Confidence)
		}
	}

	// Calculate agreement rate for each annotator
	for annotatorID, metrics := range annotatorData {
		avgConfidence := metrics.totalConfidence / float64(metrics.count)

		// Count annotations in agreement with peers
		agreementCount := 0
		comparisonCount := 0
		for _, annotations := range traceAnnotations {
			var thisAnn *Annotation
			var otherAnns []*Annotation
			for _, ann := range annotations {
				if ann.AnnotatorID == annotatorID {
					thisAnn = ann
				} else {
					otherAnns = append(otherAnns, ann)
				}
			}

			if thisAnn != nil && len(otherAnns) > 0 {
				for _, other := range otherAnns {
					comparisonCount++
					if thisAnn.PrimaryFailure == other.PrimaryFailure {
						agreementCount++
					}
				}
			}
		}

		agreementRate := 0.0
		if comparisonCount > 0 {
			agreementRate = float64(agreementCount) / float64(comparisonCount)
		}

		result.AnnotatorStats[annotatorID] = &AnnotatorStat{
			AnnotatorID:       annotatorID,
			AnnotationCount:   metrics.count,
			AverageConfidence: avgConfidence,
			AgreementRate:     agreementRate,
		}
	}
}

// analyzeConfidenceImpact examines how confidence correlates with agreement.
// Note: To avoid infinite recursion, this calculates agreement only within
// each confidence band WITHOUT recursively calling CohensKappa.
func analyzeConfidenceImpact(annotations map[string]*Annotation, result *AgreementResult) *ConfidenceAnalysis {
	ca := &ConfidenceAnalysis{}

	// Separate annotations by confidence level
	var highConf, mediumConf, lowConf []*Annotation
	for _, ann := range annotations {
		switch {
		case ann.Confidence >= 80:
			highConf = append(highConf, ann)
		case ann.Confidence >= 60:
			mediumConf = append(mediumConf, ann)
		default:
			lowConf = append(lowConf, ann)
		}
	}

	// Calculate agreement directly within each confidence band
	ca.HighConfidenceAgreement = calculateBandAgreement(highConf)
	ca.MediumConfidenceAgreement = calculateBandAgreement(mediumConf)
	ca.LowConfidenceAgreement = calculateBandAgreement(lowConf)

	// Generate observation
	if ca.HighConfidenceAgreement > ca.LowConfidenceAgreement {
		ca.Observation = "Higher confidence correlates with better agreement; consider requiring minimum confidence threshold"
	} else {
		ca.Observation = "Confidence does not strongly correlate with agreement; rubric clarity may be the limiting factor"
	}

	return ca
}

// calculateBandAgreement computes agreement within an annotation band (non-recursive).
func calculateBandAgreement(annotations []*Annotation) float64 {
	if len(annotations) < 2 {
		return 0.0
	}

	// Group by trace
	traceAnns := make(map[string][]*Annotation)
	for _, ann := range annotations {
		traceAnns[ann.TraceID] = append(traceAnns[ann.TraceID], ann)
	}

	var agreements float64
	var totalPairs float64

	// Calculate agreement on primary failure only
	for _, anns := range traceAnns {
		if len(anns) < 2 {
			continue
		}
		for i := 0; i < len(anns); i++ {
			for j := i + 1; j < len(anns); j++ {
				totalPairs++
				if anns[i].PrimaryFailure == anns[j].PrimaryFailure {
					agreements++
				}
			}
		}
	}

	if totalPairs == 0 {
		return 0.0
	}

	return agreements / totalPairs
}

// Helper functions

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// boolToString converts bool to "yes"/"no".
func boolToString(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Internal types for calculation

type ComparisonPair struct {
	ItemID string
	Rater1 string
	Rater2 string
	Code1  string
	Code2  string
}

type AgreementMetrics struct {
	Kappa float64
	Po    float64
	Pe    float64
}

type rubricMetrics struct {
	totalPairs int
	agreePairs int
}

type annotatorMetrics struct {
	count           int
	totalConfidence float64
}
