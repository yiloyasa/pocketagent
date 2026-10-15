// Chat 周边动作方法面（B3 第 5 波④）：ThreadService 一元 RPC 的 Dart 封装。
//
// 【为什么单拆一个文件】`pa_bridge.dart` 是 B2 冻结契约（本波只读，不改它
// 一行）：新增方法面若直接改它等于动冻结文件。本文件只回答「周边动作的请求
// 字节怎么组、响应字节怎么验」；错误模型与解码收口复用 `pa_bridge.dart` 的
// 现成件（`bytesOrThrow` + `PaBridgeException`/`PaBridgeErrorCodes` +
// `kPaBridgeMethodChannel`）。`_invoke` 的 try/catch 形状与
// `chat_history_transport.dart` 同款（PlatformException 透传 +
// MissingPluginException → HOST_UNAVAILABLE）：跨文件的私有方法复用不了，
// 抄的是错误语义逐字，不是新口径。
//
// 【宿主 B4 实现】Kotlin 宿主桥 B4 波次按方法名注册 handler（方法名 = RPC 名
// 首字母小写，与 daemon `ThreadService` 方法名一一对应，见 channel-contract
// §2 的「方法名 = RPC 名」总则）。在此之前如实抛 HOST_UNAVAILABLE，调用方
// （`ChatActionRunner`）进「通道未就绪」诚实占位，绝不伪造成功。
//
// 【方法面清单（10 接 1 留）】
//   已接（10，全部 unary）：submitApproval / cancelTurn / clearQueuedInputs /
//   compactThread / forkThread / rollbackThread / setThreadGoal /
//   startReview / exportThreadHtml / optimizePrompt。
//   未接（1，留后）：`DelegateSubTask`（thread.proto，daemon 内部 collab
//   回调路径：MCP `agent_delegate` 工具触发经 daemon 内 `collab_delegate.go`
//   自调；手机端直接发起委派走 B3-4 已接的 `dispatchCollab` + 协同角色体系，
//   本波九项动作无消费面——单加方法是死契约。子 agent 卡消费面落盘时按本文件
//   同模式只增一个方法，不动既有签名）。

import 'package:flutter/services.dart';

import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart'
    show
        PaBridgeErrorCodes,
        PaBridgeException,
        bytesOrThrow,
        kPaBridgeMethodChannel;

/// 周边动作方法面：ThreadService 一元 RPC 的 Dart 形态（可注入构造，测试隔离）。
///
/// 【为什么可注入而不是单例】与 `PaBridge` 同纪律：测试按通道名 mock 宿主侧
/// handler，用独立通道与生产隔离；生产用默认名即可。
class ChatActionsTransport {
  ChatActionsTransport({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(kPaBridgeMethodChannel);

  final MethodChannel _channel;

  /// 提交审批决定 —— proto `ThreadService.SubmitApproval`。
  ///
  /// arguments：`SubmitApprovalRequest` 的 wire 字节（thread_id + decision{
  /// request_id/approved/note/option}，封帧见 `chat_actions_data.dart` 的
  /// [approvalSubmitRequest]）。返回：null（`Empty` 的方法面形态：宿主按契约
  /// 回 null，回了别的即契约漂移，见 [_invokeExpectNull]）。
  ///
  /// 【为什么走一元 RPC 而不走流内 approval 帧】流内帧随连接生死：断线重连
  /// 后旧 request_id 可能已不在 pending，帧会静默丢失；一元 RPC 是
  /// reconnect-safe（request_id 不再 pending 时 daemon 显式报错，调用方如实
  /// Toast，不丢一次用户点按）。与 Android `ChatApprovalFlow`（answerApproval
  /// 走 SubmitApproval）同源。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> submitApproval(SubmitApprovalRequest request) =>
      _invokeExpectNull('submitApproval', request.writeToBuffer());

  /// 终止正在跑的轮次 —— proto `ThreadService.CancelTurn`。
  ///
  /// arguments：`CancelTurnRequest` 的 wire 字节（thread_id）。返回：null。
  ///
  /// 【与流内 cancel 帧的分工】流内 cancel（`chat_session.dart` 的 [TurnSession.
  /// cancel]）是首选（daemon 在流内直接 cancel，不另开通道）；本方法是流不可
  /// 用/已断时的兜底与「更多面板」里可点的那一个： running 态才显示（无轮次可
  /// 终时不摆入口）。两者语义一致（终止本会话当前轮），不并存抢状态。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> cancelTurn(String threadId) => _invokeExpectNull(
        'cancelTurn',
        CancelTurnRequest(threadId: threadId).writeToBuffer(),
      );

  /// 清空排队输入 —— proto `ThreadService.ClearQueuedInputs`（pi 0.87.1
  /// clear_queue 上行， unary）。
  ///
  /// arguments：`ClearQueuedInputsRequest` 的 wire 字节（thread_id）。返回：
  /// `ClearQueuedInputsResponse` 的 wire 字节（steered/follow_up 两笔计数，
  /// daemon 口径与 pi 的 steer/followUp 划分一致，见 thread.proto）。
  ///
  /// 【与 TurnSession.clearQueue 的分工】`clearQueue` 只收敛本地排队计数与
  /// phase（流面无清队帧，见 `chat_session.dart` 文件头）；本方法是真清除
  /// daemon 侧队列的那一下。调用成功后调用方同时调 `clearQueue` 收敛本地态，
  /// 两处一起才是「清干净了」。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ClearQueuedInputsResponse> clearQueuedInputs(String threadId) =>
      _invoke(
        'clearQueuedInputs',
        ClearQueuedInputsRequest(threadId: threadId).writeToBuffer(),
        _decodeClearQueuedInputs,
      );

  /// 压缩会话历史 —— proto `ThreadService.CompactThread`。
  ///
  /// arguments：`CompactThreadRequest` 的 wire 字节（thread_id）。返回：null。
  ///
  /// 【可见性】`compact:<agentId>` 位（daemon 按 AgentID 分派 codex/pi/zcode
  /// 原生压缩实现，dsh 无位——无位不显示压缩入口，见 `chat_actions_data.dart`
  /// 的 [visibleChatActions]）。运行中轮次未终局时调用方先挡（Android
  /// `doCompact` 同款：跑着就压等于丢上下文）。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<void> compactThread(String threadId) => _invokeExpectNull(
        'compactThread',
        CompactThreadRequest(threadId: threadId).writeToBuffer(),
      );

  /// 分叉会话 —— proto `ThreadService.ForkThread`。
  ///
  /// arguments：`ForkThreadRequest` 的 wire 字节（thread_id + last_turn_id；
  /// 空 = 从最新处分叉）。返回：`Thread` 的 wire 字节（**新会话**，原会话
  /// 不动——调用方拿到新 id 后按需跳转，不替用户切走）。
  ///
  /// 【可见性】`fork:<agentId>` 位（现仅 codex/pi 系实现；dsh 无位）。
  /// 【甲级写操作】调用方执行前有显式确认（fork 代价高，小屏一点就中）。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<Thread> forkThread(String threadId, {String lastTurnId = ''}) =>
      _invoke(
        'forkThread',
        ForkThreadRequest(threadId: threadId, lastTurnId: lastTurnId)
            .writeToBuffer(),
        _decodeThread('forkThread 返回值'),
      );

  /// 回滚 N 个回合 —— proto `ThreadService.RollbackThread`。
  ///
  /// arguments：`RollbackThreadRequest` 的 wire 字节（thread_id + num_turns，
  /// 必须 > 0，见 `chat_actions_data.dart` 的 [validateRollbackTurns]）。
  /// 返回：`Thread` 的 wire 字节（回滚后的会话真值，客户端不自己拼状态）。
  ///
  /// 【可见性】`rollback:<agentId>` 位（现仅 codex 实现）。
  /// 【甲级写操作】调用方执行前有显式确认 + 后果说明（只改对话记录，不改已
  /// 落盘的文件，见 [rollbackConsequence]）。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<Thread> rollbackThread(String threadId, int numTurns) => _invoke(
        'rollbackThread',
        RollbackThreadRequest(threadId: threadId, numTurns: numTurns)
            .writeToBuffer(),
        _decodeThread('rollbackThread 返回值'),
      );

  /// 设置会话目标 —— proto `ThreadService.SetThreadGoal`。
  ///
  /// arguments：`SetThreadGoalRequest` 的 wire 字节（thread_id + goal，空 goal
  /// 由调用方前置拦截，见 [validateThreadGoal]——空目标过桥等于许诺做不到的
  /// 语义）。返回：`Thread` 的 wire 字节（落库后读回来的真值，调用方按它刷
  /// 目标行，不拿入参冒充）。
  ///
  /// 【可见性】无能力位门控（SetThreadGoal 无 agent 隔离，daemon 侧全路由可达；
  /// pi 会话按 C3 `builtinSlashCommandsFor` 不同款——pi 无 goal 概念，调用方
  /// 仍可透传，daemon 侧按实际能力回错，不由客户端写死名单）。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<Thread> setThreadGoal(String threadId, String goal) => _invoke(
        'setThreadGoal',
        SetThreadGoalRequest(threadId: threadId, goal: goal).writeToBuffer(),
        _decodeThread('setThreadGoal 返回值'),
      );

  /// 发起计划审查 —— proto `ThreadService.StartReview`。
  ///
  /// arguments：`StartReviewRequest` 的 wire 字节（thread_id + focus；空 focus
  /// = daemon 按默认口径审）。返回：`StartReviewResponse` 的 wire 字节
  /// （review_id + started；fire-and-return：审查事件以 item/* 事件流回**原
  /// 会话**，调用方不轮询，Toast 一句即收）。
  ///
  /// 【可见性】`review:<agentId>` 位（现仅 codex：review/start 是 codex
  /// app-server 的对等能力）。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<StartReviewResponse> startReview(
    String threadId, {
    String focus = '',
  }) =>
      _invoke(
        'startReview',
        StartReviewRequest(threadId: threadId, focus: focus).writeToBuffer(),
        _decodeStartReview,
      );

  /// 导出 HTML 存档 —— proto `ThreadService.ExportThreadHtml`。
  ///
  /// arguments：`ExportThreadHtmlRequest` 的 wire 字节（thread_id）。返回：
  /// `ExportThreadHtmlResponse` 的 wire 字节（path：电脑侧落盘绝对路径；
  /// 手机端按 path 经 `ReadFile` 取文件，不拼 URL、不猜目录）。
  ///
  /// 【可见性】`export_html:<agentId>` 位（现仅 pi 提供；agentId 空 = 未知，
  /// 调用方按「无位」处理，不替 daemon 猜——见 [htmlExportGateOf]）。
  /// 【错误码】同 [submitApproval]（另见 [exportFailureMessage] 的人话分支）。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ExportThreadHtmlResponse> exportThreadHtml(String threadId) => _invoke(
        'exportThreadHtml',
        ExportThreadHtmlRequest(threadId: threadId).writeToBuffer(),
        _decodeExportThreadHtml,
      );

  /// 优化提示词 —— proto `ThreadService.OptimizePrompt`。
  ///
  /// arguments：`OptimizePromptRequest` 的 wire 字节（text + style + agent_id
  /// + model_id + effort；style 空 = 默认改写口径）。返回：
  /// `OptimizePromptResponse` 的 wire 字节（optimized：改写后的文本；空 =
  /// 无需改写，调用方如实展示，不回填空串吞掉用户原文）。
  ///
  /// 【只读语义】改写不影响任何会话状态（失败只 Toast，不摆 UI）。
  /// 【可见性】`text_generation:<agentId>` 位（daemon 侧 `TextGenerator`
  /// 接口能力的位化形态）。
  /// 【错误码】同 [submitApproval]。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<OptimizePromptResponse> optimizePrompt({
    required String text,
    String style = '',
    String agentId = '',
    String modelId = '',
    String effort = '',
  }) =>
      _invoke(
        'optimizePrompt',
        OptimizePromptRequest(
          text: text,
          style: style,
          agentId: agentId,
          modelId: modelId,
          effort: effort,
        ).writeToBuffer(),
        _decodeOptimizePrompt,
      );

  // ── 内部：调用 + 解码（与 PaBridge._invoke 同契约，见文件头） ──

  Future<T> _invoke<T>(
    String method,
    Object? arguments,
    T Function(Object? raw) decode,
  ) async {
    try {
      final Object? raw =
          await _channel.invokeMethod<Object?>(method, arguments);
      return decode(raw);
    } on PlatformException catch (e) {
      // 宿主 code 原样透传：code 是 UI 分支依据，改写等于让上层和宿主真实错误脱钩。
      throw PaBridgeException(e.code, e.message ?? '', details: e.details);
    } on MissingPluginException catch (e) {
      // 通道无 handler = 宿主桥未实现（B4 前）或宿主进程未起。如实上报，不伪造返回。
      throw PaBridgeException(
        PaBridgeErrorCodes.hostUnavailable,
        e.message ?? '宿主桥未注册：$kPaBridgeMethodChannel（B4 波次实现）',
      );
    }
  }

  Future<void> _invokeExpectNull(String method, Object? arguments) async {
    final Object? raw = await _invoke<Object?>(method, arguments, (r) => r);
    if (raw != null) {
      throw PaBridgeException(
        PaBridgeErrorCodes.decodeFailed,
        '$method 契约要求宿主回 null，实际 ${raw.runtimeType}',
      );
    }
  }

  ClearQueuedInputsResponse _decodeClearQueuedInputs(Object? raw) =>
      bytesOrThrow(
        'clearQueuedInputs 返回值',
        raw,
        'ClearQueuedInputsResponse',
        ClearQueuedInputsResponse.fromBuffer,
      );

  Thread Function(Object? raw) _decodeThread(String where) =>
      (Object? raw) => bytesOrThrow(
            where,
            raw,
            'Thread',
            Thread.fromBuffer,
          );

  StartReviewResponse _decodeStartReview(Object? raw) => bytesOrThrow(
        'startReview 返回值',
        raw,
        'StartReviewResponse',
        StartReviewResponse.fromBuffer,
      );

  ExportThreadHtmlResponse _decodeExportThreadHtml(Object? raw) => bytesOrThrow(
        'exportThreadHtml 返回值',
        raw,
        'ExportThreadHtmlResponse',
        ExportThreadHtmlResponse.fromBuffer,
      );

  OptimizePromptResponse _decodeOptimizePrompt(Object? raw) => bytesOrThrow(
        'optimizePrompt 返回值',
        raw,
        'OptimizePromptResponse',
        OptimizePromptResponse.fromBuffer,
      );
}
