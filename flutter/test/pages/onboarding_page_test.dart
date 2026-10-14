import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/onboarding.dart';

/// Onboarding 页测试（Kotlin `ui/Onboarding.kt` 页本体行为对位）。
///
/// 验收口径：页清单来自纯函数 [onboardPages]（另见 onboarding_steps_test），
/// 这里只测 widget 层——翻页、指示点、完成位标注、按钮去向与回调接线。
void main() {
  Future<void> pumpOnboarding(
    WidgetTester tester, {
    bool connected = false,
    VoidCallback? onFinished,
    VoidCallback? onOpenSettings,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        home: OnboardingPage(
          connected: connected,
          onFinished: onFinished,
          onOpenSettings: onOpenSettings,
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('六页横滑引导_欢迎页起始_CTA 是下一步', (tester) async {
    await pumpOnboarding(tester);

    expect(find.text('欢迎使用 PocketAgent'), findsOneWidget);
    expect(find.text('用手机,遥控你电脑上的 AI 干活。'), findsOneWidget);
    expect(find.text('跳过'), findsOneWidget);
    expect(find.text('下一步'), findsOneWidget);
    // 非首页没有「上一步」
    expect(find.text('上一步'), findsNothing);
  });

  testWidgets('圆点指示器数量=页数_当前点放大', (tester) async {
    await pumpOnboarding(tester);

    final List<AnimatedContainer> dots =
        tester.widgetList<AnimatedContainer>(find.byType(AnimatedContainer)).toList();
    // 底部指示点是唯一一组 AnimatedContainer
    expect(dots.length, 6);
    // AnimatedContainer 不暴露 width 字段，按渲染尺寸断言「当前点放大」。
    final Size firstSize = tester.getSize(find.byType(AnimatedContainer).at(0));
    final Size secondSize = tester.getSize(find.byType(AnimatedContainer).at(1));
    expect(firstSize.width, greaterThan(secondSize.width), reason: '当前点放大');
  });

  testWidgets('横滑切页_按脊柱顺序前进后退', (tester) async {
    await pumpOnboarding(tester);

    await tester.fling(find.byType(PageView), const Offset(-400, 0), 800);
    await tester.pumpAndSettle();
    expect(find.text('第一步 · 让手机找到电脑'), findsOneWidget);

    await tester.fling(find.byType(PageView), const Offset(-400, 0), 800);
    await tester.pumpAndSettle();
    expect(find.text('第二步 · 配对'), findsOneWidget);

    // 往回滑回到欢迎页
    await tester.fling(find.byType(PageView), const Offset(400, 0), 800);
    await tester.pumpAndSettle();
    expect(find.text('第一步 · 让手机找到电脑'), findsOneWidget);
  });

  testWidgets('底部 CTA 逐页前进_末页变开始使用并回调完成', (tester) async {
    int finished = 0;
    await pumpOnboarding(tester, onFinished: () => finished++);

    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('第一步 · 让手机找到电脑'), findsOneWidget);

    // 翻到末页（收尾页）：欢迎 + 4 步 = 还需 4 次
    for (int i = 0; i < 4; i++) {
      await tester.tap(find.text('下一步'));
      await tester.pumpAndSettle();
    }
    expect(find.text('一切就绪'), findsOneWidget);
    expect(find.text('开始使用'), findsOneWidget);

    await tester.tap(find.text('开始使用'));
    await tester.pumpAndSettle();
    expect(finished, 1);
  });

  testWidgets('上一步按钮_非首页才可见且回退一页', (tester) async {
    await pumpOnboarding(tester);

    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('第一步 · 让手机找到电脑'), findsOneWidget);

    final Finder prev = find.widgetWithText(TextButton, '上一步');
    expect(prev, findsOneWidget);
    await tester.tap(prev);
    await tester.pumpAndSettle();
    expect(find.text('欢迎使用 PocketAgent'), findsOneWidget);
  });

  testWidgets('任意页都可跳过_提前结束教程', (tester) async {
    int finished = 0;
    await pumpOnboarding(tester, onFinished: () => finished++);

    expect(find.text('跳过'), findsOneWidget);
    await tester.tap(find.text('跳过'));
    await tester.pumpAndSettle();
    expect(finished, 1);
  });

  testWidgets('已连接_连接与配对页只标注已完成_不重复展开打扰', (tester) async {
    await pumpOnboarding(tester, connected: true);

    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('第一步 · 让手机找到电脑'), findsOneWidget);
    expect(find.text('已完成,不用再操作'), findsOneWidget);
    // 已完成步骤的替换话术直达
    expect(find.text('电脑已经连上了,这一步直接跳过。'), findsOneWidget);

    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('第二步 · 配对'), findsOneWidget);
    expect(find.text('已经配对好了,这一步直接跳过。'), findsOneWidget);
  });

  testWidgets('排障页直达按钮_去设置_接线由调用方注入', (tester) async {
    int openSettings = 0;
    await pumpOnboarding(tester, onOpenSettings: () => openSettings++);

    // 欢迎 + 4 步 → 排障页是第 5 页（index 4）
    for (int i = 0; i < 4; i++) {
      await tester.tap(find.text('下一步'));
      await tester.pumpAndSettle();
    }
    expect(find.text('连不上?'), findsOneWidget);

    await tester.tap(find.text('去设置填写地址'));
    await tester.pumpAndSettle();
    expect(openSettings, 1);
  });

  testWidgets('没收 settings 接线时_排障按钮不假装修好了', (tester) async {
    await pumpOnboarding(tester);
    for (int i = 0; i < 4; i++) {
      await tester.tap(find.text('下一步'));
      await tester.pumpAndSettle();
    }
    expect(find.text('去设置填写地址'), findsOneWidget);
    // 没有注入回调也不应抛异常、不应退出教程
    await tester.tap(find.text('去设置填写地址'));
    await tester.pumpAndSettle();
    expect(find.text('连不上?'), findsOneWidget);
  });

  testWidgets('前三步与收尾页都有页内直达按钮', (tester) async {
    await pumpOnboarding(tester);

    // 欢迎页不放直达按钮（脊柱还没开始），后退钮也不可见。
    expect(find.text('去首页发现列表'), findsNothing);

    // 第 1 步（连接）→ 第 2 步（配对）→ 第 3 步（首指令）逐页核按钮。
    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('去首页发现列表'), findsOneWidget);
    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('去首页'), findsOneWidget);
    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('去发第一条指令'), findsOneWidget);

    // 排障页 → 收尾页：收尾页同样带直达按钮（回主页发第一条指令）。
    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('下一步'));
    await tester.pumpAndSettle();
    expect(find.text('一切就绪'), findsOneWidget);
    expect(find.text('去发第一条指令'), findsOneWidget);
  });
}
