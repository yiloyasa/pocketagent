// 协同（Collab）主面板的模型 + 纯函数层（B4 第 4 波）。
//
// 【它是什么】Android `ui/CollabState.kt`（派发前配置）+ `ui/collab/` 一族
// （CollabSessionState / CollabSessionReducer / CollabCopy，948 行）的 Dart 对位：
// daemon 是协同状态的唯一事实来源，本文件只是它的**客户端投影**——快照
// （List/Get）恢复整屏，事件流跟随留 B5（transport 无 stream 面，见文件末
// 「留 B5」清单）。投影永不编造 daemon 没给的事实：受理不写成完成，等待不是
// 终局，没有耗时字段就不显示耗时。
//
// 【为什么单拆一个文件】页面（collab.dart）只留界面；状态词表、动作可用性、
// 派发判定、徽章映射这些「最容易被顺手改个条件悄悄反转且零报错」的规则集中成
// 纯函数，单测直接命中（collab_session_data_test.dart）——与 Kotlin 把同一批
// 规则抽成 CollabSessionReducer/CollabLogic 是同一个理由。
//
// 【文案为什么只有一份】Kotlin 有两份 CollabCopy（ui/CollabLogic.kt 管派发前、
// ui/collab/CollabCopy.kt 管派发后，靠别名 import 区分）。Dart 侧没有包层级
// 冲突，合并成一份 [CollabCopy]，两段注释各标来源；措辞逐字对位（含半角逗号），
// 双端同一句话长同一样。

import '../components/pa_status_chip.dart';

// ── 七词状态词表 ────────────────────────────────────────────────────────────

/// 协同状态（七词词表，Task 7 统一口径；人话词见 [CollabCopy.statusWord]）：
///
///   已提交(SUBMITTED，客户端事实：指令已发出、daemon 真值未达) → 已受理
///   (ACCEPTED) → 执行中(RUNNING) → 等待确认(WAITING_APPROVAL) → 已完成
///   (COMPLETED) / 未完成(FAILED) / 已取消(CANCELLED)。
///
/// 【为什么保留 daemon 原文】daemon 侧状态是 string 口径（proto 注释：状态集
/// 稳定后再冻结枚举），新版本可能给出本客户端不认识的取值——映射为
/// [unknown] 并把原文留在 [CollabMemberUi.statusRaw] / [CollabSessionUi.statusRaw]，
/// 展示层如实降级，不猜、不编一个相近的已知状态。
enum CollabStatus {
  /// 客户端事实：DispatchCollab 已发出，快照/受理回执还没回来。
  submitted(''),

  accepted('ACCEPTED'),
  running('RUNNING'),
  waitingApproval('WAITING_APPROVAL'),
  completed('COMPLETED'),
  failed('FAILED'),
  cancelled('CANCELLED'),

  /// daemon 给了本客户端不认识的状态串（新版本新增）；原文见 statusRaw。
  unknown('');

  const CollabStatus(this.wireName);

  /// daemon 状态原文（已知状态时与 [wireName] 同值；[unknown] 恒空，原文另存）。
  final String wireName;

  /// 终局判定（与 daemon `store.CollabStatusIsFinished` 同口径）：终局不复活。
  bool get isFinished =>
      this == CollabStatus.completed ||
      this == CollabStatus.failed ||
      this == CollabStatus.cancelled;

  /// daemon 状态串 → 状态（纯函数）：认识的按词表落，其余（含空串）落
  /// [unknown]——不猜、不就近凑一个已知状态。
  static CollabStatus fromWire(String raw) => switch (raw) {
    'ACCEPTED' => CollabStatus.accepted,
    'RUNNING' => CollabStatus.running,
    'WAITING_APPROVAL' => CollabStatus.waitingApproval,
    'COMPLETED' => CollabStatus.completed,
    'FAILED' => CollabStatus.failed,
    'CANCELLED' => CollabStatus.cancelled,
    _ => CollabStatus.unknown,
  };
}

/// 成员卡上的一笔挂起审批（映射 proto `ApprovalRequest` 的最小可展示集）。
class CollabApprovalUi {
  const CollabApprovalUi({required this.requestId, required this.label});

  /// 真实 request_id：审批必须对应真实 thread_id/request_id（全局红线）。
  final String requestId;

  /// 展示用短句（daemon 侧把工具名/理由放进来，原样透传，不加工）。
  final String label;
}

/// 协同成员卡的客户端投影（整卡真值，非 diff；对应 proto `CollabMemberSnapshot`）。
class CollabMemberUi {
  const CollabMemberUi({
    required this.agentId,
    required this.roleId,
    required this.threadId,
    required this.status,
    required this.statusRaw,
    required this.modelId,
    required this.effort,
    required this.progress,
    required this.summary,
    required this.error,
    required this.pendingApprovals,
    required this.workspace,
  });

  final String agentId;
  final String roleId;

  /// 该成员的子线程 id：取消/重试/查看会话都按它定位；受理失败时为空。
  final String threadId;
  final CollabStatus status;

  /// daemon 状态原文（[CollabStatus.unknown] 降级展示用；已知状态时与 wire 同）。
  final String statusRaw;
  final String modelId;
  final String effort;

  /// daemon 侧有界的一句话进度（不是原始日志；空 = 无）。
  final String progress;

  /// 成功时的终稿摘要；失败/取消时为空，不伪造。
  final String summary;

  /// 失败/取消的人话原因（daemon 原文；空 = 无）。
  final String error;

  final List<CollabApprovalUi> pendingApprovals;

  /// 该成员实际工作区（隔离 worktree 时与会话级不同；空 = 未报）。
  final String workspace;

  /// 成员卡在状态树里的定位：有子线程按线程（取消/重试同锚）；受理失败的卡
  /// 没有线程，按 agent 兜位（对位 Kotlin `CollabMemberUi.key()`）。
  String get key => threadId.isNotEmpty ? 't:$threadId' : 'a:$agentId';
}

/// 成员计数快照（汇总行的原料；「N/M 已完成 · X 个等待确认」由
/// [collabSummaryLine] 拼装）。
class CollabSessionCounts {
  const CollabSessionCounts({
    required this.total,
    required this.completed,
    required this.failed,
    required this.cancelled,
    required this.waiting,
    required this.active,
  });

  final int total;
  final int completed;
  final int failed;
  final int cancelled;

  /// 等审批（等待确认）成员数。
  final int waiting;

  /// 仍在受理/执行中的成员数。
  final int active;
}

/// 协同会话状态树（对应 proto `CollabSessionSnapshot` 的客户端投影）。
///
/// 【没有 lastSequence / needsResync】那两个字段服务事件流（帧按序应用、缺口
/// 标记全量重建）。本波 transport 不做 stream（WatchCollabSession 留 B5），
/// 整屏由一次性快照重建，不需要断点与缺口标记——不预埋本波用不到的字段。
class CollabSessionUi {
  const CollabSessionUi({
    this.sessionId,
    this.status = CollabStatus.submitted,
    this.statusRaw = '',
    this.instruction = '',
    this.sessionError = '',
    this.workspace = '',
    this.members = const <CollabMemberUi>[],
    this.loading = false,
    this.error,
  });

  /// 当前协同会话 id；null = 本次没有会话（还没派发，或 daemon 没报进行中会话）。
  final String? sessionId;

  final CollabStatus status;

  /// daemon 状态原文（[CollabStatus.unknown] 降级展示用）。
  final String statusRaw;

  /// 全体成员共用的那条指令原文。
  final String instruction;

  /// 会话级人话原因（daemon 给的；FAILED/CANCELLED 时非空）。
  final String sessionError;

  /// 协同工作区（未隔离时各成员共享；空 = 未报）。
  final String workspace;

  final List<CollabMemberUi> members;

  /// 正在做恢复/重建（GetCollabSession 在飞）。
  final bool loading;

  /// 传输层/操作层错误的人话呈现（daemon 会话级错误在 [sessionError]）。
  final String? error;

  /// 空状态（进入页面、尚未恢复/尚未派发）。
  static const CollabSessionUi initial = CollabSessionUi();
}

/// 会话列表行（`ListCollabSessions` 的投影：id / 指令摘要 / 状态 / 成员数 / 时间）。
///
/// 【为什么另立一个行模型】列表只需要五个字段，整张 `CollabSessionSnapshot`
/// （含全体成员卡）拖进来会让「渲染什么」和「wire 有什么」失去区分；映射是
/// 纯函数（collab_bridge.dart），行上的每个字都能单测钉死从哪来。
class CollabSessionRow {
  const CollabSessionRow({
    required this.id,
    required this.instruction,
    required this.status,
    required this.statusRaw,
    required this.memberCount,
    required this.roleNames,
    required this.timeLabel,
  });

  /// 会话 id（GetCollabSession / 三个写动作都按它定位）。
  final String id;

  /// 指令原文（列表行可能截断显示，字段给全量）。
  final String instruction;

  final CollabStatus status;
  final String statusRaw;

  /// 成员数（daemon 快照的 members 长度；0 = 还没有成员受理回执）。
  final int memberCount;

  /// 成员角色显示名（未配置角色 / 空角色名的成员不占位；顺序 = 受理顺序）。
  final List<String> roleNames;

  /// 展示用时间标签（[collabSessionTimeLabel] 的产物；空 = daemon 没给时间）。
  final String timeLabel;
}

/// 协同策略的只读投影（对应 proto `CollabPolicy` + `CollabPolicyResponse`）。
///
/// 【为什么 effective 与 policy 都要】`policy` 是用户配置（未设置的项缺席 =
/// 沿用默认），`effective` 是打底默认之后的实际生效值（全字段齐备）。策略区
/// 展示「实际生效」的事实，所以以 [effective*] 为准；[revision] / [policyPath]
/// 只是附注，本波不写（SetCollabPolicy 留 B5，甲级写操作）。
class CollabPolicyUi {
  const CollabPolicyUi({
    this.collaborationEnabled,
    this.nativeDelegateEnabled,
    this.fanoutEnabled,
    this.maxParallelMembers,
    this.effectiveCollaborationEnabled,
    this.effectiveNativeDelegateEnabled,
    this.effectiveFanoutEnabled,
    this.effectiveMaxParallelMembers,
    this.revision = 0,
    this.policyPath = '',
  });

  final bool? collaborationEnabled;
  final bool? nativeDelegateEnabled;
  final bool? fanoutEnabled;
  final int? maxParallelMembers;

  final bool? effectiveCollaborationEnabled;
  final bool? effectiveNativeDelegateEnabled;
  final bool? effectiveFanoutEnabled;
  final int? effectiveMaxParallelMembers;

  /// 每次成功写入 +1（只读展示，本波不写）。
  final int revision;

  /// 落盘位置（daemon 侧配置文件路径；空 = 未报）。
  final String policyPath;

  /// 空策略（daemon 什么都没报）：页面据此说「电脑端没有返回策略」。
  static const CollabPolicyUi empty = CollabPolicyUi();
}

/// 派发成员材料（选人 + 角色绑定；页面成品，gateway 不再读选人状态）。
class CollabMemberSpec {
  const CollabMemberSpec({required this.agentId, this.roleId});

  final String agentId;

  /// 协同角色 id；null/空 = 未配置（daemon 用 agent 默认）。
  final String? roleId;
}

/// 单个成员的派发受理结果（对应 proto `DispatchCollabItem`）。
///
/// 【一位失败不拖垮其余】并行扇出的应有语义：items 与请求 members 一一对应，
/// 失败项带 daemon 的人话原因，页面逐位如实呈现，不因一位失败整单报错。
class CollabDispatchItemUi {
  const CollabDispatchItemUi({
    required this.agentId,
    required this.threadId,
    required this.ack,
    required this.error,
  });

  final String agentId;

  /// 为该成员建的子线程 id；受理失败时为空。
  final String threadId;

  /// 是否受理（子线程已建、轮次已起飞）。受理 ≠ 执行成功。
  final bool ack;

  /// 受理失败的人话原因（agent 不存在 / 角色未定义等）；成功时为空。
  final String error;
}

/// 派发结果（对应 proto `DispatchCollabResponse`）。
class CollabDispatchOutcome {
  const CollabDispatchOutcome({required this.sessionId, required this.items});

  /// 本次协同的会话 id（便于前端归组与事后回溯）。
  final String sessionId;

  final List<CollabDispatchItemUi> items;
}

// ── reducer 纯函数（对位 CollabSessionReducer / CollabLogic）────────────────

/// 七词状态 → 人话词（纯函数）。[unknown] 不编词：返回 daemon 原文（认识的中
/// 文化，不认识的原样显示）；原文也空时给一句如实的「电脑端没有给出状态」。
String collabStatusWord(CollabStatus status, [String? raw]) => switch (status) {
  CollabStatus.submitted => CollabCopy.wordSubmitted,
  CollabStatus.accepted => CollabCopy.wordAccepted,
  CollabStatus.running => CollabCopy.wordRunning,
  CollabStatus.waitingApproval => CollabCopy.wordWaitingApproval,
  CollabStatus.completed => CollabCopy.wordCompleted,
  CollabStatus.failed => CollabCopy.wordFailed,
  CollabStatus.cancelled => CollabCopy.wordCancelled,
  CollabStatus.unknown =>
    (raw != null && raw.trim().isNotEmpty) ? raw : CollabCopy.statusAbsent,
};

/// 七词状态 → 设计系统五态徽章（只借色档与形通道，**词**仍用七词）。
///
/// [CollabStatus.unknown] 返回 null：未知状态不进任何已知徽章，由调用方走
/// 「原文 + 降级说明」的诚实呈现（对位 Kotlin `editorialBadgeOf`）。
PAStatusKind? collabBadgeKindOf(CollabStatus status) => switch (status) {
  CollabStatus.submitted ||
  CollabStatus.accepted ||
  CollabStatus.running => PAStatusKind.running,
  CollabStatus.waitingApproval => PAStatusKind.pending,
  CollabStatus.completed => PAStatusKind.done,
  CollabStatus.failed || CollabStatus.cancelled => PAStatusKind.failed,
  CollabStatus.unknown => null,
};

/// 会话级状态由成员卡派生（不独立记账）：有人在跑 → RUNNING；有人等审批 →
/// WAITING_APPROVAL；全员终局按 FAILED > CANCELLED > COMPLETED 聚合；空名册 →
/// ACCEPTED。UNKNOWN 成员不计入任何桶（与 daemon 的 switch 落空一致）。
CollabStatus collabDeriveStatus(List<CollabMemberUi> members) {
  if (members.isEmpty) return CollabStatus.accepted;
  var running = false;
  var waiting = false;
  var failed = false;
  var cancelled = false;
  for (final CollabMemberUi m in members) {
    switch (m.status) {
      case CollabStatus.accepted:
      case CollabStatus.running:
        running = true;
      case CollabStatus.waitingApproval:
        waiting = true;
      case CollabStatus.failed:
        failed = true;
      case CollabStatus.cancelled:
        cancelled = true;
      case CollabStatus.completed:
      case CollabStatus.submitted:
      case CollabStatus.unknown:
        break;
    }
  }
  if (running) return CollabStatus.running;
  if (waiting) return CollabStatus.waitingApproval;
  if (failed) return CollabStatus.failed;
  if (cancelled) return CollabStatus.cancelled;
  return CollabStatus.completed;
}

/// 成员计数（汇总行的原料）。
CollabSessionCounts collabMemberCounts(List<CollabMemberUi> members) {
  var completed = 0;
  var failed = 0;
  var cancelled = 0;
  var waiting = 0;
  var active = 0;
  for (final CollabMemberUi m in members) {
    switch (m.status) {
      case CollabStatus.completed:
        completed++;
      case CollabStatus.failed:
        failed++;
      case CollabStatus.cancelled:
        cancelled++;
      case CollabStatus.waitingApproval:
        waiting++;
      case CollabStatus.accepted:
      case CollabStatus.running:
      case CollabStatus.submitted:
        active++;
      case CollabStatus.unknown:
        break;
    }
  }
  return CollabSessionCounts(
    total: members.length,
    completed: completed,
    failed: failed,
    cancelled: cancelled,
    waiting: waiting,
    active: active,
  );
}

/// 「N/M 已完成 · X 个等待确认」汇总行（绝不显示笼统的「运行中」）。
///
///  - 比例段永远在（0/3 也如实说）；
///  - 有等待确认的成员才追加「等待确认」段；
///  - 有未完成（失败/取消）的成员才追加「未完成」段；
///  - 没有成员返回 null，不占位。
String? collabSummaryLine(CollabSessionCounts counts) {
  if (counts.total <= 0) return null;
  final List<String> parts = <String>[
    '${counts.completed}/${counts.total} ${CollabCopy.wordCompleted}',
  ];
  if (counts.waiting > 0) {
    parts.add('${counts.waiting} 个${CollabCopy.wordWaitingApproval}');
  }
  final int notCompleted = counts.failed + counts.cancelled;
  if (notCompleted > 0) parts.add('$notCompleted 个${CollabCopy.wordFailed}');
  return parts.join(' · ');
}

/// 单个成员卡的动作可用性：每个入口的可点态与禁用原因由同一个纯函数给出——
/// 永远不出现「按钮灰了但界面上找不到原因」（对位 Kotlin `memberActions`）。
class CollabMemberActions {
  const CollabMemberActions({
    required this.canOpenSession,
    required this.openSessionReason,
    required this.canRetry,
    required this.retryReason,
    required this.canCancel,
    required this.cancelReason,
  });

  final bool canOpenSession;
  final String? openSessionReason;
  final bool canRetry;
  final String? retryReason;
  final bool canCancel;
  final String? cancelReason;
}

CollabMemberActions collabMemberActions(CollabMemberUi m) {
  final bool hasThread = m.threadId.isNotEmpty;
  final bool canRetry = m.status.isFinished && hasThread;
  final bool canCancel = !m.status.isFinished && hasThread;
  // 禁用原因优先级与 Kotlin `memberActions` 逐条同序：先看有没有子线程
  // （取消/重试/查看的共同前置），再看状态前置条件。
  final String? retryReason = canRetry
      ? null
      : hasThread
      ? CollabCopy.reasonRetryActive
      : CollabCopy.reasonNoThread;
  final String? cancelReason = canCancel
      ? null
      : hasThread
      ? CollabCopy.reasonCancelFinished
      : CollabCopy.reasonNoThread;
  return CollabMemberActions(
    canOpenSession: hasThread,
    openSessionReason: hasThread ? null : CollabCopy.reasonOpenNoThread,
    canRetry: canRetry,
    retryReason: retryReason,
    canCancel: canCancel,
    cancelReason: cancelReason,
  );
}

/// 会话级动作（取消整个协同）。
class CollabSessionActions {
  const CollabSessionActions({required this.canCancel, this.cancelReason});

  final bool canCancel;
  final String? cancelReason;
}

CollabSessionActions collabSessionActions(CollabSessionUi session) {
  final bool canCancel =
      session.members.isNotEmpty &&
      session.members.any((m) => !m.status.isFinished);
  return CollabSessionActions(
    canCancel: canCancel,
    cancelReason: canCancel ? null : CollabCopy.reasonSessionAllFinished,
  );
}

/// 派发判定结果：[canDispatch] 决定按钮可点，[disabledReason] 决定按钮旁边那句
/// 「为什么现在点不了」。两件事由同一个纯函数给出（对位 `collabDispatchDecision`）。
class CollabDispatchDecision {
  const CollabDispatchDecision({
    required this.canDispatch,
    this.disabledReason,
  });

  final bool canDispatch;

  /// 不可派发的人话原因；[canDispatch] = true 时为 null。
  final String? disabledReason;
}

/// 可派发判定（纯函数）：至少一位参与者 + 指令非空 + 没有正在飞的一单。
///
/// 原因优先级 = 参与者 > 指令 > 提交中（用户按「从上往下补条件」的顺序补齐）。
CollabDispatchDecision collabDispatchDecision({
  required int selectedCount,
  required bool instructionReady,
  required bool dispatching,
}) {
  if (selectedCount <= 0) {
    return const CollabDispatchDecision(
      canDispatch: false,
      disabledReason: CollabCopy.reasonNoParticipant,
    );
  }
  if (!instructionReady) {
    return const CollabDispatchDecision(
      canDispatch: false,
      disabledReason: CollabCopy.reasonNoInstruction,
    );
  }
  if (dispatching) {
    return const CollabDispatchDecision(
      canDispatch: false,
      disabledReason: CollabCopy.reasonDispatchInFlight,
    );
  }
  return const CollabDispatchDecision(canDispatch: true, disabledReason: null);
}

/// 参与区的选中数摘要（trailing 小字）；0 人也如实说，不藏数字。
String collabSelectionSummary(int selectedCount) =>
    selectedCount <= 0 ? '未选参与者' : '已选 $selectedCount 位';

/// 从 capabilities 提取「已接入协同」的 agent id（纯函数）。
///
/// daemon 报 `collab:<agentId>`——语义是「该 agent 已把本地 collab MCP 挂进
/// 自己的会话」。判定完全由 daemon 的接线事实驱动（新增一家自动出现），手机端
/// **不写名单**；能力缺失 = 该家尚未接线，如实不显示，不做假入口。
/// 口径对位 Android `resolveCollabAgentIds`（HomeScreen.kt:2216）：去空、去重、
/// 排序（同数据两次渲染同序，不跳动）。
List<String> resolveCollabAgentIds(List<String> capabilities) {
  const String family = 'collab:';
  final Set<String> ids = <String>{};
  for (final String bit in capabilities) {
    if (!bit.toLowerCase().startsWith(family)) continue;
    final String id = bit.substring(family.length).trim();
    if (id.isNotEmpty) ids.add(id);
  }
  final List<String> out = ids.toList()..sort();
  return out;
}

/// agent 多选：把 id 加进/移出已选集合（纯函数，加减对称）。
///
/// 【为什么不原地改】调用方可能传 const 集合（编译期常量）或不可变视图，原地
/// `..add`/`..remove` 会在这些入参上抛 Unsupported operation；纯函数语义 =
/// 返回新集合，入参一个字不动。
Set<String> toggleCollabSelection(Set<String> selected, String id) {
  final Set<String> next = Set<String>.of(selected);
  if (!next.remove(id)) next.add(id);
  return next;
}

/// 角色显示名回落（纯函数）：name 非空用 name，否则回落 id（对位
/// `collabRoleDisplayName`）。两者都空（理论不发生）时回落空串，调用方不占位。
String collabRoleDisplayName(String name, String id) =>
    name.trim().isNotEmpty ? name : id;

/// 协同页人话文案层（对位 Kotlin 两份 CollabCopy 的合并；见文件头说明）。
///
/// 纪律（与 Kotlin 同）：
///  1. 七词状态词表是 daemon 状态字符串 → 人话的**唯一转换点**；
///  2. 每个禁用按钮旁的禁用原因（按钮灰 = 旁边一定有原因）；
///  3. 未知 kind / 未知状态的降级文案（原文照登 + 人话说明，不猜）；
///  4. 禁「未知/错误」裸字样（AUDIT §3.3）——单测按禁词表扫描本类全部常量。
abstract final class CollabCopy {
  // ---- 七词状态词表（顺序即生命周期）----

  static const String wordSubmitted = '已提交';
  static const String wordAccepted = '已受理';
  static const String wordRunning = '执行中';
  static const String wordWaitingApproval = '等待确认';
  static const String wordCompleted = '已完成';
  static const String wordFailed = '未完成';
  static const String wordCancelled = '已取消';

  /// daemon 状态原文也为空时的兜底（不编词，如实说没给）。
  static const String statusAbsent = '(电脑端没有给出状态)';

  // ---- 未知状态降级（不猜，原文 + 人话说明）----

  /// 未知状态说明（跟在原文后面；对齐 P2 未知事件的降级风格）。
  static const String unknownStatusNote = '电脑端返回了新版本的状态,当前版本还没有对应的展示方式;以上是它的原文';

  // ---- 断线与恢复（本波无事件流，保留呈现口径）----

  /// 断线呈现：快照保留，如实说「显示的是最后状态」。
  static const String disconnectNotice = '连接中断,显示的是最后状态;恢复连接后会自动跟进';

  /// 进入页面/重开页面时首次恢复的等待文案。
  static const String loadingRestore = '正在从电脑端恢复协同状态…';

  // ---- 派发前（对位 ui/CollabLogic.kt CollabCopy）----

  /// 无能力位：daemon 没报任何 `collab:<id>`。
  static const String emptyNoCapability = '当前连接的电脑尚未接入协同能力,无法发起多 agent 协同';

  /// 有能力位但没勾选任何人。
  static const String emptyNoSelection = '先勾选这次要一起干活的 agent';

  /// 读取能力位中的等待文案。
  static const String loadingAgents = '正在读取电脑端可协同的 agent…';

  /// 角色弹层的「未配置角色」项。
  static const String roleNone = '未配置角色';

  /// 派发按钮的禁用原因（[collabDispatchDecision] 同源，按钮灰 = 旁边有原因）。
  static const String reasonNoParticipant = '先勾选至少一位参与者,才能派发';
  static const String reasonNoInstruction = '写下要协同完成的任务,即可派发';
  static const String reasonDispatchInFlight = '正在派发,请稍候';

  static const String btnDispatch = '派发';
  static const String btnDispatching = '派发中…';

  // ---- 会话级 ----

  /// 会话横幅的指令回显兜底（daemon 没给指令原文时）。
  static const String sessionTitleFallback = '协同任务';

  /// 有会话但还没有成员受理回执。
  static const String membersPendingNote = '还没有成员受理回执;受理后这里会逐个出现成员卡';

  /// 派发已受理回执未达时（七词之「已提交」）的一句话补充。
  static const String progressSubmittedNote = '指令已提交给电脑端,等待受理回执';

  // ---- 结果分区（Task 7：成员原始结果 vs 汇总 Agent 结果）----

  static const String sectionMemberResults = '成员原始结果';
  static const String sectionSummaryResult = '汇总 Agent 结果';

  /// 结果区空态：还没有成员收尾（诚实等待，不伪造结果）。
  static const String memberResultsEmpty = '还没有成员收尾;每位成员完成后,它的原始结果会列在这里';

  /// 诚实空态：P1 协同没有汇总 Agent，不伪造一份「汇总」。
  static const String summaryResultNone =
      '本次协同没有安排汇总 Agent;各成员的结果在上方分别列出,由你自行比较';

  /// diff 纪律：不自动合并，先比较/查看，再由用户选择。
  static const String diffMergeNotice = '各成员对文件的改动不会自动合并;先逐个查看、比较,再决定采用哪一份';

  /// 成员结果行的「查看」入口（打开该成员会话，真实导航）。
  static const String viewResult = '查看结果';

  /// 比较 / 选择入口的文案（计划 Task 7 要求备齐）。P1 没有比较页与采用动作，
  /// **不摆假入口**——只放文案常量，等比较视图落地后接线。
  static const String compareResults = '比较结果';
  static const String chooseResult = '选择采用';

  // ---- 成员卡与会话级动作 ----

  static const String actionRetry = '重试该成员';
  static const String actionCancelMember = '取消该成员';
  static const String actionCancelSession = '取消整个协同';
  static const String actionViewReason = '查看原因';
  static const String actionHideReason = '收起原因';

  /// 挂起审批行（成员卡上）：前缀 + daemon 的动作短句。
  static const String approvalWaitingPrefix = '等待你确认:';

  /// 查看会话入口（有子线程且有导航通道才摆；没有通道就没有假入口）。
  static const String actionOpenSession = '查看会话';

  /// 写操作确认流（本波新增，对位 Android 的 pendingXxx 确认弹层纪律）：
  /// 取消/重试都是甲级写操作，执行前必须显式确认，标题/正文/确认键各一句。
  static const String confirmCancelSessionTitle = '取消整个协同?';
  static const String confirmCancelSessionBody =
      '电脑端会逐成员发起取消,已收尾的成员不受影响;取消后本次协同不再继续。';
  static const String confirmCancelMemberTitle = '取消该成员?';
  static const String confirmCancelMemberBody = '该成员会停止执行,它已产生的改动不会被自动撤销。';
  static const String confirmRetryMemberTitle = '重试该成员?';
  static const String confirmRetryMemberBody = '电脑端会以新一轮次重跑这位成员,旧结果保留备查,不覆盖。';
  static const String confirmOk = '确认';
  static const String confirmCancel = '再想想';

  // ---- 禁用原因（每个禁用按钮旁显示；与 daemon 侧前置条件同口径）----

  /// 重试：daemon 只允许对终局成员重试（RetryCollabMember 前置条件）。
  static const String reasonRetryActive = '该成员还没收尾,不能重试;要重来请先取消它';

  /// 重试/取消/查看会话共同前置：成员得有自己的子线程（受理失败的卡没有）。
  static const String reasonNoThread = '该成员没有自己的会话线程';

  /// 取消：daemon 对已终局成员幂等返回原卡，客户端如实说「无需取消」。
  static const String reasonCancelFinished = '该成员已收尾,无需取消';

  /// 取消整个协同：所有成员都已终局。
  static const String reasonSessionAllFinished = '所有成员都已收尾,没有可取消的成员';

  /// 查看会话：还没有 thread_id。
  static const String reasonOpenNoThread = '该成员还没有自己的会话,受理后才能查看';
}

// ── 留 B5（本波不摆假入口，注释交接）───────────────────────────────────────
//
// 1. **WatchCollabSession 实时流**：proto `ThreadService.WatchCollabSession` 是
//    服务器流（回放 + 跟随，缓冲 64 DROP_OLDEST）。B2 契约事件面只到
//    conn_state / host_info / host_event（channel-contract.md §4），stream 消费
//    属 B5。本波因此是**快照模式**：整屏由 List/Get 一次性恢复，页面显式说明
//    「快照模式，实时更新留后续波次」——不假装有实时跟进。
// 2. **SetCollabPolicy 策略写入**：改协同策略是甲级写操作（且 proto 要求整包
//    + revision CAS，只提交一个字段会把其余项重置回默认——实测踩过），本波
//    只读呈现 [CollabPolicyUi]，写入留 B5。
// 3. **会话内消息明细**（成员子线程的消息流）：那是 Chat 页的消费面
//    （chat/{threadId}），协同页只给「查看会话」出口，不在这里复刻一份。
// 4. **结果分区的比较/采用入口**：文案常量已备（[CollabCopy.compareResults] /
//    [CollabCopy.chooseResult]），P1 没有比较页与采用动作，不摆假入口。
