// TaskCenter 页面测试：两页签 / 三态（空态·列表·离线）/ 未读徽标 / 分区 / 本地动作。
//
// 【为什么没有「加载中」用例】本地 store 首帧即有快照，页面**不造假加载延迟**
// （给一个瞬间完成的转圈等于骗一次动画）——这条本身由「pumpWidget 后直接断言
// 列表在场」钉死（见「首帧即列表」用例）。
//
// 【数据源判定】本页无真通道可接（proto 无 task/notification RPC，见
// task_center_data.dart 文件头），所以也没有「宿主未注册」的错误态用例——数据
// 就在本机，唯一能错的是 store 自身（其行为在 task_center_data_test.dart）。
// 这里测的是「给定 store，页面渲染什么、点了发生什么」。

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/pages/task_center.dart';
import 'package:pocketagent_ui/pages/task_center_data.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';

// 固定时钟轴（与数据层测试同一根）。
const int t0 = 1728901200000;

TaskItem msgTask(
  String threadId, {
  String title = '修复登录页崩溃',
  TaskStatus status = TaskStatus.running,
  bool read = false,
  int rounds = 1,
  int startedAtMs = t0,
}) =>
    TaskItem(
      id: messageTaskId(threadId),
      threadId: threadId,
      kind: TaskKind.message,
      title: title,
      status: status,
      startedAtMs: startedAtMs,
      read: read,
      rounds: rounds,
    );

TaskItem dlTask(
  String fileKey, {
  String title = 'a.zip',
  TaskStatus status = TaskStatus.running,
  double progress = 0,
  bool read = false,
  String threadId = 'th-1',
  String sourceTitle = '',
}) =>
    TaskItem(
      id: downloadTaskId(fileKey),
      threadId: threadId,
      kind: TaskKind.download,
      title: title,
      status: status,
      progress: progress,
      startedAtMs: t0,
      read: read,
      sourceTitle: sourceTitle,
    );

void main() {
  /// 预置任务清单：经公开方法面写入（不是页面私有假数据源）——本地 store 的写入
  /// 面就是任务中心的全部数据来源，测试用同一条路径播种，与生产同构。
  ///
  /// 【为什么不用磁盘快照播种】磁盘快照的恢复规则会把 RUNNING 翻成 INTERRUPTED
  /// （杀进程的任务不可能还活着），RUNNING 用例会被静默改语义——这条规则本身在
  /// task_center_data_test.dart 有专测，这里不需要借道。
  TaskCenterStore storeOf(List<TaskItem> tasks) {
    final TaskCenterStore store = TaskCenterStore();
    for (final TaskItem t in tasks) {
      if (t.kind == TaskKind.message) {
        store.trackMessage(t.threadId, t.title, nowMs: t.startedAtMs);
        switch (t.status) {
          case TaskStatus.done:
            store.finishMessage(t.threadId, ok: true);
          case TaskStatus.failed:
            store.finishMessage(t.threadId, ok: false);
          case TaskStatus.canceled:
          case TaskStatus.interrupted:
          case TaskStatus.running:
            break;
        }
      } else {
        store.trackDownload(
          taskFileKey(t),
          t.title,
          threadId: t.threadId,
          nowMs: t.startedAtMs,
        );
        if (t.status != TaskStatus.running) {
          store.finishDownload(
            t.id,
            ok: t.status == TaskStatus.done,
            localUri: t.localUri,
            sizeText: t.sizeText,
          );
        } else if (t.progress > 0) {
          store.updateDownload(t.id, t.progress);
        }
        if (t.sourceTitle.isNotEmpty) {
          store.annotateDownload(t.id, t.sourceTitle);
        }
      }
      if (t.read) store.markRead(t.id);
    }
    return store;
  }

  Future<void> pumpPage(
    WidgetTester tester, {
    required TaskCenterStore store,
    bool hostOffline = false,
    ValueChanged<String>? onOpenThread,
    VoidCallback? onOpenUserData,
    ValueChanged<String>? onRetryMessage,
    ValueChanged<TaskItem>? onRetryDownload,
    int Function()? nowMs,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: TaskCenterPage(
            store: store,
            hostOffline: hostOffline,
            onOpenThread: onOpenThread,
            onOpenUserData: onOpenUserData,
            onRetryMessage: onRetryMessage,
            onRetryDownload: onRetryDownload,
            nowMs: nowMs,
          ),
        ),
      ),
    );
    // 本地 store 首帧同步可用：一帧足够，**不需要 pumpAndSettle 的等待**
    // （也没有可settle 的动画——进度条是静态的）。
    await tester.pump();
  }

  /// 推进页面起的那只「重试中」占位计时器（冷却窗口 = kTaskRetryCooldownMs）。
  ///
  /// fake_async 接管 `Future.delayed` 但不接管系统时钟，所以门闩的冷却判定要靠
  /// [nowMs] 注入钉死，占位的消失则靠这里推进虚拟时间——两边各管一段，
  /// 与生产上「真时钟 + 真延时」同构。
  Future<void> drainRetryCooldown(WidgetTester tester) async {
    await tester.pump(const Duration(milliseconds: kTaskRetryCooldownMs + 50));
    await tester.pump();
  }

  group('空态', () {
    testWidgets('消息页签空 → 「还没有处理中的任务」+ 引导文案', (tester) async {
      await pumpPage(tester, store: storeOf(const <TaskItem>[]));

      expect(find.text('还没有处理中的任务'), findsOneWidget);
      expect(find.text('在会话里发一条消息，这里会显示它的进度'), findsOneWidget);
      expect(find.byType(PAStatusChip), findsNothing);
      // 顶部统计卡仍在（0/0/0/0），空态只替代列表区。
      expect(find.text('等待你处理'), findsOneWidget);
    });

    testWidgets('下载页签空 → 「还没有下载任务」+ 引导文案', (tester) async {
      await pumpPage(tester, store: storeOf(const <TaskItem>[]));

      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(find.text('还没有下载任务'), findsOneWidget);
      expect(find.text('在会话里点文件旁的下载，进度会汇总到这里'), findsOneWidget);
    });

    testWidgets('只有下载任务时：下载页签有列表，消息页签空态（两页签分家）',
        (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[dlTask('C:\\tmp\\a.zip')]),
      );

      expect(find.text('还没有处理中的任务'), findsOneWidget,
          reason: '默认停消息页签：只有下载任务 = 消息侧真空');

      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(find.text('a.zip'), findsOneWidget);
      expect(find.text('还没有下载任务'), findsNothing);
    });
  });

  group('列表渲染', () {
    testWidgets('行六要素：标题 / 时间 / 状态徽章（四词）/ 来源会话 / 大小 / 轮次',
        (tester) async {
      // 下载行：落盘态 + 来源会话 + 大小；消息行：合并过 3 轮的执行。
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('C:\\tmp\\a.zip', 'a.zip', threadId: 'th-1');
      store.finishDownload(
        'dl:C:\\tmp\\a.zip',
        ok: true,
        localUri: 'content://media/42',
        sizeText: '1.2MB',
      );
      store.annotateDownload('dl:C:\\tmp\\a.zip', '修复登录页崩溃');
      for (int i = 0; i < 3; i++) {
        store.trackMessage('th-2', i == 2 ? '修复登录页崩溃' : '第 ${i + 1} 轮');
      }

      await pumpPage(tester, store: store);

      // 下载行（下载页签）
      await tester.tap(find.text('文件下载'));
      await tester.pump();
      expect(find.text('a.zip'), findsOneWidget);
      expect(find.text('来自会话：修复登录页崩溃'), findsOneWidget);
      expect(find.text('1.2MB'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(PAStatusChip),
          matching: find.text('已完成'),
        ),
        findsOneWidget,
      );

      // 消息行（消息页签）：轮次 >1 显示「N 轮」
      await tester.tap(find.text('消息任务'));
      await tester.pump();
      expect(find.text('修复登录页崩溃'), findsOneWidget);
      expect(find.text('3 轮'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(PAStatusChip),
          matching: find.text('正在执行'),
        ),
        findsOneWidget,
      );
    });

    testWidgets('首帧即列表（本地 store 无加载态，不造假转圈）', (tester) async {
      await pumpPage(tester, store: storeOf(<TaskItem>[msgTask('th-1')]));

      expect(find.text('修复登录页崩溃'), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsNothing,
          reason: '瞬间可得的本地数据不许配加载动画');
    });

    testWidgets('进行中任务显示静态进度条（无无限动画，pumpAndSettle 可收敛）',
        (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          TaskItem(
            id: 'dl:C:\\tmp\\a.zip',
            threadId: 'th-1',
            kind: TaskKind.download,
            title: 'a.zip',
            status: TaskStatus.running,
            progress: 0.6,
            startedAtMs: t0,
          ),
        ]),
      );
      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(find.text('a.zip'), findsOneWidget);
      // 静态进度条不是 CircularProgressIndicator：它不参与 pumpAndSettle 的
      // 无限动画等待（Kotlin 侧是无限扫动，这里按页面文件头说明改静态）。
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });

    testWidgets('行内徽章与分区头同源：失败行进「未完成」分区', (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-1', title: '在跑'),
          msgTask('th-2', title: '挂了', status: TaskStatus.failed),
        ]),
      );

      expect(find.text('正在执行'), findsWidgets,
          reason: '分区头与行内徽章同一个词（findsWidgets = 头 + 徽章两处）');
      expect(find.text('未完成'), findsWidgets);
    });
  });

  group('未读徽标', () {
    testWidgets('未读数 = 未读任务数；点按全部标为已读后归零', (tester) async {
      final TaskCenterStore store = storeOf(<TaskItem>[
        msgTask('th-1', read: false),
        msgTask('th-2', title: '第二条', read: false),
        msgTask('th-3', title: '已看过', read: true),
      ]);

      await pumpPage(tester, store: store);

      final Finder unreadCard =
          find.byKey(const ValueKey<String>('task-center-unread'));
      expect(find.descendant(of: unreadCard, matching: find.text('2')),
          findsOneWidget);

      await tester.tap(unreadCard);
      await tester.pump();

      expect(store.unreadCount, 0, reason: '显式全读（本页不用 Kotlin 的自动已读）');
      expect(find.descendant(of: unreadCard, matching: find.text('0')),
          findsOneWidget);
    });

    testWidgets('无未读时徽标不可点（没有假动作）', (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[msgTask('th-1', read: true)]),
      );

      final Finder unreadCard =
          find.byKey(const ValueKey<String>('task-center-unread'));
      expect(find.descendant(of: unreadCard, matching: find.text('0')),
          findsOneWidget);

      await tester.tap(unreadCard, warnIfMissed: false);
      await tester.pump();

      expect(find.text('修复登录页崩溃'), findsOneWidget,
          reason: '点不可点的卡片不该有任何反应');
    });

    testWidgets('点单个任务行 → 该条标已读（未读 -1）', (tester) async {
      final TaskCenterStore store = storeOf(<TaskItem>[
        msgTask('th-1'),
        msgTask('th-2', title: '第二条'),
      ]);

      await pumpPage(tester, store: store);
      expect(store.unreadCount, 2);

      await tester.tap(find.text('修复登录页崩溃'));
      await tester.pump();

      expect(store.unreadCount, 1);
    });
  });

  group('本地动作', () {
    testWidgets('行尾移除：从列表消失，未读数同步减', (tester) async {
      final TaskCenterStore store = storeOf(<TaskItem>[
        msgTask('th-1', title: '留下'),
        msgTask('th-2', title: '删我'),
      ]);

      await pumpPage(tester, store: store);
      expect(store.unreadCount, 2);

      await tester.tap(find.byKey(
        const ValueKey<String>('task-row-msg:th-2-remove'),
      ));
      await tester.pump();

      expect(find.text('删我'), findsNothing);
      expect(find.text('留下'), findsOneWidget);
      expect(store.unreadCount, 1);
    });

    testWidgets('「清理已完成的任务」只清终局，RUNNING 不动', (tester) async {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('th-1', '在跑');
      store.trackMessage('th-2', '已收尾');
      store.finishMessage('th-2', ok: true);

      await pumpPage(tester, store: store);

      await tester.tap(find.byKey(const ValueKey<String>('task-center-clear-finished')));
      await tester.pump();

      expect(find.text('在跑'), findsOneWidget);
      expect(find.text('已收尾'), findsNothing);
    });

    testWidgets('清理按钮只在有终局任务时出现', (tester) async {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('th-1', '在跑');

      await pumpPage(tester, store: store);

      expect(
        find.byKey(const ValueKey<String>('task-center-clear-finished')),
        findsNothing,
        reason: '全在跑时没有可清理的（Kotlin hasFinished 同款）',
      );
    });

    testWidgets('点消息行 → 回来源会话回调（带 threadId）', (tester) async {
      final List<String> opened = <String>[];
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[msgTask('th-42')]),
        onOpenThread: opened.add,
      );

      await tester.tap(find.text('修复登录页崩溃'));
      await tester.pump();

      expect(opened, <String>['th-42']);
    });

    testWidgets('点下载行 → 用户数据管理回调（不注入则不跳转也不报错）', (tester) async {
      final TaskCenterStore store = storeOf(<TaskItem>[
        dlTask('C:\\tmp\\a.zip', status: TaskStatus.done),
      ]);
      await pumpPage(tester, store: store);

      await tester.tap(find.text('文件下载'));
      await tester.pump();
      await tester.tap(find.text('a.zip'));
      await tester.pump();

      expect(store.snapshot.single.read, isTrue,
          reason: '点了就标已读——跳转未接线不影响已读这个本地事实');
    });
  });

  group('重试出口（无位不显示假入口）', () {
    testWidgets('不注入重试回调 → 没有重试按钮，文案不指向不存在的按钮',
        (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-1', title: '挂了', status: TaskStatus.failed),
        ]),
      );

      expect(find.byKey(const ValueKey<String>('task-row-msg:th-1-retry')),
          findsNothing,
          reason: 'TurnSession.resyncFromDaemon 未接线：没有按钮就没有假入口');
      expect(find.text('去会话里找回这次的结果'), findsOneWidget,
          reason: '文案与真实存在的出口对齐（点行进会话）');
      expect(find.text('点右侧重试，找回这次的结果'), findsNothing);
      // 移除永远在（纯本地动作）。
      expect(find.byKey(const ValueKey<String>('task-row-msg:th-1-remove')),
          findsOneWidget);
    });

    testWidgets('注入 onRetryMessage → 失败消息行出重试按钮，点按回传 threadId',
        (tester) async {
      final List<String> retried = <String>[];
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-42', title: '挂了', status: TaskStatus.failed),
        ]),
        onRetryMessage: retried.add,
      );

      await tester.tap(
          find.byKey(const ValueKey<String>('task-row-msg:th-42-retry')));
      await tester.pump();

      expect(retried, <String>['th-42']);
      await drainRetryCooldown(tester);
    });

    testWidgets('注入 onRetryDownload → URL 来源失败下载行出「重新下载」', (tester) async {
      final List<TaskItem> retried = <TaskItem>[];
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          TaskItem(
            id: 'dl:https://x.test/a.png',
            threadId: 'th-1',
            kind: TaskKind.download,
            title: 'a.png',
            status: TaskStatus.failed,
            startedAtMs: t0,
          ),
        ]),
        onRetryDownload: retried.add,
      );

      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(
        find.byKey(const ValueKey<String>(
            'task-row-dl:https://x.test/a.png-retry')),
        findsOneWidget,
        reason: 'URL 来源只要有网就能重下',
      );
      await tester.tap(find.byKey(
        const ValueKey<String>('task-row-dl:https://x.test/a.png-retry'),
      ));
      await tester.pump();

      expect(retried, hasLength(1));
      expect(taskFileKey(retried.single), 'https://x.test/a.png');
      await drainRetryCooldown(tester);
    });

    testWidgets('blob 来源（无可重放来源）→ 没有重下按钮，文案说清只能移除',
        (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          TaskItem(
            id: 'dl:blob:https://x.test/abc',
            threadId: 'th-1',
            kind: TaskKind.download,
            title: 'blob 文件',
            status: TaskStatus.failed,
            startedAtMs: t0,
          ),
        ]),
        onRetryDownload: (_) {},
      );

      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(
        find.byKey(const ValueKey<String>('task-row-dl:blob:https://x.test/abc-retry')),
        findsNothing,
      );
      expect(find.byIcon(Icons.refresh_rounded), findsNothing);
      expect(find.text('内容在会话里还在；可回会话重新保存，或移除这条记录'),
          findsOneWidget);
    });

    testWidgets('重试冷却门闩：冷却内连点被吞、冷却外放行', (tester) async {
      final List<String> retried = <String>[];
      // 可控时钟：fake_async 接管 Future.delayed 但不接管系统时钟，门闩的 now
      // 必须由这里给（生产走系统时钟，与 Kotlin System.currentTimeMillis 同源）。
      int clock = t0;
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-42', title: '挂了', status: TaskStatus.failed),
        ]),
        onRetryMessage: retried.add,
        nowMs: () => clock,
      );

      final Finder retryBtn =
          find.byKey(const ValueKey<String>('task-row-msg:th-42-retry'));
      final Finder retryingLabel =
          find.byKey(const ValueKey<String>('task-row-msg:th-42-retrying'));

      await tester.tap(retryBtn);
      await tester.pump();
      expect(retried, <String>['th-42']);
      expect(retryingLabel, findsOneWidget, reason: '冷却窗口内显示「重试中」');

      // 冷却内（差 1ms）推进时钟再点：门闩拦下。
      clock += kTaskRetryCooldownMs - 1;
      await drainRetryCooldown(tester);
      expect(retryingLabel, findsNothing, reason: '占位随冷却结束消失');
      await tester.tap(retryBtn);
      await tester.pump();
      expect(retried, <String>['th-42'],
          reason: 'now-last < cooldown 判负：差 1ms 都不放行');

      // 冷却外：放行，第二次重试真的发出去。
      clock += 1;
      await tester.tap(retryBtn);
      await tester.pump();
      expect(retried, <String>['th-42', 'th-42'],
          reason: '冷却外放行 = 门闩只挡连点，不挡正常重试');
      await drainRetryCooldown(tester);
    });

    testWidgets('冷却窗口内按钮被「重试中」占位替换（连点物理上点不到）',
        (tester) async {
      int retries = 0;
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-42', title: '挂了', status: TaskStatus.failed),
        ]),
        onRetryMessage: (_) => retries += 1,
      );

      final Finder retryBtn =
          find.byKey(const ValueKey<String>('task-row-msg:th-42-retry'));
      await tester.tap(retryBtn);
      await tester.pump();
      // 冷却窗口内按钮被占位替换：此时物理上不存在可点的重试按钮。
      expect(find.byKey(const ValueKey<String>('task-row-msg:th-42-retry')),
          findsNothing);

      expect(retries, 1,
          reason: '1500ms 冷却内连点防护（Kotlin RetryGate 同款）');
      await drainRetryCooldown(tester);
    });
  });

  group('离线分支（hostOffline）', () {
    testWidgets('消息类运行中：徽章隐去 + 「离线 · 恢复连接后继续」', (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[msgTask('th-1')]),
        hostOffline: true,
      );

      expect(find.text('离线 · 恢复连接后继续'), findsOneWidget);
      expect(find.text('离线待恢复'), findsOneWidget, reason: '分区头同步改名');
      expect(
        find.descendant(
          of: find.byType(PAStatusChip),
          matching: find.text('正在执行'),
        ),
        findsNothing,
        reason: '断连时消息类执行状态不可信（Kotlin README：未连接时绝不显示）',
      );
    });

    testWidgets('下载类运行中断连照常：徽章与分区头都是「正在执行」', (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[dlTask('C:\\tmp\\a.zip')]),
        hostOffline: true,
      );

      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(find.text('正在执行'), findsWidgets,
          reason: '下载是手机本机行为，断连也照常归执行分区');
      expect(find.text('离线 · 恢复连接后继续'), findsNothing);
    });

    testWidgets('断连 + 消息类失败：不给重试入口（即使注入了回调）', (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-1', title: '挂了', status: TaskStatus.failed),
        ]),
        hostOffline: true,
        onRetryMessage: (_) {},
      );

      expect(find.byKey(const ValueKey<String>('task-row-msg:th-1-retry')),
          findsNothing,
          reason: '离线时消息类重试必然失败，不让人白点');
      expect(find.text('恢复连接后去会话里找回这次的结果'), findsOneWidget);
    });
  });

  group('分区', () {
    testWidgets('五类任务按 正在执行/已完成/未完成 分区，空组不出现', (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          msgTask('th-run', title: '在跑'),
          msgTask('th-done', title: '完了', status: TaskStatus.done),
          msgTask('th-failed', title: '挂了', status: TaskStatus.failed),
        ]),
      );

      // 分区头 = 状态词本身（与行内徽章同源）：头 + 徽章两处同词。
      for (final String word in <String>['正在执行', '已完成', '未完成']) {
        expect(find.text(word), findsWidgets,
            reason: '分区头与行内徽章长出同一个词（Kotlin 分区头纪律）');
      }
      expect(find.text('等待你处理'), findsOneWidget,
          reason: '四词分区没有「等待你处理」组（Kotlin 该组恒空，死分支不搬）');
      expect(find.text('离线待恢复'), findsNothing, reason: '在线时没有该分区');
    });

    testWidgets('两页签各自分组：下载任务的失败行只在下载页签的「未完成」组',
        (tester) async {
      await pumpPage(
        tester,
        store: storeOf(<TaskItem>[
          TaskItem(
            id: 'dl:https://x.test/a.png',
            threadId: 'th-1',
            kind: TaskKind.download,
            title: '图.png',
            status: TaskStatus.failed,
            startedAtMs: t0,
          ),
        ]),
      );

      // 消息页签：空态（没有消息任务）
      expect(find.text('还没有处理中的任务'), findsOneWidget);

      await tester.tap(find.text('文件下载'));
      await tester.pump();

      expect(find.text('图.png'), findsOneWidget);
      expect(find.text('未完成'), findsWidgets);
      expect(find.text('还没有下载任务'), findsNothing);
    });
  });
}
