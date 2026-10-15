// Chat 历史页测试（B3 第 5 波子步②）：三态 + 离线快照兜底 + 恢复按钮行为。
//
// 【三态怎么测】数据源是注入点（`ChatHistorySource`），可控 fake 把
// 「还在加载」「拉失败了」「空历史」摆到台面上：加载态用 Completer 卡住，
// 错误态用一次性失败的源并验证重试真重拉。
//
// 【必须分家的两条路径】（红线：不留假绿）
//   - 「通道没就绪（HOST_UNAVAILABLE→B4 人话）」+ 无快照 → 错误态；
//   - 「真读了、真没有」→ 空态「还没有消息」。
// 快照兜底是第三条路径：拉失败 + 有快照 → 离线徽标 + 快照行（不是错误态，
// 也不是空态）；空快照 + 拉失败 → 错误态（不把失败伪装成空）。
//
// 【恢复按钮只钉转发】点「恢复到会话」→ onReplay 收到与 `historyToFrames`
// 一致的帧序列；连点两次两次都到（原样转发，去重与幂等归 C1 `replayFrames`，
// 本页不断言重复帧——replay 幂等不断言重复帧，不造假）。

import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_history.dart';
import 'package:pocketagent_ui/pages/chat_history_data.dart';
import 'package:pocketagent_ui/pages/chat_snapshot_store.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';

/// 卡住不返回的源：用来把「加载中」钉在屏幕上。
class _PendingSource implements ChatHistorySource {
  final Completer<GetThreadHistoryResponse> completer =
      Completer<GetThreadHistoryResponse>();

  @override
  Future<GetThreadHistoryResponse> loadHistory(String threadId) =>
      completer.future;
}

ThreadMessage _msg(String role, String text, {int at = 1000}) => ThreadMessage(
      role: role,
      blocks: <ContentBlock>[ContentBlock(text: TextBlock(text: text))],
      createdAtUnixMs: Int64(at),
      messageId: 'm-$at-$text',
      turnId: 'turn-1',
    );

GetThreadHistoryResponse _twoMsgs() => GetThreadHistoryResponse(
      messages: <ThreadMessage>[
        _msg('user', '先修登录页', at: 1000),
        _msg('assistant', '改完了', at: 2000),
      ],
    );

void main() {
  Future<ChatSnapshotStore> freshStore() async =>
      ChatSnapshotStore(prefs: InMemoryChatSnapshotPrefs(), nowMs: () => 9000);

  Future<void> pumpHistory(
    WidgetTester tester, {
    required String threadId,
    ChatHistorySource? source,
    ChatSnapshotStore? store,
    ChatReplayCallback? onReplay,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: ChatHistoryPage(
            threadId: threadId,
            source: source,
            snapshotStore: store,
            onReplay: onReplay,
            nowMs: () => 9000,
          ),
        ),
      ),
    );
  }

  group('三态', () {
    testWidgets('加载中 → 「正在加载历史…」，数据到了切换成列表', (tester) async {
      final _PendingSource src = _PendingSource();
      final ChatSnapshotStore store = await freshStore();
      await pumpHistory(tester, threadId: 't-1', source: src, store: store);
      await tester.pump();

      expect(find.byKey(const ValueKey<String>('chat-history-loading')),
          findsOneWidget);
      expect(find.text('正在加载历史…'), findsOneWidget);
      expect(find.text('先修登录页'), findsNothing);

      src.completer.complete(_twoMsgs());
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('chat-history-loading')),
          findsNothing);
      expect(find.byKey(const ValueKey<String>('chat-history-list')),
          findsOneWidget);
      expect(find.text('先修登录页'), findsOneWidget);
      expect(find.text('改完了'), findsOneWidget);
    });

    testWidgets('空历史（真读了、真没有）→ 空态，不摆恢复按钮', (tester) async {
      final FakeChatHistorySource src =
          FakeChatHistorySource(response: GetThreadHistoryResponse());
      await pumpHistory(
          tester, threadId: 't-1', source: src, store: await freshStore());
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('chat-history-empty')),
          findsOneWidget);
      expect(find.text('还没有消息'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-history-error')),
          findsNothing);
      expect(find.byKey(const ValueKey<String>('chat-history-restore')),
          findsNothing,
          reason: '无帧可喂时不摆点了没反应的入口');
    });

    testWidgets('加载失败（无快照）→ 错误态；与空态严格区分；重试真重拉',
        (tester) async {
      final FakeChatHistorySource src = FakeChatHistorySource(
          failure: '桌面端会话历史通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
              '会话历史暂不可用。');
      await pumpHistory(
          tester, threadId: 't-1', source: src, store: await freshStore());
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('chat-history-error')),
          findsOneWidget);
      expect(find.text('会话历史加载失败'), findsOneWidget);
      expect(find.textContaining('未就绪'), findsOneWidget,
          reason: 'HOST_UNAVAILABLE 必须点名未就绪，不许吞成一句含糊的失败');
      expect(find.byKey(const ValueKey<String>('chat-history-empty')),
          findsNothing,
          reason: '读失败绝不许伪装成「没有历史」（假空态）');

      src.failure = null;
      src.response = _twoMsgs();
      await tester.tap(find.byKey(const ValueKey<String>('chat-history-retry')));
      await tester.pumpAndSettle();

      expect(src.requested, <String>['t-1', 't-1'],
          reason: '重试必须真的重新调用数据源');
      expect(find.byKey(const ValueKey<String>('chat-history-error')),
          findsNothing);
      expect(find.text('先修登录页'), findsOneWidget);
    });
  });

  group('离线快照兜底', () {
    testWidgets('拉失败 + 有快照 → 离线徽标 + 快照行（不是错误态）',
        (tester) async {
      final ChatSnapshotStore store = await freshStore();
      await store.saveSnapshot(
        threadId: 't-1',
        messages: <ThreadMessage>[_msg('user', '快照里的字', at: 500)],
        sequence: 0,
      );
      final FakeChatHistorySource src =
          FakeChatHistorySource(failure: 'daemon 不可达');
      await pumpHistory(tester, threadId: 't-1', source: src, store: store);
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('chat-history-offline-badge')),
          findsOneWidget);
      expect(find.textContaining('离线快照'), findsOneWidget,
          reason: '副本必须自报家门：用户要分得清看的是快照还是真数据');
      expect(find.text('快照里的字'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-history-error')),
          findsNothing);
    });

    testWidgets('拉失败 + 空快照 → 错误态（不把失败伪装成空）', (tester) async {
      final ChatSnapshotStore store = await freshStore();
      await store.saveSnapshot(
          threadId: 't-1', messages: <ThreadMessage>[], sequence: 0);
      final FakeChatHistorySource src =
          FakeChatHistorySource(failure: 'daemon 不可达');
      await pumpHistory(tester, threadId: 't-1', source: src, store: store);
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('chat-history-error')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('chat-history-empty')),
          findsNothing);
    });

    testWidgets('成功拉取后落快照：下次断线能兜底', (tester) async {
      final ChatSnapshotStore store = await freshStore();
      final FakeChatHistorySource src =
          FakeChatHistorySource(response: _twoMsgs());
      await pumpHistory(tester, threadId: 't-1', source: src, store: store);
      await tester.pumpAndSettle();

      final ChatSnapshot? snapshot = await store.loadSnapshot('t-1');
      expect(snapshot, isNotNull);
      expect(
        snapshot!.messages.map((ChatSnapshotMsg m) => m.text).toList(),
        <String>['先修登录页', '改完了'],
      );
    });
  });

  group('恢复到会话', () {
    testWidgets('点恢复 → onReplay 收到与 historyToFrames 一致的帧', (tester) async {
      final GetThreadHistoryResponse response = _twoMsgs();
      final FakeChatHistorySource src =
          FakeChatHistorySource(response: response);
      final List<List<TurnServerMessage>> replayed =
          <List<TurnServerMessage>>[];
      await pumpHistory(
        tester,
        threadId: 't-1',
        source: src,
        store: await freshStore(),
        onReplay: (List<TurnServerMessage> frames) async {
          replayed.add(frames);
        },
      );
      await tester.pumpAndSettle();

      await tester
          .tap(find.byKey(const ValueKey<String>('chat-history-restore')));
      await tester.pumpAndSettle();

      final List<TurnServerMessage> want = historyToFrames(response);
      expect(replayed, hasLength(1));
      expect(replayed.single.length, want.length);
      for (int i = 0; i < want.length; i++) {
        expect(replayed.single[i].textDelta.delta, want[i].textDelta.delta,
            reason: '第 $i 帧按序原样转发，不改帧内容');
      }
    });

    testWidgets('连点两次 → 两次都到且帧相同（本页不去重，幂等归 C1）',
        (tester) async {
      final FakeChatHistorySource src =
          FakeChatHistorySource(response: _twoMsgs());
      final List<List<TurnServerMessage>> replayed =
          <List<TurnServerMessage>>[];
      await pumpHistory(
        tester,
        threadId: 't-1',
        source: src,
        store: await freshStore(),
        onReplay: (List<TurnServerMessage> frames) async {
          replayed.add(frames);
        },
      );
      await tester.pumpAndSettle();

      await tester
          .tap(find.byKey(const ValueKey<String>('chat-history-restore')));
      await tester.pumpAndSettle();
      await tester
          .tap(find.byKey(const ValueKey<String>('chat-history-restore')));
      await tester.pumpAndSettle();

      expect(replayed, hasLength(2));
      expect(
        replayed[0].map((TurnServerMessage f) => f.textDelta.delta).toList(),
        replayed[1].map((TurnServerMessage f) => f.textDelta.delta).toList(),
      );
    });

    testWidgets('快照兜底态点恢复 → 快照重建的帧同样可喂', (tester) async {
      final ChatSnapshotStore store = await freshStore();
      await store.saveSnapshot(
        threadId: 't-1',
        messages: <ThreadMessage>[_msg('assistant', '快照里的字', at: 500)],
        sequence: 0,
      );
      final FakeChatHistorySource src =
          FakeChatHistorySource(failure: 'daemon 不可达');
      final List<List<TurnServerMessage>> replayed =
          <List<TurnServerMessage>>[];
      await pumpHistory(
        tester,
        threadId: 't-1',
        source: src,
        store: store,
        onReplay: (List<TurnServerMessage> frames) async {
          replayed.add(frames);
        },
      );
      await tester.pumpAndSettle();

      await tester
          .tap(find.byKey(const ValueKey<String>('chat-history-restore')));
      await tester.pumpAndSettle();

      expect(replayed, hasLength(1));
      expect(replayed.single.single.textDelta.delta, '快照里的字');
    });

    testWidgets('onReplay 为 null → 按钮禁用（不摆点了没反应的入口）',
        (tester) async {
      final FakeChatHistorySource src =
          FakeChatHistorySource(response: _twoMsgs());
      await pumpHistory(
          tester, threadId: 't-1', source: src, store: await freshStore());
      await tester.pumpAndSettle();

      // C1 未落盘且调用方没给回调：按钮在场但禁用，点它不抛、不跳。
      expect(find.byKey(const ValueKey<String>('chat-history-restore')),
          findsOneWidget);
      await tester
          .tap(find.byKey(const ValueKey<String>('chat-history-restore')),
              warnIfMissed: false);
      await tester.pumpAndSettle();
      expect(find.text('先修登录页'), findsOneWidget);
    });
  });
}
