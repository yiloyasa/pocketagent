// 协同角色纯函数测试（B4 第 4 波）：id 校验 / 显示名回落 / 摘要行拼接 /
// agent 下拉的位提取 / 表单映射 / effort 档位。
//
// 【为什么逐条钉死】这些规则最容易被「顺手改个条件」悄悄反转，而反转后没有
// 任何报错，只能等用户派派失败、看到「未定义的协同角色 xx」才暴露
// （Android CollabRolesLogicTest.kt 的注释原话）。这里照它的三条主线逐条
// 对位，并补 Dart 侧新增的位提取/表单映射用例。
//
// 【口径对账】id 校验三条（空/空白/超 64）与 daemon collab_roles.go:111-124
// 同序同义：文案取 Kotlin 短句；空格判定覆盖 tab/换行（Kotlin 用
// isWhitespace，Dart 用 \s 正则，边界逐条摆）；超长按 **rune 数**（与 Go
// len([]rune(id)) 同口径，中文/emoji 不按字节误杀）。
//
// 全部纯函数，不拖通道、不拖 MaterialApp：JVM 直接跑，不碰真机/daemon。

import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/collab_roles_data.dart';

CollabRole role({
  String id = '',
  String name = '',
  String agentId = '',
  String description = '',
  String instructions = '',
  String model = '',
  String effort = '',
  String workspace = '',
}) =>
    CollabRole(
      id: id,
      name: name,
      agentId: agentId,
      description: description,
      instructions: instructions,
      model: model,
      effort: effort,
      workspace: workspace,
    );

void main() {
  group('validateCollabRoleId：三条客户端先行校验（与 daemon 同口径）', () {
    test('空 id（含全空白）→ 人话「不能为空」', () {
      expect(validateCollabRoleId(''), '协同角色 id 不能为空');
      expect(validateCollabRoleId('   '), '协同角色 id 不能为空');
      expect(validateCollabRoleId('\t\n'), '协同角色 id 不能为空');
    });

    test('含空白（空格/tab/换行）→ 人话「不能含空格」', () {
      expect(validateCollabRoleId('reviewer x'), '协同角色 id 不能含空格');
      expect(validateCollabRoleId('reviewer\tx'), '协同角色 id 不能含空格');
      expect(validateCollabRoleId('reviewer\nx'), '协同角色 id 不能含空格');
      expect(validateCollabRoleId('reviewer\rx'), '协同角色 id 不能含空格');
      // 全角空格也是空白（Kotlin isWhitespace 同口径）
      expect(validateCollabRoleId('reviewer\u3000x'), '协同角色 id 不能含空格');
    });

    test('超长（> 64 rune）→ 人话「过长」；恰好 64 合法（边界）', () {
      expect(
        validateCollabRoleId('a' * (kCollabRoleIdMax + 1)),
        '协同角色 id 过长(上限 $kCollabRoleIdMax)',
      );
      expect(validateCollabRoleId('a' * kCollabRoleIdMax), isNull);
      // 中文按 rune 数计（与 Go len([]rune) 同口径，不按 UTF-16 长度）
      expect(validateCollabRoleId('审' * (kCollabRoleIdMax + 1)),
          '协同角色 id 过长(上限 $kCollabRoleIdMax)');
      expect(validateCollabRoleId('审' * kCollabRoleIdMax), isNull);
    });

    test('合法 id → null（空 = 合法）', () {
      expect(validateCollabRoleId('reviewer'), isNull);
      expect(validateCollabRoleId('sec-review-01'), isNull);
      // 星号/hash 无实际危害，不禁止（daemon 注释原话）
      expect(validateCollabRoleId('role#1*'), isNull);
      // 前后空白被 trim 后判定（daemon 同样先 trim）
      expect(validateCollabRoleId('  reviewer  '), isNull);
    });

    test('优先级：空 > 空白 > 过长（先命中先报，与 daemon 同序）', () {
      // 空且超长 → 报空（第一条）
      expect(validateCollabRoleId('   '), '协同角色 id 不能为空');
      // 含空白且超长 → 报空白（第二条先于长度）
      expect(
        validateCollabRoleId('${'a' * 100} b'),
        '协同角色 id 不能含空格',
      );
    });
  });

  group('collabRoleDisplayName：显示名回落（对位 Kotlin 同款）', () {
    test('name 非空用 name；空/纯空白回落 id', () {
      expect(collabRoleDisplayName(role(id: 'reviewer', name: '安全审查者')),
          '安全审查者');
      expect(collabRoleDisplayName(role(id: 'reviewer', name: '')), 'reviewer');
      expect(collabRoleDisplayName(role(id: 'reviewer', name: '   ')), 'reviewer');
    });

    test('两者都空（理论不发生）→ 回落 id 原文空串，不编「未命名」', () {
      expect(collabRoleDisplayName(role(id: '', name: '')), '');
    });
  });

  group('collabRoleSummaryLine：三段摘要（agent · model · effort）', () {
    test('agent + model + effort 三段全上，中间「·」分隔', () {
      expect(
        collabRoleSummaryLine(
            role(id: 'r', agentId: 'codex', model: 'gpt-5', effort: 'high')),
        'Codex · gpt-5 · high',
      );
    });

    test('只填 model / 只填 effort 各补一段', () {
      expect(
        collabRoleSummaryLine(role(id: 'r', agentId: 'pi', model: 'claude-3')),
        'Pi · claude-3',
      );
      expect(
        collabRoleSummaryLine(role(id: 'r', agentId: 'zcode', effort: 'medium')),
        'ZCode · medium',
      );
    });

    test('model/effort 全空 → 只留 agent 名', () {
      expect(collabRoleSummaryLine(role(id: 'r', agentId: 'codex')), 'Codex');
    });

    test('agent 名为已知 id → 走展示名映射（与全 App 命名一致）', () {
      expect(collabRoleSummaryLine(role(id: 'r', agentId: 'codex')), 'Codex');
      expect(collabRoleSummaryLine(role(id: 'r', agentId: 'opencode')),
          'OpenCode');
      expect(collabRoleSummaryLine(role(id: 'r', agentId: 'hermes')), 'Hermes');
      // 空 agentId 回落 id（不是空串）——展示名映射只作用于真实 agent id
      expect(collabRoleSummaryLine(role(id: 'r', agentId: '')), 'r');
    });

    test('agent 名为未知 id → 原样显示（不透传名单，不自造缩写）', () {
      expect(
        collabRoleSummaryLine(role(id: 'r', agentId: 'myagent', effort: 'low')),
        'myagent · low',
      );
    });

    test('agent 空但 id 非空 → 回落 id 原文（不是空串）', () {
      expect(collabRoleSummaryLine(role(id: 'reviewer')), 'reviewer');
    });

    test('agent 与 id 都空 → 空串（调用方据此不占位、不伪造）', () {
      expect(collabRoleSummaryLine(role(id: '')), '');
    });

    test('纯空白字段视同空（不显示空白段）', () {
      expect(
        collabRoleSummaryLine(
            role(id: 'r', agentId: 'codex', model: '   ', effort: '  ')),
        'Codex',
      );
    });
  });

  group('selectedAgentLabel：展示名映射（未知 id 透传）', () {
    test('五家 + claude/空串各有展示名', () {
      expect(selectedAgentLabel('codex'), 'Codex');
      expect(selectedAgentLabel('pi'), 'Pi');
      expect(selectedAgentLabel('zcode'), 'ZCode');
      expect(selectedAgentLabel('hermes'), 'Hermes');
      expect(selectedAgentLabel('dsh'), 'DSH');
      expect(selectedAgentLabel('opencode'), 'OpenCode');
      expect(selectedAgentLabel('claude'), 'Claude');
      expect(selectedAgentLabel(''), 'Codex');
    });

    test('未知 id 原样返回（不显「未知」）', () {
      expect(selectedAgentLabel('brand-new-agent'), 'brand-new-agent');
    });
  });

  group('resolveCollabAgentIds：collab 位提取（对位 HomeScreen.kt:2216）', () {
    test('只取 collab: 族，去空去重排序', () {
      expect(
        resolveCollabAgentIds(<String>[
          'collab:pi',
          'collab:codex',
          'agent:codex',
          'collab:pi',
          'collab:',
          'collab:  ',
          'collab:zcode',
        ]),
        <String>['codex', 'pi', 'zcode'],
      );
    });

    test('大小写不敏感（Kotlin ignoreCase 同款）', () {
      expect(
        resolveCollabAgentIds(<String>['COLLAB:codex', 'Collab:pi']),
        <String>['codex', 'pi'],
      );
    });

    test('没有 collab 位 → 空列表（不是假名单）', () {
      expect(resolveCollabAgentIds(<String>['agent:codex', 'fs']), isEmpty);
      expect(resolveCollabAgentIds(const <String>[]), isEmpty);
    });
  });

  group('collabEditorAgentCandidates：编辑页下拉候选（诚实降级）', () {
    test('collab 位优先 + agent 位补充，合并去重排序', () {
      expect(
        collabEditorAgentCandidates(<String>[
          'collab:pi',
          'agent:codex',
          'agent:pi',
          'collab:codex',
        ]),
        <String>['codex', 'pi'],
      );
    });

    test('只有 agent 位 → 候选就是 agent 位集合', () {
      expect(
        collabEditorAgentCandidates(<String>['agent:codex', 'agent:zcode']),
        <String>['codex', 'zcode'],
      );
    });

    test('一位都没有 → 空候选（页面走手填降级，不编五家名单）', () {
      expect(collabEditorAgentCandidates(<String>['fs', 'git']), isEmpty);
      expect(collabEditorAgentCandidates(const <String>[]), isEmpty);
    });
  });

  group('collabEditorDefaultAgentId：下拉默认值（Kotlin 同款优先级）', () {
    test('collab 第一家 > agent 候选第一家 > 空串', () {
      expect(
        collabEditorDefaultAgentId(
            <String>['agent:codex', 'collab:pi', 'collab:codex']),
        'codex',
        reason: 'collab 位排序后第一家（codex < pi）',
      );
      expect(
        collabEditorDefaultAgentId(<String>['agent:zcode', 'agent:codex']),
        'codex',
        reason: '没有 collab 位 → 候选排序后第一家',
      );
      expect(collabEditorDefaultAgentId(<String>['fs']), '');
    });
  });

  group('collabRoleFromForm：表单 → proto（trim 口径对位 daemon）', () {
    test('七个字段 trim；instructions 刻意不 trim', () {
      final CollabRole r = collabRoleFromForm(
        id: '  reviewer  ',
        name: '  安全审查者  ',
        agentId: '  codex  ',
        description: '  只找安全问题  ',
        // 行首空格是用户有意的排版（daemon protoToCollabRole 同款例外）
        instructions: '  必须列证据\n  - 第一项',
        model: '  codex/gpt-5  ',
        effort: '  high  ',
        workspace: '  G:/repo  ',
      );
      expect(r.id, 'reviewer');
      expect(r.name, '安全审查者');
      expect(r.agentId, 'codex');
      expect(r.description, '只找安全问题');
      expect(r.instructions, '  必须列证据\n  - 第一项');
      expect(r.model, 'codex/gpt-5');
      expect(r.effort, 'high');
      expect(r.workspace, 'G:/repo');
    });

    test('空表单 → 全空角色（daemon 会以 id 空拒绝，客户端先校验）', () {
      final CollabRole r = collabRoleFromForm(
        id: '',
        name: '',
        agentId: '',
        description: '',
        instructions: '',
        model: '',
        effort: '',
        workspace: '',
      );
      expect(r.id, isEmpty);
      expect(r.instructions, isEmpty);
      expect(validateCollabRoleId(r.id), isNotNull,
          reason: '空 id 过不了客户端校验，保存按钮因此禁用');
    });
  });

  group('collabEffortLabel：五档 + 表外透传', () {
    test('五档各有中文名；空 = 跟随默认', () {
      expect(collabEffortLabel(''), '跟随默认');
      expect(collabEffortLabel('low'), '低');
      expect(collabEffortLabel('medium'), '中');
      expect(collabEffortLabel('high'), '高');
      expect(collabEffortLabel('xhigh'), '极高');
    });

    test('表外值原样透传（不显「未知」）', () {
      expect(collabEffortLabel('max'), 'max');
    });

    test('档位表恰好五档（与 Android effortOptions 同款）', () {
      expect(kCollabEffortOptions, <String>['', 'low', 'medium', 'high', 'xhigh']);
    });
  });
}
