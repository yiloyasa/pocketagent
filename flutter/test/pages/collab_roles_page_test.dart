// CollabRoles 列表页测试（B4 第 4 波）：三态 + 行渲染 + 删除确认流 + 空态 +
// store 行为注入 + 诚实占位两条路径。
//
// 【两条必须分家的路径】宿主未注册（B4 前）→ 错误态「协同角色加载失败」+
// 文案点名「未就绪（B4 波次）」；宿主正常回空响应 → 空态「还没有配置协同
// 角色」。各钉一条且互斥断言（红线：不留假绿——「通道没就绪」绝不许伪装成
// 「没有角色」）。
//
// 【删除确认流】破坏性写操作必须过显式确认：弹层写清后果（引用它的委派会报
// 「未定义的协同角色」、不可撤销），确认键是具体动作名（「删除」而不是
// 「确定」）；取消 = 中性键，删除 = danger 描边（主按钮永不红的纪律）。
// 弹层里什么都不点 = 不删（误触防护）。
//
// 【store 注入】页面构造注入 [CollabRolesStore]（生产由根装配共享；测试各
// 拿各的实例互不串味）。store 行为的细节（wire 往返/消息提取）在
// collab_roles_state_test.dart；这里测「给定 store，页面渲染什么、点了发生
// 什么」。

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/components/pa_button.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/collab_roles.dart';
import 'package:pocketagent_ui/pages/collab_roles_state.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

CollabRole demoRole({
  String id = 'reviewer',
  String name = '安全审查者',
  String agentId = 'codex',
  String description = '只找安全问题',
  String model = 'codex/gpt-5',
  String effort = 'high',
}) =>
    CollabRole(
      id: id,
      name: name,
      agentId: agentId,
      description: description,
      model: model,
      effort: effort,
    );

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  /// 宿主 wire 行为可切换的装配：默认全部方法抛 MissingPluginException
  /// （= B4 前宿主未注册的生产路径）；测试按需换成正常应答。
  void mockHost({
    List<CollabRole>? roles,
    Object? saveError,
    Object? deleteError,
  }) {
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      switch (call.method) {
        case 'listCollabRoles':
          return ListCollabRolesResponse(roles: roles ?? const <CollabRole>[])
              .writeToBuffer();
        case 'setCollabRole':
          if (saveError != null) throw saveError;
          return SetCollabRoleResponse().writeToBuffer();
        case 'deleteCollabRole':
          if (deleteError != null) throw deleteError;
          return DeleteCollabRoleResponse().writeToBuffer();
        default:
          throw MissingPluginException(
              'No implementation found for method ${call.method}');
      }
    });
  }

  Future<void> pumpRoles(
    WidgetTester tester, {
    CollabRolesStore? store,
    ValueChanged<String>? onOpenEditor,
    VoidCallback? onBack,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: CollabRolesPage(
            store: store,
            onOpenEditor: onOpenEditor ?? (String id) {},
            onBack: onBack ?? () {},
          ),
        ),
      ),
    );
    // store 的 load() 是 fire-and-forget：等它的加载 Future 收敛后再断言。
    await tester.pumpAndSettle();
  }

  group('三态', () {
    testWidgets('加载中 → 「正在加载协同角色…」，数据到了切换成列表', (tester) async {
      // 卡住不返回的宿主：把「加载中」钉在屏幕上（不靠时序碰运气）。
      final Completer<List<int>> pending = Completer<List<int>>();
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'listCollabRoles') return pending.future;
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });
      final CollabRolesStore store = CollabRolesStore();

      await tester.pumpWidget(
        MaterialApp(
          theme: PocketTheme.fromBrightness(PABrightness.dark),
          home: Scaffold(
            body: CollabRolesPage(
              store: store,
              onOpenEditor: (String id) {},
              onBack: () {},
            ),
          ),
        ),
      );
      await tester.pump();
      expect(find.text('正在加载协同角色…'), findsOneWidget);
      expect(find.text('安全审查者'), findsNothing);

      pending.complete(
          ListCollabRolesResponse(roles: <CollabRole>[demoRole()]).writeToBuffer());
      await tester.pumpAndSettle();

      expect(find.text('正在加载协同角色…'), findsNothing);
      expect(find.text('安全审查者'), findsOneWidget);
    });

    testWidgets('宿主未注册（B4 前）→ 错误态点名「未就绪（B4 波次）」，绝不是空态',
        (tester) async {
      // 不 mock = 复刻生产路径：宿主桥未注册 → HOST_UNAVAILABLE。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });

      await pumpRoles(tester);

      expect(find.text('协同角色加载失败'), findsOneWidget);
      expect(find.textContaining('未就绪（B4 波次）'), findsOneWidget);
      expect(find.text('还没有配置协同角色'), findsNothing,
          reason: '通道没就绪不许走空态——那是「真没有角色」的伪装版（红线：不留假绿）');
      expect(find.text('重试'), findsOneWidget, reason: '错误态给重试出口');
    });

    testWidgets('重试：把宿主「装上」后重试应转绿（重试不是装饰）', (tester) async {
      bool registered = false;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (!registered) {
          throw MissingPluginException('No implementation found');
        }
        return ListCollabRolesResponse(roles: <CollabRole>[demoRole()])
            .writeToBuffer();
      });

      await pumpRoles(tester);
      expect(find.text('协同角色加载失败'), findsOneWidget);

      registered = true;
      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(find.text('协同角色加载失败'), findsNothing);
      expect(find.text('安全审查者'), findsOneWidget);
    });

    testWidgets('宿主正常回空响应 → 空态「还没有配置协同角色」+ 新建入口',
        (tester) async {
      mockHost(roles: const <CollabRole>[]);

      await pumpRoles(tester);

      expect(find.text('还没有配置协同角色'), findsOneWidget);
      expect(find.textContaining('B4 波次'), findsNothing,
          reason: '空态不许出现错误路径的词——两条路径断言必须互相排斥');
      expect(find.text('协同角色加载失败'), findsNothing);
      expect(find.text('新建协同角色'), findsOneWidget);
    });
  });

  group('行渲染', () {
    testWidgets('显示名 + agent/model/effort 摘要 + 职责预览三行落位', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);

      await pumpRoles(tester);

      expect(find.text('安全审查者'), findsOneWidget);
      expect(find.text('Codex · codex/gpt-5 · high'), findsOneWidget,
          reason: '摘要行 = 展示名 agent · model · effort（三段同款口径）');
      expect(find.text('只找安全问题'), findsOneWidget);
    });

    testWidgets('name 空 → 回落 id；agent 空 → 回落 id 原文', (tester) async {
      mockHost(roles: <CollabRole>[
        demoRole(id: 'tester', name: '', agentId: ''),
      ]);

      await pumpRoles(tester);

      expect(find.text('tester'), findsOneWidget);
    });

    testWidgets('summary 全空（agent 与 id 皆空）→ 不占位、不伪造', (tester) async {
      mockHost(roles: <CollabRole>[demoRole(id: '', name: '无 id 角色', agentId: '')]);

      await pumpRoles(tester);

      expect(find.text('无 id 角色'), findsOneWidget);
      // 没有摘要行（· 分隔的三段一条都没有）
      expect(find.textContaining(' · '), findsNothing);
    });

    testWidgets('多行渲染 + 删除键每行各一枚（按 id 定位）', (tester) async {
      mockHost(roles: <CollabRole>[
        demoRole(id: 'reviewer'),
        demoRole(id: 'tester', name: '测试员', agentId: 'pi'),
      ]);

      await pumpRoles(tester);

      expect(find.byKey(const ValueKey<String>('collab-roles-name-reviewer')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('collab-roles-name-tester')),
          findsOneWidget);
      expect(
          find.byKey(const ValueKey<String>('collab-roles-delete-reviewer')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('collab-roles-delete-tester')),
          findsOneWidget);
    });
  });

  group('删除确认流（破坏性写操作）', () {
    testWidgets('确认弹层写清后果 + 取消 = 不删（误触防护）', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);

      await pumpRoles(tester);

      await tester.tap(find.byKey(const ValueKey<String>('collab-roles-delete-reviewer')));
      await tester.pumpAndSettle();

      // 后果写明白：引用它的委派会报「未定义的协同角色」+ 不可撤销
      expect(find.text('删除协同角色'), findsOneWidget);
      expect(find.textContaining('未定义的协同角色 reviewer'), findsOneWidget);
      expect(find.textContaining('不可撤销'), findsOneWidget);

      // 取消：中性键（不是危险色）
      await tester.tap(find.widgetWithText(PAButton, '取消'));
      await tester.pumpAndSettle();
      expect(find.text('删除协同角色'), findsNothing);
      expect(find.text('安全审查者'), findsOneWidget, reason: '取消了就一行不少');
    });

    testWidgets('确认删除 → 行消失；失败 → daemon 原文进可关闭错误条', (tester) async {
      mockHost(
        roles: <CollabRole>[demoRole()],
        deleteError: PlatformException(
          code: 'INTERNAL',
          message: '删除失败: 角色文件被占用',
        ),
      );

      await pumpRoles(tester);

      await tester.tap(find.byKey(const ValueKey<String>('collab-roles-delete-reviewer')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(PAButton, '删除'));
      await tester.pumpAndSettle();

      // daemon 原文如实透传（不泛化成「操作失败」）
      expect(find.text('删除失败: 角色文件被占用'), findsOneWidget);
      // 失败不动行
      expect(find.text('安全审查者'), findsOneWidget);

      // 错误条可关闭
      await tester.tap(
          find.byKey(const ValueKey<String>('collab-roles-delete-error-close')));
      await tester.pumpAndSettle();
      expect(find.text('删除失败: 角色文件被占用'), findsNothing);
    });

    testWidgets('删到只剩空 → 页面转空态（不是空白）', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);

      await pumpRoles(tester);

      await tester.tap(find.byKey(const ValueKey<String>('collab-roles-delete-reviewer')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(PAButton, '删除'));
      await tester.pumpAndSettle();

      expect(find.text('安全审查者'), findsNothing);
      expect(find.text('还没有配置协同角色'), findsOneWidget);
    });
  });

  group('入口（进编辑器）', () {
    testWidgets('点行 → onOpenEditor(role.id)', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);
      final List<String> opened = <String>[];

      await pumpRoles(tester, onOpenEditor: opened.add);

      await tester.tap(find.text('安全审查者'));
      await tester.pumpAndSettle();

      expect(opened, <String>['reviewer']);
    });

    testWidgets('空态新建 → onOpenEditor("")（空 id = 新建）', (tester) async {
      mockHost(roles: const <CollabRole>[]);
      final List<String> opened = <String>[];

      await pumpRoles(tester, onOpenEditor: opened.add);

      await tester.tap(find.widgetWithText(PAButton, '新建协同角色'));
      await tester.pumpAndSettle();

      expect(opened, <String>['']);
    });

    testWidgets('有数据时底部也有新建入口（长列表不用回顶部）', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);
      final List<String> opened = <String>[];

      await pumpRoles(tester, onOpenEditor: opened.add);

      await tester.tap(find.widgetWithText(PAButton, '新建协同角色'));
      await tester.pumpAndSettle();

      expect(opened, <String>['']);
    });

    testWidgets('返回键 → onBack', (tester) async {
      mockHost(roles: <CollabRole>[demoRole()]);
      int backCalls = 0;

      await pumpRoles(tester, onBack: () => backCalls += 1);

      await tester.tap(find.byKey(const ValueKey<String>('collab-roles-back')));
      await tester.pumpAndSettle();

      expect(backCalls, 1);
    });
  });
}
