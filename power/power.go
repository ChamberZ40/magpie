// Package power holds the process's claim on keeping the machine awake.
//
// A chat bridge is only reachable while the host is awake: once macOS idle
// sleeps, the platform websocket drops and messages are not delivered until
// something wakes the machine back up. On a laptop with the stock power
// settings that happens within minutes of the user walking away, which makes
// the bridge look broken.
//
// macOS models this as a refcount of power assertions — reasons not to sleep —
// that any process may add to and that the kernel drops when the process goes
// away. Holding one for the lifetime of the daemon is therefore strictly better
// than editing pmset: a global pmset change outlives a crash, and a Mac that
// never sleeps again is a far worse failure than one that sleeps too eagerly.
//
// The assertion is taken by running Apple's /usr/bin/caffeinate rather than by
// calling IOPMAssertionCreateWithName directly, because the latter needs cgo
// and the release build cross-compiles six platforms with CGO_ENABLED=0.
//
// This package is a leaf: it imports only the standard library, so config and
// cmd can both depend on it without coupling to each other.
package power

import (
	"fmt"
	"strconv"
	"strings"
)

// Mode is how aggressively the process should keep the machine awake.
type Mode string

const (
	// ModeOff takes no assertion; the machine sleeps on its own schedule.
	ModeOff Mode = "off"
	// ModeAlways keeps the machine awake on any power source.
	ModeAlways Mode = "always"
	// ModeACOnly keeps the machine awake only while it is plugged in. The
	// kernel enforces the power-source half of this, not us.
	ModeACOnly Mode = "ac_only"
)

// ParseMode validates a configured prevent_sleep value. An empty string means
// the operator did not configure one, which is ModeOff.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(ModeOff):
		return ModeOff, nil
	case string(ModeAlways):
		return ModeAlways, nil
	case string(ModeACOnly):
		return ModeACOnly, nil
	default:
		return "", fmt.Errorf("power: unknown prevent_sleep mode %q (want %q, %q, or %q)",
			s, ModeOff, ModeAlways, ModeACOnly)
	}
}

// caffeinateArgs builds the argv for a mode, or nil when no process is needed.
//
// The flags come straight from caffeinate(8):
//
//	-i  prevent idle sleep, honored on battery and on AC
//	-s  prevent system sleep, "valid only when system is running on AC power"
//	-w  release the assertion once the given pid exits
//
// ModeACOnly is therefore just -s: the kernel already ignores that assertion on
// battery, so we never have to read or poll the power source ourselves.
//
// pid is this process. Passing it is what makes a stuck assertion impossible:
// without -w, a caffeinate orphaned by `kill -9` would be reparented and go on
// holding the assertion until reboot, and nothing would point at the cause.
func caffeinateArgs(mode Mode, pid int) []string {
	switch mode {
	case ModeAlways:
		return []string{"-i", "-s", "-w", strconv.Itoa(pid)}
	case ModeACOnly:
		return []string{"-s", "-w", strconv.Itoa(pid)}
	default:
		return nil
	}
}
