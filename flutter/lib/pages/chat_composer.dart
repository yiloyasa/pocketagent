// Chat 输入区 UI（B3 第 5 波③）：文本框 + 发送 + 附件入口 + 斜杠面板 +
// 模型行 + 语音诚实占位。
//
// 【对位】Android `ui/chat/ChatComposer.kt`（`ChatComposerArea`：附件 chips /
// 斜杠卡面 / 输入框 / 模型行 / 工具行）——本文件是它的 Flutter 最小对应物：
// 只回答「输入区五件摆出来、门控对、回调形状 frozen」；动画（涟漪/波形/辉光）、
// 沉浸胶囊、审批策略 chip、上下文环、Git 面板、优化按钮等 Android 侧能力留
// 后续波次，不摆假件。
//
// 【与 TurnSession 接线（C4 组装 ChatPage 时接）】本文件**不 import
// chat_session.dart**：构造注入回调，C4 组装：
//   - `onSubmit(String)`：C4 调 `TurnSession.startTurn(TurnInput)`（空闲）或
//     `steer(text)`（运行中追问）；composer 保证交出的是非空原文（见下）。
//   - `onSlashPick(SlashPick)`：FILL 已由 composer 回填输入框；GOAL/COMPACT/
//     REVIEW 由 C4 执行对应动作（目标弹窗 / CompactThread / StartReview）。
//   - 附件是**受控**的：C4 持有 `attachments` 列表，转 `TurnInput` blocks
//     （文本说明 + `ImageBlock` 内联，与 Android `TurnSession.buildTurnInput`
//     同形）随首帧发出；composer 只展示 chips + 拾取/移除回调出口。
//
// 【不造假三条】
//   1. 图片入口认 `image_input:<agentId>` 位（`composerImageEntryVisible`）：
//      无位不渲染图片按钮（文本附件按钮不受影响）；agent 未知（空）同样无位。
//   2. 语音无端侧依赖（pubspec 冻结，无语音引擎）：不摆假录音键，只渲染
//      「语音输入随后续波次」占位行（与 Browser 页 WebView 未引入前只做目标
//      确认同款纪律）。
//   3. 模型行只展示 + 切换回调出口：`onModelTap` 为空就不给点击态（无 chevron），
//      具体选择器留 C4/Settings②复用，不摆假下拉。

import 'dart:async';

import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'chat_composer_data.dart';
import 'chat_input_transport.dart';

/// Chat 输入区（受控附件 + 回调接线，见文件头 C4 契约）。
///
/// 【frozen 回调签名（C4 读）】`onSubmit: ValueChanged<String>`——交出**原文**
/// （首尾空白保留，代码粘贴的缩进不丢；非空由 [validateComposerInput] 保证，
/// 空输入下发送键禁用，调不到）；提交后输入框本地清空（与 Android `onSend
/// 成功清输入框` 同款；附件列表是受控的，C4 在 `onSubmit` 里同步清空自己持
/// 有的列表）。
class ChatComposer extends StatefulWidget {
  const ChatComposer({
    super.key,
    required this.threadId,
    required this.agentId,
    this.capabilities = const <String>[],
    this.modelName = '',
    this.effortLabel = '默认',
    this.attachments = const <ChatPendingAttachment>[],
    this.slashTransport,
    this.initialText = '',
    this.enabled = true,
    required this.onSubmit,
    this.onSlashPick,
    this.onModelTap,
    this.onPickTextFile,
    this.onPickImage,
    this.onRemoveAttachment,
  });

  /// 会话 id（斜杠命令拉取的 thread_id 口径；切换会话切缓存键）。
  final String threadId;

  /// 会话归属 agent（可空：GetThread 还在路上时空串，门控按「无位」处理）。
  final String? agentId;

  /// 能力位快照（`slash:`/`compact:`/`review:`/`image_input:` 四位消费，
  /// 由构造注入——C4 从连接态 `hostCapabilities` 透传）。
  final List<String> capabilities;

  /// 当前生效模型名（空 = 跟随桌面，如实展示，不编假名）。
  final String modelName;

  /// 思考强度中文标签（如「默认」/「高」，透传展示）。
  final String effortLabel;

  /// 待发送附件（受控：C4 持有，composer 只展示 + 移除出口）。
  final List<ChatPendingAttachment> attachments;

  /// 斜杠命令 transport（null = 生产默认通道；测试注入独立通道隔离；
  /// 不拉 daemon 命令时传 null 也行——面板退回内置伪命令兜底）。
  final ChatInputTransport? slashTransport;

  /// 输入框初值（草稿恢复用，默认空）。
  final String initialText;

  /// 输入可用（离线快照态由 C4 置 false：发送禁用 + 面板不弹 + hint 换文案）。
  final bool enabled;

  /// 提交出口（frozen：`ValueChanged<String>`，见类文档）。
  final ValueChanged<String> onSubmit;

  /// 斜杠点选出口（FILL 已回填，内置伪命令动作留 C4；可空 = 不关心点选）。
  final ValueChanged<SlashPick>? onSlashPick;

  /// 模型行点击出口（可空 = 无点击态，不摆假选择器）。
  final VoidCallback? onModelTap;

  /// 文本附件拾取出口（可空 = 不渲染文本附件入口）。
  final VoidCallback? onPickTextFile;

  /// 图片拾取出口（可空 = 不渲染图片入口；另受能力位门控，见文件头）。
  final VoidCallback? onPickImage;

  /// 附件移除出口（参数为下标；可空 = chips 不带移除键）。
  final ValueChanged<int>? onRemoveAttachment;

  @override
  State<ChatComposer> createState() => _ChatComposerState();
}

class _ChatComposerState extends State<ChatComposer> {
  late final TextEditingController _controller =
      TextEditingController(text: widget.initialText);
  late final ChatInputTransport _transport =
      widget.slashTransport ?? ChatInputTransport();

  /// 按会话缓存的 daemon 命令展平视图（Android `SlashCommandCache` 的本会话
  /// 形态：进会话/线程切换才重拉；同会话内一次拉取后不再逐字符请求——
  /// [_slashAsked] 是「问过即止」闸，空表也记，避免空结果时每敲一字刷一次
  /// RPC；错误重试经面板错误行显式触发）。
  final Map<String, List<SlashCommandInfo>> _slashCache =
      <String, List<SlashCommandInfo>>{};
  final Set<String> _slashAsked = <String>{};
  bool _slashLoading = false;
  String? _slashError;

  @override
  void initState() {
    super.initState();
    _controller.addListener(_onTextChanged);
    _maybeFetchSlash();
  }

  @override
  void didUpdateWidget(ChatComposer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.threadId != widget.threadId) {
      _slashError = null;
      _maybeFetchSlash();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _onTextChanged() {
    setState(() {});
    _maybeFetchSlash();
  }

  bool get _offline => !widget.enabled;

  /// `/` 触发拉候选（守卫五连：无词元 / 非面板会话 / 离线 / 问过 / 在飞都停）。
  void _maybeFetchSlash() {
    if (slashTokenOf(_controller.text) == null) return;
    if (!slashPanelAgentOf(widget.agentId, widget.capabilities)) return;
    if (_offline) return;
    if (_slashAsked.contains(widget.threadId) || _slashLoading) return;
    if (_slashCache.containsKey(widget.threadId)) return;
    unawaited(_fetchSlash());
  }

  Future<void> _fetchSlash() async {
    setState(() {
      _slashLoading = true;
      _slashError = null;
    });
    try {
      final List<SlashCommandInfo> infos = (await _transport.listSlashCommands(
        widget.threadId,
      ))
          .commands
          .map(slashInfoFromProto)
          .toList();
      if (!mounted) return;
      setState(() {
        _slashLoading = false;
        _slashAsked.add(widget.threadId);
        // 空表不入缓存（与 Android `SlashCommandCache.put` 同款：新会话在电脑
        // 落地后，下次进会话还能拉到命令）；问过闸照记，本次会话内不再刷。
        if (infos.isNotEmpty) _slashCache[widget.threadId] = infos;
      });
    } on Exception catch (e) {
      if (!mounted) return;
      setState(() {
        _slashLoading = false;
        _slashAsked.add(widget.threadId);
        _slashError = e.toString();
      });
    }
  }

  void _retrySlash() {
    _slashAsked.remove(widget.threadId);
    unawaited(_fetchSlash());
  }

  List<SlashCommandInfo> get _cachedSlash =>
      _slashCache[widget.threadId] ?? const <SlashCommandInfo>[];

  List<SlashCommandInfo> get _visibleCandidates => filterSlashCandidates(
        slashCandidatesFor(
          widget.agentId,
          _cachedSlash,
          widget.capabilities,
        ),
        _controller.text,
      );

  bool get _panelOpen => slashPanelVisible(
        widget.agentId,
        _controller.text,
        _cachedSlash,
        _offline,
        widget.capabilities,
      );

  void _submit() {
    if (validateComposerInput(_controller.text) != null || _offline) return;
    widget.onSubmit(_controller.text);
    _controller.clear();
  }

  void _pickSlash(SlashCommandInfo cmd) {
    final SlashPick pick = SlashPick(
      name: cmd.name,
      action: cmd.action,
      agentId: widget.agentId,
    );
    if (cmd.action == SlashPickAction.fill) {
      setState(() {
        _controller.text = applySlashPick(
          _controller.text,
          cmd,
          widget.agentId,
        );
        _controller.selection = TextSelection.fromPosition(
          TextPosition(offset: _controller.text.length),
        );
      });
    } else {
      // 内置伪命令：执行动作留 C4；敲出的 "/词元" 是唤起面板的临时手段，
      // 本地先摘掉（与 Android 点选后 `stripSlashToken` 同款），避免残留词元
      // 被当成正文发出；词元后已敲文字保留给 C4 作动作输入（如 /计划 的 focus）。
      setState(() {
        _controller.text = stripSlashToken(_controller.text);
        _controller.selection = TextSelection.fromPosition(
          TextPosition(offset: _controller.text.length),
        );
      });
    }
    widget.onSlashPick?.call(pick);
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String? inputError = validateComposerInput(_controller.text);
    final bool canSend = !_offline && inputError == null;
    final bool showImageEntry =
        widget.onPickImage != null &&
        composerImageEntryVisible(widget.agentId, widget.capabilities);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        if (widget.attachments.isNotEmpty) _buildAttachRow(pa),
        if (_panelOpen) _buildSlashPanel(pa),
        if (!_panelOpen &&
            slashUnknownAgentHintVisible(
              widget.agentId,
              _controller.text,
              _offline,
              widget.capabilities,
            ))
          _buildUnknownHint(pa),
        TextField(
          key: const ValueKey<String>('composer-input'),
          controller: _controller,
          enabled: widget.enabled,
          minLines: 1,
          maxLines: 5,
          textInputAction: TextInputAction.newline,
          style: PATypography.body.copyWith(color: pa.fgPrimary),
          decoration: InputDecoration(
            hintText: _offline ? '离线中，可继续编辑草稿；重连后发送…' : '继续输入，或询问当前进度…',
            hintStyle: PATypography.body.copyWith(color: pa.fgTertiary),
            errorText: inputError == '输入不能为空' ? null : inputError,
          ),
        ),
        const SizedBox(height: 4),
        _buildModelRow(pa),
        const SizedBox(height: 4),
        Row(
          children: <Widget>[
            if (widget.onPickTextFile != null)
              IconButton(
                key: const ValueKey<String>('composer-attach-file'),
                onPressed: widget.enabled ? widget.onPickTextFile : null,
                tooltip: '添加文本附件',
                icon: Icon(Icons.attach_file_rounded, color: pa.fgSecondary),
              ),
            // 图片入口：能力位门控（无位不渲染，见文件头不造假①）。
            if (showImageEntry)
              IconButton(
                key: const ValueKey<String>('composer-attach-image'),
                onPressed: widget.enabled ? widget.onPickImage : null,
                tooltip: '添加图片',
                icon: Icon(Icons.image_outlined, color: pa.fgSecondary),
              ),
            Expanded(
              child: Row(
                children: <Widget>[
                  Icon(
                    Icons.mic_none_rounded,
                    size: 16,
                    color: pa.fgTertiary,
                  ),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Text(
                      '语音输入随后续波次',
                      key: const ValueKey<String>('composer-voice-note'),
                      style: PATypography.caption
                          .copyWith(color: pa.fgTertiary),
                    ),
                  ),
                ],
              ),
            ),
            IconButton(
              key: const ValueKey<String>('composer-send'),
              onPressed: canSend ? _submit : null,
              tooltip: '发送',
              icon: Icon(
                Icons.arrow_upward_rounded,
                color: canSend ? pa.brand : pa.fgTertiary,
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildAttachRow(PAColors pa) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Wrap(
        spacing: 8,
        runSpacing: 4,
        children: <Widget>[
          for (int i = 0; i < widget.attachments.length; i++)
            _buildAttachChip(pa, i, widget.attachments[i]),
        ],
      ),
    );
  }

  Widget _buildAttachChip(
    PAColors pa,
    int index,
    ChatPendingAttachment attachment,
  ) {
    final IconData icon;
    switch (attachment.kind) {
      case ComposerAttachmentKind.image:
        icon = Icons.image_outlined;
      case ComposerAttachmentKind.text:
        icon = Icons.description_outlined;
      case ComposerAttachmentKind.unsupported:
        icon = Icons.block_outlined;
    }
    return Chip(
      key: ValueKey<String>('composer-attach-chip-$index'),
      avatar: Icon(icon, size: 16, color: pa.fgSecondary),
      label: Text(
        attachment.name,
        style: PATypography.caption.copyWith(color: pa.fgPrimary),
      ),
      // 移除键落在可点的 deleteIcon 上（Chip 的 onDeleted 渲染的删除钮无 key
      // 透传位，key 钉在图标上，tap 中心即落钮内——与面板 item 同款可测性）。
      deleteIcon: Icon(
        Icons.close_rounded,
        key: ValueKey<String>('composer-attach-remove-$index'),
        size: 16,
      ),
      onDeleted: widget.onRemoveAttachment == null
          ? null
          : () => widget.onRemoveAttachment!(index),
      deleteButtonTooltipMessage: '移除附件',
    );
  }

  Widget _buildSlashPanel(PAColors pa) {
    final List<SlashCommandInfo> candidates = _visibleCandidates;
    // Card 即 Material 祖先：ListTile 的背景/涟漪画在最近的 Material 上，
    // 直接套 Container + BoxDecoration 会在 debug 断言里报遮挡（ListTile
    // background may be invisible），故面板容器用 Card。
    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: 240),
      child: Card(
        key: const ValueKey<String>('composer-slash-panel'),
        margin: const EdgeInsets.only(bottom: 4),
        color: pa.bgSurface2,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(color: pa.fgTertiary.withValues(alpha: 0.3)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
              child: Text(
                '${candidates.length} 个命令 · 上下滑动查看',
                style:
                    PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
            ),
            Flexible(
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: candidates.length,
                itemBuilder: (BuildContext context, int i) {
                  final SlashCommandInfo cmd = candidates[i];
                  return ListTile(
                    key: ValueKey<String>('composer-slash-${cmd.name}'),
                    dense: true,
                    title: Text(
                      '/${cmd.name}',
                      style: PATypography.body.copyWith(
                        color: pa.brand,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    subtitle: cmd.description.isEmpty
                        ? null
                        : Text(
                            cmd.description,
                            style: PATypography.caption
                                .copyWith(color: pa.fgSecondary),
                          ),
                    onTap: () => _pickSlash(cmd),
                  );
                },
              ),
            ),
            if (_slashLoading)
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
                child: Text(
                  '命令加载中…',
                  key: const ValueKey<String>('composer-slash-loading'),
                  style:
                      PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ),
            if (_slashError != null && !_slashLoading)
              ListTile(
                key: const ValueKey<String>('composer-slash-error'),
                dense: true,
                title: Text(
                  '命令拉取失败，点击重试',
                  style:
                      PATypography.caption.copyWith(color: pa.danger),
                ),
                onTap: _retrySlash,
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildUnknownHint(PAColors pa) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4, left: 4),
      child: Text(
        kSlashUnknownAgentHint,
        key: const ValueKey<String>('composer-slash-unknown'),
        style: PATypography.caption.copyWith(color: pa.fgTertiary),
      ),
    );
  }

  Widget _buildModelRow(PAColors pa) {
    final String label = widget.modelName.isEmpty
        ? '模型：跟随桌面'
        : widget.effortLabel == '默认' || widget.effortLabel.isEmpty
            ? widget.modelName
            : '${widget.modelName} · ${widget.effortLabel}';
    final Widget row = Padding(
      padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Text(
              label,
              key: const ValueKey<String>('composer-model-row'),
              style:
                  PATypography.caption.copyWith(color: pa.fgSecondary),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          // 无回调不给 chevron（不造假③：不摆点不开的假选择器）。
          if (widget.onModelTap != null)
            Icon(Icons.expand_more_rounded,
                size: 18, color: pa.fgSecondary),
        ],
      ),
    );
    if (widget.onModelTap == null) return row;
    return Tooltip(
      message: '切换模型与思考强度',
      child: InkWell(
        key: const ValueKey<String>('composer-model-tap'),
        onTap: widget.onModelTap,
        child: row,
      ),
    );
  }
}
