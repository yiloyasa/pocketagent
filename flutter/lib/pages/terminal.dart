// 终端页（B3 第 6 波）：会话列表 + 纯文本终端视图，全走真通道。
//
// 【页面职责】会话列表（listTerminals 三态）+ 终端视图（输出块列表 + 底部
// 输入行 + 发送）+ attach/close 动作 + 尺寸变化发 resize。数据源默认
// [BridgeTerminalSessionsSource] / [BridgeTerminalSession]（见
// terminal_data.dart）；宿主桥未注册（B4 波次前）时如实进错误态
// 「桌面端终端通道未就绪（B4 波次）」，**不伪装成空列表**（两条路径分别有
// 测试钉死）。
//
// 【xterm 不做】无 xterm 依赖，不自研终端仿真器；输出块按纯文本块渲染
// （可滚动、等宽字体、增量拼接、超长截断 + 诚实「已截断」标注），原文透传，
// 不摆假光标、不摆假 shell 提示符。WebView/xterm 桥留后续波次。

import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'pa_page_kit.dart';
import 'terminal_data.dart';

export 'terminal_data.dart' show TerminalSession, TerminalSessionFactory;

/// 终端页：真通道会话列表 + 纯文本终端视图。
///
/// 数据源与会话工厂都是注入点：测试传可控 fake；生产不传即真通道
/// （`dev.pocketagent/bridge` 方法面 + `terminal_events` 事件面）。
class TerminalPage extends StatefulWidget {
  const TerminalPage({
    super.key,
    required this.onBack,
    this.sessionsSource,
    this.sessionFactory,
    this.initialCols = 80,
    this.initialRows = 24,
  });

  /// 返回（Kotlin `onBack` → popBackStack）。
  final VoidCallback onBack;

  final TerminalSessionsSource? sessionsSource;
  final TerminalSessionFactory? sessionFactory;

  /// 初始 PTY 尺寸（对位 TerminalScreen.kt:475-476 的 80×24）。
  final int initialCols;
  final int initialRows;

  @override
  State<TerminalPage> createState() => _TerminalPageState();
}

/// 三态：加载中 / 加载失败 / 已就绪（就绪后再分「空列表」与「有会话」）。
typedef _TerminalPhase = PaAsyncPhase;

/// 单个会话的视图态（输出累积 + 退出码 + 会话错误）。
class _TerminalView {
  _TerminalView() : output = const TerminalText(text: '', truncated: false);

  TerminalText output;

  /// truncated 一旦为 true 就闩锁（掐掉的字节回不来，标注不能自己消失）。
  bool truncated = false;
  int? exitCode;
  String sessionError = '';
}

class _TerminalPageState extends State<TerminalPage> {
  late TerminalSessionsSource _sessionsSource;
  late TerminalSessionFactory _sessionFactory;
  _TerminalPhase _phase = _TerminalPhase.loading;
  String _listError = '';
  List<TerminalSessionRow> _sessions = const <TerminalSessionRow>[];

  /// 存活会话（切回列表不 close，PTY 服务端保持运行；dispose 时全关）。
  final Map<String, TerminalSession> _live = <String, TerminalSession>{};
  final Map<String, _TerminalView> _views = <String, _TerminalView>{};

  /// 活跃会话的输出/退出码双订阅（共进退；取消时不 await —— broadcast 订阅的
  /// cancel future 在 widget 测试的 FakeAsync 下不结束（实测），但取消动作
  /// 本身同步生效，close 语义不依赖它完成）。
  final Map<String, List<StreamSubscription<Object?>>> _subs =
      <String, List<StreamSubscription<Object?>>>{};

  String? _activeId;
  bool _attaching = false;
  String _attachError = '';
  late int _cols;
  late int _rows;
  late final TextEditingController _inputCtrl;
  late final ScrollController _scrollCtrl;

  @override
  void initState() {
    super.initState();
    _sessionsSource = widget.sessionsSource ?? BridgeTerminalSessionsSource();
    _sessionFactory =
        widget.sessionFactory ?? ((String id) => BridgeTerminalSession(sessionId: id));
    _cols = widget.initialCols;
    _rows = widget.initialRows;
    _inputCtrl = TextEditingController();
    _scrollCtrl = ScrollController();
    _load();
  }

  @override
  void dispose() {
    for (final List<StreamSubscription<Object?>> subs in _subs.values) {
      for (final StreamSubscription<Object?> sub in subs) {
        // ignore: discarded_futures
        sub.cancel();
      }
    }
    for (final TerminalSession session in _live.values) {
      // ignore: discarded_futures
      session.close();
    }
    _inputCtrl.dispose();
    _scrollCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    if (_phase != _TerminalPhase.loading || _listError.isNotEmpty) {
      setState(() {
        _phase = _TerminalPhase.loading;
        _listError = '';
      });
    }
    try {
      final List<TerminalSessionRow> sessions = await _sessionsSource.list();
      if (!mounted) return;
      setState(() {
        _sessions = sessions;
        _phase = _TerminalPhase.ready;
      });
    } on TerminalLoadFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _phase = _TerminalPhase.error;
        _listError = e.message;
      });
    } catch (e) {
      // 兜底：数据源的意外错误也要落进错误态，不许裸抛给框架。
      if (!mounted) return;
      setState(() {
        _phase = _TerminalPhase.error;
        _listError = '$e';
      });
    }
  }

  _TerminalView _viewOf(String id) =>
      _views.putIfAbsent(id, _TerminalView.new);

  Future<void> _attach(String id) async {
    final bool fresh = !_live.containsKey(id);
    final TerminalSession session =
        _live.putIfAbsent(id, () => _sessionFactory(id));
    setState(() {
      _activeId = id;
      _attaching = fresh;
      _attachError = '';
    });
    if (!fresh) return;
    final _TerminalView view = _viewOf(id);
    final StreamSubscription<List<int>> outSub =
        session.output.listen((List<int> bytes) {
      if (!mounted) return;
      setState(() {
        final TerminalText step = appendTerminalOutput(
          view.output.text,
          utf8.decode(bytes, allowMalformed: true),
        );
        view.output = step;
        view.truncated = view.truncated || step.truncated;
      });
      _scrollToBottom();
    }, onError: (Object e) {
      if (!mounted) return;
      setState(() {
        view.sessionError = _sessionErrorText(e);
      });
    });
    final StreamSubscription<int> exitSub =
        session.exitCodes.listen((int code) {
      if (!mounted) return;
      setState(() {
        view.exitCode = code;
      });
    }, onError: (Object e) {
      if (!mounted) return;
      setState(() {
        view.sessionError = _sessionErrorText(e);
      });
    });
    // 两个订阅共进退（存两份，取消时一起摘）。
    _subs[id] = <StreamSubscription<Object?>>[outSub, exitSub];
    try {
      // open 自带当前尺寸（cols/rows 进 OpenTerminal）；后续尺寸变化走 resize。
      await session.open(cols: _cols, rows: _rows);
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _attachError = terminalAttachErrorText(e);
        _attaching = false;
      });
    } on TerminalAttachFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _attachError = e.message;
        _attaching = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _attachError = '$e';
        _attaching = false;
      });
    }
    // finally 里不 return：Dart 里 finally 中的 return 会吞掉 in-flight 的
    // 异常与返回值（control_flow_in_finally）。mounted 检查在这里做——
    // 异常分支里已各自 return，未走到这里的都是正常路径。
    if (!mounted) return;
    setState(() {
      _attaching = false;
    });
  }

  String _sessionErrorText(Object e) {
    if (e is TerminalStreamFailure) return e.message;
    if (e is PaBridgeException) return terminalAttachErrorText(e);
    return '$e';
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollCtrl.hasClients) return;
      _scrollCtrl.jumpTo(_scrollCtrl.position.maxScrollExtent);
    });
  }

  void _toList() {
    // 切回列表不断流：订阅保持，输出继续在后台累积（截断上限内）；
    // PTY 无回放，晚订会丢帧 —— 不取消订阅就不丢。
    setState(() {
      _activeId = null;
      _attachError = '';
    });
  }

  Future<void> _closeActive() async {
    final String? id = _activeId;
    if (id == null) return;
    final TerminalSession? session = _live.remove(id);
    _views.remove(id);
    final List<StreamSubscription<Object?>>? subs = _subs.remove(id);
    if (subs != null) {
      for (final StreamSubscription<Object?> sub in subs) {
        // ignore: discarded_futures
        sub.cancel();
      }
    }
    try {
      await session?.close();
    } on PaBridgeException catch (e) {
      // 本地视图已摘除，关闭失败只记一条 attach 语境的人话（不挡回列表）。
      if (!mounted) return;
      setState(() {
        _activeId = null;
        _attachError = terminalAttachErrorText(e);
      });
      await _load();
      return;
    }
    if (!mounted) return;
    setState(() {
      _activeId = null;
      _attachError = '';
    });
    await _load();
  }

  void _send() {
    final String? id = _activeId;
    final TerminalSession? session = id == null ? null : _live[id];
    if (session == null) return;
    final String text = _inputCtrl.text;
    if (text.isEmpty) return;
    // 行输入：原文 + `\n`（回车也是一个字节；TerminalSession.sendInput
    // 只收原始字节，不偷渡「发送」语义）。
    session.sendInput(utf8.encode('$text\n'));
    _inputCtrl.clear();
  }

  void _changeSize({int dCols = 0, int dRows = 0}) {
    final String? id = _activeId;
    final TerminalSession? session = id == null ? null : _live[id];
    final int nextCols = (_cols + dCols).clamp(1, 1000000);
    final int nextRows = (_rows + dRows).clamp(1, 1000000);
    if (nextCols == _cols && nextRows == _rows) return;
    setState(() {
      _cols = nextCols;
      _rows = nextRows;
    });
    // 尺寸变化即发 resize（PTY 行列同步；open 时已带初始尺寸）。
    session?.resize(cols: _cols, rows: _rows);
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String? activeId = _activeId;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaSubPageHeader(
          title: '终端',
          backKey: 'terminal-back',
          onBack: activeId == null ? widget.onBack : _toList,
        ),
        if (activeId != null)
          _buildSessionView(context, pa, activeId)
        else
          ..._buildListBody(context, pa),
      ],
    );
  }

  List<Widget> _buildListBody(BuildContext context, PAColors pa) {
    switch (_phase) {
      case _TerminalPhase.loading:
        return <Widget>[
          Padding(
            padding: const EdgeInsets.only(top: 60),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  SizedBox(
                    width: 26,
                    height: 26,
                    child: CircularProgressIndicator(
                        strokeWidth: 2.5, color: pa.brand),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    '正在加载终端会话…',
                    style: PATypography.caption.copyWith(color: pa.fgSecondary),
                  ),
                ],
              ),
            ),
          ),
        ];
      case _TerminalPhase.error:
        return <Widget>[
          Padding(
            padding: const EdgeInsets.only(top: 48),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Icon(Icons.error_outline_rounded,
                      size: 30, color: pa.danger),
                  const SizedBox(height: 10),
                  Text(
                    '终端会话列表加载失败',
                    style: PATypography.body.copyWith(
                      color: pa.fgPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (_listError.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 6),
                    Text(
                      _listError,
                      key: const ValueKey<String>('terminal-list-error'),
                      textAlign: TextAlign.center,
                      style:
                          PATypography.caption.copyWith(color: pa.fgTertiary),
                    ),
                  ],
                  const SizedBox(height: 14),
                  PAButton(
                    key: const ValueKey<String>('terminal-retry'),
                    label: '重试',
                    expanded: false,
                    onPressed: _load,
                  ),
                ],
              ),
            ),
          ),
        ];
      case _TerminalPhase.ready:
        if (_sessions.isEmpty) {
          // 空态 = 通道真读了、daemon 真回了空列表（真「还没有会话」）；
          // 通道没就绪/读失败走上面的错误态，绝不会落到这里（两条路径分家）。
          return <Widget>[
            Padding(
              padding: const EdgeInsets.only(top: 48),
              child: Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Text(
                      '还没有终端会话',
                      key: const ValueKey<String>('terminal-empty'),
                      style: PATypography.body.copyWith(
                        color: pa.fgPrimary,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 6),
                    Text(
                      '去电脑上开一个终端，会话会出现在这里',
                      textAlign: TextAlign.center,
                      style:
                          PATypography.caption.copyWith(color: pa.fgTertiary),
                    ),
                  ],
                ),
              ),
            ),
          ];
        }
        return <Widget>[
          for (final TerminalSessionRow row in _sessions)
            Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: _TerminalRow(
                key: ValueKey<String>('terminal-row-${row.id}'),
                row: row,
                onTap: () => _attach(row.id),
              ),
            ),
        ];
    }
  }

  Widget _buildSessionView(BuildContext context, PAColors pa, String id) {
    final _TerminalView view = _viewOf(id);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        Row(
          children: <Widget>[
            TextButton.icon(
              key: const ValueKey<String>('terminal-to-list'),
              onPressed: _toList,
              icon: Icon(Icons.arrow_back_rounded,
                  size: 16, color: pa.fgSecondary),
              label: Text(
                '会话列表',
                style: PATypography.caption.copyWith(
                  color: pa.fgSecondary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            const Spacer(),
            TextButton(
              key: const ValueKey<String>('terminal-close'),
              onPressed: _closeActive,
              child: Text(
                '关闭',
                style: PATypography.caption.copyWith(
                  color: pa.danger,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ],
        ),
        if (_attaching)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 24),
            child: Center(
              child: Text(
                '正在连接终端…',
                style: PATypography.caption.copyWith(color: pa.fgSecondary),
              ),
            ),
          ),
        if (_attachError.isNotEmpty)
          PACard(
            padding:
                const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            child: Text(
              _attachError,
              key: const ValueKey<String>('terminal-attach-error'),
              style: PATypography.caption.copyWith(color: pa.danger),
            ),
          ),
        if (view.sessionError.isNotEmpty)
          PACard(
            padding:
                const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            child: Text(
              view.sessionError,
              key: const ValueKey<String>('terminal-session-error'),
              style: PATypography.caption.copyWith(color: pa.danger),
            ),
          ),
        PACard(
          color: pa.bgCode,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              if (view.truncated) ...<Widget>[
                Text(
                  terminalTruncationNotice(),
                  key: const ValueKey<String>('terminal-truncated'),
                  style: PATypography.caption.copyWith(color: pa.warn),
                ),
                const SizedBox(height: 8),
              ],
              if (view.output.text.isEmpty)
                Text(
                  '等待终端输出…',
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                )
              else
                SizedBox(
                  height: 320,
                  child: SingleChildScrollView(
                    controller: _scrollCtrl,
                    child: SelectableText(
                      view.output.text,
                      key: const ValueKey<String>('terminal-output'),
                      style: PATypography.code.copyWith(color: pa.fgPrimary),
                    ),
                  ),
                ),
              if (view.exitCode != null) ...<Widget>[
                const SizedBox(height: 8),
                Text(
                  terminalExitText(view.exitCode!),
                  key: const ValueKey<String>('terminal-exit'),
                  style: PATypography.caption.copyWith(color: pa.fgSecondary),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 10),
        Row(
          children: <Widget>[
            Expanded(
              child: TextField(
                key: const ValueKey<String>('terminal-input'),
                controller: _inputCtrl,
                style: PATypography.body.copyWith(color: pa.fgPrimary),
                onSubmitted: (_) => _send(),
                decoration: InputDecoration(
                  isDense: true,
                  hintText: '输入命令后发送（回车执行）…',
                  hintStyle:
                      PATypography.caption.copyWith(color: pa.fgTertiary),
                  filled: true,
                  fillColor: pa.bgSurface,
                  contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12, vertical: 10),
                  enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(10),
                    borderSide: BorderSide(color: pa.hairline),
                  ),
                  focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(10),
                    borderSide: BorderSide(color: pa.brand, width: 1.5),
                  ),
                ),
              ),
            ),
            const SizedBox(width: 8),
            PAButton(
              key: const ValueKey<String>('terminal-send'),
              label: '发送',
              expanded: false,
              onPressed: _send,
            ),
          ],
        ),
        const SizedBox(height: 8),
        Row(
          children: <Widget>[
            Text(
              '列 $_cols 行 $_rows',
              key: const ValueKey<String>('terminal-size'),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
            const Spacer(),
            IconButton(
              key: const ValueKey<String>('terminal-cols-minus'),
              tooltip: '列数减少',
              onPressed: () => _changeSize(dCols: -10),
              icon: Icon(Icons.remove_rounded,
                  size: 16, color: pa.fgSecondary),
            ),
            IconButton(
              key: const ValueKey<String>('terminal-cols-plus'),
              tooltip: '列数增加',
              onPressed: () => _changeSize(dCols: 10),
              icon:
                  Icon(Icons.add_rounded, size: 16, color: pa.fgSecondary),
            ),
            IconButton(
              key: const ValueKey<String>('terminal-rows-minus'),
              tooltip: '行数减少',
              onPressed: () => _changeSize(dRows: -5),
              icon: Icon(Icons.remove_rounded,
                  size: 16, color: pa.fgSecondary),
            ),
            IconButton(
              key: const ValueKey<String>('terminal-rows-plus'),
              tooltip: '行数增加',
              onPressed: () => _changeSize(dRows: 5),
              icon:
                  Icon(Icons.add_rounded, size: 16, color: pa.fgSecondary),
            ),
          ],
        ),
        Padding(
          padding: const EdgeInsets.only(top: 4),
          child: Text(
            '等宽纯文本渲染：ANSI 转义按原文显示，xterm 着色随后续波次接入。',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ),
      ],
    );
  }
}

/// 会话行：id + 副文案（shell · cwd · 运行中/已退出）+ 存活点。
class _TerminalRow extends StatelessWidget {
  const _TerminalRow({super.key, required this.row, required this.onTap});

  final TerminalSessionRow row;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return PACard(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
      onTap: onTap,
      child: Row(
        children: <Widget>[
          Container(
            width: 10,
            height: 10,
            decoration: BoxDecoration(
              color: row.alive ? pa.ok : pa.fgTertiary,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  row.id.isEmpty ? '(未命名会话)' : row.id,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: PATypography.body.copyWith(
                    color: pa.fgPrimary,
                    fontWeight: FontWeight.w600,
                    fontFamily: PATypography.monoFont,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  terminalSessionSubtitleOf(row),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ],
            ),
          ),
          Icon(
            Icons.chevron_right_rounded,
            size: 16,
            color: pa.fgTertiary,
          ),
        ],
      ),
    );
  }
}
