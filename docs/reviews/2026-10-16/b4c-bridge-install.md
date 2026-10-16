# B4c 接线报告：`BridgeMethods` 挂进 Flutter 宿主引擎，替换诚实缺席壳

- 日期：2026-10-16
- 波次：B4c（B4b 成品 → 宿主引擎挂载点；B4d = 事件面推流，未做）
- 边界（严格按任务书）：`flutterhost/FlutterHostEngine.kt` 的 `installChannelShell` 壳替换 + `flutterhost/` 下测试。**未碰** `bridge/`、`ui/`、`MainActivity.kt`、`daemon/`、`proto/`、`flutter/`、`tools/`。未 commit / add / push。

## 一、接线点前后对比

### 前（B4a 诚实缺席壳）

`installChannelShell(messenger)` 自己建一个 `MethodChannel`，handler 对**任何** method 回：

```kotlin
result.error(
    HOST_UNAVAILABLE,
    "Flutter 宿主桥方法面本波(B4a)未接线：通道壳已就绪,方法实现" +
        "(bridge/BridgeMethods.kt)与 AgentConnection 取数面留待 B4b。" +
        "（调用 method = ${call.method}）",
    null,
)
```

事件面三条 EventChannel 静默（listen 收下、不发一帧、cancel 干净退出）。

### 后（B4c）

```kotlin
fun installChannelShell(messenger: BinaryMessenger) {
    // 方法面：B4b 的成品桥。
    BridgeMethods.install(messenger, AgentConnectionBridge())

    // 事件面：静默壳——不推流，也不 error（B4d 才接）。
    for (channelName in listOf(
        BRIDGE_EVENT_CHANNEL, BRIDGE_TURN_EVENT_CHANNEL, BRIDGE_TERMINAL_EVENT_CHANNEL,
    )) { EventChannel(messenger, channelName).setStreamHandler(/* 静默 */) }
}
```

**唯一实质改动就是这一行**（壳自建的 MethodChannel + 占位 handler 消失，换成 B4b 的生产装配入口）。事件面一行未动——本波没接就是没接，不推假帧。

## 二、签名适配（与任务书预估的差异，以实际代码为准）

任务书预估「`BridgeMethods.install(messenger, connection)`（可能带 connection 默认参数）」。实际读 `bridge/BridgeFlutterWiring.kt:50`：

```kotlin
fun BridgeMethods.Companion.install(
    messenger: BinaryMessenger,
    connection: BridgeConnection,          // ← 无默认值，必须显式传
): BridgeMethods = BridgeMethods(FlutterBinaryMessengerAdapter(messenger), connection)
```

| 预估 | 实际 | 适配方式 |
|---|---|---|
| 可能有 connection 默认参数 | **无默认参数**，`connection` 必填 | 显式传 `AgentConnectionBridge()`（生产连接适配器，bridge 包内，只读 import） |
| 壳「setMethodCallHandler 换个实现」 | 不需要壳自己建 channel——`install` 内部已建 `MethodChannel(messenger, BRIDGE_CHANNEL_NAME)` 并注册 handler | 直接调 `install`，壳不再 import `MethodChannel`（该 import 已删） |
| `BridgeMethods(messenger, AgentConnectionBridge())` 直接构造 | 构造参数是桥内部 seam `BridgeBinaryMessenger`，不是 Flutter `BinaryMessenger` | 必须走 `install` 扩展函数（它内部套 `FlutterBinaryMessengerAdapter`）——这也是 B4b 设计意图（调用点不需要知道桥内部有 seam） |

**编译形态细节（踩过）**：`install` 是 `BridgeMethods.Companion` 的扩展函数，编译后落在文件 facade `BridgeFlutterWiringKt`，反射签名是 3 个参数 `(Companion 接收者, BinaryMessenger, BridgeConnection)`——不是 Companion 的 declaredMethods，也不是 2 参数。测试里已按此写。

## 三、线程 / 生命周期核对

`install` 在 engine 创建路径（`warmup` → `installChannelShell(created.dartExecutor.binaryMessenger)`，`FlutterHostEngine.kt:96`）**同步**执行，只做两件轻事：

1. `FlutterBinaryMessengerAdapter` 构造（持 messenger 引用，无副作用）；
2. `BridgeMethods.init` → `messenger.setMethodCallHandler(...)` = `MethodChannel.setMethodCallHandler`（注册，不建连）。

**`AgentConnectionBridge` 是薄适配**（读 `bridge/BridgeConnection.kt:173-269`）：五条 StateFlow 读面 `get() = AgentConnection.connState.value`；六个服务取数入口 `AgentConnection.hostService()` 等是 `HostServiceGrpc.newBlockingStub(channel())`，`channel()` 懒建（`AgentConnection.kt:1219`）——**构造期不建连、不探活**。故同步执行无初始化顺序问题：Dart 侧何时 invoke 才会真正走到 gRPC。

> 实测边界（写进了测试注释）：六个服务取数入口**不在**纯 JVM 可调面内——`channel()` 首行 `android.util.Log.i` 在 JVM 下抛 "not mocked"。故测试只覆盖「构造 + StateFlow 读面」，stub 取用归真机。

## 四、测试证据（「壳真换了」判决）

基线先绿：**1953 用例 / 0 失败**（203 份 XML，18:01:12 同一轮）。改后：**1964 用例 / 0 失败 / 0 错误 / 0 跳过**（204 份 XML，19:03:33 同一轮，0.2 分钟新鲜）——**+11 条**。

### 新增 `FlutterHostBridgeInstallTest.kt`（12 条，判决核心）

判决结构 = 「契约外仍缺席」×「契约内已通」两条同时成立，才叫壳真换了：

| 用例 | 断言 |
|---|---|
| `contract-foreign method name still gets HOST_UNAVAILABLE from the bridge` | 契约外假方法名 → HOST_UNAVAILABLE，且消息点名「契约漂移」（桥的 else 分支措辞，非壳时代措辞） |
| `the six unimplemented bidirectional methods stay honestly HOST_UNAVAILABLE` | `startTurn`/`turnStreamStart`/`turnStreamSend`/`turnStreamCancel`/`terminalAttach`/`terminalWrite` → 仍 HOST_UNAVAILABLE（桥如实留后，**不伪造**） |
| `getConnectionState returns the production snapshot through the shell wiring` | **生产路径判决**：不换连接实现，直接驱动壳内那一行 install；摆布 `AgentConnection` 五条 StateFlow → 得五键 Map（B4a 时代这里必是 HOST_UNAVAILABLE） |
| `getConnectionState goes through the real bridge and returns the five key snapshot` | fake connection 版：走 `BridgeMethods.install` + `FakeConnection`，五键逐值断言 |
| `pairCode reaches the grpc facing seam and returns the pair code` | fake pairing 数据只有被读到才出现在回包 → 证明派发到 `hostService()` |
| `getHostInfo request reaches the grpc layer and the reply bytes round trip` | 请求字节 → 解析 → stub 留痕（`updateHostInfoCalls == 1`）→ 回字节与 fake 数据逐字节相等 |
| `listThreads request bytes cross the bridge untouched` | 换服务族（threadService）证明不是只接了 hostService；`lastListThreadsRequest` 非空证明字节真被 parseFrom |
| `bridge failures surface contract codes not shell invented ones` | gRPC UNAVAILABLE → NOT_CONNECTED + daemon 原文 message；失败路径不写 capabilities 快照 |
| `event channels stay silent after the method face is wired` | 三条 EventChannel listen ACK 为空成功包（静默），合计 4 个 handler |
| `the wiring entry point keeps an injectable connection seam` | 反射 `BridgeFlutterWiringKt.install` = 3 参（Companion, BinaryMessenger, BridgeConnection） |
| `the shell instantiates the production connection adapter` | `AgentConnectionBridge()` 可构造 + 五条 StateFlow 读面可用（初始值） |
| `the bridge registers on the contract method channel name` | 注册键 = `dev.pocketagent/bridge` |

### 更新既有测试（不删不弱化）

**`FlutterHostChannelShellTest.kt`**：删去 3 条只对 B4a 壳成立的判决（「任何 method → HOST_UNAVAILABLE」「契约内方法名也 HOST_UNAVAILABLE」「message 点名 B4b」）——它们测的正是被替换掉的东西。保留并改写 5 条壳**仍负责**的面（事件面静默、4 通道不挂通配、幂等重挂、通道真注册），新增 1 条 `method face replies come from the bridge not from a shell placeholder`（应答码须出自契约 §5 七码表、消息不得含「未接线」/「B4a」）。净 5→6 条。

**`FlutterHostEngineTest.kt`**：`host unavailable code matches the contract literal` 加强为跨包同值断言（flutterhost 常量 == `BridgeErrorCodes.HOST_UNAVAILABLE`）；新增 `method channel name matches the bridge package channel name`（`BRIDGE_METHOD_CHANNEL` == `BRIDGE_CHANNEL_NAME`，防跨包通道名漂移）。

### 变异验证（证明判决测试不是空转）

临时把 `BridgeMethods.install(...)` 那一行换回 B4a 式占位 handler，重跑：

```
FlutterHostBridgeInstallTest > getConnectionState returns the production snapshot through the shell wiring FAILED
FlutterHostBridgeInstallTest > contract-foreign method name still gets HOST_UNAVAILABLE from the bridge FAILED
FlutterHostBridgeInstallTest > the six unimplemented bidirectional methods stay honestly HOST_UNAVAILABLE FAILED
1964 tests completed, 3 failed
```

3 条判决用例精确失败（含**生产路径**那条），随后还原并复跑全绿（1964/0）。变异脚本已删除，无残留。

## 五、APK 证据

```
gradle assembleDebug  → BUILD SUCCESSFUL in 1m 32s
app-debug.apk 535,799,389 bytes  sha256 = 2A6159801CFF7D54E8D09CFF47A104F28A81E0BB35F146BC2157F2080196E846
拷回核验通过（字节数 + sha256 一致）
lib/arm64-v8a/libflutter.so   395,316,440
lib/armeabi-v7a/libflutter.so 348,884,936
lib/x86_64/libflutter.so      390,101,840   ← 三 ABI 与 B4a/B4e 基线一致
```

产物自证（「源码改对 ≠ 跑着的是新的」）：

- `installChannelShell` 与 `AgentConnectionBridge` 同在 `classes13.dex`（接线代码真在 dex 里）；
- 旧壳占位字符串 `B4a` / `B4b` 在 `classes13.dex` 中 **0 命中**（壳自己的占位消息确实没了，不是「加了一份新的、旧的还在」）。

## 六、文件清单

| 文件 | 动作 |
|---|---|
| `android/app/src/main/java/dev/pocketagent/app/flutterhost/FlutterHostEngine.kt` | 改：`installChannelShell` 壳替换为 `BridgeMethods.install(messenger, AgentConnectionBridge())`；删 `MethodChannel` import；类/方法 KDoc 与 `HOST_UNAVAILABLE` 常量注释同步为 B4c 口径 |
| `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostBridgeInstallTest.kt` | 新增（12 条，「壳真换了」判决） |
| `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostChannelShellTest.kt` | 改：删 3 条 B4a 壳专属判决，改 5 条，增 1 条桥来源断言 |
| `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostEngineTest.kt` | 改：1 条加强 + 1 条新增跨包一致性 |

`git status` 仅上述 3 改 1 增，无其他文件被碰。

## 七、未做 / 留待

- **事件面三条 EventChannel 仍静默**（B4d）：`conn_state` / `host_info` / `host_event` 的合流消费面未接。Dart 侧现在能调通 24 条方法，但收不到任何推流帧——判定宿主在不在走方法面探针（契约 §2.5 的原设计）。
- **真机未验**（`adb devices` 为空）：Flutter UI 打开后 Dart 侧实际调用 daemon 的端到端链路，留待有设备时补。
- **测试口径变化如实记录**：B4a 的「任何 method → HOST_UNAVAILABLE」三条判决在本波**必然失效**（它们测的被替换了），已按「删被替换物的判决、保留仍成立的面」处理；判「壳真换了」的责任移交 `FlutterHostBridgeInstallTest`（并用变异验证过它接得住）。
