import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/main.dart';
import 'package:pocketagent_ui/pages/git.dart';
import 'package:pocketagent_ui/pages/onboarding.dart';
import 'package:pocketagent_ui/pages/onboarding_prefs.dart';
import 'package:pocketagent_ui/pages/splash.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

import 'support/pump_helpers.dart';

/// 根装配测试：Splash 覆盖层 → Onboarding → 4 Tab HomeShell 的启动路由
/// （Kotlin `MainActivity.kt:112-127` 覆盖层 + `RootNav` startDestination 语义）。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  // HomeShell 用 IndexedStack 同时构建四个 Tab：ThreadsPage 的默认数据源
  // （BridgeThreadsDataSource）会真发 `dev.pocketagent/bridge` 调用。宿主桥
  // B4 波次前不存在——真实设备上如实得 HOST_UNAVAILABLE；而 flutter_tester
  // 的**未 mock 通道**只在真实事件循环里送错（fake-async 下永不送达），
  // 加载转圈会让 pumpAndSettle 超时。按生产路径伪造「宿主未注册」，让线程
  // 页走它自己的诚实路径（错误态），本文件只管路由断言。
  setUp(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
      const MethodChannel(kPaBridgeMethodChannel),
      (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      },
    );
  });

  // ── B3 第 5/6 波 + 第 4 波路由外衣（AppBar 溢出菜单 → push → 返回 pop） ──
  // 只钉「入口点得动、页面出得来、返回回得去」；各页内部行为归各页测试。
  for (final (String title, String backKey) in const <(String, String)>[
    ('终端', 'pa-terminal-back'),
    ('任务中心', 'pa-taskcenter-back'),
    ('协同', 'pa-collab-back'),
    ('协同角色', 'pa-collabroles-back'),
    ('浏览器', 'pa-browser-back'),
  ]) {
    testWidgets('AppBar 溢出菜单 → $title 路由页；返回 → pop 回 4 Tab', (tester) async {
      await tester.pumpWidget(
        PocketAgentApp(onboardingPrefs: InMemoryOnboardingPrefs(done: true)),
      );
      await pumpUntilSplashDone(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey<String>('pa-more-menu')));
      await tester.pumpAndSettle();
      await tester.tap(find.text(title));
      await tester.pumpAndSettle();

      expect(find.text(title), findsWidgets);
      expect(find.byKey(ValueKey<String>(backKey)), findsOneWidget);

      await tester.tap(find.byKey(ValueKey<String>(backKey)));
      await tester.pumpAndSettle();
      // 返回后 4 Tab 壳回来（会话 Tab 标题可见，且菜单按钮仍在）。
      // 回壳判据：4 Tab 导航栅与溢出菜单都回来了。
      // “会话”在 AppBar 标题与 Nav 标签各一份，因此用 findsWidgets。
      expect(find.byKey(const ValueKey<String>('pa-more-menu')), findsOneWidget);
    });
  }

  // 协同角色路由只针「入口点得动、页面出得来、返回回得去」。
  // 角色行点指→编辑器子路由（onOpenEditor）由 collab_roles_page_test
  // 用注入 store 覆盖（真实路由下 host 未就续时角色页是错误态、无行可点）。
  testWidgets('协同角色路由页：入口→页面→返回 pop', (tester) async {
    await tester.pumpWidget(
      PocketAgentApp(onboardingPrefs: InMemoryOnboardingPrefs(done: true)),
    );
    await pumpUntilSplashDone(tester);
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey<String>('pa-more-menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('协同角色'));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey<String>('pa-collabroles-back')),
        findsOneWidget);

    await tester.tap(find.byKey(const ValueKey<String>('pa-collabroles-back')));
    await tester.pumpAndSettle();
    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byKey(const ValueKey<String>('pa-more-menu')), findsOneWidget);
  });
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

  testWidgets('AppBar Git 入口 → 全屏 Git 路由页；返回即 pop 回 4 Tab 壳',
      (tester) async {
    await tester.pumpWidget(
      PocketAgentApp(onboardingPrefs: InMemoryOnboardingPrefs(done: true)),
    );
    await pumpUntilSplashDone(tester);
    await tester.pumpAndSettle();

    // 4 Tab 位不动（契约的 tab3 在 Flutter 侧是独立路由页，见 main.dart 入口注释）
    final NavigationBar nav = tester.widget<NavigationBar>(find.byType(NavigationBar));
    expect(nav.destinations, hasLength(4));

    await tester.tap(find.byKey(const ValueKey<String>('pa-open-git')));
    await tester.pumpAndSettle();

    // Git 页进了路由栈：全屏页 + 仓库选择卡（候选/手输入口都在）
    expect(find.byType(GitPage), findsOneWidget);
    expect(find.byKey(const ValueKey<String>('git-repo-input')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey<String>('pa-git-back')));
    await tester.pumpAndSettle();
    expect(find.byType(GitPage), findsNothing);
    expect(find.byType(NavigationBar), findsOneWidget);
  });
}
