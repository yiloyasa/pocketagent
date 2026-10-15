// 协同 7 个方法面测试（B4 第 4 波追补）——mock 的仍是「宿主桥的 wire 行为」，
// PaBridge 全程真实执行。模式与 pa_bridge_threads_test.dart /
// pa_bridge_files_git_test.dart 完全一致（同一套 mock handler + 同一套错误映射
// 断言），只是文件按方法面拆分。
//
// 【字节来源】响应字节一律由 Dart gen stub 现组（`XxxResponse(...)
// .writeToBuffer()`），等价 Kotlin 宿主 `XxxResponse.newBuilder().addXxx(...)`
// 的写法；会话/成员卡逐字段摆真值（string 状态口径、int64 时间戳、repeated
// 嵌套成员与审批），通道层对账的就是这些字段。空响应用空消息 writeToBuffer()
// ——0 字节也是合法 proto。
//
// 【为什么没有 WatchCollabSession 用例】它是服务器流，本波 transport 不做
// stream 面（B2 契约事件面只到 conn_state/host_info/host_event；stream 消费留
// B5）——不测一个不存在的方法，也不假装它存在。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/runtime.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
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

  /// 一张成员卡（daemon 侧的真实字段形状：string 状态 + int64 之外的标量 +
  /// 嵌套审批）。供多个方法的响应用。
  CollabMemberSnapshot memberCard({
    String agentId = 'codex',
    String roleId = 'reviewer',
    String threadId = 'thread-1',
    String status = 'RUNNING',
    String progress = '正在读 src/ 下的登录模块',
  }) =>
      CollabMemberSnapshot(
        agentId: agentId,
        roleId: roleId,
        threadId: threadId,
        status: status,
        modelId: 'codex/gpt-5',
        effort: 'high',
        progress: progress,
        summary: '',
        error: '',
        pendingApprovals: <ApprovalRequest>[
          ApprovalRequest(requestId: 'req-1', reason: '即将执行 rm -rf build'),
        ],
        workspace: r'G:\repo',
      );

  CollabSessionSnapshot sessionSnapshot({
    String sessionId = 'cs-1',
    String instruction = '修一下登录页的崩溃',
    String status = 'RUNNING',
    String error = '',
  }) =>
      CollabSessionSnapshot(
        sessionId: sessionId,
        instruction: instruction,
        status: status,
        workspace: r'G:\repo',
        createdAtUnixMs: Int64(1728901200000),
        updatedAtUnixMs: Int64(1728904800000),
        members: <CollabMemberSnapshot>[memberCard()],
        error: error,
      );

  group('dispatchCollab：参数编码与返回解码', () {
    test('默认（无参）= 空 DispatchCollabRequest 字节；响应逐字段对账',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return DispatchCollabResponse(
          sessionId: 'cs-42',
          items: <DispatchCollabItem>[
            DispatchCollabItem(
                agentId: 'codex', threadId: 'thread-1', ack: true),
            DispatchCollabItem(
              agentId: 'pi',
              threadId: '',
              ack: false,
              error: '协同角色 reviewer 未定义',
            ),
          ],
        ).writeToBuffer();
      });

      final DispatchCollabResponse response = await bridge.dispatchCollab();

      expect(seen!.method, 'dispatchCollab');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 空字节数组（编码总则 §2.1）');
      expect(response.sessionId, 'cs-42');
      expect(response.items, hasLength(2));
      // 一位受理失败不拖垮其余：items 与请求 members 一一对应、顺序一致
      expect(response.items[0].ack, isTrue);
      expect(response.items[0].threadId, 'thread-1');
      expect(response.items[1].ack, isFalse);
      expect(response.items[1].threadId, isEmpty);
      expect(response.items[1].error, '协同角色 reviewer 未定义',
          reason: '受理失败的人话原因原样到达，页面逐位如实呈现');
    });

    test('带 instruction/members：request 字段原样过桥，宿主可按 proto 回读',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return DispatchCollabResponse().writeToBuffer();
      });

      await bridge.dispatchCollab(DispatchCollabRequest(
        instruction: '修一下登录页的崩溃',
        members: <CollabMember>[
          CollabMember(agentId: 'codex', roleId: 'reviewer'),
          CollabMember(agentId: 'pi'),
        ],
      ));

      final DispatchCollabRequest wire =
          DispatchCollabRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.instruction, '修一下登录页的崩溃');
      expect(wire.members, hasLength(2));
      expect(wire.members[0].agentId, 'codex');
      expect(wire.members[0].roleId, 'reviewer');
      expect(wire.members[1].agentId, 'pi');
      expect(wire.members[1].roleId, isEmpty,
          reason: '未配置角色 = 空串（daemon 用 agent 默认）');
    });
  });

  group('getCollabSession / cancelCollabSession：快照返回解码', () {
    test('session_id 过桥；响应整屏字段（状态 string 口径 + int64 + 成员卡）',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return sessionSnapshot().writeToBuffer();
      });

      final CollabSessionSnapshot snap =
          await bridge.getCollabSession(GetCollabSessionRequest(
        sessionId: 'cs-42',
      ));

      expect(seen!.method, 'getCollabSession');
      expect(seen!.arguments, isA<Uint8List>());
      final GetCollabSessionRequest wire =
          GetCollabSessionRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.sessionId, 'cs-42');
      expect(snap.instruction, '修一下登录页的崩溃');
      expect(snap.status, 'RUNNING');
      expect(snap.createdAtUnixMs.toInt(), 1728901200000,
          reason: 'int64 → fixnum Int64，toInt 后对账');
      expect(snap.members, hasLength(1));
      final CollabMemberSnapshot m = snap.members.single;
      expect(m.agentId, 'codex');
      expect(m.roleId, 'reviewer');
      expect(m.threadId, 'thread-1');
      expect(m.status, 'RUNNING');
      expect(m.modelId, 'codex/gpt-5');
      expect(m.effort, 'high');
      expect(m.pendingApprovals, hasLength(1));
      expect(m.pendingApprovals.single.requestId, 'req-1');
      expect(m.pendingApprovals.single.reason, '即将执行 rm -rf build',
          reason: '嵌套 ApprovalRequest 随成员卡同一份字节到达');
      expect(m.workspace, r'G:\repo');
    });

    test('cancelCollabSession：session_id 过桥；应答是收敛后的会话快照',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        // 取消应答 = 收敛后的快照（会话级 CANCELLED + 会话级原因）
        return sessionSnapshot(
          status: 'CANCELLED',
          error: '用户取消了整个协同',
        ).writeToBuffer();
      });

      final CollabSessionSnapshot snap = await bridge.cancelCollabSession(
        CancelCollabSessionRequest(sessionId: 'cs-42'),
      );

      expect(seen!.method, 'cancelCollabSession');
      final CancelCollabSessionRequest wire =
          CancelCollabSessionRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.sessionId, 'cs-42');
      expect(snap.status, 'CANCELLED');
      expect(snap.error, '用户取消了整个协同');
    });
  });

  group('listCollabSessions：参数编码与返回解码', () {
    test('include_finished 原样过桥；sessions[] 逐条解（顺序保持 daemon 原序）',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListCollabSessionsResponse(
          sessions: <CollabSessionSnapshot>[
            sessionSnapshot(sessionId: 'cs-2'),
            sessionSnapshot(
              sessionId: 'cs-1',
              status: 'COMPLETED',
            ),
          ],
        ).writeToBuffer();
      });

      final ListCollabSessionsResponse response =
          await bridge.listCollabSessions(
              ListCollabSessionsRequest(includeFinished: true));

      final ListCollabSessionsRequest wire =
          ListCollabSessionsRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.includeFinished, isTrue);
      expect(response.sessions, hasLength(2));
      expect(response.sessions[0].sessionId, 'cs-2',
          reason: 'daemon 按创建时间倒序给，客户端不重排');
      expect(response.sessions[1].status, 'COMPLETED');
    });

    test('默认（无参）= 空请求字节；宿主回空响应 = 合法空列表，不是错误',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListCollabSessionsResponse().writeToBuffer();
      });

      final ListCollabSessionsResponse response =
          await bridge.listCollabSessions();

      expect(seen!.arguments as Uint8List, isEmpty);
      expect(response.sessions, isEmpty);
    });
  });

  group('cancelCollabMember / retryCollabMember：成员卡返回解码', () {
    test('session_id + thread_id 过桥；应答是该成员的新卡', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return memberCard(status: 'CANCELLED', progress: '').writeToBuffer();
      });

      final CollabMemberSnapshot card = await bridge.cancelCollabMember(
        CancelCollabMemberRequest(sessionId: 'cs-42', threadId: 'thread-1'),
      );

      final CancelCollabMemberRequest wire =
          CancelCollabMemberRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.sessionId, 'cs-42');
      expect(wire.threadId, 'thread-1',
          reason: '取消/重试都按成员的子线程 id 定位');
      expect(card.status, 'CANCELLED');
      expect(card.agentId, 'codex');
    });

    test('retryCollabMember：应答是新代籍的成员卡（不覆盖旧结果）', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return memberCard(status: 'ACCEPTED', progress: '').writeToBuffer();
      });

      final CollabMemberSnapshot card = await bridge.retryCollabMember(
        RetryCollabMemberRequest(sessionId: 'cs-42', threadId: 'thread-1'),
      );

      final RetryCollabMemberRequest wire =
          RetryCollabMemberRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.sessionId, 'cs-42');
      expect(wire.threadId, 'thread-1');
      expect(card.status, 'ACCEPTED');
      expect(card.threadId, 'thread-1',
          reason: 'thread_id 不变（新轮次生成新结果，定位锚不变）');
    });
  });

  group('getCollabPolicy：空请求字节 + 策略响应解码', () {
    test('空请求 = 空字节数组；policy/effective 分离解（optional 语义）',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return CollabPolicyResponse(
          // 用户配置：只设了一项（其余缺席 = 沿用默认）
          policy: CollabPolicy(collaborationEnabled: true),
          revision: Int64(3),
          policyPath: r'C:\Users\x\.pocketagentd\collab-policy.json',
          // effective：打底默认后全字段齐备
          effective: CollabPolicy(
            collaborationEnabled: true,
            nativeDelegateEnabled: false,
            fanoutEnabled: true,
            maxParallelMembers: 4,
          ),
        ).writeToBuffer();
      });

      final CollabPolicyResponse response = await bridge.getCollabPolicy();

      expect(seen!.method, 'getCollabPolicy');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空消息请求 = 空字节数组（§2.1，同 getHostInfo）');
      // policy = 用户配置：未设置的项缺席（has 为 false），不是「改成 false」
      expect(response.policy.hasCollaborationEnabled(), isTrue);
      expect(response.policy.collaborationEnabled, isTrue);
      expect(response.policy.hasFanoutEnabled(), isFalse,
          reason: 'proto3 optional 缺席 = 沿用默认，与「设置为 false」是两件事');
      // effective = 实际生效值：全字段齐备
      expect(response.effective.fanoutEnabled, isTrue);
      expect(response.effective.maxParallelMembers, 4);
      expect(response.revision.toInt(), 3);
      expect(response.policyPath, isNotEmpty);
    });

    test('旧 daemon 空响应字节（0 字节）= 合法空策略，不是错误', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              CollabPolicyResponse().writeToBuffer());

      final CollabPolicyResponse response = await bridge.getCollabPolicy();

      expect(response.policy.hasCollaborationEnabled(), isFalse);
      expect(response.effective.hasCollaborationEnabled(), isFalse);
      expect(response.revision.toInt(), 0);
    });
  });

  group('协同方法面：错误映射（7 条共用同一套语义，逐条各钉一次）', () {
    /// 每条方法各跑一遍「宿主桥未实现」→ HOST_UNAVAILABLE 的复刻（生产形态：
    /// Android 宿主未注册 handler → 引擎回 null → MissingPluginException）。
    ///
    /// 闭包写成 `() async => ...` 而不是 `(b) => b.method()`：后者的返回类型会
    /// 被推断成调用表达式的静态类型（void 形态时报「expression has type void」），
    /// async 闭包统一收敛成 Future，7 条方法一个 helper 吃得下。
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

    test('dispatchCollab：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async => bridge.dispatchCollab());
    });

    test('getCollabSession：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async =>
          bridge.getCollabSession(GetCollabSessionRequest(sessionId: 'cs-1')));
    });

    test('listCollabSessions：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛',
        () async {
      await expectHostUnavailable(() async => bridge.listCollabSessions());
    });

    test('cancelCollabSession：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛',
        () async {
      await expectHostUnavailable(() async => bridge.cancelCollabSession(
          CancelCollabSessionRequest(sessionId: 'cs-1')));
    });

    test('cancelCollabMember：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛',
        () async {
      await expectHostUnavailable(() async => bridge.cancelCollabMember(
          CancelCollabMemberRequest(sessionId: 'cs-1', threadId: 't-1')));
    });

    test('retryCollabMember：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛',
        () async {
      await expectHostUnavailable(() async => bridge.retryCollabMember(
          RetryCollabMemberRequest(sessionId: 'cs-1', threadId: 't-1')));
    });

    test('getCollabPolicy：宿主桥未实现 → HOST_UNAVAILABLE 如实上抛', () async {
      await expectHostUnavailable(() async => bridge.getCollabPolicy());
    });

    test('宿主 PlatformException → code/message 原样透传（NOT_CONNECTED）',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'NOT_CONNECTED',
          message: '还没连上主机',
        );
      });
      await expectLater(
        bridge.getCollabSession(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having((PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.getCollabSession(),
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
        bridge.getCollabSession(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('CollabSessionSnapshot'))),
      );
    });
  });
}
