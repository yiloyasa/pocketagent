// Chat 轮次会话（B3 第 5 波①）：会话级 turn 状态机。
//
// 【对位】Android `ui/TurnSession.kt`（1816 行）：轮次启动 / 帧泵 / sequence
// 水位 / 断线 resync / runningThreadIds / CancelTurn / ClearQueuedInputs。
// 本文件是它的 Flutter 骨架：状态机 + 水位 + 流面接线先行；用量卡 / 思考落卡 /
// 检查点落盘 / 看门狗等 Android 侧能力留后续波次，不摆假字段。
//
// 【首帧走哪条：定为 stream 面统一】`startTurn` 只走 `TurnStreamBridge`：
// `turnStreamStart(threadId)` 建流 + `turnStreamSend(input 帧)` 发首帧。
// 旧 unary `PaBridge.startTurn(TurnInput)` 保留给兼容（B4 前宿主只有首帧方法、
// 无流面时的降级口），本类不调用它——两条首帧路径并存必有一条是幽灵，必须定一条。
//
// 【steer 语义】运行中追问经同一条流发 `TurnClientMessage{supplement}` 帧，
// daemon 侧 codex 插话（turn/steer）或入队，回执见 Steered/Queued 帧。
// hermes 的 Steer 恒返回 steered=false（ACP 无运行中插话，输入落队列）：
// 文案与徽标只跟回执帧（[deliveryAckNotice]），不按能力位预判——
// 与 TurnDelivery.kt:39-46 的修正同源，注释见 chat_timeline.dart。
//
// 【clearQueue 是本地收敛】daemon 侧 `ClearQueuedInputs` 一元 RPC 在本流面
// 无方法对应（冻结三方法 + turnResync 之外不加帧——`TurnClientMessage` 无清队帧）：
// [clearQueue] 只收敛本地排队计数与 phase（queued → running），真清除随 B4
// 方法面补齐（TODO 钉死，不假装已清 daemon 队列）。

import 'dart:async';

import 'package:flutter/foundation.dart';

import '../gen/pocketagent/v1/common.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/chat_turn_stream.dart';
import 'chat_timeline.dart'
    show deliveryAckNotice, kDeliveryQueued, kDeliverySteered;

/// 轮次相位（TurnSession.kt State.running/recovering/queuedCount/终局的收敛）。
enum TurnPhase {
  /// 空闲：无在飞轮次（可 startTurn）。
  idle,

  /// 运行中：流已建、等终局帧。
  running,

  /// 运行中且有排队输入（queuedCount > 0；daemon 紧接着会发 QueueFlushed +
  /// Started 跑下一条，流与运行态保持）。
  queued,

  /// 取消中：cancel 帧已发，等终局帧收尾。
  cancelling,

  /// 终局：失败（FAILED 帧已到，无排队剩余）。
  failed,

  /// 终局：完成（COMPLETED 帧已到，无排队剩余）。
  completed,
}

/// 会话级 turn 状态机（单会话实例；多会话各持一份，见 [runningThreadIds]）。
///
/// 【可注入】[stream] 默认走生产通道；测试注入独立通道名的
/// `TurnStreamBridge` + mock 宿主行为（mock 的是宿主 wire 行为，不是本类自己）。
/// 本类是 [ChangeNotifier]：phase/水位/正文/徽标任一变化即 notify（后续波次
/// mini 条与时间线直接订阅）。
class TurnSession extends ChangeNotifier {
  TurnSession({required this.threadId, TurnStreamBridge? stream})
      : _stream = stream ?? TurnStreamBridge();

  /// 会话 id（建流 / 续传 / 首帧的 threadId 口径）。
  final String threadId;

  final TurnStreamBridge _stream;
  StreamSubscription<TurnServerMessage>? _sub;

  TurnPhase _phase = TurnPhase.idle;
  int _lastSequence = 0;
  int _queuedCount = 0;
  bool _hasOverflow = false;
  bool _needsFullHistory = false;
  String _currentTurnId = '';
  String _streamText = '';
  String _thinkingText = '';
  ApprovalRequest? _pendingApproval;
  String? _errorMessage;
  String? _streamError;
  String? _deliveryNotice;

  /// 全局"仍在跑"的线程 id 快照（mini 条与 Home 共用——本波先暴露，后续波次消费）。
  ///
  /// 口径 = TurnSession.kt `runningThreadIds()`：running（含 queued /
  /// cancelling，即流未结算）才在集合里；终局/起流失败即移除。按需拉快照，
  /// 不改任何状态（只读）。
  static final Set<String> _runningIds = <String>{};
  static Set<String> get runningThreadIds =>
      Set<String>.unmodifiable(_runningIds);

  TurnPhase get phase => _phase;
  bool get isRunning =>
      _phase == TurnPhase.running ||
      _phase == TurnPhase.queued ||
      _phase == TurnPhase.cancelling;

  /// 已应用的事件断点（本会话收到的最大 `TurnServerMessage.sequence`）。
  ///
  /// 语义 = TurnSession.kt `lastSequence`：实时流帧与续传快照帧都推进它；
  /// sequence<=0 的帧不推进不拦截（旧 daemon 无续传能力）。
  int get lastSequence => _lastSequence;
  int get queuedCount => _queuedCount;

  /// 是否有过队列满拒收（overflow 徽标挂载位，时间线据此渲染「队列已满，未送达」）。
  bool get hasOverflow => _hasOverflow;

  /// 是否需退回全量回填（收到 ResyncRequired：断点已作废，增量无从谈起；
  /// 全量拉取留 C2，本波只置位）。
  bool get needsFullHistory => _needsFullHistory;
  String get currentTurnId => _currentTurnId;

  /// 流式正文累积（增量拼接；终局帧到达后保持，供时间线投影消费）。
  String get streamText => _streamText;
  String get thinkingText => _thinkingText;

  /// 挂起的审批请求（真交互留 C4；幽灵审批卡治理：approvalResolved 即撤卡）。
  ApprovalRequest? get pendingApproval => _pendingApproval;
  String? get errorMessage => _errorMessage;

  /// 事件流解码错误（坏帧不毒化流：记诊断，不改 phase；流保持打开）。
  String? get streamError => _streamError;

  /// 投递回执 Toast 文案（帧驱动；UI 消费后经 [takeDeliveryNotice] 清 null）。
  String? get deliveryNotice => _deliveryNotice;

  /// 取走回执文案（取后清 null，与 Kotlin「UI 层消费后清 null」同纪律）。
  String? takeDeliveryNotice() {
    final String? n = _deliveryNotice;
    _deliveryNotice = null;
    return n;
  }

  /// 开一轮（走 stream 面统一，见文件头）。
  ///
  /// 要求 [phase] 为 idle/completed/failed（运行中追问走 [steer]，否则抛
  /// StateError——两条入口语义不混）。成功后 phase=running 并进
  /// [runningThreadIds]；宿主缺席（HOST_UNAVAILABLE）如实上抛，phase 回 idle，
  /// 不留"假运行"。
  Future<void> startTurn(TurnInput input) async {
    if (isRunning) {
      throw StateError('轮次运行中，追问请走 steer()（startTurn 只开新轮）');
    }
    _resetTurnState();
    _phase = TurnPhase.running;
    _runningIds.add(threadId);
    _subscribe();
    notifyListeners();
    try {
      await _stream.turnStreamStart(threadId);
      await _stream.turnStreamSend(TurnClientMessage(input: input).writeToBuffer());
    } on Exception catch (_) {
      await _unsubscribe();
      _runningIds.remove(threadId);
      _phase = TurnPhase.idle;
      notifyListeners();
      rethrow;
    }
  }

  /// 运行中追问：经同一条流发 supplement 帧（daemon 插话或排队，回执帧定）。
  ///
  /// 要求运行中（idle/终局下调用抛 StateError，调用方走 [startTurn]）。
  /// 发送成功不改本地态——最终是 Steered 还是 Queued 只看回执帧（hermes 恒
  /// 落 Queued，见文件头）。
  Future<void> steer(String text) async {
    if (!isRunning) {
      throw StateError('当前无运行中轮次，steer 无处可插（请走 startTurn）');
    }
    if (text.trim().isEmpty) {
      throw ArgumentError.value(text, 'text', '追问文本不能为空');
    }
    final TurnClientMessage frame = TurnClientMessage(
      supplement: TurnSupplement(
        blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: text))],
      ),
    );
    await _stream.turnStreamSend(frame.writeToBuffer());
  }

  /// 终止正在跑的轮次：流内 cancel 帧优先（daemon 在流内直接 cancel）。
  ///
  /// 空闲/终局下调用是空操作（无轮次可终）。发送成功后 phase=cancelling，
  /// 本地收尾等随后到达的 FAILED/COMPLETED 帧；发送失败 phase 不变、错误上抛。
  Future<void> cancel() async {
    if (!isRunning) return;
    final TurnClientMessage frame = TurnClientMessage(
      cancel: CancelSignal(reason: '用户在手机端终止'),
    );
    await _stream.turnStreamSend(frame.writeToBuffer());
    _phase = TurnPhase.cancelling;
    notifyListeners();
  }

  /// 拆流资源（页面 dispose / 显式中断后调用）。
  ///
  /// 【为什么终局不自动调】双向流随 daemon 关流自行回收；Dart 侧只在"不再
  /// 消费本通道"时显式拆（dispose 路径），避免把终局帧之后紧跟的 QueueFlushed
  /// + Started 误杀。
  Future<void> closeStream() => _stream.turnStreamCancel();

  /// 清排队输入（本地收敛；daemon 真清除随 B4 方法面补齐，见文件头）。
  void clearQueue() {
    _queuedCount = 0;
    if (_phase == TurnPhase.queued) {
      _phase = TurnPhase.running;
    }
    notifyListeners();
  }

  /// 断线续传：`GetThreadEventsSince` 快照 → 按序补帧。
  ///
  /// transport 面是一次性方法面（server-stream 在 Dart 侧无 EventChannel
  /// 对应物：`turnResync(threadId, sinceSequence)` 取快照字节数组，Kotlin 侧
  /// 收齐流后一次返回，B4 语义）。快照经 [replayFrames] 喂入（幂等），终局帧
  /// 在其中结算；宿主缺席如实上抛，不伪造"已同步"。
  Future<void> resyncFromDaemon() async {
    final List<TurnServerMessage> frames =
        await _stream.turnResync(threadId, _lastSequence);
    replayFrames(frames);
  }

  /// 时间线回放 API（C2 调）：把历史帧按序喂进时间线。
  ///
  /// 幂等：同 sequence 不重复进（`_lastSequence` 水位判定，见 [applyFrame]）；
  /// C2 的恢复回放调它（replay=true：回放补历史，不写本次提交的回执 Toast）。
  void replayFrames(List<TurnServerMessage> frames) {
    for (final TurnServerMessage f in frames) {
      applyFrame(f, replay: true);
    }
  }

  /// 应用一帧（实时流与回放共用同一状态机；返回 true = 终局帧）。
  ///
  /// sequence 水位：`sequence > 0 && sequence <= _lastSequence` 的帧直接丢弃
  /// （重复/乱序到达）；`sequence > 0` 的新帧推进水位；0 不推进不拦截。
  bool applyFrame(TurnServerMessage frame, {bool replay = false}) {
    final int seq = frame.sequence.toInt();
    if (seq > 0) {
      if (seq <= _lastSequence) return false;
      _lastSequence = seq;
    }
    var sawEnd = false;
    switch (frame.whichMsg()) {
      case TurnServerMessage_Msg.started:
        _currentTurnId = frame.started.turnId;
      case TurnServerMessage_Msg.textDelta:
        _streamText += frame.textDelta.delta;
      case TurnServerMessage_Msg.thinkingDelta:
        final ThinkingDelta td = frame.thinkingDelta;
        if (td.snapshot.isNotEmpty) {
          _thinkingText = td.snapshot;
        } else {
          _thinkingText += td.delta;
        }
      case TurnServerMessage_Msg.toolCall:
        // 工具卡投影归 chat_timeline.dart；会话状态机不记工具态（骨架分工）。
        break;
      case TurnServerMessage_Msg.usage:
        // 用量进上下文窗口卡（后续波次）；骨架不记。
        break;
      case TurnServerMessage_Msg.codexEvent:
        final CodexEvent ev = frame.codexEvent;
        if (ev.kind == CodexEvent_Kind.ERROR && !ev.willRetry) {
          _errorMessage = ev.message;
        }
      case TurnServerMessage_Msg.approvalRequest:
        _pendingApproval = frame.approvalRequest;
      case TurnServerMessage_Msg.approvalResolved:
        final ApprovalResolved resolved = frame.approvalResolved;
        final ApprovalRequest? cur = _pendingApproval;
        if (cur == null ||
            resolved.requestId.isEmpty ||
            resolved.requestId == cur.requestId) {
          _pendingApproval = null;
        }
      case TurnServerMessage_Msg.steered:
        // 真插话：文案只报 daemon 实际回执（回放不写本次提交的 Toast）。
        if (!replay) {
          _deliveryNotice = deliveryAckNotice(kDeliverySteered);
        }
      case TurnServerMessage_Msg.queued:
        final int pos = frame.queued.position;
        if (pos > 0) {
          _queuedCount += 1;
          if (_phase == TurnPhase.running) _phase = TurnPhase.queued;
          if (!replay) {
            _deliveryNotice = deliveryAckNotice(kDeliveryQueued);
          }
        } else {
          // 队列满被拒收：daemon 没接收，置 overflow 徽标位，不摆 Toast。
          _hasOverflow = true;
        }
      case TurnServerMessage_Msg.queueFlushed:
        _queuedCount = _queuedCount > 0 ? _queuedCount - 1 : 0;
        if (_queuedCount == 0 && _phase == TurnPhase.queued) {
          _phase = TurnPhase.running;
        }
      case TurnServerMessage_Msg.completed:
        sawEnd = true;
        if (_streamText.isEmpty) {
          _streamText = _finalText(frame.completed.finalBlocks);
        }
        _errorMessage = null;
        _pendingApproval = null;
        _settleTerminal(TurnPhase.completed);
      case TurnServerMessage_Msg.failed:
        sawEnd = true;
        _errorMessage = frame.failed.error.message.isNotEmpty
            ? frame.failed.error.message
            : '任务失败';
        if (_streamText.isEmpty && frame.failed.partialText.isNotEmpty) {
          _streamText = frame.failed.partialText;
        }
        _pendingApproval = null;
        _settleTerminal(TurnPhase.failed);
      case TurnServerMessage_Msg.resyncRequired:
        // 断点作废：退回全量回填（全量拉取留 C2，本波只置位，不断言已恢复）。
        _lastSequence = 0;
        _needsFullHistory = true;
      case TurnServerMessage_Msg.notSet:
        break;
    }
    notifyListeners();
    return sawEnd;
  }

  @override
  void dispose() {
    unawaited(_unsubscribe());
    super.dispose();
  }

  // ── 内部 ──

  void _resetTurnState() {
    _queuedCount = 0;
    _hasOverflow = false;
    _needsFullHistory = false;
    _currentTurnId = '';
    _streamText = '';
    _thinkingText = '';
    _pendingApproval = null;
    _errorMessage = null;
    _streamError = null;
    _deliveryNotice = null;
  }

  void _settleTerminal(TurnPhase terminal) {
    if (_queuedCount > 0) {
      // 还有排队输入：daemon 紧接着发 QueueFlushed + Started 跑下一条，
      // 这里不收尾（保持运行态与流订阅，中途收尾会丢流引用）。
      if (_phase == TurnPhase.running || _phase == TurnPhase.cancelling) {
        _phase = TurnPhase.queued;
      }
      return;
    }
    _phase = terminal;
    _runningIds.remove(threadId);
    unawaited(_unsubscribe());
  }

  void _subscribe() {
    unawaited(_unsubscribe());
    _sub = _stream.events.listen(
      (TurnServerMessage frame) => applyFrame(frame),
      onError: (Object e, StackTrace _) {
        // 坏帧不毒化流：记诊断，不改 phase，流保持打开等后续帧。
        _streamError = e.toString();
        notifyListeners();
      },
    );
  }

  Future<void> _unsubscribe() async {
    final StreamSubscription<TurnServerMessage>? sub = _sub;
    _sub = null;
    if (sub != null) await sub.cancel();
  }

  String _finalText(Iterable<ContentBlock> blocks) {
    final StringBuffer sb = StringBuffer();
    for (final ContentBlock b in blocks) {
      if (b.hasText() && b.text.text.isNotEmpty) sb.write(b.text.text);
    }
    return sb.toString();
  }
}
