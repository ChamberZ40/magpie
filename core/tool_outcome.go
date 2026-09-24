package core

import (
	"slices"
	"strings"
)

// ToolOutcome is how a finished tool call turned out. Three values, not two:
// an agent can report a status nobody here recognizes, and the honest answer
// to that is "unknown" rather than a guess in either direction.
type ToolOutcome int

const (
	// ToolOutcomeSucceeded also covers a call that reported nothing at all,
	// which is what a still-running step looks like.
	ToolOutcomeSucceeded ToolOutcome = iota
	ToolOutcomeFailed
	ToolOutcomeUnknown
)

var (
	toolStatusFailed = []string{"failed", "failure", "error", "denied", "rejected"}
	toolStatusOK     = []string{"completed", "complete", "success", "succeeded", "ok", "done", "finished"}
)

// ClassifyToolResult decides how a call turned out from the three signals an
// agent may report, most explicit first: the success flag, then a status word
// this package recognizes, then the exit code.
//
// It lives in core because two different decisions depend on it and they must
// not drift: whether to keep the call's output (here) and what mark to draw on
// its row (the platform). It used to exist only inside the Feishu renderer,
// so core could not consult it and the other platforms had no verdict at all.
func ClassifyToolResult(status string, exitCode *int, success *bool) ToolOutcome {
	normalized := strings.ToLower(strings.TrimSpace(status))

	switch {
	case success != nil:
		if *success {
			return ToolOutcomeSucceeded
		}
		return ToolOutcomeFailed
	case slices.Contains(toolStatusFailed, normalized):
		return ToolOutcomeFailed
	case exitCode != nil:
		if *exitCode == 0 {
			return ToolOutcomeSucceeded
		}
		return ToolOutcomeFailed
	case normalized == "" || slices.Contains(toolStatusOK, normalized):
		return ToolOutcomeSucceeded
	}
	return ToolOutcomeUnknown
}
