// B2 通道契约 —— Dart ⇄ Kotlin 宿主桥的 MethodChannel / EventChannel 封装。
//
// 【为什么集中在这一个文件】UI 页面不该知道通道名、参数编码与错误码：契约的两份
// 载体是本文件（代码即契约）+ flutter/docs/channel-contract.md（人读的签名/错误码
// 清单），两者与 proto/v1/ 同源 —— proto 变更时 gen stub 重生成并同步文档。
//
// 【为什么不做假实现】Kotlin 宿主桥要到 B4 波次才注册这些方法与事件。宿主缺席时
// invokeMethod 拿到的是 MissingPluginException，这里如实转成
// PaBridgeException(HOST_UNAVAILABLE) 上抛，绝不返回编造的空 HostInfo 或假"已连接"
// —— 调用方必须能区分"连不上主机"和"宿主桥根本不在"，这决定了 UI 是该重试还是该
// 等待宿主起进程。测试侧用 mock handler 模拟宿主（test/transport/），mock 的是宿主的
// wire 行为，不是 PaBridge 自己。

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:protobuf/protobuf.dart'
    show GeneratedMessage, InvalidProtocolBufferException;

import '../gen/pocketagent/v1/fs.pb.dart';
import '../gen/pocketagent/v1/git.pb.dart';
import '../gen/pocketagent/v1/host.pb.dart';
import '../gen/pocketagent/v1/runtime.pb.dart'
    show CollabPolicyResponse, GetCollabPolicyRequest;
import '../gen/pocketagent/v1/thread.pb.dart';

/// MethodChannel：宿主方法面（Dart → Kotlin，请求应答）。
///
/// 【B4】Kotlin 宿主桥 B4 波次按此名注册 handler；在此之前宿主缺席，方法调用
/// 如实抛 `PaBridgeException(HOST_UNAVAILABLE)`。
const String kPaBridgeMethodChannel = 'dev.pocketagent/bridge';

/// EventChannel：宿主事件面（Kotlin → Dart，推流）。
const String kPaBridgeEventChannel = 'dev.pocketagent/bridge/events';

/// 错误码常量表（契约的一部分，UI 按 code 分支）。
///
/// 【两段式，为什么】上半段是 Dart 侧自产：解码失败与宿主缺席只有 Dart 能判定；
/// 下半段是**契约规定宿主必须按这些 code 抛 PlatformException**（B4 波次实现），
/// Dart 侧只透传不改写——若 Dart 侧改写宿主 code，UI 分支就会和宿主真实错误脱钩。
abstract final class PaBridgeErrorCodes {
  // ── Dart 侧自产（宿主缺席 / 宿主返回值与契约不符）──

  /// 通道无 handler：Kotlin 宿主桥尚未实现（B4 前）或宿主进程未起。
  static const String hostUnavailable = 'HOST_UNAVAILABLE';

  /// 宿主返回值的结构/字节与契约不符（方法返回与事件 payload 共用，
  /// 消息里带具体方法名或事件键定位）。
  static const String decodeFailed = 'DECODE_FAILED';

  // ── 宿主侧须返回（B4 对齐；Dart 透传）──

  /// 尚未连上 daemon（连接类 RPC 在未连接时调用）。
  static const String notConnected = 'NOT_CONNECTED';

  /// 未授权：token 缺失/过期，需先 Authorize/配对。
  static const String authRequired = 'AUTH_REQUIRED';

  /// 连接地址非法（host 侧判定；空地址 Dart 侧本地即拒，不过桥）。
  static const String invalidAddr = 'INVALID_ADDR';

  /// 配对码不可读（来源非本机回环 / 配对未开启，对应 proto
  /// `GetPairingInfoResponse.pairing_enabled=false` 的拒绝语义）。
  static const String pairDisabled = 'PAIR_DISABLED';

  /// 宿主内部错误（无法归入以上各码时的兜底，message 须给人话原因）。
  static const String internal = 'INTERNAL';
}

/// 通道错误：Dart 侧自产 + 宿主 PlatformException 透传，统一成一种异常类型。
///
/// 【为什么透传 code/message/details 三件套】UI 按 code 分支（重试/去配对/只展示），
/// details 保留宿主附加结构（如可重试标记），丢任何一件都会让上层退化成"只知道
/// 出错了"。
class PaBridgeException implements Exception {
  const PaBridgeException(this.code, this.message, {this.details});

  /// 错误码，见 [PaBridgeErrorCodes]；宿主 code 不在表内时原样保留（透传不改写）。
  final String code;

  /// 人话原因（宿主 message 或 Dart 侧解码定位信息）。
  final String message;

  /// 宿主附加结构，可空。
  final Object? details;

  @override
  String toString() => 'PaBridgeException($code): $message';
}

/// 连接态原子字段小类 —— Kotlin `AgentConnection` 五条 StateFlow 的 Dart 形态
/// （connState / connTransport / connAddress / connectionEpoch / hostCapabilities，
/// 见 docs/research/2026-10-14-page-contracts.md §3.1）。
///
/// 【为什么是小类而不是 proto 消息】proto/v1 里没有连接态消息：这些值是宿主进程
/// 内部状态，不经过 daemon，gen stub 里自然也没有对应类型；硬造一个"看起来像 gRPC
/// 契约"的消息会凭空多出一份假同源。
///
/// 【为什么 state/transport 是 String 不是 enum】值域由宿主单方面定义（Kotlin 侧
/// 现值 "checking"/"ok"/"down" 与传输标签），Dart 枚举会变成第二份写死名单——
/// 宿主加档位时旧 APK 必然掉进 UNRECOGNIZED/未知分支，违反"未知输入透传"红线。
class PaConnState {
  const PaConnState({
    required this.state,
    required this.transport,
    required this.address,
    required this.epoch,
    required this.capabilities,
  });

  /// 从宿主 wire Map 严格解码（方法返回与 conn_state 事件共用）。
  ///
  /// 【为什么严格缺键即失败】五个键对应宿主五条 StateFlow，恒有值；缺键/类型不符
  /// = 宿主与本文档契约漂移，静默补零值会把"契约坏了"伪装成"状态正常"（假绿）。
  factory PaConnState.fromPlatform(Object? raw) {
    if (raw is! Map) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '连接态须为 Map，实际 ${raw.runtimeType}',
      );
    }
    String string(String key) {
      final Object? v = raw[key];
      if (v is! String) {
        throw PaBridgeException(
          PaBridgeErrorCodes.decodeFailed,
          '连接态键 $key 须为 String，实际 ${v.runtimeType}（缺键时为 null）',
        );
      }
      return v;
    }

    // 按 wire 键序依次校验：报错定位到最先违约的键（契约文档同序）。
    final String state = string('connState');
    final String transport = string('connTransport');
    final String address = string('connAddress');

    final Object? epoch = raw['connectionEpoch'];
    if (epoch is! int) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '连接态键 connectionEpoch 须为 int，实际 ${epoch.runtimeType}',
      );
    }
    final Object? caps = raw['hostCapabilities'];
    if (caps is! List) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '连接态键 hostCapabilities 须为 List<String>，实际 ${caps.runtimeType}',
      );
    }
    final capabilities = <String>[];
    for (final Object? c in caps) {
      if (c is! String) {
        throw PaBridgeException(
          PaBridgeErrorCodes.decodeFailed,
          'hostCapabilities 元素须为 String，实际 ${c.runtimeType}',
        );
      }
      capabilities.add(c);
    }
    return PaConnState(
      state: state,
      transport: transport,
      address: address,
      epoch: epoch,
      capabilities: capabilities,
    );
  }

  /// 连接态："checking" / "ok" / "down"（宿主定义，未知值原样透传）。
  final String state;

  /// 传输标签（如 grpc+tls / adb-reverse，宿主定义）。
  final String transport;

  /// 当前连接地址（PaHostChip 展示用）。
  final String address;

  /// 重连成功自增 = 全局刷新信号（Kotlin connectionEpoch）。
  final int epoch;

  /// 能力位唯一真值快照（terminal / fs / git / agent:codex / collab:* / runtime.v1 …）。
  final List<String> capabilities;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    if (other is! PaConnState) {
      return false;
    }
    if (other.state != state ||
        other.transport != transport ||
        other.address != address ||
        other.epoch != epoch ||
        other.capabilities.length != capabilities.length) {
      return false;
    }
    for (var i = 0; i < capabilities.length; i++) {
      if (other.capabilities[i] != capabilities[i]) {
        return false;
      }
    }
    return true;
  }

  @override
  int get hashCode =>
      Object.hash(state, transport, address, epoch, Object.hashAll(capabilities));

  @override
  String toString() =>
      'PaConnState($state, $transport, $address, epoch=$epoch, '
      'caps=${capabilities.length})';
}

/// 事件类型（EventChannel payload 的 `type` 字段值域）。
///
/// 【为什么值域是闭合枚举】Flutter UI 与 Kotlin 宿主桥打进同一个 APK、同一个
/// versionCode（双端同版政策），事件类型不可能"新宿主对旧 UI"单边演进；因此未知
/// 类型 = 契约漂移，按 DECODE_FAILED 显式报错，而不是静默吞掉——吞掉即假绿。
enum PaConnEventType {
  /// 连接态快照：`PaConnState`（Kotlin connState/connTransport/connAddress/
  /// connectionEpoch/hostCapabilities 五条流的合流快照）。
  connState,

  /// 主机信息：proto `HostInfo` wire 字节（gen stub）。
  hostInfo,

  /// 全局主机事件：proto `HostEvent` wire 字节（Kotlin hostEvents，对应
  /// `HostService.WatchEvents` 流，含 sequence 续传水位）。
  hostEvent,
}

/// 事件流元素：`type` 决定三个载荷字段里哪一个在场，其余恒为 null。
///
/// 【为什么不用 proto 消息当载体本身】proto 里没有"连接态"消息（见 [PaConnState]
/// 注释）；连接态走 Map、proto 消息走 wire 字节，Envelope 的 `type` 负责判别——
/// 这正是 channel-contract.md §4 的 payload 表。
class PaConnEvent {
  const PaConnEvent.connState(this.connState)
      : type = PaConnEventType.connState,
        hostInfo = null,
        hostEvent = null;

  const PaConnEvent.hostInfo(this.hostInfo)
      : type = PaConnEventType.hostInfo,
        connState = null,
        hostEvent = null;

  const PaConnEvent.hostEvent(this.hostEvent)
      : type = PaConnEventType.hostEvent,
        connState = null,
        hostInfo = null;

  final PaConnEventType type;

  /// [type] == [PaConnEventType.connState] 时在场。
  final PaConnState? connState;

  /// [type] == [PaConnEventType.hostInfo] 时在场。
  final HostInfo? hostInfo;

  /// [type] == [PaConnEventType.hostEvent] 时在场。
  final HostEvent? hostEvent;

  @override
  String toString() => 'PaConnEvent($type)';
}

/// B2 通道契约的 Dart 封装（可注入构造，方便测试隔离）。
///
/// 【为什么可注入而不是单例】测试要按通道名 mock 宿主侧 handler；注入通道让测试
/// 用独立通道与生产隔离，也避免全局单例状态串测试。生产用默认名即可。
class PaBridge {
  PaBridge({
    MethodChannel? methodChannel,
    EventChannel? eventChannel,
  })  : _methods = methodChannel ?? const MethodChannel(kPaBridgeMethodChannel),
        _events = eventChannel ?? const EventChannel(kPaBridgeEventChannel);

  final MethodChannel _methods;
  final EventChannel _events;

  // ── 方法面（首批 6 条 + B3-3 追补 listThreads + B3 第 3 波追补 listDirectory /
  // readFile / listBranches + B4 追补协同 10 条 unary；宿主侧一律【Kotlin
  // 宿主桥 B4 波次实现】）─────────────────────────────────────────────

  /// 主机信息 —— proto `HostService.GetHostInfo`（proto/v1/host.proto）。
  ///
  /// arguments：`GetHostInfoRequest` 的 wire 字节（空消息 → 空字节数组，长度 0——
  /// 空字节仍是"按 proto 走"的契约形式，不做 null/bytes 两种形态的版本二分）。
  /// 返回：`HostInfo` 的 wire 字节 → gen stub 反序列化。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<HostInfo> getHostInfo() => _invoke(
        'getHostInfo',
        GetHostInfoRequest().writeToBuffer(),
        _decodeHostInfo,
      );

  /// 建立到 daemon 的连接。
  ///
  /// arguments：`{'addr': String, 'token': String}`（addr 为主机地址，
  /// token 为空 = 尚未配对/免鉴权宿主；token 只在进程内过桥，不进日志）。
  /// 返回：null = 请求受理。**成功与否不看本返回值**——Kotlin 连接是异步的，
  /// 真值是 `conn_state` 事件 / [getConnectionState] 快照（对应 Kotlin
  /// connState 流）。空地址在 Dart 本地即拒（`INVALID_ADDR`），不必过桥。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> connect(String addr, String token) async {
    if (addr.trim().isEmpty) {
      throw const PaBridgeException(
        PaBridgeErrorCodes.invalidAddr,
        '连接地址不能为空',
      );
    }
    await _invokeExpectNull('connect', <String, Object?>{
      'addr': addr,
      'token': token,
    });
  }

  /// 断开当前连接。arguments：null；返回：null。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> disconnect() => _invokeExpectNull('disconnect', null);

  /// 连接态快照 —— Kotlin `AgentConnection` 五条 StateFlow 的一次性读取。
  ///
  /// arguments：null。返回：Map（键见 [PaConnState.fromPlatform]）。
  /// 【为什么同时要快照和事件】事件流是"订阅后才收"且无缓冲：页面冷起、断线期间
  /// 的态必须靠本方法补初值，事件只负责之后的增量。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<PaConnState> getConnectionState() =>
      _invoke('getConnectionState', null, PaConnState.fromPlatform);

  /// 当前配对码 —— proto `HostService.GetPairingInfo` 的 `pair_code` 字段
  /// （本机回环来源才可读；不可读时宿主须抛 `PAIR_DISABLED`）。
  ///
  /// arguments：null（请求为 `GetPairingInfoRequest` 空消息，空参数即省一次编码，
  /// 返回才是 proto 关心的一半）。返回：非空 String 配对码。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<String> pairCode() => _invoke('pairCode', null, (Object? raw) {
        if (raw is! String || raw.isEmpty) {
          throw PaBridgeException(
            PaBridgeErrorCodes.decodeFailed,
            'pairCode 须返回非空 String 配对码，实际 ${raw.runtimeType}'
            '（配对不可读时宿主应抛 PAIR_DISABLED 而非空返回）',
          );
        }
        return raw;
      });

  /// 开一轮对话 —— proto `TurnService.StartTurn` 的首帧输入。
  ///
  /// arguments：`TurnInput` 的 wire 字节（宿主自行包成 `TurnClientMessage{input}`
  /// 进双向流；thread_id + blocks + background 与 gRPC 契约同源）。返回：null。
  /// 轮次的过程帧（增量/审批/完成）不走本方法，走 `host_event` 之外的 turn 通道
  /// （B3 波次另行接入，见 channel-contract.md §6）。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> startTurn(TurnInput input) =>
      _invokeExpectNull('startTurn', input.writeToBuffer());

  /// 会话列表 —— proto `ThreadService.ListThreads`（proto/v1/thread.proto:18）。
  ///
  /// arguments：`ListThreadsRequest` 的 wire 字节（空请求 = 空字节数组，编码总则
  /// §2.1 同 getHostInfo）。返回：`ListThreadsResponse` 的 wire 字节 → gen stub
  /// 反序列化（threads[] 逐条是 proto `Thread`）。
  ///
  /// 【为什么默认不带参】B3-3 只接列表首屏：搜索/分组/归档三维筛选仍是页面本地
  /// 纯函数（applyThreadFilter），服务端过滤由调用方按需填 request 字段——
  /// 方法面保持「请求消息原样编码」，不在这里写死任何筛选默认值。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——这就是 B4 前 Threads 页的诚实占位路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListThreadsResponse> listThreads([ListThreadsRequest? request]) => _invoke(
        'listThreads',
        (request ?? ListThreadsRequest()).writeToBuffer(),
        _decodeListThreads,
      );

  /// 目录列表 —— proto `FileSystemService.ListDirectory`（proto/v1/fs.proto:14）。
  ///
  /// arguments：`ListDirectoryRequest` 的 wire 字节（path 空 = 服务端回落
  /// workspace，见 fs.proto 与 daemon fs.go:40-43；include_hidden 原样过桥，
  /// Dart 侧不写死默认值）。返回：`ListDirectoryResponse` 的 wire 字节 → gen
  /// stub 反序列化（entries[] 逐条是 proto `DirectoryEntry`：name/path/is_dir/
  /// size_bytes/modified_unix_ms）。
  ///
  /// 【为什么不是流式】daemon 侧 ListDirectory 是一次性应答（os.ReadDir 全量），
  /// 只有 ReadFile 才是分块流（见 [readFile]）；把非流 RPC 硬做成流只会多一层
  /// 假契约。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——这就是 B4 前 Files 页的诚实占位路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListDirectoryResponse> listDirectory([ListDirectoryRequest? request]) =>
      _invoke(
        'listDirectory',
        (request ?? ListDirectoryRequest()).writeToBuffer(),
        _decodeListDirectory,
      );

  /// 读文件预览 —— proto `FileSystemService.ReadFile`（proto/v1/fs.proto:18，
  /// **服务器流** `FileChunk`）。
  ///
  /// 【流怎么收成方法】daemon 的 ReadFile 按 64KB 分块推流（fs.go:93-117）：
  /// 首块带 total_size 与 text 标记，其余块只带 data。本波只做「文本文件预览」，
  /// 消费面只需要第一块 + 是否还有后续（超出 max_bytes 的诚实提示），因此宿主侧
  /// 把流汇聚成**首块**回传（B4 实现约定，见 channel-contract.md），Dart 侧拿到的
  /// 是单份 `FileChunk` wire 字节；下载/分块消费是另一条链路（PaDownloader），
  /// 不在本方法面里假装支持。
  ///
  /// arguments：`ReadFileRequest` 的 wire 字节（path + max_bytes；0 = 服务端
  /// 默认上限）。返回：`FileChunk` 的 wire 字节 → gen stub 反序列化
  /// （data / total_size / text / modified_unix_ms）。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——B4 前 Files 页预览的诚实占位路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<FileChunk> readFile([ReadFileRequest? request]) => _invoke(
        'readFile',
        (request ?? ReadFileRequest()).writeToBuffer(),
        _decodeReadFile,
      );

  /// 分支列表 —— proto `GitService.ListBranches`（proto/v1/git.proto:20）。
  ///
  /// arguments：`ListBranchesRequest` 的 wire 字节（repo = 仓库绝对路径）。
  /// 返回：`ListBranchesResponse` 的 wire 字节 → gen stub 反序列化
  /// （branches[] 逐条是 proto `GitBranch`：name/is_current/is_remote；
  /// daemon git.go:213-229 由 `git branch -a` 产出，is_remote 按名字含 "/" 判定）。
  ///
  /// 【本波只读】checkoutBranch / createBranch 等写操作方法面本波不接——
  /// 红线「手机端写电脑文件/仓库」待拍板；页面因此不摆切换/新建入口。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——B4 前 Git 页的诚实占位路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListBranchesResponse> listBranches([ListBranchesRequest? request]) =>
      _invoke(
        'listBranches',
        (request ?? ListBranchesRequest()).writeToBuffer(),
        _decodeListBranches,
      );

  // ── 协同（B4 第 4 波；proto `ThreadService` 的 12 个 Collab RPC 中选 10 条
  // unary；WatchCollabSession 是服务器流，本波**不做**——见方法清单末的交接说明）

  /// 派发一条共同指令 —— proto `ThreadService.DispatchCollab`
  /// （proto/v1/thread.proto:154）。
  ///
  /// arguments：`DispatchCollabRequest` 的 wire 字节（instruction + members；
  /// members 至少一个，role_id 空 = 未配置）。返回：`DispatchCollabResponse`
  /// 的 wire 字节（session_id + items，items 与请求 members 一一对应、顺序一致）。
  ///
  /// 【受理 ≠ 执行成功】response 只回答「受理了没有」：每位成员的 ack/error 在
  /// items 里，结果看各自子线程的状态（GetCollabSession / 成员会话页）。一位
  /// 受理失败不影响其余（并行扇出的应有语义），页面据此逐位如实呈现。
  /// 【为什么不是流】本 RPC 本身是 unary；「派发后跟进」由 getCollabSession
  /// 快照承担，WatchCollabSession 事件流留 B5（transport 无 stream 面）。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——这就是 B4 前 Collab 页派发路径的诚实占位。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<DispatchCollabResponse> dispatchCollab(
          [DispatchCollabRequest? request]) =>
      _invoke(
        'dispatchCollab',
        (request ?? DispatchCollabRequest()).writeToBuffer(),
        _decodeDispatchCollab,
      );

  /// 单个协同会话快照 —— proto `ThreadService.GetCollabSession`
  /// （proto/v1/thread.proto:166）。
  ///
  /// arguments：`GetCollabSessionRequest` 的 wire 字节（session_id；daemon 对
  /// 不存在的 id 走 gRPC NotFound，人话原因由宿主 message 透传）。返回：
  /// `CollabSessionSnapshot` 的 wire 字节（会话级状态 + 全体成员卡片）。
  ///
  /// 【为什么先要这个方法】协同状态的唯一事实来源是 daemon：进页面/派发后/写
  /// 动作应答后都靠它恢复整屏。Kotlin 侧的恢复链是「快照先行 + 事件流跟随」
  /// （CollabSessionRepository.openLatest），本波只有快照这一半。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<CollabSessionSnapshot> getCollabSession(
          [GetCollabSessionRequest? request]) =>
      _invoke(
        'getCollabSession',
        (request ?? GetCollabSessionRequest()).writeToBuffer(),
        _decodeGetCollabSession,
      );

  /// 协同会话列表 —— proto `ThreadService.ListCollabSessions`
  /// （proto/v1/thread.proto:168）。
  ///
  /// arguments：`ListCollabSessionsRequest` 的 wire 字节（include_finished；
  /// false = 只看进行中的）。返回：`ListCollabSessionsResponse` 的 wire 字节
  /// （sessions[] 按创建时间倒序，客户端不重排）。
  /// 【为什么默认不带参】与 listThreads 同纪律：方法面保持「请求消息原样编码」，
  /// 不在这里写死任何筛选默认值；include_finished 由调用方按页面语义给。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListCollabSessionsResponse> listCollabSessions(
          [ListCollabSessionsRequest? request]) =>
      _invoke(
        'listCollabSessions',
        (request ?? ListCollabSessionsRequest()).writeToBuffer(),
        _decodeListCollabSessions,
      );

  /// 取消整个协同 —— proto `ThreadService.CancelCollabSession`
  /// （proto/v1/thread.proto:174）。
  ///
  /// arguments：`CancelCollabSessionRequest` 的 wire 字节（session_id）。返回：
  /// `CollabSessionSnapshot` 的 wire 字节 —— **收敛后的会话快照**（daemon 逐
  /// 成员发起取消后回整屏真值，客户端整屏换真，不自己拼状态）。
  /// 【甲级写操作】页面执行前有显式确认流；防重复提交由页面的在飞闸承担。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<CollabSessionSnapshot> cancelCollabSession(
          [CancelCollabSessionRequest? request]) =>
      _invoke(
        'cancelCollabSession',
        (request ?? CancelCollabSessionRequest()).writeToBuffer(),
        _decodeCancelCollabSession,
      );

  /// 取消一个成员 —— proto `ThreadService.CancelCollabMember`
  /// （proto/v1/thread.proto:176）。
  ///
  /// arguments：`CancelCollabMemberRequest` 的 wire 字节（session_id +
  /// thread_id；thread_id 是该成员的子线程 id，取消/重试/审批都按它定位）。
  /// 返回：`CollabMemberSnapshot` 的 wire 字节（该成员的新卡；daemon 对已终局
  /// 成员幂等返回原卡，客户端如实呈现「无需取消」而不重试调用）。
  /// 【甲级写操作】同 cancelCollabSession：确认流 + 在飞闸。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<CollabMemberSnapshot> cancelCollabMember(
          [CancelCollabMemberRequest? request]) =>
      _invoke(
        'cancelCollabMember',
        (request ?? CancelCollabMemberRequest()).writeToBuffer(),
        _decodeCancelCollabMember,
      );

  /// 重试一个成员 —— proto `ThreadService.RetryCollabMember`
  /// （proto/v1/thread.proto:178）。
  ///
  /// arguments：`RetryCollabMemberRequest` 的 wire 字节（session_id +
  /// thread_id）。返回：`CollabMemberSnapshot` 的 wire 字节（该成员的新卡；
  /// daemon 以新代籍重建轮次，**不覆盖旧结果**）。
  /// 【daemon 前置条件】只允许对终局成员重试；对未收尾成员调用会得人话拒绝
  /// （客户端按钮态同源禁用，见 collab_session_data.dart 的 memberActions）。
  /// 【甲级写操作】同 cancelCollabSession：确认流 + 在飞闸。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<CollabMemberSnapshot> retryCollabMember(
          [RetryCollabMemberRequest? request]) =>
      _invoke(
        'retryCollabMember',
        (request ?? RetryCollabMemberRequest()).writeToBuffer(),
        _decodeRetryCollabMember,
      );

  /// 协同策略快照（只读）—— proto `RuntimeService.GetCollabPolicy`
  /// （proto/v1/runtime.proto:39）。
  ///
  /// arguments：null（`GetCollabPolicyRequest` 是空消息——按编码总则 §2.1 仍走
  /// 「空请求 = 空字节数组」的 wire 形态，与 getHostInfo 同款，不做 null/bytes
  /// 两种形态的版本二分）。返回：`CollabPolicyResponse` 的 wire 字节
  /// （policy = 用户配置 / effective = 打底默认后的实际生效值 / revision /
  /// policy_path）。
  ///
  /// 【为什么本波只读】`SetCollabPolicy` 是甲级写操作，且 proto 钉死「提交的是
  /// 完整策略不是补丁」：改单项必须先 Get、在完整对象上改、再 Set（带 revision
  /// 做 CAS）——只提交一个字段会把其余项重置回默认（daemon 侧实测踩过）。写入
  /// 面留 B5，本波只读呈现。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<CollabPolicyResponse> getCollabPolicy() => _invoke(
        'getCollabPolicy',
        GetCollabPolicyRequest().writeToBuffer(),
        _decodeGetCollabPolicy,
      );

  /// 协同角色列表 —— proto `ThreadService.ListCollabRoles`
  /// （proto/v1/thread.proto:143）。
  ///
  /// arguments：null（`ListCollabRolesRequest` 是空消息——按编码总则 §2.1 仍走
  /// 「空请求 = 空字节数组」的 wire 形态，与 getHostInfo / getCollabPolicy 同款，
  /// 不做 null/bytes 两种形态的版本二分）。返回：`ListCollabRolesResponse`
  /// 的 wire 字节（roles[] 逐条是 proto `CollabRole`：id/name/agent_id/
  /// description/instructions/model/effort/workspace 八字段；daemon 按 id
  /// 升序返回，客户端不重排）。
  ///
  /// 【空列表 = 真空态】用户还没配过任何角色（daemon collab_roles.go:71-73
  /// 的注释明说：空表返回空数组而不是错误，让手机端区分「还没配」与「取不到」）；
  /// **通道不可用是另一条路径**（HOST_UNAVAILABLE → 页面错误态），两者在 UI 与
  /// 测试里严格区分，不许用空列表冒充错误（防假空态，同 listThreads 纪律）。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——这就是 B4 前 CollabRoles 页的诚实占位路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListCollabRolesResponse> listCollabRoles() => _invoke(
        'listCollabRoles',
        ListCollabRolesRequest().writeToBuffer(),
        _decodeListCollabRoles,
      );

  /// 保存协同角色（新增/编辑共用的幂等 upsert）—— proto
  /// `ThreadService.SetCollabRole`（proto/v1/thread.proto:145）。
  ///
  /// arguments：`SetCollabRoleRequest` 的 wire 字节（role = 完整八字段角色；
  /// id 是幂等键——同 id 覆盖，异 id 新增）。返回：`SetCollabRoleResponse`
  /// 的 wire 字节（role = **落库后读回来的真值**，daemon collab_roles.go:88-91
  /// 刻意不用入参原样回：id 可能被 trim 过，手机端拿到什么就显示什么，
  /// 不会「看起来存进去了但引用时又说没有」）。
  ///
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）；daemon 侧的 InvalidArgument
  /// （id 空/含空格/超长 64，collab_roles.go:111-124）以宿主 message 原样透传，
  /// Dart 不改写——页面把它如实显示（客户端前置校验只是第一道闸门，不是吞掉
  /// 后端错误的借口）。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<SetCollabRoleResponse> setCollabRole(CollabRole role) => _invoke(
        'setCollabRole',
        SetCollabRoleRequest(role: role).writeToBuffer(),
        _decodeSetCollabRole,
      );

  /// 删除协同角色 —— proto `ThreadService.DeleteCollabRole`
  /// （proto/v1/thread.proto:146）。
  ///
  /// arguments：`DeleteCollabRoleRequest` 的 wire 字节（id；daemon 先 trim 再删，
  /// collab_roles.go:96）。返回：`DeleteCollabRoleResponse` 的 wire 字节
  /// （空消息——本 RPC 无回显字段，成功与否看有没有抛错）。
  ///
  /// 【幂等语义】删不存在的 id，daemon 不报错（collab_roles.go:94-95「视为已
  /// 达成目标」）；本地照常移除该行。**破坏性写操作**：页面执行前有显式二次
  /// 确认流（引用它的委派会报「未定义的协同角色」，确认弹层把这句话写明白）。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）；daemon 对空 id 回
  /// InvalidArgument，message 原样透传。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<DeleteCollabRoleResponse> deleteCollabRole(String id) => _invoke(
        'deleteCollabRole',
        DeleteCollabRoleRequest(id: id).writeToBuffer(),
        _decodeDeleteCollabRole,
      );

  // ── 协同 RPC 全集与本波取舍（12 选 10）───────────────────────────────────
  //
  // 已接（10，全部 unary）：dispatchCollab / getCollabSession /
  // listCollabSessions / cancelCollabSession / cancelCollabMember /
  // retryCollabMember / getCollabPolicy / listCollabRoles / setCollabRole /
  // deleteCollabRole。
  //
  // 未接（2，留 B5，注释交接）：
  //  1. `WatchCollabSession`（thread.proto:172，**服务器流**）——B2 契约事件面
  //     只到 conn_state/host_info/host_event（channel-contract.md §4），stream
  //     消费留 B5；本波用一次性快照（List/Get）+ 写动作应答闭环，不假装有实时
  //     跟进。
  //  2. `SetCollabPolicy`（runtime.proto:40，甲级写操作 + 整包 CAS 语义）——见
  //     getCollabPolicy 的「为什么本波只读」。
  //
  // 角色三条（listCollabRoles / setCollabRole / deleteCollabRole）由「协同角色」
  // 两页（collab_roles.dart / collab_role_editor.dart）经
  // collab_roles_state.dart 的 store 消费；Collab 主面板只读该 store 的投影。

  // ── 事件面 ──────────────────────────────────────────────────────────────

  /// 宿主事件流（EventChannel `dev.pocketagent/bridge/events`）。
  ///
  /// 【无缓冲语义】broadcast 流：订阅即收，订阅前推过的事件不补——连接态初值用
  /// [getConnectionState] 快照补齐，事件只承担增量；这与 Kotlin 侧
  /// MutableSharedFlow(replay=0) 的语义一致。
  /// 【错误语义】payload 违约/未知类型 → 流错误 `DECODE_FAILED`；宿主
  /// `EventSink.error` → 流错误（code 原样）。流本身不因单个坏事件关闭。
  /// 【宿主缺席不进流】EventChannel 的 'listen' 握手错由 Flutter 框架转投
  /// FlutterError.reportError，不会变成流错误——判定宿主在不在必须走方法面
  /// （任一方法会如实得 HOST_UNAVAILABLE），不能等事件流报错。契约见
  /// channel-contract.md §4。
  Stream<PaConnEvent> get events => _decodedEvents;

  late final Stream<PaConnEvent> _decodedEvents = _events
      .receiveBroadcastStream()
      .map(_decodeEvent)
      .transform(
        StreamTransformer<PaConnEvent, PaConnEvent>.fromHandlers(
          handleError: (
            Object error,
            StackTrace stack,
            EventSink<PaConnEvent> sink,
          ) {
            // 宿主侧错误统一收敛成 PaBridgeException，UI 只需要认识一种异常；
            // PaBridgeException（解码失败等已定性错误）原样透传。
            if (error is PlatformException) {
              sink.addError(
                PaBridgeException(error.code, error.message ?? '',
                    details: error.details),
                stack,
              );
            } else if (error is MissingPluginException) {
              sink.addError(
                PaBridgeException(
                  PaBridgeErrorCodes.hostUnavailable,
                  error.message ??
                      '事件通道无 handler：$kPaBridgeEventChannel',
                ),
                stack,
              );
            } else {
              sink.addError(error, stack);
            }
          },
        ),
      );

  // ── 内部：调用 + 解码 ───────────────────────────────────────────────────

  /// 真发 invokeMethod：宿主缺席（MissingPluginException）与宿主错误
  /// （PlatformException）都如实转成 [PaBridgeException]，返回值交 [decode] 校验。
  Future<T> _invoke<T>(
    String method,
    Object? arguments,
    T Function(Object? raw) decode,
  ) async {
    try {
      final Object? raw = await _methods.invokeMethod<Object?>(method, arguments);
      return decode(raw);
    } on PlatformException catch (e) {
      // 宿主 code 原样透传：code 是 UI 分支依据，改写等于让上层和宿主真实错误脱钩。
      throw PaBridgeException(e.code, e.message ?? '', details: e.details);
    } on MissingPluginException catch (e) {
      // 通道无 handler = 宿主桥未实现（B4 前）或宿主进程未起。如实上报，不伪造返回。
      throw PaBridgeException(
        PaBridgeErrorCodes.hostUnavailable,
        e.message ?? '宿主桥未注册：$kPaBridgeMethodChannel（B4 波次实现）',
      );
    }
  }

  /// 无返回语义的方法：宿主按契约回 null，回了别的就是契约漂移，显式报错。
  Future<void> _invokeExpectNull(String method, Object? arguments) async {
    final Object? raw = await _invoke<Object?>(method, arguments, (r) => r);
    if (raw != null) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '$method 契约要求宿主回 null，实际 ${raw.runtimeType}',
      );
    }
  }

  HostInfo _decodeHostInfo(Object? raw) => bytesOrThrow(
        'getHostInfo 返回值', raw, 'HostInfo', HostInfo.fromBuffer);

  ListThreadsResponse _decodeListThreads(Object? raw) => bytesOrThrow(
        'listThreads 返回值',
        raw,
        'ListThreadsResponse',
        ListThreadsResponse.fromBuffer,
      );

  ListDirectoryResponse _decodeListDirectory(Object? raw) => bytesOrThrow(
        'listDirectory 返回值',
        raw,
        'ListDirectoryResponse',
        ListDirectoryResponse.fromBuffer,
      );

  FileChunk _decodeReadFile(Object? raw) => bytesOrThrow(
        'readFile 返回值', raw, 'FileChunk', FileChunk.fromBuffer);

  ListBranchesResponse _decodeListBranches(Object? raw) => bytesOrThrow(
        'listBranches 返回值',
        raw,
        'ListBranchesResponse',
        ListBranchesResponse.fromBuffer,
      );

  DispatchCollabResponse _decodeDispatchCollab(Object? raw) => bytesOrThrow(
        'dispatchCollab 返回值',
        raw,
        'DispatchCollabResponse',
        DispatchCollabResponse.fromBuffer,
      );

  CollabSessionSnapshot _decodeGetCollabSession(Object? raw) => bytesOrThrow(
        'getCollabSession 返回值',
        raw,
        'CollabSessionSnapshot',
        CollabSessionSnapshot.fromBuffer,
      );

  ListCollabSessionsResponse _decodeListCollabSessions(Object? raw) =>
      bytesOrThrow(
        'listCollabSessions 返回值',
        raw,
        'ListCollabSessionsResponse',
        ListCollabSessionsResponse.fromBuffer,
      );

  CollabSessionSnapshot _decodeCancelCollabSession(Object? raw) =>
      bytesOrThrow(
        'cancelCollabSession 返回值',
        raw,
        'CollabSessionSnapshot',
        CollabSessionSnapshot.fromBuffer,
      );

  CollabMemberSnapshot _decodeCancelCollabMember(Object? raw) => bytesOrThrow(
        'cancelCollabMember 返回值',
        raw,
        'CollabMemberSnapshot',
        CollabMemberSnapshot.fromBuffer,
      );

  CollabMemberSnapshot _decodeRetryCollabMember(Object? raw) => bytesOrThrow(
        'retryCollabMember 返回值',
        raw,
        'CollabMemberSnapshot',
        CollabMemberSnapshot.fromBuffer,
      );

  CollabPolicyResponse _decodeGetCollabPolicy(Object? raw) => bytesOrThrow(
        'getCollabPolicy 返回值',
        raw,
        'CollabPolicyResponse',
        CollabPolicyResponse.fromBuffer,
      );

  ListCollabRolesResponse _decodeListCollabRoles(Object? raw) => bytesOrThrow(
        'listCollabRoles 返回值',
        raw,
        'ListCollabRolesResponse',
        ListCollabRolesResponse.fromBuffer,
      );

  SetCollabRoleResponse _decodeSetCollabRole(Object? raw) => bytesOrThrow(
        'setCollabRole 返回值',
        raw,
        'SetCollabRoleResponse',
        SetCollabRoleResponse.fromBuffer,
      );

  DeleteCollabRoleResponse _decodeDeleteCollabRole(Object? raw) =>
      bytesOrThrow(
        'deleteCollabRole 返回值',
        raw,
        'DeleteCollabRoleResponse',
        DeleteCollabRoleResponse.fromBuffer,
      );

  /// 判别 Envelope：`type` + 载荷字段（channel-contract.md §4 的 payload 表）。
  PaConnEvent _decodeEvent(Object? raw) {
    if (raw is! Map) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '事件 payload 须为 Map，实际 ${raw.runtimeType}',
      );
    }
    final Object? type = raw['type'];
    switch (type) {
      case 'conn_state':
        return PaConnEvent.connState(PaConnState.fromPlatform(raw));
      case 'host_info':
        final String where = '事件 host_info.info';
        return PaConnEvent.hostInfo(
          bytesOrThrow(where, raw['info'], 'HostInfo', HostInfo.fromBuffer),
        );
      case 'host_event':
        final String where = '事件 host_event.event';
        return PaConnEvent.hostEvent(
          bytesOrThrow(where, raw['event'], 'HostEvent', HostEvent.fromBuffer),
        );
      default:
        throw PaBridgeException(
          PaBridgeErrorCodes.decodeFailed,
          '未知事件类型 $type（契约见 flutter/docs/channel-contract.md §4；'
          '双端同 APK 同版本，未知类型 = 契约漂移，需同步改契约而非静默吞掉）',
        );
    }
  }
}

/// 宿主返回值必须是 proto wire 字节；回了别的类型即契约违约。
Uint8List _expectBytes(String where, Object? raw) {
  if (raw is Uint8List) {
    return raw;
  }
  throw PaBridgeException(
    PaBridgeErrorCodes.decodeFailed,
    '$where 须为 proto wire 字节(Uint8List)，实际 ${raw.runtimeType}',
  );
}

/// 十份同型解码的唯一收口：验形（[_expectBytes]）→ 反序列化（[parse]）→
/// 坏字节统一成 [PaBridgeException]（DECODE_FAILED）。
///
/// 【为什么收敛在这里】getHostInfo / listThreads / listDirectory / readFile /
/// listBranches 与七个 proto 事件/协同载荷，十处的差别只是 `where` 定位串与
/// `parse` 构造器，try/catch 形状逐字相同。泛型化后消息文本按调用点原样
/// 传入（[typeName] 进消息），逐字不变——测试钉的就是这两段文本。
/// 【为什么不表驱动方法面】每条方法的 Dart 文档注释是契约文档的一部分
/// （channel-contract.md 的人读镜像），表驱动会把注释冲散；这里只收内部
/// 解码路径，方法面声明保持逐条手写。
T bytesOrThrow<T extends GeneratedMessage>(
  String where,
  Object? raw,
  String typeName,
  T Function(List<int>) parse,
) {  final Uint8List bytes = _expectBytes(where, raw);
  try {
    return parse(bytes);
  } on InvalidProtocolBufferException catch (e) {
    throw PaBridgeException(
      PaBridgeErrorCodes.decodeFailed,
      '$where 的 $typeName 字节不可解析：$e',
    );
  }
}