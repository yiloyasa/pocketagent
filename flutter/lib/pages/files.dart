// 文件页（B3 第 3 波）：目录列表 + 面包屑 + 文本预览，全走真通道。
//
// 【页面职责】page-contracts §1.4 Files：目录/文件两型行渲染、面包屑导航、
// 三态纪律、readFile 文本预览。数据源默认 [BridgeDirectorySource]
// （PaBridge `listDirectory` → proto `DirectoryEntry` 五字段映射，见
// files_bridge.dart）；宿主桥未注册（B4 波次前）时如实进错误态「桌面端文件
// 通道未就绪（B4 波次）」，**不伪装成空目录**（两条路径分别有测试钉死）。
//
// 【本波不做的】写操作（新建/删除/重命名/下载）——红线「手机端写电脑文件」待
// 拍板；文件名递归搜索——daemon 的 SearchContent 是**内容**搜索，proto 没有
// 文件名搜索 RPC（page-contracts §1.4 明说），搜索框只过滤当前目录已加载条目
// （与 Android 同款范围，且界面上写清范围，不做"看起来能搜全部"的假承诺）；
// 盘符列表（ListRoots）——transport 本波没有这个方法面，根段是 daemon 回落
// workspace 的「默认目录」，点了就是空 path 的 ListDirectory。
import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'files_bridge.dart';

/// 文件页：真通道目录树（只读）。
///
/// 数据源/预览源都是注入点：测试传可控源或带 mock 通道的桥源；生产不传即
/// [BridgeDirectorySource] / [BridgeFilePreviewSource]。
class FilesPage extends StatefulWidget {
  const FilesPage({
    super.key,
    this.source,
    this.previewSource,
    this.initialPath = '',
  });

  final FilesDirectorySource? source;
  final FilesPreviewSource? previewSource;

  /// 初始目录（空 = daemon 回落 workspace）。生产入口不带初始路径。
  final String initialPath;

  @override
  State<FilesPage> createState() => _FilesPageState();
}

/// 三态：加载中 / 加载失败 / 已就绪（就绪后再分「空目录」与「被筛空」）。
enum _FilesPhase { loading, error, ready }

class _FilesPageState extends State<FilesPage> {
  late FilesDirectorySource _source;
  late FilesPreviewSource _previewSource;
  _FilesPhase _phase = _FilesPhase.loading;
  String _errorMessage = '';
  List<FileEntry> _entries = const <FileEntry>[];

  /// 当前目录路径；空串 = daemon 回落 workspace（面包屑根段）。
  String _path = '';
  String _query = '';
  late final TextEditingController _queryCtrl;

  /// 预览层：非空 = 正在预览某个文件（覆盖在列表上的卡片）。
  FileEntry? _previewing;
  bool _previewLoading = false;
  String _previewError = '';
  String _previewText = '';
  bool _previewTruncated = false;

  @override
  void initState() {
    super.initState();
    _source = widget.source ?? BridgeDirectorySource();
    _previewSource = widget.previewSource ?? BridgeFilePreviewSource();
    _path = widget.initialPath;
    _queryCtrl = TextEditingController();
    _load();
  }

  @override
  void dispose() {
    _queryCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final String target = _path;
    if (_phase != _FilesPhase.loading || _errorMessage.isNotEmpty) {
      setState(() {
        _phase = _FilesPhase.loading;
        _errorMessage = '';
      });
    }
    try {
      final List<FileEntry> entries = await _source.load(target);
      if (!mounted) return;
      setState(() {
        _entries = entries;
        _path = target;
        _phase = _FilesPhase.ready;
        _previewing = null;
        _previewError = '';
      });
    } on FilesLoadFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _phase = _FilesPhase.error;
        _errorMessage = e.message;
      });
    } catch (e) {
      // 兜底：数据源的意外错误也要落进错误态，不许裸抛给框架。
      if (!mounted) return;
      setState(() {
        _phase = _FilesPhase.error;
        _errorMessage = '$e';
      });
    }
  }

  /// 当前目录过滤（纯本地，只打已加载条目的名字——搜索范围如实写在小字里）。
  List<FileEntry> get _visible {
    final String q = _query.trim().toLowerCase();
    if (q.isEmpty) return _entries;
    return _entries
        .where((FileEntry e) => e.name.toLowerCase().contains(q))
        .toList(growable: false);
  }

  Future<void> _openEntry(FileEntry e) async {
    if (e.isDir) {
      // 进目录：先翻路径再加载（_load 读的是 _path 当前值）
      setState(() => _path = e.path);
      await _load();
      return;
    }
    // 文件：先过大小闸（纯函数，不发起 readFile 也知道能不能预览）
    final String reject = filePreviewRejection(e);
    if (reject.isNotEmpty) {
      setState(() {
        _previewing = e;
        _previewLoading = false;
        _previewError = reject;
        _previewText = '';
      });
      return;
    }
    setState(() {
      _previewing = e;
      _previewLoading = true;
      _previewError = '';
      _previewText = '';
      _previewTruncated = false;
    });
    try {
      final FilePreviewContent content = await _previewSource.readText(e);
      if (!mounted) return;
      setState(() {
        _previewLoading = false;
        _previewText = content.text;
        _previewTruncated = content.truncated;
      });
    } on FilePreviewFailure catch (err) {
      if (!mounted) return;
      setState(() {
        _previewLoading = false;
        _previewError = err.message;
      });
    } catch (err) {
      if (!mounted) return;
      setState(() {
        _previewLoading = false;
        _previewError = '$err';
      });
    }
  }

  void _closePreview() {
    setState(() {
      _previewing = null;
      _previewError = '';
      _previewText = '';
      _previewTruncated = false;
    });
  }

  void _goTo(String target) {
    if (target == _path && _phase == _FilesPhase.ready) return;
    setState(() => _path = target);
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        _buildBreadcrumbBar(pa),
        Expanded(
          child: _previewing != null
              ? _buildPreview(context, pa)
              : _buildList(context, pa),
        ),
      ],
    );
  }

  // ── 面包屑 ─────────────────────────────────────────────────────────────

  Widget _buildBreadcrumbBar(PAColors pa) {
    final List<PathCrumb> crumbs = breadcrumbSegmentsOf(_path);
    return SizedBox(
      height: 44,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 14),
        itemCount: crumbs.length,
        separatorBuilder: (BuildContext _, int _) => Padding(
          padding: const EdgeInsets.symmetric(horizontal: 2),
          child: Icon(Icons.chevron_right_rounded, size: 14, color: pa.fgTertiary),
        ),
        itemBuilder: (BuildContext _, int i) {
          final PathCrumb c = crumbs[i];
          final bool isLast = i == crumbs.length - 1;
          return GestureDetector(
            key: ValueKey<String>('files-crumb-${c.target}'),
            onTap: isLast ? null : () => _goTo(c.target),
            child: Center(
              child: Text(
                c.label,
                maxLines: 1,
                style: PATypography.caption.copyWith(
                  fontFamily: PATypography.monoFont,
                  color: isLast ? pa.fgPrimary : pa.brand,
                  fontWeight: isLast ? FontWeight.w600 : FontWeight.w500,
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  // ── 列表三态 ───────────────────────────────────────────────────────────

  Widget _buildList(BuildContext context, PAColors pa) {
    switch (_phase) {
      case _FilesPhase.loading:
        return _LoadingView(message: '正在加载目录…');
      case _FilesPhase.error:
        return _ErrorView(message: _errorMessage, onRetry: _load);
      case _FilesPhase.ready:
        return _buildReadyBody(pa);
    }
  }

  Widget _buildReadyBody(PAColors pa) {
    final List<FileEntry> visible = _visible;
    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 4, 14, 24),
      children: <Widget>[
        _buildSearchField(pa),
        const SizedBox(height: 8),
        if (_entries.isEmpty)
          _EmptyView(
            title: '这个目录是空的',
            hint: 'daemon 没有返回任何条目；若电脑上应有内容，点「重试」再读一次',
            actionLabel: '重试',
            onAction: _load,
          )
        else if (visible.isEmpty)
          _EmptyView(
            title: '当前目录没有匹配的文件',
            hint: '搜索只过滤已加载的当前目录条目，不含子目录',
            actionLabel: '清除搜索',
            onAction: () {
              _queryCtrl.clear();
              setState(() => _query = '');
            },
          )
        else
          PACard(
            padding: EdgeInsets.zero,
            child: Column(
              children: <Widget>[
                for (int i = 0; i < visible.length; i++) ...<Widget>[
                  if (i > 0)
                    const Padding(
                      padding: EdgeInsets.only(left: 52),
                      child: Divider(height: 1),
                    ),
                  _FileRow(
                    entry: visible[i],
                    onTap: () => _openEntry(visible[i]),
                  ),
                ],
              ],
            ),
          ),
      ],
    );
  }

  Widget _buildSearchField(PAColors pa) {
    return TextField(
      key: const ValueKey<String>('files-search'),
      controller: _queryCtrl,
      style: PATypography.body.copyWith(color: pa.fgPrimary),
      onChanged: (String v) => setState(() => _query = v),
      decoration: InputDecoration(
        isDense: true,
        hintText: '按文件名过滤当前目录…',
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
    );
  }

  // ── 预览 ───────────────────────────────────────────────────────────────

  Widget _buildPreview(BuildContext context, PAColors pa) {
    final FileEntry e = _previewing!;
    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 8, 14, 24),
      children: <Widget>[
        Row(
          children: <Widget>[
            IconButton(
              key: const ValueKey<String>('files-preview-back'),
              icon: const Icon(Icons.arrow_back_rounded, size: 18),
              color: pa.fgSecondary,
              tooltip: '返回列表',
              onPressed: _closePreview,
            ),
            Expanded(
              child: Text(
                e.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: PATypography.body.copyWith(
                  color: pa.fgPrimary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            Text(
              humanFileSize(e.sizeBytes),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
        ),
        const SizedBox(height: 10),
        if (_previewLoading)
          const _LoadingView(message: '正在读取文件…')
        else if (_previewError.isNotEmpty)
          _EmptyView(
            title: '无法预览这个文件',
            hint: _previewError,
            actionLabel: '返回列表',
            onAction: _closePreview,
          )
        else
          PACard(
            color: pa.bgCode,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                if (_previewTruncated) ...<Widget>[
                  Text(
                    '文件还有后续内容未显示（预览只读前'
                    '${humanFileSize(kFilePreviewMaxBytes)}）',
                    style: PATypography.caption.copyWith(color: pa.warn),
                  ),
                  const SizedBox(height: 8),
                ],
                SelectableText(
                  _previewText,
                  style: PATypography.code.copyWith(color: pa.fgPrimary),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

/// 目录/文件一行：图标 + 名 + 元信息（大小/时间）。
///
/// 图标两型：目录恒文件夹；文件按类型粗分（文本/代码类用文档图标，其余用
/// 通用文件图标）——不建 FileKind 那种 24 档枚举：本波只读预览，行内只需要
/// 「这是目录还是文件、多大、什么时候」三个事实。
class _FileRow extends StatelessWidget {
  const _FileRow({required this.entry, required this.onTap});

  final FileEntry entry;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final FileEntry e = entry;
    final String meta =
        fileEntryMetaLine(e, nowYear: DateTime.now().year);
    return InkWell(
      key: ValueKey<String>('files-row-${e.path}'),
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
        child: Row(
          children: <Widget>[
            Container(
              width: 38,
              height: 38,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: (e.isDir ? pa.ok : pa.brand).withValues(alpha: 0.14),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(
                e.isDir ? Icons.folder_rounded : _fileIconOf(e.name),
                size: 18,
                color: e.isDir ? pa.ok : pa.brand,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    e.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: PATypography.body.copyWith(
                      color: pa.fgPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (meta.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      meta,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style:
                          PATypography.caption.copyWith(color: pa.fgTertiary),
                    ),
                  ],
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
      ),
    );
  }
}

/// 文件图标：能文本预览的（源码/文本/数据类）用代码图标，其余用通用文件图标。
///
/// 【为什么不再细分】行内图标只回答「这东西是不是能点开看」。真正的类型分型
/// （图片/PDF/Office）属于 Browser 的 PcFilePreview 能力矩阵，本波预览只有
/// 文本一种，图标分太细反而许诺了没有的能力。
IconData _fileIconOf(String name) {
  final String lower = name.toLowerCase();
  const List<String> textExts = <String>[
    'txt', 'md', 'markdown', 'json', 'yaml', 'yml', 'toml', 'xml', 'html',
    'css', 'js', 'ts', 'tsx', 'dart', 'go', 'py', 'rs', 'java', 'kt', 'kts',
    'c', 'h', 'cpp', 'hpp', 'cs', 'rb', 'php', 'sh', 'bat', 'ps1', 'sql',
    'log', 'ini', 'cfg', 'conf', 'env', 'properties', 'gradle', 'proto',
    'csv', 'tsv', 'gitignore', 'editorconfig',
  ];
  for (final String ext in textExts) {
    if (lower.endsWith('.$ext')) return Icons.code_rounded;
  }
  return Icons.description_rounded;
}

/// 加载态（固定 padding 而不是全屏居中：ListView 内的居中要靠
/// shrinkWrap/sliver 技巧，徒增复杂度，加载态只要「一眼可见在转」）。
class _LoadingView extends StatelessWidget {
  const _LoadingView({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(top: 60),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            SizedBox(
              width: 26,
              height: 26,
              child: CircularProgressIndicator(
                strokeWidth: 2.5,
                color: pa.brand,
              ),
            ),
            const SizedBox(height: 12),
            Text(
              message,
              style: PATypography.caption.copyWith(color: pa.fgSecondary),
            ),
          ],
        ),
      ),
    );
  }
}

/// 错误态：标题 + 人话原因 + 重试出口（重试是真的重新 load，见页面 _load）。
class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final Future<void> Function() onRetry;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return SingleChildScrollView(
      padding: const EdgeInsets.only(top: 48),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.error_outline_rounded, size: 30, color: pa.danger),
            const SizedBox(height: 10),
            Text(
              '目录加载失败',
              style: PATypography.body.copyWith(
                color: pa.fgPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
            if (message.isNotEmpty) ...<Widget>[
              const SizedBox(height: 6),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Text(
                  message,
                  textAlign: TextAlign.center,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ),
            ],
            const SizedBox(height: 14),
            PAButton(
              label: '重试',
              expanded: false,
              onPressed: onRetry,
            ),
          ],
        ),
      ),
    );
  }
}

/// 空态：区分「本来就没有」（空目录）与「被筛没了」（给清除出口）。
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
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Text(
                hint,
                textAlign: TextAlign.center,
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
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
