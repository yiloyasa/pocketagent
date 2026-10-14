// 设置页「连接与配对」分组 —— B3 第 2 步①：B2 transport 真通道的第一个消费方。
//
// 【为什么这一步先做连接子步】page-contracts §1.8 把 Settings 定为 XL，第①子步
// 就是「一级页 + connection（B2 配对/连接流的消费者）」：它同时验证快照
// （getConnectionState）、事件（conn_state 增量）、方法（pairCode/connect）三种
// 通道形态在 UI 上的落法，后面 agent/capabilities/downloads/console 都复用这套
// 错误码 → 文案的分支方式。
//
// 【诚实红线】Kotlin 宿主桥要到 B4 波次才注册 handler（channel-contract.md §6）。
// 在此之前 connect 必然抛 MissingPluginException → HOST_UNAVAILABLE，本页如实显示
// 「桌面端桥接未就绪（B4 波次）」，**绝不因为调用没炸就宣告连接成功**：connect 的
// null 返回按契约只是「请求受理」，成功与否以上方状态条（conn_state 真值）为准。

import 'dart:async';

import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';

/// 连接态三态文案：宿主 `connState` 的 `checking`/`ok`/`down` → 用户可读中文。
///
/// 【为什么未知值原样透传】`state` 的值域由宿主单方面定义（pa_bridge 注释：
/// Dart 枚举会变成第二份写死名单），未知档位直接把原值显示出来 —— 未知输入透传
/// 红线，UI 不许显「未知/错误」。
String connStateLabelOf(String state) => switch (state) {
      'checking' => '检查中',
      'ok' => '已连接',
      'down' => '已断开',
      _ => state,
    };

/// 错误码 → 用户可读文案（channel-contract.md §5 错误模型）。
///
/// 【为什么按 code 分支而不是按 message】message 是宿主人话原因（可能本地化、
/// 可能带细节），code 才是 UI 分支依据（重试 / 去配对 / 仅展示）；Dart 侧不改写
/// 宿主 code，所以表外的新码也必须能显示 —— 末支把 code 原样带给用户，宁可难看
/// 也不吞（吞掉就是把「宿主加了新错」伪装成「一切正常」）。
String paBridgeErrorText(PaBridgeException e) {
  final String code = e.code;
  if (code == PaBridgeErrorCodes.hostUnavailable) {
    // B4 前的常态：如实说明是哪一层没就绪，用户不会误以为是自己填错了。
    return '桌面端桥接未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '连接功能暂不可用。';
  }
  if (code == PaBridgeErrorCodes.invalidAddr) {
    return '连接地址无效：请填写 host:port（例如 192.168.1.20:7246）。';
  }
  if (code == PaBridgeErrorCodes.authRequired) {
    return '需要配对：token 缺失或已过期，请用配对码重新授权。';
  }
  if (code == PaBridgeErrorCodes.notConnected) {
    return '尚未连上 daemon：请先在桌面端确认连接后再试。';
  }
  if (code == PaBridgeErrorCodes.pairDisabled) {
    return '配对码不可读：仅本机回环来源可读，或桌面端未开启配对。';
  }
  if (code == PaBridgeErrorCodes.decodeFailed) {
    return '通道返回与契约不符（DECODE_FAILED）：${e.message}';
  }
  if (code == PaBridgeErrorCodes.internal) {
    return '桌面端内部错误：${e.message}';
  }
  // 表外 code：原样透传（契约规定宿主 code 不在表内时 Dart 保留，不改写）。
  return '$code：${e.message}';
}

/// 连接与配对分组的三块内容：状态条 / 配对码 / 手动连接表单。
///
/// 渲染成一列（块间用与 [_Group] 同款的左缩进分隔线），外层仍由 settings.dart
/// 的 `_Group` 包 PACard —— 保证与其余三分组同一张卡片质感。
class ConnectionSection extends StatefulWidget {
  const ConnectionSection({super.key, this.bridge});

  /// 通道注入点：测试传可 mock 的 [PaBridge]；生产不传即用默认
  /// `dev.pocketagent/bridge` 方法面 + 事件面。
  final PaBridge? bridge;

  @override
  State<ConnectionSection> createState() => _ConnectionSectionState();
}

class _ConnectionSectionState extends State<ConnectionSection> {
  late final PaBridge _bridge;
  late final TextEditingController _addrCtrl;
  late final TextEditingController _tokenCtrl;

  /// 状态条：快照初值（`getConnectionState`），事件（`conn_state`）只负责增量。
  bool _connLoading = true;
  PaConnState? _conn;
  String? _connErrorText;

  /// 配对码：显式点按才读（避免进页面就向宿主要敏感值）。
  bool _pairLoading = false;
  String? _pairCode;
  String? _pairErrorText;

  /// 手动连接：受理 ≠ 已连接（见文件头注释）。
  bool _connecting = false;
  String? _connectNotice;
  String? _connectErrorText;

  StreamSubscription<PaConnEvent>? _events;

  @override
  void initState() {
    super.initState();
    _bridge = widget.bridge ?? PaBridge();
    _addrCtrl = TextEditingController();
    _tokenCtrl = TextEditingController();
    _loadConnState();
  }

  @override
  void dispose() {
    _events?.cancel();
    _addrCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  /// 初值/重试：方法面快照。
  ///
  /// 【为什么先快照再订阅事件】事件是无缓冲 broadcast（订阅前推的不补），冷起时
  /// 只有快照能给出初值；同时快照成功 = 方法面探针证明宿主在场，才订阅事件面 ——
  /// 宿主缺席时订阅只会把 listen 握手错转投 FlutterError.reportError（框架日志），
  /// 契约本身也要求「判定宿主在不在走方法面，不能等事件流报错」。
  Future<void> _loadConnState() async {
    setState(() {
      _connLoading = true;
      _connErrorText = null;
    });
    try {
      final PaConnState state = await _bridge.getConnectionState();
      if (!mounted) return;
      setState(() {
        _conn = state;
        _connLoading = false;
      });
      _subscribeConnEvents();
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _conn = null;
        _connLoading = false;
        _connErrorText = paBridgeErrorText(e);
      });
    } catch (e) {
      // 兜底：页面任何路径都不许把未捕获异常甩给框架（既有 widget 测试会红）。
      if (!mounted) return;
      setState(() {
        _conn = null;
        _connLoading = false;
        _connErrorText = '读取连接状态失败：$e';
      });
    }
  }

  void _subscribeConnEvents() {
    if (_events != null) return;
    _events = _bridge.events.listen(
      (PaConnEvent event) {
        if (!mounted) return;
        if (event.type == PaConnEventType.connState &&
            event.connState != null) {
          setState(() => _conn = event.connState);
        }
      },
      onError: (Object error) {
        // 坏事件不毒化流（契约 §4）：状态条保留最近一次已知值，错误只做提示行，
        // 下一帧好事件仍会覆盖上来。
        if (!mounted || error is! PaBridgeException) return;
        setState(() => _connErrorText = paBridgeErrorText(error));
      },
    );
  }

  Future<void> _loadPairCode() async {
    setState(() {
      _pairLoading = true;
      _pairErrorText = null;
    });
    try {
      final String code = await _bridge.pairCode();
      if (!mounted) return;
      setState(() {
        _pairLoading = false;
        _pairCode = code;
      });
    } on PaBridgeException catch (e) {
      // 读失败就不显示任何码（不编造、不保留上次的码冒充本次结果）。
      if (!mounted) return;
      setState(() {
        _pairLoading = false;
        _pairCode = null;
        _pairErrorText = paBridgeErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _pairLoading = false;
        _pairCode = null;
        _pairErrorText = '读取配对码失败：$e';
      });
    }
  }

  Future<void> _connect() async {
    setState(() {
      _connecting = true;
      _connectNotice = null;
      _connectErrorText = null;
    });
    try {
      await _bridge.connect(_addrCtrl.text, _tokenCtrl.text);
      if (!mounted) return;
      setState(() {
        _connecting = false;
        // 契约 §3.2：返回 null = 请求受理，不是「已连上」——Kotlin 连接是异步的，
        // 真值看状态条。文案必须把这两件事分开，否则就是在替宿主宣告成功。
        _connectNotice = '请求已受理：是否连接成功以上方状态条为准（连接是异步的）。';
      });
      await _loadConnState();
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _connecting = false;
        _connectErrorText = paBridgeErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _connecting = false;
        _connectErrorText = '连接失败：$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        _buildStatusRow(context),
        const _InsetDivider(),
        _buildPairRow(context),
        if (_pairErrorText != null) _caption(context, _pairErrorText!, danger: true),
        const _InsetDivider(),
        _buildConnectForm(context),
      ],
    );
  }

  // ── 状态条 ──────────────────────────────────────────────────────────────

  Widget _buildStatusRow(BuildContext context) {
    final PAColors pa = context.pa;
    final String label;
    final Color labelColor;
    if (_connLoading) {
      label = '检查中';
      labelColor = pa.warn;
    } else if (_conn != null) {
      label = connStateLabelOf(_conn!.state);
      labelColor = switch (_conn!.state) {
        'ok' => pa.ok,
        'down' => pa.danger,
        _ => pa.fgSecondary,
      };
    } else {
      // 快照读不到（宿主缺席/契约漂移）：如实说读不到，不冒充「已连接」也不
      // 冒充「已断开」——断开是「连过又掉了」，我们并不知道。
      label = '状态不可读';
      labelColor = pa.danger;
    }

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              const _IconBox(icon: Icons.swap_horiz_rounded),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      '连接状态',
                      style: PATypography.body.copyWith(color: pa.fgPrimary),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      label,
                      key: const ValueKey<String>('pa-conn-state-label'),
                      style: PATypography.caption.copyWith(
                        color: labelColor,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
              ),
              if (_conn != null)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: <Widget>[
                    if (_conn!.transport.isNotEmpty)
                      Text(
                        _conn!.transport,
                        style: PATypography.caption
                            .copyWith(color: pa.fgTertiary),
                      ),
                    if (_conn!.address.isNotEmpty)
                      Text(
                        _conn!.address,
                        style: PATypography.caption
                            .copyWith(color: pa.fgTertiary),
                      ),
                  ],
                ),
            ],
          ),
          if (_connErrorText != null)
            _caption(context, _connErrorText!, danger: true),
          if (_connErrorText != null)
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                key: const ValueKey<String>('pa-conn-retry'),
                onPressed: _loadConnState,
                child: Text(
                  '重试',
                  style: PATypography.caption.copyWith(
                    color: context.pa.brand,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  // ── 配对码 ──────────────────────────────────────────────────────────────

  Widget _buildPairRow(BuildContext context) {
    final PAColors pa = context.pa;
    final String valueText;
    if (_pairLoading) {
      valueText = '读取中…';
    } else if (_pairCode != null) {
      valueText = _pairCode!;
    } else {
      valueText = '仅本机回环来源可读，点右侧读取';
    }
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Row(
        children: <Widget>[
          const _IconBox(icon: Icons.password_rounded),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  '配对码',
                  style: PATypography.body.copyWith(color: pa.fgPrimary),
                ),
                const SizedBox(height: 3),
                Text(
                  valueText,
                  key: const ValueKey<String>('pa-pair-code-value'),
                  style: PATypography.caption.copyWith(
                    color: _pairCode != null ? pa.fgPrimary : pa.fgTertiary,
                    fontWeight:
                        _pairCode != null ? FontWeight.w700 : FontWeight.w400,
                  ),
                ),
              ],
            ),
          ),
          TextButton(
            key: const ValueKey<String>('pa-pair-read'),
            onPressed: _pairLoading ? null : _loadPairCode,
            child: Text(
              '读取配对码',
              style: PATypography.caption.copyWith(
                color: pa.brand,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ],
      ),
    );
  }

  // ── 手动连接表单 ────────────────────────────────────────────────────────

  Widget _buildConnectForm(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            '手动连接',
            style: PATypography.body.copyWith(
              color: pa.fgPrimary,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 3),
          Text(
            '填 host:port 与配对 token（未配对可留空），手动发起连接',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
          const SizedBox(height: 10),
          TextField(
            key: const ValueKey<String>('pa-connect-addr'),
            controller: _addrCtrl,
            style: PATypography.body.copyWith(color: pa.fgPrimary),
            decoration: _inputDecoration(pa, '主机地址 host:port'),
          ),
          const SizedBox(height: 8),
          TextField(
            key: const ValueKey<String>('pa-connect-token'),
            controller: _tokenCtrl,
            obscureText: true,
            style: PATypography.body.copyWith(color: pa.fgPrimary),
            decoration: _inputDecoration(pa, '配对 token（仅进程内过桥，不进日志）'),
          ),
          const SizedBox(height: 12),
          Row(
            children: <Widget>[
              PAButton(
                key: const ValueKey<String>('pa-connect-submit'),
                label: '连接',
                expanded: false,
                onPressed: _connecting ? null : _connect,
              ),
              if (_connecting) ...<Widget>[
                const SizedBox(width: 10),
                SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: pa.brand,
                  ),
                ),
              ],
            ],
          ),
          if (_connectNotice != null) ...<Widget>[
            const SizedBox(height: 8),
            Text(
              _connectNotice!,
              key: const ValueKey<String>('pa-connect-notice'),
              style: PATypography.caption.copyWith(color: pa.ok),
            ),
          ],
          if (_connectErrorText != null) ...<Widget>[
            const SizedBox(height: 8),
            Text(
              _connectErrorText!,
              key: const ValueKey<String>('pa-connect-error'),
              style: PATypography.caption.copyWith(color: pa.danger),
            ),
          ],
        ],
      ),
    );
  }

  InputDecoration _inputDecoration(PAColors pa, String hint) => InputDecoration(
        isDense: true,
        hintText: hint,
        hintStyle: PATypography.caption.copyWith(color: pa.fgTertiary),
        filled: true,
        fillColor: pa.bgSurface2,
        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: BorderSide(color: pa.hairline),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: BorderSide(color: pa.brand, width: 1.5),
        ),
      );

  Widget _caption(BuildContext context, String text, {required bool danger}) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.fromLTRB(58, 0, 14, 8),
      child: Text(
        text,
        style: PATypography.caption
            .copyWith(color: danger ? pa.danger : pa.fgTertiary),
      ),
    );
  }
}

/// 与 settings.dart `_Group` 同款的左缩进分隔线（58 = 图标列宽 + 间距）。
class _InsetDivider extends StatelessWidget {
  const _InsetDivider();

  @override
  Widget build(BuildContext context) => const Padding(
        padding: EdgeInsets.only(left: 58),
        child: Divider(),
      );
}

/// 与 `_SettingsRow` 同款的 32×32 圆角图标块。
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
