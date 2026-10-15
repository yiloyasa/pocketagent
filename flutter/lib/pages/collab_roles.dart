// 协同角色列表页（page-contracts §1.11 CollabRoles）——看到 / 新增 / 编辑 /
// 删除。
//
// 【页面职责】对位 Android `ui/CollabRolesScreen.kt`（317 行）：三态
// （LOADING/READY/ERROR）+ 行渲染（显示名 + agent/model/effort 摘要）+ 删除
// 二次确认 + 空态（还没有配置协同角色 + 新建入口）+ 右上新建进编辑器
// （空 id = 新建）。
//
// 【数据来源】[CollabRolesStore]（collab_roles_state.dart）：进页 load()，
// 编辑器保存/删除后 store 自行刷新，返回本页即见最新——与 Kotlin 的
// 「状态对象跨页共享」同构，只是 Dart 侧用构造注入实现共享（见 store 文件
// 头的「对位 object」说明）。页面只认 roles/loadState/loadError 三个读面。
//
// 【诚实红线（B4 前）】宿主桥未注册 → listCollabRoles 必然抛
// HOST_UNAVAILABLE，store 进 ERROR 态，本页显示「协同角色加载失败」+ daemon
// 原文 + 重试，**不伪装成空列表**（空态与错误态两条路径，测试各钉一条，
// 互斥断言——红线：不留假绿）。
//
// 【删除确认的视觉纪律】破坏性动作 = 危险色**描边**按钮（PAButton 的
// danger 变体），主按钮（primary 品牌蓝）永不红——确认弹层里「删除」用
// danger，「取消」用 neutral（pa_button.dart 的设计红线照搬）。
//
// 【删除失败怎么呈现】Android 用 Toast 透传 daemon 原文；Flutter 侧没有
// Toast 基础设施（theme/components 都不在授权内），改为在弹层关闭后于列表
// 顶部显示一条可关闭的错误条（文案 = daemon 原文，不泛化）。成功时 store 已
// 自行移除该行，页面直接少一行。
import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'collab_roles_data.dart';
import 'collab_roles_state.dart';
import 'pa_page_kit.dart';

/// 协同角色列表页。
///
/// 三个注入点（测试可控，生产走真通道）：
///  - [store]：角色三态 + save/delete（默认 [CollabRolesStore]，生产由根装配
///    持共享实例——B4 接线点）；
///  - [onOpenEditor]：点行 / 点新建 → 编辑器（空 id = 新建）；
///  - [onBack]：返回（Kotlin SubPageScaffold 的 onBack）。
class CollabRolesPage extends StatefulWidget {
  const CollabRolesPage({
    super.key,
    this.store,
    required this.onOpenEditor,
    required this.onBack,
  });

  final CollabRolesStore? store;
  final ValueChanged<String> onOpenEditor;
  final VoidCallback onBack;

  @override
  State<CollabRolesPage> createState() => _CollabRolesPageState();
}

/// 三态：加载中 / 加载失败 / 已就绪（就绪后再分「空列表」与「有角色」）。
///
/// 与 threads.dart 的 `_ThreadsPhase` 同名同义同序——直接复用共享词汇
/// [PaAsyncPhase]（typedef 零运行时开销、零渲染；switch 仍是穷尽匹配，
/// analyze 守着）。
typedef _RolesPhase = PaAsyncPhase;

class _CollabRolesPageState extends State<CollabRolesPage> {
  late final CollabRolesStore _store = widget.store ?? CollabRolesStore();

  /// 页面本地三态镜像（store 的 loadState → 页面 phase）。
  ///
  /// 【为什么要镜像而不是直接读 store】widget 测试要按 phase 断言，且
  /// store 的 load() 是 fire-and-forget（Kotlin 同款）：状态收敛发生在
  /// microtask 里，页面用 setState 把新 phase 落到下一帧。镜像还让「删除
  /// 失败错误条」这类纯页面状态与 store 的数据状态分开。
  _RolesPhase _phase = _RolesPhase.loading;
  String _errorText = '';

  /// 删除失败的人话原因（daemon 原文透传，可关闭；下次删除前清掉）。
  String? _deleteError;

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// 进页即 load（Kotlin `LaunchedEffect(Unit) { CollabRolesState.load() }`）。
  ///
  /// 等 store 的加载 Future 收敛后再翻页面 phase——不用 sleep，也不用
  /// `pumpAndSettle` 赌时序。
  Future<void> _load() async {
    if (_phase != _RolesPhase.loading || _errorText.isNotEmpty) {
      setState(() {
        _phase = _RolesPhase.loading;
        _errorText = '';
      });
    }
    _store.load();
    await _store.loadCompleted;
    if (!mounted) return;
    setState(() => _syncPhase());
  }

  /// store 三态 → 页面 phase（含错误文案）。
  void _syncPhase() {
    switch (_store.loadState) {
      case CollabRolesLoadState.loading:
        _phase = _RolesPhase.loading;
        _errorText = '';
      case CollabRolesLoadState.error:
        _phase = _RolesPhase.error;
        _errorText = collabRolesErrorText(_store.loadError);
      case CollabRolesLoadState.ready:
        _phase = _RolesPhase.ready;
        _errorText = '';
    }
  }

  /// 删除确认流（甲级写操作：执行前必须显式确认）。
  ///
  /// 确认弹层把后果写明白（引用它的委派会报「未定义的协同角色」，正在进行的
  /// 协作可能中断，不可撤销）——Android 同款文案口径。
  Future<void> _confirmDelete(CollabRole role) async {
    final String name = collabRoleDisplayName(role);
    final bool? ok = await showDialog<bool>(
      context: context,
      builder: (BuildContext dialogContext) {
        return AlertDialog(
          backgroundColor: context.pa.bgSurface,
          title: Text(
            '删除协同角色',
            key: const ValueKey<String>('collab-roles-delete-title'),
            style: PATypography.titleM.copyWith(color: context.pa.fgPrimary),
          ),
          content: Text(
            '确定删除「$name」(${role.id})?\n\n'
            '删除后,引用它的委派会报「未定义的协同角色 ${role.id}」,'
            '正在进行的协作可能中断。此操作不可撤销。',
            key: const ValueKey<String>('collab-roles-delete-body'),
            style: PATypography.bodySmall.copyWith(
              color: context.pa.fgSecondary,
            ),
          ),
          actions: <Widget>[
            // 取消 = neutral 描边（中性，不抢破坏性动作的语义色）。
            PAButton(
              label: '取消',
              variant: PAButtonVariant.neutral,
              expanded: false,
              onPressed: () => Navigator.of(dialogContext).pop(false),
            ),
            // 删除 = danger 描边（主按钮永不红的纪律：破坏性动作用描边表达）。
            PAButton(
              label: '删除',
              variant: PAButtonVariant.danger,
              expanded: false,
              onPressed: () => Navigator.of(dialogContext).pop(true),
            ),
          ],
        );
      },
    );
    if (ok != true) return;
    await _runDelete(role);
  }

  Future<void> _runDelete(CollabRole role) async {
    setState(() => _deleteError = null);
    final String? err = await _store.delete(role.id);
    if (!mounted) return;
    if (err != null) {
      // daemon 原文如实透传（不泛化成「操作失败」）；store 未动，行还在。
      setState(() => _deleteError = err);
      return;
    }
    // 成功：store 已自行移除该行，页面重同步 phase（列表可能变空 → 空态）。
    setState(() => _syncPhase());
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        PaSubPageHeader(
          title: '协同角色',
          backKey: 'collab-roles-back',
          onBack: widget.onBack,
          // 唯一次操作：新建（空态与有数据都一样可达——Kotlin 把新建放在
          // Scaffold 右上，Flutter 侧收在头部右侧同一语义位）。
          onRefresh: null,
          bottomPadding: 14,
        ),
        Expanded(
          child: _buildBody(pa),
        ),
      ],
    );
  }

  Widget _buildBody(PAColors pa) {
    switch (_phase) {
      case _RolesPhase.loading:
        return _RolesLoadingView(message: '正在加载协同角色…');
      case _RolesPhase.error:
        return _RolesErrorView(
          message: _errorText,
          onRetry: _load,
        );
      case _RolesPhase.ready:
        return _buildReadyBody(pa);
    }
  }

  Widget _buildReadyBody(PAColors pa) {
    final List<CollabRole> roles = _store.roles;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
      children: <Widget>[
        if (_deleteError != null)
          _DeleteErrorBar(
            message: _deleteError!,
            onDismiss: () => setState(() => _deleteError = null),
          ),
        if (roles.isEmpty)
          _RolesEmptyView(onNew: () => widget.onOpenEditor(''))
        else ...<Widget>[
          for (int i = 0; i < roles.length; i++) ...<Widget>[
            if (i > 0) const SizedBox(height: 10),
            _RoleRow(
              role: roles[i],
              onTap: () => widget.onOpenEditor(roles[i].id),
              onDelete: () => _confirmDelete(roles[i]),
            ),
          ],
          const SizedBox(height: 10),
          // 底部新建入口（列表长时不用回顶部；与空态入口同一回调）。
          PAButton(
            label: '新建协同角色',
            icon: Icons.add_rounded,
            expanded: false,
            onPressed: () => widget.onOpenEditor(''),
          ),
        ],
      ],
    );
  }
}

/// 单个角色行：显示名 + agent/model/effort 摘要 + 职责预览；点进编辑，
/// 删除走二次确认。
class _RoleRow extends StatelessWidget {
  const _RoleRow({
    required this.role,
    required this.onTap,
    required this.onDelete,
  });

  final CollabRole role;
  final VoidCallback onTap;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String summary = collabRoleSummaryLine(role);
    final String description = role.description.trim();
    return PACard(
      padding: const EdgeInsets.fromLTRB(14, 12, 6, 12),
      onTap: onTap,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  collabRoleDisplayName(role),
                  key: ValueKey<String>('collab-roles-name-${role.id}'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: PATypography.bodyEmphasis.copyWith(color: pa.fgPrimary),
                ),
                if (summary.isNotEmpty) ...<Widget>[
                  const SizedBox(height: 3),
                  Text(
                    summary,
                    key: ValueKey<String>('collab-roles-summary-${role.id}'),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: PATypography.caption.copyWith(color: pa.fgSecondary),
                  ),
                ],
                if (description.isNotEmpty) ...<Widget>[
                  const SizedBox(height: 3),
                  Text(
                    description,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: PATypography.caption.copyWith(color: pa.fgTertiary),
                  ),
                ],
              ],
            ),
          ),
          IconButton(
            key: ValueKey<String>('collab-roles-delete-${role.id}'),
            onPressed: onDelete,
            tooltip: '删除',
            icon: const Icon(Icons.delete_outline_rounded, size: 18),
            color: pa.fgTertiary,
          ),
        ],
      ),
    );
  }
}

/// 空态：人话说明 + 新建入口（无数据时绝不显示「未知/错误」，AUDIT §3.3）。
class _RolesEmptyView extends StatelessWidget {
  const _RolesEmptyView({required this.onNew});

  final VoidCallback onNew;

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
              '还没有配置协同角色',
              key: const ValueKey<String>('collab-roles-empty-title'),
              style: PATypography.body.copyWith(
                color: pa.fgPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 6),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Text(
                '配置后,委派子任务时直接引用角色 id 即可复用同一套岗位设定',
                textAlign: TextAlign.center,
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
            ),
            const SizedBox(height: 14),
            PAButton(
              label: '新建协同角色',
              icon: Icons.add_rounded,
              expanded: false,
              onPressed: onNew,
            ),
          ],
        ),
      ),
    );
  }
}

/// 加载态（固定 padding 而不是全屏居中：ListView 内的居中要靠 shrinkWrap/
/// sliver 技巧，徒增复杂度，加载态只要「一眼可见在转」）。
class _RolesLoadingView extends StatelessWidget {
  const _RolesLoadingView({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(top: 48),
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

/// 错误态：标题 + 人话原因 + 重试出口（原因原样透传，不泛化；重试是真的
/// 重新 load）。
class _RolesErrorView extends StatelessWidget {
  const _RolesErrorView({required this.message, required this.onRetry});

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
              '协同角色加载失败',
              key: const ValueKey<String>('collab-roles-error-title'),
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

/// 删除失败条（Flutter 侧没有 Toast：列表页顶部可关闭错误条，daemon 原文）。
class _DeleteErrorBar extends StatelessWidget {
  const _DeleteErrorBar({required this.message, required this.onDismiss});

  final String message;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: PACard(
        color: pa.danger.withValues(alpha: 0.12),
        borderColor: pa.danger,
        padding: const EdgeInsets.fromLTRB(14, 10, 6, 10),
        child: Row(
          children: <Widget>[
            Expanded(
              child: Text(
                message,
                key: const ValueKey<String>('collab-roles-delete-error'),
                style: PATypography.caption.copyWith(color: pa.danger),
              ),
            ),
            IconButton(
              key: const ValueKey<String>('collab-roles-delete-error-close'),
              onPressed: onDismiss,
              tooltip: '关闭',
              icon: const Icon(Icons.close_rounded, size: 16),
              color: pa.danger,
            ),
          ],
        ),
      ),
    );
  }
}
