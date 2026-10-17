# B2 通道契约 —— Flutter ⇄ Kotlin 宿主桥（MethodChannel / EventChannel）

> **本契约与 `proto/v1/` 同源；proto 变更时：Dart gen stub 重生成（附录 A）+
> 本文档同步。** 方法名、事件 `type`、错误码、Map 键名都是契约字符串，双端同改。
>
> - Dart 侧实现：`flutter/lib/transport/pa_bridge.dart`（代码即契约），
>   测试 `flutter/test/transport/pa_bridge_test.dart` 逐条钉死（mock 宿主 wire 行为）；
>   同一条方法通道上的其余五个 transport 面见 §1 末段。
> - Kotlin 宿主侧：**B4 波次已落地**——`installBridge`（方法面 + 三条事件面 +
>   双向流 wiring 三件套）由 `flutterhost/FlutterHostEngine.installChannelShell`
>   在 FlutterEngine 挂载时装配，dispatch 分支 30 条，接线状态见 §6。
>   §3 表末列「宿主桥」逐行给出状态，**15 条「有意未接」的清单与理由见 §8**。
>   未接的方法名仍会收到 `HOST_UNAVAILABLE`（不伪造任何返回，红线：不留假绿）。

## 1. 通道

| 通道 | 类型 | 名称 | 方向 | Dart 常量 | Kotlin 常量 |
|---|---|---|---|---|---|
| 方法面 | MethodChannel | `dev.pocketagent/bridge` | Dart → Kotlin，请求应答 | `kPaBridgeMethodChannel`（pa_bridge.dart:43） | `BRIDGE_CHANNEL_NAME`（BridgeChannel.kt:60） |
| 事件面 | EventChannel | `dev.pocketagent/bridge/events` | Kotlin → Dart，推 Envelope Map | `kPaBridgeEventChannel`（pa_bridge.dart:46） | `BRIDGE_EVENT_CHANNEL_NAME`（BridgeEventChannel.kt:16） |
| Turn 事件面 | EventChannel | `dev.pocketagent/bridge/turn_events` | Kotlin → Dart，推裸 wire 字节 | `kTurnStreamEventChannel`（chat_turn_stream.dart:49） | `BRIDGE_TURN_EVENT_CHANNEL_NAME`（BridgeEventChannel.kt:19） |
| Terminal 事件面 | EventChannel | `dev.pocketagent/bridge/terminal_events` | Kotlin → Dart，推裸 wire 字节 | `kTerminalEventChannel`（terminal_stream.dart:37） | `BRIDGE_TERMINAL_EVENT_CHANNEL_NAME`（BridgeEventChannel.kt:22） |

四条通道名逐字一致由 `pa_audit` 的 `channel_contract_sync` 规则逐轮对账
（本表 ↔ Dart 常量 ↔ Kotlin 常量，三方集合相等才放行）。

方法面**一条通道**上分装 §3 表的全部方法名，由六个 Dart transport 面共用同一
通道（宿主在同一 handler 表里注册全部方法名）：`transport/pa_bridge.dart`、
`transport/terminal_stream.dart`、`transport/chat_turn_stream.dart`、
`pages/chat_history_transport.dart`、`pages/chat_input_transport.dart`、
`pages/chat_actions_transport.dart`。新增方法面**不需要**新通道，也不要新通道。

## 2. 编码总则

1. **有 proto 请求消息的方法**：`arguments` = 请求消息 `writeToBuffer()` 的
   wire 字节；空消息 = 空字节数组（长度 0——不做 null/bytes 两种形态的版本二分）。
2. **无 proto 请求消息的方法**：`arguments` = Map 或 null（逐条见 §3）。
3. **返回值**：proto 消息 = wire 字节（`Uint8List`）；连接态 = Map；无返回语义
   的方法 = null（宿主回了别的即 `DECODE_FAILED`，显式抓契约漂移）。
4. **错误透传**：宿主抛 `PlatformException(code, message, details)` → Dart 侧
   `PaBridgeException(code, message, details)` 三件套原样透传。code 是 UI 分支
   依据（重试/去配对/仅展示），Dart 侧**不改写**宿主 code。
5. **宿主缺席**：方法面 `invokeMethod` 得 `MissingPluginException`
   （Android 引擎对无 handler 通道回 null 应答）→ `PaBridgeException(HOST_UNAVAILABLE)`。
   注意：**事件面握手错不进流**（Flutter `receiveBroadcastStream` 把 'listen' 的
   异常转投 `FlutterError.reportError`）——判定宿主在不在必须走方法面探针，
   不能等事件流报错（test/transport 有对账测试钉死该行为）。
6. **Dart 侧解码严格**：结构/字节/键不符 = `DECODE_FAILED` 显式失败，不补零值、
   不静默吞。唯一例外见 §4（未知事件类型也是显式失败，不透传——双端同 APK
   同 versionCode，不存在"新宿主对旧 UI"单边演进）。

## 3. 方法清单（45 条 = B3 全波次 + B5 Settings ③ + B3 第 5 波④ Chat 动作；宿主桥接线状态见末列）

| # | 方法名（契约字符串） | Dart 签名 | arguments | 返回 | 宿主桥 |
|---|---|---|---|---|---|
| 1 | `getHostInfo` | `Future<HostInfo> getHostInfo()` | `GetHostInfoRequest` 字节（空） | `HostInfo` 字节 | 已接 |
| 2 | `connect` | `Future<void> connect(String addr, String token)` | `{'addr': String, 'token': String}` | null | 已接 |
| 3 | `disconnect` | `Future<void> disconnect()` | null | null | 已接 |
| 4 | `getConnectionState` | `Future<PaConnState> getConnectionState()` | null | Map（五键，见下） | 已接 |
| 5 | `pairCode` | `Future<String> pairCode()` | null | 非空 String | 已接 |
| 6 | `startTurn` | `Future<void> startTurn(TurnInput input)` | `TurnInput` 字节 | null | 已接（故意拒，见 §3.6） |
| 7 | `listThreads` | `Future<ListThreadsResponse> listThreads([ListThreadsRequest?])` | `ListThreadsRequest` 字节（缺省 = 空消息） | `ListThreadsResponse` 字节 | 已接 |
| 8 | `listDirectory` | `Future<ListDirectoryResponse> listDirectory([ListDirectoryRequest?])` | `ListDirectoryRequest` 字节（缺省 = 空消息） | `ListDirectoryResponse` 字节 | 已接 |
| 9 | `readFile` | `Future<FileChunk> readFile([ReadFileRequest?])` | `ReadFileRequest` 字节（缺省 = 空消息） | `FileChunk` 字节（**首块**，见 §3.9） | 已接 |
| 10 | `listBranches` | `Future<ListBranchesResponse> listBranches([ListBranchesRequest?])` | `ListBranchesRequest` 字节（缺省 = 空消息） | `ListBranchesResponse` 字节 | 已接 |
| 11 | `listCollabRoles` | `Future<ListCollabRolesResponse> listCollabRoles()` | `ListCollabRolesRequest` 字节（空消息） | `ListCollabRolesResponse` 字节 | 已接 |
| 12 | `setCollabRole` | `Future<SetCollabRoleResponse> setCollabRole(CollabRole role)` | `SetCollabRoleRequest` 字节（**外面包一层**：`role` 字段承载 `CollabRole`） | `SetCollabRoleResponse` 字节 | 已接 |
| 13 | `deleteCollabRole` | `Future<DeleteCollabRoleResponse> deleteCollabRole(String id)` | `DeleteCollabRoleRequest` 字节（字段按 proto，非 Map） | `DeleteCollabRoleResponse` 字节 | 已接 |
| 14 | `dispatchCollab` | `Future<DispatchCollabResponse> dispatchCollab(DispatchCollabRequest req)` | 请求字节 | `DispatchCollabResponse` 字节 | 已接 |
| 15 | `getCollabSession` | `Future<CollabSessionSnapshot> getCollabSession([GetCollabSessionRequest?])` | `GetCollabSessionRequest` 字节（缺省 = 空消息） | `CollabSessionSnapshot` 字节 | 已接 |
| 16 | `listCollabSessions` | `Future<ListCollabSessionsResponse> listCollabSessions([ListCollabSessionsRequest?])` | `ListCollabSessionsRequest` 字节（缺省 = 空消息） | `ListCollabSessionsResponse` 字节 | 已接 |
| 17 | `cancelCollabSession` | `Future<CollabSessionSnapshot> cancelCollabSession([CancelCollabSessionRequest?])` | `CancelCollabSessionRequest` 字节（session_id） | 快照字节 | 已接 |
| 18 | `cancelCollabMember` | `Future<CollabMemberSnapshot> cancelCollabMember([CancelCollabMemberRequest?])` | `CancelCollabMemberRequest` 字节 | 快照字节 | 已接 |
| 19 | `retryCollabMember` | `Future<CollabMemberSnapshot> retryCollabMember([RetryCollabMemberRequest?])` | `RetryCollabMemberRequest` 字节 | 快照字节 | 已接 |
| 20 | `getCollabPolicy` | `Future<CollabPolicyResponse> getCollabPolicy()` | `GetCollabPolicyRequest` 字节（空消息） | `CollabPolicyResponse` 字节 | 已接 |
| 21 | `listTerminals` | `Future<ListTerminalsResponse> listTerminals([ListTerminalsRequest?])` | `ListTerminalsRequest` 字节（缺省 = 空消息） | `ListTerminalsResponse` 字节 | 已接 |
| 22 | `terminalAttach` | `Future<void> terminalAttach(String terminalId)` | `{'terminalId': String}` | null | 已接 |
| 23 | `terminalWrite` | `Future<void> terminalWrite(Uint8List frameBytes)` | `TerminalClientMessage` 字节 | null | 已接 |
| 24 | `terminalClose` | `Future<void> terminalClose(String terminalId)` | `{'terminalId': String}` | null | 已接 |
| 25 | `turnStreamStart` | `Future<void> turnStreamStart(String threadId)` | `{'threadId': String}` | null | 已接 |
| 26 | `turnStreamSend` | `Future<void> turnStreamSend(Uint8List frameBytes)` | `TurnClientMessage` 字节 | null | 已接 |
| 27 | `turnStreamCancel` | `Future<void> turnStreamCancel()` | null | null | 已接 |
| 28 | `turnResync` | `Future<List<TurnServerMessage>> turnResync(String threadId, int sinceSequence)` | `{'threadId': String, 'afterSequence': int}` | `List<Uint8List>`（快照帧数组） | 已接 |
| 29 | `getThreadHistory` | `Future<GetThreadHistoryResponse> getThreadHistory(String threadId)` | 请求字节 | 响应字节 | 已接 |
| 30 | `listSlashCommands` | `Future<ListSlashCommandsResponse> listSlashCommands(String threadId, {bool refresh})` | 请求字节 | 响应字节 | 已接 |
| 31 | `getRuntimeState` | `Future<RuntimeStateResponse> getRuntimeState()` | `GetRuntimeStateRequest` 字节（空） | `RuntimeStateResponse` 字节 | 有意未接（见 §8） |
| 32 | `getRuntimePolicy` | `Future<RuntimePolicyResponse> getRuntimePolicy()` | `GetRuntimePolicyRequest` 字节（空） | `RuntimePolicyResponse` 字节 | 有意未接（见 §8） |
| 33 | `getControlPlane` | `Future<ControlPlane> getControlPlane()` | `GetControlPlaneRequest` 字节（空） | `ControlPlane` 字节 | 有意未接（见 §8） |
| 34 | `listUnknownEvents` | `Future<ListUnknownEventsResponse> listUnknownEvents([ListUnknownEventsRequest?])` | `ListUnknownEventsRequest` 字节（缺省 = 空消息） | `ListUnknownEventsResponse` 字节 | 有意未接（见 §8） |
| 35 | `getUnknownEvent` | `Future<UnknownEventResponse> getUnknownEvent([GetUnknownEventRequest?])` | `GetUnknownEventRequest` 字节（缺省 = 空消息） | `UnknownEventResponse` 字节 | 有意未接（见 §8） |
| 36 | `submitApproval` | `Future<void> submitApproval(SubmitApprovalRequest request)` | `SubmitApprovalRequest` 字节（thread_id + decision） | null | 有意未接（见 §8） |
| 37 | `cancelTurn` | `Future<void> cancelTurn(String threadId)` | `CancelTurnRequest` 字节（thread_id） | null | 有意未接（见 §8） |
| 38 | `clearQueuedInputs` | `Future<ClearQueuedInputsResponse> clearQueuedInputs(String threadId)` | `ClearQueuedInputsRequest` 字节（thread_id） | `ClearQueuedInputsResponse` 字节 | 有意未接（见 §8） |
| 39 | `compactThread` | `Future<void> compactThread(String threadId)` | `CompactThreadRequest` 字节（thread_id） | null | 有意未接（见 §8） |
| 40 | `forkThread` | `Future<Thread> forkThread(String threadId, {String lastTurnId})` | `ForkThreadRequest` 字节（thread_id + last_turn_id） | `Thread` 字节（**新会话**） | 有意未接（见 §8） |
| 41 | `rollbackThread` | `Future<Thread> rollbackThread(String threadId, int numTurns)` | `RollbackThreadRequest` 字节（thread_id + num_turns） | `Thread` 字节（回滚后真值） | 有意未接（见 §8） |
| 42 | `setThreadGoal` | `Future<Thread> setThreadGoal(String threadId, String goal)` | `SetThreadGoalRequest` 字节（thread_id + goal） | `Thread` 字节（落库后真值） | 有意未接（见 §8） |
| 43 | `startReview` | `Future<StartReviewResponse> startReview(String threadId, {String focus})` | `StartReviewRequest` 字节（thread_id + focus） | `StartReviewResponse` 字节（review_id + started） | 有意未接（见 §8） |
| 44 | `exportThreadHtml` | `Future<ExportThreadHtmlResponse> exportThreadHtml(String threadId)` | `ExportThreadHtmlRequest` 字节（thread_id） | `ExportThreadHtmlResponse` 字节（path） | 有意未接（见 §8） |
| 45 | `optimizePrompt` | `Future<OptimizePromptResponse> optimizePrompt({required String text, String style, String agentId, String modelId, String effort})` | `OptimizePromptRequest` 字节（text + style + agent_id + model_id + effort） | `OptimizePromptResponse` 字节（optimized） | 有意未接（见 §8） |

> **末列「宿主桥」的口径**（`pa_audit` 的 `channel_contract_sync` 规则逐行对账）：
>
> - `已接` = `BridgeMethods.dispatch` 有同名分支（共 30 条，其中 `startTurn` 是
>   **故意抛 HOST_UNAVAILABLE** 的死入口，理由见 §3.6）；
> - `有意未接` = 契约声明了方法名、Dart 侧也在调，但宿主桥**故意不注册**
>   （共 15 条）。调用结果是 `HOST_UNAVAILABLE`，**不是**契约漂移——理由与
>   逐条清单见 §8。
>
> 对账式：`{已接} == Kotlin dispatch 分支集` 且 `{已接 ∪ 有意未接} == Dart 通道面
> 方法集` 且 `{已接 ∪ 有意未接} == 本表`。任一不等即红线。

### 3.1 `getHostInfo` —— proto `HostService.GetHostInfo`（proto/v1/host.proto）

- 返回字节 → gen stub `HostInfo`（`flutter/lib/gen/pocketagent/v1/host.pb.dart`）。
- 宿主侧语义：解析 `GetHostInfoRequest` → 转发 daemon `GetHostInfo` →
  `HostInfo` 字节回传（B4）。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。

### 3.2 `connect(addr, token)`

- `token` 为空 = 尚未配对/免鉴权宿主；token 只在进程内过桥，不进日志。
- 返回 null = **请求受理**，不是"已连上"——Kotlin 连接是异步的，真值是
  `conn_state` 事件 / `getConnectionState()` 快照（对应 Kotlin connState 流）。
- 空地址在 Dart 本地即拒（`INVALID_ADDR`），不产生跨端调用；宿主侧地址
  解析失败同样回 `INVALID_ADDR`。另有 `INTERNAL`。

### 3.3 `disconnect()`

- arguments/返回均 null；断开后的状态变化同样走 `conn_state`。

### 3.4 `getConnectionState()` —— Kotlin `AgentConnection` 五条 StateFlow 的一次性快照

返回 Map 五键（**键名 = Kotlin 字段名**，B4 实现可直接取 StateFlow 值拼装）：

| 键 | 类型 | Dart 字段（`PaConnState`） | Kotlin 源（page-contracts §3.1） |
|---|---|---|---|
| `connState` | String | `state` | `connState`（"checking"/"ok"/"down"） |
| `connTransport` | String | `transport` | `connTransport` |
| `connAddress` | String | `address` | `connAddress` |
| `connectionEpoch` | int | `epoch` | `connectionEpoch`（重连自增 = 全局刷新信号） |
| `hostCapabilities` | List\<String\> | `capabilities` | `hostCapabilities`（能力位唯一真值） |

- **为什么同时要快照和事件**：事件流订阅即收、无缓冲（§4），页面冷起与断线
  期间的态必须靠本方法补初值，事件只负责之后的增量。
- 五键恒有值（StateFlow 语义）；缺键/类型不符 = 契约漂移 → `DECODE_FAILED`，
  消息定位到最先违约的键（校验顺序 = 上表键序）。
- `state`/`transport` 是 String 不是 enum：值域宿主单方面定义，Dart 枚举会变成
  第二份写死名单，未知值按透传处理（未知输入透传红线）。
- 错误码：`HOST_UNAVAILABLE` / `DECODE_FAILED`（快照读取本身不依赖连接）。

### 3.5 `pairCode()` —— proto `HostService.GetPairingInfo` 的 `pair_code` 字段

- 返回非空 String 配对码；空/null 返回 = `DECODE_FAILED`。
- 安全语义随 proto：配对码只应答本机回环来源（USB reverse 后手机请求即回环）；
  来源不允许/未开启时宿主抛 `PAIR_DISABLED`，**不得**用空返回代替错误。
- `GetPairingInfoResponse` 的 `pairing_enabled` / `daemon_version` 暂不过桥
  （首批只取配对码；需要时按同规则扩方法，不复用本方法返回类型）。

### 3.6 `startTurn(input)` —— proto `TurnService.StartTurn` 首帧

- **现状 = 方法名已登记、宿主分支存在但故意抛 `HOST_UNAVAILABLE`**（§3 表末列
  `已接（故意拒）`；真源 `BridgeMethods.kt:438`）。它是本表唯一的「有分支即拒绝」
  条目，与 §8 的「有意未接」是两种不同的缺席：本条占位并给理由，那 15 条连分支
  都不注册。
- arguments = `TurnInput`（thread_id + blocks + background + model_id + effort）
  的 wire 字节。**设计形态**（未启用）：宿主包成 `TurnClientMessage{input}` 送进
  双向流。空消息字段规则同 proto（空 = 沿用/默认）。
- **为什么不接**：unary 只发首帧、没有消费面，「发了首帧却看不到任何过程帧」比
  不接更糟（假象）。轮次的完整形态是双向流 `turnStreamStart` + `turnStreamSend` +
  `turnStreamCancel`（§3 表 #25–#27，已接）。
- Dart 侧 `PaBridge.startTurn`（`pa_bridge.dart:373`）保留为**零生产调用方**的兼容
  入口：`flutter/lib` 内唯一同名调用点是 `chat_page.dart:143` 的
  `TurnSession.startTurn`（页层类，内部走双流），不是本方法。
- 错误码（当前实际）：`HOST_UNAVAILABLE`（宿主唯一抛出的码，message 点名「无消费面
  的 unary 首帧入口」并指引改用双流）。Dart 解码侧：`NOT_CONNECTED` /
  `AUTH_REQUIRED` / `INTERNAL` 三码在本方法**不会到达**（宿主不实现、不透传）。

### 3.7 `listThreads([request])` —— proto `ThreadService.ListThreads`（proto/v1/thread.proto:18，B3-3 追加）

- 返回字节 → gen stub `ListThreadsResponse`（`threads[]` 即 proto `Thread`）。
- 为什么默认不带参：B3-3 只吃列表面，置顶/归档/三维筛选都在页端版本化
  （`applyThreadFilter`），请求面保持「空消息原文」——不在传参层写死任何筛选默认值。
- 列表为空 = 真空态（手机端显示「还没有会话」）；**通道不可用是另一条路径**
  （`HOST_UNAVAILABLE` →「桌面端会话通道未就绪（B4 波次）」），两者在 UI 与测试里
  严格区分，不许用空列表冒充错误（防假空态）。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。
- 宿主侧语义（B4）：转发 daemon `ListThreads` → `ListThreadsResponse` 字节回传。

### 3.8 `listDirectory([request])` —— proto `FileService.ListDirectory`（B3-4 追加）

- 返回字节 → gen stub `ListDirectoryResponse`（目录/文件两型条目）。
- 为什么默认不带参：路径在 `ListDirectoryRequest.path`（空 = daemon 默认工作区），
  面包屑导航是页端纯函数（`breadcrumbSegmentsOf`），传参层不写死任何路径默认值。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。
- 宿主侧语义（B4）：转发 daemon `ListDirectory` → `ListDirectoryResponse` 字节回传。

### 3.9 `readFile([request])` —— proto `FileService.ReadFile`（B3-4 追加；**服务器流收成首块**）

- proto 侧 `ReadFile` 是 server stream（分块回传大文件）。Dart 面**只取首块**：
  `FileChunk` 的 `data` / `total_size` / `text` / `modified_unix_ms`。
- **256 KB 预览帽在 Dart 侧调用前判定**：请求体超过即本地拒绝，不产生跨端调用；
  `total_size > data.length` 时页面诚实标注「已截断」（不假装看完全文）。
- 二进制/超大文件 → 页面诚实提示「不支持预览」，不用错误码代替能力声明。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。
- 【为什么流收成首块还留在方法面】B2 契约只到「方法 + 三类事件」，Turn 双向流
  与文件分块流的 Dart 消费面留 B5；本波 readFile 只解决「文本文件点开看一眼」，
  首块语义与 256KB 帽组合已覆盖该场景，完整流式读取不摆假入口。

### 3.10 `listBranches([request])` —— proto `GitService.ListBranches`（B3-4 追加）

- 返回字节 → gen stub `ListBranchesResponse`（分支列表 + 当前分支标识）。
- 工作区选择：`ListBranchesRequest` 的路径字段（空 = daemon 默认）；页端 roots 列表
  来自 `getHostInfo` 的 workspaces + 手输路径二选一。
- **只读**：切分支/新建分支不摆入口（写操作留后续波次拍板）。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。

### 3.11 `getRuntimeState()` —— proto `RuntimeService.GetRuntimeState`（B5 Settings ③ 追加）

- 返回字节 → gen stub `RuntimeStateResponse`（安装身份 / 主机 / daemon 版本 /
  两级策略 revision / `runtime_state` / `contracts[]` / `data_dir`）。
- **空字段 = daemon 未报告**：各字段是普通 proto3 string，空串 = 这一项没报，
  页面据此说「未报告」，不补零值不猜（与空态/错误态三分同一纪律）。
- `runtime_state` 是 String 不是 enum：值域由 daemon 单方面定义（ready/degraded），
  Dart 枚举会变成第二份写死名单，未知值透传（未知输入透传红线）。
- `contracts[]` 六个 model 字段（session/resume/approval/subagent/memory/cache）
  是 daemon 给的机器可读答案，客户端据此排版，**不按 agent 名分支**。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。

### 3.12 `getRuntimePolicy()` —— proto `RuntimeService.GetRuntimePolicy`（B5 Settings ③ 追加）

- 返回字节 → gen stub `RuntimePolicyResponse`（`policy` = 用户配置 / `revision` /
  `policy_path` / `effective_global` = 打底默认后的实际生效额度）。
- **effective 与 policy 必须分开显示**：policy 是带缺席字段的部分对象，用户只改
  一项时其余项线上就是 0；客户端没有「内置基线」这份数据，把 0 显示成「封禁」
  就是编造。页面只渲染 `effective_global` 里 `has*` 为真的项。
- `revision` 0 = 从未落盘（仍在内置基线），不是「版本号坏了」。
- **本波只读**：`SetRuntimePolicy` 是甲级写操作（完整策略 + revision CAS），
  写入面留后续波次（见 §3 末的 runtime 域取舍清单）。
- 错误码：同上（`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL` +
  `HOST_UNAVAILABLE` / `DECODE_FAILED`）。

### 3.13 `getControlPlane()` —— proto `RuntimeService.GetControlPlane`（B5 Settings ③ 追加）

- 返回字节 → gen stub `ControlPlane`（daemon 版本 / 主机名 / 系统 / 启动时刻 /
  `domains[]` 各功能域状态与可控点 / `data_dir_label`）。
- **人话全部由 daemon 给**：`title` / `summary` / `label` / `hint` 客户端不翻译，
  只按 `id` 匹配与排版；新域上线只是多一个条目，客户端不改版面。
- `status_code` 如实映射：2 可能只是「未装配/取不到」，客户端把 `summary` 原文
  显示出来即可，**不自行把 2 渲染成「故障」**。
- **只给标签不给绝对路径**：proto 明示路径里的用户名属敏感信息；落盘全路径在
  各策略响应的 `policy_path`。
- 错误码：同上。

### 3.14 `listUnknownEvents([request])` —— proto `RuntimeService.ListUnknownEvents`（B5 Settings ③ 追加）

- 返回字节 → gen stub `ListUnknownEventsResponse`（`items[]` 摘要 + `total` +
  `dropped`）。
- **空列表 = 真空态**（daemon 台账没登记过未知事件）；**通道不可用是另一条路径**
  （`HOST_UNAVAILABLE` → 页面错误态），两者严格区分，不许用空列表冒充错误
  （防假空态，同 listThreads / listTerminals 纪律）。
- **`total`/`dropped` 必须呈现**：只回最近 N 条而不说，用户会以为「就这么几条」；
  台账脚注把「更早的已丢弃」变成明面事实。
- 请求五筛选项（kind/agent_id/thread_id/processing_status/limit）原样过桥，
  Dart 侧不写死任何筛选默认值（方法面保持「请求消息原文编码」）。
- 错误码：同上。

### 3.15 `getUnknownEvent([request])` —— proto `RuntimeService.GetUnknownEvent`（B5 Settings ③ 追加）

- 返回字节 → gen stub `UnknownEventResponse`（`summary` + `raw_json` 脱敏原文 +
  `decoded_fields_json` + `digest[]` 字段透析表 + `translation` 人话解读 +
  `recommended_next_step` / `support_bundle_hint`）。
- **原文永远可查**：`summary` 的翻译是摘要不是裁决，冲突以 `raw_json` 为准 ——
  详情页必须能把原文呈现出来（密钥类字段已由 daemon 打码）。
- `digest[]` 旧 daemon 为空：整块不出现，不占位不编造。
- **只读**：本方法只拉详情；`AcknowledgeUnknownEvent` /
  `ExecuteUnknownEventAction` 是甲级写操作，本波不接（见 §3 末取舍清单）。
- 错误码：同上。

### 3.16 runtime 域 RPC 全集与本波取舍（B5 Settings ③ 记账）

Dart 侧已实现（6）：`getRuntimeState`（§3.11）/ `getRuntimePolicy`（§3.12）/
`getControlPlane`（§3.13）/ `listUnknownEvents`（§3.14）/ `getUnknownEvent`
（§3.15）+ `getCollabPolicy`（§3 表 #20，协同波次先落的同域只读面）。

⚠️ 「已实现」说的只是 **Dart 面**；宿主桥接线状态一律看 §3 表末列——其中 5 条
runtime 只读（`getRuntimeState` / `getRuntimePolicy` / `getControlPlane` /
`listUnknownEvents` / `getUnknownEvent`）标 `有意未接`（理由见 §8），在 Android
壳里调会拿到 `HOST_UNAVAILABLE`；`getCollabPolicy` 是 `已接`。

**不列入 §3 契约表的 proto RPC（7，留后续波次，注释交接）**——它们既没有方法名、
Dart 面也没有调用点，与 §8 的「有意未接」不是一回事：

| RPC | 类别 | 为什么本波不接 |
|---|---|---|
| `SetRuntimePolicy` | 甲级写 | proto 明文「提交完整策略不是补丁 + revision CAS」；只提交一个字段会把其余项重置回默认（daemon 侧实测踩过） |
| `SetCollabPolicy` | 甲级写 | 同款完整策略 + CAS 语义 |
| `AcknowledgeUnknownEvent` | 甲级写 | 改台账处置状态，需确认流 |
| `ExecuteUnknownEventAction` | 甲级写 | 白名单动作 + 乐观并发（conflict 是结构化结果不是 error） |
| `GetAgentRuntime` | 只读深查 | 按 agent_id 逐个查资源/记忆/检查点/恢复四项，属 console「逐家深查」面，本波只做总览三块 |
| `GetResourceUsage` | 只读深查 | 资源租约观察值，同上属深查面 |
| `ListThreadMemories` / `ClearThreadMemory` | 只读 + 写 | Kotlin 侧清单与清除在同一个弹层里，拆开接只剩半个功能 |

console 页（`flutter/lib/pages/settings_console.dart`）因此**没有任何写通道**：
不摆开关、不摆确认/执行按钮，只读呈现三块 + 未知事件详情。门控纪律：
`runtime.v1` 位不在 → 三块内容整块不渲染、只给一句明话，且不产生任何 runtime
域调用（旧 daemon 上这些 RPC 必然 UNIMPLEMENTED）。

## 4. 事件清单（EventChannel `dev.pocketagent/bridge/events`）

payload 统一为 Envelope Map：`type` 判别 + 载荷字段：

| `type` | 载荷 | Dart 类型 | 说明 |
|---|---|---|---|
| `conn_state` | 五键与 `type` 平铺（同 §3.4 键表） | `PaConnEvent.connState` → `PaConnState` | 五条 StateFlow 任一变化 → 推**全量快照**一帧（不发单字段增量，避免乱序拼装） |
| `host_info` | `info`: `HostInfo` wire 字节 | `PaConnEvent.hostInfo` → gen stub | 主机信息/能力位变化推送 |
| `host_event` | `event`: `HostEvent` wire 字节 | `PaConnEvent.hostEvent` → gen stub | Kotlin `hostEvents`（`HostService.WatchEvents` 全局流；`sequence` 与 `TurnServerMessage.sequence` 同一序列空间，断线续传水位） |

语义：

- **无缓冲 broadcast**：订阅即收，订阅前推过的事件不补（与 Kotlin
  `MutableSharedFlow(replay=0)` 一致）；初值用 `getConnectionState()` 补。
- **载荷字段严格在场**：`PaConnEvent` 上非本类型的载荷字段恒为 null。
- **坏事件不毒化流**：payload 违约 → 流错误 `DECODE_FAILED`（消息定位到键），
  流保持打开，后续事件照发（断连重连会连续推多帧，一帧坏不能杀整条流）。
- **未知 `type` = 流错误 `DECODE_FAILED`**，不静默吞：双端打进同一个 APK
  （双端同版政策），未知类型只能是契约漂移，需同步改本文档 + `pa_bridge.dart`。
- 宿主 `EventSink.error` → 流错误，code 原样透传。
- 宿主缺席（通道无 handler）**不产生流错误**（见 §2.5）——宿主存活探针走方法面。

### 4.1 另两条事件通道：裸 wire 字节（**不走**上面的 Envelope Map）

`dev.pocketagent/bridge/turn_events` 与 `dev.pocketagent/bridge/terminal_events`
每一帧就是**一条 proto 消息的 wire 字节**（`Uint8List`）——没有 `type` 判别、没有
Map 包装：

| 通道 | 帧 | Dart 解码点 | 宿主推送点 |
|---|---|---|---|
| `bridge/turn_events` | `TurnServerMessage` wire 字节 | `chat_turn_stream.dart` 的 `_decodeFrame`（:202）→ `bytesOrThrow` | `BridgeEventChannels` 的 StartTurn 会话（裸字节直推） |
| `bridge/terminal_events` | `TerminalServerMessage` wire 字节 | `terminal_stream.dart` 的 `_decodeEvent`（:196）→ `bytesOrThrow` | `BridgeEventChannels` 的 Attach 会话（裸字节直推） |

- **为什么不套 Envelope**：内容类型由**通道名**唯一确定（一通道一消息），再包一层
  `type` 判别等于把同一条信息写两遍；判别位只留给「一条通道多种载荷」的主事件面。
- **坏帧一样不毒化流**：载荷违约 → 流错误 `DECODE_FAILED`，流保持打开（与本节
  「坏事件不毒化流」同纪律），后续帧照收。
- **续传水位**：`turn_events` 的 `TurnServerMessage.sequence` 与 `turnResync`
  （`GetThreadEventsSince` 的 `after_sequence`，§3 表 #28）同一序列空间。

## 5. 错误码（`PaBridgeErrorCodes`，UI 按 code 分支）

| 码 | 产生方 | 触发 |
|---|---|---|
| `HOST_UNAVAILABLE` | Dart | 通道无 handler：B4 未实现 / 宿主进程未起 |
| `DECODE_FAILED` | Dart | 宿主返回值或事件 payload 与本契约不符（结构/字节/键/类型） |
| `NOT_CONNECTED` | 宿主（B4） | 尚未连上 daemon 时调用连接类方法 |
| `AUTH_REQUIRED` | 宿主（B4） | token 缺失/过期，需先配对 |
| `INVALID_ADDR` | Dart / 宿主 | Dart：`connect` 空地址本地即拒；宿主：地址不可解析 |
| `PAIR_DISABLED` | 宿主（B4） | 配对码不可读（非回环来源 / 配对未开启） |
| `INTERNAL` | 宿主（B4） | 兜底：宿主内部错误，message 须给人话原因 |

宿主 code 不在表内时 Dart 原样保留（透传不改写）；新增宿主错误码必须回填本表。

## 6. Kotlin 宿主侧接线状态（B4 波次：**已落地**）

装配入口 `installBridge`（`bridge/BridgeMethods.kt`）由
`flutterhost/FlutterHostEngine.installChannelShell` 在 FlutterEngine 挂载时调用，
三件套一次装好：事件面 `BridgeEventChannels` → 双向流 `BridgeStreamWiring` →
方法面 `BridgeMethods`（顺序不可换：methods 的 dispatch 要经 wiring 取 stub 与
sink；wiring 又要持 channels 的建流 machinery）。

- [x] 在 FlutterEngine 挂载时注册 MethodChannel `dev.pocketagent/bridge`
      handler：方法名按 §3 表逐字对齐（**30 条 dispatch 分支**；另 15 条「有意未接」
      见 §8）。
- [x] 返回值一律走 proto `writeToBuffer()` 字节（与 gRPC 契约同源，复用
      `HostInfo`/`HostEvent`/`TurnInput`/`ListThreadsResponse` 的既有 Kotlin 生成类）。
- [x] `getConnectionState`：直接取 `AgentConnection` 五条 StateFlow 当前值拼
      §3.4 五键 Map（键名 = 字段名；五键恒有值，StateFlow 语义）。
- [x] `listThreads` / `listDirectory` / `listBranches`：转发对应 daemon RPC，
      `repeated` 字段按 proto 原样字节回传（不给手机端做投影/筛选，那是页端的事）。
- [x] 双向流六条（`turnStreamStart`/`Send`/`Cancel` + `terminalAttach`/`Write`/
      `Close`）：经 `BridgeStreamWiring` seam 取 async stub 与 sink；seam 未注入或
      sink 不在即 `HOST_UNAVAILABLE` **点名缺什么**，不建「建了流却收不到帧」的假流；
      建流后有界等待请求 observer 就绪（`STREAM_READY_TIMEOUT_MS`），保证紧随的首帧
      有落点。
- [x] `turnResync`：`GetThreadEventsSince` 是 server stream，收齐后一次性返回
      `List<Uint8List>`（§3 表 #28；空数组 = 无新帧）。
- [x] `readFile`：proto 是 server stream，宿主只回**首块** `FileChunk`（§3.9）；
      空流按 §5 `INTERNAL` 显式失败，不拿空消息冒充首块。
- [x] 注册三条 EventChannel 的 StreamHandler：`bridge/events`（`conn_state`
      全量快照帧 + `host_info` + `host_event`，三条各走独立协程 + `supervisorScope`，
      一源挂不杀全流）、`bridge/turn_events`、`bridge/terminal_events`（后两条按 §4.1
      推裸 wire 字节）。
- [x] 错误码对齐 §5：宿主侧只抛表内 code + 人话 message；`BridgeErrors.contractCodeOf`
      是唯一映射表（7 码逐条钉死在 `BridgeErrorsTest`），`pairCode` 走
      `pairCodeContractCodeOf` 专属覆盖（PERMISSION_DENIED → `PAIR_DISABLED`）。
- [x] `pairCode` 复用 `GetPairingInfo` 的回环来源自检（安全语义不得放松）；
      配对未开启或码为空一律 `PAIR_DISABLED`，不用空返回代替错误。
- [x] 方法面与事件面**共享同一个 `AgentConnection` 实例**：`host_info` 帧只来自
      「经本实例的 `updateHostInfo` 成功路径」，两个连接各走各的会让 `host_info` 帧
      永远静默。
- [x] 未知方法名走 `else` 分支：`HOST_UNAVAILABLE` + 把「Android 桥有意未接（§8）」
      与「契约漂移（§3 未同步）」两种可能并列写出，指向本文档，由排查者裁决
      （不替人下结论、也不在代码里写死方法名单）。
- [ ] **真机验证待设备**：以上装配与链路目前只有 JVM 单测（`BridgeMethodsTest` /
      `BridgeErrorsTest` / `BridgeEventChannelsTest` / `FlutterHostBridgeInstallTest`）
      与 Dart 侧 mock 宿主 wire 覆盖；`adb devices` 当前**无设备**，真机冷启、事件面
      实推、双向流首帧落地均未在真机验过（2026-10-17）。

## 7. 类型来源总表（哪些直接用 gen 消息 / 哪些是小类）

| Dart 类型 | 来源 | 用途 |
|---|---|---|
| `HostInfo` / `HostEvent` / `TurnInput` / `GetHostInfoRequest` | gen stub（`lib/gen/pocketagent/v1/`，protoc 生成勿手改） | 方法返回 / 方法入参 / 事件载荷 |
| `PaConnState` | 本契约小类 | 连接态五键（proto/v1 无对应消息——宿主进程内部态不经过 daemon，不造假同源） |
| `PaConnEvent` | 本契约小类 | 事件 Envelope（`type` + 三种载荷） |
| `PaBridgeException` / `PaBridgeErrorCodes` | 本契约小类 | 错误模型（§5） |
| `RuntimeStateResponse` / `RuntimePolicyResponse` / `ControlPlane` / `ListUnknownEventsResponse` / `UnknownEventResponse` / 五条请求消息 | gen stub（`lib/gen/pocketagent/v1/runtime.pb.dart`） | runtime 域 5 条只读方法（§3.11–§3.15）的返回/入参；**零小类包装**——这些消息就是契约本身，页面侧的展示投影（额度行/标签词表）是页端纯函数，不进通道层 |

## 8. 宿主桥「有意未接」清单（15 条 —— D-1 的文档半）

§3 表末列标 `有意未接` 的方法名：**Dart 侧可以调**（代码在、测试在、UI 在用），
**Android 宿主桥故意不注册** —— 调用会拿到 `HOST_UNAVAILABLE`。这不是契约漂移，
也不是「忘了做」；把它标出来是为了让 Dart 调用方与排查者一眼分得清。

**共同原因（一条）**：Android 自有 UI 走 `AgentConnection.<service>()` 直接打 gRPC
stub，根本不经过 MethodChannel。同一个 daemon RPC 在两个壳里各有一条调用路径，
桥上再注册一份只会多出一条当前零消费者的路径 —— 加分支等于扩 RPC 契约面
（甲级面，无消费者不摆假入口，红线：不新增写死的名单 / 不摆假入口）。

| 组 | 方法 | Android 侧实际路径（不过桥） |
|---|---|---|
| runtime 域只读 5 条 | `getRuntimeState` / `getRuntimePolicy` / `getControlPlane` / `listUnknownEvents` / `getUnknownEvent` | `AgentConnection.runtimeService()`（settings_console 的 console 页直接消费） |
| Chat 周边动作 10 条 | `submitApproval` / `cancelTurn` / `clearQueuedInputs` / `compactThread` / `forkThread` / `rollbackThread` / `setThreadGoal` / `startReview` / `exportThreadHtml` / `optimizePrompt` | `AgentConnection.threadService()`（`ChatViewModel` 的同名 suspend 函数） |

**处置纪律**：

1. **不加分支**。要接就走 §3 其余 30 条的同款写法（`parseProto` 收字节 → `rpc(method)`
   转发 → `toByteArray()` 回传），并在 §3 表把末列改成 `已接`；**改完机器立刻红**
   ——那是提醒你顺手把 §8 的对应行删掉，不是提醒你补文档。
2. **兜底文案不许替人下结论**。`BridgeMethods` 的 `else` 分支拿不到「这是有意未接
   还是真漂移」的信息（区分它就得在代码里写死方法名单），所以它把两种可能**并列**
   写出来并指向本文档，由排查者对照 §3 末列与本节裁决。
3. **这两组方法的 Dart 面仍然要真接**。它们是 Flutter UI 的活路径（如 console 页的
   总览、Chat 的审批/压缩/导出按钮），在 Flutter 自己的壳里走独立宿主，走 Android
   壳时才拿 `HOST_UNAVAILABLE`——UI 必须有诚实占位，不许拿空态冒充成功。

## 附录 A：Dart gen stub 重生成（2026-10-14 本机实测）

版本锚：`protoc` libprotoc **36.1**（`C:\protoc\bin\protoc.exe`）、
`protoc-gen-dart`（protoc_plugin **25.1.0**，`$env:PUB_CACHE\bin`）、
运行时 `protobuf ^6.1.0` / `fixnum ^1.1.1`（`pubspec.yaml`，生成头声明即此版本）。

```powershell
# 仓库根执行；protoc-gen-dart.bat 需要 dart 在 PATH，PUB_CACHE\bin 需在 PATH
$env:PATH = "$env:PUB_CACHE\bin;C:\src\flutter\bin;$env:PATH"
C:\protoc\bin\protoc.exe -I proto --dart_out=grpc:flutter/lib/gen/pocketagent proto/v1/*.proto
```

- 输出路径 = proto 相对路径（`-I proto` → `v1/*.proto` →
  `flutter/lib/gen/pocketagent/v1/*.dart`，31 个文件）。
- **实测记录（2026-10-14）**：同套参数输出到临时目录后与
  `flutter/lib/gen/pocketagent/v1/` 现有 31 个文件逐字节 diff（0 差异）——
  本文档的命令是可复现的真命令，不是事后追忆。
- 重生成后必过：`flutter analyze`（零问题）+ `flutter test`
  （`test/gen/wire_fixture_test.dart` 会钉死线格式漂移）。

## 附录 B：wire fixture 重生成（通道测试的字节来源）

```bash
python flutter/tools/protofixture/gen_fixtures.py   # protoc 现生成 + python protobuf marshal
```

产物 `tools/protofixture/fixtures/*.b64` + `manifest.json`（生成环境账本）。
`test/transport/pa_bridge_test.dart` 的 `getHostInfo`/`host_info`/`host_event`
用例直接读 `host_info.b64` / `host_event.b64`：通道层反序列化必须与 wire 层
对账用的是**同一份第三方编码器字节**，不手搓常量消息自说自话。
