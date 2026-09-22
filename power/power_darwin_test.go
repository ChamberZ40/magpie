//go:build darwin

package power

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// stubCaffeinate points caffeinatePath at a script that ignores its arguments
// and sleeps, so the lifecycle can be exercised without taking a real
// system-wide power assertion during `go test`.
//
// The script records its own pid and then execs, which keeps that pid: the
// process Prevent kills is the one the test watches.
func stubCaffeinate(t *testing.T) (pidFile string) {
	t.Helper()

	dir := t.TempDir()
	pidFile = filepath.Join(dir, "pid")
	path := filepath.Join(dir, "caffeinate")

	script := "#!/bin/sh\necho $$ > " + pidFile + "\nexec sleep 300\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	prev := caffeinatePath
	caffeinatePath = path
	t.Cleanup(func() { caffeinatePath = prev })

	return pidFile
}

// waitForPIDFile gives the stub a moment to start and report its pid.
func waitForPIDFile(t *testing.T, pidFile string) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(pidFile)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("stub never wrote %s", pidFile)
	return 0
}

// alive reports whether pid is still a live process. Signal 0 performs the
// permission and existence checks without delivering anything.
func alive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func TestPrevent_OffSpawnsNothing(t *testing.T) {
	pidFile := stubCaffeinate(t)

	release, err := Prevent(ModeOff)
	if err != nil {
		t.Fatalf("Prevent(off): %v", err)
	}
	release()

	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(pidFile); err == nil {
		t.Fatal("off mode started a process")
	}
}

func TestPrevent_ReleaseStopsTheProcess(t *testing.T) {
	pidFile := stubCaffeinate(t)

	release, err := Prevent(ModeAlways)
	if err != nil {
		t.Fatalf("Prevent(always): %v", err)
	}

	pid := waitForPIDFile(t, pidFile)
	if !alive(pid) {
		t.Fatalf("stub pid %d not running", pid)
	}

	release()

	// release blocks on Wait, so the process must already be reaped.
	if alive(pid) {
		t.Fatalf("pid %d still alive after release", pid)
	}
}

func TestPrevent_ReleaseIsIdempotent(t *testing.T) {
	pidFile := stubCaffeinate(t)

	release, err := Prevent(ModeACOnly)
	if err != nil {
		t.Fatalf("Prevent(ac_only): %v", err)
	}
	waitForPIDFile(t, pidFile)

	release()
	release() // must not panic or block on the already-closed channel
}

func TestPrevent_MissingBinaryIsNotFatal(t *testing.T) {
	prev := caffeinatePath
	caffeinatePath = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { caffeinatePath = prev })

	release, err := Prevent(ModeAlways)
	if err == nil {
		t.Fatal("Prevent with a missing binary returned no error")
	}
	if release == nil {
		t.Fatal("Prevent returned a nil release on failure; callers always call it")
	}
	release() // must be safe even though nothing started
}

// A stub cannot catch a typo'd or removed caffeinate option, so take a genuine
// assertion briefly and confirm the real binary accepted our argv.
func TestPrevent_RealCaffeinateAcceptsOurFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("takes a real system power assertion")
	}
	if _, err := os.Stat(caffeinatePath); err != nil {
		t.Skipf("caffeinate unavailable: %v", err)
	}

	release, err := Prevent(ModeAlways)
	if err != nil {
		t.Fatalf("Prevent(always): %v", err)
	}
	defer release()

	// caffeinate rejects bad flags immediately, so a process still running a
	// moment later means the argv parsed.
	time.Sleep(300 * time.Millisecond)

	pattern := "caffeinate -i -s -w " + strconv.Itoa(os.Getpid())
	if err := exec.Command("/usr/bin/pgrep", "-f", pattern).Run(); err != nil {
		t.Fatalf("real caffeinate not running as %q: %v", pattern, err)
	}
}
