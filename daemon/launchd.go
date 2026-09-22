//go:build darwin

package daemon

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ChamberZ40/magpie/appid"
)

var runLaunchctl = func(args ...string) (string, error) {
	cmd := exec.Command("launchctl", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

type launchdManager struct{}

// CheckLinger is not defined here: check_linger_other.go already provides the
// non-Linux stub under //go:build !linux, which covers darwin. Declaring it in
// this darwin-tagged file too breaks the macOS build with "CheckLinger
// redeclared in this block".

func newPlatformManager() (Manager, error) {
	return &launchdManager{}, nil
}

func (*launchdManager) Platform() string { return "launchd" }

func (m *launchdManager) Install(cfg Config) error {
	plistPath := launchdPlistPath()

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	// Unload existing service first (ignore errors) so we do not leave a stale
	// job behind when switching between GUI and headless sessions.
	bootoutLaunchdTargets()

	plist := buildPlist(cfg)
	// 0600: plist may contain captured secret values (config.toml ${ENV}
	// placeholders and any EnvDiscoverer extension output). User-only
	// LaunchAgents path; root can still read but that is the user's own
	// machine boundary. os.WriteFile only applies perm on create, so
	// Chmod afterwards is required to harden reinstalls of files that
	// pre-existed at 0644 from earlier magpie versions.
	if err := os.WriteFile(plistPath, []byte(plist), 0600); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if err := os.Chmod(plistPath, 0600); err != nil {
		return fmt.Errorf("chmod plist: %w", err)
	}

	domain := preferredLaunchdDomain()
	if out, err := runLaunchctl("bootstrap", domain, plistPath); err != nil {
		return fmt.Errorf("launchctl bootstrap: %s (%w)", out, err)
	}

	if _, err := runLaunchctl("kickstart", "-kp", launchdTarget(domain)); err != nil {
		return fmt.Errorf("launchctl kickstart: %w", err)
	}
	return nil
}

func (m *launchdManager) Uninstall() error {
	bootoutLaunchdTargets()

	plistPath := launchdPlistPath()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

func (*launchdManager) Start() error {
	if _, target, _, ok := loadedLaunchdTarget(); ok {
		out, err := runLaunchctl("kickstart", "-kp", target)
		if err != nil {
			return fmt.Errorf("start: %s (%w)", out, err)
		}
		return nil
	}

	domain := preferredLaunchdDomain()
	plistPath := launchdPlistPath()
	var out string
	if _, err := runLaunchctl("bootstrap", domain, plistPath); err != nil {
		// already bootstrapped — try kickstart
		out, err = runLaunchctl("kickstart", "-kp", launchdTarget(domain))
		if err != nil {
			return fmt.Errorf("start: %s (%w)", out, err)
		}
	}
	return nil
}

func (*launchdManager) Stop() error {
	var lastOut string
	var lastErr error
	for _, target := range launchdTargets() {
		out, err := runLaunchctl("bootout", target)
		if err == nil {
			return nil
		}
		lastOut = out
		lastErr = err
	}
	if lastErr != nil {
		return fmt.Errorf("stop: %s (%w)", lastOut, lastErr)
	}
	return nil
}

func (*launchdManager) Restart() error {
	domain := preferredLaunchdDomain()
	if loadedDomain, _, _, ok := loadedLaunchdTarget(); ok && domain != launchdGUIDomain() {
		domain = loadedDomain
	}
	target := launchdTarget(domain)
	bootoutLaunchdTargets()

	plistPath := launchdPlistPath()

	// launchd bootout is asynchronous; retry bootstrap with backoff
	// to avoid "Bootstrap failed: 5" race condition.
	var out string
	var err error
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		out, err = runLaunchctl("bootstrap", domain, plistPath)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("restart: %s (%w)", out, err)
	}
	if _, err := runLaunchctl("kickstart", "-kp", target); err != nil {
		return fmt.Errorf("restart kickstart: %w", err)
	}
	return nil
}

func (*launchdManager) Status() (*Status, error) {
	st := &Status{Platform: "launchd"}

	plistPath := launchdPlistPath()
	if _, err := os.Stat(plistPath); err != nil {
		return st, nil
	}
	st.Installed = true

	_, _, out, ok := loadedLaunchdTarget()
	if !ok {
		return st, nil
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "pid = ") {
			if pid, err := strconv.Atoi(strings.TrimPrefix(trimmed, "pid = ")); err == nil && pid > 0 {
				st.PID = pid
				st.Running = true
			}
		}
		if strings.Contains(trimmed, "state = running") {
			st.Running = true
		}
	}
	return st, nil
}

// ── helpers ─────────────────────────────────────────────────

func launchdPlistPath() string {
	return launchdPlistPathFor(launchdLabel())
}

func launchdPlistPathFor(label string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

// launchdLabel is the label this machine's service is addressed by.
//
// A service installed before the rename keeps running as com.cc-connect.service
// until it is uninstalled, so status, restart and uninstall have to address it
// by that name — and install has to overwrite that plist in place, rather than
// leave it loaded and add a second service under the new label. Running
// `daemon uninstall` once, then installing again, moves onto appid.ServiceLabel.
func launchdLabel() string {
	if _, err := os.Stat(launchdPlistPathFor(appid.ServiceLabel)); err == nil {
		return appid.ServiceLabel
	}
	if _, err := os.Stat(launchdPlistPathFor(appid.LegacyServiceLabel)); err == nil {
		return appid.LegacyServiceLabel
	}
	return appid.ServiceLabel
}

func launchdUserDomain() string {
	return fmt.Sprintf("user/%d", os.Getuid())
}

func launchdGUIDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func preferredLaunchdDomain() string {
	guiDomain := launchdGUIDomain()
	if _, err := runLaunchctl("print", guiDomain); err == nil {
		return guiDomain
	}
	return launchdUserDomain()
}

func launchdDomains() []string {
	preferred := preferredLaunchdDomain()
	guiDomain := launchdGUIDomain()
	userDomain := launchdUserDomain()
	if preferred == guiDomain {
		return []string{guiDomain, userDomain}
	}
	return []string{userDomain, guiDomain}
}

func launchdTarget(domain string) string {
	return fmt.Sprintf("%s/%s", domain, launchdLabel())
}

func launchdTargets() []string {
	domains := launchdDomains()
	targets := make([]string, 0, len(domains))
	for _, domain := range domains {
		targets = append(targets, launchdTarget(domain))
	}
	return targets
}

func loadedLaunchdTarget() (string, string, string, bool) {
	for _, domain := range launchdDomains() {
		target := launchdTarget(domain)
		out, err := runLaunchctl("print", target)
		if err == nil {
			return domain, target, out, true
		}
	}
	return "", "", "", false
}

func bootoutLaunchdTargets() {
	for _, target := range launchdTargets() {
		_, _ = runLaunchctl("bootout", target)
	}
}

// templateOwnedEnvKeys are keys the plist template renders directly; if
// they also appear in cfg.EnvExtra the template version wins.
//
// The pre-rename names are listed too. They are still honored on read, so
// letting one through from a captured environment would quietly override the
// managed value — and it would emit a duplicate <key> into the plist.
var templateOwnedEnvKeys = map[string]struct{}{
	appid.EnvName("LOG_FILE"):        {},
	appid.EnvName("LOG_MAX_SIZE"):    {},
	appid.EnvName("LOG_MAX_BACKUPS"): {},
	"CC_LOG_FILE":                    {},
	"CC_LOG_MAX_SIZE":                {},
	"CC_LOG_MAX_BACKUPS":             {},
	"PATH":                           {},
}

// renderEnvExtraPlist returns the serialized key/value pairs (without the
// surrounding <dict> wrapper) for cfg.EnvExtra, sorted by key and with
// invalid keys / empty values dropped. Both keys and values are XML-escaped.
func renderEnvExtraPlist(envExtra map[string]string) string {
	if len(envExtra) == 0 {
		return ""
	}
	keys := make([]string, 0, len(envExtra))
	for k := range envExtra {
		if _, owned := templateOwnedEnvKeys[k]; owned {
			continue
		}
		if !isValidEnvName(k) {
			slog.Warn("daemon: launchd: dropping invalid env name from EnvExtra",
				"key", k)
			continue
		}
		if envExtra[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "\t\t<key>%s</key>\n\t\t<string>%s</string>\n",
			xmlEscape(k), xmlEscape(envExtra[k]))
	}
	return b.String()
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func buildPlist(cfg Config) string {
	envPATH := cfg.EnvPATH
	if envPATH == "" {
		envPATH = "/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin"
	}
	envExtra := renderEnvExtraPlist(cfg.EnvExtra)
	// User-supplied paths can legitimately contain XML-special characters
	// ('&', '<', '>', '"', '\''). Without escaping, `launchctl bootstrap`
	// rejects the plist with a parse error and daemon install fails. The
	// label is a hard-coded constant; LogMaxSize is an int; envExtra is
	// escaped by renderEnvExtraPlist.
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
	<key>LimitLoadToSessionType</key>
	<array>
		<string>Aqua</string>
		<string>Background</string>
	</array>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>EnvironmentVariables</key>
	<dict>
		<key>MAGPIE_LOG_FILE</key>
		<string>%s</string>
		<key>MAGPIE_LOG_MAX_SIZE</key>
		<string>%d</string>
		<key>MAGPIE_LOG_MAX_BACKUPS</key>
		<string>%d</string>
		<key>PATH</key>
		<string>%s</string>
%s	</dict>
	<key>StandardOutPath</key>
	<string>/dev/null</string>
	<key>StandardErrorPath</key>
	<string>/dev/null</string>
</dict>
</plist>
`, launchdLabel(), xmlEscape(cfg.BinaryPath), xmlEscape(cfg.WorkDir), xmlEscape(cfg.LogFile), cfg.LogMaxSize, cfg.LogMaxBackups, xmlEscape(envPATH), envExtra)
}
