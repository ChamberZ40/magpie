package core

import (
	"strings"
	"testing"
)

// A CLI too old for the configured model says so in its own words. The reply
// has to tell the operator what to do about it, and keep the CLI's own text,
// which names the exact version and update command — core cannot know either.
func TestAgentErrorMessage_AsksForAnUpgradeWhenTheCLIIsTooOld(t *testing.T) {
	errMsg := "Claude Code 2.1.278 does not support this model; version 2.1.280 or newer is required. Run 'claude update' to upgrade."

	got := agentErrorMessage(NewI18n(LangEnglish), errMsg)
	for _, want := range []string{"upgrade", "/new", errMsg} {
		if !strings.Contains(got, want) {
			t.Errorf("message = %q, want it to contain %q", got, want)
		}
	}
}

func TestAgentErrorMessage_FallsBackToTheGenericError(t *testing.T) {
	got := agentErrorMessage(NewI18n(LangEnglish), "API Error: 529 overloaded")
	if got != "❌ Error: API Error: 529 overloaded" {
		t.Errorf("message = %q, want the generic error line", got)
	}
}

func TestAgentErrorMessage_SessionNotFoundKeepsItsOwnMessage(t *testing.T) {
	got := agentErrorMessage(NewI18n(LangEnglish), "Session not found: abc")
	if got != NewI18n(LangEnglish).T(MsgSessionNotFound) {
		t.Errorf("message = %q, want the session-expired message", got)
	}
}
