# zcode 字段级覆盖矩阵（2026-10-07）

执行器审计：zcode（fork app-server）能回传的**全部**通知/事件/字段 → daemon 处置 → 证据。
本次目标：消灭用户可见维度的静默丢弃。**未运行任何程序**（含 go/测试/node/zcode 本体）。

## 事实源

| 项 | 值 |
|---|---|
| 本机安装 | `%LOCALAPPDATA%\Programs\ZCode\resources\glm\zcode.cjs`（14,820,968 字节，mtime 2026-09-29；自报版本 0.16.9 系 fork） |
| 定位链 | `zcodeapp.LocateZCode()`（client.go：`ZCODE_NODE`/`ZCODE_CJS` env → 标准安装位） |
| 提取方式 | 只读静态检索内嵌 zod schema（`discriminatedUnion`/`m.enum`/`m.object`），未执行该文件 |
| 佐证 | `docs/verify/evidence/zcode-p0-probe.md`（0.16.5 实测）、`zcode-0169-probe.log`、`zc-subagents-probe.log` |

处置词表：**已映射**=进既有 pb TurnServerMessage 变体；**留痕**=诊断台账+限频诊断帧（nativeNoticeRecorder，unknown_event 契约）；**丢弃**=不上任何通道（需给出理由）。

## A. 顶层通知（stdio，notify 注册表 BR/jV/va）

| method | payload 关键字段 | 处置 | 证据 |
|---|---|---|---|
| `session/event` | 25 型 union，见 B 节 | 已映射/留痕（按型） | bundle `eGt=m.discriminatedUnion("type",…)`；Run switch |
| `state.updated` | {type:"state.updated", scope:server\|workspace\|session, workspace?, sessionId?, revision, reason?, patch} | 已映射：patch→投影 Started（模型/思考档同步）+ PROJECTED 留痕 | bundle `z5i`；zcode.go state.updated 分支；0169-probe 实测 |
| `startup/storageState` | {schemaVersion:1, attemptId, sequence, databaseId, databaseKind:session\|tasks-index, phase:checking\|…\|ready\|failed, migration?, elapsedMs, completed?, total?, errorCode?} | 留痕（PASSTHROUGH；failed 只在启动期，进程级日志另有收口） | bundle `vYe` + 注册表 BR.storageStartup；词表已有人话 |
| `process/resourceSample` / `process/mcpResourceSamples` / `process/mcpTelemetry` / `process/toolExecResource` | 遥测采样 | 留痕（PASSTHROUGH 预收编批） | 注册表 BR；knowledge_zcode 2026-09-30 批 |
| `plugins/operationProgress` | 插件操作进度 | 留痕（PASSTHROUGH 预收编批） | 注册表 BR |
| `interaction/providerRuntimeHeadersCancelled` | 通知（非反向请求） | 留痕（PASSTHROUGH 预收编批） | 注册表 BR |
| `v4/telemetry/event` | 内核遥测 | 留痕（PASSTHROUGH；0.16.9 实测在发） | 注册表 jV；zc-subagents-probe ×8 |
| `v4/conversation/frame` / `v4/telemetry/local-ttft` / `v4/cua/permission-observation` | jV 同族遥测/镜像帧 | **当前 UNKNOWN**（zcodeNoticeKind 未收）→ 建议反馈：与 v4/telemetry/event 同族同口径预收编 PASSTHROUGH。表在 native_notifications.go（本轮禁区），未动 | 注册表 jV={conversationFrame, conversationTelemetryFact, localTtftFacts, cuaPermissionObservation} |
| `computer-use/operation-event` | 桌面轮次操作镜像 | 留痕（PASSTHROUGH；实测在发） | 注册表 va；zc-subagents-probe ×4 |

## B. session/event 载荷 union（fork 25 型，bundle eGt 逐型提取）

| type | payload（bundle zod） | 处置 | 证据 |
|---|---|---|---|
| `turn.started` | {turnNumber, input, inputId?, queryId?, inputSource?, inputVisibility?, executionKind:agent\|controlOnly, targetId?, messageId?, foregroundExecutionId?, intent?, originMeta?, backgroundSource:bash\|subagent, attachments?} | 已映射（日志"已接收任务"）+ PASSTHROUGH 留痕 | bundle `vsr`；Run turn.started 分支 |
| `turn.completed` | {response, tokenCount, usage?, toolCallCount, historyRoundCount?, duration, cacheStats?{totalMessages,cachedMessages,lastCacheHit,cacheReadTokens?}, inputId?, resultType:success\|cancelled\|error_max_turns\|error_max_budget\|error_during_execution\|error_max_tool_calls, backgroundSubagentResultConsumed?} | 已映射：response→终稿兜底、usage 六 token→Usage 帧、resultType→轮次结束原因（非 success 判失败）；cacheStats/tokenCount/toolCallCount/duration/historyRoundCount/serverToolUse{webSearchRequests,webFetchRequests} 无 pb 承载位，丢弃（理由：用量卡只认 Usage 字段；服务端无对应变体，不伪造） | bundle `ksr` + usage 内嵌 schema；mapZcodeTurnCompleted/zcodeTurnCompletedResultType |
| `turn.failed` | {error:{type,message,stack?,code?,detail?,underlyingErrorMessage?,underlyingErrorDetail?,attribution?,retryable?,data?}, turnPhase, inputId?, backgroundSubagentResultConsumed?} | 已映射：error.message→TurnFailed；message 空时 turnPhase 兜底文案；stack/detail/code/retryable 无独立承载位（文案已含主因），丢弃理由成立 | bundle `xsr`+`QHt`；mapZcodeTurnFailed |
| `model.streaming` | 13 值 kind enum，见 C 节 | 已映射（text/reasoning delta）；其余值见 C | bundle `msr`+`fsr` |
| `tool.updated` | 7 型 kind union，见 D 节（**本轮修复主体**） | 见 D | bundle `Asr`（discriminatedUnion("kind")） |
| `session.created` | {mode, contextWindow} | PASSTHROUGH 留痕（daemon 主动建会话，回执） | bundle `gsr` |
| `session.resumed` | {directory, interruptedToolCount, messageCount, partCount, recoveredCompactTimelineCount?, recoveredSteerInputCount?, resumedTodoCount?} | PASSTHROUGH 留痕 | bundle `_sr` |
| `session.titleUpdated` | {messageID?, previousTitle, source:default\|first_input\|generated\|custom, title} | 已映射：title→TitleHandler（双端同步）；previousTitle/source 无承载位，丢弃（标题同步已闭环） | bundle `ysr`；Run session.titleUpdated 分支 |
| `session.closed` | {reason?} | PASSTHROUGH 留痕 | bundle `Msr` |
| `session.updated` | `$x`（自由形状；0.16.9 实测携 model_request_* 遥测） | PASSTHROUGH 留痕（旧 bundle ≤0.16.5 兼容分支仍投影模型态） | bundle `A_("session.updated",$x)`；0169-probe |
| `checkpoint.created` / `rewind.triggered` | `$x`（emission 现场含 rewindId/scope/strategy/targetMessageId/compactBoundaryId/reason） | fork·rewind 反馈：PASSTHROUGH 留痕+限频诊断帧（原始 JSON 随帧上屏）。深度投影需 proto 新字段，未动 | bundle `A_(…,$x)`；emission `createEvent(lt.RewindTriggered,…)` |
| `streamRecovery.updated` | `$x` | PASSTHROUGH 留痕 | 同上 |
| `message.upserted` | {content, attachments?, toolCalls?, type?, compactBoundary?} | PASSTHROUGH 留痕（桌面记录流镜像；流式正文唯一来源是 model.streaming） | bundle `Tsr`；knowledge_zcode 2026-09-30 复验批 |
| `message.removed` | {messageId, reason?} | PASSTHROUGH 留痕 | bundle `Isr` |
| `part.started` / `part.upserted` | {part:qZe} | PASSTHROUGH 留痕（同上镜像粒度） | bundle `UHt` |
| `part.delta` | {messageId, partId, field?:text\|reasoning\|input\|output, delta} | PASSTHROUGH 留痕（增量镜像与 model.streaming 重复） | bundle `Csr` |
| `part.removed` | {messageId, partId, reason?} | PASSTHROUGH 留痕 | bundle `Esr` |
| `permission.requested` | {requestId?, toolCallId, toolName, riskLevel:low\|medium\|high\|critical, reason, input, suggestedPermissionUpdates?{type:"addRules",behavior,rules}, origin?, options[]:{optionId,kind,name,description?,response}, childSessionId?, background?} | 未知管道留痕（UNKNOWN，**既有主模型裁决**：与反向请求 interaction/requestPermission 的双收关系+投影形状需带审批真实轮次实证；本轮新增 bundle 静态形状见左，见建议反馈） | bundle `Psr`+`wYe`+`BZe` |
| `permission.resolved` | {requestId?, toolCallId, toolName?, decision, reason?, modifiedInput?, inputSummary?, childSessionId?, background?} | 同上（UNKNOWN 留痕） | bundle `Rsr` |
| `userInput.requested` | {requestId, prompt, inputType?:text\|choice\|confirm, choices?[string]} | 同上（UNKNOWN 留痕） | bundle `Dsr` |
| `userInput.resolved` | {requestId, value?, cancelled?} | 同上（UNKNOWN 留痕） | bundle `Osr` |
| `turn.steerQueued` | {pendingInputId, inputId?, queryId?, input, inputPreview, inputSize, commandKind?:sendText\|sendGoalCommand\|compact, source?:plan_approval_feedback\|workflow_refine_feedback, toolDisallowlist?, delivery?:queue\|guide, targetTurnId, queueLength, intent?} | PASSTHROUGH 留痕（Steerer 明确不实现：对 running session 发 send 被 -32010 拒，证据 steer-probe） | bundle `Ssr`+`F5i`+`j5i`+`bsr` |
| `turn.steerDrained` | {pendingInputIds[], queryIds?, targetTurnId, injectedMessageIds[], drainedInputs?[{pendingInputId,messageId,text,delivery?,intent?,toolDisallowlist?}]} | PASSTHROUGH 留痕（同上） | bundle `wsr` |
| envelope 外壳 | {eventId, sessionId, turnId?, seq, traceId?, timestamp, deliveryKind?} | sessionId 过滤后消费；turnId/traceId/timestamp 未用（同会话单轮串行，见建议反馈风险①） | bundle `hsr`+`A_` |

## C. model.streaming 的 kind 枚举（13 值）

| kind | 处置 | 证据/理由 |
|---|---|---|
| `text_delta` / `reasoning_delta` | 已映射：TextDelta / ThinkingDelta（HideReasoning 过滤） | 探针实测；mapZcodeStreamDelta |
| `start` / `finish` / `text_start` / `text_end` / `reasoning_start` / `reasoning_end` | 丢弃（理由：生命周期标记，无内容字段；msr strict schema 仅有 delta/toolCallId 等定位字段） | bundle `fsr` 枚举 + `msr` strict |
| `error` | 丢弃（理由：载荷无错误文本字段，真错误由 turn.failed 承载并已映射） | 同上 |
| `tool_input_start` / `tool_input_delta` / `tool_input_end` / `tool_call` | 丢弃（理由：与 tool.updated 起始帧重复，入参已在卡片 JsonArgs 完整透出） | 同上 |

## D. tool.updated 的 kind union（7 型）——本轮修复主体

| kind | fork payload | 修复前处置 | 修复后处置 |
|---|---|---|---|
| `scheduled` | Zpe{toolCallId, toolName?, parentToolCallId?, source?:subagent, agentId?, agentType?, background?, childSessionId?, childToolCallId?, description?} + {toolName, input?, inputByteLength?, inputOmitted?, inputRef?:model_stream, dependencies?, parallelGroupIndex?, canRunParallel?, schedule?} | **静默丢弃**（旧映射只认 tool_call）→ 用户看不到工具起始卡 | **已映射 STARTED**（input 完整入 JsonArgs；子代理识别并记 subagentCalls）；旧 `tool_call` 键保留兼容 |
| `started` | Zpe + {startedAt}（toolName 可选） | **静默丢弃** | **已映射 STARTED**（空名兜底"tool"） |
| `progress` | Zpe + {elapsedMs?, pid?, stdoutBytes?, stderrBytes?, outputBytes?, outputPreview?{text,fullText?,totalLines,totalBytes,linesEstimated?}, stdoutTail?, stderrTail?} | **纯静默**（三映射全不中且无留痕） | **已映射 UPDATE 帧**（pi 同款：预览/stdout/stderr 进输出，Ok=true，elapsedMs→ElapsedMs） |
| `result` | Zpe + {result:k5i(=unknown，实测含 success/content/display?/perf?), duration} | 已映射（但 content 非字符串时反序列化失败→整帧丢失；duration 丢） | **已映射+补齐**：content 按 RawMessage 兜底原文透出；duration→ElapsedMs |
| `error` | Zpe + {error:QHt} | **静默丢弃**（旧映射只认 tool_error）→ 用户看不到失败卡 | **已映射 FINISHED+Ok=false**（error.message/message 兜底文案）；旧 `tool_error` 键保留兼容 |
| `batch` | {kind:"batch", toolCallIds[], successCount, errorCount}（无单卡 id） | **纯静默** | 留痕（KNOWN_PASSTHROUGH，限频诊断帧+台账；理由：并行组汇总是诊断面，单工具结果已各有卡片） |
| `raw` | Zpe + {payload:unknown}（桌面桥原始透传） | **纯静默** | 留痕（KNOWN_PASSTHROUGH；理由：无稳定形状可投影，原文在台账可查） |

测试钉子：`agent/zcode_toolkinds_20261007_test.go`（scheduled/started/progress/error/result 非串 content/分类器）。

## E. 反向请求（服务器→客户端，stdio）

| method | 参数字段 | 处置 | 证据 |
|---|---|---|---|
| `interaction/requestPermission` | {requestId?, toolCallId, toolName, riskLevel, reason, input, suggestedPermissionUpdates?, origin?, options[]:{optionId,kind,name,description?,response}, childSessionId?, background?} | 已映射：审批卡（toolCallId/toolName/input）；riskLevel/reason 卡上未显式展示（卡结构无该位，reason 进审批通道文案）；options 仅用于展示，应答按 schema Jb strict 固定 {decision,reason}（optionId 路径是桌面内部形态，回会被拒） | bundle Psr；handleZcodePermission；zcprobe 实测 |
| `interaction/requestUserInput` | {requestId, sessionId, turnId?, toolCallId?, toolName?, prompt?, questions?[{question,header,options[]:{value,label,description?,preview?},multiSelect?}], input?, origin?, schema?} | 已映射：反问卡（label/description/preview 全透，多选标注）；应答按 jge strict {action,content,reason}，按问题原文收集 | bundle DXn/jge（2026-09-18 提取）；handleZcodeUserInput |
| `elicitation/create` | form{mode?,message,requestedSchema{type,properties,required}} / url{mode:"url",message,elicitationId,url} | 已映射 form→反问卡（enum→点选，属性名排序稳定）；url 模式回 -32601（手机无外开 URL 呈现形态，不伪造 accept） | handleZcodeElicitation |
| `session/requestRuntimePreferences` | schema IKe：nativeSearchEnhancementsEnabled 必填无默认 | 已映射：autoAnswer 回全默认（缺失则 session/create -32022 超时） | client.go autoAnswer；实测 |
| `startup/storagePath` | — | 已映射：autoAnswer 回 storagePathReady{reuse:true} | 同上 |
| `interaction/requestProviderRuntimeHeaders` | {providerId, accountAccess{accountType,mode}} | 已映射：accounts 桥回 headersApplied/requestAuth 或失败态（不伪造） | client.go autoAnswer + accounts.go |
| `interaction/requestOfficialMcpAuthHeaders` | 鉴权头反问 | 安全拒绝 -32601 + UNKNOWN_REQUEST 台账（桌面端职责，已知拒答；人话在词表） | client.go dispatch 兜底；knowledge_zcode 第 4 条 |
| 其余未知反向请求 | 任意 | 未知管道：unknownPassthroughQuery 开启时先弹通用确认卡（KNOWN_PASSTHROUGH），否则 -32601 + UNKNOWN_REQUEST 台账（脱敏原文） | zcode.go reqHandler 兜底 |

## F. 我方请求的应答字段（请求面核对）

| 请求 | 应答消费字段 | 未消费字段（处置=理由） | 证据 |
|---|---|---|---|
| `session/create` / `session/resume` / `session/subscribe` | session.sessionId/title；projection.{contextWindow,totalTokenCount,status,mode}；settings.model.available[]（ref/label/providerLabel/contextWindow/reasoning.defaultLevel/levels/properties.inputFormat）；settings.thoughtLevel.*；slashCommands[]；messages[]（历史） | todos/todoGroups/goalStats/runtime（快照段）：丢弃——无用户可见帧承载位，占用环与用量已走 projection+usage | snapshot.go 全字段；zcodeResumeReply |
| `slashCommands[]` 元素 | name/description/inputHint | `source`(builtin\|custom)：丢弃——pb.SlashCommand 无该字段（禁改 proto），面板功能不受影响 | bundle `XHt`={name,description,inputHint?,source?} |
| `session/setModel` / `session/setMode` / `session/goal` / `session/compact` / `session/close` / `session/send` / `session/stop` | 结果/错误码 | — | zcode.go 控制面 |
| `session/usage`（未使用） | — | daemon 用 turn.completed usage + projection，不另打 | bundle va.sessionUsage |
| `session/subagents`（0.16.9） | {revision, childSessionIds, running[], ended{total,items,nextCursor?}} | nextCursor 翻页未跟（SubagentsHandler 消费首页；清单为辅助卡，量级小）；如需严格对齐可后续补 | bundle `W5i`/`Lsr`/`Fsr`；zcode_subagents.go |

## G. 静默丢弃总账（本轮处置后仍成立的，全部有理由）

1. model.streaming 生命周期 kind（C 节 4+6 值）：无内容/重复载体。
2. turn.completed 的 cacheStats/tokenCount/toolCallCount/duration/historyRoundCount/serverToolUse：pb.Usage 无承载位（需新字段，见建议反馈③）。
3. slashCommands[].source：pb 无承载位。
4. turn.started 的 input/attachments 回显：输入已在手机端本地展示。
5. envelope 的 traceId/timestamp：诊断台账已含原始载荷。
6. 批注：D 节 batch/raw、permission.* 四件套走留痕管道，非丢弃。

## 建议反馈（交主模型）

① **事件不按 turnId 过滤**：Run 接受同 session 的全部事件。fork turn.started 携 backgroundSource=bash|subagent，若后台轮事件走同一订阅流，存在前台正文被污染的理论通道（无法静态证实订阅粒度，未改；建议主模型用真实后台任务实测后裁决）。
② `v4/conversation/frame`、`v4/telemetry/local-ttft`、`v4/cua/permission-observation`（jV 同族）当前落 UNKNOWN；建议按 v4/telemetry/event 同口径预收编 PASSTHROUGH——分类器在 native_notifications.go（本轮禁区）。
③ permission.requested/resolved、userInput.requested/resolved 四件套维持 UNKNOWN（既有裁决），但本轮已取得 payload 的 bundle 静态形状（见 B 节），具备"预收编 PASSTHROUGH"的同款证据条件；是否推翻遗留⑤请主模型裁决。
④ session/subagents 的 ended.nextCursor 未翻页。
⑤ pb.Usage 无 web 检索次数/cacheStats 承载位；如需透出 serverToolUse 须动 proto（本轮禁改）。
