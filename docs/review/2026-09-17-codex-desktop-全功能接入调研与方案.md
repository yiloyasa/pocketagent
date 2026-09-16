# Codex Desktop 全功能接入 PocketAgent · 调研与方案

> 日期:2026-09-17 基准:PocketAgent(Android + Go daemon + proto/v1),其 `codex-schema/` 为 Codex app-server 协议快照
> 调研对象:Codex 桌面端 / CLI 共用的 app-server JSON-RPC 协议(以本仓库 `codex-schema/` 两份 schema 为准)
> 方法:解析 `codex_app_server_protocol.v2.schemas.json`(579 个定义)与 `codex_app_server_protocol.schemas.json`,
> 得到完整方法面、通知面、反向请求面与配置面;再逐项比对 `daemon/internal/agent/codex*.go`、`daemon/internal/server/*.go`、
> `proto/v1/*.proto`、`SettingsScreen.kt` 的实际实现,得出覆盖率与接入判定。
> **与既有文档的关系**:`2026-09-16-codex-desktop-功能调研与接入分析.md` 是"config.toml 键"视角的调研,
> 其 P0(行为与降噪分区、思考强度补档)已经落地(见 §3.2)。本文是**协议全量视角**,是那份文档的超集,
> 不推翻它的结论,但把它没覆盖的六成协议面补全。

---

## 0. 结论摘要

| 维度 | 实测数字 | 我们的现状 |
|---|---|---|
| app-server 客户端方法 | **95 个** | 只用了 **9 个** |
| 服务端通知 | **75 条** | 处理了约 **21 条** |
| 服务端反向请求(需客户端应答) | **10 个** | 真正建模的只有 **1 个**(命令执行审批) |
| 配置项(`config/read`) | **23 个字段** | 已接 9 个 |
| 托管策略(`configRequirements/read`) | **约 25 项** | **完全没接** |

**三句话结论:**

1. **安全和行为偏好这两块已经做得很扎实**(审批策略、沙箱、模型/强度、降噪四项),这是最该做的部分,做完了。
2. 真正的短板不在"少接了几个设置开关",而在**三处实现缺陷**:审批被**坍缩成同一种"shell 命令"卡片**、**非审批类反向请求一律被拒绝**(会打断 token 刷新、MCP 询问、动态工具调用)、**`goal` 只存本地没下发给 Codex**。这三处不修,后面接什么都是歪的。
3. 最值得补的增量是**新的审批面**(guardian / 自动审查)、**用量与限流**(用户最关心的"还能不能用")、**会话组织**(分组 / fork / 回滚),以及**托管策略的正确尊重**(被管理员策略禁止的选项不该出现在手机上)。

---

## 1. 证据来源

| 来源 | 用途 |
|---|---|
| `codex-schema/codex_app_server_protocol.v2.schemas.json`(533KB,579 定义) | 方法面、通知面、`Config` 全字段、`ConfigRequirements` |
| `codex-schema/codex_app_server_protocol.schemas.json`(628KB) | `ServerRequest` 聚合(10 项,含 v1 遗留别名) |
| `codex-schema/ServerRequest.json`、`ServerNotification.json` | 反向请求与通知的方法名、报文字段 |
| `daemon/internal/agent/codex.go`(28.6KB)、`codex_events.go`(13.4KB) | 实际用了哪些方法、处理了哪些通知、审批怎么应答 |
| `daemon/internal/server/{thread,model,server}.go` | 我们的会话/模型/设置服务 |
| `proto/v1/model.proto`、`thread.proto` | 手机与 daemon 的契约现状 |
| `android/.../SettingsScreen.kt` | 设置页实际暴露了哪些项 |

---

## 2. 协议面全清点

### 2.1 客户端 → 服务端:95 个方法(按域分组)

| 域 | 方法 | 我们是否用 |
|---|---|---|
| **初始化** | `initialize` | ✅ |
| 会话生命周期 | `thread/start`、`thread/resume`、`thread/fork`、`thread/archive`、`thread/unarchive`、`thread/delete`、`thread/rollback`、`thread/close`、`thread/unsubscribe`、`thread/loaded/list`、`thread/read` | 用 2/11 |
| 会话元信息 | `thread/list`、`thread/name/set`、`thread/metadata/update`、`thread/inject_items`、`thread/shellCommand` | 用 1/5 |
| 会话分组 | `threadSection/list`、`threadSection/create`、`threadSection/update`、`threadSection/delete`、`thread/section/move` | 全未用 |
| 常驻目标 | `thread/goal/set`、`thread/goal/get`、`thread/goal/clear` | 全未用 |
| 压缩 | `thread/compact/start` | 未用 |
| 回合 | `turn/start`、`turn/steer`、`turn/interrupt` | 用 2/3 |
| 审查 | `review/start` | 未用 |
| guardian 审批 | `thread/approveGuardianDeniedAction` | 未用 |
| 模型 | `model/list`、`modelProvider/capabilities/read` | 用 1/2 |
| 配置 | `config/read`、`config/value/write`、`config/batchWrite`、`config/mcpServer/reload`、`configRequirements/read` | 用 1/5 |
| 实验特性 | `experimentalFeature/list`、`experimentalFeature/enablement/set` | 全未用 |
| 权限档案 | `permissionProfile/list` | 未用 |
| MCP | `mcpServerStatus/list`、`mcpServer/oauth/login`、`mcpServer/resource/read`、`mcpServer/tool/call` | 用 1/4 |
| 技能 | `skills/list`、`skills/extraRoots/set`、`skills/config/write` | 全未用 |
| hooks | `hooks/list` | 未用 |
| 插件与市场 | `plugin/list`、`plugin/installed`、`plugin/read`、`plugin/install`、`plugin/uninstall`、`plugin/skill/read`、`plugin/share/*`(6)、`marketplace/add|remove|upgrade` | 全未用 |
| 应用 | `app/list`、`app/read`、`app/installed` | 全未用 |
| 文件 | `fs/readFile`、`fs/writeFile`、`fs/readDirectory`、`fs/createDirectory`、`fs/getMetadata`、`fs/remove`、`fs/copy`、`fs/watch`、`fs/unwatch` | 全未用 |
| 命令执行 | `command/exec`、`command/exec/write`、`command/exec/terminate`、`command/exec/resize` | 全未用 |
| 账户 | `account/read`、`account/login/start`、`account/login/cancel`、`account/logout`、`account/rateLimits/read`、`account/rateLimitResetCredit/consume`、`account/usage/read`、`account/workspaceMessages/read`、`account/sendAddCreditsNudgeEmail` | 全未用 |
| Windows 沙箱 | `windowsSandbox/setupStart`、`windowsSandbox/readiness` | 全未用 |
| 外部 agent | `externalAgentConfig/detect`、`externalAgentConfig/import`、`externalAgentConfig/import/recordHistory`、`externalAgentConfig/import/readHistories` | 全未用 |
| 搜索与反馈 | `fuzzyFileSearch`、`feedback/upload` | 全未用 |

### 2.2 服务端 → 客户端:10 个反向请求(**这是审批的真身**)

| 方法 | 作用 | 现状 |
|---|---|---|
| `item/commandExecution/requestApproval` | 命令执行审批 | ✅ 已接(但被当成唯一形态) |
| `item/fileChange/requestApproval` | 文件变更审批 | ⚠️ 被当成"shell 命令"处理 |
| `item/permissions/requestApproval` | 权限提升审批(文件系统/网络) | ⚠️ 同上 |
| `applyPatchApproval`(v1 遗留) | 补丁审批 | ⚠️ 同上 |
| `execCommandApproval`(v1 遗留) | 命令审批 | ⚠️ 同上 |
| `item/tool/requestUserInput` | **agent 反问用户** | ❌ **一律自动拒绝** |
| `mcpServer/elicitation/request` | MCP 服务器询问用户 | ❌ **一律自动拒绝** |
| `item/tool/call` | 动态工具调用 | ❌ **一律自动拒绝** |
| `account/chatgptAuthTokens/refresh` | **刷新登录令牌** | ❌ **一律自动拒绝** |
| `attestation/generate` | 设备证明 | ❌ 自动拒绝 |

### 2.3 服务端通知:75 条(按主题合并)

| 主题 | 条数 | 代表 |
|---|---|---|
| 回合与条目 | 约 14 | `turn/started`、`turn/completed`、`item/started`、`item/completed`、`item/agentMessage/delta` |
| 推理过程 | 3 | `item/reasoning/textDelta`、`item/reasoning/summaryTextDelta`、`item/reasoning/summaryPartAdded` |
| 命令与进程 | 6 | `item/commandExecution/outputDelta`、`item/commandExecution/terminalInteraction`、`process/exited`、`command/exec/outputDelta` |
| 文件变更 | 3 | `item/fileChange/outputDelta`、`item/fileChange/patchUpdated`、`fs/changed` |
| 计划与 diff | 2 | `turn/plan/updated`、`turn/diff/updated` |
| 会话状态 | 约 14 | `thread/status/changed`、`thread/name/updated`、`thread/goal/updated`、`thread/queue/changed`、`thread/compacted` |
| 审批与审查 | 4 | `item/autoApprovalReview/started|completed`、`autoApprovalReview/strictReviewRequired`、`serverRequest/resolved` |
| MCP 与插件 | 5 | `mcpServer/startupStatus/updated`、`item/mcpToolCall/progress`、`app/list/updated` |
| 账户与用量 | 3 | `account/updated`、`account/rateLimits/updated`、`account/login/completed` |
| 告警 | 7 | `warning`、`guardianWarning`、`configWarning`、`deprecationNotice`、`windows/worldWritableWarning` |
| 语音实时 | 8 | `thread/realtime/*`(音频、转写、SDP) |
| 其他 | 约 6 | `model/rerouted`、`model/verification`、`turn/moderationMetadata`、`remoteControl/status/changed` |

### 2.4 配置面:`config/read` 的 23 个字段与关键枚举

```
analytics                     model_reasoning_effort      sandbox_workspace_write
approval_policy               model_reasoning_summary     service_tier
approvals_reviewer            model_verbosity             tools
compact_prompt                review_model                web_search
desktop                       sandbox_mode                model_context_window
developer_instructions        model_auto_compact_token_limit
forced_chatgpt_workspace_id   model_auto_compact_token_limit_scope
forced_login_method           instructions                model
```

| 键 | 取值 |
|---|---|
| `approval_policy` | `untrusted` / `on-request` / `never`(另有对象形态) |
| `sandbox_mode` | `read-only` / `workspace-write` / `danger-full-access` |
| `approvals_reviewer` | `user` / `auto_review` / `guardian_subagent` |
| `model_reasoning_summary` | `auto` / `concise` / `detailed` / `none` |
| `model_verbosity` | `low` / `medium` / `high` |
| `web_search` | `disabled` / `cached` / `indexed` / `live` |
| `model_auto_compact_token_limit_scope` | `total` / `body_after_prefix` |
| `personality`(线程级参数,不在 Config) | `none` / `friendly` / `pragmatic` |

**一个关键细节**:`model_reasoning_effort` 在 schema 里是**开放字符串**(`minLength: 1`,由模型自己声明可用档位),不是固定枚举。
所以手机端**不该硬编码档位列表**,应以 `model/list` 上报为准(见 §4.2)。

### 2.5 托管策略:`configRequirements/read`(约 25 项,**完全没接**)

```
allowedApprovalPolicies   allowedSandboxModes    allowedPermissionProfiles
allowedWebSearchModes     allowedWindowsSandboxImplementations
defaultPermissions        autoReview             featureRequirements
browserUse                computerUse            models
modelCatalogJson          chatgptBaseUrl         logDir
sqliteHome                checkForUpdateOnStartup
cliAuthCredentialsStore   allowAppshots          allowLoginShell
allowManagedHooksOnly     allowRemoteControl     enforceResidency
windowsSandboxPrivateDesktop
```

**这是本次调研里最被忽略、但影响最直接的东西**:企业/受管机器上,管理员会把这里配成"只允许用某几种审批策略/沙箱模式"。
手机端不看它,就会**给用户展示一批实际会被拒绝的选项** —— 用户点了没反应,只会觉得 App 坏了。

---

## 3. 覆盖率实测

### 3.1 我们已经用到的 9 个客户端方法

```
initialize            thread/start        thread/resume      thread/list
turn/start            turn/interrupt      model/list
mcpServerStatus/list  config/value/write
```

### 3.2 已经做扎实的部分(不要回退)

| 能力 | 落点 | 评价 |
|---|---|---|
| 审批策略 + 沙箱模式 | 设置页「权限与安全」,走每轮 `turn/start` 透传 | ✅ 正确 |
| 模型 + 思考强度 | 设置页「模型与思考」+ 会话内覆盖 | ✅ 正确 |
| 行为与降噪 4 项 | `proto/v1/model.proto:38-41`(6~9 号字段) | ✅ 2026-09-16 文档的 P0 已落地 |
| 「绝不写 `config.toml`」红线 | `codex.go` 注释 + 只走内存 `config/value/write` | ✅ **必须保持** |
| 流式文本 / 思考 | `agentMessage`+`delta`、`reasoning`+`delta` 的子串匹配 | ✅ 稳健(能自动兼容新变体) |
| 工具过程 | `item/started`、`item/completed`、`item/*/outputDelta`、`patchUpdated` | ✅ 基本够 |
| 额度告警 | `account/rateLimits/updated` | ✅ 已转发 |

### 3.3 三处必须修的实现缺陷

#### 缺陷 1:所有审批被坍缩成同一种"shell 命令"卡片

`codex.go:456-492` 的反向请求处理,只要方法名含 `approval` 就统一构造:

```go
call := pb.ToolCallBlock{
    Name: "shell",
    JsonArgs: mustJSON({"command": ..., "cwd": ..., "reason": ...}),
}
```

**后果**:文件变更审批、权限提升审批(v1 的 `applyPatchApproval` / `permissions`)也会被显示成一条 "shell 命令"。
用户在手机上看到的是**命令**,而实际要发生的是**改文件**或**放开网络访问** —— 他点"允许"时并不知道自己在允许什么。审批卡在这个形态下失去意义。

**改法**:按方法名分派,至少分四类(命令 / 文件变更 / 权限提升 / 补丁),`pb.ToolCallBlock.Name` 用真实类型,
并把该类型特有的字段(变更的文件列表、申请的读写路径与网络域)带出来。

#### 缺陷 2:非审批类反向请求一律被拒绝

```go
if !strings.Contains(strings.ToLower(method), "approval") {
    _ = c.Respond(id, map[string]string{"decision": "decline"})
    return
}
```

**这一句同时打死了四类请求**,后果逐条都真实:

| 被拒的请求 | 真实后果 |
|---|---|
| `account/chatgptAuthTokens/refresh` | **Token 刷新被拒** —— 长会话可能掉登录态;且应答形状也不对(它期望 tokens,不是 decision) |
| `item/tool/requestUserInput` | agent 想问用户一个问题,被静默拒绝 → 只能猜,或直接失败 |
| `mcpServer/elicitation/request` | 同类问题,发生在 MCP 侧 |
| `item/tool/call` | 动态工具调用被拒 |

**改法**:改成**按方法名白名单分派**。登记的转给手机(命令/文件/权限转审批卡,反问类转"agent 在问你"的输入卡);
未登记的才拒绝,且**拒绝时要用该请求期望的应答形状**。

#### 缺陷 3:`goal`(常驻目标)只存本地,没下发给 Codex

`server/thread.go` 的 `SetThreadGoal` 只做 `t.Goal = req.Goal` 后落库。而 Codex 有完整的
`thread/goal/set`、`thread/goal/get`、`thread/goal/clear`,外加状态机
(`active` / `paused` / `blocked` / `usageLimited` / `budgetLimited` / `complete`)、
`tokenBudget`、`tokensUsed`、`timeUsedSeconds`,并有 `thread/goal/updated` / `thread/goal/cleared` 通知。

**后果**:手机端设的"常驻目标"只在我们自己的库里,Codex 根本不知道。表现就是"我明明设了目标,它好像没当回事"。

---

## 4. 逐域接入判定

| # | 域 | 判定 | 一句话理由 |
|---|---|---|---|
| 1 | 审批策略 / 沙箱模式 | ✅ 已接,保持 | 移动端最核心的安全闸门 |
| 2 | 四种审批形态分派 | 🔴 **必修** | 现在审批卡在误导用户(缺陷 1) |
| 3 | 反向请求白名单 | 🔴 **必修** | 现在会打断 token 刷新与 agent 反问(缺陷 2) |
| 4 | `goal` 下发 | 🔴 **必修** | 现在是个假开关(缺陷 3) |
| 5 | 模型 + 思考强度 | ✅ 已接,建议改**动态档位** | 见 §4.2 |
| 6 | 行为与降噪 4 项 | ✅ 已接 | 上一轮成果 |
| 7 | **guardian / 自动审查** | 🟡 **建议接(P1)** | 新审批面,不接会"任务莫名失败" |
| 8 | **用量 / 限流 / 积分** | 🟡 **建议接(P1)** | 用户最关心"还能不能用" |
| 9 | **`configRequirements`** | 🟡 **建议接(P1)** | 不看它就会展示无效选项 |
| 10 | 会话分组 `threadSection/*` | 🟡 建议接(P2) | 手机端"项目分组"的天然抓手 |
| 11 | `thread/fork` / `thread/rollback` | 🟡 建议接(P2) | 移动端纠错场景真实存在 |
| 12 | 技能 / hooks / 插件可见性 | 🟡 只读(P2) | 回答"它到底有哪些能力" |
| 13 | MCP 状态(`status` / `authStatus`) | 🟡 增强只读(P1) | 现有实现把 `status` 丢了 |
| 14 | `command/exec`(带 tty) | 🟡 评估(P3) | 可能替代自研 PTY,见 §4.8 |
| 15 | `fs/*`(app-server 自带文件 API) | ❌ 不用 | 我们已有 gRPC FS,重复建设 |
| 16 | `fuzzyFileSearch` | 🟡 可选(P3) | 手机端跳文件,但 FilesScreen 已有路径 |
| 17 | `review/start` | 🟡 可选(P3) | "审查代码"是好场景,收益待评估 |
| 18 | `windowsSandbox/*` | 🟡 只读状态 | Windows 用户会真遇到 |
| 19 | `externalAgentConfig/*` | 🟡 值得研究(P3) | 官方支持从别的 agent 导入配置,见 §7 |
| 20 | `thread/realtime/*`(语音) | ❌ 不接 | 8 条通知全是音频流,与"遥控器"定位无关 |
| 21 | `model_provider` / `auth.json` / 凭证 | ❌ **坚决不接** | 上一份文档已论证,继续成立 |
| 22 | `desktop` / UI 偏好 / `notify` | ❌ 不接 | 桌面本地 |
| 23 | `account/login` / `logout` | ❌ 不接 | 登录是桌面独占动作 |

### 4.1 审批新面:guardian 与自动审查(P1,最值得补)

Codex 支持 `approvals_reviewer` 为 `auto_review` 或 `guardian_subagent` —— **由一个审查者先替用户判断**。相关面:

- 通知 `item/autoApprovalReview/started` / `completed`,携带 `GuardianApprovalReview`:`rationale`、`riskLevel`、`status`;
- `status` 取值 `inProgress` / `approved` / `denied` / `timedOut` / `aborted`;
- `autoApprovalReview/strictReviewRequired`(需要严格审查);
- `guardianWarning`(我们**已经**在转发,说明这条路踩到过);
- 客户端方法 `thread/approveGuardianDeniedAction` —— **审查者拒绝之后,用户可以推翻**。

**为什么必须接**:用户的 `approvals_reviewer` 一旦不是 `user`(他自己在桌面上设的),我们的审批卡就再也不会弹,
而动作可能**被自动拒绝**。手机端不展示"审查者为什么拒、风险等级多少、你可以推翻",用户看到的就是"任务莫名其妙失败"。

**做法**:把 `item/autoApprovalReview/*` 转成"审查中 / 被拒"的状态卡,展示 `rationale` 与 `riskLevel`;
被拒时给"仍然执行"按钮,走 `thread/approveGuardianDeniedAction`。

### 4.2 模型与思考强度:改为动态档位

以 `model/list` 返回的模型能力为准生成档位选项(顺带保留已有的 `inputModalities` → `supportsImages` 映射)。
好处:模型升级或换账号时不用改 App。

### 4.3 用量、限流与积分(P1)

- `account/rateLimits/read` → `RateLimitSnapshot`:`primary` / `secondary` 窗口(`usedPercent`、`resetsAt`、`windowDurationMins`)、`planType`、`credits`、`spendControlReached`、`rateLimitReachedType`;
- `account/usage/read`、`account/rateLimitResetCredit/consume`(用积分重置额度);
- 通知 `account/rateLimits/updated` **已接**,但只当告警转发,**没落成可查看的面板**。

**建议**:会话页/设置页加"额度"卡片:当前窗口用了多少、什么时候重置、有没有触顶。用户在外面发任务前,得先知道还有没有额度。

### 4.4 托管策略必须尊重(P1)

1. 启动或进设置页时读一次 `configRequirements/read`;
2. `allowedApprovalPolicies` / `allowedSandboxModes` / `allowedWebSearchModes` 里没有的取值,**不渲染**或标灰并说明原因;
3. 避免"用户点了、Codex 拒绝、App 无反馈"。

### 4.5 技能、hooks、插件(P2,只读)

`skills/list`(`SkillSummary`:`name` / `description` / `enabled` / `path`)、`hooks/list`、
`plugin/list`(字段丰富:`enabled`、`installed`、`version`、`authPolicy`、`disabledReason`)、`marketplace/*`,
通知 `skills/changed`、`app/list/updated`。

**只读**。价值在于回答"这台电脑上这个 agent 到底有哪些能力 / 为什么某个技能不生效"。
`disabledReason` 特别值得展示 —— 它直接解释"为什么用不了"。

### 4.6 实验特性与权限档案(只读优先)

`experimentalFeature/list`(`ExperimentalFeature`:`name` / `enabled` / `defaultEnabled` / `stage` / `announcement`)、
`permissionProfile/list`(`PermissionProfileSummary`:`id` / `description` / `allowed`)。
写入口 `experimentalFeature/enablement/set` **不建议给手机** —— 这类开关改行为边界,排查成本高。

### 4.7 会话组织:分组 / 目标 / 分支(P2)

| 能力 | 说明 |
|---|---|
| `threadSection/*` | 侧栏分组(`ThreadSection`:`id` / `name` / `appearance`),`thread/section/move` 移动;`thread/list` 支持 `sectionId` 过滤 |
| `thread/goal/*` | 常驻目标 + 状态机(见缺陷 3) |
| `thread/fork` | 从某个回合分叉(`lastTurnId`) |
| `thread/rollback` | 回滚 N 个回合(`numTurns`) |
| `thread/compact/start` | 手动压缩(我们现在的 `CompactThread` 是空实现) |
| `thread/list` 的隐藏能力 | `searchTerm`、`archived`、`cursor`、`sortKey` / `sortDirection`、`sourceKinds`、`modelProviders`、`useStateDbOnly` —— 我们只用了最简调用 |
| `thread/read` / `thread/loaded/list` | 读单个线程全文 / 列出当前已加载线程 |

**手机端最有价值的三个**:分组(对应"项目")、搜索、回滚(手机上点错了想撤销,比在电脑上操作更自然)。
`fork` 与 `rollback` 都是破坏性/分叉性操作,必须二次确认并说清后果。

### 4.8 `command/exec`:值得评估的替代方案(P3)

app-server 自带命令执行接口:`command/exec` 支持 `tty: true`、`streamStdin`、`streamStdoutStderr`,
配 `command/exec/write` / `terminate` / `resize` 与 `command/exec/outputDelta` 通知。

我们现在的终端是**自研 PTY**(`daemon/internal/term/manager.go` + xterm.js),工作正常、有独立价值。
**长期**值得评估:若 `command/exec` 能覆盖 PTY 场景,终端页可少维护一套实现,且天然与 Codex 沙箱策略一致。

**现在是"记录待评估",不是"替换计划"。** 现有终端没有坏,不要为了统一而重写。

### 4.9 明确不接的部分

| 项 | 理由 |
|---|---|
| `model_provider` / `openai_base_url` / `auth.json` / `cliAuthCredentialsStore` | 凭证类。改了会切换认证模式并禁用账号功能,手机误点即打残桌面端 |
| `account/login/*` / `logout` | 登录是桌面独占动作 |
| `notify` / `desktop` / 主题 / 宠物 / 快捷键 | 纯桌面 UI |
| `thread/realtime/*`(8 条通知) | 语音实时会话,与产品定位无关 |
| `fs/*` | 已有 gRPC FS 服务,重复建设 |
| `attestation/generate` | 设备证明,桌面独占 |

---

## 5. 交互设计

### 5.1 审批卡必须按形态分化(缺陷 1 的对症方案)

| 形态 | 卡片该显示什么 | 主要动作 |
|---|---|---|
| **命令执行** | 命令全文(等宽、可复制)、工作目录、理由 | 允许一次 / 总是允许 / 拒绝 |
| **文件变更** | 受影响文件列表 + 差异预览(复用 `GitScreen` 的 diff 渲染) | 允许 / 拒绝 |
| **权限提升** | 申请的是哪些 `read` / `write` 路径、哪些网络域 | 允许一次 / 拒绝 |
| **agent 反问** | 问题原文 + 候选项(`ToolRequestUserInputQuestion` 已有 `label` / `description`) | 选择 / 自由输入 / 跳过 |

另外两条:

- **拒绝必须能带说明**:`ApprovalDecision.note` 字段已经有了,但 UI 上没有让用户填的地方。补一个可选输入框 —— 拒绝理由会进 agent 上下文,能显著减少来回。
- **`item/commandExecution/terminalInteraction` 完全没处理**:命令运行中可能要用户输入(`sudo` 密码、`y/n` 确认)。不处理就是**命令永久挂住**。至少先做成"命令在等输入"的可见状态 + 输入框。

### 5.2 键盘与输入

- **聊天输入区运行中应可用**:`turn/steer` 协议已存在,现在 UI 却把输入框锁死(`enabled = !st.running`)。改成运行中发送 = 补充指令。
- **审批尽量点按**:允许/拒绝是按钮;只有"反问"类才需要打字。
- **终端辅助键栏**:Codex 侧是自研 PTY + xterm,缺 `Ctrl` / `Esc` / 方向键的软键盘入口。需要补一行可横向滚动的辅助键栏,并采用**粘滞修饰键**设计:点 `Ctrl` 高亮,下一次按键带上修饰后自动复位,连点两次锁定(触感与视觉反馈沿用现有动效语言)。
- **命令全文必须可复制**:Codex 生成的命令经常需要用户在电脑上核对。

### 5.3 长任务可见性

`turn/plan/updated` 已经接到 `CodexEvent.plan_steps`,这是 Codex 的一个优势,保持。需要补:

- **`turn/diff/updated`**:一轮累计的改动 diff。回答"它到底改了什么",现在没接;
- **审查状态**(`item/autoApprovalReview/*`):让"为什么卡住了"有答案;
- **`serverRequest/resolved`**:用来清掉手机上已展示但已过期/已被自动处理的审批卡,避免"幽灵审批卡";
- **额度**:发起长任务前先看得到还能不能用。

---

## 6. 会话管理

### 6.1 现状

我们只能 `thread/list`(最简参数)、靠本地 store 的 `CodexThreadID` 做映射、`thread/name/updated` 同步标题、
删除时清 rollout 文件。**分组、搜索、fork、回滚、手动压缩一律没有。**

### 6.2 建议

| 能力 | 做法 |
|---|---|
| 分组 | 读 `threadSection/list` 做映射;`thread/list` 带 `sectionId` 过滤。手机上呈现为"项目分组" |
| 搜索 | `thread/list` 直接带 `searchTerm`(别在手机端本地过滤,那样只能过滤已加载的页) |
| 分页 | 已有 `cursor` + `NextCursor` 循环;**列表页要限制页数**,别一次拉完几千条 |
| 归档 | `thread/archive` / `unarchive` 未用;现在"归档"只作用于我们自己的列表 → 语义不一致 |
| fork | 二次确认 + 说明"会新建一个会话" |
| 回滚 | `thread/rollback(numTurns)` 是破坏性操作,必须强确认 |
| 手动压缩 | `CompactThread` 从空实现改为调 `thread/compact/start` |
| 目标 | 修缺陷 3:改用 `thread/goal/*`,订阅 `thread/goal/updated` |

**一条原则**:能由 Codex 侧作为真相的,就不要在我们这边存第二份 —— `goal` 就是现成的反例。

---

## 7. 多 agent 与会话呈现

### 7.1 一个官方就有的能力,值得单独提

app-server 提供 `externalAgentConfig/detect`(探测可导入的外部 agent 配置)、`externalAgentConfig/import`,
配套 `import/progress` / `completed` 通知与 `import/readHistories` 历史记录。

**这说明"从别的 agent 搬配置"在 Codex 侧是一等公民。** 对多 agent 议题有两点价值:一是未来要做
"把其他 agent 的配置导入 Codex",**不用自己发明机制**;二是反向不成立 —— 它是"导入到 Codex",不是"Codex 导出给别人"。

**建议**:先只做"探测 + 只读展示",写入放到后面单独评估。

### 7.2 会话列表的归属问题

多 agent 情况下,会话列表必须能区分归属。Codex 侧的落点:

- 会话行标 `AgentID = "codex"` 徽标;
- `providerFor()` 在找不到对应 provider 时会**静默回退到默认 provider** —— 必须改成显式失败或明确降级提示,否则会出现"选了 A 却发给 Codex";
- "电脑上已有会话"列表已有实现(`ListCodexThreads`),补上**分组信息**(`sectionId` / `threadSection`)就能与桌面端侧栏对齐 —— 这是 Codex 的一个优势:它在协议里有分组概念。

---

## 8. 连接可靠性

现有能力(候选回退链、`NetworkCallback`、指数退避、`connectionEpoch`、前台保活、
**轮次用 `context.Background()` 与手机连接解耦**)对 Codex 已经够用,**不需改**。需要补的具体点:

| 点 | 说明 |
|---|---|
| `gen` + `thread/resume` 握手 | 已有,是踩过坑换来的(子进程重启后不 resume 会被静默吞消息、永远转圈)。**后续改动不要绕过它** |
| `serverRequest/resolved` | 反向请求已解决 —— 目前没处理,是"幽灵审批卡"的来源 |
| `thread/status/changed` | 已有,是"任务是否还在跑"的权威来源。**重连后应以此为准**,不要靠本地计时器猜 |
| 子进程崩溃 | 现在是自动重启一次;重启后所有在跑的 turn 都断了 —— 应主动发 `TurnFailed` 并置 IDLE,**不要让手机继续转圈** |
| `configWarning` / `deprecationNotice` | 能收到但未见处理;值得转成"设置没生效"的提示,否则用户改了设置没反应会以为是 App 的问题 |

---

## 9. 断网可见

结论与 `2026-09-16-全面审查-缓存与离线.md` 一致:**现在零内容缓存,断线即失明**。
那份文档已给出 C3 / O1 / O2 方案,这里补 Codex 独有的三点:

1. **`turn/diff/updated` 是做离线快照的好材料** —— "我离开后它改了什么文件"比完整对话更值得优先缓存;
2. **Codex 没有内置的会话导出接口**。所以"导出会话存档"这条路要么自己渲染 HTML,要么跳过 —— 协议里没有现成能力可用;
3. **额度快照同样要缓存**:断网时至少能知道"上次看到的额度与重置时间"。

优先级仍按原文档:L1(列表 + 最近消息快照,成本低)> 额度与 diff 快照 > 其他。

---

## 10. 安全边界

| 事项 | 处置 |
|---|---|
| **不写 `~/.codex/config.toml`** | **红线,继续守**。新设置一律走"每轮透传 + 子进程内存 `config/value/write`" |
| 凭证类 | 不实现,代码层面不存在这条路径 |
| `configRequirements` | 尊重管理员上限,不给无效选项(§4.4) |
| `experimentalFeature/enablement/set` | 不给手机写入口 |
| 链路安全 | 手机端 token 明文 + gRPC 明文(`usePlaintext`)是既有债。**Codex 侧尤其严重**:它能在电脑上执行任意命令。建议尽快做 TLS + `EncryptedSharedPreferences` |
| 审批默认 | 未识别 / 超时 / 手机断线时**默认拒绝**(现状正确,保持) |
| `item/tool/call` | 现在一律拒绝。改成白名单后要**特别小心** —— 这是让 agent 调用动态工具的入口,放行需要明确的用户动作 |

---

## 11. 分阶段实施与验收

### P0 —— 修三处缺陷(不加新功能)

1. 审批按方法名分派成四类,`ToolCallBlock.Name` 用真实类型(缺陷 1);
2. 反向请求改白名单:识别 `requestUserInput` / `mcpServer/elicitation` / `authTokens/refresh`,并**按各自期望的应答形状**回复(缺陷 2);
3. `goal` 改走 `thread/goal/*` 并订阅 `thread/goal/updated`(缺陷 3)。

**验收**:① 触发一次文件变更审批,手机显示"文件变更 + 文件列表"而不是"shell 命令";
② 长会话不因 token 刷新被拒而掉登录;③ 手机上设的目标在电脑端 Codex 里可见,且状态变化能同步回手机。

### P1 —— 补齐最影响体验的三块

4. guardian / 自动审查:状态卡 + `thread/approveGuardianDeniedAction`;
5. 额度面板:`account/rateLimits/read` + 积分重置;
6. `configRequirements` 门控选项;MCP 补 `status` / `authStatus`;`terminalInteraction` 不再挂死。

**验收**:`approvals_reviewer` 设为 `guardian_subagent` 时,被拒动作在手机上能看到理由与风险等级并能推翻执行;
受管机器上设置页不出现被策略禁止的选项。

### P2 —— 会话组织与结构

7. 分组(`threadSection/*`)、搜索(`searchTerm`)、归档语义对齐、手动压缩、fork / 回滚(带强确认);
8. 技能 / hooks / 插件只读页(含 `disabledReason`)。

### P3 —— 评估类(先出结论再动手)

9. `command/exec` 与自研 PTY 的取舍评估;
10. `review/start`、`fuzzyFileSearch`、`externalAgentConfig` 探测的收益评估。

### 贯穿全程

11. 离线快照(复用既有 C3 / O1 / O2 方案)+ `turn/diff/updated` 优先缓存;
12. TLS + token 加密;
13. 契约变更一律**只增字段不改语义**,两份 proto 改完立即比对 MD5。

---

## 12. 待决策

| # | 问题 | 备选 |
|---|---|---|
| D1 | 实验特性列表 | 只读展示 / 给写入口(建议只读) |
| D2 | fork / 回滚 | 都做 / 只做回滚(更贴近"手机上点错了") / 都不做 |
| D3 | guardian 被拒后的"仍然执行" | 直接给按钮 / 需二次输入确认 |
| D4 | 归档语义 | 对齐 Codex(`thread/archive`) / 保持只影响手机列表 |
| D5 | `command/exec` | 现在就评估 / 记为长期候选(建议后者) |
| D6 | 插件与技能 | 只读 / 完全不展示 |

---

## 13. 代码落点

| 文件 | 位置 | 改动 |
|---|---|---|
| `daemon/internal/agent/codex.go` | `ReqHandler` ~456-492 | **P0 核心**:按方法名分派四类审批 + 白名单化非审批请求 |
| `daemon/internal/agent/codex.go` | `handleNotif` 内 | 新增 `item/autoApprovalReview/*`、`serverRequest/resolved`、`turn/diff/updated`、`thread/goal/updated` |
| `daemon/internal/agent/codex.go` | 反向请求应答处 | 按请求类型给正确应答形状(尤其 `authTokens/refresh`) |
| `daemon/internal/server/thread.go` | `SetThreadGoal` / `CompactThread` | 改走 `thread/goal/set` 与 `thread/compact/start` |
| `daemon/internal/server/model.go` | 新增 RPC | 额度、托管策略、技能/插件只读 |
| `proto/v1/model.proto` | 追加 | 额度、托管策略、审查状态等**只读**字段(编号顺延) |
| `proto/v1/thread.proto` | `TurnServerMessage` | 新增审查状态 / 用量 / diff 的消息位;`ToolCallBlock` 承载真实审批类型 |
| `android/.../proto/v1/*.proto` | 同上 | **必须与 daemon 侧一致** |
| `SettingsScreen.kt` | 「权限与安全」~937 / 「集成 · MCP」~1180 | 托管策略门控;MCP 显示 `status` / `authStatus`;新增「额度」分区 |
| `ChatScreen.kt` | 审批卡渲染 / 输入区 ~1664 | 四形态分化 + 拒绝理由输入;运行中可用(`turn/steer`) |
| `TerminalScreen.kt` + `terminal.html` | — | 辅助键栏(与其他 agent 共用) |

---

## 14. 一句话总结

**Codex 这块的地基是对的 —— 红线守住了、安全与降噪该接的接了、流式与工具过程也稳。真正的欠账不是"少接了几个开关",
而是三处会让用户被误导或任务中断的实现缺陷:审批坍缩成一种卡片、非审批反向请求一律拒绝、`goal` 只存本地。
先把这三处修掉,再补 guardian 审查、额度面板和托管策略门控 —— 这三样分别回答用户在外面最想知道的
"它为什么停了"、"我还能不能用"、"为什么这个选项点了没用"。**
