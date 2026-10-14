import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/radii.dart';
import '../theme/typography.dart';

/// 三变体：primary 品牌蓝实心 / danger 红描边 / neutral 中性描边。
///
/// 设计红线：**主按钮（primary）永不红**——危险动作用 danger 描边表达，
/// 不把主操作染成红色。
enum PAButtonVariant { primary, danger, neutral }

class PAButton extends StatelessWidget {
  const PAButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.variant = PAButtonVariant.primary,
    this.icon,
    this.expanded = true,
    this.dangerFill = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final PAButtonVariant variant;
  final IconData? icon;

  /// 是否撑满父容器宽度（行内小按钮传 false）。
  final bool expanded;

  /// danger 变体是否填充底色（默认描边，仅破坏性二次确认强调时用）。
  final bool dangerFill;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final bool enabled = onPressed != null;

    final (Color bg, Color fg, Color border, double borderWidth) = switch (
      variant
    ) {
      PAButtonVariant.primary => (pa.brand, pa.onBrand, Colors.transparent, 0),
      PAButtonVariant.danger =>
        (dangerFill ? pa.danger : Colors.transparent, pa.danger, pa.danger, 1.5),
      PAButtonVariant.neutral =>
        (Colors.transparent, pa.fgPrimary, pa.hairline, 1),
    };

    final Widget labelWidget = Row(
      mainAxisSize: expanded ? MainAxisSize.max : MainAxisSize.min,
      mainAxisAlignment: MainAxisAlignment.center,
      children: <Widget>[
        if (icon != null) ...<Widget>[
          Icon(icon, size: 18, color: fg),
          const SizedBox(width: 6),
        ],
        Text(
          label,
          style: PATypography.body.copyWith(
            color: fg,
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );

    final ButtonStyle style = ButtonStyle(
      backgroundColor: WidgetStatePropertyAll<Color>(
        enabled ? bg : bg.withValues(alpha: 0.4),
      ),
      foregroundColor: WidgetStatePropertyAll<Color>(fg),
      overlayColor: WidgetStatePropertyAll<Color>(fg.withValues(alpha: 0.12)),
      side: WidgetStatePropertyAll<BorderSide>(
        borderWidth == 0
            ? BorderSide.none
            : BorderSide(color: border, width: borderWidth),
      ),
      minimumSize: const WidgetStatePropertyAll<Size>(Size(64, 44)),
      padding: const WidgetStatePropertyAll<EdgeInsets>(
        EdgeInsets.symmetric(horizontal: 18),
      ),
      shape: const WidgetStatePropertyAll<OutlinedBorder>(
        RoundedRectangleBorder(borderRadius: PARadii.buttonBR),
      ),
      textStyle: WidgetStatePropertyAll<TextStyle>(
        PATypography.body.copyWith(fontWeight: FontWeight.w600),
      ),
    );

    final Widget button = variant == PAButtonVariant.primary
        ? FilledButton(
            style: style,
            onPressed: onPressed,
            child: labelWidget,
          )
        : OutlinedButton(
            style: style,
            onPressed: onPressed,
            child: labelWidget,
          );

    if (!enabled) {
      return Opacity(opacity: 0.45, child: button);
    }
    return button;
  }
}
