import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/splash.dart';

/// 把 Splash 覆盖层的定时链路推进到摘层为止。
///
/// 为什么不用 `pumpAndSettle`：Splash 的节拍器是 `Future.delayed`（Kotlin
/// `delay(...)` 的直译），两拍之间没有待渲染帧时 `pumpAndSettle` 会提前返回
/// （它只等 `hasScheduledFrame`），覆盖层还盖在主页上，后续 tap 会被
/// `AbsorbPointer` 吃掉而「假失败」。故按固定步长显式推进时钟，直到覆盖层真被摘掉。
Future<void> pumpUntilSplashDone(WidgetTester tester) async {
  for (int i = 0; i < 120; i++) {
    if (find.byType(SplashOverlay).evaluate().isEmpty) return;
    await tester.pump(const Duration(milliseconds: 100));
  }
  fail('Splash 覆盖层在 12s 仿真时间内没有摘层');
}
