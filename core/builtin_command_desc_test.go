package core

import (
	"strings"
	"testing"
)

// GetAllCommands looks a built-in command's description up by using the command
// name itself as the MsgKey (see Engine.GetAllCommands). That indirection has no
// compile-time link between the command table and the translation table, so a
// command added without a matching entry silently ships a description that is
// just its own name — "/tts tts" in the Feishu help card and in every bot menu
// the platform registers.
//
// These tests are the missing link: they fail when the two tables drift apart,
// in any of the five supported languages.

func TestBuiltinCommands_HaveLocalizedDescriptions(t *testing.T) {
	langs := []Language{
		LangEnglish, LangChinese, LangTraditionalChinese, LangJapanese, LangSpanish,
	}
	for _, c := range builtinCommands {
		if c.id == "" {
			t.Errorf("builtinCommands entry %v has an empty id", c.names)
			continue
		}
		for _, lang := range langs {
			got := Translate(MsgKey(c.id), lang)
			switch {
			case got == "":
				t.Errorf("/%s [%s]: description is empty", c.id, lang)
			case got == c.id:
				// Translate falls back to the raw key when no entry exists.
				t.Errorf("/%s [%s]: no translation — help card renders the bare "+
					"command name; add a MsgBuiltinCmd entry keyed %q", c.id, lang, c.id)
			}
		}
	}
}

// Descriptions carry their argument list inline ("…, 参数: [a|b]") because the
// help card has no separate column for it. A command that takes arguments and
// does not advertise them is indistinguishable from one that takes none.
func TestBuiltinCommands_ArgTakingCommandsAdvertiseArgs(t *testing.T) {
	// Commands whose handler branches on args[0]. Kept explicit rather than
	// derived: the point is to notice when a handler grows arguments and its
	// description does not follow.
	withArgs := []string{
		"model", "reasoning", "mode", "lang", "provider", "memory", "allow",
		"quiet", "tts", "timer", "heartbeat", "web", "workspace", "cron", "git",
	}
	for _, id := range withArgs {
		desc := Translate(MsgKey(id), LangEnglish)
		if !strings.Contains(desc, "arg") {
			t.Errorf("/%s: description %q does not advertise its arguments", id, desc)
		}
	}
}
