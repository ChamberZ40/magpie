# @z40/magpie

Put your AI coding agent in your chat app. Send a message in Feishu or WeChat;
the agent runs locally in your own project directory; its output comes back to
the chat.

Agents: Claude Code, Codex, Cursor Agent, GitHub Copilot CLI, and any
ACP-speaking agent. Platforms: Feishu (Lark), WeChat Work, Weixin.

## Quick start

**First, install the agent CLI you want to drive and log into it.** Magpie
spawns it as a subprocess and inherits its credentials, so an agent that is
not logged in fails on the first message. For Claude Code:

```bash
npm install -g @anthropic-ai/claude-code
claude          # log in, then quit
```

**1. Install magpie.** This downloads the prebuilt binary for your platform
from the matching GitHub release:

```bash
npm install -g @z40/magpie
```

**2. Create the config.** The first run writes `~/.magpie/config.toml` and
exits — it does not start the bridge yet:

```bash
magpie
# Created default config at /Users/you/.magpie/config.toml
# Please edit this file to add your agent and platform credentials, then run magpie again.
```

**3. Fill it in.** Open that file and set `work_dir` to the project you want
the agent to work in — it must already exist. Then add your bot credentials.
For Feishu you can skip the hand-editing:

```bash
magpie feishu setup     # QR onboarding, or --app to bind an existing bot
```

For WeChat Work and Weixin, see the setup guides:
[WeChat Work](https://github.com/ChamberZ40/magpie/blob/main/docs/wecom.md),
[Weixin](https://github.com/ChamberZ40/magpie/blob/main/docs/weixin.md).
`magpie config example` prints every option, annotated.

**4. Start it.**

```bash
magpie
```

Message the bot from your chat app. A reply means the round trip works.

**5. Keep it running** after you close the terminal:

```bash
magpie daemon install     # installs and starts a launchd/systemd service
magpie daemon logs -f     # follow the logs
```

### If something does not work

```bash
magpie doctor             # checks the local setup: agent CLI, config, permissions
magpie config path        # where the config actually resolved to
magpie --help             # every command, plus the agents and platforms this build ships
```

## Documentation

Full documentation: https://github.com/ChamberZ40/magpie

## License and credits

[MIT](https://github.com/ChamberZ40/magpie/blob/main/LICENSE).

Magpie is a fork of [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect),
which declares MIT in its `npm/package.json`. Most of the code is upstream's;
this fork trims it to the agents and platforms listed above and no longer
tracks upstream. It is not affiliated with or endorsed by the upstream project.
