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
import 'package:protobuf/protobuf.dart' show InvalidProtocolBufferException;

import '../gen/pocketagent/v1/host.pb.dart';
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

  // ── 方法面（首批 6 条；宿主侧一律【Kotlin 宿主桥 B4 波次实现】）──────────

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

  HostInfo _decodeHostInfo(Object? raw) =>
      _parseHostInfo('getHostInfo 返回值', _expectBytes('getHostInfo 返回值', raw));

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
          _parseHostInfo(where, _expectBytes(where, raw['info'])),
        );
      case 'host_event':
        final String where = '事件 host_event.event';
        return PaConnEvent.hostEvent(
          _parseHostEvent(where, _expectBytes(where, raw['event'])),
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

/// proto 返回值必须是 wire 字节；宿主回了别的类型即契约违约。
Uint8List _expectBytes(String where, Object? raw) {
  if (raw is Uint8List) {
    return raw;
  }
  throw PaBridgeException(
    PaBridgeErrorCodes.decodeFailed,
    '$where 须为 proto wire 字节(Uint8List)，实际 ${raw.runtimeType}',
  );
}

/// 坏字节不裸抛 protobuf 异常：上层 UI 只认 [PaBridgeException]，
/// 异常类型混两种会逼每个调用方各写一遍兜底（迟早漏一个）。
HostInfo _parseHostInfo(String where, Uint8List bytes) {
  try {
    return HostInfo.fromBuffer(bytes);
  } on InvalidProtocolBufferException catch (e) {
    throw PaBridgeException(
      PaBridgeErrorCodes.decodeFailed,
      '$where 的 HostInfo 字节不可解析：$e',
    );
  }
}

HostEvent _parseHostEvent(String where, Uint8List bytes) {
  try {
    return HostEvent.fromBuffer(bytes);
  } on InvalidProtocolBufferException catch (e) {
    throw PaBridgeException(
      PaBridgeErrorCodes.decodeFailed,
      '$where 的 HostEvent 字节不可解析：$e',
    );
  }
}
