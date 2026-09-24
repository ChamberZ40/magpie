package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Codex records the directory it was launched from, and /list scopes the picker
// to the project's work_dir. Requiring the two to be string-equal hid almost
// everything: a repo checked out under the work_dir is its own cwd, so a session
// started there could never be resumed from chat even though it belongs to the
// same project tree.
func TestCwdWithinWorkDir(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sessionCwd string
		workDir    string
		want       bool
	}{
		{"same directory", "/Users/me/code", "/Users/me/code", true},
		{"direct child", "/Users/me/code/magpie", "/Users/me/code", true},
		{"deep descendant", "/Users/me/code/a/b/c", "/Users/me/code", true},
		{"parent is not a descendant", "/Users/me", "/Users/me/code", false},
		{"sibling", "/Users/me/other", "/Users/me/code", false},
		// The reason this cannot be a plain string prefix test: "code-old"
		// starts with "code" without being inside it.
		{"prefix but not a child", "/Users/me/code-old", "/Users/me/code", false},
		{"unrelated tree", "/Users/me/Documents/notes", "/Users/me/code", false},
		{"trailing separator on work dir", "/Users/me/code/magpie", "/Users/me/code/", true},
		{"unclean path", "/Users/me/code/./magpie", "/Users/me/code", true},
		// An empty work_dir means the project never scoped itself, so nothing
		// should be filtered out.
		{"no work dir keeps everything", "/anywhere", "", true},
		// A rollout with no recorded cwd cannot be placed; dropping it would
		// hide a resumable session on no evidence.
		{"no session cwd is kept", "", "/Users/me/code", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cwdWithinWorkDir(tc.sessionCwd, tc.workDir); got != tc.want {
				t.Errorf("cwdWithinWorkDir(%q, %q) = %v, want %v", tc.sessionCwd, tc.workDir, got, tc.want)
			}
		})
	}
}

// End to end through ListSessions: the subdirectory session is the one that used
// to vanish, and the sibling tree is what stops this from being "list everything".
func TestAgentListSessions_IncludesSubdirectoriesOfWorkDir(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "code")
	codexHome := t.TempDir()
	sessionsDir := filepath.Join(codexHome, "sessions", "2026", "09", "24")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("create sessions directory: %v", err)
	}

	writeRollout := func(name, sessionID, cwd string) {
		t.Helper()
		cwdJSON, err := json.Marshal(cwd)
		if err != nil {
			t.Fatalf("encode cwd %q: %v", cwd, err)
		}
		body := `{"type":"session_meta","payload":{"id":"` + sessionID + `","cwd":` + string(cwdJSON) + `,"source":"vscode"}}` + "\n" +
			`{"type":"response_item","payload":{"role":"user","content":[{"type":"input_text","text":"fix the login bug"}]}}` + "\n"
		if err := os.WriteFile(filepath.Join(sessionsDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write rollout %s: %v", name, err)
		}
	}

	writeRollout("rollout-at-root.jsonl", "at-root", workDir)
	writeRollout("rollout-in-subdir.jsonl", "in-subdir", filepath.Join(workDir, "magpie"))
	writeRollout("rollout-outside.jsonl", "outside", filepath.Join(root, "Documents"))

	agent := &Agent{workDir: workDir, codexHome: codexHome}
	sessions, err := agent.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	got := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		got[s.ID] = true
	}
	for _, want := range []string{"at-root", "in-subdir"} {
		if !got[want] {
			t.Errorf("ListSessions() dropped session %q from the work_dir tree: %v", want, got)
		}
	}
	if got["outside"] {
		t.Errorf("ListSessions() returned a session from outside work_dir: %v", got)
	}
}
