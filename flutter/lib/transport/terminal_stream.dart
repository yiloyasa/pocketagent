// 终端流面（B3 第 6 波）：daemon `TerminalService` 的 Dart 桥形态。
//
// 【与 C1 的 turn 流面同型、不可共文件】同一套形状（EventChannel 按帧推流 +
// unary 快照 + attach/write/close 方法 + `PaBridgeException` 错误模型），但
// 通道名、方法名、帧类型全部不同 —— 共文件会把两个服务的契约字符串混进同一
// 处，改一个服务的名就得动另一个服务的文件。模式复用，文件隔离。
//
// 【通道】
//   方法面：复用 `dev.pocketagent/bridge`（与 `PaBridge` 同一 MethodChannel，
//     方法名字符串不同，无冲突；宿主 B4 波次按名注册）。
//   事件面：`dev.pocketagent/bridge/terminal_events` —— 宿主订阅 daemon
//     `TerminalService.Attach` 双向流，逐帧把 `TerminalServerMessage` 的 wire
//     字节推下来（B4 实现；宿主缺席 = 流静默，与 `PaBridge` 事件面同款语义，
//     见下 `events` 注释）。
//
// 【xterm 不做】本文件只搬字节，不解析 ANSI、不做终端仿真；输出块按纯文本块
// 渲染（等宽、可滚动、增量拼接、超长截断，见 terminal_data.dart）。
// WebView/xterm 桥留后续波次。

import 'dart:async';

import 'package:flutter/services.dart';

import '../gen/pocketagent/v1/terminal.pb.dart';
import 'pa_bridge.dart'
    show
        PaBridgeErrorCodes,
        PaBridgeException,
        bytesOrThrow,
        kPaBridgeMethodChannel;

/// 终端事件通道：宿主 → Dart 的 `TerminalServerMessage` 逐帧推送。
///
/// 【B4】Kotlin 宿主桥 B4 波次按此名注册 StreamHandler；在此之前宿主缺席，
/// 事件流静默（握手错被框架吞掉，不进流 —— 与 `PaBridge.events` 同款行为，
/// 判定宿主在不在必须走方法面探针，不能等事件流报错）。
const String kTerminalEventChannel = 'dev.pocketagent/bridge/terminal_events';

/// 终端流桥：`TerminalService` 的 Dart 侧形状（可注入构造，方便测试隔离）。
///
/// 【为什么可注入而不是单例】与 `PaBridge` 同纪律：测试要按通道名 mock 宿主
/// 侧 handler；注入通道让测试用独立通道与生产隔离。生产用默认名即可。
class TerminalStreamBridge {
  TerminalStreamBridge({
    MethodChannel? methodChannel,
    EventChannel? eventChannel,
  })  : _methods = methodChannel ?? const MethodChannel(kPaBridgeMethodChannel),
        _events = eventChannel ?? const EventChannel(kTerminalEventChannel);

  final MethodChannel _methods;
  final EventChannel _events;

  /// 会话列表快照 —— proto `TerminalService.ListTerminals`
  /// （proto/v1/terminal.proto:16）。
  ///
  /// arguments：`ListTerminalsRequest` 的 wire 字节（空请求 = 空字节数组，
  /// 编码总则与 `getHostInfo` 同款）。返回：`ListTerminalsResponse` 的 wire
  /// 字节 → gen stub 反序列化（sessions[] 逐条是 proto `TerminalSessionInfo`）。
  ///
  /// 【空列表 = 真空态】daemon 没有存活会话时回空数组而不是错误；**通道不可
  /// 用是另一条路径**（HOST_UNAVAILABLE → 页面错误态），两者严格区分，不许用
  /// 空列表冒充错误（防假空态，与 listThreads 同纪律）。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListTerminalsResponse> listTerminals([
    ListTerminalsRequest? request,
  ]) =>
      _invoke(
        'listTerminals',
        (request ?? ListTerminalsRequest()).writeToBuffer(),
        (Object? raw) => bytesOrThrow(
          'listTerminals 返回值',
          raw,
          'ListTerminalsResponse',
          ListTerminalsResponse.fromBuffer,
        ),
      );

  /// 订阅一个终端的 Attach 双向流 —— 告诉宿主「把这个 id 的 PTY 流接进来」。
  ///
  /// arguments：`{'terminalId': String}`（proto `OpenTerminal.session_id` /
  /// `CloseTerminalRequest.session_id` 的那串 id；复用已存在会话时服务端回
  /// `Resumed` 帧，见 terminal.proto:68-74）。返回：null = 订阅受理（首帧
  /// `opened`/`resumed` 走事件面，不走本返回值）。
  ///
  /// 【open 帧不走这里】`TerminalClientMessage{open}`（含 cwd/shell/cols/rows）
  /// 由调用方按 proto 封帧后经 [terminalWrite] 送入流；本方法只做订阅管理。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> terminalAttach(String terminalId) =>
      _invokeExpectNull('terminalAttach', <String, String>{
        'terminalId': terminalId,
      });

  /// 往 Attach 流里写一帧调用方封好的 `TerminalClientMessage` 字节。
  ///
  /// arguments：`TerminalClientMessage` 的 wire 字节（input/resize/open 帧由
  /// 调用方按 proto 封帧，封帧函数见 terminal_data.dart 的
  /// `terminalInputFrame` / `terminalResizeFrame` / `terminalOpenFrame`）。
  /// 返回：null = 宿主已收下（不代表 daemon 已执行，执行结果看事件面回帧）。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> terminalWrite(Uint8List clientMessageBytes) =>
      _invokeExpectNull('terminalWrite', clientMessageBytes);

  /// 关闭一个终端 —— proto `TerminalService.CloseTerminal`。
  ///
  /// arguments：`{'terminalId': String}`。返回：null = 关闭受理。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> terminalClose(String terminalId) =>
      _invokeExpectNull('terminalClose', <String, String>{
        'terminalId': terminalId,
      });

  /// 终端事件流（EventChannel `dev.pocketagent/bridge/terminal_events`）。
  ///
  /// 元素 = 解码后的 `TerminalServerMessage`（opened/output/exit/error/
  /// resumed 五帧，见 terminal.proto:40-48）。输出帧的 `data` 是原始 PTY
  /// 字节（含 ANSI），由消费方原文透传渲染，不在这里转义。
  ///
  /// 【无缓冲语义】broadcast 流：订阅即收，订阅前推过的帧不补 —— PTY 本来就
  /// 无回放（terminal.proto:69「PTY 无回放」），丢帧不补是协议诚实，不是 bug。
  /// 【坏帧不毒化流】payload 违约 → 流错误 `DECODE_FAILED`，流保持打开，后续
  /// 帧照收（断连重连会连续推多帧，一帧坏不能杀整条流）。
  /// 【宿主缺席不进流】'listen' 握手错由框架转投 `FlutterError.reportError`，
  /// 不会变成流错误 —— 判定宿主在不在必须走方法面探针。
  Stream<TerminalServerMessage> get events => _decodedEvents;

  late final Stream<TerminalServerMessage> _decodedEvents = _events
      .receiveBroadcastStream()
      .map(_decodeEvent)
      .transform(
        StreamTransformer<TerminalServerMessage, TerminalServerMessage>
            .fromHandlers(
          handleError: (
            Object error,
            StackTrace stack,
            EventSink<TerminalServerMessage> sink,
          ) {
            // 与 `PaBridge.events` 同款收敛：宿主侧错误统一成 PaBridgeException，
            // UI 只需要认识一种异常；已定性的 PaBridgeException 原样透传。
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
                  error.message ?? '事件通道无 handler：$kTerminalEventChannel',
                ),
                stack,
              );
            } else {
              sink.addError(error, stack);
            }
          },
        ),
      );

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
    final Object? raw = await _invoke<Object?>(method, arguments, (Object? r) => r);
    if (raw != null) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '$method 契约要求宿主回 null，实际 ${raw.runtimeType}',
      );
    }
  }

  /// 单帧解码：事件 payload 必须是 `TerminalServerMessage` wire 字节。
  TerminalServerMessage _decodeEvent(Object? raw) => bytesOrThrow(
        'terminal_events 事件',
        raw,
        'TerminalServerMessage',
        TerminalServerMessage.fromBuffer,
      );
}
