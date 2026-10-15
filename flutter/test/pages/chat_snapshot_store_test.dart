// Chat 离线快照 store 测试（B3 第 5 波子步②）。
//
// 【测什么】快照往返（整体覆盖语义）+ 有效性判定（版本号 + 水位）+ 裁剪上限
// （60 条 / 400KB，最新优先）+ 损坏按无快照处理（不崩、不猜）。
// 编解码是纯函数（`toJson`/`tryFromJson`），持久化走内存 KV——换真持久化时
// 「读 → 解码 → 判定」的链路已在这里跑过多轮。

import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_snapshot_store.dart';

ThreadMessage _msg(String role, String text, {int at = 1000}) => ThreadMessage(
      role: role,
      blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: text))],
      createdAtUnixMs: Int64(at),
      messageId: 'm-$at',
      turnId: 'turn-1',
    );

void main() {
  ChatSnapshotStore store({int now = 9000}) => ChatSnapshotStore(
        prefs: InMemoryChatSnapshotPrefs(),
        nowMs: () => now,
      );

  group('save/load：整体覆盖往返', () {
    test('存取字段落位：threadId/消息序/水位/时间戳', () async {
      final ChatSnapshotStore s = store();
      await s.saveSnapshot(
        threadId: 't-1',
        messages: <ThreadMessage>[
          _msg('user', '先修登录页'),
          _msg('assistant', '改完了'),
        ],
        sequence: 0,
      );

      final ChatSnapshot? got = await s.loadSnapshot('t-1');

      expect(got, isNotNull);
      expect(got!.threadId, 't-1');
      expect(
        got.messages,
        <ChatSnapshotMsg>[
          const ChatSnapshotMsg(role: 'user', text: '先修登录页', savedAtMs: 9000),
          const ChatSnapshotMsg(
              role: 'assistant', text: '改完了', savedAtMs: 9000),
        ],
      );
      expect(got.sequence, 0);
      expect(got.savedAtMs, 9000);
      expect(got.version, kChatSnapshotVersion);
    });

    test('覆盖而非合并：第二次保存整体替换第一次', () async {
      final ChatSnapshotStore s = store();
      await s.saveSnapshot(
          threadId: 't-1',
          messages: <ThreadMessage>[_msg('user', '旧')],
          sequence: 0);
      await s.saveSnapshot(
          threadId: 't-1',
          messages: <ThreadMessage>[_msg('user', '新')],
          sequence: 0);

      final ChatSnapshot? got = await s.loadSnapshot('t-1');

      expect(got!.messages.single.text, '新');
    });

    test('无快照 → null；空 threadId 读写都是 no-op（null，不抛）', () async {
      final ChatSnapshotStore s = store();
      expect(await s.loadSnapshot('never-saved'), isNull);
      expect(await s.loadSnapshot(''), isNull);
      await s.saveSnapshot(threadId: '', messages: <ThreadMessage>[], sequence: 0);
      expect(await s.loadSnapshot(''), isNull);
    });

    test('clear 删掉快照；删不存在的不抛（幂等）', () async {
      final ChatSnapshotStore s = store();
      await s.saveSnapshot(
          threadId: 't-1',
          messages: <ThreadMessage>[_msg('user', 'hi')],
          sequence: 0);
      await s.clearSnapshot('t-1');
      expect(await s.loadSnapshot('t-1'), isNull);
      await s.clearSnapshot('t-1');
      await s.clearSnapshot('');
    });
  });

  group('损坏与版本：按无快照处理', () {
    test('坏 JSON → null（不崩，下次覆写自愈）', () async {
      final InMemoryChatSnapshotPrefs prefs = InMemoryChatSnapshotPrefs();
      await prefs.write('t-1', '{not json');
      final ChatSnapshotStore s =
          ChatSnapshotStore(prefs: prefs, nowMs: () => 0);

      expect(await s.loadSnapshot('t-1'), isNull);
    });

    test('形状不对（msgs 非法）→ null', () async {
      final InMemoryChatSnapshotPrefs prefs = InMemoryChatSnapshotPrefs();
      await prefs.write('t-1', jsonEncode(<String, Object?>{'v': 1}));
      final ChatSnapshotStore s =
          ChatSnapshotStore(prefs: prefs, nowMs: () => 0);

      expect(await s.loadSnapshot('t-1'), isNull);
    });

    test('旧版本号 → null（不猜不迁移，只读副本删了无影响）', () async {
      final InMemoryChatSnapshotPrefs prefs = InMemoryChatSnapshotPrefs();
      await prefs.write(
        't-1',
        jsonEncode(<String, Object?>{
          'v': 0,
          'thread': 't-1',
          'seq': 0,
          'at': 1,
          'msgs': <Object?>[],
        }),
      );
      final ChatSnapshotStore s =
          ChatSnapshotStore(prefs: prefs, nowMs: () => 0);

      expect(await s.loadSnapshot('t-1'), isNull);
    });
  });

  group('有效性判定（纯函数：版本号 + 水位）', () {
    ChatSnapshot snap({int version = 1, String id = 't', int seq = 0}) =>
        ChatSnapshot(
          threadId: id,
          messages: const <ChatSnapshotMsg>[],
          sequence: seq,
          savedAtMs: 1,
          version: version,
        );

    test('null/旧版/空 id/负水位 → 无效；正常 → 有效', () {
      expect(isSnapshotValid(null), isFalse);
      expect(isSnapshotValid(snap()), isTrue);
      expect(isSnapshotValid(snap(version: 0)), isFalse);
      expect(isSnapshotValid(snap(version: 2)), isFalse);
      expect(isSnapshotValid(snap(id: '')), isFalse);
      expect(isSnapshotValid(snap(seq: -1)), isFalse);
    });

    test('水位：快照 seq < live → 过期；未知水位（0）永不过期', () {
      expect(isSnapshotStale(snap(seq: 5), 7), isTrue);
      expect(isSnapshotStale(snap(seq: 5), 5), isFalse);
      expect(isSnapshotStale(snap(seq: 5), 3), isFalse);
      expect(isSnapshotStale(snap(seq: 0), 99), isFalse,
          reason: '0 = 未知水位（历史帧无序号）：不知道就不断言过期');
    });
  });

  group('裁剪上限（最新优先，Android 同值）', () {
    test('70 条 → 留最新 60 条，顺序升序不变', () async {
      final ChatSnapshotStore s = store();
      final List<ThreadMessage> many = List<ThreadMessage>.generate(
        70,
        (int i) => _msg('user', 'msg-$i', at: i),
      );
      await s.saveSnapshot(threadId: 't-1', messages: many, sequence: 0);

      final ChatSnapshot? got = await s.loadSnapshot('t-1');

      expect(got!.messages, hasLength(kChatSnapshotMaxMsgs));
      expect(got.messages.first.text, 'msg-10');
      expect(got.messages.last.text, 'msg-69');
    });

    test('体积超 400KB → 只留最新一条（超限丢最旧）', () async {
      final ChatSnapshotStore s = store();
      final String big = 'x' * (150 * 1024); // 150K 字 ≈ 300KB
      await s.saveSnapshot(
        threadId: 't-1',
        messages: <ThreadMessage>[
          _msg('user', '旧-$big', at: 1),
          _msg('user', '新-$big', at: 2),
        ],
        sequence: 0,
      );

      final ChatSnapshot? got = await s.loadSnapshot('t-1');

      expect(got!.messages, hasLength(1));
      expect(got.messages.single.text.startsWith('新-'), isTrue);
    });
  });

  group('toHistoryResponse：文本-only 重建', () {
    test('role/文本/时间回填；稳定身份留空（快照不存）', () async {
      final ChatSnapshotStore s = store();
      await s.saveSnapshot(
        threadId: 't-1',
        messages: <ThreadMessage>[_msg('assistant', '改完了')],
        sequence: 0,
      );

      final GetThreadHistoryResponse shadow =
          (await s.loadSnapshot('t-1'))!.toHistoryResponse();

      expect(shadow.messages, hasLength(1));
      expect(shadow.messages.single.role, 'assistant');
      expect(shadow.messages.single.blocks.single.text.text, '改完了');
      expect(shadow.messages.single.createdAtUnixMs.toInt(), 9000);
      expect(shadow.messages.single.messageId, isEmpty,
          reason: '快照不存稳定身份：定位靠顺序，稳定跳转需在线拉全量');
      expect(shadow.messages.single.turnId, isEmpty);
    });
  });
}
