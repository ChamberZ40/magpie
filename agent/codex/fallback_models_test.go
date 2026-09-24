package codex

import (
	"strings"
	"testing"
)

// Codex's /model chooser falls through to this list whenever no live source
// answers — no [[providers]] in config, no model_catalog.json / models_cache.json
// under CODEX_HOME, and no OPENAI_API_KEY for the /v1/models call. That is the
// normal state for anyone signed in through the CLI's own subscription auth, so
// the fallback is what most operators actually see. It was still listing o3 and
// the gpt-4.1 family long after Codex CLI stopped shipping them.
//
// The wanted ids were read out of codex-cli 0.155.1's own binary rather than
// from memory. The retired ones are the exact entries this list used to hold.
func TestCodexFallbackModelsAreCurrentGeneration(t *testing.T) {
	models := codexFallbackModels()
	if len(models) == 0 {
		t.Fatal("codexFallbackModels() is empty: /model would offer nothing")
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

	for _, want := range []string{"gpt-5.6-sol", "gpt-5.6-luna", "gpt-5.6-terra"} {
		if !names[want] {
			t.Errorf("fallback is missing current model %q: %v", want, names)
		}
	}
	for _, retired := range []string{"o4-mini", "o3", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "codex-mini-latest"} {
		if names[retired] {
			t.Errorf("fallback still offers retired model %q", retired)
		}
	}

	// The list and the filter that guards the /v1/models path have to agree: an
	// entry isCodexChatModel rejects would be offered by the chooser and then
	// dropped by the very next code path that sees it.
	for _, m := range models {
		if !isCodexChatModel(m.Name) {
			t.Errorf("fallback offers %q, which isCodexChatModel rejects", m.Name)
		}
	}
}

// codexModelLooksForeign is the startup heads-up for a model from the wrong
// vendor — the exact misconfiguration switch-agent.sh used to leave behind when
// it changed the agent type and left `model` pointing at the other CLI's model.
func TestCodexModelLooksForeign(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  bool
	}{
		{"gpt-5.6-sol", false},
		{"gpt-5.3-codex", false},
		{"o3", false},
		{"codex-mini-latest", false},
		{"claude-opus-5", true},
		{"gemini-2.5-pro", true},
		// Nothing configured is not a misconfiguration: codex picks its own
		// default, so warning here would fire for every default install.
		{"", false},
		{"   ", false},
	} {
		if got := codexModelLooksForeign(tc.model); got != tc.want {
			t.Errorf("codexModelLooksForeign(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

// A warning whose text does not name the escape hatch is a warning people
// cannot act on: New() cannot tell a wrong-vendor model from a legitimate
// custom [model_providers.*] entry, because providers arrive later via
// SetProviders. The message has to say so.
func TestCodexForeignModelWarningNamesTheCustomProviderEscapeHatch(t *testing.T) {
	if !strings.Contains(codexForeignModelWarning, "model_providers") {
		t.Errorf("warning does not mention custom providers, so a legitimate setup reads it as an error: %q", codexForeignModelWarning)
	}
}
