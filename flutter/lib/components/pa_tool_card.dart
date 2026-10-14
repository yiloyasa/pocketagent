import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/radii.dart';
import '../theme/typography.dart';
import 'pa_card.dart';
import 'pa_status_chip.dart';

/// 折叠工具卡（方案 §二「工具卡」）：
/// L1 卡头 = 工具名 + 参数摘要 + 耗时 + 状态；tap 展开输出体（等宽、截断）。
///
/// 连续同工具调用由上层合并后经 [repeatCount] 显示 `×N`，卡片本身不猜状态。
class PAToolCard extends StatefulWidget {
  const PAToolCard({
    super.key,
    required this.toolName,
    required this.argsSummary,
    required this.duration,
    required this.status,
    this.output = '',
    this.repeatCount = 1,
  });

  final String toolName;

  /// 参数摘要（单行截断）。
  final String argsSummary;

  /// 耗时文案，如 `1.2s` / `340ms`。
  final String duration;

  final PAStatusKind status;

  /// 工具输出体（等宽展示，超出截断）。
  final String output;

  /// 合并后的连续调用次数（1 = 未合并）。
  final int repeatCount;

  @override
  State<PAToolCard> createState() => _PAToolCardState();
}

class _PAToolCardState extends State<PAToolCard> {
  bool _expanded = false;

  void _toggle() => setState(() => _expanded = !_expanded);

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final bool hasOutput = widget.output.isNotEmpty;

    return PACard(
      padding: EdgeInsets.zero,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          InkWell(
            onTap: hasOutput ? _toggle : null,
            borderRadius:
                const BorderRadius.vertical(top: Radius.circular(PARadii.card)),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Icon(
                    _expanded
                        ? Icons.expand_more_rounded
                        : Icons.chevron_right_rounded,
                    size: 18,
                    color: pa.fgTertiary,
                  ),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        Row(
                          children: <Widget>[
                            Flexible(
                              child: Text(
                                widget.toolName,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: PATypography.body.copyWith(
                                  color: pa.fgPrimary,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ),
                            if (widget.repeatCount > 1) ...<Widget>[
                              const SizedBox(width: 6),
                              _RepeatBadge(count: widget.repeatCount),
                            ],
                            const SizedBox(width: 8),
                            PAStatusChip(status: widget.status),
                          ],
                        ),
                        const SizedBox(height: 2),
                        Text(
                          widget.argsSummary,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: PATypography.code.copyWith(
                            color: pa.fgSecondary,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      widget.duration,
                      style: PATypography.caption
                          .copyWith(color: pa.fgTertiary),
                    ),
                  ),
                ],
              ),
            ),
          ),
          if (_expanded && hasOutput)
            Container(
              width: double.infinity,
              margin: const EdgeInsets.fromLTRB(14, 0, 14, 14),
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: pa.bgCode,
                borderRadius: BorderRadius.circular(PARadii.button),
                border: Border.all(color: pa.hairline, width: 1),
              ),
              child: Text(
                widget.output,
                maxLines: 12,
                overflow: TextOverflow.ellipsis,
                style: PATypography.code.copyWith(color: pa.fgSecondary),
              ),
            ),
        ],
      ),
    );
  }
}

/// 连续调用合并计数徽标。
class _RepeatBadge extends StatelessWidget {
  const _RepeatBadge({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
      decoration: BoxDecoration(
        color: pa.bgSurface2,
        borderRadius: BorderRadius.circular(PARadii.button),
      ),
      child: Text(
        '×$count',
        style: PATypography.caption.copyWith(
          color: pa.fgSecondary,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
