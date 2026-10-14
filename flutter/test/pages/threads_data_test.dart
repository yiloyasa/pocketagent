// Threads 数据层纯函数测试：三维筛选（agent × 状态 × 搜索）+ fake 数据源。
//
// 【为什么单独测纯函数】筛选是本页唯一的"业务逻辑"，它必须能在不拖 MaterialApp
// 的情况下被钉死——widget 测试负责三态与渲染，这里负责交集/大小写/顺序这些
// 一改就容易悄悄错的细节。两层都绿才算筛选没坏。

import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/pages/threads_data.dart';

void main() {
  const ThreadSummary t1 = ThreadSummary(
    id: 't1',
    title: '修复登录页 token 过期跳转',
    agentId: 'codex',
    summary: '已改完 refresh 拦截器，正在跑单测',
    time: '14:02',
    status: PAStatusKind.running,
  );
  const ThreadSummary t2 = ThreadSummary(
    id: 't2',
    title: '给 daemon 加配对码速率限制',
    agentId: 'pi',
    summary: '5 次失败后指数退避',
    time: '13:41',
    status: PAStatusKind.done,
  );
  const ThreadSummary t3 = ThreadSummary(
    id: 't3',
    title: '终端页 ANSI 色彩还原',
    agentId: 'codex',
    summary: 'OSC 256 色已支持',
    time: '10-12',
    status: PAStatusKind.done,
  );
  const List<ThreadSummary> all = <ThreadSummary>[t1, t2, t3];

  group('applyThreadFilter', () {
    test('无筛选档原样返回（含原列表顺序，不重排不拷贝语义）', () {
      final List<ThreadSummary> out =
          applyThreadFilter(all, ThreadFilter.none);
      expect(out, <ThreadSummary>[t1, t2, t3]);
      expect(ThreadFilter.none.isNone, isTrue);
    });

    test('agent 维度：只留该 agent 的行', () {
      final List<ThreadSummary> out =
          applyThreadFilter(all, const ThreadFilter(agentId: 'codex'));
      expect(out, <ThreadSummary>[t1, t3]);
    });

    test('agent 维度：数据里没有的 agentId 得空列表（不回退成"全部"）', () {
      expect(
        applyThreadFilter(all, const ThreadFilter(agentId: 'zcode')),
        isEmpty,
      );
    });

    test('状态维度：只留该状态的行', () {
      final List<ThreadSummary> out = applyThreadFilter(
        all,
        const ThreadFilter(status: PAStatusKind.done),
      );
      expect(out, <ThreadSummary>[t2, t3]);
    });

    test('搜索维度：标题命中', () {
      final List<ThreadSummary> out =
          applyThreadFilter(all, const ThreadFilter(query: '配对码'));
      expect(out, <ThreadSummary>[t2]);
    });

    test('搜索维度：摘要命中（摘要也属于"人记得的内容"）', () {
      final List<ThreadSummary> out =
          applyThreadFilter(all, const ThreadFilter(query: 'OSC'));
      expect(out, <ThreadSummary>[t3]);
    });

    test('搜索维度：大小写不敏感 + 前后空白裁掉', () {
      expect(
        applyThreadFilter(all, const ThreadFilter(query: '  token ')),
        <ThreadSummary>[t1],
      );
      expect(
        applyThreadFilter(all, const ThreadFilter(query: 'daemon')),
        <ThreadSummary>[t2],
      );
    });

    test('搜索不打 id：机器串不是人会输的关键词', () {
      expect(
        applyThreadFilter(all, const ThreadFilter(query: 't1')),
        isEmpty,
        reason: 'matchesThreadSearch 是找会话不是找主键',
      );
    });

    test('三维度 AND 交集：逐维收紧', () {
      final List<ThreadSummary> out = applyThreadFilter(
        all,
        const ThreadFilter(
          agentId: 'codex',
          status: PAStatusKind.done,
          query: 'ANSI',
        ),
      );
      expect(out, <ThreadSummary>[t3]);

      // 同一组合去掉搜索词 → 放宽到该 agent 的全部已完成
      expect(
        applyThreadFilter(
          all,
          const ThreadFilter(agentId: 'codex', status: PAStatusKind.done),
        ),
        <ThreadSummary>[t3],
      );
      // agent 维度冲突 → 交集为空
      expect(
        applyThreadFilter(
          all,
          const ThreadFilter(agentId: 'pi', status: PAStatusKind.running),
        ),
        isEmpty,
      );
    });
  });

  group('ThreadFilter.isNone', () {
    test('三维度全放行才算未筛选', () {
      expect(const ThreadFilter().isNone, isTrue);
      expect(const ThreadFilter(query: '   ').isNone, isTrue,
          reason: '纯空白等价于没搜');
      expect(const ThreadFilter(agentId: 'codex').isNone, isFalse);
      expect(const ThreadFilter(status: PAStatusKind.done).isNone, isFalse);
      expect(const ThreadFilter(query: 'x').isNone, isFalse);
    });
  });

  group('FakeThreadsDataSource', () {
    test('默认返回 B1 六条示例数据（顶栏文案标"静态示例"的前提）', () async {
      final List<ThreadSummary> got = await const FakeThreadsDataSource().load();
      expect(got, hasLength(6));
      expect(got, kSampleThreads);
      expect(got.first.agentId, 'codex');
    });

    test('failure 注入如实抛 ThreadsLoadFailure（错误态的数据来源）', () async {
      const FakeThreadsDataSource src =
          FakeThreadsDataSource(failure: 'daemon 不可达');
      await expectLater(
        src.load(),
        throwsA(isA<ThreadsLoadFailure>().having(
            (ThreadsLoadFailure e) => e.message, 'message', 'daemon 不可达')),
      );
    });
  });

  group('ThreadSummary', () {
    test('按值相等（列表去重/断言用，不靠 identical）', () {
      const ThreadSummary copy = ThreadSummary(
        id: 't1',
        title: '修复登录页 token 过期跳转',
        agentId: 'codex',
        summary: '已改完 refresh 拦截器，正在跑单测',
        time: '14:02',
        status: PAStatusKind.running,
      );
      expect(copy, t1);
      expect(copy.hashCode, t1.hashCode);
      expect(
        const ThreadSummary(
          id: 't1',
          title: '别的标题',
          agentId: 'codex',
          summary: 's',
          time: '14:02',
          status: PAStatusKind.running,
        ),
        isNot(t1),
      );
    });
  });
}
