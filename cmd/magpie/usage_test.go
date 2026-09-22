package main

import (
	"strings"
	"testing"
)

// The banner used to be a hand-written string listing agents and platforms.
// It drifted: it advertised Telegram, Slack, Discord, DingTalk, LINE, QQ and
// three agents that are not in this build, while omitting Copilot and ACP that
// are. Deriving it from the registry makes that drift impossible, and makes a
// selective build (make build AGENTS=claudecode) describe itself correctly.
func TestBannerLinesComeFromTheRegistry(t *testing.T) {
	got := describeRegistered([]string{"codex", "claudecode", "acp"}, displayNameOfAgent)

	// Sorted by display name, so the banner is stable across runs rather than
	// reshuffling with Go's map iteration order.
	want := "ACP, Claude Code, Codex"
	if got != want {
		t.Errorf("describeRegistered = %q, want %q", got, want)
	}
}

func TestDescribeRegisteredHandlesEmptyBuild(t *testing.T) {
	// Every agent excluded at build time is unusual but legal; the banner must
	// not print a dangling label with nothing after it.
	if got := describeRegistered(nil, displayNameOfAgent); got != "(none compiled in)" {
		t.Errorf("describeRegistered(nil) = %q, want the none placeholder", got)
	}
}

// An agent or platform added later without a display-name entry should still
// appear, under its registry name, rather than silently vanishing.
func TestDescribeRegisteredFallsBackToTheRegistryName(t *testing.T) {
	got := describeRegistered([]string{"brandnew"}, displayNameOfAgent)
	if got != "brandnew" {
		t.Errorf("describeRegistered = %q, want the raw registry name", got)
	}
}

// "lark" and "feishu" are two registry entries for one platform. Listing the
// display name once is the point of mapping them to the same label — reading
// "Feishu/Lark, ..., lark" makes it look like two separate integrations.
func TestDescribeRegisteredCollapsesAliases(t *testing.T) {
	got := describeRegistered([]string{"feishu", "lark", "wecom"}, displayNameOfPlatform)

	want := "Feishu/Lark, WeChat Work"
	if got != want {
		t.Errorf("describeRegistered = %q, want %q", got, want)
	}
}

func TestUsageListsEveryRegisteredCommand(t *testing.T) {
	usage := usageText()
	for name := range topLevelCommandHandlers {
		// config-example is deliberately absent: it is the deprecated spelling
		// and the usage text documents it separately.
		if name == "config-example" {
			continue
		}
		if !strings.Contains(usage, "\n  "+name+" ") {
			t.Errorf("usage text does not document the %q command", name)
		}
	}
}

// The inverse direction: the text must not promise commands that do not exist.
// It used to list "feishu download", which no code path handled.
func TestUsageDoesNotPromiseMissingCommands(t *testing.T) {
	if strings.Contains(usageText(), "download") {
		t.Error("usage text still mentions the feishu download command, which does not exist")
	}
}
