// 终端页的模型 + 纯函数 + 真通道数据源（B3 第 6 波）。
//
// 【为什么单独一个文件】与 threads_bridge.dart / files_bridge.dart /
// git_bridge.dart 同一分工：①「输出块怎么拼、截到哪、退出码说什么、
// resize 帧怎么封」是纯函数，可被单测逐条钉死；②「真通道怎么包」=
// [BridgeTerminalSessionsSource]（列表）+ [BridgeTerminalSession]（会话）。
// terminal.dart 只留界面（三态 + 输出块 + 输入行）。三层各自可测，回归时
// 分得清是拼接错、通道错还是渲染错。
//
// 【诚实红线（B4 前）】Kotlin 宿主桥 B4 波次才注册 `dev.pocketagent/bridge`
// 与 `dev.pocketagent/bridge/terminal_events`；在此之前 listTerminals /
// terminalAttach 必然抛 HOST_UNAVAILABLE，如实翻成人话错误，页面进错误态。
// **绝不返回空列表冒充「没有会话」**：空态与错误态是两条路径，测试各钉一条。
//
// 【xterm 不做】无 xterm 依赖，不自研终端仿真器；输出块按纯文本块渲染
// （可滚动、等宽字体、增量拼接、超长截断 + 诚实「已截断」标注），原文透传，
// 不摆假光标、不摆假 shell 提示符。WebView/xterm 桥留后续波次。

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import '../gen/pocketagent/v1/terminal.pb.dart';
import '../transport/pa_bridge.dart';
import '../transport/terminal_stream.dart';

// ── 会话抽象 ──────────────────────────────────────────────────────────────
// （从 terminal.dart 迁入：页面与数据源共用同一形状，避免 pages ↔ data
// 循环 import；terminal.dart 按名重导出，旧 import 路径继续有效。）

/// 终端 PTY 会话抽象（daemon `TerminalService.Attach` 双向流的 Dart 侧形状）。
///
/// 帧对应（proto/v1/terminal.proto）：
///  · `open` → `OpenTerminal`（session_id / cwd / shell / cols / rows）；
///    sessionId 由实现方在构造时持有（Kotlin `UUID`，TerminalScreen.kt:473，
///    取前 8 位）；幂等重连时复用同一个 id（服务端回 `Resumed` 帧）；
///  · [sendInput] → `TerminalInput`（原始击键字节，UTF-8/ANSI 不转义）；
///  · [resize] → `TerminalResize`（窗口尺寸联动）；
///  · [output] → `TerminalOutput`（原始 PTY 输出字节，含 ANSI，由纯文本块
///    原文透传渲染 —— 无 xterm，不做转义解析）；
///  · [exitCodes] → `TerminalExit.code`（正常只发一次；`TerminalError` 转为
///    流错误 [TerminalStreamFailure]）。
///
/// 生命周期：`open` → `output` 订阅 → … → [close]（对位 `closeTerminal`）。
/// 页面切回列表时不 close（PTY 服务端保持运行）；dispose 时必须 `close`
/// （Kotlin `DisposableEffect` 关流同款纪律）。
abstract class TerminalSession {
  /// 客户端持有的会话 id（幂等重连用，对位 proto `session_id`）。
  String get sessionId;

  /// PTY 原始输出字节流（含 ANSI，纯文本块原文透传）。
  Stream<List<int>> get output;

  /// 退出码流（正常只发一次）。
  Stream<int> get exitCodes;

  /// 打开 PTY（cwd 空 = daemon 默认目录；shell 空 = 主机默认 shell）。
  Future<void> open({String? cwd, String? shell, int cols = 80, int rows = 24});

  /// 发送原始击键字节（UTF-8/ANSI，不转义）。
  void sendInput(List<int> data);

  /// 终端尺寸变化后调用（PTY 行列同步）。
  void resize({required int cols, required int rows});

  /// 关闭会话（对位 `closeTerminal` + 关流）。
  Future<void> close();
}

/// 会话工厂 —— 页面只认这一个口子（测试注入假会话，生产走真实现）。
typedef TerminalSessionFactory = TerminalSession Function(String sessionId);

// ── 输出增量拼接 / 截断 ───────────────────────────────────────────────────

/// 输出块行数上限：超出即掐头，只保留最后这么多行。
///
/// 【为什么是 500】手机屏一次能看的也就几十行；500 行 ≈ 一次中等构建日志，
//  再多纯文本渲染也开始卡，且截断标注会如实告诉用户掐掉了多少 —— 上限本身
/// 不是精确科学，是「卡顿」与「可回看」之间的诚实折中，有标注就不怕选错。
const int kTerminalMaxLines = 500;

/// 纯文本输出（含已拼接的全部行）+ 本次拼接是否掐过头。
class TerminalText {
  const TerminalText({required this.text, required this.truncated});

  /// 已拼接的全部正文（被截断时 = 保留下来的尾部）。
  final String text;

  /// 本次拼接是否掐掉了头部（页面据此展示截断标注；一旦为 true 就闩锁，
  /// 后续不再回落 —— 掐掉的字节回不来，标注不能自己消失）。
  final bool truncated;

  @override
  bool operator ==(Object other) =>
      other is TerminalText &&
      other.text == text &&
      other.truncated == truncated;

  @override
  int get hashCode => Object.hash(text, truncated);
}

/// 增量拼接（纯函数）：`current + chunk`，超 [maxLines] 行即掐头留尾。
///
/// 【为什么按行掐、不按字节掐】PTY 输出是行流（命令输出一行一个结果），按行
/// 掐保证留下的是「完整的最后 N 行」而不是「半行残字节」；ANSI 转义被拦腰截断
/// 的风险也随整行保留而降低（纯文本渲染本来就不解析转义，原文透传）。
TerminalText appendTerminalOutput(
  String current,
  String chunk, {
  int maxLines = kTerminalMaxLines,
}) {
  final String joined = '$current$chunk';
  final List<String> lines = joined.split('\n');
  if (lines.length <= maxLines) {
    return TerminalText(text: joined, truncated: false);
  }
  return TerminalText(
    text: lines.sublist(lines.length - maxLines).join('\n'),
    truncated: true,
  );
}

/// 截断标注文案（纯函数）：页面在 `truncated` 闩锁后展示这一条。
String terminalTruncationNotice({int maxLines = kTerminalMaxLines}) =>
    '—— 输出过长，已截断，仅保留最后 $maxLines 行 ——';

// ── 退出码文案 ────────────────────────────────────────────────────────────

/// 退出码 → 人话（纯函数）：0 也是事实（正常退出），不报喜也不报忧；
/// 非 0 只陈述码值，不猜原因（原因在输出里，不在这里编）。
String terminalExitText(int code) {
  if (code == 0) return '终端已正常退出（退出码 0）';
  return '终端已退出（退出码 $code）';
}

// ── 帧封帧（调用方按 proto 封帧，供 terminalWrite 用）──────────────────────

/// open 帧封帧：`TerminalClientMessage{open}` 的 wire 字节。
///
/// 空 cwd/shell = 沿用 daemon 默认（与 proto 注释同义）；cols/rows 默认
/// 80×24（对位 TerminalScreen.kt:475-476 的 `cols = 80` / `rows = 24`）。
Uint8List terminalOpenFrame({
  required String sessionId,
  String cwd = '',
  String shell = '',
  int cols = 80,
  int rows = 24,
}) =>
    TerminalClientMessage(
      open: OpenTerminal(
        sessionId: sessionId,
        cwd: cwd,
        shell: shell,
        cols: cols,
        rows: rows,
      ),
    ).writeToBuffer();

/// input 帧封帧：`TerminalClientMessage{input}` 的 wire 字节（原始击键字节，
//  不转义；行输入由调用方自行补 `\n` 表示回车 —— 回车也是一个字节，不偷渡语义）。
Uint8List terminalInputFrame(List<int> data) => TerminalClientMessage(
      input: TerminalInput(data: data),
    ).writeToBuffer();

/// input 帧封帧（字符串便捷版）：按 UTF-8 编码后与 [terminalInputFrame] 同形.
Uint8List terminalInputFrameFromString(String text) =>
    terminalInputFrame(utf8.encode(text));

/// resize 帧封帧：`TerminalClientMessage{resize}` 的 wire 字节（尺寸协商）。
Uint8List terminalResizeFrame({required int cols, required int rows}) =>
    TerminalClientMessage(
      resize: TerminalResize(cols: cols, rows: rows),
    ).writeToBuffer();

// ── 会话行模型 ────────────────────────────────────────────────────────────

/// 会话列表行 —— proto `TerminalSessionInfo` 的 Dart 投影（四字段）。
class TerminalSessionRow {
  const TerminalSessionRow({
    required this.id,
    required this.shell,
    required this.cwd,
    required this.alive,
  });

  /// 会话 id（对位 proto `session_id`，attach/close 按它定位）。
  final String id;

  final String shell;
  final String cwd;

  /// 服务端存活标记（daemon 真值，原文透传，不做客户端二次派生）。
  final bool alive;

  @override
  bool operator ==(Object other) =>
      other is TerminalSessionRow &&
      other.id == id &&
      other.shell == shell &&
      other.cwd == cwd &&
      other.alive == alive;

  @override
  int get hashCode => Object.hash(id, shell, cwd, alive);
}

/// proto `TerminalSessionInfo` → [TerminalSessionRow]（逐字段对应，无加工；
/// int64 经 toInt 收口；顺序保持 daemon 原序，UI 不重排）。
TerminalSessionRow terminalSessionRowOf(TerminalSessionInfo s) =>
    TerminalSessionRow(
      id: s.sessionId,
      shell: s.shell,
      cwd: s.cwd,
      alive: s.alive,
    );

/// 行副文案（纯函数）：shell/cwd 有什么说什么，都没有就不编；
/// 存活态恒为最后一节（`运行中` / `已退出` —— daemon `alive` 的原文翻译）。
String terminalSessionSubtitleOf(TerminalSessionRow row) {
  final List<String> parts = <String>[];
  if (row.shell.trim().isNotEmpty) parts.add(row.shell);
  if (row.cwd.trim().isNotEmpty) parts.add(row.cwd);
  parts.add(row.alive ? '运行中' : '已退出');
  return parts.join(' · ');
}

// ── 错误文案 ──────────────────────────────────────────────────────────────

/// 错误码 → 页面人话文案（页面错误态直接展示这一条）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名「终端通道未就绪
/// （B4 波次）」。其余码原样透传宿主 message（不改写、不吞 —— 与
/// settings/threads/files/git/collab 页同一套透传纪律）；宿主没给 message
/// 时退 code，宁可难看也不吞。
String terminalLoadErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端终端通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '终端会话列表暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

/// attach/write 路径失败文案：与列表同款的 HOST_UNAVAILABLE 定制 + 透传，
/// 收尾按连接语境改词。
String terminalAttachErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端终端通道未就绪（B4 波次）：终端连接暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

// ── 失败类型（页面据此进各自的错误态）──────────────────────────────────────

/// 会话列表加载失败：只带人话原因，不吞原始错误类型。
class TerminalLoadFailure implements Exception {
  const TerminalLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'TerminalLoadFailure: $message';
}

/// 会话连接失败（attach/open 路径）：与列表失败分开，页面能分别指路。
class TerminalAttachFailure implements Exception {
  const TerminalAttachFailure(this.message);

  final String message;

  @override
  String toString() => 'TerminalAttachFailure: $message';
}

/// 流内服务端错误：daemon 经 `TerminalError` 帧上报的会话错误（转成
/// `output` 流的错误事件，不混进 PTY 字节 —— 错误字样不冒充终端输出）。
class TerminalStreamFailure implements Exception {
  const TerminalStreamFailure(this.message);

  final String message;

  @override
  String toString() => 'TerminalStreamFailure: $message';
}

// ── 数据源 ────────────────────────────────────────────────────────────────

/// 会话列表数据源抽象 —— 页面只认这一个口子（测试注入可控源，生产走真通道）。
abstract class TerminalSessionsSource {
  /// 拉一次会话列表；失败以 [TerminalLoadFailure] 上抛（页面据此进错误态）。
  Future<List<TerminalSessionRow>> list();
}

/// 真通道列表数据源：`TerminalStreamBridge.listTerminals()` →
/// `ListTerminalsResponse` 字节已在 transport 层验形 —— 这里只做：
/// 错误 → [TerminalLoadFailure] 人话；成功 → 行模型。
///
/// [bridge] 是注入点：测试按通道名 mock 宿主 wire 行为（bridge 全程真实）。
class BridgeTerminalSessionsSource implements TerminalSessionsSource {
  BridgeTerminalSessionsSource({TerminalStreamBridge? bridge})
      : _bridge = bridge ?? TerminalStreamBridge();

  final TerminalStreamBridge _bridge;

  @override
  Future<List<TerminalSessionRow>> list() async {
    final ListTerminalsResponse response;
    try {
      response = await _bridge.listTerminals();
    } on PaBridgeException catch (e) {
      // **不在这里回退成空列表**：那会把错误粉饰成「没有会话」（假空态），
      // 空态与错误态两条路径从此再也分不开（红线：不留假绿）。
      throw TerminalLoadFailure(terminalLoadErrorText(e));
    }
    return response.sessions
        .map(terminalSessionRowOf)
        .toList(growable: false);
  }
}

/// 真通道会话：[TerminalSession] 的 transport 实现（一层薄包，无渲染逻辑）。
///
/// open 序列 = `terminalAttach(id)`（订阅 Attach 流）→ 订阅事件帧 →
/// `terminalWrite(open 帧)`（cwd/shell/cols/rows 进 `OpenTerminal`，复用存活
/// 会话时服务端回 `Resumed` 帧）；input/resize 同样经 `terminalWrite` 送帧。
///
/// 【单活跃 attach 假设】`TerminalOutput` 帧不带 session_id（proto 原样），
//  多会话复用时输出帧无法归属 —— 移动端一次只看一个终端，输出帧恒归当前
/// attach 的会话；切回列表不 close（PTY 服务端保持运行，PTY 本来就无回放，
/// terminal.proto:69），重进复用同一对象续订。
class BridgeTerminalSession implements TerminalSession {
  // 初始化形参 `this._sessionId` 会把公开参数名改成私有的 `_sessionId`，
  // 三处调用点（sessionId:）全破；保持显式初始化列表。
  BridgeTerminalSession({
    required String sessionId,
    TerminalStreamBridge? bridge,
  })  : _sessionId = sessionId, // ignore: prefer_initializing_formals
        _bridge = bridge ?? TerminalStreamBridge();

  final String _sessionId;
  final TerminalStreamBridge _bridge;

  final StreamController<List<int>> _out =
      StreamController<List<int>>.broadcast();
  final StreamController<int> _exit = StreamController<int>.broadcast();
  StreamSubscription<TerminalServerMessage>? _sub;
  bool _attached = false;
  bool _closed = false;

  @override
  String get sessionId => _sessionId;

  @override
  Stream<List<int>> get output => _out.stream;

  @override
  Stream<int> get exitCodes => _exit.stream;

  @override
  Future<void> open({
    String? cwd,
    String? shell,
    int cols = 80,
    int rows = 24,
  }) async {
    if (_closed) {
      throw StateError('终端会话已关闭，不能重开（另建一个会话对象）');
    }
    // 先续订事件、再 attach、最后写 open 帧：opened/resumed/首波输出都可能
    // 在 attach 后立刻到达，晚订就丢（broadcast 无缓冲，PTY 无回放）。
    await _dropSubscription();
    _sub = _bridge.events.listen(
      _onFrame,
      onError: (Object e, StackTrace s) => _out.addError(e, s),
    );
    try {
      await _bridge.terminalAttach(_sessionId);
      await _bridge.terminalWrite(
        terminalOpenFrame(
          sessionId: _sessionId,
          cwd: cwd ?? '',
          shell: shell ?? '',
          cols: cols,
          rows: rows,
        ),
      );
    } catch (_) {
      await _dropSubscription();
      rethrow;
    }
    _attached = true;
  }

  @override
  void sendInput(List<int> data) {
    _requireAttached('sendInput');
    _bridge.terminalWrite(terminalInputFrame(data)).then(
      (_) {},
      onError: (Object e, StackTrace s) => _out.addError(e, s),
    );
  }

  @override
  void resize({required int cols, required int rows}) {
    _requireAttached('resize');
    _bridge.terminalWrite(
      terminalResizeFrame(cols: cols, rows: rows),
    ).then(
      (_) {},
      onError: (Object e, StackTrace s) => _out.addError(e, s),
    );
  }

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    _attached = false;
    await _dropSubscription();
    try {
      await _bridge.terminalClose(_sessionId);
    } finally {
      await _out.close();
      await _exit.close();
    }
  }

  void _requireAttached(String what) {
    if (_closed || !_attached) {
      throw StateError('终端尚未连接（先 open），$what 无处可去');
    }
  }

  Future<void> _dropSubscription() async {
    await _sub?.cancel();
    _sub = null;
  }

  void _onFrame(TerminalServerMessage message) {
    switch (message.whichMsg()) {
      case TerminalServerMessage_Msg.output:
        _out.add(message.output.data);
      case TerminalServerMessage_Msg.exit:
        _exit.add(message.exit.code);
      case TerminalServerMessage_Msg.error:
        // 服务端错误走流错误，不混进 PTY 字节（见 [TerminalStreamFailure]）。
        _out.addError(TerminalStreamFailure(message.error.message));
      case TerminalServerMessage_Msg.opened:
      case TerminalServerMessage_Msg.resumed:
      case TerminalServerMessage_Msg.notSet:
        break;
    }
  }
}
