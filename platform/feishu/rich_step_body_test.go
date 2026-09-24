package feishu

import (
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// A turn's tool calls share one panel, so each call gets one line and has to
// earn every character on it. What a reader wants is which command ran, which
// file was read, which skill fired — not that six of them said "status: ok".
// core expresses tool_detail = "summary" by leaving Result unset, so these
// tests pin the other end of that contract too.

func TestRichStepBody_IsOneLineOfActionAndTarget(t *testing.T) {
	exit := 0
	success := true
	step := core.ToolStep{
		Kind:     core.ToolStepKindTool,
		Name:     "Bash",
		Summary:  `{"command":"npm test"}`,
		Status:   "completed",
		ExitCode: &exit,
		Success:  &success,
		Done:     true,
	}

	got := richStepBody(step, "en")

	if lines := strings.Split(got, "\n"); len(lines) != 1 {
		t.Fatalf("body = %q (%d lines), want one line per call", got, len(lines))
	}
	for _, want := range []string{"Run tests", "npm test"} {
		if !strings.Contains(got, want) {
			t.Errorf("body = %q, want it to contain %q", got, want)
		}
	}
	// A successful call is the normal case; saying so on every row is noise.
	for _, unwanted := range []string{"status:", "exit:", "✓", "Succeeded"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("body = %q, want no %q on a successful row", got, unwanted)
		}
	}
}

// Failure is the exception, and the exception is what a mark is for.
func TestRichStepBody_MarksFailuresWithTheExitCode(t *testing.T) {
	exit := 2
	failed := false
	step := core.ToolStep{
		Kind:     core.ToolStepKindTool,
		Name:     "Bash",
		Summary:  `{"command":"./deploy.sh"}`,
		Status:   "failed",
		ExitCode: &exit,
		Success:  &failed,
		Done:     true,
	}

	got := richStepBody(step, "en")

	for _, want := range []string{"./deploy.sh", "✗", "Failed", "exit 2"} {
		if !strings.Contains(got, want) {
			t.Errorf("body = %q, want it to contain %q", got, want)
		}
	}
}

// An exit code alone is enough to call it: no Success flag, no status word.
func TestRichStepBody_MarksAFailureKnownOnlyByItsExitCode(t *testing.T) {
	exit := 1
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"false"}`,
		ExitCode: &exit, Done: true,
	}

	if got := richStepBody(step, "en"); !strings.Contains(got, "✗") {
		t.Errorf("body = %q, want a failure mark for a non-zero exit code", got)
	}
}

// A status the renderer does not recognize must be echoed, not guessed at.
// Reporting "cancelled" as a clean success is worse than saying nothing.
func TestRichStepBody_EchoesAStatusItCannotJudge(t *testing.T) {
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Read", Summary: `{"file_path":"/x.go"}`,
		Status: "cancelled", Done: true,
	}

	got := richStepBody(step, "en")
	if !strings.Contains(got, "cancelled") {
		t.Errorf("body = %q, want the unrecognized status echoed", got)
	}
	if strings.Contains(got, "✗") {
		t.Errorf("body = %q, want no failure mark for a status that is not a failure", got)
	}
}

// tool_detail = "full" is the only thing that buys raw output, and it hangs
// under the one-line row rather than replacing it.
func TestRichStepBody_FullLevelAppendsTheRawOutput(t *testing.T) {
	exit := 0
	success := true
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"npm test"}`,
		Status: "completed", ExitCode: &exit, Success: &success,
		Result: "all 42 tests passed", Done: true,
	}

	got := richStepBody(step, "en")
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("body = %q (%d lines), want the row plus its output", got, len(lines))
	}
	if !strings.Contains(lines[0], "npm test") {
		t.Errorf("first line = %q, want the one-line row", lines[0])
	}
	if lines[1] != "all 42 tests passed" {
		t.Errorf("second line = %q, want the raw output", lines[1])
	}
}

// The failure mark is user-facing text, so it localizes like everything else.
func TestRichStepBody_FailureMarkIsLocalized(t *testing.T) {
	exit := 1
	step := core.ToolStep{
		Kind: core.ToolStepKindTool, Name: "Bash", Summary: `{"command":"false"}`,
		ExitCode: &exit, Done: true,
	}

	for _, tc := range []struct{ lang, want string }{
		{"en", "Failed"},
		{"zh", "失败"},
		{"ja", "失敗"},
	} {
		if got := richStepBody(step, tc.lang); !strings.Contains(got, tc.want) {
			t.Errorf("lang=%q body = %q, want it to contain %q", tc.lang, got, tc.want)
		}
	}
}

// Thinking rows share the renderer but none of this: they have no status, no
// exit code and no output to mark up.
func TestRichStepBody_LeavesThinkingRowsAlone(t *testing.T) {
	step := core.ToolStep{Kind: core.ToolStepKindThinking, Summary: "weighing options"}

	if got := richStepBody(step, "en"); got != "weighing options" {
		t.Errorf("body = %q, want the thinking text unchanged", got)
	}
}
