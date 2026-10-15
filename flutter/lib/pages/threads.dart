import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../components/pa_status_chip.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'pa_page_kit.dart';
import 'threads_bridge.dart';
import 'threads_data.dart';

/// 会话列表：agent 圆角矩形头像 + 标题 + 摘要 + 状态 chip + 时间。
///
/// B3 第 3 步：数据源默认走真通道 —— [BridgeThreadsDataSource]（PaBridge
/// `listThreads` → proto `Thread` 六字段映射，见 threads_bridge.dart）。宿主桥
/// 未注册（B4 波次前）时如实进错误态「桌面端会话通道未就绪（B4 波次）」，
/// **不伪装成空列表**——空态「还没有会话」与错误态是两条路径（分别有测试钉死）。
/// 内存 fake（[FakeThreadsDataSource]）保留给测试注入；三维筛选为本地纯函数。
class ThreadsPage extends StatefulWidget {
  const ThreadsPage({super.key, this.source});

  /// 数据源注入点：测试传可控 fake / 带 mock 通道的桥数据源；生产不传即
  /// [BridgeThreadsDataSource]（真走 `dev.pocketagent/bridge` 方法面）。
  final ThreadsDataSource? source;

  @override
  State<ThreadsPage> createState() => _ThreadsPageState();
}

/// 三态：加载中 / 加载失败 / 已就绪（就绪后再分「空列表」与「被筛空」）。
///
/// 与 settings_capabilities.dart 的 `_ModelsPhase` 同名同义同序 —— 收敛为
/// pa_page_kit.dart 的共享词汇 [PaAsyncPhase]（typedef 零运行时开销、零渲染，
/// 改名不碰像素；switch 仍是穷尽匹配）。两页的字段名/方法名保持原样，页面
/// 其余代码一行不动。
typedef _ThreadsPhase = PaAsyncPhase;

class _ThreadsPageState extends State<ThreadsPage> {
  late ThreadsDataSource _source;
  _ThreadsPhase _phase = _ThreadsPhase.loading;
  String _errorMessage = '';
  List<ThreadSummary> _threads = const <ThreadSummary>[];
  ThreadFilter _filter = ThreadFilter.none;
  late final TextEditingController _queryCtrl;

  @override
  void initState() {
    super.initState();
    // 默认数据源 = 真通道（B3-3）：走 `dev.pocketagent/bridge` 的 listThreads；
    // 宿主桥未注册（B4 波次前）时如实进错误态，**不伪装成空列表**（见
    // threads_bridge.dart 的诚实占位说明）。测试注入 [source] 即旁路真通道。
    _source = widget.source ?? BridgeThreadsDataSource();
    _queryCtrl = TextEditingController();
    _load();
  }

  @override
  void dispose() {
    _queryCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    // 首次加载（initState 触发）时初值就是 loading/空错误文案，跳过 setState：
    // 这一帧还没建完，重复标脏只是白跑一帧，语义上也没有状态要翻。
    if (_phase != _ThreadsPhase.loading || _errorMessage.isNotEmpty) {
      setState(() {
        _phase = _ThreadsPhase.loading;
        _errorMessage = '';
      });
    }
    try {
      final List<ThreadSummary> threads = await _source.load();
      if (!mounted) return;
      setState(() {
        _threads = threads;
        _phase = _ThreadsPhase.ready;
      });
    } on ThreadsLoadFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _phase = _ThreadsPhase.error;
        _errorMessage = e.message;
      });
    } catch (e) {
      // 兜底：数据源的意外错误也要落进错误态，不许裸抛给框架。
      if (!mounted) return;
      setState(() {
        _phase = _ThreadsPhase.error;
        _errorMessage = '$e';
      });
    }
  }

  void _setFilter({
    Object? agentId = _keep,
    Object? status = _keep,
    String? query,
  }) {
    setState(() {
      _filter = ThreadFilter(
        agentId: identical(agentId, _keep) ? _filter.agentId : agentId as String?,
        status: identical(status, _keep) ? _filter.status : status as PAStatusKind?,
        query: query ?? _filter.query,
      );
    });
  }

  static const Object _keep = Object();

  /// 列表里出现过的 agent（首次出现序，去重）——筛选条从数据派生，
  /// **不写死 agent 名单**；能力位 `agent:<id>` 门控等接上 capabilities 后再收。
  List<String> get _agentIds {
    final List<String> ids = <String>[];
    for (final ThreadSummary t in _threads) {
      if (!ids.contains(t.agentId)) ids.add(t.agentId);
    }
    return ids;
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
      case _ThreadsPhase.loading:
        return <Widget>[
          // 用固定 padding 而不是全屏居中：ListView 内的居中要靠
          // shrinkWrap/sliver 技巧，徒增复杂度，加载态只要「一眼可见在转」。
          Padding(
            padding: const EdgeInsets.only(top: 60),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  SizedBox(
                    width: 26,
                    height: 26,
                    child: CircularProgressIndicator(strokeWidth: 2.5, color: pa.brand),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    '正在加载会话…',
                    style: PATypography.caption.copyWith(color: pa.fgSecondary),
                  ),
                ],
              ),
            ),
          ),
        ];
      case _ThreadsPhase.error:
        return <Widget>[
          Padding(
            padding: const EdgeInsets.only(top: 48),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Icon(Icons.error_outline_rounded, size: 30, color: pa.danger),
                  const SizedBox(height: 10),
                  Text(
                    '会话列表加载失败',
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
                      style: PATypography.caption.copyWith(color: pa.fgTertiary),
                    ),
                  ],
                  const SizedBox(height: 14),
                  PAButton(
                    label: '重试',
                    expanded: false,
                    onPressed: _load,
                  ),
                ],
              ),
            ),
          ),
        ];
      case _ThreadsPhase.ready:
        return _buildReadyBody(context, pa);
    }
  }

  List<Widget> _buildReadyBody(BuildContext context, PAColors pa) {
    if (_threads.isEmpty) {
      return <Widget>[
        _EmptyView(
          // 空态 = 通道真读了、daemon 真回了空列表（真「还没有会话」）；
          // 通道没就绪/读失败走上面的错误态，绝不会落到这里（两条路径分家）。
          title: '还没有会话',
          hint: '桌面端还没有会话记录，去电脑上开一个任务吧',
        ),
      ];
    }
    final List<ThreadSummary> visible = applyThreadFilter(_threads, _filter);
    if (visible.isEmpty) {
      return <Widget>[
        _EmptyView(
          title: '没有匹配的会话',
          hint: '当前筛选：${_filterLabel()}',
          actionLabel: '清除筛选',
          onAction: () {
            _queryCtrl.clear();
            _setFilter(agentId: null, status: null, query: '');
          },
        ),
      ];
    }
    return <Widget>[
      _buildFilterBar(pa),
      const SizedBox(height: 4),
      for (final ThreadSummary t in visible)
        Padding(
          padding: const EdgeInsets.only(bottom: 10),
          child: _ThreadRow(thread: t, onTap: () {}),
        ),
    ];
  }

  String _filterLabel() {
    final List<String> parts = <String>[
      if (_filter.agentId != null) 'agent=${_filter.agentId}',
      if (_filter.status != null) '状态=${PAStatusChip.labelOf(_filter.status!)}',
      if (_filter.query.trim().isNotEmpty) '搜索=${_filter.query.trim()}',
    ];
    return parts.join(' · ');
  }

  Widget _buildFilterBar(PAColors pa) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          TextField(
            key: const ValueKey<String>('threads-search'),
            controller: _queryCtrl,
            style: PATypography.body.copyWith(color: pa.fgPrimary),
            onChanged: (String v) => _setFilter(query: v),
            decoration: InputDecoration(
              isDense: true,
              hintText: '搜索会话（标题 / 摘要）',
              hintStyle: PATypography.caption.copyWith(color: pa.fgTertiary),
              prefixIcon: Icon(Icons.search_rounded, size: 18, color: pa.fgTertiary),
              filled: true,
              fillColor: pa.bgSurface,
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
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
          if (_agentIds.length > 1) ...<Widget>[
            const SizedBox(height: 8),
            Wrap(
              spacing: 6,
              runSpacing: 6,
              children: <Widget>[
                for (final String agentId in _agentIds)
                  _agentChip(pa, agentId),
              ],
            ),
          ],
          const SizedBox(height: 8),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: <Widget>[
              for (final PAStatusKind kind in PAStatusKind.values)
                _statusChip(pa, kind),
            ],
          ),
        ],
      ),
    );
  }

  Widget _agentChip(PAColors pa, String agentId) {
    final bool selected = _filter.agentId == agentId;
    return FilterChip(
      key: ValueKey<String>('threads-agent-$agentId'),
      label: Text(agentId),
      selected: selected,
      showCheckmark: false,
      onSelected: (bool v) =>
          _setFilter(agentId: v ? agentId : null),
      backgroundColor: pa.bgSurface2,
      selectedColor: pa.agentAccent.withValues(alpha: 0.18),
      side: BorderSide(
        color: selected ? pa.agentAccent : pa.hairline,
      ),
      labelStyle: PATypography.caption.copyWith(
        color: selected ? pa.agentAccent : pa.fgSecondary,
        fontWeight: FontWeight.w600,
      ),
    );
  }

  Widget _statusChip(PAColors pa, PAStatusKind kind) {
    final bool selected = _filter.status == kind;
    final Color color = PAStatusChip.colorOf(kind, pa);
    return FilterChip(
      key: ValueKey<String>('threads-status-$kind'),
      label: Text(PAStatusChip.labelOf(kind)),
      selected: selected,
      showCheckmark: false,
      onSelected: (bool v) => _setFilter(status: v ? kind : null),
      backgroundColor: pa.bgSurface2,
      selectedColor: color.withValues(alpha: 0.16),
      side: BorderSide(color: selected ? color : pa.hairline),
      labelStyle: PATypography.caption.copyWith(
        color: selected ? color : pa.fgSecondary,
        fontWeight: FontWeight.w600,
      ),
    );
  }
}

/// 单行卡片：头像 + 标题/时间 + 摘要/状态 chip（B1 版式的直译，数据换真模型）。
class _ThreadRow extends StatelessWidget {
  const _ThreadRow({required this.thread, required this.onTap});

  final ThreadSummary thread;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final ThreadSummary t = thread;
    return PACard(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
      onTap: onTap,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Container(
            width: 40,
            height: 40,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: pa.agentAccent.withValues(alpha: 0.16),
              borderRadius: BorderRadius.circular(10),
            ),
            // agentId 原样进头像（不缩写成 cx/hm 这类自造缩写：缩写规则一旦
            // 写死就成了第二份 agent 名单，新 agent 会撞车）。
            child: FittedBox(
              fit: BoxFit.scaleDown,
              child: Text(
                t.agentId,
                style: PATypography.bodySmall.copyWith(
                  color: pa.agentAccent,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        t.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: PATypography.body.copyWith(
                          color: pa.fgPrimary,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      t.time,
                      style:
                          PATypography.caption.copyWith(color: pa.fgTertiary),
                    ),
                  ],
                ),
                const SizedBox(height: 3),
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        t.summary,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: PATypography.bodySmall
                            .copyWith(color: pa.fgSecondary),
                      ),
                    ),
                    const SizedBox(width: 8),
                    PAStatusChip(status: t.status),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 空态：区分「本来就没有」与「被筛没了」（后者给清除筛选出口）。
class _EmptyView extends StatelessWidget {
  const _EmptyView({
    required this.title,
    required this.hint,
    this.actionLabel,
    this.onAction,
  });

  final String title;
  final String hint;
  final String? actionLabel;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(top: 48),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Text(
              title,
              style: PATypography.body.copyWith(
                color: pa.fgPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              hint,
              textAlign: TextAlign.center,
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
            if (actionLabel != null) ...<Widget>[
              const SizedBox(height: 12),
              TextButton(
                onPressed: onAction,
                child: Text(
                  actionLabel!,
                  style: PATypography.caption.copyWith(
                    color: pa.brand,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
