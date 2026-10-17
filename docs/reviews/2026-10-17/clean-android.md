# 整洁债清单 — Android (Kotlin/Compose) 域

> 只读审查 · 2026-10-17 · 未改动任何代码/文件，未 git add/commit/push
> 已过一轮「误报剔除」复核：表达式体 fun 行数虚高问题已修正（§2.2 记录 7 组误报移除、§2.6 记录嵌套深度误报校正），报告数字以复核后口径为准。
> 量化脚本：`C:\Users\yanzi\AppData\Local\Temp\opencode\{analyze,patterns,scan*,check*,callbacks,...}.py`
> 数据基线：`android/app/src` 381 个 .kt 文件（main 173 / test 208），2906 个已识别 fun
> 审查框架：命名 / 函数-Composable / 状态管理 / 注释 / 错误处理 / 边界结构 / 测试整洁

---

## 0. 总览（影响 × 置信度）

| 分级 | 条目数 | 摘要 |
|---|---|---|
| **P0**（结构性硬伤，改造成本高但收益最大） | 5 | ChatTimelineArea 35 参 1093 行 / ChatComposerArea 43 参 918 行 / ChatScreen 6149 行 / SettingsScreen 4579 行 / 回调瀑布 4 层 |
| **P1**（明确反模式，局部可改） | 11 | 巨型 when 6 处 / mutableStateOf 355 处散落 / LaunchedEffect(Unit) 30 处 / 日期戳注释 273 处 / 单字母变量 553 处 / SheetDialogButton 四写 + tab 芯片五写 / Toast 99 处 |
| **P2**（卫生问题，随手工艺） | 8 | 空 catch 2 处 / println 0（干净）/ TODO 2 处（已归属）/ runCatching 丢弃约 12 处 / 重复 fun 名 63 组 / 断言无消息 30% / 测试样板重复 / 无 testkit |

**审查中「查了但干净」的项**（同样重要，避免后续重复劳动）：
- `val isX` 非布尔：**0 处**。
- 中文标识符：**0 处**（中文只在注释/字符串/HTML 模板）。
- `m` 前缀残留：**1 处**（`BrowserScreen.kt:1016`）。
- 注释掉的代码：**0 大段**（5 处命中全是说明文字）。
- TODO/FIXME 无归属：**0 处**（2 个 TODO 都有批次归属）。
- `println/print`：**0 处**。
- `collectAsState` 缺 initial：**0 处**（全部 StateFlow）。
- composable 嵌套 > 3 层：**0 处**（实际 ≤2 层，非债务）。
- 谎言注释：抽查 20 处高危，**20/20 与实现对账一致**。
- 超长 test（>100 行）：**0 个**。

红线区（本次只标注、不建议改）：
- `android/app/src/main/proto/`（生成代码）— 未纳入统计。
- `ui/theme/`（PaEditorial/PaGlass/PaHacker/PaTheme/PaContrast）五套 token 数值 — 只审结构不碰数值。
- `AgentConnection.kt` 的 TLS/pin/keystore/token 安全层 — 只标注命名与注释问题。
- `bridge/`（B4 波）与 `flutterhost/` — 审但标注「新代码谨慎大改」。

---

## 1. 命名

### 1.1 谎言名（`getX` 实际改状态 / `val isX` 非布尔）— P2 · 高置信

| 证据 | 反模式 | 说明 |
|---|---|---|
| `ui/component/OfflineStore.kt:339 fun getUrl` | 谎言名 | 函数体在给 `capturedAt/mime/name` **赋值**（解析缓存 key），读起来像纯 getter，实际是「解析并填充出参结构」。 |
| `AgentConnection.kt:1328 fun getCollabSession` / `1444 fun getThread` | 谎言名（轻度） | 名字像缓存读，实际会触发 RPC（阻塞网络）。「get」在 Kotlin 里通常暗示便宜；建议 `fetchCollabSession` / `loadThread`。 |
| `bridge/BridgeConnection.kt:60-84` 共 12 个 `getX(request)` | 谎言名（成体系） | 每个都是 100+ 行阻塞 RPC 包装，`get` 一律应为 `fetch`/`load`。 |
| `ChatScreen.kt:2319 fun setInput(value: String)` / `3032 fun setApproval(id)` | 谎言名（轻度） | `setX` 暗示写字段，实际触发发送/审批 RPC 链。 |

`val isX` 非布尔扫描：**0 处硬伤**（14 处命中全部是真布尔，如 `ChatScreen.kt:1114 isSubagent`）。这一项干净。

### 1.2 噪声万金油类名 — P2 · 高置信（全量 19 处）

```
main/.../PinTrustManager.kt:21            class PinTrustManager              ← 红线区（安全层）只标注
main/.../bridge/BridgeChannel.kt:43       fun interface BridgeMethodCallHandler
main/.../bridge/BridgeEventChannel.kt:44  interface BridgeEventStreamHandler
main/.../ui/ThreadDeletionCoordinator.kt:34   object ThreadDeletionCoordinator
main/.../ui/chat/ChatViewModel.kt:65      interface ChatRpcGateway
main/.../ui/chat/ChatViewModel.kt:80      class RealChatRpcGateway
main/.../ui/component/DrawerController.kt:95  class DrawerController
main/.../ui/component/SenseVoiceModelManager.kt:51  object SenseVoiceModelManager
main/.../ui/voice/VoiceController.kt:64   class VoiceController
```
判断：命名本身**有域前缀**（`ThreadDeletion` / `SenseVoiceModel` / `Bridge*`），不属于「Util/Helper 万金油」重症；`ChatRpcGateway` 的 `Real` 前缀是实现类惯用法。**不建议批量改名**，P2 记录即可。

### 1.3 缩写/单字母变量 — P1 · 高置信

`val/var <单字母>` 声明共 **553 处**（非 lambda 作用域）：

| 变量 | 次数 | 典型证据 |
|---|---|---|
| `s` | 99 | `ChatScreen.kt:2059 val s = steered.coerceAtLeast(0)` |
| `p` | 65 | `TokenStore.kt:183 val p = TokenMigration.plan(...)` |
| `d` | 45 | `CfbReader.kt:12 private val d: ByteArray` |
| `t` | 45 | `BrowserScreen.kt:356 val t = raw.trim()` |
| `r` | 43 | `ChatScreen.kt:3608 val r = chatVm.fetchEarlierHistory(...)` |
| `e` | 40 | `CfbReader.kt:99 val e = entries.firstOrNull{...}` |
| `b/f/v/g/a/n/c/q/m/w/o/i/h/k/j/u/l` | 344 | 散布 |

另有 **146 个 fun 带单字母参数**：`CfbReader.kt:147 u16(o)` / `i32(o)` / `XlsLegacy.kt:378 readChars(b, o, cch, high)` / `FilesScreen.kt:161 loadDir(p: String)` / `ErrorCopy.kt:17 humanizeError(e: Throwable?)`。

判断：`CfbReader/XlsLegacy/PcFilePreview` 是二进制解析域，`b/o/i` 有域内共识，**不算债**。真正该改的是 UI/业务层的 `ChatScreen.kt:2059-2060`（`s`=steered、`f`=followUp）、`FilesScreen.kt:161 loadDir(p)`、`ThreadsScreen.kt:1897 pcAgentVisible(...)` 一类——**短作用域可接受，跨 10+ 行使用的必须改名**。

`m` 前缀残留：全库 **1 处**（`BrowserScreen.kt:1016 val mLower = mime.lowercase()`）——Kotlin 里应叫 `lowerMime`。P2。

### 1.4 中文/英文混名 — P2 · 高置信

代码标识符内含中文：**0 处**（1407 处中文命中全部在注释/字符串字面量/HTML 模板里，如 `BrowserScreen.kt:1166 <div>内置浏览器</div>`）。中文文案硬编码在 .kt 里是另一回事（见 §5.3）。命名卫生这一项干净。

### 1.5 同一概念多个名字 — P1 · 中高置信

`connState` / `connSt` / `connectionEpoch` / `reconnectTick` / `connStateTop` / `connStateFrame` 六个名字指同一族连接状态：
- `ChatScreen.kt:3024 val connState = ...` vs `ChatScreen.kt:3838 val connSt = ...`（**同一文件两个名字**）
- `SettingsScreen.kt:1046 val connStateTop = ...` vs `SettingsScreen.kt:3271 val connState = ...`（同一文件两个名字）
- `HomeScreen.kt:365 reconnectTick` / `GitScreen.kt:300 reconnectTick` / `HomeScreen.kt:400 connState` / `HomeScreen.kt:401 connTransport`

`loadError` / `settingsError` / `pairError` / `hostError` / `modelError` / `mcpError` / `capabilitiesError` 七个错误态（SettingsScreen 546-586）语义重叠度高。

重复 fun 名跨文件 63 组（`refresh` 6 处、`send` 7 处、`load` 4 处、`dispatch` 2 处、`reduce` 4 处、`extractRpcMessage` 2 处、`summaryOf` 2 处）——多数是不同类的同名方法（可接受），但 `extractRpcMessage`（`CollabRolesState.kt:104` 与 `RuntimeStateLogic.kt:1031` 逐字重复）与 `summaryOf`（`DesktopEchoLogic.kt:65` 与 `HistoryQuickNavLogic.kt:88`）是**真复制**，应收敛。

---

## 2. 函数 / Composable

### 2.1 超长 Composable Top 15 — P0 · 高置信

| # | 位置 | 行数 | 参数 | 备注 |
|---|---|---|---|---|
| 1 | `ui/chat/ChatTimeline.kt:181 ChatTimelineArea` | **1093** | **35** | 全库最大 composable |
| 2 | `ui/chat/ChatComposer.kt:156 ChatComposerArea` | **918** | **43** | 输入区全功能 |
| 3 | `ui/SettingsScreen.kt:2145 AgentPage` | 637 | 2 | |
| 4 | `ui/ChatScreen.kt:4748 ApprovalSheet` | 418 | 3 | |
| 5 | `ui/CollabScreen.kt:104 CollabScreen` | 412 | 1 | |
| 6 | `ui/component/PaApprovalDialog.kt:162 ApprovalBody` | 377 | 2 | |
| 7 | `ui/SettingsScreen.kt:3801 CacheSection` | 354 | 4 | |
| 8 | `ui/ChatScreen.kt:5172 ThreadGitPanel` | 299 | 2 | |
| 9 | `ui/SettingsScreen.kt:3255 ConnectionPage` | 288 | 1 | |
| 10 | `ui/ChatScreen.kt:5670 ExportChatSheet` | 268 | 7 | |
| 11 | `ui/AppDrawer.kt:378 DrawerPanel` | 267 | 6 | |
| 12 | `ui/component/RecentThreadsDrawer.kt:79` | 248 | 4 | |
| 13 | `MainActivity.kt:162 RootNav` | 246 | 0 | 14 个 Screen 的直接分发 |
| 14 | `ui/SettingsScreen.kt:3555 DownloadsPage` | 227 | 1 | |
| 15 | `ui/RuntimeSection.kt:1227 ThreadMemorySheet` | 225 | 2 | |

另有 16 个 200+ 行 composable（`ContextUsageCard` 208 行 14 参、`ChatQuickModelsPanel` 206 行 10 参、`SwipeRevealRow` 205 行、`UnknownEventDetailSheet` 216 行、`UnknownEventCardRow` 204 行 8 参、`ToolCallCard` 216 行、`CollabRoleEditorScreen` 220 行）。

### 2.2 超长普通函数 — P1 · 高置信

> **口径修正（逐条人工复核后）**：脚本的 brace 匹配会把**表达式体 fun**（`fun x(...) = expr`，无 `{`）之后的 fun 误并进它的 body。复核后 **7 组为误报、已从债清单移除**（记录在表后，防后续复扫再误判）。真实超长普通函数如下。

| 位置 | 行数 | 判定 |
|---|---|---|
| `bridge/BridgeMethods.kt:118 dispatch` | **339（属实）** | 33 分支巨型 when（见 §2.5）；B4 波新代码，标注谨慎大改 |
| `ui/HomeScreen.kt:716 LazyListScope.connectedHomeItems` | **337（属实）** | 716-1052 行；首页「已连接」LazyColumn 全部内容构造 |
| `ui/TurnSession.kt:917 applyServerFrame` | **298（属实）** | 18 分支帧分发 |
| `ui/HomeScreen.kt:1171 LazyListScope.discoveryItems` | **209（属实）** | 同上形态，发现态内容构造 |
| `ui/ThreadsScreen.kt:336 doRefresh` | **196（属实：336-531）** | 局部 suspend fun：快照 + 离线横幅 + 列表拉取 + 分组 + pin 全在一处 |
| `AgentConnection.kt:1608 startAutoReconnect` | **143（属实：1608-1750）** | 红线区（连接安全层）只标注：探活循环 + tokenReady 门 + 指数退避 + mDNS 自愈在一处 |
| `bridge/BridgeConnection.kt:60-84` 共 12 个 `getX` | 103-127 each | 同构 RPC 样板（见 §6.4）；B4 波，标注谨慎 |

**误报移除记录（真身 1-2 行，全部是有单测或明确契约的纯函数）：**

| 脚本报的 | 真身 |
|---|---|
| `ChatTimeline.kt:166 userGroupNeighbors` 1108 行 | **2 行**（166-167），真机 P0 崩溃修复成果，JVM 单测钉死 |
| `ChatScreen.kt:4726 askIsAskAction` 440 行 | **1 行**（`= name in ApprovalAskNames`） |
| `HomeScreen.kt:1167 manualAddSpec` 213 行 | **1 行**（1167-1168） |
| `ControlPlaneLogic.kt:431 controlPlaneUnsupported` 170 行 | **1 行**（431-432，`= Status...UNIMPLEMENTED`） |
| `ThreadsScreen.kt:1890/1897/1906/2297/2300` 五个 144-175 行 | **各 1-2 行**，KDoc 明写「纯函数,单测钉死」 |
| `Onboarding.kt:64/67 isDone/markDone` 185/182 行 | **各 2 行**（64-65 / 67-68） |

教训固化：行数统计必须单独识别表达式体 fun（无 `{` → 到下一声明为止），否则整体虚高。§2.1 composable 表的被统对象全部带 `{`，结论不受影响。

### 2.3 参数 > 5（回调瀑布）— P0 · 高置信

**MainActivity→HomeScreen 回调链实测 4 层：**

```
MainActivity.RootNav (MainActivity.kt:247)
  └─ HomeScreen(12 个 on*/参数)                     HomeScreen.kt:318-343
       └─ AppDrawer(12 个 on*/参数)                 AppDrawer.kt:205-250  （HomeScreen 内部再传一遍）
            └─ 各二级 Screen(8-9 个 on*/参数)        ThreadsScreen 8 / ChatScreen 9 / FilesScreen 8 / GitScreen 8 / TaskCenterScreen 8
                 └─ ChatTimelineArea(35 参)         ChatTimeline.kt:181-236
                 └─ ChatComposerArea(43 参)         ChatComposer.kt:156-199
```

`ChatComposerArea` 43 参数全列表（`ChatComposer.kt:156-199`）：
`threadId, session, verifyingSettled, ctx, hapticView, input, setInput, attachments, threadAgentId, slashCommands, offlineSnapshot, hostCaps, slashGoalSheet, onCompact, onStartReview, onOpenAttachment, voiceLandedTick, voiceListening, voiceState, voiceReplaceMode, voiceElapsedSec, micPermission, voiceController, reportVoiceUnavailable, expandedInputOpen, modelName, effortLabel, showQuickModels, modelFlashTick, sessionAgent, codexSettings, onSetApproval, windowIsFallback, contextWindow, showContextCard, showGitPanel, pickFiles, promptOptState, onOptimizeInput, onSend, immersed, onExpandImmersed, expandFocusTick`

`onOpen*` 家族在全库重复声明 17 次 `onOpenSettings` / 17 次 `onOpenBrowser` / 14 次 `onOpenThread` / 13 次 `onOpenTerminal` / 13 次 `onOpenTaskCenter` / 12 次 `onGoTab` / 12 次 `onBack` / 7 次 `onBackToHome` —— **同一个导航意图在每个 Screen 签名里重新声明一遍**。RootNav 里 5 个 Screen 各自写 6-8 行 `nav.navigate` lambda（`MainActivity.kt:247-320`），完全是同一组导航动作的复制。

### 2.4 布尔参数控制行为 — P1 · 高置信

**111 个 fun 带 Boolean 参数**（main 源码）。典型：

| 证据 | 问题 |
|---|---|
| `ChatScreen.kt:2807 applyPromptCandidate(overwrite: Boolean)` | true/false 两条完全不同的行为路径 |
| `GitScreen.kt:273 stagePath(path, staged: Boolean)` | 一个函数干 stage+unstage 两件事 |
| `TurnSession.kt:917 applyServerFrame(..., replay: Boolean)` | replay 与否是两条状态机路径 |
| `XlsLegacy.kt:378 readChars(b, o, cch, high: Boolean)` | |
| `ui/theme/PaTheme.kt:60 apply(dark, hacker = false)` / `PaGlass.kt:65` / `PaEditorial.kt:156` | 三处同签名 `apply(dark, hacker)` — 红线区，只标注重复 |

`RolePickRow(title, desc, selected, onClick)`、`ScopeTab(label, active, onClick)`、`GitTab(label, selected, onClick)`、`KindTab(label, selected, ...)` 这类 `(label, selected, onClick)` 三件套在 5 个文件里各写一遍（`CollabScreen.kt:1081` / `ContextUsageCard.kt:304` / `GitScreen.kt:891` / `TaskCenterScreen.kt:595` / `HomeScreen.kt`），是**重复的 Composable 变体**（见 §6.3）。

### 2.5 巨型 when 分发 — P1 · 高置信

| 位置 | 分支数 | 跨度 |
|---|---|---|
| `bridge/BridgeMethods.kt:121` `when (method)` | **33** | 334 行 |
| `ui/ThreadsScreen.kt:1229` `when {` | **32** | 379 行 |
| `ui/TurnSession.kt:925` `when (ev.msgCase)` | 18 | 284 行 |
| `ui/PcFilePreview.kt:429` | 18 | 61 行 |
| `ui/PcFilePreview.kt:898` | 18 | 44 行 |
| `ui/PcFilePreview.kt:743` | 17 | 64 行 |
| `ui/GitScreen.kt:417` | 16 | 194 行 |
| `ui/PcFilePreview.kt:168` / `852` | 12 / 14 | |
| `ui/XlsLegacy.kt:140` | 18 | 69 行 |

`BridgeMethods.dispatch` 是 method-name 字符串分发（gRPC 桥），改造成注册表（`Map<String, Handler>`）可消除 33 分支，**但属 B4 波新代码，标注「谨慎大改」**。`ThreadsScreen.kt:1229` 的 32 分支 when 是行类型分发（`SessionRow.Local/Pc/PcAgent/...`），可用 sealed class 多态或 `when(row)` 类型分发收敛。

### 2.6 嵌套 composable 深度 — 结论：非债务（工具结论 + 人工校正）

brace 匹配脚本给出的「深度 31/20/19」是**误报**（脚本把文件里所有后续 fun 都算成容器——expression-body fun `PcSessionCard(...) = PcSessionRow(...)`（ThreadsScreen.kt:2144）没有 `{}`，结束行找不到，于是文件后半部分全部 fun 都被算进它的 body）。改用「col-N 闭合括号 + expression-body 单行」口径人工复核后：

| 位置 | 深度 | 说明 |
|---|---|---|
| `ui/ThreadsScreen.kt:1339 @Composable fun SessionRowItem` | **2（局部 composable）** | 244 行局部 composable 声明在 `ThreadsScreen` 的 LazyColumn lambda 里，`@Composable` 与 `fun` 同行；它自己再调 `PcAgentSessionCard` → `SwipeRevealRow` → `Column`，实际渲染嵌套 ≥5 层 UI 节点 |

**结论：本域的 composable 函数嵌套深度全部 ≤ 2，没有 >3 层的函数嵌套。**「嵌套 >3 层」在本域不是债务——真正深的是 UI 节点树（`ThreadsScreen` 内 `LazyColumn > item > SwipeRevealRow > Column > PcAgentSessionCard > ...`），那是 Composable 内容的自然形态，不算反模式。脚本初期输出的 depth=31/20/19 是 expression-body fun（`PcSessionCard(...) = PcSessionRow(...)`，2144 行）结束行误判导致的误报，已人工校正（见附注 3）。

局部 composable `SessionRowItem` 是**最值得抽出去**的一个：244 行、藏在 1657 行的 ThreadsScreen 里、无法单测。

---

## 3. 状态管理反模式

### 3.1 `mutableStateOf` 散落 — P0 · 高置信

全库 **355 处** `mutableStateOf(`，其中非 ViewModel 文件 **273 处 `var x by remember { mutableStateOf(...) }`**：

| 文件 | 数量 |
|---|---|
| `ui/ChatScreen.kt` | **63** |
| `ui/theme/PaEditorial.kt` | 40（红线区 token 结构，只标注） |
| `ui/SettingsScreen.kt` | 28（另有 26 个 `val ... = remember{...}`，合 54 个状态声明） |
| `ui/ThreadsScreen.kt` | 23 |
| `ui/TurnSession.kt` | 19 |
| `ui/theme/PaTheme.kt` | 19（红线区） |
| `AgentConnection.kt` | 16（红线区安全层，只标注） |
| `ui/GitScreen.kt` / `BrowserScreen.kt` / `HomeScreen.kt` / `FilesScreen.kt` | 15/18/12/16 |

`ChatScreen.kt` 单文件 63 个 `remember { mutableStateOf }`：`showModelCard/showHistoryPanel/showQuickModels/showContextCard/showDrawer/showNewThread/showGitPanel/showExport/showMore/slashGoalSheet/rollbackOpen/expandedInputOpen/planPanelOpen` **13 个面板开关** + `rollbackBusy/compactBusy/forkBusy/reviewBusy` 4 个 busy + `recentThreads/recentLoading/attachments/hostName/threadModelId/usageRefreshTick/...`——**典型的「多个相关 state 无组合」**，应收敛为 `ChatPanelsState` / `ChatBusyState` State Object。

`SettingsScreen.kt:546-586` 连续 22 个 `var ... by remember`（addr/token/settings/mcpList/mcpLoading/mcpError/codexUsage/usageLoading/capabilities/capabilitiesLoading/capabilitiesError/host/showApprovalPreview/connectionSaving/settingsError/settingsFeedback/pairError/applyToExistingThreads/showModelPicker/modelChoices/modelError/hostError）——**正是任务描述里「Settings 20+ 字段」的实证**。可分组：`ConnectionFormState` / `McpState` / `UsageState` / `CapabilityState` / `ModelPickerState`。

`AgentConnection.kt:909-1161` 16 个 `by mutableStateOf`（defaultModel/defaultEffort/selectedModel/selectedAgent/uiTextScale/thinkTextScale/historyRoundsState/toolDetailsExpandedState/selectedEffort/showThinkingState/optimizePromptEnabledState/optimizeAgent/optimizeModel/themeModeState/multiAgentState/drawerCollapsedState）是**全局单例可变状态**——比 Screen 级散落更危险（任何页面写、所有页面读），但属连接安全域，**只标注不建议动结构**。

### 3.2 `remember` 缺 key 导致状态错乱 — P1 · 中置信（高危点）

| 位置 | 风险 |
|---|---|
| `ChatScreen.kt:2392 promptOptState by remember { mutableStateOf(PromptOptimizeState()) }` | **无 key**：换 threadId 时提示优化状态不重置（旁边 2399 行的 `promptPreview` 就有 `remember(threadId)`） |
| `ChatScreen.kt:2397 promptOptSeq by remember { mutableStateOf(0) }` | 无 key，跨会话序号错乱风险 |
| `ChatScreen.kt:2548 voiceReplaceMode = remember { mutableStateOf(false) }` | 无 key |
| `ChatScreen.kt:2588-2629` 13 个面板开关全部 `remember { mutableStateOf(false) }` 无 key | 切会话层面板不关（部分场景是期望行为，需逐个确认） |
| `SettingsScreen.kt:3011 expanded by remember { mutableStateOf(false) }` | 无 key，列表项复用 |
| `SettingsScreen.kt:546 addr by remember { mutableStateOf(AgentConnection.configuredAddress()) }` | 只取初始值，地址变更后不跟随 |

已正确用 key 的正面样本：`ChatScreen.kt:502 remember(event.title, event.rawJson)` / `686 remember(streaming)` / `1272 rememberSaveable(item.toolId)` / `2318 remember(threadId)` / `2731 remember(threadId)` / `3159-3167 remember(threadId)` ×5。**同一文件两种纪律并存**，是评审要盯的点。

### 3.3 `LaunchedEffect(Unit)` 滥用 — P1 · 高置信

**30 处** `LaunchedEffect(Unit)`。分类：

**真首帧语义（合理，可保留）**：`ChatScreen.kt:5234 visibleState.targetState = true`（注释明确写了 2026-10-04 审查 :6802 的首帧 target=false 坑）、`PaSheet.kt:217 entered = true`、`TypingDots.kt:70`、`AppDrawer.kt:260`。

**「只在首帧跑却依赖会变的值」（高危）**：
- `ChatScreen.kt:666` — 注释自称「只在进入组合时跑一次：历史加载/回滚/往回滚动重置」，但 `arm` 参数变化时不重跑（KDoc 654 行有说明，属知情决策）
- `ChatScreen.kt:3500 LaunchedEffect(Unit) { snapshotFlow { ... } }` — **知情且正确**（注释 3497 行解释为何用 Unit+snapshotFlow 而非 key）
- `ChatScreen.kt:3858 LaunchedEffect(Unit)` — 需确认读取的连接状态是否会变
- `FilesScreen.kt:318` / `HomeScreen.kt:442` / `RuntimeSection.kt:110,187` / `SettingsScreen.kt:1593,3858,4453` / `Splash.kt:97` / `SubAgentCard.kt:102` / `TaskCenterScreen.kt:338` / `TerminalScreen.kt:110` / `ThreadsScreen.kt:1931` / `ConnectionBanner.kt:122` / `RecentThreadsDrawer.kt:105` / `TerminalBlocks.kt:296,475` / `UnknownEventDetailSheet.kt:101` — 15 处加载型 `LaunchedEffect(Unit)`，**没有参数 key**：如果这些 composable 因配置变更/进程重建重组，不会重新加载（`rememberSaveable` 缺失叠加）。疑似高危，需逐个确认。

### 3.4 `collectAsState` 缺 initial — P2 · 高置信（干净，非债务）

全部 44 处 `collectAsState()` 消费的都是 `StateFlow`（`AgentConnection.connState/hostCapabilities/connectionEpoch/connTransport/connError/minAppVersionRequired/appVersionOutdated/recentWorkspaces`、`TaskCenter.tasks/unreadCount`、`ChatViewModel.state/session`、`VoiceController.state`、`SenseVoiceModelManager.state`、`ModelSelectionRepository.state`、`CollabSessionRepository.state`、`PaNotifications.pendingChatNavigation`、`MdnsDiscovery.hosts`）——StateFlow 必须有初始值，**无缺 initial 问题**。这一项干净。

### 3.5 Compose 里直接做 IO/重计算 — P1 · 中高置信

| 位置 | 问题 |
|---|---|
| `ChatScreen.kt:390,442,446` | `context.contentResolver.openInputStream(uri)` + `BitmapFactory.decodeByteArray` 在 composable 函数体里（附件缩略图解码） |
| `SettingsScreen.kt:2813-2815` | `contentResolver.delete` + `java.io.File(it).delete()` 在 composable 里删文件 |
| `SettingsScreen.kt:4228 webDataUsage(ctx)` | 遍历目录算体积，`CacheSection` 重组即重算 |
| `SettingsScreen.kt:759 writeCodex` | 8 参数 + 83 行，写 Codex 配置（RPC+文件） |
| `BrowserScreen.kt:276-291` | `contentResolver.query/openInputStream/readBytes` 在 composable |
| `ui/component/PaHaptics.kt:109` | `Settings.System.getInt` 系统查询在 composable 路径 |
| `Onboarding.kt:64-68` | `isDone/markDone` 各 185/182 行的 SharedPreferences 包装（不是 IO 在 composable，是函数过长） |

已在改善的正面样本：`ChatScreen.kt:2672` 注释「2026-10-09 效果层外移：模型大表已收进 ChatViewModel(ChatState) — 只读投影」——**迁移进行中，ChatScreen 仍有残留**。

---

## 4. 注释

### 4.1 变更日志式注释（日期戳）— P1 · 高置信

**273 处**注释带 `2026-09-xx ~ 2026-10-16` 日期。Top 文件：

| 文件 | 处数 |
|---|---|
| `ui/ChatScreen.kt` | ~60 |
| `ui/SettingsScreen.kt` | ~15 |
| `AgentConnection.kt` | ~14 |
| `ui/chat/*` | ~25 |
| `ui/RuntimeSection.kt` | ~7 |
| `bridge/BridgeChannel.kt` / `BridgeConnection.kt` / `BridgeErrors.kt` / `BridgeMethods.kt` | ~8（B4 波，标注谨慎） |

典型：`ChatScreen.kt:2177 // 【2026-10-02 修正】选中模型无动态档位时…`、`SettingsScreen.kt:4244 // 【2026-09-30 合并】原生设置/Model/Reasoning effort 与下面…`、`MainActivity.kt:94 // 【色板并轨 2026-10-09】`。

判断：这些注释的**信息量很高**（解释了为什么这么写、钉了什么回归），日期是「何时修的」元数据。Clean Code 的立场是「为什么」该留、「何时」该进 git log。但**批量删除会丢失决策上下文**，建议：保留「为什么」正文、把日期降为次要（或不作为清理目标）。标注为 P1 但不建议本轮动。

### 4.2 谎言注释抽查（20 处高危对账）— P2 · 中高置信（基本属实）

| # | 位置 | 注释主张 | 对账结果 |
|---|---|---|---|
| 1 | `SlashCommandsLogic.kt:74-79` | 「不再是门控唯一依据，只作兼容层」 | ✅ 属实：函数体 `isZcode \|\| isCodex \|\| isPi`，且 82 行有统一入口 `slashPanelAgentOf` |
| 2 | `SlashCommandsLogic.kt:345-351` | LRU 淘汰口径（accessOrder/removeEldestEntry） | ✅ 属实（与 ThreadDeletionCoordinator.purgeInMemory 联动有测试） |
| 3 | `PaScreenScaffold.kt:53-64` | 「插槽 lambda 必须整体 remember，key=捕获值」 | ✅ 属实：注释与 ChatScreen 3906/4010 的 `remember(slot)` 落地一致 |
| 4 | `PaScreenScaffold.kt:58` | 「`@Stable` 不能标在函数参数上」 | ✅ 属实（Kotlin 事实） |
| 5 | `ChatScreen.kt:3497-3499` | 「LaunchedEffect(Unit)+snapshotFlow 只订阅不重跑」 | ✅ 属实（3500-3504 实现一致） |
| 6 | `ChatViewModel.kt:229-233` | 「TurnSession.State 不是 StateFlow，照任务口径用 snapshotFlow」 | ✅ 属实 |
| 7 | `ChatScreen.kt:2672-2674` | 「效果层外移：模型大表已收进 ChatViewModel，只读投影」 | ✅ 属实（2675-2676 读 chatState.models） |
| 8 | `ChatScreen.kt:2307-2309` | 「10s 巡检循环与投影同一生命周期，onDispose 停掉」 | ✅ 属实 |
| 9 | `ChatScreen.kt:654-656` | 「LaunchedEffect(Unit) 只在进入组合时跑一次」 | ✅ 属实（知情决策） |
| 10 | `ChatScreen.kt:5230-5234` | 「首帧 target=false 组合坑（2026-10-04 :6802）」 | ✅ 属实 |
| 11 | `ChatScreen.kt:4104-4105` | 「黑客骨架走裸 Column，常规走编辑式 PaScreenScaffold」 | ✅ 属实（4105 `if (PaHacker.isHacker)`） |
| 12 | `ChatTimeline.kt:577-582` | 「去掉 remember(st) 本地态，直通 st.streamText」 | ✅ 属实（583 `by remember(st) { mutableStateOf("") }` → 注释说下游不消费；有轻微不一致：变量仍叫 displayedStream） |
| 13 | `HomeScreen.kt:151-152` | 「PaSheet 基座组件随主题换肤」 | ✅ 属实 |
| 14 | `SenseVoiceModelManager.kt:48-49` | 「needsDownload 目前无消费，TODO(主模型裁剪)」 | ✅ 属实且有 TODO 归属（唯一 2 处 TODO 之一） |
| 15 | `TurnCheckpointStore.kt:14-16` | 「LRU 上限原 save() 硬编码，2026-09-30 提取」 | ✅ 属实（17-18 行常量化） |
| 16 | `OfflineStore.kt:15-21` | 「partitions 是缓存管理页唯一清单，清理只扫自有目录」 | ✅ 属实 |
| 17 | `ThreadsScreen.kt:1239-1241` | 「读取失败只在没有收窄型临时态时展示」 | ✅ 属实（1242 `narrowingActive` 判定） |
| 18 | `SettingsScreen.kt:560-566` | 「能力 loading/error 必须与确实是空的分开」 | ✅ 属实（563/567 三个独立 state） |
| 19 | `SettingsScreen.kt:934-941` | 「二级页只记一层返回，不引入完整导航栈」 | ✅ 属实（937 `subPageReturn`） |
| 20 | `AppDrawer.kt:222-223` | 「未传 onOpenCollab = 条目置灰不消失（抽屉恒定修复）」 | ✅ 属实（抽屉恒定集合，AppDrawer.kt:398 注释） |

**结论：注释诚实度很高。** 20/20 与实现对账一致。谎言注释不是本域的债。

### 4.3 大段注释掉的代码 — P2 · 高置信（几乎干净）

代码形状的注释行全库 **5 处**，且全部是**说明性文字恰好以代码词开头**（`// rememberSaveable:进程重建…`、`// remember 计算体不可调用 composable…`），**没有大段被注释掉的代码**。这一项干净。

### 4.4 TODO 无归属 — P2 · 高置信（干净）

全库 TODO/FIXME/XXX/HACK 仅 **2 处**，且都有归属：
- `ui/component/SenseVoiceModelManager.kt:48` `TODO(主模型裁剪)` — 归属「主模型裁剪」批次
- `ui/component/SenseVoiceModelManager.kt:102` `TODO(UI 接线,主模型裁剪)` — 同上

---

## 5. 错误处理

### 5.1 空 catch 块 — P1 · 高置信（仅 2 处）

| 位置 | 代码 |
|---|---|
| `AgentConnection.kt:806` | `catch (_: Exception) { }` |
| `ui/TerminalScreen.kt:725` | `try { ro.onCompleted() } catch (_: Exception) {}` |

`AgentConnection.kt:806` 属连接安全域，**只标注**（放锁外探测路径，可能是知情地「探测失败就失败」）。`TerminalScreen.kt:725` 建议至少 `Log.w(TAG, ...)`。

### 5.2 catch 只 print / runCatching 结果丢弃 — P1 · 中高置信

`println/print` 调用：**0 处**（干净）。

`runCatching` 共 **186 处**。结果丢弃的（既无 `val` 赋值也无 `getOrNull/getOrDefault/onSuccess`）经脚本粗筛 + 人工确认，**绝大多数是「静默关流/删缓存」的知情用法**（`PaCache.kt:76-79` clear WebView 数据、`PaDownloader.kt:100/135/220/254/303 sink?.close()`、`OfflineStore.kt:223 deleteRecursively`、`ThreadSnapshot.kt:287-318` 删缓存）。真正值得记的是：

| 位置 | 问题 |
|---|---|
| `ChatScreen.kt:3083` | `runCatching { ... }` 独立成句，结果完全丢弃（需确认是否有意的「尽力而为」） |
| `ChatScreen.kt:5214` / `5690` | 同上形态 |
| `GitScreen.kt:98` | `runCatching { ... }` 丢弃 |
| `ThreadsScreen.kt:375/453` | `runCatching { ... }` 丢弃 |
| `PaNotifications.kt:408/468/484/500` | 4 处丢弃（通知发送失败静默） |
| `RuntimeStateLogic.kt:961/1003` | 丢弃 |
| `UndoBar.kt:138` | `scope.launch { runCatching { job() } }` — 撤销失败静默，用户无感知 |

建议：至少统一成 `runCatching { }.onFailure { Log.w(TAG, "ctx", it) }` 或明确注释「静默是预期」。

### 5.3 Toast 硬编码文案散落 — P1 · 高置信

`Toast.makeText` **99 处**，Top：

| 文件 | 处数 |
|---|---|
| `ui/ChatScreen.kt` | 9 |
| `ui/SettingsScreen.kt` | 7 |
| `ui/ThreadsScreen.kt` | 12 |
| `ui/HomeScreen.kt` | 5 |
| `ui/BrowserScreen.kt` | 4 |
| `ui/FilesScreen.kt` / `ui/chat/ChatComposer.kt` / `ui/unknown/UnknownEventDetailSheet.kt` | 各 2 |
| 其余 13 个文件 | 各 1 |

文案直接内联在调用点，无 `strings.xml`、无文案表。错误信息拼接模式重复 10+ 次：`"…失败:${e.message ?: "连接失败"}"`（`HomeScreen.kt:394`、`ThreadsScreen.kt:525/711/1483/1699`、`SettingsScreen.kt:630/656/837`、`ChatViewModel.kt:335/346/462/558`）。

已有收拢努力：`ui/ErrorCopy.kt`（`humanizeError` / `cancelTurnFailureCopy` / `approvalAnswerFailureCopy`）、`ui/voice/VoiceErrors.kt`、`ui/ThreadsRowLogic.kt`、`ui/component/VoiceErrors.kt`——**文案表已存在但未全覆盖**，ChatScreen/ThreadsScreen 是漏网大户。

### 5.4 错误码字符串硬编码 — P2 · 中置信

`"[A-Z_]{5,}"` 形态字面量：main 侧 `RuntimeStateLogic.kt` 12 处、`PromptOptimizeLogic.kt` 9 处、`collab/CollabSessionReducer.kt` 9 处、`bridge/BridgeErrors.kt` 7 处、`collab/CollabSessionState.kt` 6 处。多数是 proto enum 名/方法名（桥契约需要），但 `PromptOptimizeLogic` 的 9 处错误码应有集中定义（`ErrorCopy.kt` 模式）。

---

## 6. 边界与结构

### 6.1 单文件 god object Top 10 — P0 · 高置信

| # | 文件 | 行数 | 判断 |
|---|---|---|---|
| 1 | `ui/ChatScreen.kt` | **6149** | 会话页全部：UI + 局部状态 + RPC 效果 + 纯函数 + 附件解码 |
| 2 | `ui/SettingsScreen.kt` | **4579** | 设置页全部：连接/用量/能力/缓存/下载/控制台/原生配置 |
| 3 | `ui/ThreadsScreen.kt` | 2790 | 会话列表全部 + 244 行局部 composable |
| 4 | `ui/HomeScreen.kt` | 2451 | |
| 5 | `AgentConnection.kt` | 1978 | 红线区（安全层）只标注 |
| 6 | `ui/TurnSession.kt` | 1816 | 轮次状态机 |
| 7 | `ui/RuntimeSection.kt` | 1532 | |
| 8 | `ui/PcFilePreview.kt` | 1434 | HTML/PDF/XLS 渲染 |
| 9 | `ui/BrowserScreen.kt` | 1310 | |
| 10 | `ui/chat/ChatTimeline.kt` | 1273 | |

`ChatScreen.kt` 已做过拆分（`ui/chat/ChatRoute.kt` 入口包、`ChatState.kt` 状态、`ChatViewModel.kt`、`ChatSheets.kt`、`ChatComposer.kt`、`ChatTimeline.kt`、`ChatQuickPanels.kt`、`ChatApproval.kt`、`ChatImmersive.kt`），**但主文件仍 6149 行**——拆分拆出了「壳」，核心 composable（`ChatScreen` 本体 + `ApprovalSheet` + `ThreadGitPanel` + `ExportChatSheet` + 63 个 remember）还在原地。

### 6.2 Util 类无状态方法堆 — P2 · 中置信

没有 `XxxUtil/XxxUtils/XxxHelper` 类。但 `XlsLegacy.kt` / `OfficeLegacy.kt` / `CfbReader.kt` / `PcFilePreview.kt`（合 2400+ 行）是**格式解析工具堆**，命名上是「文件」不是「Util」，可接受。`PaFormat.kt`（`formatBytesLabel` / `formatTokenCount`）是纯函数集合，与 `RuntimeStateLogic.kt:481 formatBytesLabel`、`SettingsScreen.kt:317 formatTokenCount`、`TerminalBlocks.kt:136 formatTokenCount` **三处重复**（§1.5）。

### 6.3 重复的 Composable 变体 — P1 · 中高置信

`PaSheet` 调用点 **78 处**（main 60 / test 18），`PaScreenScaffold` 9 处。**收敛纪律已建且有测试钉死**（`PaSheetConvergenceTest.kt` / `PaSheetDialogConvergenceTest.kt` / `SheetScrollOverflowRegressionTest.kt` 断言「不得回潮裸 AlertDialog」）。仍存在的变体：

| 变体 | 证据 |
|---|---|
| 裸 `AlertDialog` 回潮 | `ChatScreen.kt:5422` / `ChatScreen.kt:5438`（共 2 处 material3.AlertDialog，测试只覆盖 6 个指定文件） |
| 裸 `Dialog` | `CollabRolesScreen.kt:273` / `FilesScreen.kt:718,863` / `SettingsScreen.kt:2889` |
| `(label, selected, onClick)` tab 芯片五写 | `CollabScreen.kt:1081 RolePickRow` / `ContextUsageCard.kt:304 ScopeTab` / `GitScreen.kt:891 GitTab` / `TaskCenterScreen.kt:595 KindTab` / `HomeScreen.kt` |
| `PulseDot` 两写 | `HomeScreen.kt:1944 PulseDot(color, pulse = true)` / `ThreadStatusBadge.kt:109 PulseDot8` |
| `SheetDialogButton` 四写 | `SettingsScreen.kt:4553 SettingsSheetDialogButton` / `RuntimeSection.kt:1506 RuntimeSheetDialogButton` / `GitScreen.kt:1134 GitSheetDialogButton` / `UnknownEventDetailSheet.kt:658 SheetDialogButton` — **四个文件同一个「PaSheet centered 弹窗按钮」各写一份 27 行**，是最明确的重复 composable |
| `Skeleton*` 行七写 | `PaSkeleton.kt:123-194`（SkeletonSessionRow/Bubble/FileRow/CardBlock/HomeRecentRow/HostChoice）+ `ThreadsScreen.kt:2199-2232` 又自绘一套 ThreadsSkeleton* |

### 6.4 bridge/ 与 ui/ 职责渗漏 — P2 · 中置信（新代码谨慎）

`bridge/`（B4 波）**没有**碰 Compose UI 概念（无 `Modifier/Color/mutableStateOf/remember`），职责干净。但：

| 位置 | 渗漏点 |
|---|---|
| `flutterhost/FlutterHostEntry.kt:63` | `Toast.makeText(activity, "Flutter UI 入口不可用（未安装）", ...)` — **入口层直接弹 UI 反馈**，属 flutterhost 职责边界模糊（可接受：它是宿主壳，但文案硬编码） |
| `bridge/BridgeMethods.kt:103` | `"宿主内部错误($method): ..."` 中文错误文案在桥层 |
| `bridge/BridgeConnection.kt:60-84` | 12 个 `getX(request)` 每个 100+ 行，是同构样板（可用代码生成或泛型辅助收敛），B4 波**标注不建议大改** |

### 6.5 AgentConnection / BridgeConnection 方法面重复 — P2 · 标注（红线）

`AgentConnection.kt` 与 `bridge/BridgeConnection.kt` 有 **12 组同名同签名方法**（`getHostInfo/getPairingInfo/getCollabSession/getThreadHistory/getThreadEventsSince/getCollabPolicy/listCollabSessions/cancelCollabSession/cancelCollabMember/retryCollabMember/hostService/threadService/...`）——两个 RPC 门面并行维护，改一个漏一个无报错。属安全/桥接域，**只标注**。

---

## 7. 测试整洁

### 7.1 总量与分布

- test 文件 **208 个**，合 **33475 行**。
- Top：`BridgeMethodsTest.kt` 1423 / `VoiceSessionReducerTest.kt` 770 / `FlutterHostEventChannelsInstallTest.kt` 735 / `ControlPlaneLogicTest.kt` 703 / `ChatStateReducerTest.kt` 604。
- 反引号 test 名 **1569 个**（中文可读名，好）。
- `@Test` fun > 100 行：**0 个**（干净）。

### 7.2 断言无消息 — P1 · 中高置信

`assertEquals/assertTrue/...` 共 6054 处（行级），其中 **1819 处（30%）所在行零逗号**（即铁定无消息参数）。虽然有 `assertTrue("xxx 应 yyy", cond)` 的良好样本大量存在（如 `PaSheetDialogConvergenceTest.kt:67`），但 30% 无消息断言在失败时只报 `expected:<x> but was:<y>`，排查成本高。

### 7.3 共享 fixture 未收 testkit — P1 · 中高置信

**Android 侧没有 testkit 模块。** 现有共享 fixture 散落：

| 文件 | 内容 |
|---|---|
| `test/.../ui/SourceKit.kt` | 源码定位工具（读 android/app/src 下的文件） |
| `test/.../ui/TaskCenterFixtures.kt` | TaskCenter task 工厂 |
| `test/.../bridge/BridgeTestFixtures.kt` | 417 行桥 fixture |
| `test/.../ui/component/SnapshotMsgFileFixture.kt` | 快照文件 fixture |
| `test/.../ui/component/HistoryQuickNavLogicTest.kt` 等 | 各测试内部私有 factory |

重复样板实证（14 行窗口归一化）：
- `package ... + import org.junit.Assert.{assertEquals,assertFalse,assertTrue} + import org.junit.Test` 头在 **28 个文件**逐字重复
- `AgentConnectionCandidatesTest.kt:98 candidateOrderStillUsbFirst` 与 `AgentConnectionTofuAndMdnsTest.kt:268 no mdns discovery keeps legacy candidate order` **测试体逐字相同**
- `BridgeMethodsTest.kt` 内部 5 处重复 `val f = StreamFixture(); f.subscribeTurn(); val start = f.invoke(...); awaitUntil(...)`
- `setUp/tearDown` 重复 13/10 处（AgentConnection 相关测试各自 reset 单例）

B3 轮建过 testkit（任务描述提及）——**Android 侧没有同款**，建议建 `android/app/src/test/.../testkit/`（或独立 source set）收拢 `SourceKit` / fixture 工厂 / `resetAgentConnectionForTest` 样板。

---

## P0/P1 典型修复代价估算与风险（挑 3 个）

### 案例 A：ChatTimelineArea / ChatComposerArea 参数瀑布（P0）

- **位置**：`ui/chat/ChatTimeline.kt:181`（35 参 / 1093 行）、`ui/chat/ChatComposer.kt:156`（43 参 / 918 行）
- **反模式**：超长 Composable + 参数 > 5 + 回调瀑布第 4 层
- **修复方向**：按注释已声明的分段（顶栏/计划卡/子代理/终端块/消息菜单）拆成 6-8 个 composable，各自收窄参数；把 35 参数压成 `ChatTimelineSectionState`（data class）+ 少量回调
- **代价估算**：**高（3-5 人日纯机械拆分 + 1-2 日回归）**。1093 行里 13 个 `MutableState` 直传参数（`messageSelectArmed/peekSub/approvalDialogOpen/planPanelOpen/showExport/showMore/showHistoryPanel/unreadBlocks/followBottom`）——这 9 个本身就是该收敛的 State Object，先收敛再拆可降风险
- **修复风险**：**高**。① 插槽稳定性契约（`PaScreenScaffold.kt:53-64` KDoc：lambda 必须 remember 且 key=捕获值，否则每帧新分配导致无法 skip）——拆函数时漏一个 remember 就是性能回归；② 单测只覆盖 `userGroupNeighbors` 纯函数，UI 拆分无测试保护；③ `ChatTimelineArea` 是 `ColumnScope` 扩展（黑客/编辑式双骨架共用），拆分时 scope 传递易错
- **硬禁区**：不碰 `ui/theme/` token；不动 `TurnSession.State` 结构（单例语义）

### 案例 B：ChatScreen 6149 行 + 63 个 remember（P0）

- **位置**：`ui/ChatScreen.kt`（63 处 `mutableStateOf`、13 个面板开关、7 个巨型 composable）
- **反模式**：god object + 状态散落 + Compose 内 IO
- **修复方向**：① 先收敛 `ChatPanelVisibilityState`（13 个开关一个 data class）+ `ChatBusyState`（4 个 busy）；② 把 `ApprovalSheet`(418)/`ThreadGitPanel`(299)/`ExportChatSheet`(268) 移到 `ui/chat/`（与 ChatSheets 同目录）；③ 附件解码（390/442/446）移到 `remember` 或 ViewModel
- **代价估算**：**中高（2-3 日收敛状态 + 2 日搬迁 composable + 1 日回归）**
- **修复风险**：**中高**。① `remember` key 纪律不统一（§3.2：`promptOptState` 无 key vs `promptPreview` 有 key）——收敛成 State Object 时**必须保留原有 key 语义**，否则引入「换会话状态不重置」的新 bug；② 已有 `ChatState`/`ChatViewModel` 投影层（2026-10-09 效果层外移进行中），再抽一层要避免双向真值来源；③ 6149 行里大量 2026-09/10 日期注释记录决策，搬迁时注释要跟着走
- **硬禁区**：不动 `AgentConnection` 安全层；不碰审批三态机（`EditorialApproval`/`PaApprovalDialog` 有测试钉死）

### 案例 C：SheetDialogButton 四写 + tab 芯片五写（P1）

- **位置**：`SettingsScreen.kt:4553` / `RuntimeSection.kt:1506` / `GitScreen.kt:1134` / `UnknownEventDetailSheet.kt:658`（四份 27 行同构）；`CollabScreen.kt:1081` / `ContextUsageCard.kt:304` / `GitScreen.kt:891` / `TaskCenterScreen.kt:595`（四份 tab 芯片）
- **反模式**：重复 Composable 变体
- **修复方向**：`SheetDialogButton` 提升到 `ui/component/`（与 `PaApprovalDialog.ApprovalButton` 合并或并列）；tab 芯片统一为 `PaChoiceChip(label, selected, onClick)`
- **代价估算**：**低（0.5-1 日）**
- **修复风险**：**低-中**。① 四处 `SheetDialogButton` 的色调参数已各自定义（`SettingsSheetButtonTones` 等 data class），合并时要保留 secondary/danger 两态；② `PaSheetDialogConvergenceTest.kt` 用**源码字符串包含断言**（`src.contains("PaSheet(onDismissRequest = onDismiss, centered = true)")`）——重构后这些测试会红，需同步改断言；③ 视觉回归无自动化测试，靠人眼
- **硬禁区**：`ui/theme/` token 数值不动（按钮色调取槽位，只改结构不改色值）

---

## 8. 与仓库审核约定（AGENTS.md / AUDIT.md / DEVELOPING.md）的对账

本审查是**纯只读审查**，按仓库工作流约定自证如下：

| 约定项 | 本审查的状态 |
|---|---|
| **改动分级**（AUDIT §1） | 丁级（仅新增 `docs/reviews/2026-10-17/clean-android.md` 一份 md），无代码/资源改动 |
| **测试水位不许倒退** | 未跑测试、未改代码，Android 2002 用例 / daemon 30 包水位不受影响（本文件不声称任何新测试结论） |
| **不留假绿** | 本文件所有数字来自脚本实扫 + 逐条人工复核（见 §2.2 误报移除记录、§2.6 误报校正），未使用 Gradle/Go 结果 |
| **红线一票否决项** | 未碰 proto 双副本；未碰 `ui/theme/` token 数值；未碰 AgentConnection TLS/pin/keystore/token 层；未伪造任何审批 decision；未改写用户桌面配置 |
| **缺陷即测试**（AUDIT §4） | 本文件是债务清单不是修复，**未附回归用例**；任何后续按本清单动手的修复必须按丙类及以上补 JVM 单测 |
| **真值锚点漂移**（AUDIT §3.2 提示项 5） | 未改 DEVELOPING.md §0.3；若后续按本清单改造并影响测试数/文件行数基线，需同步该表 |

**与 DEVELOPING.md §0.3「挂账未核实」三项的关联**（本审查的新证据）：

| 挂账项 | 本审查的发现 |
|---|---|
| ① FileKind 枚举快照 Mcp 色不随换盘 | 未复扫（属 theme 运行时序问题，非本次整洁框架） |
| ② SettingsScreen statusLabel 收口四词口径 | **部分坐实**：`statusLabel` 在 3 个文件重复定义（`GoalDisplayLogic.kt:25` / `SettingsScreen.kt:2795` / `SubAgentsState.kt:113`），见 §1.5「同一概念多个名字」——收口确有必要，且应在收口时一并统一四词口径 |
| ③ BrowserScreen 离线快照 toast 带"截至 X" | 未复扫 |


## 附：量化口径与工具

| 脚本 | 用途 |
|---|---|
| `analyze.py` | fun/composable 块长、参数个数、文件行数（`funcs.json` 输出） |
| `patterns.py` | 通用正则扫描框架（mutableStateOf/LaunchedEffect(Unit)/TODO/catch/println/runCatching） |
| `scan3.py` | 噪声类名 / 布尔参数 / getter 名 / 注释代码 |
| `scan7.py` | 中文标识符 / Toast 分布 |
| `scan11.py` | 测试重复块（N 行窗口归一化） |
| `scan19*.py` / `scan_nest*.py` | composable 嵌套深度（多轮修正，最终用 brace 匹配 + expression-body 处理） |
| `callbacks.py` / `rootnav.py` | 回调链与 Screen 签名 |

**已知工具局限（最终口径）**：
1. **表达式体 fun 陷阱（本审查最大的坑，已修正）**：`fun x(...) = expr` 没有 `{`，brace 匹配会把后续 fun 并进它的 body → 行数虚高（最高虚到 1108 行）。修法：无 `{` 即认为函数体到下一个顶层声明为止。§2.1/§2.2 表格均已按修正后口径呈现，误报记录保留在 §2.2。
2. **composable 长度含 KDoc**：从 `fun` 行量到闭合 `}`，KDoc 计入，报告值为上界（真实代码体略短）。
3. **参数计数**：默认值为 lambda 的（`onX: () -> Unit = {}`）按 1 个计；`ColumnScope.` / `LazyListScope.` 接收者不计入。
4. **嵌套深度**：最终口径 = col-N 闭合括号 + 表达式体单行；结论「全部 ≤2 层，非债务」（§2.6）。
5. **when 分支数**：按 `X ->` 行计数，`when { 条件 -> }` 与 `when(x) { 常量 -> }` 未区分，分支数为近似值。
6. **注释掉的代码**：靠行首关键词启发式，只能证「没有大段」不能证「绝对没有」；实际命中 5 处全是说明文字。
7. **测试重复块**：14 行窗口归一化，只能抓到逐字相同的块；参数不同但结构相同的样板（如 5 处 tab 芯片）靠人工归组，见 §6.3。