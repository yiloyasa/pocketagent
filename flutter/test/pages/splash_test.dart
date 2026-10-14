import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/splash.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';

/// Splash 覆盖层测试（Kotlin `ui/Splash.kt` 行为对位）。
///
/// 覆盖层语义（MainActivity.kt:112-127）在 `main_routing_test.dart` 里验收；
/// 这里只验收动画链路本身：立拍 → 逐字 → 收线 → 淡出 → onDone，以及减弱动效分支。
void main() {
  Widget host(VoidCallback onDone, {bool reduceMotion = false}) {
    final Widget app = MaterialApp(
      theme: PocketTheme.fromBrightness(PABrightness.dark),
      home: SplashOverlay(onDone: onDone),
    );
    if (!reduceMotion) return app;
    return MediaQuery(
      data: const MediaQueryData(disableAnimations: true),
      child: app,
    );
  }

  testWidgets('首帧即覆盖全屏_且还没有到 onDone', (tester) async {
    int doneCount = 0;
    await tester.pumpWidget(host(() => doneCount++));

    expect(find.byType(SplashOverlay), findsOneWidget);
    // 淡入没走完 → 品牌名还没开始打（typed=0）
    expect(find.text('Pocket agent'), findsNothing);
    expect(doneCount, 0);

    // 断言完成后推进异步链走完（testWidgets 收尾要求无挂起 timer）。
    await pumpSplashSettled(tester);
  });

  testWidgets('淡入走完后逐字打出品牌名_不是整句硬现', (tester) async {
    int doneCount = 0;
    await tester.pumpWidget(host(() => doneCount++));

    // t≈140ms 开始打字；推到 600ms 时「Pocket」六字已出、整句未完（空格更慢），
    // 处于「打了一半」的中间态：140 + 6×58 = 488ms 打完 'Pocket'，
    // 空格 delay 120ms 到 608ms 才进入 'agent'。
    await tester.pump(const Duration(milliseconds: 600));
    final String mid = _brandText(tester);
    expect(mid.isNotEmpty, isTrue);
    expect(mid, isNot('Pocket agent'), reason: '逐字打出的中间态不应已是整句');
    expect(mid.startsWith('Pocket'), isTrue,
        reason: '打字必须从词首循序往后，不能跳字');
    expect(doneCount, 0);

    // 收尾：清挂起 timer（断言均已执行完）。
    await pumpSplashSettled(tester);
  });

  testWidgets('整段动画播完_淡出结束后 onDone 回调一次并摘层', (tester) async {
    int doneCount = 0;
    await tester.pumpWidget(host(() => doneCount++));

    // 打完字（含光标）
    await tester.pump(const Duration(milliseconds: 900));
    expect(find.text('Pocket agent'), findsOneWidget);
    expect(doneCount, 0);

    // 推进到 1600ms：动画(~1.6s)已过、淡出已起
    await tester.pump(const Duration(milliseconds: 400));
    expect(doneCount, 0, reason: '淡出中途还没到交接时点');

    // 淡出结束 → onDone
    await tester.pump(const Duration(milliseconds: 400));
    expect(doneCount, 1);

    // 之后再推进也不会重复回调
    await tester.pump(const Duration(milliseconds: 400));
    expect(doneCount, 1);

    await pumpSplashSettled(tester);
    // AnimatedSwitcher 的退场动画在 setState 之后才起跳，等整链 settle 后
    // 断言字标内容确已摘除（覆盖层本体由调用方决定摘不摘，这里只看内容）。
    expect(find.text('Pocket agent'), findsNothing);
  });

  testWidgets('减弱动效_瞬时到位并短驻留后交接', (tester) async {
    int doneCount = 0;
    await tester.pumpWidget(host(() => doneCount++, reduceMotion: true));
    // 首帧里 _run 只发起了 setState，再 pump 一帧让瞬时到位的字标落到树上。
    await tester.pump();

    // 不播任何逐字过程，字标/进度线瞬时到位
    expect(find.text('Pocket agent'), findsOneWidget);
    expect(doneCount, 0);

    await tester.pump(const Duration(milliseconds: 300));
    expect(doneCount, 0, reason: '还要驻留 420ms');

    await tester.pump(const Duration(milliseconds: 300));
    expect(doneCount, 1, reason: '驻留 420 + 淡出等 120 后交接');
    await tester.pump(const Duration(milliseconds: 300));
    expect(doneCount, 1);

    await pumpSplashSettled(tester);
  });

  testWidgets('画布走设计系统 bg.base（三盘同槽位换肤）', (tester) async {
    await tester.pumpWidget(host(() {}));

    final ColoredBox canvas = tester.widget<ColoredBox>(
      find.byKey(const ValueKey<String>('splash-canvas')),
    );
    expect(canvas.color, PAColors.dark.bgBase);

    await pumpSplashSettled(tester);
  });
}

/// 把 Splash 的异步节拍链推进到 `_finish()` 之后（覆盖普通与减弱动效两支），
/// 清掉 `testWidgets` 收尾时不允许挂起的 `Future.delayed` timer。
Future<void> pumpSplashSettled(WidgetTester tester) async {
  for (int i = 0; i < 120; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

/// 取出当前帧渲染出的品牌名（含未打完时的光标符）。
String _brandText(WidgetTester tester) {
  final Text text = tester.widget<Text>(find.byType(Text));
  final InlineSpan span = text.textSpan ?? TextSpan(text: text.data ?? '');
  return span.toPlainText();
}
