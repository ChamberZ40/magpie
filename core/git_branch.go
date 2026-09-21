package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Git repository discovery for the reply footer.
//
// This reads .git/HEAD itself instead of running `git rev-parse`. The footer is
// rebuilt on every card repaint — several times a second on a streaming turn —
// and forking a process that often to read a value which changes maybe once an
// hour is not a trade worth making. A HEAD read is one open of a ~40-byte file.
//
// The cost of owning the parse is the two formats below, both stable parts of
// git's on-disk layout.

const (
	// gitHeadRefPrefix marks a symbolic HEAD: "ref: refs/heads/<branch>".
	gitHeadRefPrefix = "ref: refs/heads/"
	// gitDirFilePrefix marks a .git *file* rather than a directory, used by
	// linked worktrees and submodules: "gitdir: <path>".
	gitDirFilePrefix = "gitdir: "
	// gitShortSHALen is how much of a detached HEAD's sha to show. Matches what
	// git itself abbreviates to by default.
	gitShortSHALen = 7
	// gitDiscoveryMaxDepth bounds the climb toward the filesystem root so a
	// pathological path cannot spin. No real checkout is anywhere near this deep.
	gitDiscoveryMaxDepth = 64
)

// richFooterGitGlyph marks the branch segment. A glyph rather than a localized
// word: branch names are not translatable, and a label would be the only part
// of the segment that needed i18n.
const richFooterGitGlyph = "⎇"

// gitBranch returns the branch checked out in dir, or the abbreviated commit
// when HEAD is detached. Returns "" when dir is not inside a git repository,
// which is the normal case for a non-repo workdir and not an error.
func gitBranch(dir string) string {
	gitDir := findGitDir(dir)
	if gitDir == "" {
		return ""
	}
	return parseGitHead(readGitFile(filepath.Join(gitDir, "HEAD")))
}

// findGitDir climbs from dir toward the root looking for .git, resolving the
// "gitdir:" indirection when .git is a file. Returns "" if there is none.
//
// Like git, the search stops at the first .git found: a broken pointer means
// this checkout is broken, not that the enclosing repository owns these files.
func findGitDir(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	cur, err := filepath.Abs(dir)
	if err != nil {
		slog.Debug("git footer: cannot resolve work dir", "dir", dir, "error", err)
		return ""
	}
	for i := 0; i < gitDiscoveryMaxDepth; i++ {
		candidate := filepath.Join(cur, ".git")
		info, err := os.Stat(candidate)
		switch {
		case err == nil && info.IsDir():
			return candidate
		case err == nil:
			return resolveGitDirFile(candidate, cur)
		case !os.IsNotExist(err):
			// Unreadable is worth a line; missing is the common case and is not.
			slog.Debug("git footer: cannot stat .git", "path", candidate, "error", err)
			return ""
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
	return ""
}

// resolveGitDirFile reads a .git file and returns the directory it points at,
// relative paths being relative to the checkout that holds the file.
func resolveGitDirFile(path, workDir string) string {
	line := strings.TrimSpace(readGitFile(path))
	target, ok := strings.CutPrefix(line, gitDirFilePrefix)
	if !ok {
		return ""
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(workDir, target)
	}
	return target
}

// readGitFile reads a small git metadata file, returning "" on any failure.
// Every caller treats absence as "not a repository", so failures are logged at
// debug rather than surfaced — a footer must never be the reason a reply fails.
func readGitFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("git footer: cannot read git file", "path", path, "error", err)
		}
		return ""
	}
	return string(data)
}

// parseGitHead turns HEAD's contents into a display string: the branch name for
// a symbolic ref, the abbreviated sha for a detached HEAD, "" for anything else.
func parseGitHead(head string) string {
	head = strings.TrimSpace(head)
	if head == "" {
		return ""
	}
	if branch, ok := strings.CutPrefix(head, gitHeadRefPrefix); ok {
		return strings.TrimSpace(branch)
	}
	if isHexSHA(head) {
		if len(head) > gitShortSHALen {
			return head[:gitShortSHALen]
		}
		return head
	}
	return ""
}

// isHexSHA reports whether s looks like a full object name. Anything else in
// HEAD — a ref outside refs/heads, a malformed file — is not something the
// footer can render usefully.
func isHexSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		isHexDigit := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHexDigit {
			return false
		}
	}
	return true
}
