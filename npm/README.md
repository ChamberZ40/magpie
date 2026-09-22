# @chamberz40/magpie

Put your AI coding agent in your chat app. Send a message in Feishu or WeChat;
the agent runs locally in your own project directory; its output comes back to
the chat.

Agents: Claude Code, Codex, Cursor Agent, GitHub Copilot CLI, and any
ACP-speaking agent. Platforms: Feishu (Lark), WeChat Work, Weixin.

## Install

```bash
npm install -g @chamberz40/magpie
```

This downloads the prebuilt binary for your platform from the matching GitHub
release and installs it as `magpie`.

## Usage

```bash
magpie                              # first run creates ~/.magpie/config.toml
magpie --config /path/to/config.toml
```

The first run prints a Web admin URL (`http://localhost:9820`) where you create
a project and paste your bot credentials.

## Documentation

Full documentation: https://github.com/ChamberZ40/magpie
