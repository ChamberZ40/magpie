package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ChamberZ40/magpie/core"
)

// displayNameOfAgent maps a registry name to the name users know the tool by.
// Only entries that differ from the registry name need to be listed; anything
// missing falls through to the registry name itself, so an agent added later
// still shows up in the banner without touching this table.
var displayNameOfAgent = map[string]string{
	"claudecode": "Claude Code",
	"codex":      "Codex",
	"cursor":     "Cursor",
	"copilot":    "GitHub Copilot",
	"acp":        "ACP",
}

var displayNameOfPlatform = map[string]string{
	"feishu": "Feishu/Lark",
	// Registered as its own name, but the same platform as feishu; mapping
	// both to one label lets describeRegistered collapse them.
	"lark":   "Feishu/Lark",
	"wecom":  "WeChat Work",
	"weixin": "Weixin (personal)",
}

// describeRegistered renders registry names as a human-readable list for the
// banner, sorted so the output is stable rather than following Go's map
// iteration order. Names sharing a display label (feishu and lark) collapse
// into one entry.
//
// It takes the names as an argument instead of calling the registry itself so
// the formatting can be tested without depending on which plugins this binary
// was built with.
func describeRegistered(names []string, display map[string]string) string {
	if len(names) == 0 {
		// A build that excludes everything is legal (make build AGENTS=...),
		// and a bare "Supports:" with nothing after it reads like a bug.
		return "(none compiled in)"
	}
	seen := make(map[string]bool, len(names))
	labels := make([]string, 0, len(names))
	for _, n := range names {
		label := n
		if d, ok := display[n]; ok {
			label = d
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return strings.Join(labels, ", ")
}

// usageText builds the --help output. It is separate from printUsage so tests
// can assert on it without capturing stderr.
func usageText() string {
	v := version
	if v == "" || v == "dev" {
		v = "dev"
	}

	// Both lines are derived from the registry rather than hand-written: the
	// hand-written ones drifted badly, advertising platforms this project has
	// never had while omitting two agents it does.
	agents := describeRegistered(core.ListRegisteredAgents(), displayNameOfAgent)
	platforms := describeRegistered(core.ListRegisteredPlatforms(), displayNameOfPlatform)

	return fmt.Sprintf(`
                                 _
 _ __ ___    __ _   __ _  _ __  (_)  ___
| '_ ' _ \  / _' | / _' || '_ \ | | / _ \
| | | | | || (_| || (_| || |_) || ||  __/
|_| |_| |_| \__,_| \__, || .__/ |_| \___|
                   |___/ |_|               %s

  Bridge your messaging platforms to local AI coding agents.
  Agents:    %s
  Platforms: %s

  GitHub:  https://github.com/ChamberZ40/magpie
  Docs:    https://github.com/ChamberZ40/magpie/blob/main/INSTALL.md

Usage:
  magpie [flags]
  magpie <command> [args]

Flags:
  --config <path>    Path to config file (default: ./config.toml or ~/.magpie/config.toml)
  --force            Kill any existing instance with the same config before starting
  --version          Print version and exit
  --help             Show this help message

Commands:
  daemon             Manage magpie as a background service (systemd/launchd/schtasks)
    install          Install and start the daemon service
    uninstall        Remove the daemon service
    start            Start the daemon
    stop             Stop the daemon
    restart          Restart the daemon
    status           Show daemon status
    logs             View daemon logs (-f to follow, -n N for last N lines)

  web                Configure the web admin and open the dashboard
                     (does not start the bridge — run 'magpie' for that)

  doctor             Diagnose the local setup
    runas            Check the run_as_user isolation prerequisites

  send               Send a message to an active session via internal API
                     (-m <text> | --stdin, -p <project>, -s <session>)

  cron               Manage scheduled tasks
    add              Create a scheduled task (-c <expr> --prompt <text>)
    list             List scheduled tasks
    info             Show one task in detail
    edit             Change a single field of a task
    enabled          Enable or disable a task
    exec             Trigger a scheduled task immediately
    run              Run a task's command without scheduling it
    del              Delete a scheduled task by ID

  timer              Manage one-off timers
    add              Schedule a one-off reminder
    list             List pending timers
    del              Cancel a pending timer

  at                 Alias for 'timer'

  sessions           Browse session history
    list             List all sessions (pipe-friendly)
    show <id>        Show session messages (-n N for last N)
    prune            Delete old sessions

  agent-sid          Print the agent session ID for the current session

  relay              Cross-project message relay
    send             Send a message to another project and get the response

  provider           Manage API providers for projects
    add              Add a provider (--project, --name, --api-key, ...)
    list             List providers (--project)
    model            Show or switch the active model
    presets          List the built-in provider presets
    global           Manage providers shared by every project
    remove           Remove a provider (--project, --name)
    import           Import providers from cc-switch

  feishu             Setup Feishu/Lark bot credentials
    setup            Smart setup (QR create or bind when --app is provided)
    new              Force QR onboarding to create a new bot
    bind             Bind existing app_id/app_secret

  weixin             Setup Weixin personal (ilink) via QR or token
    setup            QR login, or bind when --token is provided
    new              Force QR login
    bind             Bind existing ilink bot token

  config             Manage configuration
    example          Print a complete annotated config.toml example
    format           Format the config file (alias: fmt)
    path             Print the resolved config file path

  config-example     (deprecated: use 'config example' instead)

Examples:
  magpie                          Start with default config
  magpie --config /path/to.toml   Start with a specific config file
  magpie daemon install           Install as a system service
  magpie daemon logs -f           Follow daemon logs
  magpie doctor runas             Check run_as_user prerequisites
  magpie send -m "hello"          Send a message to the active session
  magpie cron list                List all scheduled tasks
  magpie feishu setup             Setup Feishu/Lark bot credentials
  magpie weixin setup             Setup Weixin (ilink) with QR or --token
  magpie config format            Format the config file
  magpie config example > c.toml  Save example config to a file
`, v, agents, platforms)
}
