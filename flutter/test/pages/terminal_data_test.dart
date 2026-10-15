// 终端纯函数 + 真通道数据源测试（B3 第 6 波）。
//
// 【两层各自的证词】
//   ① 纯函数（拼接/截断/退出码/封帧/行映射/副文案/错误文案）不拖通道，
//      逐条钉死「块上的字从 wire 哪个字段来 + 什么规则」；
//   ② 数据源走真 bridge + mock 宿主 wire 行为：两条**必须分家**的路径 ——
//      HOST_UNAVAILABLE → 人话错误（点名 B4 波次）vs 空响应 → 真空列表 ——
//      各钉一条（红线：不留假绿，「通道没就绪」绝不许伪装成「没有会话」）；
//      会话路径再钉 open/input/resize/close 的帧形状与输出/退出/错误的流向。

import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/terminal.pb.dart';
import 'package:pocketagent_ui/pages/terminal_data.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';
import 'package:pocketagent_ui/transport/terminal_stream.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kTerminalEventChannel);
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  group('appendTerminalOutput：增量拼接 + 掐头留尾', () {
    test('空起拼两块 = 直接相加，不过限不截断', () {
      final TerminalText step1 = appendTerminalOutput('', 'hello\n');
      expect(step1.text, 'hello\n');
      expect(step1.truncated, isFalse);

      final TerminalText step2 = appendTerminalOutput(step1.text, 'world\n');
      expect(step2.text, 'hello\nworld\n');
      expect(step2.truncated, isFalse);
    });

    test('恰好等于上限行 → 不截断（<= 语义）', () {
      final String three = List<String>.generate(3, (int i) => 'l$i').join('\n');
      final TerminalText step = appendTerminalOutput('', three, maxLines: 3);

      expect(step.truncated, isFalse);
      expect(step.text, three);
    });

    test('超出上限 → 掐头留尾（整行保留，不断半行）', () {
      final TerminalText step = appendTerminalOutput(
        'l0\nl1\n',
        'l2\nl3\n',
        maxLines: 3,
      );

      expect(step.truncated, isTrue);
      expect(step.text.split('\n'), <String>['l2', 'l3', '']);
    });

    test('超大单块一次掐到位 + 后续小块不再回落（页面负责闩锁）', () {
      final String big =
          List<String>.generate(10, (int i) => 'l$i').join('\n');
      final TerminalText step1 = appendTerminalOutput('', big, maxLines: 4);

      expect(step1.truncated, isTrue);
      expect(step1.text.split('\n').first, 'l6',
          reason: '10 行（无尾空段）留最后 4 行：l6/l7/l8/l9');

      final TerminalText step2 =
          appendTerminalOutput(step1.text, 'x', maxLines: 4);
      expect(step2.truncated, isFalse,
          reason: '本步没溢出 —— 但页面必须闩锁 step1 的 true（掐掉的字节回不来）');
    });

    test('空块拼接 = 原样返回（不凭空多行）', () {
      final TerminalText step = appendTerminalOutput('a\n', '');

      expect(step.text, 'a\n');
      expect(step.truncated, isFalse);
    });
  });

  group('terminalTruncationNotice：诚实截断标注', () {
    test('默认标注逐字（含 500 行上限数）', () {
      expect(
        terminalTruncationNotice(),
        '—— 输出过长，已截断，仅保留最后 500 行 ——',
      );
    });

    test('自定义上限数进标注', () {
      expect(terminalTruncationNotice(maxLines: 3), contains('3 行'));
    });
  });

  group('terminalExitText：退出码文案', () {
    test('0 = 正常退出（报喜不报忧都不做，只陈述）', () {
      expect(terminalExitText(0), '终端已正常退出（退出码 0）');
    });

    test('非 0 只陈述码值，不猜原因', () {
      expect(terminalExitText(1), '终端已退出（退出码 1）');
      expect(terminalExitText(137), contains('137'));
    });
  });

  group('封帧函数： TerminalClientMessage wire 往返', () {
    test('open 帧：session/cwd/shell/cols/rows 逐字段过桥', () {
      final Uint8List bytes = terminalOpenFrame(
        sessionId: 'sess-1',
        cwd: '/home/u',
        shell: 'bash',
        cols: 120,
        rows: 30,
      );

      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(bytes);
      expect(wire.whichMsg(), TerminalClientMessage_Msg.open);
      expect(wire.open.sessionId, 'sess-1');
      expect(wire.open.cwd, '/home/u');
      expect(wire.open.shell, 'bash');
      expect(wire.open.cols, 120);
      expect(wire.open.rows, 30);
    });

    test('open 帧默认值：空 cwd/shell + 80×24（对位 Android 默认）', () {
      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(
        terminalOpenFrame(sessionId: 's'),
      );

      expect(wire.open.cwd, isEmpty);
      expect(wire.open.shell, isEmpty);
      expect(wire.open.cols, 80);
      expect(wire.open.rows, 24);
    });

    test('input 帧：原始字节原样过桥（不转义）', () {
      final Uint8List bytes = terminalInputFrame(<int>[0x1B, 0x5B, 0x41]);

      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(bytes);
      expect(wire.whichMsg(), TerminalClientMessage_Msg.input);
      expect(wire.input.data, <int>[0x1B, 0x5B, 0x41]);
    });

    test('input 字符串版：UTF-8 编码（含中文多字节）', () {
      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(
        terminalInputFrameFromString('你好\n'),
      );

      expect(wire.whichMsg(), TerminalClientMessage_Msg.input);
      expect(utf8.decode(wire.input.data), '你好\n');
    });

    test('resize 帧：cols/rows 逐字段过桥', () {
      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(
        terminalResizeFrame(cols: 100, rows: 40),
      );

      expect(wire.whichMsg(), TerminalClientMessage_Msg.resize);
      expect(wire.resize.cols, 100);
      expect(wire.resize.rows, 40);
    });
  });

  group('行映射：TerminalSessionInfo → TerminalSessionRow', () {
    test('四字段逐字段对应（顺序保持 daemon 原序）', () {
      final List<TerminalSessionRow> rows = <TerminalSessionRow>[
        terminalSessionRowOf(TerminalSessionInfo(
            sessionId: 'b', shell: 'bash', cwd: '/b', alive: true)),
        terminalSessionRowOf(TerminalSessionInfo(
            sessionId: 'a', shell: 'zsh', cwd: '/a', alive: false)),
      ];

      expect(rows[0].id, 'b');
      expect(rows[0].alive, isTrue);
      expect(rows[1].shell, 'zsh');
      expect(rows[1].alive, isFalse);
    });

    test('副文案：有什么说什么；存活态恒为最后一节', () {
      expect(
        terminalSessionSubtitleOf(
            const TerminalSessionRow(id: 'x', shell: 'bash', cwd: '/home/u', alive: true)),
        'bash · /home/u · 运行中',
      );
      expect(
        terminalSessionSubtitleOf(
            const TerminalSessionRow(id: 'x', shell: '', cwd: '', alive: false)),
        '已退出',
        reason: 'shell/cwd 缺席不编占位，只剩存活态',
      );
      expect(
        terminalSessionSubtitleOf(
            const TerminalSessionRow(id: 'x', shell: 'bash', cwd: '', alive: false)),
        'bash · 已退出',
      );
    });
  });

  group('错误文案：HOST_UNAVAILABLE 定制 vs 透传', () {
    test('HOST_UNAVAILABLE → 点名 B4 波次（列表与连接两条）', () {
      const PaBridgeException e = PaBridgeException(
          PaBridgeErrorCodes.hostUnavailable, 'no host');

      expect(terminalLoadErrorText(e),
          allOf(contains('桌面端终端通道未就绪'), contains('B4 波次')));
      expect(terminalAttachErrorText(e),
          allOf(contains('桌面端终端通道未就绪'), contains('B4 波次')));
    });

    test('其余码 message 原样透传；空 message 退 code', () {
      expect(
        terminalLoadErrorText(
            const PaBridgeException('NOT_CONNECTED', '还没连上主机')),
        '还没连上主机',
      );
      expect(terminalLoadErrorText(const PaBridgeException('X', '')), 'X');
    });
  });

  group('BridgeTerminalSessionsSource：真通道 + mock 宿主 wire', () {
    test('成功 → 行模型（顺序保持 daemon 原序）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return ListTerminalsResponse(sessions: <TerminalSessionInfo>[
          TerminalSessionInfo(sessionId: 'zeta', shell: 'bash', alive: true),
          TerminalSessionInfo(sessionId: 'alpha', shell: 'zsh', alive: true),
        ]).writeToBuffer();
      });

      final List<TerminalSessionRow> rows =
          await BridgeTerminalSessionsSource().list();

      expect(rows.map((TerminalSessionRow r) => r.id).toList(),
          <String>['zeta', 'alpha']);
    });

    test('宿主桥未实现 → HOST_UNAVAILABLE → TerminalLoadFailure 点名 B4 波次',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method listTerminals');
      });

      await expectLater(
        BridgeTerminalSessionsSource().list(),
        throwsA(isA<TerminalLoadFailure>().having(
          (TerminalLoadFailure e) => e.message,
          'message',
          allOf(contains('桌面端终端通道未就绪'), contains('B4 波次')),
        )),
      );
    });

    test('宿主正常回空响应 → 真空列表（与错误路径分家的另一条）', () async {
      messenger.setMockMethodCallHandler(methodChannel,
          (MethodCall call) async => ListTerminalsResponse().writeToBuffer());

      expect(await BridgeTerminalSessionsSource().list(), isEmpty,
          reason: '空会话列表是合法事实，但必须走返回值而不是异常');
    });
  });

  group('BridgeTerminalSession：open/input/resize/close 帧形状与流向', () {
    final List<MethodCall> calls = <MethodCall>[];
    MockStreamHandlerEventSink? sink;

    Future<BridgeTerminalSession> openSession(
      String id, {
      int cols = 80,
      int rows = 24,
    }) async {
      calls.clear();
      sink = null;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        calls.add(call);
        return null;
      });
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink events) =>
              sink = events,
        ),
      );
      final BridgeTerminalSession session =
          BridgeTerminalSession(sessionId: id);
      await session.open(cols: cols, rows: rows);
      await pumpEventQueue();
      return session;
    }

    test('open 序列：terminalAttach(id) → terminalWrite(open 帧)', () async {
      final BridgeTerminalSession session =
          await openSession('sess-1', cols: 120, rows: 30);

      expect(calls, hasLength(2));
      expect(calls[0].method, 'terminalAttach');
      expect((calls[0].arguments as Map<Object?, Object?>)['terminalId'],
          'sess-1');
      expect(calls[1].method, 'terminalWrite');
      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(
          calls[1].arguments as Uint8List);
      expect(wire.whichMsg(), TerminalClientMessage_Msg.open);
      expect(wire.open.sessionId, 'sess-1');
      expect(wire.open.cols, 120);
      expect(wire.open.rows, 30);

      await session.close();
    });

    test('sendInput → terminalWrite(input 帧)，字节原文过桥', () async {
      final BridgeTerminalSession session = await openSession('sess-1');
      calls.clear();

      session.sendInput(utf8.encode('ls\n'));
      await pumpEventQueue();

      expect(calls, hasLength(1));
      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(
          calls.single.arguments as Uint8List);
      expect(wire.whichMsg(), TerminalClientMessage_Msg.input);
      expect(utf8.decode(wire.input.data), 'ls\n');

      await session.close();
    });

    test('resize → terminalWrite(resize 帧)', () async {
      final BridgeTerminalSession session = await openSession('sess-1');
      calls.clear();

      session.resize(cols: 100, rows: 40);
      await pumpEventQueue();

      expect(calls, hasLength(1));
      final TerminalClientMessage wire = TerminalClientMessage.fromBuffer(
          calls.single.arguments as Uint8List);
      expect(wire.whichMsg(), TerminalClientMessage_Msg.resize);
      expect(wire.resize.cols, 100);
      expect(wire.resize.rows, 40);

      await session.close();
    });

    test('output 帧 → output 流（原文字节）；exit 帧 → exitCodes 流', () async {
      final BridgeTerminalSession session = await openSession('sess-1');

      expectLater(session.output, emits(<int>[0x68, 0x69]));
      expectLater(session.exitCodes, emits(1));
      sink!.success(TerminalServerMessage(
        output: TerminalOutput(data: [0x68, 0x69]),
      ).writeToBuffer());
      sink!.success(TerminalServerMessage(
        exit: TerminalExit(sessionId: 'sess-1', code: 1),
      ).writeToBuffer());

      await pumpEventQueue();
      await session.close();
    });

    test('error 帧 → output 流错误 TerminalStreamFailure（不混进字节）',
        () async {
      final BridgeTerminalSession session = await openSession('sess-1');

      expectLater(
        session.output,
        emitsError(isA<TerminalStreamFailure>().having(
          (TerminalStreamFailure e) => e.message,
          'message',
          'pty 已销毁',
        )),
      );
      sink!.success(TerminalServerMessage(
        error: TerminalError(sessionId: 'sess-1', message: 'pty 已销毁'),
      ).writeToBuffer());
      // 先把错误帧泵进 output 流，再 close：close 会摘事件订阅 + 关流，
      // 不泵就直接关，错误帧到不了（平台通道分发要走事件轮）。
      await pumpEventQueue();

      await session.close();
    });

    test('close → terminalClose(id)；重复 close 只发一次', () async {
      final BridgeTerminalSession session = await openSession('sess-1');
      calls.clear();

      await session.close();
      await session.close();

      expect(calls, hasLength(1));
      expect(calls.single.method, 'terminalClose');
      expect((calls.single.arguments as Map<Object?, Object?>)['terminalId'],
          'sess-1');
    });

    test('未 open 就 sendInput/resize → StateError（输入无处可去不静默吞）',
        () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => null);
      final BridgeTerminalSession session =
          BridgeTerminalSession(sessionId: 'sess-x');

      expect(() => session.sendInput(<int>[0x41]), throwsStateError);
      expect(
          () => session.resize(cols: 80, rows: 24), throwsStateError);

      await session.close();
    });
  });
}
