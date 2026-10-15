// Chat 历史方法面（B3 第 5 波子步②）：proto `ThreadService.GetThreadHistory`
// （proto/v1/thread.proto:39）的 Dart 封装。
//
// 【为什么单拆一个文件】`pa_bridge.dart` 是 B2 冻结契约（本波只读，见下），
// 新增方法面若直接改它等于动冻结文件；本文件只回答「历史请求字节怎么组、
// 响应字节怎么验」，错误模型与解码收口复用 `pa_bridge.dart` 的现成件。
//
// 【与 pa_bridge.dart 的关系】只读消费三件套，不改它一行：
//   - `kPaBridgeMethodChannel`：同一条 MethodChannel（宿主 B4 按此名注册）；
//   - `bytesOrThrow`：响应验形 + 反序列化唯一收口（坏字节 → DECODE_FAILED）；
//   - `PaBridgeErrorCodes` / `PaBridgeException`：错误码与异常形状（UI 按 code 分支）。
// `_invoke` 的 try/catch 形状在这里抄了 10 行（PlatformException 透传 +
// MissingPluginException → HOST_UNAVAILABLE）：跨文件的私有方法复用不了，
// 抄的是错误语义逐字，不是新口径。
//
// 【宿主 B4 实现】Kotlin 宿主桥 B4 波次注册 `getThreadHistory`：解析
// `GetThreadHistoryRequest` → 转发 daemon `GetThreadHistory` →
// `GetThreadHistoryResponse` 字节回传。在此之前如实抛 HOST_UNAVAILABLE，
// 页面进「通道未就绪」错误态，绝不伪装成空历史。
//
// 【SearchThreadMessages 为什么不在本波】见 chat_history_data.dart 文件头的
// 「搜索口径裁决」：本波页面无搜索 UI，单加方法是死契约；需要时按本文件同
// 模式（请求字节 → 响应字节 → bytesOrThrow）只增一个方法，不动既有签名。

import 'package:flutter/services.dart';

import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart'
    show
        PaBridgeErrorCodes,
        PaBridgeException,
        bytesOrThrow,
        kPaBridgeMethodChannel;

/// 历史方法面：`getThreadHistory` 的 Dart 形态（可注入构造，方便测试隔离）。
///
/// 【为什么可注入而不是单例】与 `PaBridge` 同纪律：测试按通道名 mock 宿主侧
/// handler，用独立通道与生产隔离；生产用默认名即可。
class ChatHistoryTransport {
  ChatHistoryTransport({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(kPaBridgeMethodChannel);

  final MethodChannel _channel;

  /// 拉一次会话历史 —— proto `ThreadService.GetThreadHistory`。
  ///
  /// arguments：`GetThreadHistoryRequest` 的 wire 字节（thread_id + limit +
  /// rounds；limit = 0 且 rounds = 0 即「全量」，兼容旧客户端语义，
  /// 见 thread.proto:447-456 —— 导出全量与断流恢复回填都依赖这个语义）。
  /// 返回：`GetThreadHistoryResponse` 的 wire 字节 → gen stub 反序列化
  /// （messages[] 逐条是 proto `ThreadMessage`）。
  ///
  /// 【limit 与 rounds 并存】先按 limit 尾截，再按 rounds 叠加收窄（daemon 侧
  /// 口径，thread.proto:453）；首屏调用方显式传 rounds（见
  /// `BridgeChatHistorySource` 默认最近 20 轮），服务端不内置默认。
  /// 【错误码】`NOT_CONNECTED` / `AUTH_REQUIRED` / `INTERNAL`（宿主）+
  /// `HOST_UNAVAILABLE` / `DECODE_FAILED`（Dart）。宿主缺席时如实抛
  /// HOST_UNAVAILABLE——这就是 B4 前历史页的诚实占位路径。
  /// 【Kotlin 宿主桥 B4 波次实现】
  Future<GetThreadHistoryResponse> getThreadHistory(
    String threadId, {
    int limit = 0,
    int rounds = 0,
  }) async {
    final Object? raw;
    try {
      raw = await _channel.invokeMethod<Object?>(
        'getThreadHistory',
        GetThreadHistoryRequest(
          threadId: threadId,
          limit: limit,
          rounds: rounds,
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
      'getThreadHistory 返回值',
      raw,
      'GetThreadHistoryResponse',
      GetThreadHistoryResponse.fromBuffer,
    );
  }
}
