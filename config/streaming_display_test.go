package config

import "testing"

// The rich-card streaming path pushed a frame every 200ms / 20 chars, as bare
// literals in core/engine.go. Those numbers are a network-conditions tradeoff,
// not a constant: they have to be tunable per deployment, and they have to move
// in step with the Feishu client's print_frequency_ms (see docs/feishu.md —
// pushing slower than the client prints makes the client setting inert).
//
// Resolution mirrors the rest of [display]: project overrides global, per
// field, and anything unset falls back to the historical literal.

func intPtr(v int) *int { return &v }

func TestEffectiveStreaming_DefaultsMatchThePreviousLiterals(t *testing.T) {
	got := EffectiveStreaming(&Config{}, nil)

	want := StreamingDisplay{
		ThrottleMS:            DefaultStreamThrottleMS,
		ThrottleChars:         DefaultStreamThrottleChars,
		FallbackThrottleMS:    DefaultStreamFallbackThrottleMS,
		FallbackThrottleChars: DefaultStreamFallbackThrottleChars,
	}
	if got != want {
		t.Errorf("EffectiveStreaming() = %+v, want %+v", got, want)
	}
	// Guard the values themselves: silently changing them changes how often
	// every deployment hits the platform's rate limiter.
	if want.ThrottleMS != 200 || want.ThrottleChars != 20 ||
		want.FallbackThrottleMS != 1500 || want.FallbackThrottleChars != 30 {
		t.Errorf("defaults drifted from the pre-config literals: %+v", want)
	}
}

func TestEffectiveStreaming_ProjectOverridesGlobalPerField(t *testing.T) {
	cfg := &Config{Display: DisplayConfig{Streaming: &StreamingDisplayConfig{
		ThrottleMS:    intPtr(60),
		ThrottleChars: intPtr(5),
	}}}
	proj := &ProjectConfig{Display: &DisplayConfig{Streaming: &StreamingDisplayConfig{
		ThrottleMS: intPtr(90),
	}}}

	got := EffectiveStreaming(cfg, proj)

	if got.ThrottleMS != 90 {
		t.Errorf("ThrottleMS = %d, want 90 (project wins)", got.ThrottleMS)
	}
	if got.ThrottleChars != 5 {
		t.Errorf("ThrottleChars = %d, want 5 (inherited from global, not reset by the project block)", got.ThrottleChars)
	}
	if got.FallbackThrottleMS != DefaultStreamFallbackThrottleMS {
		t.Errorf("FallbackThrottleMS = %d, want the default %d", got.FallbackThrottleMS, DefaultStreamFallbackThrottleMS)
	}
}

func TestValidateDisplayConfig_RejectsStreamingValuesThatDefeatThrottling(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfg     *StreamingDisplayConfig
		wantErr bool
	}{
		{"throttle_ms below the floor", &StreamingDisplayConfig{ThrottleMS: intPtr(5)}, true},
		{"throttle_ms at the floor", &StreamingDisplayConfig{ThrottleMS: intPtr(MinStreamThrottleMS)}, false},
		// The engine pushes when EITHER the interval OR the character delta is
		// exceeded, so throttle_chars = 0 makes every single token pass the
		// check and the interval never gets a say. That is precisely the
		// unthrottled flood the interval exists to prevent.
		{"throttle_chars zero", &StreamingDisplayConfig{ThrottleChars: intPtr(0)}, true},
		{"throttle_chars one", &StreamingDisplayConfig{ThrottleChars: intPtr(1)}, false},
		{"negative fallback interval", &StreamingDisplayConfig{FallbackThrottleMS: intPtr(-1)}, true},
		{"nothing set", &StreamingDisplayConfig{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDisplayConfig("display", &DisplayConfig{Streaming: tc.cfg})
			if tc.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
