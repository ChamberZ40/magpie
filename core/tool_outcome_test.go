package core

import "testing"

// The three signals an agent can report about a finished call disagree often
// enough that precedence matters: the explicit flag first, then a status word
// the table recognizes, then the exit code.
//
// What is left over is Unknown, and Unknown is deliberately not Succeeded.
// Rounding an outcome nobody recognized up to success is the one answer that
// actively misleads — and it is what decides whether the call's output is worth
// keeping, so guessing wrong there throws away the explanation.
func TestClassifyToolResult(t *testing.T) {
	yes, no := true, false
	zero, two := 0, 2

	for _, tc := range []struct {
		name     string
		status   string
		exitCode *int
		success  *bool
		want     ToolOutcome
	}{
		{"the success flag outranks a scary exit code", "", &two, &yes, ToolOutcomeSucceeded},
		{"the failure flag outranks a clean exit code", "", &zero, &no, ToolOutcomeFailed},
		{"a failure word, no flag", "error", nil, nil, ToolOutcomeFailed},
		{"a failure word outranks a clean exit code", "failed", &zero, nil, ToolOutcomeFailed},
		{"a clean exit code, no flag or word", "", &zero, nil, ToolOutcomeSucceeded},
		{"a non-zero exit code, no flag or word", "", &two, nil, ToolOutcomeFailed},
		{"a recognized ok word", "completed", nil, nil, ToolOutcomeSucceeded},
		{"a call that reported nothing yet", "", nil, nil, ToolOutcomeSucceeded},
		{"a word the table does not know", "cancelled", nil, nil, ToolOutcomeUnknown},
		{"case and padding do not change the verdict", "  FAILED  ", nil, nil, ToolOutcomeFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyToolResult(tc.status, tc.exitCode, tc.success); got != tc.want {
				t.Errorf("ClassifyToolResult(%q, %v, %v) = %v, want %v",
					tc.status, tc.exitCode, tc.success, got, tc.want)
			}
		})
	}
}
