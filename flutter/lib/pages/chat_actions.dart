// Chat 周边动作装配（B3 第 5 波④）：审批真交互 + 更多面板 + 运行中操作条 +
// 目标/回滚/审查/优化四张小表。
//
// 【装配不重写】本文件只做三件事：① timeline 占位审批卡点按 →
// `PAApprovalSheet` 真交互 → `SubmitApproval` → Toast；② 更多面板九项按
// [visibleChatActions] 过滤展示，点选经 [ChatActionRunner] 执行对应 RPC；
// ③ 运行中操作条（终止/清排队，running 才显示）。纯判定与文案全在
// `chat_actions_data.dart`，方法面全在 `chat_actions_transport.dart`——这里
// 不重复口径，只做接线。

import 'package:flutter/material.dart';

import '../components/pa_approval_sheet.dart';
import '../components/pa_button.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart'
    show PaBridgeErrorCodes, PaBridgeException;
import 'chat_actions_data.dart';
import 'chat_actions_transport.dart';
import 'chat_timeline.dart' show ApprovalAction;

// ── 动作执行器 ──────────────────────────────────────────────────────────

/// 周边动作执行器：transport + threadId + 通知出口配齐，调用方只管点。
///
/// 【通知纪律】每个方法成功/失败都经 [notify] 报一句人话（成功返回通知串，
/// 失败返回人话失败串——调用方无需再分支，widget 测试钉的就是这些串）；
/// optimize/fork 另有结构化返回（见各方法），失败时为 null（已通知）。
class ChatActionRunner {
  ChatActionRunner({
    required this.transport,
    required this.threadId,
    required this.notify,
  });

  final ChatActionsTransport transport;
  final String threadId;
  final ValueChanged<String> notify;

  /// 提交审批决定（timeline 占位卡点按 → 审批表确认后调这里）。
  Future<String> submitApproval(
    ApprovalAction action, {
    String note = '',
    String option = '',
  }) =>
      _guard(
        '审批',
        () async {
          await transport.submitApproval(
            approvalSubmitRequest(
              threadId: threadId,
              decision: approvalDecisionFrame(
                requestId: action.requestId,
                approved: action.approved,
                note: note,
                option: option,
              ),
            ),
          );
          return approvalResultNotice(approved: action.approved);
        },
      );

  /// 终止当前轮次（一元 CancelTurn 兜底；流内 cancel 走 `TurnSession.cancel`，
  /// 见 transport 注释的分工）。
  Future<String> cancelTurn() => _guard(
        '终止轮次',
        () async {
          await transport.cancelTurn(threadId);
          return '已发送终止请求，等待轮次收尾';
        },
      );

  /// 清空排队输入（daemon 真清除 + 本地收敛由调用方随后调
  /// `TurnSession.clearQueue`，见 transport 注释）。
  Future<String> clearQueue() => _guard(
        '清空排队',
        () async {
          final ClearQueuedInputsResponse r =
              await transport.clearQueuedInputs(threadId);
          return queueClearedNotice(r.steered, r.followUp);
        },
      );

  /// 压缩会话历史（调用方保证运行态不点，见 transport 注释）。
  Future<String> compact() => _guard(
        '压缩会话',
        () async {
          await transport.compactThread(threadId);
          return '压缩请求已提交，完成后会提示压缩状态';
        },
      );

  /// 分叉会话（lastTurnId 空 = 从最新处分叉；返回新会话 id，空 = 失败已通知）。
  Future<String?> fork({String lastTurnId = ''}) async {
    final String? v = validateForkLastTurnId(lastTurnId);
    if (v != null) {
      notify(v);
      return null;
    }
    return _guard(
      '分叉会话',
      () async {
        final Thread t =
            await transport.forkThread(threadId, lastTurnId: lastTurnId);
        return t.id.isNotEmpty
            ? '已分叉出新会话'
            : '已分叉出新会话（新 id 待同步）';
      },
    );
  }

  /// 回滚 N 回合（参数先过 [validateRollbackTurns]，不过桥拦）。
  Future<String> rollback(int numTurns, {int turnTotal = 1 << 30}) {
    final String? v = validateRollbackTurns(numTurns, turnTotal);
    if (v != null) {
      notify(v);
      return Future<String>.value(v);
    }
    return _guard(
      '回滚',
      () async {
        await transport.rollbackThread(threadId, numTurns);
        return '已回滚 $numTurns 回合';
      },
    );
  }

  /// 设置会话目标（空目标本地拦，不过桥）。
  Future<String> setGoal(String goal) {
    final String? v = validateThreadGoal(goal);
    if (v != null) {
      notify(v);
      return Future<String>.value(v);
    }
    return _guard(
      '设置目标',
      () async {
        await transport.setThreadGoal(threadId, goal.trim());
        return '已设置会话目标';
      },
    );
  }

  /// 发起计划审查（fire-and-return：事件流回原会话，这里只报受理）。
  Future<String> startReview({String focus = ''}) => _guard(
        '计划审查',
        () async {
          final StartReviewResponse r = await transport.startReview(
            threadId,
            focus: normalizeReviewFocus(focus),
          );
          return r.started
              ? '已发起计划审查，审查结果将以事件流回本会话'
              : '审查请求已受理（daemon 未确认开始）';
        },
      );

  /// 导出 HTML（返回落盘 path，空 = 失败已通知；调用方按 path 经 ReadFile 取）。
  Future<String?> exportHtml() => _guard(
        '导出 HTML',
        () async {
          final ExportThreadHtmlResponse r =
              await transport.exportThreadHtml(threadId);
          return r.path.isNotEmpty ? '已导出：${r.path}' : '导出完成（路径待同步）';
        },
      );

  /// 优化提示词（返回前后文本对，null = 失败已通知；只读，不动会话状态）。
  Future<OptimizePair?> optimize(
    String text, {
    String style = '',
    String agentId = '',
    String modelId = '',
    String effort = '',
  }) async {
    final String? v = validateOptimizeInput(text);
    if (v != null) {
      notify(v);
      return null;
    }
    try {
      final OptimizePromptResponse r = await transport.optimizePrompt(
        text: text,
        style: style,
        agentId: agentId,
        modelId: modelId,
        effort: effort,
      );
      return OptimizePair(original: text, optimized: r.optimized);
    } on PaBridgeException catch (e) {
      notify(_failureText('优化提示词', e));
      return null;
    }
  }

  // ── 内部 ──

  /// 执行 + 通知收口：成功报受理串；`PaBridgeException` 报人话失败串。
  Future<T> _guard<T>(String label, Future<T> Function() fn) async {
    try {
      final T ok = await fn();
      if (ok is String) notify(ok);
      return ok;
    } on PaBridgeException catch (e) {
      final String msg = _failureText(label, e);
      notify(msg);
      // 调用方（面板/sheet）靠返回值继续流程：失败时 String 方法原样返回
      // 人话串，非 String 方法返回 null（已通知，不再抛——抛了等于让 UI 再翻
      // 一次同样的错）。
      if (null is T) return null as T;
      return msg as T;
    }
  }

  String _failureText(String label, PaBridgeException e) {
    if (e.code == PaBridgeErrorCodes.hostUnavailable) {
      return '桌面端动作通道未就绪（B4 波次）：$label暂不可用。';
    }
    if (e.message.isNotEmpty) return '$label失败：${e.message}';
    return '$label失败（${e.code}）';
  }
}

// ── 审批真交互 ──────────────────────────────────────────────────────────

/// timeline 占位审批卡点按后的真交互：弹 `PAApprovalSheet` → 用户拍板 →
/// `SubmitApproval` → 结果 Toast。
///
/// [reason]/[details] 来自占位卡的 tile 载荷（原样展示，不改写）；sheet 被
/// 取消（返回 null）时直接收，不发 RPC、不报 Toast——没拍板就没有动作。
Future<void> handleApprovalAction(
  BuildContext context, {
  required ChatActionsTransport transport,
  required String threadId,
  required ApprovalAction action,
  String reason = '',
  String details = '',
  String note = '',
  required ValueChanged<String> notify,
}) async {
  final bool? ok = await PAApprovalSheet.show(
    context,
    PAApprovalSheet(
      title: '需要审批',
      riskLabel: action.approved ? '风险：待确认' : '风险：待确认',
      detail: reason.isEmpty ? null : reason,
      info: <String, String>{
        if (details.isNotEmpty) '详情': details,
        '请求': action.requestId,
      },
      onAllow: () => Navigator.of(context).pop(true),
      onDeny: () => Navigator.of(context).pop(false),
      onCancel: () => Navigator.of(context).pop(),
    ),
  );
  if (ok == null) return;
  await ChatActionRunner(
    transport: transport,
    threadId: threadId,
    notify: notify,
  ).submitApproval(
    ApprovalAction(requestId: action.requestId, approved: ok),
    note: note,
  );
}

// ── 运行中操作条 ────────────────────────────────────────────────────────

/// 运行中操作条：终止 + 清排队（running 才显示，否则收成空盒——调用方无需再
/// 分支，显示纪律收敛在这里）。
class ChatRunningBar extends StatelessWidget {
  const ChatRunningBar({
    super.key,
    required this.isRunning,
    this.queuedCount = 0,
    this.busy = false,
    this.onCancel,
    this.onClearQueue,
  });

  final bool isRunning;
  final int queuedCount;
  final bool busy;
  final VoidCallback? onCancel;
  final VoidCallback? onClearQueue;

  @override
  Widget build(BuildContext context) {
    if (!isRunning) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      child: Row(
        children: <Widget>[
          Expanded(
            child: PAButton(
              key: const ValueKey<String>('chat-running-cancel'),
              label: busy ? '正在终止…' : '终止当前轮次',
              variant: PAButtonVariant.danger,
              expanded: true,
              onPressed: (busy || onCancel == null) ? null : onCancel,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: PAButton(
              key: const ValueKey<String>('chat-running-clear'),
              label: queuedCount > 0 ? '清空排队（$queuedCount）' : '清空排队',
              variant: PAButtonVariant.neutral,
              expanded: true,
              onPressed:
                  (busy || onClearQueue == null) ? null : onClearQueue,
            ),
          ),
        ],
      ),
    );
  }
}

// ── 更多面板 ────────────────────────────────────────────────────────────

/// 更多面板：九项行（调用方按 [visibleChatActions] 预过滤后传入，点选经
/// [onAction] 交出——面板不认能力位、不调 RPC，只做展示与分发）。
class ChatActionsPanel extends StatelessWidget {
  const ChatActionsPanel({
    super.key,
    required this.items,
    required this.onAction,
  });

  final List<ChatActionId> items;
  final ValueChanged<ChatActionId> onAction;

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) {
      return const SafeArea(
        top: false,
        child: Padding(
          padding: EdgeInsets.fromLTRB(20, 16, 20, 24),
          child: Text('暂无可用动作'),
        ),
      );
    }
    return SafeArea(
      top: false,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          const Padding(
            padding: EdgeInsets.fromLTRB(20, 12, 20, 4),
            child: Text(
              '会话动作',
              style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
            ),
          ),
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: items.length,
              itemBuilder: (BuildContext context, int i) {
                final ChatActionId id = items[i];
                return ListTile(
                  key: ValueKey<String>('chat-action-${id.name}'),
                  title: Text(chatActionLabel(id)),
                  subtitle: _subtitle(id),
                  onTap: () => onAction(id),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget? _subtitle(ChatActionId id) {
    switch (id) {
      case ChatActionId.fork:
        return const Text('复制出一个新会话，原会话不动');
      case ChatActionId.rollback:
        return const Text('抹掉最近若干回合，只改对话记录');
      case ChatActionId.compact:
      case ChatActionId.cancel:
      case ChatActionId.clearQueue:
      case ChatActionId.review:
      case ChatActionId.goal:
      case ChatActionId.export:
      case ChatActionId.optimize:
        return null;
    }
  }
}

/// 以底部弹层展示更多面板；返回被点的动作 id（取消返回 null）。
Future<ChatActionId?> showChatActionsPanel(
  BuildContext context, {
  required List<ChatActionId> items,
}) {
  return showModalBottomSheet<ChatActionId>(
    context: context,
    isScrollControlled: true,
    builder: (BuildContext context) => ChatActionsPanel(
      items: items,
      onAction: (ChatActionId id) => Navigator.of(context).pop(id),
    ),
  );
}

// ── 目标表 ──────────────────────────────────────────────────────────────

/// 设置会话目标表（输入 + 保存；空目标本地拦，不调 [onSave]）。
class ChatGoalSheet extends StatefulWidget {
  const ChatGoalSheet({super.key, this.initialGoal = '', this.onSave});

  final String initialGoal;

  /// 保存出口（null = 只展示不保存，保存键禁用——不摆点了没反应的入口）。
  final ValueChanged<String>? onSave;

  @override
  State<ChatGoalSheet> createState() => _ChatGoalSheetState();
}

class _ChatGoalSheetState extends State<ChatGoalSheet> {
  late final TextEditingController _controller =
      TextEditingController(text: widget.initialGoal);
  String? _error;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            const Text(
              '设置会话目标',
              style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
            ),
            const SizedBox(height: 4),
            const Text(
              '目标会作为常驻上下文，agent 每一轮都会看到',
              style: TextStyle(color: Colors.grey, fontSize: 12),
            ),
            const SizedBox(height: 10),
            TextField(
              key: const ValueKey<String>('chat-goal-field'),
              controller: _controller,
              minLines: 3,
              maxLines: 6,
              decoration: InputDecoration(
                hintText: '例如：全量改 Kotlin，改动只碰 daemon 模块…',
                errorText: _error,
              ),
            ),
            const SizedBox(height: 12),
            PAButton(
              key: const ValueKey<String>('chat-goal-save'),
              label: '保存目标',
              expanded: true,
              onPressed: widget.onSave == null
                  ? null
                  : () {
                      final String? v = validateThreadGoal(_controller.text);
                      if (v != null) {
                        setState(() => _error = v);
                        return;
                      }
                      widget.onSave!(_controller.text.trim());
                    },
            ),
          ],
        ),
      ),
    );
  }
}

/// 以底部弹层展示目标表。
Future<void> showChatGoalSheet(
  BuildContext context, {
  String initialGoal = '',
  required ValueChanged<String> onSave,
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    builder: (BuildContext context) => ChatGoalSheet(
      initialGoal: initialGoal,
      onSave: (String goal) {
        Navigator.of(context).pop();
        onSave(goal);
      },
    ),
  );
}

// ── 回滚表 ──────────────────────────────────────────────────────────────

/// 回滚表（回合选项 + 后果说明 + 确认；无回合可回时只展示说明，不摆确认键）。
class ChatRollbackSheet extends StatefulWidget {
  const ChatRollbackSheet({super.key, required this.turnTotal, this.onConfirm});

  /// 当前会话回合总数（选项与上限全从它算）。
  final int turnTotal;

  /// 确认出口（null = 只展示不执行，确认键禁用）。
  final ValueChanged<int>? onConfirm;

  @override
  State<ChatRollbackSheet> createState() => _ChatRollbackSheetState();
}

class _ChatRollbackSheetState extends State<ChatRollbackSheet> {
  late int _picked = rollbackTurnOptions(widget.turnTotal).isEmpty
      ? 0
      : rollbackTurnOptions(widget.turnTotal).first;

  @override
  Widget build(BuildContext context) {
    final List<int> options = rollbackTurnOptions(widget.turnTotal);
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            const Text(
              '回滚会话',
              style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
            ),
            const SizedBox(height: 4),
            Text(
              options.isEmpty
                  ? '当前没有可回滚的回合'
                  : rollbackConsequence(_picked),
              style: const TextStyle(color: Colors.grey, fontSize: 12),
            ),
            const SizedBox(height: 8),
            Flexible(
              child: RadioGroup<int>(
                groupValue: _picked,
                // onChanged 非空类型（RadioGroup 契约）：不可选时由 RadioListTile
                // 的 enabled:false 挡交互，这里收null值做防御。
                onChanged: (int? v) {
                  if (v != null && widget.onConfirm != null) {
                    setState(() => _picked = v);
                  }
                },
                child: ListView.builder(
                  shrinkWrap: true,
                  itemCount: options.length,
                  itemBuilder: (BuildContext context, int i) {
                    final int n = options[i];
                    return RadioListTile<int>(
                      key: ValueKey<String>('chat-rollback-opt-$n'),
                      title: Text(rollbackOptionLabel(n, widget.turnTotal)),
                      value: n,
                      toggleable: true,
                      enabled: widget.onConfirm != null,
                    );
                  },
                ),
              ),
            ),
            if (options.isNotEmpty) ...<Widget>[
              const SizedBox(height: 8),
              PAButton(
                key: const ValueKey<String>('chat-rollback-confirm'),
                label: '确认回滚',
                variant: PAButtonVariant.danger,
                expanded: true,
                onPressed: widget.onConfirm == null
                    ? null
                    : () => widget.onConfirm!(_picked),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

// ── 审查表 ──────────────────────────────────────────────────────────────

/// 发起计划审查表（焦点可选填，空 = daemon 按默认口径审）。
class ChatReviewSheet extends StatefulWidget {
  const ChatReviewSheet({super.key, this.onStart});

  /// 发起出口（null = 只展示不执行，发起键禁用）。
  final ValueChanged<String>? onStart;

  @override
  State<ChatReviewSheet> createState() => _ChatReviewSheetState();
}

class _ChatReviewSheetState extends State<ChatReviewSheet> {
  final TextEditingController _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            const Text(
              '发起计划审查',
              style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
            ),
            const SizedBox(height: 4),
            const Text(
              '审查结果将以事件流回到本会话，可空着直接发起',
              style: TextStyle(color: Colors.grey, fontSize: 12),
            ),
            const SizedBox(height: 10),
            TextField(
              key: const ValueKey<String>('chat-review-field'),
              controller: _controller,
              minLines: 2,
              maxLines: 4,
              decoration: const InputDecoration(
                hintText: '审查侧重（可选），例如：只看并发安全…',
              ),
            ),
            const SizedBox(height: 12),
            PAButton(
              key: const ValueKey<String>('chat-review-start'),
              label: '发起审查',
              expanded: true,
              onPressed: widget.onStart == null
                  ? null
                  : () => widget.onStart!(normalizeReviewFocus(_controller.text)),
            ),
          ],
        ),
      ),
    );
  }
}

// ── 优化表 ──────────────────────────────────────────────────────────────

/// 优化提示词表（输入 → 优化 → 前后对照 → 应用；只读语义，应用即把改写后
/// 文本经 [onApply] 交出，调用方回填输入框，不直接发送）。
class ChatOptimizeSheet extends StatefulWidget {
  const ChatOptimizeSheet({super.key, this.onOptimize, this.onApply});

  /// 优化执行（null = 只展示输入框，优化键禁用）。
  final Future<OptimizePair?> Function(String text)? onOptimize;

  /// 应用出口（改写后文本；null = 不展示应用键）。
  final ValueChanged<String>? onApply;

  @override
  State<ChatOptimizeSheet> createState() => _ChatOptimizeSheetState();
}

class _ChatOptimizeSheetState extends State<ChatOptimizeSheet> {
  final TextEditingController _controller = TextEditingController();
  bool _working = false;
  String? _error;
  OptimizePair? _pair;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _run() async {
    final Future<OptimizePair?> Function(String)? fn = widget.onOptimize;
    if (fn == null || _working) return;
    final String? v = validateOptimizeInput(_controller.text);
    if (v != null) {
      setState(() => _error = v);
      return;
    }
    setState(() {
      _working = true;
      _error = null;
    });
    try {
      final OptimizePair? pair = await fn(_controller.text);
      if (!mounted) return;
      setState(() {
        _working = false;
        _pair = pair;
        if (pair != null && !pair.hasChange) {
          _error = pair.optimized.trim().isEmpty ? '无需改写，原样可用' : '改写前后一致，原样可用';
        }
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _working = false;
        _error = '优化失败';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final OptimizePair? pair = _pair;
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            const Text(
              '优化提示词',
              style: TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
            ),
            const SizedBox(height: 10),
            TextField(
              key: const ValueKey<String>('chat-optimize-input'),
              controller: _controller,
              minLines: 3,
              maxLines: 6,
              decoration: InputDecoration(
                hintText: '粘贴要改写的提示词…',
                errorText: _error,
              ),
            ),
            const SizedBox(height: 12),
            PAButton(
              key: const ValueKey<String>('chat-optimize-run'),
              label: _working ? '正在优化…' : '优化',
              expanded: true,
              onPressed:
                  (widget.onOptimize == null || _working) ? null : _run,
            ),
            if (pair != null && pair.hasChange) ...<Widget>[
              const SizedBox(height: 10),
              Container(
                key: const ValueKey<String>('chat-optimize-result'),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  border: Border.all(color: Colors.grey.shade300),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(pair.optimized),
              ),
              const SizedBox(height: 10),
              PAButton(
                key: const ValueKey<String>('chat-optimize-apply'),
                label: '应用改写',
                variant: PAButtonVariant.neutral,
                expanded: true,
                onPressed: widget.onApply == null
                    ? null
                    : () => widget.onApply!(pair.optimized),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
