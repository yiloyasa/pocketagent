import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/main.dart';
import 'package:pocketagent_ui/pages/onboarding.dart';
import 'package:pocketagent_ui/pages/onboarding_prefs.dart';
import 'package:pocketagent_ui/pages/splash.dart';

import 'support/pump_helpers.dart';

/// 根装配测试：Splash 覆盖层 → Onboarding → 4 Tab HomeShell 的启动路由
/// （Kotlin `MainActivity.kt:112-127` 覆盖层 + `RootNav` startDestination 语义）。
void main() {
  testWidgets('首启:Splash 覆盖 Onboarding,播完摘层后才点得到教程', (tester) async {
    final InMemoryOnboardingPrefs prefs = InMemoryOnboardingPrefs();
    await tester.pumpWidget(PocketAgentApp(onboardingPrefs: prefs));
    await tester.pump();

    // 首启未看过教程 → 底层是 Onboarding，4 Tab 壳不在树上
    expect(find.byType(SplashOverlay), findsOneWidget);
    expect(find.text('欢迎使用 PocketAgent'), findsOneWidget);
    expect(find.byType(HomeShell), findsNothing);
    expect(find.byType(NavigationBar), findsNothing);

    // 覆盖层吃点击：教程按钮现在点不动（Kotlin clickable(indication=null) 同语义）
    await tester.tap(find.text('跳过'), warnIfMissed: false);
    await tester.pump();
    expect(prefs.isDone, isFalse, reason: '覆盖层还在，不可能完成教程');
    expect(find.byType(SplashOverlay), findsOneWidget);

    // 播完 → 摘层 → 覆盖层不再拦截
    await pumpUntilSplashDone(tester);
    expect(find.byType(SplashOverlay), findsNothing);
    expect(find.text('欢迎使用 PocketAgent'), findsOneWidget);
  });

  testWidgets('完成引导后回 HomeShell_并写持久化标志', (tester) async {
    final InMemoryOnboardingPrefs prefs = InMemoryOnboardingPrefs();
    await tester.pumpWidget(PocketAgentApp(onboardingPrefs: prefs));
    await pumpUntilSplashDone(tester);

    expect(find.byType(HomeShell), findsNothing);

    await tester.tap(find.text('跳过'));
    await tester.pumpAndSettle();

    expect(prefs.isDone, isTrue, reason: 'Kotlin finishOnboarding 会 markDone');
    expect(find.byType(HomeShell), findsOneWidget);
    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(OnboardingPage), findsNothing);

    // 四个 Tab 都能切（顶栏标题与 Nav 选中标签都会变成「工具」，故 findsWidgets）
    await tester.tap(find.byIcon(Icons.handyman_outlined));
    await tester.pumpAndSettle();
    expect(find.text('工具'), findsWidgets);
    final NavigationBar nav2 =
        tester.widget<NavigationBar>(find.byType(NavigationBar));
    expect(nav2.selectedIndex, 1);
  });

  testWidgets('教程可横滑切页', (tester) async {
    await tester.pumpWidget(
      PocketAgentApp(onboardingPrefs: InMemoryOnboardingPrefs()),
    );
    await pumpUntilSplashDone(tester);

    await tester.fling(find.byType(PageView), const Offset(-400, 0), 800);
    await tester.pumpAndSettle();
    expect(find.text('第一步 · 让手机找到电脑'), findsOneWidget);

    await tester.fling(find.byType(PageView), const Offset(-400, 0), 800);
    await tester.pumpAndSettle();
    expect(find.text('第二步 · 配对'), findsOneWidget);
  });

  testWidgets('下次启动已 done:直接 HomeShell,教程不再出现', (tester) async {
    final InMemoryOnboardingPrefs prefs = InMemoryOnboardingPrefs(done: true);
    await tester.pumpWidget(PocketAgentApp(onboardingPrefs: prefs));
    await pumpUntilSplashDone(tester);

    expect(find.byType(HomeShell), findsOneWidget);
    expect(find.byType(OnboardingPage), findsNothing);
    expect(find.text('欢迎使用 PocketAgent'), findsNothing);
    expect(prefs.isDone, isTrue);

    // 四个 Tab 全在（不许倒退）
    final NavigationBar nav = tester.widget<NavigationBar>(find.byType(NavigationBar));
    expect(nav.destinations, hasLength(4));
    expect(nav.selectedIndex, 0);
  });

  testWidgets('Splash 摘层前 HomeShell 已在树上并行组合(不是串行闸)', (tester) async {
    await tester.pumpWidget(
      PocketAgentApp(onboardingPrefs: InMemoryOnboardingPrefs(done: true)),
    );
    await tester.pump();

    // Kotlin 覆盖率语义：RootNav 在 Splash 之下并行组合，Splash 只负责摘层。
    expect(find.byType(SplashOverlay), findsOneWidget);
    expect(
      find.byType(HomeShell).evaluate(),
      isNotEmpty,
      reason: 'Splash 播放期间主页必须已组合好，不是等播完才开始',
    );

    // 用例收尾前推进摘层，避免挂起 timer（testWidgets 要求无未决定时器）。
    await pumpUntilSplashDone(tester);
    await tester.pumpAndSettle();
    expect(find.byType(SplashOverlay), findsNothing);
  });
}
