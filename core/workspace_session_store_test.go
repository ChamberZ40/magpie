package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// namedStubStoreAgent is a stub agent registered under a unique name so the
// engine can recreate it for a workspace.
type namedStubStoreAgent struct {
	stubAgent
	name string
}

func (a *namedStubStoreAgent) Name() string { return a.name }

// TestGetOrCreateWorkspaceAgent_NoStorePathStaysInMemory guards against a
// regression where an engine without a session store resolved Dir("") to "."
// and wrote per-workspace store files into the current working directory.
func TestGetOrCreateWorkspaceAgent_NoStorePathStaysInMemory(t *testing.T) {
	agentName := "test-workspace-store-inmemory"
	RegisterAgent(agentName, func(_ map[string]any) (Agent, error) {
		return &namedStubStoreAgent{name: agentName}, nil
	})

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	before := storeFilesIn(t, cwd)

	e := NewEngine("test", &namedStubStoreAgent{name: agentName},
		[]Platform{&stubPlatformEngine{n: "plain"}}, "", LangEnglish)
	e.SetMultiWorkspace(t.TempDir(), filepath.Join(t.TempDir(), "bindings.json"))

	_, sessions, err := e.getOrCreateWorkspaceAgent(normalizeWorkspacePath(t.TempDir()))
	if err != nil {
		t.Fatalf("getOrCreateWorkspaceAgent returned error: %v", err)
	}

	if got := sessions.StorePath(); got != "" {
		t.Errorf("workspace store path = %q, want empty (in-memory)", got)
	}
	if after := storeFilesIn(t, cwd); len(after) != len(before) {
		t.Errorf("store files in %s: got %d, want %d (leaked %v)",
			cwd, len(after), len(before), difference(before, after))
	}
}

// TestGetOrCreateWorkspaceAgent_StorePathSiblingFile verifies the per-workspace
// store still lands next to the engine's own store when one is configured.
func TestGetOrCreateWorkspaceAgent_StorePathSiblingFile(t *testing.T) {
	agentName := "test-workspace-store-sibling"
	RegisterAgent(agentName, func(_ map[string]any) (Agent, error) {
		return &namedStubStoreAgent{name: agentName}, nil
	})

	stateDir := t.TempDir()
	storePath := filepath.Join(stateDir, "sessions.json")

	e := NewEngine("test", &namedStubStoreAgent{name: agentName},
		[]Platform{&stubPlatformEngine{n: "plain"}}, storePath, LangEnglish)
	e.SetMultiWorkspace(t.TempDir(), filepath.Join(t.TempDir(), "bindings.json"))

	_, sessions, err := e.getOrCreateWorkspaceAgent(normalizeWorkspacePath(t.TempDir()))
	if err != nil {
		t.Fatalf("getOrCreateWorkspaceAgent returned error: %v", err)
	}

	got := sessions.StorePath()
	if dir := filepath.Dir(got); dir != stateDir {
		t.Errorf("workspace store dir = %q, want %q", dir, stateDir)
	}
	if base := filepath.Base(got); !strings.HasPrefix(base, "test_ws_") ||
		!strings.HasSuffix(base, ".json") {
		t.Errorf("workspace store file = %q, want test_ws_<hash>.json", base)
	}
}

// storeFilesIn lists the per-workspace store files directly inside dir.
func storeFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*_ws_*.json"))
	if err != nil {
		t.Fatalf("glob store files in %s: %v", dir, err)
	}
	return matches
}

// difference returns the entries present in after but not in before.
func difference(before, after []string) []string {
	seen := make(map[string]struct{}, len(before))
	for _, p := range before {
		seen[p] = struct{}{}
	}
	added := make([]string, 0, len(after))
	for _, p := range after {
		if _, ok := seen[p]; !ok {
			added = append(added, p)
		}
	}
	return added
}
