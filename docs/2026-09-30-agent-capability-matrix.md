# 四家 Agent 能力位矩阵（2026-09-30 · 批次 4/4 产出）

> **基线**：HEAD `a5f3f3e`（feat(android,daemon): 深度体验优化 P1 四项+能力位 image_input+Compact 路由修复）。
> **本轮批次上下文**：四批次并行——批次 1~3 在改生产源码（image_input 位 Android 侧切换：硬编码 dsh → 能力位判定；native config 写后失效链补全等）；本批次只产出文档与测试，不碰生产代码。凡标注「（本轮切换中）」的格子，以批次 1~3 合入后的代码为准。
> **证据分级**（沿用 2026-09-25 gap-analysis 口径）：`(实证)`=探针/live 实测、`(代码)`=仓库内代码路径、`(bundle)`=本机二进制静态提取、`(留观)`=已评估暂不做。出处写「文件:行」的均指基线 `a5f3f3e` 时的位置。
> **口径纠偏**：daemon 侧生成的位格式是 `<位>:<agentId>`（如 `image_input:dsh`），**没有** `capability:` 统一前缀；此前沟通稿写「`capability:<agentId>` 位（host.go:151 一带）」，位置对（host.go:151 正是 image_input 位生成行）、格式表述不准，以本矩阵为准。

## 一、能力位怎么来的（生成机制，30 秒版）

daemon 在 `GetHostInfo.capabilities` 里对**每个已注册 provider** 做接口类型断言，逐位如实宣告（`daemon/internal/server/host.go:75-155`）：谁有真实现谁带位，**缺位 = 手机端入口隐藏，不摆假按钮**。同一循环生成宿主级静态位。手机端统一入口是纯函数 `agentCapableOf(bit, agentId, capabilities)`（`android/.../ui/SlashCommandsLogic.kt:86`，大小写/空白不敏感、agentId 空=拒绝、只认同 agent 的位）。

能力位之外还有第二套「能力声明」：runtime 描述符 `runtime.Capability`（`daemon/internal/runtime/descriptor.go:43-72`，逐家形状见 `daemon/internal/agent/runtime_descriptors.go`）。它不进 GetHostInfo、不做 UI 门控，是运行时契约面，本矩阵单列为 B 区。

## 二、主矩阵 A：GetHostInfo 能力位（host.go 生成的全集，13 位族 + 6 宿主级）

图例：✅已接（有位有实现有出处）｜✅*条件位（接口在场但生成还需运行时真值）｜❌不适用｜⛔受阻｜🔭留观。

| 能力位族（格式 `<位>:<agentId>`） | codex | pi | zcode | dsh | 判据（host.go 断言的接口/形状） | 出处 |
|---|---|---|---|---|---|---|
| `agent:<id>`（pi 追加版本 `agent:pi:v0.87.1`） | ✅ | ✅ | ✅ | ✅ | 已注册即宣告（echo/glm/deepseek 同理） | host.go:77,201-210（`agentCapability`）；版本格式回归钉 OnboardingAgentCapabilityTest.kt |
| `agent_settings:<id>`（设置页七项偏好，PrefsAware） | ✅ | ✅ | ✅ | ✅ | `agent.PrefsAware`（ApplyPrefs 全量快照覆盖），四家编译期钉死 | provider.go:93-105；host.go:78-80 |
| `agent_native_settings:<id>`（原生设置目录透出） | ❌ | ✅ | ❌ | ✅ | `agent.NativeConfigOptions`，实现者仅 pi/dsh | native_config_options.go:51（接口）；pi_config_options.go:28、dsh_config_options.go:36；host.go:84-86；zcode 缺实现=遗留⑦ |
| `text_generation:<id>`（提示词优化） | ✅ | ✅ | ✅ | ❌ | `agent.TextGenerator`（GenerateText），dsh 无已验证文本生成路径 | text_generator.go:17-104（三家 GenerateText）；host.go:87-89 |
| `collab:<id>`（跨 agent 协同入口） | ❌ | ✅* | ❌ | ✅* | `agent.CollabAware && CollabReady()`——**接口只有 pi/dsh 实现**；`*` = 还要接线真值：pi 看 `piapp.CollabEnvInjection()`（本机目录），dsh 看会话级 MCP 注入（可测控） | provider.go:56-60；pi.go:148、dsh.go:167；host.go:93-95；判定回归钉 collab_ready_test.go；codex/zcode 未实现 CollabAware（codex 子代理走独立机制，见 B 区 subagents 行） |
| `steer:<id>`（运行中真插话） | ✅ | ✅ | ❌ | ❌ | `agent.Steerer`：codex 原生 turn/steer、pi 原生插队；zcode 实测 session/send 轮中被 -32010 拒、dsh 无 | codex_steer.go:51、pi.go:1340；host.go:102-104；zcode 实测结论 host.go:97-99 注释 |
| `queue_clear:<id>`（撤回上游排队输入） | ❌ | ✅ | ❌ | ❌ | `agent.QueueClearer`，仅 pi 0.87.1 原生 clear_queue（子进程内存队列） | pi.go:1376-1389（接口+编译钉死+ClearQueue）；host.go:109-111 |
| `compact:<id>`（压缩上下文） | ✅ | ✅ | ✅ | ❌ | 三种原生形状任一：codex `CompactThread(string)` / pi `CompactThread(*store.Thread)` / zcode `CompactSession(string)`；dsh 无压缩实现 | codex_compact.go:123、pi.go:1468、zcode.go:735；host.go:121-129（三形状互斥都宣告同一位） |
| `fork:<id>`（分叉会话） | ✅ | ❌ | ❌ | ❌ | `ForkThread(codexThreadID, lastTurnID)`，路由只落 codexProvider | codex_threads.go:308；host.go:130-132 |
| `rollback:<id>`（回滚回合） | ✅ | ❌ | ❌ | ❌ | `RollbackThread(codexThreadID, numTurns)`（0.156.1 起 rollback→thread/revert 已迁移） | codex_threads.go:357；host.go:133-135；迁移记录 gap-analysis「Codex 0.156.1」节 |
| `review:<id>`（计划审查发起） | ✅ | ❌ | ❌ | ❌ | `StartReview(*store.Thread, focus)`（review/start） | codex_review.go:65；host.go:136-138 |
| `export_html:<id>`（HTML 存档导出） | ❌ | ✅ | ❌ | ❌ | `ExportHTML(*store.Thread)`，pi 专属 | pi.go:1503；host.go:139-141 |
| `image_input:<id>`（图片附件真正进入模型输入） | ✅ | ✅ | ✅ | ❌ | `agent.ImageInputter && SupportsImageInput()`：codex turn/start LocalImage、pi prompt/steer 携 base64、zcode session/send attachments 三家 Run 有真实图片路径；dsh `promptCapabilities.image` 实测 false，Run 门控如实报错 | image_input_capability.go:36-61（接口+四家编译钉死+查证结论）；host.go:150-152；daemon 侧位生成已被 43ee0cf/a5f3f3e 落地；Android 侧切换=本轮批次 1~3 落地中（见接线表） |
| **宿主级**（无 agentId 后缀）：`terminal` `fs` `git` `agent` `event_sequence` `runtime.v1` | — | — | — | — | 固定追加（`runtime.v1`=运行时控制面入口；`event_sequence`=事件断点续传） | host.go:154-159；Android 侧 runtime.v1 门控 RuntimeStateLogic.runtimeControlCapable |

> 守护：A 区全集与代码的一致性由两层测试钉住——`daemon/internal/server/host_capability_bits_test.go`（真实四家构造器跑 GetHostInfo 对表）与本轮新增 `daemon/internal/agent/capability_gate_test.go`（位族词汇表 ↔ 本文档对表，位增删即红）。

## 三、主矩阵 B：无位的机制与 runtime 描述符能力（GetHostInfo 之外）

这些能力**没有 GetHostInfo 位**，门控走事件形状 / 描述符 / agent 名单兜底——列进来是为了回答「为什么没有位也能用/不能用」。

| 能力 | codex | pi | zcode | dsh | 事实与出处 |
|---|---|---|---|---|---|
| usage（用量上报） | ✅ 轮次应答+usageCache | ✅ 0.87.1 事件驱动（per-message 终帧增量）+get_session_stats 兜底 | ✅ turn.completed payload.usage（实证窗口与投影一致）；`usage.delta` 已收编词表；`session/usage` RPC 未接（遗留⑥） | ✅ `usage_update`/`context_usage` → UsageUpdate 投影（used/size 缺一不投影） | codex：runtime_descriptors.go:92（capabilities_cache）；pi：pi_events.go:240-242 + pi_usage_stream_test.go（证据 pi-087-usage-probe.log）；zcode：zcode.go:15,1121-1145；dsh：dsh.go:1013-1030 |
| slash_commands（斜杠命令面板） | ✅ skills 即命令 | ✅ get_commands 真名映射 | ✅ 会话快照 slashCommands[] | ❌（无位无名单内资格） | daemon `ListSlashCommands` 按 agent 路由（codex_commands.go、pi.go SlashCommandsFor）；Android `isSlashPanelAgent` 三源名单、无位（SlashCommandsLogic.kt:73-74）——将来第四家接入要么补实现要么引入位，名单是已登记的债 |
| subagents（子代理可见性） | ✅ 原生（item 透传+thread/read 子代理历史） | ✅ 原生（磁盘 subagent-artifacts） | ✅ 已接（轮次终局 session/subagents 拉取+存根；真实委派轮 live 受凭据阻塞=遗留⑧；描述符仍保守取 SubagentNone） | ❌ | codex_subagents.go / codex_threadread.go；pi_subagents.go；zcode_subagents.go + server/zcode_subagent_sync.go；描述符 runtime_descriptors.go:99/125/152/171 |
| approval（审批请求） | ✅ 原生 | ❌（未取证到发射，ApprovalNone） | ❌（同上；permission.requested/userInput.requested 双轨留观=遗留⑤） | ✅ 原生（全家唯一实测发射点 dsh.go:1071） | 描述符 runtime_descriptors.go:98/124/144/171-177；无位，走 TurnServerMessage_ApprovalRequest 事件面 |
| plan_review（计划卡投影） | ✅ review:<id> 位（发起）+ turn/plan/updated 步骤卡投影（`thread/plan/updated` 词表键疑幻影死分支=遗留⑨） | ❌ | ❌ | 🔭 已接 boundary 版（0.1.5-rc.3 bundle 实发不含计划变体；投影是给 0.1.7-next 的兜底=遗留②） | codex_events.go:260；dsh_plan.go:1-30（形状全部来自随包 ACP SDK schema）；stale-audit S1 |
| memory_injection（语义记忆四层） | ✅ Hybrid（memory.native+compaction） | ✅ Hybrid（同左） | ❌ Daemon 侧承担 | ❌ Daemon 侧承担 | runtime/memory.go:10（MemoryKind 四层，规格 §6.3）；描述符 runtime_descriptors.go:100/126/153/173；共享后端统一交付 runtime_descriptors.go:317-353 |
| native session 收编（适配器，非「导入」） | ✅ codex_session.go | ✅ pi_session.go（P1） | ✅ zcode_session.go | ✅ dsh_session.go（P2） | HYBRID 四家 NativeSessionFactory 全收编（runtime_descriptors.go:330-341 注释）；跨 daemon「导入」（AgentSessionImporter）dsh 明示不做（dsh.go:15），全仓无实现——该接口目前只存在于注释提及，**不是代码事实** |

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

## 四、能力位 → Android UI 接线对照表

| 位 | Android 门控函数 | 生效入口 | 出处（基线 a5f3f3e） |
|---|---|---|---|
| steer:<id> | `agentCapableOf("steer", …)` → `steerSubmitNotice` | 运行中发送回执两态文案（「已插入运行中任务」/「已排队」） | ChatScreen.kt:2238、1181-1185 |
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

1. **dsh 探针 4/8 项受阻**。现状：P0 探针 8 项仅 4 项有实测（gap-analysis「DSH 0.1.5-rc.3」节）。阻塞：缺 `DEEPSEEK_API_KEY`。解锁：用户提供 key 后补跑探针。归属：dsh 重探轮（与第 2 条合并执行）。
2. **dsh 0.1.7（next 通道）重探**。现状：本机装 rc.3；0.1.7 在 next 且明示 breaking。阻塞：未进 latest，重探成本高且形状可能再变。解锁：进 latest 后统一重探三件事——plan_update 变体实发形状（dsh_plan.go boundary 版接得住没有）、image 能力是否翻真（翻真需 live 复验识图后翻 SupportsImageInput 动态版，见 image_input_capability.go:25-28）、权限模式/Auto review。归属：dsh 重探轮。
3. **codex thread/turns/list 等只读方法**。现状：A 级实测评估完毕，维持留观（docs/verify/evidence/codex-turnslist-probe.log；gap-analysis 留观处置 2）。阻塞：无收益场景——thread/read 唯一消费方是子代理历史（短线程），分页无收益；thread/search 要开 experimentalApi 门禁（-32600），违反「不碰实验面」。解锁：「超长会话手机端打开卡顿」或「桌面全量搜索」需求实化。归属：需求实化后的 codex 会话管理轮。
4. **pi 原生设置写通道**。现状：pi 只读呈现（NativeConfigOptions 实现里 Writable=false 口径）。阻塞：写会落**用户桌面全局** `~/.pi` 配置，触红线（native_config_options.go:5-7），是否破例需拍板。解锁：产品侧明确「手机改桌面全局配置」是否可接受 + 若可接受加显式确认 UI。归属：设置透出第二轮（拍板前的纯产品决策，不耗工程）。
5. **zcode permission.requested / userInput.requested 双轨**。现状：词表留观（gap-analysis「ZCode 0.16.9」节）。阻塞：需带真实审批轮的 live 实证才能定投影形状。解锁：可复现审批的 zcode 轮次（依赖可用账户适配，同第 8 条凭据问题）。归属：zcode 审批增强轮。
6. **zcode session/usage 未接（P2）**。现状：用量已由 turn.completed payload.usage + usage.delta 事件覆盖（zcode.go:15,1121-1145），session/usage RPC 未接。阻塞：现有投影够用，收益未实化。解锁：出现口径缺口（如历史用量回填）再接。归属：P2 随取随用。
7. **zcode 原生设置透出无 NativeConfigOptions 实现**。现状：实现者仅 pi/dsh（host_capability_bits_test.go:122 钉死 codex/zcode 无位）。阻塞：zcode 设置面（permission.mode 专属分组）与「目录型」接口形状不匹配，硬套会伪造目录。解锁：明确 zcode 设置透出的 UI 呈现方案后单独实现。归属：原生设置透出第二轮。
8. **zcode subagents live 验证**。现状：链路已接（轮次终局拉取+存根），live 测试就位 `go test -tags pa_live -run TestZcodeLiveSubagents ./internal/agent/`（PA_LIVE_ZCODE=1，zcode_subagents_live_test.go）。阻塞：本机凭据不可用（SSR gpt 401 / deepseek auth_failed，无 accounts 适配）。解锁：可用账户。归属：凭据就绪后的收尾轮（顺带补真实委派轮的 running/ended 实测帧）。
9. **codex `thread/plan/updated` 疑幻影方法**。现状：词表键+两处分类分支挂在 thread/plan/updated，但 vendored schema 只有 `turn/plan/updated`、真实投影也走它（codex_events.go:260；stale-audit S1 判定）。阻塞：无 live 帧证据定案（若 live 从未出现该 method，键与分支是死代码）。解锁：下一轮 codex 探针观察真实 method 名。归属：下一轮 codex 探针；定案后清理（删键/删分支或改名，属生产代码改动，不在本批次范围）。

## 六、守护与回归钉子索引

- **daemon**：`internal/server/host_capability_bits_test.go`（四家真实构造器 × GetHostInfo 位对表，既有）；`internal/agent/capability_gate_test.go`（本轮新增：host.go 位族词汇表 ↔ 本文档第二节对表 + 四家逐位表态对表，位族增删或文档不同步即红）。
- **android**：`ThreadActionGateTest.kt`（compact/fork/rollback/steer/queue_clear 门控+agentCapableOf 口径，既有）；本轮新增 `AgentCapableOfBoundaryTest.kt`（agentCapableOf 边界：未知 agent/未知位/空集/大小写空白/冒号带空格不误命中）。
