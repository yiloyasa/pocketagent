# 声明式运行时适配器 + 瘦契约设计

日期 2026-10-13 ｜ 状态 草案 ｜ 范围 daemon `internal/agent` + `proto/v1`（只增不改，冻结 93 RPC）
目标：把“加一家 / 跟一次上游 breaking = 手写 adapter + 全链改 N 处”换成“改一份声明 + 网关运行时解释执行”。
本文所有行号均为 2026-10-13 实测（`main`，daemon 0.3.1 / 93 RPC）。

## 0. 结论先行（给审核看的三句话）

1. 现状加一个 codex 反向请求 method 要动 **9 类位置**（§1 实测清单）；加一个通知 method 要动 **6 类位置**，其中 proto 加 Kind 即甲级（`AUDIT.md:21`）。
2. 本设计只做两件事：**A. 网关 + 方法声明表**（method→卡类型→应答枚举→透传字段→UI schema，全部数据化）；**B. 瘦契约**（冻结 93 RPC，所有未来 method 只走新增的 1 个 `GenericEvent` oneof 分支 + 既有能力发现通道，不再加 RPC / 不再加 `CodexEvent.Kind`）。
3. 未知 method 的默认行为与辅线 B 的关系不变：**B 关 = 今天的行为逐字节一致；B 开 = 通用确认框是声明表的 `fallback` 条目，不是第五套手写分支**（§3）。

---

## 1. 现状成本实证：为什么手写 adapter 必死

### 1.1 通知面：一个 method 的接线点（codex 为例）

`daemon/internal/agent/codex_events.go:199` 的 `switch method` 有 **15 个 case 组、约 19 个 method 名**（`L200` warning ～ `L416` terminalInteraction，`L437` default 转 `codexExtendedNotificationEvent`）；
`codex_events_ext.go:62` 的第二梯队 switch 有 **13 个 case 组、约 18 个 method 名**（`L68` autoApprovalReview 对 ～ `L267` authRecovery 对，`L291` return false 丢弃）。
再加上 `codex.go:633-729` 的 `codexGlobalNoticeKinds` 登记表（PROJECTED / PASSTHROUGH 两档，约 45 条目，表外一律 `KindUnknownNotification`，`codex.go:732`）和 `runtime/knowledge_codex.go` 词表。

**新增 1 个通知 method 的必改清单（6 类）：**

| # | 位置 | 实测 |
|---|---|---|
| 1 | 事件 switch 分支 | `codex_events.go:199` 加 case（或 `codex_events_ext.go:62`）；字段解析 struct 手写；空 message 兜底在 `L445` |
| 2 | 台账分类登记 | `codex.go:633-729` 加条目，否则全局钩子侧记 UNKNOWN，与投影事实不符（2026-10-02 漏登记 16 个的教训见 `L644-673` 注释） |
| 3 | 人话词表 | `runtime/knowledge_codex.go` 补词条，否则诊断页无话可说 |
| 4 | proto（如需新 Kind） | `thread.proto:715-743` 现 18 个 Kind 值；加值 = 改 proto = **甲级**（`AUDIT.md:21`，L0+L1+L2 全量 + 双副本 + `gen_proto.py`） |
| 5 | App 渲染 | 按 Kind / 卡名分支加渲染（Android 侧 `ApprovalAskNames` / CodexEvent 卡同款模式） |
| 6 | 回归用例 | 缺陷即测试（`AUDIT.md:91`）；存量 `codex_events_test.go` 等逐 case 钉死 |

`item/*` 工具投影还有第三个 switch：`codex_items.go:59` 按 `head.Type`（fileChange / mcpToolCall / webSearch）分叉，新类型自动进 `maybeRecordOpaqueItem` 透传 + `UNKNOWN_NOTIFICATION` 台账（`codex_items.go:13-18` 注释）——这是今天唯一“不改代码也能活”的口子，但只给透传不给成形卡。

### 1.2 反向请求面：一个 method 的接线点（以 `currentTime/read` 0.160.0 新增为例）

`classifyCodexRequest`（`codex_requests.go:62`）现有 **8 个 case 组、11 个 method 名**（`L64` 命令审批对 ～ `L83` 不支持三元组，`L89` 兜底 `reqUnknown`）。
`currentTime/read`（`L76`）上线时除分类外，还在 `codex.go:1596-1606` 加了专案应答分支（`{"currentTimeAt": unix}`，注释写明 fail-closed 不许落 `-32601` 兜底）。

**新增 1 个审批类 method 的必改清单（9 类）：**

| # | 位置 | 实测 |
|---|---|---|
| 1 | 分类 | `codex_requests.go:62` 加 case（精确匹配，白名单，`L13`） |
| 2 | 参数 struct | `L318-380` 同款：`commandApprovalParams` / `fileChangeApprovalParams` / `patchApprovalParams` / `permissionsApprovalParams` / `askUserParams` 手写其一 |
| 3 | 建卡分支 | `buildApprovalCall`（`L388`），含 `L404` writeStdin 分叉先例 |
| 4 | 应答分支 | `handleCodexApproval` 内 `switch kind`（`codex.go:1064`，命令三审批 `L1065-1072` / 权限 `L1074` / 反问 `L1088` / MCP `L1095`），应答形状各不相同（decision / permissions / answers / action） |
| 5 | 枚举判定 | `isLegacyApproval`（`L107`）现为形态判定（`!HasPrefix(item/)`，`L92-109` 注释），新命名空间 method 自动落对分支——**但这是用“命名约定”代替“声明”，上游一改前缀即错**（`AUDIT.md:131` 挂牌唯一存量二分） |
| 6 | 选项卡 | `codexApprovalOptions`（`L221`）+ `codexApprovalDecision`（`L149`）各调一次 `isLegacyApproval`（`L152/157/223`） |
| 7 | 卡名与人话 | `toolName*` 常量（`L45-58`）+ `ApprovalReason`（`L532`，`unknown_request` 通用卡见 `L548-551`） |
| 8 | 调用方接线 | `codex.go:1590` 分类 → `L1613` B 透传 → `L1635` `-32601` → `L1645` `handleCodexApproval`；等手机期间还有 `registerPendingServerRequest` 竞速（`L1009`）、`codexApprovalWaitBudget` 超时（`L1035`）两套治理 |
| 9 | App + 用例 | 卡名分派渲染 + `parseApprovalChoice` 信封（`L184`）+ 约 27 处测试引用（实测 `*_test.go` 命中 27） |

非审批类更糟：`unsupportedRequestMessage`（`L558`）按 method 名 `switch` 写文案（`L560-566`），加一个“明确不支持”的 method 也要改代码。

### 1.3 横向：五家各写一遍

未知反向请求的 B 开关今天是“一份骨架 + 四份手写接线”：骨架 `askUnknownApproval`（`unknown_passthrough.go:162`）+ dsh（`L220`，`Respond` 空对象）/ zcode（`L233`，`rawID` 回显）/ codex（`L262`，`askWithResolvedRace` + budget=0 复用 server 侧 2min）/ pi（`L356`，无 `-32601` 通道，拒绝走 `cancelled:true`，`L380`）。pi 还有前置闸：已知表 `piUIRequestKnown`（`L300`，dialog 4 + fire-and-forget 5 种）+ 帧判别 `L323` + 分派 `dispatchPiExtensionUIRequest`（`pi.go:888`，notify 先行 `L889` → 已知静默 `L894` → 在飞闸 `L901`，上限 `piUnknownUIInFlightLimit=16`，`pi.go:828`）+ 忙拒绝 `piRejectUnknownUIBusy`（`pi.go:851`）。
**加第六家 = 把上面四家的“应答形状闭包 + 台账口径 + 超时预算 + 已知表”再手写一遍**，外加 `provider.go:154` `DetectProviders` 注册、`runtime_descriptors.go:53` `describeRuntime` + shape（`L88-202` 五家形态）、`DeleteNativeSession` 四实现（`L259-308`）、`PrefsAware` 断言（`provider.go:107-112`）、host 能力位、server `providerForThread`。这就是“加一家 = 11 步”的实证来源；“30-50 文件”是把每家的 `*_test.go`（codex 侧 30+ 个测试文件）也计入——数字不夸张，**且每步都在别的包里，编译器帮不上忙**（拼错一个 method 字符串，测试不覆盖就漏到线上）。

### 1.4 契约侧：93 RPC 是现状，不是目标

实测（`proto/v1/*.proto` 以 `^rpc` 计）：fs 9 / git 10 / host 6 / model 9 / runtime 14 / terminal 3 / thread 42，**合计 93**，与 `DEVELOPING.md:33` 一致（`AUDIT.md:61` 的 84 是 2026-09-26 旧数，锚点漂移，合入时顺手订正，不属本设计改动）。
`TurnServerMessage` oneof 现 13 分支 + `sequence=14`（`thread.proto:634-664`，`resync_required=15`）；`CodexEvent` 18 Kind（`L715-743`）；`ApprovalRequest` C1 增量 options/details（`L831-833`）；`ThinkingDelta.snapshot=2`（`L793-796`，`FIELD_ALLOCATIONS.md:186-193` 登记）。**每次上游加一种卡就加一个 Kind / 加一对 RPC，就是甲级全量审核**——瘦契约要掐断的正是这条线。

---

## 2. 声明 schema + 网关运行时流程

### 2.1 一句话

今天散在 9 类位置的“method 字符串 → 行为”全部收进**一份 per-agent 声明表**（随 daemon 发版的静态 JSON，热更新不在本设计内）；网关是唯一的解释器：传输层只给 `(method, params, id)`，网关查表、建卡、应答、记账。**加 method = 加表行，不改 Go**（App 侧见 §2.4）。

### 2.2 声明格式（v1，只够表达今天已有的全部行为，不多一格）

```jsonc
// daemon/internal/agent/spec/<agent>.json（静态随包，名字即 agent_id）
{
  "agent": "codex",
  "spec_version": 1,
  "methods": [
    {
      "method": "item/commandExecution/requestApproval",  // 精确匹配；另支持 "prefix": "item/" 仅用于枚举判定替代 isLegacyApproval
      "dir": "request",            // notify | request | response
      "card": {                    // 建什么卡（复用既有 proto，不新增消息）
        "target": "approval",      // approval | tool_call | codex_event | usage | none(只记账)
        "name": "shell",           // ToolCallBlock.name 原词（shell/file_change/apply_patch/permissions/request_user_input/mcp_elicitation/write_stdin/mcp_tool/web_search/unknown_request）
        "kind": "STATUS",          // codex_event 时填 CodexEvent.Kind 原词；tool_call 时填 STARTED/FINISHED/UPDATE
        "fields": {                // JSONPath(参数) -> 卡 JSON 键 / proto 字段；取不到 = 缺席，不伪造
          "$.command": "command", "$.cwd": "cwd", "$.reason": "reason",
          "$.kind": "kind"
        },
        "template": {              // 人话组装（今天散在各分支的 Sprintf，收进文案模板；变量只来自 fields）
          "reason": "agent 请求执行命令",
          "details_from": ["proposedExecpolicyAmendment", "commandActions"]
        },
        "ui": {                    // App 渲染契约（今天隐含在卡 JSON 键名里，显式化）
          "options": "approval3",  // approval3(三按钮) | qa_form(反问表单) | confirm(确认) | none
          "options_enum": "new",   // new(accept系) | legacy(approved系) —— 替代 isLegacyApproval 的前缀猜测
          "questions_from": "questions",
          "secret_flag": "isSecret", "other_flag": "isOther"
        }
      },
      "reply": {                   // 怎么应答（今天 handleCodexApproval switch kind 的形状，数据化）
        "allow":  {"decision": "accept"},
        "deny":   {"decision": "decline"},
        "session_allow": {"decision": "acceptForSession"},
        "abort":  {"decision": "cancel"},
        "answers_from": null,      // 反问类填 "note->answers"（splitAnswerNote/answerNoteMap 语义，函数保留，行为不变）
        "passthrough": ["permissions"]  // 原样回授字段（权限类 permissions 原样授予，codex.go:1080）
      },
      "budget_ms": 120000,         // 等手机预算；0 = 复用通道默认（codex server 侧 2min，unknown_passthrough.go:293-294）
      "ledger": "KNOWN_PROJECTED", // 台账分档（KNOWN_PROJECTED / KNOWN_PASSTHROUGH；未知走 §3）
      "safety": "approval"         // approval(红线：必须经用户) | notify | capacity(容量拒绝可代答，pi 忙闸同款)
    }
  ],
  "fallback": {                    // §3 的默认行为，同样声明化（不是代码分支）
    "unknown_notify":  {"ledger": "UNKNOWN_NOTIFICATION", "project": false},
    "unknown_request": {"card": "unknown_request", "b_gate": true, "deny": {"jsonrpc": -32601}, "pi_deny": {"cancelled": true}},
    "unknown_response": {"ledger": "UNKNOWN_RESPONSE"}
  }
}
```

字段与今天代码的对应（删减依据）：`card.target/name` 覆盖 `buildApprovalCall` 全部分支（`codex_requests.go:388-525`）+ `codexItemToolCard` 三类（`codex_items.go:59`）+ `MapStatusEvent`（`pi_events.go:421`）+ `MapUINotify`（`L605`）；`reply` 覆盖 `codex.go:1064-1100` 四形状 + pi `UIResponseFromDecision`（`pi_events.go:827`）；`ui.options_enum` 消掉 `isLegacyApproval`（`codex_requests.go:107`）；`budget_ms` 收编 `codexApprovalWaitBudget` / `dshApprovalTimeout` / `zcodeUIBudget` / `piUIBudget` 四个散常量；`ledger` 收编 `codexGlobalNoticeKinds`（`codex.go:633-729`）+ `codexLateResponseLedger`（`L759`）+ `piUIRequestKnown`（`unknown_passthrough.go:300`）。

**显式不收的**（v1 边界，免得 schema 膨胀成第二门语言）：应答 payload 的图灵完备变换（`splitAnswerNote` 归一、`answerNoteMap` 四形状、`piToolFooter` 脚注、`codexCommandDetails` 拼文案——沉淀为网关内建函数，声明只填函数名）；`serverRequest/resolved` 竞速、`steer` 目标登记、自动压缩状态机（行为层，不属方法声明）。

### 2.3 网关运行时流程（取代各家 `reqHandler` / `notifHandler` 里的 switch）

```
传输帧 (method, params, id)
  │ 1. 查表：spec.methods 精确命中？→ 条目；否则 → fallback（§3）
  ├─ 2. dir=notify：按 card.fields 抽字段 → 建卡(target 对应帧) → sink；target=none 只记账（今天 recordAs / recordLedgerOnly / maybeRecordOpaqueItem 三条路统一为这一步）
  ├─ 2. dir=request：
  │     a. approve==nil → 回 error（-32000 codex / cancelled pi），不伪造（codex.go:991-994 现行语义，保留）
  │     b. 建卡 → approve(budget_ms) → reply 模板渲染 → Respond（形状差异收进模板，网关只做 JSON 组装）
  │     c. 超时/通道失败/拒绝 → 按 fallback.deny 回写 + 台账（失败安全方向不变：最坏是“该允许的没允许”，绝不反向）
  │     d. resolved 竞速 / 在飞闸 / waiting 旗标：网关统一提供（codex L1009 / pi L901 两处收编为一处）
  └─ 3. 记账：每帧一律 `EventEnvelope{SourceAgent, SourceMethod, NormalizedKind, RawPayload: redactNativePayload, SafetyClass, ProcessingStatus}`（`runtime/unknown_events.go:97` 形状，KNOWN 两档也进台账——今天 PROJECTED 只上屏不留痕是可观测性缺口，一并补上）
```

网关是纯函数 + 两个注入点（`approve Hook`，`respond` 闭包），单测用“表行 + 帧”矩阵钉死；今天 27 处 codex request 测试收编为“声明表示例帧” golden 测试，**用例数只增不减**（`AUDIT.md:93`）。

### 2.4 App 侧：按 `name` 渲染，不按 method 分支

手机端今天已按 `ToolCallBlock.name` 分派（shell / request_user_input / mcp_elicitation / pi_ui_confirm / unknown_request 通用卡），未知 name 走通用工具卡（`codex_items.go:29-35` 注释即此纪律）。本设计把该纪律升为契约：**App 只认 `card.name + ui.*`，不认 method 名**。新 method 的卡只要 `name` 落在已有集合里，旧 App 零改动可渲染；全新交互形态（新 `ui` 类型）才需要 App 发版——发版节奏从“上游一动就两端齐发”降为“只有新卡型才动 App”。

---

## 3. 未知 method / 反向请求的默认行为（与辅线 B 的关系）

### 3.1 默认行为总表（B 关 = 今天逐字节一致，B 开 = fallback 条目生效）

| 方向 | B 关（默认） | B 开 | 台账 | 行号锚点 |
|---|---|---|---|---|
| 未知 notify | 丢弃（codex）/ 不透明透传 + 留痕（item 族）/ STATUS 上屏（六家共用信封） | 同左（B 不管通知面） | `UNKNOWN_NOTIFICATION`（表外默认，`codex.go:732`）；item 新类型 `codex_items.go:13-18` | `codex_events.go:437-443`, `codex.go:1559` |
| 未知 request（codex/zcode/dsh/hermes） | `-32601` + 台账，不伪造 decision | 通用确认卡（`unknown_request`，脱敏 method+payload 原文）→ 允许回 `{}` + `KNOWN_PASSTHROUGH`；拒绝/超时/通道缺失落回左列同一段代码 | `UNKNOWN_REQUEST` / `KNOWN_PASSTHROUGH`（`unknown_passthrough.go:199-207`） | `codex.go:1607-1636`, `zcode.go:1946`, `dsh.go:1672`, `hermes.go:872` |
| 未知 request（pi） | 静默（无 `-32601` 通道） | 同上确认卡 → 允许回 `{}`；拒绝/超时/超限回 `cancelled:true` + 台账（`L377-388`） | `UNKNOWN_REQUEST`（含忙拒绝，`pi.go:860`） | `unknown_passthrough.go:356-390`, `pi.go:851-908` |
| 未知 response | 调用方既有 pending/error 行为不变，只记账 | 同左 | `UNKNOWN_RESPONSE`（`unknown_response.go:13`），已知迟到家族走 `codexLateResponseLedger`（`codex.go:759`） | — |

### 3.2 与辅线 B 的关系（一句话 + 三条纪律）

**B 开关就是 fallback.unknown_request 的 `b_gate` 总闸**（策略源 `runtime-policy.json` 的 `unknown_request_passthrough`，`runtime.proto:130-138`，查询点 `unknownPassthroughQuery`，`unknown_passthrough.go:53`，现读零缓存 `L68-80`）：

1. **B 不新增语义**：确认卡形状（`buildUnknownRequestCall`，`L90`）、三分支日志、允许回 `{}`（尽力应答非业务结果，`L1-12` 文件头注释）、拒绝安全方向（`L150-157`），全部原样保留，只是触发条件从“四家手写 `if unknownPassthroughQuery()`”变成“网关读 fallback 条目”。
2. **B 永远可以整体关**：关 = 网关跳过确认分支，行为与今天 OFF 态逐字节一致（含 pi 的静默、codex 只登记 `reqUnknown` 不登记 `reqUnsupported` 的区分，`codex.go:1625`）。
3. **B 的危险动作白名单不变**：`ExecuteUnknownEventAction` 白名单 inspect/acknowledge/retry_later/open_native、绝无 approve 类（`runtime.proto:68-78`）；通用确认卡的“允许”语义仍是“用户看过脱敏原文后授权这次试探”（`L110-112`），不是放行执行——诊断页 `UserActionDescriptor.safe` 红线（`runtime.proto:184-195`）不受影响。

---

## 4. 瘦契约：冻结 93 RPC，走 GenericEvent + CapabilityDiscovery

### 4.1 冻结声明

- **93 RPC 冻结**：7 服务（fs 9 / git 10 / host 6 / model 9 / runtime 14 / terminal 3 / thread 42，§1.4 实测）不再为“新 method / 新卡型”加 RPC。`CodexEvent.Kind` 18 值冻结（`thread.proto:715-743`），不再为新通知加 Kind。
- 允许的例外（仍走甲级 + `FIELD_ALLOCATIONS.md:3-10` 登记）：真正的新**服务域**（如全新控制面域）、安全语义变更。**新 agent 接入、新 method、新卡型一律不算例外**。

### 4.2 GenericEvent（唯一的事件扩展点，stream 内分支，不新增 RPC）

```proto
// thread.proto（只增）：TurnServerMessage oneof 加 generic_event = 16
// （oneof 现用 1-13,15；14=sequence、15=resync_required 已占；16 为下一个空号，登记见 §4.4）
message GenericEvent {
  string source_agent = 1;   // "codex"，原样，不翻译
  string source_method = 2;  // 原生方法名原文
  string card_name = 3;      // 声明表 card.name（App 按它渲染，§2.4）
  string title = 4;          // 人话标题（daemon 模板产出）
  string message = 5;        // 人话正文
  string payload_json = 6;   // 脱敏后结构化原文（redactNativePayload 同源，8KiB 上限）
  string reply_hint = 7;     // request 类：应答模板 id（App 侧确认框用；空 = 纯通知）
  string spec_version = 8;   // 声明表版本（诊断对账用）
}
```

路由规则：凡 `card.name` 落在既有集合（shell/file_change/…/反问两类）的方法，**继续走既有强类型分支**（ApprovalRequest / ToolCallUpdate / CodexEvent），零兼容问题；只有**全新卡型**才走 `GenericEvent`，App 通用卡渲染（标题 + 正文 + 原文折叠 + 允许/拒绝双按钮）。`CodexEvent.raw_json`（`thread.proto:762`）与 `plan_steps`（`L765`）是本模式的先例——先有透传字段，后有强类型收编。

### 4.3 CapabilityDiscovery（唯一的能力扩展点，复用既有 RPC，不新增 RPC）

- daemon 侧：`GetRuntimeState` 的 `RuntimeContractReport.capabilities`（`runtime.proto:381-395`）+ `GetHostInfo` 能力位（`<位>:<agentId>` 格式）追加 `adapter:<agent>:<spec_version>` 与 `generic_card:<card_name>` 两类位。判定源 = 该 agent 声明表实际装载的 method 集合（**装载事实**，不是意愿开关——与 `CollabReady` 接线事实纪律同款，`provider.go:56-67`）。
- App 侧：只按能力位决定入口显隐（未知位忽略，不显“未知/错误”，`AUDIT.md:73-76` 三问）；`generic_card:*` 缺席时收到 `GenericEvent` 降级为“原文折叠卡”，不断连、不红屏。
- 诊断侧：`ListUnknownEvents` / `GetUnknownEvent`（`runtime.proto:28-33`）保持为“声明表缺 method”的唯一可见出口——上游发版先在诊断页出现，收编只是给它加表行。

### 4.4 FIELD_ALLOCATIONS 扩展点登记变更（草案，steward 分配前不动 .proto）

在 `proto/FIELD_ALLOCATIONS.md` 新增一节“声明式适配器扩展点（2026-10-13，本设计）”：

| 消息 | 字段 | 号 | 类型 | 说明 |
|---|---|---|---|---|
| TurnServerMessage（oneof 新分支） | generic_event | 16 | GenericEvent（新增消息，字段 1-8 见 §4.2） | 全新卡型唯一出口；既有 1-13、15 一字不动 |
| GenericEvent（新增消息） | source_agent…spec_version | 1-8 | 见 §4.2 | 字段号从 1 起 |
| CodexEvent | — | 19、20+ | 冻结 | 19=plan_steps 已占；20 起预留给网关内建函数输出，**不再为新通知分配 Kind**（Kind 枚举冻结，见 §4.1） |
| RuntimeContractReport | capabilities | 5（复用） | repeated string（既有） | 新增 `adapter:*` / `generic_card:*` 位词汇，不占新号 |

存量只增不改纪律（`FIELD_ALLOCATIONS.md:7-10`）全部保留；本节是**第一次“为不再分配而登记”**——把冻结本身写进登记表，免得后人误开新号。

### 4.5 迁移路径（存量强类型分支 → 声明表驱动，行为不变）

1. **影子运行**：网关与既有 switch 并行跑同一帧，比对“建卡输出 JSON 相等 + 应答 payload 相等 + 台账分档相等”，差异进日志不影响行为。
2. **逐家翻转**：codex（面最宽）→ pi → zcode → dsh → hermes → openai_compat；每家翻转 = 该家 switch 缩成“查表失败才进旧分支”的兜底，再到删旧分支。每步独立发版（§5）。
3. **删码条件**：某家影子零差异连续覆盖一个上游版本周期 + 该家未知 method 诊断页可查 + B 开关两态各一遍回归（`AUDIT.md:85` 常驻用例）。

---

## 5. 分阶段落地（每阶段可独立发版）与验收标准

| 阶段 | 内容 | 不碰什么 | 验收标准（全部满足才发版） |
|---|---|---|---|
| P0 网关骨架 + 影子 | 新增 `spec/` 表（先只登记 codex 全部 method，行为复述现状）+ 网关只读影子比对；`isLegacyApproval` 改为读表（`options_enum`），行为矩阵两态（item/ 前缀内外各一例） | 既有 switch、proto、App | daemon 全量 `go test` 绿 + 12 包 race；影子零差异（codex 全量单测帧）；`pa_audit.py` 红线 0；性能：单帧查表 < 1ms（map 精确匹配，无正则） |
| P1 通知面收编 | codex 通知 switch（`codex_events.go:199` + ext `:62`）逐组翻转为查表；`codexGlobalNoticeKinds` 由声明 `ledger` 字段生成（表仍保留作 golden 对照）；KNOWN_PROJECTED 也进台账（§2.3 步骤 3） | 反向请求路径、proto、App | 帧级 golden：抽样前/后 JSON 全等；诊断页 UNKNOWN 条目数不增反降（收编项从 UNKNOWN 移入 KNOWN）；L1 全量 + 受影响 L2 冒烟（`AUDIT.md:49` 节奏） |
| P2 请求面收编 | `classify` + `buildApprovalCall` + `handleCodexApproval` switch kind 翻转为查表 + 模板应答；`unsupportedRequestMessage` 文案进表；B 确认分支改为 fallback 条目；pi 在飞闸/忙拒绝收进网关统一实现 | proto、App（卡 JSON 键位不变，旧 App 零感知） | 审批三按钮两版枚举矩阵（新/旧 × 允许/会话允许/拒绝/中止）全过；`approve==nil`、超时、resolved 竞速、忙超限四条失败安全路径各一用例；B ON/OFF 两态未知请求用例（`AUDIT.md:85`） |
| P3 瘦契约落地 | `GenericEvent`（oneof 16）+ `adapter:*` / `generic_card:*` 能力位 + App 通用卡；FIELD_ALLOCATIONS §4.4 节入库；`gen_proto.py --check` 双副本一致 | 既有 13 分支语义；93 RPC 数量不变（只加 oneof 分支，不加 RPC） | 甲级审核（L0 产物自证 + L1 全量 + L2 受影响全量，`AUDIT.md:21`）；旧 App 收 `GenericEvent` 不炸（未知 oneof 分支忽略验证）；新 App 通用卡三态（通知/确认/原文折叠）真机各一遍 |
| P4 删旧码 + 冻结 | 删各家 switch / `isLegacyApproval` 函数（附录 A 挂牌销案）/ `codexGlobalNoticeKinds` 静态表（保留 golden）；`pa_audit` linter 加规则“`switch method` / 按 method 名 switch 新增即报警”；`AUDIT.md:61` 的 84 RPC 旧数订正为 93 | 行为 | 全量测试数只增不减；新 method 端到端演习：加一行表 + golden 帧，无 Go 改动走完 P0-P3 验收子集；文档锚点（DEVELOPING §0.3、§7 待整改行 `isLegacyApproval`）同步更新 |

### 回滚条件（任一触发即回滚到上一阶段 tag，不修不补）

1. **审批误路由**：任一审批应答枚举与翻转前 golden 不一致（allow/deny 反向或新旧版串扰）——红线 1（伪造 decision）等价，直接回滚。
2. **B 语义漂移**：OFF 态与历史逐字节不一致（含 pi 静默、`reqUnsupported` 不登记的区分）；ON 态允许路应答非 `{}` / 拒绝路未落 `-32601`（pi 非 `cancelled:true`）。
3. **影子差异率**：P0/P1 影子比对差异 > 0 且 24h 内定位不到是“旧分支 bug 还是网关 bug”（分不清 = 旧分支先赢，回滚网关）。
4. **契约破坏**：双副本 MD5 不一致、旧 App 对新包崩溃 / 丢帧、93 RPC 计数非预期变化未在提交信息说明（`AUDIT.md:61`）。
5. **性能/可观测退化**：P99 帧延迟回归 > 10%、台账丢条（sequence 断号）、日志量暴涨（模板渲染循环）。
6. **上游 breaking 撞网关 bug**：同一窗口既有上游新 method 又有网关缺陷——先回滚网关，再用旧路径的 UNKNOWN 台账接住新 method（旧路径的保底能力是回滚的前提，P4 删码前必须验证过回滚演习）。

---

## 6. 非目标与红线重申

- 不做声明表热更新 / 控制面下发表（策略仍只有 `SetRuntimePolicy` / `SetCollabPolicy` 既有通道，`runtime.proto:24-40`）。
- 不碰用户桌面配置（`~/.codex` 等只读，`DEVELOPING.md:58-64`）；不替用户做审批决定（没能力的回 error，`DEVELOPING.md:59`）。
- 不新增写死名单/顺序/版本二分（`AUDIT.md:73` 三问）；`pa_audit` 附录 A 挂牌只减不增，P4 销 `isLegacyApproval` 一项。
- UI 禁显“未知/错误”（`AUDIT.md:76`）；无能力不显示假入口（`DEVELOPING.md:425`）。

## 附：行号索引（实测锚点，复核用）

- `daemon/internal/agent/codex_events.go:199`（switch）、`L200-416`（各 case）、`L437-443`（default）、`L451`（用量）
- `daemon/internal/agent/codex_events_ext.go:62`（第二梯队 switch）、`L291`（return false）
- `daemon/internal/agent/codex_requests.go:62`（classify）、`L107`（isLegacyApproval）、`L112/149/221/388/532/558`（决策/选项/建卡/人话/不支持文案）、`L184`（选项信封）
- `daemon/internal/agent/codex.go:633-729`（台账登记表）、`L759`（迟到应答分类）、`L988`（审批处理）、`L1064`（应答 switch）、`L1526/1553/1559`（用量/事件/留痕）、`L1588-1645`（reqHandler）
- `daemon/internal/agent/unknown_passthrough.go:48/53/90/162/220/233/262/300/356`（卡名/查询/建卡/骨架/四家接线与已知表）
- `daemon/internal/agent/unknown_response.go:13`、`daemon/internal/agent/pi.go:828/851/888`、`daemon/internal/agent/pi_events.go:421/605/794/827`
- `daemon/internal/agent/provider.go:56/69/78/90/154`、`daemon/internal/agent/runtime_descriptors.go:53`、`daemon/internal/agent/codex_items.go:59`、`daemon/internal/agent/native_notifications.go:1-7`
- `proto/v1/thread.proto:16-179`（42 RPC）、`:634-664`（TurnServerMessage）、`:715-743`（Kind）、`proto/v1/runtime.proto:19/124/171/285/381`、`proto/FIELD_ALLOCATIONS.md:3-10/186-202`
- `DEVELOPING.md:232-264`（§7）、`AUDIT.md:21/73-85/124-132`
