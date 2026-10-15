// Chat 路由级页面（B3 第 5 波④）：TurnSession + 时间线 + 历史入口 +
// 输入区 + 周边动作的组装。
//
// 【只组装不重写】本文件决定五件怎么摆、信号怎么走，不重复任何一方的逻辑：
//   - 会话状态机归 `chat_session.dart`（startTurn/steer/cancel/clearQueue/
//     replayFrames 全是它的方法，这里只调）；
//   - 帧→卡投影归 `chat_timeline.dart`（`ChatTimelineAccumulator` 全量回放与
//     去重只走它，这里只喂）；
//   - 输入校验/斜杠门控归 C3 composer（`onSubmit: ValueChanged<String>` /
//     `onSlashPick: ValueChanged<SlashPick>` frozen 签名，这里只接）；
//   - 动作判定/文案归 `chat_actions_data.dart`，方法面归
//     `chat_actions_transport.dart`，这里只按结果摆。
//
// 【对外契约（frozen，主会话收口接 main.dart 路由用）】
// `ChatPage({required String threadId, ...})`：threadId 必需，其余全可选注入
// （session/timeline 依赖可注入 fake，测试钉组装不断言真通道）。
//
// 【时间线 live 投影的诚实边界】`_tiles` = accumulator 快照（用户消息 +
// 历史回放全量帧，权威）+ 会话 live 态的粗投影（正文/思考/审批占位/终局标记，
// 只保证"跑着能看见字在流"）。粗投影不替代 C2 回放：重进会话的权威历史走
// 历史页「恢复到会话」→ accumulator + session.replayFrames 双喂（幂等由 C1
// 保证）。

import 'package:flutter/material.dart';

import '../gen/pocketagent/v1/common.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import 'chat_actions.dart';
import 'chat_actions_data.dart';
import 'chat_actions_transport.dart';
import 'chat_composer.dart';
import 'chat_composer_data.dart' show SlashPick, SlashPickAction;
import 'chat_history.dart';
import 'chat_history_data.dart';
import 'chat_session.dart';
import 'chat_timeline.dart';

/// Chat 路由级页面（见文件头契约）。
class ChatPage extends StatefulWidget {
  const ChatPage({
    super.key,
    required this.threadId,
    this.agentId = '',
    this.capabilities = const <String>[],
    this.session,
    this.actionsTransport,
    this.historySource,
    this.onOpenHistory,
    this.onNotify,
  });

  /// 会话 id（`chat/{threadId}` 的 threadId；建流/首帧/回放的口径）。
  final String threadId;

  /// 会话归属 agent（可空：GetThread 还在路上时空串，门控按「无位」处理）。
  final String agentId;

  /// 能力位快照（`hostCapabilities` 透传；九项过滤与 composer 门控共用）。
  final List<String> capabilities;

  /// 会话状态机注入点（测试传带 fake 流的 `TurnSession`；生产不传即新建）。
  final TurnSession? session;

  /// 周边动作方法面注入点（测试传独立通道实例；生产默认通道）。
  final ChatActionsTransport? actionsTransport;

  /// 历史数据源注入点（测试传可控 fake；生产不传即真通道）。
  final ChatHistorySource? historySource;

  /// 历史入口覆盖（测试钉「点了历史」用；生产 null = 推历史页）。
  final VoidCallback? onOpenHistory;

  /// 通知出口覆盖（测试录串用；生产 null = SnackBar）。
  final ValueChanged<String>? onNotify;

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  late final TurnSession _session;
  late final bool _ownsSession;
  late final ChatActionsTransport _actionsTransport;
  late final ChatTimelineAccumulator _accumulator =
      ChatTimelineAccumulator();
  String _lastInput = '';
  bool _actionBusy = false;

  void _notify(String msg) {
    final ValueChanged<String>? fn = widget.onNotify;
    if (fn != null) {
      fn(msg);
      return;
    }
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg)),
    );
  }

  late final ChatActionRunner _runner = ChatActionRunner(
    transport: _actionsTransport,
    threadId: widget.threadId,
    notify: _notify,
  );

  @override
  void initState() {
    super.initState();
    _ownsSession = widget.session == null;
    _session = widget.session ?? TurnSession(threadId: widget.threadId);
    _actionsTransport = widget.actionsTransport ?? ChatActionsTransport();
    _session.addListener(_onSessionChanged);
  }

  @override
  void dispose() {
    _session.removeListener(_onSessionChanged);
    if (_ownsSession) _session.dispose();
    super.dispose();
  }

  void _onSessionChanged() {
    // 投递回执 Toast 文案是帧驱动的（TurnSession.takeDeliveryNotice 取后清
    // null，与 Kotlin「UI 层消费后清 null」同纪律）：这里是唯一消费点。
    final String? notice = _session.takeDeliveryNotice();
    if (notice != null) _notify(notice);
    if (mounted) setState(() {});
  }

  // ── 提交链路（composer → session） ──

  Future<void> _submit(String text) async {
    final TurnSession s = _session;
    _accumulator.addUserMessage(text);
    _lastInput = text;
    if (mounted) setState(() {});
    try {
      if (s.isRunning) {
        // 运行中追问走 steer（daemon 插话或排队，回执帧定，见 session 注释）。
        await s.steer(text);
      } else {
        await s.startTurn(
          TurnInput(
            threadId: widget.threadId,
            blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: text))],
          ),
        );
      }
    } on Exception catch (e) {
      _notify('发送失败：$e');
    }
  }

  void _onSlashPick(SlashPick pick) {
    switch (pick.action) {
      case SlashPickAction.fill:
        // FILL 已由 composer 回填输入框，这里无动作。
        break;
      case SlashPickAction.goal:
        showChatGoalSheet(
          context,
          onSave: (String goal) => _runner.setGoal(goal),
        );
        break;
      case SlashPickAction.compact:
        _runGuarded(() => _runner.compact());
        break;
      case SlashPickAction.review:
        showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          builder: (BuildContext context) => ChatReviewSheet(
            onStart: (String focus) {
              Navigator.of(context).pop();
              _runGuarded(() => _runner.startReview(focus: focus));
            },
          ),
        );
        break;
    }
  }

  Future<void> _runGuarded(Future<Object?> Function() fn) async {
    if (_actionBusy) return;
    _actionBusy = true;
    try {
      await fn();
    } finally {
      _actionBusy = false;
    }
  }

  // ── 历史入口 ──

  void _openHistory() {
    final VoidCallback? override = widget.onOpenHistory;
    if (override != null) {
      override();
      return;
    }
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (BuildContext context) => Scaffold(
          appBar: AppBar(title: const Text('会话历史')),
          body: ChatHistoryPage(
            threadId: widget.threadId,
            source: widget.historySource,
            onReplay: _replay,
          ),
        ),
      ),
    );
  }

  /// 「恢复到会话」：历史帧双喂（session 状态机 + 时间线投影，幂等由 C1/C2
  /// 两侧保证，本页只负责按序原样转发）。
  Future<void> _replay(List<TurnServerMessage> frames) async {
    _session.replayFrames(frames);
    _accumulator.applyFrames(frames);
    if (mounted) setState(() {});
  }

  // ── 更多面板 ──

  Future<void> _openMore() async {
    final List<ChatActionId> items = visibleChatActions(
      agentId: widget.agentId,
      capabilities: widget.capabilities,
      isRunning: _session.isRunning,
      queuedCount: _session.queuedCount,
    );
    final ChatActionId? picked =
        await showChatActionsPanel(context, items: items);
    if (picked == null || !mounted) return;
    await _dispatchAction(picked);
  }

  Future<void> _dispatchAction(ChatActionId id) async {
    switch (id) {
      case ChatActionId.cancel:
        await _runGuarded(() => _runner.cancelTurn());
        break;
      case ChatActionId.clearQueue:
        await _runGuarded(() async {
          final String notice = await _runner.clearQueue();
          // daemon 真清除成功后收敛本地态（transport 注释的分工）；失败串里
          // 带「失败」二字时不收敛——没清掉就别假装清掉了。
          if (!notice.contains('失败') && !notice.contains('未就绪')) {
            _session.clearQueue();
          }
          return notice;
        });
        break;
      case ChatActionId.compact:
        if (_session.isRunning) {
          _notify('轮次运行中，等完成后再压缩');
          return;
        }
        await _runGuarded(() => _runner.compact());
        break;
      case ChatActionId.fork:
        await _confirmFork();
        break;
      case ChatActionId.rollback:
        await _openRollback();
        break;
      case ChatActionId.review:
        if (_session.isRunning) {
          _notify('轮次运行中，等完成后再发起审查');
          return;
        }
        if (!mounted) return;
        await showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          builder: (BuildContext context) => ChatReviewSheet(
            onStart: (String focus) {
              Navigator.of(context).pop();
              _runGuarded(() => _runner.startReview(focus: focus));
            },
          ),
        );
        break;
      case ChatActionId.goal:
        if (!mounted) return;
        await showChatGoalSheet(
          context,
          onSave: (String goal) => _runner.setGoal(goal),
        );
        break;
      case ChatActionId.export:
        await _runGuarded(() => _runner.exportHtml());
        break;
      case ChatActionId.optimize:
        if (!mounted) return;
        await showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          builder: (BuildContext context) => ChatOptimizeSheet(
            onOptimize: (String text) => _runner.optimize(
              text,
              agentId: widget.agentId,
            ),
            onApply: (String optimized) {
              Navigator.of(context).pop();
              _notify('已回填改写结果到输入框（未发送）');
              _applyOptimized = optimized;
              _composerKey = UniqueKey();
              if (mounted) setState(() {});
            },
          ),
        );
        break;
    }
  }

  /// 优化结果回填（输入框初值形态：composer 是 State 持有 controller，初值只
  /// 在构造时生效——这里经 `_composerKey` 重建 composer 带入，与草稿恢复同形）。
  String _applyOptimized = '';
  Key _composerKey = UniqueKey();

  Future<void> _confirmFork() async {
    final bool? ok = await showDialog<bool>(
      context: context,
      builder: (BuildContext context) => AlertDialog(
        title: const Text('分叉会话'),
        content: Text(forkConsequence()),
        actions: <Widget>[
          TextButton(
            key: const ValueKey<String>('chat-fork-cancel'),
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          TextButton(
            key: const ValueKey<String>('chat-fork-confirm'),
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('确认分叉'),
          ),
        ],
      ),
    );
    if (ok == true && mounted) {
      await _runGuarded(() => _runner.fork());
    }
  }

  /// 回滚：回合总数来自真历史（`historyRoundCount` 数 user 起轮），不是本地
  /// 猜——总数未知/拉失败时如实 Toast，不摆假选项。
  Future<void> _openRollback() async {
    final ChatHistorySource source =
        widget.historySource ?? BridgeChatHistorySource();
    late final GetThreadHistoryResponse resp;
    try {
      resp = await source.loadHistory(widget.threadId);
    } on ChatHistoryLoadFailure catch (e) {
      _notify(e.message);
      return;
    }
    if (!mounted) return;
    final int total = historyRoundCount(resp);
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (BuildContext context) => ChatRollbackSheet(
        turnTotal: total,
        onConfirm: (int n) {
          Navigator.of(context).pop();
          _runGuarded(() => _runner.rollback(n, turnTotal: total));
        },
      ),
    );
  }

  // ── 时间线 tiles（权威快照 + live 粗投影） ──

  List<ChatTile> get _tiles {
    final List<ChatTile> tiles = _accumulator.tiles.toList();
    if (_session.streamText.isNotEmpty) {
      tiles.add(ChatTile(kind: ChatTileKind.text, text: _session.streamText));
    }
    if (_session.thinkingText.isNotEmpty) {
      tiles.add(
        ChatTile(
          kind: ChatTileKind.text,
          text: _session.thinkingText,
          thinking: true,
        ),
      );
    }
    final ApprovalRequest? pending = _session.pendingApproval;
    if (pending != null) {
      tiles.add(
        ChatTile(
          kind: ChatTileKind.approval,
          approvalId: pending.requestId,
          approvalReason: pending.reason,
          approvalDetails: pending.details,
        ),
      );
    }
    if (_session.phase == TurnPhase.completed) {
      tiles.add(const ChatTile(kind: ChatTileKind.completed));
    }
    if (_session.phase == TurnPhase.failed) {
      tiles.add(
        ChatTile(
          kind: ChatTileKind.failed,
          text: _session.errorMessage ?? '任务失败',
        ),
      );
    }
    return tiles;
  }

  void _onApproval(ApprovalAction action) {
    String reason = '';
    String details = '';
    for (final ChatTile t in _accumulator.tiles) {
      if (t.kind == ChatTileKind.approval && t.approvalId == action.requestId) {
        reason = t.approvalReason;
        details = t.approvalDetails;
      }
    }
    final ApprovalRequest? pending = _session.pendingApproval;
    if (reason.isEmpty && pending != null && pending.requestId == action.requestId) {
      reason = pending.reason;
      details = pending.details;
    }
    handleApprovalAction(
      context,
      transport: _actionsTransport,
      threadId: widget.threadId,
      action: action,
      reason: reason,
      details: details,
      notify: _notify,
    );
  }

  void _onRetry() {
    if (_lastInput.isEmpty || _session.isRunning) return;
    _submit(_lastInput);
  }

  // ── 渲染 ──

  @override
  Widget build(BuildContext context) {
    final bool running = _session.isRunning;
    return Scaffold(
      appBar: AppBar(
        title: const Text('会话'),
        actions: <Widget>[
          IconButton(
            key: const ValueKey<String>('chatpage-history'),
            tooltip: '历史',
            icon: const Icon(Icons.history_rounded),
            onPressed: _openHistory,
          ),
          IconButton(
            key: const ValueKey<String>('chatpage-more'),
            tooltip: '更多动作',
            icon: const Icon(Icons.more_horiz_rounded),
            onPressed: _openMore,
          ),
        ],
      ),
      body: Column(
        children: <Widget>[
          Expanded(
            child: ChatTimeline(
              tiles: _tiles,
              isRunning: running,
              errorText: _session.streamError,
              onApproval: _onApproval,
              onRetry: _lastInput.isEmpty ? null : _onRetry,
            ),
          ),
          ChatRunningBar(
            isRunning: running,
            queuedCount: _session.queuedCount,
            busy: _actionBusy,
            onCancel: () => _runGuarded(() => _runner.cancelTurn()),
            onClearQueue: () => _runGuarded(() async {
              final String notice = await _runner.clearQueue();
              if (!notice.contains('失败') && !notice.contains('未就绪')) {
                _session.clearQueue();
              }
              return notice;
            }),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(14, 4, 14, 12),
            child: ChatComposer(
              key: _composerKey,
              threadId: widget.threadId,
              agentId: widget.agentId,
              capabilities: widget.capabilities,
              initialText: _applyOptimized,
              onSubmit: _submit,
              onSlashPick: _onSlashPick,
            ),
          ),
        ],
      ),
    );
  }
}
