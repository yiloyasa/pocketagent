# 六家横切旁路审计:事件消费路径的静默丢弃(2026-10-07)

- 范围:`daemon/internal/agent/` 全部 provider(codex / pi / zcode / dsh / hermes / echo,另有 openai_compat 附记)的事件消费路径;未知事件管道契约 = `runtime.UnknownEventLog`(unknown-events.json,200 条滚动窗)→ `runtime.PresentUnknownEvent` → `server/unknown_events.go` summary → App 处理中心(`server/unknown_event_e2e_test.go` 钉住的链路)。
- 管道事实:server 侧(`unknown_events.go`)按台账**逐条来源无关**地读与呈现,Presentation 按 normalized_kind 套固定文案(unknown_action.go);因此"接线"的充要条件就是 **provider 把事件记进台账**。台账档位:UNKNOWN_NOTIFICATION / UNKNOWN_REQUEST / UNKNOWN_RESPONSE / KNOWN_PROJECTED / KNOWN_PASSTHROUGH。
- 红线遵守:本审计的代码改动只落 `hermesapp/**`、`agent/hermes*.go`(+既有 hermes 测试文件随签名更新);codex/pi/zcode/dsh 的旁路**只报告不动手**,见 §三移交。

## 一、六家接线结论表

| provider | 全局/常驻钩子 → 台账 | 轮内事件泵 default → 台账 | 未知反向请求 → 台账 | 未知响应 → 台账 | 结论 |
|---|---|---|---|---|---|
| codex | ✅ attachGlobalHandler → recordProviderEvent(codexGlobalNoticeKinds 全登记表,schema 对表测试钉死) | ✅ default → codexNotificationEvent else notices.recordAs(codexMethodNoticeKind 同表) | ✅ reqUnknown → -32601 + UNKNOWN_REQUEST(B 开关 ON 走透传确认 + KNOWN_PASSTHROUGH) | ✅ UnknownResponseHandler → codexLateResponseLedger 分档 | **接线完整** |
| pi | ✅ clientFor GlobalHandler → recordProviderEvent(piEventNoticeKind/piNoticeKind) | ✅ consume default → MapStatusEvent else notices.recordAs(piEventNoticeKind);piLedgerOnlyEvent 家族 recordLedgerOnly | ✅ extension_ui_request 未知 method → consumePiUnknownUIRequest(允许回 {} + KNOWN_PASSTHROUGH;拒绝/超时回 cancelled:true + UNKNOWN_REQUEST) | ✅ recordUnknownResponse | **接线完整** |
| zcode | ✅ attachGlobalHandler → recordProviderEvent(zcodeLedgerMethod 拆壳 + zcodeNoticeKind) | ✅ 泵 default → notices.recordAs(zcodeNoticeKind);唯二例外见 §二 Z-1/Z-2 | ✅ reqHandler 兜底 UNKNOWN_REQUEST(已知拒答也如实入账,宁噪不漏) | ✅ recordUnknownResponse | **接线完整**(泵内三个已映射 case 的失配尾巴除外,§二) |
| dsh | ✅ newClientLocked GlobalHandler → recordProviderEvent(dshGlobalNoticeKind 先拆壳再分类) | ✅ handleSessionUpdate default → notices.recordAs(dshNoticeKind);未知 tool status/坏形状各有 recordAs | ✅ handleDshRequest 未知分支 → -32601 + UNKNOWN_REQUEST(B 开关同款) | ✅ recordUnknownResponse | **接线完整**(泵层坏帧/异会话过滤见 §二 D-1,全家常态) |
| hermes | ❌ **改前完全没有台账接线** | ❌ projectSessionUpdate 四类具名变体 + 未知变体双静默 | ✅ handleRequestPermission 未知方法回 -32601(但无台账登记) | ❌ readLoop 迟到应答:abandoned 命中留日志,未命中静默 | **本轮收口**(见 §二 H-1..H-4) |
| echo | —(无外部事件源) | —(合成事件,源码即全集) | —(不产生) | —(不产生) | **无旁路;回传面与 pb 映射一一对应,完备**(field-coverage-hermes.md §五.3) |

附记:openai_compat 为直连 HTTP 生成器(openai_compat.go),无 JSON-RPC 事件流,无未知事件面;其 watchdog/审批路径已有专测(openai_compat_watchdog_test.go / _approval_test.go)。本轮未发现旁路。

## 二、旁路清单与处置

### 已修(权限内:hermes)

| 编号 | 位置(改前行为) | 性质 | 处置 |
|---|---|---|---|
| H-1 | `agent/hermes.go` projectSessionUpdate:user_message_chunk / current_mode_update / available_commands_update / session_info_update 四类**已具名**变体解析后被弃,无投影无台账 | 静默丢弃(用户可见:会话命名/轮转出处完全不可见) | 带标题的 session_info → STATUS 卡;四类全部落台账 KNOWN_PASSTHROUGH(user_message_chunk 不重投影,手机已展示用户输入,dsh 同口径) |
| H-2 | 同函数 default 兜底缺失:认不出的 session/update 变体(解析层留 Raw)整条消失 | 静默丢弃(真未知事件不可见) | 新增 default:hermesNoticeKind 分档 → 未知变体 UNKNOWN_NOTIFICATION 台账 + recordAs 限频上屏;已知变体空转按 KNOWN_* 留痕不冒充未知 |
| H-3 | `hermesapp/client.go` readLoop default:非 session/update 通知"daemon 侧无消费方,静默丢" | 静默丢弃(transport 层) | 新增包级 `SetUnknownNotificationHook`(agent/hermes.go 的 NewHermesProvider 注入 recordProviderEvent("hermes",…) UNKNOWN_NOTIFICATION);未装钩子回落日志留痕 |
| H-4 | `hermesapp/client.go` readLoop 应答分支:id 解析失败的应答帧 `continue` | 静默丢弃(低频,上游异常帧) | 日志留痕(应答帧无错误帧可回,这是它唯一能去的地方) |
| H-5 | `hermesapp/acp.go` Prompt 只回 stopReason,终局 `usage` 五字段整块丢 | 静默丢弃(用量维度) | PromptOutcome{StopReason,Usage} → Run 返回 `*pb.Usage`(thread.go usagePtr 通道落轮次/线程累计) |
| H-6 | `hermesapp/acp.go` tool_call 的 `locations` 字段不解析 | 字段级静默丢弃(工具卡文件位置) | 解析 + 工具卡 JsonArgs `locations` |

### 只报告不动手(别家,移交主模型裁决)

| 编号 | 位置 | 现状 | 建议 |
|---|---|---|---|
| Z-1 | `agent/zcode.go` 事件泵 `case "model.streaming"`:`mapZcodeStreamDelta` !ok → `continue` | text/reasoning 之外的 payload(上游加新 delta 种类)静默丢,无台账 | 失配尾巴落 `notices.recordAs(zcodeNoticeKind("model.streaming")+":<kind>")` 一类留痕 |
| Z-2 | `agent/zcode.go` `case "tool.updated"`:三个 mapper 全不中 → 注释"P0 不映射",无台账;scheduled/tool_input_*/batch 消失 | 同上 | 同上;或至少 recordLedgerOnly PASSTHROUGH |
| Z-3 | `agent/zcode.go` 泵入口:`json.Unmarshal(nm.params,&ev) != nil || ev.SessionID != sid → continue` | 坏帧静默(异会话过滤与四家同款,刻意;**坏帧**部分建议区分) | 解包失败单独落 UNKNOWN_NOTIFICATION(异会话过滤维持原样) |
| C-1 | `agent/codex.go` item/started、item/completed、turn/completed 等 case:参数解包失败或 `n.ThreadId != codexTID` → `return` 静默 | 异会话过滤刻意;坏形状与异线程无法区分 | 解包失败落 UNKNOWN;线程过滤维持(与 dsh/zcode 同口径) |
| D-1 | `agent/dsh.go` 泵 `json.Unmarshal(&env) != nil || env.SessionID != sid → continue` | 同 Z-3 | 同 Z-3 |
| P-1 | `agent/pi.go` consume:各 case 的 Map* 系列 !ok 分支已接 notices.recordAs(pi 分类的兜底最完整),未发现新增旁路 | — | 无需动作(记录核对结论) |
| 全家 | `session/cancel` 后泵卸钩前的尾帧、共享适配器异会话帧:四家 + hermes 一致刻意过滤 | 设计取舍 | 维持;写入台账会被并发会话流量刷爆 200 条滚动窗,得不偿失 |

## 三、本轮改动清单(全部在权限内)

- `daemon/internal/hermesapp/acp.go`:StreamEvent.Variant、ToolCallEvent.Locations/ToolLocation、updateBody.locations、PromptOutcome/TurnUsage、Prompt 签名((string,error)→(*PromptOutcome,error))。
- `daemon/internal/hermesapp/client.go`:未知通知钩子(SetUnknownNotificationHook/UnknownNotificationHook,atomic)、readLoop default 重定向、应答帧 id 解析失败日志。
- `daemon/internal/agent/hermes.go`:hermesClient.Prompt 签名、NewHermesProvider 注入未知通知钩子(sync.Once)、Run 终局 usage→*pb.Usage、projectSessionUpdate 六类新 case + hermesNoticeKind/hermesLedgerMethod。
- 测试钉子(新文件):`agent/hermes_field_coverage_20261007_test.go`(8 钉:usage 经 Run 返回/缺席不伪造/标题卡/用户回显台账留痕/未知变体台账+限频卡/已知空转分档/locations 进卡/钩子接线+分档器对表);`hermesapp/prompt_outcome_test.go`(5 钉:usage 应答形状/无 usage/locations 解析/Variant/transport 钩子重定向,管道直控不起子进程)。
- 既有文件随签名最小更新:`agent/hermes_provider_test.go`、`agent/hermes_cancel_test.go`、`agent/hermes_pump_wiring_test.go`、`hermesapp/client_test.go`;探针 `cmd/hermsessprobe/main.go`、`cmd/hermapprovalprobe/main.go`(hermes 探针,仅适配新签名)。

## 四、风险与遗留

- 未运行任何程序(含 go build/test、python、hermes 本体)——编译期正确性靠静态核对,交主模型验证。
- `Prompt` 签名变更波及 cmd 两个探针:已同步;若外部还有未入库调用方(未发现),编译期即暴露。
- session_info_update 卡片措辞「hermes · 会话命名」在压缩头轮转时也会出现(轮转帧常带标题)——语义仍是"会话标识事实",如实不算误报;若嫌重复可后续只在 provenance 字段缺席时发卡。
- hermes 的 `session/request_permission` 未知方法路径(-32601)历史上无台账登记;四家同位路径(codex/zcode/dsh)有登记、hermes 没有。本轮**未动**(handleRequestPermission 在 agent/hermes.go 权限内,但审批链语义改动超出"旁路重定向"最小面)——若按"宁噪不漏"口径应补一条 UNKNOWN_REQUEST 登记,移交主模型裁决。
