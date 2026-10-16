// Settings ③downloads 子步测试 —— mock 的仍是「宿主桥的 wire 行为」，
// PaBridge 全程真实。
//
// 【验收三件事】① 分区渲染：下载分区走本地 TaskCenterStore 真值（条数 + 合计
// 字节），缓存/语音模型/其他如实标「未统计」（不用 0 冒充没占用）；② 三态分家：
// 宿主运行时块 HOST_UNAVAILABLE → 错误态带重试，读成功但字段空 → 「未报告」，
// 本地清单空 → 空态，三者互不冒充；③ 只读纪律：全页没有清理/删除入口，
// 每个分区行只有「清理留后续波次」的诚实文案，跨端调用只有 getRuntimeState。
//
// 【为什么本地清单用 store 公开方法面播种】store 的写入面就是下载分区的数据
// 来源本身，测试经同一路径写（Kotlin 侧读 TaskCenter.tasks 同构），不页面私有
// 假数据源。

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/runtime.pb.dart';
import 'package:pocketagent_ui/pages/settings_downloads.dart';
import 'package:pocketagent_ui/pages/task_center_data.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kPaBridgeEventChannel);

  late TestDefaultBinaryMessenger messenger;
  late List<MethodCall> calls;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    calls = <MethodCall>[];
    // 本页不订阅事件面；仍注册空实现，防 listen 握手错进框架日志。
    messenger.setMockStreamHandler(
      eventChannel,
      MockStreamHandler.inline(
        onListen: (Object? args, MockStreamHandlerEventSink sink) {},
      ),
    );
  });

  void mockMethods(FutureOr<Object?> Function(MethodCall call) onMethod) {
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      calls.add(call);
      return onMethod(call);
    });
  }

  /// `RuntimeStateResponse` 的 wire 字节（宿主侧应答形态）。
  Uint8List runtimeStateBytes({
    String daemonVersion = '0.4.0',
    String runtimeState = 'ready',
    String hostId = 'host-1',
    String dataDir = r'C:\Users\x\AppData\Local\pocketagentd',
  }) =>
      RuntimeStateResponse(
        daemonVersion: daemonVersion,
        runtimeState: runtimeState,
        hostId: hostId,
        dataDir: dataDir,
      ).writeToBuffer();

  /// 经 store 公开写入面播种下载条目（与生产同构的一条路径）。
  /// 【为什么按 fileKey 播种】`trackDownload` 的入参是 fileKey（内部拼
  /// `dl:` 前缀），与 `finishDownload` 的 id 入参不是同一个东西 —— 照 id 传会
  /// 生成 `dl:dl:xxx`，一条都落不进清单（这条坑由第一个用例当场踩到）。
  TaskCenterStore storeOf(List<TaskItem> tasks) {
    final TaskCenterStore store = TaskCenterStore();
    for (final TaskItem t in tasks) {
      final String fileKey = t.id.substring(3);
      store.trackDownload(fileKey, t.title, threadId: t.threadId,
          nowMs: 1728901200000);
      if (t.status == TaskStatus.done) {
        store.finishDownload(downloadTaskId(fileKey), ok: true,
            sizeText: t.sizeText, localUri: t.localUri);
      }
      if (t.sourceTitle.isNotEmpty) {
        store.annotateDownload(downloadTaskId(fileKey), t.sourceTitle);
      }
    }
    return store;
  }

  Future<void> pumpDownloadsPage(
    WidgetTester tester, {
    TaskCenterStore? store,
    VoidCallback? onBack,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: SettingsDownloadsPage(
            store: store,
            bridge: PaBridge(),
            onBack: onBack ?? () {},
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('纯函数：下载汇总与分区', () {
    test('parseHumanSize：四档单位 + 无单位回落 B + 空/不可解析 → null', () {
      expect(parseHumanSize('1.2MB'), 1258291);
      expect(parseHumanSize('340KB'), 348160);
      expect(parseHumanSize('512B'), 512);
      expect(parseHumanSize('2 GB'), 2147483648);
      expect(parseHumanSize('900'), 900, reason: '无单位按 B（Kotlin 同款兜底）');
      expect(parseHumanSize(''), isNull);
      expect(parseHumanSize('未知大小'), isNull, reason: '解析不出 = 未知，不是 0');
    });

    test('downloadsTotalOf：只数已完成；有条目体积未知 → unknown=true', () {
      final List<TaskItem> tasks = <TaskItem>[
        TaskItem(
          id: 'dl:a.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'a.zip',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '1MB',
        ),
        TaskItem(
          id: 'dl:b.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'b.zip',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '未知大小',
        ),
        // 未完成的不占最终体积。
        TaskItem(
          id: 'dl:c.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'c.zip',
          status: TaskStatus.running,
          startedAtMs: 0,
          sizeText: '9MB',
        ),
        // 消息类任务不进来。
        TaskItem(
          id: 'msg:th-1',
          threadId: 'th-1',
          kind: TaskKind.message,
          title: '消息',
          status: TaskStatus.done,
          startedAtMs: 0,
        ),
      ];
      final DownloadsTotal total = downloadsTotalOf(tasks);
      expect(total.count, 2);
      expect(total.bytes, 1 << 20, reason: '未知条目不计入合计（不猜 0）');
      expect(total.unknown, isTrue);
      expect(total.label, '2 个文件 · 占用无法获取',
          reason: '有条目体积未知时如实说无法获取，不给偏小的数字');
    });

    test('downloadsTotalOf：全部已知 → 合计文案带人类可读体积', () {
      final TaskCenterStore store = storeOf(<TaskItem>[
        TaskItem(
          id: 'dl:a.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'a.zip',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '2MB',
        ),
        TaskItem(
          id: 'dl:b.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'b.zip',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '1MB',
        ),
      ]);
      final DownloadsTotal total = downloadsTotalOf(store.snapshot);
      expect(total.count, 2);
      expect(total.unknown, isFalse);
      expect(total.label, '2 个文件 · 共 3 MB');
    });

    test('storagePartitionsOf：下载分区取本地真值，其余三项 bytes=null', () {
      final List<StoragePartition> parts =
          storagePartitionsOf(storeOf(<TaskItem>[
        TaskItem(
          id: 'dl:a.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'a.zip',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '1MB',
        ),
      ]).snapshot);
      expect(parts.map((StoragePartition p) => p.id).toList(),
          <String>['downloads', 'cache', 'voice-model', 'other']);
      expect(parts[0].bytes, 1 << 20);
      expect(parts[1].bytes, isNull, reason: '未统计 ≠ 0');
      expect(parts[2].bytes, isNull);
      expect(parts[3].bytes, isNull);
    });

    test('storagePartitionsOf：零条目 → 下载分区也标未统计（0 B 是假数字）', () {
      final List<StoragePartition> parts =
          storagePartitionsOf(const <TaskItem>[]);
      expect(parts[0].bytes, isNull,
          reason: '「0 个文件」不是「占了 0 字节」：前者该说没内容，后者是查过了');
      expect(parts[0].note, contains('还没有已完成的下载'));
    });

    test('downloadsPageErrorText：HOST_UNAVAILABLE 点名 B4，其余委托通用映射', () {
      expect(
        downloadsPageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.hostUnavailable, 'x'),
        ),
        contains('桌面端桥接未就绪（B4 波次）'),
      );
      expect(
        downloadsPageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.notConnected, 'x'),
        ),
        contains('尚未连上 daemon'),
      );
    });
  });

  group('分区渲染：下载分区真值 + 未统计如实标注', () {
    testWidgets('下载条目进分区汇总与列表；其余分区标「未统计」', (tester) async {
      mockMethods((MethodCall c) => runtimeStateBytes());
      final TaskCenterStore store = storeOf(<TaskItem>[
        TaskItem(
          id: 'dl:report.pdf',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'report.pdf',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '2MB',
          sourceTitle: '修复登录页崩溃',
        ),
        TaskItem(
          id: 'dl:notes.txt',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'notes.txt',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '512B',
        ),
      ]);

      await pumpDownloadsPage(tester, store: store);

      // 分区行：下载分区带真实合计，其余三项未统计。
      expect(find.text('下载内容'), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-storage-size-downloads')))
            .data,
        '2.0 MB',
        reason: '下载分区合计 = 2MB + 512B 的 sizeText 解析值（一位小数口径同 Files 页）',
      );
      expect(find.text('缓存'), findsOneWidget);
      expect(find.text('语音模型'), findsOneWidget);
      expect(find.text('其他'), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-storage-size-cache')))
            .data,
        '未统计',
        reason: '本地统计面未接入：如实标未统计，不用 0 冒充「没占用」',
      );
      // 每个分区都有清理的诚实文案，且**不是**可点按钮。
      for (final String id in <String>[
        'downloads',
        'cache',
        'voice-model',
        'other',
      ]) {
        expect(
          tester
              .widget<Text>(
                  find.byKey(ValueKey<String>('pa-storage-cleanup-$id')))
              .data,
          contains('清理留后续波次'),
        );
      }
      expect(find.byType(TextButton), findsNothing,
          reason: '本页没有任何按钮：清理/删除都是写操作，不摆假入口');

      // 下载内容分组：标题带汇总（0 个不摆 0）。
      expect(find.text('下载内容 · 2 个文件 · 共 2.0 MB'), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-download-row-dl:report.pdf')),
          findsOneWidget);
      expect(find.byKey(const ValueKey('pa-download-row-dl:notes.txt')),
          findsOneWidget);
      expect(find.textContaining('来自「修复登录页崩溃」'), findsOneWidget);
      expect(find.textContaining('已完成'), findsWidgets);
    });

    testWidgets('下载条目体积未知 → 汇总「占用无法获取」，分区大小同口径',
        (tester) async {
      mockMethods((MethodCall c) => runtimeStateBytes());
      final TaskCenterStore store = storeOf(<TaskItem>[
        TaskItem(
          id: 'dl:a.zip',
          threadId: 'th-1',
          kind: TaskKind.download,
          title: 'a.zip',
          status: TaskStatus.done,
          startedAtMs: 0,
          sizeText: '',
        ),
      ]);

      await pumpDownloadsPage(tester, store: store);

      expect(find.text('下载内容 · 1 个文件 · 占用无法获取'), findsOneWidget);
      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey('pa-storage-size-downloads')))
            .data,
        '未统计',
        reason: '分区大小与汇总同一口径：合计不可信时分区也标未统计',
      );
    });

    testWidgets('本地清单空 → 空态（不是错误态，也不是假列表）', (tester) async {
      mockMethods((MethodCall c) => runtimeStateBytes());

      await pumpDownloadsPage(tester, store: TaskCenterStore());

      // 分区行：下载分区 0 个 → 不摆 0，说明文字说清「清单里还没有」。
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-storage-size-downloads')))
            .data,
        '未统计',
        reason: '没有条目时合计不可算：标未统计而不是 0 B',
      );
      // 下载内容分组标题：0 个不摆 0，说「暂无下载内容」。
      expect(find.text('下载内容 · 暂无下载内容'), findsOneWidget);
      // 下载内容分组：空态文案（Kotlin DownloadsPage 同款）。
      expect(find.byKey(const ValueKey('pa-downloads-empty')), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-downloads-host-error')), findsNothing,
          reason: '空态与宿主错误态是两条路径');
      expect(find.byKey(const ValueKey('pa-download-row-dl:x')), findsNothing);
    });
  });

  group('电脑端运行时块：三态分家', () {
    testWidgets('读成功 → daemon 版本 / 运行时 / 数据目录逐项呈现', (tester) async {
      mockMethods((MethodCall c) => runtimeStateBytes());

      await pumpDownloadsPage(tester, store: TaskCenterStore());

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-downloads-host-line')))
            .data,
        contains('daemon 0.4.0'),
      );
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-downloads-host-datadir')))
            .data,
        contains('pocketagentd'),
      );
      expect(find.byKey(const ValueKey('pa-downloads-host-error')), findsNothing);
      // 只读：全页只发 getRuntimeState 一次。
      expect(calls.map((MethodCall c) => c.method).toList(), <String>[
        'getRuntimeState'
      ], reason: '本地块不走通道；宿主块只读一次');
    });

    testWidgets('读成功但 data_dir 空 → 如实「未报告」，不摆路径占位', (tester) async {
      mockMethods((MethodCall c) => runtimeStateBytes(dataDir: ''));

      await pumpDownloadsPage(tester, store: TaskCenterStore());

      expect(find.byKey(const ValueKey('pa-downloads-host-empty')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-downloads-host-empty')))
            .data,
        contains('未报告数据目录'),
      );
    });

    testWidgets('宿主缺席 → HOST_UNAVAILABLE 错误态 + 重试恢复', (tester) async {
      bool fail = true;
      mockMethods((MethodCall c) async {
        if (fail) {
          throw MissingPluginException(
              'No implementation found for method getRuntimeState');
        }
        return runtimeStateBytes();
      });

      await pumpDownloadsPage(tester, store: TaskCenterStore());

      expect(
        find.textContaining('桌面端桥接未就绪（B4 波次）'),
        findsOneWidget,
        reason: '占位文案点名是哪一层没就绪，不含糊成「读取失败」',
      );
      expect(find.byKey(const ValueKey('pa-downloads-host-error')), findsOneWidget);
      // 宿主块失败不拖垮本地块：分区与空态照常渲染。
      expect(find.text('下载内容'), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-downloads-empty')), findsOneWidget);

      fail = false;
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('pa-downloads-host-retry')),
        120,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pa-downloads-host-retry')));
      await tester.pumpAndSettle();

      expect(find.textContaining('桌面端桥接未就绪'), findsNothing);
      expect(find.byKey(const ValueKey('pa-downloads-host-datadir')),
          findsOneWidget);
    });

    testWidgets('宿主回 NOT_CONNECTED → 文案透传宿主 code 语义', (tester) async {
      mockMethods((MethodCall c) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '还没连上');
      });

      await pumpDownloadsPage(tester, store: TaskCenterStore());

      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey('pa-downloads-host-error')))
            .data,
        contains('尚未连上 daemon'),
      );
    });
  });

  group('路由出口', () {
    testWidgets('返回键回一级页（onBack 被调用）', (tester) async {
      mockMethods((MethodCall c) => runtimeStateBytes());
      bool backed = false;

      await pumpDownloadsPage(
        tester,
        store: TaskCenterStore(),
        onBack: () => backed = true,
      );

      await tester.tap(find.byKey(const ValueKey('pa-downloads-back')));
      await tester.pumpAndSettle();
      expect(backed, isTrue);
    });
  });
}
