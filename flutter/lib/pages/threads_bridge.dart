// Threads 真通道数据源（B3-3）：proto `Thread` wire → [ThreadSummary] 六字段映射
// + 走 [PaBridge.listThreads] 的生产数据源。
//
// 【为什么单独一个文件】本文件回答两件事：①「列表行的六个字段各自从 wire 的哪
// 个字段来」——纯函数，可被单测逐字段钉死；②「真通道怎么包」——
// [BridgeThreadsDataSource]。threads_data.dart 保持模型 + 筛选纯函数 + 内存 fake
// 的原有分工；页面只多一个默认数据源接线。三层各自可测，回归时能分清是映射错、
// 通道错还是渲染错。
//
// 【诚实红线（B4 前）】Kotlin 宿主桥 B4 波次才注册 `dev.pocketagent/bridge`
// （channel-contract.md §6）；在此之前 listThreads 必然抛 HOST_UNAVAILABLE。
// 这里把它如实翻成 [ThreadsLoadFailure]（文案点名「桌面端会话通道未就绪
// （B4 波次）」），页面进错误态。**绝不返回空列表冒充「没有会话」**：空态与
// 错误态是两条路径，test/pages/threads_bridge_test.dart 两条都钉死（红线：不留假绿）。
//
// 【为什么时间/状态映射照抄 Android】Kotlin 是同一 app 的既有实现（同一套词表
// 与档位纪律）：threadRowTime / threadAgentIdOf / lastTurnErrorSummary /
// editorialStatusOf 的口径逐条对齐，双端同一会话长同一样，避免第二份自造口径
// 慢慢漂移。

import '../components/pa_status_chip.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart';
import 'threads_data.dart';

// ── 单行时间标签 ────────────────────────────────────────────────────────────

/// 会话行的统一时间标签（纯函数）：刚刚 / N 分钟前 / 今天 HH:mm / 昨天 HH:mm /
/// MM-dd（跨年补 yyyy）。口径 = Android `threadRowTime`（ThreadsRowLogic.kt:67）。
///
/// 【为什么返回 String 而不是 DateTime】列表行只消费「人话标签」，格式化收口在
/// 映射层；[nowMs] 显式注入（生产传当前时钟）——否则「N 分钟前」不可单测，
/// 测试跑在不同的机器时钟上必飘。
///
/// 【边界】时间戳缺席（[unixMs] <= 0）返回空串（不显示假时间）；客户端时钟偏慢
/// （时间戳在未来）回落绝对日期档，不显示「负 N 分钟前」——与 Android 同纪律。
String threadRowTimeLabel(int unixMs, int nowMs) {
  if (unixMs <= 0) return '';
  final int diff = nowMs - unixMs;
  if (diff >= 0) {
    if (diff < 60000) return '刚刚';
    if (diff < 3600000) return '${diff ~/ 60000} 分钟前';
  }
  final DateTime t = DateTime.fromMillisecondsSinceEpoch(unixMs);
  final DateTime now = DateTime.fromMillisecondsSinceEpoch(nowMs);
  final DateTime dayStart = DateTime(t.year, t.month, t.day);
  final DateTime todayStart = DateTime(now.year, now.month, now.day);
  // 用「当天零点」差换算天数并四舍五入：跨夏令时的 23/25 小时天不会被整除截断
  // （与 Android Math.round 同款换算）。
  final int dayDiff =
      ((todayStart.millisecondsSinceEpoch - dayStart.millisecondsSinceEpoch) /
              86400000)
          .round();
  if (dayDiff == 0) return '今天 ${_hhmm(t)}';
  if (dayDiff == 1) return '昨天 ${_hhmm(t)}';
  if (t.year == now.year) return _mmdd(t);
  return '${t.year}-${_mmdd(t)}';
}

String _hhmm(DateTime t) =>
    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

String _mmdd(DateTime t) =>
    '${t.month.toString().padLeft(2, '0')}-${t.day.toString().padLeft(2, '0')}';

// ── 状态映射 ────────────────────────────────────────────────────────────────

/// proto `ThreadStatus`（+ 审批数 + 最后错误）→ 设计系统五态 chip。
///
/// 【优先级照抄 Android `editorialStatusOf`（EditorialStatus.kt）】
/// 审批 > 运行 > 失败 > 完成：
///   - 挂着审批（`pending_approvals` 非空）或 WAITING_APPROVAL → 等人（pending）；
///   - RUNNING → running；
///   - IDLE 且有 `last_turn_error` → failed（中断就是失败，不美化）；
///   - UNSPECIFIED（daemon 未上报 / 未知枚举值被 proto3 折进 unknown fields）→
///     failed 档：Android 对 UNSPECIFIED 同为最保守档「未完成」；本页五词表没有
///     「未完成」，落到同色系的「失败」——**不冒充「已完成」**（宁可让人点进去看，
///     也不把「不知道」画成「成功」）；
///   - 其余（IDLE 无错 / ARCHIVED）→ done（ARCHIVED 与 Android else 档一致；
///     归档会话默认不经查询出现，见 proto `ListThreadsRequest.include_archived`）。
PAStatusKind threadStatusKindOf({
  required ThreadStatus status,
  required int pendingApprovals,
  required String lastTurnError,
}) {
  if (pendingApprovals > 0 ||
      status == ThreadStatus.THREAD_STATUS_WAITING_APPROVAL) {
    return PAStatusKind.pending;
  }
  if (status == ThreadStatus.THREAD_STATUS_RUNNING) {
    return PAStatusKind.running;
  }
  if (status == ThreadStatus.THREAD_STATUS_IDLE &&
      lastTurnError.trim().isNotEmpty) {
    return PAStatusKind.failed;
  }
  if (status == ThreadStatus.THREAD_STATUS_UNSPECIFIED) {
    return PAStatusKind.failed;
  }
  return PAStatusKind.done;
}

// ── 行字段映射 ──────────────────────────────────────────────────────────────

/// 标题兜底：空标题显示「(未命名)」（Android ThreadsScreen.kt:1988 同款，
/// 不摆空白行）。非空标题原样透传，不做任何加工。
String threadRowTitle(String title) => title.isEmpty ? '(未命名)' : title;

/// agent 归属：空串按 proto/store 兼容规则落回 "codex"（thread.proto:506
/// 「老记录缺省 = codex」，daemon `threadAgentID` 与 Android `threadAgentIdOf`
/// 同一口径）；非空原样透传——未知 agent id 不做白名单、不进缩写表。
String threadAgentIdOf(String agentId) => agentId.isEmpty ? 'codex' : agentId;

/// 最后错误摘要（纯函数）：取第一个非空行、裁到 [maxLen] 字加省略号。
/// 口径 = Android `lastTurnErrorSummary`（ThreadStatusBadge.kt:48）——daemon 的
/// `last_turn_error` 常带方法名前缀与多行，列表行只放得下人话首行。
String lastTurnErrorSummary(String raw, {int maxLen = 40}) {
  String firstLine = '';
  for (final String line in raw.split('\n')) {
    final String trimmed = line.trim();
    if (trimmed.isNotEmpty) {
      firstLine = trimmed;
      break;
    }
  }
  if (firstLine.isEmpty) return '';
  return firstLine.length <= maxLen ? firstLine : '${firstLine.substring(0, maxLen)}…';
}

/// 行副文案（纯函数）：
///   - 失败档且有 `last_turn_error` → 错误首行摘要（不用点进会话就能看到原因，
///     Android ThreadsScreen.kt:2010 同款优先级）；
///   - 否则 `message_count > 0` → 「N 条消息」事实陈述；
///   - 都没有 → 空串（不造占位文案）。
String threadRowSummary(PAStatusKind kind, String lastTurnError, int messageCount) {
  if (kind == PAStatusKind.failed) {
    final String err = lastTurnErrorSummary(lastTurnError);
    if (err.isNotEmpty) return err;
  }
  if (messageCount > 0) return '$messageCount 条消息';
  return '';
}

// ── proto Thread → ThreadSummary ────────────────────────────────────────────

/// 单条 proto `Thread` → [ThreadSummary]（六字段来源见各行的表达式注解）。
///
/// [nowMs] 显式注入：时间标签是相对当前时钟的格式化结果（见 [threadRowTimeLabel]）。
ThreadSummary threadSummaryOf(Thread t, {required int nowMs}) {
  final PAStatusKind kind = threadStatusKindOf(
    status: t.status,
    pendingApprovals: t.pendingApprovals.length,
    lastTurnError: t.lastTurnError,
  );
  return ThreadSummary(
    // id(1)：会话 id（`chat/{threadId}` 的 threadId）
    id: t.id,
    // title(2)：自动摘要标题；空 → 「(未命名)」占位
    title: threadRowTitle(t.title),
    // agent_id(16)：归属 agent；空 → "codex"（老记录兼容）
    agentId: threadAgentIdOf(t.agentId),
    // summary：无专字段——失败档取 last_turn_error(20) 首行摘要，否则
    // message_count(9) 的「N 条消息」（口径与 Android 行渲染同源）
    summary: threadRowSummary(kind, t.lastTurnError, t.messageCount),
    // time：updated_at_unix_ms(8) 格式化成行标签（Android threadRowTime）
    time: threadRowTimeLabel(t.updatedAtUnixMs.toInt(), nowMs),
    // status：ThreadStatus(6) + pending_approvals(21) + last_turn_error(20)
    status: kind,
  );
}

/// `ListThreadsResponse` → 行列表（纯函数；顺序保持 daemon 原序，不重排）。
List<ThreadSummary> threadSummariesOfResponse(
  ListThreadsResponse response, {
  required int nowMs,
}) =>
    response.threads
        .map((Thread t) => threadSummaryOf(t, nowMs: nowMs))
        .toList(growable: false);

// ── 生产数据源 ──────────────────────────────────────────────────────────────

/// 错误码 → 页面人话文案（页面错误态直接展示这一条）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名「会话通道未就绪（B4 波次）」
/// 让用户知道不是自己操作错了，也不是「没有会话」。其余码原样透传宿主 message
/// （不改写、不吞——与 settings 页同一套透传纪律）；宿主没给 message 时退 code，
/// 宁可难看也不吞。
String threadsLoadErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端会话通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '会话列表暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

/// 真通道数据源：`PaBridge.listThreads()` → `ListThreadsResponse` 字节已在
/// transport 层验形（Uint8List + proto 反序列化，坏字节抛 DECODE_FAILED）——
/// 这里只做「响应字节已成型之后」的两件事：错误 → [ThreadsLoadFailure] 人话；
/// 成功 → 六字段映射。
///
/// [bridge] 与 [nowMs] 都是注入点：测试按通道名 mock 宿主 wire 行为（PaBridge
/// 全程真实），并用固定时钟钉死时间标签。
class BridgeThreadsDataSource implements ThreadsDataSource {
  BridgeThreadsDataSource({PaBridge? bridge, int Function()? nowMs})
      : _bridge = bridge ?? PaBridge(),
        _nowMs = nowMs ?? _systemNowMs;

  static int _systemNowMs() => DateTime.now().millisecondsSinceEpoch;

  final PaBridge _bridge;
  final int Function() _nowMs;

  @override
  Future<List<ThreadSummary>> load() async {
    final ListThreadsResponse response;
    try {
      response = await _bridge.listThreads();
    } on PaBridgeException catch (e) {
      // 页面错误态只认一条人话原因（ThreadsLoadFailure）：「通道未就绪」与
      // 「宿主/解码出错」都在错误态这一条路径上，具体是哪种由文案说清
      // （HOST_UNAVAILABLE 点名 B4）。**不在这里回退成空列表**：那会把错误
      // 粉饰成「没有会话」（假空态），两条路径从此再也分不开。
      throw ThreadsLoadFailure(threadsLoadErrorText(e));
    }
    return threadSummariesOfResponse(response, nowMs: _nowMs());
  }
}
