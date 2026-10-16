// 协同主面板（B4 第 4 波，page-contracts §1.10 Collab 的第一梯队）。
//
// 【页面职责】一条指令扇出给多个对等 agent 并行协作的编排台。对位 Android
// `ui/CollabScreen.kt`（1118 行）的**主面板第一梯队**：
//   1. 参与协同区（agent 多选芯片 + 选中数 + 无能力位诚实空态）；
//   2. 会话列表（ListCollabSessions 快照：id / 指令 / 状态 / 成员数 / 时间）；
//   3. 会话详情（GetCollabSession 快照：会话横幅 + 成员卡 + 三个写动作的确认流）；
//   4. 角色弹层（读 collab_roles_state store；进页 load()）+ 角色选择 → 派发；
//   5. 策略区**只读**呈现（GetCollabPolicy 快照；SetCollabPolicy 留 B5）。
//
// 【与 Android 的差异（如实登记，不假装等价）】
//   - **快照模式，没有实时流**：Kotlin 进页是「GetCollabSession 快照 + WatchCollabSession
//     事件流跟随」（CollabSessionRepository.openLatest）。B2 契约事件面只到
//     conn_state/host_info/host_event，stream 消费留 B5——本波用一次性快照，
//     页面显式写明「快照模式，实时更新留后续波次」，不摆假实时。
//   - **参与者名单只到能力位**：Kotlin 的参与者全集还合并了 agent 静态目录
//     （buildAgents）与本地停用偏好（disabledAgents），Flutter 侧没有这两个数据源
//     （proto 无 listAgents RPC），参与者 = `collab:<id>` 能力位投影。没有的态
//     （电脑端未安装 / 已停用）就不显示，不编。
//   - **成员卡没有耗时**：proto `CollabMemberSnapshot` 没有 duration 字段
//     （11 个字段逐一核对），改显真实存在的 model/effort。没有 wire 支撑的数字
//     一律不显示。
//   - **多会话列表是本波增强**：Kotlin 的 listCollabSessions 只用于 openLatest
//     取最新进行中会话 id，页面本身是单会话树。本波按任务书落列表 + 选中详情
//     两段式（列表可切会话，详情跟随选中项）。
//
// 【诚实红线（B4 前）】宿主桥未注册 → 7 个方法面全部抛 HOST_UNAVAILABLE，
// 列表/详情/策略分别进各自的错误态，文案点名「桌面端协同通道未就绪（B4 波次）」，
// **不伪装成空列表**（两条路径分别有测试钉死，互斥断言）。
//
// 【写操作确认流】取消整个协同 / 取消成员 / 重试成员都是甲级写操作：执行前必须
// 显式确认（标题 + 后果说明 + 确认键），确认后才发 RPC；失败把 daemon 原文
// 如实呈现在动作行下方。防重复提交双保险：确认弹层关闭即禁用 + 在飞闸。

import 'dart:async';

import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../components/pa_status_chip.dart';
import '../gen/pocketagent/v1/host.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'collab_bridge.dart';
import 'collab_session_data.dart';
import 'pa_page_kit.dart';

/// 协同主面板。
///
/// 三个注入点（测试可控，生产走真通道）：
///  - [gateway]：会话/写动作/策略的统一网关（默认 [BridgeCollabGateway]）；
///  - [rolesSource]：角色数据源（默认 [BridgeCollabRolesSource] →
///    `collab_roles_state.dart` 的 store，进页 load()）；
///  - [capabilities]：能力位快照（默认由 [PaBridge.getHostInfo] 读；host_info
///    事件增量在事件消费统一波次接入，届时快照仍作初值）。
class CollabPage extends StatefulWidget {
  const CollabPage({
    super.key,
    this.gateway,
    this.rolesSource,
    this.capabilities,
  });

  final CollabGateway? gateway;
  final CollabRolesSource? rolesSource;

  /// 能力位列表（`collab:<id>` 决定参与者全集）；null = 还没读到
  /// getHostInfo（页面先显示「正在读取…」，读不到按无能力位诚实空态处理）。
  final List<String>? capabilities;

  @override
  State<CollabPage> createState() => _CollabPageState();
}

/// 会话列表三态：加载中 / 加载失败 / 已就绪（就绪后再分「空列表」与「有会话」）。
///
/// 与 threads/collab_roles/settings_* 各页的同名同义同序枚举收敛为
/// pa_page_kit.dart 的共享词汇 [PaAsyncPhase]（typedef 零运行时开销、零渲染，
/// 改名不碰像素；本页 switch/if 比较照旧）。
typedef _SessionsPhase = PaAsyncPhase;

/// 会话详情三态（与列表分开：列表读到了不代表详情读到了）。
///
/// 【为什么不并入 [PaAsyncPhase]】本枚举多一个 [idle] 初态（未选中会话时整个
/// 详情区收起，见 `_buildDetailSection` 的首行判断）——四态与共享词汇的三态
/// 不同型，硬并要么给 PaAsyncPhase 加一个没人用的 idle，要么把 idle 塞进
/// loading（那会让「没选中」和「正在拉」在 UI 上分不开）。保留独立枚举。
enum _DetailPhase { idle, loading, error, ready }

/// 策略区三态（与上面两个都分开：策略读失败不影响会话呈现）。
typedef _PolicyPhase = PaAsyncPhase;

/// 角色区三态（进页 load()；失败不阻塞派发区——角色只是可选项）。
///
/// 【与 collab_roles.dart 的 _RolesPhase 是同一个 typedef】两边都指向
/// [PaAsyncPhase]，各自文件内私有、互不可见，但值域同源——将来若有一处要加
/// 第四态，两处会同时被 analyze 的穷尽匹配抓到，不会只改一处。
typedef _RolesPhase = PaAsyncPhase;

/// 待确认的写操作（确认流的唯一状态：弹层开着的时候只有一个待确认项）。
class _PendingWrite {
  const _PendingWrite._({
    required this.title,
    required this.body,
    required this.confirmLabel,
    required this.run,
  });

  final String title;
  final String body;
  final String confirmLabel;

  /// 确认后执行的动作（返回 Future；页面据此显示在飞状态与失败原因）。
  final Future<void> Function() run;

  static _PendingWrite cancelSession({
    required String sessionId,
    required Future<void> Function() run,
  }) => _PendingWrite._(
    title: CollabCopy.confirmCancelSessionTitle,
    body: CollabCopy.confirmCancelSessionBody,
    confirmLabel: CollabCopy.actionCancelSession,
    run: run,
  );

  static _PendingWrite cancelMember({
    required String agentId,
    required Future<void> Function() run,
  }) => _PendingWrite._(
    title: CollabCopy.confirmCancelMemberTitle,
    body: '${CollabCopy.confirmCancelMemberBody}（$agentId）',
    confirmLabel: CollabCopy.actionCancelMember,
    run: run,
  );

  static _PendingWrite retryMember({
    required String agentId,
    required Future<void> Function() run,
  }) => _PendingWrite._(
    title: CollabCopy.confirmRetryMemberTitle,
    body: '${CollabCopy.confirmRetryMemberBody}（$agentId）',
    confirmLabel: CollabCopy.actionRetry,
    run: run,
  );
}

class _CollabPageState extends State<CollabPage> {
  late CollabGateway _gateway;
  late CollabRolesSource _rolesSource;
  late PaBridge _bridge;
  List<String> _capabilities = const <String>[];

  // ── 会话列表 ──
  _SessionsPhase _sessionsPhase = _SessionsPhase.loading;
  String _sessionsError = '';
  List<CollabSessionRow> _sessions = const <CollabSessionRow>[];

  // ── 会话详情（跟随选中项）──
  String? _selectedId;
  _DetailPhase _detailPhase = _DetailPhase.idle;
  String _detailError = '';
  CollabSessionUi _detail = CollabSessionUi.initial;

  // ── 策略区（只读）──
  _PolicyPhase _policyPhase = _PolicyPhase.loading;
  String _policyError = '';
  CollabPolicyUi _policy = CollabPolicyUi.empty;

  // ── 角色区 ──
  _RolesPhase _rolesPhase = _RolesPhase.loading;
  String _rolesError = '';
  List<CollabRole> _roles = const <CollabRole>[];

  // ── 派发前配置（对位 Kotlin ui.CollabState：选人 / 绑角色 / 硬闸）──
  final Set<String> _selected = <String>{};
  final Map<String, String> _roleBinding = <String, String>{};
  bool _dispatching = false;
  String? _lastInstruction;

  final TextEditingController _cmdCtrl = TextEditingController();

  /// 正在飞的写操作标签（null = 空闲）；防重复提交第二道闸。
  ///
  /// 第一道是确认弹层（不确认不发 RPC）；本闸挡「确认后连点/确认弹层未关时
  /// 再点另一张卡」——同一时刻只允许一个写操作在飞。
  String? _writeInFlight;

  /// 写操作失败的人话原因（动作行下方；下次动作前清掉）。
  String? _writeError;

  /// 派发结果里的受理失败项（一位失败不拖垮其余；详情就位后清掉）。
  List<CollabDispatchItemUi> _dispatchRejects = const <CollabDispatchItemUi>[];

  @override
  void initState() {
    super.initState();
    _gateway = widget.gateway ?? BridgeCollabGateway();
    _rolesSource = widget.rolesSource ?? const BridgeCollabRolesSource();
    _bridge = PaBridge();
    _capabilities = widget.capabilities ?? const <String>[];
    _loadSessions();
    _loadRoles();
    _loadPolicy();
    // 注入了 capabilities 就不再自己读 getHostInfo（测试/调用方已给快照）。
    if (widget.capabilities == null) {
      _loadCapabilities();
    }
  }

  @override
  void dispose() {
    _cmdCtrl.dispose();
    super.dispose();
  }

  // ── 数据装载 ────────────────────────────────────────────────────────────

  /// 能力位快照（getHostInfo 的 capabilities）：参与者全集的唯一来源。
  ///
  /// 【读不到不刷红】能力位只是参与者名单的来源，读不到时参与区显示无能力位
  /// 空态（用户仍可在电脑端启用协作后重试）；会话列表/详情/策略各自独立成败。
  Future<void> _loadCapabilities() async {
    try {
      final HostInfo info = await _bridge.getHostInfo();
      if (!mounted) return;
      setState(() => _capabilities = info.capabilities);
    } on PaBridgeException catch (e) {
      // 读不到 = 没有参与者名单（诚实空态由参与区呈现）；原因进调试输出，
      // 不在参与区重复一条错误（那是列表/详情/策略的错误态职责）。
      debugPrint('collab: 能力位读取失败（${e.code}），参与区按无能力位呈现');
    }
  }

  Future<void> _loadSessions() async {
    if (_sessionsPhase != _SessionsPhase.loading || _sessionsError.isNotEmpty) {
      setState(() {
        _sessionsPhase = _SessionsPhase.loading;
        _sessionsError = '';
      });
    }
    try {
      final List<CollabSessionRow> rows = await _gateway.listSessions(
        includeFinished: true,
        nowMs: DateTime.now().millisecondsSinceEpoch,
        roleNames: _roleNamesById(),
      );
      if (!mounted) return;
      setState(() {
        _sessions = rows;
        _sessionsPhase = _SessionsPhase.ready;
        // 默认选中第一项（proto 注明按创建时间倒序 = 最新那个），与 Kotlin
        // openLatest「要最新的进行中会话」同策略；没有会话就不选。
        if (_selectedId == null && rows.isNotEmpty) {
          _selectedId = rows.first.id;
        }
      });
      if (_selectedId != null) {
        await _loadDetail(_selectedId!);
      }
    } on CollabSessionsLoadFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _sessionsPhase = _SessionsPhase.error;
        _sessionsError = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _sessionsPhase = _SessionsPhase.error;
        _sessionsError = '$e';
      });
    }
  }

  Future<void> _loadDetail(String sessionId) async {
    setState(() {
      _selectedId = sessionId;
      _detailPhase = _DetailPhase.loading;
      _detailError = '';
      _detail = CollabSessionUi.initial;
    });
    try {
      final CollabSessionUi detail = await _gateway.getSession(sessionId);
      if (!mounted) return;
      setState(() {
        _detail = detail;
        _detailPhase = _DetailPhase.ready;
      });
    } on CollabSessionDetailFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _detailPhase = _DetailPhase.error;
        _detailError = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _detailPhase = _DetailPhase.error;
        _detailError = '$e';
      });
    }
  }

  /// 进页即 load()（Android CollabScreen.kt:124 `LaunchedEffect` 同语义）。
  ///
  /// 【失败不阻塞派发】角色只是可选项（可以不配角色派发）：读不到时角色弹层
  /// 如实说明原因，「未配置角色」项照常可用。
  Future<void> _loadRoles() async {
    setState(() => _rolesPhase = _RolesPhase.loading);
    try {
      final List<CollabRole> roles = await _rolesSource.load();
      if (!mounted) return;
      setState(() {
        _roles = roles;
        _rolesPhase = _RolesPhase.ready;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _rolesPhase = _RolesPhase.error;
        _rolesError = '$e';
      });
    }
  }

  Future<void> _loadPolicy() async {
    setState(() {
      _policyPhase = _PolicyPhase.loading;
      _policyError = '';
    });
    try {
      final CollabPolicyUi policy = await _gateway.getPolicy();
      if (!mounted) return;
      setState(() {
        _policy = policy;
        _policyPhase = _PolicyPhase.ready;
      });
    } on CollabPolicyLoadFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _policyPhase = _PolicyPhase.error;
        _policyError = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _policyPhase = _PolicyPhase.error;
        _policyError = '$e';
      });
    }
  }

  // ── 派生 ────────────────────────────────────────────────────────────────

  /// 可参与协同的 agent id 全集（`collab:<id>` 能力位投影，纯函数派生）。
  List<String> get _collabAgentIds => resolveCollabAgentIds(_capabilities);

  Map<String, String> _roleNamesById() {
    final Map<String, String> out = <String, String>{};
    for (final CollabRole r in _roles) {
      out[r.id] = collabRoleDisplayName(r.name, r.id);
    }
    return out;
  }

  CollabDispatchDecision get _decision => collabDispatchDecision(
    selectedCount: _selected.length,
    instructionReady: _cmdCtrl.text.trim().isNotEmpty,
    dispatching: _dispatching,
  );

  // ── 交互 ────────────────────────────────────────────────────────────────

  void _toggleAgent(String id) {
    setState(() {
      final Set<String> next = toggleCollabSelection(_selected, id);
      _selected
        ..clear()
        ..addAll(next);
    });
  }

  void _bindRole(String agentId, String? roleId) {
    setState(() {
      if (roleId == null || roleId.trim().isEmpty) {
        _roleBinding.remove(agentId);
      } else {
        _roleBinding[agentId] = roleId;
      }
    });
  }

  /// 派发一条共同指令（对位 Kotlin `CollabState.dispatch` + 仓库层 dispatch）。
  ///
  /// 【防重复提交双保险】按钮禁用（[CollabDispatchDecision]）是第一道，
  /// [_dispatching] 硬闸是第二道（挡 IME 回车/连点）；硬闸期间重复调用直接忽略。
  Future<void> _dispatch() async {
    final String cmd = _cmdCtrl.text.trim();
    if (_dispatching) return;
    if (cmd.isEmpty || _selected.isEmpty) return;
    final List<CollabMemberSpec> members = _selected
        .map((id) => CollabMemberSpec(agentId: id, roleId: _roleBinding[id]))
        .toList(growable: false);
    setState(() {
      _dispatching = true;
      _lastInstruction = cmd;
      _dispatchRejects = const <CollabDispatchItemUi>[];
      _writeError = null;
    });
    try {
      final CollabDispatchOutcome outcome = await _gateway.dispatch(
        cmd,
        members,
      );
      if (!mounted) return;
      if (outcome.sessionId.isEmpty) {
        // daemon 受理了却没给会话 id：无法跟进进度，如实说（不编一个 id）。
        setState(() {
          _detailPhase = _DetailPhase.error;
          _detailError = '协同已派发,但电脑端没有返回会话 id,无法跟进进度';
        });
        return;
      }
      _cmdCtrl.clear();
      // 受理失败项如实呈现（一位失败不拖垮其余；成功项照常进详情）。
      setState(() {
        _dispatchRejects = outcome.items
            .where((i) => !i.ack)
            .toList(growable: false);
      });
      await _loadSessions();
      if (!mounted) return;
      await _loadDetail(outcome.sessionId);
    } on CollabDispatchFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _detailPhase = _DetailPhase.error;
        _detailError = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _detailPhase = _DetailPhase.error;
        _detailError = '$e';
      });
    } finally {
      if (mounted) setState(() => _dispatching = false);
    }
  }

  /// 确认后执行：发 RPC → 刷新详情 → 失败如实呈现。
  ///
  /// 【弹层为什么在这里之前就由调用方 pop】确认键先 `Navigator.pop` 再调本方法：
  /// 弹层的关闭不依赖 RPC 结果（否则慢网络下用户会被弹层按住），RPC 期间由动作
  /// 行上的「…」在飞态表达进度。
  Future<void> _runWrite(_PendingWrite pending) async {
    // 在飞闸：同一时刻只允许一个写操作（挡连点与多卡并发）。
    if (_writeInFlight != null) return;
    setState(() {
      _writeInFlight = pending.confirmLabel;
      _writeError = null;
    });
    try {
      await pending.run();
    } on CollabActionFailure catch (e) {
      if (!mounted) return;
      setState(() => _writeError = e.message);
    } catch (e) {
      if (!mounted) return;
      setState(() => _writeError = '$e');
    } finally {
      if (mounted) setState(() => _writeInFlight = null);
    }
  }

  // ── 构建 ────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final List<String> agentIds = _collabAgentIds;
    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 8, 14, 28),
      children: <Widget>[
        _buildIntroCard(pa),
        const SizedBox(height: 14),
        _buildParticipantsSection(pa, agentIds),
        const SizedBox(height: 14),
        _buildSessionsSection(pa),
        const SizedBox(height: 14),
        _buildDetailSection(pa),
        const SizedBox(height: 14),
        _buildPolicySection(pa),
        const SizedBox(height: 16),
        _buildDispatchBar(pa),
      ],
    );
  }

  // ── 说明卡（石墨重点面对位：协同身份一眼可辨）────────────────────────────

  Widget _buildIntroCard(PAColors pa) {
    return PACard(
      color: pa.bgSurface2,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Icon(Icons.account_tree_rounded, size: 20, color: pa.agentAccent),
              const SizedBox(width: 10),
              Text(
                '一条任务，多个视角。',
                style: PATypography.titleM.copyWith(color: pa.fgPrimary),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            '把同一指令并行交给选中的 Agent；每位参与者分别回报，不混为一条对话。',
            style: PATypography.caption.copyWith(color: pa.fgSecondary),
          ),
          const SizedBox(height: 8),
          // 快照模式的诚实说明（本波无实时流，不假装有——见文件头「留 B5」）。
          Text(
            '当前为快照模式：进入页面与每次操作后拉取一次电脑端状态，实时更新留后续波次。',
            key: const ValueKey<String>('collab-snapshot-mode'),
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ],
      ),
    );
  }

  // ── 参与协同 ────────────────────────────────────────────────────────────

  Widget _buildParticipantsSection(PAColors pa, List<String> agentIds) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Row(
          children: <Widget>[
            Text(
              '参与协同',
              style: PATypography.titleM.copyWith(color: pa.fgPrimary),
            ),
            const SizedBox(width: 8),
            Text(
              collabSelectionSummary(_selected.length),
              key: const ValueKey<String>('collab-selection-summary'),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          '电脑端接入协同的 agent 都在这里；点选后一起派发',
          style: PATypography.caption.copyWith(color: pa.fgTertiary),
        ),
        const SizedBox(height: 8),
        if (agentIds.isEmpty)
          _EmptyView(
            title: '暂无可协同的 agent',
            hint: _capabilities.isEmpty
                ? CollabCopy.emptyNoCapability
                : '电脑端启用协作能力后，这里会列出可选项',
            textKey: 'collab-participants-empty',
          )
        else
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: <Widget>[
              for (final String id in agentIds)
                _AgentChip(
                  agentId: id,
                  selected: _selected.contains(id),
                  roleName: _roleBinding[id] == null
                      ? null
                      : collabRoleDisplayName(
                          _roleNameOf(_roleBinding[id]!) ?? '',
                          _roleBinding[id]!,
                        ),
                  onTap: () => _toggleAgent(id),
                  onBindRole: () => _openRoleSheet(id),
                ),
            ],
          ),
      ],
    );
  }

  String? _roleNameOf(String roleId) {
    for (final CollabRole r in _roles) {
      if (r.id == roleId) return r.name;
    }
    return null;
  }

  // ── 会话列表 ────────────────────────────────────────────────────────────

  Widget _buildSessionsSection(PAColors pa) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text('协同会话', style: PATypography.titleM.copyWith(color: pa.fgPrimary)),
        const SizedBox(height: 8),
        switch (_sessionsPhase) {
          _SessionsPhase.loading => const _LoadingView(
            message: CollabCopy.loadingRestore,
            textKey: 'collab-sessions-loading',
          ),
          _SessionsPhase.error => _ErrorView(
            title: '会话列表加载失败',
            message: _sessionsError,
            onRetry: _loadSessions,
            textKey: 'collab-sessions-error',
          ),
          _SessionsPhase.ready =>
            _sessions.isEmpty
                ? _EmptyView(
                    title: '还没有协同会话',
                    hint: '在下面写下任务、选好参与者，点「派发」即可开始第一次协同',
                    textKey: 'collab-sessions-empty',
                  )
                : Column(
                    children: <Widget>[
                      for (int i = 0; i < _sessions.length; i++) ...<Widget>[
                        if (i > 0) const SizedBox(height: 8),
                        _SessionRow(
                          row: _sessions[i],
                          selected: _sessions[i].id == _selectedId,
                          onTap: () => _loadDetail(_sessions[i].id),
                        ),
                      ],
                    ],
                  ),
        },
      ],
    );
  }

  // ── 会话详情 ────────────────────────────────────────────────────────────

  Widget _buildDetailSection(PAColors pa) {
    if (_detailPhase == _DetailPhase.idle) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text('会话详情', style: PATypography.titleM.copyWith(color: pa.fgPrimary)),
        const SizedBox(height: 8),
        switch (_detailPhase) {
          _DetailPhase.idle => const SizedBox.shrink(),
          _DetailPhase.loading => const _LoadingView(
            message: CollabCopy.loadingRestore,
            textKey: 'collab-detail-loading',
          ),
          _DetailPhase.error => _ErrorView(
            title: '会话详情加载失败',
            message: _detailError,
            onRetry: _selectedId == null
                ? _loadSessions
                : () => _loadDetail(_selectedId!),
            textKey: 'collab-detail-error',
          ),
          _DetailPhase.ready => _buildDetailBody(pa),
        },
        if (_dispatchRejects.isNotEmpty) ...<Widget>[
          const SizedBox(height: 8),
          _DispatchRejectsCard(rejects: _dispatchRejects),
        ],
        if (_writeError != null) ...<Widget>[
          const SizedBox(height: 8),
          Text(
            _writeError!,
            key: const ValueKey<String>('collab-write-error'),
            style: PATypography.caption.copyWith(color: pa.danger),
          ),
        ],
      ],
    );
  }

  Widget _buildDetailBody(PAColors pa) {
    final CollabSessionUi session = _detail;
    final CollabSessionActions actions = collabSessionActions(session);
    final Map<String, String> roleNames = _roleNamesById();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        // 会话横幅：指令回显 + 汇总行 + 会话级原因 + 取消整个协同
        PACard(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Expanded(
                    child: Text(
                      session.instruction.trim().isNotEmpty
                          ? session.instruction
                          : CollabCopy.sessionTitleFallback,
                      key: const ValueKey<String>('collab-session-instruction'),
                      style: PATypography.bodyEmphasis.copyWith(
                        color: pa.fgPrimary,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  _StatusBadge(status: session.status, raw: session.statusRaw),
                ],
              ),
              if (collabSummaryLine(collabMemberCounts(session.members))
                  case final String summary) ...<Widget>[
                const SizedBox(height: 4),
                Text(
                  summary,
                  key: const ValueKey<String>('collab-session-summary'),
                  style: PATypography.caption.copyWith(color: pa.fgSecondary),
                ),
              ],
              if (session.sessionError.trim().isNotEmpty) ...<Widget>[
                const SizedBox(height: 8),
                Text(
                  session.sessionError,
                  key: const ValueKey<String>('collab-session-error'),
                  style: PATypography.caption.copyWith(color: pa.danger),
                ),
              ],
              const SizedBox(height: 6),
              // 会话级动作：取消整个协同（可点 = 主色入口；禁用 = 灰字 + 原因）
              if (actions.canCancel)
                _ActionText(
                  label: CollabCopy.actionCancelSession,
                  textKey: 'collab-cancel-session',
                  enabled: true,
                  busy: _writeInFlight == CollabCopy.actionCancelSession,
                  onTap: () => _openWriteConfirm(
                    _PendingWrite.cancelSession(
                      sessionId: session.sessionId ?? '',
                      run: () async {
                        final CollabSessionUi next = await _gateway
                            .cancelSession(session.sessionId ?? '');
                        if (!mounted) return;
                        setState(() {
                          _detail = next;
                          _detailPhase = _DetailPhase.ready;
                        });
                      },
                    ),
                  ),
                )
              else
                Text(
                  '${CollabCopy.actionCancelSession} · ${actions.cancelReason ?? ''}',
                  key: const ValueKey<String>('collab-cancel-session-reason'),
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
            ],
          ),
        ),
        const SizedBox(height: 8),
        if (session.members.isEmpty)
          Text(
            CollabCopy.membersPendingNote,
            key: const ValueKey<String>('collab-members-pending'),
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          )
        else
          for (final CollabMemberUi m in session.members)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: _MemberCard(
                member: m,
                roleName: collabMemberRoleName(m, roleNames),
                busyLabel: _writeInFlight,
                onRetry: () => _openWriteConfirm(
                  _PendingWrite.retryMember(
                    agentId: m.agentId,
                    run: () async {
                      final CollabMemberUi next = await _gateway.retryMember(
                        session.sessionId ?? '',
                        m.threadId,
                      );
                      if (!mounted) return;
                      setState(() => _applyMember(next));
                    },
                  ),
                ),
                onCancel: () => _openWriteConfirm(
                  _PendingWrite.cancelMember(
                    agentId: m.agentId,
                    run: () async {
                      final CollabMemberUi next = await _gateway.cancelMember(
                        session.sessionId ?? '',
                        m.threadId,
                      );
                      if (!mounted) return;
                      setState(() => _applyMember(next));
                    },
                  ),
                ),
              ),
            ),
      ],
    );
  }

  /// 写动作应答的成员卡上树（按 key 替换，顺序保持受理序；卡不在名册时不追加，
  /// 触发一次详情重建拿真名册——与 Kotlin `applyMember` 的键定位同规则）。
  void _applyMember(CollabMemberUi next) {
    final List<CollabMemberUi> members = _detail.members;
    final int idx = members.indexWhere((m) => m.key == next.key);
    if (idx < 0) {
      final String? sid = _detail.sessionId;
      if (sid != null) _loadDetail(sid);
      return;
    }
    setState(() {
      final List<CollabMemberUi> list = List<CollabMemberUi>.of(members);
      list[idx] = next;
      _detail = CollabSessionUi(
        sessionId: _detail.sessionId,
        status: collabDeriveStatus(list),
        statusRaw: _detail.statusRaw,
        instruction: _detail.instruction,
        sessionError: _detail.sessionError,
        workspace: _detail.workspace,
        members: list,
        loading: false,
        error: null,
      );
    });
  }

  // ── 策略区（只读）───────────────────────────────────────────────────────

  Widget _buildPolicySection(PAColors pa) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text('协同策略', style: PATypography.titleM.copyWith(color: pa.fgPrimary)),
        const SizedBox(height: 4),
        Text(
          '只读呈现电脑端当前生效的策略；修改留后续波次',
          key: const ValueKey<String>('collab-policy-readonly-note'),
          style: PATypography.caption.copyWith(color: pa.fgTertiary),
        ),
        const SizedBox(height: 8),
        switch (_policyPhase) {
          _PolicyPhase.loading => const _LoadingView(
            message: '正在读取协同策略…',
            textKey: 'collab-policy-loading',
          ),
          _PolicyPhase.error => _ErrorView(
            title: '策略读取失败',
            message: _policyError,
            onRetry: _loadPolicy,
            textKey: 'collab-policy-error',
          ),
          _PolicyPhase.ready => _buildPolicyBody(pa),
        },
      ],
    );
  }

  Widget _buildPolicyBody(PAColors pa) {
    final CollabPolicyUi p = _policy;
    final List<_PolicyRowData> rows = <_PolicyRowData>[
      _PolicyRowData(
        label: '协同总开关',
        value: _boolLabel(p.effectiveCollaborationEnabled),
        textKey: 'collab-policy-collaboration',
      ),
      _PolicyRowData(
        label: '原生委派',
        value: _boolLabel(p.effectiveNativeDelegateEnabled),
        textKey: 'collab-policy-delegate',
      ),
      _PolicyRowData(
        label: '协同扇出',
        value: _boolLabel(p.effectiveFanoutEnabled),
        textKey: 'collab-policy-fanout',
      ),
      _PolicyRowData(
        label: '单次扇出成员上限',
        value: _limitLabel(p.effectiveMaxParallelMembers),
        textKey: 'collab-policy-limit',
      ),
    ];
    return PACard(
      padding: EdgeInsets.zero,
      child: Column(
        children: <Widget>[
          for (int i = 0; i < rows.length; i++) ...<Widget>[
            if (i > 0)
              const Padding(
                padding: EdgeInsets.only(left: 14),
                child: Divider(height: 1),
              ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
              child: Row(
                children: <Widget>[
                  Expanded(
                    child: Text(
                      rows[i].label,
                      style: PATypography.bodySmall.copyWith(
                        color: pa.fgPrimary,
                      ),
                    ),
                  ),
                  Text(
                    rows[i].value,
                    key: ValueKey<String>(rows[i].textKey),
                    style: PATypography.caption.copyWith(color: pa.fgSecondary),
                  ),
                ],
              ),
            ),
          ],
          if (p.policyPath.isNotEmpty || p.revision > 0)
            Padding(
              padding: const EdgeInsets.fromLTRB(14, 0, 14, 12),
              child: Text(
                <String>[
                  if (p.revision > 0) '策略版本 ${p.revision}',
                  if (p.policyPath.isNotEmpty) p.policyPath,
                ].join(' · '),
                key: const ValueKey<String>('collab-policy-meta'),
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
            ),
        ],
      ),
    );
  }

  static String _boolLabel(bool? v) {
    if (v == null) return '电脑端没有返回';
    return v ? '已开启' : '已关闭';
  }

  static String _limitLabel(int? v) {
    if (v == null) return '电脑端没有返回';
    // proto 注明 <=0 = 不限；如实照述，不编一个数字。
    return v <= 0 ? '不限' : '$v 位';
  }
  // ── 派发区 ──────────────────────────────────────────────────────────────

  Widget _buildDispatchBar(PAColors pa) {
    final CollabDispatchDecision decision = _decision;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        if (_lastInstruction != null &&
            (_detail.sessionId == null || _dispatching)) ...<Widget>[
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Icon(Icons.account_tree_rounded, size: 16, color: pa.brand),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  _lastInstruction!,
                  key: const ValueKey<String>('collab-last-instruction'),
                  style: PATypography.bodySmall.copyWith(color: pa.fgPrimary),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            _dispatching
                ? CollabCopy.btnDispatching
                : CollabCopy.progressSubmittedNote,
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
          const SizedBox(height: 8),
        ],
        if (_selected.isEmpty && _collabAgentIds.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text(
              CollabCopy.emptyNoSelection,
              key: const ValueKey<String>('collab-no-selection'),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ),
        // 禁用原因与按钮同源（按钮灰 = 旁边一定有原因）；可派发时不占一行。
        if (!decision.canDispatch && decision.disabledReason != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Text(
              decision.disabledReason!,
              key: const ValueKey<String>('collab-dispatch-reason'),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: <Widget>[
            Expanded(
              child: TextField(
                key: const ValueKey<String>('collab-cmd-input'),
                controller: _cmdCtrl,
                minLines: 1,
                maxLines: 5,
                style: PATypography.body.copyWith(color: pa.fgPrimary),
                onChanged: (_) => setState(() {}),
                onSubmitted: (_) {
                  if (_decision.canDispatch) _dispatch();
                },
                decoration: InputDecoration(
                  isDense: true,
                  hintText: '描述需要协同完成的任务…',
                  hintStyle: PATypography.body.copyWith(color: pa.fgTertiary),
                  filled: true,
                  fillColor: pa.bgSurface,
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 12,
                  ),
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
              key: const ValueKey<String>('collab-dispatch-button'),
              label: _dispatching
                  ? CollabCopy.btnDispatching
                  : CollabCopy.btnDispatch,
              expanded: false,
              onPressed: decision.canDispatch ? _dispatch : null,
            ),
          ],
        ),
      ],
    );
  }

  // ── 弹层 ────────────────────────────────────────────────────────────────

  /// 打开角色选择弹层（芯片的「配角色」入口）。
  void _openRoleSheet(String agentId) {
    final List<CollabRole> roles = _roles
        .where((r) => r.agentId == agentId)
        .toList(growable: false);
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.pa.bgSurface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (BuildContext sheetContext) {
        return _RolePickerSheet(
          agentId: agentId,
          currentRoleId: _roleBinding[agentId],
          roles: roles,
          rolesPhase: _rolesPhase,
          rolesError: _rolesError,
          onPick: (String? roleId) {
            // 先关弹层再落绑定（顺序反了会让弹层关掉前的最后一帧读旧绑定）。
            Navigator.of(sheetContext).pop();
            _bindRole(agentId, roleId);
          },
          onDismiss: () => Navigator.of(sheetContext).pop(),
        );
      },
    );
  }

  /// 打开写操作确认弹层（取消/重试都在这里过一道显式确认）。
  ///
  /// 【为什么必须确认】取消/重试是甲级写操作（改的是电脑端的执行）：误触一次
  /// 的代价不可逆（取消后不会自动恢复，重试会多跑一轮次）。弹层把后果写明白，
  /// 确认键也带具体动作名（「取消整个协同」而不是光秃秃的「确定」）。
  void _openWriteConfirm(_PendingWrite pending) {
    // 在飞时不叠弹层（一个写操作在飞，第二个请求连弹层都不开）。
    if (_writeInFlight != null) return;
    setState(() => _writeError = null);
    showDialog<void>(
      context: context,
      builder: (BuildContext dialogContext) {
        return AlertDialog(
          backgroundColor: context.pa.bgSurface,
          title: Text(
            pending.title,
            key: const ValueKey<String>('collab-confirm-title'),
            style: PATypography.titleM.copyWith(color: context.pa.fgPrimary),
          ),
          content: Text(
            pending.body,
            key: const ValueKey<String>('collab-confirm-body'),
            style: PATypography.bodySmall.copyWith(
              color: context.pa.fgSecondary,
            ),
          ),
          actions: <Widget>[
            TextButton(
              key: const ValueKey<String>('collab-confirm-dismiss'),
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: Text(
                CollabCopy.confirmCancel,
                style: PATypography.caption.copyWith(
                  color: context.pa.fgSecondary,
                ),
              ),
            ),
            TextButton(
              key: const ValueKey<String>('collab-confirm-ok'),
              onPressed: () {
                Navigator.of(dialogContext).pop();
                _runWrite(pending);
              },
              child: Text(
                pending.confirmLabel,
                style: PATypography.caption.copyWith(
                  color: context.pa.brand,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ],
        );
      },
    );
  }
}

// ── 零件 ────────────────────────────────────────────────────────────────────

/// 策略区的一行（标签 + 值 + 测试锚点键）。
class _PolicyRowData {
  const _PolicyRowData({
    required this.label,
    required this.value,
    required this.textKey,
  });

  final String label;
  final String value;
  final String textKey;
}

/// 单个 agent 多选芯片：选中 = 薄荷浅底 + 主色描边；角色绑定显示在芯片下方。
///
/// 【为什么点芯片本身是选人、次级入口才是配角色】选人是主操作（点一下就够），
/// 配角色是可选细化；把两者挤在同一个手势上会误触。
class _AgentChip extends StatelessWidget {
  const _AgentChip({
    required this.agentId,
    required this.selected,
    required this.onTap,
    required this.onBindRole,
    this.roleName,
  });

  final String agentId;
  final bool selected;
  final String? roleName;
  final VoidCallback onTap;
  final VoidCallback onBindRole;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return GestureDetector(
      key: ValueKey<String>('collab-agent-$agentId'),
      onTap: onTap,
      child: Container(
        width: 96,
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
        decoration: BoxDecoration(
          color: selected ? pa.brand.withValues(alpha: 0.16) : pa.bgSurface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: selected ? pa.brand : pa.hairline),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Icon(
                  selected
                      ? Icons.check_circle_rounded
                      : Icons.radio_button_unchecked_rounded,
                  size: 15,
                  color: selected ? pa.brand : pa.fgTertiary,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    agentId,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: PATypography.caption.copyWith(
                      color: selected ? pa.fgPrimary : pa.fgSecondary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            // 角色行：已绑定 = 角色名（主色，可点开弹层改）；未绑定 = 「配角色」
            // 入口（灰字，点了才开弹层）——两条路都不摆假状态。
            GestureDetector(
              key: ValueKey<String>('collab-bind-role-$agentId'),
              onTap: onBindRole,
              child: Text(
                roleName ?? '配角色',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: PATypography.caption.copyWith(
                  fontSize: 11,
                  color: roleName != null ? pa.brand : pa.fgTertiary,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 七词状态徽章：词用七词词表，色/形通道借设计系统五态 chip。
///
/// UNKNOWN 不进徽章——原文照登 + 降级说明（不猜、不编一个相近的已知状态）。
class _StatusBadge extends StatelessWidget {
  const _StatusBadge({required this.status, required this.raw});

  final CollabStatus status;
  final String raw;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final PAStatusKind? kind = collabBadgeKindOf(status);
    if (kind == null) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: <Widget>[
          Text(
            collabStatusWord(status, raw),
            key: const ValueKey<String>('collab-status-raw'),
            style: PATypography.caption.copyWith(color: pa.fgSecondary),
          ),
          Text(
            CollabCopy.unknownStatusNote,
            style: PATypography.caption.copyWith(
              fontSize: 11,
              color: pa.fgTertiary,
            ),
          ),
        ],
      );
    }
    return PAStatusChip(status: kind, label: collabStatusWord(status, raw));
  }
}

/// 会话列表行：指令摘要 + 状态徽章 + 成员数 + 角色摘要 + 时间。
class _SessionRow extends StatelessWidget {
  const _SessionRow({
    required this.row,
    required this.selected,
    required this.onTap,
  });

  final CollabSessionRow row;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String instruction = row.instruction.trim().isNotEmpty
        ? row.instruction
        : CollabCopy.sessionTitleFallback;
    final List<String> meta = <String>[
      '${row.memberCount} 位成员',
      if (row.roleNames.isNotEmpty) '角色：${row.roleNames.join('、')}',
      if (row.timeLabel.isNotEmpty) row.timeLabel,
    ];
    return PACard(
      color: selected ? pa.brand.withValues(alpha: 0.10) : null,
      borderColor: selected ? pa.brand : null,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      onTap: onTap,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  instruction,
                  key: ValueKey<String>('collab-session-row-${row.id}'),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: PATypography.bodySmall.copyWith(
                    color: pa.fgPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  meta.join(' · '),
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          _StatusBadge(status: row.status, raw: row.statusRaw),
        ],
      ),
    );
  }
}

/// 成员卡：agent + 角色 + 七词徽章 + model/effort + 进度/原因 + 三动作（确认流）。
///
/// 【每个禁用按钮旁必有原因】重试/取消的可点态与禁用原因同源
/// （[collabMemberActions]）：能点的给主色入口，不能点的灰着并写明为什么——
/// 不会出现「灰着但不知道为什么」。
class _MemberCard extends StatelessWidget {
  const _MemberCard({
    required this.member,
    required this.roleName,
    required this.onRetry,
    required this.onCancel,
    this.busyLabel,
  });

  final CollabMemberUi member;
  final String? roleName;

  /// 正在飞的写操作标签（用于把对应动作置为在飞态）。
  final String? busyLabel;
  final VoidCallback onRetry;
  final VoidCallback onCancel;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final CollabMemberActions actions = collabMemberActions(member);
    return PACard(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      member.agentId,
                      key: ValueKey<String>('collab-member-${member.key}'),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: PATypography.bodyEmphasis.copyWith(
                        color: pa.fgPrimary,
                      ),
                    ),
                    if (roleName != null) ...<Widget>[
                      const SizedBox(height: 1),
                      Text(
                        '角色 · $roleName',
                        style: PATypography.caption.copyWith(
                          color: pa.fgSecondary,
                        ),
                      ),
                    ],
                    if (member.modelId.isNotEmpty || member.effort.isNotEmpty)
                      Text(
                        <String>[
                          if (member.modelId.isNotEmpty) member.modelId,
                          if (member.effort.isNotEmpty) member.effort,
                        ].join(' · '),
                        style: PATypography.caption.copyWith(
                          color: pa.fgTertiary,
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              _StatusBadge(status: member.status, raw: member.statusRaw),
            ],
          ),
          // daemon 的一句话进度（有界摘要，不是原始日志）；终局成员不叠进度行
          if (member.progress.isNotEmpty &&
              !member.status.isFinished) ...<Widget>[
            const SizedBox(height: 6),
            Text(
              member.progress,
              style: PATypography.caption.copyWith(color: pa.fgSecondary),
            ),
          ],
          // 挂起审批：等的是用户，不是系统在跑（应答入口在成员会话页，本波不摆）
          for (final CollabApprovalUi approval in member.pendingApprovals)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Text(
                CollabCopy.approvalWaitingPrefix +
                    (approval.label.isNotEmpty
                        ? approval.label
                        : '(电脑端没有给出动作说明)'),
                style: PATypography.caption.copyWith(color: pa.warn),
              ),
            ),
          // 终局成员的原因（daemon 人话原文，按成员各自展示）
          if (member.status.isFinished && member.error.isNotEmpty) ...<Widget>[
            const SizedBox(height: 6),
            Text(
              member.error,
              key: ValueKey<String>('collab-member-error-${member.key}'),
              style: PATypography.caption.copyWith(color: pa.danger),
            ),
          ],
          // 成功成员的终稿摘要（失败/取消时为空，不伪造）
          if (member.status.isFinished &&
              member.summary.isNotEmpty) ...<Widget>[
            const SizedBox(height: 6),
            Text(
              member.summary,
              style: PATypography.caption.copyWith(color: pa.fgPrimary),
            ),
          ],
          const SizedBox(height: 6),
          Row(
            children: <Widget>[
              _ActionText(
                label: CollabCopy.actionRetry,
                textKey: 'collab-retry-${member.key}',
                enabled: actions.canRetry,
                reason: actions.retryReason,
                busy: busyLabel == CollabCopy.actionRetry,
                onTap: onRetry,
              ),
              const SizedBox(width: 10),
              _ActionText(
                label: CollabCopy.actionCancelMember,
                textKey: 'collab-cancel-member-${member.key}',
                enabled: actions.canCancel,
                reason: actions.cancelReason,
                busy: busyLabel == CollabCopy.actionCancelMember,
                onTap: onCancel,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// 卡上动作：可点 = 主色入口；禁用 = 灰字 + 禁用原因（不占「能点」的形）。
class _ActionText extends StatelessWidget {
  const _ActionText({
    required this.label,
    required this.textKey,
    required this.enabled,
    required this.onTap,
    this.reason,
    this.busy = false,
  });

  final String label;
  final String textKey;
  final bool enabled;
  final String? reason;
  final bool busy;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    if (enabled) {
      return GestureDetector(
        key: ValueKey<String>(textKey),
        onTap: busy ? null : onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Text(
            busy ? '$label…' : label,
            style: PATypography.caption.copyWith(
              color: busy ? pa.fgTertiary : pa.brand,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      );
    }
    return Text(
      '$label · ${reason ?? '当前不可用'}',
      key: ValueKey<String>('$textKey-reason'),
      style: PATypography.caption.copyWith(color: pa.fgTertiary),
    );
  }
}

/// 派发受理失败卡（一位失败不拖垮其余；逐位列出 daemon 给的人话原因）。
class _DispatchRejectsCard extends StatelessWidget {
  const _DispatchRejectsCard({required this.rejects});

  final List<CollabDispatchItemUi> rejects;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return PACard(
      color: pa.warn.withValues(alpha: 0.12),
      borderColor: pa.warn,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            '${rejects.length} 位成员没有被受理，其余成员照常执行',
            key: const ValueKey<String>('collab-dispatch-rejects'),
            style: PATypography.caption.copyWith(
              color: pa.fgPrimary,
              fontWeight: FontWeight.w600,
            ),
          ),
          for (final CollabDispatchItemUi item in rejects)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                '${item.agentId}：${item.error.isNotEmpty ? item.error : '(电脑端没有给出原因)'}',
                style: PATypography.caption.copyWith(color: pa.fgSecondary),
              ),
            ),
        ],
      ),
    );
  }
}

/// 角色选择底部弹层：未配置角色（解绑）+ 该 agent 的角色 + 加载/失败态。
///
/// 【管理角色入口为什么不摆】协同角色管理页（collabRoles）由另一子代理承接，
/// 本波没有到那页的导航通道——没有通道就没有假入口（宁缺勿假）。
class _RolePickerSheet extends StatelessWidget {
  const _RolePickerSheet({
    required this.agentId,
    required this.currentRoleId,
    required this.roles,
    required this.rolesPhase,
    required this.rolesError,
    required this.onPick,
    required this.onDismiss,
  });

  final String agentId;
  final String? currentRoleId;
  final List<CollabRole> roles;
  final _RolesPhase rolesPhase;
  final String rolesError;
  final void Function(String? roleId) onPick;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Expanded(
                  child: Text(
                    '为 $agentId 选角色',
                    style: PATypography.titleM.copyWith(color: pa.fgPrimary),
                  ),
                ),
                IconButton(
                  key: const ValueKey<String>('collab-role-sheet-close'),
                  onPressed: onDismiss,
                  tooltip: '关闭',
                  icon: const Icon(Icons.close_rounded, size: 18),
                  color: pa.fgSecondary,
                ),
              ],
            ),
            Text(
              '同一 agent 可绑定不同岗位设定',
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
            const SizedBox(height: 12),
            Flexible(
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    _RolePickRow(
                      title: CollabCopy.roleNone,
                      desc: '按 agent 默认行为执行',
                      selected: currentRoleId == null,
                      textKey: 'collab-role-none',
                      onTap: () => onPick(null),
                    ),
                    const SizedBox(height: 8),
                    switch (rolesPhase) {
                      _RolesPhase.loading => Text(
                        CollabCopy.loadingAgents,
                        key: const ValueKey<String>('collab-roles-loading'),
                        style: PATypography.caption.copyWith(
                          color: pa.fgTertiary,
                        ),
                      ),
                      _RolesPhase.error => Text(
                        rolesError,
                        key: const ValueKey<String>('collab-roles-error'),
                        style: PATypography.caption.copyWith(color: pa.danger),
                      ),
                      _RolesPhase.ready =>
                        roles.isEmpty
                            ? Text(
                                '该 agent 还没有专属角色；不配角色也能派发',
                                key: const ValueKey<String>(
                                  'collab-roles-empty',
                                ),
                                style: PATypography.caption.copyWith(
                                  color: pa.fgTertiary,
                                ),
                              )
                            : Column(
                                children: <Widget>[
                                  for (final CollabRole role in roles)
                                    Padding(
                                      padding: const EdgeInsets.only(bottom: 8),
                                      child: _RolePickRow(
                                        title: collabRoleDisplayName(
                                          role.name,
                                          role.id,
                                        ),
                                        desc: _roleSummary(role),
                                        selected: currentRoleId == role.id,
                                        textKey: 'collab-role-${role.id}',
                                        onTap: () => onPick(role.id),
                                      ),
                                    ),
                                ],
                              ),
                    },
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// 角色摘要行（对位 `collabRoleSummaryLine`）：agent + model/effort，缺项省略。
  static String _roleSummary(CollabRole role) {
    final List<String> parts = <String>[
      if (role.agentId.isNotEmpty) role.agentId,
      if (role.model.isNotEmpty) role.model,
      if (role.effort.isNotEmpty) role.effort,
    ];
    return parts.join(' · ');
  }
}

/// 角色选择项：选中 = 主色描边 + 对勾。
class _RolePickRow extends StatelessWidget {
  const _RolePickRow({
    required this.title,
    required this.desc,
    required this.selected,
    required this.textKey,
    required this.onTap,
  });

  final String title;
  final String desc;
  final bool selected;
  final String textKey;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: pa.bgSurface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: selected ? pa.brand : pa.hairline),
        ),
        child: Row(
          children: <Widget>[
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    title,
                    key: ValueKey<String>(textKey),
                    style: PATypography.body.copyWith(
                      color: pa.fgPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (desc.isNotEmpty) ...<Widget>[
                    const SizedBox(height: 2),
                    Text(
                      desc,
                      style: PATypography.caption.copyWith(
                        color: pa.fgTertiary,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            if (selected) Icon(Icons.check_rounded, size: 18, color: pa.brand),
          ],
        ),
      ),
    );
  }
}

// ── 三态公共件（与 Threads/Files/Git 页同款：固定 padding，一眼可见在转）────

class _LoadingView extends StatelessWidget {
  const _LoadingView({required this.message, required this.textKey});

  final String message;
  final String textKey;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 18),
      child: Row(
        children: <Widget>[
          SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2.5, color: pa.brand),
          ),
          const SizedBox(width: 10),
          Text(
            message,
            key: ValueKey<String>(textKey),
            style: PATypography.caption.copyWith(color: pa.fgSecondary),
          ),
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({
    required this.title,
    required this.message,
    required this.onRetry,
    required this.textKey,
  });

  /// 错误态标题（Threads/Files/Git 三页同款「XX 加载失败」口径）。
  final String title;
  final String message;
  final Future<void> Function() onRetry;
  final String textKey;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: pa.bgSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: pa.hairline),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            title,
            style: PATypography.body.copyWith(
              color: pa.fgPrimary,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            message,
            key: ValueKey<String>(textKey),
            style: PATypography.caption.copyWith(color: pa.danger),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerLeft,
            child: PAButton(label: '重试', expanded: false, onPressed: onRetry),
          ),
        ],
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView({
    required this.title,
    required this.hint,
    required this.textKey,
  });

  final String title;
  final String hint;
  final String textKey;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: pa.bgSurface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: pa.hairline),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            title,
            key: ValueKey<String>(textKey),
            style: PATypography.body.copyWith(
              color: pa.fgPrimary,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            hint,
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ],
      ),
    );
  }
}
