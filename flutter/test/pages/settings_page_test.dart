// Settings ③子步测试：一级页 ↔ 二级页路由（sub 参数）+ normalizeSettingsSub 纯函数。
//
// 【为什么对表 Android】`normalizeSettingsSub`（SettingsScreen.kt:449）是深链
// `settings?sub=` 的唯一入口，Flutter 侧同名同语义：已知值原样、旧值
// userdata→downloads、空/未知 → null（一级页）。测试与 Android
// `SettingsSearchFilterTest` 的深链用例逐条对齐。
//
// 【一级页 ↔ 子页】sub=agent/capabilities/downloads/console 进各自子页、返回键
// 回一级页；sub=connection 与未知值都停一级页（连接面在 B1 落法里就是一级页
// 首组）。「运行时」分组按 `runtime.v1` 位门控（无位不出现）。全部真走
// MethodChannel mock 宿主应答（PaBridge 全程真实）。

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/pages/settings.dart';
import 'package:pocketagent_ui/pages/settings_agent.dart';
import 'package:pocketagent_ui/pages/settings_capabilities.dart';
import 'package:pocketagent_ui/pages/settings_console.dart';
import 'package:pocketagent_ui/pages/settings_downloads.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kPaBridgeEventChannel);

  late TestDefaultBinaryMessenger messenger;

  /// 跨端调用流水（本文件多个用例断言「只产生了哪些调用」——门控与只读纪律的
  /// 唯一客观证据：页面说自己只读，就得看它真的没发写方法）。
  late List<MethodCall> calls;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    calls = <MethodCall>[];
    // 一级页含 ConnectionSection（getConnectionState 快照 + 事件订阅）；
    // 子页含 getHostInfo。宿主应答在这里统一伪造。
    messenger.setMockStreamHandler(
      eventChannel,
      MockStreamHandler.inline(
        onListen: (Object? args, MockStreamHandlerEventSink sink) {},
      ),
    );
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      calls.add(call);
      switch (call.method) {
        case 'getConnectionState':
          return <Object?, Object?>{
            'connState': 'down',
            'connTransport': '',
            'connAddress': '',
            'connectionEpoch': 0,
            'hostCapabilities': <Object?>['terminal'],
          };
        case 'getHostInfo':
          return HostInfo(
            hostId: 'host-1',
            displayName: 'YANZI-PC',
            capabilities: <String>['terminal'],
          ).writeToBuffer();
        default:
          return null;
      }
    });
  });

  Future<void> pumpSettings(WidgetTester tester, {String? sub}) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: SettingsPage(
            brightness: PABrightness.dark,
            onBrightnessChanged: (PABrightness b) {},
            bridge: PaBridge(),
            sub: sub,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('normalizeSettingsSub（纯函数，与 Android 同语义）', () {
    test('已知值原样返回（全集 = connection/downloads/agent/capabilities/console）',
        () {
      for (final String sub in <String>[
        'connection',
        'downloads',
        'agent',
        'capabilities',
        'console',
      ]) {
        expect(normalizeSettingsSub(sub), sub);
      }
    });

    test('旧值 userdata → downloads（任务中心下载卡的历史深链不断链）', () {
      expect(normalizeSettingsSub('userdata'), 'downloads');
    });

    test('空串 / null / 空白 / 未知值 → null（回一级页，不猜落点）', () {
      expect(normalizeSettingsSub(''), isNull);
      expect(normalizeSettingsSub(null), isNull);
      // 消亡页的旧 id 与任意未知值都回一级页（Android 用例同表：daemon/appearance/whatever）。
      expect(normalizeSettingsSub('daemon'), isNull);
      expect(normalizeSettingsSub('appearance'), isNull);
      expect(normalizeSettingsSub('whatever'), isNull);
      expect(normalizeSettingsSub('Search'), isNull, reason: '大小写敏感，不搞模糊匹配');
      expect(normalizeSettingsSub('userdata2'), isNull);
    });
  });

  /// 一级页是懒构建 ListView：Agent 组现在排在连接/外观/语音之后（不动既有
  /// 首屏布局——widget_test 钉着「连接与配对 + 外观」首屏可见），断言前先滚到。
  /// 提升到文件级：运行时分组用例（第③子步新增）同样要滚。
  Future<void> scrollTo(WidgetTester tester, String text) async {
    await tester.scrollUntilVisible(
      find.text(text),
      220,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pumpAndSettle();
  }

  group('一级页 ↔ 二级页路由', () {
    testWidgets('默认（无 sub）→ 一级页：两个入口行都在', (tester) async {
      await pumpSettings(tester);

      expect(find.text('连接与配对'), findsOneWidget);
      await scrollTo(tester, 'Agent 与能力');
      expect(find.text('Agent 与能力'), findsOneWidget);
      expect(find.text('模型'), findsOneWidget);
      expect(find.byType(SettingsAgentPage), findsNothing);
      expect(find.byType(SettingsCapabilitiesPage), findsNothing);
    });

    testWidgets('sub=agent → Agent 子页；返回键回一级页', (tester) async {
      await pumpSettings(tester, sub: 'agent');

      expect(find.byType(SettingsAgentPage), findsOneWidget);
      expect(find.text('能力位总览'), findsOneWidget);
      expect(find.text('terminal'), findsWidgets, reason: '宿主应答的位真到达子页');

      await tester.tap(find.byKey(const ValueKey('pa-agent-back')));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsAgentPage), findsNothing);
      // 回一级页后滚回顶部验首组（懒构建下最稳的可见锚点）。
      expect(find.text('连接与配对'), findsOneWidget);
      expect(find.text('连接状态'), findsOneWidget);
    });

    testWidgets('sub=capabilities → 模型子页；返回键回一级页', (tester) async {
      await pumpSettings(tester, sub: 'capabilities');

      expect(find.byType(SettingsCapabilitiesPage), findsOneWidget);
      expect(find.text('模型'), findsWidgets);

      await tester.tap(find.byKey(const ValueKey('pa-models-back')));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsCapabilitiesPage), findsNothing);
      expect(find.text('连接与配对'), findsOneWidget);
    });

    testWidgets('未知 sub 与空 sub → 一级页（不猜落点）', (tester) async {
      await pumpSettings(tester, sub: 'whatever');
      expect(find.byType(SettingsAgentPage), findsNothing);
      expect(find.byType(SettingsCapabilitiesPage), findsNothing);
      expect(find.text('连接与配对'), findsOneWidget);
    });

    testWidgets('sub=downloads → 真页（下载分区 + 下载内容 + 电脑端运行时）',
        (tester) async {
      await pumpSettings(tester, sub: 'downloads');

      // 真页在树上（不再是诚实占位）。
      expect(find.byType(SettingsDownloadsPage), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-pending-body')), findsNothing,
          reason: '第③子步后 downloads 有真页，占位文案不再出现');
      expect(find.text('下载与存储'), findsOneWidget);
      // 三个分组都在：存储分区（含未统计标注）/ 下载内容 / 电脑端运行时。
      expect(find.text('存储分区'), findsOneWidget);
      expect(find.text('未统计'), findsWidgets,
          reason: '本地统计面未接入的分区如实标未统计，不用 0 冒充');
      expect(find.byKey(const ValueKey('pa-downloads-empty')), findsOneWidget,
          reason: '本地清单空 = 空态（不是错误态）');

      // 不是一级页也不是别的子页。
      expect(find.byType(SettingsAgentPage), findsNothing);
      expect(find.text('连接与配对'), findsNothing);

      await tester.tap(find.byKey(const ValueKey('pa-downloads-back')));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsDownloadsPage), findsNothing);
      expect(find.text('连接与配对'), findsOneWidget);
    });

    testWidgets('sub=userdata 兼容映射 → downloads 真页', (tester) async {
      await pumpSettings(tester, sub: 'userdata');
      expect(find.text('下载与存储'), findsOneWidget);
      expect(find.byType(SettingsDownloadsPage), findsOneWidget);
    });

    testWidgets('入口行点按进子页：Agent 与能力行 → agent 子页', (tester) async {
      await pumpSettings(tester);

      await scrollTo(tester, 'Agent 与能力');
      await tester.tap(find.text('Agent 与能力'));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsAgentPage), findsOneWidget);
      expect(find.text('能力位总览'), findsOneWidget);
    });

    testWidgets('入口行点按进子页：模型行 → 模型子页（生产默认源 = 诚实占位）',
        (tester) async {
      await pumpSettings(tester);

      await scrollTo(tester, '模型');
      await tester.tap(find.text('模型'));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsCapabilitiesPage), findsOneWidget);
      // 生产默认 BridgeModelsDataSource 无真桥接：如实 HOST_UNAVAILABLE 占位。
      expect(find.textContaining('桌面端桥接未就绪（B4 波次）'), findsOneWidget);
    });

    testWidgets('sub=console → 控制台真页；无 runtime.v1 位 → 如实说明不摆假块',
        (tester) async {
      // 本文件 setUp 的宿主应答只宣告 terminal 位：控制台进门即无位路径。
      await pumpSettings(tester, sub: 'console');

      expect(find.byType(SettingsConsolePage), findsOneWidget);
      expect(find.text('电脑控制台'), findsOneWidget);
      // 门控①：无位不渲染三块内容，只给一句明话（不摆假开关/假列表）。
      expect(find.byKey(const ValueKey('pa-console-no-capability')), findsOneWidget);
      expect(find.text('运行时状态'), findsNothing);
      expect(find.text('未知事件'), findsNothing);
      // 位读不到时也不产生 runtime 域调用（不把 UNIMPLEMENTED 摆给用户看）。
      expect(
        calls.map((MethodCall c) => c.method).toSet(),
        <String>{'getConnectionState', 'getHostInfo'},
      );

      await tester.tap(find.byKey(const ValueKey('pa-console-back')));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsConsolePage), findsNothing);
      expect(find.text('连接与配对'), findsOneWidget);
    });

    testWidgets('sub=connection → 一级页（连接面就是一级页首组，不另开二级页）',
        (tester) async {
      await pumpSettings(tester, sub: 'connection');
      // 一级页的「连接与配对」组里 ConnectionSection 在树上。
      expect(find.text('连接状态'), findsOneWidget);
      expect(find.byType(SettingsAgentPage), findsNothing);
    });
  });

  group('一级页「运行时」分组：runtime.v1 位门控', () {
    /// 换一份宿主能力位应答（getConnectionState 的 hostCapabilities 是门控输入）。
    void mockCaps(List<String> capabilities) {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        switch (call.method) {
          case 'getConnectionState':
            return <Object?, Object?>{
              'connState': 'ok',
              'connTransport': 'grpc+tls',
              'connAddress': '192.168.1.20:7246',
              'connectionEpoch': 1,
              'hostCapabilities': <Object?>[...capabilities],
            };
          case 'getHostInfo':
            return HostInfo(
              hostId: 'host-1',
              displayName: 'YANZI-PC',
              capabilities: capabilities,
            ).writeToBuffer();
          default:
            return null;
        }
      });
    }

    testWidgets('位在 → 「运行时」分组与「电脑控制台」行出现，点按进真页',
        (tester) async {
      mockCaps(<String>['terminal', 'runtime.v1']);
      await pumpSettings(tester);

      await scrollTo(tester, '运行时');
      expect(find.text('运行时'), findsOneWidget);
      expect(find.text('电脑控制台'), findsOneWidget);

      await tester.tap(find.text('电脑控制台'));
      await tester.pumpAndSettle();
      expect(find.byType(SettingsConsolePage), findsOneWidget);
      // 位在：三块内容按各自三态渲染（这里宿主对 runtime 域方法回 null →
      // DECODE_FAILED 错误态，由 settings_console_test.dart 逐块钉，这里只验门控放行）。
      expect(find.text('运行时状态'), findsOneWidget);
    });

    testWidgets('位不在 → 「运行时」整组不出现（无能力不显示假入口）',
        (tester) async {
      mockCaps(<String>['terminal', 'fs']);
      await pumpSettings(tester);

      await scrollTo(tester, '关于');
      expect(find.text('运行时'), findsNothing);
      expect(find.text('电脑控制台'), findsNothing);
    });

    testWidgets('宿主缺席（能力位读不到）→ 运行时组不出现，其余分组照常',
        (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        // getConnectionState 抛 MissingPluginException = 宿主桥未注册。
        throw MissingPluginException('No implementation found for method ${call.method}');
      });
      await pumpSettings(tester);

      // 一级页其余分组不依赖能力位，照常渲染；运行时组无从判定 → 不出现
      // （不渲染错误横幅：这一组现在是「不可判定」，不是「坏了」）。
      expect(find.text('连接与配对'), findsOneWidget);
      expect(find.text('运行时'), findsNothing);
      expect(find.byType(SettingsConsolePage), findsNothing);
    });
  });
}
