package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// historyStubAgent lists sessions and can also serve the transcript of one it
// did not create in this process — the Codex-app / earlier-run case, which is
// exactly when a /switch confirmation carrying nothing but an id is useless.
type historyStubAgent struct {
	stubListAgent
	history    []HistoryEntry
	err        error
	askedID    string
	askedLimit int
}

func (a *historyStubAgent) GetSessionHistory(_ context.Context, sessionID string, limit int) ([]HistoryEntry, error) {
	a.askedID = sessionID
	a.askedLimit = limit
	if a.err != nil {
		return nil, a.err
	}
	if limit > 0 && len(a.history) > limit {
		return a.history[len(a.history)-limit:], nil
	}
	return a.history, nil
}

func transcript(base time.Time, texts ...string) []HistoryEntry {
	entries := make([]HistoryEntry, 0, len(texts))
	for i, text := range texts {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		entries = append(entries, HistoryEntry{
			Role:      role,
			Content:   text,
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return entries
}

// The tail, not the head: the session picker already shows the opening prompt as
// each row's title, so repeating it teaches nothing. What a resumed session
// needs is where the conversation got to.
func TestCmdSwitch_PreviewsTheTailOfTheSessionSwitchedInto(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	agent := &historyStubAgent{
		stubListAgent: stubListAgent{sessions: []AgentSessionInfo{
			{ID: "sess-1", Summary: "fix the login bug", MessageCount: 14, ModifiedAt: base},
		}},
		history: transcript(base, "alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"),
	}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	msg := &Message{SessionKey: "test:user1", ReplyCtx: "ctx"}

	e.cmdSwitch(p, msg, []string{"1"})

	sent := p.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 reply, got %d: %v", len(sent), sent)
	}
	reply := sent[0]

	if !strings.Contains(reply, "fix the login bug") {
		t.Errorf("reply dropped the switch confirmation:\n%s", reply)
	}
	for _, want := range []string{"charlie", "delta", "echo", "foxtrot", "golf"} {
		if !strings.Contains(reply, want) {
			t.Errorf("preview is missing %q:\n%s", want, reply)
		}
	}
	for _, unwanted := range []string{"alpha", "bravo"} {
		if strings.Contains(reply, unwanted) {
			t.Errorf("preview should stop at the last %d messages but included %q:\n%s",
				sessionPreviewEntries, unwanted, reply)
		}
	}

	if agent.askedID != "sess-1" {
		t.Errorf("preview read transcript %q, want the session being switched into", agent.askedID)
	}
	if agent.askedLimit != sessionPreviewEntries {
		t.Errorf("preview asked for %d entries, want %d", agent.askedLimit, sessionPreviewEntries)
	}
}

// An agent that cannot serve transcripts at all must still get a clean
// confirmation — no empty header, no second reply.
func TestCmdSwitch_WithoutTranscriptSupportStillConfirms(t *testing.T) {
	agent := &stubListAgent{sessions: []AgentSessionInfo{
		{ID: "sess-1", Summary: "no transcript here", MessageCount: 3, ModifiedAt: time.Now()},
	}}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	msg := &Message{SessionKey: "test:user1", ReplyCtx: "ctx"}

	e.cmdSwitch(p, msg, []string{"1"})

	sent := p.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 reply, got %d: %v", len(sent), sent)
	}
	want := e.i18n.Tf(MsgSwitchSuccess, "no transcript here", "sess-1", 3)
	if sent[0] != want {
		t.Errorf("reply = %q, want exactly the confirmation %q", sent[0], want)
	}
}

// The preview is a convenience. A transcript that cannot be read is worth a log
// line, never a failed switch — the session has already been switched at that
// point, so reporting failure would be a lie.
func TestCmdSwitch_TranscriptErrorDoesNotBlockTheSwitch(t *testing.T) {
	agent := &historyStubAgent{
		stubListAgent: stubListAgent{sessions: []AgentSessionInfo{
			{ID: "sess-1", Summary: "unreadable transcript", MessageCount: 9, ModifiedAt: time.Now()},
		}},
		err: errors.New("transcript file vanished"),
	}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	msg := &Message{SessionKey: "test:user1", ReplyCtx: "ctx"}

	e.cmdSwitch(p, msg, []string{"1"})

	sent := p.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 reply, got %d: %v", len(sent), sent)
	}
	want := e.i18n.Tf(MsgSwitchSuccess, "unreadable transcript", "sess-1", 9)
	if sent[0] != want {
		t.Errorf("reply = %q, want exactly the confirmation %q", sent[0], want)
	}
	if got := e.sessions.GetOrCreateActive("test:user1").GetAgentSessionID(); got != "sess-1" {
		t.Errorf("active agent session = %q, want sess-1 — the switch itself must survive", got)
	}
}

// Switching back to a session this process already drove must not go to disk:
// the in-memory history is the same conversation and is always at least as
// fresh as the transcript the agent flushed.
func TestSessionPreview_PrefersInMemoryHistory(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	agent := &historyStubAgent{history: transcript(base, "from-disk")}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)

	s := e.sessions.GetOrCreateActive("test:user1")
	s.SetAgentSessionID("sess-1", "codex")
	s.AddHistory("user", "from-memory")

	preview := e.sessionPreview(agent, s)
	if !strings.Contains(preview, "from-memory") {
		t.Errorf("preview ignored the in-memory history:\n%s", preview)
	}
	if agent.askedID != "" {
		t.Errorf("preview hit the agent transcript for a session already in memory (asked %q)", agent.askedID)
	}
}

// cardMarkdown concatenates every markdown block of a card, which is where the
// switch confirmation and its preview live.
func cardMarkdown(card *Card) string {
	var sb strings.Builder
	for _, elem := range card.Elements {
		if md, ok := elem.(CardMarkdown); ok {
			sb.WriteString(md.Content)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// Picking a session from the /list card is how it is actually done — nobody
// types "/switch 4". That path did not go through cmdSwitch at all: it ran a
// second copy of the switch logic and then simply redrew the list, so the
// preview added to the typed command was invisible where it mattered.
func TestSwitchCardAction_ShowsTheConfirmationAndPreview(t *testing.T) {
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	agent := &historyStubAgent{
		stubListAgent: stubListAgent{sessions: []AgentSessionInfo{
			{ID: "sess-1", Summary: "fix the login bug", MessageCount: 14, ModifiedAt: base},
		}},
		history: transcript(base, "alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"),
	}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	sessionKey := "test:user1"

	card := e.handleCardNav("act:/switch 1", sessionKey)
	if card == nil {
		t.Fatal("card action returned no card")
	}
	body := cardMarkdown(card)

	if !strings.Contains(body, "fix the login bug") {
		t.Errorf("card dropped the switch confirmation:\n%s", body)
	}
	for _, want := range []string{"charlie", "delta", "echo", "foxtrot", "golf"} {
		if !strings.Contains(body, want) {
			t.Errorf("card preview is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "alpha") {
		t.Errorf("card preview should stop at the last %d messages:\n%s", sessionPreviewEntries, body)
	}
	if got := e.sessions.GetOrCreateActive(sessionKey).GetAgentSessionID(); got != "sess-1" {
		t.Errorf("active agent session = %q, want sess-1", got)
	}
}

// A query the picker cannot resolve must fall back to the list rather than
// claim a switch that did not happen.
func TestSwitchCardAction_NoMatchFallsBackToTheList(t *testing.T) {
	agent := &stubListAgent{sessions: []AgentSessionInfo{
		{ID: "sess-1", Summary: "only session", MessageCount: 2, ModifiedAt: time.Now()},
	}}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)

	card := e.handleCardNav("act:/switch 99", "test:user1")
	if card == nil {
		t.Fatal("card action returned no card")
	}
	if countCardActionValues(card, "act:/switch ") == 0 {
		t.Errorf("expected the session list back, got:\n%s", cardMarkdown(card))
	}
}

// The card path used to wipe the Session's History on every switch, which the
// typed command deliberately does not do: switching back to a session magpie
// already drove returns the *existing* Session, and clearing it throws away the
// conversation that /history and the preview both read.
func TestSwitchCardAction_KeepsHistoryOfASessionSwitchedBackInto(t *testing.T) {
	agent := &stubListAgent{sessions: []AgentSessionInfo{
		{ID: "sess-1", Summary: "first", MessageCount: 2, ModifiedAt: time.Now()},
		{ID: "sess-2", Summary: "second", MessageCount: 2, ModifiedAt: time.Now().Add(-time.Hour)},
	}}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	sessionKey := "test:user1"

	first := e.sessions.GetOrCreateActive(sessionKey)
	first.SetAgentSessionID("sess-1", "codex")
	first.AddHistory("user", "something worth keeping")

	e.handleCardNav("act:/switch 2", sessionKey)
	e.handleCardNav("act:/switch 1", sessionKey)

	back := e.sessions.GetOrCreateActive(sessionKey)
	if back.GetAgentSessionID() != "sess-1" {
		t.Fatalf("switched back to %q, want sess-1", back.GetAgentSessionID())
	}
	if len(back.GetHistory(0)) == 0 {
		t.Error("switching back through the card wiped the session history")
	}
}
