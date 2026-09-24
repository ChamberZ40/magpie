package core

import (
	"fmt"
	"log/slog"
	"strings"
)

// sessionPreviewEntries is how many trailing messages /switch replays. Five is
// enough to recognise where a conversation got to without turning a switch
// confirmation into a wall of text; /history is there for more.
const sessionPreviewEntries = 5

// resolveHistory returns the last n messages of a session, preferring what this
// process holds in memory and falling back to the agent's own transcript.
//
// The fallback is the whole point: a session started in the Codex app, in the
// terminal, or under an earlier magpie process has no in-memory history at all,
// and those are precisely the sessions worth switching into.
func (e *Engine) resolveHistory(agent Agent, s *Session, n int) []HistoryEntry {
	if s == nil {
		return nil
	}
	if entries := s.GetHistory(n); len(entries) > 0 {
		return entries
	}

	agentSID := s.GetAgentSessionID()
	if agentSID == "" || agent == nil {
		return nil
	}
	provider, ok := agent.(HistoryProvider)
	if !ok {
		return nil
	}
	entries, err := provider.GetSessionHistory(e.ctx, agentSID, n)
	if err != nil {
		// Warn rather than surface: every caller has already done the thing the
		// user asked for, and a missing transcript must not report that as a
		// failure.
		slog.Warn("session history unavailable from agent transcript",
			"agent_session", agentSID, "error", err)
		return nil
	}
	return entries
}

// formatHistoryEntries renders messages as one icon-prefixed, timestamped block
// each. Shared so /history, the history card and the /switch preview cannot
// drift into three different layouts.
func formatHistoryEntries(entries []HistoryEntry, maxLen int) string {
	var sb strings.Builder
	for _, h := range entries {
		icon := "👤"
		if h.Role == "assistant" {
			icon = "🤖"
		}
		sb.WriteString(fmt.Sprintf("%s [%s]\n%s\n\n",
			icon, h.Timestamp.Format("15:04:05"), truncateHistoryEntry(h.Content, maxLen)))
	}
	return sb.String()
}

// sessionPreview renders the tail of a session being switched into, so the chat
// shows what the conversation was about instead of just its id and a count.
// Returns "" when there is nothing to show, which callers append unconditionally.
func (e *Engine) sessionPreview(agent Agent, s *Session) string {
	entries := e.resolveHistory(agent, s, sessionPreviewEntries)
	if len(entries) == 0 {
		return ""
	}
	return e.i18n.Tf(MsgSwitchPreview, len(entries)) + "\n\n" +
		formatHistoryEntries(entries, e.historyEntryMaxLen())
}
