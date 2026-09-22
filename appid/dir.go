package appid

import (
	"fmt"
	"os"
	"path/filepath"
)

// HomeDir returns the per-user state directory — config.toml, the session
// store, logs, the API socket. Normally ~/.magpie.
//
// If ~/.magpie does not exist but the pre-rename ~/.cc-connect does, the old
// path is returned, so an install that predates the rename keeps its config,
// sessions and logs without the user moving anything. Once ~/.magpie exists it
// always wins, so a fresh install is never dragged back onto the old name.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("appid: locate home directory: %w", err)
	}
	return DirIn(home), nil
}

// WorkspaceDir returns the directory this project writes into a workspace —
// attachments and images downloaded from chat.
//
// The fallback matters more here than for HomeDir: this directory is created
// inside the user's own repository, and many of them have .cc-connect/ in a
// .gitignore. Adopting the existing directory rather than creating a second
// one keeps those entries working and avoids littering a checkout with two
// near-identical directories.
func WorkspaceDir(workDir string) string {
	return DirIn(workDir)
}

// DirIn resolves this project's directory inside parent: the current name if
// it exists, otherwise the legacy name if that is the only one present,
// otherwise the current name (so it is what gets created).
//
// Existence is the test rather than, say, a migration marker, because it is
// the one signal available without writing anything — callers use this on
// read paths where creating state would be wrong.
func DirIn(parent string) string {
	current := filepath.Join(parent, DirName)
	if isDir(current) {
		return current
	}
	if legacy := filepath.Join(parent, LegacyDirName); isDir(legacy) {
		return legacy
	}
	return current
}

// isDir reports whether path exists and is a directory. A plain file sitting
// at the name does not count: it cannot be adopted, and treating it as the
// state directory would turn every later write into a confusing ENOTDIR.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
