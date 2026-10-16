// runtime 域 5 条只读方法面测试（B5 Settings ③追补）—— mock 的仍是「宿主桥的
// wire 行为」，PaBridge 全程真实。模式与 pa_bridge_collab_test.dart /
// pa_bridge_files_git_test.dart 完全一致（同一套 mock handler + 同一套错误映射
// 断言），只是文件按方法面拆分。
//
// 【字节来源】响应字节一律由 Dart gen stub 现组（`XxxResponse(...)
// .writeToBuffer()`），等价 Kotlin 宿主 `XxxResponse.newBuilder()...` 的写法；
// 每条逐字段摆真值（string 状态口径、int64 revision、repeated 嵌套形态卡与
// 透析表），通道层对账的就是这些字段。
//
// 【为什么没有写操作用例】setRuntimePolicy / setCollabPolicy /
// acknowledgeUnknownEvent / executeUnknownEventAction 本波**不接**（甲级写 +
// 完整策略 CAS 语义）——不测一个不存在的方法，也不假装它存在（红线：不留假绿）。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/runtime.pb.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;
  late PaBridge bridge;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    bridge = PaBridge();
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  /// 一份运行时形态申报（daemon 侧真实字段形状：六个 model 字段摊平）。
  RuntimeContractReport contractReport({
    String agentId = 'codex',
    bool declared = true,
  }) =>
      RuntimeContractReport(
        agentId: agentId,
        displayName: 'Codex',
        version: '0.160.0',
        declared: declared,
        capabilities: <String>['turn', 'approval'],
        issues: <RuntimeContractIssue>[
          RuntimeContractIssue(code: 'SESSION_MODEL_MISSING', detail: '未申报会话形态'),
        ],
        limitations: <RuntimeLimitation>[
          RuntimeLimitation(capability: 'subagent', reason: '暂不支持子代理'),
        ],
        sessionModel: 'NATIVE',
        resumeModel: 'reattach',
        approvalModel: 'daemon',
        subagentModel: 'delegate',
        memoryModel: 'hybrid',
        cacheModel: 'daemon',
      );

  group('getRuntimeState：空请求字节 + 响应整屏字段解码', () {
    test('默认（无参）= 空 GetRuntimeStateRequest 字节；逐字段对账', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return RuntimeStateResponse(
          installationId: 'inst-1',
          hostId: 'host-1',
          daemonVersion: '0.4.0',
          budgetPolicyRevision: Int64(3),
          collabPolicyRevision: Int64(1),
          runtimeState: 'degraded',
          sampledAtUnixMs: Int64(1728901200000),
          contracts: <RuntimeContractReport>[
            contractReport(),
            contractReport(agentId: 'pi', declared: false),
          ],
          dataDir: r'C:\Users\x\AppData\Local\pocketagentd',
        ).writeToBuffer();
      });

      final RuntimeStateResponse state = await bridge.getRuntimeState();

      expect(seen!.method, 'getRuntimeState');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空消息请求 = 空字节数组（编码总则 §2.1，同 getHostInfo）');
      expect(state.installationId, 'inst-1');
      expect(state.daemonVersion, '0.4.0');
      expect(state.budgetPolicyRevision.toInt(), 3);
      expect(state.collabPolicyRevision.toInt(), 1);
      expect(state.runtimeState, 'degraded',
          reason: 'runtime_state 是 string 口径（ready/degraded），不透改成 enum');
      expect(state.sampledAtUnixMs.toInt(), 1728901200000);
      expect(state.dataDir, isNotEmpty);
      // contracts[] 嵌套逐条解：六个 model 字段 + issues/limitations 都在。
      expect(state.contracts, hasLength(2));
      final RuntimeContractReport c = state.contracts.first;
      expect(c.agentId, 'codex');
      expect(c.displayName, 'Codex');
      expect(c.declared, isTrue);
      expect(c.sessionModel, 'NATIVE');
      expect(c.resumeModel, 'reattach');
      expect(c.approvalModel, 'daemon');
      expect(c.subagentModel, 'delegate');
      expect(c.memoryModel, 'hybrid');
      expect(c.cacheModel, 'daemon');
      expect(c.issues.single.code, 'SESSION_MODEL_MISSING');
      expect(c.limitations.single.reason, '暂不支持子代理');
      expect(state.contracts[1].declared, isFalse,
          reason: '连形态都没申报 = declared=false（与「申报了但有问题」分开）');
    });

    test('旧 daemon 空响应字节（0 字节）= 合法空总览，不是错误', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              RuntimeStateResponse().writeToBuffer());

      final RuntimeStateResponse state = await bridge.getRuntimeState();

      // 空字段 = 这一项没报（页面据此说「未报告」），不是读失败。
      expect(state.daemonVersion, isEmpty);
      expect(state.runtimeState, isEmpty);
      expect(state.contracts, isEmpty);
      expect(state.dataDir, isEmpty);
    });
  });

  group('getRuntimePolicy：空请求字节 + policy/effective 分离解码', () {
    test('空请求 = 空字节数组；effective_global 逐项按 optional 语义解', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return RuntimePolicyResponse(
          revision: Int64(7),
          policyPath: r'C:\Users\x\.pocketagentd\runtime-policy.json',
          effectiveGlobal: RuntimeLimits(
            ramWorkingBytes: Int64(512 * 1024 * 1024),
            ramEventBufferBytes: Int64(64 * 1024 * 1024),
            diskCacheBytes: Int64(2 * 1024 * 1024 * 1024),
            maxParallelTurns: 3,
          ),
        ).writeToBuffer();
      });

      final RuntimePolicyResponse policy = await bridge.getRuntimePolicy();

      expect(seen!.method, 'getRuntimePolicy');
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空消息请求 = 空字节数组（§2.1，同 getCollabPolicy）');
      expect(policy.revision.toInt(), 7);
      expect(policy.policyPath, isNotEmpty);
      final RuntimeLimits limits = policy.effectiveGlobal;
      expect(limits.hasRamWorkingBytes(), isTrue);
      expect(limits.ramWorkingBytes.toInt(), 512 * 1024 * 1024);
      expect(limits.hasDiskCacheBytes(), isTrue);
      expect(limits.hasMaxParallelTurns(), isTrue);
      expect(limits.maxParallelTurns, 3);
      expect(limits.hasDiskConversationBytes(), isFalse,
          reason: 'optional 缺席 = 未设置，与「显式设成 0」是两件事'
              '（页面据此不渲染该项，不把 0 显示成「封禁」）');
    });

    test('effective 全缺席（旧 daemon）= 合法空额度，不是错误', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              RuntimePolicyResponse().writeToBuffer());

      final RuntimePolicyResponse policy = await bridge.getRuntimePolicy();

      expect(policy.hasEffectiveGlobal(), isFalse);
      expect(policy.revision.toInt(), 0, reason: '0 = 从未落盘（仍在内置基线）');
    });
  });

  group('getControlPlane：空请求字节 + 域列表解码', () {
    test('空请求 = 空字节数组；domains[] 按域逐条解（人话全由 daemon 给）',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ControlPlane(
          daemonVersion: '0.4.0',
          hostName: 'YANZI-PC',
          os: 'windows',
          uptimeUnixMs: Int64(1728901200000),
          dataDirLabel: 'LOCALAPPDATA',
          domains: <ControlDomain>[
            ControlDomain(
              id: 'budget',
              title: '预算',
              summary: '全局并发 3',
              writable: true,
              statusCode: 0,
              items: <ControlItem>[
                ControlItem(
                  id: 'budget.global.ram_event_buffer_bytes',
                  label: '事件缓冲',
                  current: '64 MB',
                  kind: 'number',
                  hint: '调大更抗断连重连',
                ),
              ],
            ),
            ControlDomain(
              id: 'resources',
              title: '资源',
              summary: '未检测到活跃租约',
              writable: false,
              statusCode: 2,
            ),
          ],
        ).writeToBuffer();
      });

      final ControlPlane plane = await bridge.getControlPlane();

      expect(seen!.method, 'getControlPlane');
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空消息请求 = 空字节数组（§2.1）');
      expect(plane.daemonVersion, '0.4.0');
      expect(plane.hostName, 'YANZI-PC');
      expect(plane.os, 'windows');
      expect(plane.uptimeUnixMs.toInt(), 1728901200000);
      // 只给标签不给绝对路径（proto 明示用户名属敏感信息）。
      expect(plane.dataDirLabel, 'LOCALAPPDATA');
      expect(plane.domains, hasLength(2));
      final ControlDomain budget = plane.domains.first;
      expect(budget.id, 'budget', reason: '客户端按 id 匹配，不按 title');
      expect(budget.title, '预算');
      expect(budget.writable, isTrue);
      expect(budget.statusCode, 0);
      expect(budget.items.single.current, '64 MB');
      expect(budget.items.single.kind, 'number');
      expect(budget.items.single.hint, isNotEmpty);
      expect(plane.domains[1].statusCode, 2,
          reason: '2 可能只是「未装配/取不到」：如实映射，客户端不自行渲染成故障');
    });
  });

  group('listUnknownEvents / getUnknownEvent：台账列表与详情', () {
    test('默认（无参）= 空请求字节；items/total/dropped 逐字段对账', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListUnknownEventsResponse(
          items: <UnknownEventSummary>[
            UnknownEventSummary(
              eventId: 'ue-1',
              sequence: Int64(42),
              receivedAtUnixMs: Int64(1728901200000),
              sourceAgent: 'codex',
              sourceMethod: 'tools/call',
              threadId: 'th-1',
              normalizedKind: 'UNKNOWN_REQUEST',
              safetyClass: 'UNKNOWN',
              processingStatus: 'NEW',
              revision: Int64(0),
              summary: 'agent 调用了未注册的工具',
              plainTitle: '一个不认识的工具调用',
              requiresUser: true,
              canOpenNative: true,
            ),
          ],
          total: Int64(5),
          dropped: Int64(2),
        ).writeToBuffer();
      });

      final ListUnknownEventsResponse response =
          await bridge.listUnknownEvents();

      expect(seen!.method, 'listUnknownEvents');
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 全部（五筛选项全缺省）');
      final UnknownEventSummary s = response.items.single;
      expect(s.eventId, 'ue-1');
      expect(s.sequence.toInt(), 42);
      expect(s.sourceMethod, 'tools/call', reason: '原生方法名原文不翻译');
      expect(s.normalizedKind, 'UNKNOWN_REQUEST');
      expect(s.safetyClass, 'UNKNOWN', reason: '分类规则未定：如实标注 UNKNOWN');
      expect(s.processingStatus, 'NEW');
      expect(s.summary, isNotEmpty);
      expect(s.plainTitle, isNotEmpty);
      expect(s.requiresUser, isTrue);
      expect(s.canOpenNative, isTrue);
      // total/dropped 必须过桥：只回最近 N 条而不说，用户会以为「就这么几条」。
      expect(response.total.toInt(), 5);
      expect(response.dropped.toInt(), 2);
    });

    test('带筛选参数：kind/agent_id/limit 原样过桥，宿主可按 proto 回读',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListUnknownEventsResponse().writeToBuffer();
      });

      await bridge.listUnknownEvents(ListUnknownEventsRequest(
        kind: 'UNKNOWN_REQUEST',
        agentId: 'codex',
        limit: 20,
      ));

      final ListUnknownEventsRequest wire =
          ListUnknownEventsRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.kind, 'UNKNOWN_REQUEST');
      expect(wire.agentId, 'codex');
      expect(wire.limit, 20,
          reason: '方法面保持「请求消息原样编码」，不写死任何筛选默认值');
    });

    test('空台账 = 合法空列表，不是错误（与 HOST_UNAVAILABLE 分家）', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              ListUnknownEventsResponse().writeToBuffer());

      final ListUnknownEventsResponse response =
          await bridge.listUnknownEvents();

      expect(response.items, isEmpty);
      expect(response.total.toInt(), 0);
    });

    test('getUnknownEvent：event_id 过桥；原文/透析表/人话解读整包解', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return UnknownEventResponse(
          summary: UnknownEventSummary(eventId: 'ue-9', sourceMethod: 'tools/call'),
          rawJson: '{"method":"tools/call","params":{"x":1}}',
          decodedFieldsJson: '{"method":"tools/call"}',
          note: '',
          digest: <UnknownEventField>[
            UnknownEventField(
              key: 'method',
              kind: 'text',
              preview: 'tools/call',
              meaning: '方法名',
            ),
          ],
          translation: 'agent 发来了一个我们不认识的操作。',
          recommendedNextStep: '可在电脑端查看原文。',
          supportBundleHint: '导出支持包时会一并带上。',
        ).writeToBuffer();
      });

      final UnknownEventResponse detail = await bridge.getUnknownEvent(
        GetUnknownEventRequest(eventId: 'ue-9'),
      );

      expect(seen!.method, 'getUnknownEvent');
      final GetUnknownEventRequest wire =
          GetUnknownEventRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.eventId, 'ue-9');
      // 脱敏原文永远可查（翻译是摘要不是裁决，冲突以原文为准）。
      expect(detail.rawJson, contains('tools/call'));
      expect(detail.decodedFieldsJson, isNotEmpty);
      expect(detail.digest.single.key, 'method');
      expect(detail.digest.single.preview, 'tools/call');
      expect(detail.digest.single.meaning, '方法名');
      expect(detail.translation, isNotEmpty);
      expect(detail.recommendedNextStep, isNotEmpty);
      expect(detail.supportBundleHint, isNotEmpty);
    });
  });

  group('runtime 方法面：错误映射（5 条共用同一套语义，逐条各钉一次）', () {
    /// 每条方法各跑一遍「宿主桥未实现」→ HOST_UNAVAILABLE 的复刻（生产形态：
    /// Android 宿主未注册 handler → 引擎回 null → MissingPluginException）。
    Future<void> expectHostUnavailable(Future<Object?> Function() call) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });
      await expectLater(
        call(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    }

    test('getRuntimeState：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async => bridge.getRuntimeState());
    });

    test('getRuntimePolicy：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async => bridge.getRuntimePolicy());
    });

    test('getControlPlane：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async => bridge.getControlPlane());
    });

    test('listUnknownEvents：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛',
        () async {
      await expectHostUnavailable(() async => bridge.listUnknownEvents());
    });

    test('getUnknownEvent：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async => bridge.getUnknownEvent(
          GetUnknownEventRequest(eventId: 'ue-1')));
    });

    test('宿主 PlatformException → code/message 原样透传（NOT_CONNECTED）',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(code: 'NOT_CONNECTED', message: '还没连上主机');
      });
      await expectLater(
        bridge.getRuntimeState(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having((PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.listUnknownEvents(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('Uint8List'))),
      );
    });

    test('坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常',
        () async {
      final Uint8List corrupt = Uint8List.fromList([0xFF, 0xFF, 0xFF]);
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => corrupt);

      await expectLater(
        bridge.getUnknownEvent(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('UnknownEventResponse'))),
      );
    });
  });
}
