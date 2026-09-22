<p align="center">
  <img src="./docs/images/banner.svg" alt="Magpie Banner" width="800"/>
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README.zh-CN.md">中文</a>
</p>

**Magpie puts your AI coding agent in your chat app.** You send a message in
Feishu or WeChat; the agent runs locally, in your own project directory, with
your own credentials; its output comes back to the chat. Nothing about your code
leaves your machine except the text you'd have typed into the agent anyway.

The point is untethering. The agent keeps working on the long task while you
walk away from the desk, and you steer it from your phone.

<p align="center">
  <img src="docs/images/connector.png" alt="Magpie Architecture" width="90%"/>
</p>


## Quick start

Assumes Go 1.25+ and Node.js (the build embeds a Web UI), and an agent CLI that
is already installed **and logged in** — see [Install](#install) for both.

```bash
git clone https://github.com/ChamberZ40/magpie.git
cd magpie
make build                 # produces ./magpie
./magpie                   # writes ~/.magpie/config.toml, prints http://localhost:9820
```

Open that URL, create a project, paste your bot credentials, save. Magpie
hot-reloads. Message the bot to confirm the round trip works.


## What's included

| Agent | Config |
|-------|--------|
| Claude Code | `type = "claudecode"` |
| Codex (OpenAI) | `type = "codex"` |
| Cursor Agent | `type = "cursor"` |
| GitHub Copilot CLI | `type = "copilot"` |
| ACP | `type = "acp"` — any [ACP-compatible agent](https://agentclientprotocol.com/get-started/agents), e.g. OpenClaw or Hermes |

| Platform | Connection | Public IP needed? | Setup guide |
|----------|------------|-------------------|-------------|
| Feishu (Lark) | WebSocket | No | [docs/feishu.md](docs/feishu.md) |
| WeChat Work | WebSocket / Webhook | No (WS) / Yes (Webhook) | [docs/wecom.md](docs/wecom.md) |
| Weixin (personal, ilink) | HTTP long polling | No | [docs/weixin.md](docs/weixin.md) |

Per-platform capabilities:

| Capability | Feishu | WeCom | Weixin *(personal)* |
|------------|:------:|:-----:|:-------------------:|
| Text & slash commands | ✅ | ✅ | ✅ |
| Markdown / cards | ✅ | ⚠️ | ✅ |
| Streaming / chunked replies | ✅ | ✅ | ✅ |
| Images & files | ✅ | ✅ | ✅ |
| Voice / STT / TTS | ⚠️ | ⚠️ | ✅ |
| Private (DM) | ✅ | ✅ | ✅ |
| Group / channel | ✅ | ✅ | ✅ |

⚠️ means partial, or needs extra configuration — the voice row in particular
requires `[speech]` / TTS providers in `config.toml`.


## Install

### 1. Install an agent CLI — first

Magpie is a bridge to *local* agent CLIs, so the agent has to exist before
Magpie starts. Skip this and Magpie exits with `claudecode: claude CLI not
found in PATH` (or the equivalent), and the Web UI never comes up.

```bash
# Claude Code
brew install --cask claude-code            # macOS / Linux Homebrew
npm install -g @anthropic-ai/claude-code   # or, any platform via npm

# OpenAI Codex
npm install -g @openai/codex

# GitHub Copilot CLI
npm install -g @github/copilot
```

Cursor Agent: follow <https://docs.cursor.com/agent>. Any other ACP-speaking
agent is configured as `type = "acp"`.

```bash
claude --version       # or: codex / copilot / cursor-agent
```

### 2. Authenticate it

Run the agent once interactively so it stores credentials in your home
directory:

```bash
claude login           # opens a browser
codex login            # or: copilot / cursor-agent — see the agent's docs
```

Skip this and Magpie still starts, but the agent rejects every prompt with an
auth error.

### 3. Install Magpie

**From source** — the canonical path. Needs Go 1.25+ and Node.js, because
`make build` also rebuilds the embedded Web UI.

```bash
git clone https://github.com/ChamberZ40/magpie.git
cd magpie
make build                 # produces ./magpie
```

**From npm** — downloads a prebuilt binary from the matching GitHub release:

```bash
npm install -g @chamberz40/magpie
```

**From a GitHub release** — grab the archive for your platform from
[Releases](https://github.com/ChamberZ40/magpie/releases), unpack it, and put
`magpie` on your `PATH`.

### 4. First run

```bash
magpie                     # auto-creates ~/.magpie/config.toml
```

It prints the admin URL:

```
Web admin:  http://localhost:9820
```

To move it off 9820, set `port` under `[management]` in `config.toml`.

> `magpie web` **only** opens the browser and the config UI — it does not start
> the bridge. Keep `magpie` itself running.

### 5. Add platform credentials

In the Web UI, create a project, add a platform (Feishu / WeChat Work / Weixin),
and paste the credentials from that platform's developer console. Save; Magpie
hot-reloads. Send a message to your bot to confirm.


## Run as a service

```bash
magpie daemon install --config ~/.magpie/config.toml
magpie daemon start
magpie daemon status
magpie daemon restart
magpie daemon stop
magpie daemon uninstall
```

This installs a launchd agent on macOS, a systemd unit on Linux, and a Task
Scheduler task named `magpie` on Windows. On Linux, run
`loginctl enable-linger $USER` so the unit survives logout — `daemon install`
warns when linger is off.

### Keeping the host awake

A bridge is only reachable while its host is awake. Once the machine idle
sleeps, the platform connection drops and messages sit undelivered until
something wakes it — on a laptop with stock power settings, minutes after you
walk away. Which defeats the entire point of steering the agent from your phone.

```toml
[power]
prevent_sleep = "ac_only"   # "off" (default), "always", or "ac_only"
```

Magpie holds a macOS power assertion for as long as it runs and releases it on
shutdown. It deliberately does **not** touch `pmset`: a global setting like that
outlives a crash, and a Mac that never sleeps again is a far worse failure than
one that sleeps too eagerly.

| Value | Effect |
|-------|--------|
| `off` | No assertion. The machine sleeps on its own schedule. Default. |
| `always` | Stay awake on battery and on AC. |
| `ac_only` | Stay awake only while plugged in. |

Limits worth knowing before you rely on it:

- **macOS only.** On Linux and Windows a non-`off` value is logged and ignored.
- **Closing the lid always sleeps an Apple Silicon laptop**, unless it's in
  clamshell mode with an external display and power. No software assertion can
  override that. This setting covers "lid open, left alone".
- **Not hot-reloadable.** Restart the service after changing it.


## Configure

Config lives at `~/.magpie/config.toml`. The Web UI (`magpie web`) edits it
visually — projects, platforms, providers — with no TOML editing. By hand:

```bash
mkdir -p ~/.magpie
magpie config example > ~/.magpie/config.toml
vim ~/.magpie/config.toml
```

[config.example.toml](config.example.toml) is the annotated reference for every
option. The minimum viable shape is one project = one agent + one platform:

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "claudecode"          # or codex, cursor, copilot, acp

[projects.agent.options]
work_dir = "/path/to/project"
mode = "default"

[[projects.platforms]]
type = "feishu"              # or wecom, weixin

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
```

One process can run many projects, each with its own agent + platform pair.

### Keep secrets out of the file

Any option value may reference an environment variable, which is how to avoid
committing credentials:

```toml
app_secret = "${FEISHU_APP_SECRET}"
```

### Privileged commands

`admin_from` lists the user IDs allowed to run privileged commands such as
`/dir` and `/shell`. It belongs under `[[projects]]` — **not** under
`[projects.platforms.options]`:

```toml
[[projects]]
admin_from = "alice,bob"
```

Use `/whoami` or `/status` in chat to find your own user ID.

### Session reset on idle

Projects rotate to a fresh session after inactivity. This prevents context
drift, where stale history (failed commands, debugging noise) is repeatedly
re-ingested via `--continue` and starts to dominate the model's attention. The
previous session is preserved and stays reachable via `/list` and `/switch`.

```toml
[[projects]]
reset_on_idle_mins = 30   # default when unset; 0 disables rotation
```

### Permission mode

```toml
[projects.agent.options]
mode = "default"
```

Switchable at runtime with `/mode`. Values are agent-specific: Claude Code takes
`default` / `acceptEdits` / `auto` / `plan` / `bypassPermissions`, Codex takes
`suggest` / `auto-edit` / `full-auto` / `yolo`, Cursor takes `default` / `force`
/ `plan` / `ask`, Copilot takes `default` / `bypassPermissions`.

### Attachment send-back

Agents can push generated files back into the chat:

```toml
attachment_send = "on"          # default "on"; "off" blocks image/file send-back
max_attachment_size_mb = 50     # default 50 MiB
```

```bash
magpie send --image /absolute/path/to/chart.png
magpie send --file /absolute/path/to/report.pdf
magpie send --tts "Hello from magpie"
```

Currently delivered on Feishu. Absolute paths are safest; `--image` and `--file`
may both be repeated. This switch is independent of the agent's `/mode` — it
only gates `magpie send`, and ordinary text replies keep working when it is
`off`. Voice send-back uses the `[speech]` TTS config instead.

If your agent does not natively inject the system prompt, run `/bind setup` (or
`/cron setup`) once in chat after rebuilding, to refresh the Magpie
instructions in the project memory file.

### Scheduled tasks

```bash
/cron add 0 6 * * * Summarize GitHub trending
```

### OS-user isolation (`run_as_user`)

On Linux/macOS a project can spawn its agent under a different Unix user, for
file-system isolation from the supervisor user running Magpie. Currently
supported by Claude Code.

```toml
[[projects]]
name = "claude-sandboxed"
run_as_user = "partseeker-coder"
run_as_env = ["PGSSLROOTCERT"]
```

The target user needs passwordless sudo from the supervisor, no sudo of its own,
read+write on `work_dir`, and its own `~/.claude/settings.json` with whatever
credentials the agent uses. Under `claude.ai` OAuth, symlink the target user's
`~/.claude/.credentials.json` to the supervisor's copy so token refresh stays in
sync — see the
[environment propagation checklist](./docs/usage.md#environment-propagation-what-moves-into-the-target-users-home).

Audit before starting:

```bash
magpie doctor user-isolation
```

Three go/no-go preflight gates plus an isolation probe reporting what the target
user can and cannot read. Magpie refuses to start if a gate fails or the probe
finds a cross-user leak.


## Selective builds

Every agent and platform is imported through its own `plugin_*.go` file behind a
build tag, so a build can carry a subset. All of them are included by default.

```bash
make build AGENTS=claudecode PLATFORMS_INCLUDE=feishu
make build AGENTS=claudecode,codex PLATFORMS_INCLUDE=feishu,wecom
make build EXCLUDE=weixin,wecom

go build -tags 'no_weixin no_wecom' ./cmd/magpie   # without Make
```

Available tags: `no_acp`, `no_claudecode`, `no_codex`, `no_copilot`,
`no_cursor`, `no_feishu`, `no_wecom`, `no_weixin`.


## Runtime commands

Typed in chat, not in a shell.

```
/new [name]                 Start a new session
/list                       List sessions
/switch <id>                Switch session
/current                    Show current session
/dir [path|reset]           Show, switch, or reset the work directory
/dir <number> | /dir -      Jump through directory history
/mode [name]                Show or switch permission mode
/model [switch <alias>]     List or switch model
/provider [switch <name>]   List or switch API provider
/git [subcommand]           Read-only repository queries
/cron, /timer               Recurring and one-shot scheduled tasks
/cancel                     Interrupt the current turn
/whoami, /status            Identity and session state
```

`/dir reset` restores the configured `work_dir` and clears the persisted
override in `data_dir/projects/<project>.state.json`.

Full reference: [docs/usage.md](docs/usage.md).


## Documentation

- [docs/usage.md](docs/usage.md) — complete feature and command reference
- [INSTALL.md](INSTALL.md) — step-by-step install guide written for an AI agent to follow
- [config.example.toml](config.example.toml) — annotated configuration template
- [docs/management-api.md](docs/management-api.md) — HTTP management API
- [docs/bridge-protocol.md](docs/bridge-protocol.md) — WebSocket protocol for third-party platform adapters
- [CLAUDE.md](CLAUDE.md) / [AGENTS.md](AGENTS.md) — architecture and contribution rules


## License

[MIT](LICENSE) — use it however you like, commercially included; the only
condition is keeping the copyright and permission notice.

Magpie began as a fork of [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect),
which declares MIT in `npm/package.json` but ships no `LICENSE` file. So
[LICENSE](LICENSE) carries both that notice and this project's.
