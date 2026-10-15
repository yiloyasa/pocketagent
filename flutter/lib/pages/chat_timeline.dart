// Chat 时间线骨架（B3 第 5 波①）：帧 → 消息卡的投影 + tile 渲染。
//
// 【对位】Android `ui/chat/ChatTimeline.kt`（时间线渲染）与 `ui/TurnDelivery.kt`
// （投递状态机纯函数）。本文件回答三件事：① 投递徽标/Toast 文案的 Dart 移植
// （与 TurnDelivery.kt 逐条对位，见下）；② 帧累积器 [ChatTimelineAccumulator]
// （同 thread 同 sequence 去重 + 增量拼接，C2 恢复回放与实时流共用同一投影）；
// ③ 五型 tile 渲染 [ChatTimeline]（文本 / 工具调用 / 审批占位 / 完成 / 失败）。
//
// 【审批卡真交互留 C4】审批 tile 只渲染占位 + 回调出口（[ApprovalAction] 经
// [ChatTimeline.onApproval] 交出，requestId 原样回填）；C4 才接应答链路。
// 【codex 事件卡留后续波次】CodexEvent 帧本骨架忽略（用量/上下文窗口卡同理），
// 不摆假卡——五型之外不渲染即诚实。

import 'package:flutter/material.dart';

import '../gen/pocketagent/v1/common.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';

// ── TurnDelivery.kt 移植 ──────────────────────────────────────────────────
//
// 【为什么移植到这里】徽标文案是时间线渲染的输入（徽标挂在用户消息 tile 下），
// 投递回执 Toast 文案与徽标同源（deliveryAckNotice）；TurnSession 的状态机
// （chat_session.dart）复用同一份函数——单源，不存在第二份口径。
// flutter/lib 内此前无 TurnDelivery 对位（grep 确认：仅 task_center 注释提及），
// 故按 TurnDelivery.kt:17-52 原样移植（含 hermes 修正注释）。

/// 已发出，等 daemon 回执。
const String kDeliveryPending = 'pending';

/// 已插入运行中的轮次（codex turn/steer 成功）。
const String kDeliverySteered = 'steered';

/// 排队中，等当前轮次终局后自动执行。
const String kDeliveryQueued = 'queued';

/// 队列满被拒收 —— 这条消息 daemon 没有接收。
const String kDeliveryOverflow = 'overflow';

/// 时间线小标签文案（空串 = 不显示标签，普通用户消息）。
///
/// 口径 = TurnDelivery.kt `deliveryLabel`：pending →「发送中…」；
/// steered →「已插入运行中任务」；queued → position>1「排队中 · 第N位」否则
/// 「排队中 · 下一条」；overflow →「队列已满，未送达」；其余空串。
String deliveryLabel(String state, int queuePos) {
  switch (state) {
    case kDeliveryPending:
      return '发送中…';
    case kDeliverySteered:
      return '已插入运行中任务';
    case kDeliveryQueued:
      return queuePos > 1 ? '排队中 · 第$queuePos位' : '排队中 · 下一条';
    case kDeliveryOverflow:
      return '队列已满，未送达';
    default:
      return '';
  }
}

/// 投递回执的 Toast 文案（帧驱动，与 [deliveryLabel] 徽标同源）。
///
/// Steered/Queued 帧到达时按 daemon 的实际回执展示，不按能力位预判：
/// hermes 带 `steer:<id>` 位，但 daemon 的 Steer 恒返回 steered=false
/// （ACP 协议无运行中插话，输入落队列）——提交瞬间按位报"已插入"是不实文案
/// （TurnDelivery.kt:39-46 同款修正）。其余状态返回 null = 不摆 Toast。
String? deliveryAckNotice(String deliverState) {
  switch (deliverState) {
    case kDeliverySteered:
      return '已插入运行中任务';
    case kDeliveryQueued:
      return '已排队，当前轮结束后执行';
    default:
      return null;
  }
}

/// Queued 回执是否被 daemon 接受（position<=0 = 拒收，不得计入排队数）。
///
/// 口径 = TurnDelivery.kt `queuedAckAccepted`。
bool queuedAckAccepted(int position) => position > 0;

// ── Tile 模型（五型） ─────────────────────────────────────────────────────

/// 时间线 tile 五型：文本 / 工具调用 / 审批占位 / 完成 / 失败。
enum ChatTileKind {
  /// 用户消息与助手正文（含思考流落卡：[ChatTile.thinking] 置真）。
  text,

  /// 工具调用卡（一次调用一条：STARTED 建卡，FINISHED/UPDATE 原地回填）。
  toolCall,

  /// 审批占位卡（真交互留 C4：只渲染 + [ApprovalAction] 回调出口）。
  approval,

  /// 轮次完成标记（正文已在 text tile 落卡，本 tile 只做终局标记）。
  completed,

  /// 轮次失败标记（[ChatTile.text] = 人话错误原因）。
  failed,
}

/// 审批动作（回调出口载荷）：requestId 原样回填，approved 为用户选择。
class ApprovalAction {
  const ApprovalAction({required this.requestId, required this.approved});

  final String requestId;
  final bool approved;
}

/// 时间线上一张卡。
class ChatTile {
  const ChatTile({
    required this.kind,
    this.text = '',
    this.thinking = false,
    this.toolName = '',
    this.toolStatus = '',
    this.toolId = '',
    this.approvalId = '',
    this.approvalReason = '',
    this.approvalDetails = '',
    this.deliveryState = '',
    this.queuePos = 0,
    this.sequence = 0,
    this.isUser = false,
  });

  final ChatTileKind kind;

  /// 正文（text tile）/ 工具输出（toolCall tile）/ 错误原因（failed tile）。
  final String text;

  /// text tile 是否来自思考流（渲染为弱化样式，与正文区分）。
  final bool thinking;

  /// 工具名（空 → 渲染兜底「工具」，与 Kotlin `ifBlank{"工具"}` 同纪律）。
  final String toolName;

  /// 工具状态：running / success / failure（透传，不另造档位）。
  final String toolStatus;

  /// 工具调用 id（结果回填的匹配键）。
  final String toolId;

  /// 审批请求 id（回调出口原样回填）。
  final String approvalId;

  /// 审批理由（agent 给出的 reason）。
  final String approvalReason;

  /// 审批详情（options/details 的人话摘要，C4 展开三按钮）。
  final String approvalDetails;

  /// 投递态（用户 tile）：pending / steered / queued / overflow，空 = 普通消息。
  final String deliveryState;

  /// 排队位（queued 徽标「第N位」用）。
  final int queuePos;

  /// 来源帧 sequence（去重/排序用；0 = 无序号帧）。
  final int sequence;

  /// 是否用户消息（徽标挂载位；false = 助手/系统卡）。
  final bool isUser;

  /// Steered/Queued 徽标文案（空串 = 不显示）。
  String get deliveryBadge => deliveryLabel(deliveryState, queuePos);

  ChatTile copyWith({String? deliveryState, int? queuePos, String? text, String? toolStatus}) {
    return ChatTile(
      kind: kind,
      text: text ?? this.text,
      thinking: thinking,
      toolName: toolName,
      toolStatus: toolStatus ?? this.toolStatus,
      toolId: toolId,
      approvalId: approvalId,
      approvalReason: approvalReason,
      approvalDetails: approvalDetails,
      deliveryState: deliveryState ?? this.deliveryState,
      queuePos: queuePos ?? this.queuePos,
      sequence: sequence,
      isUser: isUser,
    );
  }
}

// ── 帧累积器 ──────────────────────────────────────────────────────────────

/// 帧 → tile 的投影（纯 Dart，无 widget 依赖，可单测直测）。
///
/// 【去重】同 sequence 只进一次（`_seenSeq`；sequence<=0 的帧恒应用——
/// 旧 daemon 无续传能力，不作废不拦截）。【增量拼接】textDelta 追加进草稿，
/// 工具卡/审批卡/终局帧到达时落卡（与 TurnSession.kt commitThinking 三时点同思路：
/// 第一段正文 / 第一个工具调用 / 本轮结束）。
/// 【回放共用】实时流与 C2 恢复回放（经 TurnSession.replayFrames 喂帧）走同一
/// 投影，回放出的时间线与"没断过线"一致。
class ChatTimelineAccumulator {
  final List<ChatTile> _tiles = <ChatTile>[];
  final Set<int> _seenSeq = <int>{};
  final StringBuffer _draft = StringBuffer();
  final StringBuffer _thinking = StringBuffer();
  int _maxSeq = 0;

  /// 当前 tile 快照（只读）。
  List<ChatTile> get tiles => List<ChatTile>.unmodifiable(_tiles);

  /// 已应用的最大 sequence（断线续传点；0 = 无可信断点）。
  int get lastSequence => _maxSeq;

  /// 用户发一条消息（先挂「发送中…」，等 Steered/Queued 回执帧改徽标）。
  void addUserMessage(String text, {String deliveryState = kDeliveryPending}) {
    _tiles.add(ChatTile(
      kind: ChatTileKind.text,
      text: text,
      isUser: true,
      deliveryState: deliveryState,
    ));
  }

  /// 按序喂多帧（C2 恢复回放调这个；幂等：同 sequence 不重复进）。
  void applyFrames(Iterable<TurnServerMessage> frames) {
    for (final TurnServerMessage f in frames) {
      applyFrame(f);
    }
  }

  /// 应用一帧。返回 true = 终局帧（completed/failed），调用方据此收尾。
  bool applyFrame(TurnServerMessage frame) {
    final int seq = frame.sequence.toInt();
    if (seq > 0) {
      if (!_seenSeq.add(seq)) return false;
      if (seq > _maxSeq) _maxSeq = seq;
    }
    switch (frame.whichMsg()) {
      case TurnServerMessage_Msg.textDelta:
        _flushThinking();
        _draft.write(frame.textDelta.delta);
        return false;
      case TurnServerMessage_Msg.thinkingDelta:
        final ThinkingDelta td = frame.thinkingDelta;
        if (td.snapshot.isNotEmpty) {
          // 权威全文（thinking_end 终局/历史回放）：整体替换，不依赖拼接完整性。
          _thinking
            ..clear()
            ..write(td.snapshot);
        } else {
          _thinking.write(td.delta);
        }
        return false;
      case TurnServerMessage_Msg.toolCall:
        _onToolCall(frame.toolCall, seq);
        return false;
      case TurnServerMessage_Msg.approvalRequest:
        _flushThinking();
        _flushDraft(seq);
        final ApprovalRequest req = frame.approvalRequest;
        _tiles.add(ChatTile(
          kind: ChatTileKind.approval,
          approvalId: req.requestId,
          approvalReason: req.reason,
          approvalDetails: req.details,
          sequence: seq,
        ));
        return false;
      case TurnServerMessage_Msg.approvalResolved:
        _onApprovalResolved(frame.approvalResolved, seq);
        return false;
      case TurnServerMessage_Msg.steered:
        _applyDeliveryAck(kDeliverySteered, 0);
        return false;
      case TurnServerMessage_Msg.queued:
        final int pos = frame.queued.position;
        if (queuedAckAccepted(pos)) {
          _applyDeliveryAck(kDeliveryQueued, pos);
        } else {
          // 队列满被拒收：这条消息 daemon 没接收，徽标如实呈现，不摆 Toast。
          _applyDeliveryAck(kDeliveryOverflow, 0);
        }
        return false;
      case TurnServerMessage_Msg.queueFlushed:
        // 队首开始执行：所有「排队中」变回普通用户消息。
        for (var i = 0; i < _tiles.length; i++) {
          final ChatTile t = _tiles[i];
          if (t.isUser && t.deliveryState == kDeliveryQueued) {
            _tiles[i] = t.copyWith(deliveryState: '', queuePos: 0);
          }
        }
        return false;
      case TurnServerMessage_Msg.completed:
        _flushThinking();
        final String body = _takeDraft();
        final String finalText = _finalText(frame.completed.finalBlocks);
        if (body.isNotEmpty) {
          _tiles.add(ChatTile(kind: ChatTileKind.text, text: body, sequence: seq));
        } else if (finalText.isNotEmpty) {
          // 终局兜底：没发过 delta 的轮次，正文只在 completed.final_blocks。
          _tiles.add(ChatTile(kind: ChatTileKind.text, text: finalText, sequence: seq));
        }
        _tiles.add(ChatTile(kind: ChatTileKind.completed, sequence: seq));
        return true;
      case TurnServerMessage_Msg.failed:
        _flushThinking();
        final String partial = _takeDraft();
        if (partial.isNotEmpty) {
          _tiles.add(ChatTile(kind: ChatTileKind.text, text: partial, sequence: seq));
        } else if (frame.failed.partialText.isNotEmpty) {
          _tiles.add(ChatTile(
            kind: ChatTileKind.text,
            text: frame.failed.partialText,
            sequence: seq,
          ));
        }
        final String err = frame.failed.error.message.isNotEmpty
            ? frame.failed.error.message
            : '任务失败';
        _tiles.add(ChatTile(kind: ChatTileKind.failed, text: err, sequence: seq));
        return true;
      case TurnServerMessage_Msg.started:
      case TurnServerMessage_Msg.usage:
      case TurnServerMessage_Msg.codexEvent:
      case TurnServerMessage_Msg.resyncRequired:
      case TurnServerMessage_Msg.notSet:
        // started：轮次开始标记（turnId 归会话状态机记，投影不摆卡）；
        // usage：用量进上下文窗口卡（后续波次），时间线不摆；
        // codexEvent：原生事件卡留后续波次，本骨架不摆假卡；
        // resyncRequired：断点作废信号归 TurnSession（needsFullHistory），投影层忽略；
        // notSet：空帧忽略。
        return false;
    }
  }

  // ── 内部 ──

  void _flushThinking() {
    if (_thinking.isEmpty) return;
    _tiles.add(ChatTile(
      kind: ChatTileKind.text,
      text: _thinking.toString().trim(),
      thinking: true,
    ));
    _thinking.clear();
  }

  void _flushDraft(int seq) {
    if (_draft.isEmpty) return;
    _tiles.add(ChatTile(kind: ChatTileKind.text, text: _draft.toString(), sequence: seq));
    _draft.clear();
  }

  String _takeDraft() {
    final String s = _draft.toString();
    _draft.clear();
    return s;
  }

  void _onToolCall(ToolCallUpdate tc, int seq) {
    if (tc.phase == ToolCallUpdate_Phase.STARTED) {
      _flushThinking();
      _flushDraft(seq);
      _tiles.add(ChatTile(
        kind: ChatTileKind.toolCall,
        toolName: tc.call.name.isNotEmpty ? tc.call.name : '工具',
        toolStatus: 'running',
        toolId: tc.call.id,
        sequence: seq,
      ));
      return;
    }
    // FINISHED/UPDATE：结果回填到同一次调用的那张卡（同 id 原地更新），
    // 而不是再插一条——工具卡"一次调用一条"（TurnSession.kt TOOL_CALL 分支同纪律）。
    var idx = -1;
    for (var i = _tiles.length - 1; i >= 0; i--) {
      final ChatTile t = _tiles[i];
      if (t.kind != ChatTileKind.toolCall) continue;
      if (tc.call.id.isNotEmpty && t.toolId == tc.call.id) {
        idx = i;
        break;
      }
      if (tc.call.id.isEmpty && t.toolStatus == 'running') {
        idx = i;
        break;
      }
    }
    final String name = tc.call.name.isNotEmpty
        ? tc.call.name
        : (idx >= 0 ? _tiles[idx].toolName : '工具');
    final String callId =
        tc.call.id.isNotEmpty ? tc.call.id : (idx >= 0 ? _tiles[idx].toolId : '');
    String output = '';
    String status = 'success';
    if (tc.hasResult()) {
      output = tc.result.output;
      status = tc.result.ok ? 'success' : 'failure';
    }
    final ChatTile updated = ChatTile(
      kind: ChatTileKind.toolCall,
      text: output,
      toolName: name,
      toolStatus: status,
      toolId: callId,
      sequence: seq,
    );
    if (idx >= 0) {
      _tiles[idx] = updated;
    } else {
      _tiles.add(updated);
    }
  }

  void _onApprovalResolved(ApprovalResolved resolved, int seq) {
    // 幽灵审批卡治理：该审批已在电脑端被处理，立即撤卡并附一条说明。
    var idx = -1;
    for (var i = _tiles.length - 1; i >= 0; i--) {
      final ChatTile t = _tiles[i];
      if (t.kind != ChatTileKind.approval) continue;
      if (resolved.requestId.isEmpty || t.approvalId == resolved.requestId) {
        idx = i;
        break;
      }
    }
    if (idx >= 0) _tiles.removeAt(idx);
    final String note =
        resolved.note.isNotEmpty ? resolved.note : '这条审批已在电脑端被处理';
    _tiles.add(ChatTile(kind: ChatTileKind.text, text: '审批已在电脑端处理：$note', sequence: seq));
  }

  /// 回执落到时间线：Steered/Queued 都指向最早一条还没回执的用户消息
  /// （同一条流先发先回，顺序天然一致），原位更新投递态。
  void _applyDeliveryAck(String state, int queuePos) {
    for (var i = 0; i < _tiles.length; i++) {
      final ChatTile t = _tiles[i];
      if (t.isUser && t.deliveryState == kDeliveryPending) {
        _tiles[i] = t.copyWith(deliveryState: state, queuePos: queuePos);
        return;
      }
    }
  }

  String _finalText(Iterable<ContentBlock> blocks) {
    final StringBuffer sb = StringBuffer();
    for (final ContentBlock b in blocks) {
      if (b.hasText() && b.text.text.isNotEmpty) sb.write(b.text.text);
    }
    return sb.toString();
  }
}

// ── Widget（时间线骨架） ───────────────────────────────────────────────────

/// Chat 时间线骨架：五型 tile 渲染 + Steered/Queued 徽标 + 空态。
///
/// [tiles] 由 [ChatTimelineAccumulator]（或 TurnSession 投影）供给；
/// [isRunning] 尾部挂「运行中…」；[errorText] 非空时顶部挂失败横幅；
/// 审批卡按钮经 [onApproval] 交出（真交互留 C4）；失败卡重试经 [onRetry]。
class ChatTimeline extends StatelessWidget {
  const ChatTimeline({
    super.key,
    required this.tiles,
    this.isRunning = false,
    this.errorText,
    this.onApproval,
    this.onRetry,
  });

  final List<ChatTile> tiles;
  final bool isRunning;
  final String? errorText;
  final ValueChanged<ApprovalAction>? onApproval;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    if (tiles.isEmpty && !isRunning && (errorText == null || errorText!.isEmpty)) {
      return const Center(child: Text('还没有消息，先发一句话开始吧'));
    }
    return ListView(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
      children: <Widget>[
        if (errorText != null && errorText!.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text('同步失败：$errorText',
                style: const TextStyle(color: Colors.red)),
          ),
        for (final ChatTile tile in tiles) _tileWidget(context, tile),
        if (isRunning)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 8),
            child: Text('运行中…', style: TextStyle(color: Colors.grey)),
          ),
      ],
    );
  }

  Widget _tileWidget(BuildContext context, ChatTile tile) {
    switch (tile.kind) {
      case ChatTileKind.text:
        return _textTile(tile);
      case ChatTileKind.toolCall:
        return _toolTile(tile);
      case ChatTileKind.approval:
        return _approvalTile(tile);
      case ChatTileKind.completed:
        return const Padding(
          padding: EdgeInsets.symmetric(vertical: 4),
          child: Text('任务完成', style: TextStyle(color: Colors.green)),
        );
      case ChatTileKind.failed:
        return _failedTile(tile);
    }
  }

  Widget _textTile(ChatTile tile) {
    final TextStyle style = tile.thinking
        ? const TextStyle(color: Colors.grey, fontStyle: FontStyle.italic)
        : TextStyle(color: tile.isUser ? Colors.blue : Colors.black);
    final String body = tile.thinking ? '思考：${tile.text}' : tile.text;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Column(
        crossAxisAlignment:
            tile.isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: <Widget>[
          Text(body, style: style),
          if (tile.deliveryBadge.isNotEmpty)
            Text(tile.deliveryBadge,
                style: const TextStyle(color: Colors.grey, fontSize: 12)),
        ],
      ),
    );
  }

  Widget _toolTile(ChatTile tile) {
    final String head = '工具：${tile.toolName}（${tile.toolStatus}）';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(head, style: const TextStyle(fontWeight: FontWeight.bold)),
          if (tile.text.isNotEmpty) Text(tile.text),
        ],
      ),
    );
  }

  Widget _approvalTile(ChatTile tile) {
    // 占位卡：真交互（应答链路）留 C4；按钮只做回调出口。
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            const Text('需要审批', style: TextStyle(fontWeight: FontWeight.bold)),
            if (tile.approvalReason.isNotEmpty) Text(tile.approvalReason),
            if (tile.approvalDetails.isNotEmpty)
              Text(tile.approvalDetails,
                  style: const TextStyle(color: Colors.grey)),
            Row(
              children: <Widget>[
                TextButton(
                  onPressed: onApproval == null
                      ? null
                      : () => onApproval!(
                          ApprovalAction(requestId: tile.approvalId, approved: true)),
                  child: const Text('允许'),
                ),
                TextButton(
                  onPressed: onApproval == null
                      ? null
                      : () => onApproval!(
                          ApprovalAction(requestId: tile.approvalId, approved: false)),
                  child: const Text('拒绝'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _failedTile(ChatTile tile) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text('任务失败：${tile.text}',
              style: const TextStyle(color: Colors.red)),
          if (onRetry != null)
            TextButton(onPressed: onRetry, child: const Text('重试')),
        ],
      ),
    );
  }
}
