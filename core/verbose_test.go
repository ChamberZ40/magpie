package core

import (
	"strings"
	"testing"
)

// /verbose is deliberately not a toggle. Cycling through three levels blind —
// which is what /quiet does with its three modes — means you have to fire the
// command repeatedly and read the reply to find out where you landed. A bare
// /verbose therefore only reports.

func newVerboseEngine(t *testing.T) (*Engine, *stubPlatformEngine, *Message) {
	t.Helper()
	p := &stubPlatformEngine{n: "test"}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(DisplayCfg{Mode: "full", ThinkingMessages: true, ToolDetail: ToolDetailSummary})
	return e, p, &Message{SessionKey: "test:user1", ReplyCtx: "ctx"}
}

func TestCmdVerbose_BareReportsWithoutChanging(t *testing.T) {
	e, p, msg := newVerboseEngine(t)
	saved := 0
	e.SetDisplaySaveFunc(func(*string, *bool, *int, *int, *string) error {
		saved++
		return nil
	})

	e.cmdVerbose(p, msg, nil)

	if e.display.ToolDetail != ToolDetailSummary {
		t.Errorf("ToolDetail = %q, want it untouched at %q", e.display.ToolDetail, ToolDetailSummary)
	}
	if saved != 0 {
		t.Errorf("displaySaveFunc called %d times, want 0 — reporting must not write config", saved)
	}
	if len(p.sent) != 1 || !strings.Contains(p.sent[0], ToolDetailSummary) {
		t.Fatalf("sent = %q, want the current level reported", p.sent)
	}
}

func TestCmdVerbose_SetsAndPersists(t *testing.T) {
	e, p, msg := newVerboseEngine(t)
	var got *string
	e.SetDisplaySaveFunc(func(_ *string, _ *bool, _, _ *int, toolDetail *string) error {
		got = toolDetail
		return nil
	})

	e.cmdVerbose(p, msg, []string{"FULL"}) // case is tolerated, as elsewhere

	if e.display.ToolDetail != ToolDetailFull {
		t.Errorf("ToolDetail = %q, want %q", e.display.ToolDetail, ToolDetailFull)
	}
	if got == nil || *got != ToolDetailFull {
		t.Fatalf("persisted %v, want %q", got, ToolDetailFull)
	}
}

func TestCmdVerbose_RejectsUnknownLevel(t *testing.T) {
	e, p, msg := newVerboseEngine(t)

	e.cmdVerbose(p, msg, []string{"loud"})

	if e.display.ToolDetail != ToolDetailSummary {
		t.Errorf("ToolDetail = %q, want the bad argument to change nothing", e.display.ToolDetail)
	}
	if len(p.sent) != 1 || !strings.Contains(p.sent[0], "/verbose") {
		t.Fatalf("sent = %q, want usage", p.sent)
	}
}

// SaveDisplayConfig always writes the global [display] section, but a project's
// own tool_detail outranks it on reload. Without this warning the setting looks
// like it silently reverted.
func TestCmdVerbose_WarnsWhenProjectPinsTheLevel(t *testing.T) {
	e, p, msg := newVerboseEngine(t)
	e.display.ToolDetailFromProject = true

	e.cmdVerbose(p, msg, []string{ToolDetailNone})

	if len(p.sent) != 1 {
		t.Fatalf("sent = %q, want one reply", p.sent)
	}
	if !strings.Contains(p.sent[0], "tool_detail") {
		t.Errorf("reply = %q, want it to name the project setting that will win", p.sent[0])
	}
}
