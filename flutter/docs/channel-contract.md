# B2 通道契约 —— Flutter ⇄ Kotlin 宿主桥（MethodChannel / EventChannel）

> **本契约与 `proto/v1/` 同源；proto 变更时：Dart gen stub 重生成（附录 A）+
> 本文档同步。** 方法名、事件 `type`、错误码、Map 键名都是契约字符串，双端同改。
>
> - Dart 侧实现：`flutter/lib/transport/pa_bridge.dart`（代码即契约），
>   测试 `flutter/test/transport/pa_bridge_test.dart` 逐条钉死（mock 宿主 wire 行为）。
> - Kotlin 宿主侧：**B4 波次待实现**（清单见 §6）。宿主缺席时 Dart 如实抛
>   `HOST_UNAVAILABLE`，不伪造任何返回（红线：不留假绿）。

## 1. 通道

| 通道 | 类型 | 名称 | 方向 |
|---|---|---|---|
| 方法面 | MethodChannel | `dev.pocketagent/bridge` | Dart → Kotlin，请求应答 |
| 事件面 | EventChannel | `dev.pocketagent/bridge/events` | Kotlin → Dart，推流 |

Dart 侧常量：`kPaBridgeMethodChannel` / `kPaBridgeEventChannel`（`pa_bridge.dart`）。

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

## 3. 方法清单（首批 6 条 + B3-3 追加 1 条 + B3-4 追加 3 条；宿主侧全部【B4 待实现】）

| # | 方法名（契约字符串） | Dart 签名 | arguments | 返回 |
|---|---|---|---|---|
| 1 | `getHostInfo` | `Future<HostInfo> getHostInfo()` | `GetHostInfoRequest` 字节（空） | `HostInfo` 字节 |
| 2 | `connect` | `Future<void> connect(String addr, String token)` | `{'addr': String, 'token': String}` | null |
| 3 | `disconnect` | `Future<void> disconnect()` | null | null |
| 4 | `getConnectionState` | `Future<PaConnState> getConnectionState()` | null | Map（五键，见下） |
| 5 | `pairCode` | `Future<String> pairCode()` | null | 非空 String |
| 6 | `startTurn` | `Future<void> startTurn(TurnInput input)` | `TurnInput` 字节 | null |
| 7 | `listThreads` | `Future<ListThreadsResponse> listThreads([ListThreadsRequest?])` | `ListThreadsRequest` 字节（缺省 = 空消息） | `ListThreadsResponse` 字节 |
| 8 | `listDirectory` | `Future<ListDirectoryResponse> listDirectory([ListDirectoryRequest?])` | `ListDirectoryRequest` 字节（缺省 = 空消息） | `ListDirectoryResponse` 字节 |
| 9 | `readFile` | `Future<FileChunk> readFile([ReadFileRequest?])` | `ReadFileRequest` 字节（缺省 = 空消息） | `FileChunk` 字节（**首块**，见 §3.9） |
| 10 | `listBranches` | `Future<ListBranchesResponse> listBranches([ListBranchesRequest?])` | `ListBranchesRequest` 字节（缺省 = 空消息） | `ListBranchesResponse` 字节 |

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

- arguments = `TurnInput`（thread_id + blocks + background + model_id + effort）
  的 wire 字节；宿主自行包成 `TurnClientMessage{input}` 进双向流。空消息字段
  规则同 proto（空 = 沿用/默认）。
- 返回 null；轮次过程帧（增量/审批/完成/失败）**不走本方法**——那是
  Turn 双向流的 Dart 形态，B3 波次另行接入（本波只有发起，没有消费面）。
- 错误码：`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）。

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

## 6. Kotlin 宿主侧待实现（B4 波次清单）

- [ ] 在 FlutterEngine 挂载时注册 MethodChannel `dev.pocketagent/bridge`
      handler：10 个方法按 §3 签名（method 名字符串逐字对齐）。
- [ ] 返回值一律走 proto `writeToBuffer()` 字节（与 gRPC 契约同源，复用
      `HostInfo`/`HostEvent`/`TurnInput`/`ListThreadsResponse` 的既有 Kotlin 生成类）。
- [ ] `getConnectionState`：直接取 `AgentConnection` 五条 StateFlow 当前值拼
      §3.4 五键 Map（键名 = 字段名）。
- [ ] `listThreads` / `listDirectory` / `listBranches`：转发对应 daemon RPC，
      `repeated` 字段按 proto 原样字节回传（不给手机端做投影/筛选，那是页端的事）。
- [ ] `readFile`：proto 是 server stream，宿主只回**首块** `FileChunk`（§3.9）；
      分块流的完整消费面留 B5，本波不实现。
- [ ] 注册 EventChannel `dev.pocketagent/bridge/events` StreamHandler：
      五条 StateFlow 合流（任一变化 → `conn_state` 全量快照帧）、
      `hostEvents`（WatchEvents）逐帧 → `host_event`（字节直转）；
      `HostInfo` 变化 → `host_info`。
- [ ] 错误码对齐 §5（宿主侧只抛表内 code + 人话 message）。
- [ ] `pairCode` 复用 `GetPairingInfo` 的回环来源自检（安全语义不得放松）。
- 本波不做：Turn 双向流的 Dart 消费面（`TurnServerMessage` 增量/审批）——B3。

## 7. 类型来源总表（哪些直接用 gen 消息 / 哪些是小类）

| Dart 类型 | 来源 | 用途 |
|---|---|---|
| `HostInfo` / `HostEvent` / `TurnInput` / `GetHostInfoRequest` | gen stub（`lib/gen/pocketagent/v1/`，protoc 生成勿手改） | 方法返回 / 方法入参 / 事件载荷 |
| `PaConnState` | 本契约小类 | 连接态五键（proto/v1 无对应消息——宿主进程内部态不经过 daemon，不造假同源） |
| `PaConnEvent` | 本契约小类 | 事件 Envelope（`type` + 三种载荷） |
| `PaBridgeException` / `PaBridgeErrorCodes` | 本契约小类 | 错误模型（§5） |

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
