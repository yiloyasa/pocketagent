// Threads 列表的模型 + 三维筛选纯函数 + 内存 fake 数据源。
//
// 【为什么单拆一个文件】threads.dart 只留界面（三态 + 行渲染），模型与筛选是
// 可被单测直接命中的纯逻辑；混进 widget 文件会让「筛选逻辑单测」被迫拖着
// MaterialApp 跑，回归时分不清是逻辑错还是渲染错。
//
// 【数据源现状（B3-3）】生产默认已切到真通道 BridgeThreadsDataSource
// （threads_bridge.dart：PaBridge listThreads → proto Thread 六字段映射）。
// 本文件的 [FakeThreadsDataSource] 保留给测试注入与离线预览——fake 就是 fake：
// 它从不冒充生产数据源，页面也只在被显式注入时才会用到它。

import '../components/pa_status_chip.dart';

/// 会话列表行 —— page-contracts §1.3 Threads 契约的投影。
///
/// 【为什么字段这么少】本波只做列表展示与三维筛选，六个字段是行渲染 + 筛选的
/// 最小闭集（id/title/agentId/摘要/时间/状态）。写操作（重命名/归档/分组…）与
/// 分组/归档维度要等 ThreadService 接线，届时字段可增，但**不许在这里预埋用不到
/// 的猜测字段**（没有证据的字段 = 假契约）。
class ThreadSummary {
  const ThreadSummary({
    required this.id,
    required this.title,
    required this.agentId,
    required this.summary,
    required this.time,
    required this.status,
  });

  /// 会话 id（`chat/{threadId}` 的 threadId）。
  final String id;

  final String title;

  /// agent 标识（能力位 `agent:<id>` 的 `<id>`：codex/pi/zcode/dsh/hermes…）。
  ///
  /// 值域由宿主能力位决定，Dart 侧不建白名单——未知 agentId 原样进头像与筛选条。
  final String agentId;

  /// 最后一条消息摘要。
  final String summary;

  /// 展示用时间（Kotlin 侧已格式化好的字符串，如 `14:02` / `昨天` / `10-12`）。
  final String time;

  /// 行状态（复用设计系统五态 chip，组件与筛选共用同一映射）。
  final PAStatusKind status;

  @override
  bool operator ==(Object other) =>
      other is ThreadSummary &&
      other.id == id &&
      other.title == title &&
      other.agentId == agentId &&
      other.summary == summary &&
      other.time == time &&
      other.status == status;

  @override
  int get hashCode => Object.hash(id, title, agentId, summary, time, status);
}

/// 三维筛选（交集）—— 对应 Kotlin `ThreadsScreen` 的 `agentFilter` ×
/// `statusFilter` × `query`（§1.3「三维筛选（交集）」）。
///
/// 【为什么三颗字段都可空/可空串】`null`/空 = 该维度不参与筛选（「全部」档），
/// 页面不必为了表达「不过滤」再造哨兵值；三维度是 AND 关系，逐维收紧。
///
/// 【为什么不在这里放「全部/归档/某分组」】那一维（`filter`）依赖分组与归档数据，
/// 属 ThreadService 波次；本波没有数据源能回答它，先加就是死字段。
class ThreadFilter {
  const ThreadFilter({this.agentId, this.status, this.query = ''});

  /// 无筛选（三维度全放行）。
  static const ThreadFilter none = ThreadFilter();

  /// agent 维度；`null` = 全部 agent。
  final String? agentId;

  /// 状态维度；`null` = 全部状态。
  final PAStatusKind? status;

  /// 本地搜索关键词；空串 = 不搜索。
  final String query;

  /// 是否处于「未筛选」档（用于区分空态文案：没数据 vs 被筛没了）。
  bool get isNone => agentId == null && status == null && query.trim().isEmpty;
}

/// 纯函数：按 [filter] 三维度 AND 筛出匹配行（顺序保持原列表序）。
///
/// 【为什么搜索只打 title/summary】Kotlin `matchesThreadSearch` 是纯本地搜索，
/// 目标是「找回会话」：标题与摘要是人记得的内容；打 id 进搜索只会命中机器串，
/// 假装支持了人不会输的东西。大小写不敏感用 `toLowerCase()`（不做 locale 分支，
/// 避免为不存在的输入引入名单）。
List<ThreadSummary> applyThreadFilter(
  List<ThreadSummary> threads,
  ThreadFilter filter,
) {
  final String? agent = filter.agentId;
  final PAStatusKind? status = filter.status;
  final String query = filter.query.trim().toLowerCase();
  if (agent == null && status == null && query.isEmpty) {
    return threads;
  }
  return threads.where((ThreadSummary t) {
    if (agent != null && t.agentId != agent) return false;
    if (status != null && t.status != status) return false;
    if (query.isNotEmpty &&
        !(t.title.toLowerCase().contains(query) ||
            t.summary.toLowerCase().contains(query))) {
      return false;
    }
    return true;
  }).toList(growable: false);
}

/// 列表数据源抽象 —— 三维筛选之外的「数据从哪来」只有这一个口子。
///
/// 【为什么抽象成接口而不是直接写死 fake】B3 后续波次接 ThreadService
/// （`listThreads` 本地导入态 / `listCodexThreads` / `listAgentThreads` /
/// `listThreadSections` 四源 + `ThreadSnapshot` 离线兜底）时，页面构造注入即可，
/// 三态与筛选逻辑不用动。
abstract class ThreadsDataSource {
  /// 拉一次列表；失败以 [ThreadsLoadFailure] 上抛（页面据此进错误态）。
  Future<List<ThreadSummary>> load();
}

/// 加载失败：只带人话原因，不吞原始错误类型。
class ThreadsLoadFailure implements Exception {
  const ThreadsLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'ThreadsLoadFailure: $message';
}

/// 内存 fake 数据源 —— **不是真会话**，测试注入与离线预览用。
///
/// 【为什么可 const 且零延迟】没有后台轮询/分页，加载即完成：立即完成的
/// Future 让 `pumpAndSettle` 能收敛（引入假延时只会让 widget 测试多一个
/// 必须显式推进的时钟）。需要测加载态时，测试侧注入带 Completer 的源。
///
/// 【失败注入】[failure] 非空时 `load()` 如实抛错，用来验证错误态与重试，
/// 不是装饰。
class FakeThreadsDataSource implements ThreadsDataSource {
  const FakeThreadsDataSource({
    this.threads = kSampleThreads,
    this.failure,
  });

  final List<ThreadSummary> threads;
  final String? failure;

  @override
  Future<List<ThreadSummary>> load() async {
    if (failure != null) {
      throw ThreadsLoadFailure(failure!);
    }
    return threads;
  }
}

/// 示例数据：B1 静态骨架的六条内容（测试与离线预览用；生产默认走真通道）。
const List<ThreadSummary> kSampleThreads = <ThreadSummary>[
  ThreadSummary(
    id: 'thread-1',
    title: '修复登录页 token 过期跳转',
    agentId: 'codex',
    summary: '已改完 refresh 拦截器，正在跑单测',
    status: PAStatusKind.running,
    time: '14:02',
  ),
  ThreadSummary(
    id: 'thread-2',
    title: '给 daemon 加配对码速率限制',
    agentId: 'pi',
    summary: '5 次失败后指数退避，proto 无需改动',
    status: PAStatusKind.done,
    time: '13:41',
  ),
  ThreadSummary(
    id: 'thread-3',
    title: '重构文件树虚拟列表',
    agentId: 'dsh',
    summary: '超出 5000 节点时首屏卡顿，需要分片加载',
    status: PAStatusKind.warn,
    time: '11:27',
  ),
  ThreadSummary(
    id: 'thread-4',
    title: '同步 android 副本 proto',
    agentId: 'zcode',
    summary: 'gen_proto.py --check 报 MD5 不一致',
    status: PAStatusKind.failed,
    time: '昨天',
  ),
  ThreadSummary(
    id: 'thread-5',
    title: '写 0.4.0 发布说明',
    agentId: 'hermes',
    summary: '等 daemon 侧文档定稿后合并',
    status: PAStatusKind.pending,
    time: '昨天',
  ),
  ThreadSummary(
    id: 'thread-6',
    title: '终端页 ANSI 色彩还原',
    agentId: 'codex',
    summary: 'OSC 256 色已支持，真彩序列待补',
    status: PAStatusKind.done,
    time: '10-12',
  ),
];
