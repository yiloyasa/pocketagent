// Settings / Threads 页族的**共享内部件**（B3-1/B3-2/F5/B3-3 迁移波次收敛产物）。
//
// 【为什么落在 pages/ 而不是 components/】收敛任务书只授权改 pages/ 域（theme/
// components 已由 H2 单独收敛，见 docs/reviews/2026-10-14/convergence-flutter-theme.md）。
// 本文件的每个件都是「迁移波次几个新文件各写一份的同款局部样式」的去重，只被
// pages/ 消费，不是设计系统资产。公共契约（pa_bridge.dart / channel-contract.md）
// 零改动；本文件也没有新增大 API —— 都是 pages 私域件的共享化。
//
// 【等价优先】每个件的「出处」注释点名它从哪个文件的哪个私有类搬运而来；各处
// 源码在收敛前已逐行 diff。合并后渲染结果与合并前逐像素等价（结构、常量、key、
// padding 全同），唯一例外是 [PaCardCaption] 替掉 settings_capabilities 的
// `_padded` 时少一层 Align —— Align 对 Text 子节点是布局恒等（loose 约束下
// Text 取自身宽度、左对齐，与 centerLeft 同位），详见该件注释。

import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';

// ── 分组块族 ────────────────────────────────────────────────────────────────

/// 分组块：组标题 + [PACard]（padding 0）+ 块间左缩进分隔线。
///
/// 出处：settings.dart `_Group` 与 settings_agent.dart `_Section` —— 两份实现
/// **逐字节相同**（收敛前已 diff 确认：仅类名与文档注释不同）。合并无行为变化。
/// 视觉契约（bottom 22、标题 left 4 bottom 8、letterSpacing 0.4、58 缩进分隔线）
/// 与 Android SettingsScreen 分组同款，一个常量都不动。
class PaPageSection extends StatelessWidget {
  const PaPageSection({super.key, required this.title, required this.children});

  final String title;

  /// 组内子块（行 / 自定义块，如 `ConnectionSection`），块间以左缩进分隔线隔开。
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 22),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Padding(
            padding: const EdgeInsets.only(left: 4, bottom: 8),
            child: Text(
              title,
              style: PATypography.caption.copyWith(
                color: pa.fgTertiary,
                fontWeight: FontWeight.w600,
                letterSpacing: 0.4,
              ),
            ),
          ),
          PACard(
            padding: EdgeInsets.zero,
            child: Column(
              children: <Widget>[
                for (int i = 0; i < children.length; i++) ...<Widget>[
                  if (i > 0) const PaInsetDivider(),
                  children[i],
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 与 [PaPageSection] 同款的左缩进分隔线（58 = 图标列宽 + 间距）。
///
/// 出处：settings_connection.dart `_InsetDivider`；settings.dart `_Group` 与
/// settings_agent.dart `_Section` 原先各内联一份同款字面量。收责任一。
class PaInsetDivider extends StatelessWidget {
  const PaInsetDivider({super.key});

  @override
  Widget build(BuildContext context) => const Padding(
        padding: EdgeInsets.only(left: 58),
        child: Divider(),
      );
}

/// 与 settings.dart `_SettingsRow` 同款的 32×32 圆角图标块。
///
/// 出处：settings_connection.dart `_IconBox` 与 settings_agent.dart `_IconBox`
/// —— 两份仅文档注释不同，实现逐字节相同（收敛前 diff 确认）。
class PaIconBox extends StatelessWidget {
  const PaIconBox({super.key, required this.icon});

  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Container(
      width: 32,
      height: 32,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: pa.bgSurface2,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Icon(icon, size: 17, color: pa.fgSecondary),
    );
  }
}

// ── 二级页头部 ──────────────────────────────────────────────────────────────

/// 二级页头部：返回箭头 + 标题（Android `SubPageScaffold` 的最小对应物）。
///
/// 出处：settings_agent.dart `_SubHeader`（bottom 14）与 settings_capabilities.dart
/// `_SubHeader`（bottom 10 + 可选刷新键）—— 两份差异**只是** bottom 距离与刷新键
/// 有无，这里参数化，调用点原样传参，渲染逐像素等价。settings.dart 的
/// `_PendingSubPage` 头部是第三份同款 Row（诚实占位页），收敛时也走本件。
class PaSubPageHeader extends StatelessWidget {
  const PaSubPageHeader({
    super.key,
    required this.title,
    required this.backKey,
    required this.onBack,
    this.onRefresh,
    this.bottomPadding = 14,
  });

  final String title;

  /// 返回键的 [ValueKey] 字符串（各页测试锚点：pa-agent-back / pa-models-back /
  /// pa-pending-back 等，原样透传，不猜不默认）。
  final String backKey;
  final VoidCallback onBack;

  /// 非空 = 头部右侧渲染刷新键（仅模型页有；键名 `pa-models-refresh` 与收敛前一致）。
  final VoidCallback? onRefresh;

  /// 头部与下方内容的间距：agent 页 14、模型页 10（各自原值，见出处注释）。
  final double bottomPadding;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: EdgeInsets.only(bottom: bottomPadding),
      child: Row(
        children: <Widget>[
          IconButton(
            key: ValueKey<String>(backKey),
            onPressed: onBack,
            tooltip: '返回',
            icon: Icon(Icons.arrow_back_rounded, color: pa.fgSecondary),
          ),
          const SizedBox(width: 2),
          Expanded(
            child: Text(
              title,
              style: PATypography.titleM.copyWith(color: pa.fgPrimary),
            ),
          ),
          if (onRefresh != null)
            IconButton(
              key: const ValueKey<String>('pa-models-refresh'),
              onPressed: onRefresh,
              tooltip: '刷新',
              icon: Icon(Icons.refresh_rounded, color: pa.fgSecondary),
            ),
        ],
      ),
    );
  }
}

// ── 三态页族的公共词汇 ──────────────────────────────────────────────────────

/// 页面三态：加载中 / 读失败 / 已就绪（就绪后再分「空列表」与「有内容」，
/// 那是页面自己的事，状态机只管前三态）。
///
/// 出处：settings_capabilities.dart `_ModelsPhase` 与 threads.dart `_ThreadsPhase`
/// —— 两个私有枚举同名同义同序。收敛为共享词汇：枚举本身零渲染，换名字不碰
/// 任何像素；两页的 `switch` 仍是穷尽匹配（analyze 守着）。
enum PaAsyncPhase { loading, error, ready }

// ── 卡片内文案行 / 错误+重试 ────────────────────────────────────────────────

/// 卡片内一条 caption 文案行（键 + 颜色 + 内边距都可配）。
///
/// 出处（三处私有件合一）：
///  - settings_agent.dart `_caption`：padding `fromLTRB(14, 6, 14, 10)`，fgTertiary；
///  - settings_capabilities.dart `_padded`：padding `symmetric(14, 12)`，fgTertiary，
///    外包一层 `Align(centerLeft)`；
///  - settings_connection.dart `_caption(danger: true)`：padding
///    `fromLTRB(58, 0, 14, 8)`（58 = 与行内容左对齐的缩进），danger 色。
///
/// 【为什么可以去掉 caps 的 Align】`Padding` 给子节点的是 loose 约束，
/// `Align(centerLeft)` 传给 `Text` 的仍是同一组 loose 约束：Text 取自身宽度、
/// 左缘落在 padding 内缘 —— 与没有 Align 时逐像素同位置（Align 的
/// widthFactor 为 null 时自身撑满，只改自己的盒子，不改子节点布局结果）。
/// 这是本文件唯一的「结构性删层」，渲染等价，无测试钉在 Align 上（钉的是文案
/// 与 key：pa-models-loading / pa-models-empty）。
class PaCardCaption extends StatelessWidget {
  const PaCardCaption({
    super.key,
    required this.text,
    required this.textKey,
    this.padding = const EdgeInsets.fromLTRB(14, 6, 14, 10),
    this.danger = false,
  });

  final String text;

  /// 文案的 [ValueKey] 字符串（各页测试锚点原样透传）。
  final String textKey;

  /// 内边距：agent 页默认 14/6/14/10；模型页 symmetric(14,12)；connection 页
  /// fromLTRB(58, 0, 14, 8)。三处原值各自传入，不统一成新数值。
  final EdgeInsetsGeometry padding;

  /// danger = 错误文案红（connection 页的两处错误行）；其余为 fgTertiary。
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: padding,
      child: Text(
        text,
        key: ValueKey<String>(textKey),
        style: PATypography.caption
            .copyWith(color: danger ? pa.danger : pa.fgTertiary),
      ),
    );
  }
}

/// 错误态块：danger 文案 + 左对齐「重试」键（settings 三页的错误态同款装配）。
///
/// 出处：settings_agent.dart `_buildCapsBody` 的错误分支与
/// settings_capabilities.dart `_buildBody` 的错误分支 —— 两份**逐字节相同**
/// （Padding h14/v12 + Column(start)[Text(danger, key), Align(TextButton(key,
/// '重试', caption brand w600))]），只有 key 与文案来源不同。settings_connection.dart
/// 状态条里的重试键是同一枚 TextButton（外层 Padding 由状态条自己给）。
class PaErrorRetry extends StatelessWidget {
  const PaErrorRetry({
    super.key,
    required this.errorText,
    required this.errorKey,
    required this.onRetry,
    required this.retryKey,
  });

  final String errorText;
  final String errorKey;
  final VoidCallback onRetry;
  final String retryKey;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            errorText,
            key: ValueKey<String>(errorKey),
            style: PATypography.caption.copyWith(color: pa.danger),
          ),
          Align(
            alignment: Alignment.centerLeft,
            child: PaRetryButton(onPressed: onRetry, retryKey: retryKey),
          ),
        ],
      ),
    );
  }
}

/// 「重试」TextButton：caption + 主色 + w600，左对齐槽位。
///
/// 出处：settings_connection.dart 状态条（键 pa-conn-retry）、settings_agent.dart
/// （pa-agent-retry）、settings_capabilities.dart（pa-models-retry）三处同款；
/// threads.dart 错误态用的是 [PAButton]「重试」（另一套视觉，不并）。
class PaRetryButton extends StatelessWidget {
  const PaRetryButton({super.key, required this.onPressed, required this.retryKey});

  final VoidCallback onPressed;
  final String retryKey;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Align(
      alignment: Alignment.centerLeft,
      child: TextButton(
        key: ValueKey<String>(retryKey),
        onPressed: onPressed,
        child: Text(
          '重试',
          style: PATypography.caption.copyWith(
            color: pa.brand,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}
