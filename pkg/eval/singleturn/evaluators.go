package singleturn

// Evaluators for single-turn tool-selection evals (lesson 03).
//
// Three scorers mirroring the TypeScript evaluators.ts:
//
//	toolsSelected   — binary: did agent select ALL expected tools? (golden)
//	toolsAvoided    — binary: did agent avoid ALL forbidden tools? (negative)
//	ToolSelectionF1 — F1 score: precision × recall balance (secondary)

// ToolsSelected returns 1 if all expected tools are present in the result,
// 0 otherwise. Intended for "golden" category test cases.
func ToolsSelected(result SingleTurnResult, target EvalTarget) float64 {
	if len(target.ExpectedTools) == 0 {
		return 1
	}
	selected := make(map[string]bool, len(result.ToolNames))
	for _, t := range result.ToolNames {
		selected[t] = true
	}
	for _, expected := range target.ExpectedTools {
		if !selected[expected] {
			return 0
		}
	}
	return 1
}

// ToolsAvoided returns 1 if none of the forbidden tools appear in the result,
// 0 if any forbidden tool was selected. Intended for "negative" category.
func ToolsAvoided(result SingleTurnResult, target EvalTarget) float64 {
	if len(target.ForbiddenTools) == 0 {
		return 1
	}
	selected := make(map[string]bool, len(result.ToolNames))
	for _, t := range result.ToolNames {
		selected[t] = true
	}
	for _, forbidden := range target.ForbiddenTools {
		if selected[forbidden] {
			return 0
		}
	}
	return 1
}

// ToolSelectionF1 computes the F1 score (harmonic mean of precision and recall)
// for partial credit. Intended for "secondary" category test cases where
// multiple valid approaches exist.
//
//	precision = |selected ∩ expected| / |selected|
//	recall    = |selected ∩ expected| / |expected|
//	F1        = 2 × (precision × recall) / (precision + recall)
func ToolSelectionF1(result SingleTurnResult, target EvalTarget) float64 {
	if len(target.ExpectedTools) == 0 {
		return 1
	}
	if len(result.ToolNames) == 0 {
		return 0
	}

	expectedSet := make(map[string]bool, len(target.ExpectedTools))
	for _, t := range target.ExpectedTools {
		expectedSet[t] = true
	}

	selectedSet := make(map[string]bool, len(result.ToolNames))
	for _, t := range result.ToolNames {
		selectedSet[t] = true
	}

	// Intersection
	var truePositives float64
	for t := range selectedSet {
		if expectedSet[t] {
			truePositives++
		}
	}

	precision := truePositives / float64(len(selectedSet))
	recall := truePositives / float64(len(expectedSet))

	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

// Score runs the appropriate evaluator(s) based on the test case category.
// Returns a map of scorer name → score (0–1).
func Score(result SingleTurnResult, target EvalTarget) map[string]float64 {
	scores := make(map[string]float64)
	switch target.Category {
	case CategoryGolden:
		scores["tools_selected"] = ToolsSelected(result, target)
	case CategoryNegative:
		scores["tools_avoided"] = ToolsAvoided(result, target)
	case CategorySecondary:
		scores["tool_selection_f1"] = ToolSelectionF1(result, target)
	default:
		// Run all scorers for uncategorised cases.
		scores["tools_selected"] = ToolsSelected(result, target)
		scores["tools_avoided"] = ToolsAvoided(result, target)
		scores["tool_selection_f1"] = ToolSelectionF1(result, target)
	}
	return scores
}
