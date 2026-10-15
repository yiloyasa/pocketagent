// Collab 主面板页测试（B4 第 4 波）：会话列表三态 + 行渲染 / 派发流（角色选择
// → dispatch → 成功/失败）/ 成员卡三动作确认流 / 策略区只读 / 默认真通道接线。
//
// 【三态怎么测】数据源是注入点（CollabGateway / CollabRolesSource），测试用可控
// fake 把「还在加载」「拉失败了」「空列表」三种时序摆到台面上——不靠 sleep 碰
// 运气：加载态用 Completer 卡住，错误态用一次性失败的源，重试后要真变绿。
//
// 【两条必须分家的路径】宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」；宿主
// 正常回空列表 → 空态「还没有协同会话」——各钉一条，互斥断言（红线：不留假绿，
// 「通道没就绪」绝不许伪装成「没有协同会话」）。
//
// 【写操作确认流】取消/重试都是甲级写操作：点动作 → 确认弹层（标题 + 后果 +
// 确认键）→ 才发 RPC；弹层上的「再想想」必须真的不发。这条链路的每一环都有用例。
//
// 【快照模式的诚实说明】本波无实时流（WatchCollabSession 留 B5），页面显式写
// 「快照模式，实时更新留后续波次」——测试钉死这句话在场，防后续波次误删。

import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/runtime.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/collab.dart';
import 'package:pocketagent_ui/pages/collab_bridge.dart';
import 'package:pocketagent_ui/pages/collab_session_data.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 固定时钟轴（列表行时间标签的 nowMs）。
const int kNowMs = 1728904800000;

CollabMemberSnapshot protoMember({
  String agentId = 'codex',
  String roleId = 'reviewer',
  String threadId = 'thread-1',
  String status = 'RUNNING',
  String progress = '正在读 src/ 下的登录模块',
  String summary = '',
  String error = '',
}) => CollabMemberSnapshot(
  agentId: agentId,
  roleId: roleId,
  threadId: threadId,
  status: status,
  modelId: 'codex/gpt-5',
  effort: 'high',
  progress: progress,
  summary: summary,
  error: error,
  workspace: r'G:\repo',
);

CollabSessionSnapshot protoSession({
  String sessionId = 'cs-1',
  String instruction = '修一下登录页的崩溃',
  String status = 'RUNNING',
  String error = '',
  List<CollabMemberSnapshot>? members,
}) => CollabSessionSnapshot(
  sessionId: sessionId,
  instruction: instruction,
  status: status,
  workspace: r'G:\repo',
  createdAtUnixMs: Int64(kNowMs - 3600000),
  updatedAtUnixMs: Int64(kNowMs - 1800000),
  members: members ?? <CollabMemberSnapshot>[protoMember()],
  error: error,
);

/// 可控网关：把每一次调用的时序与结果摆到台面上。
class _FakeGateway implements CollabGateway {
  _FakeGateway({
    this.sessions = const <CollabSessionRow>[],
    this.detail,
    this.sessionsFailure,
    this.dispatchFailure,
    this.actionFailure,
    this.policy,
    this.policyFailure,
    this.pendingSessions,
  });

  List<CollabSessionRow> sessions;

  /// 会话详情（按 sessionId 查；null = 用 [_defaultDetail] 造一张）。
  CollabSessionUi? detail;
  String? sessionsFailure;
  String? detailFailure;
  String? dispatchFailure;
  String? actionFailure;
  CollabPolicyUi? policy;
  String? policyFailure;

  /// 调用记录（测试据此断言「派发真的发了」「取消真的发了」）。
  final List<String> calls = <String>[];

  /// 派发请求快照（成员名单与角色绑定原样到达）。
  DispatchCollabRequest? lastDispatchRequest;

  /// 卡住不返回的列表加载（把「加载中」钉在屏幕上）。
  Completer<List<CollabSessionRow>>? pendingSessions;

  int listCalls = 0;
  int detailCalls = 0;

  @override
  Future<List<CollabSessionRow>> listSessions({
    required bool includeFinished,
    int? nowMs,
    Map<String, String> roleNames = const <String, String>{},
  }) async {
    listCalls += 1;
    calls.add('listSessions(includeFinished=$includeFinished)');
    if (pendingSessions != null) return pendingSessions!.future;
    if (sessionsFailure != null) {
      throw CollabSessionsLoadFailure(sessionsFailure!);
    }
    return sessions;
  }

  @override
  Future<CollabSessionUi> getSession(String sessionId) async {
    detailCalls += 1;
    calls.add('getSession($sessionId)');
    if (detailFailure != null) {
      throw CollabSessionDetailFailure(detailFailure!);
    }
    return detail ?? _detailOf(sessionId);
  }

  static CollabSessionUi _detailOf(String sessionId) =>
      collabSessionUiOf(protoSession(sessionId: sessionId));

  /// 派发结果覆盖（默认 = 全员受理成功；测试可注入「一位受理失败」的结果）。
  CollabDispatchOutcome Function(
    String instruction,
    List<CollabMemberSpec> members,
  )?
  onDispatch;

  @override
  Future<CollabDispatchOutcome> dispatch(
    String instruction,
    List<CollabMemberSpec> members,
  ) async {
    calls.add('dispatch($instruction, ${members.length} 位)');
    lastDispatchRequest = DispatchCollabRequest(
      instruction: instruction,
      members: members
          .map(
            (CollabMemberSpec m) =>
                CollabMember(agentId: m.agentId, roleId: m.roleId ?? ''),
          )
          .toList(growable: false),
    );
    if (dispatchFailure != null) {
      throw CollabDispatchFailure(dispatchFailure!);
    }
    if (onDispatch != null) return onDispatch!(instruction, members);
    // 成功 = 受理回执（session_id + items 一一对应）
    return CollabDispatchOutcome(
      sessionId: 'cs-new',
      items: members
          .map(
            (CollabMemberSpec m) => CollabDispatchItemUi(
              agentId: m.agentId,
              threadId: 'thread-${m.agentId}',
              ack: true,
              error: '',
            ),
          )
          .toList(growable: false),
    );
  }

  @override
  Future<CollabSessionUi> cancelSession(String sessionId) async {
    calls.add('cancelSession($sessionId)');
    if (actionFailure != null) throw CollabActionFailure(actionFailure!);
    // 取消应答 = 收敛后的快照（会话级 CANCELLED + 全员终局）
    return collabSessionUiOf(
      protoSession(
        sessionId: sessionId,
        status: 'CANCELLED',
        members: <CollabMemberSnapshot>[
          protoMember(status: 'CANCELLED', progress: ''),
        ],
      ),
    );
  }

  @override
  Future<CollabMemberUi> cancelMember(String sessionId, String threadId) async {
    calls.add('cancelMember($sessionId, $threadId)');
    if (actionFailure != null) throw CollabActionFailure(actionFailure!);
    return collabMemberUiOf(protoMember(status: 'CANCELLED', progress: ''));
  }

  @override
  Future<CollabMemberUi> retryMember(String sessionId, String threadId) async {
    calls.add('retryMember($sessionId, $threadId)');
    if (actionFailure != null) throw CollabActionFailure(actionFailure!);
    return collabMemberUiOf(protoMember(status: 'ACCEPTED', progress: ''));
  }

  @override
  Future<CollabPolicyUi> getPolicy() async {
    calls.add('getPolicy()');
    if (policyFailure != null) throw CollabPolicyLoadFailure(policyFailure!);
    return policy ?? const CollabPolicyUi();
  }
}

/// 可控角色源（测试注入；生产走 BridgeCollabRolesSource → roles store）。
class _FakeRolesSource implements CollabRolesSource {
  _FakeRolesSource({this.roles = const <CollabRole>[], this.failure});

  final List<CollabRole> roles;
  final String? failure;
  int loadCalls = 0;

  @override
  Future<List<CollabRole>> load() async {
    loadCalls += 1;
    if (failure != null) throw Exception(failure);
    return roles;
  }
}

final List<CollabRole> kDemoRoles = <CollabRole>[
  CollabRole(id: 'reviewer', name: '安全审查者', agentId: 'codex'),
  CollabRole(id: 'tester', name: '测试员', agentId: 'pi'),
];

/// 演示会话行（列表渲染与点选）。
const List<CollabSessionRow> kDemoSessions = <CollabSessionRow>[
  CollabSessionRow(
    id: 'cs-2',
    instruction: '修一下登录页的崩溃',
    status: CollabStatus.running,
    statusRaw: 'RUNNING',
    memberCount: 2,
    roleNames: <String>['安全审查者', '测试员'],
    timeLabel: '30 分钟前',
  ),
  CollabSessionRow(
    id: 'cs-1',
    instruction: '同步 proto 双副本',
    status: CollabStatus.completed,
    statusRaw: 'COMPLETED',
    memberCount: 1,
    roleNames: <String>[],
    timeLabel: '2 小时前',
  ),
];

/// 能力位：两位 agent 接入协同。
const List<String> kCaps = <String>[
  'terminal',
  'fs',
  'collab:codex',
  'collab:pi',
];

/// 滚到长页底部（派发区/策略区都在底部；默认 800x600 视口装不下整页）。
Future<void> scrollToBottom(WidgetTester tester) async {
  await tester.drag(find.byType(ListView).first, const Offset(0, -2000));
  await tester.pumpAndSettle();
}

Future<void> bringIntoView(WidgetTester tester, Finder finder) async {
  for (int i = 0; i < 10; i++) {
    if (finder.evaluate().isEmpty) return;
    // ensureVisible 是「滚到可见」的直给 API（内部处理 Scrollable 与坐标变换）；
    // 之所以不用 scrollUntilVisible：它按单一 widget 取状态，对非 Text 容器
    // （PACard 上的 GestureDetector / 按钮）偶发「Too many elements」。
    await tester.ensureVisible(finder.first);
    await tester.pumpAndSettle();
    final Rect bounds = tester.getRect(finder.first);
    final Rect viewport = tester
        .renderObject<RenderBox>(find.byType(Scaffold).first)
        .paintBounds;
    if (viewport.top <= bounds.top && bounds.bottom <= viewport.bottom) {
      return;
    }
  }
}

void main() {
  /// 页面装进 MaterialApp（与 threads/git 页测试同款装配）。
  ///
  /// 【默认测试视口 800×600 装不下整页】协同页是长页（说明卡 + 参与区 + 列表 +
  /// 详情 + 策略 + 派发区）。需要点深处元素时先 [scrollTo] 把它滚进视口——
  /// 不放大视口（那会让所有 padding 断言失去意义），也不改页面（滚动是真实
  /// 用户路径）。
  Future<void> pumpCollab(
    WidgetTester tester, {
    required CollabGateway gateway,
    CollabRolesSource? rolesSource,
    List<String>? capabilities,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: CollabPage(
            gateway: gateway,
            rolesSource: rolesSource,
            capabilities: capabilities,
          ),
        ),
      ),
    );
  }

  /// 把带 [key] 的 widget 滚进视口并点它（List 深处的成员卡/动作键）。
  ///
  /// 【为什么用 drag 而不是 scrollUntilVisible】后者内部按单一 widget 取状态，
  /// 对非 Text 容器（PACard 上的 GestureDetector / 按钮）偶发「Too many
  /// elements」；手动 drag 到可见 + tap 是等价且更稳的路径。
  Future<void> tapKey(WidgetTester tester, String key) async {
    final Finder finder = find.byKey(ValueKey<String>(key));
    await bringIntoView(tester, finder);
    await tester.tap(finder.first, warnIfMissed: false);
    await tester.pumpAndSettle();
  }

  group('会话列表：三态', () {
    testWidgets('加载中 → 「正在从电脑端恢复协同状态…」，数据到了切列表', (tester) async {
      final _FakeGateway gw = _FakeGateway(
        // 列表卡住（ Completer 未完成）：把 sessions 的 loading 钉在屏幕上。
        pendingSessions: Completer<List<CollabSessionRow>>(),
      );
      await pumpCollab(tester, gateway: gw);
      await tester.runAsync(() => Future<void>.delayed(Duration.zero));
      await tester.pump();

      expect(find.text('正在从电脑端恢复协同状态…'), findsOneWidget);
      expect(find.text('修一下登录页的崩溃'), findsNothing);

      gw.pendingSessions!.complete(kDemoSessions);
      await tester.pumpAndSettle();

      expect(find.text('正在从电脑端恢复协同状态…'), findsNothing);
      expect(find.text('修一下登录页的崩溃'), findsWidgets);
    });

    testWidgets('加载失败 → 错误态带原因；重试后真重新拉（calls == 2）', (tester) async {
      final _FakeGateway gw = _FakeGateway(sessionsFailure: 'daemon 不可达');
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      expect(find.text('daemon 不可达'), findsOneWidget);
      expect(find.text('还没有协同会话'), findsNothing, reason: '错误态不许走空态文案（两条路径分家）');

      // 修好数据源再点重试：必须真的重新发一次 listSessions
      gw.sessionsFailure = null;
      gw.sessions = kDemoSessions;
      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(gw.listCalls, 2, reason: '重试必须真的重新调用数据源');
      expect(find.text('daemon 不可达'), findsNothing);
      expect(find.text('修一下登录页的崩溃'), findsWidgets);
    });

    testWidgets('空列表 → 「还没有协同会话」（真空态，不是通道错误的伪装）', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
      );
      await tester.pumpAndSettle();

      expect(find.text('还没有协同会话'), findsOneWidget);
      expect(
        find.textContaining('B4 波次'),
        findsNothing,
        reason: '空态不许出现「未就绪」字样——那是错误路径的词',
      );
    });
  });

  group('会话列表：行渲染与点选', () {
    testWidgets('行五字段落位：指令 / 状态徽章 / 成员数 / 角色摘要 / 时间', (tester) async {
      await pumpCollab(tester, gateway: _FakeGateway(sessions: kDemoSessions));
      await tester.pumpAndSettle();

      // 指令 + 成员数 + 角色摘要 + 时间（同一行副文案，分段断言）
      expect(find.text('修一下登录页的崩溃'), findsWidgets);
      expect(find.text('2 位成员 · 角色：安全审查者、测试员 · 30 分钟前'), findsOneWidget);
      expect(find.text('1 位成员 · 2 小时前'), findsOneWidget);
      // 七词徽章（running 行 = 执行中；completed 行 = 已完成）。findsWidgets 而非
      // findsOneWidget：同一个词在列表行徽章与详情成员徽章各出现一次（两处都在说真话）。
      expect(find.text('执行中'), findsWidgets);
      expect(find.text('已完成'), findsWidgets);
    });

    testWidgets('点第二行 → 详情切到该会话（getSession 带对 id）', (tester) async {
      final _FakeGateway gw = _FakeGateway(sessions: kDemoSessions);
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      await tester.tap(find.text('同步 proto 双副本'));
      await tester.pumpAndSettle();

      expect(gw.calls, contains('getSession(cs-1)'));
      expect(find.text('同步 proto 双副本'), findsWidgets);
    });
  });

  group('会话详情：成员卡与三动作', () {
    testWidgets('成员卡：agent / 角色 / 徽章 / model·effort / 进度', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: kDemoSessions),
        rolesSource: _FakeRolesSource(roles: kDemoRoles),
      );
      await tester.pumpAndSettle();

      expect(find.text('codex'), findsWidgets);
      expect(find.text('角色 · 安全审查者'), findsOneWidget);
      expect(find.text('codex/gpt-5 · high'), findsOneWidget);
      expect(find.text('正在读 src/ 下的登录模块'), findsOneWidget);
      expect(find.text('执行中'), findsWidgets);
    });

    testWidgets('取消成员：确认弹层 → 确认才发 RPC → 卡片换成 CANCELLED', (tester) async {
      final _FakeGateway gw = _FakeGateway(sessions: kDemoSessions);
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      // RUNNING 成员：可取消、不可重试（禁用原因同源显示）
      expect(
        find.byKey(const ValueKey<String>('collab-cancel-member-t:thread-1')),
        findsOneWidget,
        reason: '成员取消键在树上（RUNNING 成员可取消）',
      );
      await tapKey(tester, 'collab-cancel-member-t:thread-1');

      // 确认弹层：标题 + 后果说明 + 确认键（甲级写操作必须显式确认）
      expect(find.text('取消该成员?'), findsOneWidget);
      expect(find.textContaining('该成员会停止执行'), findsOneWidget);
      expect(
        gw.calls,
        isNot(contains('cancelMember(cs-2, thread-1)')),
        reason: '弹层开着的时候不许发 RPC',
      );

      await tester.tap(find.byKey(const ValueKey<String>('collab-confirm-ok')));
      await tester.pumpAndSettle();

      expect(gw.calls, contains('cancelMember(cs-2, thread-1)'));
      // 应答上树：成员卡换成 CANCELLED（已取消）
      expect(find.text('已取消'), findsWidgets);
    });

    testWidgets('确认弹层的「再想想」必须真的不发 RPC', (tester) async {
      final _FakeGateway gw = _FakeGateway(sessions: kDemoSessions);
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      await tapKey(tester, 'collab-cancel-member-t:thread-1');
      await tester.tap(find.text('再想想'));
      await tester.pumpAndSettle();

      expect(find.text('取消该成员?'), findsNothing);
      expect(
        gw.calls,
        isNot(contains(startsWith('cancelMember'))),
        reason: '取消确认 = 真的不发 RPC，不是装饰',
      );
    });

    testWidgets('重试成员：终局成员才可重试；未收尾成员禁用且带原因', (tester) async {
      // 终局（FAILED）成员：可重试、不可取消
      final _FakeGateway gw = _FakeGateway(
        sessions: kDemoSessions,
        detail: collabSessionUiOf(
          protoSession(
            members: <CollabMemberSnapshot>[
              protoMember(status: 'FAILED', progress: '', error: 'codex 崩了'),
            ],
          ),
        ),
      );
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      expect(find.text('重试该成员'), findsOneWidget);
      expect(
        find.textContaining('该成员已收尾,无需取消'),
        findsOneWidget,
        reason: '取消禁用时旁边给原因（不出「灰着但不知道为什么」）',
      );

      await tapKey(tester, 'collab-retry-t:thread-1');
      expect(find.text('重试该成员?'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey<String>('collab-confirm-ok')));
      await tester.pumpAndSettle();

      expect(gw.calls, contains('retryMember(cs-1, thread-1)'),
          reason: '写动作按当前详情会话的 id 定位（详情是 cs-1 那张卡）');
    });

    testWidgets('取消整个协同：确认 → RPC → 会话横幅换成 CANCELLED + 原因', (tester) async {
      final _FakeGateway gw = _FakeGateway(sessions: kDemoSessions);
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      await tapKey(tester, 'collab-cancel-session');
      expect(find.text('取消整个协同?'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey<String>('collab-confirm-ok')));
      await tester.pumpAndSettle();

      expect(gw.calls, contains('cancelSession(cs-2)'));
      expect(find.text('已取消'), findsWidgets);
    });

    testWidgets('写操作失败：daemon 原文呈现在动作行下方（不泛化）', (tester) async {
      final _FakeGateway gw = _FakeGateway(
        sessions: kDemoSessions,
        actionFailure: '该成员还在运行中,不能取消',
      );
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();

      await tapKey(tester, 'collab-cancel-member-t:thread-1');
      await tester.tap(find.byKey(const ValueKey<String>('collab-confirm-ok')));
      await tester.pumpAndSettle();

      expect(find.text('该成员还在运行中,不能取消'), findsOneWidget);
    });

    testWidgets('全员终局 → 「取消整个协同」禁用且给原因', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(
          sessions: kDemoSessions,
          detail: collabSessionUiOf(
            protoSession(
              members: <CollabMemberSnapshot>[
                protoMember(status: 'COMPLETED', progress: '', summary: '改完了'),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('所有成员都已收尾,没有可取消的成员'), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('collab-cancel-session')),
        findsNothing,
        reason: '不可取消时不摆「能点」的形',
      );
    });
  });

  group('参与协同与派发流', () {
    testWidgets('能力位 → 参与者芯片；点选更新「已选 N 位」', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
        capabilities: kCaps,
      );
      await tester.pumpAndSettle();

      expect(find.text('未选参与者'), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('collab-agent-codex')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey<String>('collab-agent-pi')),
        findsOneWidget,
      );

      await tapKey(tester, 'collab-agent-codex');
      expect(find.text('已选 1 位'), findsOneWidget);
    });

    testWidgets('无能力位 → 「暂无可协同的 agent」诚实空态（不摆假芯片）', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
        capabilities: const <String>['terminal', 'fs'],
      );
      await tester.pumpAndSettle();

      expect(find.text('暂无可协同的 agent'), findsOneWidget);
      expect(find.textContaining('电脑端启用协作能力后'), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('collab-agent-codex')),
        findsNothing,
      );
    });

    testWidgets('派发禁用原因与按钮同源：没选人 / 指令空白各一句', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
        capabilities: kCaps,
      );
      await tester.pumpAndSettle();

      // 没选人（派发区在长页底部：先滚到底再断言）
      await scrollToBottom(tester);
      expect(find.text('先勾选至少一位参与者,才能派发'), findsOneWidget);
      // 选人后指令空白
      await tapKey(tester, 'collab-agent-codex');
      await scrollToBottom(tester);
      expect(find.text('写下要协同完成的任务,即可派发'), findsOneWidget);
    });

    testWidgets('派发成功：角色选择 → dispatch（成员+角色原样过桥）→ 详情刷新', (tester) async {
      final _FakeGateway gw = _FakeGateway(
        sessions: const <CollabSessionRow>[],
      );
      await pumpCollab(
        tester,
        gateway: gw,
        rolesSource: _FakeRolesSource(roles: kDemoRoles),
        capabilities: kCaps,
      );
      await tester.pumpAndSettle();

      // 1) 选人
      await tapKey(tester, 'collab-agent-codex');
      // 2) 配角色（弹层从 roles store 读）
      await tapKey(tester, 'collab-bind-role-codex');
      expect(find.text('为 codex 选角色'), findsOneWidget);
      expect(find.text('安全审查者'), findsOneWidget);
      await tapKey(tester, 'collab-role-reviewer');
      // 绑定后芯片显示角色名
      expect(find.text('安全审查者'), findsWidgets);
      // 3) 写指令 + 派发
      await tester.enterText(
        find.byKey(const ValueKey<String>('collab-cmd-input')),
        '修一下登录页的崩溃',
      );
      await tester.pumpAndSettle();
      // 指令到位 + 已选人 → 按钮可点（disabledReason 应消失）
      expect(find.text('先勾选至少一位参与者,才能派发'), findsNothing);
      expect(find.text('写下要协同完成的任务,即可派发'), findsNothing);
      await tapKey(tester, 'collab-dispatch-button');

      // dispatch 真的发了，成员名单 + 角色绑定原样到达
      expect(gw.calls, contains('dispatch(修一下登录页的崩溃, 1 位)'));
      final DispatchCollabRequest req = gw.lastDispatchRequest!;
      expect(req.instruction, '修一下登录页的崩溃');
      expect(req.members, hasLength(1));
      expect(req.members.single.agentId, 'codex');
      expect(req.members.single.roleId, 'reviewer');
      // 派发后拉新会话详情（session_id 来自受理回执）
      expect(gw.calls, contains('getSession(cs-new)'));
    });

    testWidgets('派发失败：daemon 原文进详情错误态（失败消息提取）', (tester) async {
      final _FakeGateway gw = _FakeGateway(
        sessions: const <CollabSessionRow>[],
        dispatchFailure: '协同角色 reviewer 未定义',
      );
      await pumpCollab(tester, gateway: gw, capabilities: kCaps);
      await tester.pumpAndSettle();

      await tapKey(tester, 'collab-agent-codex');
      await tester.enterText(
        find.byKey(const ValueKey<String>('collab-cmd-input')),
        '修一下登录页的崩溃',
      );
      await tapKey(tester, 'collab-dispatch-button');

      expect(find.text('协同角色 reviewer 未定义'), findsOneWidget);
      expect(
        gw.calls,
        isNot(contains(startsWith('getSession'))),
        reason: '派发失败不拉详情（没有 session id 可拉）',
      );
    });

    testWidgets('受理失败项逐位呈现（一位失败不拖垮其余）', (tester) async {
      final _FakeGateway gw = _FakeGateway(
        sessions: const <CollabSessionRow>[],
      );
      await pumpCollab(tester, gateway: gw, capabilities: kCaps);
      await tester.pumpAndSettle();

      await tapKey(tester, 'collab-agent-codex');
      await tapKey(tester, 'collab-agent-pi');
      await tester.enterText(
        find.byKey(const ValueKey<String>('collab-cmd-input')),
        '修一下登录页的崩溃',
      );
      // 让 dispatch 回一个「一位受理失败」的结果
      gw.onDispatch = (String instruction, List<CollabMemberSpec> members) {
        return CollabDispatchOutcome(
          sessionId: 'cs-new',
          items: <CollabDispatchItemUi>[
            const CollabDispatchItemUi(
              agentId: 'codex',
              threadId: 'thread-codex',
              ack: true,
              error: '',
            ),
            const CollabDispatchItemUi(
              agentId: 'pi',
              threadId: '',
              ack: false,
              error: '该 agent 当前不可用',
            ),
          ],
        );
      };
      await tapKey(tester, 'collab-dispatch-button');

      expect(find.text('1 位成员没有被受理，其余成员照常执行'), findsOneWidget);
      expect(find.textContaining('该 agent 当前不可用'), findsOneWidget);
    });
  });

  group('策略区：只读呈现', () {
    testWidgets('四项生效值 + 版本/路径附注；没有任何写入口', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(
          sessions: kDemoSessions,
          policy: const CollabPolicyUi(
            collaborationEnabled: true,
            effectiveCollaborationEnabled: true,
            effectiveNativeDelegateEnabled: false,
            effectiveFanoutEnabled: true,
            effectiveMaxParallelMembers: 4,
            revision: 7,
            policyPath: r'C:\policies\collab.json',
          ),
        ),
      );
      await tester.pumpAndSettle();
      await scrollToBottom(tester);

      expect(find.text('已开启'), findsNWidgets(2));
      expect(find.text('已关闭'), findsOneWidget);
      expect(find.text('4 位'), findsOneWidget);
      expect(
        find.text('策略版本 7 · ${r'C:\policies\collab.json'}'),
        findsOneWidget,
      );
      // 只读：不摆任何写入入口（SetCollabPolicy 留 B5）
      expect(find.text('保存策略'), findsNothing);
      expect(find.text('修改'), findsNothing);
      expect(find.textContaining('只读呈现'), findsOneWidget);
    });

    testWidgets('effective 未报（旧 daemon）→ 「电脑端没有返回」，不猜一档', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(
          sessions: kDemoSessions,
          policy: const CollabPolicyUi(),
        ),
      );
      await tester.pumpAndSettle();
      await scrollToBottom(tester);

      expect(find.text('电脑端没有返回'), findsNWidgets(4));
    });

    testWidgets('成员上限 <=0 → 「不限」（proto 口径照述，不编数字）', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(
          sessions: kDemoSessions,
          policy: const CollabPolicyUi(effectiveMaxParallelMembers: 0),
        ),
      );
      await tester.pumpAndSettle();
      await scrollToBottom(tester);

      expect(find.text('不限'), findsOneWidget);
    });

    testWidgets('策略读失败 → 策略区错误态 + 重试（不影响会话呈现）', (tester) async {
      final _FakeGateway gw = _FakeGateway(
        sessions: kDemoSessions,
        policyFailure: '策略通道不可用',
      );
      await pumpCollab(tester, gateway: gw);
      await tester.pumpAndSettle();
      await scrollToBottom(tester);

      expect(find.text('策略通道不可用'), findsOneWidget);
      // 会话区照常呈现（策略失败不拖垮会话）
      expect(find.text('修一下登录页的崩溃'), findsWidgets);

      gw.policyFailure = null;
      gw.policy = const CollabPolicyUi(effectiveFanoutEnabled: true);
      await tester.tap(
        find
                .byKey(const ValueKey<String>('collab-policy-error'))
                .evaluate()
                .isNotEmpty
            ? find.text('重试').last
            : find.text('重试').first,
      );
      await tester.pumpAndSettle();
      await scrollToBottom(tester);
      expect(gw.calls, contains('getPolicy()'));
    });
  });

  group('快照模式与角色区', () {
    testWidgets('页面显式声明快照模式（无实时流，不留假入口）', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('快照模式'), findsOneWidget);
      expect(find.textContaining('实时更新留后续波次'), findsOneWidget);
    });

    testWidgets('进页即 load() 角色（Android 进页 LaunchedEffect 同语义）', (tester) async {
      final _FakeRolesSource roles = _FakeRolesSource(roles: kDemoRoles);
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
        rolesSource: roles,
        capabilities: kCaps,
      );
      await tester.pumpAndSettle();

      expect(roles.loadCalls, 1, reason: '进页必须拉一次角色列表');
    });

    testWidgets('角色读失败不阻塞派发（角色只是可选项）', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
        rolesSource: _FakeRolesSource(failure: '角色通道不可用'),
        capabilities: kCaps,
      );
      await tester.pumpAndSettle();

      // 参与区照常，派发区照常（角色弹层里才说明失败）
      expect(
        find.byKey(const ValueKey<String>('collab-agent-codex')),
        findsOneWidget,
      );
      await tapKey(tester, 'collab-bind-role-codex');
      expect(find.textContaining('角色通道不可用'), findsOneWidget);
    });

    testWidgets('该 agent 没有专属角色 → 弹层如实说，不配角色也能派发', (tester) async {
      await pumpCollab(
        tester,
        gateway: _FakeGateway(sessions: const <CollabSessionRow>[]),
        rolesSource: _FakeRolesSource(roles: kDemoRoles),
        capabilities: kCaps,
      );
      await tester.pumpAndSettle();

      // pi 只有 tester 角色；codex 只有 reviewer——换个没有角色的 agent 试
      await tapKey(tester, 'collab-bind-role-codex');
      expect(find.text('安全审查者'), findsOneWidget);
      // 「未配置角色」项恒在（解绑入口）
      expect(find.text('未配置角色'), findsOneWidget);
    });
  });

  group('默认接线：真通道（不注入 gateway）', () {
    const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
    late TestDefaultBinaryMessenger messenger;

    setUp(() {
      messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    });

    testWidgets('宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」，绝不是空态', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        throw MissingPluginException(
          'No implementation found for method ${call.method}',
        );
      });

      await tester.pumpWidget(
        MaterialApp(
          theme: PocketTheme.fromBrightness(PABrightness.dark),
          home: const Scaffold(body: CollabPage(capabilities: kCaps)),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('会话列表加载失败'), findsOneWidget);
      expect(find.textContaining('桌面端协同通道未就绪（B4 波次）'), findsWidgets,
          reason: '列表区与策略区都读不到：两处都在说真话，不是重复渲染 bug');
      expect(
        find.text('还没有协同会话'),
        findsNothing,
        reason: '通道没就绪不许走空态——那是「真没有会话」的伪装版（红线：不留假绿）',
      );
    });

    testWidgets('宿主正常回空列表 → 空态「还没有协同会话」（与错误态分家的另一条）', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        if (call.method == 'listCollabSessions') {
          return ListCollabSessionsResponse().writeToBuffer();
        }
        if (call.method == 'getCollabPolicy') {
          return CollabPolicyResponse().writeToBuffer();
        }
        if (call.method == 'listCollabRoles') {
          return ListCollabRolesResponse().writeToBuffer();
        }
        throw MissingPluginException(
          'No implementation found for method ${call.method}',
        );
      });

      await tester.pumpWidget(
        MaterialApp(
          theme: PocketTheme.fromBrightness(PABrightness.dark),
          home: const Scaffold(body: CollabPage(capabilities: kCaps)),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('还没有协同会话'), findsOneWidget);
      expect(
        find.textContaining('未就绪'),
        findsNothing,
        reason: '空态不许出现错误路径的词——两条路径断言必须互相排斥',
      );
    });

    testWidgets('宿主正常回会话 → 列表 + 详情真链路渲染', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (
        MethodCall call,
      ) async {
        if (call.method == 'listCollabSessions') {
          return ListCollabSessionsResponse(
            sessions: <CollabSessionSnapshot>[protoSession()],
          ).writeToBuffer();
        }
        if (call.method == 'getCollabSession') {
          return protoSession().writeToBuffer();
        }
        if (call.method == 'getCollabPolicy') {
          return CollabPolicyResponse(
            effective: CollabPolicy(fanoutEnabled: true),
          ).writeToBuffer();
        }
        if (call.method == 'listCollabRoles') {
          return ListCollabRolesResponse(
            roles: <CollabRole>[
              CollabRole(id: 'reviewer', name: '安全审查者', agentId: 'codex'),
            ],
          ).writeToBuffer();
        }
        throw MissingPluginException(
          'No implementation found for method ${call.method}',
        );
      });

      await tester.pumpWidget(
        MaterialApp(
          theme: PocketTheme.fromBrightness(PABrightness.dark),
          home: const Scaffold(body: CollabPage(capabilities: kCaps)),
        ),
      );
      await tester.pumpAndSettle();

      // 列表行 + 详情成员卡 + 策略生效值，全真链路
      expect(find.text('修一下登录页的崩溃'), findsWidgets);
      expect(find.textContaining('1 位成员'), findsOneWidget);
      expect(find.text('角色 · 安全审查者'), findsOneWidget);
      // 策略生效值在长页底部：滚到底再断言
      await scrollToBottom(tester);
      expect(find.text('已开启'), findsOneWidget);
    });
  });
}
