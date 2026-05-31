package singleturn_test

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/singleturn"
)

func result(tools ...string) singleturn.SingleTurnResult {
	calls := make([]singleturn.ToolCall, len(tools))
	for i, t := range tools {
		calls[i] = singleturn.ToolCall{ToolName: t}
	}
	return singleturn.SingleTurnResult{ToolCalls: calls, ToolNames: tools, SelectedAny: len(tools) > 0}
}

// ─── ToolsSelected ─────────────────────────────────────────────────────────

func TestToolsSelected_AllPresent(t *testing.T) {
	r := result("read_file", "write_file")
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file", "write_file"}}
	if singleturn.ToolsSelected(r, target) != 1 {
		t.Error("all expected tools present → should be 1")
	}
}

func TestToolsSelected_Missing(t *testing.T) {
	r := result("read_file")
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file", "write_file"}}
	if singleturn.ToolsSelected(r, target) != 0 {
		t.Error("missing write_file → should be 0")
	}
}

func TestToolsSelected_ExtraToolsOK(t *testing.T) {
	r := result("read_file", "list_files", "write_file")
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file", "write_file"}}
	if singleturn.ToolsSelected(r, target) != 1 {
		t.Error("extra tools should not penalise golden score")
	}
}

func TestToolsSelected_EmptyExpected(t *testing.T) {
	r := result()
	target := singleturn.EvalTarget{}
	if singleturn.ToolsSelected(r, target) != 1 {
		t.Error("no expected tools → should pass")
	}
}

// ─── ToolsAvoided ──────────────────────────────────────────────────────────

func TestToolsAvoided_NoForbiddenUsed(t *testing.T) {
	r := result("read_file")
	target := singleturn.EvalTarget{ForbiddenTools: []string{"delete_file", "run_command"}}
	if singleturn.ToolsAvoided(r, target) != 1 {
		t.Error("no forbidden tool used → should be 1")
	}
}

func TestToolsAvoided_ForbiddenUsed(t *testing.T) {
	r := result("read_file", "run_command")
	target := singleturn.EvalTarget{ForbiddenTools: []string{"run_command"}}
	if singleturn.ToolsAvoided(r, target) != 0 {
		t.Error("forbidden tool used → should be 0")
	}
}

func TestToolsAvoided_NoTools(t *testing.T) {
	r := result() // no tools used
	target := singleturn.EvalTarget{ForbiddenTools: []string{"delete_file"}}
	if singleturn.ToolsAvoided(r, target) != 1 {
		t.Error("no tools used → all forbidden tools avoided → should be 1")
	}
}

func TestToolsAvoided_EmptyForbidden(t *testing.T) {
	r := result("run_command")
	target := singleturn.EvalTarget{}
	if singleturn.ToolsAvoided(r, target) != 1 {
		t.Error("empty forbidden list → should always pass")
	}
}

// ─── ToolSelectionF1 ───────────────────────────────────────────────────────

func TestF1_PerfectMatch(t *testing.T) {
	r := result("read_file", "write_file")
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file", "write_file"}}
	if singleturn.ToolSelectionF1(r, target) != 1.0 {
		t.Error("perfect match → F1 should be 1.0")
	}
}

func TestF1_PartialMatch(t *testing.T) {
	r := result("read_file") // selected 1 of 2
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file", "write_file"}}
	f1 := singleturn.ToolSelectionF1(r, target)
	// precision=1.0, recall=0.5, F1 = 2*1.0*0.5/1.5 ≈ 0.667
	if f1 <= 0 || f1 >= 1 {
		t.Errorf("partial match should be between 0 and 1, got %.3f", f1)
	}
}

func TestF1_NoMatch(t *testing.T) {
	r := result("list_files") // wrong tool
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file"}}
	if singleturn.ToolSelectionF1(r, target) != 0 {
		t.Error("no match → F1 should be 0")
	}
}

func TestF1_Empty(t *testing.T) {
	r := result()
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file"}}
	if singleturn.ToolSelectionF1(r, target) != 0 {
		t.Error("no tools selected → F1 should be 0")
	}
}

// ─── Score dispatcher ──────────────────────────────────────────────────────

func TestScore_GoldenCategory(t *testing.T) {
	r := result("read_file")
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file"}, Category: singleturn.CategoryGolden}
	scores := singleturn.Score(r, target)
	if _, ok := scores["tools_selected"]; !ok {
		t.Error("golden category should produce tools_selected score")
	}
	if _, ok := scores["tools_avoided"]; ok {
		t.Error("golden category should not produce tools_avoided score")
	}
}

func TestScore_NegativeCategory(t *testing.T) {
	r := result()
	target := singleturn.EvalTarget{ForbiddenTools: []string{"run_command"}, Category: singleturn.CategoryNegative}
	scores := singleturn.Score(r, target)
	if _, ok := scores["tools_avoided"]; !ok {
		t.Error("negative category should produce tools_avoided score")
	}
}

func TestScore_SecondaryCategory(t *testing.T) {
	r := result("read_file")
	target := singleturn.EvalTarget{ExpectedTools: []string{"read_file"}, Category: singleturn.CategorySecondary}
	scores := singleturn.Score(r, target)
	if _, ok := scores["tool_selection_f1"]; !ok {
		t.Error("secondary category should produce tool_selection_f1 score")
	}
}
