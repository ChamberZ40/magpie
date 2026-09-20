package core

import (
	"strings"
	"testing"
)

// The help card used to render a hardcoded list of built-in commands only, so
// commands declared in config.toml's [[commands]] — or dropped into an agent
// command directory — were invisible there. Users had to already know about
// /commands to find them. They now sit at the bottom of the System tab.

func newHelpEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
}

func TestHelpCard_TabCountUnchangedByCustomCommands(t *testing.T) {
	e := newHelpEngine(t)
	if got := countCardActionValues(e.renderHelpCard(), "nav:/help "); got != 4 {
		t.Fatalf("help tab count = %d, want 4", got)
	}

	e.commands.Add("codex", "Switch to the Codex agent", "", "switch-agent.sh codex", "", "config")
	if got := countCardActionValues(e.renderHelpCard(), "nav:/help "); got != 4 {
		t.Errorf("help tab count = %d after adding a custom command, want 4 — "+
			"custom commands belong in the System tab, not a tab of their own", got)
	}
}

func TestHelpCard_SystemTabListsCustomCommandsAfterBuiltins(t *testing.T) {
	e := newHelpEngine(t)
	e.commands.Add("codex", "Switch to the Codex agent", "", "switch-agent.sh codex", "", "config")
	e.commands.Add("claude", "Switch back to Claude Code", "", "switch-agent.sh claude", "", "config")

	card := e.renderHelpGroupCard("system")
	text := card.RenderText()

	for _, want := range []string{"**/codex**", "Switch to the Codex agent", "**/claude**", "Switch back to Claude Code"} {
		if !strings.Contains(text, want) {
			t.Errorf("System tab missing %q\n--- card ---\n%s", want, text)
		}
	}
	// Built-ins keep their place; custom rows are appended below them.
	if got, want := strings.Index(text, "**/status**"), strings.Index(text, "**/claude**"); got > want {
		t.Errorf("custom commands render above the built-ins (status at %d, claude at %d)", got, want)
	}
	// Tapping the row must run the command, like typing it would.
	if _, ok := findCardAction(card, "cmd:/codex"); !ok {
		t.Error("custom command row has no cmd: action")
	}
	// They must not leak into the other tabs.
	if strings.Contains(e.renderHelpGroupCard("session").RenderText(), "**/codex**") {
		t.Error("custom command leaked into the Session tab")
	}
}

// A custom command's description is looked up from the command itself, never
// through the i18n table — otherwise it would hit the same fallback that made
// built-ins render as their bare name.
func TestHelpCard_CustomCommandWithoutDescriptionFallsBackToItsBody(t *testing.T) {
	e := newHelpEngine(t)
	e.commands.Add("deploy", "", "", "make deploy && echo done", "", "config")
	e.commands.Add("review", "", "Please review {{args}} carefully", "", "", "config")

	text := e.renderHelpGroupCard("system").RenderText()

	if !strings.Contains(text, "$ make deploy") {
		t.Errorf("exec command without description should show its command line\n%s", text)
	}
	if !strings.Contains(text, "Please review") {
		t.Errorf("prompt command without description should show its prompt\n%s", text)
	}
	if strings.Contains(text, "**/deploy**  deploy") {
		t.Error("description fell back to the bare command name")
	}
}

// ListAll iterates a map, so without an explicit sort the rows would shuffle
// between renders of the same card.
func TestHelpCard_CustomCommandsAreOrderedStably(t *testing.T) {
	e := newHelpEngine(t)
	for _, n := range []string{"zeta", "alpha", "mike", "bravo"} {
		e.commands.Add(n, "desc "+n, "p", "", "", "config")
	}

	first := e.renderHelpGroupCard("system").RenderText()
	for i := 0; i < 5; i++ {
		if got := e.renderHelpGroupCard("system").RenderText(); got != first {
			t.Fatalf("System tab is unstable across renders (attempt %d)", i+1)
		}
	}

	idx := func(name string) int { return strings.Index(first, "**/"+name+"**") }
	if !(idx("alpha") < idx("bravo") && idx("bravo") < idx("mike") && idx("mike") < idx("zeta")) {
		t.Errorf("custom commands not sorted by name\n%s", first)
	}
}

// The card renders one tappable row per command, so a user with dozens of
// commands would bury the built-ins. Overflow defers to /commands.
func TestHelpCard_CustomCommandsCapRowsAndPointAtCommands(t *testing.T) {
	e := newHelpEngine(t)
	for i := 0; i < helpCardCustomMax+7; i++ {
		e.commands.Add(string(rune('a'+i/26))+string(rune('a'+i%26)), "d", "p", "", "", "config")
	}

	rows := e.helpCardCustomItems()
	runnable := 0
	for _, item := range rows {
		if strings.HasPrefix(item.action, "cmd:/") {
			runnable++
		}
	}
	if runnable > helpCardCustomMax {
		t.Errorf("custom rows = %d, want at most %d", runnable, helpCardCustomMax)
	}
	if _, ok := findCardAction(e.renderHelpGroupCard("system"), "nav:/commands"); !ok {
		t.Error("overflowing System tab should offer a way to see the rest")
	}
}
