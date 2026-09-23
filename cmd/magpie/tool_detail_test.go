package main

import (
	"testing"

	"github.com/ChamberZ40/magpie/config"
	"github.com/ChamberZ40/magpie/core"
)

// core is stdlib-only and cannot import config, so the three tool detail levels
// are spelled out in both packages. This is the only package that sees both.
func TestToolDetailConstantsAgree(t *testing.T) {
	pairs := []struct {
		name        string
		cfgV, coreV string
	}{
		{"none", config.ToolDetailNone, core.ToolDetailNone},
		{"summary", config.ToolDetailSummary, core.ToolDetailSummary},
		{"full", config.ToolDetailFull, core.ToolDetailFull},
	}
	for _, p := range pairs {
		if p.cfgV != p.coreV {
			t.Errorf("%s: config has %q, core has %q — a config file written against one would be read as unknown by the other",
				p.name, p.cfgV, p.coreV)
		}
	}
}

func TestProjectPinsToolDetail(t *testing.T) {
	level := config.ToolDetailFull
	tests := []struct {
		name string
		proj *config.ProjectConfig
		want bool
	}{
		{"no project at all", nil, false},
		{"project without a display section", &config.ProjectConfig{}, false},
		{"display section that says nothing about tool detail", &config.ProjectConfig{Display: &config.DisplayConfig{}}, false},
		{"project pins the level", &config.ProjectConfig{Display: &config.DisplayConfig{ToolDetail: &level}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectPinsToolDetail(tt.proj); got != tt.want {
				t.Errorf("projectPinsToolDetail = %v, want %v", got, tt.want)
			}
		})
	}
}
