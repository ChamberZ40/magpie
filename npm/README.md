# @z40/magpie

Put your AI coding agent in your chat app. Send a message in Feishu or WeChat;
the agent runs locally in your own project directory; its output comes back to
the chat.

Agents: Claude Code, Codex, Cursor Agent, GitHub Copilot CLI, and any
ACP-speaking agent. Platforms: Feishu (Lark), WeChat Work, Weixin (personal
WeChat).

## Before you start

You need a coding agent CLI that is installed **and logged in** — magpie runs
it for you and uses its login. For example, Claude Code: run `claude` once and
log in. No public IP or server is needed; everything runs on this machine.

## Get a first reply

```bash
npm install -g @z40/magpie
magpie init
```

`magpie init` asks a few questions, one at a time: which agent to use, which
directory it works in (default: the current one), and which chat app to
connect. Feishu and Weixin both show a QR code in the terminal; scan it with
your phone. It then turns on the recommended settings (each one can be declined)
and starts magpie, as a background service or in the terminal. Message your
bot; a reply means it works. On Weixin, send the first message yourself —
magpie can only reply once it has seen one.

Everything lands in `~/.magpie/config.toml`. Running `magpie init` again is
safe: it keeps what is there and asks before adding anything.

### By hand, instead

```bash
magpie feishu setup     # Feishu / Lark: creates a bot for you
magpie weixin setup     # Weixin: links your personal WeChat
```

Then open `~/.magpie/config.toml`, set `work_dir` under
`[projects.agent.options]` to an existing project directory (and `type` under
`[projects.agent]` if you want `codex`, `cursor`, `copilot` or `acp` instead of
Claude Code), and run `magpie`.

## Keep it running

To keep magpie up after you close the terminal:

```bash
magpie daemon install     # installs and starts a launchd/systemd service
magpie daemon logs -f     # follow the logs
```

## If something does not work

```bash
magpie doctor             # checks the agent CLI, config and permissions
magpie config path        # where the config actually resolved to
magpie --help             # every command, plus the agents and platforms this build ships
```

- **The agent says it is not logged in** — run the agent CLI (e.g. `claude`)
  once by hand and log in.
- **`work_dir ... does not exist`** — point `work_dir` at a real directory, or rerun `magpie init`.
- **`needs at least one [[projects.platforms]]`** — no chat app is connected yet: run `magpie init`.

Platform guides, for options beyond the QR-code setup:
[Feishu](https://github.com/ChamberZ40/magpie/blob/main/docs/feishu.md) ·
[Weixin](https://github.com/ChamberZ40/magpie/blob/main/docs/weixin.md) (Chinese) ·
[WeChat Work](https://github.com/ChamberZ40/magpie/blob/main/docs/wecom.md) (Chinese).
`magpie config example` prints every config option, annotated.

## Documentation

Full documentation: https://github.com/ChamberZ40/magpie

## License and credits

[MIT](https://github.com/ChamberZ40/magpie/blob/main/LICENSE).

Magpie is a fork of [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect),
which declares MIT in its `npm/package.json`. Most of the code is upstream's;
this fork trims it to the agents and platforms listed above and no longer
tracks upstream. It is not affiliated with or endorsed by the upstream project.
