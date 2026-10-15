// Chat 输入区 transport 面（B3 第 5 波③）：proto
// `ThreadService.ListSlashCommands`（proto/v1/thread.proto:106）的 Dart 封装。
//
// 【为什么单拆一个文件】`pa_bridge.dart` 是 B2 冻结契约（本波只读），新增
// 方法面若直接改它等于动冻结文件；本文件只回答「斜杠命令请求字节怎么组、
// 响应字节怎么验」，错误模型与解码收口复用 `pa_bridge.dart` 的现成件（与
// `chat_history_transport.dart` 同模式）。
//
// 【宿主 B4 实现】Kotlin 宿主桥 B4 波次注册 `listSlashCommands`：解析
// `ListSlashCommandsRequest` → 按 agent_id 自动路由 daemon
// `ListSlashCommands`（zcode=会话快照 slashCommands[]；codex=skills 映射；
// pi=get_commands 真名映射）→ `ListSlashCommandsResponse` 字节回传。在此之前
// 如实抛 HOST_UNAVAILABLE，面板退回内置伪命令兜底，绝不伪装成空命令表。
//
// 【附件上传面判定口径（2026-10-15 全仓 grep）】proto/v1 与 android 全仓无
// `UploadAttachment`/`AttachFile` 类 RPC——附件**不走独立上传 RPC**：发送时
// 走 `TurnInput` blocks 内联（文本块附 `[附件已上传到电脑: pcPath]` 说明 +
// 图片块 `ImageBlock{data, media_type}`），与 Android
// `TurnSession.buildTurnInput` 同形，随 `turnStreamSend` 首帧/追问帧发出。
// 故本文件**不编新 RPC**；`TurnInput` 组装是 C4 的事（composer 只交出原文 +
// 受控附件表，见 chat_composer.dart）。

import 'package:flutter/services.dart';

import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart'
    show
        PaBridgeErrorCodes,
        PaBridgeException,
        bytesOrThrow,
        kPaBridgeMethodChannel;

/// 输入区方法面：`listSlashCommands` 的 Dart 形态（可注入构造，方便测试隔离）。
///
/// 【为什么可注入而不是单例】与 `PaBridge` 同纪律：测试按通道名 mock 宿主侧
/// handler，用独立通道与生产隔离；生产用默认名即可。
class ChatInputTransport {
  ChatInputTransport({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(kPaBridgeMethodChannel);

  final MethodChannel _channel;

  /// 拉一次本会话可用的斜杠命令 —— proto `ThreadService.ListSlashCommands`。
  ///
  /// arguments：`ListSlashCommandsRequest` 的 wire 字节（thread_id 必填；
  /// refresh = true 强制跳过 daemon 侧 10 分钟 TTL 缓存重拉——日常面板只走
  /// 缓存，进会话/下拉刷新才置 true，与 Android `SlashCommandCache` 分工同款）。
  /// 返回：`ListSlashCommandsResponse` 的 wire 字节 → gen stub 反序列化
  /// （commands[] 逐条是 proto `SlashCommand`：name/description/input_hint；
  /// 空 = 非面板会话或快照暂不可用，调用方不显示 daemon 候选——与「空列表是
  /// 真空态」纪律一致，不拿空冒充错误）。
  ///
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——B4 前斜杠面板走内置伪命令兜底路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<ListSlashCommandsResponse> listSlashCommands(
    String threadId, {
    bool refresh = false,
  }) async {
    final Object? raw;
    try {
      raw = await _channel.invokeMethod<Object?>(
        'listSlashCommands',
        ListSlashCommandsRequest(
          threadId: threadId,
          refresh: refresh,
        ).writeToBuffer(),
      );
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
    return bytesOrThrow(
      'listSlashCommands 返回值',
      raw,
      'ListSlashCommandsResponse',
      ListSlashCommandsResponse.fromBuffer,
    );
  }
}
