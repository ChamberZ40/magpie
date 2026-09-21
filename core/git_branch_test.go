package core

import (
	"os"
	"path/filepath"
	"testing"
)

// The footer reads .git/HEAD directly rather than shelling out to git. A
// streaming turn repaints the card several times a second, and forking a
// process each time to read a value that changes maybe once an hour is not a
// trade worth making. These tests pin the file formats that decision commits us
// to parsing ourselves.

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestGitBranch_ReadsHEAD(t *testing.T) {
	tests := []struct {
		name string
		head string
		want string
	}{
		{"branch", "ref: refs/heads/main\n", "main"},
		{"slashed branch", "ref: refs/heads/feat/git-footer\n", "feat/git-footer"},
		{"no trailing newline", "ref: refs/heads/main", "main"},
		// Detached HEAD holds a raw sha. Showing all 40 chars would swamp the
		// footer, and the short form is what every other tool prints.
		{"detached", "9fceb02d0ae598e95dc970b74767f19372d61af8\n", "9fceb02"},
		{"empty", "", ""},
		{"garbage", "not a ref\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, ".git", "HEAD"), tt.head)
			if got := gitBranch(dir); got != tt.want {
				t.Errorf("gitBranch = %q, want %q", got, tt.want)
			}
		})
	}
}

// The agent's work dir is usually a subdirectory of the repo root, so the
// lookup has to climb — the same way git itself finds a repository.
func TestGitBranch_ClimbsToRepoRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	nested := filepath.Join(root, "core", "internal", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := gitBranch(nested); got != "main" {
		t.Errorf("gitBranch(nested) = %q, want %q", got, "main")
	}
}

// In a linked worktree or a submodule, .git is a file pointing elsewhere.
func TestGitBranch_FollowsGitdirFile(t *testing.T) {
	base := t.TempDir()
	realGit := filepath.Join(base, "store", "worktrees", "wt")
	writeFile(t, filepath.Join(realGit, "HEAD"), "ref: refs/heads/side-quest\n")

	work := filepath.Join(base, "work")
	writeFile(t, filepath.Join(work, ".git"), "gitdir: "+realGit+"\n")

	if got := gitBranch(work); got != "side-quest" {
		t.Errorf("gitBranch = %q, want %q", got, "side-quest")
	}
}

func TestGitBranch_EmptyOutsideRepo(t *testing.T) {
	for _, dir := range []string{t.TempDir(), "", filepath.Join(t.TempDir(), "does-not-exist")} {
		if got := gitBranch(dir); got != "" {
			t.Errorf("gitBranch(%q) = %q, want empty", dir, got)
		}
	}
}

// A .git file pointing at a path with no HEAD must not be mistaken for a repo,
// and must not climb past it either — git stops at the first .git it finds.
func TestGitBranch_BrokenGitdirYieldsNothing(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, ".git", "HEAD"), "ref: refs/heads/outer\n")
	work := filepath.Join(base, "work")
	writeFile(t, filepath.Join(work, ".git"), "gitdir: /nowhere/at/all\n")

	if got := gitBranch(work); got != "" {
		t.Errorf("gitBranch = %q, want empty — a broken gitdir is not the parent repo", got)
	}
}
