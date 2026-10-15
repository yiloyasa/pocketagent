// Chat 输入区 transport 面测试（B3 第 5 波③）——mock 的是「宿主桥的 wire
// 行为」，ChatInputTransport 全程真实执行。模式与 `chat_history_data_test`
// 的 transport 组一致（独立注入通道 + setMockMethodCallHandler），只是通道换
// 成输入区独立通道（`test/chat_input_methods`），与生产通道隔离。
//
// 【字节来源】响应字节全部由 Dart gen stub 现场组帧
// （ListSlashCommandsResponse → writeToBuffer）：等价 Kotlin 宿主
// `ListSlashCommandsResponse.newBuilder()...build().toByteArray()` 的写法；
// 坏字节用非法 wire bytes 钉 DECODE_FAILED。

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_input_transport.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel channel = MethodChannel('test/chat_input_methods');
  late TestDefaultBinaryMessenger messenger;
  late ChatInputTransport transport;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    transport = ChatInputTransport(channel: channel);
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  group('transport 面 listSlashCommands：参数编码与返回解码', () {
    test('请求字节 = ListSlashCommandsRequest（thread_id 原样过桥）', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        seen = call;
        return ListSlashCommandsResponse().writeToBuffer();
      });

      final ListSlashCommandsResponse response =
          await transport.listSlashCommands('thread-7');

      expect(seen!.method, 'listSlashCommands');
      final ListSlashCommandsRequest wire =
          ListSlashCommandsRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.threadId, 'thread-7');
      expect(wire.refresh, isFalse, reason: '日常面板走缓存，不强制刷新');
      expect(response.commands, isEmpty);
    });

    test('refresh=true 透传（进会话/下拉刷新语义）', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        seen = call;
        return ListSlashCommandsResponse().writeToBuffer();
      });

      await transport.listSlashCommands('thread-7', refresh: true);

      final ListSlashCommandsRequest wire =
          ListSlashCommandsRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.refresh, isTrue);
    });

    test('响应字节 → commands 逐条回填（name/description/inputHint）', () async {
      final ListSlashCommandsResponse want = ListSlashCommandsResponse(
        commands: <SlashCommand>[
          SlashCommand(
            name: 'goal',
            description: '常驻目标',
            inputHint: '/goal [pause|resume|…]',
          ),
          SlashCommand(name: 'init', description: '生成项目说明'),
        ],
      );
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        return want.writeToBuffer();
      });

      final ListSlashCommandsResponse got =
          await transport.listSlashCommands('thread-7');

      expect(got.commands.length, 2);
      expect(got.commands[0].name, 'goal');
      expect(got.commands[0].description, '常驻目标');
      expect(got.commands[0].inputHint, '/goal [pause|resume|…]');
      expect(got.commands[1].name, 'init');
      expect(got.commands[1].inputHint, isEmpty);
    });

    test('宿主缺席 → HOST_UNAVAILABLE（B4 前面板走内置兜底）', () async {
      // 复刻生产：宿主未注册 handler → 引擎回 null 应答 →
      // invokeMethod 抛 MissingPluginException（与 pa_bridge_test 同范式，
      // flutter_tester 对无 handler 的方法调用不会自发抛，必须由 mock 显式抛）。
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        throw MissingPluginException(
          'No implementation found for method ${call.method}',
        );
      });
      try {
        await transport.listSlashCommands('thread-7');
        fail('宿主缺席必须抛 HOST_UNAVAILABLE，不伪造空表');
      } on PaBridgeException catch (e) {
        expect(e.code, PaBridgeErrorCodes.hostUnavailable);
      }
    });

    test('宿主回非字节 → DECODE_FAILED（契约漂移显式失败）', () async {
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        return 'not-bytes';
      });

      try {
        await transport.listSlashCommands('thread-7');
        fail('坏返回必须抛 DECODE_FAILED');
      } on PaBridgeException catch (e) {
        expect(e.code, PaBridgeErrorCodes.decodeFailed);
        expect(e.message, contains('listSlashCommands 返回值'));
      }
    });

    test('宿主错误 code 原样透传（UI 按 code 分支）', () async {
      messenger.setMockMethodCallHandler(channel, (MethodCall call) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '未连接');
      });

      try {
        await transport.listSlashCommands('thread-7');
        fail('宿主错误必须透传');
      } on PaBridgeException catch (e) {
        expect(e.code, 'NOT_CONNECTED');
      }
    });
  });
}
