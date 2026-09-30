package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ChamberZ40/magpie/config"
)

// agentChoice is one agent type `magpie init` can offer.
type agentChoice struct {
	Type   string
	Label  string
	Binary string
	Found  bool
}

// initAgents lists the agents with a fixed CLI to look for, in the order they
// are offered. acp is absent on purpose: which binary it runs is the user's
// choice, so there is nothing to detect.
var initAgents = []agentChoice{
	{Type: "claudecode", Label: "Claude Code", Binary: "claude"},
	{Type: "codex", Label: "Codex", Binary: "codex"},
	{Type: "cursor", Label: "Cursor Agent", Binary: "agent"},
	{Type: "copilot", Label: "GitHub Copilot CLI", Binary: "copilot"},
}

func detectAgents(registered []string, lookPath func(string) (string, error)) []agentChoice {
	inBuild := make(map[string]bool, len(registered))
	for _, r := range registered {
		inBuild[r] = true
	}
	var out []agentChoice
	for _, a := range initAgents {
		if !inBuild[a.Type] {
			continue
		}
		_, err := lookPath(a.Binary)
		out = append(out, agentChoice{Type: a.Type, Label: a.Label, Binary: a.Binary, Found: err == nil})
	}
	return out
}

// defaultAgentIndex prefers the configured agent when it is installed, then
// the first installed one, then the first offered.
func defaultAgentIndex(agents []agentChoice, configured string) int {
	first := -1
	for i, a := range agents {
		if !a.Found {
			continue
		}
		if a.Type == configured {
			return i
		}
		if first < 0 {
			first = i
		}
	}
	if first >= 0 {
		return first
	}
	return 0
}

// resolveWorkDir expands ~ and requires an existing directory: agents refuse
// to start in one that is missing.
func resolveWorkDir(raw string) (string, error) {
	dir := strings.TrimSpace(raw)
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve ~: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", raw, err)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s does not exist", abs)
	}
	if err != nil {
		return "", fmt.Errorf("check %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}

// initSetting is one recommended option `magpie init` offers, on by default.
type initSetting struct {
	label      string
	feishuOnly bool
	darwinOnly bool
	apply      func(project string, on bool) error
}

func sectionSetting(label, section, key, onRaw, offRaw string) initSetting {
	return initSetting{label: label, apply: func(_ string, on bool) error {
		raw := offRaw
		if on {
			raw = onRaw
		}
		return config.SaveSectionValue(section, key, raw)
	}}
}

func feishuSetting(label, platformType string, on, off []config.OptionValue) initSetting {
	return initSetting{label: label, feishuOnly: true, apply: func(project string, enable bool) error {
		values := off
		if enable {
			values = on
		}
		return config.SavePlatformOptions(project, platformType, values)
	}}
}

// recommendedSettings returns the options worth turning on for a new user,
// limited to the chat apps the project has and the OS it runs on.
func recommendedSettings(platformTypes []string, goos string) []initSetting {
	var settings []initSetting
	if goos == "darwin" {
		s := sectionSetting("Keep this Mac awake while it is plugged in, so the bot stays reachable",
			"power", "prevent_sleep", `"ac_only"`, `"off"`)
		s.darwinOnly = true
		settings = append(settings, s)
	}
	settings = append(settings,
		sectionSetting("Show replies as rich cards", "display", "card_mode", `"rich"`, `"legacy"`),
		sectionSetting("Show full tool details (commands, file paths)", "display", "tool_detail", `"full"`, `"summary"`),
	)

	feishuType := ""
	for _, t := range platformTypes {
		if t == "feishu" || t == "lark" {
			feishuType = t
			break
		}
	}
	if feishuType != "" {
		settings = append(settings,
			feishuSetting("Feishu: give each group-chat thread its own session", feishuType,
				[]config.OptionValue{{Key: "thread_isolation", Raw: "true"}},
				[]config.OptionValue{{Key: "thread_isolation", Raw: "false"}}),
			feishuSetting("Feishu: show progress in one updating card", feishuType,
				[]config.OptionValue{{Key: "progress_style", Raw: `"card"`}},
				[]config.OptionValue{{Key: "progress_style", Raw: `"legacy"`}}),
			feishuSetting("Feishu: react ✅ to your message when the agent is done", feishuType,
				[]config.OptionValue{{Key: "done_emoji", Raw: `"Done"`}},
				[]config.OptionValue{{Key: "done_emoji", Raw: `"none"`}}),
		)
	}
	return settings
}
