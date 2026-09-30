package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const agentPatchFixture = `# top comment
[[projects]]
name = "a"

[projects.agent]
# keep me
type = "claudecode"

[projects.agent.options]
work_dir = "/path/to/your/project" # inline
mode = "default"

[[projects]]
name = "b"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/b"
`

func withConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := ConfigPath
	ConfigPath = path
	t.Cleanup(func() { ConfigPath = old })
	return path
}

func TestSaveAgentWorkDirAndType_TouchOnlyTheTargetProject(t *testing.T) {
	path := withConfigFile(t, agentPatchFixture)

	if err := SaveAgentWorkDir("a", "/real/dir"); err != nil {
		t.Fatalf("SaveAgentWorkDir: %v", err)
	}
	if err := SaveAgentType("a", "cursor"); err != nil {
		t.Fatalf("SaveAgentType: %v", err)
	}

	cfg, err := LoadPermissive(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := cfg.Projects[0], cfg.Projects[1]
	if a.Agent.Type != "cursor" || a.Agent.Options["work_dir"] != "/real/dir" {
		t.Errorf("project a = %q %v, want cursor at /real/dir", a.Agent.Type, a.Agent.Options["work_dir"])
	}
	if b.Agent.Type != "codex" || b.Agent.Options["work_dir"] != "/b" {
		t.Errorf("project b changed: %q %v", b.Agent.Type, b.Agent.Options["work_dir"])
	}

	data, _ := os.ReadFile(path)
	for _, keep := range []string{"# top comment", "# keep me", "# inline"} {
		if !strings.Contains(string(data), keep) {
			t.Errorf("comment %q was lost", keep)
		}
	}
}

func TestSaveAgentType_UnknownProject(t *testing.T) {
	withConfigFile(t, agentPatchFixture)
	if err := SaveAgentType("nope", "codex"); err == nil {
		t.Error("want an error for a project that does not exist")
	}
}

func TestSaveSectionValue_CreatesOrUpdatesSection(t *testing.T) {
	path := withConfigFile(t, agentPatchFixture)

	if err := SaveSectionValue("power", "prevent_sleep", `"ac_only"`); err != nil {
		t.Fatal(err)
	}
	if err := SaveSectionValue("power", "prevent_sleep", `"always"`); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPermissive(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Power.PreventSleep; got != "always" {
		t.Errorf("prevent_sleep = %q, want always", got)
	}
	if n := len(cfg.Projects); n != 2 {
		t.Errorf("projects = %d after patch, want 2", n)
	}
}

func TestSavePlatformOptions_WritesIntoTheTypedPlatform(t *testing.T) {
	path := withConfigFile(t, agentPatchFixture+`
[[projects.platforms]]
type = "weixin"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_x" # mine
`)
	err := SavePlatformOptions("b", "feishu", []OptionValue{
		{Key: "thread_isolation", Raw: "true"},
		{Key: "progress_style", Raw: `"card"`},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPermissive(path)
	if err != nil {
		t.Fatal(err)
	}
	pl := cfg.Projects[1].Platforms
	if len(pl[0].Options) != 0 {
		t.Errorf("weixin platform got options %v", pl[0].Options)
	}
	opts := pl[1].Options
	if opts["thread_isolation"] != true || opts["progress_style"] != "card" || opts["app_id"] != "cli_x" {
		t.Errorf("feishu options = %v", opts)
	}

	if err := SavePlatformOptions("b", "lark", []OptionValue{{Key: "x", Raw: "1"}}); err == nil {
		t.Error("want an error when the project has no platform of that type")
	}
}
