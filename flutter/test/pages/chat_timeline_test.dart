// Chat 时间线骨架测试（B3 第 5 波①）：
// ① TurnDelivery.kt 移植纯函数（徽标文案逐字钉死）；
// ② 帧累积器（去重 / 增量拼接 / 五型投影 / 回执徽标）；
// ③ widget 五型渲染 + 徽标 + 空态 + 审批回调出口。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_timeline.dart';

void main() {
  TurnServerMessage seqFrame(TurnServerMessage f, int seq) {
    f.sequence = Int64(seq);
    return f;
  }

  TurnServerMessage textFrame(String d, int seq) =>
      seqFrame(TurnServerMessage(textDelta: TextDelta(delta: d)), seq);

  ToolCallUpdate toolStarted(String id, String name) => ToolCallUpdate(
        phase: ToolCallUpdate_Phase.STARTED,
        call: ToolCallBlock(id: id, name: name, jsonArgs: '{}'),
      );

  ToolCallUpdate toolFinished(String id, String output, {bool ok = true}) =>
      ToolCallUpdate(
        phase: ToolCallUpdate_Phase.FINISHED,
        call: ToolCallBlock(id: id),
        result: ToolResultBlock(ok: ok, output: output),
      );

  Future<void> pumpTimeline(
    WidgetTester tester, {
    required List<ChatTile> tiles,
    bool isRunning = false,
    String? errorText,
    ValueChanged<ApprovalAction>? onApproval,
    VoidCallback? onRetry,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: ChatTimeline(
            tiles: tiles,
            isRunning: isRunning,
            errorText: errorText,
            onApproval: onApproval,
            onRetry: onRetry,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('deliveryLabel / deliveryAckNotice：TurnDelivery.kt 移植', () {
    test('徽标文案逐字（pending/steered/queued/overflow/未知）', () {
      expect(deliveryLabel(kDeliveryPending, 0), '发送中…');
      expect(deliveryLabel(kDeliverySteered, 0), '已插入运行中任务');
      expect(deliveryLabel(kDeliveryQueued, 1), '排队中 · 下一条');
      expect(deliveryLabel(kDeliveryQueued, 3), '排队中 · 第3位');
      expect(deliveryLabel(kDeliveryOverflow, 0), '队列已满，未送达');
      expect(deliveryLabel('', 0), '', reason: '普通用户消息不挂标签');
      expect(deliveryLabel('brand-new-state', 0), '',
          reason: '未知态不造标签（透传红线）');
    });

    test('回执 Toast 只跟回执帧（Steered/Queued 报，其余 null）', () {
      expect(deliveryAckNotice(kDeliverySteered), '已插入运行中任务');
      expect(deliveryAckNotice(kDeliveryQueued), '已排队，当前轮结束后执行');
      expect(deliveryAckNotice(kDeliveryPending), isNull);
      expect(deliveryAckNotice(kDeliveryOverflow), isNull,
          reason: '拒收由徽标 + 重试入口呈现，Toast 不重复报');
      expect(deliveryAckNotice(''), isNull);
    });

    test('queuedAckAccepted：position<=0 = 拒收', () {
      expect(queuedAckAccepted(1), isTrue);
      expect(queuedAckAccepted(0), isFalse);
      expect(queuedAckAccepted(-1), isFalse);
    });
  });

  group('ChatTimelineAccumulator：增量拼接 + 去重', () {
    test('textDelta 拼接，终局落一张文本卡 + 完成标记', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(textFrame('你', 1));
      acc.applyFrame(textFrame('好', 2));
      expect(acc.tiles, isEmpty, reason: '草稿未落卡前时间线无新增');
      final bool end = acc.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 3));

      expect(end, isTrue);
      expect(acc.tiles, hasLength(2));
      expect(acc.tiles[0].kind, ChatTileKind.text);
      expect(acc.tiles[0].text, '你好');
      expect(acc.tiles[1].kind, ChatTileKind.completed);
      expect(acc.lastSequence, 3);
    });

    test('同 sequence 不重复进；seq=0 恒应用', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(textFrame('a', 5));
      acc.applyFrame(textFrame('a-重复', 5));
      acc.applyFrame(TurnServerMessage(textDelta: TextDelta(delta: 'x')));
      acc.applyFrame(TurnServerMessage(textDelta: TextDelta(delta: 'y')));
      acc.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 6));

      expect(acc.tiles[0].text, 'axy');
      expect(acc.lastSequence, 6);
    });

    test('思考 snapshot 整体替换，落卡为 thinking 文本 tile', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(seqFrame(
          TurnServerMessage(
              thinkingDelta: ThinkingDelta(delta: '想一半')), 1));
      acc.applyFrame(seqFrame(
          TurnServerMessage(
              thinkingDelta: ThinkingDelta(snapshot: '权威全文')), 2));
      acc.applyFrame(textFrame('正文', 3));
      acc.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 4));

      expect(acc.tiles[0].kind, ChatTileKind.text);
      expect(acc.tiles[0].thinking, isTrue);
      expect(acc.tiles[0].text, '权威全文');
      expect(acc.tiles[1].text, '正文');
    });

    test('终局兜底：无 delta 时 final_blocks 落正文', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(seqFrame(
          TurnServerMessage(
              completed: TurnCompleted(
                  turnId: 't1',
                  finalBlocks: <ContentBlock>[
                ContentBlock(text: TextBlock(text: '终局正文'))
              ])),
          1));
      expect(acc.tiles[0].kind, ChatTileKind.text);
      expect(acc.tiles[0].text, '终局正文');
      expect(acc.tiles[1].kind, ChatTileKind.completed);
    });

    test('failed：partial 落正文 + 失败标记；空错误兜底', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      final bool end = acc.applyFrame(seqFrame(
          TurnServerMessage(
              failed: TurnFailed(
                  turnId: 't1',
                  error: Error(),
                  partialText: '做到一半')),
          1));
      expect(end, isTrue);
      expect(acc.tiles[0].text, '做到一半');
      expect(acc.tiles[1].kind, ChatTileKind.failed);
      expect(acc.tiles[1].text, '任务失败');
    });
  });

  group('ChatTimelineAccumulator：工具卡 / 审批占位', () {
    test('工具 STARTED 建卡，FINISHED 同 id 原地回填（一次调用一条）', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(seqFrame(
          TurnServerMessage(toolCall: toolStarted('c1', 'bash')), 1));
      expect(acc.tiles.single.toolStatus, 'running');
      expect(acc.tiles.single.toolName, 'bash');

      acc.applyFrame(seqFrame(
          TurnServerMessage(toolCall: toolFinished('c1', 'ok输出')), 2));
      expect(acc.tiles, hasLength(1), reason: '结果回填不另插卡');
      expect(acc.tiles.single.toolStatus, 'success');
      expect(acc.tiles.single.text, 'ok输出');
    });

    test('工具名空 → 兜底「工具」；失败结果标 failure', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(seqFrame(
          TurnServerMessage(
              toolCall: ToolCallUpdate(
                  phase: ToolCallUpdate_Phase.STARTED,
                  call: ToolCallBlock(id: 'c9'))),
          1));
      acc.applyFrame(seqFrame(
          TurnServerMessage(
              toolCall: toolFinished('c9', 'boom', ok: false)),
          2));
      expect(acc.tiles.single.toolName, '工具');
      expect(acc.tiles.single.toolStatus, 'failure');
    });

    test('审批请求 → 占位卡；approvalResolved → 撤卡 + 说明', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(seqFrame(
          TurnServerMessage(
              approvalRequest: ApprovalRequest(
                  requestId: 'r1', reason: '执行高危命令', details: 'rm 相关')),
          1));
      expect(acc.tiles.single.kind, ChatTileKind.approval);
      expect(acc.tiles.single.approvalId, 'r1');

      acc.applyFrame(seqFrame(
          TurnServerMessage(
              approvalResolved:
                  ApprovalResolved(requestId: 'r1', note: '桌面端已放行')),
          2));
      expect(
          acc.tiles.where((ChatTile t) => t.kind == ChatTileKind.approval),
          isEmpty);
      expect(acc.tiles.last.text, contains('桌面端已放行'));
    });
  });

  group('ChatTimelineAccumulator：Steered / Queued 徽标', () {
    test('Steered → 最早 pending 消息挂「已插入运行中任务」', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.addUserMessage('追问一句');
      acc.applyFrame(
          seqFrame(TurnServerMessage(steered: Steered(turnId: 't1')), 1));
      expect(acc.tiles.single.deliveryBadge, '已插入运行中任务');
    });

    test('Queued 按位挂徽标；pos=0 挂「队列已满，未送达」', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.addUserMessage('第一条');
      acc.addUserMessage('第二条');
      acc.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 3, depth: 3)), 1));
      acc.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 0, depth: 10)), 2));
      expect(acc.tiles[0].deliveryBadge, '排队中 · 第3位');
      expect(acc.tiles[1].deliveryBadge, '队列已满，未送达');
    });

    test('QueueFlushed → 排队徽标收掉（变回普通用户消息）', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.addUserMessage('排队的一条');
      acc.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 1, depth: 1)), 1));
      acc.applyFrame(seqFrame(
          TurnServerMessage(queueFlushed: QueueFlushed(remaining: 0)), 2));
      expect(acc.tiles.single.deliveryBadge, '');
    });

    test('无 pending 消息时回执是空操作（不炸）', () {
      final ChatTimelineAccumulator acc = ChatTimelineAccumulator();
      acc.applyFrame(
          seqFrame(TurnServerMessage(steered: Steered(turnId: 't1')), 1));
      expect(acc.tiles, isEmpty);
    });
  });

  group('ChatTimeline widget：五型渲染 + 徽标 + 空态', () {
    testWidgets('五型 tile 各就各位', (WidgetTester tester) async {
      await pumpTimeline(tester, tiles: <ChatTile>[
        const ChatTile(kind: ChatTileKind.text, text: '用户正文', isUser: true),
        const ChatTile(
            kind: ChatTileKind.toolCall,
            toolName: 'bash',
            toolStatus: 'success',
            text: '输出'),
        const ChatTile(
            kind: ChatTileKind.approval,
            approvalId: 'r1',
            approvalReason: '执行高危命令'),
        const ChatTile(kind: ChatTileKind.completed),
        const ChatTile(kind: ChatTileKind.failed, text: '炸了'),
      ]);

      expect(find.text('用户正文'), findsOneWidget);
      expect(find.text('工具：bash（success）'), findsOneWidget);
      expect(find.text('输出'), findsOneWidget);
      expect(find.text('需要审批'), findsOneWidget);
      expect(find.text('执行高危命令'), findsOneWidget);
      expect(find.text('任务完成'), findsOneWidget);
      expect(find.text('任务失败：炸了'), findsOneWidget);
    });

    testWidgets('Steered / Queued 徽标挂在用户消息下', (WidgetTester tester) async {
      await pumpTimeline(tester, tiles: <ChatTile>[
        const ChatTile(
            kind: ChatTileKind.text,
            text: '插队的那条',
            isUser: true,
            deliveryState: kDeliverySteered),
        const ChatTile(
            kind: ChatTileKind.text,
            text: '排队的那条',
            isUser: true,
            deliveryState: kDeliveryQueued,
            queuePos: 2),
      ]);

      expect(find.text('已插入运行中任务'), findsOneWidget);
      expect(find.text('排队中 · 第2位'), findsOneWidget);
    });

    testWidgets('空态；运行中尾巴；失败横幅', (WidgetTester tester) async {
      await pumpTimeline(tester, tiles: const <ChatTile>[]);
      expect(find.text('还没有消息，先发一句话开始吧'), findsOneWidget);

      await pumpTimeline(tester, tiles: const <ChatTile>[], isRunning: true);
      expect(find.text('运行中…'), findsOneWidget);
      expect(find.text('还没有消息，先发一句话开始吧'), findsNothing);

      await pumpTimeline(tester,
          tiles: const <ChatTile>[], errorText: '电脑端核对超时');
      expect(find.textContaining('电脑端核对超时'), findsOneWidget);
    });

    testWidgets('审批占位卡按钮是回调出口（真交互留 C4）',
        (WidgetTester tester) async {
      ApprovalAction? got;
      await pumpTimeline(
        tester,
        tiles: const <ChatTile>[
          ChatTile(
              kind: ChatTileKind.approval,
              approvalId: 'r7',
              approvalReason: '删库？'),
        ],
        onApproval: (ApprovalAction a) => got = a,
      );

      await tester.tap(find.text('允许'));
      expect(got!.requestId, 'r7');
      expect(got!.approved, isTrue);

      await tester.tap(find.text('拒绝'));
      expect(got!.approved, isFalse);
    });

    testWidgets('失败卡重试回调', (WidgetTester tester) async {
      var retried = false;
      await pumpTimeline(
        tester,
        tiles: const <ChatTile>[
          ChatTile(kind: ChatTileKind.failed, text: '炸了'),
        ],
        onRetry: () => retried = true,
      );

      await tester.tap(find.text('重试'));
      expect(retried, isTrue);
    });
  });
}
