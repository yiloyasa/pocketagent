import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/radii.dart';

/// 飞书式大圆角卡片：`surface` 底 + 1px `hairline` 边 + 14 圆角。
///
/// 静态展示传 [child] 即可；可点击时传 [onTap]（带水波纹，裁到圆角内）。
class PACard extends StatelessWidget {
  const PACard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.color,
    this.borderColor,
    this.borderWidth = 1,
    this.radius,
    this.onTap,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;

  /// 覆盖背景色（默认 `bg.surface`）。
  final Color? color;

  /// 覆盖边框色（默认 `border.hairline`）。
  final Color? borderColor;

  final double borderWidth;

  /// 覆盖圆角（默认 [PARadii.card]）。
  final double? radius;

  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final BorderRadius br =
        BorderRadius.circular(radius ?? PARadii.card);
    final BoxDecoration decoration = BoxDecoration(
      color: color ?? pa.bgSurface,
      borderRadius: br,
      border: Border.all(
        color: borderColor ?? pa.hairline,
        width: borderWidth,
      ),
    );
    final Widget content = Padding(padding: padding, child: child);

    if (onTap == null) {
      return Container(decoration: decoration, child: content);
    }
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: br,
        child: Container(decoration: decoration, child: content),
      ),
    );
  }
}
