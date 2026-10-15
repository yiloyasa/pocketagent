// 设置页「模型」二级页（路由 sub=capabilities）—— B3 第 2 步②：
// 模型与能力契约的消费者（capabilities 子页骨架）。
//
// 【本波范围】只做「模型列表」的消费形状：数据源抽象 + 响应映射纯函数 + 三态
// （加载 / 错误 / 就绪）渲染。**不放任何写操作**（改模型 = 写桌面配置相关，后续
// 波次拍板前不做 —— 与 agent 子页原生设置门控同一红线）。
//
// 【listModels 的真桥接为什么还不在这里】pa_bridge.dart 的方法面目前只有 6 条
// 具名封装（getHostInfo/connect/disconnect/getConnectionState/pairCode/startTurn），
// **没有**公开的通用 invoke；transport 是 B3-3 的领土，本波只写消费形状。因此
// [BridgeModelsDataSource] 以「未来 PaBridge.listModels」的形态定稿：B3-3/B4 给
// pa_bridge 补该方法（或公开 invoke）后，把构造闭包一换即可，本页零改动；当前
// 生产无闭包 → 如实走 HOST_UNAVAILABLE 占位，**不伪造模型列表**（不留假绿）。
//
// 【响应形状】proto/v1/model.proto：ListModelsRequest{agent_id, refresh, thread_id}
// → ListModelsResponse{models[], current_model_id, current_effort, state_error,
// effective_selection}。本波消费 models 的 id/provider/display_name/context_window/
// supports_thinking/supports_images/available + current_model_id（打「当前使用」标）；
// state_error 非空且无模型 = 读不到元数据（与「确实没有模型」区分，走错误态带
// 原文）；effective_selection（有效选择真值）与写入口一起在后续波次接。
//
// 【轮询纪律】Android 侧有 10s 巡检；本波只做手动刷新（打开一次 + 刷新键），
// 轮询随真桥接一并接入 —— 没有真通道的轮询只会空转。

import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:protobuf/protobuf.dart' show InvalidProtocolBufferException;

import '../components/pa_card.dart';
import '../components/pa_status_chip.dart';
import '../gen/pocketagent/v1/model.pb.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import 'pa_page_kit.dart';
import 'settings_connection.dart';

/// 一行模型选项 —— 本页的消费面（proto `ModelInfo` 的展示投影 + 是否当前使用）。
///
/// 【为什么再包一层而不是直接用 gen 的 ModelInfo】页面只该认识「渲染需要的字段」
/// （Android 侧 ModelOptionRow 同款纪律：行只管展示，选中态/来源由调用方算好）；
/// 且 isCurrent 是两个 proto 字段（models + current_model_id）的派生，不是
/// ModelInfo 自身字段。
@immutable
class ModelOption {
  const ModelOption({
    required this.id,
    required this.provider,
    required this.displayName,
    required this.contextWindow,
    required this.supportsThinking,
    required this.supportsImages,
    required this.available,
    required this.isCurrent,
  });

  /// 模型 id（`anthropic/claude-sonnet-4` 风格原文，透传不加工）。
  final String id;

  /// 来源 agent / provider（`ModelInfo.provider` 原文：codex/pi/hermes/…）。
  final String provider;

  /// 原生显示名（空串 = 原生没报，展示层退回 [id]，绝不显示空白行）。
  final String displayName;

  /// 上下文窗口（0 = 未报，不显示假数字）。
  final int contextWindow;

  final bool supportsThinking;
  final bool supportsImages;

  /// key 已配置且连通（`ModelInfo.available` 原文）。
  final bool available;

  /// 是否当前生效模型（`id == current_model_id`，两个字段都来自同一响应，不猜）。
  final bool isCurrent;

  /// 主文案：displayName 优先，空串退回 id（Android `modelDisplayLabel` 同款兜底）。
  String get title => displayName.isNotEmpty ? displayName : id;

  /// 能力副文案（照 Android `modelRowDetail` 口径：上下文 → 思考 → 图片 → 当前；
  /// 未知上下文（≤0）不显示假数字）。可用性不在这里 —— 它是右列的独立徽标。
  String get detail {
    final List<String> parts = <String>[
      if (contextWindow > 0) '上下文 ${contextWindow ~/ 1000}k',
      if (supportsThinking) '支持思考',
      if (supportsImages) '支持图片',
      if (isCurrent) '当前使用',
    ];
    return parts.join(' · ');
  }
}

/// 数据源抽象 —— 「模型列表从哪来」只有这一个口子。
///
/// 【为什么是接口】测试注入可控 fake（本波真桥接未就位）；B3-3/B4 之后把
/// [BridgeModelsDataSource] 接上真方法，页面一行不用改 —— 接口形状先按契约
/// 定死的意义。
abstract class ModelsDataSource {
  /// 拉一次模型列表。
  ///
  /// 失败语义：桥接错误以 [PaBridgeException] 上抛；「daemon 明确报读不到元数据」
  /// 以 [ModelsUnavailableException] 上抛（两者在页面里都是错误态，但文案来源
  /// 不同：前者走错误码映射，后者带 daemon 原文）。
  Future<List<ModelOption>> listModels();
}

/// daemon 明确报告「读不到模型元数据」（`state_error` 非空且无模型）。
///
/// 【为什么不并进 PaBridgeException】它不是一个通道错误码：通道好着呢，是 daemon
/// 侧的元数据面暂时不可用。编造一个假 bridge code 会污染错误码契约表。
class ModelsUnavailableException implements Exception {
  const ModelsUnavailableException(this.reason);

  /// daemon 的人话原因原文（`ListModelsResponse.state_error` 透传，不改写）。
  final String reason;

  @override
  String toString() => 'ModelsUnavailableException: $reason';
}

/// 严格解码 `ListModelsResponse` wire 字节：坏字节不裸抛 protobuf 异常，统一成
/// [PaBridgeException]（DECODE_FAILED）—— 与 pa_bridge.dart 的解码纪律同款，
/// 上层 UI 只认一种异常。
ListModelsResponse decodeListModelsResponse(List<int> bytes) {
  try {
    return ListModelsResponse.fromBuffer(bytes);
  } on InvalidProtocolBufferException catch (e) {
    throw PaBridgeException(
      PaBridgeErrorCodes.decodeFailed,
      'listModels 返回值字节不可解析：$e',
    );
  }
}

/// 响应消息 → 模型选项列表（纯函数，单测直接命中；顺序保持 daemon 原序）。
List<ModelOption> modelOptionsOfResponse(ListModelsResponse response) {
  final String currentId = response.currentModelId;
  return response.models
      .map(
        (ModelInfo m) => ModelOption(
          id: m.id,
          provider: m.provider,
          displayName: m.displayName,
          contextWindow: m.contextWindow,
          supportsThinking: m.supportsThinking,
          supportsImages: m.supportsImages,
          available: m.available,
          isCurrent: currentId.isNotEmpty && m.id == currentId,
        ),
      )
      .toList(growable: false);
}

/// wire 字节 → 模型选项列表（纯函数：解码 + 映射两步都可单测）。
List<ModelOption> modelOptionsOf(List<int> responseBytes) =>
    modelOptionsOfResponse(decodeListModelsResponse(responseBytes));

/// 生产数据源：调 `bridge.invoke('listModels', ...)` —— 的**未来形态**。
///
/// 【为什么构造参数是裸 invoke 闭包而不是 PaBridge 实例】pa_bridge.dart 当前没有
/// 公开的通用 invoke（方法面 6 条全是具名封装），本波又不许改 transport；于是先按
/// 「B4 后 PaBridge 会有的那个口子」的形状定稿：
/// `BridgeModelsDataSource(invoke: bridge.listModels)`（或公开 invoke 后的等价形式），
/// 页面与本类都不需要再改。当前生产构造不带闭包 → 如实抛 HOST_UNAVAILABLE。
///
/// arguments = `ListModelsRequest` 的 wire 字节（本波空请求：无会话上下文，
/// agent_id/refresh/thread_id 全缺省——真接线时按页面语义补，形状不变）。
class BridgeModelsDataSource implements ModelsDataSource {
  const BridgeModelsDataSource({this.invoke});

  /// 未来 `PaBridge` 方法面的注入点（见类注释）；null = 真桥接未就位。
  final Future<Object?> Function(String method, Object? arguments)? invoke;

  @override
  Future<List<ModelOption>> listModels() async {
    final Future<Object?> Function(String, Object?)? invoke = this.invoke;
    if (invoke == null) {
      // B4 前的常态：真桥接未就位。如实上报，不返回编造的空列表冒充「没有模型」。
      throw const PaBridgeException(
        PaBridgeErrorCodes.hostUnavailable,
        'listModels 真桥接未就位（Kotlin 宿主 B4 波次注册；本波只定消费形状）',
      );
    }
    final Object? raw = await invoke('listModels', ListModelsRequest().writeToBuffer());
    if (raw is! Uint8List) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        'listModels 返回值须为 proto wire 字节(Uint8List)，实际 ${raw.runtimeType}',
      );
    }
    final ListModelsResponse response = decodeListModelsResponse(raw);
    if (response.models.isEmpty && response.stateError.isNotEmpty) {
      // 读不到元数据 ≠ 没有模型：带 daemon 原文走错误态，不渲染空列表糊弄。
      throw ModelsUnavailableException(response.stateError);
    }
    return modelOptionsOfResponse(response);
  }
}

/// 模型页错误文案：与 connection 子页同一套 code→文案映射
/// （[paBridgeErrorText]），只把 HOST_UNAVAILABLE 的收尾按本页语境改词。
///
/// 【收敛来历】原实现自己写了一份 `if hostUnavailable … else paBridgeErrorText`
/// 的委托（settings_agent.dart 同样一份）。收责任一为 [paPageErrorText]。
String capabilitiesPageErrorText(PaBridgeException e) => paPageErrorText(
      e,
      hostUnavailableTail: '模型列表暂不可用。',
    );

/// 三态：加载中 / 读失败 / 已就绪（就绪后再分「空列表」与「有模型」）。
///
/// 与 threads.dart 的 `_ThreadsPhase` 同名同义同序 —— 收敛为 pa_page_kit.dart
/// 的共享词汇 [PaAsyncPhase]（typedef 零运行时开销、零渲染，改名不碰像素；
/// 本页 switch/if 比较照旧）。页面其余代码一行不动。
typedef _ModelsPhase = PaAsyncPhase;

/// 「模型」子页：模型列表骨架（名称 / 来源 / 可用态三列）。
///
/// 数据 = 打开时拉一次 + 顶部刷新键；轮询与真桥接在后续波次（见文件头注释）。
class SettingsCapabilitiesPage extends StatefulWidget {
  const SettingsCapabilitiesPage({super.key, this.source, required this.onBack});

  /// 数据源注入点：测试传可控 fake；生产不传即 [BridgeModelsDataSource]
  /// （B4 前如实走 HOST_UNAVAILABLE 占位）。
  final ModelsDataSource? source;

  /// 返回一级页。
  final VoidCallback onBack;

  @override
  State<SettingsCapabilitiesPage> createState() =>
      _SettingsCapabilitiesPageState();
}

class _SettingsCapabilitiesPageState extends State<SettingsCapabilitiesPage> {
  late final ModelsDataSource _source;
  _ModelsPhase _phase = _ModelsPhase.loading;
  List<ModelOption> _models = const <ModelOption>[];
  String _errorText = '';

  @override
  void initState() {
    super.initState();
    _source = widget.source ?? const BridgeModelsDataSource();
    _load();
  }

  Future<void> _load() async {
    // 首帧初值就是 loading/空错误文案（initState 触发）：跳过 setState。
    if (_phase != _ModelsPhase.loading || _errorText.isNotEmpty) {
      setState(() {
        _phase = _ModelsPhase.loading;
        _errorText = '';
      });
    }
    try {
      final List<ModelOption> models = await _source.listModels();
      if (!mounted) return;
      setState(() {
        _models = models;
        _phase = _ModelsPhase.ready;
      });
    } on ModelsUnavailableException catch (e) {
      // daemon 明确说读不到元数据：带原文呈现，不冒充「没有模型」。
      if (!mounted) return;
      setState(() {
        _models = const <ModelOption>[];
        _phase = _ModelsPhase.error;
        _errorText = '暂时读不到模型元数据：${e.reason}';
      });
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _models = const <ModelOption>[];
        _phase = _ModelsPhase.error;
        _errorText = capabilitiesPageErrorText(e);
      });
    } catch (e) {
      // 兜底：页面任何路径都不许把未捕获异常甩给框架。
      if (!mounted) return;
      setState(() {
        _models = const <ModelOption>[];
        _phase = _ModelsPhase.error;
        _errorText = '读取模型列表失败：$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaSubPageHeader(
          title: '模型',
          backKey: 'pa-models-back',
          onBack: widget.onBack,
          // 刷新 = 手动重拉（轮询随真桥接波次接入）——这是本页唯一的动作。
          onRefresh: _load,
          // 模型页原间距 10（agent 页 14）：收敛后由共享件的参数各自保持原值，
          // 不统一成新数值（等价优先）。
          bottomPadding: 10,
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(4, 0, 4, 10),
          child: Text(
            '列出电脑端 daemon 报告的模型；本页只读，改模型在后续波次。',
            style: PATypography.caption
                .copyWith(color: context.pa.fgTertiary),
          ),
        ),
        PACard(
          padding: EdgeInsets.zero,
          child: _buildBody(context),
        ),
      ],
    );
  }

  // ── 三态 + 空态 ─────────────────────────────────────────────────────────

  Widget _buildBody(BuildContext context) {
    if (_phase == _ModelsPhase.loading) {
      return PaCardCaption(
        text: '正在读取模型列表…',
        textKey: 'pa-models-loading',
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      );
    }
    if (_phase == _ModelsPhase.error) {
      // 诚实错误态 + 重试，与 agent 子页同款装配（共享件 [PaErrorRetry]）。
      return PaErrorRetry(
        errorText: _errorText,
        errorKey: 'pa-models-error',
        onRetry: _load,
        retryKey: 'pa-models-retry',
      );
    }
    if (_models.isEmpty) {
      // 读成功但一个模型都没有（旧 daemon / 空注册面）：如实说「未报告」，
      // 不冒充读失败。
      return PaCardCaption(
        text: '电脑端未报告任何模型。',
        textKey: 'pa-models-empty',
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      );
    }
    return Column(
      children: <Widget>[
        for (int i = 0; i < _models.length; i++) ...<Widget>[
          if (i > 0)
            const Padding(
              padding: EdgeInsets.only(left: 58),
              child: Divider(),
            ),
          _modelRow(context, _models[i]),
        ],
      ],
    );
  }

  /// 一行模型：名称（主文案 + 能力副文案）｜来源 / 可用态（右列上下两行）。
  ///
  /// 【为什么可用态用 PAStatusChip 的 label 覆盖而不是新组件】五态 chip 是既有
  /// 视觉资产（绿=可用 / 灰=暂不可用），新造一个「模型可用徽标」就多一份视觉。
  Widget _modelRow(BuildContext context, ModelOption m) {
    final PAColors pa = context.pa;
    final String detail = m.detail;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      child: Row(
        key: ValueKey<String>('pa-model-row-${m.id}'),
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  m.title,
                  style: PATypography.body.copyWith(color: pa.fgPrimary),
                ),
                if (m.title != m.id) ...<Widget>[
                  const SizedBox(height: 2),
                  Text(
                    m.id,
                    style:
                        PATypography.code.copyWith(color: pa.fgTertiary),
                  ),
                ],
                if (detail.isNotEmpty) ...<Widget>[
                  const SizedBox(height: 2),
                  Text(
                    detail,
                    style: PATypography.caption.copyWith(color: pa.fgTertiary),
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: <Widget>[
              if (m.provider.isNotEmpty)
                Text(
                  m.provider,
                  style: PATypography.caption.copyWith(color: pa.fgSecondary),
                ),
              const SizedBox(height: 4),
              if (m.available)
                const PAStatusChip(status: PAStatusKind.done, label: '可用')
              else
                const PAStatusChip(status: PAStatusKind.pending, label: '暂不可用'),
            ],
          ),
        ],
      ),
    );
  }
}
