# 页面契约：12+2 页面迁移消费表（2026-10-14）

> 用途：`docs/design/2026-10-14-flutter-rewrite-plan.md` §C1 —— 供 **B3 逐页迁移子代理**直接消费，不再重读 Kotlin 源码。
> 范围：`android/app/src/main/java/dev/pocketagent/app/`（`ui/` 为主 + 根目录导航装配 `MainActivity.kt` / 连接单例 `AgentConnection.kt`）。
> 行数口径：**物理行数**（python 逐行计数，含空行/注释）。注：`Get-Content | Measure-Object -Line` 给出的是**非空行**，比物理行少 ~4%（如 `ChatScreen.kt` 6165 vs 5924），本文统一用物理行数。
> RPC 口径：全部经 `AgentConnection.<service>()` 拿 stub，7 个服务 93 RPC（见 AGENTS.md §1）。文中方法名 = proto RPC 名。

---

## 0. 导航总图（先看这个再看单页）

`MainActivity.kt`（394 行）里的 `RootNav()` 是**唯一**的 NavHost。**关键契约：home 不是 5 条路由，而是 1 条路由 + 一个 `tab: Int` 内部状态**——Flutter 侧要么复刻这个"单路由多 Tab"，要么拆成 5 条路由并同步改 4 处 `onGoTab` 语义。

| route | 目标 | 备注 |
|---|---|---|
| `onboarding` | `Onboarding()` | `startDestination`（`OnboardingPrefs.isDone()==false` 时） |
| `home` | `when(tab)`：0=Home / 1=Threads / 2=Files / 3=Git / **4=TaskCenter** | `startDestination`（已看过教程时）。`tab` 是 `RootNav` 里的 `remember { mutableIntStateOf(0) }` |
| `chat/{threadId}` | `ChatScreen(threadId, …)` | 全屏，`launchSingleTop` |
| `terminal` | `TerminalScreen(onBack)` | 全屏 |
| `browser?url={url}` | `BrowserScreen(initialUrl, onBack)` | url 默认空串 |
| `settings?sub={sub}` | `SettingsScreen(openSub, …)` | sub ∈ `connection`/`downloads`/`agent`/`capabilities`/`console`；旧值 `userdata`→`downloads`；未知→一级页（`normalizeSettingsSub`，SettingsScreen.kt:449） |
| `collab` | `CollabScreen(...)` | |
| `collabRoles` | `CollabRolesScreen(...)` | |
| `collabRoleEditor/{roleId}` | `CollabRoleEditorScreen(roleId, …)` | roleId 空 = 新建 |

三条全局导航约定（Flutter 必须等价复刻）：

1. **返回层级**：`backToHome = { tab = 0; openHomeDrawer += 1 }` —— 二级页按返回 → 回主页**并自动展开侧边栏**（`openHomeDrawer` 自增信号，HomeScreen.kt:350 `LaunchedEffect` 消费 → `drawerCtrl.open()`）。
2. **通知深链**：`PaNotifications.pendingChatNavigation`（StateFlow）→ `nav.navigate("chat/$tid")`，冷启/热启同入口（MainActivity.kt:181-188）。
3. **转场**：spring 化（`PaMotion.nav()`），进 1/20 屏宽滑入+淡入，出淡出（MainActivity.kt:213-216）。Splash 是**覆盖层**不是串行闸：RootNav 在其下并行组合，`onDone` 只负责摘覆盖层（MainActivity.kt:112-127）。

---

## 1. 页面总清单（12+2）

### 1.1 Home（工作台）— `home` / tab 0

- **文件**：`ui/HomeScreen.kt` **2453 行 / 117.2KB**；辅助 `ui/component/HomeWorkbenchLogic.kt` 132 行、`ui/attention/AttentionCenterState.kt` 192 行 + `ui/attention/AttentionItem.kt`。
- **RPC 依赖**
  - `HostService.getHostInfo`（`connectWithFallback()` 失败后兜底，HomeScreen.kt:370-371）
  - `ThreadService.listThreads`（最近会话 + 注意力三分，:376）
  - `ThreadService.createThread`（新建任务面板，:643）
  - 能力命中时才打：`RuntimeService.listUnknownEvents`（经 `RuntimeControlState.refreshUnknownEvents()`，:451）；`ThreadService.listCollabRoles`（经 `CollabRolesState.load()`，:414）
- **状态字段**（全部页面内 `remember { mutableStateOf }`，非 ViewModel）
  - `host: HostInfo?`、`recent: List<PThread>`、`loadError`、`lastLoadedAtMs`、`firstLoadDone`、`reloadKey`
  - 订阅共享态：`connState` / `connTransport` / `connectionEpoch` / `hostCapabilities` / `disabledAgents` / `mdnsDiscovery.hosts`
  - 派生：`attention = partitionHomeAttention(recent, disabledAgents)`（待审批/运行中/其余）、`runningAgentIds`、`barSessions`（`buildPaRunningSessions`，5s 轮询 `TurnSession.runningThreadIds()`）、`attentionItems`（未知事件+审批+恢复报告）、`showAttentionCenter`
  - 浮层：`showManage` / `guideAgent` / `showNewThread` / `pairHost` / `drawerCtrl`（`overlayActive` 驱动推背模糊）
- **能力位门控**
  - `agent:<id>` 或 `agent:<id>:<ver>` → agent 磁贴点亮（`resolveAgentAvailability` :176）；灰显 → `AgentGuideSheet`（`resolveAgentInstalledVersion` :199 取版本）
  - 未知新 agent id 由 `buildAgents` :256 的 `extra` 分支自动追加（**无静态名单**）
  - `collab:<id>` → `multiAgent` → 协同行 + 抽屉「多 Agent 协同」入口才挂（:409-410）
  - `runtime.v1` → 消息中心才拉未知事件（:449-451）
  - `AgentConnection.disabledAgents`（本地偏好）→ 隐藏 agent 磁贴/协同行/最近会话（`filterAgents` :2423）
- **导航**
  - 入：`home` tab0；二级页返回（`openDrawerSignal`）
  - 出：`chat/{id}`（注意力卡/最近会话/mini 条）、`tab=1`（点 agent 磁贴 / "全部会话"）、`terminal`、`settings`（顶栏齿轮 + `PaHostChip` 点按）、`tab=4`（任务中心）、`browser`、`collab`、`collabRoles`、`onboarding`（抽屉"重看教程"）
  - 抽屉：`AppDrawer`（Home 侧主导航唯一挂载点）

### 1.2 Chat（会话）— `chat/{threadId}`

- **文件**：`ui/ChatScreen.kt` **6165 行 / 323.7KB**；拆分层 `ui/chat/`：`ChatViewModel` 830、`ChatState` 564、`ChatTimeline` 1273、`ChatComposer` 1187、`ChatSheets` 461、`ChatQuickPanels` 328、`ChatApproval` 208、`ChatImmersive` 261、`ChatRoute` 44。
  轮次状态机 `ui/TurnSession.kt` **1800 行 / 94.6KB**（页面级单例，**离开页面流不断**）+ `TurnDelivery.kt` 78。
  支撑：`SlashCommandsLogic` 336、`HistoryMessageMapper` 194、`HistoryRoundsLogic` 51、`ui/history/*` 552、`ui/model/*` 1168、`ui/prompt/*` 992 + `PromptOptimizeLogic` 254、`ui/voice/*` ~1000、`MarkdownText` 729、`CodeSyntax` 405、`AttachmentPreview` 272、`ContextUsageCard` 490、`SubAgentsState` ~360 + `SubAgentCard` ~630。
- **RPC 依赖**（最重的一页，**4 个服务全用**）
  - **ThreadService**：`startTurn`（**双向流**，TurnSession.kt:820 `stub.startTurn(observer)`）、`getThread`、`getThreadHistory`（首屏 + 更早 + 恢复回放）、`getThreadEventsSince`（增量恢复）、`cancelTurn`、`submitApproval`、`createThread`、`searchThreadMessages`（:3241）、`exportThreadHtml`（:5902）、`listThreads`（RecentThreadsDrawer）、`forkThread` / `rollbackThread` / `compactThread` / `startReview` / `clearQueuedInputs` / `setThreadGoal` / `approveGuardianDeniedAction`（全在 `ChatViewModel` gateway :66-160）、`optimizePrompt`（提示词优化，`PromptOptimizeRepository` :124）
  - **HostService**：`getHostInfo`（ChatViewModel prefetchStatic）
  - **ModelService**：`getCodexSettings` / `setCodexSettings`；**ModelServiceSlow**：`listModels`（选择器 + 10s 巡检 `startModelInspection` :729）
  - **FileSystem**：`writeFile`（附件上传到 `<workspace>\.pocketagent-uploads\`，:3105）
  - **GitService**：`status` / `diff` / `commit`（会话内嵌 git 卡，:5194 / :5350 / :5459）
  - 流：`hostEvents`（AgentConnection 的 `HostService.WatchEvents` 全局流）由 TurnSession 消费
- **状态字段**
  - **ChatViewModel.state : `StateFlow<ChatState>`**（页面唯一单点 collect，:2278）：`offlineSnapshot(At)`、`roundsLoaded`、`truncated`、`loadingEarlier`、`identity`（历史身份索引）、`oldestMessageId`、`threadTitle/Status/Workspace/AgentId`、`loadError`、`slashCommands`、`models`、`listOffset`
  - **ChatViewModel.session : `StateFlow<TurnSessionUiState>`**（TurnSession 只读投影，:2295，`bindSession/unbindSession` 配对在 `DisposableEffect`）：`running/streaming/recovering/turnTerminal/pendingApproval/approvalError/errorMessage/recoveryError/runningModel/runningEffort/queuedCount/historyLoaded/turnUsage/totalUsage/liveContextWindow/compactionInProgress/lastCompactedAt/guardianOverridden/itemsEmpty/itemCount/lastQueuedIndex/userTurnCount/latestPlan`
  - 页面本地：`input`（直挂 `st.draft`）、`inputRevision`、`promptOptState/promptPreview/promptStyle`、`voiceState`、`verifyingSettled`（10s 看门狗）、`hostCaps`、`connState`、`modelSelectionState`
  - 滚动/未读：`shouldAutoScroll` / `accumulateUnreadBlocks` / `distanceFromBottomPx`（:1455-1494）
- **能力位门控**（统一入口 `agentCapableOf(bit, agentId, caps)`，SlashCommandsLogic.kt:86，匹配 `"<bit>:<agentId>"`，**agentId 为空一律 false**）
  | 位 | 控制的 UI |
  |---|---|
  | `compact:<id>` | 「压缩上下文」入口 + `/压缩` 伪命令（ChatScreen:2087 / SlashCommandsLogic:125） |
  | `fork:<id>` | 「分叉」（:2091） |
  | `rollback:<id>` | 「回滚」（:2095） |
  | `review:<id>` | `/计划` 伪命令（SlashCommandsLogic:126） |
  | `export_html:<id>` | HTML 导出按钮 `htmlExportGate`（:5632，UNSUPPORTED → 隐藏） |
  | `image_input:<id>` | **附件入口隐藏** `attachmentEntryHiddenFor`（:2108，取反） |
  | `steer:<id>` | 提交瞬间仅「无位=已排队」诚实预判；**有位不预判**（:3659 `steerSubmitNotice` 有位返回 null），Toast 改由 Steered/Queued 回执帧驱动（:2357-2363 消费 `TurnSession.deliveryAckNotice`，与时间线徽标同源；hermes 带位但 Steer 恒 false → 落「已排队」） |
  | `queue_clear:pi` + agent==pi + queuedCount>0 | 「撤回排队」按钮（`queueClearVisible` :2048） |
  | `runtime.v1` | 未知事件相关卡片（经 RuntimeControlState） |
  - 另：`TurnSession.daemonSupportsEventSequence(caps)`（`event.sequence` 类位，TurnSession.kt:1203）→ 增量恢复 vs 全量回放 `resumePlan`
- **导航**
  - 入：`chat/{id}` ← Home/Threads/TaskCenter/RecentThreadsDrawer/Collab 进度卡/通知深链
  - 出：`onBack`→`popBackStack`；`onOpenThread`→`chat/{id}`(singleTop)；`onGoTab(n)` / `onOpenTaskCenter`→**先 `popBackStack("home")` 再 `tab=n`**；`terminal`；`settings`；`settings?sub=capabilities`（技能预算警告卡）；`browser?url=`
  - 侧栏：**不挂 AppDrawer**，挂 `component/RecentThreadsDrawer`（:4333）——最近会话 + 底部全局入口

### 1.3 Threads（全部会话）— `home` / tab 1

- **文件**：`ui/ThreadsScreen.kt` **2836 行 / 148.8KB**；辅助 `ThreadsRowLogic` 194、`ThreadOrganize` 96、`ThreadDeletionCoordinator` 102、`component/ThreadPins` 54、`component/ThreadSnapshot` 393。
- **RPC 依赖**（ThreadService 最全的一处）
  - 列表四源：`listThreads`（本地导入态）、`listCodexThreads`（电脑未导入 codex）、`listAgentThreads`（pi/zcode 未导入，按 agent）、`listThreadSections`（分组）
  - 写操作：`createThread`、`renameThread`、`deleteThread`、`archiveThread`、`forkThread`、`moveThreadToSection`、`importCodexThread`、`importAgentThread`
  - 分组 CRUD：`createThreadSection` / `renameThreadSection` / `deleteThreadSection`
  - `HostService.getHostInfo`（:用于 workspace 候选）
  - 离线兜底：`ThreadSnapshot`（本地快照，`offline` 标志决定横幅配色）
- **状态字段**
  - 数据：`threads`、`host`、`sections`、`pcThreads`、`agentPcThreads`、`pinned`
  - 三维筛选（交集）：`filter`（全部/归档/未分组/某分组，单值互斥）、`agentFilter`（初值 `AgentConnection.selectedAgent`）、`statusFilter`（等待处理/执行中/已完成/未完成）；`query` 纯本地搜索（`matchesThreadSearch`）
  - 视图：`renamingId`/`renameField`（单一真相在页面，组件不留副本）、`organizeTarget`、`sectionSheet`/`sectionField`/`sectionBusy`、`openKey`（左滑单行互斥）、`listState`、`undo`、`scrollChipsToEnd`
  - 状态位：`loadError`、`offline`、`retrying`、`pcError`、`loading`、`importingId`、`pcActionConfirm`、`pendingPcActions`
  - 订阅：`hostCapabilities`、`connectionEpoch`（触发 `refresh()`）、`connState/connTransport/connAddress`（顶栏主机胶囊）
- **能力位门控**：`agent:<id>` → `agentFilterOptions(capabilities)`（:2828）动态生成 agent 筛选条（**硬编码三家已删**）；`disabledAgents` 参与过滤。
- **导航**
  - 入：`home` tab1（Home agent 磁贴 / "全部会话" / Chat 抽屉"全部会话"）
  - 出：`chat/{id}`（点卡）、`terminal`、`settings`、`tab=4`、`browser`；返回走 `threadsBackTarget` **8 级分级回退**（抽屉→重命名→新建→组织→分组面板→搜索→状态筛选→筛选→主页，:283）

### 1.4 Files（文件）— `home` / tab 2

- **文件**：`ui/FilesScreen.kt` **958 行 / 46KB**；辅助 `FilesListLogic` 104、`FolderPicker` 249、`PcFilePreview` 1434（预览主体，主要被 Browser 消费）、`component/FileKind` 220、`component/PathNav` 88。
- **RPC 依赖**：`FileSystemService.listRoots` / `listDirectory` / `createDirectory` / `deleteEntry` / `moveEntry` / `writeFile`（重命名=move）；预览走 `PcFilePreview`→读文件；"下载到手机" 走 `PaDownloader.readFile` 流。
  **注意：当前无文件名搜索 RPC**（`SearchContent` 是内容搜索），`searchQuery` 只过滤当前目录（FilesScreen.kt:132-137）——Flutter 侧别许诺递归搜索。
- **状态字段**：`path`、`showRoots`、`entries`、`loading`、`error`、`searchOpen/searchQuery`、`showCreate`、`askDownload`、`openKey`、`listState`、`undo`、`renaming/renameText`、`pendingDelete`、`crumbs`（`pathBreadcrumbs`）、`drawerCtrl`。
- **能力位门控**：无（本页不读 capabilities）。
- **导航**：入 = `home` tab2；出 = `browser?url=<编码后的 pc 路径>`（可预览文件交内置浏览器）、`terminal`、`settings`、`tab=4`；返回 `onBackToHome`（展开抽屉）。

### 1.5 Git — `home` / tab 3

- **文件**：`ui/GitScreen.kt` **1160 行 / 52.5KB**；辅助 `GitStatusLogic` 90。
- **RPC 依赖**：`GitService.status` / `listBranches` / `log` / `diff` / `stage` / `unstage` / `commit` / `checkoutBranch` / `createBranch` / `discardChanges`；`HostService.getHostInfo`（workspaces + recentWorkspaces 合并成 `candidates`，:93 / :304）。
- **状态字段**：`repo`、`status: GitStatus?`、`diff`、`log`、`branches`、`tab`（变更/历史/分支 3 段）、`notRepo`、`busy`、`error`、`showRepoPicker`/`repoInput`/`candidates`、`showCommit`/`commitMsg`、`showCreateBranch`/`newBranch`、`pendingDiscard`/`pendingCheckout`、`diffTitle`、`openKey`、`drawerCtrl`；订阅 `connectionEpoch`（:300 触发 refresh）。
- **能力位门控**：无。
- **导航**：入 = `home` tab3（或带 `workspacePath`）；出 = `terminal`、`settings`、`tab=4`、`browser`；返回 **6 层分级回退** `backOneLevel()`（仓库选择→提交→建分支→diff→丢弃→切分支→抽屉→主页，:177）。

### 1.6 Terminal（终端）— `terminal`

- **文件**：`ui/TerminalScreen.kt` **758 行 / 33.3KB**；`component/TerminalBlocks` 578（终端样式块，也给 Chat 用）。
- **RPC 依赖**
  - `TerminalService.attach` —— **双向流**（`terminalServiceAsync().attach(responseObserver)` :683，`requestObserver` :610）
  - `TerminalService.closeTerminal`（:742）
  - `HostService.getHostInfo`（:117，探 workspace）
  - `FileSystemService.listDirectory`（:128/:147/:169，工作区候选探测，非 git 仓库时换目录）
- **状态字段**：`bridge.state` / `bridge.error`（WebView+xterm.js 桥，`bindWebView`/`onPageReady`/`sendInput`/`sendResize`/`start`/`close`，:572-741）、`recents`（`AgentConnection.recentWorkspaces`）、`sessionId`、`requestObserver`、`cols/rows`。
- **能力位门控**：无（但整页隐含"有 TerminalService 的 daemon"——旧 daemon 会 UNIMPLEMENTED）。
- **导航**：入 = 各页抽屉「终端」（`composable("terminal")`，**全屏、不在 home tab 体系内**）；出 = 仅 `onBack` → `popBackStack`。**无抽屉、无侧栏**。

### 1.7 TaskCenter（任务中心）— `home` / tab 4

- **文件**：`ui/TaskCenterScreen.kt` **928 行 / 43.3KB**；状态源 `ui/component/TaskCenter.kt` 398 + `component/TaskRetryGate` 84。
- **RPC 依赖**：**本页零直连 RPC**。
  - 数据 = `TaskCenter.tasks`（本地 SharedPreferences 持久化，杀进程不丢；`init` 在 MainActivity:70）
  - 重试消息 → `TurnSession.resyncFromDaemon(threadId)`（内部 `getThreadHistory` + `getThreadEventsSince`）
  - 重试下载 → `PaDownloader.downloadFromUrl` / `downloadFromPc`（后者 `fileDownloadService.readFile` 流）
  - 打开文件 → `PaDownloader.openLocalFile` / `onOpenBrowser`
- **状态字段**：`all: List<Task>`、`kindTab`（MESSAGE/DOWNLOAD 两段）、`connState`→`hostOffline`、`retryGates`（每行一门闩）、`retryingIds`、`missingFileIds`、`drawerCtrl`。
- **能力位门控**：无（只有 `hostOffline` 分支：离线时消息类不显示"正在执行"、不给需要在线的重试入口，:285）。
- **导航**：入 = `tab=4`（Home 抽屉 / 各页 `onOpenTaskCenter` / Chat 内 `popBackStack("home")+tab=4`）；出 = `chat/{id}`（点消息任务）、`browser?url=`（下载项）、`settings?sub=downloads`（卡片本体 `onOpenUserData`）、`tab=n`；返回 3 级回退（抽屉→页签→主页，:226）。

### 1.8 Settings（设置）— `settings?sub={sub}`

- **文件**：`ui/SettingsScreen.kt` **4695 行 / 241KB**（单文件含 5 个二级页）；
  内嵌二级页：`ConnectionPage` :3371、`DownloadsPage` :3671、`AgentPage` :2145、`CapabilitiesPage` :2023、`console` → `RuntimeSection()` :4554。
  支撑：`ui/RuntimeSection.kt` 1548、`ui/RuntimeStateLogic.kt` 1092（`RuntimeControlState` 在 :657）、`ui/ControlPlaneLogic.kt` 600、`ControlDirectSwitchLogic` 123、`DesktopEchoLogic` 128、`SettingsOverviewLogic` 87、`CacheCleanupLogic` 80、`ui/model/*` 1168（`ModelSelectionRepository`/`Reducer`/`State`/`ModelEffortCard`/`ModelOptionRow`）、`ui/WorkflowBCapability` 156、`EditorialApproval` 410（`ApprovalPreviewSheet` :3126）。
- **RPC 依赖**
  - `HostService.getHostInfo`（:读 host + capabilities + `updateHostCapabilities` :667）
  - **ModelService**：`getCodexSettings` / `setCodexSettings`
  - **ModelServiceSlow**：`listModels`、`listCodexCapabilities`（技能/hooks/插件清单 :708）、`listCodexUsage`（额度/限流）、`listMcpServers`、`listAgentConfigOptions`、`getCodexSettings` / `setCodexSettings`（原生配置区）
  - **console 页（RuntimeSection）**：`RuntimeService.getRuntimeState` / `getRuntimePolicy` / `setRuntimePolicy` / `getCollabPolicy` / `setCollabPolicy` / `getAgentRuntime` / `listUnknownEvents` / `getUnknownEvent` / `acknowledgeUnknownEvent` / `executeUnknownEventAction` / `listThreadMemories` / `clearThreadMemory`；`ControlPlaneLogic.getControlPlane`；`ModelService.setCodexSettings`（回写）
  - 连接页内部 = `SaveConnectionOps` → `AgentConnection` 的 `probeHost`/`connectWithFallback`/`authorizeWithPairCodeDetailed`（本质 `HostService.getHostInfo` + 令牌链）
- **状态字段**（`SettingsScreenBody` :536-599 是全页状态堆，20+ 个）
  - `addr`/`token`、`settings: CodexSettings?`、`writeMutex`+`writeVersion`（写串行化）
  - `mcpList/mcpLoading/mcpError`、`codexUsage/usageLoading`、`capabilities/capabilitiesLoading/capabilitiesError`（**加载中 ≠ 真空 ≠ 读失败**，三态必须分）
  - `host`、`connectionSaving`、`settingsError/settingsFeedback`、`pairError`（**独立于 connError**，T-32）、`applyToExistingThreads`、`showModelPicker`/`modelChoices`/`modelError`、`showApprovalPreview`
  - `settingStates: Map<String,String>`（daemon 逐项回真实状态 `setting_status`，:598）——**别用统一"已保存"糊弄**
  - 视图态：`subPage`（`null`=一级页）、`subPageReturn`、`showDrawer`…；订阅 `connError`、`connectionEpoch`
- **能力位门控**
  - `runtime.v1` → 「系统与调试 → 控制台」整块入口才渲染（:1647-1649）；`RuntimeSection` 自身也再判一次（`runtimeControlCapable`，RuntimeSection.kt:85）
  - `agent_native_settings:<agentId>` → `NativeConfigOptionsSection` 才出现（:4394）
  - `agent_settings:<runningAgent>` → 原生设置区（:2763）
  - `capabilities` 页 = `listCodexCapabilities` 结果按 `CodexCapability.kind`（skill/hook/plugin）分组（`WorkflowBCapability.kt`）
  - `host.capabilitiesList` → optimizer 可见性（:1556）
- **导航**
  - 入：所有页顶栏齿轮 / `PaHostChip` / 抽屉「设置」；深链 `settings?sub=connection|downloads|capabilities|console|agent`
  - 出：`browser?url=`（下载页打开已下载网页/文本）；`onBack`→popBackStack；页内 `subPage` **不走路由**（是页面内状态，Flutter 侧若拆路由需同步改 `settingsBackTargetOf` :493）

### 1.9 Browser（内置浏览器）— `browser?url={url}`

- **文件**：`ui/BrowserScreen.kt` **1310 行 / 68.3KB**；`component/WebNavPolicy` 110（信任判定）、`component/WebDownloadBridge` 62（JS 桥）、`ui/PcFilePreview` 1434（本地内容渲染主体）。
- **RPC 依赖**
  - `HostService.listDevServers`（电脑 dev server 面板，:315）
  - `FileSystemService.readFile`（**流式**，`fileDownloadService().readFile` :217，用于 `pc:D:\...` 远端文件下载/渲染）
  - 其余全在手机本地 WebView
- **状态字段**：`input`、`progress`、`pageTitle`、`canGoBack/canGoForward`、`loadingPc`、`webRef`、`desktopView`、`lastLocal`（rawHtml+label 缓存）、`defaultUa`、`pcPath`/`localUri`（决定顶栏是"下载"还是"分享"）、`bridgeRegistered`、`showServers`/`scanning`/`servers`/`scanHint`/`scanError`。
- **能力位门控**：无 capabilities；**安全门控**=`WebNavPolicy.trustedBridgeSource(url)` —— 只对 App 预览 / 本地文件 / 私网 daemon 挂 `WebDownloadBridge`，公网站点必须卸载（:142-150）。**这是安全红线，Flutter 必须等价**。
- **导航**：入 = `browser?url=` ← Home 抽屉、Files 预览、Chat 链接、TaskCenter 下载、Settings 下载页、Git；出 = 仅 `onBack`。

### 1.10 Collab（多 Agent 协同编排台）— `collab`

- **文件**：`ui/CollabScreen.kt` **1118 行 / 52.5KB**；`ui/CollabLogic` 285、`ui/CollabState` 87；会话层 `ui/collab/`：`CollabSessionReducer` 315、`CollabSessionRepository` 284、`CollabSessionState` 148、`CollabCopy` 151（合计 948）。
- **RPC 依赖**（全部挂 **ThreadService**，薄封装在 `AgentConnection` :1328-1403）
  - 派发：`dispatchCollab`（`CollabSessionRepository.dispatch` :165）
  - 会话：`getCollabSession` / `listCollabSessions` / `cancelCollabSession` / `cancelCollabMember` / `retryCollabMember`
  - 流：`watchCollabSession(sessionId, afterSequence)`（**服务端流**，回放 + 跟随，缓冲 64 DROP_OLDEST）
  - 进入页面：`CollabSessionRepository.openLatest()`（快照先行 + 事件流跟随，:129）；离开 `onPageHidden()` 停流保快照
  - `CollabRolesState.load()` → `ThreadService.listCollabRoles`（:124）
- **状态字段**
  - 共享：`CollabSessionRepository.state : StateFlow<CollabSessionState>`（**单一状态树**，reducer 投影，:135）
  - `CollabState`（object 级跨页保留）：`selected`、`roleBinding`、`dispatching`、`lastInstruction`
  - 页面本地：`cmd`、`roleFor`（角色弹层）
  - 派生：`participants`/`readyIds`/`unavailableNote`、`decision = collabDispatchDecision(...)`（灰按钮必带原因）
- **能力位门控**：`collab:<id>`（`resolveCollabAgentIds`，HomeScreen.kt:2232）决定参与者全集；`disabledAgents` 参与剪除（`retainSelection` :127，选中集与可派发集同步）；无位时文案 `CollabCopy.EMPTY_NO_CAPABILITY`（CollabLogic.kt:263）。`readyIds` 为 READY 才可勾。
- **导航**：入 = Home 抽屉「多 Agent 协同」（仅 `multiAgent` 时挂）+ 消息中心；出 = `collabRoles`（右上齿轮 + 弹层"管理协同角色"）、`chat/{threadId}`（成员进度卡"查看会话"）、`onBack`。

### 1.11 CollabRoles（协同角色列表）— `collabRoles`

- **文件**：`ui/CollabRolesScreen.kt` **317 行 / 13.2KB**；状态 `ui/CollabRolesState.kt` 109（object 级）、`ui/CollabRolesLogic` 57。
- **RPC 依赖**：`ThreadService.listCollabRoles`（`CollabRolesState.load()` :54，进页 `LaunchedEffect` :68）、`deleteCollabRole`（:256）。
- **状态字段**（`CollabRolesState` 是**跨页共享 object**，`mutableStateOf` 而非 Flow）：`roles`、`loadState`（LOADING/…）、`loadError`；页面本地 `showDelete`。
- **能力位门控**：无（本页可直达，但入口在 Home/Collab 处已受 `collab:<id>` 门控）。
- **导航**：入 = `collabRoles` ← Home（协同行）/ CollabScreen；出 = `collabRoleEditor/{roleId}`（`onOpenEditor`，空 id = 新建）、`onBack`。

### 1.12 CollabRoleEditor（角色编辑）— `collabRoleEditor/{roleId}`

- **文件**：`ui/CollabRoleEditorScreen.kt` **343 行 / 16.2KB**。
- **RPC 依赖**：`CollabRolesState.save(role)` → `ThreadService.setCollabRole`（`doSave` :125）；读取来自 `CollabRolesState.roles`（内存态，进页前已 load）。
- **状态字段**：`idText`/`nameText`/`agentId`/`description`/`instructions`/`modelText`/`effort`/`workspaceText`、`saving`、`saveError`（**展示 daemon 还原原因**）、`idError`（`validateCollabRoleId`，新建才校验）、`agents`（下拉）。
- **能力位门控**：`AgentConnection.hostCapabilities` → `resolveCollabAgentIds` 决定 agent 下拉与默认值（:90-91，`collabFirst`）。
- **导航**：入 = `collabRoleEditor/{roleId}` ← CollabRolesScreen；出 = 保存成功 `onBack()`、返回键 `onBack()`。**无其他出口**。

### 1.13 Onboarding（新手教程）— `onboarding`

- **文件**：`ui/Onboarding.kt` **539 行 / 23.8KB**。
- **RPC 依赖**：**无主动 RPC**；只订阅 `AgentConnection.connState`（:283）决定步骤完成态 `done`（connect 步骤 = `connected`）。
- **状态字段**：`OnboardingPrefs`（独立 SharedPreferences `pa_onboarding`，`isDone`/`markDone`，:60）、`connState`、`pages = onboardPages(connected)`、`pager`（HorizontalPager）。步骤定义 `onboardSteps(connected)` :100（纯函数，单测钉死）。
- **能力位门控**：无。
- **导航**：入 = 首启 `startDestination`（`!isDone`）或 Home 抽屉"重看教程"；出 = `finishOnboarding`（**现查 `previousBackStackEntry`**：首启 `navigate("home") + popUpTo(onboarding, inclusive)`；重看 `popBackStack()`，MainActivity.kt:226-233）、`settings?sub=connection`（排障直达 `onOpenSettings`）、`onGoHome`（收尾页）。

### 1.14 Splash

- **文件**：`ui/Splash.kt` **206 行 / 8.5KB**。
- **RPC 依赖**：**无**（RootNav 在其下并行组合，RPC 预取同时跑）。
- **状态字段**：`markAlpha/markScale/glow/sweep`（4 个 `Animatable`）、`reduceMotion`、系统栏颜色（`SideEffect` 写 window，:88）。
- **能力位门控**：无。
- **导航**：**不在 NavHost 内**——是 `MainActivity.setContent` 里的覆盖层，`onDone` → `showSplash=false`（MainActivity.kt:112-127）。覆盖层吃点击（`clickable(indication=null)`）防止淡出中误触穿屏。

---

## 2. 共享组件清单（`ui/component/`）

只列**被 ≥2 个页面复用**的；括号内为引用到它的页面/文件（含 ui/ 下非 component 文件）。

| 组件 | 职责 | 使用方 |
|---|---|---|
| `PaSheet` (322) | 统一模态底部弹层（顶角 22、玻璃背景、拖拽关闭） | **16 处**：Chat/ChatSheets/Collab/Files/FolderPicker/Git/Home/ModelEffortCard/PromptOptimizePreview/RuntimeSection/Settings/SubAgentCard/Terminal/Threads/UnknownEventDetailSheet/ContextUsageCard |
| `PaScreenScaffold` (267) | 页面骨架：标题 + 导航槽(leading/trailing) + 主机状态槽 + 内容槽 | Home/Threads/TaskCenter/Settings/Collab/CollabRoles/CollabRoleEditor/Chat/ChatTimeline（Files/Git/Browser/Terminal 自绘骨架） |
| `PaHaptics` (120) | 语义触觉统一入口（confirm/success/warning，跟随系统触感开关） | 11 处：Chat 全家/Collab/Files/Git/Home/ModelEffortCard |
| `PaStatusBadge` (142) | 状态徽章（与 `EditorialStatus` 同源词表） | ChatComposer/Chat/Collab/CollabSessionReducer/Home/Settings/TaskCenter/Threads |
| `PaApprovalDialog` (612) | 审批对话框（破坏性换布局、主按钮永不红） | ChatApproval/Chat/ChatTimeline/EditorialApproval/Files/Git/RuntimeSection/Settings/UnknownEventDetailSheet |
| `PaHostChip` (205) | 主机状态胶囊（连接态+传输标签+上次更新，点按动作各页自定） | Home/Threads/Settings/ChatComposer/Chat/EditorialStatus |
| `PaDownloader` (508) | 下载到手机（URL/电脑文件双源，`fileDownloadService.readFile` 流 + TaskCenter 集成） | AgentConnection/Browser/Chat/Files/Settings/TaskCenter/MainActivity |
| `OfflineStore` (399) | 离线分区缓存（按分区注册/清扫） | AttentionCenterState/Browser/CacheCleanup/Home/RuntimeStateLogic/Settings/MainActivity |
| `TaskCenter` (398) | **任务/未读中心**（见 §3） | AppDrawer/Browser/EditorialStatus/ErrorCopy/Home/Settings/TaskCenterScreen/ThreadDeletionCoordinator/TurnSession/MainActivity |
| `ThreadSnapshot` (393) | 会话离线快照（断线时列表/最近消息可读） | Chat/ChatViewModel/TurnSession/Threads/Settings/CacheCleanup/ThreadDeletionCoordinator/MainActivity |
| `ConnectionBanner` (158) | 断连全局横幅（`connState` down→显示，重试自愈通道） | Chat/ChatTimeline/Home/Threads |
| `SwipeRevealRow` (316) | 左滑露出操作行（**单行互斥 `openKey`**） | Threads/Files/Git |
| `UndoBar` (310) | 撤销条（5s，app 级复用） | Chat/Threads/PromptOptimizeLogic/PromptOptimizePreview |
| `PaSkeleton` (254) | 骨架屏（首载窗口专用，失败绝不撒谎） | Home/Threads/PaMotion |
| `PaStateViews` (533) | 空态/错误态视图族 | Home/Settings |
| `TypingDots` (103) | 打字/流式动效 | ChatComposer/Chat/ChatTimeline/PromptOptimizePreview/SubAgentCard |
| `SplitButton` (262) | 主+副分裂按钮 | Chat/CollabRoleEditor/RuntimeSection/Settings |
| `DrawerController` (137) | 抽屉跟手控制器（`rememberDrawerController`，progress/visible/onDrag/onRelease） | **Home/Threads/Files/Git/TaskCenter/Chat 各自持有实例**（AppDrawer 是内容，controller 是状态） |
| `RecentThreadsDrawer` (396) | **会话页专用**侧栏（最近会话搜索 + 底部全局入口） | 仅 Chat |
| `MdnsDiscovery` (466) | mDNS 发现主机列表 | AgentConnection + Home |
| `PathNav`(88)/`FileKind`(220) | 面包屑分段 / 文件类型判定 | Files + FolderPicker |
| `DocFormatRegistry` (160) | 文档 MIME/打开方式注册 | Browser + TaskCenter |
| `ThreadPins`(54)/`ThreadUsageStore`(145)/`TurnCheckpointStore`(128) | 置顶 / 上下文用量 / 轮次检查点持久化 | Threads、TurnSession、MainActivity（init） |
| `PaNotifications` (504) | 通知渠道 + 审批动作接收器 + **深链 `pendingChatNavigation`** | MainActivity/Chat/ChatViewModel |
| `WebNavPolicy`(110)/`WebDownloadBridge`(62) | 网页信任策略 / JS 下载桥 | 仅 Browser（**安全边界**） |
| `HistoryQuickNav`(1149)+`HistoryQuickNavLogic`(437) | 历史快速跳转（消息定位） | Chat/ChatTimeline |
| `TerminalBlocks` (578) | 终端输出渲染块 | Chat/ChatTimeline（复用为代码/终端卡） |
| `SenseVoiceModelManager/Recognizer/Session/ModelFiles` | 端侧语音模型（下载/解压/识别） | Chat + Settings |
| `ContextMenu`+Logic (268+?) | 长按上下文菜单 | 仅 Chat（单页使用，列此备查） |
| `DeliveryBadge`/`TaskRetryGate`/`PaCache`/`PaPaths`/`PaFormat`/`PaHaptics` 见上 | | |

`ui/` 层被多页复用的**非 component** 共享件：
- `EditorialStatus.kt` (148) —— 状态词表与色调（Chat/Collab/TaskCenter/Threads/Home 共用，**迁移时必须先定**）
- `AppDrawer.kt` (923 / 40.6KB) —— 见 §3
- `MarkdownText`(729) + `CodeSyntax`(405) —— Chat 时间线主渲染（Terminal 的终端块也走）
- `FolderPicker`(249) —— Git / Terminal / Threads 选工作区
- `SubAgentCard` —— ChatTimeline / Threads（子 agent 卡）

---

## 3. 跨页状态枢纽

### 3.1 `AgentConnection`（根目录 `AgentConnection.kt`，1813 行 / 98.5KB）—— **最大的枢纽，且规划里"零重写"**

连接与会话级真值全部在这里（Kotlin 宿主保留，Flutter 只读订阅）：

| 字段 | 类型 | 语义 | 消费方 |
|---|---|---|---|
| `connState` | `StateFlow<String>` | `"checking"/"ok"/"down"` | 所有页横幅/顶栏、Onboarding、TaskCenter 离线分支、ConnectionBanner |
| `connTransport` / `connAddress` | `StateFlow<String>` | 传输标签 / 当前地址 | PaHostChip（Home/Threads/Settings/Chat） |
| `connError` | `StateFlow<ConnectionDiagnostic?>` | 连接失败诊断 | Settings 连接页 |
| `connectionEpoch` | `StateFlow<Int>` | **重连成功自增 = 全局刷新信号** | Home/Threads/Git/Chat/Settings 的 `LaunchedEffect` |
| `hostCapabilities` | `StateFlow<List<String>>` | **能力位唯一真值**（`updateHostCapabilities` 写入方：Home :375、Settings :667、RuntimeStateLogic :755） | Home/Threads/Chat/Settings/Collab/CollabRoleEditor/RuntimeSection |
| `hostEvents` | `MutableSharedFlow<HostEvent>` | `WatchEvents` 全局事件流（含 sequence 去重） | TurnSession、CollabSessionRepository |
| `selectedAgent` / `selectedModel` / `selectedEffort` | `mutableStateOf` | 当前 agent / 全局模型 / 力度 | Home（切换）、Threads（筛选初值）、Chat、Settings |
| `disabledAgents` | `mutableStateOf<Set<String>>` | 停用偏好（持久化） | Home 网格、Collab 参与者、Threads |
| `drawerCollapsed` | `Boolean`（持久化） | 抽屉折叠偏好 | AppDrawer |
| `recentWorkspaces` | `StateFlow<List<String>>` | 最近工作区（最多 6） | Terminal、Git、NewThreadSheet |
| `mdnsDiscovery` | 可空单例 | mDNS 发现 | Home 发现态、自动重连 |
| `token`/`appVersionOutdated`/`minAppVersionRequired` | — | 令牌 / 版本闸 | Settings、启动 |
| 模型偏好 API | `modelForAgent`/`setThreadModel`/`setThreadEffort`/`saveAgentDefaults`… | agent/thread/全局 三档模型选择 | Chat、Settings、ModelSelectionRepository |

### 3.2 `TurnSession`（`ui/TurnSession.kt`，1800 行）—— **轮次状态机（页面级单例，跨页存活）**

- `TurnSession.state(threadId)` 按 threadId 持有：`draft`、`items`、`pendingApproval`、`errorMessage`、`queuedCount`、`historyLoaded`、`lastSequence`、`running`…
- **写入通道**：`send` / `cancelTurn` / `answerApproval` / `retryRecovery` / `resyncFromDaemon` / `markGuardianOverridden`
- **只读投影**：`ChatViewModel.session`（snapshotFlow 桥）→ ChatScreen；`runningThreadIds()` → Home mini 条（5s 轮询）
- **持久化**：`TurnCheckpointStore`（恢复在跑轮次）、`ThreadUsageStore`、`ThreadSnapshot`
- 消费方：Chat（主）、Home（mini 条）、TaskCenter（`resyncFromDaemon` 重试）、Threads（状态徽章）

### 3.3 `AppDrawer`（`ui/AppDrawer.kt`，923 行 / 40.6KB）—— **主导航状态**

- 内容由 `drawerNavEntrySpecs()` :135 + `buildDrawerNavEntries()` :159 纯函数生成（**单测钉死条目集合**），含"消息中心恒定条目"的确定性规则。
- 读取：`TaskCenter.unreadCount`（未读徽章）、`AgentConnection.connState`（主机状态行）、`AgentConnection.drawerCollapsed`、`AgentConnection.selectedModel`（底部模型行）。
- **导航状态本身不在 AppDrawer**——`DrawerController`（每页各持一份实例）+ `RootNav` 的 `tab`/`openHomeDrawer`。Flutter 侧需要一个显式的"全局导航状态"对象来等价 `tab` + `openHomeDrawer` + `drawerCtrl`。
- 入口接线：Home（全量，含 `onOpenCollab`/`onOpenAttention`）、Threads/Files/Git/TaskCenter（`onGoTab`/`onOpenTerminal`/`onOpenSettings`/`onOpenTaskCenter`/`onOpenBrowser`）、Chat（**用 RecentThreadsDrawer 替代**）。

### 3.4 其他 object 级共享状态

| 单例 | 持有 | 消费方 |
|---|---|---|
| `TaskCenter`（component/TaskCenter.kt:22） | `tasks`、`unreadCount`（StateFlow）+ SharedPreferences 持久化（300ms 节流） | TaskCenterScreen、AppDrawer 徽章、Home、Settings、TurnSession |
| `RuntimeControlState`（RuntimeStateLogic.kt:657） | `loadState/sectionErrors/overview/recovery/budget/collab/unknownEvents/unknownLoadedAtMs` | Settings console 页（主）、**Home 消息中心**（`refreshUnknownEvents` + `unknownLoadedAtMs`） |
| `CollabSessionRepository`（collab/:45） | `state: StateFlow<CollabSessionState>`（reducer 投影 + 事件流续传水位） | CollabScreen |
| `CollabRolesState`（CollabRolesState.kt:34） | `roles/loadState/loadError`（`mutableStateOf`） | Home（数量）、Collab、CollabRoles、CollabRoleEditor |
| `CollabState`（CollabState.kt:26） | `selected/roleBinding/dispatching/lastInstruction`（**换页保留**） | CollabScreen |
| `ModelSelectionRepository`（model/:40） | `state: StateFlow<ModelSelectionState>`（线程级模型/力度选择 + 本地缓存） | Chat、Settings |
| `OnboardingPrefs`（Onboarding.kt:60） | `pa_onboarding/done` | RootNav startDestination、Onboarding |
| `OfflineStore` / `PaCache` / `PaPaths` | 分区缓存与磁盘目录 | Browser/Settings/Home/Chat |
| `PaNotifications` | 通知 + `pendingChatNavigation` | MainActivity ↔ Chat |

---

## 4. 迁移难度评级

| 页面 | 难度 | 行数（主文件 / 该页全栈） | 一句话理由 |
|---|---|---|---|
| **Splash** | **S** | 206 / 206 | 纯动画覆盖层，无 RPC 无状态，唯一约束是"覆盖 RootNav"的时机语义 |
| **Onboarding** | **S** | 539 / 539 | 纯 Pager + 文案，只订阅 `connState`，`OnboardingPrefs` 一行 SharedPreferences |
| **CollabRoleEditor** | **S** | 343 / 452（+RolesState/Logic 109） | 9 个表单字段 + `setCollabRole` 一个 RPC，无流无门控（下拉靠 capabilities） |
| **CollabRoles** | **S** | 317 / 483 | 一个列表 + 一个 RPC，状态是共享 object 的简单投影 |
| **Files** | **M** | 958 / 1841（+FolderPicker/PcFilePreview/FileKind/PathNav） | RPC 只有 6 个 FileSystem 动作，但面包屑/左滑单行互斥/预览交 Browser 的边界要复刻 |
| **Git** | **M** | 1160 / 1250 | 10 个 Git RPC 都是请求-应答；难点是 6 层返回回退 + 仓库候选解析 |
| **TaskCenter** | **M** | 928 / 1410（+TaskCenter/TaskRetryGate） | 零直连 RPC，但每行重试门闩 + 三类重试分派 + 离线分支语义必须逐条对齐 |
| **Terminal** | **L** | 758 / 1336（+TerminalBlocks） | **双向 gRPC 流 + WebView/xterm.js 桥**（bindWebView/onReady/resize/close），生命周期与窗口尺寸联动 |
| **Browser** | **L** | 1310 / 2916（+PcFilePreview/WebNavPolicy/WebDownloadBridge） | WebView + 信任源策略（**安全红线**）+ 流式读远端文件 + 手机/桌面视图切换 |
| **Collab** | **L** | 1118 / 2066（+Logic/State/collab 四件套） | 快照先行 + 事件流跟随 + 缺口重建的会话状态树，派发/取消/重试四操作的收敛语义 |
| **Threads** | **L** | 2836 / 3228 | 4 个列表源 + 3 维正交筛选 + 分组/导入/归档 11 个写 RPC + 8 级返回回退 + 离线快照 |
| **Home** | **L** | 2453 / 2783（+HomeWorkbenchLogic/attention） | 发现态配对（mDNS+配对码 TOFU）+ 注意力三分 + 能力驱动网格 + mini 条，浮层多达 6 个 |
| **Settings** | **XL** | 4695 / **~8900**（+RuntimeSection 1548 + RuntimeStateLogic 1092 + ControlPlane 600 + model/* 1168 + prompt 相关 + 各 Logic） | 单文件 5 个二级页 + 20+ 状态字段 + 三态（加载/真空/失败）纪律 + `setting_states` 逐项真实回执 + console 页又是 1548 行 |
| **Chat** | **XL** | 6165 / **~17000+**（chat/* 5156 + TurnSession 1800 + TurnDelivery + model/prompt/voice/history + Markdown/CodeSyntax/AttachmentPreview/SubAgent/ContextUsage + HistoryQuickNav 1586） | 唯一同时用 4 个服务的页；**双向 turn 流 + 增量恢复/回放状态机**；时间线滚动/未读/沉浸态；8 个能力位门控；附件上传；内嵌 git 卡；语音双引擎 |

---

## 5. B3 迁移顺序建议（依赖拓扑）

原则：**谁持有共享状态 / 谁定义契约，谁先走**。计划 §B3 的原顺序（Onboarding/Settings → Threads/Home → Files/Git/TaskCenter → Collab → Chat → Terminal/Browser）大体成立，但按依赖拓扑微调如下——关键是**先定 4 个"非页面"的共享契约**，否则每页都会重复返工。

### 第 0 波（不是页面，但必须最先定 —— 否则后面每页都要改）

1. **连接/能力态契约**（对应 §3.1 + B2 通道）：`connState/connTransport/connAddress/connectionEpoch/hostCapabilities/hostEvents` 六条流的 Dart 形态，以及 `agentCapableOf(bit, agentId)` 的纯函数等价实现。
   *依赖方：全部 12 页。*
2. **导航状态契约**：`tab`（0-4）+ `openHomeDrawer` + `DrawerController` 三者合成的"全局导航状态"，以及 `backToHome` 语义；同时定案 **home 是 1 路由 5 Tab 还是 5 路由**。
   *依赖方：除 Splash 全部。*
3. **设计系统 + 词表**：`PaEditorial`/`editorialM3Scheme` token、`EditorialStatus` 状态词、`PaErrorView` 错误文案映射（`paErrorViewOf`）。
   *依赖方：所有有状态徽章/错误横幅的页。*
4. **`TaskCenter` 与 `PaNotifications` 两个 app 级单例**（含深链 `pendingChatNavigation`）——它们不是页面，但 TaskCenterScreen、AppDrawer 徽章、Home、Chat 都读。

### 第 1 波 —— 低依赖、验证契约（计划中 "Onboarding/Settings"）

5. **Onboarding（S）** —— 只依赖 1（`connState`）+ 3，是 B2 通道的最小验收页。
6. **Splash（S）** —— 只依赖 3（主题 + 动画），可与 Onboarding 并行。
7. **Settings（XL，可先落骨架再逐 sub 填）** —— 计划把它排在最前是因为**它是能力位与偏好的写入口**：`updateHostCapabilities`、模型/力度选择、`agent_native_settings`/`runtime.v1` 门控。建议分 3 个子步：① 一级页 + `connection`（B2 配对/连接流的消费者）→ ② `agent`/`capabilities`（模型与能力契约）→ ③ `downloads`/`console`（console 依赖 RuntimeControlState，可延后）。
   *依赖：1、3、4；被 Chat（模型/审批预览）、Home（设置入口）依赖。*

### 第 2 波 —— 列表双雄（计划 "Threads/Home"，**建议 Threads 先于 Home**）

8. **Threads（L）** —— 先 Home 的理由：Threads 只依赖 1/2/3 + `ThreadPins`/`ThreadSnapshot`，是"列表 + 三维筛选 + 写操作"的完整样板，先把 ThreadService 的 11 个写 RPC 与离线快照跑通；它的抽屉只是 `AppDrawer` 的一组**非接线**调用（`onOpenAttention`/`onOpenCollab` 传 null → 这两条恒渲染但置灰，见 `buildDrawerNavEntries` AppDrawer.kt:159）。
9. **Home（L）** —— 依赖 1/2/3/4 + `AppDrawer` **全量接线**（Home 是唯一传 `onOpenAttention`/`onOpenCollab` 的调用方）+ `RuntimeControlState`（消息中心）+ `CollabRolesState`（协同行数量）+ `TurnSession.runningThreadIds`（mini 条）。**Home 必须在 Threads 之后**，因为它要把两者共用的抽屉、注意力卡、agent 网格一次接齐。

### 第 3 波 —— 工具三件套（计划 "Files/Git/TaskCenter"，顺序不变）

10. **Files（M）** —— 依赖 2/3；输出（`browser?url=`）先留空实现。
11. **Git（M）** —— 依赖 Files 的工作区概念（`FolderPicker`）+ 2/3。
12. **TaskCenter（M）** —— 依赖 4（TaskCenter 单例）+ `PaDownloader` + `TurnSession.resyncFromDaemon`；等 Chat 就绪后回填"消息找回"的真实效果。

### 第 4 波 —— 协同三页（计划 "Collab"）

13. **CollabRoles（S）→ CollabRoleEditor（S）→ Collab（L）** —— 顺序有硬依赖：CollabScreen 的角色弹层与"管理协同角色"要 `CollabRolesState.roles`，而 `CollabRolesState` 是跨页 object。**先 Roles 再 Collab**（与计划相反，理由：CollabScreen.kt:124 进页即 `CollabRolesState.load()`，无角色数据则弹层空转）。依赖 1（`collab:<id>`）+ ThreadService 会话流。

### 第 5 波 —— Chat（计划 "Chat"）

14. **Chat（XL）** —— 必须最后压轴之一，因为它依赖**前面几乎全部**：`TurnSession`、`ModelSelectionRepository`、`ThreadSnapshot`、`TaskCenter`、`PaNotifications`、`RuntimeControlState`、8 个能力位、`AppDrawer` 的对位物 `RecentThreadsDrawer`。
    建议再拆 4 个子步：① `TurnSession` + 双向 turn 流 + 时间线骨架 → ② 历史加载/离线快照/恢复回放 → ③ 输入区（附件/斜杠/语音/模型选择）→ ④ 周边（审批卡、git 卡、HTML 导出、提示词优化、子 agent 卡）。

### 第 6 波 —— 独立双雄（计划 "Terminal/Browser 最后"，不变）

15. **Terminal（L）** —— 依赖 1 + 双向流（与 Chat 的 turn 流同型，可复用流封装）；WebView/xterm 桥与 Browser 的 WebView 设施可共享。
16. **Browser（L）** —— 依赖 1 + `WebNavPolicy` 安全策略 + `PcFilePreview`；被 Files/TaskCenter/Settings/Chat 的"打开链接"出口消费，**放在它们之后**才能把出口从占位换成真页。

### 拓扑图（→ = 必须先有）

```
[0] 连接/能力流 · 导航状态 · 设计词表 · TaskCenter/PaNotifications
      │
      ├─→ [1] Onboarding · Splash · Settings(①连接 ②Agent/能力 ③下载/控制台)
      │         │
      │         └─→ [2] Threads ─→ Home（+AppDrawer 全量 + 消息中心 + mini 条）
      │                        │
      ├─→ [3] Files ─→ Git ─→ TaskCenter（回填需 TurnSession）
      │                        │
      ├─→ [4] CollabRoles ─→ CollabRoleEditor ─→ Collab
      │                        │
      └─→ [5] Chat（TurnSession→历史→输入→周边）─→ [6] Terminal · Browser
                                                        │
                                          Files/TaskCenter/Settings/Chat 的浏览器出口回填
```

**每页验收口径**（计划 §B3 红线）：`flutter analyze` 零警告 + 该页 `flutter_test` 重建 + 能力位门控用例（无位不显示假入口）+ 分级返回用例（临时态先清再回主页）。
