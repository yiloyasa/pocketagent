// Chat 周边动作纯函数层（B3 第 5 波④）：审批封帧 / 导出状态机 / 优化文本对 /
// fork-rollback 校验 / 九项可见性矩阵。
//
// 【为什么单拆一个文件】这些是无 widget 依赖的纯逻辑，单测直测；混进 widget
// 文件会让「算错还是渲错」分不清。能力位判定复用 C3 的 [agentCapableOf]
//（`chat_composer_data.dart`，只读 import，不 дубликат 口径）。

import '../gen/pocketagent/v1/thread.pb.dart';
import 'chat_composer_data.dart' show agentCapableOf;

// ── 九项动作 ────────────────────────────────────────────────────────────
//
// 【九项是什么】更多面板的可点项全集（Android `ChatMoreSheet` + 顶栏动作 +
// slash 伪命令执行段的并集，C4 一次收敛）：
//   压缩 compact / 分叉 fork / 回滚 rollback / 计划 review(StartReview) /
//   目标 goal(SetThreadGoal) / 导出 export(ExportThreadHtml) /
//   优化 optimize(OptimizePrompt) / 取消轮次 cancel(CancelTurn) /
//   清排队 clearQueue(ClearQueuedInputs)。
// 【门控双轨】六项认能力位（无位不显示——Android `compactActionOn` 等同款）；
// 目标无位门控（SetThreadGoal 无 agent 隔离）；取消/清排队认运行态（running
// 才显示，无轮次可终时不摆入口）。

/// 更多面板动作 id（九项全集，顺序即面板展示顺序）。
enum ChatActionId {
  /// 取消轮次（运行态才显示，置顶——最常用的逃生口）。
  cancel,

  /// 清排队输入（运行态才显示；紧随取消）。
  clearQueue,

  /// 压缩会话历史（`compact:<agentId>` 位）。
  compact,

  /// 分叉会话（`fork:<agentId>` 位）。
  fork,

  /// 回滚 N 回合（`rollback:<agentId>` 位）。
  rollback,

  /// 发起计划审查（`review:<agentId>` 位）。
  review,

  /// 设置会话目标（无位门控，恒显示）。
  goal,

  /// 导出 HTML 存档（`export_html:<agentId>` 位）。
  export,

  /// 优化提示词（`text_generation:<agentId>` 位）。
  optimize,
}

/// 动作中文名（面板行标题）。
String chatActionLabel(ChatActionId id) {
  switch (id) {
    case ChatActionId.cancel:
      return '终止当前轮次';
    case ChatActionId.clearQueue:
      return '清空排队输入';
    case ChatActionId.compact:
      return '压缩会话';
    case ChatActionId.fork:
      return '分叉会话';
    case ChatActionId.rollback:
      return '回滚';
    case ChatActionId.review:
      return '发起计划审查';
    case ChatActionId.goal:
      return '设置会话目标';
    case ChatActionId.export:
      return '导出 HTML';
    case ChatActionId.optimize:
      return '优化提示词';
  }
}

/// 九项可见性矩阵（纯函数，面板只渲染返回的子集）。
///
/// [agentId] 会话归属 agent（空 = 未知，按「无位」处理）；[capabilities] 能力
/// 位快照（`hostCapabilities` 透传）；[isRunning] 有在飞轮次；[queuedCount]
/// 本地排队计数（仅文档用途：清排队按钮的显示只认 [isRunning]，与任务书
/// 「running 态才显示」逐字对齐，不按条数再分档）。
List<ChatActionId> visibleChatActions({
  required String? agentId,
  required List<String> capabilities,
  required bool isRunning,
  required int queuedCount,
}) {
  final List<ChatActionId> out = <ChatActionId>[];
  if (isRunning) {
    out.add(ChatActionId.cancel);
    out.add(ChatActionId.clearQueue);
  }
  if (agentCapableOf('compact', agentId, capabilities)) {
    out.add(ChatActionId.compact);
  }
  if (agentCapableOf('fork', agentId, capabilities)) {
    out.add(ChatActionId.fork);
  }
  if (agentCapableOf('rollback', agentId, capabilities)) {
    out.add(ChatActionId.rollback);
  }
  if (agentCapableOf('review', agentId, capabilities)) {
    out.add(ChatActionId.review);
  }
  // 目标无位门控：SetThreadGoal 无 agent 隔离（daemon 全路由可达），恒显示。
  out.add(ChatActionId.goal);
  if (agentCapableOf('export_html', agentId, capabilities)) {
    out.add(ChatActionId.export);
  }
  if (agentCapableOf('text_generation', agentId, capabilities)) {
    out.add(ChatActionId.optimize);
  }
  return out;
}

// ── 审批 ────────────────────────────────────────────────────────────────

/// 审批决定封帧（timeline 占位卡点按 → 本帧 → `submitApproval` 过桥）。
///
/// [approved] 用户点的是允许还是拒绝；[note] 拒绝时的说明（agent 可见，允许
/// 时一般为空）；[option] 三按钮审批的选项 id（空 = 按 approved/note 决策，
/// daemon 视为一种操作，见 thread.proto `ApprovalDecision` C1 注释）。
ApprovalDecision approvalDecisionFrame({
  required String requestId,
  required bool approved,
  String note = '',
  String option = '',
}) =>
    ApprovalDecision(
      requestId: requestId,
      approved: approved,
      note: note,
      option: option,
    );

/// 审批提交请求封帧（threadId + decision 一次配齐，transport 只做编码）。
SubmitApprovalRequest approvalSubmitRequest({
  required String threadId,
  required ApprovalDecision decision,
}) =>
    SubmitApprovalRequest(threadId: threadId, decision: decision);

/// 审批结果 Toast 文案（帧驱动之外，提交应答的即时回执）。
String approvalResultNotice({required bool approved}) =>
    approved ? '已允许该操作' : '已拒绝该操作';

// ── 导出状态机 ──────────────────────────────────────────────────────────

/// HTML 导出的本地相位（transport 一次调用内的 UI 态，不跨调用保持）。
enum ChatExportPhase {
  /// 空闲（可点导出）。
  idle,

  /// 导出中（防重复提交）。
  exporting,

  /// 导出成功（path 已拿到，调用方按 path 取文件）。
  done,

  /// 导出失败（人话原因见 [exportFailureMessage]）。
  failed,
}

/// 导出相位中文标签（空串 = 不展示标签，调用方按需挂）。
String chatExportPhaseLabel(ChatExportPhase phase) {
  switch (phase) {
    case ChatExportPhase.idle:
      return '';
    case ChatExportPhase.exporting:
      return '正在导出…';
    case ChatExportPhase.done:
      return '导出完成';
    case ChatExportPhase.failed:
      return '导出失败';
  }
}

/// 导出失败的人话文案（宿主 message 透传第一优先，不改写、不吞）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名通道未就绪。`code`
/// 为空（旧宿主/未知错误）时退回通用文案，不编造 code。
String exportFailureMessage({required String code, required String message}) {
  if (code == 'HOST_UNAVAILABLE') {
    return '桌面端导出通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        'HTML 导出暂不可用。';
  }
  if (message.isNotEmpty) return message;
  if (code.isNotEmpty) return '导出 HTML 失败（$code）';
  return '导出 HTML 失败';
}

/// HTML 导出门控三态（Android `HtmlExportGate` 的 Dart 形态）。
enum HtmlExportGate {
  /// 有 `export_html:<agentId>` 位：直接发 RPC。
  supported,

  /// 明确无位：入口根本不显示（本枚举只服务「已进面板又掉位」的竞态，此时按
  /// 拒绝处理，不发 RPC —— 发了必败的 RPC 等于埋 bug）。
  unsupportedAgent,

  /// agentId 空（GetThread 还在路上）：等，不要替 daemon 猜。
  unknown,
}

/// 导出门控判定（Android `htmlExportGate` 原样：空 agent ≠ 不支持）。
HtmlExportGate htmlExportGateOf(
  String? agentId,
  List<String> capabilities,
) {
  final String id = agentId?.trim() ?? '';
  if (id.isEmpty) return HtmlExportGate.unknown;
  return agentCapableOf('export_html', id, capabilities)
      ? HtmlExportGate.supported
      : HtmlExportGate.unsupportedAgent;
}

// ── 提示词优化 ──────────────────────────────────────────────────────────

/// 优化前后文本对（只读语义：改写不影响任何会话状态）。
class OptimizePair {
  const OptimizePair({required this.original, required this.optimized});

  /// 用户原文（原样保留，`optimized` 为空时回填就用它）。
  final String original;

  /// 改写后的文本（空 = 无需改写，调用方如实展示，不回填空串）。
  final String optimized;

  /// 改写是否真有变化（两端掐头去尾比；优化后为空算无变化——空回填等于吞原文）。
  bool get hasChange =>
      optimized.trim().isNotEmpty &&
      optimized.trim() != original.trim();

  @override
  bool operator ==(Object other) =>
      other is OptimizePair &&
      other.original == original &&
      other.optimized == optimized;

  @override
  int get hashCode => Object.hash(original, optimized);
}

/// 优化输入校验（空/超长，口径与 composer 输入校验同源，不另造档位）。
String? validateOptimizeInput(String text) {
  if (text.trim().isEmpty) return '输入不能为空';
  if (text.length > 20000) return '输入过长，请分段优化';
  return null;
}

// ── 目标 / 审查 ─────────────────────────────────────────────────────────

/// 会话目标校验（空目标不过桥：许诺做不到的语义不如当场拦）。
String? validateThreadGoal(String goal) {
  if (goal.trim().isEmpty) return '目标不能为空（清空目标请直接关闭弹窗）';
  return null;
}

/// 审查焦点归一（空 = daemon 按默认口径审，原样透传空串，不填占位语）。
String normalizeReviewFocus(String focus) => focus.trim();

// ── fork / rollback ─────────────────────────────────────────────────────

/// 分叉起点校验（空 = 从最新处分叉，合法；非空必须非空白——空白 id 过桥必败）。
String? validateForkLastTurnId(String lastTurnId) {
  if (lastTurnId.isEmpty) return null;
  if (lastTurnId.trim().isEmpty) return '轮次 id 不能为空';
  return null;
}

/// 回滚可选的回合数（Android `rollbackOptions` 原样：固定档 1/2/3（小于总数
/// 的才留）+ 「全部」（总数）；总数 <= 0 时空——没回合可回时不摆入口）。
List<int> rollbackTurnOptions(int turnTotal) {
  if (turnTotal <= 0) return const <int>[];
  final List<int> fixed =
      <int>[1, 2, 3].where((int n) => n < turnTotal).toList();
  return <int>[...fixed, turnTotal];
}

/// 回滚选项中文标签（Android `rollbackOptionLabel` 原样：「全部」说清是多少）。
String rollbackOptionLabel(int option, int turnTotal) =>
    option >= turnTotal ? '全部（$turnTotal 回合）' : '$option 回合';

/// 回滚参数校验（numTurns 必须落在 1..turnTotal；turnTotal 未知（<=0）时拦——
/// 不替用户猜能回多少）。
String? validateRollbackTurns(int numTurns, int turnTotal) {
  if (turnTotal <= 0) return '当前没有可回滚的回合';
  if (numTurns < 1) return '请选择要回滚的回合数';
  if (numTurns > turnTotal) return '最多回滚 $turnTotal 回合';
  return null;
}

/// 回滚后果说明（Android `rollbackConsequence` 原样：只改对话记录，不改已落
/// 盘文件——用户必须在点确认前看到这一句）。
String rollbackConsequence(int turns) =>
    '将抹掉最近 $turns 回合的对话记录，agent 已改过的文件不会还原；'
    '要恢复文件改动请走 Git。';

/// 分叉后果说明（Android `forkConsequence` 原样：开一副本，原会话不动）。
String forkConsequence() =>
    '将以当前会话状态复制出一个新会话，原会话保持不动；'
    '之后两个会话各自独立，改动互不影响。';

// ── 清排队 ──────────────────────────────────────────────────────────────

/// 清排队结果 Toast 文案（daemon 回的两笔计数如实说，Android
/// `queueClearedNotice` 原样；两笔都 0 = 本来就没排队，如实说不摆功）。
String queueClearedNotice(int steered, int followUp) {
  final int s = steered < 0 ? 0 : steered;
  final int f = followUp < 0 ? 0 : followUp;
  if (s + f <= 0) return '排队本来就是空的，没有清掉任何输入';
  if (s > 0 && f > 0) return '已清除 ${s + f} 条输入（插队 $s 条，排队 $f 条）';
  if (f > 0) return '已清除 $f 条排队输入';
  return '已清除 $s 条插队输入';
}
