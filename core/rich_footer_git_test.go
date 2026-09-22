package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The branch belongs next to the work dir it describes, but on its own flag:
// someone who hides the path may still want to know which branch a turn touched.

func newGitFooterEngine(t *testing.T, workDir string) *Engine {
	t.Helper()
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
	e.SetReplyFooterEnabled(true)
	e.SetShowContextIndicator(false)
	e.SetShowWorkdirIndicator(true)
	e.SetShowGitIndicator(true)
	return e
}

func gitRepoDir(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	head := filepath.Join(dir, ".git", "HEAD")
	if err := os.MkdirAll(filepath.Dir(head), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(head, []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	return dir
}

func TestRichFooter_ShowsGitBranch(t *testing.T) {
	dir := gitRepoDir(t, "feat/git-footer")
	e := newGitFooterEngine(t, dir)

	got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
	if !strings.Contains(got, "⎇ feat/git-footer") {
		t.Errorf("footer = %q, want it to carry the branch", got)
	}
}

// The footer is two lines: what the turn did, then where it ran. Crowding both
// onto one line pushed the context bar off the visible width once a real
// workdir path was in there.
func TestRichFooter_PlaceGoesOnTheSecondLine(t *testing.T) {
	dir := gitRepoDir(t, "main")
	e := newGitFooterEngine(t, dir)
	e.SetShowContextIndicator(true)

	got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("footer = %q, want exactly 2 lines, got %d", got, len(lines))
	}
	if strings.Contains(lines[0], "⎇") || strings.Contains(lines[0], compactReplyFooterPath(dir)) {
		t.Errorf("line 1 = %q, want no workdir or branch on it", lines[0])
	}
	if !strings.Contains(lines[1], "⎇ main") {
		t.Errorf("line 2 = %q, want the branch", lines[1])
	}
	if !strings.Contains(lines[1], compactReplyFooterPath(dir)) {
		t.Errorf("line 2 = %q, want the work dir", lines[1])
	}
}

// With nothing to say about the place, there is no blank second line.
func TestRichFooter_NoSecondLineWhenPlaceIsEmpty(t *testing.T) {
	dir := t.TempDir() // not a repository
	e := newGitFooterEngine(t, dir)
	e.SetShowWorkdirIndicator(false)

	got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
	if strings.Contains(got, "\n") {
		t.Errorf("footer = %q, want a single line", got)
	}
	if strings.TrimSpace(got) == "" {
		t.Errorf("footer = %q, want line 1 to survive", got)
	}
}

func TestRichFooter_OmitsGitBranchOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	e := newGitFooterEngine(t, dir)

	got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
	if strings.Contains(got, "⎇") {
		t.Errorf("footer = %q, want no branch segment outside a repository", got)
	}
}

// The two segments are independent: hiding the path must not hide the branch,
// and hiding the branch must not hide the path.
func TestRichFooter_GitAndWorkdirFlagsAreIndependent(t *testing.T) {
	dir := gitRepoDir(t, "main")

	t.Run("git off", func(t *testing.T) {
		e := newGitFooterEngine(t, dir)
		e.SetShowGitIndicator(false)
		got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
		if strings.Contains(got, "⎇") {
			t.Errorf("footer = %q, want no branch when the flag is off", got)
		}
		if !strings.Contains(got, compactReplyFooterPath(dir)) {
			t.Errorf("footer = %q, want the work dir to survive", got)
		}
	})

	t.Run("workdir off", func(t *testing.T) {
		e := newGitFooterEngine(t, dir)
		e.SetShowWorkdirIndicator(false)
		got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
		if !strings.Contains(got, "⎇ main") {
			t.Errorf("footer = %q, want the branch without the path", got)
		}
		if strings.Contains(got, compactReplyFooterPath(dir)) {
			t.Errorf("footer = %q, want no work dir when the flag is off", got)
		}
	})
}

// replyFooterWorkDir compacts the path for display ("~/code/x"), which is not a
// path the filesystem can be asked about. The branch lookup must use the raw one.
func TestRichFooter_GitBranchUsesRawPathNotCompacted(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to compact against")
	}
	dir, err := os.MkdirTemp(home, "magpie-git-footer-")
	if err != nil {
		t.Skipf("cannot create a repo under home: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	head := filepath.Join(dir, ".git", "HEAD")
	if err := os.MkdirAll(filepath.Dir(head), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(head, []byte("ref: refs/heads/under-home\n"), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	e := newGitFooterEngine(t, dir)
	got := e.composeRichStatusFooter(false, time.Now(), e.agent, nil, dir)
	if !strings.HasPrefix(compactReplyFooterPath(dir), "~") {
		t.Fatalf("precondition: %q should compact to ~", dir)
	}
	if !strings.Contains(got, "⎇ under-home") {
		t.Errorf("footer = %q, want the branch resolved from the raw path", got)
	}
}
