// Chat 历史纯函数 + 方法面测试（B3 第 5 波子步②）。
//
// 【三层各自的证词】
//   ① 方法面（`getThreadHistory`）：mock 的是「宿主桥的 wire 行为」，
//      `ChatHistoryTransport` 全程真实执行——参数编码与返回解码 + 两条错误
//      映射（HOST_UNAVAILABLE / DECODE_FAILED），与 `pa_bridge_threads_test`
//      同模式（本文件即 transport 面的测试落点：新建文件清单里没有独立的
//      transport 测试文件，方法面用例收在这里，组名点名 transport）；
//   ② 映射纯函数：历史块 → 回放帧的逐字段落位（文本/思考/工具/图片跳过/
//      空消息占位 + sidecar 对齐 + 顺序保持）；
//   ③ 分页口径：档位收敛 / 入口判定 / 步进（Android HistoryRoundsLogic 同值）。
//
// 【搜索为什么没用例】`SearchThreadMessages` 本波不接（见
// chat_history_data.dart 文件头裁决）：没有消费面的方法不写测试占位，
// 不为不存在的输入造假绿。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_history_data.dart';
import 'package:pocketagent_ui/pages/chat_history_transport.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

ThreadMessage _msg(
  String role,
  List<ContentBlock> blocks, {
  int at = 1000,
  String mid = '',
  String turn = '',
}) =>
    ThreadMessage(
      role: role,
      blocks: blocks,
      createdAtUnixMs: Int64(at),
      messageId: mid,
      turnId: turn,
    );

ContentBlock _text(String s) => ContentBlock(text: TextBlock(text: s));

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('transport 面 getThreadHistory：参数编码与返回解码', () {
    const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
    late TestDefaultBinaryMessenger messenger;

    setUp(() {
      messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    });

    test('请求字节 = GetThreadHistoryRequest（thread_id/limit/rounds 原样过桥）',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return GetThreadHistoryResponse().writeToBuffer();
      });

      final ChatHistoryTransport transport = ChatHistoryTransport();
      final GetThreadHistoryResponse response =
          await transport.getThreadHistory('thread-7', limit: 0, rounds: 20);

      expect(seen!.method, 'getThreadHistory');
      final GetThreadHistoryRequest wire =
          GetThreadHistoryRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.threadId, 'thread-7');
      expect(wire.limit, 0);
      expect(wire.rounds, 20);
      expect(response.messages, isEmpty);
    });

    test('响应字节 → messages 逐条回填（含 blocks 与稳定身份字段）', () async {
      final GetThreadHistoryResponse want = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('user', <ContentBlock>[_text('先修登录页')],
              at: 1728901200000, mid: 'm-1', turn: 'turn-1'),
        ],
      );
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => want.writeToBuffer(),
      );

      final GetThreadHistoryResponse got =
          await ChatHistoryTransport().getThreadHistory('t');

      expect(got.messages, hasLength(1));
      expect(got.messages.single.role, 'user');
      expect(got.messages.single.blocks.single.text.text, '先修登录页');
      expect(got.messages.single.messageId, 'm-1');
      expect(got.messages.single.turnId, 'turn-1');
      expect(got.messages.single.createdAtUnixMs.toInt(), 1728901200000);
    });

    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method getThreadHistory');
      });

      await expectLater(
        ChatHistoryTransport().getThreadHistory('t'),
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
        ChatHistoryTransport().getThreadHistory('t'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having(
                (PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        ChatHistoryTransport().getThreadHistory('t'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('Uint8List'))),
      );
    });

    test('坏字节 → DECODE_FAILED，消息点名 GetThreadHistoryResponse', () async {
      messenger.setMockMethodCallHandler(methodChannel,
          (MethodCall call) async => Uint8List.fromList(<int>[0xFF, 0xFF, 0xFF]));

      await expectLater(
        ChatHistoryTransport().getThreadHistory('t'),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('GetThreadHistoryResponse'))),
      );
    });
  });

  group('historyToFrames / historyFrameMetas：字段映射', () {
    test('文本块 → textDelta 原样 + sidecar 带 role/时间/稳定身份', () {
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('user', <ContentBlock>[_text('先修登录页')],
              at: 1700, mid: 'm-1', turn: 'turn-1'),
          _msg('assistant', <ContentBlock>[_text('改完了')],
              at: 1800, mid: 'm-2', turn: 'turn-1'),
        ],
      );

      final List<TurnServerMessage> frames = historyToFrames(response);
      final List<HistoryFrameMeta> metas = historyFrameMetas(response);

      expect(frames, hasLength(2));
      expect(frames[0].textDelta.delta, '先修登录页');
      expect(frames[1].textDelta.delta, '改完了');
      // 历史帧恒无序号：0 = 按无续传能力处理（thread.proto 续传注释）。
      for (final TurnServerMessage f in frames) {
        expect(f.sequence.toInt(), 0);
      }
      // sidecar 下标对齐：role/时间/message_id/turn_id 进 meta，不进帧。
      expect(metas, hasLength(2));
      expect(
        metas[0],
        const HistoryFrameMeta(
            role: 'user', createdAtMs: 1700, messageId: 'm-1', turnId: 'turn-1'),
      );
      expect(
        metas[1],
        const HistoryFrameMeta(
            role: 'assistant',
            createdAtMs: 1800,
            messageId: 'm-2',
            turnId: 'turn-1'),
      );
    });

    test('思考块 → thinkingDelta.snapshot（整体替换语义，不走 delta）', () {
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('assistant', <ContentBlock>[
            ContentBlock(thinking: ThinkingBlock(text: '先想想方案'))
          ]),
        ],
      );

      final List<TurnServerMessage> frames = historyToFrames(response);

      expect(frames, hasLength(1));
      expect(frames.single.thinkingDelta.snapshot, '先想想方案');
      expect(frames.single.thinkingDelta.delta, isEmpty);
    });

    test('工具调用/结果 → STARTED/FINISHED 两帧，载荷逐字透传', () {
      final ToolCallBlock call =
          ToolCallBlock(id: 'c1', name: 'read', jsonArgs: '{"p":"x"}');
      final ToolResultBlock result =
          ToolResultBlock(callId: 'c1', ok: true, output: 'ok', elapsedMs: 12);
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('assistant', <ContentBlock>[
            ContentBlock(toolCall: call),
            ContentBlock(toolResult: result),
          ]),
        ],
      );

      final List<TurnServerMessage> frames = historyToFrames(response);

      expect(frames, hasLength(2));
      expect(frames[0].toolCall.phase, ToolCallUpdate_Phase.STARTED);
      expect(frames[0].toolCall.call.name, 'read');
      expect(frames[0].toolCall.call.jsonArgs, '{"p":"x"}');
      expect(frames[1].toolCall.phase, ToolCallUpdate_Phase.FINISHED);
      expect(frames[1].toolCall.result.output, 'ok');
      expect(frames[1].toolCall.result.ok, isTrue);
    });

    test('图片块跳过（无载体帧）：不产生帧也不产生元数据，对齐不断', () {
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('user', <ContentBlock>[
            ContentBlock(
                image: ImageBlock(
                    data: <int>[1, 2, 3], mediaType: 'image/png')),
            _text('看这张截图'),
          ]),
        ],
      );

      final List<TurnServerMessage> frames = historyToFrames(response);
      final List<HistoryFrameMeta> metas = historyFrameMetas(response);

      expect(frames, hasLength(1));
      expect(frames.single.textDelta.delta, '看这张截图');
      expect(metas, hasLength(1));
      expect(metas.single.role, 'user');
    });

    test('空消息与纯图片消息仍各占一帧空 textDelta（顺序不塌）', () {
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('assistant', <ContentBlock>[]),
          _msg('user', <ContentBlock>[
            ContentBlock(
                image: ImageBlock(data: <int>[9], mediaType: 'image/png'))
          ]),
          _msg('assistant', <ContentBlock>[_text('尾')]),
        ],
      );

      final List<TurnServerMessage> frames = historyToFrames(response);
      final List<HistoryFrameMeta> metas = historyFrameMetas(response);

      expect(frames, hasLength(3));
      expect(frames[0].textDelta.delta, isEmpty);
      expect(frames[1].textDelta.delta, isEmpty);
      expect(frames[2].textDelta.delta, '尾');
      expect(metas.map((HistoryFrameMeta m) => m.role).toList(),
          <String>['assistant', 'user', 'assistant']);
    });

    test('顺序保持响应原序：多块多消息按到来序展开', () {
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('user', <ContentBlock>[_text('一'), _text('二')]),
          _msg('assistant', <ContentBlock>[_text('三')]),
        ],
      );

      final List<String> deltas = historyToFrames(response)
          .map((TurnServerMessage f) => f.textDelta.delta)
          .toList();

      expect(deltas, <String>['一', '二', '三']);
    });
  });

  group('historyTextOf / historyRoleLabel', () {
    test('多文本块按换行拼；非文本块跳过；全无文本 → 空串', () {
      expect(
        historyTextOf(_msg('assistant', <ContentBlock>[_text('a'), _text('b')])),
        'a\nb',
      );
      expect(
        historyTextOf(_msg('assistant', <ContentBlock>[
          ContentBlock(thinking: ThinkingBlock(text: '想')),
          _text('说'),
        ])),
        '说',
      );
      expect(historyTextOf(_msg('assistant', <ContentBlock>[])), '');
    });

    test('角色标签：user/assistant 有中文；未知原样透传', () {
      expect(historyRoleLabel('user'), '我');
      expect(historyRoleLabel('assistant'), '助手');
      expect(historyRoleLabel('tool'), 'tool');
      expect(historyRoleLabel(''), '');
    });
  });

  group('按轮分页口径（HistoryRoundsLogic 同值）', () {
    test('clampHistoryRounds：档位内原样，否则回默认 20', () {
      expect(clampHistoryRounds(10), 10);
      expect(clampHistoryRounds(20), 20);
      expect(clampHistoryRounds(30), 30);
      expect(clampHistoryRounds(50), 50);
      expect(clampHistoryRounds(0), kDefaultHistoryRounds);
      expect(clampHistoryRounds(-1), kDefaultHistoryRounds);
      expect(clampHistoryRounds(15), kDefaultHistoryRounds);
      expect(clampHistoryRounds(100), kDefaultHistoryRounds);
    });

    test('canLoadEarlier：全量/给到头/首条非 user 都不显示入口', () {
      bool can({String? first, required int rounds, required int req}) =>
          canLoadEarlier(
              firstRole: first, roundCount: rounds, requestedRounds: req);
      expect(can(first: 'user', rounds: 20, req: 20), isTrue);
      expect(can(first: 'user', rounds: 20, req: 0), isFalse,
          reason: '全量路径永远不显示入口');
      expect(can(first: 'user', rounds: 5, req: 20), isFalse,
          reason: 'daemon 已给到会话开头');
      expect(can(first: 'assistant', rounds: 20, req: 20), isFalse,
          reason: '未落在轮边界：大概率是天生的开头');
      expect(can(first: null, rounds: 20, req: 20), isFalse);
    });

    test('nextHistoryRounds：当前累计 + 步长；步长 <1 按 1', () {
      expect(nextHistoryRounds(20, 20), 40);
      expect(nextHistoryRounds(20, 0), 21);
      expect(nextHistoryRounds(20, -5), 21);
    });

    test('historyRoundCount：user 消息条数即轮数估计', () {
      final GetThreadHistoryResponse response = GetThreadHistoryResponse(
        messages: <ThreadMessage>[
          _msg('user', <ContentBlock>[_text('一')]),
          _msg('assistant', <ContentBlock>[_text('二')]),
          _msg('user', <ContentBlock>[_text('三')]),
        ],
      );
      expect(historyRoundCount(response), 2);
      expect(historyRoundCount(GetThreadHistoryResponse()), 0);
    });
  });

  group('数据源：错误翻人话 + fake 行为', () {
    test('Bridge 源默认请求最近 20 轮；HOST_UNAVAILABLE 翻成 B4 人话', () async {
      const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
      final TestDefaultBinaryMessenger messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        throw MissingPluginException('no host');
      });

      await expectLater(
        BridgeChatHistorySource().loadHistory('t-1'),
        throwsA(isA<ChatHistoryLoadFailure>().having(
            (ChatHistoryLoadFailure e) => e.message,
            'message',
            // 【H5 收敛后收紧为全串相等】chatHistoryErrorText 的返回值改由
            // pa_page_kit.dart 的 [paChannelErrorText] 拼装，contains 不足以
            // 证明「前缀 + 通道名 + 收尾句」三段拼接逐字节未变。
            '桌面端会话历史通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
                '会话历史暂不可用。')),
        reason: '宿主没就绪必须走错误路径——返回空响应就是把错误粉饰成「没有历史」',
      );
      final GetThreadHistoryRequest wire =
          GetThreadHistoryRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.threadId, 't-1');
      expect(wire.rounds, kDefaultHistoryRounds);
      expect(wire.limit, 0);
    });

    test('Bridge 源透传宿主 message；空 message 退 code', () async {
      const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
      final TestDefaultBinaryMessenger messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;

      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '还没连上主机');
      });
      await expectLater(
        BridgeChatHistorySource().loadHistory('t'),
        throwsA(isA<ChatHistoryLoadFailure>().having(
            (ChatHistoryLoadFailure e) => e.message, 'message', '还没连上主机')),
      );

      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(code: 'INTERNAL');
      });
      await expectLater(
        BridgeChatHistorySource().loadHistory('t'),
        throwsA(isA<ChatHistoryLoadFailure>().having(
            (ChatHistoryLoadFailure e) => e.message, 'message', 'INTERNAL')),
      );
    });

    test('Fake 源：记录请求的 threadId；failure 非空如实抛错', () async {
      final FakeChatHistorySource fake = FakeChatHistorySource(
        response: GetThreadHistoryResponse(
          messages: <ThreadMessage>[_msg('user', <ContentBlock>[_text('hi')])],
        ),
      );

      final GetThreadHistoryResponse got = await fake.loadHistory('t-9');
      expect(got.messages.single.blocks.single.text.text, 'hi');
      expect(fake.requested, <String>['t-9']);

      fake.failure = 'daemon 不可达';
      await expectLater(
        fake.loadHistory('t-9'),
        throwsA(isA<ChatHistoryLoadFailure>().having(
            (ChatHistoryLoadFailure e) => e.message, 'message', 'daemon 不可达')),
      );
    });
  });
}
