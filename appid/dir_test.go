package appid

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirIn_UsesTheProjectDirName(t *testing.T) {
	parent := t.TempDir()
	if got, want := DirIn(parent), filepath.Join(parent, DirName); got != want {
		t.Errorf("DirIn = %q, want %q", got, want)
	}
}

func TestWorkspaceDir_IsTheProjectDirInsideTheCheckout(t *testing.T) {
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
	if base := filepath.Base(got); base != DirName {
		t.Errorf("HomeDir = %q, want it named %q", got, DirName)
	}
}
