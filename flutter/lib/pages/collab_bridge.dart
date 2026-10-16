// Collab 真通道数据源（B4 第 4 波）：proto wire → 页面模型的映射 + PaBridge
// 方法面封装 + 角色 store 接线。
//
// 【为什么单独一个文件】与 threads_bridge.dart / files_bridge.dart / git_bridge.dart
// 同一分工：①「行上的每个字段各自从 wire 的哪个字段来」是纯函数，可被单测逐字段
// 钉死；②「真通道怎么包」= [BridgeCollabGateway]。collab.dart 只留界面（三态 +
// 行渲染 + 确认流）。三层各自可测，回归时分得清是映射错、通道错还是渲染错。
//
// 【诚实红线（B4 前）】Kotlin 宿主桥 B4 波次才注册 `dev.pocketagent/bridge`
// （channel-contract.md §6）；在此之前本文件调的 7 个方法面全部抛
// HOST_UNAVAILABLE，如实翻成页面错误态（文案点名「桌面端协同通道未就绪
// （B4 波次）」）。**绝不返回空列表冒充「没有协同会话」**：空态与错误态是两条
// 路径，test/pages/collab_bridge_test.dart 两条都钉死（红线：不留假绿）。
//
// 【为什么没有 stream 消费】WatchCollabSession 是服务器流，B2 契约事件面只到
// conn_state/host_info/host_event，stream 消费留 B5。本波全部是一次性快照
// （List/Get）+ 写动作应答，方法面也因此全部是 unary——不假装有实时跟进。

import '../gen/pocketagent/v1/runtime.pb.dart'
    show CollabPolicy, CollabPolicyResponse;
import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart';
import 'collab_roles_state.dart' show CollabRolesStore;
import 'collab_session_data.dart';
import 'pa_page_kit.dart';
import 'threads_bridge.dart' show threadRowTimeLabel;

// ── 行/卡映射（wire → 页面模型）─────────────────────────────────────────────

/// proto `CollabMemberSnapshot` → [CollabMemberUi]（字段对照只写这一遍）。
CollabMemberUi collabMemberUiOf(CollabMemberSnapshot m) => CollabMemberUi(
  agentId: m.agentId,
  roleId: m.roleId,
  threadId: m.threadId,
  // 状态是 string 口径：认识的落七词，不认识的落 unknown（原文保留）
  status: CollabStatus.fromWire(m.status),
  statusRaw: m.status,
  modelId: m.modelId,
  effort: m.effort,
  progress: m.progress,
  summary: m.summary,
  error: m.error,
  pendingApprovals: m.pendingApprovals
      .map((a) => CollabApprovalUi(requestId: a.requestId, label: a.reason))
      .toList(growable: false),
  workspace: m.workspace,
);

/// proto `CollabSessionSnapshot` → [CollabSessionUi]（整屏投影）。
///
/// 【会话级状态以 daemon 快照为准】不在这里二次派生——快照本身就是 daemon 的
/// 真值（`deriveCollabSessionStatus` 已在服务端跑过）；客户端派生只服务事件流
/// 路径（留 B5），两条路都给同一个函数入口时以快照字段优先。
CollabSessionUi collabSessionUiOf(CollabSessionSnapshot s) => CollabSessionUi(
  sessionId: s.sessionId.isNotEmpty ? s.sessionId : null,
  status: CollabStatus.fromWire(s.status),
  statusRaw: s.status,
  instruction: s.instruction,
  sessionError: s.error,
  workspace: s.workspace,
  members: s.members.map(collabMemberUiOf).toList(growable: false),
  loading: false,
  error: null,
);

/// 成员角色显示名（纯函数）：roleId 空 = 未配置（不占位）；否则查
/// [roleNames]（roleId → 显示名，来自角色 store），查不到回落 roleId 原文
/// ——不猜一个角色名，也不显「未知」。
String? collabMemberRoleName(CollabMemberUi m, Map<String, String> roleNames) {
  if (m.roleId.trim().isEmpty) return null;
  return roleNames[m.roleId] ?? m.roleId;
}

/// proto `CollabSessionSnapshot` → [CollabSessionRow]（列表行五字段 + 角色摘要）。
///
/// [nowMs] 显式注入：时间标签是相对当前时钟的格式化结果（复用
/// [threadRowTimeLabel]，口径 = Android `threadRowTime`，双端同一会话长同一样），
/// 不注入则「N 分钟前」不可单测。
CollabSessionRow collabSessionRowOf(
  CollabSessionSnapshot s, {
  required int nowMs,
  Map<String, String> roleNames = const <String, String>{},
}) {
  final List<String> names = <String>[];
  for (final CollabMemberSnapshot m in s.members) {
    if (m.roleId.trim().isEmpty) continue;
    final String name = roleNames[m.roleId] ?? m.roleId;
    if (!names.contains(name)) names.add(name);
  }
  return CollabSessionRow(
    id: s.sessionId,
    instruction: s.instruction,
    status: CollabStatus.fromWire(s.status),
    statusRaw: s.status,
    memberCount: s.members.length,
    roleNames: List<String>.unmodifiable(names),
    timeLabel: threadRowTimeLabel(s.updatedAtUnixMs.toInt(), nowMs),
  );
}

/// `ListCollabSessionsResponse` → 行列表（顺序保持 daemon 原序——proto 注明
/// 「按创建时间倒序」，UI 不发明第二套排序）。
List<CollabSessionRow> collabSessionRowsOfResponse(
  ListCollabSessionsResponse response, {
  required int nowMs,
  Map<String, String> roleNames = const <String, String>{},
}) => response.sessions
    .map((s) => collabSessionRowOf(s, nowMs: nowMs, roleNames: roleNames))
    .toList(growable: false);

/// proto `DispatchCollabResponse` → [CollabDispatchOutcome]。
CollabDispatchOutcome collabDispatchOutcomeOf(DispatchCollabResponse r) =>
    CollabDispatchOutcome(
      sessionId: r.sessionId,
      items: r.items
          .map(
            (i) => CollabDispatchItemUi(
              agentId: i.agentId,
              threadId: i.threadId,
              ack: i.ack,
              error: i.error,
            ),
          )
          .toList(growable: false),
    );

/// proto `CollabPolicyResponse` → [CollabPolicyUi]（只读投影）。
///
/// 【为什么判 has】proto3 `optional` 字段的「未设置」与「设置为 false」是两件
/// 事：用户配置缺席 = 沿用默认，此时 [CollabPolicyUi.collaborationEnabled] 等
/// 四个 policy 侧字段给 null（页面显示「电脑端没有返回」而不是替它选一档）；
/// effective 侧 daemon 保证全字段齐备，仍按 has 判一遍兜底（旧 daemon 缺字段时
/// 同样落 null，不猜）。
CollabPolicyUi collabPolicyUiOf(CollabPolicyResponse r) {
  final CollabPolicy policy = r.policy;
  final CollabPolicy eff = r.effective;
  return CollabPolicyUi(
    collaborationEnabled: policy.hasCollaborationEnabled()
        ? policy.collaborationEnabled
        : null,
    nativeDelegateEnabled: policy.hasNativeDelegateEnabled()
        ? policy.nativeDelegateEnabled
        : null,
    fanoutEnabled: policy.hasFanoutEnabled() ? policy.fanoutEnabled : null,
    maxParallelMembers: policy.hasMaxParallelMembers()
        ? policy.maxParallelMembers
        : null,
    effectiveCollaborationEnabled: eff.hasCollaborationEnabled()
        ? eff.collaborationEnabled
        : null,
    effectiveNativeDelegateEnabled: eff.hasNativeDelegateEnabled()
        ? eff.nativeDelegateEnabled
        : null,
    effectiveFanoutEnabled: eff.hasFanoutEnabled() ? eff.fanoutEnabled : null,
    effectiveMaxParallelMembers: eff.hasMaxParallelMembers()
        ? eff.maxParallelMembers
        : null,
    revision: r.revision.toInt(),
    policyPath: r.policyPath,
  );
}

// ── 错误文案 ────────────────────────────────────────────────────────────────

/// 错误码 → 页面人话文案（页面错误态直接展示这一条）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名「协同通道未就绪（B4 波次）」
/// 让用户知道不是自己操作错了，也不是「没有协同会话」。其余码原样透传宿主
/// message（不改写、不吞——与 settings/threads/files/git 页同一套透传纪律）；
/// 宿主没给 message 时退 code，宁可难看也不吞。
///
/// 【收敛来历】本函数与 threads/files/git/terminal/chat_history 五页的同型函数
/// 控制流逐字节相同（差异只在通道名与收尾句），收责任一为
/// [paChannelErrorText]（等价论证见该函数注释）。[tail] 默认值保持原值
/// '协同数据暂不可用。'，三个派生文案的 tail 也未动。
String collabLoadErrorText(PaBridgeException e, {String tail = '协同数据暂不可用。'}) =>
    paChannelErrorText(
      e,
      channel: '协同',
      tail: 'Kotlin 宿主尚未注册 bridge 通道，$tail',
    );

/// 派发失败文案（dispatch 路径）：与 [collabLoadErrorText] 同款透传纪律，
/// 收尾按派发语境改词。
String collabDispatchErrorText(PaBridgeException e) =>
    collabLoadErrorText(e, tail: '本次派发没有发出去，请稍后重试。');

/// 写操作（取消/重试）失败文案：透传 daemon 原文——daemon 的前置条件错误
/// （如「还在运行中,不能重试」）就是用户该看到的那句话，不泛化成「操作失败」。
String collabActionErrorText(PaBridgeException e) =>
    collabLoadErrorText(e, tail: '这个操作没有生效，请稍后重试。');

// ── 失败类型（页面据此进各自的错误态）──────────────────────────────────────

/// 会话列表加载失败：只带人话原因，不吞原始错误类型。
class CollabSessionsLoadFailure implements Exception {
  const CollabSessionsLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'CollabSessionsLoadFailure: $message';
}

/// 会话详情（GetCollabSession）失败：与列表失败分开，页面能分别指路。
class CollabSessionDetailFailure implements Exception {
  const CollabSessionDetailFailure(this.message);

  final String message;

  @override
  String toString() => 'CollabSessionDetailFailure: $message';
}

/// 派发失败：含传输/宿主错误与 daemon 业务拒绝两种来源，文案都由
/// [collabDispatchErrorText] / gateway 统一成人话。
class CollabDispatchFailure implements Exception {
  const CollabDispatchFailure(this.message);

  final String message;

  @override
  String toString() => 'CollabDispatchFailure: $message';
}

/// 写操作（取消整个协同 / 取消成员 / 重试成员）失败。
class CollabActionFailure implements Exception {
  const CollabActionFailure(this.message);

  final String message;

  @override
  String toString() => 'CollabActionFailure: $message';
}

/// 策略读取失败（只读呈现区）。
class CollabPolicyLoadFailure implements Exception {
  const CollabPolicyLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'CollabPolicyLoadFailure: $message';
}

// ── 数据源抽象 ──────────────────────────────────────────────────────────────

/// 协同网关 —— 页面只认这一个口子（对位 Kotlin `AgentConnection` 的 collab
/// 薄封装 + `CollabSessionRepository` 的意图入口；UI 不直接碰 gRPC/channel）。
///
/// 【为什么合成一个接口而不是五个】协同页的每次交互都要「先有会话上下文」：
/// 列表 → 详情 → 写动作 → 再拉详情，五条各自独立的源会让页面持有五个注入点、
/// 五次错误分支；合成一个网关后页面只有一处失败处理，测试也只要一个 fake。
abstract class CollabGateway {
  /// 会话列表快照（[includeFinished] = 连已结束的一起返回）。
  ///
  /// [nowMs] 显式注入（页面传当前时钟）：时间标签是相对时钟的格式化结果，
  /// 不注入则「N 分钟前」不可单测。缺省时网关用自己的 [BridgeCollabGateway]
  /// 时钟（生产 = 系统时钟，测试可注入固定时钟）。
  Future<List<CollabSessionRow>> listSessions({
    required bool includeFinished,
    int? nowMs,
    Map<String, String> roleNames,
  });

  /// 单个会话快照（成员整卡真值）。
  Future<CollabSessionUi> getSession(String sessionId);

  /// 派发一条共同指令（扇出给 [members]）。
  Future<CollabDispatchOutcome> dispatch(
    String instruction,
    List<CollabMemberSpec> members,
  );

  /// 取消整个协同（应答是收敛后的会话快照）。
  Future<CollabSessionUi> cancelSession(String sessionId);

  /// 取消一个成员（应答是该成员的新卡）。
  Future<CollabMemberUi> cancelMember(String sessionId, String threadId);

  /// 重试一个成员（应答是该成员的新卡；新轮次不覆盖旧结果）。
  Future<CollabMemberUi> retryMember(String sessionId, String threadId);

  /// 协同策略快照（只读）。
  Future<CollabPolicyUi> getPolicy();
}

/// 真通道网关：`PaBridge` 的 7 个 unary 方法面 → 页面模型。
///
/// [bridge] / [nowMs] 都是注入点：测试按通道名 mock 宿主 wire 行为（PaBridge
/// 全程真实执行），并用固定时钟钉死时间标签。页面在 `listSessions` 显式传
/// nowMs（时钟归页面持有）；[nowMs] 是直接使用网关的调用方（含测试）的兜底。
class BridgeCollabGateway implements CollabGateway {
  BridgeCollabGateway({PaBridge? bridge, int Function()? nowMs})
    : _bridge = bridge ?? PaBridge(),
      _nowMs = nowMs ?? _systemNowMs;

  static int _systemNowMs() => DateTime.now().millisecondsSinceEpoch;

  final PaBridge _bridge;
  final int Function() _nowMs;

  @override
  Future<List<CollabSessionRow>> listSessions({
    required bool includeFinished,
    int? nowMs,
    Map<String, String> roleNames = const <String, String>{},
  }) async {
    final ListCollabSessionsResponse response;
    try {
      response = await _bridge.listCollabSessions(
        ListCollabSessionsRequest(includeFinished: includeFinished),
      );
    } on PaBridgeException catch (e) {
      // **不在这里回退成空列表**：那会把「通道没就绪」粉饰成「没有协同会话」
      // （假空态），空态与错误态两条路径从此再也分不开（红线：不留假绿）。
      throw CollabSessionsLoadFailure(collabLoadErrorText(e));
    }
    return collabSessionRowsOfResponse(
      response,
      nowMs: nowMs ?? _nowMs(),
      roleNames: roleNames,
    );
  }

  @override
  Future<CollabSessionUi> getSession(String sessionId) async {
    final CollabSessionSnapshot snapshot;
    try {
      snapshot = await _bridge.getCollabSession(
        GetCollabSessionRequest(sessionId: sessionId),
      );
    } on PaBridgeException catch (e) {
      throw CollabSessionDetailFailure(collabLoadErrorText(e));
    }
    return collabSessionUiOf(snapshot);
  }

  @override
  Future<CollabDispatchOutcome> dispatch(
    String instruction,
    List<CollabMemberSpec> members,
  ) async {
    final DispatchCollabResponse response;
    try {
      response = await _bridge.dispatchCollab(
        DispatchCollabRequest(
          instruction: instruction,
          members: members
              .map(
                (m) => CollabMember(agentId: m.agentId, roleId: m.roleId ?? ''),
              )
              .toList(growable: false),
        ),
      );
    } on PaBridgeException catch (e) {
      // 传输/宿主/daemon 拒绝的原文如实上抛（message 优先，空则退 code）。
      throw CollabDispatchFailure(collabDispatchErrorText(e));
    }
    return collabDispatchOutcomeOf(response);
  }

  @override
  Future<CollabSessionUi> cancelSession(String sessionId) async {
    final CollabSessionSnapshot snapshot;
    try {
      snapshot = await _bridge.cancelCollabSession(
        CancelCollabSessionRequest(sessionId: sessionId),
      );
    } on PaBridgeException catch (e) {
      throw CollabActionFailure(collabActionErrorText(e));
    }
    return collabSessionUiOf(snapshot);
  }

  @override
  Future<CollabMemberUi> cancelMember(String sessionId, String threadId) async {
    final CollabMemberSnapshot card;
    try {
      card = await _bridge.cancelCollabMember(
        CancelCollabMemberRequest(sessionId: sessionId, threadId: threadId),
      );
    } on PaBridgeException catch (e) {
      throw CollabActionFailure(collabActionErrorText(e));
    }
    return collabMemberUiOf(card);
  }

  @override
  Future<CollabMemberUi> retryMember(String sessionId, String threadId) async {
    final CollabMemberSnapshot card;
    try {
      card = await _bridge.retryCollabMember(
        RetryCollabMemberRequest(sessionId: sessionId, threadId: threadId),
      );
    } on PaBridgeException catch (e) {
      throw CollabActionFailure(collabActionErrorText(e));
    }
    return collabMemberUiOf(card);
  }

  @override
  Future<CollabPolicyUi> getPolicy() async {
    final CollabPolicyResponse response;
    try {
      response = await _bridge.getCollabPolicy();
    } on PaBridgeException catch (e) {
      throw CollabPolicyLoadFailure(collabLoadErrorText(e, tail: '协同策略暂不可用。'));
    }
    return collabPolicyUiOf(response);
  }
}

// ── 角色数据（复用「协同角色」两页的 collab_roles_state store，只读）─────────

/// 协同角色数据源抽象 —— 页面只认这一个口子（测试注入可控源，生产走
/// [BridgeCollabRolesSource] → `collab_roles_state.dart` 的 store）。
///
/// 【为什么页面不直接 import store】store 是「协同角色」页（另一子代理）的
/// 领域对象，本页只消费它的**只读投影**；隔一层抽象后，本页与 store 的演进
/// 互不牵连（store 加保存/删除能力时本页零改动），测试也不需要 store 的加载
/// 时序。
///
/// 元素类型 = proto `CollabRole`（gen stub 的稳定类型，双端同源）——不引入
/// store 自己的角色模型类，页面只认 proto 形状的字段（id/name/agentId/
/// description/model/effort/workspace）。
abstract class CollabRolesSource {
  /// 拉一次角色列表（进页即调；失败以人话原因上抛，页面据此说明而非假装没角色）。
  Future<List<CollabRole>> load();
}

/// 真通道角色源：[CollabRolesStore]（B4 第 4 波成品，对位 Kotlin object
/// `CollabRolesState`）+ 其 `roles` 只读投影。
///
/// 【与 store 的接触面】只用三个符号：`load()`（触发加载）、`loadState`
/// （LOADING/READY/ERROR 三档）、`roles`（proto `CollabRole` 列表）。收敛判据
/// 按**枚举名**判定（`CollabRolesLoadState` 三档），不假设 store 的其他内部
/// 形状；等不到收敛时按当前 roles 返回（诚实：有几个显示几个，不阻塞页面
/// 其余部分——角色只是派发的可选项）。
///
/// 【B4 接线点（已接线）】store 是可注入类（不是单例，见 collab_roles_state.dart
/// 文件头的「对位 object」说明）：本类持有一个默认实例，调用方（根装配）要
/// 跨页共享时显式传同一个 store 进来——方法面不动。
class BridgeCollabRolesSource implements CollabRolesSource {
  const BridgeCollabRolesSource();

  /// 角色 store（B4 第 4 波成品：`collab_roles_state.dart`）。
  ///
  /// 【为什么类级共享一个默认实例】不传 store 的调用方要的是「Kotlin object
  /// 级共享」的语义：Collab 主面板的角色弹层与角色两页看到同一份 roles。
  /// 要测试隔离的调用方显式传自己的 store（页面构造注入）。
  static final CollabRolesStore store = CollabRolesStore();

  @override
  Future<List<CollabRole>> load() async {
    store.load();
    await store.loadCompleted;
    return List<CollabRole>.of(store.roles);
  }
}
