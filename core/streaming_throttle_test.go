package core

import (
	"testing"
	"time"
)

// A zero DisplayCfg must never mean "no throttling". Several call sites build
// a DisplayCfg by listing only the fields they care about, and an unset
// interval that literally meant 0ms would push a frame per token straight into
// the platform's rate limiter. SetDisplayConfig fills the gaps instead.
func TestSetDisplayConfig_ZeroStreamingFallsBackToDefaults(t *testing.T) {
	e := &Engine{}
	e.SetDisplayConfig(DisplayCfg{Mode: "full"})

	dur, chars := e.streamThrottle(true)
	if dur != DefaultStreamThrottle {
		t.Errorf("interval = %v, want the default %v — an unset config must not disable throttling", dur, DefaultStreamThrottle)
	}
	if chars != DefaultStreamThrottleChars {
		t.Errorf("chars = %d, want the default %d", chars, DefaultStreamThrottleChars)
	}
}

func TestStreamThrottle_UsesConfiguredValues(t *testing.T) {
	e := &Engine{}
	e.SetDisplayConfig(DisplayCfg{Streaming: StreamingCfg{
		Throttle:         60 * time.Millisecond,
		ThrottleChars:    5,
		FallbackThrottle: 900 * time.Millisecond,
		FallbackChars:    25,
	}})

	t.Run("platform streams elements", func(t *testing.T) {
		dur, chars := e.streamThrottle(true)
		if dur != 60*time.Millisecond || chars != 5 {
			t.Errorf("got (%v, %d), want (60ms, 5)", dur, chars)
		}
	})

	// Without element streaming every push rewrites the whole card, which is
	// far more expensive, so the fallback pair is deliberately slower.
	t.Run("platform needs full-card patches", func(t *testing.T) {
		dur, chars := e.streamThrottle(false)
		if dur != 900*time.Millisecond || chars != 25 {
			t.Errorf("got (%v, %d), want (900ms, 25)", dur, chars)
		}
	})
}
