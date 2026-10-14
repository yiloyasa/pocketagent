import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../components/pa_status_chip.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'onboarding_steps.dart';

/// 新手教程：横向翻页式引导（Kotlin `ui/Onboarding.kt` 迁移，编辑式浅盘 ↔ 暗色同槽位）。
///
/// 脊柱 = **连接主机 → 配对 → 发第一条指令**，每步一屏一句话一动作，口语化不堆术语。
/// 页面清单与文案全部来自纯函数 [onboardPages]（Kotlin 单测钉死的同一份契约），
/// widget 层只负责翻页、指示点与按钮接线。
///
/// 【已完成步骤不重复打扰】重看时若电脑已连接，连接/配对两步只显示「已完成」，
/// 不重复展开讲 —— 完成与否来自真实连接状态，不猜（见 [connected]）。
class OnboardingPage extends StatefulWidget {
  const OnboardingPage({
    super.key,
    this.onFinished,
    this.connected = false,
    this.onOpenSettings,
  });

  /// 「跳过」或末页「开始使用」时回调 —— 调用方负责写标志并切回主页
  /// （Kotlin `finishOnboarding`，MainActivity.kt:226-233）。
  final VoidCallback? onFinished;

  /// 连接态完成位的事实来源（Kotlin `AgentConnection.connState == "ok"`）。
  ///
  /// TODO(B3 后续波次 · B2 通道)：接入 `AgentConnection` 的 connState 流后由宿主
  /// 注入；本波低依赖部分不建通道，默认 `false`（未连接）走完整讲解文案，
  /// 与真机首启一致。
  final bool connected;

  /// 排障页「去设置填写地址」去向（Kotlin `onOpenSettings` → `settings?sub=connection`）。
  ///
  /// TODO(B3 后续波次 · 路由)：接 `settings?sub=connection` 深链路由后由路由层注入；
  /// 当前留 null，点击不动作（不假装修好了）。
  final VoidCallback? onOpenSettings;

  @override
  State<OnboardingPage> createState() => _OnboardingPageState();
}

class _OnboardingPageState extends State<OnboardingPage> {
  final PageController _controller = PageController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  int _index = 0;

  List<OnboardPage> get _pages => onboardPages(widget.connected);

  void _finish() {
    final VoidCallback? cb = widget.onFinished;
    if (cb != null) {
      cb();
      return;
    }
    Navigator.of(context).maybePop();
  }

  void _next() {
    if (_index >= _pages.length - 1) {
      _finish();
      return;
    }
    _controller.nextPage(
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  void _prev() {
    if (_index <= 0) return;
    _controller.previousPage(
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  /// 页内直达按钮接线（Kotlin `onAction` when(id)）。
  void _onAction(String id) {
    switch (id) {
      case 'settings':
        widget.onOpenSettings?.call();
      case 'home':
        // Kotlin `onGoHome` 与看完同路（首启去主页、重看回主页）；
        // 当前波次没有独立的主页跳转入口，复用完成回调即可，不新增回调面。
        _finish();
    }
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final List<OnboardPage> pages = _pages;
    final bool last = _index == pages.length - 1;

    return Scaffold(
      backgroundColor: pa.bgBase,
      body: SafeArea(
        child: Column(
          children: <Widget>[
            // 顶部:跳过(任意页可提前退出)
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: <Widget>[
                  TextButton(
                    onPressed: _finish,
                    child: Text(
                      '跳过',
                      style: PATypography.body.copyWith(color: pa.fgSecondary),
                    ),
                  ),
                ],
              ),
            ),

            // 中部:翻页内容(数据驱动渲染)
            Expanded(
              child: PageView.builder(
                controller: _controller,
                itemCount: pages.length,
                onPageChanged: (int i) => setState(() => _index = i),
                itemBuilder: (BuildContext context, int i) =>
                    _OnboardPageView(page: pages[i], onAction: _onAction),
              ),
            ),

            // 底部:上一步 / 指示点 / 下一步(或 开始使用)
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 16),
              child: Row(
                children: <Widget>[
                  _PrevButton(visible: _index > 0, onPressed: _prev),
                  Expanded(
                    child: Center(child: _Dots(count: pages.length, index: _index)),
                  ),
                  SizedBox(
                    width: 120,
                    child: PAButton(
                      label: last ? '开始使用' : '下一步',
                      onPressed: _next,
                      expanded: false,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 「上一步」：非首页淡入，回首页时淡出（Kotlin AnimatedVisibility + slideIn）。
///
/// 固定宽槽位让指示点始终居中，不会因按钮进出而左右横跳；淡出结束后从树上摘除
/// （与 Kotlin AnimatedVisibility 收尾一致），首页上「上一步」不可见也不可点。
class _PrevButton extends StatelessWidget {
  const _PrevButton({required this.visible, required this.onPressed});

  final bool visible;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return SizedBox(
      width: 76,
      child: AnimatedSwitcher(
        duration: const Duration(milliseconds: 200),
        transitionBuilder: (Widget child, Animation<double> animation) {
          return SlideTransition(
            position: animation.drive(
              Tween<Offset>(
                begin: const Offset(-0.35, 0),
                end: Offset.zero,
              ),
            ),
            child: FadeTransition(opacity: animation, child: child),
          );
        },
        child: visible
            ? TextButton(
                onPressed: onPressed,
                child: Text(
                  '上一步',
                  style: PATypography.body.copyWith(color: pa.fgSecondary),
                ),
              )
            : const SizedBox.shrink(),
      ),
    );
  }
}

/// 圆点指示器：当前点放大 + 主色，尺寸/颜色都过渡（法宝 Kotlin 的硬切）。
class _Dots extends StatelessWidget {
  const _Dots({required this.count, required this.index});

  final int count;
  final int index;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List<Widget>.generate(count, (int i) {
        final bool active = i == index;
        return AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          curve: Curves.decelerate,
          margin: const EdgeInsets.symmetric(horizontal: 4),
          width: active ? 9 : 7,
          height: active ? 9 : 7,
          decoration: BoxDecoration(
            color: active ? pa.brand : pa.hairline,
            shape: BoxShape.circle,
          ),
        );
      }),
    );
  }
}

/// 单页渲染：顶部图标 + 标题(+已完成标注)+ 内容块(+可选直达按钮)。
class _OnboardPageView extends StatelessWidget {
  const _OnboardPageView({required this.page, required this.onAction});

  final OnboardPage page;
  final ValueChanged<String> onAction;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          const SizedBox(height: 8),
          // 页首图标：已完成换绿色对勾，进出两态都读得出「这步过没过」(词+色双通道)
          Center(
            child: Icon(
              page.icon,
              size: 52,
              color: page.done ? pa.ok : pa.brand,
            ),
          ),
          const SizedBox(height: 20),
          Text(
            page.title,
            textAlign: TextAlign.center,
            style: PATypography.titleL.copyWith(color: pa.fgPrimary),
          ),
          if (page.done) ...<Widget>[
            const SizedBox(height: 8),
            Center(
              child: PAStatusChip(
                status: PAStatusKind.done,
                label: '已完成,不用再操作',
              ),
            ),
          ],
          const SizedBox(height: 20),
          for (final OnboardBlock block in page.blocks) ...<Widget>[
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: switch (block) {
                OnboardLead() => _lead(context, block.text),
                OnboardBullet() => _bullet(context, block),
                OnboardNote() => _note(context, block.text),
              },
            ),
          ],
          // 页内直达按钮(排障→设置、其余→主页)：主色块 + 48dp 触区，与底部 CTA 同语言。
          if (page.actionLabel != null && page.actionId != null) ...<Widget>[
            const SizedBox(height: 4),
            PAButton(
              label: page.actionLabel!,
              onPressed: () => onAction(page.actionId!),
              icon: page.actionId == 'settings'
                  ? Icons.tune_rounded
                  : Icons.arrow_forward_rounded,
            ),
          ],
          const SizedBox(height: 24),
        ],
      ),
    );
  }

  /// 一段引导说明文字（Kotlin `OnboardBlock.Lead`）。
  Widget _lead(BuildContext context, String text) {
    final PAColors pa = context.pa;
    return Text(
      text,
      textAlign: TextAlign.center,
      style: PATypography.body.copyWith(color: pa.fgSecondary, height: 1.6),
    );
  }

  /// 带图标的小卡片块（Kotlin `OnboardBullet`：瓷白面 + 1dp 细线 + 18dp 圆角
  /// → Flutter 侧统一走设计系统 [PACard]）。
  Widget _bullet(BuildContext context, OnboardBullet b) {
    final PAColors pa = context.pa;
    return PACard(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      child: Row(
        children: <Widget>[
          if (b.icon != null) ...<Widget>[
            Icon(b.icon, size: 22, color: pa.brand),
            const SizedBox(width: 12),
          ],
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  b.title,
                  style: PATypography.body.copyWith(
                    color: pa.fgPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (b.desc != null) ...<Widget>[
                  const SizedBox(height: 3),
                  Text(
                    b.desc!,
                    style: PATypography.caption.copyWith(
                      color: pa.fgSecondary,
                      height: 1.5,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// Info 提示卡块（Kotlin `OnboardNote`）。
  Widget _note(BuildContext context, String text) {
    final PAColors pa = context.pa;
    return PACard(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      color: pa.bgSurface2,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Icon(Icons.info_outline_rounded, size: 16, color: pa.brand),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              text,
              style: PATypography.caption.copyWith(
                color: pa.fgSecondary,
                height: 1.5,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
