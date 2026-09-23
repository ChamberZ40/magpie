package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tool_messages was a boolean: either every tool call dumped its status, exit
// code and raw output into the card, or tool calls vanished entirely. The
// middle rung — name plus a one-line summary — is the one people actually want,
// and it is now the default.

func strPtr(s string) *string { return &s }

func TestEffectiveToolDetail(t *testing.T) {
	tests := []struct {
		name   string
		global *DisplayConfig
		proj   *DisplayConfig
		want   string
	}{
		{
			name:   "nothing configured defaults to summary",
			global: &DisplayConfig{},
			want:   ToolDetailSummary,
		},
		{
			name:   "compact mode derives none",
			global: &DisplayConfig{Mode: strPtr(DisplayModeCompact)},
			want:   ToolDetailNone,
		},
		{
			name:   "quiet mode derives none",
			global: &DisplayConfig{Mode: strPtr(DisplayModeQuiet)},
			want:   ToolDetailNone,
		},
		{
			// An explicit value beats the mode-derived default, exactly as
			// tool_messages did.
			name:   "explicit value overrides the mode default",
			global: &DisplayConfig{Mode: strPtr(DisplayModeQuiet), ToolDetail: strPtr(ToolDetailFull)},
			want:   ToolDetailFull,
		},
		{
			name:   "project overrides global",
			global: &DisplayConfig{ToolDetail: strPtr(ToolDetailFull)},
			proj:   &DisplayConfig{ToolDetail: strPtr(ToolDetailNone)},
			want:   ToolDetailNone,
		},
		{
			name:   "global applies when the project says nothing",
			global: &DisplayConfig{ToolDetail: strPtr(ToolDetailFull)},
			proj:   &DisplayConfig{},
			want:   ToolDetailFull,
		},
		{
			// Mirrors EffectiveCardMode: a typo must not silently disable the
			// tool panel, it falls back to the default.
			name:   "an unknown value falls back to summary",
			global: &DisplayConfig{ToolDetail: strPtr("verbose")},
			want:   ToolDetailSummary,
		},
		{
			name:   "case and padding are tolerated",
			global: &DisplayConfig{ToolDetail: strPtr("  FULL ")},
			want:   ToolDetailFull,
		},
		{
			name:   "an unknown project value falls back to the global one",
			global: &DisplayConfig{ToolDetail: strPtr(ToolDetailNone)},
			proj:   &DisplayConfig{ToolDetail: strPtr("loud")},
			want:   ToolDetailNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Display: *tt.global}
			proj := &ProjectConfig{Name: "demo", Display: tt.proj}
			_, _, got, _, _, _, _, _ := EffectiveDisplay(cfg, proj)
			if got != tt.want {
				t.Errorf("tool detail = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSaveDisplayConfigWritesToolDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[display]\nmode = \"full\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	prev := ConfigPath
	ConfigPath = path
	t.Cleanup(func() { ConfigPath = prev })

	if err := SaveDisplayConfig(nil, nil, nil, nil, strPtr(ToolDetailNone)); err != nil {
		t.Fatalf("SaveDisplayConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), `tool_detail = "none"`) {
		t.Fatalf("expected tool_detail to be persisted, got:\n%s", got)
	}
}
