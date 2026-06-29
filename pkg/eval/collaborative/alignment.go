package collaborative

import (
	"fmt"
	"sort"
	"strings"
)

// RubricAlignmentWorkflow manages the iterative refinement of rubric evaluation criteria.
//
// Workflow:
// 1. Collect annotations from multiple evaluators
// 2. Calculate Cohen's Kappa per rubric
// 3. Identify rubrics with κ < 0.6 (threshold for "substantial agreement")
// 4. Analyze disagreement patterns (false positives vs negatives)
// 5. Generate refinement suggestions
// 6. Track refinement history across iterations
//
// The goal is to progressively improve rubric clarity until κ ≥ 0.6 is consistently achieved.
type RubricAlignmentWorkflow struct {
	// RubricID the rubric being refined
	RubricID string
	// Name descriptive name
	Name string
	// Description original rubric description
	Description string
	// CurrentCriteria existing evaluation criteria
	CurrentCriteria []string
	// KappaHistory κ scores across refinement iterations
	KappaHistory []float64
	// RefinementIterations improvements made
	RefinementIterations []*RefinementIteration
	// TargetKappa goal κ score (default 0.6)
	TargetKappa float64
	// Status "pending", "in_progress", "accepted", "resolved"
	Status string
}

// RefinementIteration captures one round of rubric improvement.
type RefinementIteration struct {
	// IterationNumber 1-indexed iteration
	IterationNumber int `json:"iteration_number"`
	// PreviousKappa κ before refinement
	PreviousKappa float64 `json:"previous_kappa"`
	// NewKappa κ after refinement
	NewKappa float64 `json:"new_kappa"`
	// ChangePercent improvement ((new-old)/old)*100
	ChangePercent float64 `json:"change_percent"`
	// RefinedCriteria updated criteria
	RefinedCriteria []string `json:"refined_criteria"`
	// Changes human-readable description of changes
	Changes []string `json:"changes"`
	// DateApplied when the refinement was implemented
	DateApplied string `json:"date_applied"`
	// Evidence supporting evidence for the changes (examples of resolved disagreement)
	Evidence []string `json:"evidence"`
	// ResolvedDisagreements cases that are now correctly classified
	ResolvedDisagreements []string `json:"resolved_disagreements"`
}

// RubricRefiner generates actionable refinement suggestions.
type RubricRefiner struct {
	// RubricDefinitions maps rubric ID to its definition (from eval.AllRubrics)
	RubricDefinitions map[string]interface{}
}

// NewRubricRefiner creates a new refiner.
func NewRubricRefiner(rubricDefs map[string]interface{}) *RubricRefiner {
	return &RubricRefiner{
		RubricDefinitions: rubricDefs,
	}
}

// GenerateRefinements produces concrete suggestions for improving failing rubrics.
//
// Approach:
// 1. Analyze disagreement patterns in annotated data
// 2. Identify ambiguous criteria
// 3. Propose concrete rewording
// 4. Suggest additional test procedures or evidence requirements
//
// Returns slice sorted by priority (critical first).
func (r *RubricRefiner) GenerateRefinements(agreementResult *AgreementResult) []*RefinementWorksheet {
	worksheets := []*RefinementWorksheet{}

	for _, failure := range agreementResult.RubricFailures {
		worksheet := &RefinementWorksheet{
			RubricID:           failure.RubricID,
			CurrentKappa:       failure.KappaScore,
			Priority:           failure.Priority,
			Issues:             failure.Issues,
			DisagreementCount:  len(failure.ExamplePairs),
			Recommendations:    generateRecommendations(failure),
			ExamplesOfError:    failure.ExamplePairs,
			EstimatedImpact:    estimateRefinementImpact(failure.KappaScore),
			ProposedCriteria:   proposeCriteria(failure),
			ValidationStrategy: describeValidationStrategy(failure),
		}
		worksheets = append(worksheets, worksheet)
	}

	// Sort by priority
	sort.Slice(worksheets, func(i, j int) bool {
		priorityMap := map[string]int{"critical": 0, "high": 1, "medium": 2}
		return priorityMap[worksheets[i].Priority] < priorityMap[worksheets[j].Priority]
	})

	return worksheets
}

// RefinementWorksheet provides actionable guidance for one rubric.
type RefinementWorksheet struct {
	RubricID            string             `json:"rubric_id"`
	CurrentKappa        float64            `json:"current_kappa"`
	Priority            string             `json:"priority"` // critical, high, medium
	Issues              []string           `json:"issues"`   // types of disagreement observed
	DisagreementCount   int                `json:"disagreement_count"`
	Recommendations     []string           `json:"recommendations"`     // specific actionable steps
	ExamplesOfError     []*AnnotationPair  `json:"examples_of_error"`   // concrete disagreement cases
	EstimatedImpact     string             `json:"estimated_impact"`    // expected κ improvement
	ProposedCriteria    []string           `json:"proposed_criteria"`   // revised evaluation criteria
	ValidationStrategy  string             `json:"validation_strategy"` // how to test the refinement
}

// AlignmentReport tracks progress toward κ ≥ 0.6 across refinements.
type AlignmentReport struct {
	// Timestamp when report generated
	Timestamp string `json:"timestamp"`
	// RubricsEvaluated total count
	RubricsEvaluated int `json:"rubrics_evaluated"`
	// RubricsPassing count with κ ≥ 0.6
	RubricsPassing int `json:"rubrics_passing"`
	// RubricsCritical count with κ < 0.4 (require urgent attention)
	RubricsCritical int `json:"rubrics_critical"`
	// PassRate percentage passing ((passing/evaluated)*100)
	PassRate float64 `json:"pass_rate"`
	// AverageKappa mean κ across all rubrics
	AverageKappa float64 `json:"average_kappa"`
	// MedianKappa median κ
	MedianKappa float64 `json:"median_kappa"`
	// MostProblematicRubric rubric with lowest κ
	MostProblematicRubric *RubricFailureSummary `json:"most_problematic_rubric"`
	// LeastProblematicRubric rubric with highest κ
	LeastProblematicRubric *RubricFailureSummary `json:"least_problematic_rubric"`
	// RefinementPlan ordered list of worksheets
	RefinementPlan []*RefinementWorksheet `json:"refinement_plan"`
	// EstimatedEffort person-hours to complete refinements
	EstimatedEffort float64 `json:"estimated_effort"`
	// RoadmapMilestones phases to reach target
	RoadmapMilestones []*RoadmapMilestone `json:"roadmap_milestones"`
}

// RubricFailureSummary minimal summary for ranking rubrics.
type RubricFailureSummary struct {
	RubricID          string  `json:"rubric_id"`
	KappaScore        float64 `json:"kappa_score"`
	Priority          string  `json:"priority"`
	DisagreementCount int     `json:"disagreement_count"`
}

// RoadmapMilestone describes a refinement phase.
type RoadmapMilestone struct {
	Phase           string   `json:"phase"`
	Description     string   `json:"description"`
	TargetKappa     float64  `json:"target_kappa"`
	RubricsInScope  []string `json:"rubrics_in_scope"`
	EstimatedWeeks  int      `json:"estimated_weeks"`
	SuccessCriteria []string `json:"success_criteria"`
}

// GenerateAlignmentReport synthesizes results and roadmap.
func GenerateAlignmentReport(agreementResult *AgreementResult, refiner *RubricRefiner) *AlignmentReport {
	report := &AlignmentReport{
		Timestamp:      getCurrentTimestamp(),
		RubricsEvaluated: len(agreementResult.RubricResults),
	}

	// Count passing and critical
	var kappaScores []float64
	for _, rubricResult := range agreementResult.RubricResults {
		kappaScores = append(kappaScores, rubricResult.KappaScore)
		if rubricResult.KappaScore >= 0.6 {
			report.RubricsPassing++
		}
		if rubricResult.KappaScore < 0.4 {
			report.RubricsCritical++
		}
	}

	if report.RubricsEvaluated > 0 {
		report.PassRate = float64(report.RubricsPassing) / float64(report.RubricsEvaluated) * 100.0
		report.AverageKappa = average(kappaScores)
		report.MedianKappa = median(kappaScores)
	}

	// Find most/least problematic
	if len(agreementResult.RubricFailures) > 0 {
		worst := agreementResult.RubricFailures[0]
		report.MostProblematicRubric = &RubricFailureSummary{
			RubricID:          worst.RubricID,
			KappaScore:        worst.KappaScore,
			Priority:          worst.Priority,
			DisagreementCount: len(worst.ExamplePairs),
		}
	}

	best := findBestRubric(agreementResult.RubricResults)
	if best != nil {
		report.LeastProblematicRubric = &RubricFailureSummary{
			RubricID:   best.RubricID,
			KappaScore: best.KappaScore,
			Priority:   "none",
		}
	}

	// Generate refinement plan
	report.RefinementPlan = refiner.GenerateRefinements(agreementResult)
	report.EstimatedEffort = calculateEffort(report.RefinementPlan)

	// Create roadmap
	report.RoadmapMilestones = buildRoadmap(report)

	return report
}

// Helper functions for refinement generation

func generateRecommendations(failure *RubricRefinement) []string {
	recommendations := []string{}

	// Analyze disagreement patterns
	hasIssues := len(failure.Issues) > 0

	if hasIssues {
		for _, issue := range failure.Issues {
			if strings.Contains(issue, "false_negatives") {
				recommendations = append(recommendations, "Add more specific conditions that trigger this rubric (reduce missed cases)")
				recommendations = append(recommendations, "Provide examples of what constitutes a violation vs. non-violation")
			}
			if strings.Contains(issue, "false_positives") {
				recommendations = append(recommendations, "Clarify boundary cases and exceptions (avoid over-flagging)")
				recommendations = append(recommendations, "Define explicit thresholds or acceptance criteria")
			}
		}
	}

	// General guidance based on κ
	if failure.KappaScore < 0.3 {
		recommendations = append(recommendations, "Consider breaking rubric into more granular sub-criteria")
		recommendations = append(recommendations, "Conduct annotator training or calibration session")
	}

	if len(failure.ExamplePairs) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d disagreement cases with annotators", len(failure.ExamplePairs)))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "General: test proposed changes with at least 2 independent annotators")
	}

	return recommendations
}

func proposeCriteria(failure *RubricRefinement) []string {
	criteria := []string{}

	// Generic criteria suggestions based on issue type

	switch {
	case strings.Contains(failure.RubricID, "-SE-"):
		criteria = append(criteria, "Include specific ledger state requirements (before/after)")
		criteria = append(criteria, "Define what constitutes 'successful' settlement for this rubric")

	case strings.Contains(failure.RubricID, "-CO-"):
		criteria = append(criteria, "Specify workflow state transitions required")
		criteria = append(criteria, "Define timeout or retry boundaries")

	case strings.Contains(failure.RubricID, "-CM-"):
		criteria = append(criteria, "Clarify merchant vs. customer responsibility boundaries")
		criteria = append(criteria, "Define dispute resolution scope")

	case strings.Contains(failure.RubricID, "-LA-"):
		criteria = append(criteria, "Specify what lineage fields must be present")
		criteria = append(criteria, "Define chain-of-custody verification steps")
	}

	criteria = append(criteria, fmt.Sprintf("Add test procedure: how to verify this rubric (currently κ=%.2f)", failure.KappaScore))

	return criteria
}

func estimateRefinementImpact(kappa float64) string {
	switch {
	case kappa < 0.2:
		return "Potential 20-30 point improvement (major clarification needed)"
	case kappa < 0.4:
		return "Potential 15-25 point improvement (substantive changes recommended)"
	case kappa < 0.5:
		return "Potential 10-15 point improvement (targeted refinement)"
	default:
		return "Potential 5-10 point improvement (minor tuning)"
	}
}

func describeValidationStrategy(failure *RubricRefinement) string {
	return fmt.Sprintf(
		"1. Propose revised criteria\n"+
			"2. Conduct 3+ annotator training with examples\n"+
			"3. Have 3 independent annotators re-evaluate %d disputed cases\n"+
			"4. Recalculate κ; target κ ≥ 0.6\n"+
			"5. If κ < 0.6, iterate refinement\n",
		len(failure.ExamplePairs))
}

func extractDomain(rubricID string) string {
	// RB-DOMAIN-NNN format
	parts := strings.Split(rubricID, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "UNKNOWN"
}

func findBestRubric(results map[string]*RubricAgreement) *RubricAgreement {
	var best *RubricAgreement
	for _, r := range results {
		if best == nil || r.KappaScore > best.KappaScore {
			best = r
		}
	}
	return best
}

func calculateEffort(plan []*RefinementWorksheet) float64 {
	effort := 0.0
	for _, ws := range plan {
		switch ws.Priority {
		case "critical":
			effort += 8.0 // ~1 day
		case "high":
			effort += 4.0 // ~half day
		case "medium":
			effort += 2.0 // ~2-3 hours
		}
	}
	return effort
}

func buildRoadmap(report *AlignmentReport) []*RoadmapMilestone {
	milestones := []*RoadmapMilestone{}

	// Phase 1: Critical fixes (κ < 0.4)
	criticalRubrics := []string{}
	for _, failure := range report.RefinementPlan {
		if failure.Priority == "critical" {
			criticalRubrics = append(criticalRubrics, failure.RubricID)
		}
	}

	if len(criticalRubrics) > 0 {
		milestones = append(milestones, &RoadmapMilestone{
			Phase:          "Phase 1: Critical",
			Description:    "Fix ambiguities in rubrics with κ < 0.4",
			TargetKappa:    0.4,
			RubricsInScope: criticalRubrics,
			EstimatedWeeks: 1,
			SuccessCriteria: []string{
				fmt.Sprintf("All %d critical rubrics reach κ ≥ 0.4", len(criticalRubrics)),
				"Annotator training completed",
				"Refined criteria documented",
			},
		})
	}

	// Phase 2: High priority (0.4 ≤ κ < 0.6)
	highRubrics := []string{}
	for _, failure := range report.RefinementPlan {
		if failure.Priority == "high" {
			highRubrics = append(highRubrics, failure.RubricID)
		}
	}

	if len(highRubrics) > 0 {
		milestones = append(milestones, &RoadmapMilestone{
			Phase:          "Phase 2: High Priority",
			Description:    "Achieve substantial agreement on remaining rubrics",
			TargetKappa:    0.6,
			RubricsInScope: highRubrics,
			EstimatedWeeks: 2,
			SuccessCriteria: []string{
				fmt.Sprintf("All %d high-priority rubrics reach κ ≥ 0.6", len(highRubrics)),
				"Overall pass rate reaches 80%+",
				"Validation tests pass with 3+ annotators",
			},
		})
	}

	// Phase 3: Polish (κ ≥ 0.6, targeting 0.8+)
	if report.PassRate > 50 {
		milestones = append(milestones, &RoadmapMilestone{
			Phase:       "Phase 3: Excellence",
			Description: "Optimize rubrics for near-perfect agreement",
			TargetKappa: 0.8,
			RubricsInScope: []string{"all passing rubrics"},
			EstimatedWeeks: 3,
			SuccessCriteria: []string{
				"Target: κ ≥ 0.8 for 90%+ of rubrics",
				"Annotator training refresher completed",
				"Public documentation finalized",
			},
		})
	}

	return milestones
}

func average(nums []float64) float64 {
	if len(nums) == 0 {
		return 0
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums))
}

func median(nums []float64) float64 {
	if len(nums) == 0 {
		return 0
	}
	sorted := make([]float64, len(nums))
	copy(sorted, nums)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2.0
}

func getCurrentTimestamp() string {
	// Placeholder; in production use time.Now().Format(time.RFC3339)
	return "2026-06-06T10:00:00Z"
}
