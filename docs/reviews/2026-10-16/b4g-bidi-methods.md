# B4 接线第五步：六个双向流生命周期方法面接通（B4g）

日期：2026-10-16
范围：`turnStreamStart` / `turnStreamSend` / `turnStreamCancel` +
`terminalAttach` / `terminalWrite` / `terminalClose` 六条方法，从
HOST_UNAVAILABLE 诚实缺席 → 真 gRPC 双向流 + 与 B4d 事件面联动。

## 1. 结论

六条方法的 dispatch 分支已全部接通，走**真 gRPC 双向流**（`AgentConnection`
的两个 async stub）并与 B4d 的 `BridgeEventChannels` 预留 machinery 联动：
建流/摘流经 `startTurnStream` / `startTerminalStream` / `cancelTurnStream` /
`cancelTerminalStream`，过程帧经 Dart 订阅时收到的那个真 sink 推回。

- 改动文件（2 个，均在允许面内）：
  - `android/app/src/main/java/dev/pocketagent/app/bridge/BridgeMethods.kt`（+410 / −22）
  - `android/app/src/test/java/dev/pocketagent/app/bridge/BridgeMethodsTest.kt`（+615 / −25）
- 未改：`BridgeEventChannels.kt` / `BridgeEventChannel.kt`（B4d 成品，只读 import
  其预留 machinery）、`BridgeConnection.kt`、`flutterhost/`、`ui/`、`daemon/`、
  `proto/`、`flutter/`、`tools/`。
- 未 commit / add / push。

## 2. 基线（先绿后动）

```
PA_MIRROR=G:\pa_b4g ; python tools/device/_pa_gradle.py test
```

- 基线：**1972 用例 0 失败**（bridge 包 BridgeErrorsTest 14 / BridgeEventChannelsTest 8 /
  BridgeFlutterWiringTest 6 / BridgeMethodsTest 48，全绿）。
- 期间 B4f 子代理在 `flutterhost/` 并行落盘，其 in-flight 文件
  （`FlutterHostEventChannelsInstallTest.kt` 的旧快照、`ProbeChainTest.kt`）
  一度让整个 test source set 编译失败/单点失败——均与本次改动无关，已逐次
  核验失败堆栈归属（`BufferUnderflowException` 在其 fake messenger 的 ByteBuffer
  辅助函数；`InaccessibleObjectException` 在其反射探针）。
- 终态：**2003 用例，1 失败** = `ProbeChainTest`（B4f 的反射诊断探针，JDK 模块
  限制）。本次改动的用例 **0 失败**，B4d 既有 8 用例保持全绿。

## 3. 六方法接线方案

### 3.1 与 Dart 契约逐字对账（`flutter/docs/channel-contract.md` §3 表 22–27 行）

| 方法 | 契约 arguments | 返回 | 实现 |
|---|---|---|---|
| `turnStreamStart` | `{'threadId': String}` | null | `stringMap(method, args, "threadId")` → 取 sink → `startTurnStream` → 等 observer 就绪 → null |
| `turnStreamSend` | `TurnClientMessage` wire 字节 | null | `TurnClientMessage.parseFrom` → `observer.onNext`（经 `rpc{}` 收口）→ null |
| `turnStreamCancel` | null | null | `expectNull` → `cancelTurnStream()` → null |
| `terminalAttach` | `{'terminalId': String}` | null | `stringMap(method, args, "terminalId")` → 取 sink → `startTerminalStream` → 等 observer 就绪 → null |
| `terminalWrite` | `TerminalClientMessage` wire 字节 | null | `TerminalClientMessage.parseFrom` → `observer.onNext`（经 `rpc{}` 收口）→ null |
| `terminalClose` | `{'terminalId': String}` | null | unary `CloseTerminal` RPC（既有实现）+ `finally { cancelTerminalStream() }` → null |

两处与契约表不同、以 Dart 代码为准的口径（与 B4b 已定案的两条同纪律）：
- `terminalClose` 在 B4b 就已作为 unary 方法实现（Map → `CloseTerminalRequest`），
  本次只补「摘掉宿主侧 attach 流」——unary 只让 daemon 关 PTY，本地 bidi 流不摘
  会继续往可能已取消订阅的 sink 推幽灵帧。
- `turnStreamCancel` 只摘宿主侧资源、**不发** cancel 帧：Dart 侧显式取消走
  `turnStreamSend(cancel 帧)`（`chat_session.dart:187-195`）。

### 3.2 sink / observer 获取的协作设计（本波的核心决策）

B4d 的 `BridgeEventChannels` 把三件关键东西收在私有字段里，都不对外暴露：

1. **sink**：`turnHandler.onListen(arguments, events)` 收到的那个 Flutter
   `EventSink`。框架传入、只活在 handler 里，且 B4d 的 handler 是私有匿名对象
   （当前实现甚至直接丢弃它：`turnSession?.cancel(); turnSession = null`）。
2. **请求 observer**：`TurnStreamSession` 协程里 `stub.startTurn(observer)` 的
   返回值，私有。
3. **建流/摘流**：`startTurnStream` 等是 `internal`，同模块可调，但
   `BridgeMethods` 拿不到 `BridgeEventChannels` 实例。

`BridgeMethods` 的构造只收到 `BridgeBinaryMessenger`（方法面注册面），拿不到
事件面的 `BridgeEventMessenger`，也没有 channels 实例。**方案**（不改 B4d 一行）：

- 新增 seam `BridgeStreamWiring`（`BridgeMethods.kt` 内，public——它是
  `BridgeMethods` 公开构造器的形参类型，Kotlin 不允许 public 签名露出 internal
  类型；实现类 `BridgeEventChannelsWiring` 仍 internal）。
- `BridgeEventChannelsWiring(channels, threadStub, terminalStub, sinkFeeder)`
  把 channels 实例 + daemon async stub + sink 三者接起来。
- **observer 出口**：B4d 的 `startTurnStream(threadId, stub, sink)` 的 `stub`
  形参由调用方提供（`BridgeThreadStreamService` 是接口，可假实现）。传入记录型
  装饰器 `RecordingThreadStreamStub`：B4d 协程调 `stub.startTurn(responseObserver)`
  时，返回值（请求 observer）被留痕，`turnRequestObserver()` 即可取到。
- **sink 出口**：`SinkObservingMessenger(delegate, feed)` 装饰
  `BridgeEventMessenger`，在 `setStreamHandler` 时把 B4d 的 turn/terminal handler
  包一层：Dart 订阅进来时先收到 `onListen(arguments, events)`，把 `events` 记进
  `MutableStreamSinkFeed`，再转调 B4d 原 handler。
- **生产 stubs**：`agentThreadStreamStub()` / `agentTerminalStreamStub()` 薄包
  `AgentConnection.threadServiceAsync()` / `terminalServiceAsync()`（与
  `AgentConnectionBridge` 同纪律；`BridgeConnection.kt` 本波只读，故不经它）。

装配图（生产，一行）：

```kotlin
val feed = MutableStreamSinkFeed()
val inner = FlutterEventMessengerAdapter(messenger)          // B4d 的生产 messenger
val channels = BridgeEventChannels(SinkObservingMessenger(inner, feed), sources, log)
val wiring = BridgeEventChannelsWiring(
    channels, agentThreadStreamStub(), agentTerminalStreamStub(), feed,
)
BridgeMethods(FlutterBinaryMessengerAdapter(messenger), connection, wiring)
```

### 3.3 时序竞态：建流后 observer 未必就绪

B4d 的 `TurnStreamSession` 在自己的 IO scope 里 `launch` 后才调
`stub.startTurn(...)`——`turnStreamStart` 方法返回时那个协程未必已跑到。Dart 的
时序是 `await turnStreamStart(...)` 紧接 `await turnStreamSend(首帧)`
（`chat_session.dart:152-153`），不等就回「受理」会让首帧打到空 observer 上，
报一个"无处可发"的**假错误**。

故 `turnStreamStart` / `terminalAttach` 在建流后**有界等待** observer 就绪
（`awaitTurnRequestObserver` / `awaitTerminalRequestObserver`，上界 5s，
`STREAM_READY_TIMEOUT_MS`）再回 null。超时即结构化 HOST_UNAVAILABLE，不无限
挂住 IO 线程。等待用 `Thread.sleep(5)` 轮询而非 `Thread.yield()`——调用线程是
`Dispatchers.IO`，yield 不承诺让另一个 IO 线程的协程前进。

## 4. 测试证据

`BridgeMethodsTest`：48 → **68 用例，0 失败**（新增 20 条）。全部经 seam 注入，
`BridgeEventChannels` 是真类（B4d 成品）经公开构造器装配，桥的
dispatch → 入参解析 → machinery → observer 全链被真实执行。

新增覆盖（按任务要求的三类）：

1. **请求解析（字节 → proto 字段断言）**
   - `turnStreamSend 字节经 observer 进流_input 与 supplement 两形态`：input 帧的
     `threadId` / `blocks[0].text.text` 与 supplement 帧的 blocks 逐字段对账；
     追问不另建流（`startCalls` 恒 1）。
   - `terminalWrite 字节经 observer 进流_open 与 input 与 resize 三形态`：
     `open` 的 sessionId/cwd/cols/rows、`input` 的 data 原文、`resize` 的 cols/rows；
     三帧同一条 observer（`attachCalls` 恒 1）。
2. **与 B4d machinery 的联动（fake stub）**
   - `turnStreamStart 建流并受理` / `terminalAttach 建流并受理`：B4d 协程真调
     `stub.startTurn` / `stub.attach`（`startCalls` / `attachCalls` 判决）。
   - `过程帧经 Dart 订阅的 sink 推回来_B4d machinery 真跑`：daemon 侧
     `responseObserver.onNext(turnServerFrame(7))` → sink 收到
     `TurnServerMessage` wire 字节，反序列化 `sequence == 7`。
   - `daemon 侧流错误经 sink 报出来_方法面不吞`：`onError` → `sink.error`
     （code 原样映射 INTERNAL，message 透传 daemon 原文）。
   - `turnStreamCancel 摘掉宿主侧流` / `terminalClose 关 daemon 会话并摘掉宿主侧
     attach 流`：gRPC 流被 `cancel`（fake observer 实现
     `ClientCallStreamObserver`，B4d 的 `as? ClientCallStreamObserver<*>?.cancel`
     路径真跑到）。
   - `建流成功后紧随的首帧有落点_不因协程竞态打空 observer`：stub 故意慢 80ms
     拉宽竞态窗口，方法面仍等它就绪，首帧成功进流。
   - `建流时 daemon 迟迟不交 observer_有界等待后回结构化错误_不无限挂住`：
     stub 挂 12s（> 5s 上界）→ HOST_UNAVAILABLE「未就绪」，非 INTERNAL 兜底。
3. **错误路径（不许 panic）**
   - `turnStreamSend 坏字节回 DECODE_FAILED_不补默认值也不进流` /
     `terminalWrite 坏字节回 DECODE_FAILED_且不进流`：残字节
     `0A FF FF FF` → DECODE_FAILED，且 observer 零帧（坏帧不毒化流）。
   - `turnStreamSend 入参不是字节时回 DECODE_FAILED_不做双形态二分`：Map 形态
     → DECODE_FAILED（不搞 null/bytes 双形态）。
   - `turnStreamSend 未 start 就发回结构化错误_不许 panic` /
     `terminalWrite 未 attach 就写回结构化错误_不许 panic`：HOST_UNAVAILABLE
     「无处可发」，`errorDetails == null`（证明走的不是 handle 的 INTERNAL 兜底）。
   - `turnStreamStart 缺 threadId 键时回 DECODE_FAILED_不补空串也不建流` /
     `turnStreamCancel 收到非 null 入参时回 DECODE_FAILED`。
   - `turnStreamStart 在 Dart 未订阅事件面时回 HOST_UNAVAILABLE_不建假流` /
     `terminalAttach ...`：无 sink 不调 `startTurn`（`startCalls == 0`）——
     不建"建了流却收不到帧"的假流。
   - `observer onNext 抛 gRPC 失败照旧映射契约码_不裸抛到Flutter侧`：
     UNAVAILABLE → NOT_CONNECTED + 原文 message + details `UNAVAILABLE`。
   - `terminalClose 的 unary RPC 失败照旧映射契约码_本地摘流不受影响`：
     unary 失败仍摘本地流（`finally`），防幽灵帧源优先于报错。
4. **线程纪律**
   - dispatch 整体在 `withContext(Dispatchers.IO)`（`BridgeMethods` 既有纪律），
     `observer.onNext` 经 `rpc{}` 收口；等待用 sleep 轮询而非 yield（见 §3.3）。
5. **未注入 seam 时的诚实缺席**
   - `未注入装配 seam 时三条建流方法如实回 HOST_UNAVAILABLE_点名缺什么` /
     `send 与 write 在未注入 seam 时回 HOST_UNAVAILABLE_不 panic`：生产默认
     （`streams = null`）不建假流，消息点名缺的是装配 seam；`terminalClose` 的
     unary 面不依赖 seam（仍受理）；`turnStreamCancel` 无 seam 时是干净 no-op。

### 测试基座说明（为什么这些 fake 不算"stub 自己写的假实现"）

- `BridgeEventChannels` 是 B4d 成品真类，经其公开构造器装配——不是假实现。
- `BridgeThreadStreamService` / `BridgeTerminalStreamService` 是 B4d 定义的窄接口，
  protoc stub 是 final class 无法继承（B4b 实测），只能假实现——与 B4b/B4d 测试
  同纪律。
- `StreamFixture` 每个用例一份（`BridgeEventChannels` 自带 IO scope 与收集 job，
  用后 `shutdown()`，防协程泄漏；共享会让用例间通过 `turnSession` 串味）。
- `StreamFixture` 经 `SinkObservingMessenger` 注册 handler——与生产同一条路径，
  sink 观测逻辑本身被测。
- fake 命名带 `StreamFake` 前缀：B4d 的 `BridgeEventChannelsTest` 在**同一包**里
  已有同名 top-level fake（`FakeThreadStreamStub`），Kotlin 不允许同包同名顶层
  声明（Redeclaration）。

## 5. 产物自证

```
PA_MIRROR=G:\pa_b4g ; python tools/device/_pa_gradle.py apk
→ BUILD SUCCESSFUL，app-debug.apk 535921302 bytes，sha256=4ecded611a655761...
```

新符号已入 dex（python 解包 16 个 dex 后字节检索，全部 FOUND）：
`BridgeStreamWiring` / `BridgeEventChannelsWiring` / `SinkObservingMessenger` /
`MutableStreamSinkFeed` / `turnStreamStart` / `terminalAttach` /
`agentThreadStreamStub` / `agentTerminalStreamStub`。

XML 新鲜度：`test-results/testDebugUnitTest/*.xml` 时间戳与运行时刻差 < 1 分钟
（--rerun 强制重跑），无假绿。

## 6. 协调项登记（需主会话/后续波次处理）

### C1（必须，~10 行，flutterhost）生产装配把 wiring 接进 BridgeMethods

`BridgeMethods` 的第三个构造参数 `streams: BridgeStreamWiring? = null` 默认 null，
故**当前生产路径下六方法仍回 HOST_UNAVAILABLE**（消息已点名缺的是装配 seam，不是
模糊的"本波未实现"）。要真正生效，需在 `FlutterHostEngine.installChannelShell`
（B4f 正在改的文件）把 §3.2 的装配图落地：

```kotlin
val feed = MutableStreamSinkFeed()
val inner = FlutterEventMessengerAdapter(messenger)
val channels = BridgeEventChannels(SinkObservingMessenger(inner, feed), sources, log)
val wiring = BridgeEventChannelsWiring(
    channels, agentThreadStreamStub(), agentTerminalStreamStub(), feed,
)
BridgeMethods(FlutterBinaryMessengerAdapter(messenger), connection, wiring)
```

注意与 B4f 已落盘的 `installEventChannels(messenger, connection)` **二选一**：
两者都会向三条事件通道注册 handler，同时装会重复注册（后注册者胜）。建议 B4f 把
`installChannelShell` 里的 `installEventChannels(...)` 换成上面这段（它内部就是
`BridgeEventChannels(FlutterEventMessengerAdapter(messenger), sources, log)`，
只是多包一层 `SinkObservingMessenger` + 把实例交给方法面）。

### C2（建议，B4d 小改）sink 出口的另一种更短接法

若愿意动 B4d，可在 `turnHandler.onListen` 里把 `events` 存进一个
`@Volatile private var turnSink` 并加 `internal fun currentTurnSink()`，
`SinkObservingMessenger` 就不需要了（少一个装饰层）。本波**未**采用：B4d 是成品
只读，且装饰器方案不依赖它、测试与生产同路径。登记备选，不阻塞。

### C3（留后，非本波范围）`startTurn` unary 首帧方法

Dart 的 `PaBridge.startTurn(TurnInput)`（`pa_bridge.dart:373`）仍在用，但它只发
首帧、无消费面。本波只接双向流六条，`startTurn` 继续诚实回 HOST_UNAVAILABLE（消息
已说明理由：unary 入口"发了首帧却看不到过程帧"比不接更糟）。双向流面稳定后可一并
取舍（或让 Dart 侧全面改走 `turnStreamStart` + `turnStreamSend`）。

## 7. 文件清单

| 文件 | 动作 | 行数 |
|---|---|---|
| `android/app/src/main/java/dev/pocketagent/app/bridge/BridgeMethods.kt` | 改 | +410 / −22 |
| `android/app/src/test/java/dev/pocketagent/app/bridge/BridgeMethodsTest.kt` | 改 | +615 / −25 |
| `docs/reviews/2026-10-16/b4g-bidi-methods.md` | 新增 | 本文件 |

未动：`BridgeEventChannels.kt`、`BridgeEventChannel.kt`、`BridgeConnection.kt`、
`BridgeChannel.kt`、`BridgeErrors.kt`、`BridgeFlutterWiring.kt`、
`BridgeTestFixtures.kt`、`flutterhost/**`、`ui/**`、`daemon/**`、`proto/**`、
`flutter/**`、`tools/**`。
