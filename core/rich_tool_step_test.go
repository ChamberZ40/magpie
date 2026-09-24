package core

import (
	"testing"
	"time"
)

// The verdict is one glyph on a row, so it costs nothing at summary. The raw
// output is what floods the card, and that alone is what "full" buys.
func TestMergeRichToolResultKeepsTheVerdictAtSummaryButNotTheOutput(t *testing.T) {
	exit := 0
	success := true
	event := Event{
		Type:         EventToolResult,
		ToolName:     "Bash",
		ToolInput:    "npm test",
		ToolStatus:   "completed",
		ToolExitCode: &exit,
		ToolSuccess:  &success,
	}

	s := mergeRichToolResult(nil, event, "all 42 tests passed", 500, ToolDetailSummary)[0]
	if s.Result != "" {
		t.Errorf("Result = %q, want the raw output dropped at summary", s.Result)
	}
	if s.Status != "completed" {
		t.Errorf("Status = %q, want it kept for the row verdict", s.Status)
	}
	if s.Success == nil || !*s.Success {
		t.Errorf("Success = %v, want true so the row can show a verdict", s.Success)
	}
	if s.ExitCode == nil || *s.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", s.ExitCode)
	}
}

// Two calls to the same tool, and the first one's result comes back first.
//
// The old matcher scanned *backwards* for a name match and ignored Done, so the
// first result landed on the second call: call one stayed "running" forever and
// call two was marked finished before it was. Results arrive in call order, so
// the oldest unfinished step is the one to complete.
func TestMergeRichToolResultCompletesTheOldestCallFirst(t *testing.T) {
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "npm test"},
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "git status"},
	}

	got := mergeRichToolResult(steps, Event{ToolName: "Bash", ToolStatus: "completed"}, "", 200, ToolDetailSummary)
	if len(got) != 2 {
		t.Fatalf("steps = %#v, want the two existing steps reused", got)
	}
	if !got[0].Done {
		t.Errorf("first call Done = false, want the oldest unfinished call completed")
	}
	if got[1].Done {
		t.Errorf("second call Done = true, want it still running")
	}
}

// Claude Code's tool_result block carries tool_use_id and no tool name at all,
// so the ID is the only exact correlation available. It must win over position.
func TestMergeRichToolResultMatchesByToolUseID(t *testing.T) {
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "npm test", UseID: "toolu_1"},
		{Kind: ToolStepKindTool, Name: "Read", Summary: "/x.go", UseID: "toolu_2"},
	}

	got := mergeRichToolResult(steps, Event{ToolUseID: "toolu_2"}, "", 200, ToolDetailSummary)
	if len(got) != 2 {
		t.Fatalf("steps = %#v, want no step appended", got)
	}
	if got[0].Done {
		t.Errorf("toolu_1 Done = true, want the ID to pick toolu_2 instead of the oldest")
	}
	if !got[1].Done {
		t.Errorf("toolu_2 Done = false, want the step whose UseID matches completed")
	}
	if got[1].Name != "Read" {
		t.Errorf("Name = %q, want the matched step's own name kept", got[1].Name)
	}
}

// A nameless, ID-less result still has to land somewhere. Falling back to the
// oldest unfinished call is right; inventing a step is not — that is where the
// phantom "Tool ✓" row in the card came from.
func TestMergeRichToolResultNamelessResultCompletesTheOldestCall(t *testing.T) {
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "npm test"},
		{Kind: ToolStepKindTool, Name: "Read", Summary: "/x.go"},
	}

	got := mergeRichToolResult(steps, Event{ToolStatus: "completed"}, "", 200, ToolDetailSummary)
	if len(got) != 2 {
		t.Fatalf("steps = %#v, want no step appended for a nameless result", got)
	}
	for _, s := range got {
		if s.Name == "Tool" {
			t.Fatalf("a step named %q was invented from a nameless result: %#v", s.Name, got)
		}
	}
	if !got[0].Done {
		t.Errorf("first call Done = false, want the nameless result to complete the oldest call")
	}
	if got[0].Name != "Bash" {
		t.Errorf("Name = %q, want the step's own name left alone", got[0].Name)
	}
}

// With nothing left to complete, a nameless result has no home. Dropping it
// loses one status line; inventing a step puts a row in the card for a call
// that never happened, which is strictly worse.
func TestMergeRichToolResultDropsANamelessResultWithNothingToMatch(t *testing.T) {
	steps := []ToolStep{{Kind: ToolStepKindTool, Name: "Bash", Summary: "npm test", Done: true}}

	got := mergeRichToolResult(steps, Event{ToolStatus: "completed"}, "", 200, ToolDetailSummary)
	if len(got) != 1 {
		t.Fatalf("steps = %#v, want the unmatched nameless result dropped", got)
	}
}

// A named result with no tool-use event behind it is a different story: the
// name is enough to render an honest row, so the step is still created. Codex
// and ACP both send named results, and this is the path they rely on.
func TestMergeRichToolResultAppendsWhenTheResultIsNamed(t *testing.T) {
	got := mergeRichToolResult(nil, Event{ToolName: "Bash", ToolInput: "ls", ToolStatus: "completed"}, "", 200, ToolDetailSummary)
	if len(got) != 1 {
		t.Fatalf("steps = %#v, want one appended step", got)
	}
	if got[0].Name != "Bash" || got[0].Summary != "ls" {
		t.Errorf("step = %#v, want the event's own name and input", got[0])
	}
	if !got[0].Done {
		t.Errorf("Done = false, want an appended step to be complete")
	}
}

// A finished call must not be reopened by a later result. Without the Done
// check the matcher walks back over completed steps and overwrites them.
func TestMergeRichToolResultSkipsCallsThatAlreadyFinished(t *testing.T) {
	exit := 1
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "npm test", Status: "completed", Done: true},
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "./deploy.sh"},
	}

	got := mergeRichToolResult(steps, Event{ToolName: "Bash", ToolStatus: "failed", ToolExitCode: &exit}, "", 200, ToolDetailSummary)
	if got[0].Status != "completed" {
		t.Errorf("first call Status = %q, want the finished call left untouched", got[0].Status)
	}
	if got[1].Status != "failed" {
		t.Errorf("second call Status = %q, want the failure recorded on the unfinished call", got[1].Status)
	}
}

// A row says how long its call took. The clock starts when the call is seen
// and stops when its result lands, so a call with no start has no duration
// rather than an invented one.
func TestMergeRichToolResultRecordsHowLongTheCallTook(t *testing.T) {
	steps := []ToolStep{{Kind: ToolStepKindTool, Name: "Read", UseID: "u1", StartedAt: time.Now().Add(-2 * time.Second)}}

	got := mergeRichToolResult(steps, Event{ToolName: "Read", ToolUseID: "u1"}, "", 200, ToolDetailSummary)[0]
	if got.Duration < 2*time.Second || got.Duration > time.Minute {
		t.Errorf("Duration = %v, want about 2s", got.Duration)
	}

	orphan := mergeRichToolResult(nil, Event{ToolName: "Read"}, "", 200, ToolDetailSummary)[0]
	if orphan.Duration != 0 {
		t.Errorf("Duration = %v for a call never seen starting, want 0", orphan.Duration)
	}
}

// Without a call id, a result whose name matches no pending call still belongs
// to the oldest call in flight. Inventing a new row for it left the real call
// rendering "running" for the rest of the turn.
func TestMergeRichToolResultFallsBackToTheOldestCallOfAnyName(t *testing.T) {
	steps := []ToolStep{{Kind: ToolStepKindTool, Name: "shell"}}

	got := mergeRichToolResult(steps, Event{ToolName: "Bash", ToolStatus: "completed"}, "", 200, ToolDetailSummary)
	if len(got) != 1 {
		t.Fatalf("steps = %d, want the result paired with the pending call, not a new row", len(got))
	}
	if !got[0].Done {
		t.Error("pending call still not done — it would render running forever")
	}
}

// The any-name fallback never hands a result to a call whose own id says it is
// a different call.
func TestMergeRichToolResultFallbackSkipsACallWithAnotherID(t *testing.T) {
	steps := []ToolStep{{Kind: ToolStepKindTool, Name: "shell", UseID: "a"}}

	got := mergeRichToolResult(steps, Event{ToolName: "Bash", ToolUseID: "b"}, "", 200, ToolDetailSummary)
	if got[0].Done {
		t.Error("call a was completed by the result of call b")
	}
}
