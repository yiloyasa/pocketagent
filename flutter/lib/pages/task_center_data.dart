// TaskCenter 的任务模型 + app 级 store + 持久化抽象 + 分区/文案纯函数
// （B3 第 3 波最后一件）。
//
// 【数据源判定（本文件的存在理由）】proto/v1 全 93 RPC 逐个核对过：**没有任何
// task / notification / unread 相关 RPC**（唯一的 "Task" 字样是 thread.proto 的
// `DelegateSubTask`，属协同子任务派发，与任务中心无关）。page-contracts §1.7 明说
// 本页「零直连 RPC」：Kotlin 侧 `TaskCenter` 是 **app 级 object + SharedPreferences
// 持久化**（§3.4），数据 = 消息执行进度（TurnSession 写入）+ 文件下载进度
// （PaDownloader 写入）的手机本地聚合。
// → 因此本页是**纯本地聚合，没有真通道可接**：不接 PaBridge、不假装有远端数据，
//   落本地单例骨架（内存 store + 变更通知 + 持久化接口先行）。
//
// 【为什么单拆一个文件】页面只留界面（两页签 + 分区 + 行渲染），模型 / store /
// 纯函数在这里，可被单测直接命中——与 threads_data.dart / onboarding_prefs.dart
// 同一分工先例。
//
// 【与 Kotlin 的同源边界】store 的方法面、id 约定（msg:$threadId / dl:$fileKey）、
// 状态机迁移、磁盘行格式逐条对齐 android/.../component/TaskCenter.kt，双端同一份
// 任务清单长同一样；**不同的地方只有两处且都在页面文件头说明**（自动已读改显式
// 按钮、indeterminate 进度条改静态填充）。

import 'dart:async';

import '../components/pa_status_chip.dart';

/// 任务类别（Kotlin `TaskCenter.Kind` 的两档，页签 = 类别）。
enum TaskKind {
  /// 消息执行任务：一条会话轮次的进度投影（id = `msg:$threadId`）。
  message,

  /// 文件下载任务：一次下载到手机的进度（id = `dl:$fileKey`）。
  download,
}

/// 磁盘/契约用的类别名（与 Kotlin `Kind.name` 同值，见 [encodeTaskBlob]）。
extension TaskKindWire on TaskKind {
  String get wireName => switch (this) {
        TaskKind.message => 'MESSAGE',
        TaskKind.download => 'DOWNLOAD',
      };
}

/// 任务状态（Kotlin `TaskCenter.Status` 五档，一个不少）。
///
/// 【为什么 CANCELED 与 INTERRUPTED 分开】两者终局语义不同：CANCELED 是用户主动
/// 取消，INTERRUPTED 是杀进程/崩溃后的恢复标记（[decodeTaskBlob] 把落盘的 RUNNING
/// 一律翻成它）。分区计数上 CANCELED 不算「等待你处理」，INTERRUPTED 算。
enum TaskStatus {
  running,
  done,
  failed,
  canceled,

  /// 恢复标记：上次退出时还在跑（Kotlin 同名词 + restore 时的 RUNNING 翻档）。
  interrupted,
}

/// 磁盘/契约用的状态名（与 Kotlin `Status.name` 同值）。
extension TaskStatusWire on TaskStatus {
  String get wireName => switch (this) {
        TaskStatus.running => 'RUNNING',
        TaskStatus.done => 'DONE',
        TaskStatus.failed => 'FAILED',
        TaskStatus.canceled => 'CANCELED',
        TaskStatus.interrupted => 'INTERRUPTED',
      };
}

/// 一条任务（Kotlin `TaskCenter.Task` 的 Dart 形态）。
///
/// 【为什么是不可变类 + copyWith】Kotlin 用 `mutableStateOf` 逐字段可变（列表用
/// StateFlow 整体替换驱动 UI）；Dart 侧没有 mutableStateOf，改一个字段就换一个
/// 新实例最省心，也让「这一轮有没有变化」能靠 `==` 判定（store 据此决定要不要
/// 发通知，见 [TaskCenterStore]）。
class TaskItem {
  const TaskItem({
    required this.id,
    required this.threadId,
    required this.kind,
    required this.title,
    required this.status,
    this.progress = 0,
    this.detail = '',
    this.read = false,
    required this.startedAtMs,
    this.rounds = 1,
    this.localUri = '',
    this.sourceTitle = '',
    this.sizeText = '',
  });

  /// 任务唯一标识：消息类 = `msg:$threadId`；下载类 = `dl:$fileKey`。
  final String id;

  /// 来源会话 id（可空串：下载类允许没有会话上下文）。
  final String threadId;

  final TaskKind kind;

  final String title;

  final TaskStatus status;

  /// 进度 0..1（仅 RUNNING 有意义；Kotlin 单调不减，见 [TaskCenterStore.updateDownload]）。
  final double progress;

  /// 行内副文案（下载类常是落盘路径摘要 / 失败原因）。
  final String detail;

  /// 已读未读（未读计数 [unreadCountOf] 的唯一来源）。
  final bool read;

  /// 起始时间戳（毫秒；行标签与「清理已完成」排序都用它）。
  final int startedAtMs;

  /// 同一会话合并的执行轮数（>1 时行上显示「N 轮」）。
  final int rounds;

  /// 下载类：手机本地文件 uri（`content://` 或直路径；可能为空）。
  final String localUri;

  /// 下载类：来源会话标题（展示「来自会话：…」）。
  final String sourceTitle;

  /// 下载类：完成时的文件大小文案（如 `1.2MB`）。
  final String sizeText;

  TaskItem copyWith({
    String? title,
    TaskStatus? status,
    double? progress,
    String? detail,
    bool? read,
    int? startedAtMs,
    int? rounds,
    String? localUri,
    String? sourceTitle,
    String? sizeText,
  }) =>
      TaskItem(
        id: id,
        threadId: threadId,
        kind: kind,
        title: title ?? this.title,
        status: status ?? this.status,
        progress: progress ?? this.progress,
        detail: detail ?? this.detail,
        read: read ?? this.read,
        startedAtMs: startedAtMs ?? this.startedAtMs,
        rounds: rounds ?? this.rounds,
        localUri: localUri ?? this.localUri,
        sourceTitle: sourceTitle ?? this.sourceTitle,
        sizeText: sizeText ?? this.sizeText,
      );

  @override
  bool operator ==(Object other) =>
      other is TaskItem &&
      other.id == id &&
      other.threadId == threadId &&
      other.kind == kind &&
      other.title == title &&
      other.status == status &&
      other.progress == progress &&
      other.detail == detail &&
      other.read == read &&
      other.startedAtMs == startedAtMs &&
      other.rounds == rounds &&
      other.localUri == localUri &&
      other.sourceTitle == sourceTitle &&
      other.sizeText == sizeText;

  @override
  int get hashCode => Object.hash(
        id,
        Object.hash(threadId, kind, title, status, progress, detail, read,
            startedAtMs, rounds, localUri, sourceTitle, sizeText),
      );

  @override
  String toString() => 'TaskItem($id, $kind, $status, read=$read)';
}

/// 消息类任务 id（Kotlin `"msg:$threadId"`，同一会话只此一条）。
String messageTaskId(String threadId) => 'msg:$threadId';

/// 下载类任务 id（Kotlin `"dl:$fileKey"`，同一文件只此一条）。
String downloadTaskId(String fileKey) => 'dl:$fileKey';

/// 下载类任务的 fileKey（id 去 `dl:` 前缀；重试计划的输入）。
String taskFileKey(TaskItem task) =>
    task.kind == TaskKind.download ? task.id.substring(3) : '';

// ── 持久化抽象（接口先行，B4 接插件）────────────────────────────────────────

/// 任务中心的持久化来源（Kotlin `SharedPreferences("task_center")` 的等价抽象）。
///
/// 【为什么不直接用 shared_preferences】本波红线明确不改 pubspec.yaml、不加依赖
/// （onboarding_prefs.dart 同一先例）：这里只定接口 + 内存占位实现，等引入
/// `shared_preferences`（或经 B2 通道委托 Kotlin 宿主读写）后挂生产实现，
/// 调用点一行不用改。
///
/// 【为什么 read 同步而 write 异步】store 在构造时就要恢复落盘任务（Kotlin
/// `init → restore` 在 MainActivity.onCreate 同步路径上），读必须同步可用；写允许
/// 异步（Kotlin 本就丢到单线程 executor 上落盘）。
abstract interface class TaskCenterPrefs {
  /// 读一次快照；没有落过盘返回 null。
  TaskCenterDiskSnapshot? read();

  /// 写一次快照（实现内部可节流，见 [TaskCenterStore] 文件头关于 300ms 的说明）。
  Future<void> write(TaskCenterDiskSnapshot snapshot);
}

/// 落盘快照：任务行文本 + 未读数（Kotlin `putString("tasks") + putInt("unread")`）。
class TaskCenterDiskSnapshot {
  const TaskCenterDiskSnapshot({required this.tasksBlob, required this.unread});

  final String tasksBlob;
  final int unread;
}

/// 内存占位实现（B4 持久化接入前的默认值；测试与离线预览用）。
///
/// 【为什么存的是编码后的 blob 而不是 List】接口的语义是「磁盘上那一份」，
/// 内存实现照办：encode/decode 是纯函数且被单测钉死，将来换真持久化时，
/// 「读进来 → 恢复 → 合并」的链路已经在内存实现上跑过多轮了。
class InMemoryTaskCenterPrefs implements TaskCenterPrefs {
  TaskCenterDiskSnapshot? _snapshot;

  @override
  TaskCenterDiskSnapshot? read() => _snapshot;

  @override
  Future<void> write(TaskCenterDiskSnapshot snapshot) async {
    // 赋值在第一个 await 之前完成：调用方（store 的写路径）同步结束后即可断言，
    // 不需要测试侧再推事件循环。
    _snapshot = snapshot;
  }
}

// ── 磁盘行格式（与 Kotlin writeDisk / parseTasks 逐字段同构）────────────────

const String _kFieldSep = '\u0001';
const String _kLineSep = '\n';

/// `List&lt;TaskItem&gt;` → Kotlin `writeDisk` 同款行文本。
///
/// 【为什么要与 Kotlin 同格式】两端将来要读同一个 SharedPreferences 文件
/// （Kotlin 侧此刻就在写）：格式漂移 = 双端互相读不懂对方的任务清单。字段值里的
/// 分隔符替换成空格，与 Kotlin 一致（丢分隔符比读崩好）。
String encodeTaskBlob(List<TaskItem> tasks) => tasks
    .map((TaskItem t) => <String>[
          t.id,
          t.threadId,
          t.kind.wireName,
          t.title,
          t.status.wireName,
          t.read ? '1' : '0',
          '${t.startedAtMs}',
          '${t.progress}',
          t.detail,
          '${t.rounds}',
          t.localUri,
          t.sourceTitle,
          t.sizeText,
        ].map(_sanitize).join(_kFieldSep))
    .join(_kLineSep);

String _sanitize(String v) => v.replaceAll(_kFieldSep, ' ').replaceAll(_kLineSep, ' ');

/// Kotlin `parseTasks` 的等价实现：逐行解析，坏行**整行丢弃**（不炸、不猜）。
///
/// 【恢复规则】落盘的 RUNNING 一律翻成 [TaskStatus.interrupted]——杀进程时还在跑的
/// 任务不可能还活着，不翻就是把「未知」画成「正在执行」（假绿）。Kotlin 同款。
List<TaskItem> decodeTaskBlob(String raw) {
  final List<TaskItem> out = <TaskItem>[];
  for (final String line in raw.split(_kLineSep)) {
    if (line.trim().isEmpty) continue;
    final List<String> f = line.split(_kFieldSep);
    if (f.length < 10) continue; // 老格式缺字段：与 Kotlin 一致整行弃
    final TaskKind? kind = switch (f[2]) {
      'MESSAGE' => TaskKind.message,
      'DOWNLOAD' => TaskKind.download,
      _ => null,
    };
    if (kind == null) continue;
    final TaskStatus? status = switch (f[4]) {
      'RUNNING' => TaskStatus.interrupted, // 恢复翻档，见文件头
      'DONE' => TaskStatus.done,
      'FAILED' => TaskStatus.failed,
      'CANCELED' => TaskStatus.canceled,
      'INTERRUPTED' => TaskStatus.interrupted,
      _ => null,
    };
    if (status == null) continue;
    final int startedAt = int.tryParse(f[6]) ?? 0;
    if (startedAt <= 0) continue; // 时间戳不可信的行没有存在意义
    out.add(TaskItem(
      id: f[0],
      threadId: f[1],
      kind: kind,
      title: f[3],
      status: status,
      read: f[5] == '1',
      startedAtMs: startedAt,
      progress: double.tryParse(f[7]) ?? 0,
      detail: f[8],
      rounds: int.tryParse(f[9]) ?? 1,
      localUri: f.length > 10 ? f[10] : '',
      sourceTitle: f.length > 11 ? f[11] : '',
      sizeText: f.length > 12 ? f[12] : '',
    ));
  }
  return out;
}

// ── 纯函数：未读 / 统计 / 分区 ─────────────────────────────────────────────

/// 未读任务数（Kotlin `_unreadCount = count { !it.read }`）。
int unreadCountOf(List<TaskItem> tasks) =>
    tasks.where((TaskItem t) => !t.read).length;

/// 顶部三张统计卡（Kotlin `taskCenterStats`）：等待你处理 / 正在执行 / 已完成。
///
/// 【为什么「等待你处理」只数 FAILED+INTERRUPTED】CANCELED 是用户自己取消的，
/// 不需要用户回来处理；Kotlin 同口径（旧概览条同一套）。
class TaskCenterStats {
  const TaskCenterStats({
    required this.waitingForYou,
    required this.running,
    required this.completed,
  });

  final int waitingForYou;
  final int running;
  final int completed;
}

TaskCenterStats taskStatsOf(List<TaskItem> tasks) => TaskCenterStats(
      waitingForYou: tasks
          .where((TaskItem t) =>
              t.status == TaskStatus.failed || t.status == TaskStatus.interrupted)
          .length,
      running: tasks.where((TaskItem t) => t.status == TaskStatus.running).length,
      completed: tasks.where((TaskItem t) => t.status == TaskStatus.done).length,
    );

/// 一个分区行：分区头文案 + 组内任务（Kotlin `TaskPartitionRow`）。
class TaskPartitionRow {
  const TaskPartitionRow({
    required this.word,
    required this.offlineResume,
    required this.tasks,
  });

  /// 分区头文案（= 状态词本身，与行内徽章同源）。
  final String word;

  /// 是否「离线待恢复」分区（行内不显示状态徽章，只显示离线提示）。
  final bool offlineResume;

  final List<TaskItem> tasks;
}

/// 四词分区（Kotlin `taskPartitionRows` 的可达子集，空组不出现）。
///
/// 【为什么没有 Kotlin 的「等待你处理」分区】Kotlin 那一组筛的是
/// `editorialStatusOf(status) == WAITING_FOR_YOU`，而任务的 editorialStatusOf 只产出
/// 正在执行 / 已完成 / 未完成三档——**没有任何任务状态映射到「等待你处理」**，
/// 该分区恒为空（Kotlin 侧是死分支，靠 `filter { it.tasks.isNotEmpty() }` 兜掉）。
/// 这里不搬死分支：FAILED/INTERRUPTED 的「等用户」语义由顶部统计卡的
/// 「等待你处理」计数表达（Kotlin 同源）。
List<TaskPartitionRow> taskPartitionRows(
  List<TaskItem> tasks, {
  required bool hostOffline,
}) {
  final List<TaskPartitionRow> rows = <TaskPartitionRow>[];
  if (hostOffline) {
    final List<TaskItem> offline = tasks
        .where((TaskItem t) =>
            t.kind == TaskKind.message && t.status == TaskStatus.running)
        .toList(growable: false);
    if (offline.isNotEmpty) {
      rows.add(TaskPartitionRow(
        word: '离线待恢复',
        offlineResume: true,
        tasks: offline,
      ));
    }
  }
  void add(String word, bool Function(TaskItem) pred) {
    final List<TaskItem> group =
        tasks.where(pred).toList(growable: false);
    if (group.isNotEmpty) {
      rows.add(TaskPartitionRow(word: word, offlineResume: false, tasks: group));
    }
  }

  add('正在执行', (TaskItem t) {
    if (t.status != TaskStatus.running) return false;
    // 断连时消息类运行中落入「离线待恢复」，不在这里二次出现。
    return !(hostOffline && t.kind == TaskKind.message);
  });
  add('已完成', (TaskItem t) => t.status == TaskStatus.done);
  add('未完成', (TaskItem t) =>
      t.status != TaskStatus.running && t.status != TaskStatus.done);
  return rows;
}

// ── 纯函数：行渲染口径 ─────────────────────────────────────────────────────

/// 任务状态 → 设计系统五态 chip 的状态档（颜色通道）。
///
/// 口径 = Kotlin `editorialStatusOf(TaskCenter.Status)`：RUNNING→运行档、DONE→完成档、
/// FAILED/CANCELED/INTERRUPTED→未完成档（同色系，不细分——「没成」不美化）。
PAStatusKind taskStatusKindOf(TaskStatus status) => switch (status) {
      TaskStatus.running => PAStatusKind.running,
      TaskStatus.done => PAStatusKind.done,
      TaskStatus.failed ||
      TaskStatus.canceled ||
      TaskStatus.interrupted =>
        PAStatusKind.failed,
    };

/// 行内徽章文案（四词状态，与分区头同源）。
String taskStatusLabel(TaskStatus status) => switch (status) {
      TaskStatus.running => '正在执行',
      TaskStatus.done => '已完成',
      TaskStatus.failed || TaskStatus.canceled || TaskStatus.interrupted => '未完成',
    };

/// 行时间标签（Kotlin `SimpleDateFormat("MM-dd HH:mm")`，本地时区）。
String taskRowTimeLabel(int unixMs) {
  if (unixMs <= 0) return '';
  final DateTime t = DateTime.fromMillisecondsSinceEpoch(unixMs);
  String two(int v) => v.toString().padLeft(2, '0');
  return '${two(t.month)}-${two(t.day)} ${two(t.hour)}:${two(t.minute)}';
}

/// 离线行内提示（Kotlin `taskOfflineHint`）：只在与徽章隐去相同的条件下给词。
String? taskOfflineHint(TaskItem task, {required bool hostOffline}) {
  if (task.kind == TaskKind.message &&
      hostOffline &&
      task.status == TaskStatus.running) {
    return '离线 · 恢复连接后继续';
  }
  return null;
}

/// 重新下载的连接依赖（Kotlin `redownloadPlanOf`）：URL 前缀 → 只要手机有网；
/// Windows 盘符/UNC/POSIX 绝对路径 → 要电脑在线；其余（blob、文本另存）→ 无可重放来源。
enum RedownloadPlan { fromUrl, fromPc, none }

RedownloadPlan redownloadPlanOf(String fileKey) {
  if (fileKey.startsWith('http://') || fileKey.startsWith('https://')) {
    return RedownloadPlan.fromUrl;
  }
  if (fileKey.contains('\\')) return RedownloadPlan.fromPc;
  if (RegExp(r'^[A-Za-z]:[\\/]').hasMatch(fileKey)) return RedownloadPlan.fromPc;
  if (fileKey.startsWith('/')) return RedownloadPlan.fromPc;
  return RedownloadPlan.none;
}

/// 重新下载是否需要电脑在线（null = 没有可重放的来源）。
bool? redownloadNeedsPc(RedownloadPlan plan) => switch (plan) {
      RedownloadPlan.fromUrl => false,
      RedownloadPlan.fromPc => true,
      RedownloadPlan.none => null,
    };

/// 失败/中断行的「下一步」文案（Kotlin `failureNextStep` 的等价纯函数）。
///
/// 【与 Kotlin 的唯一分歧：hasRetryAction】Kotlin 的行尾必有重试/重下按钮（TurnSession
/// 与 PaDownloader 都在同一个 app 里）；Flutter 本波这两个依赖都未就绪，行尾按钮
/// **只在调用方注入了对应回调时才出现**（见 TaskCenterPage 的 onRetryMessage /
/// onRetryDownload）。文案因此多一个参数：没有重试按钮时不许写「点右侧重试」——
/// 那是指向一个不存在的按钮（假出口）。其余分支逐字对齐 Kotlin。
String taskNextStepCopy({
  required TaskKind kind,
  required bool hostOffline,
  required bool? redownloadNeedsPc,
  required bool hasSourceThread,
  required bool hasRetryAction,
}) {
  if (kind == TaskKind.message) {
    if (hostOffline) {
      return hasRetryAction
          ? '恢复连接后点重试，找回这次的结果'
          : '恢复连接后去会话里找回这次的结果';
    }
    return hasRetryAction ? '点右侧重试，找回这次的结果' : '去会话里找回这次的结果';
  }
  // 下载类，没有可重放的来源（blob / 文本另存）：出路是回会话或移除，不给假重试。
  if (redownloadNeedsPc == null) {
    return hasSourceThread
        ? '内容在会话里还在；可回会话重新保存，或移除这条记录'
        : '这条记录没有可再取的文件，可移除';
  }
  // 下载类，要连电脑：断线时先说清「现在取不了」，不让人白点。
  if (redownloadNeedsPc) {
    return hostOffline
        ? (hasRetryAction
            ? '电脑连不上时取不了；恢复连接后点重新下载'
            : '电脑连不上时取不了；恢复连接后再试')
        : (hasRetryAction
            ? '点右侧重新下载，从电脑再取一次'
            : '需要电脑在线，恢复连接后再试');
  }
  // 下载类，只靠手机有网（URL 来源）：与电脑连接无关，断连也照常可重试。
  return hasRetryAction ? '点右侧重新下载，再取一次' : '再取一次';
}

// ── 行尾重试门闩（Kotlin `RetryGate` 的等价实现）──────────────────────────

/// 默认重试冷却（Kotlin `TASK_RETRY_COOLDOWN_MS`）。
const int kTaskRetryCooldownMs = 1500;

/// 单行重试的冷却门闩：每次放行后 [cooldownMs] 内不再放行（连点防护）。
///
/// 【为什么门闩是纯逻辑】它只依赖时钟：[nowMs] 显式传入（生产传当前时钟），
/// 单测可以钉死「冷却内第二次点击被吞」而不需要真等 1.5 秒。
class TaskRetryGate {
  TaskRetryGate(this.cooldownMs);

  /// 默认冷却（Kotlin `TASK_RETRY_COOLDOWN_MS`）：两次重试之间的最短间隔。
  TaskRetryGate.standard() : cooldownMs = kTaskRetryCooldownMs;

  final int cooldownMs;

  /// 上次放行时刻；初值取 `-cooldownMs` 让首次点击（含 nowMs=0）必过。
  int _lastAllowedAt = -kTaskRetryCooldownMs;

  /// 尝试放行一次；false = 冷却中，本次点击应被忽略（Kotlin `tryBegin`）。
  bool tryBegin(int nowMs) {
    if (nowMs - _lastAllowedAt < cooldownMs) return false;
    _lastAllowedAt = nowMs;
    return true;
  }

  /// 是否仍在冷却窗口内（Kotlin `inFlight`，用于「重试中」占位）。
  bool inFlight(int nowMs) => nowMs - _lastAllowedAt < cooldownMs;
}

// ── app 级 store（Kotlin object TaskCenter 的 Dart 形态）───────────────────

/// 任务中心 store：内存任务清单 + 变更通知 + 持久化委托。
///
/// 【为什么不是全局单例】Kotlin 是 app 级 object（AppDrawer 徽章 / Home /
/// TurnSession 共享）；Flutter 侧共享装配要等 B4（根组件注入或 InheritedWidget），
/// 本波先给**可注入的类**——页面用构造注入（同 ThreadsPage 的 source 注入模式），
/// 测试各拿各的实例互不串味。
///
/// 【生命周期 = 实例；刻意**不**提供 close】原注释此处写的是「届时把实例提到
/// app 级即可，方法面不动」——那句是错的，已改正：提到 app 级**必然要改方法面**
/// （见下面【唯一的增长向量】）。当前树上**没有任何 app 级持有者**：两个消费方
/// 都自己建（`task_center.dart:101` / `settings_downloads.dart:236` 的
/// `widget.store ?? TaskCenterStore()`），`main.dart` 的壳也没持任务中心实例，
/// 所以 store 与它的通知流一起随页面变垃圾，没有任何跨实例的保留引用 → 现在
/// **既不存在泄漏、也不需要 close**。不给它加 `dispose()` 还有一条对位理由：
/// Kotlin 那个 `object TaskCenter` 的通知面是 `MutableStateFlow`
/// （`android/.../ui/component/TaskCenter.kt:82-88`），**StateFlow 根本没有
/// close**——Kotlin 侧同样没有「拆通知流」这一步；Dart 侧凭空补一个 `dispose()`
/// 会造出一个没有 Kotlin 对应物、又没有任何调用方的假接口（谁调它？页面 dispose
/// 只该退订，退订后 store 本身无人持有，`close` 纯属多余动作）。
///
/// 【唯一的增长向量：订阅者不取消】未 close 的 broadcast controller 只按**活跃
/// 订阅**占内存，**不按事件条数**占：没有监听者时 `add()` 的事件直接丢弃，不进
/// 任何队列（这正是下面【通知语义】里「订阅前不补」那条测试钉的语义）。今天两个
/// 页面在 `initState` 里 `listen` 且从不 `cancel`（`task_center.dart:121` /
/// `settings_downloads.dart:251`），但 store 随页面一起被回收，那个环
/// （State → store → controller → 订阅 → 闭包 → State）是**无外部根的引用环**，
/// 追踪 GC 整体回收，无泄漏。**B4 真把 store 提为 app 级单例的那天**，每开一次
/// 页面就往一个进程级 controller 上永久挂一个监听，那才是真泄漏——修法在页面侧
/// （`listen` 的返回值存字段、`dispose` 里 `cancel`，同 `BridgeTerminalSession.close`
/// 的 owner-关流纪律），不在本文件，故登记移交、不在这里加没人调的 close 方法。
///
/// 【通知语义】broadcast 流：订阅即收、订阅前推过的不补——快照用 [snapshot] 同步读
/// （本地 store 没有「加载中」可言，见页面文件头）。没有变化（逐条 `==` 相等）时
/// **不发通知**：无谓的重建比丢通知更难查。
///
/// 【300ms 写节流的去向】Kotlin 对进度类更新做 300ms 磁盘合写；Dart 侧节流属于
/// 持久化实现的内部策略（接口只有 read/write 两个动作），本波的内存实现不需要，
/// 真接入时在实现里做，store 的方法面不受影响。
class TaskCenterStore {
  // ignore: prefer_initializing_formals
  TaskCenterStore({TaskCenterPrefs? prefs}) : _prefs = prefs {
    // 构造即恢复（Kotlin `init → restore` 在 MainActivity.onCreate 同步路径上）。
    final TaskCenterDiskSnapshot? snap = _prefs?.read();
    if (snap != null) {
      final List<TaskItem> restored = decodeTaskBlob(snap.tasksBlob);
      // 未读标记以落盘值为准，但不小于清单实数（Kotlin `maxOf` 同款）。
      _unread = snap.unread > restored.where((TaskItem t) => !t.read).length
          ? snap.unread
          : restored.where((TaskItem t) => !t.read).length;
      _tasks = restored;
    }
  }

  final TaskCenterPrefs? _prefs;

  /// 变更通知的 broadcast controller。
  ///
  /// 【为什么没有 close】见类头【生命周期 = 实例；刻意不提供 close】：本 store
  /// 随实例存活，不跨页面、不跨进程；Kotlin 对位的 `MutableStateFlow` 同样没有
  /// close 可调。未 close 的代价为零——无监听者时 `add` 不缓冲（见 [tasks]）。
  final StreamController<List<TaskItem>> _controller =
      StreamController<List<TaskItem>>.broadcast();

  List<TaskItem> _tasks = <TaskItem>[];
  int _unread = 0;

  /// 是否正停留在任务中心页（Kotlin `viewing`：停留期间新任务直接落已读）。
  bool _viewing = false;

  /// 变更通知流（broadcast）。
  ///
  /// 【订阅者的义务】不订阅没人受影响；但**订了就要 `cancel`**（页面在
  /// `dispose` 里退订）——见类头【唯一的增长向量：订阅者不取消】。这是本页唯一
  /// 与流相关的生命周期动作，也是 `BridgeTerminalSession` 那条 owner-关流纪律在
  /// 订阅侧的形状：**退订由订阅方做，关闭由流的所有者做，而 store 不拥有页面**。
  Stream<List<TaskItem>> get tasks => _controller.stream;

  /// 当前清单快照（同步可读；首帧即可渲染，无加载态）。
  List<TaskItem> get snapshot => List<TaskItem>.unmodifiable(_tasks);

  /// 未读任务数（未读徽标的唯一真值）。
  int get unreadCount => _unread;

  void setViewing(bool viewing) {
    if (_viewing == viewing) return;
    _viewing = viewing;
    // Kotlin 的 viewing 语义落在 persist 里；这里等价成一次全量已读提交。
    if (_viewing) markAllRead();
  }

  // ── 消息任务 ────────────────────────────────────────────────────────────

  /// 追踪一条消息任务（Kotlin `trackMessage`）：
  /// 同一会话**已存在且还在跑** → 合并轮次；其余（不存在 / 已终局）→ 旧记录删除后
  /// 追加一条新的（终局记录不叠加，否则同一会话会出现两条）。
  ///
  /// 【为什么合并】会话里连发消息时，任务中心只应有一条该会话的记录：轮次 +1、
  /// 标题刷新、重新计未读、起始时间重置（最新一轮的时间才是人记得的时间）。
  String trackMessage(String threadId, String title, {int? nowMs}) {
    final String id = messageTaskId(threadId);
    final int now = nowMs ?? DateTime.now().millisecondsSinceEpoch;
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i >= 0 && _tasks[i].status == TaskStatus.running) {
      _commit(<TaskItem>[
        ..._tasks.sublist(0, i),
        _tasks[i].copyWith(
          title: title,
          rounds: _tasks[i].rounds + 1,
          read: false,
          startedAtMs: now,
        ),
        ..._tasks.sublist(i + 1),
      ]);
      return id;
    }
    _commit(<TaskItem>[
      ..._tasks.where((TaskItem t) => t.id != id),
      TaskItem(
        id: id,
        threadId: threadId,
        kind: TaskKind.message,
        title: title,
        status: TaskStatus.running,
        startedAtMs: now,
      ),
    ]);
    return id;
  }

  /// 消息任务终局（Kotlin `finishMessage`）：成功 → DONE + 进度满；失败 → FAILED。
  void finishMessage(
    String threadId, {
    required bool ok,
    String detail = '',
  }) {
    final String id = messageTaskId(threadId);
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0) return;
    final TaskItem t = _tasks[i];
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      t.copyWith(
        status: ok ? TaskStatus.done : TaskStatus.failed,
        progress: ok ? 1 : t.progress,
        detail: detail,
      ),
      ..._tasks.sublist(i + 1),
    ]);
  }

  // ── 下载任务 ────────────────────────────────────────────────────────────

  /// 追踪一次下载（Kotlin `trackDownload`）：不存在 → 新建；还在跑 → 原样返回
  /// （不重置进度：重复触发只是噪音）；已终局 → **原任务复活**重跑。
  String trackDownload(
    String fileKey,
    String title, {
    String threadId = '',
    int? nowMs,
  }) {
    final String id = downloadTaskId(fileKey);
    final int now = nowMs ?? DateTime.now().millisecondsSinceEpoch;
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0) {
      _commit(<TaskItem>[
        ..._tasks,
        TaskItem(
          id: id,
          threadId: threadId,
          kind: TaskKind.download,
          title: title,
          status: TaskStatus.running,
          startedAtMs: now,
        ),
      ]);
      return id;
    }
    if (_tasks[i].status == TaskStatus.running) return id;
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      _tasks[i].copyWith(
        status: TaskStatus.running,
        progress: 0,
        detail: '',
        read: false,
        startedAtMs: now,
        localUri: '',
        sizeText: '',
      ),
      ..._tasks.sublist(i + 1),
    ]);
    return id;
  }

  /// 下载进度（Kotlin `updateDownload`）：进度**单调不减**（分片回调可能乱序），
  /// detail 非空才覆盖（空 detail = 本次没有新话说）。
  void updateDownload(String id, double progress, {String detail = ''}) {
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0) return;
    final TaskItem t = _tasks[i];
    final double next = progress > t.progress ? progress : t.progress;
    final String nextDetail = detail.isNotEmpty ? detail : t.detail;
    if (next == t.progress && nextDetail == t.detail) return;
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      t.copyWith(progress: next, detail: nextDetail),
      ..._tasks.sublist(i + 1),
    ]);
  }

  /// 下载终局（Kotlin `finishDownload`）：记录本地 uri 与大小，供「打开文件」用。
  void finishDownload(
    String id, {
    required bool ok,
    String detail = '',
    String localUri = '',
    String sizeText = '',
  }) {
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0) return;
    final TaskItem t = _tasks[i];
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      t.copyWith(
        status: ok ? TaskStatus.done : TaskStatus.failed,
        progress: ok ? 1 : t.progress,
        detail: detail,
        localUri: localUri.isNotEmpty ? localUri : t.localUri,
        sizeText: sizeText.isNotEmpty ? sizeText : t.sizeText,
      ),
      ..._tasks.sublist(i + 1),
    ]);
  }

  /// 补记下载来源会话标题（Kotlin `annotateDownload`；空值不覆盖）。
  void annotateDownload(String id, String sourceTitle) {
    if (sourceTitle.isEmpty) return;
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0) return;
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      _tasks[i].copyWith(sourceTitle: sourceTitle),
      ..._tasks.sublist(i + 1),
    ]);
  }

  /// 老版本本地 uri 兜底（Kotlin `attachLocalUriIfEmpty`）：**只填空、不覆盖**。
  void attachLocalUriIfEmpty(String id, String uri) {
    if (uri.isEmpty) return;
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0 || _tasks[i].localUri.isNotEmpty) return;
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      _tasks[i].copyWith(localUri: uri),
      ..._tasks.sublist(i + 1),
    ]);
  }

  // ── 已读 / 移除 ─────────────────────────────────────────────────────────

  /// 标记已读（Kotlin `markRead`）；已读再点是 no-op（不发通知）。
  void markRead(String id) {
    final int i = _tasks.indexWhere((TaskItem t) => t.id == id);
    if (i < 0 || _tasks[i].read) return;
    _commit(<TaskItem>[
      ..._tasks.sublist(0, i),
      _tasks[i].copyWith(read: true),
      ..._tasks.sublist(i + 1),
    ]);
  }

  /// 全部标为已读（Kotlin `markAllRead`）；无未读时 no-op。
  void markAllRead() {
    if (unreadCountOf(_tasks) == 0) return;
    _commit(_tasks
        .map((TaskItem t) => t.read ? t : t.copyWith(read: true))
        .toList(growable: false));
  }

  /// 移除一条（Kotlin `remove`）。
  void remove(String id) {
    final int before = _tasks.length;
    _commit(_tasks.where((TaskItem t) => t.id != id).toList(growable: false));
    if (_tasks.length == before) return; // no-op 已由 _commit 的相等判定兜住
  }

  /// 移除某会话的全部任务（Kotlin `removeByThread`，删会话时调用）。
  ///
  /// 【为什么删会话要清任务】消息任务 id = `msg:$threadId`，会话删了记录还在，
  /// 用户点进去只会撞「会话不存在」的空转；下载任务也常挂 threadId，同理。
  void removeByThread(String threadId) {
    if (threadId.isEmpty) return;
    _commit(_tasks
        .where((TaskItem t) => t.threadId != threadId)
        .toList(growable: false));
  }

  /// 清理已完成（Kotlin `clearFinished`）：只清终局任务，RUNNING 一概不碰。
  void clearFinished() {
    _commit(_tasks
        .where((TaskItem t) => t.status == TaskStatus.running)
        .toList(growable: false));
  }

  // ── 内部：提交（封顶 → 停留已读 → 未读 → 通知 → 落盘）───────────────────

  /// 每类别已读终局任务的保留上限（Kotlin `MAX_FINISHED_PER_KIND`）。
  static const int _maxFinishedPerKind = 50;

  void _commit(List<TaskItem> next) {
    List<TaskItem> capped = _applyCap(next);
    // 停留期间的新/变任务直接落已读（Kotlin persist 的 viewing 分支）。
    if (_viewing) {
      capped = capped
          .map((TaskItem t) => t.read ? t : t.copyWith(read: true))
          .toList(growable: false);
    }
    final int unread = unreadCountOf(capped);
    final bool changed = !_listEquals(_tasks, capped) || unread != _unread;
    _tasks = capped;
    _unread = unread;
    if (!changed) return;
    _controller.add(List<TaskItem>.unmodifiable(_tasks));
    // 落盘委托：内存实现同步生效；真实现内部自行节流（见类注释）。
    _prefs?.write(TaskCenterDiskSnapshot(
      tasksBlob: encodeTaskBlob(_tasks),
      unread: _unread,
    ));
  }

  /// 封顶（Kotlin `applyCap`）：已读终局任务按起始时间倒序只留最近的 N 条/类别；
  /// RUNNING 与未读终局任务永不因封顶消失。
  List<TaskItem> _applyCap(List<TaskItem> all) {
    final int finishedRead = all
        .where((TaskItem t) => t.status != TaskStatus.running && t.read)
        .length;
    if (finishedRead <= _maxFinishedPerKind * 2) return all;
    final List<TaskItem> kept = <TaskItem>[];
    for (final TaskKind kind in TaskKind.values) {
      final List<TaskItem> list =
          all.where((TaskItem t) => t.kind == kind).toList(growable: false);
      kept.addAll(list.where((TaskItem t) =>
          t.status == TaskStatus.running || !t.read));
      final List<TaskItem> finishedRead = list
          .where((TaskItem t) => t.status != TaskStatus.running && t.read)
          .toList()
        ..sort((TaskItem a, TaskItem b) =>
            b.startedAtMs.compareTo(a.startedAtMs));
      kept.addAll(finishedRead.take(_maxFinishedPerKind));
    }
    return kept;
  }

  static bool _listEquals(List<TaskItem> a, List<TaskItem> b) {
    if (a.length != b.length) return false;
    for (int i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}
