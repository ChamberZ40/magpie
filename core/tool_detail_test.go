package core

import "testing"

// tool_detail's middle rung exists because the rich card used to print this for
// every single call:
//
//	Bash
//	npm test
//	status: ok | exit: 0
//	<the entire command output>
//
// The renderer has no knob of its own — it prints whatever fields the step
// carries — so "summary" is expressed by not filling those fields in.

func TestMergeRichToolResultDetailLevels(t *testing.T) {
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

	t.Run("full carries the verbose fields", func(t *testing.T) {
		steps := mergeRichToolResult(nil, event, "all 42 tests passed", 500, ToolDetailFull)
		if len(steps) != 1 {
			t.Fatalf("steps = %#v, want exactly one", steps)
		}
		s := steps[0]
		if s.Result != "all 42 tests passed" {
			t.Errorf("Result = %q, want the raw output", s.Result)
		}
		if s.Status != "completed" {
			t.Errorf("Status = %q, want %q", s.Status, "completed")
		}
		if s.ExitCode == nil || *s.ExitCode != 0 {
			t.Errorf("ExitCode = %v, want 0", s.ExitCode)
		}
		if s.Success == nil || !*s.Success {
			t.Errorf("Success = %v, want true", s.Success)
		}
	})

	// The raw output is the flood; the verdict fields are one glyph in the
	// panel header, so they ride along at every level. See
	// TestMergeRichToolResultKeepsTheVerdictAtSummaryButNotTheOutput.
	t.Run("summary drops the raw output but still completes the step", func(t *testing.T) {
		steps := mergeRichToolResult(nil, event, "all 42 tests passed", 500, ToolDetailSummary)
		if len(steps) != 1 {
			t.Fatalf("steps = %#v, want exactly one", steps)
		}
		s := steps[0]
		if s.Result != "" {
			t.Errorf("Result = %q, want it dropped", s.Result)
		}
		// The identifying half must survive, or the panel row says nothing.
		if s.Name != "Bash" || s.Summary != "npm test" {
			t.Errorf("got (%q, %q), want (Bash, npm test)", s.Name, s.Summary)
		}
		// Done is what stops the row from looking stuck mid-run; skipping the
		// merge entirely would have lost it.
		if !s.Done {
			t.Error("Done = false — the step would render as still running")
		}
	})

	t.Run("an existing step keeps its summary and is completed in place", func(t *testing.T) {
		existing := []ToolStep{{Kind: ToolStepKindTool, Name: "Bash", Summary: "npm test"}}
		steps := mergeRichToolResult(existing, event, "output", 500, ToolDetailSummary)
		if len(steps) != 1 {
			t.Fatalf("steps = %#v, want the existing step reused, not a second one", steps)
		}
		if !steps[0].Done || steps[0].Result != "" {
			t.Errorf("step = %#v, want done with no result", steps[0])
		}
	})
}
