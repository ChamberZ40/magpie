package core

import "testing"

// Every builtin command's description is looked up as MsgKey(c.id) — the help
// card does it at engine.go:9641, the bridge capabilities snapshot does it at
// bridge_capabilities.go:74. Translate falls back to returning the key itself
// when there is no entry, so a command added without an i18n line does not
// fail anywhere: the help card and every bridge client just quietly show the
// bare id where a description belongs.
func TestEveryBuiltinCommandHasADescription(t *testing.T) {
	for _, c := range builtinCommands {
		if len(c.names) == 0 {
			continue
		}
		for _, lang := range []Language{LangEnglish, LangChinese} {
			got := Translate(MsgKey(c.id), lang)
			if got == "" || got == c.id {
				t.Errorf("builtin %q has no %s description; MsgKey(%q) falls back to the bare id",
					c.id, lang, c.id)
			}
		}
	}
}
