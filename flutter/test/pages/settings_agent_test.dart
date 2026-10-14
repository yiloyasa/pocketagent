// Settings ②agent 子步测试 —— mock 的仍是「宿主桥的 wire 行为」，PaBridge 全程真实。
//
// 【为什么这样测】agent 子页验收三件事：① capabilities 从 getHostInfo 真解码后
// 按前缀自然分组（含未知前缀原样透传，不许显「未知」）；② 宿主缺席（MissingPlugin
// Exception → HOST_UNAVAILABLE）时诚实占位 + 重试，绝不伪造能力清单；③
// agent_native_settings:<id> 位门控两条（位在亮入口 / 位不在不出现），且本页
// 没有任何写通道（入口只展示和门控，不产生额外跨端调用）。
//
// 【为什么位数据不写死】HostInfo 字节由测试现拼（gen stub），页面不缓存也不
// 内嵌任何位族名单 —— 分组是纯函数对任意输入成立，测试只钉行为不钉名单。

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/pages/settings_agent.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kPaBridgeEventChannel);

  late TestDefaultBinaryMessenger messenger;
  late List<MethodCall> calls;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    calls = <MethodCall>[];
    // agent 子页不订阅事件面（快照 + 手动重试的闭环）；这里仍注册一个空实现，
    // 防后续用例把子页放进含 ConnectionSection 的树里时 listen 握手错进框架日志。
    messenger.setMockStreamHandler(
      eventChannel,
      MockStreamHandler.inline(
        onListen: (Object? args, MockStreamHandlerEventSink sink) {},
      ),
    );
  });

  /// getHostInfo 的 wire 字节（宿主侧应答形态：proto HostInfo）。
  Uint8List hostInfoBytes({
    List<String> capabilities = const <String>[],
    String hostId = 'host-1',
    String displayName = 'YANZI-PC',
    String daemonVersion = '0.4.0',
  }) =>
      HostInfo(
        hostId: hostId,
        displayName: displayName,
        daemonVersion: daemonVersion,
        capabilities: capabilities,
      ).writeToBuffer();

  void mockMethods(FutureOr<Object?> Function(MethodCall call) onMethod) {
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      calls.add(call);
      return onMethod(call);
    });
  }

  Future<void> pumpAgentPage(WidgetTester tester, {VoidCallback? onBack}) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: SettingsAgentPage(
            bridge: PaBridge(),
            onBack: onBack ?? () {},
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('能力位总览：按前缀自然分组', () {
    testWidgets('位族分组 + 未知前缀原样 + 位原文上 chip，不显「未知/错误」',
        (tester) async {
      mockMethods((MethodCall c) => hostInfoBytes(capabilities: <String>[
            'agent',
            'agent:codex',
            'agent:pi:v0.87.1',
            'collab:dsh',
            'fs',
            'future_family:alpha',
            'runtime.v1',
            'terminal',
          ]));

      await pumpAgentPage(tester);

      // 组头 = 组名 + 位数（先例：Android capabilityKindLabel + " · " + count）。
      // 注意：裸 `agent`（无冒号）是宿主级静态位，归宿主级组——只有 `agent:xxx`
      // 这种带后缀的才进 agent 位族（与 daemon 的生成格式一致）。
      expect(find.text('agent · 2'), findsOneWidget);
      expect(find.text('collab · 1'), findsOneWidget);
      // 无冒号的位 = 宿主级（agent / fs / runtime.v1 / terminal）。
      expect(find.text('宿主级 · 4'), findsOneWidget);
      // 未知前缀原样成为组名（透传红线：不猜、不改写）。
      expect(find.text('future_family · 1'), findsOneWidget);

      // 位原文逐个上 chip，一个不丢、一字不改。
      for (final String bit in <String>[
        'agent',
        'agent:codex',
        'agent:pi:v0.87.1',
        'collab:dsh',
        'fs',
        'future_family:alpha',
        'runtime.v1',
        'terminal',
      ]) {
        expect(find.text(bit), findsOneWidget, reason: '位 $bit 必须原样展示');
      }

      expect(find.text('未知'), findsNothing);
      expect(find.text('错误'), findsNothing);

      // 主机身份行（能力位出自哪台桌面）。
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-agent-host-line')))
            .data,
        contains('YANZI-PC'),
      );
    });

    testWidgets('读成功但无任何位 → 如实说「未报告」，不冒充成读失败',
        (tester) async {
      mockMethods((MethodCall c) => hostInfoBytes());

      await pumpAgentPage(tester);

      expect(find.byKey(const ValueKey('pa-agent-empty')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-agent-empty')))
            .data,
        contains('未报告任何能力位'),
      );
      expect(find.byKey(const ValueKey('pa-agent-error')), findsNothing);
    });
  });

  group('宿主缺席：诚实占位，不伪造能力清单', () {
    testWidgets('MissingPluginException → 「桌面端桥接未就绪（B4 波次）」+ 重试恢复',
        (tester) async {
      bool fail = true;
      mockMethods((MethodCall c) async {
        if (fail) {
          // 复刻生产：宿主未注册 handler → 引擎回 null 应答 → MissingPluginException。
          throw MissingPluginException('No implementation found for method getHostInfo');
        }
        return hostInfoBytes(capabilities: <String>['terminal']);
      });

      await pumpAgentPage(tester);

      expect(
        find.textContaining('桌面端桥接未就绪（B4 波次）'),
        findsOneWidget,
        reason: '占位文案必须点名是哪一层没就绪（B4 波次），不能含糊成「读取失败」',
      );
      expect(find.byKey(const ValueKey('pa-agent-error')), findsOneWidget);
      expect(find.text('terminal'), findsNothing,
          reason: '读失败就不许出现任何位——伪造清单 = 假绿');
      expect(find.text('原生设置'), findsNothing,
          reason: '宿主未就绪时原生设置门控无从判断，整组不出现');

      // 重试：宿主就绪后如实恢复。
      fail = false;
      await tester.tap(find.byKey(const ValueKey('pa-agent-retry')));
      await tester.pumpAndSettle();

      expect(find.text('terminal'), findsOneWidget);
      expect(find.textContaining('桌面端桥接未就绪'), findsNothing);
    });
  });

  group('agent_native_settings 门控', () {
    testWidgets('位在 → 「原生设置」入口亮起；点按只给说明，不产生写通道',
        (tester) async {
      mockMethods((MethodCall c) => hostInfoBytes(capabilities: <String>[
            'agent:pi',
            'agent_native_settings:dsh',
            'agent_native_settings:pi',
          ]));

      await pumpAgentPage(tester);

      expect(find.text('原生设置'), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-agent-native-dsh')), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-agent-native-pi')), findsOneWidget);
      expect(find.text('原生设置 · dsh'), findsOneWidget);
      expect(find.text('原生设置 · pi'), findsOneWidget);

      // 点入口：只出现「红线」说明（本波只展示与门控，真编辑面待后续波次）。
      await tester.tap(find.byKey(const ValueKey('pa-agent-native-pi')));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-agent-native-notice-pi')))
            .data,
        contains('红线'),
      );

      // 全程只有 getHostInfo 一种跨端调用 —— 没有写通道、没有第二个数据面。
      expect(
        calls.map((MethodCall c) => c.method).toSet(),
        <String>{'getHostInfo'},
        reason: '本页只读：入口是展示与门控，不许产生任何写调用',
      );
    });

    testWidgets('位不在（含空 id 的位）→ 入口不出现，不摆假开关', (tester) async {
      mockMethods((MethodCall c) => hostInfoBytes(capabilities: <String>[
            'agent:pi',
            'agent_settings:pi',
            // 族在但 id 为空：只进总览，不生成入口（入口需要可指认的目标）。
            'agent_native_settings:',
          ]));

      await pumpAgentPage(tester);

      expect(find.byKey(const ValueKey('pa-agent-native-pi')), findsNothing);
      expect(find.text('原生设置'), findsNothing);
      // 空 id 位本身仍在总览里（透传不吞）。
      expect(find.text('agent_native_settings:'), findsOneWidget);
    });
  });

  group('纯函数', () {
    test('groupCapabilityBits：前缀分组、首现序、组内原序、无冒号归宿主级', () {
      final List<CapabilityBitGroup> groups =
          groupCapabilityBits(<String>[
        'terminal',
        'agent:codex',
        'fs',
        'agent:pi',
        'agent',
        'collab:x',
      ]);
      expect(
        groups.map((CapabilityBitGroup g) => g.prefix).toList(),
        <String>['', 'agent', 'collab'],
        reason: '组序 = 首现顺序，不另造排序规则',
      );
      expect(groups[0].bits, <String>['terminal', 'fs', 'agent']);
      expect(groups[1].bits, <String>['agent:codex', 'agent:pi']);
      expect(groups[2].bits, <String>['collab:x']);
      expect(groupCapabilityBits(<String>[]), isEmpty);
    });

    test('capabilityGroupLabel：宿主级与任意前缀原样', () {
      expect(capabilityGroupLabel(''), '宿主级');
      expect(capabilityGroupLabel('agent'), 'agent');
      expect(capabilityGroupLabel('未来族'), '未来族');
    });

    test('nativeSettingsAgentIds：去重 / 空 id 跳过 / 族外忽略 / 顺序保持', () {
      expect(
        nativeSettingsAgentIds(<String>[
          'agent_native_settings:pi',
          'agent_native_settings:pi',
          'agent_native_settings:',
          'agent_settings:pi',
          'agent_native_settings:dsh',
        ]),
        <String>['pi', 'dsh'],
      );
      expect(nativeSettingsAgentIds(<String>['terminal']), isEmpty);
    });

    test('agentPageErrorText：HOST_UNAVAILABLE 点名 B4，其余委托通用错误映射', () {
      expect(
        agentPageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.hostUnavailable, 'x'),
        ),
        contains('桌面端桥接未就绪（B4 波次）'),
      );
      expect(
        agentPageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.notConnected, 'x'),
        ),
        contains('尚未连上 daemon'),
        reason: '非宿主缺席码走 connection 子页同一套映射，不另立文案表',
      );
      // 表外 code 透传（code + message 都显示，不吞、不改写）。
      final String weird = agentPageErrorText(
          const PaBridgeException('WEIRD_HOST_CODE', '新错'));
      expect(weird, contains('WEIRD_HOST_CODE'));
      expect(weird, contains('新错'));
    });
  });
}
