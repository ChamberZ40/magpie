package core

import (
	"log/slog"
)

// switchSession points sessionKey at an existing agent session, resolving query
// the same way for every entry point.
//
// The typed "/switch 4" and the button on the session picker are two different
// call chains, and they used to carry two different copies of this logic — which
// is how the picker ended up wiping history the typed command deliberately
// keeps. One implementation, no drift.
//
// Returns (nil, nil, err) when the agent cannot be listed, and (nil, nil, nil)
// when nothing matched: both are the caller's to report, since the phrasing
// differs between a chat reply and a card.
//
// NOTE: do NOT call ClearHistory on the returned Session. Switching back to a
// known agent_session_id returns the *existing* Session, whose History is the
// original conversation; wiping it makes /history and the switch preview come
// back empty after a round trip. For a freshly created Session History is
// already nil, so keeping it is a no-op.
func (e *Engine) switchSession(agent Agent, sessions *SessionManager, interactiveKey, sessionKey, query string) (*AgentSessionInfo, *Session, error) {
	agentSessions, err := agent.ListSessions(e.ctx)
	if err != nil {
		return nil, nil, err
	}
	agentSessions = e.applySessionFilter(agentSessions, sessions)

	matched := e.matchSession(agentSessions, sessions, query)
	if matched == nil {
		return nil, nil, nil
	}

	slog.Info("switchSession: cleaning up old session", "session_key", sessionKey)
	e.cleanupInteractiveState(interactiveKey)
	slog.Info("switchSession: cleanup done", "session_key", sessionKey)

	switched := sessions.SwitchToAgentSession(sessionKey, matched.ID, agent.Name(), matched.Summary)
	return matched, switched, nil
}

// switchConfirmation is what the user reads after a switch: which session they
// landed in, plus the tail of it. A confirmation naming an id and a message
// count says nothing about what the conversation was; the tail does.
func (e *Engine) switchConfirmation(agent Agent, sessions *SessionManager, matched *AgentSessionInfo, switched *Session) string {
	shortID := matched.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	displayName := sessions.GetSessionName(matched.ID)
	if displayName == "" {
		displayName = matched.Summary
	}

	text := e.i18n.Tf(MsgSwitchSuccess, displayName, shortID, matched.MessageCount)
	if preview := e.sessionPreview(agent, switched); preview != "" {
		text += "\n\n" + preview
	}
	return text
}

// handleSwitchCardAction runs a switch picked from the session card. It mirrors
// handleModelCardAction: the generic executeCardAction path can only redraw the
// list it came from, which looks to the user like the button did nothing.
//
// Anything it cannot act on falls back to the list rather than claiming a switch
// that did not happen.
func (e *Engine) handleSwitchCardAction(args, sessionKey string) *Card {
	if args == "" {
		return e.renderListCardSafe(sessionKey, 1)
	}
	agent, sessions := e.sessionContextForKey(sessionKey)
	interactiveKey := e.interactiveKeyForSessionKey(sessionKey)

	matched, switched, err := e.switchSession(agent, sessions, interactiveKey, sessionKey, args)
	if err != nil {
		return e.simpleCard(e.i18n.Tf(MsgCardTitleSessions, agent.Name(), 0), "red", err.Error())
	}
	if matched == nil {
		return e.renderListCardSafe(sessionKey, 1)
	}

	return NewCard().
		Title(e.i18n.T(MsgCardTitleSwitched), "green").
		Markdown(e.switchConfirmation(agent, sessions, matched, switched)).
		Buttons(DefaultBtn(e.i18n.T(MsgCardBack), "nav:/switch")).
		Build()
}
