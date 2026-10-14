# 四家 Agent 能力位矩阵（2026-09-30 · 批次 4/4 产出）

> **基线**：HEAD `a5f3f3e`（feat(android,daemon): 深度体验优化 P1 四项+能力位 image_input+Compact 路由修复）。2026-09-30 协议适配轮（codex 0.159.2 / pi 0.99.1 / dsh 0.2.0-rc.2 / zcode 0.16.9 复验）已合流。
> **本轮批次上下文**：四批次并行——批次 1~3 在改生产源码（image_input 位 Android 侧切换：硬编码 dsh → 能力位判定；native config 写后失效链补全等）；本批次只产出文档与测试，不碰生产代码。凡标注「（本轮切换中）」的格子，以批次 1~3 合入后的代码为准。
> **证据分级**（沿用 2026-09-25 gap-analysis 口径）：`(实证)`=探针/live 实测、`(代码)`=仓库内代码路径、`(bundle)`=本机二进制静态提取、`(留观)`=已评估暂不做。出处写「文件:行」的均指基线 `a5f3f3e` 时的位置。
> **口径纠偏**：daemon 侧生成的位格式是 `<位>:<agentId>`（如 `image_input:dsh`），**没有** `capability:` 统一前缀；此前沟通稿写「`capability:<agentId>` 位（host.go:151 一带）」，位置对（host.go:151 正是 image_input 位生成行）、格式表述不准，以本矩阵为准。
> **2026-10-14 增补**：pi fork 格更正为 ✅（证据 pi.go:1878 ForkThread＋host.go:155 piShapedForker＋pi_fork_test.go）、补 hermes 列（本表原为四家时代产物，hermes 是第 5 家）；逐格事实与来源见 `docs/research/2026-10-14-agent-adapter-gap.md`，steer 位语义定案见第二节表后注。

## 一、能力位怎么来的（生成机制，30 秒版）

daemon 在 `GetHostInfo.capabilities` 里对**每个已注册 provider** 做接口类型断言，逐位如实宣告（`daemon/internal/server/host.go:75-155`）：谁有真实现谁带位，**缺位 = 手机端入口隐藏，不摆假按钮**。同一循环生成宿主级静态位。手机端统一入口是纯函数 `agentCapableOf(bit, agentId, capabilities)`（`android/.../ui/SlashCommandsLogic.kt:86`，大小写/空白不敏感、agentId 空=拒绝、只认同 agent 的位）。

能力位之外还有第二套「能力声明」：runtime 描述符 `runtime.Capability`（`daemon/internal/runtime/descriptor.go:43-72`，逐家形状见 `daemon/internal/agent/runtime_descriptors.go`）。它不进 GetHostInfo、不做 UI 门控，是运行时契约面，本矩阵单列为 B 区。

## 二、主矩阵 A：GetHostInfo 能力位（host.go 生成的全集，13 位族 + 6 宿主级）

图例：✅已接（有位有实现有出处）｜✅*条件位（接口在场但生成还需运行时真值）｜⚠️接口在场但能力恒假（如实走排队落点）｜❌不适用｜⛔受阻｜🔭留观。

| 能力位族（格式 `<位>:<agentId>`） | codex | pi | zcode | dsh | hermes | 判据（host.go 断言的接口/形状） | 出处 |
|---|---|---|---|---|---|---|---|
| `agent:<id>`（pi 追加版本 `agent:pi:v0.87.1`） | ✅ | ✅ | ✅ | ✅ | ✅ | 已注册即宣告（echo/glm/deepseek 同理） | host.go:77,201-210（`agentCapability`）；hermes 注册点 provider.go:228-231（DetectProviders）；版本格式回归钉 OnboardingAgentCapabilityTest.kt |
| `agent_settings:<id>`（设置页七项偏好，PrefsAware） | ✅ | ✅ | ✅ | ✅ | ✅ | `agent.PrefsAware`（ApplyPrefs 全量快照覆盖），五家编译期钉死（2026-10-14 补 hermes 断言：provider.go var 块 `_ PrefsAware = (*HermesProvider)(nil)`） | provider.go:93-105（基线）；host.go:78-80；hermes ApplyPrefs 实现在 hermes.go:147 |
| `agent_native_settings:<id>`（原生设置目录透出） | ❌ | ✅ | ❌ | ✅ | ❌ | `agent.NativeConfigOptions`，实现者仅 pi/dsh | native_config_options.go:51（接口）；pi_config_options.go:28、dsh_config_options.go:36；host.go:84-86；zcode 缺实现=遗留⑦ |
| `text_generation:<id>`（提示词优化） | ✅ | ✅ | ✅ | ✅ | ✅ | `agent.TextGenerator`（GenerateText）五家全实现；dsh/hermes 为 daemon 侧 ephemeral 模拟模式（短会话+denyGenerationTool 只写文本，2026-10-14 P1 接线——协议无原生 text generation，模拟形态复刻 codex/pi 先例） | text_generator.go:17-104（三家）+ :116（hermes）+:143（dsh）；host.go:87-89；编译钉死 var _ TextGenerator 块 |
| `collab:<id>`（跨 agent 协同入口） | ✅* | ✅* | ✅* | ✅* | ✅* | `agent.CollabAware && CollabReady()`——X2 收口后五家（含 hermes）接口全部实现；`*` = 还要接线真值：pi 看 `piapp.CollabEnvInjection()`（本机目录），dsh 看部署级 profile 补丁层（`dshCollabWired`，会话级判据已随接线迁移弃用），codex 看 `-c mcp_servers.pocketagent-collab.*` argv 覆盖要素，zcode/hermes 看会话参数注入要素（三者同判据：daemon exe + 数据目录可解析，见 agent/collab_inject.go） | provider.go:56-60；pi.go:148、dsh.go CollabReady、collab_inject.go（X2）；host.go:93-95；判定回归钉 collab_ready_test.go + collab_inject_test.go；hermes CollabReady 在 collab_inject.go:205 |
| `steer:<id>`（运行中真插话） | ✅ | ✅ | ❌ | ❌ | ⚠️ | `agent.Steerer`：codex 原生 turn/steer、pi 原生插队；zcode 实测 session/send 轮中被 -32010 拒、dsh 无 | codex_steer.go:51、pi.go:1340；host.go:102-104；zcode 实测结论 host.go:97-99 注释；hermes 接口在场但 Steer 恒 false（hermes.go:609-611），2026-10-14 位语义定案见表后注 |
| `queue_clear:<id>`（撤回上游排队输入） | ❌ | ✅ | ❌ | ❌ | ❌ | `agent.QueueClearer`，仅 pi 0.87.1 原生 clear_queue（子进程内存队列） | pi.go:1376-1389（接口+编译钉死+ClearQueue）；host.go:109-111；hermes 无 QueueClearer 实现（2026-10-14 grep 零匹配） |
| `compact:<id>`（压缩上下文） | ✅ | ✅ | ✅ | ❌ | ✅ | 四种原生形状任一：codex `CompactThread(string)` / pi 与 hermes 共享 `CompactThread(*store.Thread)` / zcode `CompactSession(string)`；hermes 走 `/compress` slash 通道（探针 live 实证秒回），三道闸在 provider 侧（轮次在跑 TryLock 快拒/未落地拒/预算超时先 session/cancel），server 路由 thread.go hermes 显式分支绝不落 codex 兜底；dsh 无压缩实现（路由显式 FailedPrecondition） | codex_compact.go:123、pi.go:1468、zcode.go:735、hermes_compact.go:55（2026-10-14 接线）；host.go:121-129（形状互斥都宣告同一位）；server/hermes_compact_route_test.go 三用例 |
| `fork:<id>`（分叉会话） | ✅ | ✅ | ❌ | ❌ | ❌ | `ForkThread` **两种形状任一在场即同告一位**：codex `ForkThread(codexThreadID, lastTurnID)`（thread/fork）＋ pi `ForkThread(*store.Thread)`（0.99.1 clone），与 compact 三形状一位同一先例 | codex_threads.go:308；**pi：pi.go:1878 ForkThread＋host.go:155 `piShapedForker`（thread.go:585）＋pi_fork_test.go、server/pi_fork_route_test.go**（2026-10-14 更正：原标 pi ❌「路由只落 codexProvider」系 4 家时代残留，server 侧 host_capability_bits_test.go:83-87 一直钉 fork:pi 必须在场）；host.go:130-132（基线行号） |
| `rollback:<id>`（回滚回合） | ✅ | ❌ | ❌ | ❌ | ❌ | `RollbackThread(codexThreadID, numTurns)`（0.156.1 起 rollback→thread/revert 已迁移） | codex_threads.go:357；host.go:133-135；迁移记录 gap-analysis「Codex 0.156.1」节；hermes 无 RollbackThread（2026-10-14 grep 零匹配） |
| `review:<id>`（计划审查发起） | ✅ | ❌ | ❌ | ❌ | ❌ | `StartReview(*store.Thread, focus)`（review/start） | codex_review.go:65；host.go:136-138；hermes 无 StartReview（2026-10-14 grep 零匹配） |
| `export_html:<id>`（HTML 存档导出） | ❌ | ✅ | ❌ | ❌ | ❌ | `ExportHTML(*store.Thread)`，pi 专属 | pi.go:1503；host.go:139-141；hermes 无 ExportHTML（2026-10-14 grep 零匹配） |
| `image_input:<id>`（图片附件真正进入模型输入） | ✅ | ✅ | ✅ | ❌ | ❌ | `agent.ImageInputter && SupportsImageInput()`：codex turn/start LocalImage、pi prompt/steer 携 base64、zcode session/send attachments 三家 Run 有真实图片路径；dsh `promptCapabilities.image` 实测 false，Run 门控如实报错；hermes 为**协商条件位**——initialize 广播 image=true 时 SupportsImageInput() 才真（探针 live 实证本机为 true），未协商时位不亮、Run 带图如实拒绝；0.2.0-rc.2 起动态判定（bundle：supportsAcpImagePrompts）——2026-09-30 live 复验定案：本机路由 image=false 实帧，带图 prompt 被准入层 -32602 "inline image prompts were not advertised" 拒绝（证据 dshlive_proto020.md），定案「等上游翻真」：翻真时 SupportsImageInput() 动态放行、daemon 图片块构造自动生效，无适配债 | image_input_capability.go:36-61（接口+编译钉死+查证结论）+:74（hermes 协商位）；host.go:150-152；hermes 多块 prompt 见 hermes_image_input.go（2026-10-14 接线）；daemon 侧位生成已被 43ee0cf/a5f3f3e 落地；Android 侧切换=本轮批次 1~3 落地中（见接线表） |
| **宿主级**（无 agentId 后缀）：`terminal` `fs` `git` `agent` `event_sequence` `runtime.v1` | — | — | — | — | — | 固定追加（`runtime.v1`=运行时控制面入口；`event_sequence`=事件断点续传） | host.go:154-159；Android 侧 runtime.v1 门控 RuntimeStateLogic.runtimeControlCapable |

> 守护：A 区全集与代码的一致性由两层测试钉住——`daemon/internal/server/host_capability_bits_test.go`（真实五家构造器跑 GetHostInfo 对表，2026-10-14 补 hermes）与本轮新增 `daemon/internal/agent/capability_gate_test.go`（位族词汇表 ↔ 本文档对表，位增删即红）。
>
> **2026-10-14 增补注（P0 一致性四条）**：
> 1. **fork pi 格更正为 ✅**：pi 分叉现实已实现——pi.go:1878 `ForkThread`＋host.go:155 `piShapedForker` 断言＋`pi_fork_test.go`/`server/pi_fork_route_test.go` 在位，server 侧 `host_capability_bits_test.go:83-87` 一直钉 fork:pi 必须在场；原 ❌ 系 4 家时代残留。同步更正 `capability_gate_test.go` 的 fork 探针（此前只探 codex 形、漏探 pi 形，与 host.go 双形状判据不符）。
> 2. **hermes PrefsAware 断言补钉**：`provider.go` var 块补 `_ PrefsAware = (*HermesProvider)(nil)`（ApplyPrefs 早在 hermes.go:147）——编译期断言名单由四家扩为五家，与 DetectProviders 注册面、host.go 位生成对齐（P0-4 断言对账：provider.go 内仅此一份断言名单，补钉即全覆盖）。
> 3. **steer 位语义定案（hermes ⚠️ 位在场恒 false vs dsh ❌ 无位）**：定案前先读手机端无位分支回执——`ChatScreen.kt:2118-2119 steerSubmitNotice` 按位二选一（有位「已插入运行中任务」／缺位「已排队」），运行中发送必弹该 Toast（ChatScreen.kt:3629-3634），时间线另有 Steered/Queued 徽标如实裁定（TurnDelivery.kt）。故缺位分支**不是**静默排队无提示：dsh/zcode 的「已排队」Toast＋徽标即事实、一次到位；反过来给 dsh 补一个恒 false 的 Steer 只会把它的 Toast 从正确的「已排队」翻成不实的「已插入」。**定案：不给 dsh 补假实现，hermes 保留 Steerer 实现（位在场，与 codex/pi 同一提交路径，落点由 server 转排队的 Queued 帧纠正），steer 行为本身不改**；曾有的呈现侧代价（hermes 提交 Toast 先报「已插入」、随后被徽标纠正）已于 2026-10-14 另案修复：steerSubmitNotice 改帧驱动，Toast 只认 Steered/Queued 实际回执（TurnDelivery.deliveryAckNotice，commit 2106e9c），不再按位预判。理由同步落在 hermes.go `Steer` 注释与 commit message。
> 4. **守护测试五家对表已补（2026-10-14 P1 轮关闭）**：`host_capability_bits_test.go` Providers 表已入 hermes 真实构造器并补齐全位族格子（含 dsh/hermes text_generation 翻转、hermes compact 在场、image_input 协商条件位「未协商不虚报」）；hermes 列由调研逐格取证（`docs/research/2026-10-14-agent-adapter-gap.md`）+ 探针 live 复核（`docs/reviews/2026-10-14/dsh-hermes-capability-probe.md`）。

## 三、主矩阵 B：无位的机制与 runtime 描述符能力（GetHostInfo 之外）

这些能力**没有 GetHostInfo 位**，门控走事件形状 / 描述符 / agent 名单兜底——列进来是为了回答「为什么没有位也能用/不能用」。

| 能力 | codex | pi | zcode | dsh | hermes | 事实与出处 |
|---|---|---|---|---|---|---|
| usage（用量上报） | ✅ 轮次应答+usageCache | ✅ 0.87.1 事件驱动（per-message 终帧增量）+get_session_stats 兜底 | ✅ turn.completed payload.usage（实证窗口与投影一致）；`usage.delta` 已收编词表；`session/usage` RPC 未接（遗留⑥） | ✅ `usage_update`/`context_usage` → UsageUpdate 投影（used/size 缺一不投影） | ✅ usage_update→UsageUpdate 投影（hermes.go:679-687）＋prompt 应答 usage 落账（hermes.go:513-538、547-560） | codex：runtime_descriptors.go:92（capabilities_cache）；pi：pi_events.go:240-242 + pi_usage_stream_test.go（证据 pi-087-usage-probe.log）；zcode：zcode.go:15,1121-1145；dsh：dsh.go:1013-1030；hermes：hermes.go:679-687、513-538 |
| slash_commands（斜杠命令面板） | ✅ skills 即命令 | ✅ get_commands 真名映射 | ✅ 会话快照 slashCommands[] | ❌（无位无名单内资格） | ❌（无 `SlashCommands` 方法，grep 零匹配 → 面板按名单兜底不含 hermes，2026-10-14） | daemon `ListSlashCommands` 按 agent 路由（codex_commands.go、pi.go SlashCommandsFor）；Android `isSlashPanelAgent` 三源名单、无位（SlashCommandsLogic.kt:73-74）——将来第四家接入要么补实现要么引入位，名单是已登记的债 |
| subagents（子代理可见性） | ✅ 原生（item 透传+thread/read 子代理历史） | ✅ 原生（磁盘 subagent-artifacts） | ✅ 已接（轮次终局 session/subagents 拉取+存根；真实委派轮 live 受凭据阻塞=遗留⑧；描述符仍保守取 SubagentNone） | ❌ | ❌（2026-10-14 探针已核：ACP 协议无 subagents 事件变体，上游永久缺失，非适配债——dsh-hermes-capability-probe.md 判定表） | codex_subagents.go / codex_threadread.go；pi_subagents.go；zcode_subagents.go + server/zcode_subagent_sync.go；描述符 runtime_descriptors.go:99/125/152/171 |
| approval（审批请求） | ✅ 原生 | ❌（未取证到发射，ApprovalNone） | ❌（未取证到发射；permission.requested/userInput.requested 双轨留观=遗留⑤；2026-09-30 复验另收编 v4/telemetry/event、computer-use/operation-event 遥测与 union 记录流镜像为 PASSTHROUGH，见 knowledge_zcode.go） | ✅ 原生（2026-09-30 live 实证：仅 sandbox 升级路径发射——基座默认 workspace-write+ask，普通命令自动放行不弹卡；工作目录外写文件→沙箱拒绝→escalation→session/request_permission，allow/reject 双路实帧齐：请求 {sessionId,toolCall:{toolCallId},options:[allow-once/reject-once]}+数字 id，应答 outcome:{outcome:"selected",optionId} 双向被受理，证据 dshlive_permwrite.md） | ✅ 原生（session/request_permission 反向请求 → 手机审批卡：hermes.go:428-440 分派、862 handleRequestPermission，2026-10-14 核） | 描述符 runtime_descriptors.go:98/124/144/171-177；无位，走 TurnServerMessage_ApprovalRequest 事件面 |
| plan_review（计划卡投影） | ✅ review:<id> 位（发起）+ turn/plan/updated 步骤卡投影；`thread/plan/updated` 0.159.2 复核定案仍为幻影（codexplanprobe 两轮一致），daemon 已无死分支，回归测试钉死（codex_zcode_unknown_wiring_test.go） | ❌ | ❌ | 🔭 已接 boundary 版（0.2.0-rc.2 复证：适配器仍只发 6 种 sessionUpdate，无 plan 变体；boundary 投影继续兜底，遗留②收窄为等实发） | ⚠️ plan 卡已投影（hermes.go:668-678，CodexEvent STATUS PlanSteps「hermes · 执行计划」）；review 发起面 ❌（无 StartReview，无 review 位） | codex_events.go:260；dsh_plan.go:1-30（形状全部来自随包 ACP SDK schema）；stale-audit S1 |
| memory_injection（语义记忆四层） | ✅ Hybrid（memory.native+compaction） | ✅ Hybrid（同左） | ❌ Daemon 侧承担 | ❌ Daemon 侧承担 | ❓ 未见实现，需核（hermes 非 runtime.Participant：memoryModelOf 对未申报 Provider 回零值 turn_memory.go:242-248；调研 §一 memory 行标 Daemon，需核） | runtime/memory.go:10（MemoryKind 四层，规格 §6.3）；描述符 runtime_descriptors.go:100/126/153/173；共享后端统一交付 runtime_descriptors.go:317-353 |
| native session 收编（适配器，非「导入」） | ✅ codex_session.go | ✅ pi_session.go（P1） | ✅ zcode_session.go | ✅ dsh_session.go（P2） | ❌ 不在原生收编面（pi_session.go:44 注释明列 hermes/openai-compat；无 NativeSessionFactory 实现） | HYBRID 四家 NativeSessionFactory 全收编（runtime_descriptors.go:330-341 注释）；跨 daemon「导入」（AgentSessionImporter）dsh 明示不做（dsh.go:15），全仓无实现——该接口目前只存在于注释提及，**不是代码事实** |

### B 区附：runtime 描述符逐家形状速查（runtime.Capability，不做 UI 门控）

| 维度 | codex | pi | zcode | dsh |
|---|---|---|---|---|
| SessionModel | Hybrid | Hybrid | Hybrid | Hybrid |
| ResumeModel | Native | Reattach | Reattach | Reattach |
| ApprovalModel | Native | None | None | Native |
| SubagentModel | Native | Native | None（保守） | None |
| MemoryModel | Hybrid | Hybrid | Daemon | Daemon |
| CacheModel | Native | Daemon | Daemon | Daemon |
| nativeCaps | thread.resume、approval.request/resume、subagent.spawn/events/transcript、memory.native、memory.compaction、cache.native | thread.resume、subagent.spawn/events/transcript、memory.native、memory.compaction | thread.resume | thread.resume、approval.request/resume |

出处：`daemon/internal/agent/runtime_descriptors.go:95-184`（codexShape/piShape/zcodeShape/dshShape，每条注明证据行号）。

> hermes 未实现 `Descriptor`（runtime_descriptors.go 无 hermes 形状）→ 本速查表仍为四家；hermes 的 runtime 契约面（SessionModel/SubagentModel/MemoryModel 等）❓ 未见实现，需核。

## 四、能力位 → Android UI 接线对照表

| 位 | Android 门控函数 | 生效入口 | 出处（基线 a5f3f3e） |
|---|---|---|---|
| steer:<id> | `agentCapableOf("steer", …)` → `steerSubmitNotice` | 提交瞬间仅无位报「已排队」；有位不预判，Toast 按 Steered/Queued 回执帧裁定（「已插入运行中任务」/「已排队，当前轮结束后执行」） | ChatScreen.kt:2125、3659；TurnDelivery.kt `deliveryAckNotice` |
| queue_clear:pi | `queueClearVisible`（三重门控之一） | 撤回排队按钮（pi 会话+有排队+位在） | ChatScreen.kt:1114-1122、2767 |
| compact:<id> | `compactActionOn` | 更多面板「压缩上下文」+ /压缩 伪命令可见性 | ChatScreen.kt:1156-1158；SlashCommandsLogic.kt:124 |
| fork:<id> | `forkActionOn` | 更多面板「分叉会话」 | ChatScreen.kt:1160-1162 |
| rollback:<id> | `rollbackActionOn` | 更多面板「回滚…」 | ChatScreen.kt:1164-1166、3933 |
| review:<id> | `builtinSlashCommandsFor` 过滤 | /计划 伪命令可见性（现仅 codex） | SlashCommandsLogic.kt:125；ChatScreen.kt:1552,2342 |
| export_html:<id> | `HtmlExportGate`（`agentCapableOf("export_html", …)`） | HTML 存档导出按钮（缺位置灰降级） | ChatScreen.kt:5575-5596、5675 |
| image_input:<id> | （基线）`attachmentEntryHiddenFor` 硬编码 dsh；**本轮切换中**：`!agentCapableOf("image_input", …)` | 附件/图片入口显示 | 基线：ChatScreen.kt:1168-1178、3519-3522（注释自带切换预案）；切换由本轮批次 1~3 落地 |
| agent_settings:<id> | `capabilitiesList.contains("agent_settings:$runningAgent")` | 设置页运行 agent 的偏好入口 | SettingsScreen.kt:2275 |
| agent_native_settings:<id> | `caps.contains("agent_native_settings:$agentId")` | Agent 子页「原生设置」分组 | SettingsScreen.kt:3632-3664 |
| text_generation:<id> | `"text_generation:$it" in capabilities`（无位时回落三源名单） | 提示词优化 agent 白名单 | SettingsScreen.kt:3866 |
| collab:<id> | capabilities 过滤 `collab:` 前缀 | 主页多 agent 协同入口/回显 | HomeScreen.kt:1187-1194；CollabLogic.kt:24-28 |
| agent:<id>（含 pi 版本格式） | `resolveAgentAvailability`（前缀匹配新旧两格式） | 主页 agent 网格 + Onboarding 教程页点亮 | OnboardingAgentCapabilityTest.kt 文件头 |
| runtime.v1 | `runtimeControlCapable` | 「运行时」入口 | RuntimeStateLogic.kt（同口径大小写/空白不敏感） |

## 五、遗留决策项清单（受阻/留观区）

每条：现状 → 阻塞原因 → 解锁条件 → 建议归属轮次。

1. **dsh 探针 LLM 依赖 3 项（已关闭：2026-09-30 live 验证轮，key 就绪）**。update 全集 live 帧：0.2.0-rc.2 实测变体分布 agent_message_chunk / agent_thought_chunk / usage_update / tool_call / tool_call_update（块级粒度复确认：assistant 文本 1 帧到齐；usage_update={used:int,size:int} 整数，与 dshUsageUpdateFromUsedSize(*int64) 一致；update 体新增只读字段 messageId，不影响解析）；permission 双路：echo 提示不弹卡（基座 dsh-base cordis.patch.yml 默认 DSH_PERMISSION_MODE??workspace-write + approval ask，无文件副作用命令直接放行）——permwrite 专项实证真实审批回路：工作目录外写文件→沙箱 ACL 拒绝→模型 escalation→session/request_permission（数字 id），allow 路命令真实执行（文件落盘）、reject 路命令不得执行（文件不在、tool_call_update status=failed、模型不绕行）；cancel：stopReason=cancelled 实帧（cancel 后 6ms）。解析层逐字段一致、生产零改动；探针侧修一处反向请求应答 id 回显 bug（字符串 id → 原样回显数字 id）+ 新增 permwrite 专项（dshpermwrite.go）。证据：%TEMP%\dshlive_update-granularity.md / dshlive_permwrite.md / dshlive_cancel.md。无残余待办。
2. **dsh 0.1.7（next 通道）重探 与 image 翻真复验（已关闭：2026-09-30 两轮定案）**。0.1.7 预告的 plan_update 变体/权限模式/Auto review/archive 经 0.2.0-rc.2 实证均未进 ACP 面（archive 在 dsh-api-session-controller 桌面 API，Auto review/权限预设为 cordis 桌面面）——ACP 侧无适配债；image 翻真 live 识图复验完成：本机路由 promptCapabilities.image=false 实帧 + 带图 prompt 准入层拒绝（-32602 "inline image prompts were not advertised by this connection"），定案「等上游」（上游按所选模型 inputModalities 动态广播，翻真时 SupportsImageInput() 动态放行、daemon 图片块构造自动生效，无需改码）。证据：%TEMP%\dshlive_proto020.md。无残余待办。另：resume 上下文保持 live 复测通过（跨进程 kill→list→resume 后追问引用旧内容，重放 0 帧，证据 dshlive_resume.md）。
3. **codex thread/turns/list 等只读方法**。现状：A 级实测评估完毕，维持留观（docs/verify/evidence/codex-turnslist-probe.log；gap-analysis 留观处置 2）。阻塞：无收益场景——thread/read 唯一消费方是子代理历史（短线程），分页无收益；thread/search 要开 experimentalApi 门禁（-32600），违反「不碰实验面」。解锁：「超长会话手机端打开卡顿」或「桌面全量搜索」需求实化。归属：需求实化后的 codex 会话管理轮。0.159.2 复核：search 仍 experimentalApi 门禁；thread/read includeTurns 虽被 schema 标 deprecated 但活体实测 paginated 线程仍全量回水，留观结论不变。
4. **pi 原生设置写通道**。现状：pi 只读呈现（NativeConfigOptions 实现里 Writable=false 口径）。阻塞：写会落**用户桌面全局** `~/.pi` 配置，触红线（native_config_options.go:5-7），是否破例需拍板。解锁：产品侧明确「手机改桌面全局配置」是否可接受 + 若可接受加显式确认 UI。归属：设置透出第二轮（拍板前的纯产品决策，不耗工程）。
5. **zcode permission.requested / userInput.requested 双轨**。现状：词表留观（gap-analysis「ZCode 0.16.9」节）；2026-09-30 复验探针 0 命中，四件套分类与词表维持 UNKNOWN（测试钉 zcode_noticekind_20260930_test.go）。阻塞：需带真实审批轮的 live 实证才能定投影形状。解锁：可复现审批的 zcode 轮次（依赖可用账户适配，同第 8 条凭据问题）。归属：zcode 审批增强轮。
6. **zcode session/usage 未接（P2）**。现状：用量已由 turn.completed payload.usage + usage.delta 事件覆盖（zcode.go:15,1121-1145），session/usage RPC 未接。阻塞：现有投影够用，收益未实化。解锁：出现口径缺口（如历史用量回填）再接。归属：P2 随取随用。
7. **zcode 原生设置透出无 NativeConfigOptions 实现**。现状：实现者仅 pi/dsh（host_capability_bits_test.go:122 钉死 codex/zcode 无位）。阻塞：zcode 设置面（permission.mode 专属分组）与「目录型」接口形状不匹配，硬套会伪造目录。解锁：明确 zcode 设置透出的 UI 呈现方案后单独实现。归属：原生设置透出第二轮。
8. **zcode subagents live 验证**。现状：链路已接（轮次终局拉取+存根），live 测试就位 `go test -tags pa_live -run TestZcodeLiveSubagents ./internal/agent/`（PA_LIVE_ZCODE=1，zcode_subagents_live_test.go）。阻塞：本机凭据不可用（SSR gpt 401 / deepseek auth_failed，无 accounts 适配）。解锁：可用账户。归属：凭据就绪后的收尾轮（顺带补真实委派轮的 running/ended 实测帧）。
9. **codex `thread/plan/updated` 疑幻影方法（已关闭）**。已定案+已清理（0.156.1 轮删键删分支，2026-09-30 0.159.2 轮补回归钉），无待办。
10. **pi 0.99.1 复验（2026-09-30，piprobe 29 项全过）**。协议增量 6 命令 + 1 事件，全部留观或已收编词表；usage per-message 口径、clear_queue、followUp 线未断；get_state 新增 sessionName（只读已呈现）；set_follow_up_mode 实证落盘全局配置，红线维持只读。

## 六、守护与回归钉子索引

- **daemon**：`internal/server/host_capability_bits_test.go`（五家真实构造器 × GetHostInfo 位对表，2026-10-14 补 hermes 全格）；`internal/agent/capability_gate_test.go`（本轮新增：host.go 位族词汇表 ↔ 本文档第二节对表 + 四家逐位表态对表，位族增删或文档不同步即红）。
- **android**：`ThreadActionGateTest.kt`（compact/fork/rollback/steer/queue_clear 门控+agentCapableOf 口径，既有）；本轮新增 `AgentCapableOfBoundaryTest.kt`（agentCapableOf 边界：未知 agent/未知位/空集/大小写空白/冒号带空格不误命中）。
