// ChatPage 组装测试（B3 第 5 波④）：五件在树 + 提交链路 mock 通 +
// 历史/更多/审批/slash 接线。
//
// 【只钉组装】会话状态机、投影、输入校验、动作执行各有各的测试；这里断言
// 「摆在一起能走通」：件在树上、信号到了对方、mock 通道收到了对的字节。
// session 用带 mock 流通道的真 TurnSession（startTurn/steer 真跑）；动作用
// fake transport（不断言真通道）。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_actions_transport.dart';
import 'package:pocketagent_ui/pages/chat_composer.dart';
import 'package:pocketagent_ui/pages/chat_composer_data.dart';
import 'package:pocketagent_ui/pages/chat_history_data.dart';
import 'package:pocketagent_ui/pages/chat_page.dart';
import 'package:pocketagent_ui/pages/chat_session.dart';
import 'package:pocketagent_ui/pages/chat_timeline.dart';
import 'package:pocketagent_ui/transport/chat_turn_stream.dart';

/// 小 fake 动作方法面（本文件只用到的几条有行为，其余走默认空实现）。
class _FakeActions extends ChatActionsTransport {
  _FakeActions() : super(channel: const MethodChannel('test/page-fake'));

  final List<String> calls = <String>[];
  SubmitApprovalRequest? lastApproval;
  int lastNumTurns = 0;
  ClearQueuedInputsResponse clearResponse = ClearQueuedInputsResponse();

  @override
  Future<void> submitApproval(SubmitApprovalRequest request) async {
    calls.add('submitApproval');
    lastApproval = request;
  }

  @override
  Future<void> cancelTurn(String threadId) async {
    calls.add('cancelTurn');
  }

  @override
  Future<ClearQueuedInputsResponse> clearQueuedInputs(String threadId) async {
    calls.add('clearQueuedInputs');
    return clearResponse;
  }

  @override
  Future<void> compactThread(String threadId) async {
    calls.add('compactThread');
  }

  @override
  Future<Thread> forkThread(String threadId, {String lastTurnId = ''}) async {
    calls.add('forkThread');
    return Thread(id: 't-new');
  }

  @override
  Future<Thread> rollbackThread(String threadId, int numTurns) async {
    calls.add('rollbackThread');
    lastNumTurns = numTurns;
    return Thread(id: threadId);
  }

  @override
  Future<Thread> setThreadGoal(String threadId, String goal) async {
    calls.add('setThreadGoal');
    return Thread(id: threadId);
  }

  @override
  Future<StartReviewResponse> startReview(
    String threadId, {
    String focus = '',
  }) async {
    calls.add('startReview');
    return StartReviewResponse(started: true);
  }

  @override
  Future<ExportThreadHtmlResponse> exportThreadHtml(String threadId) async {
    calls.add('exportThreadHtml');
    return ExportThreadHtmlResponse(path: '/x/t.html');
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
    return OptimizePromptResponse(optimized: '$text!');
  }
}

ThreadMessage _msg(String role, String text) => ThreadMessage(
      role: role,
      blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: text))],
      createdAtUnixMs: Int64(1000),
      messageId: 'm-$role-$text',
      turnId: 'turn-1',
    );

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methods = MethodChannel('test/chatpage_methods');
  const EventChannel events = EventChannel('test/chatpage_events');
  late TestDefaultBinaryMessenger messenger;

  int nextId = 0;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    nextId++;
  });

  TurnSession newSession() {
    final TurnSession s = TurnSession(
      threadId: 'page-$nextId',
      stream: TurnStreamBridge(
        methodChannel: methods,
        eventChannel: events,
      ),
    );
    addTearDown(s.dispose);
    return s;
  }

  List<MethodCall> mockStreamOk() {
    final List<MethodCall> seen = <MethodCall>[];
    messenger.setMockMethodCallHandler(methods, (MethodCall call) async {
      seen.add(call);
      return null;
    });
    return seen;
  }

  Future<void> pumpPage(
    WidgetTester tester, {
    TurnSession? session,
    _FakeActions? actions,
    List<String>? capabilities,
    String agentId = 'codex',
    List<String>? notices,
    VoidCallback? onOpenHistory,
    ChatHistorySource? historySource,
    String? threadId,
  }) async {
    final TurnSession s = session ?? newSession();
    await tester.pumpWidget(
      MaterialApp(
        home: ChatPage(
          threadId: threadId ?? s.threadId,
          agentId: agentId,
          capabilities: capabilities ?? const <String>[],
          session: session == null ? null : s,
          actionsTransport: actions,
          historySource: historySource,
          onOpenHistory: onOpenHistory,
          onNotify: notices?.add,
        ),
      ),
    );
    // 注意：session == null 时页面自建会话（构造契约覆盖用例），不代管 dispose
    // 之外的任何事；该用例不提交，不会触通道。
  }

  TurnServerMessage seq(TurnServerMessage f, int n) {
    f.sequence = Int64(n);
    return f;
  }

  group('组装：五件在树', () {
    testWidgets('时间线 + 输入区 + 历史/更多入口 + 运行条占位', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions();
      final List<String> notices = <String>[];
      mockStreamOk();
      await pumpPage(tester, session: s, actions: actions, notices: notices);

      expect(find.byType(ChatTimeline), findsOneWidget);
      expect(find.byType(ChatComposer), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chatpage-history')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chatpage-more')),
          findsOneWidget);
      // 空闲态运行条收起（无按钮可点）。
      expect(find.byKey(const ValueKey<String>('chat-running-cancel')),
          findsNothing);
    });

    testWidgets('构造契约：只有 threadId 也能摆出来（其余全可选）', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(home: ChatPage(threadId: 't-only')),
      );

      expect(find.byType(ChatTimeline), findsOneWidget);
      expect(find.byType(ChatComposer), findsOneWidget);
    });
  });

  group('提交链路：composer → session（mock 通道通）', () {
    testWidgets('首句走 startTurn：建流 + 首帧 input 字节 + 用户 tile 落卡', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final List<MethodCall> seen = mockStreamOk();
      await pumpPage(tester, session: s, actions: _FakeActions());

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '修登录页',
      );
      // 发送按钮的启用态由父组件重建后计算：enterText 后必须
      // 先 pump 一帧再 tap，否则点在禁用态按钮上（静默无响应）。
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      // 异步链（tap → onSubmit → startTurn → invokeMethod → mock）要 settle
      // 才落定：单 pump() 时调用还没记上（竞态，见接管记录）。
      await tester.pumpAndSettle();

      expect(
        seen.map((MethodCall c) => c.method),
        <String>['turnStreamStart', 'turnStreamSend'],
      );
      final TurnClientMessage first =
          TurnClientMessage.fromBuffer(seen[1].arguments as Uint8List);
      expect(first.whichMsg(), TurnClientMessage_Msg.input);
      expect(first.input.blocks.single.text.text, '修登录页');
      // 用户消息落卡 + 运行条出现（running 态）。
      expect(find.text('修登录页'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-running-cancel')),
          findsOneWidget);
    });

    testWidgets('运行中追问走 steer（supplement 帧，不是第二条 startTurn）', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final List<MethodCall> seen = mockStreamOk();
      await pumpPage(tester, session: s, actions: _FakeActions());

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '第一句',
      );
      // 发送按钮的启用态由父组件重建后计算：enterText 后必须
      // 先 pump 一帧再 tap，否则点在禁用态按钮上（静默无响应）。
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      await tester.pumpAndSettle();
      final int base = seen.length;

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '追问一句',
      );
      // 发送按钮的启用态由父组件重建后计算：enterText 后必须
      // 先 pump 一帧再 tap，否则点在禁用态按钮上（静默无响应）。
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      await tester.pumpAndSettle();

      expect(seen.length, base + 1, reason: '追问只发一帧，不重建流');
      final TurnClientMessage frame =
          TurnClientMessage.fromBuffer(seen.last.arguments as Uint8List);
      expect(frame.whichMsg(), TurnClientMessage_Msg.supplement);
    });

    testWidgets('失败卡重试出口：提交后 onRetry 非空', (WidgetTester tester) async {
      final TurnSession s = newSession();
      mockStreamOk();
      await pumpPage(tester, session: s, actions: _FakeActions());

      expect(
        tester.widget<ChatTimeline>(find.byType(ChatTimeline)).onRetry,
        isNull,
      );
      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '会失败的一句',
      );
      // 发送按钮的启用态由父组件重建后计算：enterText 后必须
      // 先 pump 一帧再 tap，否则点在禁用态按钮上（静默无响应）。
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      await tester.pumpAndSettle();
      expect(
        tester.widget<ChatTimeline>(find.byType(ChatTimeline)).onRetry,
        isNotNull,
      );
    });
  });

  group('历史入口', () {
    testWidgets('onOpenHistory 覆盖被调用（默认推页分支不跑）', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      bool opened = false;
      await pumpPage(
        tester,
        session: s,
        actions: _FakeActions(),
        onOpenHistory: () => opened = true,
      );

      await tester.tap(find.byKey(const ValueKey<String>('chatpage-history')));
      expect(opened, isTrue);
    });
  });

  group('更多面板：位过滤 + 执行', () {
    testWidgets('压缩：有位显示 → 点选 → compactThread + 回执', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions();
      final List<String> notices = <String>[];
      mockStreamOk();
      await pumpPage(
        tester,
        session: s,
        actions: actions,
        notices: notices,
        capabilities: const <String>['compact:codex'],
      );

      await tester.tap(find.byKey(const ValueKey<String>('chatpage-more')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey<String>('chat-action-compact')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-action-fork')),
          findsNothing);

      await tester.tap(find.byKey(const ValueKey<String>('chat-action-compact')));
      await tester.pumpAndSettle();
      expect(actions.calls, <String>['compactThread']);
      expect(notices.single, contains('压缩'));
    });

    testWidgets('回滚：真历史数轮 → 选项 → 确认 → rollbackThread(n)', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions();
      final List<String> notices = <String>[];
      mockStreamOk();
      final FakeChatHistorySource history = FakeChatHistorySource(
        response: GetThreadHistoryResponse(messages: <ThreadMessage>[
          _msg('user', 'u1'),
          _msg('assistant', 'a1'),
          _msg('user', 'u2'),
          _msg('assistant', 'a2'),
        ]),
      );
      await pumpPage(
        tester,
        session: s,
        actions: actions,
        notices: notices,
        capabilities: const <String>['rollback:codex'],
        historySource: history,
      );

      await tester.tap(find.byKey(const ValueKey<String>('chatpage-more')));
      await tester.pumpAndSettle();
      await tester
          .tap(find.byKey(const ValueKey<String>('chat-action-rollback')));
      await tester.pumpAndSettle();

      // 两轮 → 选项 1/2（1 与全部同值收敛成单项 2 的上一档，见纯函数测试）。
      expect(find.byKey(const ValueKey<String>('chat-rollback-opt-1')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-rollback-opt-2')),
          findsOneWidget);
      expect(history.requested, <String>[s.threadId]);

      await tester.tap(find.byKey(const ValueKey<String>('chat-rollback-opt-2')));
      await tester.pump();
      await tester
          .tap(find.byKey(const ValueKey<String>('chat-rollback-confirm')));
      await tester.pumpAndSettle();
      expect(actions.calls, <String>['rollbackThread']);
      expect(actions.lastNumTurns, 2);
    });

    testWidgets('取消流：运行条终止键 → cancelTurn', (WidgetTester tester) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions();
      final List<MethodCall> seen = mockStreamOk();
      await pumpPage(tester, session: s, actions: actions);

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '跑一个',
      );
      // 发送按钮的启用态由父组件重建后计算：enterText 后必须
      // 先 pump 一帧再 tap，否则点在禁用态按钮上（静默无响应）。
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey<String>('chat-running-cancel')));
      await tester.pumpAndSettle();
      // 运行条走一元 CancelTurn 兜底（流内 cancel 帧走 session.cancel，见注释）。
      expect(actions.calls, contains('cancelTurn'));
      expect(seen.map((MethodCall c) => c.method), isNot(contains('cancelTurn')),
          reason: '一元 CancelTurn 不经过流通道（方法名空间隔离）');
    });

    testWidgets('清排队：daemon 真清除 + 本地收敛（queued → running）', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions()
        ..clearResponse = ClearQueuedInputsResponse(steered: 0, followUp: 1);
      mockStreamOk();
      await pumpPage(tester, session: s, actions: actions);

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '跑一个',
      );
      // 发送按钮的启用态由父组件重建后计算：enterText 后必须
      // 先 pump 一帧再 tap，否则点在禁用态按钮上（静默无响应）。
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      await tester.pumpAndSettle();
      // 喂一条 Queued 回执帧：本地进 queued。
      s.applyFrame(seq(TurnServerMessage(queued: Queued(position: 1)), 1));
      await tester.pump();
      expect(s.phase, TurnPhase.queued);

      await tester.tap(find.byKey(const ValueKey<String>('chat-running-clear')));
      await tester.pumpAndSettle();
      expect(actions.calls, contains('clearQueuedInputs'));
      expect(s.queuedCount, 0);
      expect(s.phase, TurnPhase.running);
    });
  });

  group('审批接线：timeline 占位 → 审批表 → SubmitApproval', () {
    testWidgets('允许 → transport 收到 approved=true + 回执', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions();
      final List<String> notices = <String>[];
      mockStreamOk();
      await pumpPage(tester, session: s, actions: actions, notices: notices);

      tester.widget<ChatTimeline>(find.byType(ChatTimeline)).onApproval!(
        const ApprovalAction(requestId: 'r1', approved: true),
      );
      await tester.pumpAndSettle();
      expect(find.text('需要审批'), findsOneWidget);

      await tester.tap(find.text('允许'));
      await tester.pumpAndSettle();
      expect(actions.calls, <String>['submitApproval']);
      expect(actions.lastApproval!.decision.requestId, 'r1');
      expect(actions.lastApproval!.decision.approved, isTrue);
      expect(notices.single, '已允许该操作');
    });
  });

  group('slash 接线：伪命令动作留页面', () {
    testWidgets('/目标 → 目标表出现', (WidgetTester tester) async {
      final TurnSession s = newSession();
      mockStreamOk();
      await pumpPage(tester, session: s, actions: _FakeActions());

      tester.widget<ChatComposer>(find.byType(ChatComposer)).onSlashPick!(
        const SlashPick(
          name: '目标',
          action: SlashPickAction.goal,
          agentId: 'codex',
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey<String>('chat-goal-field')),
          findsOneWidget);
    });

    testWidgets('/压缩 → compactThread 直调（无位时页面也拦不住就不调——'
        '位过滤在面板，slash 直调走 runner，daemon 侧回错如实报）', (
      WidgetTester tester,
    ) async {
      final TurnSession s = newSession();
      final _FakeActions actions = _FakeActions();
      final List<String> notices = <String>[];
      mockStreamOk();
      await pumpPage(
        tester,
        session: s,
        actions: actions,
        notices: notices,
      );

      tester.widget<ChatComposer>(find.byType(ChatComposer)).onSlashPick!(
        const SlashPick(
          name: '压缩',
          action: SlashPickAction.compact,
          agentId: 'codex',
        ),
      );
      await tester.pumpAndSettle();
      expect(actions.calls, <String>['compactThread']);
    });
  });
}
