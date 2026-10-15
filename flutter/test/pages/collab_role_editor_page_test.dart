// CollabRoleEditor 编辑页测试（B4 第 4 波）：八字段表单 + id 实时校验（仅
// 新建）+ 保存成功/失败路径 + agent 下拉位提取 + 无位数据诚实降级。
//
// 【id 只读校验的分寸】编辑态 id 是只读引用键：**不校验、不报错**（它必已
// 合法），但保存仍走 store → daemon 的同口径校验；daemon 若因并发边界返回
// InvalidArgument，原文经 saveError 如实显示（第一道闸门不是吞错误的借口）。
// 新建态 id 实时校验：空/含空白/超 64 三条各钉一条，保存按钮随之禁用。
//
// 【保存成功 = onBack】Kotlin `onBack()` 同款：成功后回列表页（store 已用
// 落库真值刷新列表），页面自身没有「保存成功了」的中间态。
//
// 【agent 下拉的位提取】候选 = `collab:<id>` 优先 + `agent:<id>` 补充
// （collabEditorAgentCandidates，纯函数单测在 collab_roles_data_test.dart）；
// 这里验页面**真的用位数据装配下拉**，以及没有位时降级成手填（不编五家名单）。

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_button.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/collab_role_editor.dart';
import 'package:pocketagent_ui/pages/collab_roles_state.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

CollabRole demoRole({
  String id = 'reviewer',
  String name = '安全审查者',
  String agentId = 'codex',
  String description = '只找安全问题',
  String instructions = '必须列证据',
  String model = 'codex/gpt-5',
  String effort = 'high',
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
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  /// 宿主 wire 行为：listCollabRoles 给 roles（编辑器进页预填的来源），
  /// setCollabRole 按 [saveError] 决定成败，其余方法未注册。
  void mockHost({
    List<CollabRole> roles = const <CollabRole>[],
    Object? saveError,
  }) {
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      switch (call.method) {
        case 'listCollabRoles':
          return ListCollabRolesResponse(roles: roles).writeToBuffer();
        case 'setCollabRole':
          if (saveError != null) throw saveError;
          final SetCollabRoleRequest req =
              SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
          return SetCollabRoleResponse(role: req.role).writeToBuffer();
        default:
          throw MissingPluginException(
              'No implementation found for method ${call.method}');
      }
    });
  }

  /// 预载 store（编辑器从 store 的 roles 取已存在的角色做预填——Kotlin 同款
  /// 「列表页已加载过，这里一定找得到」）。
  Future<CollabRolesStore> loadedStore(List<CollabRole> roles) async {
    final CollabRolesStore store = CollabRolesStore();
    store.load();
    await store.loadCompleted;
    return store;
  }

  Future<void> pumpEditor(
    WidgetTester tester, {
    required String roleId,
    CollabRolesStore? store,
    List<String>? capabilities,
    VoidCallback? onBack,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: CollabRoleEditorPage(
            roleId: roleId,
            store: store,
            capabilities: capabilities,
            onBack: onBack ?? () {},
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  /// 滚到可见再点：表单一屏放不下，save/effort/chip 在折叠线以下（真实设备
  /// 上用户也要滑到它）。这是测试机制，不断言本身——断言原文一律不动。
  Future<void> tapKey(WidgetTester tester, String key) async {
    // 先收键盘再滚：输入中 TextField 持焦 → 键盘 viewInsets 让 Scaffold
    // 视口变形，滚动量与命中点都跟着漂（中段 chip 会被漂出屏幕）。
    FocusManager.instance.primaryFocus?.unfocus();
    await tester.pumpAndSettle();
    final Finder target = find.byKey(ValueKey<String>(key));
    await tester.ensureVisible(target);
    await tester.pumpAndSettle();
    await tester.tap(target);
    await tester.pumpAndSettle();
  }

  /// 同上的输入版：折叠线以下的字段先滚到可见再 enterText。
  Future<void> enterTextKey(
      WidgetTester tester, String key, String text) async {
    await tester.ensureVisible(find.byKey(ValueKey<String>(key)));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(ValueKey<String>(key)), text);
    await tester.pumpAndSettle();
  }

  group('新建态：id 实时校验（仅新建才校验）', () {
    testWidgets('空 id → 输入框下红字 + 保存按钮禁用', (tester) async {
      mockHost();

      await pumpEditor(tester, roleId: '', capabilities: const <String>['collab:codex']);

      expect(find.text('协同角色 id 不能为空'), findsWidgets);
      expect(find.byKey(const ValueKey<String>('collab-role-id-inline-error')),
          findsOneWidget);
      expect(
        tester.widget<PAButton>(find.byKey(const ValueKey<String>('collab-role-save'))).onPressed,
        isNull,
        reason: 'id 非法时保存直接禁用（客户端第一道闸门）',
      );
    });

    testWidgets('含空白 id → 人话「不能含空格」（tab 也算）', (tester) async {
      mockHost();

      await pumpEditor(tester, roleId: '', capabilities: const <String>['collab:codex']);
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'my\trole');
      await tester.pumpAndSettle();

      expect(find.text('协同角色 id 不能含空格'), findsWidgets);
    });

    testWidgets('合法 id → 红字消失、保存可用', (tester) async {
      mockHost();

      await pumpEditor(tester, roleId: '', capabilities: const <String>['collab:codex']);
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'sec-review');
      await tester.pumpAndSettle();

      expect(find.text('协同角色 id 不能为空'), findsNothing);
      expect(find.text('协同角色 id 不能含空格'), findsNothing);
      expect(
        tester.widget<PAButton>(find.byKey(const ValueKey<String>('collab-role-save'))).onPressed,
        isNotNull,
      );
    });
  });

  group('保存路径', () {
    testWidgets('保存成功 → onBack（页面没有中间成功态）', (tester) async {
      mockHost();
      int backCalls = 0;

      await pumpEditor(
        tester,
        roleId: '',
        onBack: () => backCalls += 1,
        capabilities: const <String>['collab:codex'],
      );
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'reviewer');
      await tester.pumpAndSettle();
      await tapKey(tester, 'collab-role-save');

      expect(backCalls, 1, reason: 'Kotlin 保存成功后 onBack()');
      expect(find.textContaining('保存失败'), findsNothing);
    });

    testWidgets('保存失败 → daemon 原文进可关闭错误条，页面不退出', (tester) async {
      mockHost(
        saveError: PlatformException(
          code: 'INVALID_ARGUMENT',
          message: '协同角色 id 过长(65 字,上限 64):id 会被写进委派工具的参数里',
        ),
      );
      int backCalls = 0;

      await pumpEditor(
        tester,
        roleId: '',
        onBack: () => backCalls += 1,
        capabilities: const <String>['collab:codex'],
      );
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'reviewer');
      await tester.pumpAndSettle();
      await tapKey(tester, 'collab-role-save');

      expect(find.textContaining('协同角色 id 过长'), findsOneWidget,
          reason: 'daemon 还原原因原文透传，不泛化成「操作失败」');
      expect(backCalls, 0, reason: '失败不退出');
      expect(find.byKey(const ValueKey<String>('collab-role-id')), findsOneWidget);

      // 错误条可关闭（保存后视图已滚到底部，关闭键在顶部，走 tapKey 回滚）
      await tapKey(tester, 'collab-role-save-error-close');
      expect(find.textContaining('协同角色 id 过长'), findsNothing);
    });

    testWidgets('宿主未注册（B4 前）→ 保存失败文案点名 B4 波次（不假装成功）',
        (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });
      int backCalls = 0;

      await pumpEditor(
        tester,
        roleId: '',
        onBack: () => backCalls += 1,
        capabilities: const <String>['collab:codex'],
      );
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'reviewer');
      await tester.pumpAndSettle();
      await tapKey(tester, 'collab-role-save');

      expect(find.textContaining('未就绪（B4 波次）'), findsOneWidget);
      expect(backCalls, 0);
    });

    testWidgets('表单 trim 口径过桥：id/name/agentId trim，instructions 不 trim',
        (tester) async {
      SetCollabRoleRequest? seen;
      mockHost();
      // 手动接管 setCollabRole 的请求记录（mockHost 已回显，这里只加观测）
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        switch (call.method) {
          case 'listCollabRoles':
            return ListCollabRolesResponse().writeToBuffer();
          case 'setCollabRole':
            seen = SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
            return SetCollabRoleResponse(role: seen!.role).writeToBuffer();
          default:
            throw MissingPluginException('No implementation found');
        }
      });

      await pumpEditor(tester, roleId: '', capabilities: const <String>['collab:codex']);
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), '  reviewer  ');
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-name')), '  安全审查者  ');
      await enterTextKey(
          tester, 'collab-role-instructions', '  必须列证据\n  - 第一项');
      await tapKey(tester, 'collab-role-save');

      expect(seen!.role.id, 'reviewer', reason: 'id trim 后过桥');
      expect(seen!.role.name, '安全审查者');
      expect(seen!.role.agentId, 'codex', reason: 'agent 下拉默认取 collab 第一家');
      expect(seen!.role.instructions, '  必须列证据\n  - 第一项',
          reason: 'instructions 刻意不 trim（daemon protoToCollabRole 同款例外）');
    });
  });

  group('编辑态：预填 + id 只读', () {
    testWidgets('编辑态八字段用 store 里的角色预填，id 是只读行', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);
      final CollabRolesStore store = await loadedStore(<CollabRole>[demoRole()]);

      await pumpEditor(tester, roleId: 'reviewer', store: store);

      expect(find.text('安全审查者'), findsOneWidget);
      expect(find.text('只找安全问题'), findsOneWidget);
      expect(find.text('必须列证据'), findsOneWidget);
      expect(find.text('codex/gpt-5'), findsOneWidget);
      // id 只读：没有可编辑的 id 输入框，只有只读行
      expect(find.byKey(const ValueKey<String>('collab-role-id')), findsNothing);
      expect(find.byKey(const ValueKey<String>('collab-role-id-readonly')),
          findsOneWidget);
      // 只读行说明「为什么不能改」
      expect(find.textContaining('引用键'), findsWidgets);
      // 编辑态不做 id 校验（只读字段必已合法）
      expect(find.text('协同角色 id 不能为空'), findsNothing);
    });

    testWidgets('编辑态保存用落库 id（不用输入框文本）', (tester) async {
      SetCollabRoleRequest? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        switch (call.method) {
          case 'listCollabRoles':
            return ListCollabRolesResponse(roles: <CollabRole>[demoRole()])
                .writeToBuffer();
          case 'setCollabRole':
            seen = SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
            return SetCollabRoleResponse(role: seen!.role).writeToBuffer();
          default:
            throw MissingPluginException('No implementation found');
        }
      });
      final CollabRolesStore store = await loadedStore(<CollabRole>[demoRole()]);

      await pumpEditor(tester, roleId: 'reviewer', store: store);
      await tapKey(tester, 'collab-role-save');

      expect(seen!.role.id, 'reviewer');
    });

    testWidgets('找不到角色（可能已被删除）→ 诚实降级 + 返回，不做半成品编辑',
        (tester) async {
      mockHost(roles: const <CollabRole>[]);
      final CollabRolesStore store = await loadedStore(const <CollabRole>[]);
      int backCalls = 0;

      await pumpEditor(
        tester,
        roleId: 'ghost',
        store: store,
        onBack: () => backCalls += 1,
      );

      expect(find.text('找不到该角色(可能已被删除)'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('collab-role-id')), findsNothing,
          reason: '没有表单可编辑（半成品编辑 = 伪造编辑能力）');

      await tester.tap(find.byKey(const ValueKey<String>('collab-role-editor-back')));
      await tester.pumpAndSettle();
      expect(backCalls, 1);
    });
  });

  group('agent 下拉：能力位驱动', () {
    testWidgets('collab 位 → 候选项出现，默认选 collab 第一家', (tester) async {
      mockHost();

      await pumpEditor(
        tester,
        roleId: '',
        capabilities: const <String>['agent:codex', 'collab:pi', 'collab:codex'],
      );

      expect(find.byKey(const ValueKey<String>('collab-role-agent-codex')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('collab-role-agent-pi')),
          findsOneWidget);
      // 展示名映射（selectedAgentLabel）进 chip 文案
      expect(find.widgetWithText(ChoiceChip, 'Codex'), findsOneWidget);
      expect(
        tester
            .widget<ChoiceChip>(
                find.byKey(const ValueKey<String>('collab-role-agent-codex')))
            .selected,
        isTrue,
        reason: '默认值 = collab 位排序后第一家（codex < pi）',
      );
    });

    testWidgets('点另一家 → 选中切换', (tester) async {
      mockHost();

      await pumpEditor(
        tester,
        roleId: '',
        capabilities: const <String>['collab:pi', 'collab:codex'],
      );

      await tapKey(tester, 'collab-role-agent-pi');

      expect(
        tester
            .widget<ChoiceChip>(
                find.byKey(const ValueKey<String>('collab-role-agent-pi')))
            .selected,
        isTrue,
      );
      expect(
        tester
            .widget<ChoiceChip>(
                find.byKey(const ValueKey<String>('collab-role-agent-codex')))
            .selected,
        isFalse,
      );
    });

    testWidgets('没有位数据 → 手填降级 + 说明文案（不编五家名单）', (tester) async {
      mockHost();

      await pumpEditor(
        tester,
        roleId: '',
        capabilities: const <String>['fs', 'git'],
      );

      // 手填框 + 诚实说明；保存仍可用（agent_id 透传真值）
      expect(find.byKey(const ValueKey<String>('collab-role-agent-free')),
          findsOneWidget);
      expect(find.textContaining('暂未报告可协同的 agent 名单'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('collab-role-agent-codex')),
          findsNothing,
          reason: '没有位就不摆假候选（写死名单 = 第二份 agent 名单红线）');
    });

    testWidgets('手填降级下填了 agent → 保存可过桥（存在性裁决在 daemon）',
        (tester) async {
      SetCollabRoleRequest? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        switch (call.method) {
          case 'listCollabRoles':
            return ListCollabRolesResponse().writeToBuffer();
          case 'setCollabRole':
            seen = SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
            return SetCollabRoleResponse(role: seen!.role).writeToBuffer();
          default:
            throw MissingPluginException('No implementation found');
        }
      });

      await pumpEditor(
        tester,
        roleId: '',
        capabilities: const <String>[],
      );
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'reviewer');
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-agent-free')), 'hermes');
      await tester.pumpAndSettle();
      await tapKey(tester, 'collab-role-save');

      expect(seen!.role.agentId, 'hermes');
    });
  });

  group('思考强度：五档分段', () {
    testWidgets('默认「跟随默认」；可切到具体档位并过桥', (tester) async {
      SetCollabRoleRequest? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        switch (call.method) {
          case 'listCollabRoles':
            return ListCollabRolesResponse().writeToBuffer();
          case 'setCollabRole':
            seen = SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
            return SetCollabRoleResponse(role: seen!.role).writeToBuffer();
          default:
            throw MissingPluginException('No implementation found');
        }
      });

      await pumpEditor(
        tester,
        roleId: '',
        capabilities: const <String>['collab:codex'],
      );
      await tester.enterText(
          find.byKey(const ValueKey<String>('collab-role-id')), 'reviewer');
      await tapKey(tester, 'collab-role-effort-high');
      await tapKey(tester, 'collab-role-save');

      expect(seen!.role.effort, 'high');
    });
  });

  group('返回键', () {
    testWidgets('返回 → onBack（唯一出口，不含保存）', (tester) async {
      mockHost();
      int backCalls = 0;

      await pumpEditor(
        tester,
        roleId: '',
        onBack: () => backCalls += 1,
        capabilities: const <String>['collab:codex'],
      );

      await tester.tap(find.byKey(const ValueKey<String>('collab-role-editor-back')));
      await tester.pumpAndSettle();

      expect(backCalls, 1);
    });
  });
}
