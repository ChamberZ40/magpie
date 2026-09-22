package appid

import (
	"os"
	"path/filepath"
	"testing"
)

// The whole point of DirIn is that a user who installed before the rename
// never has to move a file, while a fresh install gets the new name.

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestDirIn(t *testing.T) {
	tests := []struct {
		name    string
		present []string
		want    string
	}{
		{"fresh install gets the new name", nil, DirName},
		{"pre-rename install keeps its directory", []string{LegacyDirName}, LegacyDirName},
		{"new name wins when both exist", []string{DirName, LegacyDirName}, DirName},
		{"new name alone is used", []string{DirName}, DirName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			for _, d := range tt.present {
				mkdir(t, filepath.Join(parent, d))
			}
			want := filepath.Join(parent, tt.want)
			if got := DirIn(parent); got != want {
				t.Errorf("DirIn = %q, want %q", got, want)
			}
		})
	}
}

// A file squatting on the legacy name cannot be adopted as a state directory;
// adopting it would turn every later write into an ENOTDIR far from here.
func TestDirIn_IgnoresAFileAtTheLegacyName(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, LegacyDirName), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	want := filepath.Join(parent, DirName)
	if got := DirIn(parent); got != want {
		t.Errorf("DirIn = %q, want %q", got, want)
	}
}

// WorkspaceDir is the same resolution applied to a checkout, where the
// fallback keeps an existing .cc-connect/ .gitignore entry meaningful.
func TestWorkspaceDir_AdoptsAnExistingLegacyDir(t *testing.T) {
	work := t.TempDir()
	mkdir(t, filepath.Join(work, LegacyDirName))
	if got, want := WorkspaceDir(work), filepath.Join(work, LegacyDirName); got != want {
		t.Errorf("WorkspaceDir = %q, want %q", got, want)
	}
}

func TestWorkspaceDir_FreshCheckoutGetsTheNewName(t *testing.T) {
	work := t.TempDir()
	if got, want := WorkspaceDir(work), filepath.Join(work, DirName); got != want {
		t.Errorf("WorkspaceDir = %q, want %q", got, want)
	}
}

func TestHomeDir_IsUnderTheUserHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	got, err := HomeDir()
	if err != nil {
		t.Fatalf("HomeDir: %v", err)
	}
	if filepath.Dir(got) != home {
		t.Errorf("HomeDir = %q, want it directly under %q", got, home)
	}
	if base := filepath.Base(got); base != DirName && base != LegacyDirName {
		t.Errorf("HomeDir = %q, want it named %q or %q", got, DirName, LegacyDirName)
	}
}
