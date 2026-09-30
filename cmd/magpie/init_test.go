package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/config"
)

func newTestPrompter(input string) (*prompter, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return newPrompter(strings.NewReader(input), out, false), out
}

func TestPrompter_Confirm(t *testing.T) {
	for _, tc := range []struct {
		input string
		def   bool
		want  bool
	}{
		{"\n", true, true},
		{"\n", false, false},
		{"n\n", true, false},
		{"YES\n", false, true},
		{"maybe\ny\n", false, true}, // re-asks on nonsense
		{"", true, true},            // EOF takes the default
	} {
		p, _ := newTestPrompter(tc.input)
		if got := p.confirm("ok?", tc.def); got != tc.want {
			t.Errorf("confirm(%q, def %v) = %v, want %v", tc.input, tc.def, got, tc.want)
		}
	}
}

func TestPrompter_ChooseAndAsk(t *testing.T) {
	p, out := newTestPrompter("\n9\n2\n  /x  \n\n")
	opts := []string{"a", "b", "c"}
	if got := p.choose("pick", opts, 1); got != 1 {
		t.Errorf("empty answer = %d, want the default 1", got)
	}
	if got := p.choose("pick", opts, 0); got != 1 {
		t.Errorf("out-of-range then 2 = %d, want 1", got)
	}
	if got := p.ask("dir", "/d"); got != "/x" {
		t.Errorf("ask = %q, want trimmed /x", got)
	}
	if got := p.ask("dir", "/d"); got != "/d" {
		t.Errorf("empty ask = %q, want default /d", got)
	}
	if !strings.Contains(out.String(), "2) b") {
		t.Errorf("options not listed:\n%s", out.String())
	}
}

func TestPrompter_AssumeYesNeverReads(t *testing.T) {
	p := newPrompter(strings.NewReader("n\n"), &bytes.Buffer{}, true)
	if !p.confirm("ok?", true) || p.choose("pick", []string{"a", "b"}, 1) != 1 || p.ask("d", "/d") != "/d" {
		t.Error("--yes must take every default")
	}
}

func TestDetectAgents(t *testing.T) {
	found := map[string]bool{"codex": true, "agent": true}
	look := func(bin string) (string, error) {
		if found[bin] {
			return "/bin/" + bin, nil
		}
		return "", errors.New("not found")
	}
	got := detectAgents([]string{"acp", "claudecode", "codex", "cursor"}, look)

	var names []string
	for _, a := range got {
		names = append(names, a.Type)
		if a.Type == "acp" {
			t.Error("acp has no fixed binary to detect and must not be offered")
		}
	}
	if strings.Join(names, ",") != "claudecode,codex,cursor" {
		t.Errorf("agents = %v, want registered ones in preference order", names)
	}
	if got[0].Found || !got[1].Found || !got[2].Found {
		t.Errorf("found flags wrong: %+v", got)
	}
}

func TestDefaultAgentIndex(t *testing.T) {
	agents := []agentChoice{{Type: "claudecode"}, {Type: "codex", Found: true}, {Type: "cursor", Found: true}}
	if got := defaultAgentIndex(agents, "cursor"); got != 2 {
		t.Errorf("configured+installed agent should win, got %d", got)
	}
	if got := defaultAgentIndex(agents, "claudecode"); got != 1 {
		t.Errorf("configured but missing should fall to first installed, got %d", got)
	}
	if got := defaultAgentIndex([]agentChoice{{Type: "claudecode"}}, "copilot"); got != 0 {
		t.Errorf("nothing installed should fall to the first, got %d", got)
	}
}

func TestResolveWorkDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := resolveWorkDir(filepath.Join(dir, "missing")); err == nil {
		t.Error("a missing directory must be rejected")
	}
	file := filepath.Join(dir, "f")
	_ = os.WriteFile(file, nil, 0o644)
	if _, err := resolveWorkDir(file); err == nil {
		t.Error("a file must be rejected")
	}
	got, err := resolveWorkDir(dir)
	if err != nil || got != dir {
		t.Errorf("resolveWorkDir(%q) = (%q, %v)", dir, got, err)
	}
	home, _ := os.UserHomeDir()
	if got, err := resolveWorkDir("~"); err != nil || got != home {
		t.Errorf("~ = (%q, %v), want %q", got, err, home)
	}
}

// The recommended settings are the ones the maintainer runs with; each is
// offered as on-by-default and can be declined.
func TestRecommendedSettings_ApplyOnAndOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := bootstrapConfig(path); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = path
	t.Cleanup(func() { config.ConfigPath = old })
	if _, err := config.EnsureProjectWithFeishuPlatform(config.EnsureProjectWithFeishuOptions{
		ProjectName: "my-project", PlatformType: "feishu",
	}); err != nil {
		t.Fatal(err)
	}

	settings := recommendedSettings([]string{"feishu"}, "darwin")
	if len(settings) == 0 {
		t.Fatal("no settings offered")
	}
	for _, s := range settings {
		if err := s.apply("my-project", true); err != nil {
			t.Fatalf("%s on: %v", s.label, err)
		}
	}
	cfg, err := config.LoadPermissive(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := cfg.Projects[0].Platforms[0].Options
	if cfg.Power.PreventSleep != "ac_only" ||
		cfg.Display.CardMode == nil || *cfg.Display.CardMode != "rich" ||
		cfg.Display.ToolDetail == nil || *cfg.Display.ToolDetail != "full" ||
		opts["thread_isolation"] != true || opts["progress_style"] != "card" || opts["done_emoji"] != "Done" {
		t.Errorf("recommended values not applied: power=%q display=%+v opts=%v", cfg.Power.PreventSleep, cfg.Display, opts)
	}

	for _, s := range settings {
		if err := s.apply("my-project", false); err != nil {
			t.Fatalf("%s off: %v", s.label, err)
		}
	}
	cfg, err = config.LoadPermissive(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Power.PreventSleep != "off" || cfg.Projects[0].Platforms[0].Options["thread_isolation"] != false {
		t.Errorf("declining did not turn settings off: power=%q opts=%v", cfg.Power.PreventSleep, cfg.Projects[0].Platforms[0].Options)
	}
	if _, err := config.Load(path); err != nil {
		t.Errorf("config should still load after init settings: %v", err)
	}
}

func TestRecommendedSettings_ScopedToPlatformAndOS(t *testing.T) {
	for _, s := range recommendedSettings(nil, "linux") {
		if s.feishuOnly || s.darwinOnly {
			t.Errorf("%q offered on linux without feishu", s.label)
		}
	}
}
