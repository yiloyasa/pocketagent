import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'onboarding.dart';
import 'settings_connection.dart';

/// 设置页：分组列表（连接与配对 / 外观 / 语音 / 关于），全部走 [PACard]。
///
/// 「连接与配对」是 B3 第 2 步①：真消费 B2 [PaBridge]（快照 + 事件 + pairCode/
/// connect，见 settings_connection.dart）；其余三分组仍是静态骨架，除深浅色切换
/// 与跳转外不接任何通道。
class SettingsPage extends StatelessWidget {
  const SettingsPage({
    super.key,
    required this.brightness,
    required this.onBrightnessChanged,
    this.bridge,
  });

  final PABrightness brightness;
  final ValueChanged<PABrightness> onBrightnessChanged;

  /// 通道注入点（测试用）；不传即用生产默认通道 `dev.pocketagent/bridge`。
  final PaBridge? bridge;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        _Group(
          title: '连接与配对',
          children: <Widget>[ConnectionSection(bridge: bridge)],
        ),
        _Group(
          title: '外观',
          children: <Widget>[
            _SettingsRow(
              icon: Icons.dark_mode_rounded,
              title: '深色模式',
              trailing: Switch(
                value: brightness == PABrightness.dark,
                onChanged: (bool v) => onBrightnessChanged(
                  v ? PABrightness.dark : PABrightness.light,
                ),
              ),
            ),
            _SettingsRow(
              icon: Icons.palette_outlined,
              title: '主题色',
              value: '品牌蓝',
              trailing: Container(
                width: 18,
                height: 18,
                decoration: BoxDecoration(
                  color: pa.brand,
                  borderRadius: BorderRadius.circular(5),
                ),
              ),
            ),
          ],
        ),
        _Group(
          title: '语音',
          children: <Widget>[
            _SettingsRow(
              icon: Icons.mic_rounded,
              title: '语音输入',
              value: '已开启',
            ),
            _SettingsRow(
              icon: Icons.tune_rounded,
              title: '识别灵敏度',
              value: '中',
            ),
            _SettingsRow(
              icon: Icons.graphic_eq_rounded,
              title: '离线识别模型',
              value: 'SenseVoice',
            ),
          ],
        ),
        _Group(
          title: '关于',
          children: <Widget>[
            _SettingsRow(
              icon: Icons.info_outline_rounded,
              title: '版本',
              value: '0.4.0 · versionCode 8',
            ),
            _SettingsRow(
              icon: Icons.school_rounded,
              title: '新手引导',
              onTap: () {
                Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (BuildContext context) => const OnboardingPage(),
                  ),
                );
              },
            ),
            _SettingsRow(
              icon: Icons.system_update_alt_rounded,
              title: '检查更新',
            ),
            _SettingsRow(
              icon: Icons.code_rounded,
              title: '开源许可',
            ),
          ],
        ),
      ],
    );
  }
}

class _Group extends StatelessWidget {
  const _Group({required this.title, required this.children});

  final String title;

  /// 组内子块（行 / 自定义块，如 [ConnectionSection]），块间以左缩进分隔线隔开。
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 22),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Padding(
            padding: const EdgeInsets.only(left: 4, bottom: 8),
            child: Text(
              title,
              style: PATypography.caption.copyWith(
                color: pa.fgTertiary,
                fontWeight: FontWeight.w600,
                letterSpacing: 0.4,
              ),
            ),
          ),
          PACard(
            padding: EdgeInsets.zero,
            child: Column(
              children: <Widget>[
                for (int i = 0; i < children.length; i++) ...<Widget>[
                  if (i > 0)
                    const Padding(
                      padding: EdgeInsets.only(left: 58),
                      child: Divider(),
                    ),
                  children[i],
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SettingsRow extends StatelessWidget {
  const _SettingsRow({
    required this.icon,
    required this.title,
    this.value,
    this.trailing,
    this.onTap,
  });

  final IconData icon;
  final String title;
  final String? value;
  final Widget? trailing;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final Widget? trailingWidget = trailing ??
        (onTap == null
            ? null
            : Icon(Icons.chevron_right_rounded, size: 20, color: pa.fgTertiary));

    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        child: Row(
          children: <Widget>[
            Container(
              width: 32,
              height: 32,
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: pa.bgSurface2,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(icon, size: 17, color: pa.fgSecondary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                title,
                style: PATypography.body.copyWith(color: pa.fgPrimary),
              ),
            ),
            if (value != null && value!.isNotEmpty) ...<Widget>[
              const SizedBox(width: 8),
              Flexible(
                child: Text(
                  value!,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ),
            ],
            if (trailingWidget != null) ...<Widget>[
              const SizedBox(width: 8),
              trailingWidget,
            ],
          ],
        ),
      ),
    );
  }
}
