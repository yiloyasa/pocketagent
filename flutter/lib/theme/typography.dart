import 'package:flutter/material.dart';

/// 字号 token（方案 §二）：标题 20/17 加粗、正文 15/14、辅助 12、代码 13 等宽。
///
/// 样式本身不带颜色，颜色由调用处按 [PAColors] 叠加（亮暗切换时才好换色）。
/// 代码体统一走等宽 [monoFont]；无字体资产时按平台 fallback。
abstract final class PATypography {
  /// 等宽字体族：Android/桌面命中系统 monospace，iOS 回退到上层字体。
  static const String monoFont = 'monospace';

  /// 一级标题 20 / 加粗。
  static const TextStyle titleL = TextStyle(
    fontSize: 20,
    height: 1.3,
    fontWeight: FontWeight.w700,
    letterSpacing: -0.2,
  );

  /// 二级标题 17 / 加粗。
  static const TextStyle titleM = TextStyle(
    fontSize: 17,
    height: 1.35,
    fontWeight: FontWeight.w700,
  );

  /// 正文 15。
  static const TextStyle body = TextStyle(
    fontSize: 15,
    height: 1.5,
    fontWeight: FontWeight.w400,
  );

  /// 次级正文 14。
  static const TextStyle bodySmall = TextStyle(
    fontSize: 14,
    height: 1.45,
    fontWeight: FontWeight.w400,
  );

  /// 正文强调（按钮文字 / 卡头标题 / 列表项标题）：正文 15 + 半粗。
  /// 主题按钮层与组件层共用同一出口，避免各处散写 `body + w600`。
  static TextStyle get bodyEmphasis =>
      body.copyWith(fontWeight: FontWeight.w600);

  /// 辅助信息 12。
  static const TextStyle caption = TextStyle(
    fontSize: 12,
    height: 1.4,
    fontWeight: FontWeight.w400,
  );

  /// 代码 / 参数 / 输出 13 等宽。
  static const TextStyle code = TextStyle(
    fontSize: 13,
    height: 1.5,
    fontFamily: monoFont,
  );
}
