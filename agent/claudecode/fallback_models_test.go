package claudecode

import "testing"

// The /model chooser falls through to this list whenever no live source answers
// — no [[providers]] in config and no ANTHROPIC_API_KEY for the /v1/models call.
// That is the normal state for anyone signed in through the CLI's own
// subscription auth, so the fallback is what most operators actually see.
//
// The aliases come first on purpose: `opus` / `sonnet` / `haiku` are resolved by
// the CLI itself to the current generation, so they cannot go stale the way a
// pinned id does. The pinned ids follow for operators who want a fixed model.
// Both sets were read out of claude 2.1.278's own binary rather than from memory.
func TestClaudeCodeFallbackModelsAreCurrentGeneration(t *testing.T) {
	models := claudeCodeFallbackModels()
	if len(models) == 0 {
		t.Fatal("claudeCodeFallbackModels() is empty: /model would offer nothing")
	}

	names := make(map[string]bool, len(models))
	for _, m := range models {
		if m.Name == "" {
			t.Errorf("model option has an empty Name: %#v", m)
		}
		if m.Desc == "" {
			t.Errorf("model %q has no Desc, so the chooser shows a bare id", m.Name)
		}
		if names[m.Name] {
			t.Errorf("model %q is listed twice", m.Name)
		}
		names[m.Name] = true
	}

	for _, want := range []string{
		"opus", "sonnet", "haiku",
		"claude-opus-5", "claude-sonnet-5",
	} {
		if !names[want] {
			t.Errorf("fallback is missing current model %q: %v", want, names)
		}
	}
	// Generations the CLI no longer ships. Listing one sends a switch straight
	// into a "model not found" from the agent.
	for _, retired := range []string{
		"claude-3-5-sonnet-20241022",
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
	} {
		if names[retired] {
			t.Errorf("fallback still offers retired model %q", retired)
		}
	}
}
