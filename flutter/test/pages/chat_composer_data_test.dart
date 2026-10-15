// Chat 输入区纯函数测试（B3 第 5 波③）：词元 / 过滤 / 合并 / 可见性 / 回填 /
// 校验 / 附件类型 / 门控 —— 逐条对位 Android `SlashCommandsLogic.kt` +
// `attachmentKind` + `attachmentEntryHiddenFor`。

import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_composer_data.dart';

List<SlashCommandInfo> _daemonCmds() => const <SlashCommandInfo>[
      SlashCommandInfo(
        name: 'goal',
        description: '常驻目标',
        inputHint: '/goal [pause|resume|…]',
      ),
      SlashCommandInfo(
        name: 'compact',
        description: '历史压缩',
      ),
      SlashCommandInfo(
        name: 'init',
        description: '生成项目说明',
        inputHint: '/init',
      ),
    ];

const List<String> _capsAll = <String>[
  'compact:codex',
  'compact:pi',
  'compact:zcode',
  'review:codex',
  'slash:codex',
  'slash:pi',
  'slash:zcode',
];

void main() {
  group('slashTokenOf：第一个空白前的前缀且必须带斜杠', () {
    test('基本词元', () {
      expect(slashTokenOf('/go 修复登录'), '/go');
      expect(slashTokenOf('/goal'), '/goal');
      expect(slashTokenOf('/g\n下一行'), '/g');
      expect(slashTokenOf('/'), '/', reason: '裸斜杠也算词元');
      expect(slashTokenOf('/目'), '/目', reason: '中文伪命令词元');
      expect(slashTokenOf('普通输入'), isNull);
      expect(slashTokenOf(''), isNull);
    });
  });

  group('filterSlashCandidates：大小写不敏感前缀 + 空退全量', () {
    test('前缀匹配（大小写不敏感）', () {
      final List<String> names = filterSlashCandidates(_daemonCmds(), '/GO')
          .map((SlashCommandInfo c) => c.name)
          .toList();
      expect(names, <String>['goal']);
    });

    test('裸斜杠给全量', () {
      expect(filterSlashCandidates(_daemonCmds(), '/').length, 3);
    });

    test('词元后已敲空白 → 空（在敲参数，面板收起）', () {
      expect(filterSlashCandidates(_daemonCmds(), '/goal '), isEmpty);
    });

    test('无前缀命中 → 退回全量（宽进）', () {
      expect(filterSlashCandidates(_daemonCmds(), '/zzz').length, 3);
    });

    test('候选全集为空 → 空', () {
      expect(
        filterSlashCandidates(const <SlashCommandInfo>[], '/'),
        isEmpty,
      );
    });

    test('非斜杠输入 → 空', () {
      expect(filterSlashCandidates(_daemonCmds(), '修 bug'), isEmpty);
    });
  });

  group('门控：agentCapableOf / slashPanelAgentOf', () {
    test('大小写/空白不敏感，空 agent 一律 false', () {
      expect(agentCapableOf('slash', 'codex', _capsAll), isTrue);
      expect(agentCapableOf('slash', 'Codex', _capsAll), isTrue);
      expect(agentCapableOf('slash', ' codex ', _capsAll), isTrue);
      expect(agentCapableOf('slash', null, _capsAll), isFalse);
      expect(agentCapableOf('slash', '', _capsAll), isFalse);
      expect(agentCapableOf('slash', '  ', _capsAll), isFalse);
      expect(agentCapableOf('', 'codex', _capsAll), isFalse);
    });

    test('只认绑定同一 agent 的位', () {
      expect(agentCapableOf('review', 'codex', _capsAll), isTrue);
      expect(agentCapableOf('review', 'pi', _capsAll), isFalse);
    });

    test('位优先 + 三源回落', () {
      // 位在：名单外新家靠位点亮。
      expect(
        slashPanelAgentOf('hermes', <String>[..._capsAll, 'slash:hermes']),
        isTrue,
      );
      // 位不在：名单内回落仍可用（旧 daemon 兼容垫）。
      expect(slashPanelAgentOf('zcode', const <String>[]), isTrue);
      expect(slashPanelAgentOf('codex', const <String>[]), isTrue);
      expect(slashPanelAgentOf('pi', const <String>[]), isTrue);
      // 位与名单都不在：否。
      expect(slashPanelAgentOf('hermes', const <String>[]), isFalse);
      expect(slashPanelAgentOf(null, _capsAll), isFalse);
      expect(slashPanelAgentOf('', _capsAll), isFalse);
    });
  });

  group('builtinSlashCommandsFor：伪命令随位点亮', () {
    test('codex 全位：三伪命令都在', () {
      expect(
        builtinSlashCommandsFor('codex', _capsAll)
            .map((SlashCommandInfo c) => c.name),
        <String>['目标', '压缩', '计划'],
      );
    });

    test('pi：无 /目标（落库跟随也不展示）', () {
      final List<String> names = builtinSlashCommandsFor(
        'pi',
        <String>['compact:pi', 'slash:pi'],
      ).map((SlashCommandInfo c) => c.name).toList();
      expect(names, isNot(contains('目标')));
      expect(names, contains('压缩'));
    });

    test('缺位：/压缩 /计划绝不进列表', () {
      expect(
        builtinSlashCommandsFor('codex', const <String>['slash:codex'])
            .map((SlashCommandInfo c) => c.name),
        <String>['目标'],
      );
    });
  });

  group('slashCandidatesFor：内置置顶 + daemon 重名让位', () {
    test('内置置顶，daemon 命令随后', () {
      // 注意：_daemonCmds 的 goal/compact 与内置 /目标 /压缩原生别名重名
      // （重名让位，见下条用例），置顶断言用无碰撞的 daemon 表。
      final List<SlashCommandInfo> native = const <SlashCommandInfo>[
        SlashCommandInfo(name: 'init', description: '生成项目说明'),
        SlashCommandInfo(name: 'deploy', description: '部署'),
      ];
      final List<String> names = slashCandidatesFor(
        'codex',
        native,
        _capsAll,
      ).map((SlashCommandInfo c) => c.name).toList();
      expect(names.take(3), <String>['目标', '压缩', '计划']);
      expect(names, containsAll(<String>['init', 'deploy']));
    });

    test('daemon 同名命令让内置让位（goal 已在 daemon 中）', () {
      final List<String> names = slashCandidatesFor(
        'codex',
        _daemonCmds(),
        const <String>['slash:codex'],
      ).map((SlashCommandInfo c) => c.name).toList();
      // /目标 的原生别名 goal 与 daemon 重名 → 内置 /目标 不进列表。
      expect(names, isNot(contains('目标')));
      expect(names, contains('goal'));
    });

    test('非面板会话：直接空（无伪命令兜底）', () {
      expect(
        slashCandidatesFor(
          'hermes',
          _daemonCmds(),
          const <String>[],
        ),
        isEmpty,
      );
    });
  });

  group('slashPanelVisible / slashUnknownAgentHintVisible：互斥', () {
    test('面板会话 + 斜杠词元 + 在线 → 可见', () {
      expect(
        slashPanelVisible('zcode', '/g', _daemonCmds(), false, _capsAll),
        isTrue,
      );
    });

    test('离线 / 非斜杠 / 空候选 → 不可见', () {
      expect(
        slashPanelVisible('zcode', '/g', _daemonCmds(), true, _capsAll),
        isFalse,
      );
      expect(
        slashPanelVisible('zcode', '修 bug', _daemonCmds(), false, _capsAll),
        isFalse,
      );
      expect(
        slashPanelVisible(
          'hermes',
          '/',
          const <SlashCommandInfo>[],
          false,
          const <String>[],
        ),
        isFalse,
      );
    });

    test('未知助手敲斜杠 → 空态文案分支（与面板互斥）', () {
      expect(
        slashUnknownAgentHintVisible(
          'hermes',
          '/',
          false,
          const <String>[],
        ),
        isTrue,
      );
      expect(kSlashUnknownAgentHint, '该助手暂无可用命令');
      // 位在的新家走正常面板，不进空态。
      expect(
        slashUnknownAgentHintVisible(
          'hermes',
          '/',
          false,
          const <String>['slash:hermes'],
        ),
        isFalse,
      );
      // 加载中（agent 空）与离线不提示。
      expect(
        slashUnknownAgentHintVisible(null, '/', false, const <String>[]),
        isFalse,
      );
      expect(
        slashUnknownAgentHintVisible(
          'hermes',
          '/',
          true,
          const <String>[],
        ),
        isFalse,
      );
    });
  });

  group('applySlashSelection / applySlashPick / stripSlashToken', () {
    test('替换词元并补空格，保留后续文字', () {
      expect(
        applySlashSelection('/go 修复登录', 'goal'),
        '/goal 修复登录',
      );
    });

    test('词元后已有空白不再重复补空格', () {
      expect(applySlashSelection('/go\n下一行', 'goal'), '/goal\n下一行');
    });

    test('无词元 no-op', () {
      expect(applySlashSelection('普通输入', 'goal'), '普通输入');
    });

    test('codex 技能填自然语言', () {
      expect(
        applySlashPick(
          '/mm',
          const SlashCommandInfo(name: 'mmx-cli', description: '脚手架'),
          'codex',
        ),
        '请使用 mmx-cli 技能：',
      );
    });

    test('codex 带 inputHint 的命令走模板回填（将来口径）', () {
      expect(
        applySlashPick(
          '/in',
          const SlashCommandInfo(name: 'init', inputHint: '/init'),
          'codex',
        ),
        '/init ',
      );
    });

    test('zcode/pi 走 /命令 回填', () {
      expect(
        applySlashPick('/in', const SlashCommandInfo(name: 'init'), 'pi'),
        '/init ',
      );
    });

    test('内置伪命令在回填入口 no-op（执行动作由调用方分流）', () {
      expect(
        applySlashPick(
          '/目',
          const SlashCommandInfo(name: '目标', action: SlashPickAction.goal),
          'codex',
        ),
        '/目',
      );
    });

    test('stripSlashToken 只摘词元、保留后续', () {
      expect(stripSlashToken('/目 修复登录'), '修复登录');
      expect(stripSlashToken('普通输入'), '普通输入');
    });
  });

  group('validateComposerInput：空 / 超长', () {
    test('空与纯空白拒收', () {
      expect(validateComposerInput(''), '输入不能为空');
      expect(validateComposerInput('   \n '), '输入不能为空');
    });

    test('超长给分段提示（不静默截断）', () {
      final String long = 'a' * (kComposerMaxLength + 1);
      expect(
        validateComposerInput(long),
        contains('请分段发送'),
      );
    });

    test('正常输入放行（含边界值）', () {
      expect(validateComposerInput('修登录页'), isNull);
      expect(validateComposerInput('a' * kComposerMaxLength), isNull);
    });
  });

  group('attachmentKindOf：图片 vs 文本 vs 不支持', () {
    test('mime 优先认图片', () {
      expect(
        attachmentKindOf(name: 'x.bin', mediaType: 'image/png'),
        ComposerAttachmentKind.image,
      );
    });

    test('扩展名认图片', () {
      for (final String ext in <String>[
        'png',
        'jpg',
        'jpeg',
        'gif',
        'webp',
        'bmp',
        'heic',
      ]) {
        expect(
          attachmentKindOf(name: 'a.$ext', mediaType: ''),
          ComposerAttachmentKind.image,
          reason: 'ext=$ext',
        );
      }
      // 大写扩展名同样命中。
      expect(
        attachmentKindOf(name: 'A.JPG', mediaType: ''),
        ComposerAttachmentKind.image,
      );
    });

    test('文本 mime 与文本扩展名认文本', () {
      expect(
        attachmentKindOf(name: 'a.txt', mediaType: ''),
        ComposerAttachmentKind.text,
      );
      expect(
        attachmentKindOf(name: 'main.dart', mediaType: ''),
        ComposerAttachmentKind.text,
      );
      expect(
        attachmentKindOf(name: 'nodot', mediaType: 'text/plain'),
        ComposerAttachmentKind.text,
      );
      expect(
        attachmentKindOf(name: 'cfg', mediaType: 'application/json'),
        ComposerAttachmentKind.text,
      );
    });

    test('压缩包 / 二进制 / 未知一律不支持', () {
      expect(
        attachmentKindOf(name: 'a.zip', mediaType: 'application/zip'),
        ComposerAttachmentKind.unsupported,
      );
      expect(
        attachmentKindOf(name: 'a.pdf', mediaType: 'application/pdf'),
        ComposerAttachmentKind.unsupported,
      );
      expect(
        attachmentKindOf(name: 'nodot', mediaType: ''),
        ComposerAttachmentKind.unsupported,
      );
    });
  });

  group('composerImageEntryVisible：image_input 位门控', () {
    test('有位才显示图片入口', () {
      expect(
        composerImageEntryVisible('codex', const <String>['image_input:codex']),
        isTrue,
      );
      expect(
        composerImageEntryVisible('codex', const <String>[]),
        isFalse,
        reason: 'dsh 无位：发图必败，不显示入口',
      );
      expect(
        composerImageEntryVisible('codex', const <String>['image_input:pi']),
        isFalse,
        reason: '别家的位不开我家的门',
      );
      expect(
        composerImageEntryVisible(null, const <String>['image_input:codex']),
        isFalse,
      );
      expect(composerImageEntryVisible('', const <String>[]), isFalse);
    });
  });

  group('slashInfoFromProto：daemon 命令恒为 FILL', () {
    test('三字段原样透传', () {
      final SlashCommandInfo info = slashInfoFromProto(
        SlashCommand(name: 'init', description: '说明', inputHint: '/init'),
      );
      expect(info.name, 'init');
      expect(info.description, '说明');
      expect(info.inputHint, '/init');
      expect(info.action, SlashPickAction.fill);
    });
  });
}
