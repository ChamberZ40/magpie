package core

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubRichFooterPlatform records the statusFooter every finished rich card is
// built with, so a test can assert the footer's shape rather than its wording.
type stubRichFooterPlatform struct {
	*stubRichCardSilentPlatform
	footerMu    sync.Mutex
	doneFooters []string
}

func (p *stubRichFooterPlatform) BuildRichCard(status CardStatus, _ string, _ []ToolStep, markdown string, _ bool, statusFooter string) string {
	if status == CardStatusDone {
		p.footerMu.Lock()
		p.doneFooters = append(p.doneFooters, statusFooter)
		p.footerMu.Unlock()
	}
	return "rich:" + markdown + "\n" + statusFooter
}

func (p *stubRichFooterPlatform) lastDoneFooter(t *testing.T) string {
	t.Helper()
	p.footerMu.Lock()
	defer p.footerMu.Unlock()
	if len(p.doneFooters) == 0 {
		t.Fatalf("no Done rich card was built")
	}
	return p.doneFooters[len(p.doneFooters)-1]
}

// runRichFooterTurn drives one complete turn through a rich-card platform and
// returns the statusFooter the Done card carried.
func runRichFooterTurn(t *testing.T, workDir string) string {
	t.Helper()
	p := &stubRichFooterPlatform{
		stubRichCardSilentPlatform: &stubRichCardSilentPlatform{
			stubPlatformEngine: stubPlatformEngine{n: "feishu"},
		},
	}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(DisplayCfg{Mode: "full", CardMode: "rich"})
	e.SetReplyFooterEnabled(true)

	sessionKey := "feishu:user-rich-footer"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s-rich-footer")
	agentSession.model = "gpt-5.6-sol"
	agentSession.reasoningEffort = "medium"
	agentSession.workDir = workDir
	agentSession.contextUsage = &ContextUsage{ContextWindow: 100_000, UsedTokens: 37_000}
	state := &interactiveState{
		agentSession: agentSession,
		platform:     p,
		replyCtx:     "ctx-rich-footer",
		workspaceDir: workDir,
	}
	e.interactiveStates[sessionKey] = state

	agentSession.events <- Event{Type: EventText, Content: "done"}
	agentSession.events <- Event{Type: EventResult, Content: "done", Done: true}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, "m-rich-footer", time.Now(), nil, nil, state.replyCtx)

	return p.lastDoneFooter(t)
}

// The footer has two lines because they answer different questions: line 1 is
// the turn (elapsed, model, effort, context budget), line 2 is the place it ran
// (workdir, branch). The finished card used to discard that layout and re-emit
// the legacy one-line footer under a bare elapsed line, which pushed the
// context bar off the visible width behind the longest segment — the path.
func TestRichCardDoneFooter_SplitsTheTurnFromThePlace(t *testing.T) {
	dir := t.TempDir()
	footer := runRichFooterTurn(t, dir)

	lines := strings.Split(footer, "\n")
	if len(lines) != 2 {
		t.Fatalf("footer = %q, want exactly 2 lines (turn, place)", footer)
	}
	turn, place := lines[0], lines[1]
	base := filepath.Base(dir)

	for _, want := range []string{"gpt-5.6-sol", "medium", "37%"} {
		if !strings.Contains(turn, want) {
			t.Errorf("turn line = %q, want it to carry %q", turn, want)
		}
	}
	if !strings.Contains(place, base) {
		t.Errorf("place line = %q, want it to carry the workdir %q", place, base)
	}
	if strings.Contains(turn, base) {
		t.Errorf("turn line = %q must not carry the workdir — the path is the longest segment and crowds out the context bar", turn)
	}
}
