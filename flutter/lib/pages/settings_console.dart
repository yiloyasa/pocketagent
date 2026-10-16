// 设置页「电脑控制台」二级页（路由 sub=console）—— Settings 三子步第③步。
//
// 【为什么整页只读】Kotlin 侧 console = `RuntimeSection()`（1548 行）+
// UnknownRequestConfirmSection，写操作四件套（SetRuntimePolicy / SetCollabPolicy /
// AcknowledgeUnknownEvent / ExecuteUnknownEventAction）全是甲级写：SetRuntimePolicy
// 的 proto 明文「提交完整策略不是补丁 + revision CAS」，只提交一个字段会把其余项
// 重置回默认（daemon 侧实测踩过）。本波只呈现 daemon 已经算好的真值
// （effective_global / effective / 状态 / 台账），写面一行不接、一个按钮不摆
// ——没有就是没有，不摆假按钮（红线：不留假绿）。
//
// 【三门控（本文件的纪律核心）】
//   ① `runtime.v1` 能力位门控：旧 daemon 没有这些 RPC（一律 UNIMPLEMENTED）。
//      位不在 = 不渲染三块内容，如实说明「未宣告该能力位」，不摆假开关
//      （Kotlin runtimeControlCapable 同款，SettingsScreen.kt:1647-1649）。
//   ② 三块各自三态：加载 / 读失败 / 已就绪，**一块失败不拖垮其余两块**
//      （Kotlin RuntimeControlState 五组 RPC 各拉各的、逐块记错同款）——
//      否则「这台 daemon 支持多少」会变成全有或全无。
//   ③ 空态 ≠ 错误态：台账空列表 = 「还没有未知事件」（真空态）；HOST_UNAVAILABLE
//      = 错误态带重试。两者在 UI 与测试里严格区分，不许用空列表冒充错误。
//
// 【为什么人话全部来自 daemon】文案（plain_title / translation / summary /
// label / hint）由 daemon 给中文人话，客户端只按 id 匹配与排版，不做任何术语
// 翻译；daemon 没给（旧端/旧台账条目）时如实降级为原文 + 固定说明，不编造。

import 'package:flutter/material.dart';

import '../gen/pocketagent/v1/host.pb.dart';
import '../gen/pocketagent/v1/runtime.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'pa_page_kit.dart';
import 'settings_connection.dart';
// 人类可读体积格式化（B/KB/MB/GB 口径）复用 Files 页的同名纯函数 —— 全工程
// 一份实现（口径 = Android formatBytesLabel），不为控制台再抄一版。
import 'files_bridge.dart' show humanFileSize;

/// runtime 域能力位（Kotlin `runtimeControlCapable` 判定的那一位）。
///
/// 【为什么不试错式地点亮】旧 daemon 没有这些 RPC，点了必然 UNIMPLEMENTED。
/// 摆一排必然失败的开关比不摆更糟 —— 用户会以为功能坏了。
const String kRuntimeV1Capability = 'runtime.v1';

/// 本机是否支持运行时控制面（纯函数，与 Kotlin
/// `runtimeControlCapable(capabilities)` 同语义：大小写不敏感地找 `runtime.v1`）。
bool runtimeControlCapable(List<String> capabilities) =>
    capabilities.any((String bit) => bit.toLowerCase() == kRuntimeV1Capability);

// ── 纯函数：daemon 原文字 → 人话标签（口径 = Kotlin RuntimeStateLogic 同款词表，
// 未知取值原样返回 ——透传红线，不显「未知/错误」） ──────────────────────────

/// 运行时总览状态（ready / degraded）→ 人话。
String runtimeStateLabel(String raw) => switch (raw) {
      'ready' => '就绪',
      'degraded' => '降级',
      '' => '未报告',
      _ => raw,
    };

/// 未知事件处理状态 → 人话（词表 = Kotlin unknownStatusLabel）。
String unknownStatusLabel(String raw) => switch (raw) {
      'NEW' => '待处理',
      'INSPECTED' => '已查看',
      'IGNORED' => '已忽略',
      'RETRY_LATER' => '稍后再看',
      'ESCALATED' => '已升级',
      'NEEDS_NATIVE_SUPPORT' => '待上游支持',
      '' => '未报告',
      _ => raw,
    };

/// 未知事件归类 → 人话（词表 = Kotlin unknownKindLabel）。
String unknownKindLabel(String raw) => switch (raw) {
      'UNKNOWN_NOTIFICATION' => '新通知',
      'UNKNOWN_REQUEST' => '新请求',
      'UNKNOWN_RESPONSE' => '新响应',
      '' => '未报告',
      _ => raw,
    };

/// 安全分级 → 人话。
///
/// 【为什么 UNKNOWN 不写「未知」】daemon 侧分类规则尚未定义，故安全分级恒为
/// UNKNOWN、如实标注。照抄成「未知」会被读成「我们这边出了问题」；事实是上游
/// 还没定义 —— 两者要能分开。
String safetyClassLabel(String raw) =>
    raw.isEmpty || raw == 'UNKNOWN' ? '尚未分类' : raw;

/// 台账脚注（纯函数）：**null = 无需说明**（空台账，或一条都没丢 ——
/// 不写「丢弃 0 条」这种噪音）。口径 = Kotlin unknownLedgerNote。
///
/// [total]/[dropped] 收 int（调用点把 Int64 显式 `toInt()`）：纯函数不该依赖
/// fixnum 类型，测试也不必为此 import fixnum。
String? unknownLedgerNote(int total, int dropped) {
  if (total <= 0) return null;
  if (dropped <= 0) return '累计 $total 条';
  return '累计 $total 条，超出上限已丢弃 $dropped 条';
}

/// 未知事件列表项标题：**原生方法名原文**（不翻译、不改写），只把归类/状态译成
/// 人话。方法名缺席时才退回归类文案 —— 空白标题会让整个列表看起来像坏了。
String unknownEventTitle(UnknownEventSummary s) {
  if (s.sourceMethod.isNotEmpty) return s.sourceMethod;
  if (s.normalizedKind.isNotEmpty) return unknownKindLabel(s.normalizedKind);
  return s.eventId;
}

/// 一行额度（`RuntimeLimits` 的一个 optional 项）：label + 字节数。
///
/// 【为什么按 has* 判定而不是读零值】proto3 optional 的零值分不清「这一项没设」
/// 与「显式设成 0」，而这两件事在预算里必须分开：未设 = 继承上一级；显式 0 =
/// 明令封禁。页面只渲染 daemon 明确设过的项（has* 为真），没设的不占位
/// ——把「没设」显示成「封禁」或「未知」都是编造。
@immutable
class LimitRow {
  const LimitRow({
    required this.label,
    required this.value,
    this.isCount = false,
  });

  final String label;

  /// 字节量（isCount=false）或计数值（isCount=true）。
  final int value;

  /// true = 计数值（并发轮次上限这类不带字节单位的项）。
  final bool isCount;
}

/// effective_global → 可渲染的额度行（纯函数；daemon 没设的项不进列表）。
List<LimitRow> limitRowsOf(RuntimeLimits? limits) {
  if (limits == null) return const <LimitRow>[];
  final List<LimitRow> rows = <LimitRow>[];
  void add(String label, bool present, int bytes) {
    if (present) rows.add(LimitRow(label: label, value: bytes));
  }

  add('内存 · 工作集', limits.hasRamWorkingBytes(), limits.ramWorkingBytes.toInt());
  add('内存 · 事件缓冲', limits.hasRamEventBufferBytes(),
      limits.ramEventBufferBytes.toInt());
  add('内存 · 会话索引', limits.hasRamSessionIndexBytes(),
      limits.ramSessionIndexBytes.toInt());
  add('磁盘 · 对话记录', limits.hasDiskConversationBytes(),
      limits.diskConversationBytes.toInt());
  add('磁盘 · 缓存', limits.hasDiskCacheBytes(), limits.diskCacheBytes.toInt());
  add('磁盘 · 产物', limits.hasDiskArtifactBytes(),
      limits.diskArtifactBytes.toInt());
  // max_parallel_turns 不是字节量：单独一行、不带单位换算。
  if (limits.hasMaxParallelTurns()) {
    rows.add(LimitRow(
      label: '并发轮次上限',
      value: limits.maxParallelTurns,
      isCount: true,
    ));
  }
  return List<LimitRow>.unmodifiable(rows);
}

/// 控制台页错误文案：与 connection / agent / models / downloads 子页同一套
/// code→文案映射（[paPageErrorText]），只把 HOST_UNAVAILABLE 的收尾按本页改词。
String consolePageErrorText(PaBridgeException e) => paPageErrorText(
      e,
      hostUnavailableTail: '运行时控制台暂不可用。',
    );

/// 「电脑控制台」子页：运行时状态 / 预算策略 / 未知事件台账三块（全部只读）。
///
/// 数据 = `getHostInfo`（能力位门控）+ `getRuntimeState` / `getRuntimePolicy` /
/// `listUnknownEvents` 三块各自拉取 + 点条目后 `getUnknownEvent` 详情。
/// 写操作四件套本波不接（见文件头），方法面见 pa_bridge.dart 的交接注释。
class SettingsConsolePage extends StatefulWidget {
  const SettingsConsolePage({super.key, this.bridge, required this.onBack});

  /// 通道注入点（测试用）；不传即用生产默认通道 `dev.pocketagent/bridge`。
  final PaBridge? bridge;

  /// 返回一级页。
  final VoidCallback onBack;

  @override
  State<SettingsConsolePage> createState() => _SettingsConsolePageState();
}

class _SettingsConsolePageState extends State<SettingsConsolePage> {
  late final PaBridge _bridge = widget.bridge ?? PaBridge();

  // ── 能力位门控块 ────────────────────────────────────────────────────────

  PaAsyncPhase _capsPhase = PaAsyncPhase.loading;
  List<String> _capabilities = const <String>[];
  String _capsErrorText = '';

  // ── 三块内容各自的三态（一块失败不拖垮其余两块） ────────────────────────

  PaAsyncPhase _statePhase = PaAsyncPhase.loading;
  RuntimeStateResponse? _runtimeState;
  String _stateErrorText = '';

  PaAsyncPhase _policyPhase = PaAsyncPhase.loading;
  RuntimePolicyResponse? _runtimePolicy;
  String _policyErrorText = '';

  PaAsyncPhase _eventsPhase = PaAsyncPhase.loading;
  ListUnknownEventsResponse? _unknownEvents;
  String _eventsErrorText = '';

  /// 已展开的未知事件 id（点列表项拉详情；null = 没有展开项）。
  String? _detailEventId;
  PaAsyncPhase _detailPhase = PaAsyncPhase.loading;
  UnknownEventResponse? _detail;
  String _detailErrorText = '';

  @override
  void initState() {
    super.initState();
    _loadCaps();
  }

  Future<void> _loadCaps() async {
    setState(() {
      _capsPhase = PaAsyncPhase.loading;
      _capsErrorText = '';
    });
    try {
      final HostInfo info = await _bridge.getHostInfo();
      if (!mounted) return;
      setState(() {
        _capabilities = info.capabilities;
        _capsPhase = PaAsyncPhase.ready;
      });
      // 位门控通过才拉三块：无位不产生任何 runtime 域调用（旧 daemon 上这些
      // RPC 必然 UNIMPLEMENTED，发过去只是把错误摆给用户看）。
      if (runtimeControlCapable(_capabilities)) {
        _loadState();
        _loadPolicy();
        _loadEvents();
      }
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _capabilities = const <String>[];
        _capsPhase = PaAsyncPhase.error;
        _capsErrorText = consolePageErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _capabilities = const <String>[];
        _capsPhase = PaAsyncPhase.error;
        _capsErrorText = '读取主机能力失败：$e';
      });
    }
  }

  Future<void> _loadState() async {
    setState(() {
      _statePhase = PaAsyncPhase.loading;
      _stateErrorText = '';
    });
    try {
      final RuntimeStateResponse state = await _bridge.getRuntimeState();
      if (!mounted) return;
      setState(() {
        _runtimeState = state;
        _statePhase = PaAsyncPhase.ready;
      });
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _runtimeState = null;
        _statePhase = PaAsyncPhase.error;
        _stateErrorText = consolePageErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _runtimeState = null;
        _statePhase = PaAsyncPhase.error;
        _stateErrorText = '读取运行时状态失败：$e';
      });
    }
  }

  Future<void> _loadPolicy() async {
    setState(() {
      _policyPhase = PaAsyncPhase.loading;
      _policyErrorText = '';
    });
    try {
      final RuntimePolicyResponse policy = await _bridge.getRuntimePolicy();
      if (!mounted) return;
      setState(() {
        _runtimePolicy = policy;
        _policyPhase = PaAsyncPhase.ready;
      });
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _runtimePolicy = null;
        _policyPhase = PaAsyncPhase.error;
        _policyErrorText = consolePageErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _runtimePolicy = null;
        _policyPhase = PaAsyncPhase.error;
        _policyErrorText = '读取运行时策略失败：$e';
      });
    }
  }

  Future<void> _loadEvents() async {
    setState(() {
      _eventsPhase = PaAsyncPhase.loading;
      _eventsErrorText = '';
    });
    try {
      final ListUnknownEventsResponse events =
          await _bridge.listUnknownEvents();
      if (!mounted) return;
      setState(() {
        _unknownEvents = events;
        _eventsPhase = PaAsyncPhase.ready;
      });
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _unknownEvents = null;
        _eventsPhase = PaAsyncPhase.error;
        _eventsErrorText = consolePageErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _unknownEvents = null;
        _eventsPhase = PaAsyncPhase.error;
        _eventsErrorText = '读取未知事件台账失败：$e';
      });
    }
  }

  /// 点一条未知事件 → 拉详情（只读；动作按钮本波不做，见文件头）。
  Future<void> _openDetail(String eventId) async {
    setState(() {
      _detailEventId = eventId;
      _detailPhase = PaAsyncPhase.loading;
      _detailErrorText = '';
      _detail = null;
    });
    try {
      final UnknownEventResponse detail =
          await _bridge.getUnknownEvent(GetUnknownEventRequest(eventId: eventId));
      if (!mounted) return;
      setState(() {
        _detail = detail;
        _detailPhase = PaAsyncPhase.ready;
      });
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _detail = null;
        _detailPhase = PaAsyncPhase.error;
        _detailErrorText = consolePageErrorText(e);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _detail = null;
        _detailPhase = PaAsyncPhase.error;
        _detailErrorText = '读取未知事件详情失败：$e';
      });
    }
  }

  void _closeDetail() {
    setState(() {
      _detailEventId = null;
      _detail = null;
      _detailErrorText = '';
      _detailPhase = PaAsyncPhase.loading;
    });
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaSubPageHeader(
          title: '电脑控制台',
          backKey: 'pa-console-back',
          onBack: _popBack,
        ),
        _buildBody(context),
      ],
    );
  }

  /// 返回键的公共出口（头部 + 详情卡的关闭都走它回一级页/收起详情）。
  void _popBack() => widget.onBack();

  Widget _buildBody(BuildContext context) {
    if (_capsPhase == PaAsyncPhase.loading) {
      return const PaCardCaption(
        text: '正在读取主机能力位…',
        textKey: 'pa-console-caps-loading',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    if (_capsPhase == PaAsyncPhase.error) {
      // 能力位读不到 = 门控无从判断：诚实错误态 + 重试（不猜「有位」也不猜「无位」）。
      return PaErrorRetry(
        errorText: _capsErrorText,
        errorKey: 'pa-console-caps-error',
        onRetry: _loadCaps,
        retryKey: 'pa-console-caps-retry',
      );
    }
    if (!runtimeControlCapable(_capabilities)) {
      // 门控①：无位不显示三块内容，如实说明缺的是哪一位。
      return const PaCardCaption(
        text: '电脑端未宣告 runtime.v1 能力位：运行时控制台在本机 daemon 上不可用。',
        textKey: 'pa-console-no-capability',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        PaPageSection(
          title: '运行时状态',
          children: <Widget>[_buildStateBody(context)],
        ),
        PaPageSection(
          title: '预算与资源',
          children: <Widget>[_buildPolicyBody(context)],
        ),
        PaPageSection(
          title: '未知事件',
          children: <Widget>[_buildEventsBody(context)],
        ),
        if (_detailEventId != null)
          PaPageSection(
            title: '事件详情',
            children: <Widget>[_buildDetailBody(context)],
          ),
        // 写操作交接：明说「本页只读」，不给用户留「是不是坏了」的疑问。
        Padding(
          padding: const EdgeInsets.fromLTRB(4, 0, 4, 8),
          child: Text(
            '本页只读呈现电脑端 daemon 的运行时真值；改预算/策略、确认或执行'
            '未知事件动作在后续波次开放。',
            key: const ValueKey<String>('pa-console-readonly-note'),
            style: PATypography.caption.copyWith(color: context.pa.fgTertiary),
          ),
        ),
      ],
    );
  }

  // ── 块一：运行时状态 ────────────────────────────────────────────────────

  Widget _buildStateBody(BuildContext context) {
    if (_statePhase == PaAsyncPhase.loading) {
      return const PaCardCaption(
        text: '正在读取运行时状态…',
        textKey: 'pa-console-state-loading',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    if (_statePhase == PaAsyncPhase.error) {
      return PaErrorRetry(
        errorText: _stateErrorText,
        errorKey: 'pa-console-state-error',
        onRetry: _loadState,
        retryKey: 'pa-console-state-retry',
      );
    }
    final RuntimeStateResponse state = _runtimeState!;
    final List<String> facts = <String>[
      if (state.daemonVersion.isNotEmpty) 'daemon ${state.daemonVersion}',
      if (state.runtimeState.isNotEmpty) '运行时 ${runtimeStateLabel(state.runtimeState)}',
      if (state.hostId.isNotEmpty) '主机 ${state.hostId}',
    ];
    final List<RuntimeContractReport> contracts = state.contracts;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        if (facts.isNotEmpty)
          PaCardCaption(
            text: facts.join(' · '),
            textKey: 'pa-console-state-line',
            padding: const EdgeInsets.fromLTRB(14, 12, 14, 6),
          ),
        if (contracts.isEmpty)
          const PaCardCaption(
            // 读成功但一家形态都没申报：如实说「未申报」，不造形态卡。
            text: '电脑端未申报任何 agent 的运行时形态。',
            textKey: 'pa-console-state-empty',
            padding: EdgeInsets.fromLTRB(14, 0, 14, 12),
          )
        else
          for (final RuntimeContractReport c in contracts)
            _contractCard(context, c),
      ],
    );
  }

  /// 一个 agent 的运行时形态卡（declared + 六个 model 字段 + 违约/限制条目）。
  ///
  /// 【为什么不按 agent 名分支】六个 model 字段是 daemon 给的机器可读答案，
  /// 手机端据此排版；按 agent 名字分支 = 第二份写死名单（红线）。
  Widget _contractCard(BuildContext context, RuntimeContractReport c) {
    final PAColors pa = context.pa;
    final List<String> models = <String>[
      if (c.sessionModel.isNotEmpty) '会话 ${c.sessionModel}',
      if (c.resumeModel.isNotEmpty) '恢复 ${c.resumeModel}',
      if (c.approvalModel.isNotEmpty) '审批 ${c.approvalModel}',
      if (c.subagentModel.isNotEmpty) '子代理 ${c.subagentModel}',
      if (c.memoryModel.isNotEmpty) '记忆 ${c.memoryModel}',
      if (c.cacheModel.isNotEmpty) '缓存 ${c.cacheModel}',
    ];
    final List<String> notes = <String>[
      for (final RuntimeContractIssue i in c.issues) '${i.code}：${i.detail}',
      for (final RuntimeLimitation l in c.limitations) '${l.capability}：${l.reason}',
    ];
    return Padding(
      key: ValueKey<String>('pa-console-contract-${c.agentId}'),
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  // 显示名优先、空则退回 id（不显示空白行）。
                  c.displayName.isNotEmpty ? c.displayName : c.agentId,
                  style: PATypography.body.copyWith(color: pa.fgPrimary),
                ),
              ),
              Text(
                c.declared ? '已申报' : '未申报',
                style: PATypography.caption.copyWith(color: pa.fgSecondary),
              ),
            ],
          ),
          if (c.version.isNotEmpty) ...<Widget>[
            const SizedBox(height: 2),
            Text(
              '版本 ${c.version}',
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
          if (models.isNotEmpty) ...<Widget>[
            const SizedBox(height: 2),
            Text(
              models.join(' · '),
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
          for (final String n in notes) ...<Widget>[
            const SizedBox(height: 2),
            Text(
              n,
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
          ],
        ],
      ),
    );
  }

  // ── 块二：预算与资源 ────────────────────────────────────────────────────

  Widget _buildPolicyBody(BuildContext context) {
    if (_policyPhase == PaAsyncPhase.loading) {
      return const PaCardCaption(
        text: '正在读取运行时策略…',
        textKey: 'pa-console-policy-loading',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    if (_policyPhase == PaAsyncPhase.error) {
      return PaErrorRetry(
        errorText: _policyErrorText,
        errorKey: 'pa-console-policy-error',
        onRetry: _loadPolicy,
        retryKey: 'pa-console-policy-retry',
      );
    }
    final RuntimePolicyResponse policy = _runtimePolicy!;
    final List<LimitRow> rows = limitRowsOf(policy.effectiveGlobal);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        PaCardCaption(
          text: <String>[
            '策略版本 ${policy.revision}',
            if (policy.policyPath.isNotEmpty) policy.policyPath,
          ].join(' · '),
          textKey: 'pa-console-policy-line',
          padding: const EdgeInsets.fromLTRB(14, 12, 14, 6),
        ),
        if (rows.isEmpty)
          const PaCardCaption(
            // 读成功但 effective 全缺席（旧 daemon）：如实说未设置，不显示假额度。
            text: '电脑端未设置全局额度。',
            textKey: 'pa-console-policy-empty',
            padding: EdgeInsets.fromLTRB(14, 0, 14, 12),
          )
        else
          for (final LimitRow row in rows) _limitRow(context, row),
      ],
    );
  }

  Widget _limitRow(BuildContext context, LimitRow row) {
    final PAColors pa = context.pa;
    return Padding(
      key: ValueKey<String>('pa-console-limit-${row.label}'),
      padding: const EdgeInsets.fromLTRB(14, 4, 14, 4),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Text(
              row.label,
              style: PATypography.body.copyWith(color: pa.fgPrimary),
            ),
          ),
          Text(
            row.isCount ? '${row.value}' : humanFileSize(row.value),
            style: PATypography.body.copyWith(color: pa.fgSecondary),
          ),
        ],
      ),
    );
  }

  // ── 块三：未知事件台账 ──────────────────────────────────────────────────

  Widget _buildEventsBody(BuildContext context) {
    if (_eventsPhase == PaAsyncPhase.loading) {
      return const PaCardCaption(
        text: '正在读取未知事件台账…',
        textKey: 'pa-console-events-loading',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    if (_eventsPhase == PaAsyncPhase.error) {
      return PaErrorRetry(
        errorText: _eventsErrorText,
        errorKey: 'pa-console-events-error',
        onRetry: _loadEvents,
        retryKey: 'pa-console-events-retry',
      );
    }
    final ListUnknownEventsResponse events = _unknownEvents!;
    final List<UnknownEventSummary> items = events.items;
    final String? note =
        unknownLedgerNote(events.total.toInt(), events.dropped.toInt());
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        if (note != null)
          PaCardCaption(
            text: note,
            textKey: 'pa-console-events-ledger',
            padding: const EdgeInsets.fromLTRB(14, 12, 14, 6),
          ),
        if (items.isEmpty)
          const PaCardCaption(
            // 空台账 = 真空态（daemon 没登记过未知事件），与读失败分家。
            text: '还没有未知事件：电脑端没有收到过来路不明的操作。',
            textKey: 'pa-console-events-empty',
            padding: EdgeInsets.fromLTRB(14, 0, 14, 12),
          )
        else
          for (final UnknownEventSummary s in items)
            _unknownEventRow(context, s),
      ],
    );
  }

  Widget _unknownEventRow(BuildContext context, UnknownEventSummary s) {
    final PAColors pa = context.pa;
    final String title = unknownEventTitle(s);
    final List<String> meta = <String>[
      if (s.sourceAgent.isNotEmpty) s.sourceAgent,
      if (s.normalizedKind.isNotEmpty) unknownKindLabel(s.normalizedKind),
      unknownStatusLabel(s.processingStatus),
      safetyClassLabel(s.safetyClass),
    ];
    return InkWell(
      key: ValueKey<String>('pa-console-event-${s.eventId}'),
      onTap: () => _openDetail(s.eventId),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Expanded(
                  child: Text(
                    title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: PATypography.body.copyWith(color: pa.fgPrimary),
                  ),
                ),
                if (s.requiresUser)
                  Text(
                    '需要你处理',
                    style: PATypography.caption.copyWith(color: pa.warn),
                  ),
              ],
            ),
            const SizedBox(height: 3),
            Text(
              meta.join(' · '),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            ),
            // summary 缺席时降级为固定说明（Kotlin UNKNOWN_SUMMARY_FALLBACK_NOTE
            // 同款），不造翻译。
            if (s.plainTitle.isEmpty && s.summary.isEmpty) ...<Widget>[
              const SizedBox(height: 2),
              Text(
                '此 daemon 版本未提供翻译',
                key: const ValueKey<String>('pa-console-event-fallback'),
                style: TextStyle(fontSize: 12),
              ),
            ],
          ],
        ),
      ),
    );
  }

  // ── 未知事件详情（只读） ────────────────────────────────────────────────

  Widget _buildDetailBody(BuildContext context) {
    if (_detailPhase == PaAsyncPhase.loading) {
      return const PaCardCaption(
        text: '正在读取事件详情…',
        textKey: 'pa-console-detail-loading',
        padding: EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      );
    }
    if (_detailPhase == PaAsyncPhase.error) {
      return PaErrorRetry(
        errorText: _detailErrorText,
        errorKey: 'pa-console-detail-error',
        onRetry: () => _openDetail(_detailEventId!),
        retryKey: 'pa-console-detail-retry',
      );
    }
    final UnknownEventResponse detail = _detail!;
    // 详情的人话三句：daemon 给的 plain_title / translation / recommended_next_step。
    // 都不在（旧 daemon）时整组不出现 —— 不占位、不编造。
    final List<String> sections = <String>[
      if (detail.summary.plainTitle.isNotEmpty) detail.summary.plainTitle,
      if (detail.translation.isNotEmpty) detail.translation,
      if (detail.recommendedNextStep.isNotEmpty) detail.recommendedNextStep,
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        for (int i = 0; i < sections.length; i++)
          PaCardCaption(
            text: sections[i],
            textKey: 'pa-console-detail-text-$i',
            padding: EdgeInsets.fromLTRB(14, i == 0 ? 12 : 4, 14, 4),
          ),
        // 字段透析表：daemon 拆开读的每一行（旧 daemon 为空 → 整块不出现）。
        if (detail.digest.isNotEmpty)
          for (final UnknownEventField f in detail.digest)
            Padding(
              key: ValueKey<String>('pa-console-digest-${f.key}'),
              padding: const EdgeInsets.fromLTRB(14, 4, 14, 4),
              child: Text(
                '${f.key}（${f.kind}）：${f.preview}',
                style: PATypography.code
                    .copyWith(color: context.pa.fgTertiary),
              ),
            ),
        // 脱敏原文永远可查（翻译是摘要不是裁决，冲突以原文为准）。
        if (detail.rawJson.isNotEmpty)
          PaCardCaption(
            text: detail.rawJson,
            textKey: 'pa-console-detail-raw',
            padding: const EdgeInsets.fromLTRB(14, 8, 14, 8),
          ),
        // 动作按钮本波不做：daemon 在 actions[] 里声明了哪些可执行，但执行是
        // 甲级写操作（白名单 + 乐观并发），留后续波次 —— 这里连「暂不可用」的
        // 灰按钮都不摆，只读呈现。
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton(
            key: const ValueKey<String>('pa-console-detail-close'),
            onPressed: _closeDetail,
            child: Text(
              '收起详情',
              style: PATypography.caption
                  .copyWith(color: context.pa.brand, fontWeight: FontWeight.w600),
            ),
          ),
        ),
      ],
    );
  }
}
