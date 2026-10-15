// Chat 周边动作方法面测试（B3 第 5 波④）——mock 的是「宿主桥的 wire
// 行为」，ChatActionsTransport 全程真实执行。模式与 turn_stream_test.dart
// 一致（独立注入通道 + setMockMethodCallHandler），与生产通道隔离。
//
// 【字节来源】请求字节由调用点组帧、宿主侧现解现断（方法名 + 请求消息）；
// 响应字节全部由 Dart gen stub 现场组帧 → writeToBuffer，等价 Kotlin 宿主
// `XxxResponse.newBuilder()...build().toByteArray()` 的写法；坏字节用非法
// wire bytes 钉 DECODE_FAILED。

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_actions_data.dart'
    show approvalSubmitRequest, approvalDecisionFrame;
import 'package:pocketagent_ui/pages/chat_actions_transport.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel channel = MethodChannel('test/chat_actions_methods');
  late TestDefaultBinaryMessenger messenger;
  late ChatActionsTransport transport;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    transport = ChatActionsTransport(channel: channel);
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  /// 方法面默认 mock：按方法名回 canned 响应字节，并记录调用。
  List<MethodCall> mockHost(Object? Function(MethodCall call) respond) {
    final List<MethodCall> seen = <MethodCall>[];
    messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
      seen.add(call);
      return respond(call);
    });
    return seen;
  }

  group('submitApproval：审批提交', () {
    test('方法名 + 请求字节（threadId + decision 四件套）', () async {
      final List<MethodCall> seen = mockHost((_) => null);

      await transport.submitApproval(
        approvalSubmitRequest(
          threadId: 't1',
          decision: approvalDecisionFrame(
            requestId: 'r1',
            approved: false,
            note: '太危险',
            option: 'deny-once',
          ),
        ),
      );

      expect(seen.single.method, 'submitApproval');
      final SubmitApprovalRequest wire = SubmitApprovalRequest.fromBuffer(
        seen.single.arguments as Uint8List,
      );
      expect(wire.threadId, 't1');
      expect(wire.decision.requestId, 'r1');
      expect(wire.decision.approved, isFalse);
      expect(wire.decision.note, '太危险');
      expect(wire.decision.option, 'deny-once');
    });

    test('宿主回非 null = 契约漂移 → DECODE_FAILED', () async {
      mockHost((_) => 'ok');

      await expectLater(
        transport.submitApproval(
          approvalSubmitRequest(
            threadId: 't',
            decision: approvalDecisionFrame(requestId: 'r', approved: true),
          ),
        ),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('submitApproval'))),
      );
    });
  });

  group('cancelTurn / compactThread：无返回语义', () {
    test('cancelTurn：threadId 进请求字节，宿主回 null', () async {
      final List<MethodCall> seen = mockHost((_) => null);

      await transport.cancelTurn('t-cancel');

      expect(seen.single.method, 'cancelTurn');
      expect(
        CancelTurnRequest.fromBuffer(seen.single.arguments as Uint8List)
            .threadId,
        't-cancel',
      );
    });

    test('compactThread：threadId 进请求字节，宿主回 null', () async {
      final List<MethodCall> seen = mockHost((_) => null);

      await transport.compactThread('t-compact');

      expect(seen.single.method, 'compactThread');
      expect(
        CompactThreadRequest.fromBuffer(seen.single.arguments as Uint8List)
            .threadId,
        't-compact',
      );
    });
  });

  group('clearQueuedInputs：两笔计数回传', () {
    test('steered/followUp 原样解码', () async {
      final List<MethodCall> seen = mockHost(
        (_) => ClearQueuedInputsResponse(steered: 1, followUp: 2)
            .writeToBuffer(),
      );

      final ClearQueuedInputsResponse r =
          await transport.clearQueuedInputs('t1');

      expect(seen.single.method, 'clearQueuedInputs');
      expect(
        ClearQueuedInputsRequest.fromBuffer(seen.single.arguments as Uint8List)
            .threadId,
        't1',
      );
      expect(r.steered, 1);
      expect(r.followUp, 2);
    });
  });

  group('forkThread / rollbackThread / setThreadGoal：Thread 真值回传', () {
    test('forkThread：lastTurnId 透传，返回 Thread', () async {
      final List<MethodCall> seen = mockHost(
        (_) => Thread(id: 't-forked').writeToBuffer(),
      );

      final Thread t =
          await transport.forkThread('t1', lastTurnId: 'turn-9');

      expect(seen.single.method, 'forkThread');
      final ForkThreadRequest wire =
          ForkThreadRequest.fromBuffer(seen.single.arguments as Uint8List);
      expect(wire.threadId, 't1');
      expect(wire.lastTurnId, 'turn-9');
      expect(t.id, 't-forked');
    });

    test('forkThread：lastTurnId 缺省空串（从最新处分叉）', () async {
      final List<MethodCall> seen = mockHost(
        (_) => Thread(id: 't2').writeToBuffer(),
      );

      await transport.forkThread('t1');

      expect(
        ForkThreadRequest.fromBuffer(seen.single.arguments as Uint8List)
            .lastTurnId,
        isEmpty,
      );
    });

    test('rollbackThread：numTurns 透传，返回 Thread', () async {
      final List<MethodCall> seen = mockHost(
        (_) => Thread(id: 't1').writeToBuffer(),
      );

      final Thread t = await transport.rollbackThread('t1', 3);

      expect(seen.single.method, 'rollbackThread');
      final RollbackThreadRequest wire =
          RollbackThreadRequest.fromBuffer(seen.single.arguments as Uint8List);
      expect(wire.threadId, 't1');
      expect(wire.numTurns, 3);
      expect(t.id, 't1');
    });

    test('setThreadGoal：goal 原样过桥，返回 Thread 真值', () async {
      final List<MethodCall> seen = mockHost(
        (_) => Thread(id: 't1').writeToBuffer(),
      );

      await transport.setThreadGoal('t1', '全量改 Kotlin');

      expect(seen.single.method, 'setThreadGoal');
      final SetThreadGoalRequest wire =
          SetThreadGoalRequest.fromBuffer(seen.single.arguments as Uint8List);
      expect(wire.threadId, 't1');
      expect(wire.goal, '全量改 Kotlin');
    });
  });

  group('startReview：fire-and-return 应答', () {
    test('focus 透传，reviewId/started 解码', () async {
      final List<MethodCall> seen = mockHost(
        (_) =>
            StartReviewResponse(reviewId: 'rev-1', started: true)
                .writeToBuffer(),
      );

      final StartReviewResponse r =
          await transport.startReview('t1', focus: '只看并发');

      expect(seen.single.method, 'startReview');
      final StartReviewRequest wire =
          StartReviewRequest.fromBuffer(seen.single.arguments as Uint8List);
      expect(wire.threadId, 't1');
      expect(wire.focus, '只看并发');
      expect(r.reviewId, 'rev-1');
      expect(r.started, isTrue);
    });
  });

  group('exportThreadHtml：落盘路径回传', () {
    test('path 原样解码（手机端按 path 取文件，不拼 URL）', () async {
      final List<MethodCall> seen = mockHost(
        (_) => ExportThreadHtmlResponse(path: '/ws/.pocketagent-exports/t1.html')
            .writeToBuffer(),
      );

      final ExportThreadHtmlResponse r =
          await transport.exportThreadHtml('t1');

      expect(seen.single.method, 'exportThreadHtml');
      expect(
        ExportThreadHtmlRequest.fromBuffer(seen.single.arguments as Uint8List)
            .threadId,
        't1',
      );
      expect(r.path, '/ws/.pocketagent-exports/t1.html');
    });
  });

  group('optimizePrompt：改写文本回传', () {
    test('五字段透传，optimized 解码', () async {
      final List<MethodCall> seen = mockHost(
        (_) => OptimizePromptResponse(optimized: '改好了').writeToBuffer(),
      );

      final OptimizePromptResponse r = await transport.optimizePrompt(
        text: '修一下',
        style: 'concise',
        agentId: 'codex',
        modelId: 'codex/gpt-5.4',
        effort: 'high',
      );

      expect(seen.single.method, 'optimizePrompt');
      final OptimizePromptRequest wire =
          OptimizePromptRequest.fromBuffer(seen.single.arguments as Uint8List);
      expect(wire.text, '修一下');
      expect(wire.style, 'concise');
      expect(wire.agentId, 'codex');
      expect(wire.modelId, 'codex/gpt-5.4');
      expect(wire.effort, 'high');
      expect(r.optimized, '改好了');
    });
  });

  group('错误模型（三件套与 pa_bridge 同契约）', () {
    test('宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });

      await expectLater(
        transport.compactThread('t'),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('宿主 PlatformException → code/message 原样透传', () async {
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '还没连上主机');
      });

      await expectLater(
        transport.exportThreadHtml('t'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having(
                (PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('坏响应字节 → DECODE_FAILED（定位到方法）', () async {
      mockHost((_) => Uint8List.fromList(<int>[0xFF, 0xFF, 0xFF]));

      await expectLater(
        transport.startReview('t'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('startReview 返回值'))),
      );
    });
  });
}
