// Chat 周边动作测试（B3 第 5 波④）：九项位过滤矩阵 + 纯函数 + 执行器 +
// 面板/操作条/小表装配 + 审批流 + 取消流。
//
// 【分层】位过滤与文案是纯函数（直测 `chat_actions_data.dart`）；执行器用
// fake transport 钉「调了哪个 RPC、报了哪句」；widget 只钉「摆出来 + 点了
// 有出口」，不复述纯函数结论。

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_actions.dart';
import 'package:pocketagent_ui/pages/chat_actions_data.dart';
import 'package:pocketagent_ui/pages/chat_actions_transport.dart';
import 'package:pocketagent_ui/pages/chat_timeline.dart' show ApprovalAction;
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 可控 fake 方法面：记录调用、按需回 canned 响应或抛宿主错误。
class FakeChatActionsTransport extends ChatActionsTransport {
  FakeChatActionsTransport() : super(channel: const MethodChannel('test/fake'));

  final List<String> calls = <String>[];
  SubmitApprovalRequest? lastApproval;
  String lastThreadId = '';
  int lastNumTurns = 0;
  String lastGoal = '';
  String lastFocus = '';
  String lastOptimizeText = '';
  PaBridgeException? failure;
  ClearQueuedInputsResponse clearResponse = ClearQueuedInputsResponse();
  StartReviewResponse reviewResponse = StartReviewResponse(started: true);
  ExportThreadHtmlResponse exportResponse = ExportThreadHtmlResponse();
  OptimizePromptResponse optimizeResponse = OptimizePromptResponse();

  void _maybeThrow() {
    final PaBridgeException? e = failure;
    if (e != null) throw e;
  }

  @override
  Future<void> submitApproval(SubmitApprovalRequest request) async {
    calls.add('submitApproval');
    lastApproval = request;
    lastThreadId = request.threadId;
    _maybeThrow();
  }

  @override
  Future<void> cancelTurn(String threadId) async {
    calls.add('cancelTurn');
    lastThreadId = threadId;
    _maybeThrow();
  }

  @override
  Future<ClearQueuedInputsResponse> clearQueuedInputs(String threadId) async {
    calls.add('clearQueuedInputs');
    lastThreadId = threadId;
    _maybeThrow();
    return clearResponse;
  }

  @override
  Future<void> compactThread(String threadId) async {
    calls.add('compactThread');
    lastThreadId = threadId;
    _maybeThrow();
  }

  @override
  Future<Thread> forkThread(String threadId, {String lastTurnId = ''}) async {
    calls.add('forkThread');
    lastThreadId = threadId;
    _maybeThrow();
    return Thread(id: 't-new');
  }

  @override
  Future<Thread> rollbackThread(String threadId, int numTurns) async {
    calls.add('rollbackThread');
    lastThreadId = threadId;
    lastNumTurns = numTurns;
    _maybeThrow();
    return Thread(id: threadId);
  }

  @override
  Future<Thread> setThreadGoal(String threadId, String goal) async {
    calls.add('setThreadGoal');
    lastThreadId = threadId;
    lastGoal = goal;
    _maybeThrow();
    return Thread(id: threadId);
  }

  @override
  Future<StartReviewResponse> startReview(
    String threadId, {
    String focus = '',
  }) async {
    calls.add('startReview');
    lastThreadId = threadId;
    lastFocus = focus;
    _maybeThrow();
    return reviewResponse;
  }

  @override
  Future<ExportThreadHtmlResponse> exportThreadHtml(String threadId) async {
    calls.add('exportThreadHtml');
    lastThreadId = threadId;
    _maybeThrow();
    return exportResponse;
  }

  @override
  Future<OptimizePromptResponse> optimizePrompt({
    required String text,
    String style = '',
    String agentId = '',
    String modelId = '',
    String effort = '',
  }) async {
    calls.add('optimizePrompt');
    lastOptimizeText = text;
    _maybeThrow();
    return optimizeResponse;
  }
}

ChatActionRunner runnerOf(
  FakeChatActionsTransport fake,
  List<String> notices,
) =>
    ChatActionRunner(
      transport: fake,
      threadId: 't1',
      notify: notices.add,
    );

void main() {
  group('九项位过滤矩阵（有位显示 / 无位隐藏逐项）', () {
    test('无位 + 空闲：只剩目标（恒显示）', () {
      expect(
        visibleChatActions(
          agentId: 'codex',
          capabilities: const <String>[],
          isRunning: false,
          queuedCount: 0,
        ),
        <ChatActionId>[ChatActionId.goal],
      );
    });

    test('六位全亮 + 空闲：七项按顺序摆', () {
      expect(
        visibleChatActions(
          agentId: 'codex',
          capabilities: const <String>[
            'compact:codex',
            'fork:codex',
            'rollback:codex',
            'review:codex',
            'export_html:codex',
            'text_generation:codex',
          ],
          isRunning: false,
          queuedCount: 0,
        ),
        <ChatActionId>[
          ChatActionId.compact,
          ChatActionId.fork,
          ChatActionId.rollback,
          ChatActionId.review,
          ChatActionId.goal,
          ChatActionId.export,
          ChatActionId.optimize,
        ],
      );
    });

    test('运行态：取消 + 清排队置顶（无位时也摆）', () {
      expect(
        visibleChatActions(
          agentId: 'codex',
          capabilities: const <String>[],
          isRunning: true,
          queuedCount: 2,
        ),
        <ChatActionId>[
          ChatActionId.cancel,
          ChatActionId.clearQueue,
          ChatActionId.goal,
        ],
      );
    });

    test('别家的位不开我家的门（codex 会话 + pi 位 = 无位）', () {
      expect(
        visibleChatActions(
          agentId: 'codex',
          capabilities: const <String>[
            'compact:pi',
            'fork:pi',
            'rollback:pi',
            'review:pi',
            'export_html:pi',
            'text_generation:pi',
          ],
          isRunning: false,
          queuedCount: 0,
        ),
        <ChatActionId>[ChatActionId.goal],
      );
    });

    test('agentId 空 = 未知：按无位处理（目标仍显示）', () {
      expect(
        visibleChatActions(
          agentId: '',
          capabilities: const <String>['compact:codex'],
          isRunning: false,
          queuedCount: 0,
        ),
        <ChatActionId>[ChatActionId.goal],
      );
      expect(
        visibleChatActions(
          agentId: null,
          capabilities: const <String>['compact:codex'],
          isRunning: true,
          queuedCount: 0,
        ),
        <ChatActionId>[
          ChatActionId.cancel,
          ChatActionId.clearQueue,
          ChatActionId.goal,
        ],
      );
    });

    test('大小写/空白不敏感（与 agentCapableOf 同口径）', () {
      expect(
        visibleChatActions(
          agentId: ' Codex ',
          capabilities: const <String>[' Compact:codex '],
          isRunning: false,
          queuedCount: 0,
        ),
        <ChatActionId>[ChatActionId.compact, ChatActionId.goal],
      );
    });

    test('逐项位名钉死（改名即改契约，六个分别断言）', () {
      List<ChatActionId> only(String bit) => visibleChatActions(
            agentId: 'codex',
            capabilities: <String>['$bit:codex'],
            isRunning: false,
            queuedCount: 0,
          );
      expect(only('compact'), contains(ChatActionId.compact));
      expect(only('fork'), contains(ChatActionId.fork));
      expect(only('rollback'), contains(ChatActionId.rollback));
      expect(only('review'), contains(ChatActionId.review));
      expect(only('export_html'), contains(ChatActionId.export));
      expect(only('text_generation'), contains(ChatActionId.optimize));
    });

    test('动作中文名全覆盖（九项无空名）', () {
      for (final ChatActionId id in ChatActionId.values) {
        expect(chatActionLabel(id), isNotEmpty);
      }
      expect(ChatActionId.values, hasLength(9));
    });
  });

  group('审批纯函数：封帧 + 结果文案', () {
    test('decision 四件套原样封帧', () {
      final ApprovalDecision d = approvalDecisionFrame(
        requestId: 'r1',
        approved: true,
        note: '备注',
        option: 'opt-1',
      );
      expect(d.requestId, 'r1');
      expect(d.approved, isTrue);
      expect(d.note, '备注');
      expect(d.option, 'opt-1');

      final SubmitApprovalRequest req =
          approvalSubmitRequest(threadId: 't1', decision: d);
      expect(req.threadId, 't1');
      expect(req.decision.requestId, 'r1');
    });

    test('结果文案：允许/拒绝两档', () {
      expect(approvalResultNotice(approved: true), '已允许该操作');
      expect(approvalResultNotice(approved: false), '已拒绝该操作');
    });
  });

  group('导出纯函数：状态机 + 门控 + 失败文案', () {
    test('相位标签：空闲无标签（不挂无意义徽标）', () {
      expect(chatExportPhaseLabel(ChatExportPhase.idle), isEmpty);
      expect(chatExportPhaseLabel(ChatExportPhase.exporting), '正在导出…');
      expect(chatExportPhaseLabel(ChatExportPhase.done), '导出完成');
      expect(chatExportPhaseLabel(ChatExportPhase.failed), '导出失败');
    });

    test('门控三态：空 agent ≠ 不支持', () {
      expect(htmlExportGateOf('', const <String>[]), HtmlExportGate.unknown);
      expect(htmlExportGateOf(null, const <String>[]), HtmlExportGate.unknown);
      expect(
        htmlExportGateOf('pi', const <String>['export_html:pi']),
        HtmlExportGate.supported,
      );
      expect(
        htmlExportGateOf('codex', const <String>['export_html:pi']),
        HtmlExportGate.unsupportedAgent,
      );
    });

    test('失败文案：未就绪定制，其余透传宿主 message', () {
      expect(
        exportFailureMessage(code: 'HOST_UNAVAILABLE', message: 'x'),
        contains('B4 波次'),
      );
      expect(
        exportFailureMessage(code: 'INTERNAL', message: 'daemon 炸了'),
        'daemon 炸了',
      );
      expect(
        exportFailureMessage(code: 'INTERNAL', message: ''),
        contains('INTERNAL'),
      );
      expect(exportFailureMessage(code: '', message: ''), '导出 HTML 失败');
    });
  });

  group('优化纯函数：文本对 + 输入校验', () {
    test('hasChange：同文/空改写都不算变化（空回填等于吞原文）', () {
      expect(
        const OptimizePair(original: 'a', optimized: 'b').hasChange,
        isTrue,
      );
      expect(
        const OptimizePair(original: '  a ', optimized: 'a').hasChange,
        isFalse,
      );
      expect(
        const OptimizePair(original: 'a', optimized: '  ').hasChange,
        isFalse,
      );
    });

    test('输入校验：空拦、超长拦', () {
      expect(validateOptimizeInput('  '), isNotNull);
      expect(validateOptimizeInput('x' * 20001), isNotNull);
      expect(validateOptimizeInput('正常输入'), isNull);
    });
  });

  group('目标/审查/fork/rollback 纯函数', () {
    test('目标空拦；焦点只做 trim（空透传给 daemon 默认口径）', () {
      expect(validateThreadGoal('  '), isNotNull);
      expect(validateThreadGoal('全量改 Kotlin'), isNull);
      expect(normalizeReviewFocus('  只看并发  '), '只看并发');
      expect(normalizeReviewFocus('   '), isEmpty);
    });

    test('分叉起点：空合法（从最新处），空白拦', () {
      expect(validateForkLastTurnId(''), isNull);
      expect(validateForkLastTurnId('turn-9'), isNull);
      expect(validateForkLastTurnId('   '), isNotNull);
    });

    test('回滚选项：0 空；1 只有全部；5 是 1/2/3+全部', () {
      expect(rollbackTurnOptions(0), isEmpty);
      expect(rollbackTurnOptions(-2), isEmpty);
      expect(rollbackTurnOptions(1), <int>[1]);
      expect(rollbackTurnOptions(2), <int>[1, 2]);
      expect(rollbackTurnOptions(5), <int>[1, 2, 3, 5]);
    });

    test('回滚选项标签：「全部」说清是多少', () {
      expect(rollbackOptionLabel(5, 5), contains('5'));
      expect(rollbackOptionLabel(2, 5), '2 回合');
    });

    test('回滚校验：总数未知拦；越界拦', () {
      expect(validateRollbackTurns(1, 0), isNotNull);
      expect(validateRollbackTurns(0, 5), isNotNull);
      expect(validateRollbackTurns(6, 5), contains('5'));
      expect(validateRollbackTurns(3, 5), isNull);
    });

    test('后果说明非空（确认前必须让用户看到的一句）', () {
      expect(rollbackConsequence(2), contains('2'));
      expect(rollbackConsequence(2), contains('Git'));
      expect(forkConsequence(), isNotEmpty);
    });

    test('清排队文案：空队如实说，两笔计数如实说，负数钳零', () {
      expect(queueClearedNotice(0, 0), contains('空'));
      expect(queueClearedNotice(0, 2), contains('2'));
      expect(queueClearedNotice(1, 0), contains('1'));
      expect(queueClearedNotice(1, 2), contains('3'));
      expect(queueClearedNotice(-1, -2), contains('空'));
    });
  });

  group('执行器：调对 RPC + 报对人话', () {
    test('审批流：允许 → SubmitApproval + 已允许', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];

      final String notice = await runnerOf(fake, notices).submitApproval(
        const ApprovalAction(requestId: 'r1', approved: true),
      );

      expect(fake.calls, <String>['submitApproval']);
      expect(fake.lastThreadId, 't1');
      expect(fake.lastApproval!.decision.requestId, 'r1');
      expect(fake.lastApproval!.decision.approved, isTrue);
      expect(notice, '已允许该操作');
      expect(notices, <String>['已允许该操作']);
    });

    test('审批流：拒绝带备注 → note 透传 + 已拒绝', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];

      await runnerOf(fake, notices).submitApproval(
        const ApprovalAction(requestId: 'r2', approved: false),
        note: '太危险',
      );

      expect(fake.lastApproval!.decision.approved, isFalse);
      expect(fake.lastApproval!.decision.note, '太危险');
      expect(notices.single, '已拒绝该操作');
    });

    test('审批流：宿主未就绪 → 如实报未就绪（不伪造成功）', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..failure = const PaBridgeException('HOST_UNAVAILABLE', 'no host');
      final List<String> notices = <String>[];

      final String notice = await runnerOf(fake, notices).submitApproval(
        const ApprovalAction(requestId: 'r1', approved: true),
      );

      expect(notice, contains('未就绪'));
      expect(notices.single, contains('B4 波次'));
    });

    test('审批流：宿主 message 透传（不改写）', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..failure = const PaBridgeException('INTERNAL', 'daemon 炸了');
      final List<String> notices = <String>[];

      await runnerOf(fake, notices).submitApproval(
        const ApprovalAction(requestId: 'r1', approved: true),
      );

      expect(notices.single, contains('daemon 炸了'));
    });

    test('取消流：cancelTurn + 终止回执', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];

      final String notice = await runnerOf(fake, notices).cancelTurn();

      expect(fake.calls, <String>['cancelTurn']);
      expect(fake.lastThreadId, 't1');
      expect(notice, contains('终止'));
      expect(notices.single, notice);
    });

    test('取消流：失败也报人话（调用方无需再分支）', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..failure = const PaBridgeException('NOT_CONNECTED', '还没连上');
      final List<String> notices = <String>[];

      final String notice = await runnerOf(fake, notices).cancelTurn();

      expect(notice, contains('还没连上'));
    });

    test('清排队：两笔计数进文案', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..clearResponse = ClearQueuedInputsResponse(steered: 1, followUp: 2);
      final List<String> notices = <String>[];

      final String notice = await runnerOf(fake, notices).clearQueue();

      expect(fake.calls, <String>['clearQueuedInputs']);
      expect(notice, contains('3'));
    });

    test('压缩/目标/审查/导出/fork/回滚：各调各的 RPC', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..reviewResponse =
            StartReviewResponse(reviewId: 'rev-1', started: true)
        ..exportResponse = ExportThreadHtmlResponse(path: '/x/t1.html');
      final List<String> notices = <String>[];
      final ChatActionRunner r = runnerOf(fake, notices);

      await r.compact();
      await r.setGoal('全量改 Kotlin');
      await r.startReview(focus: '只看并发');
      await r.exportHtml();
      await r.fork();
      await r.rollback(2, turnTotal: 5);

      expect(
        fake.calls,
        <String>[
          'compactThread',
          'setThreadGoal',
          'startReview',
          'exportThreadHtml',
          'forkThread',
          'rollbackThread',
        ],
      );
      expect(fake.lastGoal, '全量改 Kotlin');
      expect(fake.lastFocus, '只看并发');
      expect(fake.lastNumTurns, 2);
      expect(notices[1], '已设置会话目标');
      expect(notices[2], contains('事件流回本会话'));
      expect(notices[3], contains('/x/t1.html'));
    });

    test('审查未确认开始：如实说受理（不报"已发起成功"）', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..reviewResponse = StartReviewResponse(started: false);
      final List<String> notices = <String>[];

      await runnerOf(fake, notices).startReview();

      expect(notices.single, contains('受理'));
    });

    test('本地拦的不进通道：空目标/越界回滚/空白分叉', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];
      final ChatActionRunner r = runnerOf(fake, notices);

      await r.setGoal('   ');
      await r.rollback(0, turnTotal: 5);
      await r.rollback(9, turnTotal: 5);
      await r.fork(lastTurnId: '   ');
      await r.optimize('   ');

      expect(fake.calls, isEmpty, reason: '本地拦的参数一个都不许过桥');
      expect(notices, hasLength(5));
    });

    test('优化：原文原样进通道，返回前后对', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..optimizeResponse = OptimizePromptResponse(optimized: '改好了');
      final List<String> notices = <String>[];

      final OptimizePair? pair =
          await runnerOf(fake, notices).optimize('修一下');

      expect(fake.lastOptimizeText, '修一下');
      expect(pair, isNotNull);
      expect(pair!.original, '修一下');
      expect(pair.optimized, '改好了');
      expect(pair.hasChange, isTrue);
      expect(notices, isEmpty, reason: '成功只给对照表，不刷 Toast');
    });

    test('优化失败：null + 人话（已通知）', () async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..failure = const PaBridgeException('INTERNAL', '改写炸了');
      final List<String> notices = <String>[];

      final OptimizePair? pair =
          await runnerOf(fake, notices).optimize('修一下');

      expect(pair, isNull);
      expect(notices.single, contains('改写炸了'));
    });
  });

  group('面板 widget：摆出来 + 点了有出口', () {
    Future<void> pumpPanel(
      WidgetTester tester, {
      required List<ChatActionId> items,
      required ValueChanged<ChatActionId> onAction,
    }) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatActionsPanel(items: items, onAction: onAction),
          ),
        ),
      );
    }

    testWidgets('九项行逐项可点（id 原样交出）', (WidgetTester tester) async {
      final List<ChatActionId> picked = <ChatActionId>[];
      await pumpPanel(
        tester,
        items: ChatActionId.values.toList(),
        onAction: picked.add,
      );

      for (final ChatActionId id in ChatActionId.values) {
        await tester.tap(find.byKey(ValueKey<String>('chat-action-${id.name}')));
        expect(picked.last, id);
      }
      expect(picked, hasLength(9));
    });

    testWidgets('空表 = 诚实空态（不断言不存在的入口）', (WidgetTester tester) async {
      await pumpPanel(
        tester,
        items: const <ChatActionId>[],
        onAction: (_) {},
      );

      expect(find.text('暂无可用动作'), findsOneWidget);
    });

    testWidgets('showChatActionsPanel：点选返回值', (WidgetTester tester) async {
      ChatActionId? result;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (BuildContext context) => TextButton(
                onPressed: () async {
                  result = await showChatActionsPanel(
                    context,
                    items: const <ChatActionId>[
                      ChatActionId.compact,
                      ChatActionId.goal,
                    ],
                  );
                },
                child: const Text('开面板'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('开面板'));
      await tester.pumpAndSettle();
      expect(find.text('压缩会话'), findsOneWidget);
      expect(find.text('设置会话目标'), findsOneWidget);
      expect(find.text('分叉会话'), findsNothing);

      await tester.tap(find.byKey(const ValueKey<String>('chat-action-goal')));
      await tester.pumpAndSettle();
      expect(result, ChatActionId.goal);
    });
  });

  group('运行中操作条：running 才显示', () {
    Future<void> pumpBar(
      WidgetTester tester, {
      required bool isRunning,
      int queuedCount = 0,
      bool busy = false,
      VoidCallback? onCancel,
      VoidCallback? onClearQueue,
    }) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatRunningBar(
              isRunning: isRunning,
              queuedCount: queuedCount,
              busy: busy,
              onCancel: onCancel,
              onClearQueue: onClearQueue,
            ),
          ),
        ),
      );
    }

    testWidgets('空闲 = 空盒（无按钮可点）', (WidgetTester tester) async {
      await pumpBar(tester, isRunning: false);

      expect(find.byKey(const ValueKey<String>('chat-running-cancel')),
          findsNothing);
      expect(find.byKey(const ValueKey<String>('chat-running-clear')),
          findsNothing);
    });

    testWidgets('运行中 = 终止 + 清排队（排队数挂出来）', (WidgetTester tester) async {
      bool cancelled = false;
      bool cleared = false;
      await pumpBar(
        tester,
        isRunning: true,
        queuedCount: 2,
        onCancel: () => cancelled = true,
        onClearQueue: () => cleared = true,
      );

      expect(find.textContaining('清空排队（2）'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey<String>('chat-running-cancel')));
      await tester.tap(find.byKey(const ValueKey<String>('chat-running-clear')));
      expect(cancelled, isTrue);
      expect(cleared, isTrue);
    });

    testWidgets('busy = 双键禁用（防重复提交）', (WidgetTester tester) async {
      bool cancelled = false;
      await pumpBar(
        tester,
        isRunning: true,
        busy: true,
        onCancel: () => cancelled = true,
      );

      await tester.tap(find.byKey(const ValueKey<String>('chat-running-cancel')));
      expect(cancelled, isFalse);
    });
  });

  group('小表 widget：目标/回滚/审查/优化', () {
    testWidgets('目标表：空目标本地拦，不调 onSave', (WidgetTester tester) async {
      String? saved;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatGoalSheet(onSave: (String v) => saved = v),
          ),
        ),
      );

      await tester.tap(find.byKey(const ValueKey<String>('chat-goal-save')));
      await tester.pump();
      expect(saved, isNull);
      expect(find.textContaining('不能为空'), findsOneWidget);

      await tester.enterText(
        find.byKey(const ValueKey<String>('chat-goal-field')),
        '全量改 Kotlin',
      );
      await tester.tap(find.byKey(const ValueKey<String>('chat-goal-save')));
      expect(saved, '全量改 Kotlin');
    });

    testWidgets('目标表：onSave 空 = 保存键禁用', (WidgetTester tester) async {
      await tester.pumpWidget(
        const MaterialApp(home: Scaffold(body: ChatGoalSheet())),
      );

      await tester.tap(find.byKey(const ValueKey<String>('chat-goal-save')));
      // 禁用键 tap 无动作、无异常即过。
    });

    testWidgets('回滚表：选项 + 后果 + 确认值', (WidgetTester tester) async {
      int? picked;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatRollbackSheet(
              turnTotal: 5,
              onConfirm: (int v) => picked = v,
            ),
          ),
        ),
      );

      expect(find.byKey(const ValueKey<String>('chat-rollback-opt-1')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-rollback-opt-5')),
          findsOneWidget);
      expect(find.textContaining('Git'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey<String>('chat-rollback-opt-3')));
      await tester.pump();
      await tester.tap(
          find.byKey(const ValueKey<String>('chat-rollback-confirm')));
      expect(picked, 3);
    });

    testWidgets('回滚表：无回合 = 只展示说明，无确认键', (WidgetTester tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(body: ChatRollbackSheet(turnTotal: 0)),
        ),
      );

      expect(find.textContaining('没有可回滚'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-rollback-confirm')),
          findsNothing);
    });

    testWidgets('审查表：焦点可空，直接发起交空串', (WidgetTester tester) async {
      String? focus;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatReviewSheet(onStart: (String v) => focus = v),
          ),
        ),
      );

      await tester.tap(find.byKey(const ValueKey<String>('chat-review-start')));
      expect(focus, isEmpty);

      await tester.enterText(
        find.byKey(const ValueKey<String>('chat-review-field')),
        '  只看并发  ',
      );
      await tester.tap(find.byKey(const ValueKey<String>('chat-review-start')));
      expect(focus, '只看并发');
    });

    testWidgets('优化表：输入 → 对照 → 应用', (WidgetTester tester) async {
      String? applied;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatOptimizeSheet(
              onOptimize: (String text) async =>
                  OptimizePair(original: text, optimized: '$text（改写）'),
              onApply: (String v) => applied = v,
            ),
          ),
        ),
      );

      await tester.enterText(
        find.byKey(const ValueKey<String>('chat-optimize-input')),
        '修一下',
      );
      await tester.tap(find.byKey(const ValueKey<String>('chat-optimize-run')));
      await tester.pump();
      expect(find.byKey(const ValueKey<String>('chat-optimize-result')),
          findsOneWidget);

      await tester.tap(
          find.byKey(const ValueKey<String>('chat-optimize-apply')));
      expect(applied, '修一下（改写）');
    });

    testWidgets('优化表：无需改写 = 如实说，不摆应用键', (WidgetTester tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatOptimizeSheet(
              onOptimize: (String text) async =>
                  OptimizePair(original: text, optimized: text),
            ),
          ),
        ),
      );

      await tester.enterText(
        find.byKey(const ValueKey<String>('chat-optimize-input')),
        '修一下',
      );
      await tester.tap(find.byKey(const ValueKey<String>('chat-optimize-run')));
      await tester.pump();
      expect(find.byKey(const ValueKey<String>('chat-optimize-result')),
          findsNothing);
      expect(find.textContaining('原样可用'), findsOneWidget);
    });
  });

  group('审批真交互：占位点按 → 审批表 → SubmitApproval → Toast 串', () {
    Future<void> pumpApprover(
      WidgetTester tester, {
      required FakeChatActionsTransport fake,
      required List<String> notices,
      required ApprovalAction action,
    }) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (BuildContext context) => TextButton(
                onPressed: () => handleApprovalAction(
                  context,
                  transport: fake,
                  threadId: 't1',
                  action: action,
                  reason: '删库操作',
                  details: 'rm -rf /tmp/x',
                  notify: notices.add,
                ),
                child: const Text('模拟点按审批卡'),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('模拟点按审批卡'));
      await tester.pumpAndSettle();
    }

    testWidgets('允许 → SubmitApproval(approved=true) + 已允许', (
      WidgetTester tester,
    ) async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];
      await pumpApprover(
        tester,
        fake: fake,
        notices: notices,
        action: const ApprovalAction(requestId: 'r1', approved: true),
      );

      // 审批表真交互：理由/详情原样展示，请求 id 可查。
      expect(find.text('需要审批'), findsOneWidget);
      expect(find.text('删库操作'), findsOneWidget);
      expect(find.text('rm -rf /tmp/x'), findsOneWidget);

      await tester.tap(find.text('允许'));
      await tester.pumpAndSettle();

      expect(fake.calls, <String>['submitApproval']);
      expect(fake.lastApproval!.decision.approved, isTrue);
      expect(notices.single, '已允许该操作');
    });

    testWidgets('拒绝 → SubmitApproval(approved=false) + 已拒绝', (
      WidgetTester tester,
    ) async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];
      await pumpApprover(
        tester,
        fake: fake,
        notices: notices,
        action: const ApprovalAction(requestId: 'r9', approved: false),
      );

      // 点按出口的 approved 只决定回调载荷，最终以表上拍板为准：这里点拒绝。
      await tester.tap(find.text('拒绝'));
      await tester.pumpAndSettle();

      expect(fake.lastApproval!.decision.requestId, 'r9');
      expect(fake.lastApproval!.decision.approved, isFalse);
      expect(notices.single, '已拒绝该操作');
    });

    testWidgets('取消表 = 无 RPC、无 Toast（没拍板就没有动作）', (
      WidgetTester tester,
    ) async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport();
      final List<String> notices = <String>[];
      await pumpApprover(
        tester,
        fake: fake,
        notices: notices,
        action: const ApprovalAction(requestId: 'r1', approved: true),
      );

      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(fake.calls, isEmpty);
      expect(notices, isEmpty);
    });

    testWidgets('提交失败 → 人话通知（拍板不白拍）', (WidgetTester tester) async {
      final FakeChatActionsTransport fake = FakeChatActionsTransport()
        ..failure = const PaBridgeException('INTERNAL', 'daemon 炸了');
      final List<String> notices = <String>[];
      await pumpApprover(
        tester,
        fake: fake,
        notices: notices,
        action: const ApprovalAction(requestId: 'r1', approved: true),
      );

      await tester.tap(find.text('允许'));
      await tester.pumpAndSettle();

      expect(notices.single, contains('daemon 炸了'));
    });
  });
}
