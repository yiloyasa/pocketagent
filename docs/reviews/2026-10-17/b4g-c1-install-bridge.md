# B4g 协调项 C1 落地报告：`installBridge` 三件套装配入口

- 日期：2026-10-17
- 背景：B4g（双向流六方法接通）完成时登记协调项 C1——`BridgeMethods` 的 `streams` 参数默认 null，**生产路径六方法仍回 HOST_UNAVAILABLE**；需装配侧落地 wiring 注入，且与 B4f 的 `installEventChannels` **二选一**（否则双注册）。本报告即该协调项的落地记录（主会话执行）。
- 边界：`bridge/BridgeMethods.kt`（追加 public `installBridge`）+ `flutterhost/FlutterHostEngine.kt`（installChannelShell 改调单入口 + import 调整）；未碰 B4b/B4d/B4f/B4g 的既有逻辑。

## 一、解法：装配逻辑留 bridge 包，对外一个 public 入口

**约束**：`BridgeEventChannelsWiring` 与内部版 `installEventChannels` 都是 `internal`（只 bridge 包内可见），而调用方 `flutterhost` 在另一个包——B4g 报告说的「需 B4f 落地装配图」实质是「需一个能访问 internal 件的 public 入口」。

**落地**：`bridge/BridgeMethods.kt` 尾部新增

```kotlin
fun installBridge(
    messenger: BinaryMessenger,
    connection: BridgeConnection = AgentConnectionBridge(),
    log: BridgeLog = AndroidLog,
): BridgeEventChannels
```

内部按鸡生蛋顺序装配三件套：**channels → wiring → methods**。

### 为什么不用封装版 `installEventChannels`

封装版内部自建 channels 并返回，wiring 无从插入（wiring 持 channels、methods 持 wiring，顺序刚性）。故用 internal 两段版直接建 `BridgeEventChannels(SinkObservingMessenger(...), sources, log)`，再建 wiring，最后建 methods。

## 二、flutterhost 侧改动（B4f 接线的收窄）

`installChannelShell` 从 B4f 的两段式（`BridgeMethods.install` + `installEventChannels`）改为**只调 `installBridge` 一个入口**：

```kotlin
eventChannels?.shutdown()   // 重挂先停旧的（防一份推流源变两份）
eventChannels = installBridge(
    messenger,
    EventFeedingConnection(AgentConnectionBridge()),  // 共享实例纪律
)
```

三条纪律全部保持：
1. **共享连接实例**：`EventFeedingConnection(AgentConnectionBridge())` 由 flutterhost 传入、两路共用——host_info 帧的唯一来源是「经本实例的 updateHostInfo 成功路径」（B4f 报告的接线论证）；
2. **重挂先停旧**：`eventChannels?.shutdown()` 在前；
3. **二选一**：只调 installBridge（内部已含 `BridgeMethods.install`）——双注册会让两个 handler 抢同一通道（B4g C1 明示）。

import 同步收敛（`BridgeMethods`/`install`/`installEventChannels` 三个不再直接引用的 import 删除，`installBridge` 加入）。

## 三、验证证据

| 项 | 结果 |
|---|---|
| compile | **BUILD SUCCESSFUL**（修 1 处：BridgeMethods.kt 缺 `io.flutter.plugin.common.BinaryMessenger` import） |
| Android test（隔离镜像 `G:\pa_c1`） | **2002 用例 / 0 失败 / 0 错误 / 0 跳过**，206 份 XML 全部 00:21:06 同一轮（0.2 分钟新鲜） |
| Android apk | **BUILD SUCCESSFUL**，535,826,177 bytes，sha256 `cfeed40e…`，拷回核验通过 |
| APK dex 自证 | `installBridge` / `BridgeEventChannelsWiring` / `SinkObservingMessenger` / `BridgeStreamWiring` / `FlutterHostEngine` 五符号**全部命中**（16 dex 合并检索） |

**测试判决说明**：B4g 的 68 用例已覆盖六方法 dispatch（含 wiring 注入态）；本批生产的「六方法不再回未接线」由 dispatch 测试 + dex 自证双重覆盖，未再加易碎的 Flutter BinaryMessenger fake 测试（首版写过，接口实现面太宽易碎，删除——测试价值低于维护成本）。

## 四、B4 接线全景（此刻状态）

| 面 | 状态 |
|---|---|
| 方法面 24 unary | ✅ 真 gRPC（B4b） |
| 方法面 6 双向流 | ✅ 真 gRPC + sink/observer 经 wiring（B4g + 本批 C1 落地） |
| 主事件面 3 类 | ✅ 实时推流（B4d + B4f 挂载） |
| turn/terminal 事件面 | ✅ 帧消费 machinery 就位 + wiring 联动（B4d/B4g） |
| Flutter module 集成 | ✅ APK 含 libflutter.so ×3 + kernel_blob（B4a） |
| 导航入口 | ✅ AppDrawer 项（B4e） |

**剩余**：真机冷启与帧推送验证（adb 无设备，登记待设备）；`startTurn` unary 首帧方法继续诚实缺席（B4g C3 留后项，理由已写）。

## 五、文件清单（未 commit，留主会话统一提交）

| 文件 | 性质 |
|---|---|
| `android/app/src/main/java/dev/pocketagent/app/bridge/BridgeMethods.kt` | 追加 public `installBridge`（+1 import） |
| `android/app/src/main/java/dev/pocketagent/app/flutterhost/FlutterHostEngine.kt` | installChannelShell 改调单入口 + import 收敛 |
