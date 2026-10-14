// 设置页「Agent 与能力」二级页 —— B3 第 2 步②：能力位总览 + agent_native_settings 门控。
//
// 【为什么单独一页】page-contracts §1.8 把 Settings 定为 XL，第②子步就是「模型与
// 能力契约的消费者」：agent 子页读 `HostInfo.capabilities` 做能力位总览与门控，
// capabilities 子页（settings_capabilities.dart）做模型列表骨架。本页只做「读」。
//
// 【位分组不硬编码】位族全集见 docs/2026-09-30-agent-capability-matrix.md §二，
// 但代码**不抄那份清单**——按 `<前缀>:` 自然分组、未知前缀原样成为组名，daemon
// 加新位族时旧 APK 照常展示（未知输入透传红线；抄清单 = 第二份写死名单）。
//
// 【诚实红线】Kotlin 宿主桥 B4 波次才注册 handler（channel-contract.md §6）：
// 在此之前 getHostInfo 必抛 HOST_UNAVAILABLE → 本页如实显示「桌面端桥接未就绪
// （B4 波次）」+ 重试，绝不伪造一份能力清单（不留假绿）。
//
// 【只展示与门控，不做编辑】原生设置 = 用户桌面配置（~/.pi / ~/.dsh 等），手机端
// 改桌面配置是红线：`agent_native_settings:<id>` 位上只亮「原生设置」入口与说明，
// 真编辑面等后续波次拍板后才做 —— 本文件没有任何写通道。

import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../gen/pocketagent/v1/host.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'settings_connection.dart';

/// 一个能力位族：`prefix` = 位在首个 `:` 之前的前缀（无 `:` = 宿主级，`''`）。
@immutable
class CapabilityBitGroup {
  const CapabilityBitGroup({required this.prefix, required this.bits});

  final String prefix;
  final List<String> bits;
}

/// 能力位分组（纯函数）：按 `<前缀>:` 自然分组，**组序 = 首现顺序、组内保持原序**
/// （不另造排序规则）；无 `:` 的位（terminal / fs / git / agent / event_sequence /
/// runtime.v1）归到 prefix `''` 的宿主级组。不认识的前缀原样成为组名 —— daemon
/// 将来加新位族，旧 APK 不需要改码。
List<CapabilityBitGroup> groupCapabilityBits(List<String> capabilities) {
  final Map<String, List<String>> byPrefix = <String, List<String>>{};
  for (final String bit in capabilities) {
    final int sep = bit.indexOf(':');
    final String prefix = sep < 0 ? '' : bit.substring(0, sep);
    byPrefix.putIfAbsent(prefix, () => <String>[]).add(bit);
  }
  return byPrefix.entries
      .map(
        (MapEntry<String, List<String>> e) => CapabilityBitGroup(
          prefix: e.key,
          bits: List<String>.unmodifiable(e.value),
        ),
      )
      .toList(growable: false);
}

/// 组名展示（纯函数）：空前缀 = 宿主级；其余前缀**原样**（未知前缀不改写、
/// 不显「未知」，未知输入透传红线）。
String capabilityGroupLabel(String prefix) =>
    prefix.isEmpty ? '宿主级' : prefix;

/// 从能力位提取「有原生设置目录」的 agent id（纯函数）：扫
/// `agent_native_settings:<id>` 族，id 取 `:` 之后**原文**（透传不加工），按首现
/// 顺序去重；id 为空的位只进总览、不生成入口（入口需要可指认的目标）。
///
/// 【为什么从位里提取而不是写 agent 名单】位是 daemon 按接口断言逐家宣告的
/// 真值（host.go:106-108）；写死 codex/pi/... 会变成第二份名单，新 agent 撞车。
List<String> nativeSettingsAgentIds(List<String> capabilities) {
  const String family = 'agent_native_settings';
  final List<String> ids = <String>[];
  for (final String bit in capabilities) {
    if (!bit.startsWith('$family:')) continue;
    final String id = bit.substring(family.length + 1);
    if (id.isEmpty || ids.contains(id)) continue;
    ids.add(id);
  }
  return ids;
}

/// Agent 子页错误文案：与 connection 子页同一套 code→文案映射
/// （[paBridgeErrorText]，settings_connection.dart），只把 HOST_UNAVAILABLE 的
/// 收尾按本页语境改词 —— 该码点名的是整条 Kotlin 宿主桥未就绪，不只是连接功能。
String agentPageErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端桥接未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '能力位读取暂不可用。';
  }
  return paBridgeErrorText(e);
}

/// 「Agent 与能力」子页：能力位总览（按前缀自然分组）+ 原生设置门控入口。
///
/// 数据 = `PABridge.getHostInfo()` 的一次性快照 + 重试（不订阅事件面的原因：本步
/// 验收的是位读取与门控，快照已闭环；`host_info` 事件增量在事件消费统一波次接
/// 入，届时快照仍作初值）。
class SettingsAgentPage extends StatefulWidget {
  const SettingsAgentPage({super.key, this.bridge, required this.onBack});

  /// 通道注入点（测试用）；不传即用生产默认通道 `dev.pocketagent/bridge`。
  final PaBridge? bridge;

  /// 返回一级页（Kotlin SubPageScaffold 的 onBack）。
  final VoidCallback onBack;

  @override
  State<SettingsAgentPage> createState() => _SettingsAgentPageState();
}

class _SettingsAgentPageState extends State<SettingsAgentPage> {
  late final PaBridge _bridge;

  bool _loading = true;
  HostInfo? _host;
  String? _errorText;

  /// 点过哪个原生设置入口（只驱动说明行，不驱动任何写入 —— 见文件头「只展示与
  /// 门控」）；null = 未点过。
  String? _nativeNoticeId;

  @override
  void initState() {
    super.initState();
    _bridge = widget.bridge ?? PaBridge();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _errorText = null;
    });
    try {
      final HostInfo info = await _bridge.getHostInfo();
      if (!mounted) return;
      setState(() {
        _host = info;
        _loading = false;
      });
    } on PaBridgeException catch (e) {
      // 读失败就不保留任何旧清单冒充本次结果（不编造、不缓存冒充）。
      if (!mounted) return;
      setState(() {
        _host = null;
        _loading = false;
        _errorText = agentPageErrorText(e);
      });
    } catch (e) {
      // 兜底：页面任何路径都不许把未捕获异常甩给框架（既有 widget 测试会红）。
      if (!mounted) return;
      setState(() {
        _host = null;
        _loading = false;
        _errorText = '读取主机能力失败：$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final List<String> caps = _host?.capabilities ?? const <String>[];
    final List<String> nativeIds = nativeSettingsAgentIds(caps);
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        _SubHeader(
          title: 'Agent 与能力',
          backKey: 'pa-agent-back',
          onBack: widget.onBack,
        ),
        _Section(
          title: '能力位总览',
          children: <Widget>[_buildCapsBody(context, caps)],
        ),
        // 原生设置分组：位不在 = 整组隐藏（Android NativeConfigOptionsSection
        // 同款纪律：缺位入口隐藏，不摆假开关）；宿主未就绪也从缺。
        if (nativeIds.isNotEmpty)
          _Section(
            title: '原生设置',
            children: <Widget>[
              for (final String id in nativeIds) _nativeEntryRow(context, id),
            ],
          ),
      ],
    );
  }

  // ── 能力位总览 ──────────────────────────────────────────────────────────

  Widget _buildCapsBody(BuildContext context, List<String> caps) {
    final PAColors pa = context.pa;
    if (_loading) {
      return _caption(
        context,
        '正在读取主机能力…',
        key: const ValueKey<String>('pa-agent-loading'),
      );
    }
    if (_errorText != null) {
      // 诚实错误态 + 重试，与 connection 子页同款。
      return Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(
              _errorText!,
              key: const ValueKey<String>('pa-agent-error'),
              style: PATypography.caption.copyWith(color: pa.danger),
            ),
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                key: const ValueKey<String>('pa-agent-retry'),
                onPressed: _load,
                child: Text(
                  '重试',
                  style: PATypography.caption.copyWith(
                    color: pa.brand,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
          ],
        ),
      );
    }
    final HostInfo info = _host!;
    final String hostLine = _hostLine(info);
    final List<Widget> blocks = <Widget>[
      if (hostLine.isNotEmpty)
        _caption(
          context,
          hostLine,
          key: const ValueKey<String>('pa-agent-host-line'),
        ),
    ];
    if (caps.isEmpty) {
      // 读成功但确实一个位都没有（旧 daemon / 空注册面）：如实说"未报告"，
      // 不冒充"没有能力"。
      blocks.add(
        _caption(
          context,
          '电脑端未报告任何能力位。',
          key: const ValueKey<String>('pa-agent-empty'),
        ),
      );
    } else {
      final List<CapabilityBitGroup> groups = groupCapabilityBits(caps);
      for (final CapabilityBitGroup g in groups) {
        blocks.add(
          Padding(
            padding: const EdgeInsets.fromLTRB(14, 8, 14, 6),
            child: Text(
              // 组名 + 位数（先例：Android capabilityKindLabel + " · " + count）。
              '${capabilityGroupLabel(g.prefix)} · ${g.bits.length}',
              style: PATypography.caption.copyWith(
                color: pa.fgTertiary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        );
        blocks.add(
          Padding(
            padding: const EdgeInsets.fromLTRB(14, 0, 14, 10),
            child: Wrap(
              spacing: 6,
              runSpacing: 6,
              // 位原文进 chip（不改写、不缩写）——chip 样式走设计系统 chipTheme，
              // 不新造视觉。
              children: <Widget>[
                for (final String bit in g.bits) Chip(label: Text(bit)),
              ],
            ),
          ),
        );
      }
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: blocks,
    );
  }

  /// 主机身份一行（拿真值拼，空字段不占位）——能力位出自哪台桌面，一眼可辨。
  String _hostLine(HostInfo info) {
    final List<String> parts = <String>[];
    if (info.displayName.isNotEmpty) parts.add(info.displayName);
    if (info.daemonVersion.isNotEmpty) parts.add('daemon ${info.daemonVersion}');
    if (info.hostId.isNotEmpty) parts.add('主机 ${info.hostId}');
    return parts.join(' · ');
  }

  // ── 原生设置门控入口 ────────────────────────────────────────────────────

  Widget _nativeEntryRow(BuildContext context, String agentId) {
    final PAColors pa = context.pa;
    return InkWell(
      key: ValueKey<String>('pa-agent-native-$agentId'),
      onTap: () => setState(() => _nativeNoticeId = agentId),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                const _IconBox(icon: Icons.tune_rounded),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text(
                        '原生设置 · $agentId',
                        style: PATypography.body.copyWith(color: pa.fgPrimary),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        '桌面端原生配置目录（只读展示；编辑面待后续波次）',
                        style: PATypography.caption
                            .copyWith(color: pa.fgTertiary),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.info_outline_rounded,
                  size: 18,
                  color: pa.fgTertiary,
                ),
              ],
            ),
            if (_nativeNoticeId == agentId) ...<Widget>[
              const SizedBox(height: 8),
              Text(
                // 手机端改桌面配置是红线：本波只做展示与门控，写通道一行都不写。
                '手机端改桌面配置是红线：本波「原生设置」只做展示与门控，'
                '真编辑面在后续波次拍板后开放。',
                key: ValueKey<String>('pa-agent-native-notice-$agentId'),
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _caption(
    BuildContext context,
    String text, {
    Key? key,
  }) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 10),
      child: Text(
        text,
        key: key,
        style: PATypography.caption.copyWith(color: pa.fgTertiary),
      ),
    );
  }
}

/// 二级页头部：返回箭头 + 标题（Android SubPageScaffold 的最小对应物）。
///
/// 【为什么在文件内私有复制而非抽公共件】与 settings_connection.dart 的
/// `_InsetDivider`/`_IconBox` 同一先例：本波约束下不动 components/ 层，同为
/// 「与 settings.dart `_Group` 同款」的局部样式副本。
class _SubHeader extends StatelessWidget {
  const _SubHeader({
    required this.title,
    required this.backKey,
    required this.onBack,
  });

  final String title;
  final String backKey;
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Row(
        children: <Widget>[
          IconButton(
            key: ValueKey<String>(backKey),
            onPressed: onBack,
            tooltip: '返回',
            icon: Icon(Icons.arrow_back_rounded, color: pa.fgSecondary),
          ),
          const SizedBox(width: 2),
          Expanded(
            child: Text(
              title,
              style: PATypography.titleM.copyWith(color: pa.fgPrimary),
            ),
          ),
        ],
      ),
    );
  }
}

/// 与 settings.dart `_Group` 同款的分组块（标题 + PACard + 左缩进分隔线）。
class _Section extends StatelessWidget {
  const _Section({required this.title, required this.children});

  final String title;
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

/// 与 settings.dart `_SettingsRow` 同款的 32×32 圆角图标块。
class _IconBox extends StatelessWidget {
  const _IconBox({required this.icon});

  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Container(
      width: 32,
      height: 32,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: pa.bgSurface2,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Icon(icon, size: 17, color: pa.fgSecondary),
    );
  }
}
