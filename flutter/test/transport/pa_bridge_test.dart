// B2 通道契约测试 —— mock 的是「宿主桥的 wire 行为」，不是 PaBridge 自己。
//
// 【为什么不直接 stub PaBridge】要验的是契约本身：MethodCall 的方法名/参数编码、
// 返回字节的反序列化、错误码映射、事件 Envelope 判别。因此 mock 宿主侧 handler
// （TestDefaultBinaryMessenger 的 setMockMethodCallHandler / setMockStreamHandler），
// PaBridge 全程真实执行。「宿主未实现」的真实路径由"无 handler → HOST_UNAVAILABLE"
// 两条测试覆盖——宿主缺席时如实抛错，不是伪造数据（红线：不留假绿）。
//
// host_info / host_event 的字节直接复用 tools/protofixture/fixtures/*.b64：
// python protobuf（第三方编码器）现生成的真 wire 字节，与 test/gen/wire_fixture_test.dart
// 同一套账本——通道层的反序列化必须与 wire 层对得上，不能自己手搓字节自说自话。

import 'dart:convert';
import 'dart:io';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kPaBridgeEventChannel);
  late TestDefaultBinaryMessenger messenger;
  late PaBridge bridge;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    bridge = PaBridge();
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空，
    // 所以"无 handler"测试天然拿到干净环境，不存在状态串场。
  });

  /// 读 fixture 原始字节（与 wire_fixture_test.dart 同款：b64 包裹防 git autocrlf
  /// 改坏；两条候选路径都不中即 fail，绝不静默跳过）。
  Uint8List fixture(String name) {
    final candidates = <String>[
      '${Directory.current.path}/tools/protofixture/fixtures/$name.b64',
      '${Directory.current.path}/flutter/tools/protofixture/fixtures/$name.b64',
    ];
    for (final c in candidates) {
      final f = File(c);
      if (f.existsSync()) {
        return base64.decode(f.readAsStringSync().trim());
      }
    }
    fail('找不到 fixture $name（试过：${candidates.join(', ')}；'
        'cwd=${Directory.current.path}）');
  }

  group('方法面：参数编码与返回解码', () {
    test('getHostInfo：空 GetHostInfoRequest 字节入参 → HostInfo 反序列化', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return fixture('host_info');
      });

      final HostInfo info = await bridge.getHostInfo();

      expect(seen!.method, 'getHostInfo');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: 'GetHostInfoRequest 是空消息：空字节仍是"按 proto 走"的统一契约形式');
      // 断言值与 wire_fixture_test.dart 人工抄写的同一套哨兵值：两层用同一字节。
      expect(info.hostId, 'host-8f3a91c2');
      expect(info.displayName, 'YANZI-PC');
      expect(info.daemonVersion, '0.4.0');
      expect(info.capabilities, [
        'terminal', 'fs', 'git',
        'agent:codex', 'agent:pi',
        'collab:pi', 'runtime.v1',
      ]);
      expect(info.minAppVersion, '0.3.0');
      expect(info.recovery.turnJournal.pendingCount, 2,
          reason: '嵌套三层的 HostRecoveryReport 也随同一份字节到达');
    });

    test('connect：addr/token 键值原样过桥，宿主回 null 即受理', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.connect('192.168.1.20:7246', 'tok-secret');

      expect(seen!.method, 'connect');
      final Map<Object?, Object?> args =
          seen!.arguments as Map<Object?, Object?>;
      expect(args.keys.toSet(), {'addr', 'token'});
      expect(args['addr'], '192.168.1.20:7246');
      expect(args['token'], 'tok-secret',
          reason: 'token 原样进参数（进程内过桥）；成功与否看 conn_state，不看返回值');
    });

    test('connect：空地址本地即拒 INVALID_ADDR，不产生跨端调用', () async {
      var called = false;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        called = true;
        return null;
      });

      await expectLater(
        bridge.connect('   ', 'tok'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.invalidAddr)),
      );
      expect(called, isFalse, reason: '本地校验不过桥：过桥只会拿回同一个错');
    });

    test('disconnect：无参方法 arguments 为 null、回 null 即完成', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });

      await bridge.disconnect();

      expect(seen!.method, 'disconnect');
      expect(seen!.arguments, isNull);
    });

    test('pairCode：无参入参 → 非空 String 配对码', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return '482913';
      });

      expect(await bridge.pairCode(), '482913');
      expect(seen!.method, 'pairCode');
      expect(seen!.arguments, isNull);
    });

    test('pairCode：宿主回空串按 DECODE_FAILED 拒收（该抛 PAIR_DISABLED 而非空返回）', () async {
      messenger.setMockMethodCallHandler(methodChannel,
          (MethodCall call) async => '');

      await expectLater(
        bridge.pairCode(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)),
      );
    });

    test('getConnectionState：五键 Map → PaConnState（键名 = Kotlin 五条 StateFlow）',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return <Object?, Object?>{
          'connState': 'ok',
          'connTransport': 'adb-reverse',
          'connAddress': '127.0.0.1:7246',
          'connectionEpoch': 7,
          'hostCapabilities': <Object?>['terminal', 'fs', 'agent:codex'],
        };
      });

      final PaConnState state = await bridge.getConnectionState();

      expect(state.state, 'ok');
      expect(state.transport, 'adb-reverse');
      expect(state.address, '127.0.0.1:7246');
      expect(state.epoch, 7);
      expect(state.capabilities, ['terminal', 'fs', 'agent:codex']);
      // 未知档位值按 String 原样透传，不做 Dart 侧枚举白名单（未知输入透传红线）
      expect(
          const PaConnState(
                  state: 'warming_up',
                  transport: 'x',
                  address: 'y',
                  epoch: 0,
                  capabilities: <String>[])
              .state,
          'warming_up');
    });

    test('getConnectionState：缺键按 DECODE_FAILED 报契约漂移，不补零值', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return <Object?, Object?>{'connState': 'ok'};
      });

      await expectLater(
        bridge.getConnectionState(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('connTransport'))),
      );
    });

    test('getConnectionState：connectionEpoch 类型不符按 DECODE_FAILED 报', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return <Object?, Object?>{
          'connState': 'ok',
          'connTransport': 'grpc',
          'connAddress': 'a:1',
          'connectionEpoch': 'seven',
          'hostCapabilities': <Object?>[],
        };
      });

      await expectLater(
        bridge.getConnectionState(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('connectionEpoch'))),
      );
    });

    test('startTurn：TurnInput wire 字节入参，宿主可原样回读（与 gRPC 首帧同源）', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return null;
      });
      final ContentBlock block = ContentBlock()
        ..text = (TextBlock()..text = '把 README 翻译成中文');

      await bridge.startTurn(TurnInput(
        threadId: 'thread-42',
        blocks: <ContentBlock>[block],
        background: true,
        effort: 'high',
      ));

      expect(seen!.method, 'startTurn');
      final TurnInput wire =
          TurnInput.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.threadId, 'thread-42');
      expect(wire.blocks, hasLength(1));
      expect(wire.blocks[0].whichBlock(), ContentBlock_Block.text);
      expect(wire.blocks[0].text.text, '把 README 翻译成中文');
      expect(wire.background, isTrue);
      expect(wire.effort, 'high');
    });
  });

  group('方法面：错误映射', () {
    test('宿主 PlatformException → PaBridgeException，code/message/details 原样透传',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'NOT_CONNECTED',
          message: '还没连上主机',
          details: <String, Object?>{'retryable': true},
        );
      });

      await expectLater(
        bridge.getHostInfo(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having((PaBridgeException e) => e.message, 'message', '还没连上主机')
            .having((PaBridgeException e) => e.details, 'details',
                <String, Object?>{'retryable': true})),
      );
    });

    test('宿主桥未实现（null 应答 → MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      // 复刻生产路径：Android 宿主未注册 handler 时引擎对该通道回 null 应答，
      // Dart 侧 invokeMethod 随之抛 MissingPluginException。flutter_tester 对无
      // handler 通道回的是"空成功应答"（与 Android 不同），所以这里让 mock
      // handler 抛 MissingPluginException——setMockMethodCallHandler 收到后回
      // null 应答（test_default_binary_messenger.dart），与生产逐字节一致。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method getHostInfo');
      });

      await expectLater(
        bridge.getHostInfo(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.getHostInfo(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('String'))),
      );
    });

    test('坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常', () async {
      // 0xFF = field 31 + wire type 7（proto3 保留的非法类型）→ 解析必炸
      final Uint8List corrupt = Uint8List.fromList([0xFF, 0xFF, 0xFF]);
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => corrupt);

      await expectLater(
        bridge.getHostInfo(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('HostInfo'))),
      );
    });

    test('可注入构造：走 mock 的自定义通道，与生产通道隔离', () async {
      const MethodChannel altChannel = MethodChannel('dev.pocketagent/bridge.alt');
      var defaultHit = false;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        defaultHit = true;
        return null;
      });
      messenger.setMockMethodCallHandler(altChannel, (MethodCall call) async {
        expect(call.method, 'pairCode');
        return 'alt-pair';
      });
      final PaBridge altBridge = PaBridge(methodChannel: altChannel);

      expect(await altBridge.pairCode(), 'alt-pair');
      expect(defaultHit, isFalse, reason: '注入通道后生产通道不应被触碰');
    });
  });

  group('事件面：Envelope 判别与分发', () {
    MockStreamHandlerEventSink? sink;

    /// 订阅 [PaBridge.events]，返回收集器；listen 握手完成后 [sink] 非空。
    Future<({List<PaConnEvent> events, List<Object> errors})> subscribe(
        PaBridge target) async {
      sink = null;
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink events) =>
              sink = events,
        ),
      );
      final List<PaConnEvent> events = <PaConnEvent>[];
      final List<Object> errors = <Object>[];
      final sub = target.events.listen(events.add, onError: errors.add);
      addTearDown(sub.cancel);
      await pumpEventQueue();
      expect(sink, isNotNull, reason: 'listen 握手必须到达宿主侧');
      return (events: events, errors: errors);
    }

    test('conn_state 事件 → PaConnState（五键快照）', () async {
      final (:List<PaConnEvent> events, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(<Object?, Object?>{
        'type': 'conn_state',
        'connState': 'ok',
        'connTransport': 'grpc+tls',
        'connAddress': '100.64.0.7:7246',
        'connectionEpoch': 12,
        'hostCapabilities': <Object?>['terminal', 'fs'],
      });
      await pumpEventQueue();

      expect(errors, isEmpty);
      expect(events, hasLength(1));
      final PaConnEvent evt = events.single;
      expect(evt.type, PaConnEventType.connState);
      expect(evt.connState!.state, 'ok');
      expect(evt.connState!.transport, 'grpc+tls');
      expect(evt.connState!.address, '100.64.0.7:7246');
      expect(evt.connState!.epoch, 12);
      expect(evt.connState!.capabilities, ['terminal', 'fs']);
      expect(evt.hostInfo, isNull, reason: '非本类型载荷字段恒为 null');
      expect(evt.hostEvent, isNull);
    });

    test('host_info 事件 → HostInfo（proto wire 字节，与 getHostInfo 同源）', () async {
      final (:List<PaConnEvent> events, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(<Object?, Object?>{
        'type': 'host_info',
        'info': fixture('host_info'),
      });
      await pumpEventQueue();

      expect(errors, isEmpty);
      final PaConnEvent evt = events.single;
      expect(evt.type, PaConnEventType.hostInfo);
      expect(evt.hostInfo!.hostId, 'host-8f3a91c2');
      expect(evt.hostInfo!.lanIpv4, '192.168.1.20');
      expect(evt.connState, isNull);
      expect(evt.hostEvent, isNull);
    });

    test('host_event 事件 → HostEvent（WatchEvents 全局流载体，含 sequence）', () async {
      final (:List<PaConnEvent> events, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(<Object?, Object?>{
        'type': 'host_event',
        'event': fixture('host_event'),
      });
      await pumpEventQueue();

      expect(errors, isEmpty);
      final HostEvent hostEvent = events.single.hostEvent!;
      expect(hostEvent.whichEvent(), HostEvent_Event.turnFinished);
      expect(hostEvent.sequence, Int64(9001),
          reason: '与 TurnServerMessage.sequence 同一序列空间（断点续传水位）');
      expect(hostEvent.turnFinished.threadId, 'thread-42');
    });

    test('未知事件类型 → DECODE_FAILED 流错误，且不毒化流（后续好事件照发）', () async {
      final (:List<PaConnEvent> events, :List<Object> errors) =
          await subscribe(bridge);

      sink!.success(<Object?, Object?>{'type': 'conn_state_v2', 'x': 1});
      await pumpEventQueue();
      sink!.success(<Object?, Object?>{
        'type': 'conn_state',
        'connState': 'down',
        'connTransport': '',
        'connAddress': '',
        'connectionEpoch': 13,
        'hostCapabilities': <Object?>[],
      });
      await pumpEventQueue();

      expect(errors, hasLength(1));
      expect((errors.single as PaBridgeException).code,
          PaBridgeErrorCodes.decodeFailed);
      expect(events, hasLength(1),
          reason: '坏事件报错后流必须继续：断连重连场景会连续推多帧');
      expect(events.single.connState!.state, 'down');
    });

    test('宿主 EventSink.error → 流错误，code 原样透传', () async {
      final (:List<PaConnEvent> events, :List<Object> errors) =
          await subscribe(bridge);

      sink!.error(code: 'NOT_CONNECTED', message: '连接断了');
      await pumpEventQueue();

      expect(events, isEmpty);
      expect(errors, hasLength(1));
      final PaBridgeException e = errors.single as PaBridgeException;
      expect(e.code, 'NOT_CONNECTED');
      expect(e.message, '连接断了');
    });

    test('事件通道无 handler：握手错被框架吞掉（流静默），宿主在场以方法面为准', () async {
      // 真实行为对账（不是假绿）：Flutter 的 EventChannel.receiveBroadcastStream
      // 把 'listen' 的 MissingPluginException 转投 FlutterError.reportError
      // （框架层报告），不进流 —— 因此流错误只可能来自宿主主动 EventSink.error
      // 或 payload 违约。UI 判定"宿主在不在"必须走方法面，不能等事件流报错。
      final List<PaConnEvent> events = <PaConnEvent>[];
      final List<Object> errors = <Object>[];
      final sub = bridge.events.listen(events.add, onError: errors.add);
      addTearDown(sub.cancel);
      await pumpEventQueue();

      expect(errors, isEmpty, reason: '握手错不进流（见下方两环境说明）');
      expect(events, isEmpty);

      // 对照组：同一时刻方法面如实报 HOST_UNAVAILABLE（这才是宿主存活探针）。
      // 两个环境的握手错都"不进流"：flutter_tester 下 listen 拿空成功应答（无错）；
      // Android 生产下 listen 拿 null 应答 → MissingPluginException → 被
      // receiveBroadcastStream 转投 FlutterError.reportError，同样不进流。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });
      await expectLater(
        bridge.getConnectionState(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });
  });
}
