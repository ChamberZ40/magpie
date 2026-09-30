package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/config"
	"github.com/ChamberZ40/magpie/core"
)

// The starter config used to carry an active feishu block with placeholder
// credentials. Anyone who then connected Weixin instead kept that block, and
// every start logged "app_id is invalid" next to a working bridge.
func TestDefaultConfigTemplate_HasNoActivePlatform(t *testing.T) {
	for _, line := range strings.Split(defaultConfigTemplate(), "\n") {
		if strings.TrimSpace(line) == "[[projects.platforms]]" {
			t.Fatal("template declares a platform; a placeholder one fails at startup")
		}
	}
}

func TestDefaultConfigTemplate_PointsAtSetupCommands(t *testing.T) {
	tmpl := defaultConfigTemplate()
	for _, cmd := range setupCommands() {
		if !strings.Contains(tmpl, cmd) {
			t.Errorf("template never mentions %q", cmd)
		}
	}
}

func TestSetupCommands_OnlyRealCommandsForRegisteredPlatforms(t *testing.T) {
	registered := make(map[string]bool)
	for _, n := range core.ListRegisteredPlatforms() {
		registered[n] = true
	}
	cmds := setupCommands()
	if len(cmds) == 0 && (registered["feishu"] || registered["weixin"]) {
		t.Fatal("a platform with a setup command is registered but none is offered")
	}
	for _, cmd := range cmds {
		assertRealCommand(t, cmd)
		name := strings.Fields(cmd)[1]
		if !registered[name] && !(name == "feishu" && registered["lark"]) {
			t.Errorf("%q offered, but platform %q is not in this build", cmd, name)
		}
	}
}

func TestFirstRunGuide_NamesPathWorkDirAndSetup(t *testing.T) {
	guide := firstRunGuide("/tmp/x/config.toml")
	if !strings.Contains(guide, "/tmp/x/config.toml") {
		t.Error("guide should name the config file")
	}
	if !strings.Contains(guide, "work_dir") {
		t.Error("guide should tell the user to set work_dir")
	}
	for _, cmd := range setupCommands() {
		if !strings.Contains(guide, cmd) {
			t.Errorf("guide never mentions %q", cmd)
		}
	}
	assertOnlyRealCommands(t, guide)
}

func TestEnsureConfigForSetup_BootstrapsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")

	created, err := ensureConfigForSetup(path)
	if err != nil || !created {
		t.Fatalf("ensureConfigForSetup = (%v, %v), want (true, nil)", created, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != defaultConfigTemplate() {
		t.Fatalf("bootstrapped file differs from the template (err %v)", err)
	}

	created, err = ensureConfigForSetup(path)
	if err != nil || created {
		t.Fatalf("second call = (%v, %v), want (false, nil)", created, err)
	}
}

// Setup must be able to add a platform to the bootstrapped project: that is
// the whole point of letting it be the first command a new user runs.
func TestBootstrappedConfig_AcceptsWeixinPlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := bootstrapConfig(path); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = path
	t.Cleanup(func() { config.ConfigPath = old })

	projects, err := config.ListProjects()
	if err != nil || len(projects) != 1 {
		t.Fatalf("ListProjects = (%v, %v), want one project", projects, err)
	}
	provision, err := config.EnsureProjectWithWeixinPlatform(config.EnsureProjectWithWeixinOptions{
		ProjectName: projects[0],
		WorkDir:     t.TempDir(),
	})
	if err != nil || provision.Created || !provision.AddedPlatform {
		t.Fatalf("ensure weixin = (%+v, %v), want a platform added to the existing project", provision, err)
	}
	if _, err := config.SaveWeixinPlatformCredentials(config.WeixinCredentialUpdateOptions{
		ProjectName: projects[0],
		Token:       "tok",
		BaseURL:     "https://example.invalid",
	}); err != nil {
		t.Fatalf("save weixin: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config with one real platform should load: %v", err)
	}
	if got := cfg.Projects[0].Agent.Type; got != "claudecode" {
		t.Errorf("agent type = %q, want the template's claudecode", got)
	}
}

func TestWorkDirReminder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := bootstrapConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := workDirReminder(path, "my-project"); !strings.Contains(got, "work_dir") {
		t.Errorf("placeholder work_dir should produce a reminder, got %q", got)
	}

	data, _ := os.ReadFile(path)
	fixed := strings.Replace(string(data), placeholderWorkDir, filepath.ToSlash(dir), 1)
	if err := os.WriteFile(path, []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := workDirReminder(path, "my-project"); got != "" {
		t.Errorf("real work_dir should need no reminder, got %q", got)
	}
}

func TestConfigLoadErrorHint_NoPlatformPointsAtSetup(t *testing.T) {
	err := fmt.Errorf("config: projects[0] %w", config.ErrNoPlatforms)
	hint := configLoadErrorHint("/tmp/config.toml", err)
	for _, cmd := range setupCommands() {
		if !strings.Contains(hint, cmd) {
			t.Errorf("hint for a project without platforms never mentions %q", cmd)
		}
	}
	assertOnlyRealCommands(t, hint)

	other := configLoadErrorHint("/tmp/config.toml", errors.New("boom"))
	for _, cmd := range setupCommands() {
		if strings.Contains(other, cmd) {
			t.Errorf("unrelated error should not suggest %q", cmd)
		}
	}
}

func assertOnlyRealCommands(t *testing.T, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "magpie ") {
			assertRealCommand(t, line)
		}
	}
}

func assertRealCommand(t *testing.T, cmd string) {
	t.Helper()
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "magpie" {
		t.Errorf("%q is not a magpie command", cmd)
		return
	}
	if _, ok := topLevelCommandHandlers[fields[1]]; !ok {
		t.Errorf("%q names a command that does not exist", cmd)
	}
}
