<p align="center">
  <img src="./docs/images/banner.svg" alt="Magpie Banner" width="800"/>
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README.zh-CN.md">中文</a>
</p>

**Magpie 把你的 AI 编程 Agent 搬进聊天软件。** 你在飞书或微信里发消息，Agent
在本地、在你自己的项目目录里、用你自己的凭据执行，结果回到聊天窗口。除了你本来
就要敲给 Agent 的那些文字，代码不出本机。

它解决的是「人得守在电脑前」这件事：长任务照跑，你可以起身走开，用手机盯着。

> Magpie 是 [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect) 的 fork。
> 代码主体来自上游；本 fork 把它裁剪到自己实际用得到的 Agent 与平台，并且不再
> 跟随上游更新。本项目与上游没有隶属关系，也未获其背书。详见[许可](#许可)。

<p align="center">
  <img src="docs/images/connector.png" alt="Magpie Architecture" width="90%"/>
</p>


## 快速开始

最快的路子是 npm 包 —— 它直接下载预编译二进制，既不需要 Go 也不需要 Node 的
构建工具链：

```bash
npm install -g @z40/magpie
```

想从源码构建，则需要 Go 1.25+ 和 Node.js（构建会一并打包内嵌 Web UI），见[安装](#安装)：

```bash
git clone https://github.com/ChamberZ40/magpie.git
cd magpie
make build                 # 产物为 ./magpie
```

无论哪条路，你都还需要一个**已安装且已登录**的 Agent CLI —— magpie 以子进程方式
拉起它，并沿用它的登录凭据。然后：

```bash
magpie                     # 创建 ~/.magpie/config.toml 后退出
```

打开这个文件：把 `work_dir` 指向你要让 Agent 干活的目录（该目录必须已存在），再填上
机器人凭据 —— 飞书那半可以用 `magpie feishu setup` 代劳，`magpie config example`
会打印带注释的全量配置示例。填好后再跑一次 `magpie`，给机器人发条消息确认链路通了。

桥跑起来之后想开 Web 管理界面，用 `magpie web`，或在聊天里发 `/web setup`。


## 包含哪些

| Agent | 配置写法 |
|-------|----------|
| Claude Code | `type = "claudecode"` |
| Codex (OpenAI) | `type = "codex"` |
| Cursor Agent | `type = "cursor"` |
| GitHub Copilot CLI | `type = "copilot"` |
| ACP | `type = "acp"` —— 任何[支持 ACP 的 Agent](https://agentclientprotocol.com/get-started/agents)，如 OpenClaw、Hermes |

| 平台 | 连接方式 | 需要公网 IP？ | 接入指南 |
|------|----------|---------------|----------|
| 飞书 (Lark) | WebSocket | 不需要 | [docs/feishu.md](docs/feishu.md) |
| 企业微信 | WebSocket / Webhook | WS 不需要 / Webhook 需要 | [docs/wecom.md](docs/wecom.md) |
| 微信（个人，ilink） | HTTP 长轮询 | 不需要 | [docs/weixin.md](docs/weixin.md) |

各平台能力：

| 能力 | 飞书 | 企业微信 | 微信 *(个人)* |
|------|:----:|:--------:|:-------------:|
| 文本与斜杠命令 | ✅ | ✅ | ✅ |
| Markdown / 卡片 | ✅ | ⚠️ | ✅ |
| 流式 / 分段回复 | ✅ | ✅ | ✅ |
| 图片与文件 | ✅ | ✅ | ✅ |
| 语音 / STT / TTS | ⚠️ | ⚠️ | ✅ |
| 私聊 | ✅ | ✅ | ✅ |
| 群聊 / 频道 | ✅ | ✅ | ✅ |

⚠️ 表示部分支持或需要额外配置——尤其语音那一行，需要在 `config.toml` 里配好
`[speech]` / TTS 提供商。


## 安装

### 1. 先装 Agent CLI

Magpie 桥接的是**本地**的 Agent CLI，所以 Agent 必须先于 Magpie 存在。跳过这步，
Magpie 会直接退出并报 `claudecode: claude CLI not found in PATH`（其他 Agent 类似），
Web 管理界面也起不来。

```bash
# Claude Code
brew install --cask claude-code            # macOS / Linux Homebrew
npm install -g @anthropic-ai/claude-code   # 或用 npm，全平台

# OpenAI Codex
npm install -g @openai/codex

# GitHub Copilot CLI
npm install -g @github/copilot
```

Cursor Agent 见官方文档 <https://docs.cursor.com/agent>。其他支持 ACP 的 Agent
用 `type = "acp"` 接入。

```bash
claude --version       # 或 codex / copilot / cursor-agent
```

### 2. 登录 Agent

先交互式跑一次，让它把凭据写进你的 home 目录：

```bash
claude login           # 会打开浏览器
codex login            # 或 copilot / cursor-agent，具体见各自文档
```

跳过这步 Magpie 照样能启动，但 Agent 会用鉴权错误拒掉每一个请求。

### 3. 安装 Magpie

**从源码构建** —— 标准路径。需要 Go 1.25+ 和 Node.js，因为 `make build` 会连带
重新打包内嵌的 Web UI。

```bash
git clone https://github.com/ChamberZ40/magpie.git
cd magpie
make build                 # 产物为 ./magpie
```

**用 npm 装** —— 从对应的 GitHub Release 下载预编译二进制：

```bash
npm install -g @z40/magpie
```

**从 GitHub Release 下载** —— 在
[Releases](https://github.com/ChamberZ40/magpie/releases) 里挑你平台的压缩包，
解压后把 `magpie` 放进 `PATH`。

### 4. 首次启动

```bash
magpie                     # 自动创建 ~/.magpie/config.toml
```

它会打印管理地址：

```
Web admin:  http://localhost:9820
```

想换掉 9820，在 `config.toml` 的 `[management]` 下设 `port`。

> `magpie web` **只**打开浏览器和配置界面，**不会**启动桥接服务。
> `magpie` 本身得单独跑着。

### 5. 填平台凭据

在 Web UI 里建项目、加平台（飞书 / 企业微信 / 微信），把该平台开发者后台的凭据粘
进去。保存后 Magpie 会热重载。给机器人发条消息验证一下。


## 作为服务运行

```bash
magpie daemon install --config ~/.magpie/config.toml
magpie daemon start
magpie daemon status
magpie daemon restart
magpie daemon stop
magpie daemon uninstall
```

macOS 装的是 launchd agent，Linux 是 systemd unit，Windows 是名为 `magpie` 的
计划任务。Linux 上执行 `loginctl enable-linger $USER`，让服务在你登出后继续存活
——linger 没开时 `daemon install` 会警告。

### 不让机器睡着

桥只在宿主机醒着的时候可达。机器一进空闲休眠，平台连接就断，消息压着不投递，直到
有什么把它唤醒——笔记本按出厂电源设置，人走开几分钟就会发生。这正好抵消掉「用手机
盯着 Agent」的全部意义。

```toml
[power]
prevent_sleep = "ac_only"   # "off"（默认）、"always" 或 "ac_only"
```

Magpie 在运行期间持有一个 macOS power assertion，关停时释放。它刻意**不去动**
`pmset`：那种全局设置会在崩溃后继续生效，而一台再也不睡的 Mac，比一台睡得太勤的
Mac 糟糕得多。

| 取值 | 行为 |
|------|------|
| `off` | 不持有 assertion，机器按自己的节奏休眠。默认。 |
| `always` | 电池和交流电下都保持唤醒。 |
| `ac_only` | 只在插电时保持唤醒。 |

指望它之前，先知道这几条边界：

- **仅 macOS 生效。** Linux 和 Windows 上，非 `off` 的取值会被记一条日志然后忽略。
- **Apple Silicon 笔记本合盖必睡**，除非接了外接显示器和电源的蛤壳模式。任何软件
  assertion 都盖不住这一条。本设置解决的是「开盖放着不动」。
- **不支持热重载。** 改完要重启服务。


## 配置

配置文件在 `~/.magpie/config.toml`。Web UI（`magpie web`）可以可视化编辑
项目、平台、Provider，不用手改 TOML。想手改：

```bash
mkdir -p ~/.magpie
magpie config example > ~/.magpie/config.toml
vim ~/.magpie/config.toml
```

[config.example.toml](config.example.toml) 是每个选项的带注释参考。最小可用形态是
一个项目 = 一个 Agent + 一个平台：

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "claudecode"          # 或 codex, cursor, copilot, acp

[projects.agent.options]
work_dir = "/path/to/project"
mode = "default"

[[projects.platforms]]
type = "feishu"              # 或 wecom, weixin

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
```

一个进程可以同时跑多个项目，每个项目有自己的 Agent + 平台组合。

### 别把密钥写进文件

任何选项值都能引用环境变量，这是避免把凭据提交进仓库的办法：

```toml
app_secret = "${FEISHU_APP_SECRET}"
```

### 特权命令

`admin_from` 列出允许执行 `/dir`、`/shell` 等特权命令的用户 ID。它必须写在
`[[projects]]` 下面，**不是** `[projects.platforms.options]` 下面：

```toml
[[projects]]
admin_from = "alice,bob"
```

在聊天里用 `/whoami` 或 `/status` 查自己的用户 ID。

### 空闲自动重置会话

项目在长时间无操作后会轮换到新会话。这是为了防止上下文漂移——陈旧的历史记录
（失败的命令、调试噪音）被 `--continue` 反复重新读入，逐渐主导模型的注意力。
旧会话会保留，仍可通过 `/list` 和 `/switch` 访问。

```toml
[[projects]]
reset_on_idle_mins = 30   # 不设置时的默认值；设为 0 关闭轮换
```

### 权限模式

```toml
[projects.agent.options]
mode = "default"
```

运行时用 `/mode` 切换。取值按 Agent 而异：Claude Code 是 `default` /
`acceptEdits` / `auto` / `plan` / `bypassPermissions`，Codex 是 `suggest` /
`auto-edit` / `full-auto` / `yolo`，Cursor 是 `default` / `force` / `plan` /
`ask`，Copilot 是 `default` / `bypassPermissions`。

### 附件回传

Agent 可以把生成的文件推回聊天：

```toml
attachment_send = "on"          # 默认 "on"；设为 "off" 禁止图片/文件回传
max_attachment_size_mb = 50     # 默认 50 MiB
```

```bash
magpie send --image /absolute/path/to/chart.png
magpie send --file /absolute/path/to/report.pdf
magpie send --tts "Hello from magpie"
```

目前在飞书上投递。用绝对路径最稳妥；`--image` 和 `--file` 都可以重复传。这个开关
与 Agent 的 `/mode` 无关，只管 `magpie send`，设成 `off` 后普通文本回复照常。
语音回传走 `[speech]` 的 TTS 配置。

如果你的 Agent 不是原生注入 system prompt 的类型，重新构建后在聊天里执行一次
`/bind setup`（或 `/cron setup`），刷新项目记忆文件里的 Magpie 说明。

### 定时任务

```bash
/cron add 0 6 * * * 汇总 GitHub trending
```

### 系统用户隔离（`run_as_user`）

在 Linux/macOS 上，项目可以用另一个 Unix 用户启动 Agent，从文件系统层面与运行
Magpie 的管理用户隔离。目前仅 Claude Code 支持。

```toml
[[projects]]
name = "claude-sandboxed"
run_as_user = "partseeker-coder"
run_as_env = ["PGSSLROOTCERT"]
```

目标用户需要：管理用户到它的免密 sudo、自己没有 sudo 权限、对 `work_dir` 有读写
权限、以及自己的 `~/.claude/settings.json` 和相应凭据。如果走 `claude.ai` OAuth，
把目标用户的 `~/.claude/.credentials.json` 软链到管理用户那份，token 刷新才能同步
——详见[环境传递清单](./docs/usage.md#environment-propagation-what-moves-into-the-target-users-home)。

启动前先审计：

```bash
magpie doctor user-isolation
```

三道 go/no-go 前置检查加一次隔离探测，报告目标用户能读到什么、读不到什么。任何一
道检查不过、或探测发现跨用户泄露，Magpie 会拒绝启动。


## 按需构建

每个 Agent 和平台都通过自己的 `plugin_*.go` 配 build tag 引入，所以可以只编进一部
分。默认全部包含。

```bash
make build AGENTS=claudecode PLATFORMS_INCLUDE=feishu
make build AGENTS=claudecode,codex PLATFORMS_INCLUDE=feishu,wecom
make build EXCLUDE=weixin,wecom

go build -tags 'no_weixin no_wecom' ./cmd/magpie   # 不用 Make
```

可用 tag：`no_acp`、`no_claudecode`、`no_codex`、`no_copilot`、`no_cursor`、
`no_feishu`、`no_wecom`、`no_weixin`。


## 运行时命令

在聊天里输入，不是在 shell 里。

```
/new [name]                 新建会话
/list                       列出会话
/switch <id>                切换会话
/current                    当前会话
/dir [path|reset]           查看、切换或重置工作目录
/dir <number> | /dir -      在目录历史里跳转
/mode [name]                查看或切换权限模式
/model [switch <alias>]     查看或切换模型
/provider [switch <name>]   查看或切换 API Provider
/git [子命令]               只读的仓库查询
/cron, /timer               周期任务与一次性延时任务
/cancel                     打断当前这轮
/whoami, /status            身份与会话状态
```

`/dir reset` 会恢复配置里的 `work_dir`，并清掉持久化在
`data_dir/projects/<project>.state.json` 里的覆盖值。

完整参考见 [docs/usage.zh-CN.md](docs/usage.zh-CN.md)。


## 文档

- [docs/usage.zh-CN.md](docs/usage.zh-CN.md) —— 完整功能与命令参考
- [INSTALL.md](INSTALL.md) —— 写给 AI Agent 照着执行的安装指南
- [config.example.toml](config.example.toml) —— 带注释的配置模板
- [docs/management-api.zh-CN.md](docs/management-api.zh-CN.md) —— HTTP 管理 API
- [docs/bridge-protocol.zh-CN.md](docs/bridge-protocol.zh-CN.md) —— 第三方平台适配器的 WebSocket 协议
- [CLAUDE.md](CLAUDE.md) / [AGENTS.md](AGENTS.md) —— 架构与开发规范


## 许可

[MIT](LICENSE) —— 随便用，商用也行，唯一的条件是保留版权与许可声明。

Magpie 起初是 [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect) 的 fork，
后者在 `npm/package.json` 里声明了 MIT，但仓库里没有 `LICENSE` 文件，所以
[LICENSE](LICENSE) 同时带上了它和本项目的版权声明。
