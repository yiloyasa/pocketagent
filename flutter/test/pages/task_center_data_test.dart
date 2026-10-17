// TaskCenter 数据层测试：store 行为（add/update/remove/unread/Stream 通知）+
// 磁盘编解码往返 + 未读/统计/分区纯函数 + 重试门闩。
//
// 【为什么全在这里不拖 widget】store 与纯函数是本页唯一的业务逻辑，必须能在不拖
// MaterialApp 的情况下被钉死；widget 测试（task_center_page_test.dart）只负责
// 渲染与交互。两层都绿才算任务中心没坏。

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/pages/task_center_data.dart';

void main() {
  // 固定时钟轴：nowMs 显式注入，避免依赖真系统时钟（Kotlin 侧 System.currentTimeMillis
  // 在 Dart 侧全部收口成注入点）。
  const int t0 = 1728901200000; // 2024-10-14 13:40 (UTC+8)

  group('TaskCenterStore：消息任务生命周期', () {
    test('trackMessage → RUNNING + 未读 1；finishMessage(ok) → DONE + 仍算未读', () {
      final TaskCenterStore store = TaskCenterStore();

      final String id = store.trackMessage('th-1', '修复登录页崩溃', nowMs: t0);

      expect(id, 'msg:th-1', reason: '消息任务 id = msg:<threadId>（Kotlin 同约定）');
      expect(store.snapshot, hasLength(1));
      expect(store.snapshot.single.status, TaskStatus.running);
      expect(store.snapshot.single.progress, 0);
      expect(store.unreadCount, 1);

      store.finishMessage('th-1', ok: true, detail: '3 处改动');

      expect(store.snapshot.single.status, TaskStatus.done);
      expect(store.snapshot.single.progress, 1);
      expect(store.snapshot.single.detail, '3 处改动');
      // 未读只在 markRead / markAllRead 时清：完成 ≠ 看过（Kotlin 同语义）。
      expect(store.unreadCount, 1);
    });

    test('finishMessage(ok=false) → FAILED，进度不冒充满', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('th-1', '跑单测', nowMs: t0);
      store.updateDownload('dl:x', 0.4); // 与消息任务无关，纯噪音

      store.finishMessage('th-1', ok: false, detail: 'agent 崩了');

      final TaskItem t = store.snapshot.firstWhere((TaskItem e) => e.id == 'msg:th-1');
      expect(t.status, TaskStatus.failed);
      expect(t.progress, 0, reason: '失败不把进度画成满格');
      expect(t.detail, 'agent 崩了');
    });

    test('同一会话连发：RUNNING 中合并轮次（一条记录、轮次+1、重新计未读）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('th-1', '第一轮', nowMs: t0);
      store.markRead('msg:th-1');
      expect(store.unreadCount, 0);

      store.trackMessage('th-1', '第二轮', nowMs: t0 + 60000);

      expect(store.snapshot, hasLength(1), reason: '同一会话只此一条');
      final TaskItem t = store.snapshot.single;
      expect(t.rounds, 2);
      expect(t.title, '第二轮');
      expect(t.read, isFalse, reason: '新一轮 = 新未读');
      expect(t.startedAtMs, t0 + 60000, reason: '起始时间重置到最新一轮');
      expect(store.unreadCount, 1);
    });

    test('同会话已终局后再 trackMessage：换新记录（旧终局被替换，不叠加）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('th-1', '跑单测', nowMs: t0);
      store.finishMessage('th-1', ok: true);

      store.trackMessage('th-1', '再跑一轮', nowMs: t0 + 60000);

      expect(store.snapshot, hasLength(1));
      final TaskItem t = store.snapshot.single;
      expect(t.status, TaskStatus.running, reason: 'Kotlin 同款：终局后重开是新执行');
      expect(t.rounds, 1);
      expect(t.title, '再跑一轮');
    });

    test('finishMessage 对不存在的会话 = no-op（不新建、不发通知）', () {
      final TaskCenterStore store = TaskCenterStore();
      int notifications = 0;
      store.tasks.listen((_) => notifications += 1);

      store.finishMessage('ghost', ok: true);

      expect(store.snapshot, isEmpty);
      expect(notifications, 0, reason: 'no-op 不许惊动 UI');
    });
  });

  group('TaskCenterStore：下载任务生命周期', () {
    test('trackDownload → RUNNING；updateDownload 单调不减、非空 detail 覆盖', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('C:\\tmp\\a.zip', 'a.zip', nowMs: t0);

      store.updateDownload('dl:C:\\tmp\\a.zip', 0.3, detail: '30%');
      store.updateDownload('dl:C:\\tmp\\a.zip', 0.1, detail: '乱序分片');
      store.updateDownload('dl:C:\\tmp\\a.zip', 0.8);

      final TaskItem t = store.snapshot.single;
      expect(t.progress, 0.8, reason: '进度单调不减：乱序回调不许回退');
      expect(t.detail, '乱序分片', reason: '非空 detail 覆盖旧的');
    });

    test('updateDownload 空 detail 不覆盖（本次没有新话说）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);
      store.updateDownload('dl:k', 0.3, detail: '30%');

      store.updateDownload('dl:k', 0.5, detail: '');

      final TaskItem t = store.snapshot.single;
      expect(t.progress, 0.5);
      expect(t.detail, '30%');
    });

    test('finishDownload 记录本地 uri 与大小；ok=false → FAILED', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('https://x.test/a.png', 'a.png', nowMs: t0);

      store.finishDownload(
        'dl:https://x.test/a.png',
        ok: true,
        detail: '已存到 下载/PocketAgent',
        localUri: 'content://media/42',
        sizeText: '1.2MB',
      );

      final TaskItem t = store.snapshot.single;
      expect(t.status, TaskStatus.done);
      expect(t.progress, 1);
      expect(t.localUri, 'content://media/42');
      expect(t.sizeText, '1.2MB');
    });

    test('同文件还在跑时重复 trackDownload = 原样返回（不重置进度）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);
      store.updateDownload('dl:k', 0.6);

      final String again = store.trackDownload('k', 'a.zip', nowMs: t0 + 1000);

      expect(again, 'dl:k');
      expect(store.snapshot.single.progress, 0.6,
          reason: '重复触发只是噪音，不许把已下载的进度洗掉');
      expect(store.snapshot.single.startedAtMs, t0);
    });

    test('已终局任务再 trackDownload = 复活重跑（进度/详情/uri 全清）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);
      store.finishDownload('dl:k', ok: true, localUri: 'content://media/42', sizeText: '9MB');
      store.markRead('dl:k');

      store.trackDownload('k', 'a.zip', nowMs: t0 + 60000);

      final TaskItem t = store.snapshot.single;
      expect(t.status, TaskStatus.running);
      expect(t.progress, 0);
      expect(t.localUri, isEmpty, reason: '重跑后旧文件位置失效');
      expect(t.sizeText, isEmpty);
      expect(t.read, isFalse, reason: '复活 = 新一轮未读');
    });

    test('annotateDownload 补来源会话标题；空值不覆盖', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);

      store.annotateDownload('dl:k', '');
      expect(store.snapshot.single.sourceTitle, isEmpty);

      store.annotateDownload('dl:k', '修复登录页');
      expect(store.snapshot.single.sourceTitle, '修复登录页');
    });

    test('attachLocalUriIfEmpty 只填空不覆盖（老版本 uri 兜底）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);
      store.attachLocalUriIfEmpty('dl:k', 'content://old');

      store.attachLocalUriIfEmpty('dl:k', 'content://new');

      expect(store.snapshot.single.localUri, 'content://old');
    });
  });

  group('TaskCenterStore：已读 / 未读 / 移除 / 清理', () {
    test('markRead 单条已读；已读再点 = no-op', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('a', 'A', nowMs: t0);
      store.trackMessage('b', 'B', nowMs: t0);
      expect(store.unreadCount, 2);

      store.markRead('msg:a');
      expect(store.unreadCount, 1);

      int notifications = 0;
      store.tasks.listen((_) => notifications += 1);
      store.markRead('msg:a');
      expect(notifications, 0, reason: '已读再点不许发通知');
    });

    test('unreadCount 只数未读；markAllRead 后归零且幂等', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('a', 'A', nowMs: t0);
      store.trackMessage('b', 'B', nowMs: t0);
      store.markRead('msg:b');
      expect(store.unreadCount, 1);

      store.markAllRead();
      expect(store.unreadCount, 0);

      int notifications = 0;
      store.tasks.listen((_) => notifications += 1);
      store.markAllRead();
      expect(notifications, 0, reason: '无未读时全读 = no-op');
    });

    test('remove 单条；不存在的 id = no-op', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('a', 'A', nowMs: t0);
      store.trackMessage('b', 'B', nowMs: t0);

      store.remove('msg:a');
      expect(store.snapshot.map((TaskItem t) => t.id), <String>['msg:b']);

      int notifications = 0;
      store.tasks.listen((_) => notifications += 1);
      store.remove('msg:ghost');
      expect(notifications, 0);
      expect(store.snapshot, hasLength(1));
    });

    test('removeByThread 清该会话全部任务（消息 + 挂 threadId 的下载）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('th-1', 'A', nowMs: t0);
      store.trackDownload('k1', 'a.zip', threadId: 'th-1', nowMs: t0);
      store.trackMessage('th-2', 'B', nowMs: t0);

      store.removeByThread('th-1');

      expect(store.snapshot.map((TaskItem t) => t.id), <String>['msg:th-2']);
      store.removeByThread('');
      expect(store.snapshot, hasLength(1), reason: '空 threadId 直接返回（Kotlin 同款）');
    });

    test('clearFinished 只清终局；RUNNING 一概不碰', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('a', 'A', nowMs: t0);
      store.trackMessage('b', 'B', nowMs: t0);
      store.finishMessage('a', ok: true);
      store.markRead('msg:a');

      store.clearFinished();

      expect(store.snapshot.map((TaskItem t) => t.id), <String>['msg:b']);
      expect(store.snapshot.single.status, TaskStatus.running);
    });

    test('CANCELED 算终局：清理已完成会带走它（Kotlin status != RUNNING 口径）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('a', 'A', nowMs: t0);
      // 直接经磁盘格式写入一条 CANCELED（公开方法面没有取消入口——取消由
      // TurnSession 侧触发，store 只承接状态；见 Kotlin 同款方法面）。
      final TaskCenterStore seeded = TaskCenterStore(
        prefs: InMemoryTaskCenterPrefs()
          ..write(TaskCenterDiskSnapshot(
            tasksBlob: encodeTaskBlob(<TaskItem>[
              TaskItem(
                id: 'msg:c',
                threadId: 'th-c',
                kind: TaskKind.message,
                title: 'C',
                status: TaskStatus.canceled,
                startedAtMs: t0,
              ),
            ]),
            unread: 0,
          )),
      );
      expect(seeded.snapshot.single.status, TaskStatus.canceled);
      seeded.clearFinished();
      expect(seeded.snapshot, isEmpty);
    });
  });

  group('TaskCenterStore：Stream 通知', () {
    test('add/update/remove 都发通知；快照内容逐条相等', () async {
      final TaskCenterStore store = TaskCenterStore();
      final List<List<TaskItem>> seen = <List<TaskItem>>[];
      store.tasks.listen(seen.add);

      store.trackMessage('a', 'A', nowMs: t0);
      store.finishMessage('a', ok: true);
      store.remove('msg:a');

      // 让 broadcast 流把已派发的事件逐个送达（微任务队列）。
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      expect(seen, hasLength(3));
      expect(seen.last, isEmpty, reason: '最后一次通知 = remove 后的空清单');
      expect(seen[0].single.status, TaskStatus.running);
      expect(seen[1].single.status, TaskStatus.done);
    });

    test('updateDownload 进度推进也发通知', () async {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      final List<List<TaskItem>> seen = <List<TaskItem>>[];
      store.tasks.listen(seen.add);
      store.updateDownload('dl:k', 0.5);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      expect(seen, hasLength(1));
      expect(seen.single.single.progress, 0.5);
    });

    test('updateDownload 值没变（含乱序回退）= 不发通知', () async {
      final TaskCenterStore store = TaskCenterStore();
      store.trackDownload('k', 'a.zip', nowMs: t0);
      store.updateDownload('dl:k', 0.5);
      await Future<void>.delayed(Duration.zero);

      int notifications = 0;
      store.tasks.listen((_) => notifications += 1);
      store.updateDownload('dl:k', 0.5);
      store.updateDownload('dl:k', 0.2, detail: '');
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      expect(notifications, 0);
    });

    test('订阅前的变化不补（broadcast 语义；快照同步读）', () {
      final TaskCenterStore store = TaskCenterStore();
      store.trackMessage('a', 'A', nowMs: t0);

      final List<List<TaskItem>> seen = <List<TaskItem>>[];
      store.tasks.listen(seen.add);

      expect(seen, isEmpty, reason: '订阅即收、订阅前不补——与 Kotlin StateFlow 不同，');
      // 快照是同步真值（本地 store 没有加载态可言）。
      expect(store.snapshot, hasLength(1));
    });
  });

  // 判决组：把「为什么 TaskCenterStore 不需要 close」钉成可回归的断言。
  // 依据在 task_center_data.dart 类头【生命周期 = 实例；刻意不提供 close】
  // 与【唯一的增长向量：订阅者不取消】：未 close 的 broadcast controller 只按
  // 活跃订阅占内存、不按事件条数占；store 随实例存活，与页面一起回收。
  // 下面三条各钉住这条依据的一块，任何人把它改成「会积累」的形态都会红。
  group('TaskCenterStore：通知流的生命周期契约（无 close 的依据）', () {
    test('无人监听时的连续变更不留存：新订阅者只收后续那一次，不补旧账', () async {
      final TaskCenterStore store = TaskCenterStore();

      // 先不订阅：连续 50 次变更，每次 _commit 都会走到 _controller.add。
      for (int i = 0; i < 50; i++) {
        store.trackMessage('th-$i', 'T$i', nowMs: t0 + i);
      }

      final List<List<TaskItem>> seen = <List<TaskItem>>[];
      store.tasks.listen(seen.add);
      store.trackMessage('after', '订阅之后', nowMs: t0 + 999);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      expect(seen, hasLength(1),
          reason: '缺席的监听者不缓冲：未 close 因此不会按事件条数堆积'
              '（若改成带重放/缓冲的流，这里会收到 51 条）');
      expect(seen.single, hasLength(51));
      expect(seen.single.map((TaskItem t) => t.id), contains('msg:after'),
          reason: '收到的那一条就是订阅之后的变更');
      expect(store.snapshot, hasLength(51),
          reason: '「不留存」说的是通知流，不是清单真值——快照一条不丢');
    });

    test('退订后不再收通知；后续订阅者照常收（可释放的是订阅，不是历史）', () async {
      final TaskCenterStore store = TaskCenterStore();
      final List<List<TaskItem>> first = <List<TaskItem>>[];
      final StreamSubscription<List<TaskItem>> sub = store.tasks.listen(first.add);

      store.trackMessage('a', 'A', nowMs: t0);
      await Future<void>.delayed(Duration.zero);
      await sub.cancel();

      store.trackMessage('b', 'B', nowMs: t0);
      await Future<void>.delayed(Duration.zero);
      expect(first.map((List<TaskItem> next) => next.last.id), <String>['msg:a'],
          reason: '退订 = 订阅方释放自己那一份；store 不需要替它做任何关闭动作');

      final List<List<TaskItem>> second = <List<TaskItem>>[];
      store.tasks.listen(second.add);
      store.trackMessage('c', 'C', nowMs: t0);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);
      expect(second.map((List<TaskItem> next) => next.last.id), <String>['msg:c'],
          reason: '新订阅者照常收，且不补退订期间的变化（要全量真值走 store.snapshot）');
    });

    test('两个实例互不串味：通知流随实例走，不是隐式 app 级单例', () async {
      final TaskCenterStore one = TaskCenterStore();
      final TaskCenterStore two = TaskCenterStore();
      final List<List<TaskItem>> a = <List<TaskItem>>[];
      final List<List<TaskItem>> b = <List<TaskItem>>[];
      one.tasks.listen(a.add);
      two.tasks.listen(b.add);

      one.trackMessage('x', 'X', nowMs: t0);
      await Future<void>.delayed(Duration.zero);
      await Future<void>.delayed(Duration.zero);

      expect(a, hasLength(1));
      expect(b, isEmpty,
          reason: '生命周期 = 实例。B4 若真把 store 提为 app 级单例（通知流挂到进程'
              '级 controller），这条必须先改：届时「无 close」的前提就变了，'
              '得同时补 owner 侧退订与 close 契约，不许悄悄改绿');
      expect(two.snapshot, isEmpty);
    });
  });

  group('磁盘编解码往返（与 Kotlin writeDisk/parseTasks 同格式）', () {
    test('encode → decode 逐字段还原（含中文/空串/进度/uri）', () {
      final List<TaskItem> tasks = <TaskItem>[
        TaskItem(
          id: 'msg:th-1',
          threadId: 'th-1',
          kind: TaskKind.message,
          title: '修复登录页崩溃',
          status: TaskStatus.failed,
          progress: 0.42,
          detail: 'agent 崩了\n第二行',
          read: true,
          startedAtMs: t0,
          rounds: 3,
        ),
        TaskItem(
          id: 'dl:C:\\tmp\\a.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'a.zip',
          status: TaskStatus.done,
          progress: 1,
          read: false,
          startedAtMs: t0 + 1,
          localUri: 'content://media/42',
          sourceTitle: '来自会话标题',
          sizeText: '1.2MB',
        ),
      ];

      final String blob = encodeTaskBlob(tasks);
      final List<TaskItem> back = decodeTaskBlob(blob);

      expect(back, hasLength(2));
      expect(back[0].id, tasks[0].id);
      expect(back[0].kind, TaskKind.message);
      expect(back[0].status, TaskStatus.failed);
      expect(back[0].progress, closeTo(0.42, 1e-9));
      expect(back[0].read, isTrue);
      expect(back[0].startedAtMs, t0);
      expect(back[0].rounds, 3);
      // 字段值里的换行被替换成空格（Kotlin 同款 sanitize）：解码后不炸、行数不错。
      expect(back[0].detail, 'agent 崩了 第二行');
      expect(back[1].id, tasks[1].id);
      expect(back[1].localUri, 'content://media/42');
      expect(back[1].sourceTitle, '来自会话标题');
      expect(back[1].sizeText, '1.2MB');
    });

    test('往返保序：磁盘顺序 = 写入顺序（人不重排，daemon/写入方也不重排）', () {
      final List<TaskItem> tasks = <TaskItem>[
        TaskItem(id: 'msg:b', threadId: 'b', kind: TaskKind.message, title: 'B', status: TaskStatus.running, startedAtMs: t0 + 2),
        TaskItem(id: 'msg:a', threadId: 'a', kind: TaskKind.message, title: 'A', status: TaskStatus.running, startedAtMs: t0 + 1),
      ];

      expect(
        decodeTaskBlob(encodeTaskBlob(tasks)).map((TaskItem t) => t.id),
        <String>['msg:b', 'msg:a'],
      );
    });

    test('恢复规则：落盘 RUNNING → INTERRUPTED（杀进程的任务不可能还活着）', () {
      final String blob = encodeTaskBlob(<TaskItem>[
        TaskItem(
          id: 'msg:a',
          threadId: 'a',
          kind: TaskKind.message,
          title: 'A',
          status: TaskStatus.running,
          startedAtMs: t0,
        ),
      ]);

      final List<TaskItem> back = decodeTaskBlob(blob);

      expect(back.single.status, TaskStatus.interrupted,
          reason: '「不知道」不许画成「正在执行」（假绿）');
    });

    test('坏行整行丢弃：字段不足/未知枚举/坏时间戳', () {
      const String blob = 'msg:a\u0001a\u0001MESSAGE\u0001A\u0001RUNNING\u00010\u0001'
          '$t0\u00010\u0001\u00011\n'
          'msg:b\u0001b\u0001MESSAGE\u0001B\n' // 字段不足
          'msg:c\u0001c\u0001MESSAGE\u0001C\u0001WEIRD\u00010\u0001$t0\u00010\u0001\u00011\n' // 未知状态
          'msg:d\u0001d\u0001MESSAGE\u0001D\u0001DONE\u00010\u0001abc\u00010\u0001\u00011\n'; // 坏时间戳

      final List<TaskItem> back = decodeTaskBlob(blob);

      expect(back.map((TaskItem t) => t.id), <String>['msg:a']);
      expect(back.single.status, TaskStatus.interrupted);
    });

    test('老格式（10 字段，无 uri/sourceTitle/sizeText）向后兼容', () {
      const String legacy = 'msg:a\u0001a\u0001MESSAGE\u0001A\u0001DONE\u00011\u0001'
          '$t0\u00011\u0001done\u00012';

      final List<TaskItem> back = decodeTaskBlob(legacy);

      expect(back, hasLength(1));
      final TaskItem t = back.single;
      expect(t.id, 'msg:a');
      expect(t.status, TaskStatus.done);
      expect(t.rounds, 2);
      expect(t.localUri, isEmpty);
      expect(t.sourceTitle, isEmpty);
      expect(t.sizeText, isEmpty);
    });

    test('空 blob → 空清单；空白行不炸', () {
      expect(decodeTaskBlob(''), isEmpty);
      expect(decodeTaskBlob('\n\n  \n'), isEmpty);
    });

    test('store 构造即恢复；未读取 max(落盘值, 清单实数)', () async {
      final InMemoryTaskCenterPrefs prefs = InMemoryTaskCenterPrefs();
      await prefs.write(TaskCenterDiskSnapshot(
        tasksBlob: encodeTaskBlob(<TaskItem>[
          TaskItem(
            id: 'msg:a',
            threadId: 'a',
            kind: TaskKind.message,
            title: 'A',
            status: TaskStatus.interrupted,
            read: false,
            startedAtMs: t0,
          ),
          TaskItem(
            id: 'msg:b',
            threadId: 'b',
            kind: TaskKind.message,
            title: 'B',
            status: TaskStatus.done,
            read: false,
            startedAtMs: t0,
          ),
        ]),
        // 落盘未读数（1）小于清单实数（2）：以实数兜底，徽章不虚报也不漏报。
        unread: 1,
      ));

      final TaskCenterStore store = TaskCenterStore(prefs: prefs);

      expect(store.snapshot, hasLength(2));
      expect(store.unreadCount, 2, reason: 'Kotlin maxOf 同款口径');
    });

    test('写路径落盘：本地操作后 prefs 里的 blob 与新快照一致', () async {
      final InMemoryTaskCenterPrefs prefs = InMemoryTaskCenterPrefs();
      final TaskCenterStore store = TaskCenterStore(prefs: prefs);

      store.trackMessage('a', 'A', nowMs: t0);
      store.markRead('msg:a');

      final TaskCenterDiskSnapshot? snap = prefs.read();
      expect(snap, isNotNull);
      expect(snap!.unread, 0);
      expect(
        decodeTaskBlob(snap.tasksBlob).map((TaskItem t) => t.id),
        <String>['msg:a'],
        reason: '落盘的就是当前清单（RUNNING 原样落盘；恢复时才翻 INTERRUPTED）',
      );
    });
  });

  group('纯函数：未读 / 统计 / 分区', () {
    TaskItem task(
      String id, {
      TaskKind kind = TaskKind.message,
      TaskStatus status = TaskStatus.running,
      bool read = false,
      int startedAtMs = t0,
      String threadId = 'th-1',
    }) =>
        TaskItem(
          id: id,
          threadId: threadId,
          kind: kind,
          title: 'T$id',
          status: status,
          startedAtMs: startedAtMs,
          read: read,
        );

    test('unreadCountOf 只数未读', () {
      expect(
        unreadCountOf(<TaskItem>[
          task('a'),
          task('b', read: true),
          task('c'),
        ]),
        2,
      );
    });

    test('taskStatsOf：等待你处理只数 FAILED+INTERRUPTED（CANCELED 不算）', () {
      final TaskCenterStats stats = taskStatsOf(<TaskItem>[
        task('a', status: TaskStatus.failed),
        task('b', status: TaskStatus.interrupted),
        task('c', status: TaskStatus.canceled),
        task('d', status: TaskStatus.running),
        task('e', status: TaskStatus.done),
      ]);

      expect(stats.waitingForYou, 2,
          reason: 'CANCELED 是用户自己取消的，不需要用户回来处理');
      expect(stats.running, 1);
      expect(stats.completed, 1);
    });

    test('taskPartitionRows（在线）：正在执行 / 已完成 / 未完成，空组不出现', () {
      final List<TaskPartitionRow> rows = taskPartitionRows(
        <TaskItem>[
          task('run-1'),
          task('run-2', kind: TaskKind.download),
          task('done-1', status: TaskStatus.done),
          task('failed-1', status: TaskStatus.failed),
          task('canceled-1', status: TaskStatus.canceled),
          task('interrupted-1', status: TaskStatus.interrupted),
        ],
        hostOffline: false,
      );

      expect(rows.map((TaskPartitionRow r) => r.word),
          <String>['正在执行', '已完成', '未完成']);
      expect(rows[0].tasks.map((TaskItem t) => t.id), <String>['run-1', 'run-2']);
      expect(rows[1].tasks.map((TaskItem t) => t.id), <String>['done-1']);
      expect(rows[2].tasks.map((TaskItem t) => t.id),
          <String>['failed-1', 'canceled-1', 'interrupted-1'],
          reason: 'CANCELED 归「未完成」（Kotlin 同款：失败/取消/中断都是没成）');
    });

    test('taskPartitionRows（离线）：消息类运行中落「离线待恢复」，下载类照常执行', () {
      final List<TaskPartitionRow> rows = taskPartitionRows(
        <TaskItem>[
          task('msg-run'),
          task('dl-run', kind: TaskKind.download),
          task('done-1', status: TaskStatus.done),
        ],
        hostOffline: true,
      );

      expect(rows.map((TaskPartitionRow r) => r.word),
          <String>['离线待恢复', '正在执行', '已完成']);
      expect(rows[0].offlineResume, isTrue);
      expect(rows[0].tasks.map((TaskItem t) => t.id), <String>['msg-run'],
          reason: '断连时消息类执行状态不可信（Kotlin README：未连接时绝不显示执行中）');
      expect(rows[1].tasks.map((TaskItem t) => t.id), <String>['dl-run'],
          reason: '下载是手机本机行为，断连也照常归执行分区');
    });

    test('全空清单 → 无分区（页面空态的唯一来源）', () {
      expect(taskPartitionRows(const <TaskItem>[], hostOffline: false), isEmpty);
    });

    test('分组只用当前页签的行：消息类与下载类互不串门', () {
      final List<TaskPartitionRow> rows = taskPartitionRows(
        <TaskItem>[task('dl-only', kind: TaskKind.download)],
        hostOffline: false,
      );
      expect(rows, hasLength(1));
      expect(rows.single.tasks.single.kind, TaskKind.download);
    });
  });

  group('纯函数：行渲染口径', () {
    test('taskStatusKindOf：RUNNING/DONE 分档；FAILED/CANCELED/INTERRUPTED 同档', () {
      expect(taskStatusKindOf(TaskStatus.running), PAStatusKind.running);
      expect(taskStatusKindOf(TaskStatus.done), PAStatusKind.done);
      expect(taskStatusKindOf(TaskStatus.failed), PAStatusKind.failed);
      expect(taskStatusKindOf(TaskStatus.canceled), PAStatusKind.failed);
      expect(taskStatusKindOf(TaskStatus.interrupted), PAStatusKind.failed);
    });

    test('taskStatusLabel 用四词（与分区头同源）', () {
      expect(taskStatusLabel(TaskStatus.running), '正在执行');
      expect(taskStatusLabel(TaskStatus.done), '已完成');
      expect(taskStatusLabel(TaskStatus.failed), '未完成');
      expect(taskStatusLabel(TaskStatus.canceled), '未完成');
      expect(taskStatusLabel(TaskStatus.interrupted), '未完成');
    });

    test('taskRowTimeLabel：MM-dd HH:mm（本地时区）；<=0 返回空串', () {
      expect(taskRowTimeLabel(0), isEmpty);
      expect(taskRowTimeLabel(-1), isEmpty);
      final DateTime t = DateTime(2026, 10, 5, 9, 7);
      expect(taskRowTimeLabel(t.millisecondsSinceEpoch), '10-05 09:07');
    });

    test('taskOfflineHint：只在「断连 + 消息类 + 运行中」给词', () {
      TaskItem mk(TaskKind kind, TaskStatus status) => TaskItem(
            id: 'x',
            threadId: 'th',
            kind: kind,
            title: 'T',
            status: status,
            startedAtMs: t0,
          );

      expect(taskOfflineHint(mk(TaskKind.message, TaskStatus.running),
          hostOffline: true), '离线 · 恢复连接后继续');
      expect(taskOfflineHint(mk(TaskKind.download, TaskStatus.running),
          hostOffline: true), isNull,
          reason: '下载是手机本机行为，不断连提示');
      expect(taskOfflineHint(mk(TaskKind.message, TaskStatus.done),
          hostOffline: true), isNull);
      expect(taskOfflineHint(mk(TaskKind.message, TaskStatus.running),
          hostOffline: false), isNull);
    });

    test('taskFileKey：下载类去 dl: 前缀；消息类恒空串', () {
      final TaskItem dl = TaskItem(
        id: 'dl:C:\\tmp\\a.zip',
        threadId: 'th',
        kind: TaskKind.download,
        title: 'a.zip',
        status: TaskStatus.running,
        startedAtMs: t0,
      );
      final TaskItem msg = TaskItem(
        id: 'msg:th',
        threadId: 'th',
        kind: TaskKind.message,
        title: 'T',
        status: TaskStatus.running,
        startedAtMs: t0,
      );
      expect(taskFileKey(dl), r'C:\tmp\a.zip');
      expect(taskFileKey(msg), isEmpty);
    });
  });

  group('纯函数：redownloadPlanOf / redownloadNeedsPc', () {
    test('URL 前缀 → 只要手机有网', () {
      expect(redownloadPlanOf('https://x.test/a.png'), RedownloadPlan.fromUrl);
      expect(redownloadPlanOf('http://x.test/a.png'), RedownloadPlan.fromUrl);
      expect(redownloadNeedsPc(RedownloadPlan.fromUrl), false);
    });

    test('Windows 盘符/UNC/POSIX 绝对路径 → 要电脑在线', () {
      expect(redownloadPlanOf(r'C:\tmp\a.zip'), RedownloadPlan.fromPc);
      expect(redownloadPlanOf(r'\\wsl$\home\a.zip'), RedownloadPlan.fromPc);
      expect(redownloadPlanOf('C:/tmp/a.zip'), RedownloadPlan.fromPc);
      expect(redownloadPlanOf('/home/u/a.zip'), RedownloadPlan.fromPc);
      expect(redownloadNeedsPc(RedownloadPlan.fromPc), true);
    });

    test('blob / 文本另存 → 无可重放来源（needsPc = null）', () {
      expect(redownloadPlanOf('blob:https://x.test/abc'), RedownloadPlan.none,
          reason: '页面 blob URL 没有可再取的源');
      expect(redownloadPlanOf(''), RedownloadPlan.none);
      expect(redownloadNeedsPc(RedownloadPlan.none), isNull);
    });
  });

  group('纯函数：taskNextStepCopy（与行尾真实按钮对齐）', () {
    test('消息类：有重试按钮才说「点右侧重试」，否则说「去会话找回」', () {
      expect(
        taskNextStepCopy(
          kind: TaskKind.message,
          hostOffline: false,
          redownloadNeedsPc: null,
          hasSourceThread: true,
          hasRetryAction: true,
        ),
        '点右侧重试，找回这次的结果',
      );
      expect(
        taskNextStepCopy(
          kind: TaskKind.message,
          hostOffline: false,
          redownloadNeedsPc: null,
          hasSourceThread: true,
          hasRetryAction: false,
        ),
        '去会话里找回这次的结果',
      );
      expect(
        taskNextStepCopy(
          kind: TaskKind.message,
          hostOffline: true,
          redownloadNeedsPc: null,
          hasSourceThread: true,
          hasRetryAction: true,
        ),
        '恢复连接后点重试，找回这次的结果',
      );
    });

    test('下载类无可重放来源：有会话 → 回会话；无会话 → 只能移除', () {
      String copy({required bool hasSourceThread}) => taskNextStepCopy(
            kind: TaskKind.download,
            hostOffline: false,
            redownloadNeedsPc: null,
            hasSourceThread: hasSourceThread,
            hasRetryAction: false,
          );
      expect(copy(hasSourceThread: true), '内容在会话里还在；可回会话重新保存，或移除这条记录');
      expect(copy(hasSourceThread: false), '这条记录没有可再取的文件，可移除');
    });

    test('下载类要电脑在线：断线时先说「现在取不了」', () {
      expect(
        taskNextStepCopy(
          kind: TaskKind.download,
          hostOffline: true,
          redownloadNeedsPc: true,
          hasSourceThread: false,
          hasRetryAction: true,
        ),
        '电脑连不上时取不了；恢复连接后点重新下载',
      );
      expect(
        taskNextStepCopy(
          kind: TaskKind.download,
          hostOffline: false,
          redownloadNeedsPc: true,
          hasSourceThread: false,
          hasRetryAction: true,
        ),
        '点右侧重新下载，从电脑再取一次',
      );
    });

    test('下载类只靠手机有网（URL）：断连也照常说「点右侧重新下载」', () {
      expect(
        taskNextStepCopy(
          kind: TaskKind.download,
          hostOffline: true,
          redownloadNeedsPc: false,
          hasSourceThread: false,
          hasRetryAction: true,
        ),
        '点右侧重新下载，再取一次',
      );
    });
  });

  group('TaskRetryGate：重试冷却门闩', () {
    test('冷却内第二次点击被吞；窗口外放行', () {
      final TaskRetryGate gate = TaskRetryGate(1500);

      expect(gate.tryBegin(1000), isTrue);
      expect(gate.tryBegin(2000), isFalse, reason: '1500ms 冷却内连点防护');
      expect(gate.tryBegin(2499), isFalse);
      expect(gate.tryBegin(2500), isTrue, reason: 'now-last >= cooldown 即放行');
      expect(gate.inFlight(2600), isTrue);
      expect(gate.inFlight(4000), isFalse);
    });

    test('首次点击（含 nowMs=0）必过', () {
      expect(TaskRetryGate(1500).tryBegin(0), isTrue);
    });

    test('standard() = Kotlin TASK_RETRY_COOLDOWN_MS', () {
      expect(TaskRetryGate.standard().cooldownMs, kTaskRetryCooldownMs);
      expect(kTaskRetryCooldownMs, 1500);
    });
  });
}
