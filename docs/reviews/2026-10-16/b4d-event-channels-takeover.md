# B4d 接管报告：三条 EventChannel 帧消费面（断网残留续完）

- 日期：2026-10-16
- 背景：B4d 子代理两度被上游 DNS 故障（`copilot.tencent.com` ENOTFOUND）打断，**第二次落盘了 2 个主源文件后身亡**（无测试、有编译错）。主会话接管续完：修 5 处编译错、补 8 个测试用例、全量验证。
- 边界：新 `bridge/BridgeEventChannel.kt`（seam 层）+ `bridge/BridgeEventChannels.kt`（帧消费面）+ `bridge/BridgeEventChannelsTest.kt`（测试）；未碰 flutterhost/、ui/、daemon/、proto/、flutter/、tools/。

## 一、B4d 残留的 5 处编译错（接管修复）

| # | 错误 | 根因 | 修法 |
|---|---|---|---|
| 1 | `BridgeEventChannels.kt:246 Unresolved 'map'` | 缺 `kotlinx.coroutines.flow.map` import | 补 import |
| 2 | `BridgeEventChannel.kt:122 Return type of 'w' not a subtype` | `Log.w` 返回 Int，接口声明 Unit，表达式体把 Int 当返回值 | 改块体 |
| 3 | `ProcessHostInfoFeed` conflicting declarations | 私有 `updates` 字段与 `override val updates` 同名 | 私有字段改名 `buffer` |
| 4 | `mainHandler must be initialized` | init 块引用声明在类下方的三个 handler（Kotlin 属性初始化顺序） | 三个 handler 声明前移到 init 之前 |
| 5 | `Unresolved 'launch'` | 缺 `kotlinx.coroutines.launch` import | 补 import |

## 二、B4d 设计要点（保留，验证后确认合理）

- **三通道注册**：`BridgeEventChannels(messenger, sources, log, scope)` 全 seam 可注入——`FakeMessenger`/`FakeSources`/`FakeLog` 测试直驱；
- **主事件面三类合流**：`combine(五条 StateFlow)` + `drop(1)`（首帧=各 Flow 现值组合，契约 §4「初值由 getConnectionState 补，事件只推增量」）；host_info/host_event 字节直转（`toByteArray`）；
- **turn/terminal 本波不自动建流**：`onListen` 只记账——请求帧入口（turnStreamStart/Send）方法面本波未接，建流 = 建一条永远收不到请求帧的流（比静默更糟的假象）。**machinery 预留**（`startTurnStream`/`cancelTurnStream` 等）由测试直接驱动验证；
- **错误纪律**：单帧构建失败不进流（记日志）；数据源失败 → `sink.error`（code 原样）；订阅取消摘 gRPC 流（防泄漏）。

## 三、接管补的测试（8 用例，全字节级/形状级断言）

| 用例 | 钉什么 |
|---|---|
| `conn_state帧_五键与type平铺_键名逐字对账` | §3.4 键表六键一个不多一个不少 + 值透传 |
| `host_info与host_event帧_type判别且载荷为wire字节` | `parseFrom(bytes)` 回读对账「字节里就是这条消息」 |
| `安装时三条通道都注册handler` | 三通道名集合精确相等 |
| `主事件面listen后_state变化推全量快照帧` | 改 value → 收一帧全量快照（含竞态修复见下） |
| `主事件面onCancel后不再收帧` | 取消后改 value 零新帧、零错误 |
| `turn_events本波不建空流_listen只记账` | 零帧零错误（诚实缺席） |
| `terminal_events本波不建空流_listen只记账` | 同上 |
| `预留入口_startTurnStream后cancel摘流` | 建了能摘、摘不推错误帧（防泄漏判决点） |

**两个竞态修复**（测试机制，非产品码）：① `combine.drop(1)` 的首帧是各 Flow 现值组合——listen 后立即改 value 会被当成首帧 drop 掉；修法：先 `Thread.sleep(200)` 让协程越过首帧再改。② onCancel 用例不再 `awaitFrame`（无增量可推），睡足即可。

**复用 B4b 测试基建**：`FakeSources` 以 `BridgeConnection by delegate` 委托 `BridgeTestFixtures.FakeConnection`（20+ 快照成员零成本），不为事件测试重写一份全实现 fake。

## 四、H5 断网残留的连锁修复（同一轮网络故障的另一半）

H5（flutter 收敛线）只改了一个文件就被 DNS 故障打断，但那一改把 `chat_timeline.dart` 的类结构弄破了（APK 构建 flutter 编译红）。接管修复：

1. **删 H5 加的 `_finalText` wrapper**：它与类内原有实现字节相同（重复声明编译错）；
2. **公开 `finalBlocksText` 移到类外顶层**：H5 把它插在类体中段（顶格函数待在了类里，后续成员被挤出视觉边界）；
3. **清两处截断残骸**：H5 的插入把某方法尾部留下重复的 `return sb.toString(); }`；
4. **删类内原 `_finalText`**：调用点已全部改 `finalBlocksText`，原私有实现成死代码（analyze 抓到的 unused_element）。

**净结果**：H5 意图的那处收敛（chat_session 与 accumulator 两份 7 行字节一致的 `_finalText` → 单一公开 `finalBlocksText`）**完整落地**，且编译/测试/analyze 全绿。H5 的其余盘点（三态装配/错误文案/字节样板）未及做——遗留到下一轮收敛（见 §六）。

## 五、验证证据

| 项 | 结果 |
|---|---|
| Android test（隔离镜像 `G:\pa_b4d`） | **1972 用例 / 0 失败 / 0 错误 / 0 跳过**，205 份 XML 全部 20:47:28 同一轮（0.6 分钟新鲜） |
| Android apk | **BUILD SUCCESSFUL**，535,814,037 bytes，sha256 `4ba0b668…`，拷回核验通过 |
| flutter test | **949 全绿**（H5 收敛零倒退） |
| flutter analyze（junction） | **No issues found** |
| daemon | 本批零改动（27 包上轮全绿） |

## 六、遗留与交接

1. **turn/terminal 帧消费的「真接通」**：方法面 `turnStreamStart/Send/Cancel` 与 `terminalAttach/Write/Close` 这 6 个双向流生命周期方法在 `BridgeMethods` 里仍回 HOST_UNAVAILABLE（B4b 定案的留后项）；接通后 `BridgeEventChannels` 的预留 machinery（`startTurnStream` 等）即有调用方——**接线点已在 KDoc 写明**（dispatch 对应分支调 `startTurnStream`）。
2. **主事件面接线**：`installEventChannels` 尚未挂进 `FlutterHostEngine.installChannelShell`（B4c 领地）——**一行接线**：B4c 报告 §交接点。挂上前 Dart 侧 events 流保持静默（诚实缺席）。
3. **H5 剩余盘点**：三态装配/错误文案映射/字节传参样板/纯函数层重复——B4d 与 H5 两度被同一 DNS 故障打断，剩余项移交下一轮 flutter 收敛。
4. **真机**：adb 无设备，冷启与帧推送留待有设备时补。

## 七、文件清单（未 commit，留主会话统一提交）

| 文件 | 性质 |
|---|---|
| `android/app/src/main/java/dev/pocketagent/app/bridge/BridgeEventChannel.kt` | B4d 新建（seam 层，接管修 1 处编译错） |
| `android/app/src/main/java/dev/pocketagent/app/bridge/BridgeEventChannels.kt` | B4d 新建（接管修 4 处编译错） |
| `android/app/src/test/java/dev/pocketagent/app/bridge/BridgeEventChannelsTest.kt` | 接管补（8 用例） |
| `android/app/src/main/java/dev/pocketagent/app/flutterhost/FlutterHostEngine.kt` | B4c 产物（install 挂载） |
| `android/app/src/test/java/dev/pocketagent/app/flutterhost/FlutterHostBridgeInstallTest.kt` | B4c 产物（判决测试） |
| `flutter/lib/pages/chat_timeline.dart` | H5 残留修复（收敛完整落地） |
| `docs/reviews/2026-10-16/b4c-bridge-install.md` | B4c 报告 |
