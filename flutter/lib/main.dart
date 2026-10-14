import 'package:flutter/material.dart';

import 'pages/files.dart';
import 'pages/settings.dart';
import 'pages/threads.dart';
import 'pages/tools.dart';
import 'theme/palette.dart';
import 'theme/pocket_theme.dart';

void main() => runApp(const PocketAgentApp());

/// PocketAgent Flutter UI 根组件：暗色为默认，浅色可切。
class PocketAgentApp extends StatefulWidget {
  const PocketAgentApp({super.key});

  @override
  State<PocketAgentApp> createState() => _PocketAgentAppState();
}

class _PocketAgentAppState extends State<PocketAgentApp> {
  PABrightness _brightness = PocketTheme.defaultBrightness;

  void _toggleBrightness() {
    setState(() {
      _brightness = _brightness == PABrightness.dark
          ? PABrightness.light
          : PABrightness.dark;
    });
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
      home: HomeShell(
        brightness: _brightness,
        onToggleBrightness: _toggleBrightness,
      ),
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
