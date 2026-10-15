// Collab 映射 + 真通道数据源测试（B4 第 4 波）。
//
// 【两层各自的证词】
//   ① 纯函数（wire → 行/卡/策略）：不拖通道，逐字段钉死「行上的字从 wire 哪个
//      字段来」——string 状态口径、optional 缺席语义、时间标签、角色摘要；
//   ② 数据源走真 PaBridge + mock 宿主 wire 行为：两条**必须分家**的路径——
//      HOST_UNAVAILABLE → 人话错误（点名 B4 波次）vs 空响应 → 真空列表——各钉
//      一条（红线：不留假绿，「通道没就绪」绝不许伪装成「没有协同会话」）；
//      派发/写动作路径再钉失败消息提取（宿主 message 优先、空则退 code）。
//
// 【口径来源】字段映射逐条对位 Android `CollabSessionReducer.mapMember /
// fromSnapshot` 与 `CollabCopy`（collab_session_data.dart），双端同一会话长同
// 一样；时间标签复用 threads_bridge 的 threadRowTimeLabel（同一口径不写两份）。

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/runtime.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/collab_bridge.dart';
import 'package:pocketagent_ui/pages/collab_session_data.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 固定时钟轴（与 threads/files 测试同纪律：相对时间不可单测，必须钉死 nowMs）。
const int kNowMs = 1728904800000;

CollabMemberSnapshot protoMember({
  String agentId = 'codex',
  String roleId = 'reviewer',
  String threadId = 'thread-1',
  String status = 'RUNNING',
  String progress = '正在读 src/ 下的登录模块',
  String summary = '',
  String error = '',
  String modelId = 'codex/gpt-5',
  String effort = 'high',
}) => CollabMemberSnapshot(
  agentId: agentId,
  roleId: roleId,
  threadId: threadId,
  status: status,
  modelId: modelId,
  effort: effort,
  progress: progress,
  summary: summary,
  error: error,
  pendingApprovals: <ApprovalRequest>[
    ApprovalRequest(requestId: 'req-1', reason: '即将执行 rm -rf build'),
  ],
  workspace: r'G:\repo',
);

CollabSessionSnapshot protoSession({
  String sessionId = 'cs-1',
  String instruction = '修一下登录页的崩溃',
  String status = 'RUNNING',
  String error = '',
  int updatedAtMs = kNowMs,
  List<CollabMemberSnapshot>? members,
}) => CollabSessionSnapshot(
  sessionId: sessionId,
  instruction: instruction,
  status: status,
  workspace: r'G:\repo',
  createdAtUnixMs: Int64(kNowMs - 3600000),
  updatedAtUnixMs: Int64(updatedAtMs),
  members: members ?? <CollabMemberSnapshot>[protoMember()],
  error: error,
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('wire → 页面模型映射（纯函数）', () {
    test('collabMemberUiOf：11 字段逐一落位（string 状态 + 嵌套审批）', () {
      final CollabMemberUi m = collabMemberUiOf(protoMember());

      expect(m.agentId, 'codex');
      expect(m.roleId, 'reviewer');
      expect(m.threadId, 'thread-1');
      // 状态是 string 口径：认识的落七词，原文留在 statusRaw
      expect(m.status, CollabStatus.running);
      expect(m.statusRaw, 'RUNNING');
      expect(m.modelId, 'codex/gpt-5');
      expect(m.effort, 'high');
      expect(m.progress, '正在读 src/ 下的登录模块');
      expect(m.summary, isEmpty);
      expect(m.error, isEmpty);
      expect(m.workspace, r'G:\repo');
      expect(m.pendingApprovals, hasLength(1));
      expect(m.pendingApprovals.single.requestId, 'req-1');
      expect(
        m.pendingApprovals.single.label,
        '即将执行 rm -rf build',
        reason: '审批短句取 ApprovalRequest.reason（daemon 侧放工具名/理由）',
      );
      // 定位键：有子线程按线程（取消/重试同锚）
      expect(m.key, 't:thread-1');
    });

    test('成员卡键兜底：受理失败的卡没有子线程 → 按 agent 兜位', () {
      final CollabMemberUi m = collabMemberUiOf(
        protoMember(threadId: '', status: 'FAILED'),
      );
      expect(m.key, 'a:codex');
      expect(m.status, CollabStatus.failed);
    });

    test('不认识的成员状态串 → unknown + 原文保留（不猜）', () {
      final CollabMemberUi m = collabMemberUiOf(protoMember(status: 'PAUSED'));
      expect(m.status, CollabStatus.unknown);
      expect(m.statusRaw, 'PAUSED');
    });

    test('collabSessionUiOf：会话级字段 + 成员列表整屏投影', () {
      final CollabSessionUi s = collabSessionUiOf(
        protoSession(
          members: <CollabMemberSnapshot>[
            protoMember(agentId: 'codex'),
            protoMember(agentId: 'pi', threadId: 'thread-2'),
          ],
        ),
      );

      expect(s.sessionId, 'cs-1');
      expect(s.instruction, '修一下登录页的崩溃');
      expect(s.status, CollabStatus.running);
      expect(s.statusRaw, 'RUNNING');
      expect(s.workspace, r'G:\repo');
      expect(s.sessionError, isEmpty);
      expect(s.members, hasLength(2));
      expect(s.members[1].agentId, 'pi');
      // 快照路径：loading 收尾、传输层错误清零（快照是 daemon 真值）
      expect(s.loading, isFalse);
      expect(s.error, isNull);
    });

    test('空 session_id → null（与 Kotlin ifBlank{null} 同口径）', () {
      expect(collabSessionUiOf(protoSession(sessionId: '')).sessionId, isNull);
    });

    test('collabSessionRowOf：列表行五字段 + 角色摘要 + 时间标签', () {
      final CollabSessionRow row = collabSessionRowOf(
        protoSession(
          updatedAtMs: kNowMs - 1800000,
          members: <CollabMemberSnapshot>[
            protoMember(agentId: 'codex', roleId: 'reviewer'),
            protoMember(agentId: 'pi', roleId: 'tester', threadId: 'thread-2'),
            // 未配置角色的成员不占角色摘要位
            protoMember(agentId: 'dsh', roleId: '', threadId: 'thread-3'),
          ],
        ),
        nowMs: kNowMs,
        roleNames: const <String, String>{'reviewer': '安全审查者', 'tester': '测试员'},
      );

      expect(row.id, 'cs-1');
      expect(row.instruction, '修一下登录页的崩溃');
      expect(row.status, CollabStatus.running);
      expect(row.memberCount, 3);
      expect(row.roleNames, <String>[
        '安全审查者',
        '测试员',
      ], reason: '未配置角色的成员不占位；显示名走角色表，顺序 = 受理顺序');
      // 时间标签口径同 threads 行：一小时内 = N 分钟前
      expect(row.timeLabel, '30 分钟前');
    });

    test('角色表里没有的 roleId → 回落 roleId 原文（不猜一个角色名）', () {
      final CollabSessionRow row = collabSessionRowOf(
        protoSession(
          members: <CollabMemberSnapshot>[protoMember(roleId: 'ghost-role')],
        ),
        nowMs: kNowMs,
      );
      expect(row.roleNames, <String>['ghost-role']);
    });

    test('时间戳缺席（0）→ 时间标签空串（不显示假时间）', () {
      final CollabSessionRow row = collabSessionRowOf(
        protoSession(updatedAtMs: 0),
        nowMs: kNowMs,
      );
      expect(row.timeLabel, isEmpty);
    });

    test('collabDispatchOutcomeOf：items 一一对应（一位失败不拖垮其余）', () {
      final CollabDispatchOutcome o = collabDispatchOutcomeOf(
        DispatchCollabResponse(
          sessionId: 'cs-42',
          items: <DispatchCollabItem>[
            DispatchCollabItem(
              agentId: 'codex',
              threadId: 'thread-1',
              ack: true,
            ),
            DispatchCollabItem(
              agentId: 'pi',
              threadId: '',
              ack: false,
              error: '协同角色 reviewer 未定义',
            ),
          ],
        ),
      );
      expect(o.sessionId, 'cs-42');
      expect(o.items, hasLength(2));
      expect(o.items[0].ack, isTrue);
      expect(o.items[1].ack, isFalse);
      expect(o.items[1].error, '协同角色 reviewer 未定义');
      expect(o.items[1].threadId, isEmpty);
    });

    test('collabPolicyUiOf：policy 缺席项 = null；effective 全字段齐备', () {
      final CollabPolicyUi p = collabPolicyUiOf(
        CollabPolicyResponse(
          // 用户配置只设了一项：其余缺席 = 沿用默认（不是「改成 false」）
          policy: CollabPolicy(collaborationEnabled: true),
          revision: Int64(7),
          policyPath: r'C:\policies\collab.json',
          effective: CollabPolicy(
            collaborationEnabled: true,
            nativeDelegateEnabled: false,
            fanoutEnabled: true,
            maxParallelMembers: 4,
          ),
        ),
      );

      expect(p.collaborationEnabled, isTrue);
      expect(
        p.nativeDelegateEnabled,
        isNull,
        reason: 'policy 未设置 = 沿用默认，页面不替它选一档',
      );
      expect(p.fanoutEnabled, isNull);
      expect(p.maxParallelMembers, isNull);
      expect(p.effectiveCollaborationEnabled, isTrue);
      expect(p.effectiveNativeDelegateEnabled, isFalse);
      expect(p.effectiveFanoutEnabled, isTrue);
      expect(p.effectiveMaxParallelMembers, 4);
      expect(p.revision, 7);
      expect(p.policyPath, r'C:\policies\collab.json');
    });

    test('旧 daemon 空策略响应 → 全字段 null（页面说「电脑端没有返回」）', () {
      final CollabPolicyUi p = collabPolicyUiOf(CollabPolicyResponse());
      expect(p.collaborationEnabled, isNull);
      expect(p.effectiveCollaborationEnabled, isNull);
      expect(p.revision, 0);
      expect(p.policyPath, isEmpty);
    });
  });

  group('错误文案（透传纪律：只在 HOST_UNAVAILABLE 定制）', () {
    test('HOST_UNAVAILABLE → 点名「协同通道未就绪（B4 波次）」，不是空列表', () {
      const PaBridgeException e = PaBridgeException(
        PaBridgeErrorCodes.hostUnavailable,
        '宿主桥未注册',
      );
      expect(collabLoadErrorText(e), contains('桌面端协同通道未就绪（B4 波次）'));
      expect(collabLoadErrorText(e), contains('协同数据暂不可用。'));
    });

    test('宿主其他码 → 原样透传 message（不改写、不吞）', () {
      const PaBridgeException e = PaBridgeException('NOT_CONNECTED', '还没连上主机');
      expect(collabLoadErrorText(e), '还没连上主机');
    });

    test('宿主错误无 message → 退 code（宁可难看也不吞）', () {
      const PaBridgeException e = PaBridgeException('INTERNAL', '');
      expect(collabLoadErrorText(e), 'INTERNAL');
    });

    test('派发失败文案按派发语境收尾', () {
      const PaBridgeException e = PaBridgeException(
        PaBridgeErrorCodes.hostUnavailable,
        'x',
      );
      expect(collabDispatchErrorText(e), contains('本次派发没有发出去'));
      expect(collabActionErrorText(e), contains('这个操作没有生效'));
    });
  });

  group('BridgeCollabGateway：真通道行为（mock 宿主 wire）', () {
    const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
    late TestDefaultBinaryMessenger messenger;

    setUp(() {
      messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    });

    /// 记录调用 + 回固定字节（成功路径）。
    void hostReturns(Object? bytes) {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        return bytes;
      });
    }

    test('listSessions 成功：wire → 行列表（顺序保持 daemon 原序，不重排）', () async {
      hostReturns(
        ListCollabSessionsResponse(
          sessions: <CollabSessionSnapshot>[
            protoSession(sessionId: 'cs-2'),
            protoSession(sessionId: 'cs-1', status: 'COMPLETED'),
          ],
        ).writeToBuffer(),
      );

      final List<CollabSessionRow> rows = await BridgeCollabGateway()
          .listSessions(includeFinished: true, nowMs: kNowMs);

      expect(rows, hasLength(2));
      expect(rows[0].id, 'cs-2');
      expect(rows[1].id, 'cs-1');
      expect(rows[1].status, CollabStatus.completed);
    });

    test('listSessions 空响应（0 字节）= 真空列表；与错误路径分家', () async {
      hostReturns(ListCollabSessionsResponse().writeToBuffer());

      final List<CollabSessionRow> rows = await BridgeCollabGateway()
          .listSessions(includeFinished: false, nowMs: kNowMs);

      expect(rows, isEmpty);
    });

    test(
      'listSessions：HOST_UNAVAILABLE → CollabSessionsLoadFailure（不是空列表）',
      () async {
        messenger.setMockMethodCallHandler(methodChannel, (
          MethodCall call,
        ) async {
          throw MissingPluginException(
            'No implementation found for method ${call.method}',
          );
        });

        await expectLater(
          BridgeCollabGateway().listSessions(
            includeFinished: false,
            nowMs: kNowMs,
          ),
          throwsA(
            isA<CollabSessionsLoadFailure>().having(
              (CollabSessionsLoadFailure e) => e.message,
              'message',
              contains('未就绪（B4 波次）'),
            ),
          ),
        );
      },
    );

    test('getSession 成功：整屏字段落位（含成员卡与审批）', () async {
      hostReturns(protoSession().writeToBuffer());

      final CollabSessionUi s = await BridgeCollabGateway().getSession('cs-1');

      expect(s.sessionId, 'cs-1');
      expect(s.instruction, '修一下登录页的崩溃');
      expect(s.members, hasLength(1));
      expect(s.members.single.pendingApprovals, hasLength(1));
    });

    test(
      'getSession：宿主 NotFound（人话 message）→ CollabSessionDetailFailure 透传',
      () async {
        messenger.setMockMethodCallHandler(methodChannel, (
          MethodCall call,
        ) async {
          throw PlatformException(
            code: 'NOT_FOUND',
            message: '协同会话 cs-404 不存在',
          );
        });

        await expectLater(
          BridgeCollabGateway().getSession('cs-404'),
          throwsA(
            isA<CollabSessionDetailFailure>().having(
              (CollabSessionDetailFailure e) => e.message,
              'message',
              contains('协同会话 cs-404 不存在'),
            ),
          ),
        );
      },
    );

    test('dispatch 成功：session_id + items 原样到达', () async {
      MethodCall? seen;
      hostReturns(
        DispatchCollabResponse(
          sessionId: 'cs-42',
          items: <DispatchCollabItem>[
            DispatchCollabItem(
              agentId: 'codex',
              threadId: 'thread-1',
              ack: true,
            ),
          ],
        ).writeToBuffer(),
      );
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        seen = call;
        return DispatchCollabResponse(
          sessionId: 'cs-42',
          items: <DispatchCollabItem>[
            DispatchCollabItem(
              agentId: 'codex',
              threadId: 'thread-1',
              ack: true,
            ),
          ],
        ).writeToBuffer();
      });

      final CollabDispatchOutcome o = await BridgeCollabGateway().dispatch(
        '修一下登录页的崩溃',
        const <CollabMemberSpec>[
          CollabMemberSpec(agentId: 'codex', roleId: 'reviewer'),
          CollabMemberSpec(agentId: 'pi'),
        ],
      );

      expect(seen!.method, 'dispatchCollab');
      // 请求成员原样过桥（含角色绑定与空角色的两种形态）
      final DispatchCollabRequest wire = DispatchCollabRequest.fromBuffer(
        seen!.arguments as Uint8List,
      );
      expect(wire.instruction, '修一下登录页的崩溃');
      expect(wire.members, hasLength(2));
      expect(wire.members[0].roleId, 'reviewer');
      expect(wire.members[1].roleId, isEmpty);
      expect(o.sessionId, 'cs-42');
      expect(o.items.single.ack, isTrue);
    });

    test('dispatch 失败：宿主 message 优先提取（失败消息提取纪律）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        throw PlatformException(
          code: 'INVALID_ARGUMENT',
          message: '协同角色 reviewer 未定义',
        );
      });

      await expectLater(
        BridgeCollabGateway().dispatch('修一下登录页的崩溃', const <CollabMemberSpec>[
          CollabMemberSpec(agentId: 'codex'),
        ]),
        throwsA(
          isA<CollabDispatchFailure>().having(
            (CollabDispatchFailure e) => e.message,
            'message',
            contains('协同角色 reviewer 未定义'),
          ),
        ),
      );
    });

    test('dispatch 失败：无 message 时退 code（不泛化成「操作失败」）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        throw PlatformException(code: 'INTERNAL', message: '');
      });

      await expectLater(
        BridgeCollabGateway().dispatch('x', const <CollabMemberSpec>[
          CollabMemberSpec(agentId: 'codex'),
        ]),
        throwsA(
          isA<CollabDispatchFailure>().having(
            (CollabDispatchFailure e) => e.message,
            'message',
            'INTERNAL',
          ),
        ),
      );
    });

    test('三个写动作成功：应答分别是会话快照 / 成员卡 / 成员卡', () async {
      hostReturns(protoSession(status: 'CANCELLED').writeToBuffer());

      final CollabSessionUi cancelled = await BridgeCollabGateway()
          .cancelSession('cs-1');
      expect(cancelled.status, CollabStatus.cancelled);

      hostReturns(
        protoMember(status: 'CANCELLED', progress: '').writeToBuffer(),
      );
      final CollabMemberUi m = await BridgeCollabGateway().cancelMember(
        'cs-1',
        'thread-1',
      );
      expect(m.status, CollabStatus.cancelled);

      hostReturnscollabRetry(messenger);
      final CollabMemberUi r = await BridgeCollabGateway().retryMember(
        'cs-1',
        'thread-1',
      );
      expect(r.status, CollabStatus.accepted);
    });

    test('写动作失败：daemon 前置条件原文透传（不泛化）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        throw PlatformException(
          code: 'FAILED_PRECONDITION',
          message: '该成员还在运行中,不能重试',
        );
      });

      await expectLater(
        BridgeCollabGateway().retryMember('cs-1', 'thread-1'),
        throwsA(
          isA<CollabActionFailure>().having(
            (CollabActionFailure e) => e.message,
            'message',
            contains('该成员还在运行中,不能重试'),
          ),
        ),
      );
    });

    test('getPolicy 成功 / 失败两条路径', () async {
      hostReturns(
        CollabPolicyResponse(
          policy: CollabPolicy(collaborationEnabled: true),
          effective: CollabPolicy(
            collaborationEnabled: true,
            nativeDelegateEnabled: false,
            fanoutEnabled: true,
            maxParallelMembers: 4,
          ),
        ).writeToBuffer(),
      );

      final CollabPolicyUi p = await BridgeCollabGateway().getPolicy();
      expect(p.effectiveFanoutEnabled, isTrue);
      expect(p.effectiveMaxParallelMembers, 4);

      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        throw MissingPluginException(
          'No implementation found for method ${call.method}',
        );
      });
      await expectLater(
        BridgeCollabGateway().getPolicy(),
        throwsA(
          isA<CollabPolicyLoadFailure>().having(
            (CollabPolicyLoadFailure e) => e.message,
            'message',
            contains('协同策略暂不可用'),
          ),
        ),
      );
    });
  });
}

/// 让重试应答换一张 ACCEPTED 卡（helper 只为上面那条三动作用例服务）。
void hostReturnscollabRetry(TestDefaultBinaryMessenger messenger) {
  messenger.setMockMethodCallHandler(
    const MethodChannel(kPaBridgeMethodChannel),
    (MethodCall call) async =>
        protoMember(status: 'ACCEPTED', progress: '').writeToBuffer(),
  );
}
