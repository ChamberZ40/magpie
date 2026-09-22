//go:build darwin

package power

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

// caffeinatePath is the absolute path to Apple's assertion helper. It is a
// variable so tests can point it at a stand-in; production never reassigns it.
var caffeinatePath = "/usr/bin/caffeinate"

// Prevent takes a power assertion for the given mode and returns the function
// that releases it. The release function is safe to call more than once.
//
// A failure here is never fatal to the caller: not keeping the machine awake
// degrades the bridge, but refusing to start degrades it further. Callers are
// expected to log the error and carry on.
func Prevent(mode Mode) (release func(), err error) {
	args := caffeinateArgs(mode, os.Getpid())
	if args == nil {
		return func() {}, nil
	}

	cmd := exec.Command(caffeinatePath, args...)
	if err := cmd.Start(); err != nil {
		return func() {}, fmt.Errorf("power: start %s: %w", caffeinatePath, err)
	}

	// Reap the child so it does not linger as a zombie for the lifetime of the
	// daemon. done also lets release block until the process is really gone.
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := cmd.Wait(); err != nil {
			// Expected on release, which kills the process; only worth a line
			// at debug level. An unexpected early exit shows up as the
			// assertion silently disappearing, which pmset -g assertions shows.
			slog.Debug("power: caffeinate exited", "mode", mode, "error", err)
		}
	}()

	slog.Info("power: holding sleep assertion", "mode", mode, "pid", cmd.Process.Pid)

	var once sync.Once
	return func() {
		once.Do(func() {
			if err := cmd.Process.Kill(); err != nil {
				slog.Warn("power: could not stop caffeinate", "pid", cmd.Process.Pid, "error", err)
			}
			<-done
			slog.Info("power: released sleep assertion", "mode", mode)
		})
	}, nil
}
