// 设置页「下载与存储」二级页（路由 sub=downloads；旧深链 userdata 归一化到本页）
// —— Settings 三子步第③步。
//
// 【与 TaskCenter 是两个东西】TaskCenter 管「任务与未读」（消息/下载两类任务
// 的状态流），本页管「下载分区与存储占用」：分区大小汇总 + 清理入口的诚实占位。
// 两者的数据在本页**共享同一个** TaskCenterStore（Kotlin DownloadsPage 同样读
// TaskCenter.tasks，SettingsScreen.kt:3561）——同一份本地真值两个视角，不另造
// 第二份数据源。
//
// 【分区数据从哪来（诚实记账）】Kotlin 的存储分区（缓存/语音模型/快照/临时/
// 浏览器数据）全部来自**手机本地文件系统**（PaCache.cacheUsage / filesUsage /
// OfflineStore.allPartitionStats，SettingsScreen.kt:3830-3857）。proto 侧没有
// 任何「手机本地分区」消息面：`RuntimeStateResponse` 逐字段核对过（安装身份/
// 主机/版本/策略 revision/runtime_state/contracts/data_dir），**没有分区字段**。
// 因此本页分区按任务书纪律走「本地占位 + 注释」：
//   · 下载分区 = TaskCenterStore 的下载类任务（本地真值，可汇总可渲染）；
//   · 缓存 / 语音模型 / 其他分区 = 本地统计面未接入（需要平台文件面/宿主方法面，
//     见下方「交接」段），如实标「未统计」，**不用 0 冒充「没占用」**。
//
// 【清理 = 写操作，本波不做】清理缓存/下载条目都是破坏性写（Kotlin 侧走
// OfflineStore.clear / deleteLocalFile，且会话快照要二次确认）。本波只呈现
// 大小与状态，行内给「清理留后续波次」的诚实文案，**不摆假按钮**（红线：不留
// 假绿）。
//
// 【宿主运行时状态为什么也读一屏】「下载与存储」要回答的不只是手机本地：电脑端
// daemon 的数据目录与运行时降级态同样与「存储」相关（RuntimeStateResponse 的
// data_dir / runtime_state 是本波唯一带这两个事实的只读面）。该块走
// [getRuntimeState] 真通道，三态齐全：宿主缺席（HOST_UNAVAILABLE）→ 未就绪
// 错误态；读成功但字段空 → 未报告；与「空态」三分，不互相冒充。

import 'package:flutter/material.dart';

import '../gen/pocketagent/v1/runtime.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'pa_page_kit.dart';
import 'settings_connection.dart';
import 'task_center_data.dart';
// 人类可读体积格式化（B/KB/MB/GB 口径）复用 Files 页的同名纯函数 —— 全工程
// 一份实现（口径 = Android formatBytesLabel），不为存储页再抄一版。
import 'files_bridge.dart' show humanFileSize;
// ready/degraded 的人话标签与 console 页同一张词表（Kotlin RuntimeStateLogic
// 的 runtimeStateLabel）：一处定义两页消费，不为「下载页也显示运行时状态」
// 再抄一份映射。
import 'settings_console.dart' show runtimeStateLabel;

/// 一个存储分区行：名称 + 占用 + 说明。
///
/// 【为什么不用 0 表示未知】`bytes` 为 null = 这一项没统计到（本地统计面未接
/// 入），与「统计到了确实是 0」是两件事；前者显示「未统计」，后者显示 0 B。
/// 混同的表现是用户以为「缓存本来就没占地方」。
@immutable
class StoragePartition {
  const StoragePartition({
    required this.id,
    required this.label,
    required this.bytes,
    required this.note,
  });

  /// 分区稳定键（本页内唯一；不参与任何 wire 契约）。
  final String id;

  /// 分区人话名（Kotlin CacheCleanupLogic.Kind 的展示名同款口径）。
  final String label;

  /// 占用字节；null = 未统计（**不是 0**）。
  final int? bytes;

  /// 一行说明（为什么未统计 / 这一项是什么）。
  final String note;
}

/// 下载分区的汇总（纯函数，单测直接命中）。
///
/// 口径与 Kotlin DownloadsPage（SettingsScreen.kt:3581-3589）一致：
///   · 只数**已完成**的下载条目（未完成的不占最终体积）；
///   · 每条体积优先用 sizeText 解析出的字节数，解析不出 = 未知；
///   · 任一条未知 → `unknown = true`，汇总显示「占用无法获取」而**不是**一个
///     偏小的数字（把未知当 0 会让用户以为没占地方）。
@immutable
class DownloadsTotal {
  const DownloadsTotal({
    required this.count,
    required this.bytes,
    required this.unknown,
  });

  /// 已完成下载条目数。
  final int count;

  /// 已统计条目的字节合计（未知条目不计入）。
  final int bytes;

  /// 是否存在体积未知的条目（true → 合计不可信，页面显示「占用无法获取」）。
  final bool unknown;

  /// 汇总文案：`N 个文件 · 共 X`；有未知条目时如实说无法获取；0 个不摆 0。
  String get label {
    if (count <= 0) return '暂无下载内容';
    if (unknown) return '$count 个文件 · 占用无法获取';
    if (bytes <= 0) return '$count 个文件';
    return '$count 个文件 · 共 ${humanFileSize(bytes)}';
  }
}

/// 解析体积文案（`1.2MB` / `340KB` / `512B` 风格）→ 字节数；解析不出返回 null。
///
/// 【为什么允许本地解析】Kotlin 同款做法（parseHumanSize(d.sizeText)）：
/// sizeText 是任务条目落盘时记下的人话文案，手机本地没有第二个字节真值源
/// （MediaStore 查询在 Flutter 侧没有对应平台面），如实解析、解析不出就标未知。
int? parseHumanSize(String text) {
  final String t = text.trim();
  if (t.isEmpty) return null;
  final RegExp m = RegExp(r'^([0-9]+(?:\.[0-9]+)?)\s*([KMGTP]?B?)$', caseSensitive: false);
  final RegExpMatch? match = m.firstMatch(t);
  if (match == null) return null;
  final double value = double.parse(match.group(1)!);
  final String unit = match.group(2)!.toUpperCase();
  final int factor = switch (unit) {
    'B' || '' => 1,
    'KB' || 'K' => 1 << 10,
    'MB' || 'M' => 1 << 20,
    'GB' || 'G' => 1 << 30,
    'TB' || 'T' => 1 << 40,
    'PB' || 'P' => 1 << 50,
    _ => 1,
  };
  return (value * factor).round();
}

/// 下载分区汇总（纯函数）：只数已完成下载类任务。
DownloadsTotal downloadsTotalOf(List<TaskItem> tasks) {
  final List<TaskItem> done = tasks
      .where((TaskItem t) =>
          t.kind == TaskKind.download && t.status == TaskStatus.done)
      .toList(growable: false);
  int bytes = 0;
  bool unknown = false;
  for (final TaskItem t in done) {
    final int? size = parseHumanSize(t.sizeText);
    if (size == null) {
      unknown = true;
      continue;
    }
    bytes += size;
  }
  return DownloadsTotal(count: done.length, bytes: bytes, unknown: unknown);
}

/// 分区清单（纯函数）：下载分区走本地真值，其余分区标注未统计。
///
/// 【为什么其余分区硬编码三条】它们是 Kotlin 存储页的既有口径（缓存/语音模型/
/// 其他），且都是**手机本地**目录：本地统计面未接入前，列出来是为了让用户看见
/// 「这些项存在、本波没统计」，而不是藏起来让用户以为只有下载一项。
/// 一旦本地统计面接入（交接见下），把对应 `bytes` 从 null 换成真值即可，
/// 页面与测试的渲染形状不变。
List<StoragePartition> storagePartitionsOf(List<TaskItem> tasks) {
  final DownloadsTotal total = downloadsTotalOf(tasks);
  // 下载分区的占用：没有已完成条目时不摆 `0 B`（「0 个文件」与「占了 0 字节」
  // 是两件事，后者会被读成「查过了，确实没占」）。unknown（有条目体积未知）
  // 同理不可信 —— 两种都标未统计，由 note 说清是哪种。
  final int? downloadBytes =
      total.count <= 0 || total.unknown ? null : total.bytes;
  return <StoragePartition>[
    StoragePartition(
      id: 'downloads',
      label: '下载内容',
      bytes: downloadBytes,
      note: total.count <= 0
          ? '本地任务清单里还没有已完成的下载。'
          : '来自任务中心的 ${total.count} 个已完成下载'
              '（${total.unknown ? '占用无法获取' : humanFileSize(total.bytes)}）。',
    ),
    const StoragePartition(
      id: 'cache',
      label: '缓存',
      bytes: null,
      note: '可再生缓存（预览 / 网页快照 / 浏览器数据等）；'
          '本地文件统计面本波未接入，暂不显示占用。',
    ),
    const StoragePartition(
      id: 'voice-model',
      label: '语音模型',
      bytes: null,
      note: '离线语音识别模型按需下载后占用本机空间；'
          '本波只做展示，下载与清理入口留后续波次。',
    ),
    const StoragePartition(
      id: 'other',
      label: '其他',
      bytes: null,
      note: '其余应用数据；系统「设置 → 应用 → 存储占用」里的口径更完整。',
    ),
  ];
}

/// 下载页错误文案：与 connection / agent / models 子页同一套 code→文案映射
/// （[paPageErrorText]），只把 HOST_UNAVAILABLE 的收尾按本页语境改词。
String downloadsPageErrorText(PaBridgeException e) => paPageErrorText(
      e,
      hostUnavailableTail: '电脑端运行时信息暂不可用。',
    );

/// 「下载与存储」子页：下载分区（本地真值）+ 存储分区（未统计如实标注）+
/// 电脑端运行时（只读真通道）。
///
/// 数据 = ① 注入的 [TaskCenterStore]（本地任务清单，Kotlin DownloadsPage 同源）
/// + ② 宿主 `getRuntimeState` 的一次性快照 + 手动刷新。两块互不依赖：本地块
/// 断网照常渲染（Kotlin「离线可靠」同款），宿主块失败只让自己进错误态。
class SettingsDownloadsPage extends StatefulWidget {
  const SettingsDownloadsPage({
    super.key,
    this.store,
    this.bridge,
    required this.onBack,
  });

  /// 本地任务清单注入点（测试传可控 store；生产不传即新建 —— 与 TaskCenter
  /// 页共用同一个 app 级 store 的等价物）。
  final TaskCenterStore? store;

  /// 通道注入点（测试用）；不传即用生产默认通道 `dev.pocketagent/bridge`。
  final PaBridge? bridge;

  /// 返回一级页。
  final VoidCallback onBack;

  @override
  State<SettingsDownloadsPage> createState() => _SettingsDownloadsPageState();
}

class _SettingsDownloadsPageState extends State<SettingsDownloadsPage> {
  late final TaskCenterStore _store = widget.store ?? TaskCenterStore();
  late final PaBridge _bridge = widget.bridge ?? PaBridge();

  /// 本地任务清单（store 快照；store 是广播流，变更时重取）。
  List<TaskItem> _tasks = const <TaskItem>[];

  /// 宿主运行时块三态（与 [PaAsyncPhase] 同词汇：加载 / 读失败 / 已就绪）。
  PaAsyncPhase _hostPhase = PaAsyncPhase.loading;
  RuntimeStateResponse? _hostState;
  String _hostErrorText = '';

  @override
  void initState() {
    super.initState();
    _tasks = _store.snapshot;
    _store.tasks.listen((List<TaskItem> next) {
      if (!mounted) return;
      setState(() => _tasks = next);
    });
    _loadHost();
  }

  Future<void> _loadHost() async {
    setState(() {
      _hostPhase = PaAsyncPhase.loading;
      _hostErrorText = '';
    });
    try {
      final RuntimeStateResponse state = await _bridge.getRuntimeState();
      if (!mounted) return;
      setState(() {
        _hostState = state;
        _hostPhase = PaAsyncPhase.ready;
      });
    } on PaBridgeException catch (e) {
      // 读失败不保留旧快照冒充本次结果（不编造、不缓存冒充）。
      if (!mounted) return;
      setState(() {
        _hostState = null;
        _hostPhase = PaAsyncPhase.error;
        _hostErrorText = downloadsPageErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _hostState = null;
        _hostPhase = PaAsyncPhase.error;
        _hostErrorText = '读取电脑端运行时信息失败：$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final List<TaskItem> downloads = _tasks
        .where((TaskItem t) => t.kind == TaskKind.download)
        .toList(growable: false);
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaSubPageHeader(
          title: '下载与存储',
          backKey: 'pa-downloads-back',
          onBack: widget.onBack,
        ),
        PaPageSection(
          title: '存储分区',
          children: <Widget>[
            for (final StoragePartition p in storagePartitionsOf(_tasks))
              _partitionRow(context, p),
          ],
        ),
        PaPageSection(
          title: '下载内容 · ${downloadsTotalOf(_tasks).label}',
          children: <Widget>[
            if (downloads.isEmpty)
              const PaCardCaption(
                // Kotlin DownloadsPage 空态同款文案：说清「去哪儿下载后这里会有」。
                text: '还没有下载内容。在会话里下载或导出对话后，会出现在这里。',
                textKey: 'pa-downloads-empty',
                padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
              )
            else
              for (final TaskItem t in downloads) _downloadRow(context, t),
          ],
        ),
        PaPageSection(
          title: '电脑端运行时',
          children: <Widget>[_buildHostBody(context)],
        ),
      ],
    );
  }

  // ── 存储分区行 ──────────────────────────────────────────────────────────

  Widget _partitionRow(BuildContext context, StoragePartition p) {
    final PAColors pa = context.pa;
    final int? bytes = p.bytes;
    return Padding(
      key: ValueKey<String>('pa-storage-partition-${p.id}'),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  p.label,
                  style: PATypography.body.copyWith(color: pa.fgPrimary),
                ),
              ),
              Text(
                bytes == null ? '未统计' : humanFileSize(bytes),
                key: ValueKey<String>('pa-storage-size-${p.id}'),
                style: PATypography.body.copyWith(color: pa.fgSecondary),
              ),
            ],
          ),
          const SizedBox(height: 3),
          Text(
            p.note,
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
          const SizedBox(height: 6),
          // 清理 = 写操作，本波只给诚实文案，不摆假按钮（红线：不留假绿）。
          Text(
            '清理留后续波次：本页只读呈现占用，暂不提供清理动作。',
            key: ValueKey<String>('pa-storage-cleanup-${p.id}'),
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ],
      ),
    );
  }

  // ── 下载条目行 ──────────────────────────────────────────────────────────

  Widget _downloadRow(BuildContext context, TaskItem t) {
    final PAColors pa = context.pa;
    final List<String> meta = <String>[
      if (t.sizeText.isNotEmpty) t.sizeText,
      if (t.sourceTitle.isNotEmpty) '来自「${t.sourceTitle}」',
      _downloadStatusLabel(t.status),
    ];
    return Padding(
      key: ValueKey<String>('pa-download-row-${t.id}'),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            // 空标题 = 原生没报（Kotlin `(未命名文件)` 同款兜底，不显示空白行）。
            t.title.isEmpty ? '(未命名文件)' : t.title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: PATypography.body.copyWith(color: pa.fgPrimary),
          ),
          if (meta.isNotEmpty) ...<Widget>[
            const SizedBox(height: 3),
            Text(
              meta.join(' · '),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
        ],
      ),
    );
  }

  /// 下载条目状态 → 人话；未知取值原样返回（透传红线，不显「未知/错误」）。
  String _downloadStatusLabel(TaskStatus status) => switch (status) {
        TaskStatus.running => '下载中',
        TaskStatus.done => '已完成',
        TaskStatus.failed => '未完成',
        TaskStatus.canceled => '已取消',
        TaskStatus.interrupted => '已中断',
      };

  // ── 电脑端运行时块（只读真通道） ────────────────────────────────────────

  Widget _buildHostBody(BuildContext context) {
    if (_hostPhase == PaAsyncPhase.loading) {
      return const PaCardCaption(
        text: '正在读取电脑端运行时信息…',
        textKey: 'pa-downloads-host-loading',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    if (_hostPhase == PaAsyncPhase.error) {
      // 诚实错误态 + 重试：与空态（读成功但没字段）分家。
      return PaErrorRetry(
        errorText: _hostErrorText,
        errorKey: 'pa-downloads-host-error',
        onRetry: _loadHost,
        retryKey: 'pa-downloads-host-retry',
      );
    }
    final RuntimeStateResponse state = _hostState!;
    final List<String> facts = <String>[
      if (state.daemonVersion.isNotEmpty) 'daemon ${state.daemonVersion}',
      // ready/degraded 译成人话（词表 = Kotlin runtimeStateLabel）；空 = 未报告，
      // 该段直接不出现，不摆占位。
      if (state.runtimeState.isNotEmpty) '运行时 ${runtimeStateLabel(state.runtimeState)}',
      if (state.hostId.isNotEmpty) '主机 ${state.hostId}',
    ];
    if (state.dataDir.isEmpty) {
      return const PaCardCaption(
        // 读成功但这一项没报：如实说「未报告」，不摆路径占位符。
        text: '电脑端未报告数据目录位置。',
        textKey: 'pa-downloads-host-empty',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        PaCardCaption(
          text: facts.join(' · '),
          textKey: 'pa-downloads-host-line',
          padding: const EdgeInsets.fromLTRB(14, 12, 14, 4),
        ),
        PaCardCaption(
          // 数据目录：daemon 落盘位置（对话/产物/缓存都在它下面）。只展示不回传
          // 任何路径给写操作 —— 本页没有写通道。
          text: '数据目录 ${state.dataDir}',
          textKey: 'pa-downloads-host-datadir',
          padding: const EdgeInsets.fromLTRB(14, 0, 14, 12),
        ),
      ],
    );
  }
}

// ── 交接（后续波次） ────────────────────────────────────────────────────────
//
// 1. **本地分区统计**：缓存 / 语音模型 / 其他三项的 `bytes` 现在是 null（未统计）。
//    接入需要「手机本地目录体积」的平台面（path_provider + dart:io，或经 B2 通道
//    委托 Kotlin 宿主量 `cacheDir`/`filesDir`）。两者都要动 pubspec 或宿主桥，
//    均超出本波授权，故留占位 + 注释；接入后只改 [storagePartitionsOf] 的字节来源。
// 2. **清理动作**：Kotlin 侧清理走 OfflineStore.clear("url"/"checkpoint") /
//    ThreadSnapshot.clearAll() / PaCache，且会话快照要二次确认
//    （CacheCleanupLogic.riskOf）。写操作面（本地或宿主）拍板后再摆入口，
//    本波不摆。
// 3. **删除下载条目**：Kotlin DownloadsPage 有逐条删除 + 二次确认
//    （SettingsScreen.kt:3739-3780）。本地 store 侧 `TaskCenterStore.remove`
//    已存在，但「删手机文件本体」的确认流与文件面（openLocalFile 等价物）属同一
//    组决策，留后续波次整组接。
// 4. **下载文件查看/分享**：需要平台文件面（content URI 打开 / 系统分享），
//    同上留后续波次；本页只呈现条目与状态。
