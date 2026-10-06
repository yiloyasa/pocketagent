# codex 接入字段级全适配覆盖矩阵(2026-10-07)

- 对象:daemon 对 codex app-server(app-server 通知/反向请求/字段)的全量适配
- 本机安装版本:`@openai/codex 0.160.0`(npm 全局,prefix 派生链见 daemon/internal/hostenv/locate.go;可执行体 `vendor/x86_64-pc-windows-msvc/bin/codex.exe`,只读翻阅,**未运行**)
- 事实源(按任务优先级):
  1. 本机 codex 0.160.0 二进制内的协议面字符串(serde 方法名枚举表、字段名 literal;只读 `grep -a` 提取,未执行);
  2. 仓库快照 `codex-schema/ServerNotification.json`(83 通知,vendored 0.159.2)、`codex-schema/ServerRequest.json`(10 反向请求)、`codex-schema/v2/*.json`(逐通知定义,含 RawResponse*);
  3. 历史实证:`docs/verify/evidence/codex-01592-liveturn-20261002.log`(活体单轮收口帧带 `"status":"completed"`)、`codex-0156-probe.log`、`codex-subagent-s0.md`;
  4. 现有代码:daemon/internal/codexapp/client.go、daemon/internal/agent/codex*.go。
- 对齐结论:**vendored 0.159.2 schema 的 83 条通知在 0.160.0 二进制中逐名全命中,零消失**;0.160.0 相对快照的增量只有 3 条,全部在本轮处理(见 §2/§4)。
- 验收声明:**本轮未运行任何程序**(含 go/测试/node/codex 本体);所有证据来自只读文件翻阅。

## 1. 协议面规模

| 面 | vendored 快照(0.159.2) | 本机 0.160.0 二进制 | 处理面 |
|---|---|---|---|
| ServerNotification(通知) | 83 | 85(新增 `rawResponseItem/completed`、`rawResponse/completed`,内部用) | 登记 60 PROJECTED/PASSTHROUGH + 明确不收 23;新增 2 条落未知管道(§4) |
| ServerRequest(反向请求) | 10 | 11(新增 `currentTime/read`) | 已知 10 类全接;`currentTime/read` 本轮补齐应答 |
| ThreadItem(item/started|completed 载荷) | 19 类 | 19 类(逐名命中) | 专案分支 5 类 + 本轮新增工具卡 3 类 + 透传 11 类 |

## 2. 0.160.0 相对快照的增量(3 条)

| 方法 | 类型 | 载荷(schema v2/ 或二进制) | 处置 | 证据 |
|---|---|---|---|---|
| `rawResponseItem/completed` | 通知 | Internal-only:responseId/threadId/turnId(+item) | 不注册,走未知事件管道(runtime.UnknownEventLog,契约 daemon/internal/server/unknown_event_e2e_test.go):轮内限频透传卡 + 台账 UNKNOWN | 0.160.0 二进制通知枚举表序 `…item/completed→rawResponseItem/completed→rawResponse/completed→item/agentMessage/delta…`;codex-schema/v2/RawResponseItemCompletedNotification.json |
| `rawResponse/completed` | 通知 | Internal-only:{responseId,threadId,turnId,usage:TokenUsageBreakdown|null,usageMetadata} 同上。用户可见的用量已由 `thread/tokenUsage/updated` 投影(pb UsageUpdate),逐上游 API 调用一条的原始用量若投影会刷屏;登记表受 schema 对表测试约束(codex_notice_schema_20261002_test.go 测试①要求登记名∈快照),不满足注册前提 | 同上 | 同上 + codex-schema/v2/RawResponseCompletedNotification.json("Internal-only notification containing the exact usage from one upstream Responses API completion") |
| `currentTime/read` | 反向请求 | params {threadId};应答 {"currentTimeAt": <unix 秒>} | **本轮补齐**:classifyCodexRequest → reqCurrentTime → 如实应答(协议 fail-closed:不应答本轮在模型请求前直接停) | 0.160.0 二进制请求面 `…attestation/generate currentTime/read applyPatchApproval…` + 应答字段 literal `currentTimeAt`;公开协议文档([codex-cli time-awareness](https://codex.danielvaughan.com/2026/06/23/codex-cli-v0142-time-awareness-scheduled-utc-reminders-clock-curr-time-tool-temporal-agent-workflows/)、[current_time.rs](https://upd.dev/openai/codex/src/tag/rust-v0.143.0-alpha.24/codex-rs/app-server/src/current_time.rs));**证据分级:形状为二进制+文档双侧印证,未活体实测(红线禁跑)** |

## 3. 通知覆盖矩阵(83 条快照逐条)

图例:PROJECTED=投影进既有 pb 变体;PASSTHROUGH=已知只留痕(台账 KNOWN_PASSTHROUGH+词表);UNKNOWN=落 runtime.UnknownEventLog 诊断台账;"明确不收"=桌面产品面,出现即如实 UNKNOWN。

### 3.1 已登记:PROJECTED(有投影帧)

| 通知 | pb 投影落点 | 证据(源文件:行) |
|---|---|---|
| error | CodexEvent(ERROR/RETRY_STARTED)+ Run.lastErr | codex_events.go:46 parseCodexErrorNotification;codex.go error 分支 |
| warning | CodexEvent_WARNING | codex_events.go "warning" case |
| guardianWarning | CodexEvent_WARNING / GUARDIAN_REVIEW(双分支) | codex_events.go;codex_events_ext.go |
| turn/completed | Run 收口 + TextDelta 兜底 + lastErr + **本轮新增:turn.status 结束原因**(failed 无 error → 失败收口;interrupted → WARNING 中断卡) | codex.go turn/completed 分支;形状实证 codex-01592-liveturn-20261002.log 收口帧 |
| turn/started | steer 目标登记 + 日志(无协议帧,刻意) | codex.go turn/started 分支 |
| item/started | ToolCallUpdate STARTED(shell)/压缩卡/子代理卡/**本轮新增 fileChange/mcpToolCall/webSearch 开始卡**/其余透传 | codex.go item/started 分支;codex_items.go |
| item/completed | ToolCallUpdate FINISHED + **本轮新增 exitCode 人话**/终稿聚合/压缩卡/子代理卡/**本轮新增三类工具卡**/其余透传 | 同上;CommandExecutionThreadItem.exitCode(schema definitions) |
| item/agentMessage/delta | TextDelta | codex.go agentMessage delta 分支 |
| item/reasoning/summaryTextDelta、item/reasoning/textDelta | ThinkingDelta(HideReasoning 过滤) | codex.go isReasoningDeltaMethod 分支 |
| item/reasoning/summaryPartAdded | PASSTHROUGH(思考分段标记,投影已有主载体)→ 归 PASSTHROUGH,见 §3.2 | codex.go 登记表 |
| item/plan/delta | CodexEvent_STATUS(执行计划生成中) | codex_events.go |
| turn/plan/updated | CodexEvent_STATUS + PlanSteps[] 结构化 | codex_events.go(TurnPlanStep{step,status} 与解析逐字段一致) |
| item/commandExecution/outputDelta | CodexEvent_STATUS(命令执行过程) | codex_events.go;schema CommandExecutionOutputDeltaNotification{delta,itemId,threadId,turnId} 全接 |
| item/fileChange/outputDelta | CodexEvent_STATUS | codex_events.go |
| item/fileChange/patchUpdated | CodexEvent_STATUS(文件变更预览)+ changeFiles 审批卡积累 | codex_events.go;codex.go patchUpdated 分支;changes[{path,diff,kind}] 全接,diff 不重复上卡(turn/diff/updated 承担) |
| item/mcpToolCall/progress | CodexEvent_STATUS | codex_events.go;schema {itemId,message} 全接 |
| item/commandExecution/terminalInteraction | CodexEvent_WARNING(itemId+stdin 全接;processId 丢弃,见 §5) | codex_events.go;schema TerminalInteractionNotification |
| command/exec/outputDelta、process/outputDelta | CodexEvent_STATUS(deltaBase64 解码 + stream→Code;capReached 丢弃,见 §5) | codex_events.go;schema 带 capReached/processId(processHandle) |
| thread/tokenUsage/updated | UsageUpdate 帧 + CodexEvent_STATUS(用量卡)+ 自动压缩状态机 | codex_events.go codexTokenUsageUpdate;codex.go tokenUsage 分支;last/total 六字段+modelContextWindow 与 schema TokenUsageBreakdown 全接 |
| account/rateLimits/updated | CodexEvent_RATE_LIMIT + **本轮新增 resetsAt/windowDurationMins/rateLimitReachedType 人话** | codex_events.go;schema RateLimitSnapshot |
| mcpServer/startupStatus/updated | CodexEvent_STATUS(name/status/error/failureReason 全接) | codex_events.go;schema McpServerStatusUpdatedNotification |
| model/rerouted | CodexEvent_MODEL_REROUTED(fromModel/toModel/reason 全接) | codex_events.go |
| model/safetyBuffering/updated | CodexEvent_MODEL_SAFETY(model/fasterModel/reasons 全接;useCases/showBufferingUi 未上卡,见 §5) | codex_events_ext.go |
| thread/compacted | CodexEvent_CONTEXT_COMPACTED + 压缩状态机重置 | codex_events.go;codex_compact.go |
| thread/status/changed | CodexEvent_STATUS + **本轮新增状态/旗标人话**(notLoaded/idle/systemError/active、waitingOnApproval/waitingOnUserInput) | codex_events.go;schema ThreadStatusChangedNotification |
| thread/name/updated | TitleHandler(标题同步)+ ThreadEventHandler | codex.go;schema threadName 可空已判空 |
| thread/archived / thread/unarchived / thread/deleted / thread/reverted / thread/closed / thread/started / thread/queue/changed / skills/changed | ThreadEventHandler(server 层同步本地库) | codex.go attachGlobalHandler;thread/started 的 thread.id 回退 codexThreadIDOf(codex.go,实证 codex-subagent-s0.md) |
| thread/goal/updated、thread/goal/cleared | GoalHandler + **本轮新增:全局钩子路径同样接 GoalHandler(轮外目标变化不再只留痕)** | codex_goal.go goalStatusFromNotification(ThreadGoal.objective/status/tokensUsed/timeUsedSeconds 全接;tokenBudget/createdAt/updatedAt 丢弃,见 §5);codex.go 全局 handler 新 case |
| item/autoApprovalReview/started、item/autoApprovalReview/completed、autoApprovalReview/strictReviewRequired | CodexEvent_GUARDIAN_REVIEW(reviewId/targetItemId/riskLevel/reviewStatus/rationale 全接)+ recordGuardianNotification | codex_events_ext.go;codex_guardian.go;**本轮新增第七动作变体 writeStdin 的人话标签** |
| deprecationNotice、configWarning | CodexEvent_DEPRECATION / CONFIG_WARNING(summary/details/path 全接;configWarning.range 丢弃,见 §5) | codex_events_ext.go |
| windows/worldWritableWarning | CodexEvent_SANDBOX_WARNING(samplePaths/extraCount/failedScan 全接) | codex_events_ext.go |
| hook/started、hook/completed | CodexEvent_HOOK(run.eventName/status/statusMessage/sourcePath 全接;entries 丢弃,见 §5) | codex_events_ext.go;schema HookRunSummary |
| modelProvider/authRecoveryStarted、modelProvider/authRecoveryCompleted | **本轮升级 PROJECTED**:CodexEvent_WARNING(message+provider 全接;此前只留痕,错误维度不静默) | codex_events_ext.go 新分支;codex.go 登记表;词表词条已有(runtime/knowledge_codex.go) |
| model/verification | **PASSTHROUGH→见 §3.2**(空列表时不投影) | codex_events_ext.go |

### 3.2 已登记:PASSTHROUGH(已知、只留痕,台账+词表人话)

| 通知 | 不投影的理由 | 证据 |
|---|---|---|
| turn/diff/updated | CodexEvent_TURN_DIFF **有投影**(diff 非空时,codex_events_ext.go);登记为 PROJECTED 更准确——现状:有投影分支,登记表记 PROJECTED | codex_events_ext.go turn/diff/updated 分支;codex.go 登记表 |
| model/verification | 有条件投影(verifications 空数组返回 false → 落台账);verifications 枚举内容(trustedAccessForCyber)无 UI 消费方 | codex_events_ext.go;schema ModelVerificationNotification |
| thread/environment/connected、thread/environment/disconnected | 环境接通/掉线只留痕;无配套投影变体(CodexEvent 无对应 Kind) | codex.go 登记表;knowledge_codex.go 词条 |
| remoteControl/status/changed | 桌面远程控制开关,手机端无消费方 | codex.go;载荷 {status,serverName} 注释在案 |
| thread/settings/updated | 桌面端改设置的通知,daemon 的设置快照以自身落库为准 | codex.go;schema ThreadSettingsUpdatedNotification |
| configWarning、deprecationNotice | **实际是 PROJECTED**(§3.1),登记表口径以代码为准 | — |
| turn/moderationMetadata | metadata 是无结构透传对象(schema metadata:true),无 UI 消费方 | schema TurnModerationMetadataNotification |
| account/updated | 账号面变更(authMode/planType 枚举),用量卡已有 RATE_LIMIT 通路 | schema AccountUpdatedNotification |
| account/login/completed | 登录流程回执(success/error/loginId),daemon 不发起登录 | schema AccountLoginCompletedNotification |
| mcpServer/oauthLogin/completed | MCP 授权回执,daemon 不发起 | schema |
| process/exited | 后台进程遥测(process/spawn 族是桌面面,daemon 从不 spawn) | schema {exitCode,processHandle,stdout,stderr,capReached} |
| serverRequest/resolved | 幽灵审批卡治理已消费(codex.go resolvePendingServerRequest),登记为 PROJECTED 性质——现状登记 PASSTHROUGH,消费在先,见 §6 遗留① | codex.go serverRequest/resolved 分支 |

注:§3.1/§3.2 的登记档位以 `codexGlobalNoticeKinds`(codex.go)为唯一权威,本矩阵与其逐条核对一致;wordlist 人话在 runtime/knowledge_codex.go(本轮未改,词已齐)。

### 3.3 明确不收清单(23 条,出现即如实 UNKNOWN 留观)

thread/attachment/updated、project/changed、thread/project/updated、mcpServer/event/stream/notification、account/gatewayOAuth/changed、app/list/updated、externalAgentConfig/import/progress、externalAgentConfig/import/completed、fs/changed、fuzzyFileSearch/sessionUpdated、fuzzyFileSearch/sessionCompleted、thread/realtime/*(11 条)、windowsSandbox/setupCompleted —— 桌面产品面,手机端无消费方;穷举集钉在 codex_notice_schema_20261002_test.go 的 codexNoticeUnclaimedAllowlist(双向对表:清单⊆schema 且 schema−登记表⊆清单)。

### 3.4 0.160.0 增量 2 条通知

rawResponse/completed、rawResponseItem/completed → 未注册(注册前提不成立,见 §2),落未知事件管道:轮内有限频透传卡(nativeNoticeRecorder,每 method 2 条)+ 台账 UNKNOWN 留观。**不是静默丢弃**。

## 4. 反向请求矩阵(11 条)

| 请求 | 处置 | 应答形状 | 证据 |
|---|---|---|---|
| item/commandExecution/requestApproval、execCommandApproval | 审批卡(kind=command/writeStdin 两态;commandActions/proposedExecpolicyAmendment 人话详情;三按钮 accept/acceptForSession/cancel,旧版 approved/approved_for_session/abort) | {"decision":…} | codex_requests.go;codex-schema/CommandExecutionRequestApprovalParams.json |
| item/fileChange/requestApproval | 审批卡(文件列表来自 patchUpdated 累积) | {"decision":…} | codex_requests.go;codex.go changeFiles |
| applyPatchApproval | 审批卡(自带 fileChanges) | {"decision":…} | codex_requests.go |
| item/permissions/requestApproval | 审批卡(permissions 原样透传) | {"permissions":…,"scope":"turn"} / {"permissions":{}} | codex.go handleCodexApproval |
| item/tool/requestUserInput | 反问卡(questions 带 isSecret/isOther/options) | {"answers":{…}} | codex_requests.go splitAnswerNote |
| mcpServer/elicitation/request | 确认卡 | {"action":"accept"/"decline"} | codex.go |
| item/tool/call | reqUnsupported:回 -32601(未注册任何动态工具) | error | codex_requests.go |
| account/chatgptAuthTokens/refresh | reqUnsupported:回 -32601(不托管凭证;RespondError 语义修复在案) | error | codexapp/client.go RespondError 注释 |
| attestation/generate | reqUnsupported:回 -32601 | error | codex_requests.go |
| **currentTime/read(0.160.0 新增)** | **本轮补齐**:reqCurrentTime,如实回墙钟 | {"currentTimeAt": <unix 秒>} | codex.go reqHandler;codex_requests.go classify;证据见 §2 |
| (未知) | 辅线 B 通用确认卡 / -32601 + UNKNOWN_REQUEST 台账 | error | codex.go reqHandler |

## 5. 字段级丢弃台账(映射了事件但未上卡的字段,逐个写明理由)

| 字段 | 所在通知 | 理由 |
|---|---|---|
| processId(TerminalInteraction)、processHandle/processId(CommandExec/ProcessOutputDelta) | terminalInteraction、outputDelta 族 | 卡片以 itemId 为主键,进程句柄是桌面调度细节,无用户动作可做 |
| capReached | command/exec/outputDelta、process/outputDelta | daemon 侧自有 clip 截断口径,双截断语义叠 加反而误导 |
| startedAtMs/completedAtMs/turnId(item 帧)、summaryIndex/contentIndex | item/started|completed、reasoning delta | 手机端流式卡按到达序渲染,时间戳/索引无渲染位;turnId 关联由 Run 会话作用域承担 |
| diff(FileChangePatchUpdated 的 changes[].diff) | patchUpdated | 审批卡只列路径;diff 有 turn/diff/updated 专卡(TURN_DIFF) |
| hook run.entries[{kind,text}]、displayOrder、executionMode、handlerType、scope、startedAt/completedAt/durationMs | hook/started|completed | statusMessage 是 hooks 的人话摘要位;entries 是脚本原始输出,量大且可经 RawJson 通道追溯 |
| configWarning.range | configWarning | path 已定位文件;行列坐标无 UI 位 |
| model/safetyBuffering 的 useCases/showBufferingUi | safetyBuffering | reasons/fasterModel 已承载"为什么慢/怎么绕开";其余为桌面 UI 开关 |
| goal 的 tokenBudget/createdAt/updatedAt | thread/goal/updated | GoalHandler 同步位只有 objective/status/tokensUsed/timeUsed |
| review.userAuthorization、decisionSource、completedAtMs/startedAtMs | autoApprovalReview | riskLevel/rationale/status 已承载判定;userAuthorization 属策略侧细节 |
| itemsView、memoryCitation、phase、delivery、questions(ThreadItem/Turn) | item/completed、turn/completed | itemsView 是桌面视图选择器;memoryCitation 引用面无手机端渲染;questions 属 realtime 反问(桌面);delivery=async 枚举 |
| RawResponse* 的 usage/usageMetadata | rawResponse/completed | 见 §2:用量已由 thread/tokenUsage/updated 投影 |

## 6. 风险点与遗留

1. **登记档位微瑕(既有,未动)**:serverRequest/resolved 实际有消费(幽灵卡治理)、turn/diff/updated 与 model/verification 有条件投影,登记表记 PASSTHROUGH/PROJECTED 与代码行为有口径差;改档位会牵动 runtime 词表测试(event_translate_test.go 按 kind 显式传参,不在本轮允许触碰的文件集),仅记录。
2. **writeStdin 的审查回写形状未知**:guardianAssessmentEvent 对 writeStdin 原样透传(不猜 snake_case);codex 拒绝时原话上屏(codex_guardian.go 的失败语义),等待活体证据后再定 guardianActionTypeToWire 是否加 "write_stdin"。
3. **currentTime/read 应答形状为静态证据**(二进制字段 literal + 公开文档),未活体实测;若 0.160.x 变更形状,fake 钉子(TestCodexCurrentTimeReadAnswered)会在下一次真机验证时暴露。
4. **rawResponse×2 未注册**:下一次用 0.160.0 重新导出 schema 快照后,应把它们收编为 PASSTHROUGH(词表补词条),消除台账里的常规 UNKNOWN 源。
5. vendored 快照仍是 0.159.2 导出;本轮以二进制逐名核对补齐了版本差(3 条),其余 83+10 零漂移。

## 7. 本轮改动清单(file:line 为改动后位置)

- daemon/internal/agent/codex.go:turn/completed 的 turn.status 结束原因(failed 无 error → lastErr;interrupted → WARNING 中断卡);reqHandler 新增 reqCurrentTime 应答;全局钩子新增 thread/goal/updated|cleared → GoalHandler;登记表 authRecovery×2 升 PROJECTED;itemNotif 增 exitCode;commandExecution 收尾卡带非零退出码;item/started、item/completed 挂 codexItemToolCard。
- daemon/internal/agent/codex_requests.go:reqCurrentTime 类别 + classifyCodexRequest("currentTime/read")。
- daemon/internal/agent/codex_events.go:thread/status/changed 人话(codexThreadStatusLabel/codexThreadFlagsLabel);account/rateLimits/updated 增 resetsAt/windowDurationMins/rateLimitReachedType(codexRateLimitReachedLabel);import time。
- daemon/internal/agent/codex_events_ext.go:modelProvider/authRecovery×2 WARNING 投影;三个 label 辅助函数。
- daemon/internal/agent/codex_guardian.go:guardianActionLabel 第七变体 writeStdin。
- daemon/internal/agent/codex_items.go(新):codexItemToolCard(fileChange/mcpToolCall/webSearch → pb.TurnServerMessage_ToolCall)+ fileChangeOutcomeLabel。
- daemon/internal/agent/codex_effect_test.go:fake app-server 增加应答帧记录("<client-response>")与 PA_FAKE_CODEX_CURRENT_TIME / _ITEMS_NOTIFY / _AUTH_RECOVERY_NOTIFY / _TURN_STATUS 注入钩子。
- daemon/internal/agent/codex_fieldcoverage_20261007_test.go(新):9 个测试钉子(分类/活体应答/中断卡/失败收口/工具卡形状/authRecovery 投影/状态人话/限额字段/writeStdin 标签),payload 全部按 vendored schema 与活体探针形状构造。
- 未触碰:proto/pb 生成码、server/、piapp/zcodeapp/dshapp/hermesapp、term/manager.go、tools/device/**;无新增 exec 调用;无个人环境标识入码。
