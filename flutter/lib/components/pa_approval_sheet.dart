import 'package:flutter/material.dart';

import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/radii.dart';
import '../theme/typography.dart';
import 'pa_button.dart';

/// 底部弹层审批卡（方案 §二「审批卡」）。
///
/// 结构：标题 + 风险徽标区 + 信息区（等宽）+ 底部按钮区。
/// 按钮约定：允许 = 品牌蓝主按钮、拒绝 = 红描边、取消 = 中性；
/// **破坏性变体**换布局：粗边框 + ⚠ + 按钮序反转（拒绝提到主按钮前）。
/// 红线：主按钮永不红——`onAllow` 永远是 primary 蓝。
class PAApprovalSheet extends StatelessWidget {
  const PAApprovalSheet({
    super.key,
    required this.title,
    required this.riskLabel,
    required this.onAllow,
    required this.onDeny,
    this.destructive = false,
    this.detail,
    this.info = const <String, String>{},
    this.allowLabel = '允许',
    this.denyLabel = '拒绝',
    this.onCancel,
  });

  final String title;

  /// 风险徽标文案，如 `风险：中` / `破坏性操作`。
  final String riskLabel;

  /// 破坏性变体：粗边框 + ⚠ + 按钮序反转 + 二次确认由上层再弹一次。
  final bool destructive;

  /// 标题下的一句说明（可空）。
  final String? detail;

  /// 信息区：label -> 值（等宽展示，保序）。
  final Map<String, String> info;

  final String allowLabel;
  final String denyLabel;

  final VoidCallback onAllow;
  final VoidCallback onDeny;
  final VoidCallback? onCancel;

  /// 以模态底部弹层展示；返回 `true`=允许 / `false`=拒绝 / `null`=取消。
  static Future<bool?> show(BuildContext context, PAApprovalSheet sheet) {
    return showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (BuildContext context) => sheet,
    );
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final Color accent = destructive ? pa.danger : pa.brand;

    return Container(
      decoration: BoxDecoration(
        color: pa.bgSurface,
        borderRadius: PARadii.sheetTopBR,
        border: Border.all(
          color: destructive ? pa.danger : pa.hairline,
          // 破坏性变体 = 粗边框
          width: destructive ? 2 : 1,
        ),
      ),
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 10, 20, 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Center(
                child: Container(
                  width: 36,
                  height: 4,
                  decoration: BoxDecoration(
                    color: pa.hairline,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Row(
                children: <Widget>[
                  if (destructive) ...<Widget>[
                    Icon(
                      Icons.warning_amber_rounded,
                      size: 22,
                      color: pa.warn,
                    ),
                    const SizedBox(width: 8),
                  ],
                  Expanded(
                    child: Text(
                      title,
                      style: PATypography.titleL.copyWith(color: pa.fgPrimary),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              // 风险徽标区
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: accent.withValues(alpha: 0.14),
                  borderRadius: BorderRadius.circular(PARadii.avatar),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Icon(
                      destructive
                          ? Icons.block_rounded
                          : Icons.shield_outlined,
                      size: 14,
                      color: accent,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      riskLabel,
                      style: PATypography.caption.copyWith(
                        color: accent,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
              if (detail != null) ...<Widget>[
                const SizedBox(height: 10),
                Text(
                  detail!,
                  style: PATypography.bodySmall.copyWith(color: pa.fgSecondary),
                ),
              ],
              if (info.isNotEmpty) ...<Widget>[
                const SizedBox(height: 14),
                // 信息区（等宽）
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: pa.bgCode,
                    borderRadius: BorderRadius.circular(PARadii.button),
                    border: Border.all(color: pa.hairline, width: 1),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: info.entries
                        .map<Widget>(
                          (MapEntry<String, String> e) => Padding(
                            padding: const EdgeInsets.symmetric(vertical: 3),
                            child: Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: <Widget>[
                                SizedBox(
                                  width: 76,
                                  child: Text(
                                    e.key,
                                    style: PATypography.code
                                        .copyWith(color: pa.fgTertiary),
                                  ),
                                ),
                                Expanded(
                                  child: Text(
                                    e.value,
                                    style: PATypography.code
                                        .copyWith(color: pa.fgPrimary),
                                  ),
                                ),
                              ],
                            ),
                          ),
                        )
                        .toList(),
                  ),
                ),
              ],
              const SizedBox(height: 18),
              // 底部按钮区
              if (destructive) ...<Widget>[
                if (onCancel == null)
                  SizedBox(
                    width: double.infinity,
                    child: PAButton(
                      label: denyLabel,
                      variant: PAButtonVariant.danger,
                      dangerFill: true,
                      onPressed: onDeny,
                    ),
                  )
                else
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: PAButton(
                          label: denyLabel,
                          variant: PAButtonVariant.danger,
                          dangerFill: true,
                          onPressed: onDeny,
                        ),
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: PAButton(
                          label: '取消',
                          variant: PAButtonVariant.neutral,
                          onPressed: onCancel!,
                        ),
                      ),
                    ],
                  ),
                const SizedBox(height: 10),
                SizedBox(
                  width: double.infinity,
                  child: PAButton(
                    label: allowLabel,
                    onPressed: onAllow,
                  ),
                ),
              ] else ...<Widget>[
                SizedBox(
                  width: double.infinity,
                  child: PAButton(label: allowLabel, onPressed: onAllow),
                ),
                const SizedBox(height: 10),
                Row(
                  children: <Widget>[
                    Expanded(
                      child: PAButton(
                        label: denyLabel,
                        variant: PAButtonVariant.danger,
                        onPressed: onDeny,
                      ),
                    ),
                    if (onCancel != null) ...<Widget>[
                      const SizedBox(width: 10),
                      Expanded(
                        child: PAButton(
                          label: '取消',
                          variant: PAButtonVariant.neutral,
                          onPressed: onCancel!,
                        ),
                      ),
                    ],
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
