// 协同角色编辑页（page-contracts §1.12 CollabRoleEditor）——新增/编辑共用的
// 八字段表单。
//
// 【页面职责】对位 Android `ui/CollabRoleEditorScreen.kt`（343 行）：
// idText/nameText/agentId/description/instructions/modelText/effort/workspaceText
// 八个字段 + saving/saveError/idError + agent 下拉 + 保存成功 onBack。
//
// 【id 是引用键】新增时可编辑；编辑时只读——改了会让已有委派里的 role_id
// 引用失效（Kotlin 把只读 id 渲染成瓷白卡内的扁平行，这里同款）。
// 编辑态 id 的实时校验**不做**（Kotlin `if (isNew) validateCollabRoleId(...)`：
// 只读字段必已合法，校验它只会挡住用户保存一个本来合法的角色）。
// 但保存仍走 store → daemon 的同口径校验：daemon 若因并发边界返回
// InvalidArgument，其原文经 saveError 如实显示（第一道闸门不是吞错误的借口）。
//
// 【agent 下拉不写死】目标 agent 候选 = `collab:<id>` 位优先 + `agent:<id>` 位
// 补充（`collabEditorAgentCandidates`，data 层纯函数）。没有位数据时如实
// 降级：下拉显示手填说明 + 保存仍可用（agent_id 透传真值，存在性裁决在
// daemon 委派那一刻，collab_roles.go:106-110）。Flutter 侧没有 proto 的
// listAgents RPC，拿不到 Android buildAgents 的静态目录——**不编五家名单
// 充数**（编了就是第二份写死名单，daemon 加新 agent 时旧 APK 反而漏它）。
//
// 【workspace 暴露但标清「留空=用默认」】用户手填路径极易填错，所以输入框
// 明确提示留空即走 daemon 默认工作区，避免半成品式的「填了又不生效」——
// 字段如实透传给服务端，不做任何前端假校验（Kotlin 同款纪律）。
//
// 【诚实红线（B4 前）】宿主桥未注册 → setCollabRole 抛 HOST_UNAVAILABLE，
// 文案经 saveError 显示「协同角色通道未就绪（B4 波次）」，不假装保存成功。
import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../gen/pocketagent/v1/host.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'collab_roles_data.dart';
import 'collab_roles_state.dart';
import 'pa_page_kit.dart';

/// 协同角色编辑页。
///
/// 三个注入点（测试可控，生产走真通道）：
///  - [store]：save() + 进页预填的来源（roles 内存态，Kotlin 同款「列表页
///    已加载过，这里一定找得到」）；
///  - [capabilities]：能力位快照（agent 下拉候选；默认空 = 调用方没给，
///    页面自己读 getHostInfo 拿一次——读不到按无位数据诚实降级）；
///  - [onBack]：保存成功 / 返回键 / 找不到角色时的出口（无其他出口）。
class CollabRoleEditorPage extends StatefulWidget {
  const CollabRoleEditorPage({
    super.key,
    required this.roleId,
    this.store,
    this.capabilities,
    required this.onBack,
  });

  /// 目标角色 id；空 = 新建（Kotlin `roleId.isBlank()` 同款分流）。
  final String roleId;

  final CollabRolesStore? store;

  /// 能力位列表（`collab:<id>` / `agent:<id>` 决定 agent 下拉候选）；
  /// null = 页面自己读一次 getHostInfo（生产路径）。
  final List<String>? capabilities;

  final VoidCallback onBack;

  @override
  State<CollabRoleEditorPage> createState() => _CollabRoleEditorPageState();
}

class _CollabRoleEditorPageState extends State<CollabRoleEditorPage> {
  late final CollabRolesStore _store = widget.store ?? CollabRolesStore();
  late final bool _isNew = widget.roleId.trim().isEmpty;

  /// 已存在的角色（编辑态用；新建态为 null）。
  ///
  /// 深链进来若 store 还没加载过 → 找不到：页面显示「找不到该角色(可能已被
  /// 删除)」+ 返回，不做半成品编辑（Kotlin 同款降级）。
  CollabRole? get _existing => _isNew ? null : _store.findById(widget.roleId);

  // ── 表单状态（编辑态用已有角色预填；新增态给合理默认）──
  late final TextEditingController _idCtrl =
      TextEditingController(text: _existing?.id ?? '');
  late final TextEditingController _nameCtrl =
      TextEditingController(text: _existing?.name ?? '');
  late final TextEditingController _descCtrl =
      TextEditingController(text: _existing?.description ?? '');
  late final TextEditingController _instrCtrl =
      TextEditingController(text: _existing?.instructions ?? '');
  late final TextEditingController _modelCtrl =
      TextEditingController(text: _existing?.model ?? '');
  late final TextEditingController _wsCtrl =
      TextEditingController(text: _existing?.workspace ?? '');

  /// 目标 agent：已接入协同的第一家 → 候选第一家 → 空串（Kotlin
  /// `collabFirst ?: agents.firstOrNull()?.id ?: ""` 同款优先级）。
  ///
  /// 【为什么不用 late final + 构造期算】能力位可能是异步读回来的
  /// （[_loadCapabilities]），候选与默认值都要等它；用可变字段 + setState。
  String _agentId = '';

  /// 思考强度（空 = 跟随默认）。
  String _effort = '';

  bool _saving = false;

  /// 保存失败原因（daemon 还原的原文，如实显示；下次保存前清掉）。
  String? _saveError;

  @override
  void initState() {
    super.initState();
    _agentId = _existing?.agentId ?? '';
    _effort = _existing?.effort ?? '';
    if (widget.capabilities != null) {
      _applyCapabilities(widget.capabilities!);
    } else {
      _loadCapabilities();
    }
  }

  @override
  void dispose() {
    _idCtrl.dispose();
    _nameCtrl.dispose();
    _descCtrl.dispose();
    _instrCtrl.dispose();
    _modelCtrl.dispose();
    _wsCtrl.dispose();
    super.dispose();
  }

  /// 能力位快照（getHostInfo 的 capabilities）：agent 下拉候选的唯一来源。
  ///
  /// 【读不到不刷红】位数据只是下拉候选，读不到时按「没有位数据」降级
  /// （手填 + 提示），保存仍可用——与 Git 页的仓库候选同款纪律
  /// （快捷入口缺席不等于功能缺席）。失败原因只进调试输出，不在表单里
  /// 摆一条红。
  Future<void> _loadCapabilities() async {
    try {
      final List<String> caps = await _readCapabilities();
      if (!mounted) return;
      setState(() => _applyCapabilities(caps));
    } catch (e) {
      debugPrint('collab-role-editor: 能力位读取失败（$e），agent 下拉按无位数据降级');
    }
  }

  /// 读能力位（独立方法便于测试覆写；生产走 getHostInfo）。
  Future<List<String>> _readCapabilities() async {
    final PaBridgeForEditor bridge = _bridge;
    final List<String> caps = await bridge.capabilities();
    return caps;
  }

  /// 通道注入点（默认生产通道；测试传可控 fake）。
  PaBridgeForEditor get _bridge => _bridgeOverride ?? const _RealPaBridge();

  PaBridgeForEditor? _bridgeOverride;

  void _applyCapabilities(List<String> caps) {
    if (_agentId.isEmpty) {
      _agentId = collabEditorDefaultAgentId(caps);
    }
  }

  // ── 派生 ────────────────────────────────────────────────────────────────

  /// agent 下拉候选（能力位投影，data 层纯函数）。
  List<String> get _agents =>
      collabEditorAgentCandidates(widget.capabilities ?? const <String>[]);

  /// id 校验：仅新增态需要（编辑态 id 只读且必已合法）。null = 合法。
  String? get _idError =>
      _isNew ? validateCollabRoleId(_idCtrl.text) : null;

  /// 可保存：id 合法 + 不在飞 + agent 非空（Kotlin canSave 同款三条）。
  bool get _canSave => _idError == null && !_saving && _agentId.trim().isNotEmpty;

  // ── 保存 ────────────────────────────────────────────────────────────────

  Future<void> _save() async {
    if (!_canSave) return;
    // 编辑态用已落库的 id（只读字段，不用输入框文本——它可能是 stale 的）。
    final String effectiveId = _isNew ? _idCtrl.text.trim() : (_existing?.id ?? '');
    if (effectiveId.isEmpty) return;
    setState(() {
      _saving = true;
      _saveError = null;
    });
    final CollabRole role = collabRoleFromForm(
      id: effectiveId,
      name: _nameCtrl.text,
      agentId: _agentId,
      description: _descCtrl.text,
      instructions: _instrCtrl.text,
      model: _modelCtrl.text,
      effort: _effort,
      workspace: _wsCtrl.text,
    );
    final String? err = await _store.save(role);
    if (!mounted) return;
    setState(() => _saving = false);
    if (err != null) {
      // 后端人话原文，如实显示（不吞、不泛化成「操作失败」）；HOST_UNAVAILABLE
      // 与 load 态共用同一收口（点名 B4 波次，其余原文透传）。
      setState(() => _saveError = collabRolesErrorText(err));
      return;
    }
    // 保存成功 → 回列表页（Kotlin onBack()；store 已用落库真值刷新列表）。
    widget.onBack();
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    if (!_isNew && _existing == null) {
      return _buildNotFound(pa);
    }
    final List<String> agents = _agents;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        PaSubPageHeader(
          title: _isNew ? '新建协同角色' : '编辑协同角色',
          backKey: 'collab-role-editor-back',
          onBack: widget.onBack,
          bottomPadding: 14,
        ),
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: <Widget>[
              if (_saveError != null)
                _SaveErrorBar(
                  message: _saveError!,
                  onDismiss: () => setState(() => _saveError = null),
                ),
              _buildIdField(pa),
              _Field(
                label: '显示名',
                placeholder: '如 安全审查者(留空则回落 id)',
                controller: _nameCtrl,
                textKey: 'collab-role-name',
              ),
              _buildAgentField(pa, agents),
              _Field(
                label: '职责描述',
                placeholder: '这个岗位是干什么的(职责边界)',
                controller: _descCtrl,
                textKey: 'collab-role-description',
                multiline: true,
              ),
              _Field(
                label: '长期要求(追加到每次委派)',
                placeholder: '如:只找安全问题、不要改代码、必须列证据',
                controller: _instrCtrl,
                textKey: 'collab-role-instructions',
                multiline: true,
              ),
              _Field(
                label: '模型覆盖',
                placeholder: '留空=用该 agent 默认(形如 agent/model-id)',
                controller: _modelCtrl,
                textKey: 'collab-role-model',
              ),
              _buildEffortField(pa),
              _Field(
                label: '工作区覆盖',
                placeholder: '留空=用默认工作区(手填易错,一般无需填)',
                controller: _wsCtrl,
                textKey: 'collab-role-workspace',
              ),
              const SizedBox(height: 4),
              PAButton(
                key: const ValueKey<String>('collab-role-save'),
                label: _saving ? '保存中…' : '保存协同角色',
                onPressed: _canSave ? _save : null,
              ),
              if (_idError != null) ...<Widget>[
                const SizedBox(height: 8),
                Text(
                  _idError!,
                  key: const ValueKey<String>('collab-role-id-error'),
                  style: PATypography.caption.copyWith(color: pa.danger),
                ),
              ],
              const SizedBox(height: 8),
              ],
            ),
          ),
        ),
      ],
    );
  }

  // ── 字段装配 ────────────────────────────────────────────────────────────

  Widget _buildNotFound(PAColors pa) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        PaSubPageHeader(
          title: '编辑协同角色',
          backKey: 'collab-role-editor-back',
          onBack: widget.onBack,
          bottomPadding: 14,
        ),
        Expanded(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Text(
                '找不到该角色(可能已被删除)',
                key: const ValueKey<String>('collab-role-editor-not-found'),
                textAlign: TextAlign.center,
                style: PATypography.bodySmall.copyWith(color: pa.fgSecondary),
              ),
            ),
          ),
        ),
      ],
    );
  }

  /// id 字段：新增态可编辑（带实时校验）；编辑态只读（引用键，扁平行）。
  Widget _buildIdField(PAColors pa) {
    if (!_isNew) {
      return _ReadonlyRow(
        label: '角色 id(引用键)',
        value: _existing?.id ?? '',
        note: '只读:它是委派里的引用键,改了会让已有引用失效',
        textKey: 'collab-role-id-readonly',
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        _Field(
          label: '角色 id(引用键)',
          placeholder: '如 reviewer,只能英文/数字/横线,不能含空格',
          controller: _idCtrl,
          textKey: 'collab-role-id',
          onChanged: (_) => setState(() {}),
        ),
        if (_idError != null)
          Padding(
            padding: const EdgeInsets.only(left: 3, top: 4),
            child: Text(
              _idError!,
              key: const ValueKey<String>('collab-role-id-inline-error'),
              style: PATypography.caption.copyWith(color: pa.danger),
            ),
          ),
      ],
    );
  }

  /// 目标 agent：有候选 = 下拉选择；无候选 = 诚实降级（手填 + 说明）。
  ///
  /// 【降级为什么还能保存】agent_id 是透传真值，daemon 在委派那一刻才做
  /// 存在性裁决（collab_roles.go:106-110「今天装了明天卸载」不能变成存不进
  /// 去）。能力位缺席只是不知道有哪些家，不是不能配。
  Widget _buildAgentField(PAColors pa, List<String> agents) {
    if (agents.isEmpty) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          _Field(
            label: '目标 agent',
            placeholder: '如 codex(电脑端未报告能力位,手填 agent id)',
            controller: TextEditingController(text: _agentId),
            textKey: 'collab-role-agent-free',
            onChanged: (String v) => setState(() => _agentId = v),
          ),
          Padding(
            padding: const EdgeInsets.only(left: 3, top: 4),
            child: Text(
              '电脑端暂未报告可协同的 agent 名单;填写后保存仍然有效',
              key: const ValueKey<String>('collab-role-agent-no-caps'),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ),
        ],
      );
    }
    return _ChoiceField(
      label: '目标 agent',
      options: agents,
      selected: _agentId,
      labelOf: selectedAgentLabel,
      textKeyPrefix: 'collab-role-agent',
      onSelected: (String id) => setState(() => _agentId = id),
    );
  }

  /// 思考强度：五档分段（空 = 跟随默认；档位表见 kCollabEffortOptions）。
  Widget _buildEffortField(PAColors pa) {
    return _ChoiceField(
      label: '思考强度',
      options: kCollabEffortOptions,
      selected: _effort,
      labelOf: collabEffortLabel,
      textKeyPrefix: 'collab-role-effort',
      onSelected: (String e) => setState(() => _effort = e),
    );
  }
}

// ── 表单零件 ────────────────────────────────────────────────────────────────

/// 自绘输入框：顶部小标签 + surface 底细线输入区；multiline 时三行起。
class _Field extends StatelessWidget {
  const _Field({
    required this.label,
    required this.placeholder,
    required this.controller,
    required this.textKey,
    this.multiline = false,
    this.onChanged,
  });

  final String label;
  final String placeholder;
  final TextEditingController controller;
  final String textKey;
  final bool multiline;
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            label,
            style: PATypography.caption.copyWith(color: pa.fgSecondary),
          ),
          const SizedBox(height: 6),
          TextField(
            key: ValueKey<String>(textKey),
            controller: controller,
            onChanged: onChanged,
            minLines: multiline ? 3 : 1,
            maxLines: multiline ? 6 : 1,
            style: PATypography.bodySmall.copyWith(color: pa.fgPrimary),
            decoration: InputDecoration(
              isDense: true,
              hintText: placeholder,
              hintStyle: PATypography.caption.copyWith(color: pa.fgTertiary),
              filled: true,
              fillColor: pa.bgSurface,
              contentPadding:
                  const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
                borderSide: BorderSide(color: pa.hairline),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
                borderSide: BorderSide(color: pa.brand, width: 1.5),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// 只读行（编辑态的 id）：标签 + 值 + 一句「为什么只读」。
class _ReadonlyRow extends StatelessWidget {
  const _ReadonlyRow({
    required this.label,
    required this.value,
    required this.note,
    required this.textKey,
  });

  final String label;
  final String value;
  final String note;
  final String textKey;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            label,
            style: PATypography.caption.copyWith(color: pa.fgSecondary),
          ),
          const SizedBox(height: 6),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
            decoration: BoxDecoration(
              color: pa.bgSurface,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: pa.hairline),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  value,
                  key: ValueKey<String>(textKey),
                  style: PATypography.bodySmall.copyWith(
                    color: pa.fgPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  note,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 单选字段（agent / effort 共用）：候选 Wrap 成 chip，选中 = 主色。
///
/// 【为什么用 Wrap chip 而不是 DropdownButton】设计系统没有 dropdown 资产
/// （components/ 只读），而 chip 是既有视觉（threads 筛选条同款）；候选数
/// 量少（agent 位通常个位数，effort 恒五档），Wrap 一屏放得下，还免了
/// 「点开菜单再选」的一跳。
class _ChoiceField extends StatelessWidget {
  const _ChoiceField({
    required this.label,
    required this.options,
    required this.selected,
    required this.labelOf,
    required this.textKeyPrefix,
    required this.onSelected,
  });

  final String label;
  final List<String> options;
  final String selected;
  final String Function(String) labelOf;
  final String textKeyPrefix;
  final ValueChanged<String> onSelected;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            label,
            style: PATypography.caption.copyWith(color: pa.fgSecondary),
          ),
          const SizedBox(height: 6),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: <Widget>[
              for (final String option in options)
                ChoiceChip(
                  key: ValueKey<String>('$textKeyPrefix-$option'),
                  label: Text(labelOf(option)),
                  selected: option == selected,
                  showCheckmark: false,
                  backgroundColor: pa.bgSurface2,
                  selectedColor: pa.brand.withValues(alpha: 0.18),
                  side: BorderSide(
                    color: option == selected ? pa.brand : pa.hairline,
                  ),
                  labelStyle: PATypography.caption.copyWith(
                    color: option == selected ? pa.brand : pa.fgSecondary,
                    fontWeight: FontWeight.w600,
                  ),
                  onSelected: (_) => onSelected(option),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

/// 保存失败条：daemon 还原的原文（不泛化），可关闭。
class _SaveErrorBar extends StatelessWidget {
  const _SaveErrorBar({required this.message, required this.onDismiss});

  final String message;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Container(
        padding: const EdgeInsets.fromLTRB(14, 10, 6, 10),
        decoration: BoxDecoration(
          color: pa.danger.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: pa.danger),
        ),
        child: Row(
          children: <Widget>[
            Expanded(
              child: Text(
                message,
                key: const ValueKey<String>('collab-role-save-error'),
                style: PATypography.caption.copyWith(color: pa.danger),
              ),
            ),
            IconButton(
              key: const ValueKey<String>('collab-role-save-error-close'),
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

// ── 能力位读取的通道抽象 ────────────────────────────────────────────────────

/// 编辑器读能力位的窄接口（只一个方法）。
///
/// 【为什么不直接 import PaBridge】页面按测试惯例可注入通道；窄接口让测试
/// 用一行 fake 覆写「读能力位」这一件事，不必 mock 整个 MethodChannel 面
/// （能力位读失败只影响下拉候选，与 save 的通道行为无关，分开更好钉）。
abstract interface class PaBridgeForEditor {
  Future<List<String>> capabilities();
}

/// 生产实现：`PaBridge.getHostInfo()` 的 capabilities 字段。
class _RealPaBridge implements PaBridgeForEditor {
  const _RealPaBridge();

  @override
  Future<List<String>> capabilities() async {
    final PaBridge bridge = PaBridge();
    final HostInfo info = await bridge.getHostInfo();
    return info.capabilities;
  }
}
