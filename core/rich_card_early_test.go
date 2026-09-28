package core

import (
	"strings"
	"testing"
	"time"
)

// runEarlyRichCardTurn drives one rich-mode turn over the given events.
// msgID is what separates a user's message (the platform gave it an id) from a
// cron, timer or heartbeat prompt (none), which is the line the early card is
// drawn on.
func runEarlyRichCardTurn(t *testing.T, msgID string, events []Event) *stubRichCardSilentPlatform {
	t.Helper()
	p := &stubRichCardSilentPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(DisplayCfg{
		Mode:             "full",
		CardMode:         "rich",
		ThinkingMessages: true,
		ThinkingMaxLen:   300,
		ToolMaxLen:       500,
		ToolDetail:       ToolDetailFull,
	})
	sessionKey := "feishu:user-early-card"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s-early-card")
	state := &interactiveState{agentSession: agentSession, platform: p, replyCtx: "ctx-early-card"}
	e.interactiveStates[sessionKey] = state

	for _, ev := range events {
		agentSession.events <- ev
	}
	if len(events) == 0 {
		// Nothing will ever arrive: let the idle timeout end the turn.
		e.eventIdleTimeout = 50 * time.Millisecond
	}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, msgID, time.Now(), nil, nil, state.replyCtx)
	return p
}

// The card goes up as soon as the turn starts, not when the first text
// arrives: creating it costs two platform round trips, and paying them while
// the agent CLI is still starting hides them from the user.
func TestRichCardOpensBeforeTheFirstEventForAUserMessage(t *testing.T) {
	p := runEarlyRichCardTurn(t, "om_user", []Event{
		{Type: EventText, Content: "hello"},
		{Type: EventResult, Content: "hello", Done: true},
	})
	starts, _, updates, _ := p.snapshot()

	if len(starts) != 1 {
		t.Fatalf("SendPreviewStart called %d times, want exactly 1: %v", len(starts), starts)
	}
	if !strings.Contains(starts[0], "status=thinking") || !strings.Contains(starts[0], `body=""`) {
		t.Errorf("first card = %q, want an empty thinking card opened before any text", starts[0])
	}
	if len(updates) == 0 || !strings.Contains(updates[len(updates)-1], `status=done steps=0 body="hello"`) {
		t.Errorf("final update = %v, want the card finished with the reply", updates)
	}
}

// A user message the agent answers with a bare NO_REPLY already has a card on
// screen. Recalling it would leave a "message recalled" bar, so the card is
// finished in place and says there was nothing to say.
func TestRichCardEarlyCardFinishesQuietlyOnNoReply(t *testing.T) {
	p := runEarlyRichCardTurn(t, "om_user", []Event{
		{Type: EventText, Content: "NO_REPLY"},
		{Type: EventResult, Content: "NO_REPLY", Done: true},
	})
	starts, _, updates, deletes := p.snapshot()

	if len(starts) != 1 {
		t.Fatalf("SendPreviewStart called %d times, want 1", len(starts))
	}
	if deletes != 0 {
		t.Errorf("DeletePreviewMessage called %d times, want the card kept", deletes)
	}
	want := `status=done steps=0 body="` + NewI18n(LangEnglish).T(MsgNoReplyContent) + `"`
	if len(updates) == 0 || !strings.Contains(updates[len(updates)-1], want) {
		t.Errorf("final update = %v, want %s", updates, want)
	}
}

// Scheduled prompts are where NO_REPLY is the point ("nothing to report"), so
// they keep the old behaviour: no card until the agent has something to show.
func TestRichCardStaysLazyForAScheduledPrompt(t *testing.T) {
	p := runEarlyRichCardTurn(t, "", []Event{
		{Type: EventText, Content: "NO_REPLY"},
		{Type: EventResult, Content: "NO_REPLY", Done: true},
	})
	starts, streams, updates, deletes := p.snapshot()

	if len(starts)+len(streams)+len(updates)+deletes != 0 {
		t.Errorf("scheduled NO_REPLY left a trace: starts=%v streams=%v updates=%v deletes=%d", starts, streams, updates, deletes)
	}
}

// An early card exists before the agent has said anything, so a turn that
// dies without a result must not leave it spinning as "thinking" for good.
func TestRichCardEarlyCardIsClosedWhenTheAgentGoesSilent(t *testing.T) {
	p := runEarlyRichCardTurn(t, "om_user", nil)
	starts, _, updates, _ := p.snapshot()

	if len(starts) != 1 {
		t.Fatalf("SendPreviewStart called %d times, want 1", len(starts))
	}
	if len(updates) == 0 || !strings.Contains(updates[len(updates)-1], "status=error") {
		t.Errorf("updates = %v, want the card closed as failed", updates)
	}
}
