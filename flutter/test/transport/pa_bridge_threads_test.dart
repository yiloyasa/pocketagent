// `listThreads` 方法面测试（B3-3 追补的第 7 条方法）——mock 的仍是「宿主桥的 wire
// 行为」，PaBridge 全程真实执行。模式与 pa_bridge_test.dart 完全一致（同一套
// fixture 读取、同一套错误映射断言），只是文件按方法面拆分，避免往既有 21 个
// 用例的大文件里塞。
//
// 【字节来源】响应字节 = `ListThreadsResponse` 包一份 python protoc 现生成的真
// `Thread` 字节（fixtures/thread.b64）：嵌套的 Thread 载荷来自第三方编码器，
// 通道层逐字段对账的就是那份真字节；外层 repeated 容器由 Dart gen stub 组帧
// （等价 Kotlin 宿主 `ListThreadsResponse.newBuilder().addThreads(...)` 的写法）。
// 空响应用 `ListThreadsResponse().writeToBuffer()` —— 0 字节也是合法 proto。

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

import 'fixture_support.dart' show fixtureBytes;

/// fixture 原始字节：两条候选路径 + 失败语义见 [fixtureBytes]。
Uint8List fixture(String name) => fixtureBytes(name);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;
  late PaBridge bridge;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    bridge = PaBridge();
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  /// 宿主形态的 ListThreadsResponse 字节：python 现生成的 Thread 字节包进
  /// repeated 字段 1（见文件头「字节来源」）。
  Uint8List threadsResponseBytes(List<int> threadBytes) =>
      ListThreadsResponse(
        threads: <Thread>[Thread.fromBuffer(threadBytes)],
      ).writeToBuffer();

  group('listThreads：参数编码与返回解码', () {
    test('默认（无参）= 空 ListThreadsRequest 字节；响应字节 → ListThreadsResponse', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return threadsResponseBytes(fixture('thread'));
      });

      final ListThreadsResponse response = await bridge.listThreads();

      expect(seen!.method, 'listThreads');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 空字节数组（编码总则 §2.1，同 getHostInfo）');
      // 逐字段对账 python 编码器写进 thread.b64 的哨兵值（与 wire_fixture_test
      // 同一套账本）：enum / optional / int64 / 嵌套消息都随同一份字节到达。
      expect(response.threads, hasLength(1));
      final Thread t = response.threads.single;
      expect(t.id, 'thread-42');
      expect(t.title, '修复登录页在低网速下的崩溃');
      expect(t.status, ThreadStatus.THREAD_STATUS_WAITING_APPROVAL,
          reason: 'enum 按编号解，不是按名字');
      expect(t.agentId, 'codex');
      expect(t.updatedAtUnixMs.toInt(), 1728901200000);
      expect(t.hasTurnStartedUnixMs(), isTrue,
          reason: 'proto3 optional 显式在场随通道透传');
    });

    test('带筛选参数：request 字段原样过桥，宿主可按 proto 回读', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListThreadsResponse().writeToBuffer();
      });

      await bridge.listThreads(ListThreadsRequest(
        includeArchived: true,
        limit: 50,
        searchTerm: '登录',
        sectionId: 'sec-bugfix',
        sortKey: 'created',
        sortAscending: true,
      ));

      final ListThreadsRequest wire =
          ListThreadsRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.includeArchived, isTrue);
      expect(wire.limit, 50);
      expect(wire.searchTerm, '登录');
      expect(wire.sectionId, 'sec-bugfix');
      expect(wire.sortKey, 'created');
      expect(wire.sortAscending, isTrue);
    });

    test('宿主回空响应字节（0 字节）= 合法空列表，不是错误', () async {
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => ListThreadsResponse().writeToBuffer(),
      );

      final ListThreadsResponse response = await bridge.listThreads();

      expect(response.threads, isEmpty);
    });
  });

  group('listThreads：错误映射', () {
    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛', () async {
      // 复刻生产：Android 宿主未注册 handler 时引擎回 null 应答 → Dart 侧
      // invokeMethod 抛 MissingPluginException（同 pa_bridge_test 的复刻说明）。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method listThreads');
      });

      await expectLater(
        bridge.listThreads(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('宿主 PlatformException → code/message/details 原样透传', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'NOT_CONNECTED',
          message: '还没连上主机',
          details: <String, Object?>{'retryable': true},
        );
      });

      await expectLater(
        bridge.listThreads(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having((PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.listThreads(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('Uint8List'))),
      );
    });

    test('坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常', () async {
      final Uint8List corrupt = Uint8List.fromList([0xFF, 0xFF, 0xFF]);
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => corrupt);

      await expectLater(
        bridge.listThreads(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('ListThreadsResponse'))),
      );
    });
  });
}
