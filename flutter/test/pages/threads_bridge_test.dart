// BridgeThreadsDataSource + 字段映射纯函数测试（B3-3）。
//
// 【两层各自的证词】
//   ① 映射纯函数（时间档位 / 状态优先级 / 摘要口径 / agent 兜底）不拖通道，
//      逐条钉死「行上的字从 wire 哪个字段来」的规则；
//   ② 数据源走真 PaBridge + mock 宿主 wire 行为：成功路径对账 python 现生成的
//      thread.b64 真字节；两条**必须分家**的失败路径——HOST_UNAVAILABLE → 人话
//      错误（点名 B4 波次）vs 空响应 → 真空列表——各钉一条（红线：不留假绿，
//      「通道没就绪」绝不许伪装成「还没有会话」）。

import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/threads_bridge.dart';
import 'package:pocketagent_ui/pages/threads_data.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  Uint8List fixture(String name) {
    final candidates = <String>[
      '${Directory.current.path}/tools/protofixture/fixtures/$name.b64',
      '${Directory.current.path}/flutter/tools/protofixture/fixtures/$name.b64',
    ];
    for (final c in candidates) {
      final f = File(c);
      if (f.existsSync()) {
        return base64.decode(f.readAsStringSync().trim());
      }
    }
    fail('找不到 fixture $name（试过：${candidates.join(', ')}；'
        'cwd=${Directory.current.path}）');
  }

  group('threadRowTimeLabel：行时间档位（口径 = Android threadRowTime）', () {
    // 固定 now：2026-10-14 14:30:00（本地时区）——所有档位断言都用这一根轴。
    final int now = DateTime(2026, 10, 14, 14, 30).millisecondsSinceEpoch;
    int at(int y, int mo, int d, int h, int mi) =>
        DateTime(y, mo, d, h, mi).millisecondsSinceEpoch;

    test('缺席（<=0）返回空串：不显示假时间', () {
      expect(threadRowTimeLabel(0, now), '');
      expect(threadRowTimeLabel(-5, now), '');
    });

    test('一分钟内 = 刚刚；一小时内 = N 分钟前', () {
      expect(threadRowTimeLabel(now - 30000, now), '刚刚');
      expect(threadRowTimeLabel(now - 60000, now), '1 分钟前');
      expect(threadRowTimeLabel(now - 59 * 60000, now), '59 分钟前');
    });

    test('今天超过一小时 = 今天 HH:mm；昨天 = 昨天 HH:mm', () {
      expect(threadRowTimeLabel(at(2026, 10, 14, 9, 5), now), '今天 09:05');
      expect(threadRowTimeLabel(at(2026, 10, 13, 23, 59), now), '昨天 23:59');
    });

    test('同一年更早 = MM-dd；跨年 = yyyy-MM-dd', () {
      expect(threadRowTimeLabel(at(2026, 10, 12, 8, 0), now), '10-12');
      expect(threadRowTimeLabel(at(2025, 12, 31, 8, 0), now), '2025-12-31');
    });

    test('客户端时钟偏慢（时间戳在未来）= 绝对日期档，不出现「负 N 分钟前」', () {
      expect(threadRowTimeLabel(at(2026, 10, 14, 16, 0), now), '今天 16:00');
      expect(threadRowTimeLabel(at(2026, 10, 20, 8, 0), now), '10-20');
    });
  });

  group('threadStatusKindOf：状态优先级（审批 > 运行 > 失败 > 完成）', () {
    PAStatusKind kind({
      required ThreadStatus status,
      int pending = 0,
      String err = '',
    }) =>
        threadStatusKindOf(
            status: status, pendingApprovals: pending, lastTurnError: err);

    test('挂审批（无论什么 status）= 等人（pending）——审批优先于运行', () {
      expect(
          kind(status: ThreadStatus.THREAD_STATUS_RUNNING, pending: 1),
          PAStatusKind.pending);
      expect(kind(status: ThreadStatus.THREAD_STATUS_WAITING_APPROVAL),
          PAStatusKind.pending);
    });

    test('RUNNING = running', () {
      expect(kind(status: ThreadStatus.THREAD_STATUS_RUNNING),
          PAStatusKind.running);
    });

    test('IDLE + last_turn_error = failed（中断就是失败，不美化）', () {
      expect(
          kind(status: ThreadStatus.THREAD_STATUS_IDLE, err: 'session/resume 失败'),
          PAStatusKind.failed);
      expect(
          kind(status: ThreadStatus.THREAD_STATUS_IDLE, err: '   '),
          PAStatusKind.done,
          reason: '纯空白错误 = 没有错误');
    });

    test('UNSPECIFIED（daemon 未上报/未知枚举）= failed 档，不冒充已完成', () {
      expect(kind(status: ThreadStatus.THREAD_STATUS_UNSPECIFIED),
          PAStatusKind.failed,
          reason: '「不知道」不许画成「成功」——与 Android 对 UNSPECIFIED 的保守档同源');
    });

    test('IDLE 无错 / ARCHIVED = done', () {
      expect(kind(status: ThreadStatus.THREAD_STATUS_IDLE), PAStatusKind.done);
      expect(kind(status: ThreadStatus.THREAD_STATUS_ARCHIVED),
          PAStatusKind.done);
    });
  });

  group('lastTurnErrorSummary / threadRowSummary：摘要口径', () {
    test('取第一个非空行，裁到上限加省略号（Android 同款 40 字）', () {
      expect(lastTurnErrorSummary('session/resume\n<id> 失败: 连接超时'),
          'session/resume');
      expect(lastTurnErrorSummary('  \n\n  真正的错  '), '真正的错');
      expect(lastTurnErrorSummary('   '), '');
      final String long = 'x' * 50;
      expect(lastTurnErrorSummary(long), '${'x' * 40}…');
      expect(lastTurnErrorSummary(long).length, 41);
    });

    test('失败档且错误非空 → 错误摘要；否则 N 条消息；都没有 → 空串', () {
      expect(
          threadRowSummary(PAStatusKind.failed, 'boom\n细节', 7), 'boom');
      expect(threadRowSummary(PAStatusKind.running, '', 7), '7 条消息');
      expect(threadRowSummary(PAStatusKind.running, '', 0), '');
      expect(threadRowSummary(PAStatusKind.failed, '  ', 0), '',
          reason: '失败但错误全空、消息数 0：不造占位文案');
    });
  });

  group('threadRowTitle / threadAgentIdOf：兜底', () {
    test('空标题 → (未命名)；非空标题原样透传', () {
      expect(threadRowTitle(''), '(未命名)');
      expect(threadRowTitle('修复登录页'), '修复登录页');
    });

    test('agent_id 空 → codex（proto/store 老记录兼容口径）；未知 id 不透明白名单', () {
      expect(threadAgentIdOf(''), 'codex');
      expect(threadAgentIdOf('pi'), 'pi');
      expect(threadAgentIdOf('brand-new-agent'), 'brand-new-agent');
    });
  });

  group('BridgeThreadsDataSource：真通道行为', () {
    test('成功：python 真字节 → 六字段落位（含时间标签与状态）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        expect(call.method, 'listThreads');
        // thread.b64：WAITING_APPROVAL + updated_at=1728901200000 + agent=codex
        // + message_count=17（与 wire_fixture_test 同一套哨兵值）。
        return ListThreadsResponse(
          threads: <Thread>[Thread.fromBuffer(fixture('thread'))],
        ).writeToBuffer();
      });
      // 固定时钟：让「时间档位」可断言（1728901200000 = 2024-10-14 13:40
      // UTC+8 前后；now 取它 + 2h → 今天 HH:mm 档）。
      final BridgeThreadsDataSource source = BridgeThreadsDataSource(
        nowMs: () => 1728901200000 + 2 * 3600 * 1000,
      );

      final List<ThreadSummary> rows = await source.load();

      expect(rows, hasLength(1));
      final ThreadSummary row = rows.single;
      expect(row.id, 'thread-42');
      expect(row.title, '修复登录页在低网速下的崩溃');
      expect(row.agentId, 'codex');
      expect(row.status, PAStatusKind.pending,
          reason: 'WAITING_APPROVAL → 等人（审批优先）');
      expect(row.summary, '17 条消息');
      expect(
        row.time,
        startsWith('今天 '),
        reason: 'updated_at 是响应内时间戳，与注入时钟差 2 小时 → 今天档',
      );
    });

    test('成功（空响应 0 字节）= 真空列表；与错误路径分家', () async {
      messenger.setMockMethodCallHandler(
        methodChannel,
        (MethodCall call) async => ListThreadsResponse().writeToBuffer(),
      );
      final BridgeThreadsDataSource source = BridgeThreadsDataSource(
        nowMs: () => 0,
      );

      expect(await source.load(), isEmpty);
    });

    test('HOST_UNAVAILABLE → ThreadsLoadFailure，文案点名 B4 波次（不是空列表）',
        () async {
      // 复刻生产：宿主桥未注册 → MissingPluginException → HOST_UNAVAILABLE。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('no host');
      });
      final BridgeThreadsDataSource source = BridgeThreadsDataSource();

      await expectLater(
        source.load(),
        throwsA(isA<ThreadsLoadFailure>().having(
            (ThreadsLoadFailure e) => e.message,
            'message',
            allOf(contains('未就绪'), contains('B4 波次')))),
        reason: '宿主没就绪必须走错误路径——返回空列表就是把错误粉饰成「没有会话」',
      );
    });

    test('宿主其他错误码（NOT_CONNECTED）→ 透传宿主 message，不改写', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '还没连上主机');
      });
      final BridgeThreadsDataSource source = BridgeThreadsDataSource();

      await expectLater(
        source.load(),
        throwsA(isA<ThreadsLoadFailure>().having(
            (ThreadsLoadFailure e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('宿主错误无 message → 退回 code（宁可难看也不吞）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(code: 'INTERNAL');
      });
      final BridgeThreadsDataSource source = BridgeThreadsDataSource();

      await expectLater(
        source.load(),
        throwsA(isA<ThreadsLoadFailure>()
            .having((ThreadsLoadFailure e) => e.message, 'message', 'INTERNAL')),
      );
    });

    test('DECODE_FAILED（坏字节）→ ThreadsLoadFailure 带定位信息', () async {
      messenger.setMockMethodCallHandler(methodChannel,
          (MethodCall call) async => Uint8List.fromList(<int>[0xFF, 0xFF, 0xFF]));
      final BridgeThreadsDataSource source = BridgeThreadsDataSource();

      await expectLater(
        source.load(),
        throwsA(isA<ThreadsLoadFailure>().having(
            (ThreadsLoadFailure e) => e.message,
            'message',
            contains('不可解析'))),
      );
    });
  });
}
