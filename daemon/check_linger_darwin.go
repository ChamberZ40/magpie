package daemon

// CheckLinger is a stub on macOS: launchd has no equivalent of systemd's
// linger, so callers skip the warning. Returns (true, "").
//
// The _darwin suffix is what scopes this file. It used to be tagged !linux,
// which overlapped with the windows and unsupported declarations and broke
// every GOOS except linux and darwin. The four declarations now partition
// cleanly: systemd.go (linux), this file (darwin), windows.go (windows),
// unsupported.go (everything else).
func CheckLinger() (enabled bool, user string) {
	return true, ""
}
