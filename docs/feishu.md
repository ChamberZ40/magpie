# Feishu (Lark) Setup Guide

<p align="center">
  <a href="./feishu.md">English</a> | <a href="./feishu.zh-CN.md">中文</a>
</p>

This guide connects **magpie** to Feishu, so you can drive Claude Code remotely
through a Feishu bot.

> The Feishu developer console may show its menus in Chinese. Each console
> label below is given in English with the Chinese label in brackets, so you
> can find it either way.

## Prerequisites

- A Feishu account (personal or enterprise both work)
- A machine that can run magpie (no public IP needed)
- Claude Code installed and configured

> 💡 **Why this is easy**: magpie uses Feishu's long-connection (WebSocket)
> mode — no public IP, no domain, no reverse proxy (ngrok/frp).

---

## Quick setup (recommended)

If `magpie` is already installed, a built-in command can create a new bot or
bind an existing one, and writes the result back into `config.toml`:

```bash
# Recommended: the unified entry point
magpie feishu setup --project my-project
magpie feishu setup --project my-project --app cli_xxx:sec_xxx

# Forced modes (rarely needed)
magpie feishu new --project my-project
magpie feishu bind --project my-project --app cli_xxx:sec_xxx
```

How they differ:

| Command | What it does | When to use it |
|---------|--------------|----------------|
| `setup` | Unified entry: no credentials → `new`; credentials given → `bind` | **Use this by default** |
| `new` | Forces QR-code creation of a new app (rejects `--app`) | You explicitly want to create a new bot |
| `bind` | Forces binding existing credentials (requires `app_id`/`app_secret`) | You only want to bind credentials |

Notes:

- `setup --app ...` and `bind --app ...` are equivalent.
- `setup`/`new` print a QR code and a URL in the terminal; scan it with the
  Feishu/Lark mobile app to finish creating the app.
- If `--project` doesn't exist it is created; if it exists but has no
  `feishu`/`lark` platform, one is added.
- Only the target fields (`app_id`, `app_secret`, `allow_from`, …) are rewritten;
  existing comments and layout are preserved as far as possible.
- The flow fills in credentials. When you create the app by QR code, Feishu
  usually pre-configures permissions and event subscriptions too — **usually,
  not always**.
- So still check in the developer console that the app is published, its
  permissions are granted, and its availability scope is what you expect. In
  particular, confirm the `card.action.trigger` callback (Step 5.3).

---

## Step 1: Create a Feishu custom app

### 1.1 Open the Feishu Open Platform

Go to the [Feishu Open Platform](https://open.feishu.cn/) (Lark:
[open.larksuite.com](https://open.larksuite.com/)) and sign in.

### 1.2 Create the app

1. Click **Developer Console** [开发者后台] in the top-right corner
2. Click **Create Custom App** [创建企业自建应用]

> 💡 **Individuals can do this too**: the Open Platform lets individual
> developers create apps without enterprise verification.

### 1.3 Fill in the app details

| Field | Suggestion |
|-------|------------|
| App name | `magpie`, or any name you like |
| Description | `Claude Code remote assistant` |
| Icon | Any icon you like |

---

## Step 2: Get the credentials

### 2.1 Open the credentials page

In the app's left sidebar, click **Credentials & Basic Info** [凭证与基础信息].

### 2.2 Copy the App ID and App Secret

You'll see:

```
App ID:     cli_axxxxxxxxxxxx
App Secret: QhkMpxxxxxxxxxxxxxxxxxxxx
```

> ⚠️ **Keep these safe** — magpie needs both. If you lose the App Secret you
> will have to reset it.

### 2.3 Add them to magpie

Put the credentials in magpie's `config.toml`:

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "claudecode"

[projects.agent.options]
work_dir = "/path/to/your/project"
mode = "default"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_axxxxxxxxxxxx"
app_secret = "QhkMpxxxxxxxxxxxxxxxxxxxx"
# domain = "https://open.feishu.cn" # optional: override the runtime API/WebSocket domain
# enable_feishu_card = true  # optional: set false to fall back to plain-text replies everywhere
# thread_isolation = true    # optional: isolate group-chat sessions per Feishu thread/root message
# progress_style = "legacy"  # optional: legacy | compact | card
# done_emoji = "none"          # optional: reaction added when the agent finishes (e.g. "Done"); "none" disables
# image_batch_window_ms = 500  # optional: window for batching consecutive images (default 500ms, see below)
```

To keep the secret out of the file, reference an environment variable instead:
`app_secret = "${FEISHU_APP_SECRET}"`.

What the optional settings do:

- **`enable_feishu_card = false`** — use this if the app lacks interactive-card
  permission or the card callback isn't configured. Every command then replies
  in plain text, so a failed card send never leaves the user seeing nothing.
- **`thread_isolation = true`** — in group chats, each root message / reply
  thread gets its own agent session. Direct messages are unaffected. In
  multi-workspace mode it also binds a workspace per thread: `/workspace bind
  <name>` inside a thread doesn't affect other threads in the same group. An
  existing group-level binding stays as the default that unbound threads
  inherit, so rolling back to an older version still works.
- **`progress_style`** — `compact` merges thinking/tool progress into one
  message that updates in place, cutting noise; `legacy` keeps sending one
  message per step; `card` keeps updating a single structured card (title +
  progress blocks), which reads more clearly than plain text.
- **`domain`** — only affects runtime API / WebSocket requests. The CLI
  `setup`/`new`/`bind` onboarding flow always uses the built-in default domain.
- **`done_emoji`** — when set, magpie adds that reaction to the user's message
  each time the agent finishes (e.g. `"Done"` → ✅), removing the "OnIt"
  reaction first if present. Especially useful in quiet mode: Feishu doesn't
  push a notification when a card updates in place, so the reaction is what
  tells the user the agent is done. Unset or `"none"` disables it.
- **`image_batch_window_ms`** — how long magpie waits to merge consecutive
  images into one agent message (default 500ms). When the Feishu mobile app
  sends several images at once, each arrives as a separate event; magpie
  merges those that land inside the window. If your images are still split
  into separate turns because they arrive more than 500ms apart, raise it to
  800–1200ms; if you mostly send single images and want faster replies, lower
  it. `0` means the default 500ms.

---

## Step 3: Enable the bot capability

### 3.1 Turn on the bot

1. In the left sidebar, click **Features** [应用能力] → **Bot** [机器人]
2. Click **Enable Bot** [启用机器人]

### 3.2 Configure the bot

| Setting | Suggestion |
|---------|------------|
| Bot name | `magpie` |
| Description | `Claude Code remote assistant` |
| Avatar | Same as the app icon |

---

## Step 4: Configure permissions

### 4.1 Open permission management

In the left sidebar, click **Permissions & Scopes** [权限管理].

### 4.2 Add the required scopes

Search for and add these scopes:

| Scope | Purpose |
|-------|---------|
| `contact:user.base:readonly` | Read basic user info |
| `im:message.group_at_msg:readonly` | Receive messages that @ the bot in groups |
| `im:message.p2p_msg:readonly` | Receive and read direct messages to the bot |
| `im:message.group_msg` | Read all group messages (sensitive scope) |
| `im:message:send_as_bot` | Send replies as the bot |

### 4.3 Submit the permission request

Once the scopes are added, click **Request Release** [申请发布] so they take
effect.

---

## Step 5: Subscribe to events and callbacks (long-connection mode)

### 5.1 Open Events & Callbacks

In the left sidebar, click **Events & Callbacks** [事件与回调].

### 5.2 Event configuration

Open the **Event Configuration** [事件配置] tab.

Under **Subscription mode** [订阅方式], choose:

```
✅ Receive events through persistent connection [使用长连接接收事件]
```

Click **Save**, then **Add Events** [添加事件], and add:

| Event | Event key | Purpose |
|-------|-----------|---------|
| Receive messages [接收消息] | `im.message.receive_v1` | Receive the messages users send |

### 5.3 Callback configuration

Open the **Callback Configuration** [回调配置] tab.

Under **Subscription mode** [订阅方式], choose:

```
✅ Receive events through persistent connection [使用长连接接收事件]
```

Click **Save**, then **Add Callback** [添加回调], and add:

| Callback | Callback key | Purpose |
|----------|--------------|---------|
| Card action [卡片回调] | `card.action.trigger` | Respond to interactive-card button clicks (permission prompts, provider switching, …) |

> ⚠️ **Important**: without the `card.action.trigger` callback, clicking a card
> button (permission confirmation, provider selection, …) does nothing, and the
> Feishu client may show a loading timeout or an error. If you can't add the
> callback yet, set `enable_feishu_card = false` to turn interactive cards off;
> everything falls back to plain text.

### 5.4 Create a version

Click **Create Version** [创建版本] and publish it, so the event and callback
configuration takes effect.

---

## Step 6: Start magpie

### 6.1 Start the service

```bash
magpie
# or with an explicit config file
magpie --config /path/to/config.toml
```

### 6.2 Verify the connection

On startup magpie opens a WebSocket long connection to Feishu. The log shows:

```
level=INFO msg="feishu: bot identified" open_id=ou_xxxxxxxxxxxxxxxx
level=INFO msg="platform ready" project=my-project platform=feishu
level=INFO msg="magpie is running" projects=1
[Info] connected to wss://msg-frontier.feishu.cn/ws/v2?...
```

---

## Step 7: Publish the app

### 7.1 Submit for release

1. In the left sidebar, click **Version Management & Release** [版本管理与发布]
2. Click **Create Version** [创建版本]
3. Fill in the version number and release notes
4. Click **Save and Publish** [保存并发布]

### 7.2 Availability

- **Enterprise tenants**: an administrator must approve the release before it
  can be used
- **Personal tenants**: available immediately after publishing

---

## Step 8: Add the bot to a chat

### 8.1 Direct messages

Search for your bot's name in Feishu and send it a message.

### 8.2 Group chats

1. Open the group chat
2. Open group settings → **Bots** [群机器人]
3. Add the bot you created

---

## Example

Once set up, a conversation in Feishu looks like:

```
You:    Walk me through the structure of this project

magpie: 🤔 Thinking...
magpie: 🔧 Running: Bash(ls -la)
magpie: ✅ This is a Node.js project with the following directories...
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Feishu cloud                          │
│                                                              │
│   User message ──→ Open Platform ──→ WebSocket Gateway       │
│                                          │                   │
└──────────────────────────────────────────┼───────────────────┘
                                           │
                                           │ WebSocket long connection
                                           │ (no public IP needed)
                                           ▼
┌─────────────────────────────────────────────────────────────┐
│                     Your local machine                       │
│                                                              │
│   magpie ◄──► Claude Code CLI ◄──► your project code         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Mentions

With `resolve_mentions = true`, any `@DisplayName` in a message the bot sends is
replaced with a native Feishu @-mention.

### Configuration

```toml
[projects.platforms.options]
resolve_mentions = true
```

### Syntax

Just write `@DisplayName` — no special markup:

```
@Alice please review the inspection report
```

### Examples

**A cron job:**

```bash
magpie cron add \
  --cron "0 9 * * *" \
  --prompt "Run the daily inspection report, then tell @Alice and @Bob to review it" \
  --desc "Daily inspection"
```

**In conversation:**

When the agent's output contains `@someone`, it is matched and replaced before
the message is sent to Feishu.

### How it works

1. With `resolve_mentions` on, magpie fetches the group's member list before
   sending (lazily — only the first time)
2. The member list is cached for 1 hour to limit API calls
3. Names are matched longest first (`@Alice Smith` wins over `@Alice`), so a
   shorter name never partially matches a longer one
4. An `@xxx` that matches nobody is left as-is
5. The right Feishu @ syntax is chosen for the message type (text vs. card)

### Required permissions

One of these app scopes:

- `im:chat` (read and update group info)
- `im:chat:readonly` (read group info)
- `im:chat.members:read` (read group members)

### Caveats

- Matching is exact (`@Alice` only matches a member whose display name is
  exactly "Alice")
- If several members share a name, the first match wins
- The person being mentioned must be a member of the current group
- With `resolve_mentions` off, no member lookup ever happens

---

## Bot-to-bot mentions (`mention_map`)

`resolve_mentions` resolves `@name` by matching **group members' display
names**. When the target is **another bot / agent** rather than a person, that
bot may not appear in the member list, and name matching fails.

`mention_map` covers that case: map a display name to the bot's `open_id` by
hand, and magpie emits a native Feishu `<at user_id="...">` tag that triggers a
real @ notification.

### Configuration

```toml
[projects.platforms.options]
resolve_mentions = true                       # mention_map requires resolve_mentions = true
mention_map = { BOT-B = "ou_bot_b_open_id", BOT-A = "ou_bot_a_open_id" }
```

> `mention_map` adds to `resolve_mentions` rather than replacing it:
> - `resolve_mentions` matches group members by display name (ordinary users)
> - `mention_map` supplies explicit open_id mappings (bots not in the member list)
> - When both match the same `@name`, **`mention_map` wins**, so an explicit
>   mapping is never overridden by a member-name match.

### Example

When the agent outputs `@BOT-B please double-check the inspection report`,
magpie rewrites it before sending to Feishu:

```
<at user_id="ou_bot_b_open_id">BOT-B</at> please double-check the inspection report
```

The BOT-B bot receives a Feishu @ event and is triggered.

Typical uses:

- **Multi-agent handoff**: BOT-A's inspection finds a problem → it @BOT-B in its
  reply to trigger a fixing agent.
- **Cross-agent notification**: when one agent finishes a long (cron) task, it
  @s another agent to take over.
- **Triggering via @**: a Feishu bot that receives an @ event can drive magpie's
  session routing.

### Finding a bot's open_id

A bot's `open_id` (starts with `ou_`) is **not** its App ID (starts with `cli_`);
they are entirely different identifiers. The Credentials & Basic Info page shows
only the App ID / App Secret, **not** the bot's `open_id`.

Ways to get it, in order of preference:

1. **Read magpie's startup log (easiest, for apps you run yourself).**
   On startup magpie calls `/open-apis/bot/v3/info` to fetch its own `open_id`
   and logs:
   ```
   feishu: bot identified open_id=ou_xxxxxxxxxxxxxxxx
   ```
   Copy the value from the log.

2. **Call the bot info API (`/open-apis/bot/v3/info`).**
   With any valid `tenant_access_token`:
   ```bash
   curl -H "Authorization: Bearer t-xxxx" https://open.feishu.cn/open-apis/bot/v3/info
   # Response: { "code": 0, "bot": { "open_id": "ou_xxx", ... } }
   ```

### Caveats

- `mention_map` only takes effect with `resolve_mentions = true`; on its own it
  triggers no resolution.
- The `@name` must match a `mention_map` key exactly (case-sensitive).
- A Feishu bot's `open_id` is **per app**, not per chat — the same bot has the
  same `open_id` in every group.
- The mentioned bot must be in **the target group**, and that group must have
  bots enabled; otherwise Feishu delivers no @ event.

---

## FAQ

### Q: Long connection vs. webhook — what's the difference?

| | Long connection | Webhook |
|---|---|---|
| Public IP | ❌ Not needed | ✅ Needed |
| Domain | ❌ Not needed | ✅ Needed |
| HTTPS certificate | ❌ Not needed | ✅ Needed |
| Reverse proxy | ❌ Not needed | ✅ Needed (ngrok/frp) |
| Setup complexity | Simple | More involved |
| Typical use | Local dev, intranet | Production |

### Q: What if the long connection drops?

The Feishu SDK magpie uses reconnects automatically after a disconnect.

### Q: I send a message and nothing happens?

Check:
1. Is magpie running?
2. Did the long connection come up? (see the log)
3. Is the `im.message.receive_v1` event subscribed?

### Q: Clicking a card button does nothing, or errors?

magpie uses interactive cards for permission prompts, provider selection, and
similar actions. If a button click hangs, times out, or errors, check:

1. **Callback subscription**: the `card.action.trigger` callback is subscribed
   in the developer console (see Step 5.3)
2. **App release**: after changing subscriptions you must publish a new version
3. **Permissions**: the app has the `im:message:send_as_bot` scope

**Quick workaround**: if you can't configure the card callback yet, turn
interactive cards off in `config.toml`:

```toml
[projects.platforms.options]
enable_feishu_card = false
```

Everything then falls back to plain text; permission prompts are answered by
replying with text.

### Q: "Insufficient permissions"?

Make sure every required scope has been requested and granted under Permissions
& Scopes, and that you published a new version afterwards.

### Q: The QR-code page mentions OpenClaw — did I misconfigure something?

No. That's usually display text from Feishu's registration template; it doesn't
affect the returned `app_id`/`app_secret` or the magpie connection.

### Q: How do I debug messages?

In the developer console, **Development & Debugging** [开发调试] → **Debugging
Tool** [调试工具] lets you simulate sending messages.

---

## References

- [Feishu Open Platform](https://open.feishu.cn/) · [Lark Developer](https://open.larksuite.com/)
- [Feishu Open Platform docs](https://open.feishu.cn/document/)
- [Bot development guide](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN)
- [Event subscription docs](https://open.feishu.cn/document/ukTMukTMukTM/uUTNz4SN1MjL1UzM)
- [Scope list](https://open.feishu.cn/document/server-docs/application-scope/scope-list)
- [OpenClaw Feishu setup tutorial](https://bytedance.larkoffice.com/docx/MFK7dDFLFoVlOGxWCv5cTXKmnMh) (in Chinese)

---

## Next

- [Set up WeChat Work](./wecom.md) (in Chinese)
- [Set up Weixin (personal, ilink)](./weixin.md) (in Chinese)
- [Back to README](../README.md)
