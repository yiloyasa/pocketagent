// Chat 输入区 widget 测试（B3 第 5 波③）：空禁用 / 附件门控两条 / slash
// 面板开合与回填 / 内置伪命令只转发 / 语音占位文案 / 模型行展示与出口。
//
// 【mock 的是宿主 wire 行为】斜杠拉取走真实 `ChatInputTransport`（独立测试
// 通道 `test/composer_slash`），返回字节由 gen stub 现场组帧；发送/拾取/
// 模型切换全是回调出口断言（C4 接线点），本文件不碰 TurnSession。

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/chat_composer.dart';
import 'package:pocketagent_ui/pages/chat_composer_data.dart';
import 'package:pocketagent_ui/pages/chat_input_transport.dart';

const MethodChannel _slashChannel = MethodChannel('test/composer_slash');

void _mockSlash(List<SlashCommand> commands) {
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(_slashChannel, (MethodCall call) async {
    return ListSlashCommandsResponse(commands: commands).writeToBuffer();
  });
}

Future<void> _pumpComposer(
  WidgetTester tester, {
  String? agentId = 'codex',
  List<String> capabilities = const <String>['slash:codex'],
  String modelName = '',
  String effortLabel = '默认',
  List<ChatPendingAttachment> attachments =
      const <ChatPendingAttachment>[],
  ValueChanged<String>? onSubmit,
  ValueChanged<SlashPick>? onSlashPick,
  VoidCallback? onModelTap,
  VoidCallback? onPickTextFile,
  VoidCallback? onPickImage,
  ValueChanged<int>? onRemoveAttachment,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: ChatComposer(
          threadId: 'thread-7',
          agentId: agentId,
          capabilities: capabilities,
          modelName: modelName,
          effortLabel: effortLabel,
          attachments: attachments,
          slashTransport: ChatInputTransport(channel: _slashChannel),
          onSubmit: onSubmit ?? (_) {},
          onSlashPick: onSlashPick,
          onModelTap: onModelTap,
          onPickTextFile: onPickTextFile,
          onPickImage: onPickImage,
          onRemoveAttachment: onRemoveAttachment,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('发送键：空禁用 / 有字可用 / 提交清输入框', () {
    testWidgets('空输入禁用，有字才可用', (WidgetTester tester) async {
      await _pumpComposer(tester);
      expect(
        tester.widget<IconButton>(find.byKey(const ValueKey<String>('composer-send'))).onPressed,
        isNull,
        reason: '空输入禁用（validateComposerInput 非空即不可发）',
      );

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '修登录页',
      );
      await tester.pumpAndSettle();
      expect(
        tester.widget<IconButton>(find.byKey(const ValueKey<String>('composer-send'))).onPressed,
        isNotNull,
      );
    });

    testWidgets('提交交出原文并清空输入框', (WidgetTester tester) async {
      String? got;
      await _pumpComposer(tester, onSubmit: (String text) => got = text);
      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '修登录页',
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey<String>('composer-send')));
      await tester.pumpAndSettle();
      expect(got, '修登录页');
      expect(find.text('修登录页'), findsNothing);
    });

    testWidgets('纯空白不提交（tap 也无回调）', (WidgetTester tester) async {
      bool called = false;
      await _pumpComposer(
        tester,
        onSubmit: (_) => called = true,
      );
      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '   ',
      );
      await tester.pumpAndSettle();
      expect(
        tester.widget<IconButton>(find.byKey(const ValueKey<String>('composer-send'))).onPressed,
        isNull,
      );
      expect(called, isFalse);
    });
  });

  group('附件门控：无位不显示图片入口（文本入口不受影响）', () {
    testWidgets('无 image_input 位：只有文本附件入口', (WidgetTester tester) async {
      await _pumpComposer(
        tester,
        capabilities: const <String>['slash:codex'],
        onPickTextFile: () {},
        onPickImage: () {},
      );
      expect(
        find.byKey(const ValueKey<String>('composer-attach-file')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey<String>('composer-attach-image')),
        findsNothing,
        reason: 'dsh 式无位会话：发图必败，不显示图片入口',
      );
    });

    testWidgets('有 image_input:codex 位：图片入口出现', (WidgetTester tester) async {
      bool imageTapped = false;
      await _pumpComposer(
        tester,
        capabilities: const <String>[
          'slash:codex',
          'image_input:codex',
        ],
        onPickTextFile: () {},
        onPickImage: () => imageTapped = true,
      );
      expect(
        find.byKey(const ValueKey<String>('composer-attach-image')),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const ValueKey<String>('composer-attach-image')));
      await tester.pumpAndSettle();
      expect(imageTapped, isTrue);
    });

    testWidgets('待发送 chips 展示 + 移除出口带下标', (WidgetTester tester) async {
      int? removed;
      await _pumpComposer(
        tester,
        attachments: const <ChatPendingAttachment>[
          ChatPendingAttachment(name: 'a.png', mediaType: 'image/png'),
          ChatPendingAttachment(name: 'note.txt', mediaType: 'text/plain'),
        ],
        onRemoveAttachment: (int i) => removed = i,
      );
      expect(find.text('a.png'), findsOneWidget);
      expect(find.text('note.txt'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey<String>('composer-attach-remove-0')));
      await tester.pumpAndSettle();
      expect(removed, 0);
    });
  });

  group('斜杠面板：/ 触发拉候选 → 点选回填', () {
    testWidgets('daemon 命令进面板，点选回填 /命令', (WidgetTester tester) async {
      // inputHint 非空 → zcode/pi 式模板回填（codex 空 inputHint 技能走自然
      // 语言，见 data 层用例；此处 agent 恰为 codex，故显式带 hint）。
      _mockSlash(<SlashCommand>[
        SlashCommand(
          name: 'init',
          description: '生成项目说明',
          inputHint: '/init',
        ),
      ]);
      SlashPick? picked;
      await _pumpComposer(tester, onSlashPick: (SlashPick p) => picked = p);

      // 非斜杠输入：面板不开。
      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '修登录页',
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey<String>('composer-slash-panel')),
        findsNothing,
      );

      // 敲 /：拉候选 → 面板开（含 daemon 命令）。
      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '/',
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey<String>('composer-slash-panel')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey<String>('composer-slash-init')),
        findsOneWidget,
      );

      // 点选：回填 + 转发 FILL 事件。
      await tester.tap(find.byKey(const ValueKey<String>('composer-slash-init')));
      await tester.pumpAndSettle();
      expect(find.text('/init '), findsOneWidget);
      expect(picked, const SlashPick(name: 'init', action: SlashPickAction.fill, agentId: 'codex'));
    });

    testWidgets('内置伪命令只转发不执行（动作留 C4），词元先摘掉', (WidgetTester tester) async {
      _mockSlash(const <SlashCommand>[]);
      SlashPick? picked;
      await _pumpComposer(tester, onSlashPick: (SlashPick p) => picked = p);

      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '/目',
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey<String>('composer-slash-目标')),
        findsOneWidget,
        reason: 'codex 无需位即有 /目标兜底',
      );

      await tester.tap(find.byKey(const ValueKey<String>('composer-slash-目标')));
      await tester.pumpAndSettle();
      expect(picked, const SlashPick(name: '目标', action: SlashPickAction.goal, agentId: 'codex'));
      expect(find.text('/目'), findsNothing, reason: '唤起词元已摘，不残留');
    });

    testWidgets('未知助手敲 /：人话空态，不开空面板', (WidgetTester tester) async {
      await _pumpComposer(
        tester,
        agentId: 'hermes',
        capabilities: const <String>[],
      );
      await tester.enterText(
        find.byKey(const ValueKey<String>('composer-input')),
        '/',
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey<String>('composer-slash-panel')),
        findsNothing,
      );
      expect(find.text('该助手暂无可用命令'), findsOneWidget);
    });
  });

  group('语音：诚实占位（无假录音键）', () {
    testWidgets('显示随后续波次文案，且无录音按钮', (WidgetTester tester) async {
      await _pumpComposer(tester);
      expect(find.text('语音输入随后续波次'), findsOneWidget);
      expect(
        find.widgetWithIcon(IconButton, Icons.mic_none_rounded),
        findsNothing,
        reason: '占位行用纯 Icon + 文案，不摆可点的假录音键',
      );
    });
  });

  group('模型行：展示 + 切换回调出口（无回调不摆假选择器）', () {
    testWidgets('空模型名如实展示跟随桌面', (WidgetTester tester) async {
      await _pumpComposer(tester);
      expect(find.text('模型：跟随桌面'), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('composer-model-tap')),
        findsNothing,
        reason: '无 onModelTap：整行不可点，不摆假 chevron 入口',
      );
    });

    testWidgets('模型 · 强度展示 + 点击走出口', (WidgetTester tester) async {
      bool tapped = false;
      await _pumpComposer(
        tester,
        modelName: 'codex/gpt-5.4',
        effortLabel: '高',
        onModelTap: () => tapped = true,
      );
      expect(find.text('codex/gpt-5.4 · 高'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey<String>('composer-model-tap')));
      await tester.pumpAndSettle();
      expect(tapped, isTrue);
    });
  });
}
