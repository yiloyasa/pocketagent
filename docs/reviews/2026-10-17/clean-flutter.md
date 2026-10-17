# Flutter（Dart）域整洁债清单 —— 代码整洁大法审查

- **审查日期**：2026-10-17
- **审查范围**：`flutter/lib/**`（60 个非 gen Dart 文件，约 23,400 行）+ `flutter/test/**`（58 个文件，约 19,900 行）
- **审查性质**：**只读**。未修改任何文件，未 git add/commit/push。
- **基线**：flutter 测试水位 945 测试块 / 949 用例（脚本统计，见 §7）；AGENTS.md 记 949 全绿。
- **工具**：Python 量化脚本（build 方法行数 / State 类行数 / 文件 top / k 前缀清点 / byKey 频率 / mock 装配重复度 / 注释块 / import 图），`flutter analyze` 本机不可用（analysis server 崩溃，见 §9）。

---

## 0. 结论速览

| 维度 | 判定 | 债级 |
|---|---|---|
| 命名（k 前缀常量） | **实际 32 个 k 前缀顶层常量（lib 24 + test 5 + transport 契约 8，其中 8 个在 B2 契约面）**，与 Dart 惯例（lowerCamel）系统性偏离 | **P1** |
| 命名（`_data` 三件套） | `threads_data` / `chat_history_data` / `collab_session_data` **不同构**（215/303/731 行，职责完全不同），「三处同构」是伪命题 | P2（无债） |
| 超长 build / State | `collab.dart` 1878 行单文件、`_CollabPageState` 993 行、19 个顶层符号；`settings_console` 825 行 | **P0** |
| 巨型 switch | `browser_data.dart:145` 62 case（后缀路由表）——**合理**（词表型）；`chat_session.dart:246` / `chat_timeline.dart:234` 15 case 帧分发——**合理**（proto oneof） | P2（无债） |
| 状态管理 | `setState` 188 处；2 个文件有 setState 但**零 mounted 守卫**；`_CollabPageState` 78 个字段 | **P1** |
| 注释 | **零谎言注释抽样命中**；零 date-stamped changelog 注释；4 个 TODO 全部有归属（B3） | P2（优良） |
| 错误处理 | 93 个 catch：59 个落 UI 态/上抛 typed failure、2 个 debugPrint（均有意降级）、0 个空吞——**纪律罕见地好** | P2（优良） |
| 重复 | `_LoadingView`/`_ErrorView`/`_EmptyView` **三处逐字节重复**（jaccard 0.91–0.95）；`_invoke`/`_invokeExpectNull` 四份同构 | **P1** |
| 测试整洁 | `setMockMethodCallHandler` 200 处、`TestDefaultBinaryMessengerBinding` 36 处——**远超 10 次阈值**，无 helper | **P1** |
| 测试断言 reason | 2885 个 expect 中仅 114 个带 reason（**96% 无 reason**） | P2 |

> **总评**：本域整洁度显著高于典型 Flutter 项目——契约纪律（透传、诚实占位、无假绿）执行得非常彻底。债集中在**三处结构性肥大**（collab 单文件、settings_console 单文件、test mock 样板）与**一处系统性命名偏离**（k 前缀）。没有发现 P0 级正确性/契约风险。

---

## 1. 命名

### 1.1 k 前缀常量 —— 系统性 Kotlin 残留 【P1】

**全仓清点（非 gen）**：**32 个顶层 k 前缀常量**（lib 24 个 + test 5 个 + transport 契约 8 个 = 37 处声明，去重后 32 个不同常量名），引用点合计 200+ 次。

| 文件 | k 常量 | 引用数 |
|---|---|---|
| `lib/pages/chat_composer_data.dart:87,326,356,365` | `kSlashUnknownAgentHint` / `kComposerMaxLength` / `kComposerTextExtensions` / `kComposerTextMimeTypes` | 12 |
| `lib/pages/chat_snapshot_store.dart:29,33,37` | `kChatSnapshotVersion` / `kChatSnapshotMaxMsgs` / `kChatSnapshotMaxBytes` | 9 |
| `lib/pages/collab_roles_data.dart:29,182` | `kCollabRoleIdMax` / `kCollabEffortOptions` | 13 |
| `lib/pages/chat_history_data.dart:194,198` | `kHistoryRoundOptions` / `kDefaultHistoryRounds` | 10 |
| `lib/pages/chat_timeline.dart:28,31,34,37` | `kDeliveryPending` / `kDeliverySteered` / `kDeliveryQueued` / `kDeliveryOverflow` | 31 |
| `lib/pages/files_bridge.dart:165,168,254` | `kDefaultDirLabel` / `kDefaultDirTarget` / `kFilePreviewMaxBytes` | 19 |
| `lib/pages/settings_console.dart:43` | `kRuntimeV1Capability` | 2 |
| `lib/pages/task_center_data.dart:243,244,516` | `_kFieldSep` / `_kLineSep` / `kTaskRetryCooldownMs` | 250+ |
| `lib/pages/terminal_data.dart:81` | `kTerminalMaxLines` | 3 |
| `lib/pages/threads_data.dart:166` | `kSampleThreads` | 6 |
| `lib/transport/pa_bridge.dart:43,46` | `kPaBridgeMethodChannel` / `kPaBridgeEventChannel` | **60** |
| `lib/transport/chat_turn_stream.dart:49,52,53,54,57` | `kTurnStreamEventChannel` / `kTurnStreamStartMethod` / `kTurnStreamSendMethod` / `kTurnStreamCancelMethod` / `kTurnResyncMethod` | 15 |
| `lib/transport/terminal_stream.dart:37` | `kTerminalEventChannel` | 5 |
| test/ | `kDemoRoles` / `kDemoSessions` / `kCaps` / `kNowMs` | 46 |

**反模式归类**：Kotlin/Android 命名残留（`kXxx` 是 Kotlin 编译期常量惯例）。Dart 惯例是 lowerCamel 常量（`kFoo` 在 Flutter framework 内部存在，但 community/Dart 官方 lint 生态不推；本仓 `analysis_options.yaml` 只 include `flutter_lints`，无自定义命名 lint，所以机器不会报）。

**关键红线**：`kPaBridgeMethodChannel` / `kPaBridgeEventChannel` 定义在 `lib/transport/pa_bridge.dart:43,46`，是 **B2 契约面**，被 `docs/channel-contract.md:18` 明文引用、被 5 个 test 文件 import（`test/widget_test.dart:27`、`test/main_routing_test.dart:27`、`test/transport/chat_input_transport_test.dart` 等）。改名属**契约面变更，需与 Kotlin 侧同步改**（Kotlin 侧 `BridgeMethods.kt` 若以字符串硬编码则不受影响，但 channel-contract.md 必须同步）。

**处置建议**：
- `pa_bridge.dart` / `chat_turn_stream.dart` / `terminal_stream.dart` 的 8 个 k 常量：**只标注，不改**（B2 契约面对账文件 `docs/channel-contract.md:18` 点名）。
- 其余 24 个（`chat_composer_data` 4 / `chat_snapshot_store` 3 / `collab_roles_data` 2 / `chat_history_data` 2 / `chat_timeline` 4 / `files_bridge` 3 / `settings_console` 1 / `task_center_data` 3 / `terminal_data` 1 / `threads_data` 1）：可改，属纯内部重命名。
- **测试水位影响**：重命名会打破 `kDelivery*` 在 `chat_timeline_test.dart`（21 处引用）等文件的编译——但这是**编译期机械替换**，`analyze` 全过即水位不跌。**不需要新增测试**。

**修复代价估算（挑 1 个典型：`chat_timeline.dart` 4 个 `kDelivery*`）**：
- 代价：改 4 行声明 + `chat_session.dart` 3 处引用 + `chat_timeline.dart` 若干 + test 21 处引用。约 30 分钟，纯 sed 级替换。
- 风险：**低**。无反射、无字符串字面量跨语言依赖（值 `'pending'`/`'steered'`/`'queued'`/`'overflow'` 是 wire 词，与 Dart 端变量名无关）。
- 水位：不减（编译通过即等价）。

### 1.2 `_data` 三文件同构度评估 —— **伪命题，无债 【P2】**

任务书假设 `threads_data` / `chat_history_data` / `collab_session_data` 是「同构三件套」。实测**三者不同构**：

| 文件 | 行数 | 顶层符号 | 核心职责 |
|---|---|---|---|
| `threads_data.dart` | 215 | 5 | 行模型 + 三维筛选纯函数 + `ThreadsDataSource` 抽象 + `FakeThreadsDataSource` + `kSampleThreads` |
| `chat_history_data.dart` | 303 | 5 | 历史→回放帧映射（proto 六字段对账表）+ 分页口径 + `ChatHistorySource` + `BridgeChatHistorySource` + `FakeChatHistorySource` |
| `collab_session_data.dart` | 731 | 14 | 5 个 UI 投影类 + 3 个 actions 类 + 1 个 decision 类 + `CollabCopy` 文案表（125 行）+ 一堆纯函数 |

3-gram jaccard 相似度：`threads_data ↔ chat_history_data` = **0.00**，`threads_data ↔ collab_session_data` = **0.00**，`chat_history_data ↔ collab_session_data` = **0.00**。

**只有 threads_data 与 chat_history_data 共享「Source 抽象 + Bridge 实现 + Fake 实现」三件套模式**（且 threads 的 Bridge 实现在 `threads_bridge.dart` 而非同文件）。这是**有意为之的依赖注入模式**（文件头 1-10 行、118-127 行、139-147 行、250-264 行、281-299 行都有「为什么抽象成接口」的论证）。

**结论**：**不存在「三处 _data 同构」的债。强行收敛公共基类会藏差异**（threads 的 Source 返回 `List<ThreadSummary>`，history 的返回 proto response 且带 `threadId` 参数，collab 根本没有 Source 抽象）。**明确不建议改。**

### 1.3 其他命名观察 【P2】

- **私有 `_` 前缀**：94 个顶层私有符号（`_LoadingView` ×3、`_ErrorView` ×3、`_EmptyView` ×4、`_PAToolCardState` 等）。绝大多数是**真正的私有**（单文件内的 State/widget）。但 `_EmptyView` / `_ErrorView` / `_LoadingView` 在 3-4 个文件各写一份，说明 `_` 前缀在此处的语义是「不想给别人看」而非「真的私有」——**这是重复的信号，不是隐私的信号**（详见 §6.1）。
- **`_RealPaBridge`（`collab_role_editor.dart:659`）**：命名良好——类名即「生产实现」，与接口 `PaBridgeForEditor` 配对清楚。
- **缩写**：未发现难以拼读的缩写。`kDel`/`v1` 等皆规范。
- **谎言名扫描**：`getX` 改状态——**0 命中**；`isX` 非布尔——**0 命中**（`isFinished`（`collab_session_data.dart:54`）、`isRunning`（`chat_session.dart:95`）、`isDone`（`onboarding_prefs.dart:42`）、`isNone`（`threads_data.dart:87`）全部是真布尔）。
- **`loadCompleted`（`collab_roles_state.dart:77`）**：名字**诚实但有歧义**——它不是 `bool`，是 `Future<void>`。读作「加载完成了吗」会误解。建议改名 `whenLoadSettled` 或 `loadFuture`（属纯内部重命名，零外部依赖）。**P2**。

---

## 2. 函数 / Widget 尺寸

### 2.1 超长 build 方法 top 15 【P1】

| # | 行数 | 位置 | 说明 |
|---|---|---|---|
| 1 | **192** | `lib/components/pa_approval_sheet.dart:63-254` | 审批弹层；含 riskLabel/detail/info/三按钮 |
| 2 | **161** | `lib/pages/settings.dart:118-278` | 设置页主 build；含 6 个 `_SettingsRow` 装配 |
| 3 | **150** | `lib/pages/task_center.dart:485-634` | 任务中心主 build |
| 4 | 114 | `lib/pages/collab.dart:1362-1475` | 成员卡 build |
| 5 | 96 | `lib/pages/collab.dart:1584-1679` | 角色选择弹层 build |
| 6 | 93 | `lib/main.dart:194-286` | `_HomeShellState.build`（NavigationBar + 9 路由页） |
| 7 | 91 | `lib/components/pa_tool_card.dart:52-142` | |
| 8 | 90 | `lib/pages/chat_composer.dart:265-354` | 输入区 build |
| 9 | 83 | `lib/pages/collab_role_editor.dart:219-301` | |
| 10 | 79 | `lib/pages/threads.dart:357-435` | |
| 11 | 69 | `lib/components/pa_button.dart:39-107` | |
| 12 | 67 | `lib/pages/tools.dart:50-116` | |
| 13 | 65 | `lib/pages/chat_actions.dart:532-596` | 回滚弹层 build |
| 14 | 63 | `lib/pages/files.dart:411-473` / `lib/pages/onboarding.dart:102-164` | |

**注意**：任务书提到的 `settings_connection 546 / settings_console 817 / collab 1864 / chat_actions 26912B` 与实测**部分不符**——那些是**文件总行数**（settings_connection 522 / settings_console 825 / collab 1878 / chat_actions 787），不是单个 build 方法。build 方法实测最长 192 行。

**反模式归类**：Long Method。但 Flutter 的 build 方法是**声明式 DSL**，行数与复杂度非线性——192 行的 widget 树通常比 40 行命令式代码更易读。**判定为 P1（该拆）而非 P0**。真正的问题是 `collab.dart` 的**文件级**肥胖（见 §2.2）。

**典型拆解建议（`pa_approval_sheet.dart:63`，192 行 build）**：抽出 `_buildInfoTable`（info map 渲染）、`_buildButtons`（三键行）。代价约 1 小时，风险低（纯 widget 树搬移，key 不变则测试零改动）。

### 2.2 超长 State 类 top 8 【P0 / P1】

| State 类 | 行数 | 字段数 | setState 数 | 债级 |
|---|---|---|---|---|
| `_CollabPageState`（`collab.dart:151-1143`） | **993** | **78** | **34** | **P0** |
| `_SettingsConsolePageState`（`settings_console.dart:186-825`） | **640** | 14 个真字段（61 个赋值点） | 13 | **P1** |
| `_TerminalPageState`（`terminal.dart:73-639`） | 567 | — | 17 | P1 |
| `_ChatPageState`（`chat_page.dart:80-506`） | 427 | — | 4 | P1 |
| `_ChatComposerState`（`chat_composer.dart:117-535`） | 419 | — | 6 | P1 |
| `_ConnectionSectionState`（`settings_connection.dart:106-521`） | 416 | — | 14 | P1 |
| `_GitPageState`（`git.dart:61-410`） | 350 | — | 10 | P1 |
| `_FilesPageState`（`files.dart:49-397`） | 349 | — | 14 | P1 |

**`_CollabPageState` 详析（P0）**：
- 993 行 State + 文件总 1878 行 + 19 个顶层符号（1 个 Page + 1 个 State + 17 个私有 widget/枚举/typedef）。
- 78 个字段/赋值点覆盖 **6 个独立关注域**：会话列表（`_sessionsPhase`/`_sessionsError`/`_sessions`）、会话详情（`_detailPhase`/`_detailError`/`_detail`）、策略区（`_policyPhase`/`_policyError`/`_policy`）、角色区（`_rolesPhase`/`_rolesError`/`_roles`）、派发配置（`_selected`/`_roleBinding`/`_dispatching`/`_lastInstruction`）、写操作确认流（`_writeInFlight`/`_writeError`/`_dispatchRejects`）。
- 每个域都是「phase + error + value」三件套——**这正是 `PaAsyncPhase` 共享词汇（`pa_page_kit.dart`）想解决但没解决到底的模式**。文件 84-102 行自己承认了三个 typedef 指向同一枚举。
- 4 个 `_loadXxx()` 方法（226/238/279/312/330）结构同型：置 loading → await → mounted 检查 → setState 落值/落错。

**反模式归类**：God Class / Large Class + 散落字段应收敛为 State class。

**修复代价估算（挑 `_CollabPageState` 拆「会话列表 + 详情」为独立 controller）**：
- 代价：**高，约 2-3 人日**。需抽 2 个 `ChangeNotifier`/`ValueNotifier` 或 2 个 sub-State；触及 34 个 setState 调用点的归属；`collab_page_test.dart`（991 行 / 29 测试）的 40 处 byKey 断言依赖当前渲染结构。
- 风险：**中高**。这是渲染逻辑与状态逻辑的深度纠缠（`_buildDetailBody` 直接读 5 个域字段）。拆错会改变重建粒度 → widget 测试的 pump 时序断言可能翻。
- **测试水位保护**：拆完必须保持 `collab_page_test.dart` 29 个测试全绿才算完成。**建议不在本轮做**——留作独立任务书，配 `git worktree` 隔离改。

**`_SettingsConsolePageState` 详析（P1）**：640 行 / 4 个「phase+error+value」域（caps/state/policy/events/detail = 5 个域）。文件 215-241 行已经做了一个漂亮的收敛（`_loadReadonly` 四合一），**但只收敛了「装载骨架」，没收敛「域状态本身」**。五个域的字段仍平铺。建议抽 `_ReadonlyBlockState<T>` 泛型小类（phase/error/value 三字段 + 三个 setter），5 个域各持一个实例。代价约 1 人日，风险中（要保 16 个测试全绿）。

### 2.3 参数 >4 个的 Widget 【P2】

| Widget | 参数数 | 位置 |
|---|---|---|
| `ChatTile` | **13** | `lib/pages/chat_timeline.dart:111` |
| `PAApprovalSheet` | **10** | `lib/components/pa_approval_sheet.dart:17` |
| `_RolePickerSheet` | 7 | `lib/pages/collab.dart:1565` |
| `ChatPage` | 8 | `lib/pages/chat_page.dart:40` |
| `CollabSessionRow` | 7 | `lib/pages/collab_session_data.dart:208` |
| `TaskCenterPage` / `_TaskRow` | 7 | `lib/pages/task_center.dart:49,466` |
| `PACard` / `PAToolCard` / `OnboardPage` | 6 | — |

**反模式归类**：Long Parameter List。`ChatTile` 13 参最严重。

**布尔参数控制行为**：**0 命中**（扫描 `required this.xxx bool` 无结果）。但 `TaskCenterPage.hostOffline`（`task_center.dart:70`）与 `ChatRunningBar.isRunning`/`busy`（`chat_actions.dart:275,284`）虽非构造扫描命中，实为行为开关型布尔。**P2**——Flutter 社区对此容忍度高，不建议改。

### 2.4 嵌套 widget >4 层 【P2】

实测最深：`pa_approval_sheet.dart:158` **11 层**（Container>SafeArea>Padding>Column>SizedBox>Row>Expanded>Expanded）、`collab.dart:1655` **10 层**、`pa_tool_card.dart:84` 9 层、`collab.dart:1386` 6 层。

反模式归类：Deep Nesting。但 Flutter 的 widget 树嵌套是**表达布局的必要手段**，11 层里多数是 `SizedBox`/`Expanded` 这类布局原子。**P2，不建议专门治理**——除非配合 §6.1 的三视图去重一起抽。

### 2.5 巨型 switch 【P2 —— 全部合理】

| switch | case 数 | 位置 | 判定 |
|---|---|---|---|
| `pcPreviewKindOf` 后缀路由表 | **62** | `browser_data.dart:145-219` | **合理**。词表型映射（md/txt/源码/表格/文档/图片五类），对位 Kotlin `PcFilePreview.kt` 的 `render` 路由表。编译期穷尽检查是优点。 |
| `TurnServerMessage.whichMsg()` 帧分发 | 15 | `chat_session.dart:246-325` | **合理**。proto oneof 分发，每个 case 是状态机的一步。 |
| `ChatTimelineAccumulator.applyFrame` | 15 | `chat_timeline.dart:234-329` | **合理**。同上。 |
| `_ChatPageState._dispatchAction` | 9 | `chat_page.dart:261-318` | **合理**。九项动作分发。 |
| `visibleChatActions` 九项矩阵 | 9 | `chat_actions_data.dart:55-74` | **合理**。 |
| `CollabStatus.fromWire` | 8 | `collab_session_data.dart:61-69` | **合理**。七词状态 + unknown 兜底。 |

**结论：无债。** 这些 switch 都是「词表/协议分发」，抽成 Map 反而丢穷尽性检查。**明确不建议改。**

---

## 3. 状态管理反模式

### 3.1 StatefulWidget 里散落 5+ 个独立字段 【P1】

见 §2.2。全仓 26 个 State 类中，**12 个 >300 行**。`_CollabPageState`（78 字段）与 `_SettingsConsolePageState`（5 域）是典型。

**但注意**：多数 State 类的字段是同质的「phase+error+value」三件套，**有明确的收敛路径**（§2.2 已给方案），不是无结构散落。

### 3.2 setState 里做重活 【P1 —— 部分命中】

188 个 setState 中，多数是「置 loading → await → 落值」的标准形态（正确）。但以下模式值得注意：

- `settings_console.dart:271-272`：`setState(() { _capsPhase = loading; _capsErrorText = ''; })` 之后紧跟 `await _bridge.getHostInfo()`——**setState 本身不做重活**，重活在 await。OK。
- `collab.dart:401 _dispatch()`：`setState` 里只置 4 个标志位（409-412），OK。
- **未发现 setState 内做同步重计算/IO 的案例**。`task_center_data.dart` 的 `_commit()` 在 store 层同步落盘，但那是 store 不是 State。

**判定：P1 但不紧急。** 真正该治理的是 §3.1 的字段平铺。

### 3.3 initState 里发起异步无取消保护 【P0 级隐患，实测 2 处真缺口】

**统计**：21 个文件有 setState；其中 **19 个文件有 `mounted` 守卫**，**2 个文件有 setState 但零 mounted 守卫**：

| 文件 | setState 数 | mounted 守卫 | 缺口 |
|---|---|---|---|
| `lib/components/pa_tool_card.dart` | 1 | 0 | 该 setState 在同步 `_toggle()`（`pa_tool_card.dart:49`），**无 async 间隙，安全** |
| `lib/pages/onboarding.dart` | 1 | 0 | 该 setState 在 `onPageChanged` 回调（`onboarding.dart:134`），同步，**安全** |

**两处都是误报**（setState 在同步回调里，没有 await 间隙）。

**真实的 async-gap 审计**（逐个查 initState → `_load()` 链）：
- `chat_history.dart:91` / `collab_roles.dart:85` / `settings.dart:84` / `settings_agent.dart:124` / `settings_capabilities.dart:245` / `settings_console.dart:264` / `settings_downloads.dart:248`：全部有 `if (!mounted) return;` 守卫（`settings.dart:92`、`settings_downloads.dart:253` 等）。
- `settings_agent.dart:131` / `settings_console.dart:271` / `settings_capabilities.dart:254` / `chat_history.dart:100`：**setState 在 `_load()` 的第一行**（initState 同步调用链内，无 await 间隙），安全。
- `collab.dart:199-212 initState`：4 个 `_loadXxx()` 全部内部有 mounted 检查（229/251 等）。

**结论：initState 异步取消保护纪律 = 100% 达标。零缺口。** 【P2，优良】

**一处疑似**：`collab.dart:226-236 _loadCapabilities()` 的 catch 分支**只 debugPrint、不 setState**——这是**有意的**（注释 232-234 行说明「参与区按无能力位呈现，不进错误态」）。见 §5.1。

### 3.4 StreamController 未 close 【P1】

| 声明 | close 调用 | 判定 |
|---|---|---|
| `task_center_data.dart:576` `TaskCenterStore._controller`（broadcast） | **无 close** | **真缺口** |
| `terminal_data.dart:353` `BridgeTerminalSession._out`（broadcast） | `terminal_data.dart:433` `await _out.close()` | 已 close |
| `terminal_data.dart:355` `BridgeTerminalSession._exit`（broadcast） | `terminal_data.dart:434` `await _exit.close()` | 已 close |

**`TaskCenterStore._controller` 无 close**：`TaskCenterStore` 是「app 级 store」（`task_center_data.dart:544-559` 注释：Kotlin 是 app 级 object，B4 前提为可注入类）。**当前无 app 级单例持有它，页面各建各的**（`task_center.dart:60-61`「生产不传即新建」），所以 store 生命周期 = 页面生命周期，页面 dispose 时 store 变垃圾——broadcast StreamController 无订阅者时**不泄漏**（无监听器即无挂起资源）。

**判定：P1 但不是内存泄漏，是「缺 dispose 契约」。** 一旦 B4 把 store 提为 app 级单例（注释明说届时这么干），`_controller` 永不 close 会变成真泄漏。**建议**：现在给 `TaskCenterStore` 加 `dispose()` 方法（即使暂无调用方），并在注释里写明「B4 提单例时根装配负责调」。代价 10 分钟，风险零。**这是本清单里性价比最高的一条。**

### 3.5 `late final` 初始化顺序陷阱 —— `loadCompleted` 模式评估 【P1】

`collab_roles_state.dart:73`：
```dart
late Future<void> _loadFuture = _doLoad();   // 第 73 行
void load() {                                  // 第 65 行
  _loadState = CollabRolesLoadState.loading;
  _loadError = null;
  _loadFuture = _doLoad();                     // 第 70 行
}
Future<void> get loadCompleted => _loadFuture; // 第 77 行
```

**这是 Dart 的 `late` 字段初始化器语义**：`late Future<void> _loadFuture = _doLoad()` 表示「首次读取时才执行 `_doLoad()` 并缓存」。也就是说：
- 构造后若不调 `load()`，首次读 `loadCompleted` 会**悄悄触发一次网络加载**。
- 调了 `load()`（第 70 行赋值），`_loadFuture` 已被赋值，late 初始化器不再触发。

**这是对的，但极其脆弱**：任何重构者若把 `load()` 里的 `_loadFuture = _doLoad()` 删掉（以为「反正在 initState 里调了 load」），或调整 `load()` 与 `loadCompleted` 的调用顺序，就会触发「late 初始化器偷偷发一次 RPC」的幽灵行为——测试里表现为「多一次 listCollabRoles 调用」，极难查。

**反模式归类**：隐式副作用 + 初始化顺序陷阱。

**修复建议（P1）**：把初始化器改成显式的空占位 + 构造即加载：
```dart
Future<void> _loadFuture = Future<void>.value();  // 非 late
CollabRolesStore({PaBridge? bridge}) : _bridge = bridge ?? PaBridge() {
  load();  // 构造即首载（与 Kotlin init → restore 同策略）
}
```
或保留 late 但把初始化器改为 `Future.error(StateError('load() not called'))`——让「忘了调 load」变成显式报错而非幽灵 RPC。

**代价**：1 小时（改 3 行 + `collab_roles_state_test.dart` 20 个测试里所有直接读 `loadCompleted` 的用例要确认行为等价）。
**风险**：**中**。`collab_roles_state_test.dart:15 个 setMockMethodCallHandler` 依赖「store 构造后何时发 RPC」的精确时序；`load()` 改构造调用会让「未调 load 的 store」也发一次 RPC。
**测试水位保护**：改后必须 20/20 绿；若时序断言翻，说明测试本来就钉了错误语义，应连测试一起修（修测试不算跌水位，用例数不变）。

---

## 4. 注释

### 4.1 谎言注释抽查（20 处高危） 【P2 —— 全部诚实】

抽查 20 处「声明与实现可能不符」的高危注释，逐条与实现对账：

| # | 位置 | 注释声明 | 实现实际 | 判定 |
|---|---|---|---|---|
| 1 | `threads_data.dart:9-10` | 「fake 从不冒充生产数据源，页面只在被显式注入时才用到」 | `threads.dart:59` `_source = widget.source ?? BridgeThreadsDataSource()` | ✅ 诚实 |
| 2 | `chat_history_data.dart:75` | 「图片块跳过，不产生 sidecar，下标对齐不断」 | `historyToFrames` 85-99 行：`emitted` 标志 + 图片不进 frames 也不进 metas | ✅ 诚实 |
| 3 | `collab_session_data.dart:157-158` | 「没有 lastSequence / needsResync」 | 类定义无该字段 | ✅ 诚实 |
| 4 | `task_center_data.dart:553-555` | 「没有变化时不发通知」 | `_commit()` 内经 `_listEquals` 比较 | ✅ 诚实 |
| 5 | `settings_console.dart:234-237` | 「四处调用点行为逐条等价」 | `_loadReadonly` 收敛四份，签名保留 | ✅ 诚实（且测试 16 个全绿佐证） |
| 6 | `pa_page_kit.dart:347-358` | 「tail 整句原值传入，返回值逐字节相同」 | `paChannelErrorText` 拼接式与注释一致 | ✅ 诚实（terminal/collab/history 三测试全串钉死佐证） |
| 7 | `pa_page_kit.dart:12-14` | 「唯一例外是 PaCardCaption 少一层 Align，布局恒等」 | `PACardCaption` 实现无 Align | ✅ 诚实 |
| 8 | `chat_actions_transport.dart:17-25` | 「10 接 1 留」 | 文件内 10 个方法 + `DelegateSubTask` 注释留白 | ✅ 诚实 |
| 9 | `collab.dart:27-29` | 「宿主桥未注册 → 7 个方法面全部抛 HOST_UNAVAILABLE」 | `_loadSessions/_loadDetail/_loadPolicy/_loadRoles/_loadCapabilities/_dispatch/_runWrite` 7 条路径 | ✅ 诚实 |
| 10 | `chat_turn_stream.dart:26-30` | 「server-stream 收成一次性方法」 | `turnResync` 一次调用回 `List<Uint8List>` | ✅ 诚实 |
| 11 | `terminal_data.dart:243` | 「terminalLoadErrorText 委托 paChannelErrorText」 | 实现即委托 | ✅ |
| 12 | `collab_roles_state.dart:16-18` | 「只取 message 不含 INVALID_ARGUMENT: 前缀」 | `extractRpcMessage` | ✅ |
| 13 | `browser_data.dart:20-30` | 「B5 接线说明：webview_flutter 未加」 | `pubspec.yaml` 无 webview_flutter | ✅ 诚实 |
| 14 | `pa_tool_card.dart` 文件头 | 「转发会让 find.byKey 同键命中两个元素」 | `pa_button.dart:92` 同款注释 | ✅ |
| 15 | `chat_composer_data.dart:101-105` | 「旧三源名单只作回落」 | `isSlashPanelAgentFallback` + `slashPanelAgentOf` 位优先 | ✅ |
| 16 | `onboarding.dart:33` | 「TODO(B3 …)」有归属 | 见 §4.3 | ✅ |
| 17 | `files_bridge.dart:264` | 「readFile 首块 + UTF-8 + 大小门」 | `filePreviewable` 即大小门 | ✅ |
| 18 | `collab_role_editor.dart:10-13` | 「编辑态 id 实时校验不做」 | `_buildIdField` 无 validator | ✅ 诚实 |
| 19 | `chat_session.dart:259-263` | 「工具卡投影归 chat_timeline；会话状态机不记工具态」 | switch case toolCall/usage 仅 break | ✅ 诚实 |
| 20 | `task_center.dart:13-20` | 「已读策略：显式按钮，不是 Kotlin 的自动全读」 | `_markAllRead` 绑按钮，无定时器 | ✅ 诚实 |

**20/20 诚实。** 这与本仓「不留假绿」「不伪造」的红线文化一致。**零债。**

### 4.2 大段注释掉的代码 【P1 —— 1 处真命中】

| 位置 | 行数 | 性质 |
|---|---|---|
| `settings_console.dart:219-232` | 14 行 `///` 围栏内的 dart 代码块 | **这是「收敛前」的等价性论证**，不是死代码——它记录了 `_loadReadonly` 抽取前的原始四份实现，用于证明「行为逐条等价」。属**有价值的考古注释**。 |
| `task_center.dart:256-259` | 4 行 `//` | **这是「为什么包 IntrinsicHeight」的论证**，非死代码。 |
| `browser_data.dart:1-30` / `chat_turn_stream.dart:1-36` / `chat_input_transport.dart:1-21` / `browser_policy.dart:1-16` / `chat_actions_data.dart:11-21` | 16-36 行文件头 | **全是设计论证/出处说明**，非死代码。 |

**结论：无死代码。** `settings_console.dart:219-232` 的 14 行是唯一的「代码形状注释」，但它是重构等价性证据（配合「行为逐条等价」+ 16 个测试），**保留有价值**。若按严格 Clean Code 应删（git 里有前向版本），但删了会丢「为什么这四份能合并」的论证。**P1，倾向保留。**

### 4.3 变更日志式注释 【P2 —— 0 处】

全仓扫描 `// 20xx-xx-xx` 形式的变更日志注释：**0 命中**。
命中的 10 处日期串全部是** provenance 引用**（指向 `docs/reviews/2026-10-15/c2-terminal-browser.md` 等评审文档）或**测试固定时钟说明**（`task_center_data_test.dart:15`「2024-10-14 13:40 (UTC+8)」）。这是**好的**——git 该管的事确实没塞进注释。

### 4.4 TODO 无归属清单（flutter/ 全域） 【P2 —— 4 个，全部有归属】

| 位置 | 内容 | 归属 |
|---|---|---|
| `lib/main.dart:127` | `TODO(B3 …)` connState 路由 | B3 |
| `lib/pages/onboarding.dart:33` | `TODO(B3 …)` AgentConnection 接线 | B3 |
| `lib/pages/onboarding.dart:40` | `TODO(B3 …)` 深链 `settings?sub=connection` | B3 |
| `lib/pages/onboarding_prefs.dart:30` | `TODO(B3 …)` shared_preferences 持久化 | B3 |

**4/4 有波次归属。零无主 TODO。** 优良。

---

## 5. 错误处理

### 5.1 catch 后只 debugPrint 【P1 —— 2 处，均有意】

| 位置 | 代码 | 判定 |
|---|---|---|
| `collab.dart:231-235` | `on PaBridgeException catch (e) { debugPrint('collab: 能力位读取失败（${e.code}）…'); }` | **有意**。注释 232-234：能力位只是参与者名单来源，读不到走诚实空态，不进错误态（那是列表/详情/策略的职责）。 |
| `collab_role_editor.dart:146-149` | `catch (e) { debugPrint('collab-role-editor: 能力位读取失败…'); }` | **有意**。同上（agent 下拉候选降级为手填）。 |

**两处都不是「吞错误」**——错误的影响被显式降级为「下拉无候选 + 保存仍可用」，且注释论证了为什么这不是吞。**P1 仅因「debugPrint 是唯一可见出口」在生产构建中不可见**（Flutter release 模式 debugPrint 仍输出到系统日志，但用户看不到）。既然 UI 侧已诚实降级，**不需要改**。仅标注。

### 5.2 错误文案硬编码在 widget 里 —— 对比 CollabCopy 对位 【P1】

**已收敛的部分（优良）**：
- `CollabCopy`（`collab_session_data.dart:591-716`，125 行，~50 个文案常量）——对位 Kotlin `ui/CollabLogic.kt` 的 CollabCopy，**是全仓文案收敛的标杆**。
- `paChannelErrorText`（`pa_page_kit.dart:359`）——收敛了 8 个同型错误文案函数（threads/files×2/git/collab×3/terminal×2/chat_history），注释 317-332 有完整收敛论证。
- `paBridgeErrorText` 七码正典表（`pa_page_kit.dart` 委托）。

**未收敛的部分（债）**：
- `chat_actions.dart` 的 `_failureText`（216-222）自建一套「label 失败：message」拼装，与 `paChannelErrorText` 平行。**P1**——但它带 label 前缀，语义不同（动作失败 vs 通道失败），强行合并会藏差异。
- 文件内硬编码中文串扫描：`main.dart` 12 处、`chat_timeline.dart` 9 处、`chat_page.dart` 5 处、`chat_actions.dart` 4 处、`collab.dart` 3 处。这些多是 `Text('文字')` 一次性展示串，**非错误文案**，收敛价值低。**P2**。

**结论：错误文案收敛度已很高。剩余 4 个文件的零散展示串不值得收常量表。** 对比 Kotlin 的 `CollabCopy.kt`，Dart 侧 `CollabCopy` 是对位实现的**正面样板**，不是债。

### 5.3 `catch (_)` 空吞 【P2 —— 0 处】

扫描空 catch 块：**0 命中**。全仓 93 个 catch：47 个落 UI 态/state、2 个 debugPrint（§5.1）、2 个 rethrow、其余 42 个转换成 typed failure 上抛（`ThreadsLoadFailure` / `TerminalLoadFailure` / `CollabSessionsLoadFailure` 等）。**零空吞。优良。**

### 5.4 Future 未处理的 async 间隙 【P2 —— 1 处误报澄清】

`chat_actions.dart:211` `if (null is T) return null as T;`——这是泛型可空性的惯用法（Dart 无 `T?` 泛型特化时的标准写法），不是 async 间隙问题。**无债。**

---

## 6. 重复与结构

### 6.1 `_LoadingView` / `_ErrorView` / `_EmptyView` 三处逐字节重复 【P1 —— 本清单最实的重复债】

| 私有类 | 定义位置 | 行数 | 两两 jaccard |
|---|---|---|---|
| `_EmptyView` | `collab.dart:1837` / `files.dart:582` / `git.dart:547` / `threads.dart:439` | 42/57/57/54 | files↔git **0.95**、files↔threads **0.95**、git↔threads **0.91** |
| `_ErrorView` | `collab.dart:1786` / `files.dart:533` / `git.dart:498` | 50/47/47 | files↔git **0.95** |
| `_LoadingView` | `collab.dart:1756` / `files.dart:498` / `git.dart:463` | 29/33/33 | files↔git **0.93** |

**这是真重复**：`files.dart` 与 `git.dart` 的三个视图类在归一化（去注释/去 key 字符串/去字符串字面量）后 **95% 相同**。

**「收敛后是否藏差异」评估**：差异点枚举——
- `_EmptyView`：collab 版无 actionLabel/onAction；files/git/threads 版有。→ 参数化即可，不藏差异。
- `_ErrorView`：collab 版有 title 参数 + textKey；files/git 版无 title、无 textKey。→ 参数化。
- `_LoadingView`：collab 版有 textKey；files/git 版无；padding top 60 vs 48。→ padding 差异是真差异，需参数化或取其一。

**修复建议**：上提到 `pa_page_kit.dart`（该文件 1-14 行自述「pages 私域件的共享家，收敛产物」，正是这些件的归宿）。做成 `PaEmptyView` / `PaErrorView` / `PaLoadingView`，`textKey`/`title`/`actionLabel`/`onAction`/`topPadding` 全参数化。

**修复代价估算（挑 `_EmptyView` 4 合 1）**：
- 代价：**中，约 4-6 小时**。新建 1 个共享 widget（约 70 行）+ 改 4 个调用点 + 处理 collab 版的参数差集。
- 风险：**中**。`collab_page_test.dart`（29 测试 / 40 byKey）、`files_page_test.dart`（16）、`git_page_test.dart`（12）、`threads_page_test.dart`（11）的渲染结构断言可能因 widget 树多/少一层 `Align` 而翻。**缓解**：`pa_page_kit.dart:12-14` 已有先例——`PaCardCaption` 收敛时少一层 Align 被判定「布局恒等」并通过测试。照此办理。
- **测试水位保护**：4 个 page test 的**用例数不许减**；渲染断言若因结构变化失败，应更新为「按 key/文案断言」而非删用例。
- **不建议本轮做**——与 §2.2 的 collab 拆分有耦合（collab 的三个视图类在 collab.dart 尾部 1756-1878，拆 State 时会一起动）。建议合并成一个「collab.dart 拆分 + pages 三视图收敛」任务书。

### 6.2 `pages/` 91 文件依赖关系 【P2 —— 无债】

实测（非 gen）：`lib/pages/` **46 个文件**（不是 91；91 可能含 test/ 与历史口径）。
- **page → page import：0 处**。✅ 无循环、无横向依赖。
- page → transport import：24 处（`pa_bridge.dart` ×22、`chat_turn_stream.dart` ×1、`terminal_stream.dart` ×1）。这是**有意的分层**（页面经 transport 打通道，transport 是唯一 MethodChannel 持有者）。
- `MethodChannel` 在 pages/ 的直接使用：**仅 2 处注释提及**（`chat_session.dart:215`、`collab_role_editor.dart:652`），**零真实调用**。✅ 职责无渗漏。
- `main.dart` import 17 个 pages 文件，无 pages 反向 import main.dart（test 除外）。✅ 无循环。

**结论：pages 依赖关系干净。零债。** 唯一可议的是 `chat_actions_transport.dart` / `chat_history_transport.dart` / `chat_input_transport.dart` / `collab_bridge.dart` / `git_bridge.dart` / `files_bridge.dart` / `threads_bridge.dart` 这 7 个「transport 面」文件**住在 pages/ 而不是 transport/**——因为它们是被 `pa_bridge.dart`（B2 冻结，不许改）逼出来的「旁路 transport」（`chat_actions_transport.dart:3-5` 明确说「新增方法面若直接改它等于动冻结文件」）。**这是契约冻结的必然结果，不是结构失误。** 标注即可。

### 6.3 transport 与 pages 职责渗漏 【P2 —— 无债】

见 §6.2。`lib/transport/` 4 个文件（`pa_bridge.dart` 1100 / `chat_turn_stream.dart` 228 / `terminal_stream.dart` 202 / `chat_turn_stream.dart`）是唯一 MethodChannel 持有者；pages 只调其公开方法。**零渗漏。**

### 6.4 test/ 与 lib/ 结构镜像度 【P2】

- lib 60 文件 / test 58 文件。
- **镜像缺口**：23 个 lib 文件无同名 `_test.dart`。但其中多数有**聚合测试**覆盖：`main.dart` ← `main_routing_test.dart` + `widget_test.dart`；5 个 components ← `components_test.dart`；`collab.dart` ← `collab_page_test.dart`（991 行）；`chat_actions_data.dart` ← `chat_actions_test.dart`（986 行）；`pa_page_kit.dart` ← 各 page test 间接覆盖。
- **真缺口**：`chat_actions_transport.dart`（306 行 / 10 个方法）← `chat_actions_transport_test.dart`（15 测试）✅ 有；`chat_input_transport.dart` ← ✅；`chat_history_transport.dart` ← 由 `chat_history_data_test.dart` 间接覆盖 ✅。
- `browser_data.dart`（294 行）← `browser_data_test.dart`（22 测试）✅。

**结论：镜像度良好，无实质缺口。**

### 6.5 `_data` 文件同构度 —— 已在 §1.2 证伪。**无债。**

### 6.6 `_invoke` / `_invokeExpectNull` 四份同构 【P1】

4 个 transport 文件各有一份**逐字节相同**的 `_invoke<T>` 与 `_invokeExpectNull`（差异仅在 `_channel` vs `_methods` 字段名与 `(r) => r` vs `(Object? r) => r`）：

| 文件 | `_invoke` 位置 | `_invokeExpectNull` 位置 |
|---|---|---|
| `lib/transport/pa_bridge.dart` | 864 | 885 |
| `lib/transport/chat_turn_stream.dart` | 172 | 192 |
| `lib/transport/terminal_stream.dart` | 164 | 185 |
| `lib/pages/chat_actions_transport.dart` | 239 | 260 |

**为什么不能简单收敛**：`pa_bridge.dart` 是 **B2 冻结契约**（`chat_actions_transport.dart:3-5` 明确「本波只读，不改它一行」）。抽公共函数必须新增文件（如 `lib/transport/_invoke_core.dart`），那等于**在冻结面旁边新增共享件**——技术上可行（不动 pa_bridge 一行），但会让「B2 契约的自包含性」变糊。

**判定：P1，但因红线不建议本轮改。** 若做：新建 `lib/transport/pa_invoke.dart` 导出 `paInvoke` / `paInvokeExpectNull` 两个函数，4 个文件的私有方法改为一行委托。**代价 3 小时，风险中**（`pa_bridge_test.dart` 21 个测试的错误码断言是逐字节钉死的，委托后栈帧变了但行为不变）。**测试水位**：不减（等价重构 + 测试全绿）。

---

## 7. 测试整洁

### 7.1 重复 mock 装配 【P1 —— 远超阈值】

| 指标 | 次数 | 阈值 | 超标倍数 |
|---|---|---|---|
| `setMockMethodCallHandler` | **200** | 10 | **20×** |
| `TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger` | **36** | — | — |
| `const MethodChannel(kPaBridgeMethodChannel)` | **31** | — | — |
| `setMockStreamHandler` / `setMockDecodedMessageHandler` | 16 | — | — |

分布：`pa_bridge_test.dart` 19 / `pa_bridge_files_git_test.dart` 18 / `collab_roles_state_test.dart` 15 / `pa_bridge_collab_test.dart` 14 / `terminal_stream_test.dart` 14 / `turn_stream_test.dart` 14 …共 31 个文件。

**已有 helper（说明团队知道该收）**：
- `test/support/pump_helpers.dart`（16 行）：`pumpUntilSplashDone`。
- `test/transport/fixture_support.dart`（37 行）：`fixtureBytes`，且文件头 3-8 行有「三份测试各写一份逐字节相同，收在这里做唯一出处」的论证。

**缺的 helper**：
1. **`mockBridgeHandler(messenger, channel, handler)`** —— 消掉 200 处 `messenger.setMockMethodCallHandler(channel, (MethodCall call) async {…})` 的样板。
2. **`bridgeMessenger()`** —— 消掉 36 处 `TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger`。
3. **`kTestNowMs` 常量** —— `1728901200000` / `1728904800000` 硬编码出现 **26 次**，跨 13 个文件（`pa_bridge_runtime_test.dart` 5 / `pa_bridge_files_git_test.dart` 4 / `threads_bridge_test.dart` 3 …）。
4. **`mockHost(roles: …)` 形状** —— `collab_page_test.dart` 与 `collab_bridge_test.dart` 各有 `protoMember`/`protoSession`/`demoRole`（2 文件重复定义）。

**修复代价估算（挑 `mockBridgeHandler` helper）**：
- 代价：**中，约 4 小时**。新建 `test/support/mock_bridge.dart`（约 60 行：`mockBridgeHandler` + `bridgeMessenger` + `kTestNowMs` + `bridgeMethodChannel` 常量）；把 31 个文件的 200 处调用改写。
- 风险：**低-中**。纯测试内重构，不改生产码。风险点在「每个 handler 的返回类型/异常形状不同」，helper 要够薄（就是 `setMockMethodCallHandler` 的一层薄包装，不加语义）。
- **测试水位保护（硬约束 949 不许跌）**：
  - 用例数不变（200 处 handler 是装配不是用例）；
  - **逐个文件改、逐个文件跑** `flutter test test/<file>.dart`，全绿才下一个；
  - 每改完一个文件记录用例数（如 `collab_roles_state_test.dart` = 20），收尾核对 945 测试块总数不变；
  - **不合并、不删任何现有 test**（包括「看起来重复」的 handler 装配——它们钉的是不同方法名）。
- **判定：P1，值得做，但必须分批。** 建议按「先 transport/ 7 个文件（200 处里占 88 处），再 pages/」分两批，每批独立可回滚。

### 7.2 magic number 【P2】

高频裸数字：`7246`（端口，32 处 / 10 文件，其中生产侧 6 处：`browser_policy.dart` 2 / `onboarding_steps.dart` 3 / `settings_connection.dart` 1）、`1024`（36 处 / 4 文件）、`400`/`300`/`800`（动画时长，`splash_test.dart` / `onboarding_page_test.dart`）、`60000`（`task_center_data_test.dart` 4 处）、`1500`（`kTaskRetryCooldownMs` 的裸用，3 处）。

**判定**：多数是**测试固有时钟/端口**，收敛价值中等。`7246` 应收 `kTestPort`（它同时出现在生产 `settings_connection.dart` 的默认地址里——见 §7.4）。**P2。**

### 7.3 超长 testWidgets（>80 行） 【P2】

| 行数 | 位置 | 测试名 |
|---|---|---|
| **87** | `test/pages/settings_console_test.dart:482` | 未知事件详情：走 getUnknownEvent（只读），原文可… |
| **80** | `test/widget_test.dart:44` | 底部 4 Tab（会…工具/文件/设置）默认暗色，深浅色可… |
| **72** | `test/pages/settings_downloads_test.dart:242` | 下载条目进分区汇总与列表；其余分区标「未统计…」 |

仅 3 个超 80 行（阈值 60 行时有 3 个）。**远好于典型项目。P2，不建议拆**（拆了反而丢「一条用户旅程」的叙事）。

### 7.4 断言无 reason 的比例 【P2】

**2885 个 `expect(` 中仅 114 个带 `reason:`（96.0% 无 reason）**。

分布不均：`task_center_data_test.dart` 28/161（17%）、`task_center_page_test.dart` 22/70（31%）、`settings_downloads_test.dart` 14/57、`collab_page_test.dart` 14/93、`onboarding_steps_test.dart` 15/45（**33%，最佳**）——而 `browser_policy_test.dart` 0/73、`browser_page_test.dart` 0/9、`chat_actions_transport_test.dart` 0/41 为 0%。

**判定**：`reason:` 在测试断言失败时打印，对**回归排查**有价值。96% 缺失是**一致性债**，非正确性债。**P2**——不建议批量补（补 reason 是低价值高产出错风险的动作：写错的 reason 比没有更糟）。若要补，只补「失败时原因不显然」的断言。### 7.5 find.byKey 硬编码字符串散落 【P1】

| 域 | 次数 | 分布 |
|---|---|---|
| lib/（`ValueKey<String>('…')`） | **161** | collab.dart 30 / terminal.dart 18 / chat_composer.dart 14 / chat_actions.dart 13 / main.dart 11 / chat_history.dart 9 / collab_roles.dart 9 / collab_role_editor.dart 8 / settings_connection.dart 8 / settings_console.dart 7 / …共 22 文件 |
| test/（`find.byKey(const ValueKey<String>('…'))`） | **341** | settings_console_test 39 / chat_page_test 30 / chat_actions_test 26 / collab_role_editor_page_test 26 / chat_composer_test 25 / terminal_test 25 / settings_connection_test 24 / settings_downloads_test 22 / chat_history_test 20 / …共 23 文件 |

**高频 key**（lib 与 test 必须一致，改一处漏一处就红）：`pa-more-menu` / `pa-git-back` / `pa-chat-back` / `pa-terminal-back` / `pa-browser-back` / `pa-taskcenter-back` / `pa-collab-back` / `pa-collabroles-back` / `pa-collabroleeditor-back`（全在 `main.dart:208-469` + `main_routing_test.dart`）。

**修复建议**：建 `lib/pages/pa_keys.dart`（或 `test/support/pa_keys.dart` 若只测试侧用）收 `const String kKeyMoreMenu = 'pa-more-menu';` 等。**但注意**：key 字符串是**测试契约**，收敛成常量后 lib 与 test 共享同一常量，反而**降低「lib 改 key 而 test 没跟上」的风险**（现在是字符串字面量，编译器不帮查）。

**代价**：约 6 小时（161 + 341 处）。
**风险**：**中**。纯机械替换，但 502 处里错一处就编译失败（好消息：编译器全帮你抓）。
**测试水位**：不减（等价替换 + 全绿）。
**判定**：**P1，值得做但ROI中等。** 优先只收 `main.dart` 的 11 个 `pa-*` 路由 key（最高频、跨 lib/test 两侧、路由相关最易随导航改动漂移）。

---

## 8. P0/P1 典型修复代价与风险汇总

| # | 债 | 级 | 代价 | 风险 | 水位影响 | 红线 |
|---|---|---|---|---|---|---|
| 1 | `TaskCenterStore._controller` 无 close（§3.4） | P1 | 10 分钟 | 零 | 不减（加方法不改行为） | 无 |
| 2 | `collab_roles_state.dart` `late _loadFuture` 幽灵初始化（§3.5） | P1 | 1 小时 | **中** | 需 20/20 绿；若测试时序翻=测试钉错语义，连测试一起修 | 无 |
| 3 | `_EmptyView` 4 合 1 上提 pa_page_kit（§6.1） | P1 | 4-6 小时 | **中** | 4 个 page test 用例数不许减 | 无 |
| 4 | `_CollabPageState` 993 行拆分（§2.2） | **P0** | 2-3 人日 | **中高** | 29 测试全绿才算完 | 无 |
| 5 | test mock 装配收 helper（§7.1） | P1 | 4 小时/批 | 低-中 | **分批跑，945 测试块总数核对不变** | 硬约束 949 |
| 6 | k 前缀常量改名（非契约的 24 个）（§1.1） | P1 | 30 分钟/文件 | 低 | 编译过即不跌 | **`pa_bridge.dart` 等 transport 的 8 个只标注不改** |
| 7 | `_invoke` 四份收敛（§6.6） | P1 | 3 小时 | 中 | 等价重构+全绿 | **不碰 pa_bridge 一行，新增旁路文件** |
| 8 | `pa-*` 路由 key 收常量（§7.5） | P1 | 2 小时 | 中 | 等价替换+全绿 | 无 |

---

## 9. 红线标记（只标注，不建议本轮改）

| 红线 | 状态 | 说明 |
|---|---|---|
| `flutter/lib/gen/**`（protoc 生成） | **未审查** | 已排除（`analysis_options.yaml:20` 也整目录除外） |
| `lib/transport/pa_bridge.dart` 公开 API（B2 契约面） | **只标注** | ① 8 个 k 前缀常量改名需与 Kotlin 侧同步改（§1.1）；② `_invoke`/`_invokeExpectNull` 重复不可直接收敛，只能新增旁路文件（§6.6）；③ `channel-contract.md:18` 明文引用 `kPaBridgeMethodChannel`/`kPaBridgeEventChannel`，改名需同步该文档 |
| `main.dart` 路由与 B4e 接线 | **只标注** | `main.dart:194-286` build 93 行 + 9 个 `_XxxRoutePage` 类是路由外壳；按任务书只标注不改 |
| 测试水位 949 | **硬约束** | 所有测试侧改动（§7.1/§7.5）必须分批 + 逐文件跑 + 核对 945 测试块总数不减 |
| `channel-contract.md` 对账面 | **发现一处漂移** | contract 列 16 个方法名，Dart 侧 transport 实有 46 个公开方法；**30 个 Dart 方法未出现在 contract 的 backtick 清单里**（`cancelTurn`/`clearQueuedInputs`/`compactThread`/`forkThread`/`rollbackThread`/`setThreadGoal`/`startReview`/`exportThreadHtml`/`optimizePrompt`/`submitApproval` 等 10 个在 `chat_actions_transport.dart`，其余 20 个散在 `terminal_stream`/`chat_turn_stream`/`chat_history_transport`/`chat_input_transport`）。contract §3 标题自称「共 35 条」，§3.16 提到「runtime 域 RPC 全集与本波取舍」——**这是文档滞后于实现，不是代码债**。建议更新 channel-contract.md（属丁级文档改动）。 |
| `flutter analyze` | **本机不可用** | analysis server 崩溃（`FormatException: Unexpected end of input` at LSP handshake），非代码问题。未能验证「零 warning / 零 info」声明。建议在有可用 SDK 的环境补跑一次作为基线记录。 |

---

## 10. 建议执行顺序（若要做）

1. **先做零风险的**：§3.4 `TaskCenterStore.dispose()`（10 分钟，消一个未来泄漏）。
2. **再做契约外的纯重命名**：§1.1 的 26 个 k 常量（逐文件，编译过即完）。
3. **再做测试装配收敛**：§7.1 + §7.5，严格分批，每批核对水位。
4. **最后做结构性拆分**：§6.1 三视图收敛 → §2.2 `_CollabPageState` 拆分（P0，需独立任务书 + worktree 隔离）。
5. **明确不做**：§1.2（_data 收敛，伪命题）、§2.5（巨型 switch，全合理）、§4.2（考古注释，保留）、§5.1（debugPrint 降级，有意）。

---

## 附：量化脚本

所有扫描脚本位于 `C:\Users\yanzi\AppData\Local\Temp\opencode\scan*.py`（scan1-scan43），输出在同目录 `out*.txt`。核心口径：
- build 方法行数：正则匹配 `Widget build(` + 花括号配平。
- State 类行数：`class _XxxState extends State<` 到下一个列 0 的 `}`。
- k 前缀：`(?:static )?(?:const|final) k[A-Z]` 全文件匹配 + 引用点统计。
- 重复度：类体归一化（去注释/去空行/`ValueKey` 与字符串字面量替换为占位符）后算行集合 jaccard。
- mock 装配：`setMockMethodCallHandler` / `TestDefaultBinaryMessengerBinding` / `MethodChannel(kPaBridgeMethodChannel)` 字面计数。
