// TurnSession 状态机测试（B3 第 5 波①）——可注入的 TurnStreamBridge 配 mock
// 宿主行为，TurnSession 全程真实执行。
//
// 【分层】帧状态机（applyFrame/replayFrames）直接喂 proto 帧，不拖通道；
// 起流/追问/取消/续传四条走 mock 方法通道（断言方法名 + 参数编码 + 帧字节）；
// 事件订阅接线用一条"宿主推终局帧"的集成用例覆盖。mock 的永远是宿主 wire 行为。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_session.dart';
import 'package:pocketagent_ui/transport/chat_turn_stream.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel('test/session_methods');
  const EventChannel eventChannel = EventChannel('test/session_events');
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  int nextId = 0;

  /// 新会话（threadId 唯一，避免静态 runningThreadIds 跨用例串场）。
  TurnSession newSession() {
    nextId++;
    final TurnSession s = TurnSession(
      threadId: 'sess-$nextId',
      stream: TurnStreamBridge(
        methodChannel: methodChannel,
        eventChannel: eventChannel,
      ),
    );
    addTearDown(s.dispose);
    return s;
  }

  /// 方法面默认 mock：全部回 null（建流/发帧/拆流成功），并记录调用。
  List<MethodCall> mockMethodsOk() {
    final List<MethodCall> seen = <MethodCall>[];
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      seen.add(call);
      return null;
    });
    return seen;
  }

  TurnServerMessage seqFrame(TurnServerMessage f, int seq) {
    f.sequence = Int64(seq);
    return f;
  }

  TurnServerMessage textFrame(String d, int seq) =>
      seqFrame(TurnServerMessage(textDelta: TextDelta(delta: d)), seq);

  TurnInput inputOf(String threadId, String text) => TurnInput(
        threadId: threadId,
        blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: text))],
      );

  group('startTurn：走 stream 面统一', () {
    test('成功 → running，进 runningThreadIds；首帧 input 经 stream 发', () async {
      final TurnSession s = newSession();
      final List<MethodCall> seen = mockMethodsOk();

      await s.startTurn(inputOf(s.threadId, '修登录页'));

      expect(s.phase, TurnPhase.running);
      expect(TurnSession.runningThreadIds, contains(s.threadId));
      expect(seen.map((MethodCall c) => c.method),
          <String>['turnStreamStart', 'turnStreamSend']);
      expect(seen[0].arguments, <String, Object?>{'threadId': s.threadId});
      final TurnClientMessage first =
          TurnClientMessage.fromBuffer(seen[1].arguments as Uint8List);
      expect(first.whichMsg(), TurnClientMessage_Msg.input);
      expect(first.input.blocks.single.text.text, '修登录页');

      // 收尾：终局帧结算，避免静态集合残留本用例 id。
      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 1));
      expect(s.phase, TurnPhase.completed);
      expect(TurnSession.runningThreadIds, isNot(contains(s.threadId)));
    });

    test('运行中再 startTurn → StateError（追问走 steer）', () async {
      final TurnSession s = newSession();
      mockMethodsOk();
      await s.startTurn(inputOf(s.threadId, '第一句'));

      await expectLater(
        s.startTurn(inputOf(s.threadId, '第二句')),
        throwsStateError,
      );

      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 1));
    });

    test('宿主缺席 → HOST_UNAVAILABLE 上抛，phase 回 idle，不留假运行', () async {
      final TurnSession s = newSession();
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });

      await expectLater(
        s.startTurn(inputOf(s.threadId, 'hi')),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
      expect(s.phase, TurnPhase.idle);
      expect(TurnSession.runningThreadIds, isNot(contains(s.threadId)));
    });
  });

  group('steer：运行中追问走 supplement 帧', () {
    test('空闲 steer → StateError（请走 startTurn）', () async {
      final TurnSession s = newSession();
      mockMethodsOk();
      await expectLater(s.steer('追问'), throwsStateError);
    });

    test('空文本 → ArgumentError', () async {
      final TurnSession s = newSession();
      mockMethodsOk();
      await s.startTurn(inputOf(s.threadId, '首句'));
      await expectLater(s.steer('  '), throwsArgumentError);
      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 1));
    });

    test('运行中 steer 发 supplement 帧，本地态不动（等回执帧定）', () async {
      final TurnSession s = newSession();
      final List<MethodCall> seen = mockMethodsOk();
      await s.startTurn(inputOf(s.threadId, '首句'));

      await s.steer('补充一句');

      expect(seen.last.method, 'turnStreamSend');
      final TurnClientMessage wire =
          TurnClientMessage.fromBuffer(seen.last.arguments as Uint8List);
      expect(wire.whichMsg(), TurnClientMessage_Msg.supplement);
      expect(wire.supplement.blocks.single.text.text, '补充一句');
      expect(s.phase, TurnPhase.running,
          reason: 'Steered 还是 Queued 只看回执帧，发送成功不预判');
      expect(s.queuedCount, 0);

      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 1));
    });
  });

  group('帧状态机：sequence 水位 + 增量拼接', () {
    test('textDelta 拼接；thinking snapshot 整体替换', () {
      final TurnSession s = newSession();
      expect(s.applyFrame(textFrame('你', 1)), isFalse);
      expect(s.applyFrame(textFrame('好', 2)), isFalse);
      s.applyFrame(seqFrame(
          TurnServerMessage(
              thinkingDelta: ThinkingDelta(delta: '思考中')), 3));
      s.applyFrame(seqFrame(
          TurnServerMessage(
              thinkingDelta: ThinkingDelta(snapshot: '权威全文')), 4));

      expect(s.streamText, '你好');
      expect(s.thinkingText, '权威全文');
      expect(s.lastSequence, 4);
    });

    test('同 sequence 不重复进；旧帧丢弃；seq=0 恒应用', () {
      final TurnSession s = newSession();
      s.applyFrame(textFrame('a', 5));
      s.applyFrame(textFrame('a-重复', 5));
      s.applyFrame(textFrame('旧帧', 4));
      expect(s.streamText, 'a');
      expect(s.lastSequence, 5);

      s.applyFrame(TurnServerMessage(textDelta: TextDelta(delta: 'x')));
      s.applyFrame(TurnServerMessage(textDelta: TextDelta(delta: 'y')));
      expect(s.streamText, 'axy', reason: '无序号帧（旧 daemon）不拦截');
      expect(s.lastSequence, 5);
    });

    test('replayFrames 幂等（C2 恢复回放入口）', () {
      final TurnSession s = newSession();
      final List<TurnServerMessage> frames = <TurnServerMessage>[
        textFrame('甲', 11),
        textFrame('乙', 12),
      ];
      s.replayFrames(frames);
      s.replayFrames(frames);
      expect(s.streamText, '甲乙');
      expect(s.lastSequence, 12);
    });
  });

  group('投递回执：Steered / Queued / Overflow / Flushed', () {
    test('Steered → 回执「已插入运行中任务」，phase 不变', () {
      final TurnSession s = newSession();
      s.applyFrame(textFrame('首句', 1));
      s.applyFrame(seqFrame(TurnServerMessage(steered: Steered(turnId: 't1')), 2));

      expect(s.takeDeliveryNotice(), '已插入运行中任务');
      expect(s.takeDeliveryNotice(), isNull, reason: '消费后清 null');
      expect(s.phase, TurnPhase.idle, reason: '纯 applyFrame 不改运行相位（相位由起流/终局驱动）');
    });

    test('Queued(pos>0) → queued + 「已排队，当前轮结束后执行」；回放不写 Toast', () {
      final TurnSession s = newSession();
      s.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 2, depth: 2)), 1));

      expect(s.queuedCount, 1);
      expect(s.takeDeliveryNotice(), '已排队，当前轮结束后执行');
    });

    test('Queued(pos=0) → overflow 置位，不摆 Toast，不计排队', () {
      final TurnSession s = newSession();
      s.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 0, depth: 10)), 1));

      expect(s.hasOverflow, isTrue);
      expect(s.queuedCount, 0);
      expect(s.takeDeliveryNotice(), isNull);
    });

    test('QueueFlushed → 排队数减一；回放路径同样收敛', () {
      final TurnSession s = newSession();
      s.replayFrames(<TurnServerMessage>[
        seqFrame(TurnServerMessage(queued: Queued(position: 1, depth: 1)), 1),
        seqFrame(TurnServerMessage(queueFlushed: QueueFlushed(remaining: 0)), 2),
      ]);
      expect(s.queuedCount, 0);
      expect(s.takeDeliveryNotice(), isNull, reason: '回放补历史，不写本次提交的 Toast');
    });
  });

  group('终局：completed / failed / 排队不断尾', () {
    test('completed → completed；空草稿时 final_blocks 兜底正文', () {
      final TurnSession s = newSession();
      final bool end = s.applyFrame(seqFrame(
          TurnServerMessage(
              completed: TurnCompleted(
                  turnId: 't1',
                  finalBlocks: <ContentBlock>[
                ContentBlock(text: TextBlock(text: '终局正文'))
              ])),
          3));
      expect(end, isTrue);
      expect(s.phase, TurnPhase.completed);
      expect(s.streamText, '终局正文');
      expect(s.errorMessage, isNull);
    });

    test('failed → failed + 人话错误；空 message 兜底「任务失败」', () {
      final TurnSession s = newSession();
      s.applyFrame(seqFrame(
          TurnServerMessage(
              failed: TurnFailed(
                  turnId: 't1',
                  error: Error(code: 'E', message: ''),
                  partialText: '已产出一半')),
          1));
      expect(s.phase, TurnPhase.failed);
      expect(s.errorMessage, '任务失败');
      expect(s.streamText, '已产出一半');
    });

    test('有排队时终局帧不收尾（等 QueueFlushed + 下一条）', () async {
      final TurnSession s = newSession();
      mockMethodsOk();
      await s.startTurn(inputOf(s.threadId, '首句'));
      s.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 1, depth: 1)), 1));
      expect(s.phase, TurnPhase.queued);
      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 2));
      expect(s.phase, TurnPhase.queued,
          reason: '排队未空：保持运行态与流订阅，不中途收尾');
      expect(TurnSession.runningThreadIds, contains(s.threadId));

      s.applyFrame(seqFrame(
          TurnServerMessage(queueFlushed: QueueFlushed(remaining: 0)), 3));
      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't2')), 4));
      expect(s.phase, TurnPhase.completed);
      expect(TurnSession.runningThreadIds, isNot(contains(s.threadId)));
    });

    test('审批请求挂起；approvalResolved 撤卡（幽灵审批卡治理）', () {
      final TurnSession s = newSession();
      s.applyFrame(seqFrame(
          TurnServerMessage(
              approvalRequest: ApprovalRequest(
                  requestId: 'r1', reason: '跑 rm -rf？')), 1));
      expect(s.pendingApproval!.requestId, 'r1');
      s.applyFrame(seqFrame(
          TurnServerMessage(
              approvalResolved:
                  ApprovalResolved(requestId: 'r1', note: '桌面端已放行')),
          2));
      expect(s.pendingApproval, isNull);
    });

    test('resyncRequired → 断点作废 + needsFullHistory 置位', () {
      final TurnSession s = newSession();
      s.applyFrame(textFrame('a', 9));
      s.applyFrame(seqFrame(
          TurnServerMessage(
              resyncRequired: ResyncRequired(reason: '轮转丢窗')), 10));
      expect(s.lastSequence, 0);
      expect(s.needsFullHistory, isTrue);
    });
  });

  group('cancel / clearQueue / resyncFromDaemon', () {
    test('cancel 发流内 cancel 帧 → cancelling；空闲 cancel 是空操作', () async {
      final TurnSession s = newSession();
      final List<MethodCall> seen = mockMethodsOk();

      await s.cancel(); // 空闲：无轮次可终
      expect(seen, isEmpty);
      expect(s.phase, TurnPhase.idle);

      await s.startTurn(inputOf(s.threadId, '首句'));
      await s.cancel();
      final TurnClientMessage wire = TurnClientMessage.fromBuffer(
          seen.last.arguments as Uint8List);
      expect(seen.last.method, 'turnStreamSend');
      expect(wire.whichMsg(), TurnClientMessage_Msg.cancel);
      expect(wire.cancel.reason, isNotEmpty);
      expect(s.phase, TurnPhase.cancelling);

      // 终局 FAILED 帧收尾（取消后 daemon 以失败收轮）。
      s.applyFrame(seqFrame(
          TurnServerMessage(
              failed: TurnFailed(
                  turnId: 't1', error: Error(message: '已取消'))), 1));
      expect(s.phase, TurnPhase.failed);
      expect(TurnSession.runningThreadIds, isNot(contains(s.threadId)));
    });

    test('clearQueue 本地收敛（daemon 真清除随 B4 方法面补齐）', () async {
      final TurnSession s = newSession();
      mockMethodsOk();
      await s.startTurn(inputOf(s.threadId, '首句'));
      s.applyFrame(seqFrame(
          TurnServerMessage(queued: Queued(position: 1, depth: 1)), 1));
      expect(s.phase, TurnPhase.queued);

      s.clearQueue();
      expect(s.queuedCount, 0);
      expect(s.phase, TurnPhase.running);

      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 2));
    });

    test('resyncFromDaemon：turnResync 快照 → 补帧结算', () async {
      final TurnSession s = newSession();
      final List<MethodCall> seen = mockMethodsOk();
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen.add(call);
        if (call.method == 'turnResync') {
          return <Uint8List>[
            textFrame('补', 6).writeToBuffer(),
            seqFrame(
                    TurnServerMessage(
                        completed: TurnCompleted(turnId: 't9')), 7)
                .writeToBuffer(),
          ];
        }
        return null;
      });

      await s.resyncFromDaemon();

      expect(seen.single.method, 'turnResync');
      expect(seen.single.arguments,
          <String, Object?>{'threadId': s.threadId, 'afterSequence': 0});
      expect(s.streamText, '补');
      expect(s.phase, TurnPhase.completed);
      expect(s.lastSequence, 7);
    });

    test('resyncFromDaemon 宿主缺席 → HOST_UNAVAILABLE，不伪造已同步', () async {
      final TurnSession s = newSession();
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });

      await expectLater(
        s.resyncFromDaemon(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
      expect(s.phase, TurnPhase.idle);
    });
  });

  group('事件订阅接线：宿主推帧 → 状态机消费', () {
    test('宿主经 turn_events 推终局帧 → 起流的会话自动结算', () async {
      MockStreamHandlerEventSink? sink;
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink events) =>
              sink = events,
        ),
      );
      mockMethodsOk();
      final TurnSession s = newSession();

      await s.startTurn(inputOf(s.threadId, '首句'));
      expect(s.phase, TurnPhase.running);

      sink!.success(seqFrame(
              TurnServerMessage(completed: TurnCompleted(turnId: 'live')), 1)
          .writeToBuffer());
      await pumpEventQueue();

      expect(s.phase, TurnPhase.completed);
      expect(s.lastSequence, 1);
      expect(TurnSession.runningThreadIds, isNot(contains(s.threadId)));
    });

    test('坏帧不毒化会话：记诊断，phase 不变，后续好帧照收', () async {
      MockStreamHandlerEventSink? sink;
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink events) =>
              sink = events,
        ),
      );
      mockMethodsOk();
      final TurnSession s = newSession();
      await s.startTurn(inputOf(s.threadId, '首句'));

      sink!.success('not bytes');
      await pumpEventQueue();
      expect(s.streamError, isNotNull);
      expect(s.phase, TurnPhase.running);

      sink!.success(textFrame('续上', 1).writeToBuffer());
      await pumpEventQueue();
      expect(s.streamText, '续上');

      s.applyFrame(seqFrame(
          TurnServerMessage(completed: TurnCompleted(turnId: 't1')), 2));
    });
  });
}
