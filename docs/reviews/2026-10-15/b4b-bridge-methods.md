# B4b：Kotlin 宿主桥方法面处理器（`dev.pocketagent/bridge` MethodChannel）

> 日期：2026-10-16 · 波次：B4b（宿主桥方法面）
> 范围：`android/app/src/main/java/dev/pocketagent/app/bridge/`（新建 5 文件）+
> `android/app/src/test/java/dev/pocketagent/app/bridge/`（新建 4 文件）。
> **未 git add / commit / push。**

---

## 一、一句话结论

`BridgeMethods` 落盘：契约 §3 三十条方法中 **24 条接通**（unary + 两类 server-stream
的一次性形态），**6 条双向流生命周期方法如实回 `HOST_UNAVAILABLE`**（过程帧消费面
= EventChannel，不在本波），未知方法同样 `HOST_UNAVAILABLE` 且消息点名契约漂移。
gRPC 状态码 → 契约错误码的映射表收敛成一个纯函数并逐条钉死；所有返回值走 proto
`writeToBuffer()` 字节，测试全部**字节级**断言。**68 条新测试全过**，总水位
1946 用例（基线 1849），4 条失败全部来自并行子代理在飞的 `flutterhost` 测试
（详见 §八）。

---

## 二、文件清单（全部未 commit）

**主源码（5 个，均在 `android/app/src/main/java/dev/pocketagent/app/bridge/`）**

| 文件 | 行数 | 职责 |
|---|---|---|
| `BridgeErrors.kt` | ~180 | 7 个契约错误码常量 + `BridgePlatformException`（Flutter PlatformException 的 Kotlin 镜像）+ gRPC `Status.Code` → 契约码映射表（`contractCodeOf` / `pairCodeContractCodeOf` / `grpcFailure`） |
| `BridgeChannel.kt` | ~95 | Flutter `MethodChannel` 三件套的最小镜像（`BridgeMethodCall` / `BridgeResult` / `BridgeBinaryMessenger`）+ 通道名常量 `BRIDGE_CHANNEL_NAME` |
| `BridgeConnection.kt` | ~380 | 六个 gRPC 服务的**窄接口**（`BridgeHostService` 等）+ `BridgeConnection` seam + 生产适配器 `AgentConnectionBridge` |
| `BridgeMethods.kt` | ~445 | 方法面处理器：`when(method)` 派发 30 条 + 三种入参解析（字节/Map/null）+ `rpc()` 错误收口 + 线程纪律 |
| `BridgeFlutterWiring.kt` | ~60 | 生产入口 `BridgeMethods.install(messenger, connection)`：真 Flutter `BinaryMessenger` → 桥内部 seam 的一一适配 |

**测试（4 个，均在 `android/app/src/test/java/dev/pocketagent/app/bridge/`）**

| 文件 | 用例 | 覆盖 |
|---|---|---|
| `BridgeTestFixtures.kt` | —（装配） | 六个服务的 fake（窄接口直实现）+ `FakeConnection` + `RecordingResult`/`RecordingMessenger` + `grpcError()` + 15 个响应消息构建 helper |
| `BridgeErrorsTest.kt` | 14 | 映射表逐条（UNAVAILABLE/UNAUTHENTICATED/INVALID_ARGUMENT/DEADLINE_EXCEEDED/PERMISSION_DENIED/NOT_FOUND/UNIMPLEMENTED）+「只有三个状态码有专属映射」反向钉死 + message 口径 + 七码常量逐字 + 通道名逐字 |
| `BridgeMethodsTest.kt` | 48 | 24 个已实现方法的请求解析→stub 调用→**字节级**响应断言；6 个未实现方法→HOST_UNAVAILABLE；未知方法/大小写；错误码在真实方法面上的落地；`getConnectionState` 五键形状；`readFile` 首块/空流；`turnResync` 收齐/空流/坏水位 |
| `BridgeFlutterWiringTest.kt` | 6 | `install` 注册在契约通道名；成功返回字节形态；错误三件套形状；Map 入参原样转发；未实现方法不留假实现；`notImplemented` 语义保留 |

---

## 三、方法面对账表（契约 §3 ↔ 实现）

「入参形态」列是**实测 Dart 调用点**的形态（`pa_bridge.dart` /
`chat_turn_stream.dart` / `terminal_stream.dart` / `chat_history_transport.dart` /
`chat_input_transport.dart`），不是 §3 表的旧描述——两处不一致已在 §3.1/§3.2 记录。

| # | 方法名 | proto RPC（服务） | 入参形态 | 返回 | 状态 |
|---|---|---|---|---|---|
| 1 | `getHostInfo` | `HostService.GetHostInfo` | `GetHostInfoRequest` 字节（空消息） | `HostInfo` 字节 | ✅ |
| 2 | `connect` | —（`setCredentials` + 异步 `connectWithFallback`） | `{'addr': String, 'token': String}` | null | ✅ |
| 3 | `disconnect` | —（`shutdown` + connState=down） | null | null | ✅ |
| 4 | `getConnectionState` | —（五条 StateFlow 现值） | null | Map 五键 | ✅ |
| 5 | `pairCode` | `HostService.GetPairingInfo` | null | 非空 String | ✅ |
| 6 | `startTurn` | `TurnService.StartTurn`（双向流首帧） | `TurnInput` 字节 | null | ⛔ HOST_UNAVAILABLE |
| 7 | `listThreads` | `ThreadService.ListThreads` | `ListThreadsRequest` 字节 | `ListThreadsResponse` 字节 | ✅ |
| 8 | `listDirectory` | `FileSystemService.ListDirectory` | `ListDirectoryRequest` 字节 | `ListDirectoryResponse` 字节 | ✅ |
| 9 | `readFile` | `FileSystemService.ReadFile`（server stream） | `ReadFileRequest` 字节 | `FileChunk` **首块**字节 | ✅ |
| 10 | `listBranches` | `GitService.ListBranches` | `ListBranchesRequest` 字节 | `ListBranchesResponse` 字节 | ✅ |
| 11 | `listCollabRoles` | `ThreadService.ListCollabRoles` | 空消息字节（**不是 null**，见 §3.1） | `ListCollabRolesResponse` 字节 | ✅ |
| 12 | `setCollabRole` | `ThreadService.SetCollabRole` | `SetCollabRoleRequest` 字节 | `SetCollabRoleResponse` 字节 | ✅ |
| 13 | `deleteCollabRole` | `ThreadService.DeleteCollabRole` | `DeleteCollabRoleRequest` 字节（**不是 Map**，见 §3.1） | `DeleteCollabRoleResponse` 字节 | ✅ |
| 14 | `dispatchCollab` | `ThreadService.DispatchCollab` | `DispatchCollabRequest` 字节 | `DispatchCollabResponse` 字节 | ✅ |
| 15 | `getCollabSession` | `ThreadService.GetCollabSession` | `GetCollabSessionRequest` 字节（**不是 Map**） | `CollabSessionSnapshot` 字节 | ✅ |
| 16 | `listCollabSessions` | `ThreadService.ListCollabSessions` | `ListCollabSessionsRequest` 字节 | `ListCollabSessionsResponse` 字节 | ✅ |
| 17 | `cancelCollabSession` | `ThreadService.CancelCollabSession` | `CancelCollabSessionRequest` 字节（**不是 Map**） | `CollabSessionSnapshot` 字节 | ✅ |
| 18 | `cancelCollabMember` | `ThreadService.CancelCollabMember` | `CancelCollabMemberRequest` 字节（**不是 Map**） | `CollabMemberSnapshot` 字节 | ✅ |
| 19 | `retryCollabMember` | `ThreadService.RetryCollabMember` | `RetryCollabMemberRequest` 字节（**不是 Map**） | `CollabMemberSnapshot` 字节 | ✅ |
| 20 | `getCollabPolicy` | `RuntimeService.GetCollabPolicy` | 空消息字节 | `CollabPolicyResponse` 字节 | ✅ |
| 21 | `listTerminals` | `TerminalService.ListTerminals` | 空消息字节 | `ListTerminalsResponse` 字节 | ✅ |
| 22 | `terminalAttach` | `TerminalService.Attach`（双向流） | `{'terminalId': String}` | null | ⛔ HOST_UNAVAILABLE |
| 23 | `terminalWrite` | `TerminalService.Attach`（发帧） | `TerminalClientMessage` 字节 | null | ⛔ HOST_UNAVAILABLE |
| 24 | `terminalClose` | `TerminalService.CloseTerminal` | `{'terminalId': String}` | null | ✅ |
| 25 | `turnStreamStart` | `TurnService.StartTurn`（建流） | `{'threadId': String}` | null | ⛔ HOST_UNAVAILABLE |
| 26 | `turnStreamSend` | `TurnService.StartTurn`（发帧） | `TurnClientMessage` 字节 | null | ⛔ HOST_UNAVAILABLE |
| 27 | `turnStreamCancel` | —（拆流） | null | null | ⛔ HOST_UNAVAILABLE |
| 28 | `turnResync` | `ThreadService.GetThreadEventsSince`（server stream） | `{'threadId': String, 'afterSequence': int}` | `List<Uint8List>` | ✅ |
| 29 | `getThreadHistory` | `ThreadService.GetThreadHistory` | `GetThreadHistoryRequest` 字节 | `GetThreadHistoryResponse` 字节 | ✅ |
| 30 | `listSlashCommands` | `ThreadService.ListSlashCommands` | `ListSlashCommandsRequest` 字节 | `ListSlashCommandsResponse` 字节 | ✅ |

**统计：24 接通 / 6 未接通。**

### 3.1 与 §3 表不符、以 Dart 代码为准的两处（已核实）

1. §3 表把 #13/#15/#17/#18/#19 的 arguments 写成 `{'id': String}` /
   `{'sessionId': String}` 类 Map，但 Dart 实际发的是**请求消息 wire 字节**
   （可选参数缺省 = 空消息字节）。契约总则 §2.1「有 proto 请求消息的方法 =
   wire 字节」与代码一致，**表是旧的**。实现按字节收；若收到 Map →
   `DECODE_FAILED`（红线：不做 null/bytes 两种形态的版本二分）。
   测试 `getCollabSession 若收到 Map 形态回 DECODE_FAILED_不做双形态二分` 钉死。
2. §3 表写 #11 `listCollabRoles()` arguments = null，Dart 实际发
   `ListCollabRolesRequest().writeToBuffer()`（**空字节数组**，§2.1 同 getHostInfo）。
   实现按字节收（`ByteArray(0)` 合法）。

### 3.2 六条未实现的方法：为什么整族留后

`startTurn` / `terminalAttach` / `terminalWrite` / `turnStreamStart` /
`turnStreamSend` / `turnStreamCancel` 全部依赖**双向流的生命周期管理**，而过程帧
（增量/审批/完成、PTY 输出）的消费面在事件通道（`bridge/turn_events`、
`bridge/terminal_events`）——那不在本波。本波只建流不收帧 = 用户发了指令却永远
看不到任何回帧，那是比「通道未就绪」更糟的假象（看起来连着，其实瞎了）。故整族
回 `HOST_UNAVAILABLE`，消息点名留后事项与对应事件通道名，页面走诚实占位
（「轮次通道未就绪（B4 波次）」）。

`terminalClose`（#24）**实现了**：它是 unary `CloseTerminal`，daemon 侧关 PTY 是
完整动作，不依赖流生命周期。

---

## 四、错误码映射表（§5 ↔ gRPC Status.Code）

映射收敛在 `BridgeErrors.contractCodeOf`（一个纯函数，24 个方法共用；漏映射一个
方法就会出现「同一种失败在 A 方法 NOT_CONNECTED、在 B 方法裸 gRPC 异常」）。
**逐条定依据：**

| gRPC `Status.Code` | 契约码 | 依据 |
|---|---|---|
| `UNAVAILABLE` | `NOT_CONNECTED` | 通道未建（首次连接前）/ 半死态（OkHttp UNAVAILABLE 不自愈）/ daemon 被杀 = 「尚未连上 daemon 时调用连接类方法」（§5 NOT_CONNECTED 触发条件）。**不预检 connState**：那会造第二份真值（探活周期内可能过期），由 gRPC 自己说「这条调用确实没通」更诚实 |
| `UNAUTHENTICATED` | `AUTH_REQUIRED` | token 缺失/过期/被吊销；§5 AUTH_REQUIRED 触发条件逐字如此 |
| `PERMISSION_DENIED` | 通用表 `INTERNAL`；**`pairCode` 专属表 `PAIR_DISABLED`** | daemon 拒绝的理由因方法而异（host.go:83-85 的「配对码仅对本机回环来源提供」只在 pairCode 上成立）；通用表不抢专属语义，由 `pairCodeContractCodeOf` 接管 |
| `INVALID_ARGUMENT` | `INTERNAL`（message 透传 daemon 原文） | 契约七码没有「入参被拒」这一档（§6：宿主只抛表内 code）；最诚实的落点是兜底码 + 人话原文。若 §5 将来增码，只改 `contractCodeOf` 一处 |
| `DEADLINE_EXCEEDED` | `INTERNAL` | 电脑端响应超时（宿主侧超时，不是「没连上」）；message 透传 |
| `NOT_FOUND` | `INTERNAL` | daemon 说对象不存在（如协同会话 id）；人话原因由 message 透传（Dart 侧 getCollabSession 文档明说） |
| `UNIMPLEMENTED` | `INTERNAL` | 旧 daemon 缺该 RPC（版本不匹配）；message 退回「RPC \<方法\> 失败: \<码\>」 |
| 其余（`RESOURCE_EXHAUSTED`/`CANCELLED`/`ABORTED`/…） | `INTERNAL` | 兜底码；message 透传 |
| 非 gRPC 异常（本地编码/并发……） | `INTERNAL` | 同样不许裸抛到 Flutter 侧；message 带异常类名与原文 |

**message 口径**：优先 daemon 的 status description（人话）；daemon 没给描述时退回
`"RPC <方法> 失败: <码>"`，**绝不回空串**（§5 要求 INTERNAL 的 message 须给人话原因）。
`details` 恒带 gRPC 状态码名（`UNAVAILABLE` 等），UI 上层可展开诊断。

**反向钉死**：`七种状态码全覆盖_表内码只有约定的三个` 断言
`Status.Code.entries.filter { contractCodeOf(it) != INTERNAL }` 恰等于
`{UNAVAILABLE, UNAUTHENTICATED}` —— 新增映射必须是有意识的决定，不会是顺手加一行。

---

## 五、设计决定与实测记录

### 5.1 `BridgeConnection` 为什么收成窄接口（而不是暴露 blocking stub）

**实测（2026-10-15，三种姿势全部编译失败）**：

1. `class FakeHostStub : HostServiceGrpc.HostServiceBlockingStub` —— 生成类是
   `public final class`（Kotlin 侧 final）+ 构造器 private：
   `"This type is final, so it cannot be extended"`；
2. 继承可继承的 `AbstractBlockingStub<HostServiceBlockingStub>` —— 泛型自引用
   绑定后 `getHostInfo` 报 `"overrides nothing"`（父类里没有那些方法）；
3. Kotlin 默认参数里写 `NoopChannel()` —— `"This type has a constructor, so it
   must be initialized here"`。

结论：把 `BridgeConnection` 的服务面收成**窄接口**（`BridgeHostService` 等，只列
桥真正调用的 RPC）。副作用是接口面顺手钉住「桥没在偷偷调别的 RPC」。生产适配器
把真 stub 包一层 lambda 转发（`AgentConnectionBridge`），仍是「一问一答直通
daemon」，不引入第二份 RPC 实现。

### 5.2 线程纪律

`BridgeMethods.handle` 里 `scope.launch` + `dispatch` 内 `withContext(Dispatchers.IO)`
—— gRPC blocking stub 不上平台线程。`getConnectionState` 的五条 StateFlow 取值是
内存读，理论上是唯一可以同步的，但仍在同一 IO 上下文里执行（统一，无特例）。
`AgentConnectionBridge` 自带一个 IO scope 承载 `connect` 的异步回退连接
（`connectWithFallback` 最坏 ~10s+，绝不能压在 Flutter 平台线程上）。

### 5.3 `connect` / `disconnect` 的语义

- `connect`：`setCredentials(addr, token)`（地址进 prefs、token 进敏感库）+
  异步 `connectWithFallback(freshChannel = true)`。返回 null = **请求受理**
  （§3.2），真值由 connState / connectionEpoch 异步体现。
  `freshChannel = true` 与「用户显式重试」同档（半死传输丢弃）。
- `disconnect`：`shutdown()` + `connState.value = "down"`，让快照与 `conn_state`
  事件立刻反映「用户主动断开」，而不是停留在上一次探活的 "ok"。
  **已知边界**：探活循环随后可能自动重连——那是 app 既有行为，桥不新增
  「用户要求断开」标志位（不在本波改 AgentConnection）。

### 5.4 `getHostInfo` 的共享快照写入

桥的 `getHostInfo` 成功路径调 `connection.updateHostInfo(info)`，与探活循环同口径
（AgentConnection.updateHostInfo 的 KDoc：所有成功路径写 capabilities 共享快照）。
漏写会让某条渲染链按空 capabilities 渲染（T-DSH 问题 A 的回归形态）。
测试 `getHostInfo 成功路径写 capabilities 共享快照_与探活循环同口径` 钉死。

### 5.5 `readFile` 的首块与空流

proto 是 server stream（64KB 分块）；桥只取 `iterator.next()`（首块）。
**空流 = 违约形态**（首块必带 total_size/text）：回 `INTERNAL` 并说明，不拿空
`FileChunk` 让 Dart 侧以为「文件为空」。分块流的完整消费面（PaDownloader）不走
本方法。

### 5.6 `turnResync` 的 server-stream 收齐

Dart 面无「请求一次回一段流」的形态（EventChannel 是宿主推流），契约定为收齐后
一次性返回 `List<Uint8List>`（空 = 无新帧，**合法返回不是错误**）。
`afterSequence` 必须是数字（StandardMethodCodec 解出的 Integer）；非数字 →
`DECODE_FAILED`（契约漂移显式失败）。

### 5.7 与并行子代理（flutterhost）的接线面

`flutterhost/FlutterHostEngine.installChannelShell` 目前把方法面一律回
`HOST_UNAVAILABLE`（B4a 的诚实壳）。本波提供
`BridgeMethods.install(messenger, AgentConnectionBridge())` 一行接线（见
`BridgeFlutterWiring.kt`），其 KDoc 已说明「`BridgeMethods` 落盘后把本方法内的
MethodChannel handler 换掉」——**接线那一行归 flutterhost 子agent/主会话**，
本波不碰 `flutterhost/` 任何文件。

---

## 六、测试证据（原文）

构建命令（隔离镜像，中文路径不可用）：

```powershell
$env:PA_MIRROR='G:\pa_b4b'; python tools/device/_pa_gradle.py test
```

**基线（开工前，同一镜像同一命令）**：

```
=== 1. robocopy 同步 ===
  robocopy rc = 1 (0/1/2/3 均为成功)
=== 2. gradle :app:testDebugUnitTest --rerun ===
  gradle rc = 0  elapsed = 88.6s
  log -> C:\Users\yanzi\_pa_gradle_test.log
--- 关键行 ---
  BUILD SUCCESSFUL in 1m 28s
```

基线 XML 清点：`files=196 tests=1849 failures=0 errors=0`。

**本波收尾（同命令，`--rerun` 强制重跑；末次稳定态 15:43:23）**：

```
=== 1. robocopy 同步 ===
  robocopy rc = 3 (0/1/2/3 均为成功)
=== 2. gradle :app:testDebugUnitTest --rerun ===
  gradle rc = 1  elapsed = 38.2s
  log -> C:\Users\yanzi\_pa_gradle_test.log
--- 关键行 ---
  FlutterHostChannelShellTest > all three event channels accept listen and cancel without pushing FAILED
  1946 tests completed, 1 failed
  > Task :app:testDebugUnitTest FAILED
  BUILD FAILED in 38s
```

XML 清点（202 份，全部时间戳 `15:43:23` 同一批，无陈旧文件）：

```
dev.pocketagent.app.bridge.BridgeErrorsTest:        tests=14 failures=0 errors=0
dev.pocketagent.app.bridge.BridgeFlutterWiringTest: tests=6  failures=0 errors=0
dev.pocketagent.app.bridge.BridgeMethodsTest:       tests=48 failures=0 errors=0
bridge total = 68
all: files=202 tests=1946 failures=1 errors=0
```

**用例总数对照**：1849（基线）→ **1946**（本波），净增 **97** = 本波 68 +
并行子代理 flutterhost 的 29（`FlutterHostActivityTest` / `FlutterHostEngineTest`
/ `FlutterHostChannelShellTest` 等，非本波所为）。

**仅剩的 1 条失败不是本波代码**：`FlutterHostChannelShellTest > all three event
channels accept listen and cancel without pushing`，失败原因
`java.nio.BufferUnderflowException`（该测试自己用 `StandardMethodCodec` 解
ByteBuffer 时的边界问题，报错栈落在 `FlutterHostChannelShellTest.kt` 内部，与
本波 `bridge/` 零交集）。**动态过程记录**：本波收尾期间并行子agent在飞——
中途一度 4 条失败（含其自身编译不过），随后其自行修到 1 条；本波 68 条全程
稳定通过，从未因并行线波动而红。

---

## 七、对账差异登记（任务书要求「以实际代码为准并记录差异」）

| 项 | 预期（任务书/契约表） | 实际 | 处理 |
|---|---|---|---|
| `AgentConnection` 的方法面 | `hostService()` / `threadService()` / `threadServiceAsync()` / `terminalService()/Async()` | 与预期一致（另有 `fileService()` / `gitService()` / `runtimeService()` 一并使用，§3 的 24 条需要它们） | 无差异 |
| §3 表 arguments 形态 | 协同 5 条 + `listCollabRoles` 为 Map/null | Dart 实际发请求消息**字节** | 以 Dart 代码为准，实现按字节收；Map → DECODE_FAILED（§3.1 已记录） |
| flutter embedding 可用性 | 开工时实测**不可用**（`io.flutter.*` 全 Unresolved，探针文件编译失败），故 `BridgeChannel.kt` 先做镜像 seam | 并行子agent落地 `settings.gradle.kts` 的 `include(":flutter")` + `implementation(project(":flutter"))` 后**可用** | 保留 seam（测试仍用 fake 驱动，不建真引擎）+ 增补 `BridgeFlutterWiring.kt` 生产适配，双轨都通 |
| 镜像构建 | `PA_MIRROR='G:\pa_b4b'` 一条命令即可 | 主会话把 flutter module 挂进 android 构建后，镜像需 `G:\pa_b4b\flutter`（junction/拷贝）才够 `settings.gradle.kts` 的模块查找 | 本机已就绪；报告记录该环境依赖 |

---

## 八、遗留项（如实交接）

1. **6 条双向流方法未接通**（§3.2）：待 EventChannel（`bridge/turn_events` /
   `bridge/terminal_events`）落地后，按 `dispatch` 里对应分支的注释接流。
2. **`flutterhost/FlutterHostEngine.installChannelShell` 尚未换接本桥**：
   一行接线（`BridgeMethods.install(messenger, AgentConnectionBridge())`），
   归 flutterhost 子agent/主会话；本波不碰该目录。
3. **并行子agent 的 1 条 `FlutterHostChannelShellTest` 失败**（BufferUnderflow）
   需其自行收敛；与本波零交集。
4. **§5 表无「入参被拒」码**：`INVALID_ARGUMENT` 现落 `INTERNAL` + 原文透传；
   若将来 §5 增码，只改 `BridgeErrors.contractCodeOf` 一处。
5. **`disconnect` 后探活可能自动重连**：既有行为，未新增「用户要求断开」标志位。

---

## 九、未触碰清单（逐项核对）

`git status --short --untracked-files=all -- android` 实证：本波改动只有
`android/app/src/main/java/dev/pocketagent/app/bridge/`（5 新文件）与
`android/app/src/test/java/dev/pocketagent/app/bridge/`（4 新文件）。
**未碰**：`android/` 下其他任何文件（`build.gradle.kts` / `settings.gradle.kts` /
`AndroidManifest.xml` / `MainActivity.kt` / `AgentConnection.kt` / `flutterhost/`
等均为并行子agent或主会话所为）、`daemon/`、`proto/`、`flutter/`、`tools/`。
**未 git add / commit / push**（`git status` 显示本波 9 个文件全为 `??` untracked）。
