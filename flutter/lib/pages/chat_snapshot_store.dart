// Chat 离线快照 store（B3 第 5 波子步②）：单会话历史的只读副本。
//
// 【为什么要有它】会话历史原本完全依赖 daemon：手机一断网打开旧会话，
// 历史页只能报错"无法加载"。这里做**只读副本**：每次成功拉取后整体覆盖
// 落一份，断线时渲染快照并标注来源，恢复连接后照常走真数据。口径对齐
// Android `ThreadSnapshot.kt`（消息侧）：只留 role+text、覆盖而非合并、
// 损坏按无快照处理、下次成功拉取整体覆写自愈。
//
// 【为什么不直接用 shared_preferences】本波红线不改 pubspec.yaml、不加依赖
// （onboarding_prefs.dart / task_center_data.dart 同一先例）：这里只定
// **编解码纯函数 + KV 接口 + 内存占位实现**，等引入 `shared_preferences`
// （或经 B2 通道委托 Kotlin 宿主读写）后挂生产实现，调用点一行不用改。
//
// 【写回时机】调用方（历史页）在**每次历史拉取成功后**调 `saveSnapshot`
// 整体覆盖；失败不写（旧快照继续兜底）。覆盖而非合并：快照永远等于
// 「最近一次成功拉取的最近 N 轮」，避免离线态出现重复或过期消息。

import 'dart:convert';

import 'package:fixnum/fixnum.dart';

import '../gen/pocketagent/v1/common.pb.dart';
import '../gen/pocketagent/v1/thread.pb.dart';
import 'chat_history_data.dart' show historyTextOf;

/// 快照版本号：编解码格式的唯一真值。格式变化时升版，旧快照按无效处理
/// （[isSnapshotValid] 判版本，读到旧版当无快照，不猜不迁移——只读副本
/// 删了不影响功能，下次成功拉取即覆写自愈）。
const int kChatSnapshotVersion = 1;

/// 单会话最多保留消息条数（Android `ThreadSnapshot.MAX_MSG_PER_THREAD` 同值：
/// 从最新往回取，超限丢最旧——离线时"最近的对话"比"完整的旧对话"有用）。
const int kChatSnapshotMaxMsgs = 60;

/// 单会话文本预算（字节；Android `MAX_BYTES_PER_THREAD` 400KB 同值，
/// 费用按 UTF-16 粗估 `length * 2`，与 Kotlin `saveMessages` 同款算法）。
const int kChatSnapshotMaxBytes = 400 * 1024;

// ── 快照模型 ────────────────────────────────────────────────────────────

/// 一条消息快照（只留 role+text：图片/工具调用等无法离线还原的块跳过，
///
/// 宁可少显示，不能错显示；Android `ThreadSnapshot.Msg` 同口径）。
class ChatSnapshotMsg {
  const ChatSnapshotMsg({
    required this.role,
    required this.text,
    this.savedAtMs = 0,
  });

  final String role;
  final String text;

  /// 该条落盘时间（毫秒）。0 = 未知（旧快照无此键，调用方按未知处理，
  /// 不拿它参与排序/展示判断；Android `Msg.savedAtMs` 同款兼容语义）。
  final int savedAtMs;

  Map<String, Object?> toJson() => <String, Object?>{
        // 短键与 Kotlin `saveMessages` 同形（r/t/at），将来双端可互读；
        // 当前 Dart 侧走独立 KV，互读待 B4 验证，不许诺。
        'r': role,
        't': text,
        'at': savedAtMs,
      };

  /// 坏形状返回 null（调用方按无快照处理，不抛——损坏不崩）。
  static ChatSnapshotMsg? tryFromJson(Object? raw) {
    if (raw is! Map) return null;
    final Object? role = raw['r'];
    final Object? text = raw['t'];
    if (role is! String || text is! String) return null;
    final Object? at = raw['at'];
    return ChatSnapshotMsg(
      role: role,
      text: text,
      savedAtMs: at is int ? at : 0,
    );
  }

  @override
  bool operator ==(Object other) =>
      other is ChatSnapshotMsg &&
      other.role == role &&
      other.text == text &&
      other.savedAtMs == savedAtMs;

  @override
  int get hashCode => Object.hash(role, text, savedAtMs);
}

/// 单会话快照：落盘与恢复的完整单位。
class ChatSnapshot {
  const ChatSnapshot({
    required this.threadId,
    required this.messages,
    required this.sequence,
    required this.savedAtMs,
    this.version = kChatSnapshotVersion,
  });

  final String threadId;

  /// 消息列表（落盘时已按「最新 N 条」裁剪，顺序为时间升序，与拉取原序一致）。
  final List<ChatSnapshotMsg> messages;

  /// 事件水位：`TurnServerMessage.sequence` 同一序列空间的最大值（断线续传
  /// 的比较基准；0 = 未知水位——历史帧本身无序号，见 [historyToFrames] 注释，
  /// 未知水位永不判过期，只以版本号+存在性判定有效）。
  final int sequence;

  /// 整份快照落盘时间（毫秒；离线徽标「离线快照 · HH:mm」的时间来源）。
  final int savedAtMs;

  final int version;

  Map<String, Object?> toJson() => <String, Object?>{
        'v': version,
        'thread': threadId,
        'seq': sequence,
        'at': savedAtMs,
        'msgs': messages.map((ChatSnapshotMsg m) => m.toJson()).toList(),
      };

  /// JSON 解码（纯函数）：坏形状/版本号对不上返回 null，不抛。
  static ChatSnapshot? tryFromJson(Object? raw) {
    if (raw is! Map) return null;
    final Object? version = raw['v'];
    final Object? threadId = raw['thread'];
    final Object? sequence = raw['seq'];
    final Object? savedAtMs = raw['at'];
    final Object? msgs = raw['msgs'];
    if (version is! int ||
        threadId is! String ||
        sequence is! int ||
        savedAtMs is! int ||
        msgs is! List) {
      return null;
    }
    final List<ChatSnapshotMsg> messages = <ChatSnapshotMsg>[];
    for (final Object? m in msgs) {
      final ChatSnapshotMsg? msg = ChatSnapshotMsg.tryFromJson(m);
      if (msg == null) return null;
      messages.add(msg);
    }
    return ChatSnapshot(
      threadId: threadId,
      messages: messages,
      sequence: sequence,
      savedAtMs: savedAtMs,
      version: version,
    );
  }

  /// 快照 → 文本-only 历史响应（离线渲染与快照回放的统一入口）。
  ///
  /// 【为什么是文本-only】快照落盘时就只留了 role+text（思考/工具/图片已丢，
  /// 顺序与条数保留）：重建的 `ThreadMessage` 每条恰一个文本块，
  /// `message_id`/`turn_id` 为空（稳定身份与轮次归属是快照不存的东西，
  /// 定位靠顺序；稳定跳转需在线拉全量——不假装快照能做到）。
  GetThreadHistoryResponse toHistoryResponse() => GetThreadHistoryResponse(
        messages: messages.map(
          (ChatSnapshotMsg m) => ThreadMessage(
            role: m.role,
            blocks: <ContentBlock>[
              ContentBlock(text: TextBlock(text: m.text)),
            ],
            createdAtUnixMs: Int64(m.savedAtMs),
          ),
        ),
      );

  @override
  bool operator ==(Object other) =>
      other is ChatSnapshot &&
      other.threadId == threadId &&
      other.sequence == sequence &&
      other.savedAtMs == savedAtMs &&
      other.version == version &&
      _msgsEqual(other.messages, messages);

  @override
  int get hashCode =>
      Object.hash(threadId, sequence, savedAtMs, version, messages.length);
}

bool _msgsEqual(List<ChatSnapshotMsg> a, List<ChatSnapshotMsg> b) {
  if (a.length != b.length) return false;
  for (int i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

// ── 快照有效性判定（纯函数：版本号 + 水位） ─────────────────────────────

/// 快照是否有效：版本号对上 + threadId 非空 + 水位非负。
///
/// null = 无快照 → 无效（调用方走在线/错误分支，不冒充空态）。
bool isSnapshotValid(ChatSnapshot? snapshot) =>
    snapshot != null &&
    snapshot.version == kChatSnapshotVersion &&
    snapshot.threadId.isNotEmpty &&
    snapshot.sequence >= 0;

/// 快照相对 live 水位是否已过期：快照水位 < live 水位 → 需重拉全量。
///
/// 【未知水位永不过期】快照水位 <= 0 表示"不知道"（历史帧无序号，当前恒为
/// 0）：不知道就不断言过期，只以版本号+存在性判定有效；等事件续传波次
/// （GetThreadEventsSince）接入、调用方能喂真水位后，本函数无需改签名。
bool isSnapshotStale(ChatSnapshot snapshot, int liveSequence) {
  if (snapshot.sequence <= 0) return false;
  return snapshot.sequence < liveSequence;
}

// ── 持久化抽象（KV 接口 + 内存实现） ────────────────────────────────────

/// 快照 KV 来源（Kotlin `ThreadSnapshot` 文件快照的等价抽象）。
///
/// 【为什么是 String 级 KV 而不是对象级】接口语义是「磁盘上那一份」：
/// 编解码是纯函数且被单测钉死（`toJson`/`tryFromJson`），换真持久化时
/// 「读进来 → 解码 → 判定有效」的链路已在内存实现上跑过多轮。
/// 鉴于 `TaskCenterPrefs.read` 同步/`write` 异步的先例（构造时同步恢复），
/// 这里读写统一异步：历史页恢复本就是异步加载态里的一步，不占首帧。
abstract interface class ChatSnapshotPrefs {
  /// 读一份快照 JSON；没落过盘返回 null。
  Future<String?> read(String threadId);

  /// 写一份快照 JSON（整体覆盖）。
  Future<void> write(String threadId, String json);

  /// 删一份快照（幂等：没有也是成功）。
  Future<void> remove(String threadId);
}

/// 内存占位实现（持久化接入前的默认值；测试与离线预览用）。
class InMemoryChatSnapshotPrefs implements ChatSnapshotPrefs {
  final Map<String, String> _box = <String, String>{};

  @override
  Future<String?> read(String threadId) async => _box[threadId];

  @override
  Future<void> write(String threadId, String json) async {
    // 赋值在第一个 await 之前完成：调用方写路径同步结束后即可断言，
    // 不需要测试侧再推事件循环（与 InMemoryTaskCenterPrefs 同纪律）。
    _box[threadId] = json;
  }

  @override
  Future<void> remove(String threadId) async {
    _box.remove(threadId);
  }
}

// ── store ───────────────────────────────────────────────────────────────

/// 离线快照 store：拉取成功后覆盖写，断线时读回渲染。
class ChatSnapshotStore {
  ChatSnapshotStore({ChatSnapshotPrefs? prefs, int Function()? nowMs})
      : _prefs = prefs ?? InMemoryChatSnapshotPrefs(),
        _nowMs = nowMs ?? _systemNowMs;

  static int _systemNowMs() => DateTime.now().millisecondsSinceEpoch;

  final ChatSnapshotPrefs _prefs;
  final int Function() _nowMs;

  /// 存一份快照（整体覆盖，不合并）。
  ///
  /// [messages] 本次成功拉取的消息（调用方传响应原序）；[sequence] 本次已知
  /// 的事件水位（当前历史帧无序号，传 0 = 未知水位，见 [isSnapshotStale]）。
  /// 空 threadId 直接返回（不写盘、不抛——调用方 bug 不应炸出异常流）。
  Future<void> saveSnapshot({
    required String threadId,
    required List<ThreadMessage> messages,
    required int sequence,
  }) async {
    if (threadId.isEmpty) return;
    final int now = _nowMs();
    // 从最新往回取：条数与体积超限时丢最旧的（Android `saveMessages` 同款）。
    int used = 0;
    final List<ChatSnapshotMsg> kept = <ChatSnapshotMsg>[];
    for (final ThreadMessage m in messages.reversed) {
      if (kept.length >= kChatSnapshotMaxMsgs) break;
      final String text = historyTextOf(m);
      final int cost = text.length * 2; // UTF-16 粗估，与 Kotlin 同款
      if (used + cost > kChatSnapshotMaxBytes) break;
      used += cost;
      kept.add(ChatSnapshotMsg(role: m.role, text: text, savedAtMs: now));
    }
    final ChatSnapshot snapshot = ChatSnapshot(
      threadId: threadId,
      messages: kept.reversed.toList(growable: false),
      sequence: sequence,
      savedAtMs: now,
    );
    await _prefs.write(threadId, jsonEncode(snapshot.toJson()));
  }

  /// 读一份快照：null = 无快照（没落过盘 / JSON 损坏 / 版本对不上 /
  /// 未初始化，见 [ChatSnapshot.tryFromJson]；损坏按下次覆写自愈，不崩）。
  Future<ChatSnapshot?> loadSnapshot(String threadId) async {
    if (threadId.isEmpty) return null;
    final String? raw;
    try {
      raw = await _prefs.read(threadId);
    } catch (_) {
      return null;
    }
    if (raw == null || raw.isEmpty) return null;
    final Object? decoded;
    try {
      decoded = jsonDecode(raw);
    } catch (_) {
      return null;
    }
    final ChatSnapshot? snapshot = ChatSnapshot.tryFromJson(decoded);
    if (!isSnapshotValid(snapshot)) return null;
    return snapshot;
  }

  /// 删一份快照（幂等；会话删除流程必须调它，否则删了的会话离线还能翻出内容，
  /// Android `removeMessages` 同款理由）。
  Future<void> clearSnapshot(String threadId) async {
    if (threadId.isEmpty) return;
    try {
      await _prefs.remove(threadId);
    } catch (_) {
      // 删除的目标状态就是"没有"：底盘报错也视为达成，不抛。
    }
  }
}
