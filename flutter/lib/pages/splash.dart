import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';

/// 启动动效覆盖层（Kotlin `ui/Splash.kt` 逐条迁移 + `MainActivity.kt:112-127` 的
/// **覆盖层**时机语义）。
///
/// 【覆盖层不是串行闸】Kotlin 侧 `RootNav` 在 Splash 之下**并行组合**（RPC 预取与
/// 启动动画同跑），`onDone` 只负责摘掉覆盖层——冷启动不再白等动画播完。Dart 侧由
/// `main.dart` 的顶层 `Stack` 复刻：Splash 是 `Positioned.fill` 的最上层，
/// 主轴内容（Onboarding / HomeShell）照常组合。
///
/// 【吃点击防误触】外层 `AbsorbPointer(absorbing: true)` 吞掉指针，动画期间用户点不到
/// 底下尚未就绪的主页，与 Kotlin `Modifier.clickable(indication = null)` 同语义。
///
/// 【动画时长出处】全部取自 `Splash.kt` 字面量与 `PaMotion` spring 参数：
/// spring 无「时长」，settle ≈ 4 /(ζ·√stiffness)，据此对位成等效 tween 时长
/// （见 [_Motion] 各成员注释）。`FastOutSlowInEasing` 与 Flutter
/// `Curves.fastOutSlowIn` 是同一条三次贝塞尔（0.4, 0, 0.2, 1），直接对位。
class SplashOverlay extends StatefulWidget {
  const SplashOverlay({super.key, required this.onDone});

  /// 「动画播完 + 淡出结束」的同一个时点（Kotlin `Splash(onDone = …)`）。
  ///
  /// 调用方只负责摘层：不再承担「何时开始组合主页」的职责。
  final VoidCallback onDone;

  @override
  State<SplashOverlay> createState() => _SplashOverlayState();
}

/// Kotlin `Splash.kt` 的节拍与尺寸常量（出处：源码行号见各注释）。
abstract final class _Motion {
  /// 字标淡入时长。Kotlin: `markAlpha.animateTo(1f, PaMotion.standard())`
  /// （PaMotion.kt:62，dampingRatio=1.0 / stiffness=420 → settle≈4/√420≈195ms）。
  static const Duration markFade = Duration(milliseconds: 200);

  /// 字标缩放时长。Kotlin: `markScale.animateTo(1f, PaMotion.emphasized())`
  /// （PaMotion.kt:67，ζ=0.9 / stiffness=380 → settle≈4/(0.9·√380)≈228ms；
  /// ζ=0.9 的理论过冲仅 0.15%，故 Flutter 侧用同族 ease-out 而非 easeOutBack 的
  /// 10% 过冲，避免「廉价弹一下」）。
  static const Duration markScale = Duration(milliseconds: 230);

  /// 字标起始缩放。Kotlin: `Animatable(0.92f)`（Splash.kt:74）。
  static const double markScaleFrom = 0.92;

  /// 打字前的立拍。Kotlin: `delay(140)`（Splash.kt:112）。
  static const Duration typeStartDelay = Duration(milliseconds: 140);

  /// 逐字间隔；空格更长。Kotlin: `delay(if (FULL[i-1] == ' ') 120 else 58)`
  /// （Splash.kt:119）。打字是「内容节拍器」不是状态过渡，故保持恒定 tween。
  static const Duration charDelay = Duration(milliseconds: 58);
  static const Duration spaceDelay = Duration(milliseconds: 120);

  /// 「agent」打出瞬间的光晕扩散。Kotlin: `tween(1100, easing = FastOutSlowInEasing)`
  /// （Splash.kt:121）。
  static const Duration glowBurst = Duration(milliseconds: 1100);

  /// 「agent」打完后的补光。Kotlin: `tween(500, …)`（Splash.kt:126）。
  static const Duration glowSettle = Duration(milliseconds: 500);

  /// 进度线随打字展开。Kotlin: `tween(FULL.length * 58 + 200, …)`（Splash.kt:122）
  /// ——12 字 ×58ms + 200ms 余量。
  static const Duration sweepWithType =
      Duration(milliseconds: _fullLength * 58 + 200);

  /// 打字结束后的收线。Kotlin: `tween(320, …)`（Splash.kt:129）。
  static const Duration sweepSettle = Duration(milliseconds: 320);

  /// 收线后停留。Kotlin: `delay(460)`（Splash.kt:130）。
  static const Duration holdAfterSweep = Duration(milliseconds: 460);

  /// 减弱动效：瞬时到位后的短驻留。Kotlin: `delay(420)`（Splash.kt:101）。
  static const Duration reduceHold = Duration(milliseconds: 420);

  /// 整屏淡出。Kotlin: `fadeOut(PaMotion.standard())`（Splash.kt:138）≈195ms。
  static const Duration exitFade = Duration(milliseconds: 200);

  /// 淡出到摘层之间再等一拍。Kotlin: `delay(240)`（Splash.kt:134）。
  static const Duration exitHold = Duration(milliseconds: 240);

  /// 减弱动效分支的摘层前等待。Kotlin: `delay(120)`（Splash.kt:103）。
  static const Duration reduceExitHold = Duration(milliseconds: 120);

  /// 品牌名。Kotlin: `const val FULL = "Pocket agent"`（Splash.kt:64）。
  static const String full = 'Pocket agent';
  static const int _fullLength = full.length;

  /// 「Pocket 」这截用墨色、其后用主色。Kotlin: `if (index < 7) ink else accent`
  /// （Splash.kt:173）。
  static const int inkPrefix = 7;
}

class _SplashOverlayState extends State<SplashOverlay>
    with TickerProviderStateMixin {
  /// Kotlin 的 4 个 `Animatable`（markAlpha/markScale/glow/sweep）一一对位。
  late final AnimationController _markAlpha =
      AnimationController.unbounded(vsync: this, value: 0);
  late final AnimationController _markScale =
      AnimationController.unbounded(vsync: this, value: _Motion.markScaleFrom);
  late final AnimationController _glow =
      AnimationController.unbounded(vsync: this, value: 0);
  late final AnimationController _sweep =
      AnimationController.unbounded(vsync: this, value: 0);

  /// Kotlin `var visible by remember { mutableStateOf(true) }`。
  bool _visible = true;

  /// Kotlin `var typed by remember { mutableIntStateOf(0) }`。
  int _typed = 0;

  bool _started = false;
  bool _doneCalled = false;

  @override
  void dispose() {
    _markAlpha.dispose();
    _markScale.dispose();
    _glow.dispose();
    _sweep.dispose();
    super.dispose();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_started) return;
    _started = true;
    // 减弱动效：Flutter 侧的对应事实是系统「移除动画」设置
    // （Kotlin 用 rememberReduceMotion()）。
    _run(MediaQuery.disableAnimationsOf(context));
  }

  Future<void> _delay(Duration d) => Future<void>.delayed(d);

  void _finish() {
    if (_doneCalled) return;
    _doneCalled = true;
    widget.onDone();
  }

  Future<void> _run(bool reduceMotion) async {
    if (reduceMotion) {
      // 字标/进度线瞬时到位，整体短驻留后直接交接主页（不播任何位移/缩放）。
      setState(() => _typed = _Motion.full.length);
      _markAlpha.value = 1;
      _markScale.value = 1;
      _glow.value = 1;
      _sweep.value = 1;
      await _delay(_Motion.reduceHold);
      if (!mounted) return;
      setState(() => _visible = false);
      await _delay(_Motion.reduceExitHold);
      _finish();
      return;
    }
    // 1) 字标先立住：淡入 + 轻微放大并行（Kotlin 两条 animateTo 同时启动）。
    _markAlpha.animateTo(1, duration: _Motion.markFade, curve: Curves.decelerate);
    _markScale.animateTo(
      1,
      duration: _Motion.markScale,
      curve: Curves.decelerate,
    );
    await _delay(_Motion.typeStartDelay);

    // 2) 逐字打出；到 "agent" 时触发光晕与进度线
    final int agentStart = _Motion.full.indexOf('agent') + 1;
    final int agentEnd = agentStart + 'agent'.length;
    for (int i = 1; i <= _Motion.full.length; i++) {
      if (!mounted) return;
      setState(() => _typed = i);
      await _delay(_Motion.full[i - 1] == ' '
          ? _Motion.spaceDelay
          : _Motion.charDelay);
      if (!mounted) return;
      if (i == agentStart) {
        _glow.animateTo(
          1,
          duration: _Motion.glowBurst,
          curve: Curves.fastOutSlowIn,
        );
        _sweep.animateTo(
          1,
          duration: _Motion.sweepWithType,
          curve: Curves.fastOutSlowIn,
        );
      }
      if (i == agentEnd) {
        // "agent" 打完再补一点光晕，形成收束感。
        // 注意：FULL 长度 12 而 agentEnd=14，这一支与 Kotlin 一样**当前不可达**
        // （Kotlin 同样把 12 字符串算出 14）——逐字保留原算式，文案一改就自动生效。
        _glow.animateTo(
          1,
          duration: _Motion.glowSettle,
          curve: Curves.fastOutSlowIn,
        );
      }
    }
    _sweep.animateTo(
      1,
      duration: _Motion.sweepSettle,
      curve: Curves.fastOutSlowIn,
    );
    await _delay(_Motion.holdAfterSweep);

    // 3) 整屏淡出，交接主页（收尾快而稳，不拖节奏）
    if (!mounted) return;
    setState(() => _visible = false);
    await _delay(_Motion.exitHold);
    _finish();
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return AnimatedSwitcher(
      duration: _Motion.exitFade,
      transitionBuilder: (Widget child, Animation<double> animation) =>
          FadeTransition(
        opacity: animation,
        child: child,
      ),
      // 淡出内容是空的（只剩外层 AbsorbPointer），与 Kotlin AnimatedVisibility
      // 收掉 Box 后「看得到主页但点不着」的中间态一致。
      child: _visible
          ? _SplashStage(
              key: const ValueKey<String>('splash-stage'),
              pa: pa,
              markAlpha: _markAlpha,
              markScale: _markScale,
              glow: _glow,
              sweep: _sweep,
              typed: _typed,
            )
          : const SizedBox.shrink(key: ValueKey<String>('splash-empty')),
    );
  }
}

/// 启动画面本体（Kotlin `Splash` 的可组合内容部分）。
class _SplashStage extends StatelessWidget {
  const _SplashStage({
    super.key,
    required this.pa,
    required this.markAlpha,
    required this.markScale,
    required this.glow,
    required this.sweep,
    required this.typed,
  });

  final PAColors pa;
  final Animation<double> markAlpha;
  final Animation<double> markScale;
  final Animation<double> glow;
  final Animation<double> sweep;
  final int typed;

  @override
  Widget build(BuildContext context) {
    // 编辑式画布随当前盘换肤（Kotlin `PaEditorial.Canvas`）＝ Flutter 侧 `bg.base`。
    return ColoredBox(
      key: const ValueKey<String>('splash-canvas'),
      color: pa.bgBase,
      child: SizedBox.expand(
        child: Stack(
          alignment: Alignment.center,
          children: <Widget>[
            // 主色光晕：在 "agent" 区域柔和扩散（Kotlin：240dp 圆 + 径向渐变）。
            Center(
              child: AnimatedBuilder(
                animation: glow,
                builder: (BuildContext context, Widget? child) => Opacity(
                  opacity: (glow.value * 0.85).clamp(0.0, 1.0),
                  child: child,
                ),
                child: Container(
                  width: 240,
                  height: 240,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    gradient: RadialGradient(
                      colors: <Color>[
                        pa.brand.withValues(alpha: 0.45),
                        pa.brand.withValues(alpha: 0.14),
                        Colors.transparent,
                      ],
                    ),
                  ),
                ),
              ),
            ),
            _mark,
          ],
        ),
      ),
    );
  }

  Widget get _mark => AnimatedBuilder(
        animation: Listenable.merge(<Animation<double>>[
          markAlpha,
          markScale,
        ]),
        builder: (BuildContext context, Widget? child) => Opacity(
          opacity: markAlpha.value.clamp(0.0, 1.0),
          child: Transform.scale(
            scale: markScale.value.clamp(0.0, 1.0),
            alignment: Alignment.center,
            child: child,
          ),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            _brandText,
            const SizedBox(height: 16),
            _progressLine,
          ],
        ),
      );

  /// 品牌名逐字打出：前 [_Motion.inkPrefix] 个字符墨色，其余主色；未打完时带光标。
  Widget get _brandText {
    final String shown = _Motion.full.substring(0, typed.clamp(0, _Motion.full.length));
    final List<InlineSpan> spans = <InlineSpan>[];
    for (int i = 0; i < shown.length; i++) {
      spans.add(
        TextSpan(
          text: shown[i],
          style: TextStyle(
            color: i < _Motion.inkPrefix ? pa.fgPrimary : pa.brand,
          ),
        ),
      );
    }
    if (typed < _Motion.full.length) {
      spans.add(
        TextSpan(
          text: '▍',
          style: TextStyle(color: pa.brand.withValues(alpha: 0.65)),
        ),
      );
    }
    return Text.rich(
      TextSpan(children: spans),
      // Kotlin 用 PaEditorial.Display 档（Poppins Bold）；Flutter 侧设计系统没有
      // display 字号，取标题档放大到 36 作为启动字标的展示层级。
      style: PATypography.titleL.copyWith(
        fontSize: 36,
        height: 1.1,
        letterSpacing: -0.5,
        fontWeight: FontWeight.w800,
        color: pa.fgPrimary,
      ),
    );
  }

  /// 进度线：随打字节奏展开，停留时收满（底槽 = hairline 细线）。
  Widget get _progressLine => AnimatedBuilder(
        animation: sweep,
        builder: (BuildContext context, Widget? child) => SizedBox(
          width: 150,
          height: 3,
          child: Stack(
            children: <Widget>[
              Positioned.fill(
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: pa.hairline,
                    borderRadius: BorderRadius.circular(50),
                  ),
                ),
              ),
              Align(
                alignment: Alignment.centerLeft,
                child: FractionallySizedBox(
                  widthFactor: sweep.value.clamp(0.0, 1.0),
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      color: pa.brand,
                      borderRadius: BorderRadius.circular(50),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      );
}
