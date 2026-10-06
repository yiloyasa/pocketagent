# dsh 字段级覆盖矩阵（2026-10-07）

执行器审计：dsh（`@deepseek-ai/dsh` 0.2.0-rc.2，`--profile acp`）能回传的**全部**事件/字段 → daemon 处置 → 证据。
**未运行任何程序**（含 go/测试/node/dsh 本体）。

## 事实源

| 项 | 值 |
|---|---|
| 本机安装 | `G:\Claude Code\node_modules\@deepseek-ai\dsh`（package.json version=0.2.0-rc.2；入口 lib/bin.js） |
| 定位链 | `dshapp.LocateDsh()`：DSH_BIN → hostenv.PackageCandidates("@deepseek-ai/dsh/lib/bin.js") → PATH；经 `G:\Claude Code\dsh.cmd` 实际解析到上述包 |
| 协议面 | 随包 `@deepseek-ai/dsh-acp/lib/index.js`（55,804 字节，桥接实现）+ `@agentclientprotocol/sdk/dist/schema/types.gen.d.ts`（ACP v1 wire union） |
| 提取方式 | 只读静态检索（桥接 emitter + zod/TS union），未执行 |
| 佐证 | docs/verify/evidence/dsh-p0-probe.md、dsh-resume-compat-probe.md、dsh-config-persist-probe.md、proto020 探针（dsh_020adapt_test.go 头注） |

## A. session/update 变体（union 全集 15 型 × 桥接实发 6 型）

SDK `SessionUpdate` union（types.gen.d.ts）：user_message_chunk / agent_message_chunk / agent_thought_chunk / tool_call / tool_call_update / plan / plan_update / plan_removed / available_commands_update / current_mode_update / config_option_update / session_info_update / usage_update / compaction_update / compaction_summary_chunk。

桥接实发面（dsh-acp `notify` 调用点穷举）：**agent_thought_chunk、agent_message_chunk、tool_call、tool_call_update、usage_update、config_option_update** 共 6 型（onSessionEvent 只桥 assistant/message、tool/call、tool/result 三类 committed 事件 + topologyChanged）。

| sessionUpdate | payload（bundle 实发形状） | 处置 | 证据 |
|---|---|---|---|
| `agent_message_chunk` | {sessionUpdate, messageId?, content:{type:"text",text}} | **已映射**：TextDelta + streamText 累积 | 桥 `assistantUpdates`；handleSessionUpdate |
| `agent_message_chunk`(image) | {messageId?, content:{type:"image", data:base64, mimeType}}（助手图片输出，bundle assistantBlockToAcp 实发） | **本轮修复**：修复前 dshUpdateText 只解 text→图片整块静默丢弃；现暂存 dshTurnState.images，随 Run 终局 blocks（ContentBlock_Image）返回 + 诊断帧即时可见 | 桥 `assistantBlockToAcp`；agent/dsh.go 2026-10-07 hunk |
| `agent_message_chunk`(其它) | union 防御面（audio/resource/resource_link 等，桥不产） | 留痕（诊断透传，不再无声消失）——本轮把"解不出文本"从静默改为 notices.recordAs | handleSessionUpdate default 分支 |
| `agent_thought_chunk` | {messageId?, content:{type:"text",text}} | **已映射**：ThinkingDelta（HideReasoning 过滤）；HideReasoning 打开时丢弃=用户显式设置，非静默 | 桥 `assistantUpdates`（reasoning 块先行）；handleSessionUpdate |
| `tool_call` | {toolCallId, title=工具名, kind:"other", status:"in_progress", rawInput(解析后参数;JSON.parse 失败时为原始串), name?/content?/locations?/rawOutput?(union 字段,桥不发)} | **已映射**：STARTED 卡（名称链 title→name→kind→tool，rawInput 进 JsonArgs）；本轮补 name 兜底 | 桥 `toolCallUpdate`；handleDshToolUpdate |
| `tool_call_update` | {toolCallId, status:"completed"\|"failed", content:[{type:"content", content:{type:"text",text}\|{type:"image",…}}]} | **已映射**：FINISHED 卡（嵌套 content 解包；ok=completed/finished）；本轮补齐：非文本条目（图片/diff/terminal 防御面）留痕进诊断不再静默；content 全空且 rawOutput 在场时原文透出 | 桥 `toolResultUpdate`；dshContentText；本轮 hunk |
| `usage_update` | {used:上下文占用, size:窗口总量, cost?{amount,currency}(union 字段,桥不发)} | **已映射+本轮修复**：used→Usage.InputTokens；**size→UsageUpdate.ModelContextWindow（修复前静默丢弃，占用环无分母）**；cost 桥不实发，union 防御面（无 pb 承载位=pb.Usage.CostUsd 其实存在，见建议反馈②） | 桥 `usageUpdate`；dshUsageUpdateFromUsedSize |
| `config_option_update` | {configOptions:SessionConfigOption[]}（与 session/new\|resume 应答同构） | **已映射**：刷新原生目录缓存（模型面+设置页），解析失败走诊断透传 | 桥 `topologyChanged`；refreshCatalogFromConfigOptionUpdate |
| `user_message_chunk` | ContentChunk | 桥**不实发**；daemon 刻意不转发（自己的输入回显，手机已展示）——丢弃有理由 | handleSessionUpdate；dshNoticeKind PASSTHROUGH |
| `plan` / `plan_update` / `plan_removed` | Plan{entries} / PlanUpdate{items\|file\|markdown} / PlanRemoved | 桥**不实发**（0.2.0-rc.2 无计划桥）；daemon 防御映射为计划卡帧，形状认不出时诊断透传 | dsh_plan.go；dshNoticeKind |
| `available_commands_update` | {availableCommands[]} | 桥**不实发**；daemon 透传诊断视图（PASSTHROUGH 留痕） | handleSessionUpdate；蓝本 §4.3 |
| `current_mode_update` | {currentModeId} | 桥**不实发**（dsh 无 session/set_mode 面）；daemon 落 default→UNKNOWN 留痕。维持未知管道（契约满足），无需映射 | union types.gen.d.ts:3855；dshNoticeKind |
| `session_info_update` | {title?, updatedAt?} | 桥**不实发**；daemon UNKNOWN 留痕。若上游未来发 title，建议接 TitleHandler（见建议反馈③） | union:3896 |
| `compaction_update` | {compactionId, status:in_progress\|completed\|failed\|cancelled, summary?, error?} | 桥**不实发**（且需客户端宣告 compaction 能力，daemon 未宣告）；daemon UNKNOWN 留痕 | union:4003 |
| `compaction_summary_chunk` | {compactionId, content} | 同上 | union:4039 |
| envelope | {sessionId, update, _meta?} | sessionId 过滤后消费；_meta 按 ACP 规范不解释 | dshUpdateEnvelope |

## B. 反向请求（agent→client，CLIENT_METHODS 全集核对）

| method | 参数 | 处置 | 证据 |
|---|---|---|---|
| `session/request_permission` | {sessionId, toolCall:{toolCallId}(0.2.0 桥只带 id,无 title/input), options:[{optionId:"allow-once",name:"Allow once",kind:"allow_once"},{optionId:"reject-once",name:"Reject",kind:"reject_once"}]} | **已映射**：审批卡+ApprovalRequest 帧；按 kind 前缀选 allow/reject 侧；超时 2min/断线走拒绝（失败安全）；outcome={outcome:"selected",optionId}。卡名回落 "dsh_permission"（桥不带 title）——用户在工具卡上能看到对应 callId 的卡 | 桥 `approval/request` handler；handleDshRequest；dsh-p0-probe 第 4 项实帧 |
| `session/update`（通知，见 A） | — | 已映射 | — |
| `fs/read_text_file` / `fs/write_text_file` | — | **不可能到达**：initialize 客户端能力声明 fs 全 false（dshapp.Initialize 参数），桥侧无从发起；如被伪造到达→-32601+UNKNOWN_REQUEST 台账 | dshapp.Initialize clientCapabilities |
| `terminal/*`(5 个) / `mcp/connect` / `mcp/message` / `mcp/disconnect` / `elicitation/create` / `elicitation/complete` | — | 桥 0.2.0-rc.2 **不发起**（notify/request 调用点穷举仅 request_permission+update）；真到达走 -32601+UNKNOWN_REQUEST 台账（未知管道契约满足） | 桥 `conn.request/conn.notify` 穷举；handleDshRequest 兜底 |

## C. initialize 应答（能力快照）

| 字段 | 值（本机 bundle 静态） | 处置 |
|---|---|---|
| protocolVersion | 1 | 已映射（日志+能力判定） |
| agentInfo | {name:"deepseek-harness-acp", version:"0.0.1"} | 已映射（日志） |
| agentCapabilities.promptCapabilities.image | 动态：supportsAcpImagePrompts（attachment+llm inputModalities），本机路由 false | 已映射：SupportsImageInput 门控（false 时收图显式报错，不静默丢） |
| agentCapabilities.promptCapabilities.audio / embeddedContext | false | 未消费=daemon 从不发 audio/embeddedContext 块，无用户可见面 |
| agentCapabilities.mcpCapabilities | {http:true} | 未消费=daemon 不经 ACP 挂 MCP（collab 走部署级补丁层），如实边界 |
| agentCapabilities.sessionCapabilities | {close:{},list:{},resume:{}} | 已映射：CanClose/CanList/CanResume 门控 remountSession |
| authMethods | [] | 已映射：空=跳过 authenticate（桥侧 authenticate 也恒空实现） |

证据：桥 `implementation.initialize`；protocol.go InitializeResult。

## D. 请求应答字段（daemon→agent 的返回面核对）

| 请求 | 应答消费字段 | 未消费（处置=理由） | 证据 |
|---|---|---|---|
| `session/new` | {sessionId, configOptions[]} | — | protocol.go parseSessionState |
| `session/resume` | {configOptions[]}（rc.3 不回显 sessionId→请求参数回填，双实证） | — | parseResumeState |
| `session/list` | {sessions:[{sessionId,cwd}], **nextCursor?**} | **本轮修复**：nextCursor 修复前被静默丢弃——默认页大小 100（DEFAULT_SESSION_LIST_PAGE_SIZE），超页后 remountSession 的"在列校验"会把真实会话误判不可恢复；现跟随 nextCursor 翻页取全（上限 50 页兜底） | 桥 `listSessions`（分页+cursor 校验 ^[A-Za-z0-9_-]+$）；dshapp SessionList |
| `session/list` 参数 | cwd 过滤参数**未用**：理由=remount 校验按 sessionId+cwd 双因子在 daemon 侧完成（dshSessionEntry），传 cwd 收窄会让"cwd 变更过的会话"从列表消失、错误分类从"可恢复"变"不在列" | 桥 `params.cwd`→sameDirectory 过滤 | dsh_session.go dshSessionEntry |
| `session/set_config_option` | {}（成功空对象）；错误 -32602 带 zod data | 已映射（configId 参数名 rc.3 契约；model 值=双层编码串红线） | configoptions_test.go；dsh-config-persist-probe |
| `session/prompt` | {stopReason} | — | 见 E |
| `session/close` | {} | — | protocol.go |
| `session/cancel` | 通知，无应答 | — | protocol.go |

configOptions[] 元素字段核对：{id,name,category,type,currentValue,options}→已消费；`description`（option 级）未消费——理由：模型卡描述取自叶子 options[].description（已消费 parseModelOptionLeaves），option 级描述无承载位。

## E. session/prompt 终局 stopReason

| stopReason | 桥产生条件（TurnEndReason 映射） | 处置 | 证据 |
|---|---|---|---|
| `end_turn` | completed / aborted / blocked / error / merge-extensible 兜底 | 已映射：正常终局（文本+图片 blocks） | 桥 switch(reason.kind)；Run end_turn 分支 |
| `cancelled` | interrupted | 已映射：CANCELLED 结构化错误（Retryable，携带 partial 文本） | 桥 `interrupted`→cancelled；Run cancelled 分支 |
| `max_tokens` | max-tokens | 已映射：STOP_MAX_TOKENS 结构化错误（Details=stopReason） | 桥 `max-tokens`；Run default 分支 |
| `max_turn_requests` / `refusal` | 桥不产（SDK union 防御面） | 已映射（default 分支同样如实报错） | StopReason union:3001 |
| （空） | 应答坏帧防御 | 已映射：按 end_turn 处理（dshStopReason 缺省） | dshStopReason |

## F. 错误通道

| 来源 | 形状 | 处置 | 证据 |
|---|---|---|---|
| JSON-RPC error（prompt 途中） | {code,message,data?}；缺 key=-32603 无 data 含 "no API key" | 已映射：*RPCError 保真上浮→ErrorInfoCarrier（MISSING_CREDENTIAL 人话+原文进 Details / DSH_ERROR 原文直显） | IsMissingCredential；dshRunErrorFrom；proto020 复证 |
| session/update 桥内失败 | 桥 logger.warn 吞传输错误（"acp: session/update failed"） | 不可能到达 daemon（桥内部消化）；结果一致——没有半帧污染流 | 桥 `notify` catch |
| assistant 输出转换失败 | 桥 `AcpContentError`（internal）→ inflight.outputError→turn error 路径 | 已映射：经 prompt 错误通道如实终局 | 桥 onSessionEvent catch |

## G. 静默丢弃总账（本轮处置后仍成立的，全部有理由）

1. agent_thought_chunk 在 HideReasoning=true 时丢弃：用户显式设置。
2. user_message_chunk 不转发：自己的输入回显（且桥不实发）。
3. initialize 的 audio/embeddedContext/mcpCapabilities：无对应使用面。
4. configOptions[].description(option 级)：叶子级已消费，option 级无承载位。
5. usage_update.cost：桥不实发；union 防御面（如需透出 pb.Usage.CostUsd 见建议反馈②）。
6. current_mode_update / session_info_update / compaction_update / compaction_summary_chunk：桥不实发；落 UNKNOWN 留痕（未知管道契约满足），非丢弃。

## 建议反馈（交主模型）

① `tool_call_update` content 中的图片条目（union 允许、桥 assistantBlockToAcp 同源可产）目前只留痕不上卡：工具卡 Output 是纯文本承载位，如需图片须动 pb（禁改），维持现状。
② usage_update.cost→pb.Usage.CostUsd：pb 已有字段，若上游真发该帧可一行接上；桥现不实发，本轮未接（避免为不存在的帧写映射）。
③ session_info_update.title 若上游启用，建议接 provider TitleHandler（zcode session.titleUpdated 同款双端同步）；本轮维持 UNKNOWN 留痕。
④ 审批卡名 "dsh_permission"：桥只带 toolCallId 不带 title，若要更友好需在 daemon 侧按 callId 关联先前 tool_call 的 title（有状态映射，建议主模型裁决是否值得）。
⑤ session/list 参数 cwd 过滤未用（有意，理由见 D）；若未来会话量级大到全量翻页变慢，可改为传 cwd 收窄+分类调整。
