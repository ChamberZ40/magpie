package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rolloutLines builds a Codex JSONL transcript: a session_meta header, then the
// given lines verbatim.
func writeRolloutFile(t *testing.T, dir, name, sessionID, cwd string, lines []string) string {
	t.Helper()
	meta, err := json.Marshal(map[string]any{
		"timestamp": "2026-09-24T10:00:00.000Z",
		"type":      "session_meta",
		"payload":   map[string]any{"id": sessionID, "cwd": cwd, "timestamp": "2026-09-24T10:00:00.000Z"},
	})
	if err != nil {
		t.Fatalf("marshal session_meta: %v", err)
	}
	path := filepath.Join(dir, name)
	body := string(meta) + "\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write rollout: %v", err)
	}
	return path
}

func messageLine(t *testing.T, role, text, ts string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"timestamp": ts,
		"type":      "response_item",
		"payload": map[string]any{
			"type": "message",
			"role": role,
			"content": []map[string]string{{
				"type": map[string]string{"user": "input_text", "assistant": "output_text"}[role],
				"text": text,
			}},
		},
	})
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	return string(b)
}

// A single tool output big enough to overflow the read buffer used to end the
// scan, so /history and the /switch preview showed only the messages recorded
// before the session's first large command — the opening of the conversation
// instead of where it got to. Real rollouts hit this within the first hundred
// lines: 400KB tool-output lines are routine.
func TestGetSessionHistory_SurvivesAHugeToolOutputLine(t *testing.T) {
	codexHome := t.TempDir()
	dir := filepath.Join(codexHome, "sessions", "2026", "09", "24")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bigOutput, err := json.Marshal(map[string]any{
		"timestamp": "2026-09-24T10:00:30.000Z",
		"type":      "response_item",
		"payload": map[string]any{
			"type":   "function_call_output",
			"output": strings.Repeat("log line of a very chatty build\n", 20000), // ~600KB
		},
	})
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
	}

	lines := []string{
		messageLine(t, "user", "early question", "2026-09-24T10:00:10.000Z"),
		messageLine(t, "assistant", "early answer", "2026-09-24T10:00:20.000Z"),
		string(bigOutput),
		messageLine(t, "user", "late question", "2026-09-24T10:00:40.000Z"),
		messageLine(t, "assistant", "late answer", "2026-09-24T10:00:50.000Z"),
	}
	sid := "01a0d164-7484-78f1-bf14-039b2910c04a"
	writeRolloutFile(t, dir, "rollout-2026-09-24T10-00-16-"+sid+".jsonl", sid, t.TempDir(), lines)

	entries, err := getSessionHistory(sid, codexHome, 0)
	if err != nil {
		t.Fatalf("getSessionHistory: %v", err)
	}
	if len(entries) != 4 {
		var got []string
		for _, e := range entries {
			got = append(got, e.Role+":"+e.Content)
		}
		t.Fatalf("got %d entries %v, want all 4 — the scan stopped at the oversized line", len(entries), got)
	}
	if entries[3].Content != "late answer" {
		t.Errorf("last entry = %q, want the newest message %q", entries[3].Content, "late answer")
	}
}

// The picker's own message count comes from a second reader with the same cap, so
// a session with one big tool output advertised far fewer messages than it had.
func TestParseCodexSessionFile_CountsMessagesPastAHugeLine(t *testing.T) {
	dir := t.TempDir()
	cwd := t.TempDir()

	big, err := json.Marshal(map[string]any{
		"timestamp": "2026-09-24T10:00:30.000Z",
		"type":      "response_item",
		"payload":   map[string]any{"type": "function_call_output", "output": strings.Repeat("x", 500000)},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	sid := "01a0d164-7484-78f1-bf14-039b2910c04b"
	path := writeRolloutFile(t, dir, "rollout-2026-09-24T10-00-16-"+sid+".jsonl", sid, cwd, []string{
		messageLine(t, "user", "first prompt", "2026-09-24T10:00:10.000Z"),
		string(big),
		messageLine(t, "user", "second prompt", "2026-09-24T10:00:40.000Z"),
		messageLine(t, "user", "third prompt", "2026-09-24T10:00:50.000Z"),
	})

	info := parseCodexSessionFile(path, cwd)
	if info == nil {
		t.Fatal("parseCodexSessionFile returned nil")
	}
	if info.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3 — messages after the oversized line were not counted", info.MessageCount)
	}
}

// getSessionHistory still has to report a missing transcript rather than an empty
// conversation: the preview treats "" as "nothing to show" and stays silent.
func TestGetSessionHistory_MissingFileIsAnError(t *testing.T) {
	if _, err := getSessionHistory("does-not-exist", t.TempDir(), 5); err == nil {
		t.Error("expected an error for a session with no transcript")
	}
}
