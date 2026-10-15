// 终端页 widget 测试（B3 第 6 波）——注入可控 fake，不碰通道。
//
// 【三条路径分家】加载中 / 加载失败（带重试）/ 已就绪（空列表 vs 有会话）
// 各钉一条；HOST_UNAVAILABLE 风格的人话错误走错误态，不伪装成空列表。
// 会话视图再钉：attach 后输出块出现、输入发送行为、退出码呈现、截断标注、
// 尺寸变化发 resize、关闭回流。

import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/terminal.dart';
import 'package:pocketagent_ui/pages/terminal_data.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';

/// 可控列表源：rows/failure 双注入（失败时抛人话，与真源同形状）。
class _FakeSessionsSource implements TerminalSessionsSource {
  _FakeSessionsSource({this.rows = const <TerminalSessionRow>[], this.failure});

  List<TerminalSessionRow> rows;
  String? failure;

  @override
  Future<List<TerminalSessionRow>> list() async {
    if (failure != null) throw TerminalLoadFailure(failure!);
    return rows;
  }
}

/// 可控会话：记录 open/input/resize/close；测试侧按需推输出/退出/错误。
class _FakeSession implements TerminalSession {
  _FakeSession(this._id);

  final String _id;
  final StreamController<List<int>> _out =
      StreamController<List<int>>.broadcast();
  final StreamController<int> _exit = StreamController<int>.broadcast();

  bool opened = false;
  int openCols = 0;
  int openRows = 0;
  final List<List<int>> inputs = <List<int>>[];
  final List<(int, int)> resizes = <(int, int)>[];
  int closes = 0;

  @override
  String get sessionId => _id;

  @override
  Stream<List<int>> get output => _out.stream;

  @override
  Stream<int> get exitCodes => _exit.stream;

  @override
  Future<void> open({String? cwd, String? shell, int cols = 80, int rows = 24}) async {
    opened = true;
    openCols = cols;
    openRows = rows;
  }

  @override
  void sendInput(List<int> data) => inputs.add(data);

  @override
  void resize({required int cols, required int rows}) =>
      resizes.add((cols, rows));

  @override
  Future<void> close() async {
    closes++;
    await _out.close();
    await _exit.close();
  }

  void pushOutput(String text) =>
      _out.add(utf8.encode(text));

  void pushExit(int code) => _exit.add(code);

  void pushError(Object e) => _out.addError(e);
}

Widget _host(TerminalPage page) => MaterialApp(
      theme: PocketTheme.fromBrightness(PABrightness.dark),
      home: Scaffold(body: page),
    );

TerminalSessionRow _row(String id, {bool alive = true}) =>
    TerminalSessionRow(id: id, shell: 'bash', cwd: '/home/u', alive: alive);

void main() {
  group('会话列表三态', () {
    testWidgets('加载中 → 有会话列表（行 id + 副文案落位）', (tester) async {
      final _FakeSessionsSource source = _FakeSessionsSource(
        rows: <TerminalSessionRow>[_row('sess-1'), _row('sess-2', alive: false)],
      );
      final Map<String, _FakeSession> sessions = <String, _FakeSession>{};

      await tester.pumpWidget(_host(TerminalPage(
        onBack: () {},
        sessionsSource: source,
        sessionFactory: (String id) =>
            sessions.putIfAbsent(id, () => _FakeSession(id)),
      )));
      await tester.pump();

      expect(find.text('正在加载终端会话…'), findsNothing,
          reason: 'fake 源即时返回，一帧 pump 后应已就绪');
      expect(find.byKey(const ValueKey<String>('terminal-row-sess-1')),
          findsOneWidget);
      expect(find.text('bash · /home/u · 运行中'), findsOneWidget);
      expect(find.text('bash · /home/u · 已退出'), findsOneWidget);
    });

    testWidgets('加载失败 → 错误态带人话原因；重试后真拉到列表', (tester) async {
      final _FakeSessionsSource source = _FakeSessionsSource(
        failure: '桌面端终端通道未就绪（B4 波次）：终端会话列表暂不可用。',
      );

      await tester.pumpWidget(_host(TerminalPage(
        onBack: () {},
        sessionsSource: source,
        sessionFactory: (String id) => _FakeSession(id),
      )));
      await tester.pump();

      expect(find.text('终端会话列表加载失败'), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey<String>('terminal-list-error')))
            .data,
        contains('B4 波次'),
      );

      source.failure = null;
      source.rows = <TerminalSessionRow>[_row('sess-1')];
      await tester.tap(find.byKey(const ValueKey<String>('terminal-retry')));
      await tester.pump();

      expect(find.byKey(const ValueKey<String>('terminal-row-sess-1')),
          findsOneWidget);
    });

    testWidgets('空列表 → 真空态（不是通道错误的伪装）', (tester) async {
      await tester.pumpWidget(_host(TerminalPage(
        onBack: () {},
        sessionsSource: _FakeSessionsSource(),
        sessionFactory: (String id) => _FakeSession(id),
      )));
      await tester.pump();

      expect(find.byKey(const ValueKey<String>('terminal-empty')),
          findsOneWidget);
      expect(find.text('终端会话列表加载失败'), findsNothing);
    });

    testWidgets('返回键触发 onBack（列表态）', (tester) async {
      int backs = 0;
      await tester.pumpWidget(_host(TerminalPage(
        onBack: () => backs++,
        sessionsSource: _FakeSessionsSource(),
        sessionFactory: (String id) => _FakeSession(id),
      )));
      await tester.pump();

      await tester.tap(find.byKey(const ValueKey<String>('terminal-back')));
      expect(backs, 1);
    });
  });

  group('会话视图：attach/输出/输入/退出/截断/尺寸/关闭', () {
    Future<Map<String, _FakeSession>> pumpAttached(
      WidgetTester tester, {
      List<TerminalSessionRow> rows = const <TerminalSessionRow>[],
    }) async {
      final Map<String, _FakeSession> sessions = <String, _FakeSession>{};
      await tester.pumpWidget(_host(TerminalPage(
        onBack: () {},
        sessionsSource: _FakeSessionsSource(
          rows: rows.isEmpty ? <TerminalSessionRow>[_row('sess-1')] : rows,
        ),
        sessionFactory: (String id) =>
            sessions.putIfAbsent(id, () => _FakeSession(id)),
      )));
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey<String>('terminal-row-sess-1')));
      await tester.pump();
      return sessions;
    }

    testWidgets('点行 → attach（open 带 80×24）→ 输出块出现', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);
      final _FakeSession session = sessions['sess-1']!;

      expect(session.opened, isTrue);
      expect(session.openCols, 80);
      expect(session.openRows, 24);

      session.pushOutput('hello\n');
      await tester.pump();

      expect(
        tester
            .widget<SelectableText>(
                find.byKey(const ValueKey<String>('terminal-output')))
            .data,
        'hello\n',
      );
    });

    testWidgets('增量输出拼接：两块按序累积', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);

      sessions['sess-1']!.pushOutput('a\n');
      await tester.pump();
      sessions['sess-1']!.pushOutput('b\n');
      await tester.pump();

      expect(
        tester
            .widget<SelectableText>(
                find.byKey(const ValueKey<String>('terminal-output')))
            .data,
        'a\nb\n',
      );
    });

    testWidgets('输入行发送：原文 + 换行进 sendInput，输入框清空', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);
      final _FakeSession session = sessions['sess-1']!;

      await tester.enterText(
          find.byKey(const ValueKey<String>('terminal-input')), 'ls -la');
      await tester.tap(find.byKey(const ValueKey<String>('terminal-send')));
      await tester.pump();

      expect(session.inputs, hasLength(1));
      expect(utf8.decode(session.inputs.single), 'ls -la\n',
          reason: '行输入原文 + `\\n`（回车也是一个字节）');
      expect(
        tester
            .widget<TextField>(
                find.byKey(const ValueKey<String>('terminal-input')))
            .controller!
            .text,
        isEmpty,
      );
    });

    testWidgets('空输入不发送（无空帧进流）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);

      await tester.tap(find.byKey(const ValueKey<String>('terminal-send')));
      await tester.pump();

      expect(sessions['sess-1']!.inputs, isEmpty);
    });

    testWidgets('退出码呈现人话（非 0 只陈述码值）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);

      sessions['sess-1']!.pushExit(1);
      await tester.pump();

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey<String>('terminal-exit')))
            .data,
        '终端已退出（退出码 1）',
      );
    });

    testWidgets('超长输出 → 诚实截断标注出现（不把半份画成完整）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);

      sessions['sess-1']!.pushOutput(
        '${List<String>.generate(600, (int i) => 'l$i').join('\n')}\n',
      );
      await tester.pump();

      expect(find.byKey(const ValueKey<String>('terminal-truncated')),
          findsOneWidget);
      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey<String>('terminal-truncated')))
            .data,
        contains('500 行'),
      );
    });

    testWidgets('服务端错误帧 → 会话错误横幅（不混进输出字节）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);

      sessions['sess-1']!.pushError(const TerminalStreamFailure('pty 已销毁'));
      await tester.pump();

      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey<String>('terminal-session-error')))
            .data,
        'pty 已销毁',
      );
      expect(find.byKey(const ValueKey<String>('terminal-output')), findsNothing,
          reason: '错误字样不冒充终端输出：输出块仍为空态');
    });

    testWidgets('尺寸变化发 resize（列 +10 → 90×24 进流）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);
      final _FakeSession session = sessions['sess-1']!;

      await tester.tap(find.byKey(const ValueKey<String>('terminal-cols-plus')));
      await tester.pump();

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey<String>('terminal-size')))
            .data,
        '列 90 行 24',
      );
      expect(session.resizes.last, (90, 24));
    });

    testWidgets('关闭 → close 一次并回列表（列表重拉）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);
      final _FakeSession session = sessions['sess-1']!;

      // 会话视图是 ListView：关闭按钮在折叠线以下，先滚到可见再点
      // （与 collab 编辑器测试的 tapKey 同款机制，断言本身不动）。
      await tester.ensureVisible(
          find.byKey(const ValueKey<String>('terminal-close')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey<String>('terminal-close')));
      // close 链 + 列表重拉都是多跳 async：settle 到静态帧再断言。
      await tester.pumpAndSettle();

      expect(session.closes, 1);
      expect(find.byKey(const ValueKey<String>('terminal-row-sess-1')),
          findsOneWidget);
    });

    testWidgets('会话内返回键 → 回列表不断流（close 不应被调用）', (tester) async {
      final Map<String, _FakeSession> sessions = await pumpAttached(tester);
      int backs = 0;

      // 回列表用 header 返回键（会话态下它就是 terminal-back）。
      await tester.tap(find.byKey(const ValueKey<String>('terminal-back')));
      await tester.pump();

      expect(backs, 0);
      expect(sessions['sess-1']!.closes, 0,
          reason: '切回列表不断 PTY：close 只属于显式「关闭」');
      expect(find.byKey(const ValueKey<String>('terminal-row-sess-1')),
          findsOneWidget);
    });
  });
}
