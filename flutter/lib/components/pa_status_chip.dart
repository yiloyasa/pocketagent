import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import 'pa_deco.dart';

/// 五态状态 chip（方案 §二）：
/// running=brand · pending=grey · done=green ✓ · failed=red ✗ · warn=琥珀。
enum PAStatusKind { running, pending, done, failed, warn }

class PAStatusChip extends StatelessWidget {
  const PAStatusChip({super.key, required this.status, this.label});

  final PAStatusKind status;

  /// 覆盖默认文案（默认按 [status] 给中文标签）。
  final String? label;

  /// 状态主色（组件渲染与测试共用同一映射）。
  static Color colorOf(PAStatusKind status, PAColors pa) => switch (status) {
        PAStatusKind.running => pa.brand,
        PAStatusKind.pending => pa.fgTertiary,
        PAStatusKind.done => pa.ok,
        PAStatusKind.failed => pa.danger,
        PAStatusKind.warn => pa.warn,
      };

  /// 状态默认文案。
  static String labelOf(PAStatusKind status) => switch (status) {
        PAStatusKind.running => '运行中',
        PAStatusKind.pending => '等待中',
        PAStatusKind.done => '已完成',
        PAStatusKind.failed => '失败',
        PAStatusKind.warn => '注意',
      };

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final Color color = colorOf(status, pa);
    final String text = label ?? labelOf(status);

    return Container(
      height: 24,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      decoration: PADeco.pill(color),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          _StatusGlyph(status: status, color: color),
          const SizedBox(width: 4),
          Text(text, style: PADeco.badgeLabel(color)),
        ],
      ),
    );
  }
}

/// running/pending 用静态圆点（不做无限动画，避免测试 pumpAndSettle 卡死）。
class _StatusGlyph extends StatelessWidget {
  const _StatusGlyph({required this.status, required this.color});

  final PAStatusKind status;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final IconData? icon = switch (status) {
      PAStatusKind.done => Icons.check_rounded,
      PAStatusKind.failed => Icons.close_rounded,
      PAStatusKind.warn => Icons.warning_amber_rounded,
      PAStatusKind.running || PAStatusKind.pending => null,
    };
    if (icon != null) {
      return Icon(icon, size: 14, color: color);
    }
    return Container(
      width: 7,
      height: 7,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
    );
  }
}
