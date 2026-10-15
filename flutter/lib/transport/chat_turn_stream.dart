// Chat 轮次双向流的 Dart 形态（B3 第 5 波①）：`TurnService.StartTurn` 的
// transport 流面。
//
// 【为什么单拆一个文件】`pa_bridge.dart` 只到 unary 首帧（`startTurn(TurnInput)`：
// 只发首帧、不管回流，注释写明过程帧另行接入）——本文件就是那个「另行」：
// 双向流的建流/发帧/拆流 + 过程帧的事件消费 + 断线续传的一次性快照。方法面
// （Dart → Kotlin）与事件面（Kotlin → Dart）各走一条通道，见下。
//
// 【通道】
//   - 方法面：复用主方法通道 `dev.pocketagent/bridge`（`kPaBridgeMethodChannel`，
//     与 PaBridge 同一通道——宿主在同一 handler 表里多注册四个方法名即可）：
//       `turnStreamStart`  (args `{'threadId': String}`) → null：宿主为该会话建
//         一条 StartTurn 双向流（首帧由紧随的 turnStreamSend 发起帧承载）。
//       `turnStreamSend`  (args `TurnClientMessage` wire 字节) → null：往已建的
//         流里发一帧（input 首帧 / supplement 追问 / approval 应答 / cancel
//         取消都走这里，由调用方按 proto 封帧，本类只验形透传）。
//       `turnStreamCancel` (args null) → null：宿主拆掉本通道上的流资源。
//       `turnResync`      (args `{'threadId': String, 'afterSequence': int}`)
//         → `List<Uint8List>`：`GetThreadEventsSince` 快照的一次性形态。
//   - 事件面：`dev.pocketagent/bridge/turn_events`（本文件常量
//     [kTurnStreamEventChannel]）：Kotlin 宿主订阅 StartTurn 双向流，逐帧推
//     `TurnServerMessage` wire 字节（B4 实现）。
//
// 【server-stream 为什么收成一次性方法】Dart 侧 EventChannel 是宿主推流、
// 没有"请求一次、回一段流"的请求应答形态；而 `GetThreadEventsSince` 是
// server-stream（请求带 after_sequence、回完即关）。契约定为：Dart 发一次
// `turnResync`，Kotlin 侧收齐流后一次返回字节数组（`List<Uint8List>`，空 =
// 无新帧）。B4 语义，见 [TurnStreamBridge.turnResync]。
//
// 【宿主缺席语义（F1b 定案）】方法面无 handler → `PaBridgeException(
// HOST_UNAVAILABLE)`（调用方进「轮次通道未就绪」诚实占位）；事件面无推流 →
// 流静默（不等事件报错——判定宿主存活一律走方法面探针，与 pa_bridge §2.5 同纪律）。
//
// 【错误模型】沿用 `PaBridgeException` + `PaBridgeErrorCodes`（import
// pa_bridge.dart 只读复用：`bytesOrThrow` 收口 + 异常/错误码，不碰其实现）。
// 坏帧 → 流错误 `DECODE_FAILED`，流保持打开（坏一帧不杀整条流，与主事件面同纪律）。

import 'dart:async';

import 'package:flutter/services.dart';

import '../gen/pocketagent/v1/thread.pb.dart';
import 'pa_bridge.dart';

/// 轮次事件通道名（Kotlin → Dart，推 `TurnServerMessage` wire 字节）。
///
/// 【B4】Kotlin 宿主桥 B4 波次按此名注册 StreamHandler；在此之前宿主缺席，
/// 事件流静默（见文件头 F1b 定案）。
const String kTurnStreamEventChannel = 'dev.pocketagent/bridge/turn_events';

/// 方法面方法名（契约字符串，双端逐字对齐；前三条为 C2 同步的冻结名，不许改名）。
const String kTurnStreamStartMethod = 'turnStreamStart';
const String kTurnStreamSendMethod = 'turnStreamSend';
const String kTurnStreamCancelMethod = 'turnStreamCancel';

/// 续传快照方法名（本流面 additive：server-stream 一次性形态，B4 语义）。
const String kTurnResyncMethod = 'turnResync';

/// 轮次双向流的 transport 封装（可注入构造，方便测试隔离）。
///
/// 【为什么可注入而不是单例】与 PaBridge 同纪律：测试按通道名 mock 宿主侧
/// handler；注入通道让测试用独立通道与生产隔离。生产用默认名即可。
class TurnStreamBridge {
  TurnStreamBridge({
    MethodChannel? methodChannel,
    EventChannel? eventChannel,
  })  : _methods = methodChannel ?? const MethodChannel(kPaBridgeMethodChannel),
        _events = eventChannel ?? const EventChannel(kTurnStreamEventChannel);

  final MethodChannel _methods;
  final EventChannel _events;

  /// 为 [threadId] 建一条 StartTurn 双向流（宿主侧建流，首帧另由 [turnStreamSend] 发）。
  ///
  /// arguments：`{'threadId': threadId}`（Map 形态：后续若加建流参数只增键，
  /// 不换形态）。返回 null。
  /// 【Kotlin 宿主桥 B4 波次实现】宿主缺席如实抛 HOST_UNAVAILABLE。
  Future<void> turnStreamStart(String threadId) => _invokeExpectNull(
        kTurnStreamStartMethod,
        <String, Object?>{'threadId': threadId},
      );

  /// 往已建的流里发一帧 `TurnClientMessage`（调用方按 proto 封帧：input 首帧 /
  /// supplement 追问 / approval 应答 / cancel 取消都走这里）。
  ///
  /// arguments：`TurnClientMessage.writeToBuffer()` 的 wire 字节。返回 null。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> turnStreamSend(Uint8List clientMessageBytes) =>
      _invokeExpectNull(kTurnStreamSendMethod, clientMessageBytes);

  /// 拆掉本通道上的流资源（页面 dispose / 显式中断后调用；轮次终局帧的到达
  /// 不自动调它——双向流随 daemon 关流自行回收，见 chat_session.dart 注释）。
  ///
  /// arguments：null。返回 null。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> turnStreamCancel() =>
      _invokeExpectNull(kTurnStreamCancelMethod, null);

  /// 断线续传的一次性快照 —— proto `ThreadService.GetThreadEventsSince` 的
  /// 方法面形态（server-stream 在 Dart 侧无 EventChannel 对应物）。
  ///
  /// arguments：`{'threadId': threadId, 'afterSequence': sinceSequence}`
  /// （afterSequence = 本机已应用的最大 seq，`GetThreadEventsSinceRequest`
  /// 的字段口径）。返回：`List<Uint8List>`（每元素一帧 `TurnServerMessage`
  /// wire 字节；空 = 无新帧）。Kotlin 侧收齐流后一次返回（B4 实现约定）。
  ///
  /// 【解码位置】transport 层逐元素验形 + 反序列化（坏元素即 DECODE_FAILED，
  /// 定位到下标）；调用方（TurnSession.resyncFromDaemon）拿到的是成型帧，
  /// 只做状态机应用。
  /// 【Kotlin 宿主桥 B4 波次实现】宿主缺席如实抛 HOST_UNAVAILABLE。
  Future<List<TurnServerMessage>> turnResync(
    String threadId,
    int sinceSequence,
  ) =>
      _invoke(
        kTurnResyncMethod,
        <String, Object?>{'threadId': threadId, 'afterSequence': sinceSequence},
        _decodeResync,
      );

  /// 轮次事件流（EventChannel `dev.pocketagent/bridge/turn_events`）。
  ///
  /// 载荷 = `TurnServerMessage` wire 字节（`Uint8List`）逐帧直推。
  /// 【无缓冲 broadcast】订阅即收，订阅前推过的帧不补——续传点靠 sequence
  /// 水位 + [turnResync] 补，事件只承担增量（与主事件面 §4 同纪律）。
  /// 【坏帧不毒化流】载荷违约 → 流错误 `DECODE_FAILED`（消息定位到通道名），
  /// 流保持打开，后续帧照收。
  /// 【宿主缺席 = 静默】无推流时本流不报错（F1b 定案）；宿主存活探针走方法面
  /// （任一方法会如实得 HOST_UNAVAILABLE）。
  Stream<TurnServerMessage> get events => _decodedEvents;

  late final Stream<TurnServerMessage> _decodedEvents = _events
      .receiveBroadcastStream()
      .map(_decodeFrame)
      .transform(
        StreamTransformer<TurnServerMessage, TurnServerMessage>.fromHandlers(
          handleError: (
            Object error,
            StackTrace stack,
            EventSink<TurnServerMessage> sink,
          ) {
            // 与 PaBridge.events 同纪律：宿主侧错误收敛成 PaBridgeException，
            // UI 只需要认识一种异常；PaBridgeException（解码失败等已定性错误）原样透传。
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
                  error.message ?? '轮次事件通道无 handler：$kTurnStreamEventChannel',
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
  //
  // 【为什么与 PaBridge._invoke 同形】跨通道的调用纪律（MissingPlugin→
  // HOST_UNAVAILABLE、PlatformException 透传、无返回语义验 null）是契约本身；
  // pa_bridge.dart 的私有方法不可复用，这里按同一契约手写两份小函数，
  // 解码收口仍复用公开的 [bytesOrThrow]（H4 产物）。

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

  Future<void> _invokeExpectNull(String method, Object? arguments) async {
    final Object? raw = await _invoke<Object?>(method, arguments, (r) => r);
    if (raw != null) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '$method 契约要求宿主回 null，实际 ${raw.runtimeType}',
      );
    }
  }

  TurnServerMessage _decodeFrame(Object? raw) => bytesOrThrow(
        '轮次事件 $kTurnStreamEventChannel 帧',
        raw,
        'TurnServerMessage',
        TurnServerMessage.fromBuffer,
      );

  List<TurnServerMessage> _decodeResync(Object? raw) {
    if (raw is! List) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '$kTurnResyncMethod 契约要求宿主回 List<Uint8List> 快照，'
        '实际 ${raw.runtimeType}',
      );
    }
    final List<TurnServerMessage> out = <TurnServerMessage>[];
    for (var i = 0; i < raw.length; i++) {
      out.add(bytesOrThrow(
        '$kTurnResyncMethod 快照[$i]',
        raw[i],
        'TurnServerMessage',
        TurnServerMessage.fromBuffer,
      ));
    }
    return out;
  }
}
