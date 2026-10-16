import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'onboarding.dart';
import 'pa_page_kit.dart';
import 'settings_agent.dart';
import 'settings_capabilities.dart';
import 'settings_connection.dart';
import 'settings_console.dart';
import 'settings_downloads.dart';
import 'task_center_data.dart';

/// 深链/路由的 sub 参数归一化（纯函数，与 Android `normalizeSettingsSub`
/// SettingsScreen.kt:449 同语义）：合法二级页 id 原样返回；`userdata` 是拆页前的
/// 旧 id（任务中心下载卡的历史深链），兼容映射到 `downloads`；空串（含 null）与
/// 未知值返回 null = 停在一级页，**不猜落点**。
///
/// 【为什么未知值回一级页而不是报错】深链参数来自历史入口（旧 APK 的卡片、
/// 桌面端分享链），报错等于把「入口旧了」甩给用户；回一级页 = 用户仍能自助导航。
String? normalizeSettingsSub(String? sub) => switch (sub) {
      null || '' => null,
      'userdata' => 'downloads',
      'connection' || 'downloads' || 'agent' || 'capabilities' || 'console' =>
        sub,
      _ => null,
    };

/// 设置页：分组列表（连接与配对 / 外观 / Agent / 语音 / 运行时 / 关于），全部走
/// [PACard]。
///
/// 「连接与配对」是 B3 第 2 步①：真消费 B2 [PaBridge]（快照 + 事件 + pairCode/
/// connect，见 settings_connection.dart）；第②子步补齐 `sub=agent` /
/// `sub=capabilities` 两个二级页（settings_agent.dart / settings_capabilities.dart，
/// 与 Kotlin 相同：sub 是**页面内状态**，不走独立路由）；第③子步补齐
/// `sub=downloads` / `sub=console`（settings_downloads.dart /
/// settings_console.dart）——五个二级页自此全有真页，[_PendingSubPage] 退化为
/// 「将来新增 sub 忘接线」的兜底分支。其余分组仍是静态骨架，除深浅色切换与跳转外
/// 不接任何通道。
class SettingsPage extends StatefulWidget {
  const SettingsPage({
    super.key,
    required this.brightness,
    required this.onBrightnessChanged,
    this.bridge,
    this.sub,
    this.store,
  });

  final PABrightness brightness;
  final ValueChanged<PABrightness> onBrightnessChanged;

  /// 通道注入点（测试用）；不传即用生产默认通道 `dev.pocketagent/bridge`。
  /// 同时透传给两个子页（注入一次，全页一致）。
  final PaBridge? bridge;

  /// 深链 sub 参数（Kotlin `settings?sub={sub}` 的 Flutter 形态）：
  /// connection/downloads/agent/capabilities/console，旧值 userdata→downloads，
  /// 未知 → 一级页（[normalizeSettingsSub]）。
  final String? sub;

  /// 本地任务清单注入点（downloads 子页消费；与 TaskCenter 页共用同一个 app 级
  /// store 的等价物，测试传可控 store）。
  final TaskCenterStore? store;

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  /// 当前二级页（null = 一级页）——与 Kotlin `subPage` 同语义的页面内状态。
  String? _sub;

  /// 能力位快照（`getConnectionState` 的 `hostCapabilities`，Kotlin
  /// `AgentConnection.hostCapabilities` 的 Dart 形态）——只驱动一级页「运行时」
  /// 分组的可见性（`runtime.v1` 门控）。读失败/宿主缺席 = 无从判断 → 整组不出现
  /// （无位不显示，也不把「读不到」渲染成「没有这个功能」的错误横幅）。
  List<String> _capabilities = const <String>[];

  @override
  void initState() {
    super.initState();
    _sub = normalizeSettingsSub(widget.sub);
    _loadCapabilities();
  }

  Future<void> _loadCapabilities() async {
    try {
      final PaConnState conn =
          await (widget.bridge ?? PaBridge()).getConnectionState();
      if (!mounted) return;
      setState(() => _capabilities = conn.capabilities);
    } on PaBridgeException {
      // 门控读不到：保持空位清单 = 运行时组不出现。一级页其余分组不依赖它，
      // 不为此把整页拖进错误态（诚实：这一组现在不可判定，而不是「坏了」）。
    } catch (_) {
      // 兜底：页面任何路径都不许把未捕获异常甩给框架。
    }
  }

  @override
  void didUpdateWidget(SettingsPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.sub != widget.sub) {
      // 深链参数变化（同一实例被换 sub 重建）：重新归一化。归一化结果与当前
      // 一致（如 null → 未知值仍是 null）时保持用户已导航到的子页，不打断。
      final String? next = normalizeSettingsSub(widget.sub);
      if (next != _sub) {
        setState(() => _sub = next);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String? sub = _sub;
    if (sub == 'agent') {
      return SettingsAgentPage(
        bridge: widget.bridge,
        onBack: () => setState(() => _sub = null),
      );
    }
    if (sub == 'capabilities') {
      return SettingsCapabilitiesPage(
        source: null,
        onBack: () => setState(() => _sub = null),
      );
    }
    if (sub == 'downloads') {
      return SettingsDownloadsPage(
        store: widget.store,
        bridge: widget.bridge,
        onBack: () => setState(() => _sub = null),
      );
    }
    if (sub == 'console') {
      return SettingsConsolePage(
        bridge: widget.bridge,
        onBack: () => setState(() => _sub = null),
      );
    }
    if (sub != null && sub != 'connection') {
      // 兜底：normalizeSettingsSub 白名单里 connection 由一级页承接（见下），
      // 其余四个已知 sub 上面都已分派；这条分支只为「白名单加了新值却忘了在
      // 上面加分派」而存在 —— 静默落回一级页会让用户以为入口坏了，占位页把
      // 那条路变成一句明话。
      return _PendingSubPage(
        title: sub,
        onBack: () => setState(() => _sub = null),
      );
    }
    // sub == 'connection' 深链或 null：连接面在 B3-2 落法里就是一级页首组
    // （ConnectionSection），不再单开二级页——深链语义由一级页承接。

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaPageSection(
          title: '连接与配对',
          children: <Widget>[ConnectionSection(bridge: widget.bridge)],
        ),
        PaPageSection(
          title: '外观',
          children: <Widget>[
            _SettingsRow(
              icon: Icons.dark_mode_rounded,
              title: '深色模式',
              trailing: Switch(
                value: widget.brightness == PABrightness.dark,
                onChanged: (bool v) => widget.onBrightnessChanged(
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
        PaPageSection(
          // 组名取「Agent」而不是与入口行同名：组标题与行标题同名会让
          // 「点哪一行」在文本层面产生两个同名命中（Android 一级页同款命名）。
          title: 'Agent',
          children: <Widget>[
            _SettingsRow(
              icon: Icons.smart_toy_outlined,
              title: 'Agent 与能力',
              value: '能力位 · 原生设置门控',
              onTap: () => setState(() => _sub = 'agent'),
            ),
            _SettingsRow(
              icon: Icons.memory_rounded,
              title: '模型',
              value: '只读列表',
              onTap: () => setState(() => _sub = 'capabilities'),
            ),
          ],
        ),
        PaPageSection(
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
        // 运行时分组：`runtime.v1` 位门控（Kotlin SettingsScreen.kt:1648-1670
        // 同款纪律——位不点亮照旧不出现，无能力不显示假入口）。
        if (runtimeControlCapable(_capabilities))
          PaPageSection(
            title: '运行时',
            children: <Widget>[
              _SettingsRow(
                icon: Icons.dns_outlined,
                title: '电脑控制台',
                value: '只读 · 预算与运行时',
                onTap: () => setState(() => _sub = 'console'),
              ),
            ],
          ),
        PaPageSection(
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
            // 与 settings_connection / settings_agent 两页的 32×32 圆角图标块
            // 同款 —— 共享件见 pa_page_kit.dart（[PaIconBox]）。
            PaIconBox(icon: icon),
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

/// 兜底占位二级页：五个已知 sub（connection/downloads/agent/capabilities/console）
/// 在第③子步后全有真页，本类只剩「白名单加了新值却忘了分派」这一条路径
/// （见 build 末尾的兜底分支）。
///
/// 【为什么不做成空壳列表冒充已开通】诚实占位不假装（红线：不留假绿）——
/// 路由通了就说路由通了，内容没搬就说内容没搬。静默吞回一级页会让用户以为
/// 入口坏了；如实占位则明确告知「本波未实现」。
class _PendingSubPage extends StatelessWidget {
  const _PendingSubPage({required this.title, required this.onBack});

  final String title;
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaSubPageHeader(
          title: title,
          backKey: 'pa-pending-back',
          onBack: onBack,
        ),
        PACard(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 16),
          child: Text(
            // 不点名字段/接口细节：只说「还没搬过来」，用户能行动的是返回。
            '本页在 Flutter 迁移的后续波次实现：路由已就绪，内容尚未搬入。',
            key: const ValueKey<String>('pa-pending-body'),
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ),
      ],
    );
  }
}
