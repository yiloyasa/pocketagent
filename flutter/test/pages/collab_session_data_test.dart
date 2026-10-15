// Collab 纯函数层测试（B4 第 4 波）：七词状态词表 / 徽章映射 / 派生 / 计数 /
// 汇总行 / 动作可用性 / 派发判定 / 能力位名单 / 文案禁词。
//
// 【测什么】对位 Android `CollabSessionReducerTest` + `CollabDispatchFlowTest`
// 钉的那批规则——这些规则最容易被「顺手改个条件」悄悄反转且零报错，只能等用户
// 在真机上看见错乱的选中态/状态词才暴露。集中成纯函数后，改坏任何一条映射在
// 这里红，而不是在用户手上红。
//
// 【不测什么】wire 映射与真通道行为在 collab_bridge_test.dart；页面行为在
// collab_page_test.dart。三层各自可测，回归时分得清是逻辑错、映射错还是渲染错。

import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_status_chip.dart';
import 'package:pocketagent_ui/pages/collab_session_data.dart';

CollabMemberUi member({
  String agentId = 'codex',
  String roleId = '',
  String threadId = 'thread-1',
  CollabStatus status = CollabStatus.running,
  String statusRaw = 'RUNNING',
  String progress = '',
  String summary = '',
  String error = '',
  List<CollabApprovalUi> pendingApprovals = const <CollabApprovalUi>[],
}) => CollabMemberUi(
  agentId: agentId,
  roleId: roleId,
  threadId: threadId,
  status: status,
  statusRaw: statusRaw,
  modelId: 'codex/gpt-5',
  effort: 'high',
  progress: progress,
  summary: summary,
  error: error,
  pendingApprovals: pendingApprovals,
  workspace: r'G:\repo',
);

void main() {
  group('CollabStatus.fromWire：daemon 状态串 → 七词（string 口径，不猜）', () {
    test('六个已知状态串逐一对位', () {
      expect(CollabStatus.fromWire('ACCEPTED'), CollabStatus.accepted);
      expect(CollabStatus.fromWire('RUNNING'), CollabStatus.running);
      expect(
        CollabStatus.fromWire('WAITING_APPROVAL'),
        CollabStatus.waitingApproval,
      );
      expect(CollabStatus.fromWire('COMPLETED'), CollabStatus.completed);
      expect(CollabStatus.fromWire('FAILED'), CollabStatus.failed);
      expect(CollabStatus.fromWire('CANCELLED'), CollabStatus.cancelled);
    });

    test('空串（旧 daemon 未上报）→ unknown，不编一个相近的已知状态', () {
      expect(CollabStatus.fromWire(''), CollabStatus.unknown);
    });

    test('不认识的取值（新版本新增）→ unknown，原文交给展示层透传', () {
      expect(CollabStatus.fromWire('PAUSED'), CollabStatus.unknown);
      expect(
        CollabStatus.fromWire('running'),
        CollabStatus.unknown,
        reason: '大小写不宽容：口径是 daemon 钉死的那套全大写串',
      );
    });

    test('终局判定：completed / failed / cancelled 为终局，其余不是', () {
      expect(CollabStatus.completed.isFinished, isTrue);
      expect(CollabStatus.failed.isFinished, isTrue);
      expect(CollabStatus.cancelled.isFinished, isTrue);
      expect(CollabStatus.running.isFinished, isFalse);
      expect(CollabStatus.waitingApproval.isFinished, isFalse);
      expect(CollabStatus.submitted.isFinished, isFalse);
      expect(CollabStatus.unknown.isFinished, isFalse);
    });
  });

  group('collabStatusWord：七词词表 + 未知降级（禁「未知/错误」裸词）', () {
    test('七个词逐一对位（顺序即生命周期）', () {
      expect(collabStatusWord(CollabStatus.submitted), '已提交');
      expect(collabStatusWord(CollabStatus.accepted), '已受理');
      expect(collabStatusWord(CollabStatus.running), '执行中');
      expect(collabStatusWord(CollabStatus.waitingApproval), '等待确认');
      expect(collabStatusWord(CollabStatus.completed), '已完成');
      expect(collabStatusWord(CollabStatus.failed), '未完成');
      expect(collabStatusWord(CollabStatus.cancelled), '已取消');
    });

    test('unknown 有原文 → 原文照登（认识的中文化，不认识的原样显示）', () {
      expect(collabStatusWord(CollabStatus.unknown, 'PAUSED'), 'PAUSED');
    });

    test('unknown 无原文 → 如实说「电脑端没有给出状态」，不编词', () {
      expect(collabStatusWord(CollabStatus.unknown), '(电脑端没有给出状态)');
      expect(collabStatusWord(CollabStatus.unknown, '   '), '(电脑端没有给出状态)');
    });
  });

  group('collabBadgeKindOf：七词 → 五态徽章（UNKNOWN 不进徽章）', () {
    test('受理/执行归 running；等待确认归 pending', () {
      expect(collabBadgeKindOf(CollabStatus.submitted), PAStatusKind.running);
      expect(collabBadgeKindOf(CollabStatus.accepted), PAStatusKind.running);
      expect(collabBadgeKindOf(CollabStatus.running), PAStatusKind.running);
      expect(
        collabBadgeKindOf(CollabStatus.waitingApproval),
        PAStatusKind.pending,
      );
    });

    test('完成归 done；未完成/已取消归 failed（不美化）', () {
      expect(collabBadgeKindOf(CollabStatus.completed), PAStatusKind.done);
      expect(collabBadgeKindOf(CollabStatus.failed), PAStatusKind.failed);
      expect(collabBadgeKindOf(CollabStatus.cancelled), PAStatusKind.failed);
    });

    test('unknown → null：由调用方走「原文 + 降级说明」', () {
      expect(collabBadgeKindOf(CollabStatus.unknown), isNull);
    });
  });

  group('collabDeriveStatus：会话级状态由成员卡派生（不独立记账）', () {
    test('空名册 → ACCEPTED（与 daemon 的 switch 落空一致）', () {
      expect(
        collabDeriveStatus(const <CollabMemberUi>[]),
        CollabStatus.accepted,
      );
    });

    test('有人在跑 → RUNNING（一位在跑就不是终局）', () {
      expect(
        collabDeriveStatus(<CollabMemberUi>[
          member(status: CollabStatus.completed),
          member(agentId: 'pi', status: CollabStatus.running),
        ]),
        CollabStatus.running,
      );
    });

    test('有人等审批 → WAITING_APPROVAL（一个成员等审批、其他继续跑）', () {
      expect(
        collabDeriveStatus(<CollabMemberUi>[
          member(status: CollabStatus.completed),
          member(agentId: 'pi', status: CollabStatus.waitingApproval),
        ]),
        CollabStatus.waitingApproval,
      );
    });

    test('全员终局按 FAILED > CANCELLED > COMPLETED 聚合', () {
      expect(
        collabDeriveStatus(<CollabMemberUi>[
          member(status: CollabStatus.completed),
          member(agentId: 'pi', status: CollabStatus.failed),
          member(agentId: 'dsh', status: CollabStatus.cancelled),
        ]),
        CollabStatus.failed,
      );
      expect(
        collabDeriveStatus(<CollabMemberUi>[
          member(status: CollabStatus.cancelled),
          member(agentId: 'pi', status: CollabStatus.completed),
        ]),
        CollabStatus.cancelled,
      );
      expect(
        collabDeriveStatus(<CollabMemberUi>[
          member(status: CollabStatus.completed),
          member(agentId: 'pi', status: CollabStatus.completed),
        ]),
        CollabStatus.completed,
      );
    });

    test('UNKNOWN 成员不计入任何桶（与 daemon 的 switch 落空一致）', () {
      expect(
        collabDeriveStatus(<CollabMemberUi>[
          member(status: CollabStatus.unknown),
        ]),
        CollabStatus.completed,
        reason: '只有 UNKNOWN 成员时按「没有可计数的成员」聚合为 COMPLETED',
      );
    });
  });

  group('collabMemberCounts + collabSummaryLine：汇总行（绝不显示笼统的运行中）', () {
    test('六桶计数（UNKNOWN 不进任何桶）', () {
      final CollabSessionCounts c = collabMemberCounts(<CollabMemberUi>[
        member(status: CollabStatus.completed),
        member(agentId: 'pi', status: CollabStatus.completed),
        member(agentId: 'dsh', status: CollabStatus.failed),
        member(agentId: 'zcode', status: CollabStatus.cancelled),
        member(agentId: 'hermes', status: CollabStatus.waitingApproval),
        member(agentId: 'codex2', status: CollabStatus.running),
        member(agentId: 'codex3', status: CollabStatus.accepted),
        member(agentId: 'codex4', status: CollabStatus.submitted),
        member(agentId: 'codex5', status: CollabStatus.unknown),
      ]);
      expect(c.total, 9);
      expect(c.completed, 2);
      expect(c.failed, 1);
      expect(c.cancelled, 1);
      expect(c.waiting, 1);
      expect(c.active, 3);
    });

    test('比例段永远在（0/3 也如实说）', () {
      expect(
        collabSummaryLine(
          collabMemberCounts(<CollabMemberUi>[
            member(status: CollabStatus.running),
            member(agentId: 'pi', status: CollabStatus.accepted),
            member(agentId: 'dsh', status: CollabStatus.submitted),
          ]),
        ),
        '0/3 已完成',
      );
    });

    test('有等待确认 / 有未完成时才追加段', () {
      expect(
        collabSummaryLine(
          collabMemberCounts(<CollabMemberUi>[
            member(status: CollabStatus.completed),
            member(agentId: 'pi', status: CollabStatus.waitingApproval),
          ]),
        ),
        '1/2 已完成 · 1 个等待确认',
      );
      expect(
        collabSummaryLine(
          collabMemberCounts(<CollabMemberUi>[
            member(status: CollabStatus.failed),
            member(agentId: 'pi', status: CollabStatus.cancelled),
          ]),
        ),
        '0/2 已完成 · 2 个未完成',
      );
    });

    test('没有成员 → null，不占位', () {
      expect(
        collabSummaryLine(collabMemberCounts(const <CollabMemberUi>[])),
        isNull,
      );
    });
  });

  group('collabMemberActions：每个禁用按钮旁必有原因（同源）', () {
    test('终局 + 有子线程 → 可重试、不可取消（无需取消）', () {
      final CollabMemberActions a = collabMemberActions(
        member(status: CollabStatus.failed, threadId: 'thread-9'),
      );
      expect(a.canRetry, isTrue);
      expect(a.retryReason, isNull);
      expect(a.canCancel, isFalse);
      expect(a.cancelReason, '该成员已收尾,无需取消');
      expect(a.canOpenSession, isTrue);
    });

    test('未收尾 + 有子线程 → 可取消、不可重试（还没收尾）', () {
      final CollabMemberActions a = collabMemberActions(
        member(status: CollabStatus.running),
      );
      expect(a.canCancel, isTrue);
      expect(a.cancelReason, isNull);
      expect(a.canRetry, isFalse);
      expect(a.retryReason, '该成员还没收尾,不能重试;要重来请先取消它');
    });

    test('没有子线程（受理失败的卡）→ 三入口全禁，原因都是「没有会话线程」', () {
      final CollabMemberActions a = collabMemberActions(
        member(status: CollabStatus.failed, threadId: ''),
      );
      expect(a.canOpenSession, isFalse);
      expect(a.openSessionReason, '该成员还没有自己的会话,受理后才能查看');
      expect(a.canRetry, isFalse);
      expect(a.retryReason, '该成员没有自己的会话线程');
      expect(a.canCancel, isFalse);
      expect(a.cancelReason, '该成员没有自己的会话线程');
    });

    test('等待确认的成员可取消（等的是用户，不是不能停）', () {
      final CollabMemberActions a = collabMemberActions(
        member(status: CollabStatus.waitingApproval),
      );
      expect(a.canCancel, isTrue);
      expect(a.canRetry, isFalse);
    });
  });

  group('collabSessionActions：取消整个协同', () {
    CollabSessionUi sessionWith(List<CollabMemberUi> members) =>
        CollabSessionUi(sessionId: 'cs-1', members: members);

    test('有未收尾成员 → 可取消', () {
      final CollabSessionActions a = collabSessionActions(
        sessionWith(<CollabMemberUi>[member(status: CollabStatus.running)]),
      );
      expect(a.canCancel, isTrue);
      expect(a.cancelReason, isNull);
    });

    test('空名册 → 不可取消且给原因（不出「灰着但不知道为什么」）', () {
      final CollabSessionActions a = collabSessionActions(
        sessionWith(const <CollabMemberUi>[]),
      );
      expect(a.canCancel, isFalse);
      expect(a.cancelReason, '所有成员都已收尾,没有可取消的成员');
    });

    test('全员终局 → 不可取消且给同一原因', () {
      final CollabSessionActions a = collabSessionActions(
        sessionWith(<CollabMemberUi>[member(status: CollabStatus.completed)]),
      );
      expect(a.canCancel, isFalse);
      expect(a.cancelReason, '所有成员都已收尾,没有可取消的成员');
    });
  });

  group('collabDispatchDecision：按钮禁用态与禁用原因同源', () {
    test('零参与者 → 禁用 + 「先勾选至少一位参与者」', () {
      final CollabDispatchDecision d = collabDispatchDecision(
        selectedCount: 0,
        instructionReady: true,
        dispatching: false,
      );
      expect(d.canDispatch, isFalse);
      expect(d.disabledReason, '先勾选至少一位参与者,才能派发');
    });

    test('指令空白 → 禁用 + 「写下要协同完成的任务」', () {
      final CollabDispatchDecision d = collabDispatchDecision(
        selectedCount: 2,
        instructionReady: false,
        dispatching: false,
      );
      expect(d.canDispatch, isFalse);
      expect(d.disabledReason, '写下要协同完成的任务,即可派发');
    });

    test('提交中 → 禁用 + 「正在派发」（防重复提交的 UI 位）', () {
      final CollabDispatchDecision d = collabDispatchDecision(
        selectedCount: 2,
        instructionReady: true,
        dispatching: true,
      );
      expect(d.canDispatch, isFalse);
      expect(d.disabledReason, '正在派发,请稍候');
    });

    test('就绪 → 可派发且原因为空（可派发时不占一行）', () {
      final CollabDispatchDecision d = collabDispatchDecision(
        selectedCount: 1,
        instructionReady: true,
        dispatching: false,
      );
      expect(d.canDispatch, isTrue);
      expect(d.disabledReason, isNull);
    });

    test('原因优先级：零参与者压过指令空白（用户从上往下补条件）', () {
      final CollabDispatchDecision d = collabDispatchDecision(
        selectedCount: 0,
        instructionReady: false,
        dispatching: false,
      );
      expect(d.disabledReason, '先勾选至少一位参与者,才能派发');
    });
  });

  group('collabSelectionSummary + toggleCollabSelection：选人纯函数', () {
    test('选中数摘要：0 人也如实说，不藏数字', () {
      expect(collabSelectionSummary(0), '未选参与者');
      expect(collabSelectionSummary(1), '已选 1 位');
      expect(collabSelectionSummary(3), '已选 3 位');
    });

    test('多选切换加与减对称', () {
      Set<String> s = const <String>{};
      s = toggleCollabSelection(s, 'pi');
      expect(s, <String>{'pi'});
      s = toggleCollabSelection(s, 'codex');
      expect(s, <String>{'pi', 'codex'});
      s = toggleCollabSelection(s, 'pi');
      expect(s, <String>{'codex'});
    });
  });

  group('resolveCollabAgentIds：能力位 → 参与者名单（不写死 agent 名单）', () {
    test('只取 collab: 族，id 取冒号后原文（透传不加工）', () {
      expect(
        resolveCollabAgentIds(<String>[
          'terminal',
          'fs',
          'collab:codex',
          'collab:pi',
        ]),
        <String>['codex', 'pi'],
      );
    });

    test('大小写不敏感的族匹配（对齐 Kotlin ignoreCase）', () {
      expect(resolveCollabAgentIds(<String>['Collab:Codex']), <String>[
        'Codex',
      ]);
    });

    test('去空 + 去重 + 排序（同数据两次渲染同序，不跳动）', () {
      expect(
        resolveCollabAgentIds(<String>[
          'collab:pi',
          'collab:',
          'collab:codex',
          'collab:pi',
          'collab: ',
        ]),
        <String>['codex', 'pi'],
      );
    });

    test('没有 collab 位 → 空名单（页面走无能力位诚实空态）', () {
      expect(resolveCollabAgentIds(<String>['terminal', 'fs', 'git']), isEmpty);
    });

    test('新 agent 接线后自动出现，不需要改客户端代码', () {
      expect(
        resolveCollabAgentIds(<String>['collab:brand-new-agent']),
        <String>['brand-new-agent'],
      );
    });
  });

  group('collabRoleDisplayName：角色显示名回落', () {
    test('name 非空用 name；空回落 id', () {
      expect(collabRoleDisplayName('安全审查者', 'reviewer'), '安全审查者');
      expect(collabRoleDisplayName('', 'reviewer'), 'reviewer');
      expect(
        collabRoleDisplayName('   ', 'reviewer'),
        'reviewer',
        reason: '纯空白按空处理（daemon 侧 trim 同口径）',
      );
    });
  });

  group('CollabCopy 文案表：禁词扫描（AUDIT §3.3）', () {
    /// 文案表全部常量（人话文案层）。
    ///
    /// 【为什么扫常量】「禁显未知/错误裸字样」是文案纪律，靠人眼守不住；把表里
    /// 每条常量过一遍禁词表，回归时新增文案踩线即红。
    const List<String> allCopy = <String>[
      CollabCopy.wordSubmitted,
      CollabCopy.wordAccepted,
      CollabCopy.wordRunning,
      CollabCopy.wordWaitingApproval,
      CollabCopy.wordCompleted,
      CollabCopy.wordFailed,
      CollabCopy.wordCancelled,
      CollabCopy.statusAbsent,
      CollabCopy.unknownStatusNote,
      CollabCopy.disconnectNotice,
      CollabCopy.loadingRestore,
      CollabCopy.emptyNoCapability,
      CollabCopy.emptyNoSelection,
      CollabCopy.loadingAgents,
      CollabCopy.roleNone,
      CollabCopy.reasonNoParticipant,
      CollabCopy.reasonNoInstruction,
      CollabCopy.reasonDispatchInFlight,
      CollabCopy.btnDispatch,
      CollabCopy.btnDispatching,
      CollabCopy.sessionTitleFallback,
      CollabCopy.membersPendingNote,
      CollabCopy.progressSubmittedNote,
      CollabCopy.sectionMemberResults,
      CollabCopy.sectionSummaryResult,
      CollabCopy.memberResultsEmpty,
      CollabCopy.summaryResultNone,
      CollabCopy.diffMergeNotice,
      CollabCopy.viewResult,
      CollabCopy.compareResults,
      CollabCopy.chooseResult,
      CollabCopy.actionRetry,
      CollabCopy.actionCancelMember,
      CollabCopy.actionCancelSession,
      CollabCopy.actionViewReason,
      CollabCopy.actionHideReason,
      CollabCopy.approvalWaitingPrefix,
      CollabCopy.actionOpenSession,
      CollabCopy.confirmCancelSessionTitle,
      CollabCopy.confirmCancelSessionBody,
      CollabCopy.confirmCancelMemberTitle,
      CollabCopy.confirmCancelMemberBody,
      CollabCopy.confirmRetryMemberTitle,
      CollabCopy.confirmRetryMemberBody,
      CollabCopy.confirmOk,
      CollabCopy.confirmCancel,
      CollabCopy.reasonRetryActive,
      CollabCopy.reasonNoThread,
      CollabCopy.reasonCancelFinished,
      CollabCopy.reasonSessionAllFinished,
      CollabCopy.reasonOpenNoThread,
    ];

    test('无一条含「未知」裸词', () {
      for (final String s in allCopy) {
        expect(s.contains('未知'), isFalse, reason: '文案禁「未知」：$s');
      }
    });

    test('无一条含「错误」裸词', () {
      for (final String s in allCopy) {
        expect(s.contains('错误'), isFalse, reason: '文案禁「错误」：$s');
      }
    });

    test('无一条为空（空文案 = 占位假话）', () {
      for (final String s in allCopy) {
        expect(s.trim().isNotEmpty, isTrue);
      }
    });
  });
}
