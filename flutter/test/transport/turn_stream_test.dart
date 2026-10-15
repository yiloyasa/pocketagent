// Turn 双向流 transport 流面测试（B3 第 5 波①）——mock 的是「宿主桥的 wire
// 行为」，TurnStreamBridge 全程真实执行。模式与 pa_bridge_test.dart 一致
// （独立注入通道 + setMockMethodCallHandler / setMockStreamHandler），
// 只是通道换成轮次流面（`test/turn_methods` / `test/turn_events`），与生产通道
// 隔离，也与 Terminal/Browser 的流面文件零重叠。
//
// 【字节来源】帧字节全部由 Dart gen stub 现场组帧（TurnServerMessage →
// writeToBuffer）：等价 Kotlin 宿主 `TurnServerMessage.newBuilder()...build()
// .toByteArray()` 的写法；坏字节用非法 wire bytes 钉 DECODE_FAILED。

import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/transport/chat_turn_stream.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel('test/turn_methods');
  const EventChannel eventChannel = EventChannel('test/turn_events');
  late TestDefaultBinaryMessenger messenger;
  late TurnStreamBridge bridge;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    bridge = TurnStreamBridge(
      methodChannel: methodChannel,
      eventChannel: eventChannel,
    );
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  TurnServerMessage textFrame(String text, int seq) => TurnServerMessage(
        textDelta: TextDelta(delta: text),
        sequence: Int64(seq),
      );

  group('方法面：建流 / 发帧 / 拆流', () {
    test('turnStreamStart：threadId 进 Map，宿主回 null', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.turnStreamStart('thread-42');

      expect(seen!.method, 'turnStreamStart');
      expect(
        seen!.arguments,
        <String, Object?>{'threadId': 'thread-42'},
        reason: '建流参数是 Map 形态（后续加建流参数只增键，不换形态）',
      );
    });

    test('turnStreamSend：TurnClientMessage input 帧字节原样过桥', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      final TurnInput input = TurnInput(
        threadId: 'thread-42',
        blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: '修登录页'))],
      );
      await bridge.turnStreamSend(
        TurnClientMessage(input: input).writeToBuffer(),
      );

      expect(seen!.method, 'turnStreamSend');
      final TurnClientMessage wire =
          TurnClientMessage.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.whichMsg(), TurnClientMessage_Msg.input);
      expect(wire.input.threadId, 'thread-42');
      expect(wire.input.blocks.single.text.text, '修登录页');
    });

    test('turnStreamSend：cancel 帧同样走这里（input/steer/cancel 统一入口）',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.turnStreamSend(
        TurnClientMessage(cancel: CancelSignal(reason: '用户在手机端终止'))
            .writeToBuffer(),
      );

      final TurnClientMessage wire =
          TurnClientMessage.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.whichMsg(), TurnClientMessage_Msg.cancel);
      expect(wire.cancel.reason, '用户在手机端终止');
    });

    test('turnStreamCancel：无参，宿主回 null', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.turnStreamCancel();

      expect(seen!.method, 'turnStreamCancel');
      expect(seen!.arguments, isNull);
    });

    test('宿主回非 null = 契约漂移 → DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => 'ok',
      );

      await expectLater(
        bridge.turnStreamStart('t'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('turnStreamStart'))),
      );
    });

    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });

      await expectLater(
        bridge.turnStreamSend(Uint8List(0)),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('宿主 PlatformException → code/message 原样透传', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '还没连上主机');
      });

      await expectLater(
        bridge.turnStreamCancel(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having(
                (PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });
  });

  group('turnResync：GetThreadEventsSince 的一次性方法面形态', () {
    test('快照字节数组 → 逐帧解码（空 = 无新帧）', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return <Uint8List>[
          textFrame('你', 7).writeToBuffer(),
          textFrame('好', 8).writeToBuffer(),
        ];
      });

      final List<TurnServerMessage> frames =
          await bridge.turnResync('thread-42', 6);

      expect(seen!.method, 'turnResync');
      expect(
        seen!.arguments,
        <String, Object?>{'threadId': 'thread-42', 'afterSequence': 6},
        reason: 'afterSequence = 本机已应用的最大 seq（proto 字段口径）',
      );
      expect(frames, hasLength(2));
      expect(frames[0].textDelta.delta, '你');
      expect(frames[0].sequence.toInt(), 7);
      expect(frames[1].textDelta.delta, '好');
    });

    test('宿主回空数组 = 无新帧（不是错误）', () async {
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => <Uint8List>[],
      );

      expect(await bridge.turnResync('t', 0), isEmpty);
    });

    test('返回非 List → DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => 'not a list',
      );

      await expectLater(
        bridge.turnResync('t', 0),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.decodeFailed)),
      );
    });

    test('坏元素字节 → DECODE_FAILED 且定位到下标，不裸抛 protobuf 异常', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return <Uint8List>[
          textFrame('好', 8).writeToBuffer(),
          Uint8List.fromList(<int>[0xFF, 0xFF, 0xFF]),
        ];
      });

      await expectLater(
        bridge.turnResync('t', 7),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('快照[1]'))),
      );
    });

    test('宿主缺席 → HOST_UNAVAILABLE（诚实占位，不伪造"已同步"）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });

      await expectLater(
        bridge.turnResync('t', 0),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });
  });

  group('事件面：turn_events 逐帧推送', () {
    MockStreamHandlerEventSink? sink;

    Future<({List<TurnServerMessage> frames, List<Object> errors})> subscribe() async {
      sink = null;
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink events) =>
              sink = events,
        ),
      );
      final List<TurnServerMessage> frames = <TurnServerMessage>[];
      final List<Object> errors = <Object>[];
      final StreamSubscription<TurnServerMessage> sub =
          bridge.events.listen(frames.add, onError: errors.add);
      addTearDown(sub.cancel);
      await pumpEventQueue();
      expect(sink, isNotNull, reason: 'listen 握手必须到达宿主侧');
      return (frames: frames, errors: errors);
    }

    test('好帧 → TurnServerMessage（含 sequence 水位）', () async {
      final (:List<TurnServerMessage> frames, :List<Object> errors) =
          await subscribe();

      sink!.success(textFrame('增量', 9).writeToBuffer());
      await pumpEventQueue();

      expect(errors, isEmpty);
      expect(frames, hasLength(1));
      expect(frames.single.textDelta.delta, '增量');
      expect(frames.single.sequence.toInt(), 9);
    });

    test('坏帧 → DECODE_FAILED 流错误，且不毒化流（后续好帧照收）', () async {
      final (:List<TurnServerMessage> frames, :List<Object> errors) =
          await subscribe();

      sink!.success('not bytes');
      await pumpEventQueue();
      sink!.success(textFrame('续上', 10).writeToBuffer());
      await pumpEventQueue();

      expect(errors, hasLength(1));
      expect((errors.single as PaBridgeException).code,
          PaBridgeErrorCodes.decodeFailed);
      expect(frames, hasLength(1),
          reason: '坏帧报错后流必须继续：增量风暴里一帧坏不能杀整条流');
      expect(frames.single.textDelta.delta, '续上');
    });

    test('宿主 EventSink.error → 流错误，code 原样透传', () async {
      final (:List<TurnServerMessage> frames, :List<Object> errors) =
          await subscribe();

      sink!.error(code: 'INTERNAL', message: '宿主炸了');
      await pumpEventQueue();

      expect(frames, isEmpty);
      expect(errors, hasLength(1));
      final PaBridgeException e = errors.single as PaBridgeException;
      expect(e.code, 'INTERNAL');
      expect(e.message, '宿主炸了');
    });

    test('宿主缺席 = 流静默；存活探针走方法面（F1b 定案）', () async {
      // 本测试不设 StreamHandler：事件面无推流。
      final List<TurnServerMessage> frames = <TurnServerMessage>[];
      final List<Object> errors = <Object>[];
      final StreamSubscription<TurnServerMessage> sub =
          bridge.events.listen(frames.add, onError: errors.add);
      addTearDown(sub.cancel);
      await pumpEventQueue();

      expect(errors, isEmpty, reason: '无推流不报错（F1b：宿主缺席 = 静默）');
      expect(frames, isEmpty);

      // 对照组：同一时刻方法面如实报 HOST_UNAVAILABLE（这才是宿主存活探针）。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });
      await expectLater(
        bridge.turnStreamStart('t'),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });
  });
}
