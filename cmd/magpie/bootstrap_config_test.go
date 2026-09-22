package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/core"
)

// The generated config is the first thing a new user reads — for anyone who
// installed from npm it is step two of the quick start. It used to offer
// "gemini", "qoder", "opencode" and "iflow" as agent types and point at
// DingTalk, Telegram, Slack, Discord and LINE as platforms, none of which this
// project ships. Deriving both lines from the registry makes that drift
// impossible and makes a selective build (make build AGENTS=codex) describe
// itself honestly.

func TestDefaultConfigTemplate_NamesOnlyRegisteredAgents(t *testing.T) {
	tmpl := defaultConfigTemplate()

	registered := make(map[string]bool)
	for _, n := range core.ListRegisteredAgents() {
		registered[n] = true
	}
	for _, name := range []string{"gemini", "qoder", "opencode", "iflow"} {
		if registered[name] {
			continue // this build really does ship it; nothing to assert
		}
		if strings.Contains(tmpl, `"`+name+`"`) {
			t.Errorf("template offers agent type %q, which this build does not register", name)
		}
	}
	for name := range registered {
		if !strings.Contains(tmpl, `"`+name+`"`) {
			t.Errorf("template never mentions registered agent %q", name)
		}
	}
}

func TestDefaultConfigTemplate_NamesOnlyRegisteredPlatforms(t *testing.T) {
	tmpl := defaultConfigTemplate()

	for _, name := range []string{"DingTalk", "Telegram", "Slack", "Discord", "LINE"} {
		if strings.Contains(tmpl, name) {
			t.Errorf("template points at platform %q, which this project does not support", name)
		}
	}
}

// The example blocks carry real option keys (app_id, app_secret), so they only
// make sense if the build actually has that agent and platform compiled in.
func TestDefaultConfigTemplate_ExampleTypesAreRegistered(t *testing.T) {
	tmpl := defaultConfigTemplate()

	for _, tc := range []struct {
		kind       string
		registered []string
	}{
		{"agent", core.ListRegisteredAgents()},
		{"platform", core.ListRegisteredPlatforms()},
	} {
		want := exampleTypeFor(t, tmpl, tc.kind)
		if !contains(tc.registered, want) {
			t.Errorf("template's example %s type is %q, which is not registered in this build (%v)",
				tc.kind, want, tc.registered)
		}
	}
}

// A hint that names a command the binary does not have sends a new user
// straight into "unknown top-level command". `magpie init` was exactly that.
func TestConfigLoadErrorHint_NamesOnlyRealCommands(t *testing.T) {
	hint := configLoadErrorHint("/tmp/config.toml", errors.New("at least one [[projects]] entry is required"))

	for _, line := range strings.Split(hint, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "magpie ")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, " ")
		if _, ok := topLevelCommandHandlers[name]; !ok {
			t.Errorf("hint tells the user to run %q, which is not a command", "magpie "+name)
		}
	}
	if !strings.Contains(hint, "/tmp/config.toml") {
		t.Error("hint should name the config file the user has to edit")
	}
	// The underlying error says which rule was broken; dropping it would leave
	// the user with "fix that file" and no idea what is wrong with it.
	if !strings.Contains(hint, "at least one [[projects]] entry is required") {
		t.Error("hint should carry the underlying error")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// exampleTypeFor pulls the value of the first `type = "..."` line that follows
// the [projects.<kind>] header.
func exampleTypeFor(t *testing.T, tmpl, kind string) string {
	t.Helper()
	headers := map[string]string{
		"agent":    "[projects.agent]",
		"platform": "[[projects.platforms]]",
	}
	_, after, ok := strings.Cut(tmpl, headers[kind])
	if !ok {
		t.Fatalf("template has no %s section", kind)
	}
	_, after, ok = strings.Cut(after, `type = "`)
	if !ok {
		t.Fatalf("%s section has no type line", kind)
	}
	value, _, _ := strings.Cut(after, `"`)
	return value
}
