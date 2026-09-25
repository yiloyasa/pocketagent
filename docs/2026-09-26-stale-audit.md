# 失效物清点审计（2026-09-26）

> **性质**：丁类文档（只审计不动手）。基线 HEAD `dcdbcae`（main，含黑客主题等最新提交），审计时工作树干净。
> **方法**：全仓 grep 交叉引用 + proto/schema/探针证据比对；每条结论附文件:行号与判定依据。「可能没用」一律不入正条，进存疑节。
> **范围**：daemon（Go）与 android（Kotlin）生产代码的失效物；本轮未改任何生产代码。

---

## 一、建议删除（零风险：全仓零引用或纯本地残留，删后无行为变化）

### daemon 侧（9 条）

| # | 对象 | 位置 | 依据 |
|---|---|---|---|
| D1 | `BlockFromToolCall` | `daemon/internal/agent/tools.go:181` | 全仓（含测试）零调用。工具卡片实际由 `MapToolcallEnd` 构造 |
| D2 | `MapToolcallStart` | `daemon/internal/agent/pi_events.go:78` | 全仓零调用；其头注释自述 P1 取舍「STARTED 卡等 toolcall_end 再发」——设计决策在注释里，函数本体已无存在意义（删时把注释挪到 `MapToolcallEnd` 上方即可） |
| D3 | `CodexProvider.TurnSummary` | `daemon/internal/agent/codex.go:1781` | 全仓零调用。turn/start 的 summary 逻辑已在 `codex.go:1328-1372`（`desktopSummary`）内联重实现，此导出读回器是旧路径残骸，注释仍以现役口吻描述（兼注释漂移） |
| D4 | `AgentDescriptor.HasAll` | `daemon/internal/runtime/descriptor.go:217` | 全仓零调用（含测试） |
| D5 | `MergeReports` | `daemon/internal/runtime/agent_runtime.go:257` | 全仓零调用；`contract_suite.go` 未用其合并逻辑 |
| D6 | `MemoryLimit.Truncated` | `daemon/internal/runtime/memory.go:152` | 全仓零调用 |
| D7 | `FileStore.SetDefaultWorkspace` | `daemon/internal/store/store.go:539` | 全仓零调用（读侧 `DefaultWorkspace` 活跃，仅写侧死） |
| D8 | `FileStore.RemoveTombstone` | `daemon/internal/store/store.go:809` | 全仓零调用。注释称「重新导入 / 误删恢复时用」——规划中的调用方从未落地；真要做恢复功能时再从 git 恢复 |
| D9 | `piapp.WriteGuardExtension` | `daemon/internal/piapp/guard.go:83` | 全仓零调用。`pi.go:131` 用 `os.WriteFile` + `RenderGuardExtension` 内联了同一逻辑（`guardPath` 赋值链活跃），此导出包装被绕开 |

### android 侧（5 条）

| # | 对象 | 位置 | 依据 |
|---|---|---|---|
| D10 | `agoText`（整个候选函数） | `android/app/src/main/java/dev/pocketagent/app/ui/component/ThreadSnapshot.kt:12` | 全仓（含测试）零调用。「离线快照相对时间」实际展示走 `messagesAt` 时间戳另格式化（ChatScreen.kt:1764 一带）。注释声称的用途无消费方 |
| D11 | `runtimeOverview()` + `RuntimeOverview` data class 生产链 | `RuntimeStateLogic.kt:116-143`、唯一调用点 `RuntimeSection.kt:93` | 唯一调用 `val overview = runtimeOverview(...)` 的结果**从未被读取**（RuntimeSection 后文 1300+ 行无 `overview` 引用）；`stateLabel/degraded/version/budgetRevision/collabRevision/dataDir` 六字段全 app 零消费，仅 `RuntimeStateLogicTest.kt:124/136/146` 自测。删函数+类+对应测试段 |
| D12 | `SubAgentThreadMeta.updatedAtMs` | `SubAgentsState.kt:58`（赋值 `:220`） | write-only 字段：构造后零读取；子代理排序用 `createdAtMs`（`:92`） |
| D13 | `SettingStateLine._onRetry` 参数 + 6 处实参 | `SettingsScreen.kt:1329`；调用点 `:1701/:1751/:1819/:1867/:1915/:1949` | 函数体内从不调用 `_onRetry`，6 个调用点传的 `onRetrySettings` 实参全部为死传（`onRetrySettings` 本身在 `:1519/:1527/:1575` 另有真实消费，保留） |
| D14 | 未使用 import 62 处（24 文件） | 附录 A 全清单 | 逐名比对 import 与文件体（已排除 `getValue/setValue` 委托假阳性）；重灾区：ChatScreen.kt 11 处、FilesScreen.kt 14 处——两轮大改版（浮层收敛/真机挑刺）的遗留 |

### 资产（2 条）

| # | 对象 | 位置 | 依据 |
|---|---|---|---|
| D15 | 5 个未引用图标：`ic_lucide_bell / ic_lucide_lock / ic_lucide_log_out / ic_lucide_paperclip / ic_lucide_terminal` | `android/app/src/main/res/drawable/` | java+res 全域零引用（逐名 grep，58 个 drawable 中仅此 5 个落空）；`tools/make_icons.py` 不生成这 5 个名字，删除后不会复活 |
| D16 | `daemon/watchdog.bat`（git 跟踪）+ `daemon/watchdog_hidden.vbs`（git 跟踪） | `daemon/` 根 | ① `paguard/main.go:1-8` 注释明言「取代旧的 watchdog.bat」；② AUDIT.md:158（2026-09-25）：计划任务 action 已改**直跑 paguard.exe 绕开 wscript**，vbs 已彻底脱离守护链且其存在正是弹窗根因；③ `watchdog.bat:9` 仍硬编码 `CODEX_BIN=G:\Claude Code\codex.cmd`——README:204 明言该注入行为 2026-09-23 起已判定有害（覆盖 daemon 定位链）。误跑即注入过期路径。**删两文件须同步改写 README:204（见 R1）** |

### 本地未跟踪残留（1 条，不涉 git）

| # | 对象 | 依据 |
|---|---|---|
| D17 | `daemon/pocketagentd-old.exe`（28MB，09-25 备份件）；`daemon/{piprobe,dshprobe,zcprobe,codexturnsprobe}.exe`（均可由 `daemon/cmd/*` 同名源码重build）；根目录 `build/`（424MB 未跟踪构建产物） | `.gitignore` 已覆盖全部 `*.exe`/`build/`；备份件与 probe exe 属可重建物，磁盘紧张时清理。注意 `daemon/paguard.exe` 是计划任务现役二进制、`daemon/daemon.log`/`watchdog.log` 是现役日志，**勿删** |

---

## 二、建议改写（6 条：对象有用但内容/说法已过期）

| # | 对象 | 位置 | 现状与建议 |
|---|---|---|---|
| R1 | README:204 看门狗链描述 | `README.md:204` | 仍写「计划任务 → wscript → WMI 拉起 paguard」。AUDIT.md:158 实锤 2026-09-25 起 action 已直跑 paguard.exe。随 D16 一并改写 |
| R2 | README:28 / README:87 协议计数 | `README.md:28,87` | 写「7 服务 / 79 RPC，8 proto / 1859 行」+ 逐服务旧分解。proto/v1 实测（`grep -c '  rpc '`）：**7 服务 / 84 RPC / 2146 行**（thread 35、git 10、fs 9、runtime 13、model 9、host 5、terminal 3）。README:43 已是 84，此两处没跟上 |
| R3 | AUDIT.md:61 RPC 计数 | `AUDIT.md:61` | 「现 7 服务 / 79 RPC（2026-09-24 校正）」→ 应为 84（同 R2 实测口径） |
| R4 | `CodexProvider.Prefs` 注释漂移 | `daemon/internal/agent/codex.go:1600` | 注释称「供 GetCodexSettings 回显与 server 层合并」，实际生产零调用（server 非测试代码无 `.Prefs()`），仅 `model_lock_test.go`/`prefs_broadcast_test.go` 用。改注释为「测试读回验证用」或下沉 |
| R5 | `dist/windows/PocketAgent桌面端.zip`（git 跟踪发布物，8.3MB） | `dist/windows/`，打包于 09-23 00:30 | 落后 HEAD 三个修复轮：缺 09-25 弹窗封死链（15ac3a8/6077295）与 09-26 Job Object 退出零残留（3ab41a6）——恰是用户红线域。建议重新打包替换 |
| R6 | proto `HostEvent` 三个 oneof 分支：`workspace_changed` / `model_ready` / `update_available` | `proto/v1/host.proto`；daemon 仅 `event_sequence.go:316-320` 序号分类 switch 触及 | **双端全死**：daemon 无任何构造点（唯一生产者只发 `TurnFinished`，`event_sequence.go:332`），app 零消费。属契约层死面——删（甲类改动，两份 proto 同步）或加注释声明预留语义。因动契约，单列此档由执行代理裁决 |

---

## 三、保留（有理由的，勿动）

| # | 对象 | 理由（附证据） |
|---|---|---|
| K1 | `knowledge_zcode.go` 的 `session.updated` / `session.modelUpdated` / `settings.updated` 三键 | 用户点名的旧版兼容：文件头 0.16.9 实证校准注释完好（`:4-9,24`），且 `zcode.go:1832` 的处理分支仍活跃——旧 bundle（≤0.16.5）连入时分类/人话不能缺。**这正是「确认注释仍在」的答案：在** |
| K2 | `knowledge_codex.go` 全部 43 键 / `knowledge_pi.go` 全部 31 键 | codex 键逐名对 vendored schema（`codex-schema/codex_app_server_protocol.v2.schemas.json`）全命中，且 `codex-0156-probe.log` 第一节「命中 75/75、消失通知：无」；pi 键对 `pi-087-usage-probe.log` 实测命中（turn_start 5、message_end 8、entry_appended/summarization_retry_finished/extension_error 各 1）。唯一例外见存疑 S1 |
| K3 | `knowledge_dsh.go` 空精确表 `dshEventSummaries` | 文件头注释自述设计：通知带开放状态后缀走前缀表，精确表留待上游出现无后缀方法 |
| K4 | echo provider 永远注册 | `provider.go:192` 注释明言「没有任何 key 时也能把整条链路跑通」，契约测试基建的一部分 |
| K5 | ThreadSnapshot 旧名兼容入口 `saveThreads/loadThreads/loadMessages/clear` | `ThreadSnapshot.kt:145,148,231,323` 注释明言兼容用途，调用方实测在（`ThreadSnapshotTest.kt:82-86`、`CacheSnapshotTest.kt:49+`、设置页缓存管理、ChatScreen.kt:1758 一带） |
| K6 | 测试专用导出：`Budget.Check`（resource.go）、`Backend.FlushResourceLedger`（resource_disk.go:99）、`RecoveryReport.IsOK`（recovery_report.go）、`dshapp.InvalidateVersionCache`（client.go:175，注释自署「测试用」）、`EchoProvider.EchoRuntimeBackend`（echo_runtime.go:100，注释自署供契约测试） | 生产零调用但测试在用，属可辩护的测试出口；如追求洁癖可改 `_test` 文件或降 internal，非本轮必需 |
| K7 | `daemon/cmd/` 下 48 个 probe/verify 小工具源码 | 探针证据链的生产端（docs/verify/evidence 的 log 均出自它们），审计基建，非残留 |
| K8 | `runtime/contract_suite.go`（非测试文件但仅测试消费） | 文件头自述用途与用法示例，L1 契约用例的可执行规格，刻意非 `_test` |
| K9 | proto 中 daemon 供数、app 未读的字段块：`CodexGoalDetail.{tokens_used,token_budget,time_used_seconds}`（thread.go:859-865 在填）、`ResourceMetric.{lease_id,reserved_bytes,soft_limit_bytes,spill_policy}`（runtime_policy.go:113 在填）、`Thread.guardian_denied_*`（thread.go:65 在填）、`CodexEvent.{http_status,retry_after_ms,rationale}`（codex_events.go:99,109 在填） | **不是失效物**：生产端在填，只是 app 侧未消费（遥测/展示差距）。列入本报告仅存档防误删；删了会砍掉未来 UI 的数据源 |

---

## 四、存疑（证据不足，不动手，先探针）

| # | 事项 | 已知证据 | 缺口 |
|---|---|---|---|
| S1 | `thread/plan/updated` 可能是不存在的方法 | vendored schema 只有 `turn/plan/updated`（`grep '"thread/plan/updated"'` 0 命中，`turn/plan/updated` 命中）；实际投影走 `codex_events.go:260 case "turn/plan/updated"`；`dsh_plan.go:6` 注释亦称 codex 投影是 turn/plan/updated；`codex-0156-probe.log` 全文无 thread/plan。而 `codex.go:545,559` 的分类分支与 `knowledge_codex.go:42` 的词表键却挂在 `thread/plan/updated` 上（该键 09-17 d07ee88 引入，未见证据文档） | 若 live 从未出现过 `thread/plan/updated`，则词表键+两处分支是死分支 → 应删键/删分支或改名 turn/plan/updated。建议下一轮 codex 探针观察真实 method 名后定夺 |
| S2 | `HostEvent` 三分支（R6）是否为规范预留 | 现状双端全死（证据见 R6） | 若设计规范有「预留事件位」条款则改保留；需查规范原文后再定删/留 |
| S3 | README:43「91 份 XML / 712 用例」现值 | 写于 2026-09-25 控制面轮后 | 需真机/模拟器跑一轮 instrumented test 才能复核，本轮无法离线验证 |

---

## 五、文档漂移补遗（改写档之外的小额漂移，列出备办）

- `README.md:40`「知识库 70 条」：本轮为历史叙事存档，词表现状约 100+ 键（codex 43 / pi 31 / zcode 27 / dsh 前缀 7）；该句属「当轮完成记录」非现状声明，可不改，但下一轮改 README 时顺手换成现状口径更好。
- `README.md:38` daemon 运行态叙述（PID/热部署时间戳）属快照性内容，随 R2/R1 改写时一并刷新至当前部署。

---

## 附录 A：未使用 import 全清单（62 处 / 24 文件）

```
MainActivity.kt:15 layout.padding; :17 material3.Text
AppDrawer.kt:30 layout.widthIn
AttachmentPreview.kt:12 layout.widthIn; :20 graphics.Color
ChatScreen.kt:64 layout.windowInsetsPadding; :103 draw.drawBehind; :114 text.SpanStyle; :116 buildAnnotatedString; :121 withStyle; :122 style.TextAlign; :171 v1.ApprovalDecision; :173 v1.ContentBlock; :197 v1.TextBlock; :200 v1.ToolCallUpdate; :201 v1.TurnClientMessage; :203 v1.TurnInput; :204 v1.TurnServerMessage; :205 v1.Usage
CollabLogic.kt:4 v1.CollabRole
CollabRoleEditorScreen.kt:3 android.widget.Toast
CollabScreen.kt:9 layout.Box
FilesScreen.kt:4 foundation.Image; :13 layout.height; :14 navigationBarsPadding; :22 rememberScrollState; :25 verticalScroll; :30 IconButton; :31 ListItem; :32 MaterialTheme; :36 TopAppBar; :37 TopAppBarDefaults; :50 layout.ContentScale; :69 anim.pressable; :79 v1.GetHostInfoRequest; :82 v1.ReadFileRequest
FolderPicker.kt:4 layout.Arrangement; :9 layout.fillMaxSize
GitScreen.kt:4 foundation.clickable; :5 foundation.Image; :39 layout.ContentScale
HomeScreen.kt:14 layout.aspectRatio; :18 navigationBarsPadding
MarkdownText.kt:17 graphics.Color; :30 dev.pocketagent.app.R
PcFilePreview.kt:8 java.util.zip.ZipEntry
RuntimeSection.kt:9 layout.Arrangement
TaskCenterScreen.kt:11 foundation.Image; :56 layout.ContentScale; :72 component.humanSize
ThreadsScreen.kt:98 v1.ListModelsRequest; :106 v1.ModelInfo
TurnSession.kt:16 kotlinx.coroutines.delay
PaAnim.kt:17 layout.Spacer; :19 layout.height; :29 runtime.mutableStateOf
ContextMenu.kt:38 graphics.Color
RecentThreadsDrawer.kt:48 AgentConnection; :56 Dispatchers; :57 withContext; :58 v1.ListThreadsRequest
SplitButton.kt:30 graphics.Color
```

## 附录 B：proto 字段 app 侧零引用全表（逐消息，构建器命名已排除假阳性）

大块（正文已述）：ResourceMetric 4/10、HostEvent 3/6、Thread 3/22、CodexGoalDetail 3/5、CodexEvent 3/19、SetCodexSettingsRequest 3/14 中 `disable_paste_burst/native_config_option_id/native_config_value`（daemon 写侧/设置链在用与否未逐条核，量小不展开）。

零散（按纪律忽略，仅存档）：Error.{retryable,http_status}; SearchMatch.{line_number,line_text}; HostInfo.{auth_required,default_shell}; HostStoreHealth.{has_backup,quarantine_path}; CodexUsage.{rate_limit_reached_type,fetched_at_unix_ms}; TestModelResponse.{reply,latency_ms}; RuntimeStateResponse.{installation_id,sampled_at_unix_ms}; ListThreadsRequest.{search_term,sort_ascending}; ListDirectoryRequest.include_hidden; WriteFileRequest.expected_modified_at_ms; WriteFileResponse.new_modified_at_ms; FileDiff.has_changes; SearchContentRequest.max_results; GitDiffRequest.context_lines; GitDiffResponse.changed_files; GitCommitResponse.commit_hash; CreateBranchRequest.from_branch; AuthorizeResponse.expires_at_unix; PingRequest.client_time_unix_ms; PingResponse.server_time_unix_ms; ModelReady.model_ids; UpdateAvailable.notes_url; DevServerList.scanned_at_unix_ms; HostRecoveryReport.devices; HostTurnJournalHealth.pending_count; CodexSettings.allowed_web_search_modes; ListAgentConfigOptionsResponse/RuntimePolicyResponse/ResourceUsageResponse/AgentRuntimeResponse/ListThreadMemoriesResponse.sampled_at_unix_ms 及 policy_path; McpServerStatus.resource_template_count; ListModelsResponse.state_error; ModelInfo.supports_images; UnknownEventSummary.safety_class; OpenTerminal.env_json; Resumed.attached_ms; ListTerminalsResponse.sessions; TerminalSessionInfo.opened_at_unix_ms; ForkThreadRequest.last_turn_id; TurnClientMessage.supplement; Steered.codex_turn_id; QueueFlushed.remaining; TurnCompleted.diagnostic_id。

---

## 结论摘要

- **建议删除 17 条**（daemon 死代码 9 + android 死代码 5 + 资产 2 + 本地残留 1 中可归并者）
- **建议改写 6 条**（README×2 + AUDIT×1 + 注释漂移×1 + 发布物重打包×1 + 契约死面×1）
- **保留 9 条**（均有成文理由，含用户点名的 zcode 旧版兼容三键——注释确认在、分支确认活）
- **存疑 3 条**（最重要的 S1：codex `thread/plan/updated` 疑为指向不存在方法的键+死分支）
- 全部结论可由本文所附 grep 证据复核；未改任何生产代码，未 commit。
