import 'package:flutter/material.dart';

import 'pages/files.dart';
import 'pages/git.dart';
import 'pages/onboarding.dart';
import 'pages/onboarding_prefs.dart';
import 'pages/settings.dart';
import 'pages/splash.dart';
import 'pages/threads.dart';
import 'pages/tools.dart';
import 'theme/palette.dart';
import 'theme/pocket_theme.dart';

void main() => runApp(const PocketAgentApp());

/// PocketAgent Flutter UI 根组件：暗色为默认，浅色可切。
class PocketAgentApp extends StatefulWidget {
  const PocketAgentApp({super.key, this.onboardingPrefs});

  /// 新手教程完成标志的持久化来源。
  ///
  /// 开放注入的原因：Kotlin 侧 `RootNav` 首启读 `OnboardingPrefs.isDone()`
  /// （MainActivity.kt:167）决定 startDestination，Flutter 侧同语义；widget 测试要
  /// 同时覆盖「首启未完成」与「已完成」两条路径，没有注入点就只能改全局状态。
  final OnboardingPrefs? onboardingPrefs;

  @override
  State<PocketAgentApp> createState() => _PocketAgentAppState();
}

class _PocketAgentAppState extends State<PocketAgentApp> {
  late final OnboardingPrefs _prefs =
      widget.onboardingPrefs ?? InMemoryOnboardingPrefs();

  /// 启动路由：未看过教程 → Onboarding，否则 4 Tab HomeShell（Kotlin startDestination）。
  late bool _onboardingDone = _prefs.isDone;

  /// Splash 覆盖层是否还在（Kotlin `var showSplash by remember { mutableStateOf(true) }`）。
  bool _splashDone = false;

  PABrightness _brightness = PocketTheme.defaultBrightness;

  void _toggleBrightness() {
    setState(() {
      _brightness = _brightness == PABrightness.dark
          ? PABrightness.light
          : PABrightness.dark;
    });
  }

  /// 教程完成分流（Kotlin `finishOnboarding`）。
  ///
  /// Kotlin 首启走 `navigate("home") + popUpTo(onboarding, inclusive)`、重看走
  /// `popBackStack()`，靠 `previousBackStackEntry` 现查分流。Flutter 侧没有
  /// NavController，用「当前展示的是不是 Onboarding」这一件事表达同一个分流：
  /// 摘掉 Onboarding 即等于把主页露出来（首启），重看入口（设置页）本来就是
  /// push 上来的，回落即 pop。
  Future<void> _finishOnboarding() async {
    await _prefs.markDone();
    if (!mounted) return;
    setState(() => _onboardingDone = true);
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'PocketAgent',
      debugShowCheckedModeBanner: false,
      theme: PocketTheme.fromBrightness(PABrightness.light),
      darkTheme: PocketTheme.fromBrightness(PABrightness.dark),
      themeMode: _brightness == PABrightness.dark
          ? ThemeMode.dark
          : ThemeMode.light,
      // 【覆盖层不是串行闸】Kotlin 侧 RootNav 在 Splash 之下并行组合、Splash 只负责
      // 摘层（MainActivity.kt:102-127）。Dart 侧同构：Stack 底层是真实内容
      // （Onboarding 或 4 Tab HomeShell），Splash 以 Positioned.fill 覆盖其上，
      // 播放期间 AbsorbPointer 吃掉点击，避免淡出中误触穿屏。
      home: _AppRoot(
        onboardingDone: _onboardingDone,
        brightness: _brightness,
        onToggleBrightness: _toggleBrightness,
        onFinishOnboarding: _finishOnboarding,
        splashDone: _splashDone,
        onSplashDone: () => setState(() => _splashDone = true),
      ),
    );
  }
}

/// 根装配层：真实内容 + Splash 覆盖层。
class _AppRoot extends StatelessWidget {
  const _AppRoot({
    required this.onboardingDone,
    required this.brightness,
    required this.onToggleBrightness,
    required this.onFinishOnboarding,
    required this.splashDone,
    required this.onSplashDone,
  });

  final bool onboardingDone;
  final PABrightness brightness;
  final VoidCallback onToggleBrightness;
  final VoidCallback onFinishOnboarding;
  final bool splashDone;
  final VoidCallback onSplashDone;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: <Widget>[
        if (onboardingDone)
          HomeShell(
            brightness: brightness,
            onToggleBrightness: onToggleBrightness,
          )
        else
          OnboardingPage(
            // TODO(B3 后续波次 · B2 通道)：connected 由 connState 流注入，
            // 当前低依赖波次不建通道，默认按「未连接」讲完整步骤。
            connected: false,
            onFinished: onFinishOnboarding,
          ),
        if (!splashDone)
          Positioned.fill(
            child: AbsorbPointer(
              // 覆盖层吃点击：动画期间用户点不到尚未就绪的主页。
              absorbing: true,
              child: SplashOverlay(onDone: onSplashDone),
            ),
          ),
      ],
    );
  }
}

/// 底部 4 Tab 壳：会话 / 工具 / 文件 / 设置 + 深浅色切换开关。
class HomeShell extends StatefulWidget {
  const HomeShell({
    super.key,
    required this.brightness,
    required this.onToggleBrightness,
  });

  final PABrightness brightness;
  final VoidCallback onToggleBrightness;

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  static const List<String> _titles = <String>['会话', '工具', '文件', '设置'];

  int _tab = 0;

  @override
  Widget build(BuildContext context) {
    final bool isDark = widget.brightness == PABrightness.dark;
    return Scaffold(
      appBar: AppBar(
        title: Text(_titles[_tab]),
        actions: <Widget>[
          // Git 页导航入口（page-contracts §1.5：入 = home tab3「或带
          // workspacePath」）。【为什么不是第 5 个 Tab】契约的 tab3 在 Android
          // 是底部导航的一员，但 Flutter 壳的四个 Tab 位（会话/工具/文件/设置）
          // 已被 B1 版式与 widget_test/main_routing_test 钉死（hasLength(4)），
          // 且「工具」位目前是静态演示页——把 Git 塞进去要么挤掉设置位、要么改动
          // 两份既有测试的目的。故按「独立路由页」落：AppBar 一个入口 push 全屏
          // Git 页（Kotlin 侧 Git 在侧边栏同样是一级入口，不是 tab 内嵌页）。
          IconButton(
            key: const ValueKey<String>('pa-open-git'),
            tooltip: 'Git 仓库',
            icon: const Icon(Icons.account_tree_rounded),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (BuildContext context) => const _GitRoutePage(),
              ),
            ),
          ),
          IconButton(
            key: const ValueKey<String>('pa-brightness-toggle'),
            tooltip: isDark ? '切换到浅色' : '切换到深色',
            icon: Icon(isDark ? Icons.light_mode_rounded : Icons.dark_mode_rounded),
            onPressed: widget.onToggleBrightness,
          ),
          const SizedBox(width: 6),
        ],
      ),
      body: IndexedStack(
        index: _tab,
        children: <Widget>[
          const ThreadsPage(),
          const ToolsPage(),
          const FilesPage(),
          // 设置页直连根组件的深浅色状态
          SettingsPage(
            brightness: widget.brightness,
            onBrightnessChanged: (PABrightness b) {
              if (b != widget.brightness) widget.onToggleBrightness();
            },
          ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: (int i) => setState(() => _tab = i),
        destinations: const <NavigationDestination>[
          NavigationDestination(
            icon: Icon(Icons.forum_outlined),
            selectedIcon: Icon(Icons.forum_rounded),
            label: '会话',
          ),
          NavigationDestination(
            icon: Icon(Icons.handyman_outlined),
            selectedIcon: Icon(Icons.handyman_rounded),
            label: '工具',
          ),
          NavigationDestination(
            icon: Icon(Icons.folder_outlined),
            selectedIcon: Icon(Icons.folder_rounded),
            label: '文件',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings_outlined),
            selectedIcon: Icon(Icons.settings_rounded),
            label: '设置',
          ),
        ],
      ),
    );
  }
}

/// Git 页的路由外衣：全屏 Scaffold + 返回箭头（独立路由页的最小形态）。
///
/// 【为什么不内嵌进 IndexedStack】IndexedStack 的非选中子树不进组件树
/// （flutter_test 实测：hidden children 不可 find）——把 Git 页塞进第 5 位会
/// 让它在测试里「时有时无」，而且 tab 位已被既有测试钉死（见入口处注释）。
/// 独立 push 路由的语义与 Kotlin 侧一致：Git 是侧边栏一级入口，返回即 pop。
class _GitRoutePage extends StatelessWidget {
  const _GitRoutePage();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Git'),
        leading: IconButton(
          key: const ValueKey<String>('pa-git-back'),
          tooltip: '返回',
          icon: const Icon(Icons.arrow_back_rounded),
          onPressed: () => Navigator.of(context).pop(),
        ),
      ),
      body: const GitPage(),
    );
  }
}
