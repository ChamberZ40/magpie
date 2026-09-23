package feishu

import (
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// The tool panel has no detail knob of its own — it renders whatever fields the
// step carries. core expresses tool_detail = "summary" by leaving Status,
// ExitCode, Success and Result unset, so these tests pin the other end of that
// contract: an unset field must produce no line, not an empty one.

func TestRichStepBody_SummaryLevelRendersOneLine(t *testing.T) {
	step := core.ToolStep{
		Kind:    core.ToolStepKindTool,
		Name:    "Bash",
		Summary: "npm test",
		Done:    true,
	}

	got := richStepBody(step)

	if lines := strings.Split(got, "\n"); len(lines) != 1 {
		t.Fatalf("body = %q (%d lines), want a single line at summary detail", got, len(lines))
	}
	for _, unwanted := range []string{"status:", "exit:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("body = %q, want no %q — nothing filled those fields in", got, unwanted)
		}
	}
}

func TestRichStepBody_FullLevelKeepsStatusExitAndOutput(t *testing.T) {
	exit := 0
	success := true
	step := core.ToolStep{
		Kind:     core.ToolStepKindTool,
		Name:     "Bash",
		Summary:  "npm test",
		Status:   "completed",
		ExitCode: &exit,
		Success:  &success,
		Result:   "all 42 tests passed",
		Done:     true,
	}

	got := richStepBody(step)

	for _, want := range []string{"status: completed", "exit: 0", "all 42 tests passed"} {
		if !strings.Contains(got, want) {
			t.Errorf("body = %q, want it to contain %q", got, want)
		}
	}
}
