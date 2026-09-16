# Codex Desktop 功能调研 · 与 PocketAgent 的接入分析

> 调研日期：2026-09-16　基线：第 26 轮（commit `a53aef4`）
> 目标：弄清 Codex 桌面端「有哪些能力 / 设置项」，判断**哪些适合搬进手机遥控端**、
> **哪些不该搬**，并给出**不干扰现有功能**的接入方案与代码落点。
> 结论可直接作为实施排期的依据。

---

## 0. 一页结论（先看这张表）

| # | Codex 能力 / 设置项 | 建议 | 理由一句话 |
|---|---|---|---|
| 1 | `approval_policy`（审批策略） | ✅ 已接入 | 移动端最该有的安全闸门，现在已有 |
| 2 | `sandbox_mode`（沙箱模式） | ✅ 已接入 | 同上，与审批策略成对 |
| 3 | `model` + `model_reasoning_effort` | ✅ 已接入（建议补 2 档） | 已在"模型与思考"分区 |
| 4 | `mcp_servers`（MCP 服务器） | 🟡 只读展示已接，**写权限不建议给** | 手机上临时改 MCP 极易把桌面环境搞崩 |
| 5 | `model_providers`（模型提供方/网关） | ❌ 不接入 | 涉及 API Key，属于桌面独占敏感项 |
| 6 | `auth.json`（账号认证） | ❌ 不接入 | 触及凭证，只读展示账号名即可 |
| 7 | `personality` / `model_verbosity` | ✅ **建议接入**（高性价比） | 纯行为偏好，改了不破坏任何东西 |
| 8 | `model_reasoning_summary` / `hide_agent_reasoning` | ✅ **建议接入** | 直接缓解手机端"思考刷屏"痛点 |
| 9 | `model_auto_compact_token_limit` | 🟡 建议 P2 再做 | 偏专家项，可先只读或不展示 |
| 10 | `disable_paste_burst` | ✅ **建议接入** | 手机粘长文必踩的坑 |
| 11 | `features.multi_agent` / `network_proxy` | 🟡 建议只读展示 | 开关会改变 agent 行为边界，谨慎 |
| 12 | `agents.max_threads` / `max_depth` | 🟡 建议只读展示 | 调大在手机上收益小、风险高 |
| 13 | `notify`（桌面通知） | ❌ 不接入 | 桌面本地能力，与手机无关 |
| 14 | `history` / `CODEX_HOME`（会话存储） | ❌ 不接入 | 桌面文件系统细节 |
| 15 | 项目级 `.codex/config.toml` + 信任机制 | 🟡 **建议接入「查看」** | 手机上理解"这个项目为什么行为不同"很有用 |
| 16 | `profiles`（独立档案文件） | 🟡 建议只读展示 | 只读能解释现状，切换风险高 |
| 17 | 宠物 / 外观 / 快捷键 | ❌ 不接入 | 纯桌面 UI 偏好 |

**一句话总结**：**安全边界类（1/2/4）已接好；行为偏好类（7/8/10）是这次最值得补的增量；
凭证与提供方类（5/6）必须留在桌面。** 全部改动**只增不改**，不动任何现有字段。

> ⚠️ **最重要的架构前提（见 §1.2）**：本 daemon 有一条**红线 —— 绝不写
> `~/.codex/config.toml`**（历史上写过 `approval_policy`，直接改坏了用户的桌面端行为）。
> 现有设置全部走 "每轮 `turn/start` 透传 + 子进程内存态 `config/value/write`"。
> **新设置项必须沿用这条路径**——这既是安全约束，也意外带来了实现上的极大简化。

---

## 1. 关键事实：Codex 桌面端到底是什么

调研中两个容易搞错的前提，先澄清 —— 第二个**直接决定了本方案的架构**。

### 1.1 桌面端没有独立安装包

1. **没有独立安装的「Codex 桌面版」**。Codex 桌面端是以 **Codex 面板形式内嵌在 ChatGPT 桌面应用**里的
   （Windows/macOS 客户端）。它和 **Codex CLI 共用同一个 Codex 内核**。
2. **配置文件完全共享**：桌面端和 CLI 读的是**同一份** `~/.codex/config.toml`
   （Windows：`%USERPROFILE%\.codex\config.toml`）；认证走同一份 `~/.codex/auth.json`。
3. **配置在启动时读一次**，改完需要重启应用才生效。

### 1.2 ⚠️ 本项目有一条红线：daemon **绝不写** `config.toml`

这是本次调研**最重要的一条发现**，它推翻了"手机上直接编辑 config.toml"这个直觉方案。

`daemon/internal/agent/codex.go:676-681` 原文注释：

> **【红线】绝不写 `~/.codex/config.toml`**：那是用户桌面 codex 的全局配置文件，
> 之前把 `approval_policy` 写进去**直接改坏了桌面端行为**。

**现在是两条路由并行（都不碰磁盘配置文件）**：

| 路由 | 机制 | 生效范围 |
|---|---|---|
| A. 每轮透传 | `turn/start` 显式携带 `approvalPolicy` / `sandboxPolicy` / `model` / `effort` | **本轮立即生效**，可在会话数据里验证 |
| B. 内存写入 | app-server JSON-RPC `config/value/write` | **仅 daemon 拉起的子进程**（纯内存，不落盘） |

> A 路由的实现在 `codex.go:496-532`；B 路由在 `codex.go:699-720`
> （`config/value/write` 失败只打日志，因为"仅内存态，可忽略"）。

### 1.3 这对"接入新设置"意味着什么

**结论：新设置项也必须走"透传"，不能走"写配置文件"。** 好处比想象中大：

- ✅ **不污染用户桌面环境** —— 关掉 App，桌面端完全不受影响；
- ✅ **不需要"重启 Codex 才生效"** —— 每轮现拼参数，改完下一轮就变；
- ✅ 因此 §4.2 原本设想的"版本探测 + `supports_new_keys`"**不再必要**（见 §4.3 修订说明）。

**代价**：只对**本项目发起的轮次**生效，不影响用户在电脑上直接敲 codex。
对"手机遥控器"这个产品定位来说，这恰恰是正确边界。

---

## 2. Codex 设置项全清单（按 config.toml 分组）

### 2.1 安全与审批（已接入 ✅）

| 键 | 取值 | 现状 |
|---|---|---|
| `approval_policy` | `untrusted` / `on-request` / `never` | 设置页「审批策略」已实现（走 A 路由） |
| `sandbox_mode` | `read-only` / `workspace-write` / `danger-full-access` | 设置页「沙箱模式」已实现（走 A 路由，含 v1→v2 键名映射） |

**评估**：这两项是"手机遥控 agent"最核心的安全边界，必须放在手机首屏。
现有实现已完整（含 `SplitButton` 多选 + 审批弹窗预览），**不需要改动**。

### 2.2 模型与推理（部分已接入）

| 键 | 取值 | 现状 |
|---|---|---|
| `model` | 模型 slug | 已接入（含"跟随桌面配置"，`lastModelSeg` 处理带 `/` 的账号组 ID） |
| `model_provider` | provider 名 | ❌ 未接入（见 2.6） |
| `model_reasoning_effort` | `none`/`low`/`medium`/`high`/`xhigh`/`max` | 已接入（低/中/高/极高） |

**新发现**：官方枚举比我们实现的多两个值：
- `none` —— 完全关闭推理（最快）
- `max` —— 比 `xhigh` 更强

**建议**：把这两个值补进「思考强度」选项列表。**纯增量**，
因为走的是 A 路由（`startParams["effort"]`），daemon 只是透传字符串，
不认识的取值由 codex 侧报错或忽略，不会影响我们自己的代码路径。

### 2.3 行为偏好（**本次最值得接入的增量** ✅）

Codex 较新加入的"降噪 / 风格"配置，**纯行为偏好**：

| 键 | 取值 | 移动端价值 |
|---|---|---|
| `personality` | 如 `pragmatic` | ⭐⭐⭐ 直接决定回复是否啰嗦 |
| `model_verbosity` | `low` / `medium` / `high` | ⭐⭐⭐ 手机屏幕小，低冗长收益最大 |
| `model_reasoning_summary` | `concise` / `detailed` | ⭐⭐⭐ 控制思考过程长度 |
| `hide_agent_reasoning` | `true` / `false` | ⭐⭐⭐ 直接缓解"思考刷屏" |
| `model_auto_compact_token_limit` | 整数（token 数） | ⭐⭐ 手机端上下文更紧张 |
| `disable_paste_burst` | `true` / `false` | ⭐⭐ 手机粘长文必踩 |

**为什么这几项最适合手机**：手机端屏幕小、上下文窗口是硬约束。
`hide_agent_reasoning` + `model_reasoning_summary=concise` 组合起来，
能显著减少从 daemon 流式推过来的思考文本量 —— **同时改善可读性和传输开销**。

**✅ 走 A 路由的可行性（重要）**：这些键都能通过 app-server 的
`config/value/write`（B 路由）作用于**本 daemon 的子进程**，
和现有 4 个键用的是同一条已验证的路径 —— **实现上是"再加几行 kv"**，
不引入任何新机制。见 §4.3。

### 2.4 MCP 服务器（只读已接入 🟡）

```toml
[mcp_servers.<name>]
command = "..."
args = [...]
env = { ... }
```

**现状**：设置页「集成 · MCP 服务器」已能列出名称 + 工具数
（走 `McpStatus()` → app-server `mcpServerStatus/list`，`codex.go:730-742`）。

**建议**：**保持只读，不要给写权限。** 理由：
1. MCP 配置里通常带 `env`（API Key、token），手机上编辑等于把凭证暴露在小屏上；
2. MCP 服务器改错会让 agent 启动失败 —— 而 debug 只能回到电脑前，手机用户会卡死；
3. 收益极低：MCP 是"配置一次长期不动"的东西，不具备移动场景的即时性。

**可以增强的（低风险）**：代码里已经把 `status` 字段透传出来了
（`codex.go:109-112` 目前只取了 `name` 和 `tools`，**`status` 被丢弃**），
把它接到 proto 并在 UI 上显示 ready/starting/failed 即可 —— 让用户能**看出**某个 MCP 挂了，
然后回电脑修。这才是手机端应有的角色。

### 2.5 多智能体与并发（建议只读 🟡）

| 键 | 含义 | 建议 |
|---|---|---|
| `features.multi_agent` | 多智能体协作开关 | 只读展示 |
| `features.network_proxy` | 网络代理沙箱 | 只读展示 |
| `agents.max_threads` | 并发线程数 | 只读展示 |
| `agents.max_depth` | 嵌套深度 | 只读展示 |

都是"改变 agent 能力边界"的开关，调错了表现为"任务行为莫名其妙变了"，
排查成本高。手机端**展示**这些值能解释"为什么这台电脑上的任务行为不一样"，
但**不应提供修改入口**。

### 2.6 凭证类（**坚决不接入** ❌）

| 键 / 文件 | 说明 |
|---|---|
| `model_providers.<name>` | 含 `base_url` + `env_key`，指向第三方网关 |
| `openai_base_url` | 同上，顶层写法 |
| `auth.json` | `api_key` 或 ChatGPT OAuth `tokens` |
| `requires_openai_auth` | 是否把 ChatGPT token 发给第三方厂商 |

**必须留在桌面**，三个理由：

1. **安全**：`auth.json` 是长期凭证（官方本身就要求 `chmod 600`）。
   手机端能触发写操作 = 把凭证写权限放在丢失风险更高的设备上。
2. **可用性**：改 provider 会把应用切进 **API-key 认证模式**，
   **并禁用依赖 ChatGPT 账号的功能**。手机上误点一下就把桌面端功能打残。
3. **必要性**：provider / key 是"装机时配一次"的东西，没有移动端场景。

**额外理由（本项目特有）**：§1.2 的红线本身就是"曾经改坏过桌面行为"换来的教训。
凭证类只会比 `approval_policy` 更危险 —— **连尝试都不该尝试**。

**建议做法**：设置页「Codex 账户」分区**保持现状**（只显示账号名）。
**任何情况下不要把 key 传到手机上。**

### 2.7 桌面本地能力（不接入 ❌）

| 键 | 说明 |
|---|---|
| `notify` | 桌面本地通知 |
| `history.*` | 历史持久化配置 |
| `CODEX_HOME` | 配置目录位置（环境变量） |
| `tui.*` / 快捷键 / 主题 / 宠物 | 纯桌面 UI 偏好 |

与手机端无交互可能，跳过。

### 2.8 项目级配置与信任（**建议接入「查看」** 🟡）

**两层加载逻辑**（易踩坑，值得在手机上说明）：
- **全局**：`~/.codex/config.toml` —— 默认行为
- **项目级**：`<项目根>/.codex/config.toml` —— **只有项目被标记为 trusted 才加载**

**关键约束**：项目级**无法覆盖**敏感项（API Provider、Auth、网络代理、Telemetry），
这些只能在全局改。

**建议**：在会话页或设置里，对当前 workspace **只读展示**：
- 该项目是否存在 `.codex/config.toml`
- 是否处于 trusted

**价值**：直接回答一个高频困惑 —— "同一个 agent，为什么在这个项目里行为不一样"。

---

## 3. 接入优先级与排期建议

### P0 —— 本次就做（增量、零风险、收益直接）

1. **「行为与降噪」新分区**（设置页 Agent 子页内，新增 `SectionBlock`）
   - `hide_agent_reasoning`（开关）
   - `model_verbosity`（SplitButton：低/中/高）
   - `model_reasoning_summary`（SplitButton：精简/详细）
   - `disable_paste_burst`（开关）
2. **思考强度补齐 `none` / `max` 两个档位**（改现有选项列表，1 处）

> 这两项都走 §1.2 的 B 路由（给 `config/value/write` 加几行 kv），
> **实现量极小，且复用已验证路径**。

### P1 —— 紧随其后

3. **MCP 状态显示增强**（把已被丢弃的 `status` 接出来，区分 ready/starting/failed）
4. **项目级配置只读卡片**（有无 `.codex/config.toml` + trusted）

### P2 —— 需要更大改动，单独评估

5. `features.multi_agent` / `network_proxy` / `agents.*` 只读展示
6. `personality` 预设（需要产品上定义几档预设，避免让用户手打英文）
7. `model_auto_compact_token_limit` 数字可调

### P3 —— 明确不做

8. `model_providers` / `auth.json` / `notify` / `history` / 桌面 UI 偏好

---

## 4. 技术方案（保证不影响现有功能）

### 4.1 契约变更：只增字段，不改语义

在 `proto/v1/model.proto` 的 `CodexSettings` **追加**（编号顺延，绝不复用）：

```proto
message CodexSettings {
  string approval_policy = 1;   // 不变
  string sandbox = 2;           // 不变
  string account = 3;           // 不变
  string model = 4;             // 不变
  string effort = 5;            // 不变
  // ↓ 新增(字段号从 6 起,旧字段一个不动)
  bool   hide_agent_reasoning = 6;
  string model_verbosity = 7;
  string model_reasoning_summary = 8;
  bool   disable_paste_burst = 9;
}
```

`SetCodexSettingsRequest` 同样**追加**可选字段（编号 5 起）。

**为什么"只增不改"就够安全**：
- proto3 未知字段反序列化时被忽略，**老 daemon 收到新请求不会报错**；
- **新 App 连老 daemon** 时新字段恒为默认值（`""` / `false`），UI 走"该项不可用"分支；
- 现有 5 个字段的编号与语义一字未动 → **零回归风险**。

### 4.2 ~~版本探测（`supports_new_keys`）~~ —— **已作废，不需要**

> 原方案设想"写 config.toml 前要探测 Codex 版本，避免 `unknown field` 报错"。
> **§1.2 澄清后此风险不存在**：我们不写磁盘配置文件，
> 走的 `config/value/write` 只作用于 daemon 子进程内存态，
> 且现有代码对失败的处理就是**打日志后忽略**（`codex.go:717`）——
> 老版本 codex 不认识这个键，最坏结果只是"这项不生效"，**不会破坏任何东西**。
>
> 因此：**不做版本探测，不加 `supports_new_keys` 字段，UI 不需要门控分支。**
> 这是"红线设计"意外带来的简化收益。

### 4.3 写入路径：给现有机制加几行 kv

**唯一改动点**是 `codex.go:699-720` 那个 `for _, kv := range` 列表。
新增字段直接追加进去，和现有 4 个键完全同构：

```go
for _, kv := range []struct{ k, v string }{
    {"approval_policy", policy},
    {"sandbox_mode", sb},
    {"model", lastModelSeg(model2)},
    {"model_reasoning_effort", effort2},
    // ↓ 新增:行为与降噪(同样只作用于本 daemon 子进程)
    {"hide_agent_reasoning", boolStr(p.hideAgentReasoning)},
    {"model_verbosity", p.modelVerbosity},
    {"model_reasoning_summary", p.modelReasoningSummary},
    {"disable_paste_burst", boolStr(p.disablePasteBurst)},
} {
```

配套：`CodexProvider` 加 4 个字段 + 在 `SetDesktopSettings` 里按"非空才覆盖"的
既有语义接收（和 `approvalPolicy` 那 4 个一模一样）。

**为什么这是最安全的改法**：
- 与现有 4 个键**同一条路径、同一个错误处理**（失败仅日志）；
- 不新增 goroutine、不新增 RPC、不碰磁盘；
- 想把新功能"关掉"，删掉那 4 行 kv 即可回退。

### 4.4 App 侧：复用 `writeCodex` 的非空覆盖语义

现在 `writeCodex`（`SettingsScreen.kt:205-234`）已经是"只写非 null 字段"：

```kotlin
writeCodex(approval: String?, sandbox: String?, model: String?, effort: String?)
```

新字段用同样的模式扩展（参数表加 4 个可空参数，为 `null` 就不写）。
好处是**不会把用户没碰过的设置清空** —— 这套语义已验证过。

**另外 UI 要说明生效范围**：这些设置**只影响从 App 发起的任务**，
不会改动用户在电脑上直接使用 codex 的行为。这句提示要写进分区说明里，
否则用户会困惑"我在手机上关了思考，为什么电脑上还有"。

### 4.5 不改动的部分（明确列出，便于 review 核对）

- ✅ 不动 `CodexSettings` 现有 5 个字段的编号与语义
- ✅ 不动 `SetCodexSettingsRequest` 现有 4 个字段
- ✅ **不动"绝不写 config.toml"这条红线**（新功能同样不写磁盘）
- ✅ 不动 `turn/start` 的参数拼装逻辑（A 路由）
- ✅ 不动 `AgentConnection` 的连接/重连逻辑
- ✅ 不动 `ApprovalPreviewSheet`、`ModelPickerDialog` 的现有行为
- ✅ 不动任何账户/凭证相关代码路径
- ✅ 不动 `ListMcpServers` 的 RPC 形状（P1 只加 `status` 字段）

---

## 5. 代码落点（实施时直接按这张表改）

| 文件 | 位置 | 改动 |
|---|---|---|
| `proto/v1/model.proto` | `CodexSettings` 末 | 追加 6~9 号只读字段 |
| `proto/v1/model.proto` | `SetCodexSettingsRequest` 末 | 追加 5~8 号可选写字段 |
| `android/.../proto/v1/model.proto` | 同上 | **必须与 daemon 侧 MD5 一致** |
| `daemon/internal/agent/codex.go` | `CodexProvider` 结构体 ~`39` | 加 4 个字段 |
| `daemon/internal/agent/codex.go` | `SetDesktopSettings` ~`682` | 接 4 个新参数（非空才覆盖） |
| `daemon/internal/agent/codex.go` | kv 列表 ~`703-708` | **加 4 行 kv**（核心改动） |
| `daemon/internal/agent/codex.go` | `DesktopSettings` ~`724` | 一并回读新字段 |
| `daemon/internal/server/model.go` | `GetCodexSettings` ~`70-83` | 回填 4 个新字段 |
| `daemon/internal/server/model.go` | `SetCodexSettings` ~`85-97` | 透传 4 个新参数 |
| `SettingsScreen.kt` | `AgentPage` 内、MCP 分区之后 | 新增「行为与降噪」`SectionBlock` |
| `SettingsScreen.kt` | `writeCodex` ~`205-234` | 参数表扩展 4 个可空参数 |
| `SettingsScreen.kt` | 「思考强度」选项 ~`971-976` | 补 `none` / `max` |
| `SettingsScreen.kt`（P1） | MCP 分区 ~`998-1014` | 显示 `status`（需先接 proto 字段） |

---

## 6. 风险清单与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| 误写 `~/.codex/config.toml` | **改坏桌面端行为**（历史上已发生过） | 沿用既有红线：只走内存 `config/value/write`，不碰磁盘 |
| 老 Codex 不认新键 | 该项不生效 | 现有代码已"失败仅日志"，**无破坏性**；UI 标注"需较新 Codex" |
| 手机误改 provider / auth | 凭证泄露、功能被禁用 | **不实现**，代码层面就不存在这条路径 |
| 用户以为设置改了桌面 | 认知偏差、反复确认 | 分区说明里写明"仅影响从 App 发起的任务" |
| proto 两份文件不一致 | 编译通过但运行时解析错乱 | 改完立即 `md5sum` 比对两份 proto |
| 老 daemon + 新 App | 新字段恒为空，UI 出现空区块 | 新字段为空时**不渲染该行**（而不是渲染一个死开关） |

---

## 7. 附：参考资料

- Codex 配置基础 / 进阶 / 完整参考（官方 `developers.openai.com/codex/config-*`）
- `openai/codex` 仓库 `docs/config.md`
- 实测要点：桌面端与 CLI **共用** `~/.codex/config.toml` 与 `auth.json`；
  配置**启动时读一次**；provider 改动会切换认证模式并禁用账号相关功能。
- 本仓库既有约束：`daemon/internal/agent/codex.go:676-681`「绝不写 config.toml」红线。

---

## 8. 待你确认的点

1. **P0 的「行为与降噪」分区是否现在就做？** （复用现有路径、加 4 行 kv，我建议做）
2. `model_auto_compact_token_limit` 要不要做成可调数字？
   它更偏"专家项"，也可以只读展示或先不做。
3. `personality` 是否需要产品侧定义几档预设（务实/详细/…），避免暴露英文原值？
4. 是否需要我在**实施前**先做一次"老 daemon 兼容性"实测
   （新 App 连第 26 轮之前的 daemon），把结论补进本文档？

