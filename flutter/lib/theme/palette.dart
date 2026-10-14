import 'package:flutter/material.dart';

/// 亮度态：暗色为主（默认），浅色同语义必做。
/// 依据 `docs/design/2026-10-14-flutter-rewrite-plan.md` §二。
enum PABrightness { dark, light }

/// 设计系统颜色 token。
///
/// 组件只允许经 [PAColors] 取色，禁止在业务代码里散写 hex。
/// 两套实例 [dark] / [light] 语义一一对应，测试经 [tokens] 做完整性校验。
@immutable
class PAColors {
  const PAColors({
    required this.bgBase,
    required this.bgSurface,
    required this.bgSurface2,
    required this.bgCode,
    required this.hairline,
    required this.fgPrimary,
    required this.fgSecondary,
    required this.fgTertiary,
    required this.brand,
    required this.agentAccent,
    required this.ok,
    required this.danger,
    required this.warn,
    required this.onBrand,
  });

  /// 暗色（默认态）：近黑底 + 表面梯度 + 1px hairline。
  static const PAColors dark = PAColors(
    bgBase: Color(0xFF0F1115),
    bgSurface: Color(0xFF1A1D24),
    bgSurface2: Color(0xFF23272F),
    bgCode: Color(0xFF0B0E13),
    hairline: Color(0xFF2C3038),
    fgPrimary: Color(0xFFE8EAED),
    fgSecondary: Color(0xFF9BA0A6),
    fgTertiary: Color(0xFF6B7078),
    brand: Color(0xFF4F7CFF),
    agentAccent: Color(0xFF8B5CF6),
    ok: Color(0xFF22C55E),
    danger: Color(0xFFF5484D),
    warn: Color(0xFFF5A524),
    onBrand: Color(0xFFFFFFFF),
  );

  /// 浅色：同一套语义，取保证对比度的对应值（品牌蓝加深以满足白字 4.5:1）。
  static const PAColors light = PAColors(
    bgBase: Color(0xFFF4F5F7),
    bgSurface: Color(0xFFFFFFFF),
    bgSurface2: Color(0xFFECEEF2),
    bgCode: Color(0xFFEEF1F6),
    hairline: Color(0xFFDFE2E8),
    fgPrimary: Color(0xFF14171C),
    fgSecondary: Color(0xFF5B616B),
    fgTertiary: Color(0xFF8B919B),
    brand: Color(0xFF2E5BE8),
    agentAccent: Color(0xFF7C3AED),
    ok: Color(0xFF16A34A),
    danger: Color(0xFFDC2626),
    warn: Color(0xFFD97706),
    onBrand: Color(0xFFFFFFFF),
  );

  final Color bgBase;
  final Color bgSurface;
  final Color bgSurface2;
  final Color bgCode;
  final Color hairline;
  final Color fgPrimary;
  final Color fgSecondary;
  final Color fgTertiary;
  final Color brand;
  final Color agentAccent;
  final Color ok;
  final Color danger;
  final Color warn;
  final Color onBrand;

  /// 方案 §二 的 13 个核心 token（键名 = 文档 token 名）。
  /// 不含 [onBrand]（按钮前景，随品牌色固定为白）。
  Map<String, Color> get tokens => <String, Color>{
        'bg.base': bgBase,
        'bg.surface': bgSurface,
        'bg.surface2': bgSurface2,
        'bg.code': bgCode,
        'border.hairline': hairline,
        'fg.primary': fgPrimary,
        'fg.secondary': fgSecondary,
        'fg.tertiary': fgTertiary,
        'brand.primary': brand,
        'agent.accent': agentAccent,
        'ok': ok,
        'danger': danger,
        'warn': warn,
      };

  /// 方案 §二 的核心 token 键名全集（测试用）。
  static const List<String> tokenKeys = <String>[
    'bg.base',
    'bg.surface',
    'bg.surface2',
    'bg.code',
    'border.hairline',
    'fg.primary',
    'fg.secondary',
    'fg.tertiary',
    'brand.primary',
    'agent.accent',
    'ok',
    'danger',
    'warn',
  ];

  static PAColors of(PABrightness brightness) => switch (brightness) {
        PABrightness.dark => dark,
        PABrightness.light => light,
      };
}
