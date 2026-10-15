// Threads 页骨架测试：ThreadSummary 渲染 + 加载/错误/空三态 + 筛选交互 + 真通道默认接线。
//
// 【三态怎么测】数据源是注入点（ThreadsDataSource），测试用可控 fake 把
// 「还在加载」「拉失败了」「空列表」三种时序摆到台面上——不是靠 sleep 碰运气，
// 加载态用 Completer 卡住，错误态用一次性失败的源，重试后要真变绿。
//
// 【默认接线怎么测】最后一组不注入 source（走生产默认 BridgeThreadsDataSource
// + 真 `dev.pocketagent/bridge` 通道），用 setMockMethodCallHandler 复刻宿主 wire
// 行为（模式同 settings 页测试）。**两条路径必须分家**：宿主未注册（B4 前）→
// 错误态「未就绪（B4 波次）」；宿主正常回空响应 → 空态「还没有会话」——各钉一条，
// 防「假空态」（红线：不留假绿）。
//
// 筛选的**逻辑**在 threads_data_test.dart 用纯函数钉死；这里只验筛选条的交互
// 真的驱动了列表（点 chip / 输搜索框 → 行集合变化 → 筛空有出口）。

import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_card.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/threads.dart';
import 'package:pocketagent_ui/pages/threads_data.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 卡住不返回的源：用来把「加载中」钉在屏幕上。
class _PendingSource implements ThreadsDataSource {
  final Completer<List<ThreadSummary>> completer =
      Completer<List<ThreadSummary>>();

  @override
  Future<List<ThreadSummary>> load() => completer.future;
}

/// 第一次必失败、之后成功：验证错误态的「重试」是真的重新拉，不是装饰。
class _RetrySource implements ThreadsDataSource {
  _RetrySource(this.threads);

  final List<ThreadSummary> threads;
  int calls = 0;

  @override
  Future<List<ThreadSummary>> load() async {
    calls += 1;
    if (calls == 1) {
      throw const ThreadsLoadFailure('daemon 不可达');
    }
    return threads;
  }
}

class _StaticSource implements ThreadsDataSource {
  const _StaticSource(this.threads);

  final List<ThreadSummary> threads;

  @override
  Future<List<ThreadSummary>> load() async => threads;
}

const List<ThreadSummary> demo = <ThreadSummary>[
  ThreadSummary(
    id: 't1',
    title: '修复登录页 token 过期跳转',
    agentId: 'codex',
    summary: '已改完 refresh 拦截器，正在跑单测',
    time: '14:02',
    status: PAStatusKind.running,
  ),
  ThreadSummary(
    id: 't2',
    title: '给 daemon 加配对码速率限制',
    agentId: 'pi',
    summary: '5 次失败后指数退避',
    time: '13:41',
    status: PAStatusKind.done,
  ),
  ThreadSummary(
    id: 't3',
    title: '终端页 ANSI 色彩还原',
    agentId: 'codex',
    summary: 'OSC 256 色已支持',
    time: '10-12',
    status: PAStatusKind.done,
  ),
];

void main() {
  Future<void> pumpThreads(
    WidgetTester tester, {
    ThreadsDataSource? source,
    ValueChanged<String>? onOpenThread,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: ThreadsPage(source: source, onOpenThread: onOpenThread),
        ),
      ),
    );
  }

  // 行点按出口：onOpenThread 收到该行 threadId（Chat 路由入口，main.dart 接线）。
  testWidgets('行点按 → onOpenThread(threadId)', (WidgetTester tester) async {
    final List<String> opened = <String>[];
    await pumpThreads(
      tester,
      source: FakeThreadsDataSource(threads: kSampleThreads),
      onOpenThread: opened.add,
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text(kSampleThreads.first.title));
    await tester.pumpAndSettle();
    expect(opened, <String>[kSampleThreads.first.id]);
  });

  group('三态', () {
    testWidgets('加载中 → 「正在加载会话…」，数据到了切换成列表', (tester) async {
      final _PendingSource src = _PendingSource();
      await pumpThreads(tester, source: src);
      await tester.pump();

      expect(find.text('正在加载会话…'), findsOneWidget);
      expect(find.byType(PAStatusChip), findsNothing);
      expect(find.text('修复登录页 token 过期跳转'), findsNothing);

      src.completer.complete(demo);
      await tester.pumpAndSettle();

      expect(find.text('正在加载会话…'), findsNothing);
      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
      expect(find.byType(PAStatusChip), findsNWidgets(3));
    });

    testWidgets('加载失败 → 错误态带原因；重试后真拉到列表', (tester) async {
      final _RetrySource src = _RetrySource(demo);
      await pumpThreads(tester, source: src);
      await tester.pumpAndSettle();

      expect(find.text('会话列表加载失败'), findsOneWidget);
      expect(find.textContaining('daemon 不可达'), findsOneWidget);
      expect(find.text('修复登录页 token 过期跳转'), findsNothing);

      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(src.calls, 2, reason: '重试必须真的重新调用数据源');
      expect(find.text('会话列表加载失败'), findsNothing);
      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
    });

    testWidgets('空列表 → 「还没有会话」（真真空态，不是通道错误的伪装）', (tester) async {
      await pumpThreads(tester, source: const _StaticSource(<ThreadSummary>[]));
      await tester.pumpAndSettle();

      expect(find.text('还没有会话'), findsOneWidget);
      expect(find.textContaining('B4 波次'), findsNothing,
          reason: '空态不许出现「未就绪」字样——那是错误路径的词');
      expect(find.byType(PAStatusChip), findsNothing);
    });
  });

  group('行渲染', () {
    testWidgets('ThreadSummary 六字段落位：头像 agentId / 标题 / 摘要 / 时间 / 状态 chip',
        (tester) async {
      await pumpThreads(tester, source: const _StaticSource(demo));
      await tester.pumpAndSettle();

      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
      expect(find.text('已改完 refresh 拦截器，正在跑单测'), findsOneWidget);
      expect(find.text('14:02'), findsOneWidget);
      // agentId 原样进头像（codex 两行 = 两次出现，不缩写不查表）；
      // 限定在 PACard 内，排除筛选条上同名的 agent chip。
      expect(
        find.descendant(of: find.byType(PACard), matching: find.text('codex')),
        findsNWidgets(2),
      );
      expect(
        find.descendant(of: find.byType(PACard), matching: find.text('pi')),
        findsOneWidget,
      );
      // 三行三个状态 chip（组件渲染与筛选共用同一映射）
      expect(find.byType(PAStatusChip), findsNWidgets(3));
      expect(find.text('运行中'), findsWidgets,
          reason: 'running 行 chip 与筛选条的「运行中」都在（同词表复用）');
    });
  });

  group('三维筛选交互', () {
    testWidgets('agent 维度：点 chip 只留该 agent 的行，再点取消', (tester) async {
      await pumpThreads(tester, source: const _StaticSource(demo));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey<String>('threads-agent-codex')));
      await tester.pumpAndSettle();

      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
      expect(find.text('终端页 ANSI 色彩还原'), findsOneWidget);
      expect(find.text('给 daemon 加配对码速率限制'), findsNothing);

      await tester.tap(find.byKey(const ValueKey<String>('threads-agent-codex')));
      await tester.pumpAndSettle();
      expect(find.text('给 daemon 加配对码速率限制'), findsOneWidget);
    });

    testWidgets('状态维度：点「已完成」只留 done 行', (tester) async {
      await pumpThreads(tester, source: const _StaticSource(demo));
      await tester.pumpAndSettle();

      await tester.tap(
        find.byKey(ValueKey<String>('threads-status-${PAStatusKind.done}')),
      );
      await tester.pumpAndSettle();

      expect(find.text('给 daemon 加配对码速率限制'), findsOneWidget);
      expect(find.text('终端页 ANSI 色彩还原'), findsOneWidget);
      expect(find.text('修复登录页 token 过期跳转'), findsNothing);
      expect(find.byType(PAStatusChip), findsNWidgets(2));
    });

    testWidgets('搜索维度：输关键词实时过滤；筛空给「清除筛选」出口', (tester) async {
      await pumpThreads(tester, source: const _StaticSource(demo));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.byKey(const ValueKey<String>('threads-search')),
        'ANSI',
      );
      await tester.pumpAndSettle();
      expect(find.text('终端页 ANSI 色彩还原'), findsOneWidget);
      expect(find.text('修复登录页 token 过期跳转'), findsNothing);

      // 搜一个匹配不上的 → 被筛空（区别于「本来就没有」的空态）
      await tester.enterText(
        find.byKey(const ValueKey<String>('threads-search')),
        'zzz',
      );
      await tester.pumpAndSettle();
      expect(find.text('没有匹配的会话'), findsOneWidget);
      expect(find.text('还没有会话'), findsNothing);
      expect(find.textContaining('搜索=zzz'), findsOneWidget,
          reason: '筛空要告诉用户是被哪一维筛没的');

      await tester.tap(find.text('清除筛选'));
      await tester.pumpAndSettle();
      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
      expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey<String>('threads-search')))
            .controller!
            .text,
        isEmpty,
        reason: '清除筛选必须连搜索框一起复位，否则界面看着还在搜',
      );
    });
  });

  group('默认接线：真通道（不注入 source）', () {
    const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
    late TestDefaultBinaryMessenger messenger;

    setUp(() {
      messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    });

    /// 默认数据源 = BridgeThreadsDataSource（真 `dev.pocketagent/bridge`）：
    /// 这些用例不传 source，验的就是生产构造出的那条链路。
    Uint8List threadsBytes(List<Thread> threads) =>
        ListThreadsResponse(threads: threads).writeToBuffer();

    Thread protoThread({
      required String id,
      required String title,
      String agentId = 'codex',
      ThreadStatus status = ThreadStatus.THREAD_STATUS_IDLE,
      int messageCount = 0,
    }) =>
        Thread(
          id: id,
          title: title,
          agentId: agentId,
          status: status,
          messageCount: messageCount,
          updatedAtUnixMs: Int64(DateTime.now().millisecondsSinceEpoch),
        );

    testWidgets('宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」，绝不是空态',
        (tester) async {
      // 复刻生产：宿主未注册 handler → MissingPluginException → HOST_UNAVAILABLE。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });

      await pumpThreads(tester);
      await tester.pumpAndSettle();

      expect(find.text('会话列表加载失败'), findsOneWidget);
      expect(find.textContaining('未就绪（B4 波次）'), findsOneWidget);
      expect(find.text('还没有会话'), findsNothing,
          reason: '通道没就绪不许走空态——那是「真没有会话」的伪装版（红线：不留假绿）');
      expect(find.text('重试'), findsOneWidget, reason: '错误态给重试出口');

      // 重试必须真的重新发真调用：把宿主「装上」后重试应转绿。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return threadsBytes(<Thread>[
          protoThread(id: 't1', title: '修复登录页 token 过期跳转'),
        ]);
      });
      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(find.text('会话列表加载失败'), findsNothing);
      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
    });

    testWidgets('宿主正常回空响应 → 空态「还没有会话」（与错误态分家的另一条）',
        (tester) async {
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => ListThreadsResponse().writeToBuffer(),
      );

      await pumpThreads(tester);
      await tester.pumpAndSettle();

      expect(find.text('还没有会话'), findsOneWidget);
      expect(find.text('会话列表加载失败'), findsNothing);
      expect(find.textContaining('未就绪'), findsNothing,
          reason: '空态不许出现错误路径的词——两条路径断言必须互相排斥');
      expect(find.byType(PAStatusChip), findsNothing);
    });

    testWidgets('宿主正常回列表 → 行渲染（标题/agent 头像/消息数摘要/状态）',
        (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return threadsBytes(<Thread>[
          protoThread(
            id: 't1',
            title: '修复登录页 token 过期跳转',
            agentId: 'pi',
            status: ThreadStatus.THREAD_STATUS_RUNNING,
            messageCount: 12,
          ),
        ]);
      });

      await pumpThreads(tester);
      await tester.pumpAndSettle();

      expect(find.text('修复登录页 token 过期跳转'), findsOneWidget);
      expect(find.text('12 条消息'), findsOneWidget);
      expect(
        find.descendant(of: find.byType(PACard), matching: find.text('pi')),
        findsOneWidget,
        reason: 'agent_id 原样进头像',
      );
      expect(find.text('运行中'), findsWidgets);
    });
  });
}
