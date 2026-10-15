// Chat 历史列表页（B3 第 5 波子步②）：加载态 / 错误态 / 空态 + 简化消息行 +
// 「恢复到会话」回放入口 + 离线快照兜底。
//
// 【数据源】默认真通道 [BridgeChatHistorySource]（`getThreadHistory`，
// 宿主 B4 前如实进「通道未就绪」错误态）；测试注入 [FakeChatHistorySource]。
// 成功拉取后经 [ChatSnapshotStore] 整体覆盖落快照；拉取失败时有快照渲染快照
// （离线徽标），无快照才进错误态——空态「还没有消息」只属于「真读了、真没有」。
//
// 【行渲染为什么是简化行】C1 的 timeline tile 归 C1（`chat_timeline.dart`
// 在建中，本文件绝不碰）：这里只画文本 + 时间 + 角色三件套；
// C4/回填波次统一 tile 时再收敛，本行注释即交接点。
//
// 【回放接线】「恢复到会话」按钮调注入的 [onReplay]（`List<TurnServerMessage>`
// 按序喂入）。C1 落盘前调用方注入 fake（测试钉行为）；C1 的
// `TurnSession.replayFrames(List<TurnServerMessage>)` 落盘后，调用方把
// `session.replayFrames` 直接作为 onReplay 传入即可（签名对齐；若 C1 定为
// 同步 void，包一层 async 闭包）。幂等（同 sequence 不重复进）由 C1 保证，
// 本页只负责「按序原样转发」，不去重、不断言重复帧。
//
// 【sidecar 去哪了】`historyFrameMetas`（role/时间/稳定身份）是纯函数，
// 由回放消费方需要时用同一份响应重算——页面只转发 `frames`（onReplay 签名
// 即 C1 `replayFrames` 签名），不在 state 里留一份没人读的副本。

import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import 'chat_history_data.dart';
import 'chat_snapshot_store.dart';
import 'pa_page_kit.dart';
import 'threads_bridge.dart' show threadRowTimeLabel;

/// 回放回调：把历史帧按序喂进 live 时间线（C1 `replayFrames` 的调用方形态）。
typedef ChatReplayCallback = Future<void> Function(
    List<TurnServerMessage> frames);

/// 会话历史列表：threadId 维度的只读历史 + 恢复入口。
class ChatHistoryPage extends StatefulWidget {
  const ChatHistoryPage({
    super.key,
    required this.threadId,
    this.source,
    this.snapshotStore,
    this.onReplay,
    this.nowMs,
  });

  /// 会话 id（`chat/{threadId}` 的 threadId）。
  final String threadId;

  /// 数据源注入点：测试传可控 fake；生产不传即 [BridgeChatHistorySource]。
  final ChatHistorySource? source;

  /// 快照 store 注入点：测试传内存实现 + 固定时钟；生产默认内存实现。
  final ChatSnapshotStore? snapshotStore;

  /// 「恢复到会话」回调（C1 未落盘时注入 fake；落盘后传
  /// `session.replayFrames`）。null = 按钮禁用（不摆点了没反应的入口）。
  final ChatReplayCallback? onReplay;

  /// 时钟注入点（行时间标签是相对时钟的格式化结果；测试钉死时间用）。
  final int Function()? nowMs;

  @override
  State<ChatHistoryPage> createState() => _ChatHistoryPageState();
}

/// 三态：加载中 / 读失败 / 已就绪（与 Threads 页同款共享词汇 [PaAsyncPhase]）。
typedef _HistoryPhase = PaAsyncPhase;

class _ChatHistoryPageState extends State<ChatHistoryPage> {
  late ChatHistorySource _source;
  late ChatSnapshotStore _store;
  _HistoryPhase _phase = _HistoryPhase.loading;
  String _errorMessage = '';
  List<ThreadMessage> _messages = const <ThreadMessage>[];
  List<TurnServerMessage> _frames = const <TurnServerMessage>[];
  bool _fromSnapshot = false;
  int _snapshotAtMs = 0;
  bool _replaying = false;

  int get _nowMs => (widget.nowMs ?? _systemNowMs)();

  static int _systemNowMs() => DateTime.now().millisecondsSinceEpoch;

  @override
  void initState() {
    super.initState();
    _source = widget.source ?? BridgeChatHistorySource();
    _store = widget.snapshotStore ?? ChatSnapshotStore();
    _load();
  }

  Future<void> _load() async {
    if (_phase != _HistoryPhase.loading || _errorMessage.isNotEmpty) {
      setState(() {
        _phase = _HistoryPhase.loading;
        _errorMessage = '';
      });
    }
    GetThreadHistoryResponse response;
    try {
      response = await _source.loadHistory(widget.threadId);
    } on ChatHistoryLoadFailure catch (e) {
      if (!mounted) return;
      // 拉取失败 → 快照兜底：有有效快照渲染快照（离线徽标），无快照才进错误态。
      final ChatSnapshot? snapshot =
          await _store.loadSnapshot(widget.threadId);
      if (!mounted) return;
      if (snapshot != null && snapshot.messages.isNotEmpty) {
        final GetThreadHistoryResponse shadow = snapshot.toHistoryResponse();
        setState(() {
          _messages = shadow.messages.toList(growable: false);
          _frames = historyToFrames(shadow);
          _fromSnapshot = true;
          _snapshotAtMs = snapshot.savedAtMs;
          _phase = _HistoryPhase.ready;
        });
        return;
      }
      // 空快照（历史真空过、或根本没快照）+ 拉取失败 = 错误态：
      // 此时画空态会把「读失败」伪装成「没有历史」（假空态）。
      setState(() {
        _phase = _HistoryPhase.error;
        _errorMessage = e.message;
      });
      return;
    }
    if (!mounted) return;
    // 成功路径：先落快照（整体覆盖；水位 0 = 未知，见 isSnapshotStale 注释），
    // 再进就绪——落盘失败不挡渲染（只读副本丢了下次再写）。
    try {
      await _store.saveSnapshot(
        threadId: widget.threadId,
        messages: response.messages,
        sequence: 0,
      );
    } catch (_) {
      // 快照是只读副本：写失败不进错误态，下次成功拉取再覆写。
    }
    if (!mounted) return;
    setState(() {
      _messages = response.messages.toList(growable: false);
      _frames = historyToFrames(response);
      _fromSnapshot = false;
      _snapshotAtMs = 0;
      _phase = _HistoryPhase.ready;
    });
  }

  Future<void> _replay() async {
    final ChatReplayCallback? replay = widget.onReplay;
    if (replay == null || _replaying || _frames.isEmpty) return;
    setState(() => _replaying = true);
    try {
      // 按序原样转发：去重与幂等归 C1 `replayFrames`，本页不碰帧内容。
      await replay(List<TurnServerMessage>.unmodifiable(_frames));
    } finally {
      if (mounted) setState(() => _replaying = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
      children: <Widget>[
        ..._buildPhaseBody(context, pa),
      ],
    );
  }

  List<Widget> _buildPhaseBody(BuildContext context, PAColors pa) {
    switch (_phase) {
      case _HistoryPhase.loading:
        return <Widget>[
          Padding(
            key: const ValueKey<String>('chat-history-loading'),
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
                    '正在加载历史…',
                    style: PATypography.caption.copyWith(color: pa.fgSecondary),
                  ),
                ],
              ),
            ),
          ),
        ];
      case _HistoryPhase.error:
        return <Widget>[
          Padding(
            key: const ValueKey<String>('chat-history-error'),
            padding: const EdgeInsets.only(top: 48),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Icon(Icons.error_outline_rounded, size: 30, color: pa.danger),
                  const SizedBox(height: 10),
                  Text(
                    '会话历史加载失败',
                    style: PATypography.body.copyWith(
                      color: pa.fgPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (_errorMessage.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 6),
                    Text(
                      _errorMessage,
                      textAlign: TextAlign.center,
                      style:
                          PATypography.caption.copyWith(color: pa.fgTertiary),
                    ),
                  ],
                  const SizedBox(height: 14),
                  PAButton(
                    key: const ValueKey<String>('chat-history-retry'),
                    label: '重试',
                    expanded: false,
                    onPressed: _load,
                  ),
                ],
              ),
            ),
          ),
        ];
      case _HistoryPhase.ready:
        return _buildReadyBody(context, pa);
    }
  }

  List<Widget> _buildReadyBody(BuildContext context, PAColors pa) {
    if (_messages.isEmpty) {
      // 空态只属于「真读了、真没有」（成功路径零消息）；失败路径永远到不了
      // 这里（有快照走快照，无快照走错误态）——空态与错误态严格分家。
      return <Widget>[
        Padding(
          key: const ValueKey<String>('chat-history-empty'),
          padding: const EdgeInsets.only(top: 48),
          child: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Text(
                  '还没有消息',
                  style: PATypography.body.copyWith(
                    color: pa.fgPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  '这个会话还没有任何消息，去发第一条吧',
                  textAlign: TextAlign.center,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ],
            ),
          ),
        ),
      ];
    }
    final int now = _nowMs;
    return <Widget>[
      if (_fromSnapshot)
        Padding(
          key: const ValueKey<String>('chat-history-offline-badge'),
          padding: const EdgeInsets.only(bottom: 12),
          child: PACard(
            padding:
                const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            child: Row(
              children: <Widget>[
                Icon(Icons.cloud_off_outlined, size: 16, color: pa.fgTertiary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    // 离线快照必须自报家门：用户要分得清看的是副本还是真数据。
                    '离线快照${_snapshotAtMs > 0 ? ' · ${threadRowTimeLabel(_snapshotAtMs, now)}' : ''}：连接恢复后自动刷新',
                    style: PATypography.caption
                        .copyWith(color: pa.fgSecondary),
                  ),
                ),
                const SizedBox(width: 8),
                GestureDetector(
                  key: const ValueKey<String>('chat-history-retry'),
                  onTap: _load,
                  child: Text(
                    '重试',
                    style: PATypography.caption.copyWith(
                      color: pa.brand,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      KeyedSubtree(
        key: const ValueKey<String>('chat-history-list'),
        child: Column(
          children: <Widget>[
            for (int i = 0; i < _messages.length; i++)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: _HistoryRow(
                  key: ValueKey<String>('chat-history-row-$i'),
                  message: _messages[i],
                  nowMs: now,
                ),
              ),
          ],
        ),
      ),
      const SizedBox(height: 4),
      PAButton(
        key: const ValueKey<String>('chat-history-restore'),
        label: _replaying ? '正在恢复…' : '恢复到会话',
        expanded: true,
        // 无回调或无帧时禁用：不摆点了没反应的入口；回放中防重复提交。
        onPressed: (widget.onReplay == null || _frames.isEmpty || _replaying)
            ? null
            : _replay,
      ),
    ];
  }
}

/// 简化消息行：角色 + 时间 + 纯文本（不复用 C1 timeline tile——tile 归 C1，
/// C4/回填波次统一 tile 时收敛到同一组件，本行即交接点）。
class _HistoryRow extends StatelessWidget {
  const _HistoryRow({super.key, required this.message, required this.nowMs});

  final ThreadMessage message;
  final int nowMs;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String text = historyTextOf(message);
    final String time = message.createdAtUnixMs.toInt() > 0
        ? threadRowTimeLabel(message.createdAtUnixMs.toInt(), nowMs)
        : '';
    return PACard(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Text(
                historyRoleLabel(message.role),
                style: PATypography.caption.copyWith(
                  color: pa.agentAccent,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(width: 8),
              if (time.isNotEmpty)
                Text(
                  time,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
            ],
          ),
          if (text.isNotEmpty) ...<Widget>[
            const SizedBox(height: 6),
            Text(
              text,
              style: PATypography.body.copyWith(color: pa.fgPrimary),
            ),
          ],
        ],
      ),
    );
  }
}
