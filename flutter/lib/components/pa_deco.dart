import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/radii.dart';
import '../theme/typography.dart';

/// 组件层内部共享装饰配方：设计 token（[PAColors] / [PARadii] / [PATypography]）
/// 到装配值的最后一层收敛，消除五组件间重复的 surface/hairline/徽标装饰字面量。
///
/// - 只**组合**既有 token，不引入任何新颜色、新圆角、新字号；
/// - 每个配方与各组件收敛前手写的装饰**逐字段等价**（`pa_deco_test.dart`
///   逐项钉死），组件 widget 树结构不变，渲染逐像素等价；
/// - 业务页面不直接使用，走 PACard / PAButton / PAStatusChip 等对外组件。
abstract final class PADeco {
  /// 卡片 / 面板通用装饰：底色（默认 `bg.surface`）+ 1px `border.hairline`
  /// 边 + 圆角（默认卡片 14）。覆盖 [borderColor] / [borderWidth] 可表达
  /// 破坏性粗边框等变体（审批卡破坏性 = danger 2px）。
  static BoxDecoration card({
    required PAColors pa,
    Color? color,
    Color? borderColor,
    double borderWidth = 1,
    BorderRadius radius = PARadii.cardBR,
  }) =>
      BoxDecoration(
        color: color ?? pa.bgSurface,
        borderRadius: radius,
        border: Border.all(
          color: borderColor ?? pa.hairline,
          width: borderWidth,
        ),
      );

  /// 等宽代码 / 信息块装饰：`bg.code` 底 + hairline + 按钮 8 圆角。
  /// 工具卡输出体与审批卡信息区共用。
  static BoxDecoration codeBlock(PAColors pa) =>
      card(pa: pa, color: pa.bgCode, radius: PARadii.buttonBR);

  /// 状态 / 风险徽标底：语义色 14% 透明 + 头像 10 圆角，无边框。
  /// 状态 chip 与审批卡风险徽标共用同一「语义色徽标」视觉。
  static BoxDecoration pill(Color accent) => BoxDecoration(
        color: accent.withValues(alpha: 0.14),
        borderRadius: PARadii.avatarBR,
      );

  /// 徽标文字：辅助 12 + 半粗（状态 chip / 风险徽标 / 重复计数徽标共用），
  /// 颜色由调用处按语义色叠加。
  static TextStyle badgeLabel(Color color) => PATypography.caption.copyWith(
        color: color,
        fontWeight: FontWeight.w600,
      );
}
