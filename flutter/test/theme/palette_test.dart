import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/theme/palette.dart';

void main() {
  test('暗色 13 个核心 token 齐全，且值与设计方案一致', () {
    final Map<String, Color> dark = PAColors.dark.tokens;

    expect(dark.keys, unorderedEquals(PAColors.tokenKeys));
    expect(dark, hasLength(PAColors.tokenKeys.length));

    expect(dark['bg.base'], const Color(0xFF0F1115));
    expect(dark['bg.surface'], const Color(0xFF1A1D24));
    expect(dark['bg.surface2'], const Color(0xFF23272F));
    expect(dark['bg.code'], const Color(0xFF0B0E13));
    expect(dark['border.hairline'], const Color(0xFF2C3038));
    expect(dark['fg.primary'], const Color(0xFFE8EAED));
    expect(dark['fg.secondary'], const Color(0xFF9BA0A6));
    expect(dark['fg.tertiary'], const Color(0xFF6B7078));
    expect(dark['brand.primary'], const Color(0xFF4F7CFF));
    expect(dark['agent.accent'], const Color(0xFF8B5CF6));
    expect(dark['ok'], const Color(0xFF22C55E));
    expect(dark['danger'], const Color(0xFFF5484D));
    expect(dark['warn'], const Color(0xFFF5A524));
  });

  test('浅色与暗色 token 键集一致，且各自无重复色值', () {
    final Map<String, Color> light = PAColors.light.tokens;
    expect(light.keys, unorderedEquals(PAColors.tokenKeys));

    expect(
      PAColors.dark.tokens.values.toSet(),
      hasLength(PAColors.tokenKeys.length),
      reason: '暗色 token 出现重复值，表面梯度会被压平',
    );
    expect(
      PAColors.light.tokens.values.toSet(),
      hasLength(PAColors.tokenKeys.length),
      reason: '浅色 token 出现重复值，表面梯度会被压平',
    );
  });

  test('状态语义色互不相同，且亮暗两套取值不同（浅色为可读性加深）', () {
    const PAColors dark = PAColors.dark;
    const PAColors light = PAColors.light;

    expect(dark.ok, isNot(dark.danger));
    expect(dark.ok, isNot(dark.warn));
    expect(dark.danger, isNot(dark.warn));
    expect(dark.brand, isNot(dark.agentAccent));

    // 品牌蓝浅色必须比暗色更深（白字对比度 4.5:1）。
    expect(
      light.brand.toARGB32(),
      isNot(dark.brand.toARGB32()),
    );
    expect(light.bgBase.toARGB32(), isNot(dark.bgBase.toARGB32()));

    // 主按钮前景恒为白，红不进主按钮。
    expect(dark.onBrand, Colors.white);
    expect(light.onBrand, Colors.white);
  });
}
