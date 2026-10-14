import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/main.dart';

void main() {
  testWidgets('底部 4 Tab（会话/工具/文件/设置）默认暗色，深浅色可切', (tester) async {
    await tester.pumpWidget(const PocketAgentApp());
    await tester.pumpAndSettle();

    final NavigationBar nav =
        tester.widget<NavigationBar>(find.byType(NavigationBar));
    expect(nav.destinations, hasLength(4));
    expect(
      nav.destinations
          .map((Widget d) => (d as NavigationDestination).label)
          .toList(),
      <String>['会话', '工具', '文件', '设置'],
    );
    expect(nav.selectedIndex, 0);
    expect(find.text('静态示例数据 · B3 阶段接入真实会话'), findsOneWidget);

    // 默认暗色
    expect(
      Theme.of(tester.element(find.byType(HomeShell))).brightness,
      Brightness.dark,
    );

    // 切到设置 Tab（首屏分组）
    await tester.tap(find.byIcon(Icons.settings_outlined));
    await tester.pumpAndSettle();
    expect(find.text('连接与配对'), findsOneWidget);
    expect(find.text('外观'), findsOneWidget);

    // 深色模式开关初始为开
    final Switch initialSwitch =
        tester.widget<Switch>(find.byType(Switch).first);
    expect(initialSwitch.value, isTrue);

    // 深浅色开关（AppBar 动作）
    await tester.tap(find.byKey(const ValueKey<String>('pa-brightness-toggle')));
    await tester.pumpAndSettle();
    expect(
      Theme.of(tester.element(find.byType(HomeShell))).brightness,
      Brightness.light,
    );
    expect(
      tester.widget<Switch>(find.byType(Switch).first).value,
      isFalse,
      reason: '设置页深色模式开关与 AppBar 开关状态同步',
    );

    // 切回深色
    await tester.tap(find.byKey(const ValueKey<String>('pa-brightness-toggle')));
    await tester.pumpAndSettle();
    expect(
      Theme.of(tester.element(find.byType(HomeShell))).brightness,
      Brightness.dark,
    );

    // 滚到底部看其余分组（ListView 懒构建，屏外文本不在树里）
    final Finder settingsScrollable = find.byType(Scrollable).first;
    await tester.scrollUntilVisible(
      find.text('关于'),
      300,
      scrollable: settingsScrollable,
    );
    await tester.pumpAndSettle();
    expect(find.text('语音'), findsOneWidget);
    expect(find.text('关于'), findsOneWidget);
    expect(find.text('新手引导'), findsOneWidget);
  });

  testWidgets('设置页可进入 3 页引导并横滑', (tester) async {
    await tester.pumpWidget(const PocketAgentApp());
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.settings_outlined));
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(
      find.text('新手引导'),
      300,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('新手引导'));
    await tester.pumpAndSettle();

    expect(find.text('这台电脑是什么'), findsOneWidget);
    expect(find.text('跳过'), findsOneWidget);
    expect(find.text('下一步'), findsOneWidget);

    await tester.fling(
      find.byType(PageView),
      const Offset(-400, 0),
      800,
    );
    await tester.pumpAndSettle();
    expect(find.text('怎么连'), findsOneWidget);
  });
}
