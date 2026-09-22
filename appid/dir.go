package appid

import (
	"fmt"
	"os"
	"path/filepath"
)

// HomeDir returns the per-user state directory — config.toml, the session
// store, logs, the API socket. Always ~/.magpie.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("appid: locate home directory: %w", err)
	}
	return DirIn(home), nil
}

// WorkspaceDir returns the directory this project writes into a workspace —
// attachments and images downloaded from chat.
func WorkspaceDir(workDir string) string {
	return DirIn(workDir)
}

// DirIn resolves this project's directory inside parent.
func DirIn(parent string) string {
	return filepath.Join(parent, DirName)
}
