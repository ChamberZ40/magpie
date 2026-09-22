//go:build !darwin

package power

import "log/slog"

// Prevent is a no-op away from macOS. Linux has systemd-inhibit and Windows has
// SetThreadExecutionState, but nobody is running the daemon there yet; this
// stub keeps the call site platform-free until someone does.
func Prevent(mode Mode) (release func(), err error) {
	if mode != ModeOff {
		slog.Warn("power: prevent_sleep is only implemented on macOS, ignoring", "mode", mode)
	}
	return func() {}, nil
}
