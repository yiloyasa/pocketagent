// 任务中心页（home tab 4）—— 任务/未读中心的本地聚合视图（B3 第 3 波最后一件）。
//
// 【数据源判定：纯本地聚合，无真通道】proto/v1 全 93 RPC 逐个核对（含 host/thread/
// fs/git/model/runtime/terminal 六个 proto 全文）：**没有任何 task / notification /
// unread 相关 RPC**。page-contracts §1.7 明说本页「零直连 RPC」：Kotlin 侧数据 =
// `TaskCenter.tasks`（app 级 object + SharedPreferences 持久化），由消息执行
// （TurnSession 写入）与文件下载（PaDownloader 写入）在手机本地聚合。
// → 本页**不接 PaBridge、不显示任何远端数据**：数据来自注入的 [TaskCenterStore]
//   （构造注入，同 ThreadsPage 的 source 模式；B4 把 store 提为 app 级单例后，
//   装配点一行不用改本页）。页内也不造假加载延迟：本地 store 首帧即有快照，
//   **没有「正在加载」态**——给一个瞬间完成的加载态等于骗一次转圈。
//
// 【已读策略：显式按钮，不是 Kotlin 的自动全读】Kotlin 停留页面 500ms 后
// `markAllRead()` + `setViewing(true)`（停留期间新任务直接落已读）。本波 Flutter
// 的 store 没有 app 级共享消费者（AppDrawer 徽章、Home 都还没接线），自动已读
// 会把「未读」这个信号在无人看的时候悄悄清掉；因此改为：
//   · 顶部未读徽标只读，点它 = 「全部标为已读」（显式动作）；
//   · 点单个任务行 = 标该条已读（Kotlin 同款 `markRead`）。
//   B4 接线徽标消费方后，可把 store 的 `setViewing` 接进页面的 initState/dispose
//   还原 Kotlin 全自动语义（store 已实现该分支，测试已覆盖）。
//
// 【indeterminate 进度条改静态】Kotlin RUNNING 且 progress<=0 的下载任务是无限
// 扫动动画；Flutter widget 测试的 pumpAndSettle 会被无限动画卡死，改为静态 40%
// 填充 + 「进行中」文案（信息不丢：状态由 chip 承担，进度条只是密度提示）。
//
// 【本波没有的动作（如实留空，不造假）】
//   · 重试消息类 = TurnSession.resyncFromDaemon（B5 Chat 波次才有）；
//   · 重新下载 = PaDownloader.downloadFromUrl/downloadFromPc（B4 宿主桥 + 下载
//     服务才有）；
//   · 打开本地文件 = PaDownloader.openLocalFile；打开用户数据管理 =
//     settings?sub=downloads（该二级页当前是诚实占位）。
//   → 这些出口用可空回调表达：**不注入就不渲染对应按钮**（无位不显示假入口），
//     行内文案也同步切换成「去会话找回 / 恢复连接后再试」（不指向不存在的按钮）。

import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../components/pa_status_chip.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'task_center_data.dart';

/// 任务中心页。
///
/// 参数分三组：数据（[store]）、状态（[hostOffline]）、出口回调（全部可空）。
class TaskCenterPage extends StatefulWidget {
  const TaskCenterPage({
    super.key,
    this.store,
    this.hostOffline = false,
    this.onOpenThread,
    this.onOpenUserData,
    this.onRetryMessage,
    this.onRetryDownload,
    this.nowMs,
  });

  /// 任务数据源注入点：测试传预置 store；生产不传即新建（B4 提 app 级前的默认）。
  final TaskCenterStore? store;

  /// 与电脑是否断连（Kotlin `connState == "down"`）。
  ///
  /// 【为什么是入参而不是自己读通道】连接态真值在 Kotlin `AgentConnection`（§3.1），
  /// Flutter 侧的连接态流同样未接线（settings 连接页是自己快照自己）；这里按
  /// 页面契约收一个布尔，调用方（根装配）在 B4 把 connState 流接上即可。
  /// 断连时消息类运行中任务不显示「正在执行」徽章、改显离线提示，且重试入口
  /// 置灰（Kotlin `taskStatusView` / `taskOfflineHint` / `requestRetry` 同口径）。
  final bool hostOffline;

  /// 点消息类任务 → 回到那个会话（`chat/{threadId}`）。
  final ValueChanged<String>? onOpenThread;

  /// 点下载类任务 → 用户数据管理（查看/分享/删除都在那一页）。
  final VoidCallback? onOpenUserData;

  /// 重试消息类任务（Kotlin = TurnSession.resyncFromDaemon + 跳会话）。
  /// null = 本波未接线，**不渲染重试按钮**。
  final ValueChanged<String>? onRetryMessage;

  /// 重新下载（Kotlin = PaDownloader.downloadFromUrl/downloadFromPc）。
  /// null = 本波未接线，**不渲染重下按钮**。
  final ValueChanged<TaskItem>? onRetryDownload;

  /// 时钟注入点（测试用）：冷却门闩的 `tryBegin(now)` 与「重试中」占位的计时都走它。
  ///
  /// 【为什么默认是系统时钟】生产就是真时钟（Kotlin `System.currentTimeMillis()`
  /// 同款）；测试注入可控时钟才能钉死「冷却内被吞 / 冷却外放行」而不真等 1.5 秒
  /// ——widget 测试的 fake_async 只接管 `Future.delayed`，不接管系统时钟。
  final int Function()? nowMs;

  @override
  State<TaskCenterPage> createState() => _TaskCenterPageState();
}

/// 两页签（Kotlin `kindTab`：MESSAGE / DOWNLOAD）。
enum _TaskKindTab { message, download }

class _TaskCenterPageState extends State<TaskCenterPage> {
  late final TaskCenterStore _store = widget.store ?? TaskCenterStore();
  late List<TaskItem> _tasks = _store.snapshot;
  late int _unread = _store.unreadCount;
  _TaskKindTab _kindTab = _TaskKindTab.message;

  /// 每行一门重试门闩（Kotlin `retryGates: ConcurrentHashMap<String, RetryGate>`）。
  final Map<String, TaskRetryGate> _gates = <String, TaskRetryGate>{};

  /// 「重试中」占位的任务 id 集合（Kotlin `retryingIds`）。
  final Set<String> _retryingIds = <String>{};

  /// 当前时钟（生产 = 系统时钟；测试注入可控时钟，见 [TaskCenterPage.nowMs]）。
  int _now() => (widget.nowMs ?? _systemNowMs)();

  static int _systemNowMs() => DateTime.now().millisecondsSinceEpoch;

  @override
  void initState() {
    super.initState();
    // 订阅 store 变更（本地操作与后续真写入方都走这条通知）。
    _store.tasks.listen((List<TaskItem> next) {
      if (!mounted) return;
      setState(() => _tasks = next);
    });
  }

  /// 顶部未读徽标点按 = 全部标为已读（见文件头「已读策略」）。
  void _markAllRead() {
    setState(() {
      _store.markAllRead();
      // 本地同步 store：markAllRead 的变更会经 stream 回来，这里先同步一份，
      // 让徽标在同一帧就归零（stream 是异步派发，等它会让按钮「慢半拍」）。
      _unread = _store.unreadCount;
      _tasks = _store.snapshot;
    });
  }

  void _markRead(String id) => setState(() => _store.markRead(id));

  void _remove(String id) => setState(() => _store.remove(id));

  void _clearFinished() => setState(() => _store.clearFinished());

  /// 行尾重试统一入口：门闩 + 离线门控 + 分派（Kotlin `requestRetry` 的等价逻辑）。
  ///
  /// 【冷却计时】门闩判定走 [TaskCenterPage.nowMs]（生产 = 系统时钟，与 Kotlin
  /// `System.currentTimeMillis()` 同源）；「重试中」占位的摘除走
  /// `Future.delayed(cooldown)`——两者是同一段时间的两种读法（Kotlin 也是
  /// `System.currentTimeMillis` + `delay` 各管一段）：fake-async 推进
  /// `Future.delayed` 即可驱动 UI 侧，时钟侧由测试注入钉死。
  void _requestRetry(TaskItem task) {
    final bool isDownload = task.kind == TaskKind.download;
    // 断连时消息类与「要电脑在线的重下」不给入口（Kotlin 同款提前返回）。
    if (widget.hostOffline &&
        (!isDownload ||
            redownloadNeedsPc(redownloadPlanOf(taskFileKey(task))) == true)) {
      return;
    }
    final TaskRetryGate gate = _gates.putIfAbsent(
      task.id,
      TaskRetryGate.standard,
    );
    if (!gate.tryBegin(_now())) return;
    setState(() => _retryingIds.add(task.id));
    if (isDownload) {
      widget.onRetryDownload!(task);
    } else {
      widget.onRetryMessage!(task.threadId);
    }
    Future<void>.delayed(
      const Duration(milliseconds: kTaskRetryCooldownMs),
      () {
        if (!mounted) return;
        if (_retryingIds.remove(task.id)) setState(() {});
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final List<TaskItem> rows = _tasks
        .where((TaskItem t) =>
            _kindTab == _TaskKindTab.message
                ? t.kind == TaskKind.message
                : t.kind == TaskKind.download)
        .toList(growable: false);
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
      children: <Widget>[
        _buildStatsRow(pa),
        const SizedBox(height: 12),
        _buildKindTabs(pa),
        const SizedBox(height: 6),
        if (rows.isEmpty)
          _buildEmpty(pa)
        else ...<Widget>[
          for (final TaskPartitionRow row
              in taskPartitionRows(rows, hostOffline: widget.hostOffline)) ...<
              Widget>[
            _PartitionHeader(word: row.word),
            for (final TaskItem task in row.tasks)
              _TaskRow(
                task: task,
                hostOffline: widget.hostOffline,
                retrying: _retryingIds.contains(task.id),
                // 重试出口：消息类看 onRetryMessage；下载类看 onRetryDownload
                // 且要求有可重放来源（NONE 计划不给假重试，Kotlin 同款）。
                canRetry: task.kind == TaskKind.message
                    ? (widget.onRetryMessage != null &&
                        !widget.hostOffline &&
                        task.threadId.isNotEmpty)
                    : (widget.onRetryDownload != null &&
                        redownloadPlanOf(taskFileKey(task)) !=
                            RedownloadPlan.none &&
                        !(widget.hostOffline &&
                            redownloadNeedsPc(
                                  redownloadPlanOf(taskFileKey(task)),
                                ) ==
                                true)),
                onOpen: () => _onOpenTask(task),
                onRemove: () => _remove(task.id),
                onRetry: () => _requestRetry(task),
              ),
          ],
        ],
        // 底部「清理已完成」：有终局任务才出现（Kotlin hasFinished 同款）。
        if (_tasks.any((TaskItem t) => t.status != TaskStatus.running)) ...<Widget>[
          const SizedBox(height: 16),
          PAButton(
            key: const ValueKey<String>('task-center-clear-finished'),
            label: '清理已完成的任务',
            variant: PAButtonVariant.neutral,
            expanded: false,
            onPressed: _clearFinished,
          ),
        ],
      ],
    );
  }

  /// 点任务行：先标已读，再按类别分派出口（Kotlin 行 onClick 同款顺序）。
  void _onOpenTask(TaskItem task) {
    _markRead(task.id);
    if (task.kind == TaskKind.message) {
      if (task.threadId.isNotEmpty) widget.onOpenThread?.call(task.threadId);
      return;
    }
    widget.onOpenUserData?.call();
  }

  // ── 顶部统计三卡 + 未读徽标 ──────────────────────────────────────────────

  Widget _buildStatsRow(PAColors pa) {
    final TaskCenterStats stats = taskStatsOf(_tasks);
    // 【为什么包 IntrinsicHeight】ListView 给子节点的 height 约束是 Infinity，
    // Row 的 stretch 会把「无限高」直接灌给四张卡（RenderConstrainedBox 断言炸）。
    // IntrinsicHeight 先用子节点的内在高度定行高，stretch 才有 finite 值可拉——
    // 四张卡因此等高（Kotlin 的 Row(Modifier.weight) 在 Compose 里同效）。
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          _StatCard(
            value: '${stats.waitingForYou}',
            label: '等待你处理',
            valueColor: pa.warn,
          ),
          const SizedBox(width: 8),
          _StatCard(
            value: '${stats.running}',
            label: '正在执行',
            valueColor: pa.brand,
          ),
          const SizedBox(width: 8),
          _StatCard(
            value: '${stats.completed}',
            label: '已完成',
            valueColor: pa.ok,
          ),
          // 未读徽标：点按 = 全部已读（Kotlin 的未读只进 AppDrawer 角标，
          // 这里给页面内同源的一个显式动作，见文件头「已读策略」）。
          const SizedBox(width: 8),
          _StatCard(
            value: '$_unread',
            label: _unread > 0 ? '未读 · 点按全读' : '未读',
            valueColor: pa.agentAccent,
            onTap: _unread > 0 ? _markAllRead : null,
            key: const ValueKey<String>('task-center-unread'),
          ),
        ],
      ),
    );
  }

  // ── 两页签 ──────────────────────────────────────────────────────────────

  Widget _buildKindTabs(PAColors pa) {
    return Row(
      children: <Widget>[
        Expanded(
          child: _KindTab(
            label: '消息任务',
            selected: _kindTab == _TaskKindTab.message,
            onTap: () => setState(() => _kindTab = _TaskKindTab.message),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: _KindTab(
            label: '文件下载',
            selected: _kindTab == _TaskKindTab.download,
            onTap: () => setState(() => _kindTab = _TaskKindTab.download),
          ),
        ),
      ],
    );
  }

  Widget _buildEmpty(PAColors pa) {
    final bool messageTab = _kindTab == _TaskKindTab.message;
    return Padding(
      padding: const EdgeInsets.only(top: 40),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(
              messageTab
                  ? Icons.playlist_add_check_rounded
                  : Icons.download_rounded,
              size: 34,
              color: pa.fgTertiary,
            ),
            const SizedBox(height: 10),
            Text(
              messageTab ? '还没有处理中的任务' : '还没有下载任务',
              style: PATypography.body.copyWith(
                color: pa.fgPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              messageTab
                  ? '在会话里发一条消息，这里会显示它的进度'
                  : '在会话里点文件旁的下载，进度会汇总到这里',
              textAlign: TextAlign.center,
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
        ),
      ),
    );
  }
}

// ── 零件 ──────────────────────────────────────────────────────────────────

/// 顶部统计卡（Kotlin `StatCard`：大数字 + 小标签；可点的当按钮用）。
class _StatCard extends StatelessWidget {
  const _StatCard({
    super.key,
    required this.value,
    required this.label,
    required this.valueColor,
    this.onTap,
  });

  final String value;
  final String label;
  final Color valueColor;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final Widget content = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text(
          value,
          style: PATypography.titleM.copyWith(color: valueColor),
        ),
        const SizedBox(height: 2),
        Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: PATypography.caption.copyWith(color: pa.fgTertiary),
        ),
      ],
    );
    return Expanded(
      child: PACard(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        onTap: onTap,
        child: content,
      ),
    );
  }
}

/// 页签（Kotlin `KindTab`：选中 = 实底，未选 = 描边）。
class _KindTab extends StatelessWidget {
  const _KindTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        height: 40,
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: selected ? pa.bgSurface2 : pa.bgSurface,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: selected ? pa.brand : pa.hairline,
          ),
        ),
        child: Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: PATypography.bodySmall.copyWith(
            color: selected ? pa.fgPrimary : pa.fgSecondary,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ),
    );
  }
}

/// 分区头（Kotlin：分区头 = 状态词本身，与行内徽章同源）。
class _PartitionHeader extends StatelessWidget {
  const _PartitionHeader({required this.word});

  final String word;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(top: 16, bottom: 2),
      child: Text(
        word,
        style: PATypography.titleM.copyWith(color: pa.fgPrimary),
      ),
    );
  }
}

/// 单行任务（Kotlin `TaskRow` 的直译：图标块 + 标题/元信息/状态/下一步 + 行尾动作）。
class _TaskRow extends StatelessWidget {
  const _TaskRow({
    required this.task,
    required this.hostOffline,
    required this.retrying,
    required this.canRetry,
    required this.onOpen,
    required this.onRemove,
    required this.onRetry,
  });

  final TaskItem task;
  final bool hostOffline;
  final bool retrying;
  final bool canRetry;
  final VoidCallback onOpen;
  final VoidCallback onRemove;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final bool isDownload = task.kind == TaskKind.download;
    final PAStatusKind statusKind = taskStatusKindOf(task.status);
    final Color statusColor = PAStatusChip.colorOf(statusKind, pa);
    final String? offlineHint =
        taskOfflineHint(task, hostOffline: hostOffline);
    // 断连时消息类运行中：徽章隐去，改显离线提示（Kotlin taskStatusView 同口径）。
    final bool showStatusBadge = offlineHint == null;
    final RedownloadPlan plan =
        isDownload ? redownloadPlanOf(taskFileKey(task)) : RedownloadPlan.none;
    final bool? needsPc = redownloadNeedsPc(plan);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Container(
            width: 40,
            height: 40,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: pa.bgSurface2,
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(
              isDownload ? Icons.download_rounded : Icons.chat_bubble_rounded,
              size: 19,
              color: statusColor,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: onOpen,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          task.title.isEmpty ? '(未命名)' : task.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: PATypography.body.copyWith(
                            color: pa.fgPrimary,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                      if (task.rounds > 1) ...<Widget>[
                        const SizedBox(width: 6),
                        Text(
                          '${task.rounds} 轮',
                          style: PATypography.caption
                              .copyWith(color: pa.fgTertiary),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  Row(
                    children: <Widget>[
                      Text(
                        taskRowTimeLabel(task.startedAtMs),
                        style: PATypography.caption
                            .copyWith(color: pa.fgTertiary),
                      ),
                      if (showStatusBadge) ...<Widget>[
                        const SizedBox(width: 8),
                        PAStatusChip(
                          status: statusKind,
                          label: taskStatusLabel(task.status),
                        ),
                      ],
                    ],
                  ),
                  if (offlineHint != null) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      offlineHint,
                      style: PATypography.caption
                          .copyWith(color: pa.fgSecondary),
                    ),
                  ],
                  // 失败/中断行：说清「为什么」+「下一步」（与行尾真实按钮对齐）。
                  if (task.status == TaskStatus.failed ||
                      task.status == TaskStatus.interrupted) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      taskNextStepCopy(
                        kind: task.kind,
                        hostOffline: hostOffline,
                        redownloadNeedsPc: isDownload ? needsPc : null,
                        hasSourceThread: task.threadId.isNotEmpty,
                        hasRetryAction: canRetry,
                      ),
                      style: PATypography.caption
                          .copyWith(color: pa.fgSecondary),
                    ),
                  ],
                  if (task.sourceTitle.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      '来自会话：${task.sourceTitle}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: PATypography.caption
                          .copyWith(color: pa.fgTertiary),
                    ),
                  ],
                  if (task.detail.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      task.detail,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: PATypography.caption
                          .copyWith(color: pa.fgTertiary),
                    ),
                  ],
                  if (task.sizeText.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      task.sizeText,
                      style: PATypography.caption
                          .copyWith(color: pa.fgTertiary),
                    ),
                  ],
                  // 进行中：静态进度条（Kotlin 是无限扫动动画，见文件头说明）。
                  if (task.status == TaskStatus.running) ...<Widget>[
                    const SizedBox(height: 8),
                    _TaskProgressBar(
                      progress: task.progress,
                      color: statusColor,
                    ),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(width: 8),
          _buildTrailing(pa),
        ],
      ),
    );
  }

  /// 行尾动作（Kotlin 的 when 链直译）：重试中占位 > 重试/重下 > 移除。
  ///
  /// 「重试中」占位只覆盖真的会渲染出来的重试/重下按钮（[canRetry] 为 false 时
  /// 这个状态根本到不了，Kotlin 同款条件）。
  ///
  /// 【为什么每个按钮带 ValueKey】同一屏会有多行、每行都有移除按钮，且失败行的
  /// 状态 chip 里也有 close 形图标——测试按任务 id + 动作定位，不靠图标文本猜。
  Widget _buildTrailing(PAColors pa) {
    final bool isDownload = task.kind == TaskKind.download;
    final String keyPrefix = 'task-row-${task.id}';
    if (retrying && canRetry) {
      return SizedBox(
        key: ValueKey<String>('$keyPrefix-retrying'),
        width: 40,
        height: 40,
        child: Center(
          child: Text(
            '重试中',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ),
      );
    }
    if (canRetry) {
      return _RowAction(
        actionKey: '$keyPrefix-retry',
        icon: isDownload ? Icons.download_rounded : Icons.refresh_rounded,
        label: isDownload ? '重新下载' : '重试',
        color: pa.brand,
        onTap: onRetry,
      );
    }
    // 下载类「已完成」没有打开文件出口（PaDownloader.openLocalFile 未接线）：
    // 点整行进用户数据管理（onOpen 已分派），行尾只留移除。
    return _RowAction(
      actionKey: '$keyPrefix-remove',
      icon: Icons.close_rounded,
      label: '移除',
      color: pa.fgTertiary,
      onTap: onRemove,
    );
  }
}

/// 行尾小按钮（Kotlin `RowActionIcon`：44dp 触控区，无文字）。
class _RowAction extends StatelessWidget {
  const _RowAction({
    required this.actionKey,
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  });

  /// 稳定定位键：`task-row-<taskId>-{retry,remove}`（测试与无障碍共用）。
  final String actionKey;

  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      key: ValueKey<String>(actionKey),
      width: 40,
      height: 40,
      child: IconButton(
        onPressed: onTap,
        tooltip: label,
        padding: EdgeInsets.zero,
        constraints: const BoxConstraints(),
        icon: Icon(icon, size: 18, color: color),
      ),
    );
  }
}

/// 任务进度条：4dp 细轨；有进度按比例填充，无进度静态 40%（见页面文件头）。
class _TaskProgressBar extends StatelessWidget {
  const _TaskProgressBar({required this.progress, required this.color});

  final double progress;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final double fraction = progress > 0 && progress <= 1 ? progress : 0.4;
    return ClipRRect(
      borderRadius: BorderRadius.circular(2),
      child: SizedBox(
        height: 4,
        child: Stack(
          children: <Widget>[
            Container(color: pa.hairline),
            FractionallySizedBox(
              widthFactor: fraction,
              child: Container(color: color),
            ),
          ],
        ),
      ),
    );
  }
}
