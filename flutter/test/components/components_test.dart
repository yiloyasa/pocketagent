import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_approval_sheet.dart';
import 'package:pocketagent_ui/components/pa_button.dart';
import 'package:pocketagent_ui/components/pa_card.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/components/pa_tool_card.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/theme/radii.dart';

Widget _wrap(Widget child, {PABrightness brightness = PABrightness.dark}) {
  return MaterialApp(
    theme: PocketTheme.fromBrightness(brightness),
    home: Scaffold(body: child),
  );
}

void main() {
  testWidgets('PACard 渲染 surface 底 + hairline 边 + 14 圆角', (tester) async {
    await tester.pumpWidget(
      _wrap(const PACard(child: Text('card-body'))),
    );
    expect(find.text('card-body'), findsOneWidget);

    final Container container = tester.widget<Container>(
      find.descendant(
        of: find.byType(PACard),
        matching: find.byType(Container),
      ),
    );
    final BoxDecoration deco = container.decoration! as BoxDecoration;
    expect(deco.color, PAColors.dark.bgSurface);
    expect(deco.borderRadius, PARadii.cardBR);
    final Border border = deco.border! as Border;
    expect(border.top.width, 1);
    expect(border.top.color, PAColors.dark.hairline);
  });

  testWidgets('PAButton 三变体：primary 恒品牌蓝、永不红', (tester) async {
    await tester.pumpWidget(
      _wrap(
        Row(
          children: <Widget>[
            PAButton(label: '允许', onPressed: () {}),
            const SizedBox(width: 8),
            PAButton(label: '拒绝', variant: PAButtonVariant.danger, onPressed: () {}),
            const SizedBox(width: 8),
            PAButton(label: '取消', variant: PAButtonVariant.neutral, onPressed: () {}),
          ],
        ),
      ),
    );

    final FilledButton primary =
        tester.widget<FilledButton>(find.byType(FilledButton));
    expect(
      primary.style?.backgroundColor?.resolve(const <WidgetState>{}),
      PAColors.dark.brand,
    );
    expect(
      primary.style?.backgroundColor?.resolve(const <WidgetState>{}),
      isNot(PAColors.dark.danger),
      reason: '主按钮永不红',
    );

    final List<OutlinedButton> outlined =
        tester.widgetList<OutlinedButton>(find.byType(OutlinedButton)).toList();
    expect(outlined, hasLength(2));

    final OutlinedButton danger = tester.widget<OutlinedButton>(
      find.widgetWithText(OutlinedButton, '拒绝'),
    );
    expect(danger.style?.side?.resolve(const <WidgetState>{})?.color,
        PAColors.dark.danger);
    expect(danger.style?.foregroundColor?.resolve(const <WidgetState>{}),
        PAColors.dark.danger);

    final OutlinedButton neutral = tester.widget<OutlinedButton>(
      find.widgetWithText(OutlinedButton, '取消'),
    );
    expect(neutral.style?.side?.resolve(const <WidgetState>{})?.color,
        PAColors.dark.hairline);
  });

  testWidgets('PAStatusChip 五态文案与配色映射', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const Wrap(
          children: <Widget>[
            PAStatusChip(status: PAStatusKind.running),
            PAStatusChip(status: PAStatusKind.pending),
            PAStatusChip(status: PAStatusKind.done),
            PAStatusChip(status: PAStatusKind.failed),
            PAStatusChip(status: PAStatusKind.warn),
          ],
        ),
      ),
    );

    expect(find.text('运行中'), findsOneWidget);
    expect(find.text('等待中'), findsOneWidget);
    expect(find.text('已完成'), findsOneWidget);
    expect(find.text('失败'), findsOneWidget);
    expect(find.text('注意'), findsOneWidget);
    expect(find.byIcon(Icons.check_rounded), findsOneWidget);
    expect(find.byIcon(Icons.close_rounded), findsOneWidget);
    expect(find.byIcon(Icons.warning_amber_rounded), findsOneWidget);

    const PAColors pa = PAColors.dark;
    expect(
      PAStatusChip.colorOf(PAStatusKind.running, pa),
      pa.brand,
    );
    expect(PAStatusChip.colorOf(PAStatusKind.pending, pa), pa.fgTertiary);
    expect(PAStatusChip.colorOf(PAStatusKind.done, pa), pa.ok);
    expect(PAStatusChip.colorOf(PAStatusKind.failed, pa), pa.danger);
    expect(PAStatusChip.colorOf(PAStatusKind.warn, pa), pa.warn);
  });

  testWidgets('PAToolCard 折叠：点卡头展开等宽输出体', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const PAToolCard(
          toolName: 'shell.exec',
          argsSummary: 'git status --short',
          duration: '1.2s',
          status: PAStatusKind.done,
          output: ' M lib/main.dart',
        ),
      ),
    );

    expect(find.text('shell.exec'), findsOneWidget);
    expect(find.text(' M lib/main.dart'), findsNothing);

    await tester.tap(find.text('shell.exec'));
    await tester.pump();

    final Text output = tester.widget<Text>(find.text(' M lib/main.dart'));
    expect(output.style?.fontFamily, 'monospace');
    expect(find.text('1.2s'), findsOneWidget);
  });

  testWidgets('PAApprovalSheet 常规：标题 + 风险徽标 + 等宽信息区 + 三按钮',
      (tester) async {
    await tester.pumpWidget(
      _wrap(
        PAApprovalSheet(
          title: '执行 shell 命令',
          riskLabel: '风险：中',
          info: const <String, String>{'工具': 'shell.exec'},
          onAllow: () {},
          onDeny: () {},
          onCancel: () {},
        ),
      ),
    );

    expect(find.text('执行 shell 命令'), findsOneWidget);
    expect(find.text('风险：中'), findsOneWidget);
    expect(find.text('shell.exec'), findsOneWidget);

    final FilledButton allow =
        tester.widget<FilledButton>(find.byType(FilledButton));
    expect(
      allow.style?.backgroundColor?.resolve(const <WidgetState>{}),
      PAColors.dark.brand,
    );
    expect(find.widgetWithText(OutlinedButton, '拒绝'), findsOneWidget);
    expect(find.widgetWithText(OutlinedButton, '取消'), findsOneWidget);
    expect(find.byIcon(Icons.warning_amber_rounded), findsNothing);
  });

  testWidgets('PAApprovalSheet 破坏性：⚠ + 粗边框 + 拒绝提到主按钮前',
      (tester) async {
    await tester.pumpWidget(
      _wrap(
        PAApprovalSheet(
          title: '删除构建产物',
          riskLabel: '破坏性操作',
          destructive: true,
          onAllow: () {},
          onDeny: () {},
          onCancel: () {},
        ),
      ),
    );

    expect(find.byIcon(Icons.warning_amber_rounded), findsOneWidget);

    final Container sheet = tester.widget<Container>(
      find.byWidgetPredicate((Widget w) {
        if (w is! Container) return false;
        final Decoration? d = w.decoration;
        return d is BoxDecoration && d.borderRadius == PARadii.sheetTopBR;
      }),
    );
    final BoxDecoration deco = sheet.decoration! as BoxDecoration;
    expect(deco.borderRadius, PARadii.sheetTopBR);
    final Border border = deco.border! as Border;
    expect(border.top.width, 2, reason: '破坏性变体用粗边框');
    expect(border.top.color, PAColors.dark.danger);

    // 按钮序反转：拒绝排在主按钮（允许）之前
    final Finder allow = find.byType(FilledButton);
    final Finder deny = find.widgetWithText(OutlinedButton, '拒绝');
    expect(allow, findsOneWidget);
    expect(deny, findsOneWidget);
    expect(
      tester.getTopLeft(deny).dy,
      lessThan(tester.getTopLeft(allow).dy),
      reason: '破坏性变体按钮序反转：拒绝在允许之前',
    );
  });
}
