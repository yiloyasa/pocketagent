// `TerminalStreamBridge` 流面测试（B3 第 6 波）——mock 的仍是「宿主桥的
// wire 行为」，bridge 全程真实。模式与 pa_bridge_test.dart 完全一致（同一套
// mock handler + 同一套错误映射断言），只是通道换成终端流面：
//   方法面 `dev.pocketagent/bridge` 的 listTerminals / terminalAttach /
//     terminalWrite / terminalClose 四个方法；
//   事件面 `dev.pocketagent/bridge/terminal_events` 的 TerminalServerMessage
//     逐帧推送。
//
// 【字节来源】响应/事件字节一律由 Dart gen stub 现组（`Xxx(...).writeToBuffer()`），
// 等价 Kotlin 宿主 `Xxx.newBuilder()...` 的写法；坏字节用非法 wire type 常量。


import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/terminal.pb.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';
import 'package:pocketagent_ui/transport/terminal_stream.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kTerminalEventChannel);
  late TestDefaultBinaryMessenger messenger;
  late TerminalStreamBridge bridge;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    bridge = TerminalStreamBridge();
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  Uint8List sessionsBytes(List<TerminalSessionInfo> sessions) =>
      ListTerminalsResponse(sessions: sessions).writeToBuffer();

  TerminalSessionInfo info(String id, {bool alive = true}) =>
      TerminalSessionInfo(sessionId: id, shell: 'bash', cwd: '/home/u', alive: alive);

  group('listTerminals：参数编码与返回解码', () {
    test('默认（无参）= 空 ListTerminalsRequest 字节；会话逐字段对账', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return sessionsBytes(<TerminalSessionInfo>[
          info('sess-1'),
          info('sess-2', alive: false),
        ]);
      });

      final ListTerminalsResponse response = await bridge.listTerminals();

      expect(seen!.method, 'listTerminals');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 空字节数组（编码总则，同 listThreads）');
      expect(response.sessions, hasLength(2));
      expect(response.sessions[0].sessionId, 'sess-1');
      expect(response.sessions[0].shell, 'bash');
      expect(response.sessions[1].alive, isFalse);
    });

    test('宿主回空响应（0 会话）= 合法真空列表，不是错误', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => sessionsBytes(<TerminalSessionInfo>[]));

      final ListTerminalsResponse response = await bridge.listTerminals();

      expect(response.sessions, isEmpty);
    });
  });

  group('listTerminals：错误映射', () {
    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method listTerminals');
      });

      await expectLater(
        bridge.listTerminals(),
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
        bridge.listTerminals(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having((PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.listTerminals(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('Uint8List'))),
      );
    });

    test('坏字节（非法 wire type）→ DECODE_FAILED，消息定位到 ListTerminalsResponse',
        () async {
      final Uint8List corrupt = Uint8List.fromList([0xFF, 0xFF, 0xFF]);
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => corrupt);

      await expectLater(
        bridge.listTerminals(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('ListTerminalsResponse'))),
      );
    });
  });

  group('terminalAttach / terminalWrite / terminalClose：参数与空返回', () {
    test('terminalAttach：terminalId 原样过桥，宿主回 null 即受理', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.terminalAttach('sess-1');

      expect(seen!.method, 'terminalAttach');
      expect(
          (seen!.arguments as Map<Object?, Object?>)['terminalId'], 'sess-1');
    });

    test('terminalWrite：调用方封好的 TerminalClientMessage 字节原样过桥',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });
      final Uint8List frame = TerminalClientMessage(
        resize: TerminalResize(cols: 80, rows: 24),
      ).writeToBuffer();

      await bridge.terminalWrite(frame);

      expect(seen!.method, 'terminalWrite');
      expect(seen!.arguments, isA<Uint8List>());
      final TerminalClientMessage wire =
          TerminalClientMessage.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.whichMsg(), TerminalClientMessage_Msg.resize);
      expect(wire.resize.cols, 80);
    });

    test('terminalClose：terminalId 原样过桥，宿主回 null 即受理', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.terminalClose('sess-9');

      expect(seen!.method, 'terminalClose');
      expect(
          (seen!.arguments as Map<Object?, Object?>)['terminalId'], 'sess-9');
    });

    test('三者宿主回非 null = 契约漂移 → DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not null');

      await expectLater(
        bridge.terminalAttach('x'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('terminalAttach'))),
      );
      await expectLater(
        bridge.terminalWrite(Uint8List(0)),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.decodeFailed)),
      );
      await expectLater(
        bridge.terminalClose('x'),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.decodeFailed)),
      );
    });

    test('三者宿主缺席 → HOST_UNAVAILABLE', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });

      for (final Future<void> Function() call in <Future<void> Function()>[
        () => bridge.terminalAttach('x'),
        () => bridge.terminalWrite(Uint8List(0)),
        () => bridge.terminalClose('x'),
      ]) {
        await expectLater(
          call(),
          throwsA(isA<PaBridgeException>().having(
              (PaBridgeException e) => e.code,
              'code',
              PaBridgeErrorCodes.hostUnavailable)),
        );
      }
    });

    test('可注入构造：走 mock 的自定义通道，与生产通道隔离', () async {
      const MethodChannel altMethods = MethodChannel('dev.pocketagent/bridge.termalt');
      const EventChannel altEvents =
          EventChannel('dev.pocketagent/bridge/terminal_events.alt');
      var defaultHit = false;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        defaultHit = true;
        return null;
      });
      messenger.setMockMethodCallHandler(altMethods, (MethodCall call) async {
        expect(call.method, 'terminalClose');
        return null;
      });
      final TerminalStreamBridge alt = TerminalStreamBridge(
        methodChannel: altMethods,
        eventChannel: altEvents,
      );

      await alt.terminalClose('sess-1');

      expect(defaultHit, isFalse, reason: '注入通道后生产通道不应被触碰');
    });
  });

  group('事件面：TerminalServerMessage 逐帧解码', () {
    MockStreamHandlerEventSink? sink;

    Future<({List<TerminalServerMessage> frames, List<Object> errors})>
        subscribe(TerminalStreamBridge target) async {
      sink = null;
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink events) =>
              sink = events,
        ),
      );
      final List<TerminalServerMessage> frames = <TerminalServerMessage>[];
      final List<Object> errors = <Object>[];
      final sub = target.events.listen(frames.add, onError: errors.add);
      addTearDown(sub.cancel);
      await pumpEventQueue();
      expect(sink, isNotNull, reason: 'listen 握手必须到达宿主侧');
      return (frames: frames, errors: errors);
    }

    test('output 帧 → data 原文透传（含 ANSI 不转义）', () async {
      final (:List<TerminalServerMessage> frames, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(TerminalServerMessage(
        output: TerminalOutput(data: [0x1B, 0x5B, 0x33, 0x32, 0x6D, 0x68, 0x69]),
      ).writeToBuffer());
      await pumpEventQueue();

      expect(errors, isEmpty);
      expect(frames, hasLength(1));
      expect(frames.single.whichMsg(), TerminalServerMessage_Msg.output);
      expect(frames.single.output.data,
          <int>[0x1B, 0x5B, 0x33, 0x32, 0x6D, 0x68, 0x69],
          reason: 'ESC[32mhi：转义字节原文透传，流面不解析');
    });

    test('exit / error / opened / resumed 帧逐项解码', () async {
      final (:List<TerminalServerMessage> frames, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(TerminalServerMessage(
        exit: TerminalExit(sessionId: 'sess-1', code: 1),
      ).writeToBuffer());
      sink!.success(TerminalServerMessage(
        error: TerminalError(sessionId: 'sess-1', message: 'pty 已销毁'),
      ).writeToBuffer());
      sink!.success(TerminalServerMessage(
        opened: TerminalOpened(sessionId: 'sess-1', shell: 'bash', cwd: '/home/u'),
      ).writeToBuffer());
      sink!.success(TerminalServerMessage(
        resumed: Resumed(sessionId: 'sess-1'),
      ).writeToBuffer());
      await pumpEventQueue();

      expect(errors, isEmpty);
      expect(frames, hasLength(4));
      expect(frames[0].exit.code, 1);
      expect(frames[1].error.message, 'pty 已销毁');
      expect(frames[2].opened.shell, 'bash');
      expect(frames[3].whichMsg(), TerminalServerMessage_Msg.resumed);
    });

    test('坏帧（非字节 payload）→ DECODE_FAILED 流错误，且不毒化流', () async {
      final (:List<TerminalServerMessage> frames, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success('not bytes');
      await pumpEventQueue();
      sink!.success(TerminalServerMessage(
        output: TerminalOutput(data: [0x41]),
      ).writeToBuffer());
      await pumpEventQueue();

      expect(errors, hasLength(1));
      expect((errors.single as PaBridgeException).code,
          PaBridgeErrorCodes.decodeFailed);
      expect((errors.single as PaBridgeException).message,
          contains('Uint8List'),
          reason: '验形失败定位到 wire 字节类型（与 PaBridge._expectBytes 同文）');
      expect(frames, hasLength(1),
          reason: '坏帧报错后流必须继续：断连重连场景会连续推多帧');
      expect(frames.single.output.data, <int>[0x41]);
    });

    test('坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常', () async {
      final (:List<TerminalServerMessage> frames, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(Uint8List.fromList([0xFF, 0xFF, 0xFF]));
      await pumpEventQueue();

      expect(frames, isEmpty);
      expect(errors, hasLength(1));
      expect((errors.single as PaBridgeException).code,
          PaBridgeErrorCodes.decodeFailed);
    });

    test('宿主 EventSink.error → 流错误，code 原样透传', () async {
      final (:List<TerminalServerMessage> frames, :List<Object> errors) =
          await subscribe(bridge);

      sink!.error(code: 'NOT_CONNECTED', message: '连接断了');
      await pumpEventQueue();

      expect(frames, isEmpty);
      expect(errors, hasLength(1));
      final PaBridgeException e = errors.single as PaBridgeException;
      expect(e.code, 'NOT_CONNECTED');
      expect(e.message, '连接断了');
    });

    test('宿主缺席：无 handler 时流静默（握手错不进流），宿主存活走方法面', () async {
      final List<TerminalServerMessage> frames = <TerminalServerMessage>[];
      final List<Object> errors = <Object>[];
      final sub = bridge.events.listen(frames.add, onError: errors.add);
      addTearDown(sub.cancel);
      await pumpEventQueue();

      expect(errors, isEmpty, reason: '握手错不进流（与 PaBridge.events 同款）');
      expect(frames, isEmpty);

      // 对照组：同一时刻方法面如实报 HOST_UNAVAILABLE（这才是宿主存活探针）。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });
      await expectLater(
        bridge.listTerminals(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });
  });
}
