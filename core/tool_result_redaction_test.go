package core

import (
	"strings"
	"testing"
)

// A tool's raw output used to reach the chat verbatim — the engine only
// truncated it. The tool *input* has been redacted all along (feishu's
// sanitizeToolDetail), so a command's arguments were masked while the same
// command's output was not: `env`, `cat .env`, or a verbose curl echoing its
// own Authorization header all shipped whole.
func TestSanitizeToolResultForDisplay_MasksSecretsInToolOutput(t *testing.T) {
	out := sanitizeToolResultForDisplay(
		"API_KEY=sk-live-0123456789abcdef\nAuthorization: Bearer sk-live-fedcba9876543210", 500)

	for _, leaked := range []string{"sk-live-0123456789abcdef", "sk-live-fedcba9876543210"} {
		if strings.Contains(out, leaked) {
			t.Errorf("output = %q, want %q masked", out, leaked)
		}
	}
	if !strings.Contains(out, "[redacted]") {
		t.Errorf("output = %q, want a redaction marker", out)
	}
}

// Redaction and truncation do not commute, and only one order is safe.
// secretAssignRe matches a quoted value as a whole ("[^"]*"), so truncating
// inside the quotes leaves an opening quote with no closing one — which matches
// neither the quoted alternative nor the unquoted one, since the latter
// excludes a leading quote. Truncate first and the surviving prefix of the
// secret ships in the clear.
func TestSanitizeToolResultForDisplay_RedactsBeforeTruncating(t *testing.T) {
	raw := `API_KEY="sk-live-0123456789 rotated"`
	// Cut inside the quoted value, past the start of the secret.
	cut := strings.Index(raw, "0123") + 4

	out := sanitizeToolResultForDisplay(raw, cut)
	if strings.Contains(out, "sk-live-") {
		t.Fatalf("output = %q, want the secret masked even though truncation lands inside its quoted value", out)
	}
}

// Redaction must not become a second truncation rule: output with nothing
// sensitive in it has to come through byte-for-byte, or every tool row starts
// lying about what the command printed.
func TestSanitizeToolResultForDisplay_LeavesOrdinaryOutputAlone(t *testing.T) {
	const raw = "ok 42 tests passed\n  coverage: 91.2%\n| col | col |"
	if out := sanitizeToolResultForDisplay(raw, 500); out != raw {
		t.Errorf("output = %q, want it unchanged", out)
	}
}

// The credential word used to need a prefix in front of it, so MY_API_KEY= was
// masked and the bare API_KEY= was not — the common form was the one that got
// through. Both forms, and the shell's `export` in front of either.
func TestRedactInlineSecrets_MasksBareAndPrefixedKeys(t *testing.T) {
	for _, line := range []string{
		"API_KEY=sk-live-secretvalue",
		"MY_API_KEY=sk-live-secretvalue",
		"TOKEN=sk-live-secretvalue",
		"SECRET=sk-live-secretvalue",
		"export ANTHROPIC_API_KEY=sk-live-secretvalue",
		`CLIENT_SECRET="sk-live-secretvalue with a space"`,
	} {
		out := RedactInlineSecrets(line)
		if strings.Contains(out, "sk-live-secretvalue") {
			t.Errorf("RedactInlineSecrets(%q) = %q, want the value masked", line, out)
		}
	}
}

// The key name is not the secret, and a row that redacts it says nothing about
// which credential leaked.
func TestRedactInlineSecrets_KeepsTheKeyName(t *testing.T) {
	out := RedactInlineSecrets("API_KEY=sk-live-secretvalue")
	if out != "API_KEY=[redacted]" {
		t.Errorf("out = %q, want %q", out, "API_KEY=[redacted]")
	}
}
