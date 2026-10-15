// Chat 历史纯函数层（B3 第 5 波子步②）：历史 → 回放帧映射 + 按轮分页口径 +
// 数据源抽象（真通道包一层 + 内存 fake）。
//
// 【为什么单拆一个文件】映射与分页是可被单测直接命中的纯逻辑；混进 widget
// 文件会让「映射错还是渲染错」分不清（三层各自可测：映射错 / 通道错 / 渲染错）。
//
// ── 搜索口径裁决（本波不接 `SearchThreadMessages`，理由三条） ──
// 1. 本波页面无搜索 UI：daemon 侧 `HistorySearchRepository` / 快速导航浮层
//    （HistoryQuickNav）属周边波次④的配套，没有消费面的方法是死契约；
// 2. 搜索语义需要 UI 配合才诚实：回包 `scope`（full_thread 全量 /
//    local_snapshot 降级）与 `has_more` 分页位必须有对应的展示（"搜了 N 条、
//    还有更多"），现在接等于许诺做不到的 UX；
// 3. 只增不改：需要时在 chat_history_transport.dart 按 `getThreadHistory`
//    同模式加 `searchThreadMessages`（请求字节 → 响应字节 → bytesOrThrow），
//    不动既有签名。本波历史页内搜索不需要——行渲染后本地过滤即可，
//    不为不存在的输入引入服务端依赖。

import '../gen/pocketagent/v1/common.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart'
    show PaBridgeErrorCodes, PaBridgeException;
import 'chat_history_transport.dart';

// ── 历史 → 回放帧 ──────────────────────────────────────────────────────

/// 回放帧的 sidecar 元数据：`TurnServerMessage` 里放不下的那一半。
///
/// 【为什么需要它】`TurnServerMessage` 无 role/时间戳字段（live 流不需要：
/// 时间线靠到达顺序与 turn 边界定位），历史回放必须把「谁说的、何时、
/// 哪条、哪轮」带在边上，否则恢复进 live 时间线后排不回原序、也标不出
/// 发送人。`frames[i]` 与 `metas[i]` 下标一一对齐（跳过的块不产生帧也不
/// 产生元数据，见 [historyToFrames] 的图片块注释）。
class HistoryFrameMeta {
  const HistoryFrameMeta({
    required this.role,
    required this.createdAtMs,
    required this.messageId,
    required this.turnId,
  });

  /// 消息角色（`ThreadMessage.role` 原样透传：user / assistant；未知值不改写）。
  final String role;

  /// `ThreadMessage.created_at_unix_ms`（0 = daemon 未带，按未知处理）。
  final int createdAtMs;

  /// `ThreadMessage.message_id`（稳定身份；空 = 旧 daemon 未带，定位退回顺序）。
  final String messageId;

  /// `ThreadMessage.turn_id`（所属轮次；空 = 未知）。
  final String turnId;

  @override
  bool operator ==(Object other) =>
      other is HistoryFrameMeta &&
      other.role == role &&
      other.createdAtMs == createdAtMs &&
      other.messageId == messageId &&
      other.turnId == turnId;

  @override
  int get hashCode => Object.hash(role, createdAtMs, messageId, turnId);
}

/// 历史响应 → 回放帧序列（纯函数，供 C1 `TurnSession.replayFrames` 消费）。
///
/// 【字段映射表】（逐条注释，来源均为 proto/v1/thread.proto 与 common.proto）：
///
/// | 历史侧（ThreadMessage）            | 回放侧（TurnServerMessage）                         |
/// |-----------------------------------|-----------------------------------------------------|
/// | `blocks[i].text.text`（common 1）  | `textDelta.delta` 原样（thread 2）：流式增量的终局形态就是全文，一帧即全量 |
/// | `blocks[i].thinking.text`（common 5）| `thinkingDelta.snapshot`（thread 3）：snapshot = 权威全文、收到即整体替换（ThinkingDelta 文档语义）；历史回放一次性携带，走 snapshot 不走 delta；`signature` 展示无用，丢弃 |
/// | `blocks[i].toolCall`（common 3）   | `toolCall = ToolCallUpdate(phase: STARTED, call: 原样)`（thread 4）：id/name/jsonArgs 逐字透传 |
/// | `blocks[i].toolResult`（common 4） | `toolCall = ToolCallUpdate(phase: FINISHED, result: 原样)`（thread 4）：callId/ok/output/elapsedMs 逐字透传；UPDATE（累积进度）是 live 专属，历史只留终局 |
/// | `blocks[i].image`（common 2）      | 跳过：`TurnServerMessage` 无图片载体帧，宁可少显示不错显示（与 Android ThreadSnapshot.kt:29「只留 role+text」同纪律）；也不产生 sidecar（下标对齐不断） |
/// | `role`（1）                        | 不进帧（帧无该字段）→ 进 `metas[i].role`，见 [HistoryFrameMeta] |
/// | `created_at_unix_ms`（3）          | 不进帧 → `metas[i].createdAtMs` |
/// | `message_id`（4）/ `turn_id`（5）   | 不进帧 → `metas[i]`（稳定身份与轮次归属，回放排重与跳转用） |
/// | `sequence`                         | 历史帧恒 0 = 无序号（thread.proto:660-664「0 = 无序号，旧 daemon / 未入事件库，按无续传能力处理」）；续传水位是 live 事件流的事，不由历史帧冒充 |
///
/// 【顺序】响应原序逐条展开（daemon 已按时间升序给，客户端不重排）。
/// 【空消息】blocks 为空或全被跳过（纯图片）时仍占一帧空 `TextDelta`（对应
/// Android `historyMessageToChatItems` 的空占位 `ChatItem(role)`）：顺序与
/// 轮数不塌，渲染侧按角色画空行占位，不凭空吞消息。
List<TurnServerMessage> historyToFrames(GetThreadHistoryResponse response) {
  final List<TurnServerMessage> frames = <TurnServerMessage>[];
  for (final ThreadMessage m in response.messages) {
    bool emitted = false;
    for (final ContentBlock b in m.blocks) {
      final TurnServerMessage? frame = _frameOfBlock(b);
      if (frame != null) {
        frames.add(frame);
        emitted = true;
      }
    }
    if (!emitted) {
      frames.add(TurnServerMessage(textDelta: TextDelta(delta: '')));
    }
  }
  return frames;
}

/// 与 [historyToFrames] 下标对齐的元数据（跳过的块两边都不产生条目）。
List<HistoryFrameMeta> historyFrameMetas(GetThreadHistoryResponse response) {
  final List<HistoryFrameMeta> metas = <HistoryFrameMeta>[];
  for (final ThreadMessage m in response.messages) {
    bool emitted = false;
    for (final ContentBlock b in m.blocks) {
      if (_frameOfBlock(b) != null) {
        metas.add(HistoryFrameMeta(
          role: m.role,
          createdAtMs: m.createdAtUnixMs.toInt(),
          messageId: m.messageId,
          turnId: m.turnId,
        ));
        emitted = true;
      }
    }
    if (!emitted) {
      metas.add(HistoryFrameMeta(
        role: m.role,
        createdAtMs: m.createdAtUnixMs.toInt(),
        messageId: m.messageId,
        turnId: m.turnId,
      ));
    }
  }
  return metas;
}

/// 单块 → 单帧（null = 该块无载体，跳过；目前仅图片块）。
TurnServerMessage? _frameOfBlock(ContentBlock b) {
  switch (b.whichBlock()) {
    case ContentBlock_Block.text:
      return TurnServerMessage(textDelta: TextDelta(delta: b.text.text));
    case ContentBlock_Block.thinking:
      return TurnServerMessage(
          thinkingDelta: ThinkingDelta(snapshot: b.thinking.text));
    case ContentBlock_Block.toolCall:
      return TurnServerMessage(
          toolCall: ToolCallUpdate(
              phase: ToolCallUpdate_Phase.STARTED, call: b.toolCall));
    case ContentBlock_Block.toolResult:
      return TurnServerMessage(
          toolCall: ToolCallUpdate(
              phase: ToolCallUpdate_Phase.FINISHED, result: b.toolResult));
    case ContentBlock_Block.image:
    case ContentBlock_Block.notSet:
      return null;
  }
}

/// 单条历史消息的纯文本（快照落盘 + 简化行渲染共用）。
///
/// 【为什么只拼文本块】与 Android `ThreadSnapshot.Msg` 同口径：快照是只读
/// 副本，思考/工具调用参数是过程不是正文，图片离线不可还原——都跳过；
/// 多个文本块按换行拼（块边界保留，不断句）。
String historyTextOf(ThreadMessage message) {
  final List<String> parts = <String>[];
  for (final ContentBlock b in message.blocks) {
    if (b.whichBlock() == ContentBlock_Block.text) {
      parts.add(b.text.text);
    }
  }
  return parts.join('\n');
}

/// 角色人话标签（简化行渲染用；未知 role 原样透传，不建白名单）。
String historyRoleLabel(String role) {
  switch (role) {
    case 'user':
      return '我';
    case 'assistant':
      return '助手';
    default:
      return role;
  }
}

/// 本次响应覆盖的轮数估计（一轮 = 一条 user 起始，daemon 按轮截断时尾部必落
/// 轮边界；见 Android `HistoryRoundsLogic` 文件头——单看首条分不出截断与
/// 天生开头，`canLoadEarlier` 的两个条件合取仍是估计，误显示一次也幂等）。
int historyRoundCount(GetThreadHistoryResponse response) {
  int count = 0;
  for (final ThreadMessage m in response.messages) {
    if (m.role == 'user') count++;
  }
  return count;
}

// ── 按轮分页口径（Android HistoryRoundsLogic.kt 的 Dart 移植） ──────────

/// 设置页可选档位（顺序即循环顺序，与 Android `STEP_OPTIONS` 同值）。
const List<int> kHistoryRoundOptions = <int>[10, 20, 30, 50];

/// 默认档位（未设置过偏好时的取值；请求时由客户端显式传，服务端不内置默认，
/// 见 thread.proto:453-455 注释）。
const int kDefaultHistoryRounds = 20;

/// 任意输入收敛成合法档位：档位表里原样返回，否则回默认（prefs 脏值保护）。
int clampHistoryRounds(int v) =>
    kHistoryRoundOptions.contains(v) ? v : kDefaultHistoryRounds;

/// 时间线顶部是否显示「加载更早」入口。
///
/// [firstRole] 本次响应首条消息的 role；[roundCount] 本次返回覆盖的轮数
/// （[historyRoundCount]）；[requestedRounds] 本次请求的轮数，
/// 0 = 全量拉取（导出/恢复/resync 路径），全量路径永远不显示入口。
bool canLoadEarlier({
  required String? firstRole,
  required int roundCount,
  required int requestedRounds,
}) {
  if (requestedRounds <= 0) return false;
  if (roundCount < requestedRounds) return false;
  return firstRole == 'user';
}

/// 「加载更早」下一次请求的轮数：当前累计 + 一个档位步长。
int nextHistoryRounds(int current, int step) => current + step.clamp(1, 1 << 30);

// ── 数据源抽象 ────────────────────────────────────────────────────────

/// 加载失败：只带人话原因，不吞原始错误类型（与 `ThreadsLoadFailure` 同纪律）。
class ChatHistoryLoadFailure implements Exception {
  const ChatHistoryLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'ChatHistoryLoadFailure: $message';
}

/// 错误码 → 页面人话文案（页面错误态直接展示这一条）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名「历史通道未就绪
/// （B4 波次）」，让用户知道不是自己操作错了，也不是「没有历史」。其余码
/// 原样透传宿主 message（不改写、不吞）；宿主没给 message 时退 code。
String chatHistoryErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端会话历史通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '会话历史暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

/// 历史数据源抽象 —— 「数据从哪来」只有这一个口子（页面只认它，不认通道）。
abstract class ChatHistorySource {
  /// 拉一次 [threadId] 的历史；失败以 [ChatHistoryLoadFailure] 上抛。
  Future<GetThreadHistoryResponse> loadHistory(String threadId);
}

/// 真通道数据源：`ChatHistoryTransport.getThreadHistory` → 成功原样回响应。
///
/// [rounds] 首屏请求的轮数（默认最近 20 轮，Android 显式传 20 的同款；
/// limit 恒 0 = 不按条数截，轮截断已收窄）。
class BridgeChatHistorySource implements ChatHistorySource {
  BridgeChatHistorySource({
    ChatHistoryTransport? transport,
    this._rounds = kDefaultHistoryRounds,
  }) : _transport = transport ?? ChatHistoryTransport();

  final ChatHistoryTransport _transport;
  final int _rounds;

  @override
  Future<GetThreadHistoryResponse> loadHistory(String threadId) async {
    try {
      return await _transport.getThreadHistory(threadId, rounds: _rounds);
    } on PaBridgeException catch (e) {
      // 页面错误态只认一条人话：未就绪与宿主/解码出错都在错误态这条路径，
      // **不在这里回退成空响应**：那会把错误粉饰成「没有历史」（假空态）。
      throw ChatHistoryLoadFailure(chatHistoryErrorText(e));
    }
  }
}

/// 内存 fake 数据源 —— **不是真历史**，测试注入与离线预览用。
class FakeChatHistorySource implements ChatHistorySource {
  FakeChatHistorySource({GetThreadHistoryResponse? response, this.failure})
      : _response = response ?? GetThreadHistoryResponse();

  GetThreadHistoryResponse _response;
  set response(GetThreadHistoryResponse v) => _response = v;

  /// 非空时 `loadHistory` 如实抛错（验证错误态与重试，不是装饰）。
  String? failure;

  /// 已请求的 threadId（按调用序记录，测试断言「到底问了谁」用）。
  final List<String> requested = <String>[];

  @override
  Future<GetThreadHistoryResponse> loadHistory(String threadId) async {
    requested.add(threadId);
    if (failure != null) {
      throw ChatHistoryLoadFailure(failure!);
    }
    return _response;
  }
}
