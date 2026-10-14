import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_deco.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/radii.dart';

/// 装饰配方钉死测试：每个配方与收敛前各组件手写的装饰逐字段等价，
/// 防止后续改配方时悄悄挪动渲染（组件级渲染断言另见 components_test.dart）。
void main() {
  test('card 配方默认值 = surface 底 + hairline 1px + 卡片 14 圆角', () {
    const PAColors pa = PAColors.dark;
    final BoxDecoration deco = PADeco.card(pa: pa);

    expect(deco.color, pa.bgSurface);
    expect(deco.borderRadius, PARadii.cardBR);
    final Border border = deco.border! as Border;
    expect(border.top.width, 1);
    expect(border.top.color, pa.hairline);
  });

  test('card 配方可覆盖边框：审批卡破坏性变体 = danger 2px', () {
    const PAColors pa = PAColors.dark;
    final BoxDecoration deco = PADeco.card(
      pa: pa,
      radius: PARadii.sheetTopBR,
      borderColor: pa.danger,
      borderWidth: 2,
    );

    expect(deco.borderRadius, PARadii.sheetTopBR);
    final Border border = deco.border! as Border;
    expect(border.top.width, 2);
    expect(border.top.color, pa.danger);
  });

  test('codeBlock 配方 = bg.code 底 + hairline + 8 圆角（工具卡/审批卡共用）', () {
    const PAColors pa = PAColors.dark;
    final BoxDecoration deco = PADeco.codeBlock(pa);

    expect(deco.color, pa.bgCode);
    expect(deco.borderRadius, PARadii.buttonBR);
    final Border border = deco.border! as Border;
    expect(border.top.width, 1);
    expect(border.top.color, pa.hairline);
  });

  test('pill 配方 = 语义色 14% 底 + 头像 10 圆角，无边框', () {
    const PAColors pa = PAColors.dark;
    final BoxDecoration deco = PADeco.pill(pa.danger);

    expect(deco.color, pa.danger.withValues(alpha: 0.14));
    expect(deco.borderRadius, PARadii.avatarBR);
    expect(deco.border, isNull, reason: '徽标不描边');
  });

  test('badgeLabel 配方 = 辅助 12 + 半粗，颜色随语义色', () {
    const PAColors pa = PAColors.dark;
    final TextStyle style = PADeco.badgeLabel(pa.brand);

    expect(style.fontSize, 12);
    expect(style.fontWeight, FontWeight.w600);
    expect(style.color, pa.brand);
  });
}
