# B4f 接线报告：`BridgeEventChannels` 挂进 FlutterHostEngine（B4c/B4d 交接点）

- 日期：2026-10-16
- 波次：B4f（B4d 成品 → 宿主引擎挂载点；B4c 的既定交接点，一行接线 + 测试）
- 边界（严格按任务书）：只改 `flutterhost/FlutterHostEngine.kt`（`installChannelShell` 加事件面 install + `shutdown` 摘实例）+ `flutterhost/` 下测试。**未碰** `bridge/`、`ui/`、`MainActivity.kt`、`daemon/`、`proto/`、`flutter/`、`tools/`。未 commit / add / push。
- 基线：HEAD `2cd0749`（B4c+B4d 合成提交），Android **1972 用例 0 失败**（205 份 XML，21:07:58 同一轮）。

---

## 一、接线点前后对比

### 前（B4c 后：事件面静默壳）

```kotlin
fun installChannelShell(messenger: BinaryMessenger) {
    // 方法面：B4b 的成品桥。
    BridgeMethods.install(messenger, AgentConnectionBridge())

    // 事件面：静默壳——不推流，也不 error。
    for (channelName in listOf(
        BRIDGE_EVENT_CHANNEL, BRIDGE_TURN_EVENT_CHANNEL, BRIDGE_TERMINAL_EVENT_CHANNEL,
    )) {
        EventChannel(messenger, channelName).setStreamHandler(
            object : EventChannel.StreamHandler {
                override fun onListen(arguments: Any?, events: EventChannel.EventSink?) { }
                override fun onCancel(arguments: Any?) { }
            },
        )
    }
}
```

Dart 侧 events 流：listen 收下、零帧、cancel 干净退出——「没有流」而非「流坏了」（B4d 报告 §六-2 的诚实缺席态）。

### 后（B4f）

```kotlin
// engine 级字段：持有帧消费面实例，供 shutdown 停协程（防泄漏）
@Volatile private var eventChannels: BridgeEventChannels? = null

fun installChannelShell(messenger: BinaryMessenger) {
    // 方法面与事件面共享同一个连接实例（B4d host_info 帧的唯一来源）
    val connection = EventFeedingConnection(AgentConnectionBridge())

    BridgeMethods.install(messenger, connection)          // 方法面（B4b/B4c）

    eventChannels?.shutdown()                             // 重挂路径：先停旧实例
    eventChannels = installEventChannels(messenger, connection)  // 事件面（B4d）
}

fun shutdown() {
    synchronized(this) {
        eventChannels?.shutdown()                         // 先停事件面协程
        eventChannels = null
        engine?.let { FlutterEngineCache.getInstance().remove(ENGINE_ID) }
        engine?.destroy()
        engine = null
    }
}
```

**实质改动三处**：

1. `installChannelShell` 内的静默壳（21 行 `EventChannel` + 匿名 handler）换成 `installEventChannels(messenger, connection)` 一行；
2. 方法面与事件面**共享同一个 `EventFeedingConnection` 实例**（原 B4c 是两个独立的 `AgentConnectionBridge()`）；
3. 新增 engine 级 `eventChannels` 字段 + `shutdown` 摘它。

壳不再 import `EventChannel`（该 import 已删）；三条 EventChannel 的注册改由 B4d 的 `BridgeEventChannels.init` 经 `FlutterEventMessengerAdapter` 完成。

---

## 二、签名适配（与任务书预估的差异，以实际代码为准）

任务书预估「`BridgeEventChannels(FlutterEventMessengerAdapter(messenger), AgentEventSources())`（或等价组合）」。实际读 `bridge/BridgeEventChannels.kt:506`，B4d 已留好生产装配入口：

| 预估 | 实际 | 适配方式 |
|---|---|---|
| `new BridgeEventChannels(FlutterEventMessengerAdapter(messenger), AgentEventSources())` 手工装配 | **`installEventChannels(messenger, connection, log)` 生产工厂**（同文件 506 行），内部按 connection 是否 `BridgeEventSources` 决定是否包 `AgentEventSources` | 调工厂，不手工拼 adapter/sources（双份装配路径 = B4b/B4d 反复强调的漂移面） |
| sources 用 `AgentEventSources()`（默认 delegate = 新 `AgentConnectionBridge()`） | `AgentEventSources(connection)`——**connection 由调用方传入** | 传共享实例；这正是 host_info 帧能生效的关键 |
| 方法面/事件面各自 `AgentConnectionBridge()` | B4d KDoc 明确要求**同一个 `EventFeedingConnection` 实例** | 壳内建一次 `val connection = EventFeedingConnection(AgentConnectionBridge())`，两个 install 都用它 |
| 持有实例供 shutdown | `BridgeEventChannels.shutdown()` 是 public（326 行：摘 mainJob/turnSession/terminalSession + `scope.cancel()`） | 壳加 `eventChannels` 字段，`shutdown()` 里 `?.shutdown()` + 置 null |

**一个自己加的防线（任务书未要求）**：`installChannelShell` 重挂时先 `eventChannels?.shutdown()`。生产路径 `warmup` 有双检锁不会重挂，但测试/配置变更路径会——两个 `BridgeEventChannels` 各带一个 IO scope 与收集 job，不停就是「一份推流源变两份」（Dart 侧一次变化收两份相同帧）+ 旧 scope 泄漏。有判决测试（见 §四）。

**`shutdown` 的调用顺序**：事件面先停、引擎后 destroy。反过来的话引擎已 destroy 而收集 job 仍在跑，sink 指向已死的 messenger。

---

## 三、测试证据（新增 `FlutterHostEventChannelsInstallTest.kt`，10 用例）

### 判决结构

| 用例 | 钉什么 | 判决等级 |
|---|---|---|
| `event channels are really wired and the main face pushes real frames` | **生产路径最强判决**：不换任何实现，驱动壳内那一行的完整装配；改 `AgentConnection` 五条 StateFlow → 收到 `{'type':'conn_state', 五键}` 帧，且帧经**真 EventChannel sink** 离开宿主（messenger.send 的字节解码回来逐键对账）；同时断言注册面 = 4 通道且与 bridge 包契约字符串同值 | 主事件面真挂了 |
| `turn and terminal event faces stay silent because the request frame entry is not wired` | turn/terminal listen 后**零帧**（请求帧入口未接，不建空流） | 不伪造 |
| `host_info frames come from the shared decorated connection instance` | 经 `ProcessHostInfoFeed` 记录 → 收到 host_info 帧，字节 parseFrom 回读对账 | 共享实例 + 字节直转 |
| `host_event frames come from the watch events global flow` | `AgentConnection.hostEvents.tryEmit` → 收到 host_event 帧，sequence 回读对账 | SharedFlow 源接对了 |
| `the shell holds the b4d BridgeEventChannels instance` | 反射读壳私有 `eventChannels` 字段：必须是 bridge 包的 `BridgeEventChannels`（B4d 成品），且 shutdown 可调、幂等 | 实例形态 |
| `the event face is built through the production install entry point` | `BridgeEventChannels.sources` 必须是 `AgentEventSources`（`installEventChannels` 的产物）——防壳绕过工厂自己装配 | 装配路径唯一 |
| `method face and event face share one decorated connection instance` | 沿注册链四层反射反查：`BridgeMethods.connection === AgentEventSources.delegate`，且两者都是 `EventFeedingConnection`、其 delegate 是 `AgentConnectionBridge` | 共享 + 装饰 |
| `after engine shutdown the event face stops pushing frames` | listen → 改值收到帧 → `shutdown()` → 再改值睡足 → **不得再出现 down/7 帧**；壳引用清空 | **防泄漏判决点** |
| `reinstalling the shell does not stack event push sources` | 重挂后同一批现值只应推**一份**收敛帧 | 重挂不叠加推流源 |
| `the seam level installer is the same entry point the shell calls` | 驱动 `installEventChannels` 内部重载换 seam sources，帧形状逐键一致 | 测试覆盖的就是壳走的那条 |

### 驱动方式（为什么纯 JVM 能驱动生产路径）

`installChannelShell` 入参是 `BinaryMessenger`（接口），壳内那一行走 `installEventChannels(messenger, EventFeedingConnection(AgentConnectionBridge()))`：生产 sources 直读进程级 `AgentConnection` 的五条 StateFlow（纯 Kotlin 内存）与 `hostEvents` / `ProcessHostInfoFeed`（纯 Kotlin 流）。于是**不注入任何 fake connection**，改 StateFlow 现值 → 收帧。`ShellMessenger` 同时扮演平台侧：`EventChannel` 的 sink 推流走 `BinaryMessenger.send(通道名, 字节)`，fake 把 send 的字节按通道名收下，于是「帧真的离开了 Kotlin 宿主」可被解码断言（与 Dart 侧收到的同形）。

### 既有测试（不删不弱化，只改口径）

`FlutterHostBridgeInstallTest.kt` 的 `event channels stay silent after the method face is wired` → 改名 `event channels accept listen with a success envelope after the wiring`：原断言「静默」正是被本波替换掉的事实（B4a→B4c→B4f 三代口径）。改后断言**握手面**（listen ACK 是空成功包、不是 error envelope）——握手错不进流（契约 §2.5），这条与推流内容正交，仍然值得钉。类 KDoc 补了与 `FlutterHostEventChannelsInstallTest` 的分工说明。其余 11 条一字未动。

`FlutterHostChannelShellTest.kt` / `FlutterHostEngineTest.kt` 未改（它们的断言是注册面/常量面，本波不触碰）。

### 测试机制踩坑（如实记录）

1. **回包 double-flip**：`stream()` 内部已 `apply { flip() }`（与既有 fake 同款），调用方再 flip 一次 → `BufferUnderflowException`。改为直接 decode。
2. **`encodeSuccessEnvelope` 不回 flip**：框架的 sink 推流 buffer 是「写完、position 在末尾」（javap 实证），fake 的 `snapshotBytes()` 必须在 duplicate 上 flip 才能取到内容。
3. **`combine` 重组帧数不定（实测 flake）**：连写五个 StateFlow value，combine 可能重组成一帧也可能截成若干帧，最后一帧未必含全部五个新值（一次实跑 `expected:<[terminal, fs]> but was:<[]>`）。修法：`awaitConvergedFrame(messenger, mapOf(...))` 按**内容**等帧（等一帧含指定键值），不等帧数——这是测试机制修竞态，产品行为无变化。
4. **`MutableSharedFlow.emit` 是 suspend**：host_event 用例改 `tryEmit`（非挂起路径；此刻主事件面已订阅，帧必达收集端）。
5. **反射读私有字段**：`Field.setAccessible` 对 JDK 模块内类（`java.util.HashMap.serialVersionUID`）抛 `InaccessibleObjectException`；全部用 `runCatching` 兜住，且只遍历自家编译产物。lambda 捕获字段名是编译器生成的 `arg$N`，故**按类型**取捕获值而非按名字。
6. **临时探针已删**：为确认反射链形状，曾加一个 `ProbeChainTest`（dump 注册链），用完即删；镜像里残留的旧 XML 已清（XML 核验份数与用例数对齐）。

---

## 四、变异验证（证明判决测试不是空转）

按 B4c 纪律做了三组变异（每次改 `FlutterHostEngine.kt` → 跑全量 → 记录 → 还原 → 复跑全绿）：

| # | 变异 | 结果 |
|---|---|---|
| A | `installChannelShell` 不挂事件面（`if (now < 0) install`） | **13 条失败**：本测试 10 条全失败 + `FlutterHostBridgeInstallTest` 1 条 + `FlutterHostChannelShellTest` 2 条（注册面/握手面判决） |
| B | `shutdown()` 不摘事件面实例 | **1 条失败**：`after engine shutdown the event face stops pushing frames` 精确失败（防泄漏判决点接得住） |
| C | 事件面用**另一个**连接实例（B4d KDoc 点名的那种错法） | **1 条失败**：`method face and event face share one decorated connection instance` 精确失败（共享实例纪律接得住） |

还原后复跑：**1982 / 0 失败**（连跑三轮，flake 修复后稳定）。变异代码零残留（`git diff` 无 `MUTATION` 字样）。

> 注：变异 A 一度写成「死分支」（`if (now >= 0)` 恒真），首跑全绿——**变异写进死分支等于没变异**，已改成活分支重验。这也是「不许假绿」纪律在自己工具上的应用。

---

## 五、测试水位

### 本波边界的干净绿轮（基线 `2cd0749`，并发会话动 `bridge/` 之前）

```
基线（HEAD 2cd0749）  : 1972 用例 / 0 失败 / 0 错误 / 0 跳过，205 份 XML（21:07:58 同一轮）
改后（本波）          : 1982 用例 / 0 失败 / 0 错误 / 0 跳过，206 份 XML（22:57:05 同一轮，0.3 分钟新鲜）
抬升                  : +10 用例（FlutterHostEventChannelsInstallTest 10 条；FlutterHostBridgeInstallTest 改名不计条数）
连跑稳定性            : 修复 combine 竞态后连跑 3 轮全绿
```

`python tools/device/_pa_test_report.py 'G:\pa_b4f\app\build\test-results\testDebugUnitTest'` 核验：全部 XML 均在 10 分钟内（`--rerun` 由 `_pa_gradle.py` 强制，防 UP-TO-DATE 假绿）。

### 最终一轮（并发会话的批次 B 落定后，2001 用例）

本波边界**分文件归因全绿**（13 条红全部在批次 B 的 `BridgeMethodsTest.kt`，见 §八）：

| flutterhost 文件 | 用例 | 失败 | 错误 |
|---|---|---|---|
| `FlutterHostActivityTest` | 11 | 0 | 0 |
| `FlutterHostBridgeInstallTest` | 12 | 0 | 0 |
| `FlutterHostChannelShellTest` | 5 | 0 | 0 |
| `FlutterHostEngineTest` | 12 | 0 | 0 |
| `FlutterHostEntryTest` | 4 | 0 | 0 |
| `FlutterHostEventChannelsInstallTest`（本波新增） | 10 | 0 | 0 |
| **小计** | **54** | **0** | **0** |
| `BridgeMethodsTest`（批次 B，非本波） | — | 13 | 0 |

---

## 六、APK 证据

```
gradle assembleDebug  → BUILD SUCCESSFUL in 1m 9s
app-debug.apk 535,819,825 bytes  sha256 = 303D9921FBDC697740B2790E6E5CDD0F6B0A091ED07D6290894424FD2B777B43
拷回核验通过（字节数 + sha256 一致）
lib/arm64-v8a/libflutter.so   395,316,440
lib/armeabi-v7a/libflutter.so 348,884,936
lib/x86_64/libflutter.so      390,101,840   ← 三 ABI 与 B4a/B4e 基线一致
```

产物自证（「源码改对 ≠ 跑着的是新的」），`dexdump` 实证：

1. **接线代码真在 dex 里**：`classes13.dex` 的 `FlutterHostEngine.installChannelShell` 反汇编可见
   `new-instance EventFeedingConnection` → `sget-object .eventChannels` → `invoke-virtual BridgeEventChannels.shutdown` → `invoke-static BridgeEventChannelsKt.installEventChannels$default` → `sput-object .eventChannels`；
   `FlutterHostEngine.shutdown` 首段即 `sget-object .eventChannels` → `invoke-virtual BridgeEventChannels.shutdown` → `sput-object null`（事件面先停，见 §二）。
2. **B4d 成品真在 dex 里**：`classes8.dex` 含 `BridgeEventChannels` 及全部内部类（`mainHandler$1` / `turnHandler$1` / `terminalHandler$1` / `connStateFrames$1` / `hostInfoFrames$*` / `hostEventFrames_*`）与 `BridgeEventMessenger` / `BridgeEventSink` / `BridgeEventStreamHandler` seam 层。
3. **三条契约通道名在 dex 里**：`classes13.dex` 字符串表中 `dev.pocketagent/bridge/events` / `turn_events` / `terminal_events` 与 `dev.pocketagent/bridge` 并列。
4. **旧静默壳已绝迹**：`classes13.dex` 中「B4d 才接线」「本波不推流」「静默壳」「EventChannel.StreamHandler」**0 命中**——壳自己的静默 handler 确实没了，不是「加了一份新的、旧的还在」。

---

## 七、文件清单

| 文件 | 动作 | sha256 |
|---|---|---|
| `android/app/src/main/java/dev/pocketagent/app/flutterhost/FlutterHostEngine.kt` | 改：`installChannelShell` 事件面静默壳 → `installEventChannels(messenger, connection)`；方法面/事件面共享 `EventFeedingConnection`；新增 `eventChannels` 字段；`shutdown` 摘事件面；删 `EventChannel` import；类/方法 KDoc 与 `HOST_UNAVAILABLE` 注释同步为 B4f 口径 | `808F25B5A36AFDA6B8888C209691419A4D9A7182096D5273925C56577316F5C0` |
| `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostEventChannelsInstallTest.kt` | 新增（10 条，「事件面真挂了」判决 + 防泄漏 + 共享实例 + 变异验证接得住） | `55F9A9EA0B096665C19EDE7EE6098DF7C1DC3F70A2B7EB42E51ECD37BD9608BF` |
| `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostBridgeInstallTest.kt` | 改：1 条改名+收窄断言口径（静默 → 握手面）；1 条因批次 B 把 5 条双向流方法接上而收窄为 `startTurn` 单条 + 断言改契约级；类 KDoc 补与新测试文件的分工；其余 10 条未动 | `4B728E5522B3AB159BACDF1B7190C8D4D60216F6EDDFC4108A7028CF0E01BAF5` |
| `docs/reviews/2026-10-16/b4f-event-channels-install.md` | 本报告 | — |

`git status` 中本波的改动 = 上表前 3 项（2 改 1 增）+ 本报告。其余改动见 §八（并发会话，非本波）。

---

## 八、工作区交叉污染（如实记录，非本波改动）

开工时 `git status` 干净（HEAD `2cd0749`）。本波收尾时工作区出现**两批非本波改动**（并发会话在飞）：

| 批次 | 文件 | 内容 | 对本波的影响 |
|---|---|---|---|
| A（flutter 收敛） | `flutter/lib/pages/{chat_history_data,collab,collab_bridge,pa_page_kit,terminal_data,chat_page,settings_console}.dart` + `flutter/test/pages/*_test.dart` | B4d 报告 §六-3 遗留的「H5 剩余盘点」式收敛（`paChannelErrorText` 收编、`PaAsyncPhase` typedef 共享） | 无 |
| B（双向流六方法） | `android/app/src/main/java/dev/pocketagent/app/bridge/BridgeMethods.kt` + `android/app/src/test/java/dev/pocketagent/app/bridge/BridgeMethodsTest.kt` | 本波 §九记录的「下一波」：`turnStreamStart/Send/Cancel`、`terminalAttach/Write` 接入 `BridgeEventChannels` 预留 machinery（新增 `BridgeStreamWiring` seam），`startTurn` 继续诚实缺席 | **有，但可分离** |

### 批次 B 的两次冲击与本波的适配

1. **第一次**：批次 B 编辑中态（缺 `streamsNotWired` 等定义）→ `:app:compileDebugKotlin` 红。本波未碰 `bridge/`（任务书明令只读 import），等其编译通过后复跑。
2. **第二次**：批次 B 编译通过后，其行为变化击穿本波两条既有断言的**措辞层面**：
   - `FlutterHostBridgeInstallTest.the six unimplemented bidirectional methods stay honestly HOST_UNAVAILABLE`——原断言桥的留后消息须含「未接线/事件面/EventChannel」字样。批次 B 把 5 条双向流方法接上后，`startTurn` 的留后消息改了措辞（仍如实回 HOST_UNAVAILABLE，但不再含那三个词）。
   - `FlutterHostEventChannelsInstallTest.the seam level installer is the same entry point the shell calls`——`combine` 重组帧数竞态（同 §三-3 的 flake，这条路上一轮没修到）。

   适配方式（**都不弱化判决，只把断言从「桥的人话措辞」收窄到「契约硬事实」/「等内容不等帧数」）：
   - 前者改名 `startTurn stays honestly HOST_UNAVAILABLE instead of faking a first frame`，六条列表收窄为 `startTurn` 一条（另 5 条已被批次 B 真接上，不再是「未实现」），断言改为「错误码 = HOST_UNAVAILABLE + 消息点名方法名」。**桥的留后人话措辞属 `bridge/` 的演进面，flutterhost 的判决不该被它拖红**（这一点写进了该用例 KDoc）。
   - 后者改用 `awaitSinkConvergedFrame`（等内容收敛，见 §三-3）。

### 最终一轮的分离核验（2001 用例，13 失败全部归属批次 B）

```
flutterhost 六个文件：Activity 11 / BridgeInstall 12 / ChannelShell 5 / Engine 12 / Entry 4 / EventChannelsInstall 10
                    = 54 条，全部 0 失败 0 错误
全仓 2001 用例 13 失败 —— 13 条全部在 android/.../bridge/BridgeMethodsTest.kt（批次 B 的测试文件）
```

即：**本波的接线与测试在其自身边界内全绿**；全仓红的部分是批次 B 尚未收尾的测试，归其作者。

> 取证成本如实记：为把批次 B 隔掉单独验本波，试过 `git archive HEAD` 建隔离树直跑 gradlew、试过手工还原镜像里的 `bridge/`——两度被「`_pa_gradle.py` 每次 robocopy 覆盖镜像」绕回来；最终改用「分文件 XML 归因」这条路，反而更直接（一眼看出 13 条红全在批次的文件里）。结论不变，但路径比预期曲折。

---

## 九、未做 / 留待

- **turn / terminal 帧消费的「真接通」**：本波收尾时，并发会话（批次 B）已在 `bridge/BridgeMethods.kt` 接入 5 条双向流方法（`turnStreamStart/Send/Cancel`、`terminalAttach/Write`）并经新增的 `BridgeStreamWiring` seam 取 sink/stub；**但壳尚未把 `BridgeStreamWiring` 实例接进去**（`BridgeMethods.streams` 默认 null → 五方法如实回 HOST_UNAVAILABLE 点名缺什么）。故本波两条事件面**仍保持静默**（listen 只记账、零帧），测试 `turn and terminal event faces stay silent because the request frame entry is not wired` 钉死，不伪造。下一步是 `installChannelShell` 里把 `BridgeEventChannels` 实例经该 seam 接给 `BridgeMethods`——属装配侧一行，归批次 B 的收尾或下一波。
- **`startTurn`（§3.6 unary 首帧入口）继续诚实缺席**：无消费面的 unary 入口，「发了首帧却看不到过程帧」比不接更糟；批次 B 维持 HOST_UNAVAILABLE，本波 `FlutterHostBridgeInstallTest` 有一条钉它。
- **真机未验**（`adb devices` 为空）：Flutter UI 打开后 Dart 侧实际收帧的端到端链路（冷启 → listen → daemon 侧连接态变化 → Dart 收到 conn_state 帧），留待有设备时补。本报告只声称 JVM 单测 + APK 产物层面的证据。
- **host_info 的覆盖边界**（B4d 已定案，本波未扩大）：只覆盖「经本实例的 `updateHostInfo` 成功路径」；探活/回退连接内部自调的 `updateHostInfo` 不过桥，故探活周期内 daemon 改 capabilities 而 Dart 没调 `getHostInfo` 时，host_info 帧不推（conn_state 帧会推，capabilities 在五键里）。
