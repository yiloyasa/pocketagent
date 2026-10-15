// Terminal 占位页渲染 + TerminalSession 契约形状测试。
//
// 【占位页只有就绪说明态】无真会话（xterm 依赖 + Attach 双向流都在 B5），
// 三态不适用 —— 断言诚实文案逐字，改一个字就红。
//
// 【会话抽象用假实现验形状】B5 的真实现（双向流 + xterm 桥）必须满足的最小
// 契约：open → sendInput 进 output 出 → resize 记录 → close 收尾。
// 这里用内存假实现把「形状」先钉死，真实现照此替换。
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/terminal.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';

/// 内存回显假会话：sendInput 的字节原样进 output（PTY 回显的最小模型）。
class _EchoSession implements TerminalSession {
  @override
  String get sessionId => 'sess-echo-1';

  final StreamController<List<int>> _out =
      StreamController<List<int>>.broadcast();
  final StreamController<int> _exit = StreamController<int>.broadcast();

  bool opened = false;
  String? cwd;
  String? shell;
  int cols = 0;
  int rows = 0;
  bool closed = false;

  @override
  Stream<List<int>> get output => _out.stream;

  @override
  Stream<int> get exitCodes => _exit.stream;

  @override
  Future<void> open({String? cwd, String? shell, int cols = 80, int rows = 24}) async {
    opened = true;
    this.cwd = cwd;
    this.shell = shell;
    this.cols = cols;
    this.rows = rows;
  }

  @override
  void sendInput(List<int> data) => _out.add(data);

  @override
  void resize({required int cols, required int rows}) {
    this.cols = cols;
    this.rows = rows;
  }

  @override
  Future<void> close() async {
    closed = true;
    await _out.close();
    await _exit.close();
  }
}

Widget _host(TerminalPage page) => MaterialApp(
      theme: PocketTheme.fromBrightness(PABrightness.dark),
      home: page,
    );

void main() {
  testWidgets('初始_标题 + 加载态（真列表页契约）', (tester) async {
    // 【占位契约已退役】page 中途从占位改成真列表实现（loading/error/ready
    // 三态，空态 key terminal-empty），本用例同步改断言首帧：标题 + 加载文案。
    // 错误态/空态/列表态由 terminal_test.dart 的真通道用例覆盖。
    await tester.pumpWidget(_host(TerminalPage(onBack: () {})));

    expect(find.text('终端'), findsOneWidget);
    expect(find.text('正在加载终端会话…'), findsOneWidget);
  });

  testWidgets('返回键触发 onBack', (tester) async {
    int backs = 0;
    await tester.pumpWidget(_host(TerminalPage(onBack: () => backs++)));

    await tester.tap(find.byKey(const ValueKey<String>('terminal-back')));
    expect(backs, 1);
  });

  group('TerminalSession 契约形状（假实现）', () {
    test('open 记录参数_默认 80x24', () async {
      final _EchoSession s = _EchoSession();
      await s.open();
      expect(s.opened, isTrue);
      expect(s.cols, 80);
      expect(s.rows, 24);
      expect(s.sessionId, isNotEmpty);
      await s.close();
    });

    test('sendInput 的字节进 output（回显）', () async {
      final _EchoSession s = _EchoSession();
      await s.open(cwd: r'D:\proj', shell: 'powershell.exe');
      expect(s.cwd, r'D:\proj');
      expect(s.shell, 'powershell.exe');

      expectLater(s.output, emits(<int>[65]));
      s.sendInput(<int>[65]);
      await s.close();
    });

    test('resize 更新尺寸', () async {
      final _EchoSession s = _EchoSession();
      await s.open();
      s.resize(cols: 120, rows: 30);
      expect(s.cols, 120);
      expect(s.rows, 30);
      await s.close();
      expect(s.closed, isTrue);
    });
  });
}
