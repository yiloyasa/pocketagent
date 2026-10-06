# pi 1.0.2 字段级全适配审计 — 覆盖矩阵

- 日期：2026-10-07
- 事实源：本机实装 pi **1.0.2**（npm 全局 prefix 下的 `@earendil-works/pi-coding-agent`，经 `piapp/locate.go` → hostenv 派生链定位；**只读核对，未运行任何程序**）
- 协议源缩写（均相对 `<pi 包根>`）：
  - **types.d.ts** = `dist/modes/rpc/rpc-types.d.ts`
  - **rpc-mode.js** = `dist/modes/rpc/rpc-mode.js`
  - **json-event.js** = `dist/modes/json-event.js`
  - **session.d.ts / session.js** = `dist/core/agent-session.d.ts` / `.js`
  - **core-types.d.ts** = `node_modules/@earendil-works/pi-agent-core/dist/types.d.ts`
  - **ai-types.d.ts** = `node_modules/@earendil-works/pi-ai/dist/types.d.ts`
  - **totals.js** = `dist/core/usage-totals.js`
  - **rpc-cmds.md / json.md** = `docs/rpc-commands.md` / `docs/json.md`
- 我方处理源缩写（相对 `daemon/internal/`）：
  - **client.go** = `piapp/client.go`；**pi.go / pi_events.go / pi_selection.go / pi_followup.go** = `agent/` 同名文件
  - **unk.go** = `agent/unknown_passthrough.go`；**natn.go** = `agent/native_notifications.go`
- 处置档位：**投影**（映射进既有 pb TurnServerMessage 变体上屏）/ **专线**（审批/未知请求确认管道）/ **台账**（KNOWN_PASSTHROUGH/PROJECTED 落诊断台账，不上屏）/ **有因丢弃**（写明理由）/ **G#**（本轮补齐项，详见末节）

## 0. stdout 帧的全集与入口

pi 1.0.2 的 `runRpcMode` 只有三类 stdout 输出（rpc-mode.js:28-30 `output()` 全部调用点核对）：

| 帧 | 入口证据 | daemon 入口 | 处置 |
|---|---|---|---|
| `response`（命令应答，success/error） | rpc-mode.js:31-39,608,635 | client.go dispatch → pending 表；无主应答 → `UnknownResponseHandler`（client.go:640-643） | 已映射；无主应答走**未知响应管道**（agent/unknown_response.go，`recordUnknownResponse`） |
| `extension_ui_request`（dialog + fire-and-forget） | rpc-mode.js:77,84-193 | client.go:650-674（dialog 专线 + emit 事件） | 见 §3 |
| 会话事件（AgentSessionEvent 经 toJsonEvent 原样透传） | rpc-mode.js:265-270 + json-event.js:16-28 | client.go:675-677 default → emit → Run 事件泵 + GlobalHandler 台账 | 见 §1/§2 |

注：`toJsonEvent` 只对 `message_update` 重塑（剥 partial、usage 提到帧顶、toolcall_start 补 id/toolName，json-event.js:1-28），其余事件**原样透传**——因此事件全集 = AgentSessionEvent 并集（session.d.ts:47-107 ⊃ core-types.d.ts:450-495）。

## 1. 会话事件逐类（24 类权威全集 + 台账分档）

分档钉子：agent/pi_event_union_20260930_test.go（24 类逐一过 `piEventNoticeKind`）。

| 事件 | 载荷字段（协议源） | daemon 处置 | 证据（我方） |
|---|---|---|---|
| `agent_start` | 无（core-types.d.ts:451） | 台账 PASSTHROUGH；轮次骨架，不投影（每轮必来，投影=噪音） | natn.go piNoticeKind:249；pi.go piLedgerOnlyEvent 理由注释 |
| `agent_end` | `messages[]`, `willRetry`（session.d.ts:48-52） | **投影（用量）**：取末条 assistant usage 入 lastUsage 兜底链；`willRetry` 的可见信号由随后的 auto_retry_start 卡承担（有因不重复投影） | pi.go `usageFromAgentEnd`（cost 按对象解析，0.85.1/0.87.1 探针）；pi_usage_stream_test.go |
| `agent_settled` | 无（session.d.ts:53） | **投影（轮次终局）**：置 settled 标记 + runCancel(errSettled) | pi.go Run consume；pi_followup.go 守卫 |
| `turn_start` | 无（core-types.d.ts:455） | 台账 only（piLedgerOnlyEvent，骨架事件理由见 pi.go 注释） | pi.go:745-751 |
| `turn_end` | `message`, `toolResults`（core-types.d.ts:456-458） | 台账 PASSTHROUGH（骨架；正文已由 message_update 流承担） | pi_event_union 测试：49 |
| `message_start` | `message`（core-types.d.ts:459-461） | 台账 only（载荷含 system prompt 全量 ~70KB，投影会顶开时间线——理由注释） | pi.go piLedgerOnlyEvent |
| `message_update` | 重塑后：`usage`, `assistantMessageEvent`（json-event.js:23-27） | **主投影**：usage 终帧入账即时推 UsageUpdate；text_delta→TextDelta、thinking_delta→ThinkingDelta、toolcall_end→ToolCallUpdate(STARTED)；嵌入子事件逐字段见 §1.1 | pi.go consume message_update 案；pi_events.go MapTextDelta/MapThinkingDelta/MapToolcallEnd；pi_usage_stream_test.go |
| `message_end` | `message.stopReason/errorMessage`（0.99.1 实帧） | 台账 only + **例外投影**：stopReason=length/error → WARNING 卡（截断/模型流异常收尾） | pi.go `MapAssistantStop` 调用点；pi_events.go MapAssistantStop；pi_display_tui_test.go |
| `tool_execution_start` | `toolCallId,toolName,args`,**`parentToolCallId?`**（core-types.d.ts:462-466 + session.d.ts:38-43） | **投影** STARTED 卡（toolcall_end 缺席的兜底）；`parentToolCallId` 不解析（嵌套工具调用拍平呈现，手机无嵌套卡形态，卡片/结果不丢——有因丢弃） | pi_events.go MapToolExecStart；pi.go consume |
| `tool_execution_update` | 同上 + `partialResult`（累积值） | **投影** UPDATE 卡（原地替换），daemon 实测 elapsed 进脚注 | pi_events.go MapToolExecUpdate（0.99.1 deepseek 实帧注释）；pi_events_test.go |
| `tool_execution_end` | 同上 + `result`, `isError` | **投影** FINISHED 卡；structuredContent 的 wall_time_seconds/exit_code/truncated 进脚注 | pi_events.go MapToolExecEnd/piToolFooter；pi_events_test.go |
| `queue_update` | `steering[]`, `followUp[]`（session.d.ts:71-73） | **投影**：原生队列对账（Queued/QueueFlushed 回执链）+ QUEUE 状态卡 | pi_followup.go reconcileNativeQueue；pi_events.go queue_update 案 |
| `compaction_start` | `reason: manual\|threshold\|overflow`（session.d.ts:74-76） | **投影** CONTEXT_COMPACTED(started) + 压缩闸门（暂停中程用量推送） | pi_events.go:compaction_start 案；pi.go consume |
| `compaction_end` | `reason, result?, aborted, willRetry, errorMessage?`（session.d.ts:86-92；json.md:140-157） | **投影** 四态（aborted/failed/completed+溢出恢复）；**G5**：completed 态补读 `result.tokensBefore/estimatedTokensAfter` 上卡 | pi_events.go:408-447；pi_0991_adapt_test.go；新钉 pi_field_coverage_20261007_test.go TestCompactionEndCarriesResultTokens。`result.summary` 不上屏（给模型的摘要文本，上屏会顶开时间线——有因）；end 帧的 `reason` 不重复投影（start 卡已带——有因） |
| `entry_appended` | `entry: SessionEntry`（session.d.ts:77-78） | 台账 PASSTHROUGH（8KiB 截断留痕；内容已由流式事件承担） | pi_event_union 测试：48 |
| `session_info_changed` | `name?`（session.d.ts:79-80） | **投影（标题）**：GlobalHandler → TitleHandler → server 落库，双端同步 | pi.go clientFor SetGlobalHandler；pi_events.go MapSessionNameChange |
| `thinking_level_changed` | `level`（session.d.ts:81-82） | **投影**：InvalidateModels + emitRuntimeSelection（Started 帧） | pi.go consume 顶部；natn.go decodeRuntimeSelection pi 分支 |
| `auto_retry_start` | `attempt, maxAttempts, delayMs, errorMessage`（session.d.ts:93-98；json.md:166） | **投影** RETRY_STARTED 卡（delayMs→"约 N 秒后"） | pi_events.go auto_retry_start 案；pi_0991_adapt_test.go |
| `auto_retry_end` | `success, attempt, finalError?`（session.d.ts:99-101；json.md:167,170） | **投影** RETRY_STOPPED 卡 | pi_events.go auto_retry_end 案；pi_0991_adapt_test.go |
| `summarization_retry_scheduled` | `attempt, maxAttempts, delayMs, errorMessage`（session.d.ts:102-107；json.md:175；session.js:2950-2961） | **G2 前=台账限 2 卡后静默（补齐）**；现 **投影** RETRY_STARTED 卡，字段全透出 | pi_events.go:493-514；新钉 TestMapStatusEventSummarizationRetryFamily |
| `summarization_retry_attempt_start` | `source: branchSummary` \| `{source: compaction, reason}`（session.d.ts:102-107；json.md:176,180；session.js:2962-2967） | **G2 补齐**：**投影** RETRY_STARTED 卡（分支摘要/上下文摘要两呈现） | pi_events.go:516-532 |
| `summarization_retry_finished` | 无（session.js:2968-2971） | **G2 补齐**：**投影** RETRY_STOPPED 卡（最终失败不走此帧，走 compaction_end errorMessage） | pi_events.go:534-542 |
| `bash_execution_update` | `id?, delta`（session.d.ts:104-107；json.md:184-188） | 台账 PASSTHROUGH。**有因丢弃**：daemon 从不发 RPC `bash` 命令，真出现只可能来自桌面端同进程操作；留痕即诊断价值 | pi.go piEventNoticeKind 注释（0.99.1 增补段）；knowledge_pi.go 词条 |
| `extension_error` | `extensionPath, event, error`（rpc-mode.js:260；json.md:190-194） | **投影** ERROR 卡 + 台账 PASSTHROUGH（双路不冲突） | pi_events.go extension_error 案；pi_events_test.go TestMapStatusEvent |
| `abort` | —（0.99.1 全集已消失，piprobe 实证） | 死代码防护词条保留（防旧版回连落 UNKNOWN） | pi_event_union 测试 TestPiLegacyEventsStillClassified |
| `model_changed` / `model_select` / `state_changed` | —（**1.0.2 全集无此三类**，0.85.1 时代防御残留） | consume 有真实投影分支（InvalidateModels+Started）；**G6 补齐**：分档改 PROJECTED，与实际行为一致 | pi.go:727-731；新钉 TestPiDefensiveSelectionEventsProjected |

## 1.1 message_update.assistantMessageEvent 子事件逐字段

外层重塑保证：`usage` 每帧都在帧顶（json-event.js:25）；子事件已剥 `partial`（json-event.js:7-14）；`toolcall_start` 追加 `id/toolName`（json-event.js:8）。

| 子事件 | 载荷（ai-types.d.ts:566-618） | daemon 处置 | 证据 |
|---|---|---|---|
| `start` / `text_start` / `thinking_start` | contentIndex(+partial 已剥) | 有因不投影：块边界无用户可见增量；正文/思考由 *_delta 流承担 | pi.go consume 注释（"STARTED 等 end"段） |
| `text_delta` | `delta` | **投影** TextDelta + textBuf 拼接 | pi_events.go MapTextDelta；pi_events_test.go |
| `text_end` | `content`（全文） | 有因不投影：全文=已收 delta 之和，重复上屏；usage 已在帧顶层入账 | 同上 |
| `thinking_delta` | `delta` | **投影** ThinkingDelta（HideReasoning 快照语义过滤） | pi.go emitThinkingFor；pi_slevel_test.go |
| `thinking_end` | `content`（全文） | 同 text_end（有因不投影） | — |
| `toolcall_start` | `id`, `toolName` | 有因不投影：参数未齐不出半截卡（P1 取舍），STARTED 等 toolcall_end；tool_execution_start 兜底保证不悬空 | pi_events.go MapToolcallEnd 注释 |
| `toolcall_delta` | `delta`（参数 JSON 增量） | 有因不投影：拼装过程噪音 | 同上 |
| `toolcall_end` | `toolCall{id,name,arguments}` | **投影** STARTED 卡（参数完整） | pi_events.go MapToolcallEnd；pi_events_test.go |
| `done` | `reason: stop\|length\|toolUse\|deferred` | 有因不单独投影：终态由 message_end.stopReason 承载（length/error→MapAssistantStop 卡），toolUse/stop 是正常路径 | pi_events.go MapAssistantStop 注释 |
| `error` | `reason: aborted\|error`, `error: AssistantMessage` | 有因不单独投影：同帧 message 携带 stopReason=error+errorMessage，message_end 卡 + auto_retry_*/TurnFailed 链条已覆盖，不双重报警 | 同上 |

## 2. 应答数据（daemon 实际下发的命令 × 消费字段）

| 命令 | 应答 data 形状（types.d.ts） | daemon 消费 | 未消费字段与理由 |
|---|---|---|---|
| `prompt` | `{disposition: started\|handled\|queued}`（types.d.ts:162-170；session.d.ts:162-163） | success/error 判定（受理前拒绝→PI_PROMPT_REJECTED） | `disposition`：受理后的排队/续跑可见性由 Queued/Steered 回执链与事件流承担（pi_followup 对账），应答值冗余——有因 |
| `steer` / `follow_up` | `{disposition: handled\|queued}` | 成败判定（失败回落 daemon 队列兜底） | 同上 |
| `abort` | 无 data | 成败判定 | — |
| `clear_queue` | `{steering[], followUp[]}` | **全消费**（撤销条数计数） | piapp/queue.go ClearedQueue；queue_test.go |
| `get_state` | RpcSessionState 12 字段（types.d.ts:148-161） | 消费 `sessionFile/sessionId`（挂载缓存）+ `model.{id,provider}/thinkingLevel`（选择回读） | `isStreaming/isCompacting/steeringMode/followUpMode/sessionName/autoCompactionEnabled/messageCount/pendingMessageCount`：轮次/队列/压缩状态均以事件流为准，快照字段冗余——有因（pi_selection.go；client.go GetState） |
| `set_model` | `Model`（完整模型对象） | success 判定 + "Model not found" 降级路径 | data 体内字段：生效真值以下一步 get_state 回读为准（pi_selection.go 1.0.2 对齐段）——有因 |
| `get_available_models` | `{models[]}` | **全消费**（id/name/provider/reasoning/input/contextWindow/thinkingLevelMap→pb.ModelInfo） | 其余模型元数据字段无 UI 面——有因；pi.go probePiModels；pi_models_probe_test.go |
| `set_thinking_level` | 无 data | success 判定 | — |
| `get_available_thinking_levels` | `{levels[]}` | **全消费**（档位校验/钳制语义） | pi_selection.go:67-85 |
| `compact` | CompactionResult（`summary, firstKeptEntryId, tokensBefore, estimatedTokensAfter?, usage?, details?`；compaction.d.ts:18-31） | 消费 `tokensBefore/estimatedTokensAfter`（完成日志） | `summary` 是给模型的摘要文本不上屏；`firstKeptEntryId/usage/details` 无 UI 面——有因；piapp/export.go |
| `get_session_stats` | SessionStats（`tokens{...}, cost, contextUsage{tokens,contextWindow,percent}` + 计数字段；rpc-cmds.md:536-559；session.js:3345-3401） | 消费 tokens/cost/contextUsage{window}；percent 由 App 用 tokens/window 自算——有因 | `userMessages/assistantMessages/toolCalls/toolResults/totalMessages/sessionFile/sessionId`：用量卡不需要——有因。**G1：`cost` 1.0.2 为纯数字（totals.js:1-18），此前按对象硬解导致整帧解析失败、用量链路静默哑火；已双形状兼容** |
| `export_html` | `{path}` | **全消费** | piapp/export.go parseExportHTMLResponse |
| `switch_session` | `{cancelled}` | **全消费**（cancelled→错误；get_state 复核） | client.go SwitchSession |
| `clone` | `{cancelled}` | **全消费**（分叉） | pi.go ForkThread；pi_fork_test.go |
| `get_entries` | `{entries[], leafId}` | **全消费**（游标推进/断点回放） | piapp/entries.go EntriesPage.LastID/LeafID |
| `get_tree` | `{tree[], leafId}` | 消费 `leafId` 判空（分叉前置闸） | `tree` 体：分叉走 clone 不走树导航——有因；pi.go ForkThread |
| `get_last_assistant_text` | `{text}` | **全消费**（无 text_delta 轮的正文兜底） | pi.go Run 收尾 |
| `set_session_name` | 无 data | success 判定 | pi.go sendSessionName |
| `get_commands` | `{commands[{name,description,source,sourceInfo}]}` | 消费 name/description/source | `sourceInfo`：面板无溯源面——有因；piapp/commands.go；pi.go SlashCommandsFor |
| 未使用命令 | `new_session / cycle_model / cycle_thinking_level / set_steering_mode / set_follow_up_mode / set_auto_compaction / set_auto_retry / abort_retry / bash / abort_bash / fork / get_fork_messages / get_messages` | 不下发。`set_auto_*` 是**红线**（会写用户 ~/.pi 全局配置，pi_followup.go 文件头）；`bash` 有安全面（守护扩展不覆盖 RPC bash）；其余无对应手机功能 | rpc-mode.js:294-571 全 handler 核对 |

## 3. extension_ui_request 逐 method（types.d.ts:406-462）

| method | 字段 | daemon 处置 | 证据 |
|---|---|---|---|
| `confirm` | `title, message, timeout?` | **专线**：approve hook → pi_ui_confirm 审批卡；超时/断线/拒绝回 `{"cancelled":true}` | pi_events.go ApprovalCallFromUI/UIResponseFromDecision；pi.go Run UIRequestHandler；pi_events_test.go |
| `select` | `title, options[], timeout?` | **专线**：反问表单卡（选项全透出，真选才回 value，否则 cancelled） | 同上；TestApprovalCallFromUISelectPassesOptions |
| `input` | `title, placeholder?, timeout?` | **专线**：反问表单卡；**G4：`placeholder` 此前被 parseUIRequest 丢弃，现捕获并随卡 JSON `questions[0].placeholder` 透出** | piapp/client.go:144-158,699-733；pi_events.go:768-796；新钉 uirequest_hints_test.go + TestApprovalCallFromUIPlaceholderPrefill |
| `editor` | `title, prefill?`（无 timeout） | **专线**：反问表单卡；**G4：`prefill` 同上透出** | 同上 |
| `notify` | `message, notifyType?: info\|warning\|error` | **G3 前=Run 路径整类静默（补齐）**；现 **投影** STATUS/WARNING/ERROR 卡（不回写：fire-and-forget 无应答通道） | pi_events.go:579-603 MapUINotify；pi.go:1058-1063；新钉 TestMapUINotifyCard |
| `setStatus` | `statusKey, statusText` | 有因丢弃（不上屏）：桌面 TUI 状态栏文本，手机无状态栏面，投影必刷屏；台账留痕（GlobalHandler→KNOWN_PASSTHROUGH） | pi.go piEventNoticeKind；natn.go piNoticeKind:253 |
| `setWidget` | `widgetKey, widgetLines, widgetPlacement?` | 有因丢弃：桌面 TUI 编辑器挂件（aboveEditor/belowEditor），rpc-mode.js:123-136 明说组件工厂类在 RPC 模式已不支持，仅字符串数组可透——无手机对应面；台账留痕 | 同上 |
| `setTitle` | `title` | 有因丢弃：终端标题（会话标题同步走 session_info_changed 专线，与此无关）；台账留痕 | 同上 |
| `set_editor_text` | `text` | 有因丢弃：桌面 TUI 输入框内容注入（斜杠命令展开等），手机端无"替用户改输入框"通道且不该有；台账留痕 | 同上 |
| 未知 method（上游新增） | 任意 | **未知请求确认管道**：开关 ON 弹通用确认卡（允许→回 `{}`；拒绝/超时→回 `{"cancelled":true}`+台账 UNKNOWN_REQUEST）；OFF 态维持历史静默 | unk.go consumePiUnknownUIRequest:337-391；pi_unknown_passthrough_test.go |
| （dialog 公共）`timeout` | 扩展自带超时 | 捕获（UIRequest.TimeoutMs，含字符串容错）但审批预算统一 piUIBudget=120s（比扩展值保守，扩展侧超时自行 resolve 默认值）——有因 | client.go parseUIRequest；pi.go piUIBudget |

## 4. 本轮补齐项（G1..G6）与看门狗自查

| # | 内容 | 改动 | 测试钉 |
|---|---|---|---|
| G1 | `get_session_stats.cost` 双形状兼容（1.0.2 纯数字 / ≤0.99.x 对象）。旧解析在 1.0.2 下整帧失败 → parseSessionStats nil → 基线/轮询/收尾/窗口**全部静默哑火**（P0） | pi_events.go:309-326 `piStatsCost`（自定义 UnmarshalJSON）+ pi_events.go:328-341 piSessionStats.Cost 换型（`.Cost.Total` 访问面不变） | pi_field_coverage_20261007_test.go TestSessionStatsCostNumberShape102（rpc-cmds.md:536-559 逐字节夹具 + 对象形状防回滚） |
| G2 | summarization 重试三态零投影（原 recordAs 限 2 卡后静默，重试等待被误读卡死） | pi_events.go:493-542 MapStatusEvent 三案 → RETRY_STARTED/RETRY_STOPPED 卡 | 同文件 TestMapStatusEventSummarizationRetryFamily（json.md:175-177 逐字节夹具） |
| G3 | fire-and-forget `notify` 整类静默（用户可见通知丢失） | pi_events.go:579-603 MapUINotify + pi.go:1054-1062 consume 例外分支（info→STATUS/warning→WARNING/error→ERROR，不回写） | 同文件 TestMapUINotifyCard |
| G4 | input.`placeholder` / editor.`prefill` 双段丢弃（解析→审批卡） | piapp/client.go:144-158（UIRequest 两字段）+ :699-733（解析）；pi_events.go:617-627（uiAskQuestion 两字段）+ :643-660（buildAskUserCall 非空才进卡，老卡形状零变化）+ :768-796（ApprovalCallFromUI 签名扩参）；pi.go:876 调用点 | piapp/uirequest_hints_test.go 三例 + agent 侧 TestApprovalCallFromUIPlaceholderPrefill（含"无提示时键不出现"字节兼容钉） |
| G5 | compaction_end `result.tokensBefore/estimatedTokensAfter` 不呈现 | pi_events.go:408-447 completed 态消息带"上下文 N → 约 M tokens"；aborted/failed 形状不变 | 同文件 TestCompactionEndCarriesResultTokens（json.md:140-157 夹具） |
| G6 | `model_changed/model_select/state_changed` 有真实投影分支却分档 UNKNOWN（诊断页会被旧版回连灌噪音） | pi.go:727-731 piEventNoticeKind 收录为 PROJECTED | 同文件 TestPiDefensiveSelectionEventsProjected |

**看门狗打点自查（本任务最高风险点）**：事件打点在 `piapp.Client.SetEventHandler` 生产侧——Run 里 `c.SetEventHandler(func(ev){ wdMu.Lock; lastEvent=time.Now; wdMu.Unlock; notifCh <- ev })`（pi.go:902-910），**每个事件先打点再入泵**，与消费侧如何分流无关。本轮全部改动（G2/G3/G5 的映射、G3 的 sink 直推）都在消费 goroutine（`consume`）或其下游纯函数里，**没有任何新路径绕过事件泵**；G4 只改审批卡形状，`uiInFlight` 豁免位包裹不变（pi.go:875-877）；G1/G6 不触碰事件路径。无新增等待手机的 goroutine，无新增 `uiInFlight` 计数需求。结论：任何 pi 事件照旧刷新 `lastEvent`，长工具执行不会被新路径误判停摆。

## 5. 遗留与建议（交主模型决策，本轮未动）

1. **G1 的发现窗口**：1.0.2 下所有 stats 类用量（含收尾 UsageUpdate 的 total_usage/窗口）自升级起就已静默归零，只有 0.87.1+ 流式 per-message 累计链路还在撑"本轮增量"。建议在发布说明提示"pi 1.0.2 用量显示修复"。
2. **approval 卡 `placeholder/prefill` 的 App 侧渲染**是可选增强（旧 App 忽略），若要显示需 Android 端 ApprovalSheet 认这两个键（本轮不碰 server/与客户端）。
3. `notify` 卡未做每轮限频（与 extension_error 卡同口径）。若上游出现高频 notify 扩展，可考虑挂 recordAs 式限频——现状与 pi TUI"每条必弹"一致，先观察。
4. `SessionStats.contextUsage.percent`/计数字段、`disposition`、`get_state` 冗余快照字段等"有因不消费"项，若未来手机端要做"会话统计页"可直接从台账/新增 RPC 取，不需要新协议。
