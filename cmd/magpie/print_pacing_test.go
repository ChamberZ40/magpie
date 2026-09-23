package main

import (
	"strings"
	"testing"

	"github.com/ChamberZ40/magpie/config"
)

// Pushing frames slower than the client is told to print them is a silent
// failure: the config is valid, both values are applied, and the on-screen
// result is identical to not having set card_print_frequency_ms at all. The
// user is left tuning a knob that cannot move. Say so at startup instead.

func projWithPrintFrequency(ms any) *config.ProjectConfig {
	return &config.ProjectConfig{
		Name: "demo",
		Platforms: []config.PlatformConfig{
			{Type: "feishu", Options: map[string]any{"card_print_frequency_ms": ms}},
		},
	}
}

func TestClientPrintPacingWarning(t *testing.T) {
	for _, tc := range []struct {
		name      string
		proj      *config.ProjectConfig
		throttle  int
		wantWarn  bool
		mustNamed []string
	}{
		{
			name:      "server pushes slower than the client prints",
			proj:      projWithPrintFrequency(int64(50)),
			throttle:  200,
			wantWarn:  true,
			mustNamed: []string{"demo", "200", "50"},
		},
		{
			name:     "server pushes faster, the client setting can work",
			proj:     projWithPrintFrequency(int64(50)),
			throttle: 40,
			wantWarn: false,
		},
		{
			name:     "equal pacing is fine",
			proj:     projWithPrintFrequency(int64(50)),
			throttle: 50,
			wantWarn: false,
		},
		{
			name:     "no client pacing configured, nothing to contradict",
			proj:     &config.ProjectConfig{Name: "demo", Platforms: []config.PlatformConfig{{Type: "feishu"}}},
			throttle: 200,
			wantWarn: false,
		},
		{
			name:     "unparseable value is not our error to report here",
			proj:     projWithPrintFrequency("fast"),
			throttle: 200,
			wantWarn: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := clientPrintPacingWarning(tc.proj, config.StreamingDisplay{ThrottleMS: tc.throttle})

			if tc.wantWarn && got == "" {
				t.Fatal("expected a warning, got none")
			}
			if !tc.wantWarn && got != "" {
				t.Fatalf("expected no warning, got %q", got)
			}
			for _, want := range tc.mustNamed {
				if !strings.Contains(got, want) {
					t.Errorf("warning should name %q so the user knows what to change; got %q", want, got)
				}
			}
		})
	}
}
