# OpenClaw 卡片体验对齐设计

**Date:** 2026-09-23
**Goal:** 让 magpie 的飞书卡片达到 openclaw-lark 的流式观感，并把「工具执行显示」和「流式效果」收敛成一组可快速开关的配置项。

---

## 0. 调查状态（接手前先读）

**本文档内所有 `文件:行号` 锚点均已核实，不需要重新调查。** 已覆盖范围：

| 已查清 | 仓库 |
|---|---|
| 飞书流式卡片完整链路（token → 卡片更新） | `openclaw-lark`、`openclaw/extensions/feishu` |
| 那两个 loading 点的实现（`custom_icon` + `img_key`） | `openclaw-lark/src/card/builder.js:526-536` |
| `/verbose` 三态语义、解析、session 持久化、门控落点 | `openclaw/src/auto-reply/` |
| openclaw 插件 SDK 导出面（哪些能 import、哪些必须抄） | `openclaw/src/plugin-sdk/` |
| magpie 现有飞书卡片 / 流式 / 进度卡实现 | `magpie/core/`、`magpie/platform/feishu/` |
| magpie 全部 display / streaming / 平台配置键（含未文档化的） | `magpie/config/config.go` |

两个已确认的**否定结论**，别再花时间验证：

- openclaw 的 verbose 门控（`createVerboseGate` 等）是 **module-private，未经 SDK 暴露**，
  openclaw-lark 自己在 channel 侧重写了一遍。没有「官方实现」可 import。
- `openclaw-lark/src/` 只有 `.js` + `.d.ts` 编译产物，**无 TS 源码**；
  `未命名/` 目录和 `openclaw-lark-full.patch` 也都是编译产物，不是源码。

magpie 是 **Go** 项目、不跑 openclaw，所以 openclaw 插件 SDK 无法直接 import ——
本次是**逻辑翻译**，不是依赖引入。

---

## 1. 术语对齐（先读这节，否则后面全是坑）

openclaw 和 magpie 有两组**名字相近但语义不同**的机制，混淆它们是这次改动最大的风险。

| openclaw | magpie 现有 | 是否等价 |
|---|---|---|
| `/verbose off\|on\|full` | `/quiet full\|compact\|quiet` | ❌ **不等价** |
| 控制**卡片内部**工具详略 | 控制**消息条数**（工具事件是否独立成条） | |
| session 持久化 `verboseLevel` | `[display].mode` + 两个布尔 | |

精确语义（openclaw `agent-runner-helpers.ts:76-83`）：

```
off  → 工具调用完全不显示
on   → 显示工具名 + 摘要
full → 额外显示工具原始输出 / exit code
```

magpie 的 `mode`：

```
full    → thinking/tool 事件各自独立成消息
compact → 隐藏 thinking/tool，每段文本独立发送
quiet   → 隐藏 thinking/tool，所有文本合并进同一张卡
```

⚠️ **命名冲突**：两者都有 `full`，含义不同。见 §7 的待决问题。

另一组容易混的：

| 机制 | 配置路径 | 控制什么 |
|---|---|---|
| `card_mode` | `[display]` | 回复走富卡片聚合，还是逐条消息 |
| `progress_style` | `[projects.platforms.options]` | 进度卡（progress card）的渲染风格 |

这两套**互不感知**。`card_mode = "rich"` 开不出 progress card，`progress_style = "card"` 也不影响回复卡。

---

## 2. 现状盘点

### 2.1 已经有的（可直接复用）

| 能力 | 实现位置 |
|---|---|
| cardkit 两步流：创建实体 + 按元素推内容 | `platform/feishu/feishu.go:4562`、`:4606` |
| `streaming_mode: true` 卡片配置 | `platform/feishu/feishu.go:6602` |
| 单调递增 sequence | `feishuPreviewHandle.sequence`，`feishu.go:3943-3951` |
| 流式元素 id 常量 | `richCardMainTextElementID = "main_text"`，`feishu.go:5197` |
| 引擎侧节流 + 结束补帧 | `core/engine.go:5380-5386`、`:5799-5805` |
| 整卡替换（终态卡） | `feishu.go:4758` `updateCardEntity` |
| 折叠面板（Reasoning / Tools / Updates） | `feishu.go:6470`、`:6533-6560` |
| 状态色 header 四态 | `feishu.go:4025-4043` `progressStateMeta` |
| emoji reaction 作为「处理中」 | `feishu.go:948-950`，`reaction_emoji` / `done_emoji` |
| 结构化进度卡 payload | `core/progress_compact.go:18-24` |

### 2.2 现有可配置项（真实键名）

**工具执行显示**

```toml
[display]
mode = "full"              # full | compact | quiet
tool_messages = true       # 工具事件是否产生输出
tool_max_len = 500         # 截断长度，0 = 不截断
thinking_messages = true
thinking_max_len = 300
card_mode = "legacy"       # legacy | rich —— 已实现，但未写进 config.example.toml 和 docs
```

Struct：`config/config.go:209-221`。合并逻辑 `EffectiveDisplay()` `config/config.go:842-943`，
`card_mode` 走单独的 `EffectiveCardMode()` `config/config.go:997-1012`。

支持按项目覆盖（逐字段）：

```toml
[[projects]]
name = "x"
[projects.display]
tool_messages = false      # 只覆盖这一个，其余继承全局
```

**平台侧**

```toml
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
progress_style = "legacy"  # legacy(默认) | compact | card —— progress card 唯一开关
enable_feishu_card = true  # false ⇒ 全部回退纯文本
```

解析在 `platform/feishu/feishu.go:281-425`（`map[string]any` 逐键断言，无 struct）。

**流式**

```toml
[stream_preview]           # 仅全局，无 per-project
enabled = true
interval_ms = 1500
min_delta_chars = 30
max_chars = 2000
disabled_platforms = []    # 已实现，未文档化

[projects.agent.options]
stream_partial_text = true # 仅 Claude Code；false ⇒ 无 token 级流式，打字机失效
```

### 2.3 硬编码、当前不可配

| 值 | 位置 | 说明 |
|---|---|---|
| ~~200ms / 20 字符~~ | ~~`core/engine.go:5383-5384`~~ | ✅ 已改为可配，见 Phase 1 |
| ~~1500ms / 30 字符~~ | ~~`core/engine.go:5380-5381`~~ | ✅ 已改为可配，见 Phase 1 |
| progress card 不节流 | `core/progress_compact.go:313` | 依赖 `ProgressUpdateInterval()`，无 in-tree 实现 ⇒ 实际为 0。**见 §5 末的独立修复** |
| progress card 上限 10 条 | `core/progress_compact.go:311` | |
| 消息分片 4000 | `core/engine.go:32` | |

> **原「关键」提醒已失效**：`card_mode = "rich"` 的富卡片路径过去完全不受配置控制，
> 现在由 `[display.streaming]` 管。`[stream_preview]` 仍只作用于 legacy 路径的
> `core/streaming.go` —— 两套配置并存且互不覆盖，这是 G6 要收敛的东西。

---

## 3. 差距清单（vs openclaw-lark）

| # | 差距 | 影响 | 成本 |
|---|---|---|---|
| ~~G1~~ | ~~卡片 config 只有裸 `streaming_mode`，缺 `streaming_config` 子字段~~ | ✅ **已修**，见 Phase 1 | — |
| G2 | 无 loading 指示器（openclaw 那两个点） | 长间隔（工具执行中）时卡片看起来「死了」 | 中（需上传图） |
| G3 | 无 `tool_detail` 三态，工具详略不可调 | 只能全开或全关（`tool_messages` 布尔），没有「只看摘要」档 | 中 |
| ~~G4~~ | ~~富卡片节流硬编码~~ | ✅ **已修**，见 Phase 1 | — |
| G5 | 进度卡拿不到打字机 | 走 inline card JSON，无 `cardID`，只能整卡 patch | 高（架构级） |
| G6 | 三套并行流式实现 | `core/streaming.go` / `engine.go` rich 分支 / `progress_compact.go` 各一套节流与生命周期 | 高 |
| G7 | 流式中途 markdown 可能截断 | 每帧推全量文本，无代码块/表格闭合保护 | 中 |
| G8 | `card_mode` 等 5 个键无法通过聊天命令持久化 | `SaveDisplayConfig` 只覆盖 5 个键（`config/config.go:1816-1845`） | 低 |

本文档覆盖 **G1–G4 + G8**，另含一项与本次对齐正交的既有 bug 修复（进度卡不节流，见 §5 末）。
G5/G6/G7 单列后续计划，见 §8。

**当前进度**：G1 + G4 已交付（Phase 1）。G2 / G3 / G8 待开工。

---

## 4. 目标配置面

设计原则：**尊重 core / platform 边界**。CLAUDE.md 禁止 core 硬编码平台名，所以
平台无关的语义放 `[display]`，飞书 CardKit 专有字段放 platform options。

### 4.1 `[display]` 新增

```toml
[display]
card_mode = "rich"           # legacy | rich
tool_detail = "summary"      # NEW: none | summary | full —— 工具执行详略
                             # none    = 完全不显示工具调用
                             # summary = 显示工具名 + 摘要（默认）
                             # full    = 额外显示原始输出 / exit code

[display.streaming]          # ✅ 已实装
throttle_ms = 200            # 服务端推送最小间隔（原硬编码 200）
throttle_chars = 20          # 最小新增字符数（原硬编码 20）
fallback_throttle_ms = 1500  # 整卡 Patch 回退路径（原硬编码 1500）
fallback_throttle_chars = 30 # 同上（原硬编码 30）
```

`[display.streaming]` 放在 `[display]` 下的好处：自动继承既有的
per-project 覆盖机制（`[projects.display.streaming]`），无需新写合并逻辑。

### 4.2 platform options 新增

```toml
[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
progress_style = "card"

# NEW: 飞书 CardKit 客户端渲染参数
card_print_frequency_ms = 50   # 客户端逐字间隔，对应飞书 streaming_config.print_frequency_ms
card_print_step = 1            # 每次打印字符数，对应 print_step
card_loading_icon = true       # 是否在流式卡片末尾挂 loading 指示器
card_loading_icon_img_key = "" # 自己上传的 img_key；留空 ⇒ 不挂（见 §6 陷阱 2）
```

### 4.3 快速配方

**「我只想要最接近 openclaw 的效果」**

```toml
[display]
card_mode = "rich"
tool_detail = "summary"

[display.streaming]
throttle_ms = 60

[[projects.platforms]]
type = "feishu"
[projects.platforms.options]
progress_style = "card"
card_print_frequency_ms = 50
card_print_step = 1
card_loading_icon = true
card_loading_icon_img_key = "img_v3_xxx"   # 换成你自己上传的
```

> ⚠️ 这份配方里的 `progress_style = "card"` 会走进**当前不节流**的进度卡路径
> （见 §5 末的独立修复）。在那一项修掉之前，这个配方在高频工具调用下有撞限频风险。

**「关掉所有工具执行显示」**

```toml
[display]
tool_detail = "none"
```

**「只在某个项目里开 full」**

```toml
[[projects]]
name = "debug-heavy"
[projects.display]
tool_detail = "full"
```

**「完全关掉流式，退回整块回复」**

```toml
[projects.agent.options]
stream_partial_text = false
```

---

## 5. 实施计划

按性价比排序。每阶段独立可交付、可回滚。

### Phase 1 — 流式节奏：节流可配 + `streaming_config`（G4 + G1）✅ 已完成

> 原 Phase 1 与 Phase 3 合并交付。理由见 §6 陷阱 1 —— 这是一个不可分的改动：
> 只改服务端节流，客户端渲染节奏不变；只改 `print_frequency_ms`，服务端喂不上帧。

**实际落地**（`go build ./...` + `go test ./...` 全绿，core/config/cmd/feishu 跑过 `-race`）：

| 交付 | 位置 |
|---|---|
| `StreamingDisplayConfig` 四字段 `*int`，逐字段 project 覆盖 | `config/config.go:239-251`、合并逻辑 `:1061-1085` |
| 默认值常量 200/20/1500/30 | `config/config.go:228-231` |
| 校验下限 `throttle_ms >= 20`、chars `>= 1` | `config/config.go:1199-1209`、`MinStreamThrottleMS` `:236` |
| core 侧 `StreamingCfg` + `normalizeStreamingCfg` | `core/engine.go:347-384`，装配点 `:948` |
| 跨包默认值漂移守卫 | `cmd/magpie/streaming_defaults_test.go` |
| 飞书 `streaming_config` 注入，`streaming bool` → `cardStreaming{enabled, printFreqMs, printStep}` | `platform/feishu/`，测试 `rich_card_streaming_config_test.go` |
| 新平台选项 `card_print_frequency_ms` / `card_print_step`（默认 50/1，`<=0` 与非数字在 `newPlatform` 报错） | `platform/feishu/feishu.go` |
| 联动告警（`throttle_ms > card_print_frequency_ms`） | `cmd/magpie/print_pacing.go`，表驱动测试 `print_pacing_test.go` |
| `config.example.toml` 双语文档 | `:160-172`、`:1133` |

几个值得记下的实现判断（都比原计划更严）：

- **零值不等于关闭节流**。`normalizeStreamingCfg` 对未设字段回落默认值 —— 否则漏配一个字段就等于
  取消节流，正好是要防的事故。
- **`chars` 下限是 1 而不是 0**。引擎是 OR 语义（时间到 **或** 字符数到就推），`chars = 0` 会让
  时间间隔彻底失效。这条原计划没写。
- **未设的 pacing 值省略而不是写 0**。`print_frequency_ms: 0` 会让客户端一次性吐完，
  恰好是本次要修的观感问题。
- **告警放在 `cmd/`**。那是唯一同时拿得到 display 配置和平台 options 的装配点；放 `config/`
  会让 config 认识飞书专有键，违反 CLAUDE.md 的边界要求。实现上不认平台类型，扫所有平台的
  options 找该键 —— 将来别的平台用同名选项自动获得这个检查。

### Phase 2 — `tool_detail` 三态（G3）

**Files:** `config/config.go`、`core/engine.go`、`core/i18n.go`、`platform/feishu/feishu.go`、`cmd/magpie/main.go`

> ⚠️ **本节的 `core/engine.go:NNNN` 锚点是 Phase 1 之前测得的。** Phase 1 在
> `core/engine.go:334-384` 插入了 `StreamingCfg` 等约 50 行，其后所有行号已下移。
> 开工前先 grep 定位，别直接按数字跳。

1. `DisplayConfig` 加 `ToolDetail *string \`toml:"tool_detail"\``，常量
   `ToolDetailNone / ToolDetailSummary / ToolDetailFull`
2. `EffectiveToolDetail()`，仿 `EffectiveCardMode()`（`config/config.go:997`）写法，
   非法值静默回落 `summary`
3. `core.DisplayCfg` 加 `ToolDetail string`，默认 `"summary"`
4. **删除 `tool_messages`**（见 §7 决策 3），涉及面比想象的广，逐处清：
   - `config/config.go` 的 `DisplayConfig.ToolMessages` 与 `EffectiveDisplay` 派生逻辑
   - `core/engine.go` 的 `DisplayCfg.ToolMessages` 及全部门控分支
   - `cmd/magpie/main.go:553`、`:1646` 装配；`:1204-1205` Management API 的
     `updates["tool_messages"]`
   - `core/i18n.go:334` `MsgStatusToolMessages` + `:2440` 五语言文案
   - `config.example.toml:127`、`:186`
   - `core/engine_test.go` 十余处 `ToolMessages:` 构造（改为 `ToolDetail:`）

   > 🚧 **`cmd/magpie/main.go:1204-1205` 这一处单独拦下，不随本步骤删。**
   > 那是 Management API 的 `updates["tool_messages"]`，属于**对外 HTTP 契约**，
   > 删掉是破坏性 API 变更，和删一个配置键不是一回事。用户拍板「直接删」时看的是配置面，
   > 未必知道底下还连着一个接口。需单独确认：跟着删，还是保留接口字段做映射到 `tool_detail`。
   > **未确认前不动这一小步。**
5. **门控落点**（`none` ⇒ 不显示，`summary` ⇒ 名称+摘要，`full` ⇒ additionally 原始输出）：
   - rich card 工具行：`none` ⇒ `break`
   - 工具消息渲染：`none` ⇒ 跳过
   - `EventToolResult`：仅 `full` 进入
   - 飞书工具面板：`summary` 只出 name + summary，`full` 带 result / exit code
6. 新命令 `/verbose [none|summary|full]`（命令名保留 openclaw 习惯，配置键用 `tool_detail`）：
   照抄 `cmdQuiet` 结构，但**裸 `/verbose` 只回报当前值不切换**（对齐 openclaw
   `directive-handling.impl.ts:219-227`）
7. 扩展 `SaveDisplayConfig`（`config/config.go:1816-1845`）支持 `tool_detail`（顺带修 G8）
8. `core/i18n.go` 补文案，**五语言（EN / ZH / zh-TW / JA / ES）齐全**

> **命名提醒**：`platform/feishu/feishu.go` 已有 `sanitizeToolDetail`（`:5722`）、
> `extractToolDetailFromJSON`（`:5600`）、`extractToolDetailFromSummary`（`:5652`）、
> `classifyCommandToolDetail`（`:5550`）—— 这些指的是「某次工具调用的详情文本」，
> 与配置项「显示多少详情」不是一回事。别在 feishu 包里新增裸 `toolDetail` 标识符，
> 门控变量建议叫 `detailLevel` 之类，避免两种含义在同一文件里互相干扰。
>
> 另注意 `extractToolDetailFromSummary` 里的 `Summary` 和新取值 `summary` 无关。

**测试**：三态各自的门控断言；`/verbose` 无参只读不写；持久化回写 `[display]` 后重载生效；
`tool_messages` 在全仓库无残留（可加一条 grep 断言或直接靠编译失败兜底）。

### `thinking_messages` 保持布尔（已决，不是待办）

`tool_messages` 删掉后，thinking 侧仍是布尔 `thinking_messages`，工具侧是三态 `tool_detail`。
**这个不对称是刻意保留的，不要"补齐"。**

理由：工具侧存在「只报名字 / 报完整参数与输出」这个真实的中间档，thinking 侧没有对应物。
强行对称等于为了配置表好看造一个没人要的取值。配置面的不对称如实反映了两件事本来就不一样。

### Phase 3 — loading 指示器（G2）

**Files:** `platform/feishu/feishu.go`

1. 常量 `richCardLoadingIconElementID = "loading_icon"`，与 `richCardMainTextElementID`（`feishu.go:5197`）放一起
2. `buildRichCardJSONBytes` 在 `streaming == true` **且** `card_loading_icon_img_key` 非空时，
   于元素列表末尾 append：

```go
map[string]any{
    "tag":     "markdown",
    "content": " ",
    "icon": map[string]any{
        "tag":     "custom_icon",
        "img_key": loadingIconImgKey,
        "size":    "16px 16px",
    },
    "element_id": richCardLoadingIconElementID,
}
```

3. 终态卡（`streaming == false`）**不加**该元素 —— 整卡替换时自然消失，不做显式隐藏
   （openclaw 同做法，`streaming-card-controller.js:615-621`）

**测试**：流式帧含该元素、终态帧不含；img_key 为空时不产生该元素（不能推空 key，飞书会报错）。

### 独立修复 — 进度卡不节流（与本次对齐正交，建议优先）

**这不是「对齐 openclaw 之后可以更好」，是当下就存在的撞限频风险。**

`ProgressUpdateThrottler` 只有接口声明（`core/interfaces.go:328`）和测试 stub
（`progress_compact_test.go:232`），**没有任何平台实现**。因此
`core/progress_compact.go:313` 的类型断言必然失败，`minUpdateInterval` 恒为 0 ——
进度卡的每次更新都直接打飞书 API，无任何节流。

影响面：默认 `progress_style = "legacy"` 时 writer 本身是 disabled 的，所以默认配置不受影响。
但 **§4.3 的推荐配方里就写了 `progress_style = "card"`** —— 也就是说，本文档推荐的配置会把
用户直接送进这条不节流的路径。这一项不修，Phase 1-3 做完反而更容易撞限频（流式更密集）。

**Files:** `platform/feishu/feishu.go`

1. 飞书实现 `ProgressUpdateThrottler`，返回值从 platform options 读
   （建议键名 `progress_throttle_ms`，默认 300）
2. 或者：把 `progress_compact.go:313` 的断言失败路径从「0」改成一个保守的内置默认值，
   不依赖平台实现

**倾向方案 1** —— 保持 core 不感知平台、由平台声明自己的能力，符合 CLAUDE.md 的边界要求。
但方案 2 更小、能立即消除风险，可以先落 2 再补 1。

**测试**：断言 `minUpdateInterval` 非 0；高频事件下实际 API 调用次数被压缩。

### 飞书实测验收要点

Phase 1 的观感效果**只能靠真机验证**，单测覆盖不到。实测前先明确要看什么，否则拿到一个
模糊印象没法转成决策。

**先记录实测用的是哪套参数。** 若 `[display.streaming]` 和 `card_print_frequency_ms`
都没配，走的是默认组合 200ms 推送 + 50ms/1 字符揭示 —— 那**不是** §4.3 推荐的配方，
而是 §6 陷阱 1a 描述的矛盾组合，且启动告警照不到。用默认跑出来的结论不能外推到推荐配方。

盯三件事：

1. **卡片文字与回复进度的相对位置。** 长回复（500 字以上）生成到一半时，卡片显示的内容
   是紧跟生成、还是明显落后？落后的话，差距是恒定还是越拉越大？
2. **收尾瞬间。** 终态整卡替换时，是自然衔接，还是「唰」地跳出一大段之前没显示的文字？
3. **工具执行间隙。** 卡片是静止但有内容，还是看起来卡死？（这条是 G2 loading 指示器
   要解决的问题，实测能确认它值不值得做。）

判读：

| 观察 | 结论 | 动作 |
|---|---|---|
| 跟得紧、收尾自然 | 客户端每帧重置揭示目标（陷阱 1a 的 A） | 默认值可用，§4.3 是锦上添花 |
| 越拉越大、收尾跳一大段 | 客户端逐字消费、欠账累积（陷阱 1a 的 B） | **默认值有害**，必须改默认或强制 §4.3 组合，并修告警盲区 |

⚠️ 如果出现 B，正确结论是「默认组合不对」而**不是**「`streaming_config` 没用」——
这两个结论会导向完全相反的动作，别搞反。

---

## 6. 已知陷阱

### 陷阱 1：`print_frequency_ms` 单独改无效

飞书客户端的 `print_frequency_ms: 50` 只是**客户端渲染上限**。服务端 200ms 才推一帧的话，
客户端每 200ms 才拿到新内容，50ms 的设置无从发挥 —— 观感不会有任何变化。

**这两个参数必须一起调**，且服务端要推得比客户端快：`throttle_ms <= print_frequency_ms`。
建议组合 `throttle_ms = 60` + `print_frequency_ms = 50`。

#### 1a. 默认组合本身就踩着这个陷阱，而且告警看不见

Phase 1 之后的默认值：

| 项 | 默认 | 来源 |
|---|---|---|
| `display.streaming.throttle_ms` | 200 | `config/config.go:228` |
| `card_print_frequency_ms` | 50 | `platform/feishu/feishu.go:207` |
| `card_print_step` | 1 | `platform/feishu/feishu.go:208` |

即**默认配置就是 `throttle_ms(200) > print_frequency_ms(50)`** —— 正是陷阱 1 描述的矛盾。
而 `clientPrintPacingWarning`（`cmd/magpie/print_pacing.go:27`）读的是
`p.Options["card_print_frequency_ms"]`，键未设置时 `optionAsInt` 返回 `ok=false` 直接
`continue`。**告警只检查显式配置的值，而生效值来自另一个包里的默认常量** —— 两者不是同一个数。
结果：最常见的路径（什么都不配）恰好是告警唯一照不到的地方。

算一下量级：`print_step=1` / `print_frequency_ms=50` ⇒ 客户端揭示上限 **20 字符/秒**。
服务端按 200ms 或 20 字符（OR 语义）推送，喂入可达 **100+ 字符/秒**。

这里有一个**我们尚未验证的关键未知**：飞书客户端在新帧到达时，是
(A) 把揭示目标重置为最新全文（则 `print_frequency_ms` 只是帧内平滑，默认组合无害），还是
(B) 按队列逐字消费（则欠账逐渐累积，长回复会越拖越慢，最后被终态整卡替换一次性跳到全文）。

**这决定了默认值是「平滑」还是「劣化」，必须靠实测区分**，不能靠读代码。见 §5 末的验收要点。

### 陷阱 2：`img_key` 不能照抄 openclaw 的

openclaw 那个 `img_v3_02vb_496bec09-...` 是**它自己的飞书应用**上传的资源。`img_key` 按
app 维度隔离，magpie 的应用引用会直接报错。

必须自己上传：`POST /open-apis/im/v1/images`，`image_type=message`，拿到 key 后写进
`card_loading_icon_img_key`。key 长期有效，一次性动作。

静态图也可以 —— `custom_icon` 不要求 GIF。

### 陷阱 3：`/verbose` 写全局，`[projects.display]` 会盖回来

`SaveDisplayConfig` **永远写全局 `[display]`**，不写 `[projects.display]`
（`config/config.go:1816-1845`）。而 `EffectiveDisplay()` 里 project 优先级更高。

后果：项目里设了 `[projects.display].verbose` 的话，聊天里 `/verbose` 改完当场生效
（只改运行时 engine），**重启后被 project 值盖回**。

`/quiet` 现在就有这个问题。文档里必须提醒，或在 `/verbose` 检测到 project 覆盖时回一条警告。

### 陷阱 4：终态卡可能残留光标

openclaw 收尾是**两步**：先 `setCardStreamingMode({streamingMode: false})` 关打字机，
再整卡替换（`streaming-card-controller.js:575-621`）。

magpie 的 `updateCardEntity`（`feishu.go:4758`）只改内容，**没有显式关 `streaming_mode`**。
如果终态卡上观察到残留光标，就是这里 —— 需要补一次 settings 调用，或确保替换用的卡片
JSON 里 `streaming_mode: false`（Phase 4 的 `streaming` 参数已经能表达这个）。

### 陷阱 5：`tool_detail` 与 `mode` 的 `full` 撞名

已通过改名化解 —— 键名从 `verbose` 改成 `tool_detail` 正是为了消掉这个歧义。见 §7 决策 1。
但两者仍各有一个 `full` 值，文档和 `/verbose` 输出里要写清楚它们管的不是一回事。

---

## 7. 待决问题 → 已全部拍板

1. **键名**：✅ 定为 **`tool_detail`**，取值 `none | summary | full`。
   放弃对齐 openclaw 的 `verbose` 命名 —— `[display]` 里同时出现 `mode = "full"`
   （工具事件独立成消息）和 `verbose = "full"`（显示工具原始输出）是真实歧义，
   而键名与 openclaw 一致对 magpie 用户没有价值：要的是效果一致，不是配置文件长得一样。
   **命令名仍用 `/verbose`**（肌肉记忆有价值），配置键与命令名不同名是刻意的。

2. **`mode` 与 `tool_detail` 的交互**：✅ 定为 **(a)** —— `mode` 是粗粒度总闸，
   `tool_detail` 是闸内的详略调节。`mode = "quiet"` 压制 `tool_detail = "full"`。

3. **`tool_messages` 去留**：✅ 定为 **直接删，不做兼容层**。
   依据：magpie 2026-09-22 重置 1.0.0、npm 仅单版本、用户近零，且上一轮刚移除过
   CC_ 兼容层，先例一致。清理清单见 Phase 2 第 4 步。

---

## 8. 后续计划（不在本次范围）

- **G5 进度卡走 cardkit 实体** —— 让进度卡也能打字机。需要 `progress_compact.go:466-490`
  从 inline card JSON 改成 `createCardEntity` + 元素推送，并给 `compactProgressWriter`
  引入 `cardID` / `sequence`。
- **G6 统一三套流式实现** —— 建议先把 rich card 定为唯一目标形态，另两套冻结为降级路径，
  再逐步收敛。当前继续在 rich 分支上加功能会让偏移扩大。
- **G7 流式 markdown 闭合保护** —— 每帧全量推送时，对未闭合的代码块 / 表格补临时闭合标记，
  避免流式中途渲染出半个代码块。

---

## 9. 文档产出

本次改动完成后需同步：

- ~~`config.example.toml` 补 `[display.streaming]`~~ —— ✅ Phase 1 已补（`:160-172`、`:1133`）
- `config.example.toml` —— 仍需补 `display.tool_detail`，以及**补上现存但至今缺失的**
  `display.card_mode` 和 `stream_preview.disabled_platforms`
- `docs/feishu.md` —— 新增「卡片流式效果调优」一节，含 §4.3 的快速配方
- `docs/usage.md` / `docs/usage.zh-CN.md` —— 命令表加 `/verbose`
- `core/i18n.go` —— 五语言文案（并删除 `MsgStatusToolMessages`）
