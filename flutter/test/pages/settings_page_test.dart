// Settings ②子步测试：一级页 ↔ 二级页路由（sub 参数）+ normalizeSettingsSub 纯函数。
//
// 【为什么对表 Android】`normalizeSettingsSub`（SettingsScreen.kt:449）是深链
// `settings?sub=` 的唯一入口，Flutter 侧同名同语义：已知值原样、旧值
// userdata→downloads、空/未知 → null（一级页）。测试与 Android
// `SettingsSearchFilterTest` 的深链用例逐条对齐。
//
// 【一级页 ↔ 子页】sub=agent/capabilities 进各自子页、返回键回一级页；
// sub=downloads/console 如实占位（本波未实现）；sub=connection 与未知值都停
// 一级页（连接面在 B1 落法里就是一级页首组）。全部真走 MethodChannel mock 宿主
// 应答（PaBridge 全程真实）。

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/pages/settings.dart';
import 'package:pocketagent_ui/pages/settings_agent.dart';
import 'package:pocketagent_ui/pages/settings_capabilities.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kPaBridgeEventChannel);

  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    // 一级页含 ConnectionSection（getConnectionState 快照 + 事件订阅）；
    // 子页含 getHostInfo。宿主应答在这里统一伪造。
    messenger.setMockStreamHandler(
      eventChannel,
      MockStreamHandler.inline(
        onListen: (Object? args, MockStreamHandlerEventSink sink) {},
      ),
    );
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
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

  group('一级页 ↔ 二级页路由', () {
    /// 一级页是懒构建 ListView：Agent 组现在排在连接/外观/语音之后（不动既有
    /// 首屏布局——widget_test 钉着「连接与配对 + 外观」首屏可见），断言前先滚到。
    Future<void> scrollTo(WidgetTester tester, String text) async {
      await tester.scrollUntilVisible(
        find.text(text),
        220,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();
    }

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

    testWidgets('sub=downloads → 诚实占位（路由通了，内容后续波次搬入）', (tester) async {
      await pumpSettings(tester, sub: 'downloads');
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-pending-body')))
            .data,
        contains('后续波次'),
      );
      // 不是一级页也不是别的子页。
      expect(find.byType(SettingsAgentPage), findsNothing);
      expect(find.text('连接与配对'), findsNothing);

      await tester.tap(find.byKey(const ValueKey('pa-pending-back')));
      await tester.pumpAndSettle();
      expect(find.text('连接与配对'), findsOneWidget);
    });

    testWidgets('sub=userdata 兼容映射 → downloads 占位', (tester) async {
      await pumpSettings(tester, sub: 'userdata');
      expect(find.text('下载与存储'), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-pending-body')), findsOneWidget);
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

    testWidgets('sub=connection → 一级页（连接面就是一级页首组，不另开二级页）',
        (tester) async {
      await pumpSettings(tester, sub: 'connection');
      // 一级页的「连接与配对」组里 ConnectionSection 在树上。
      expect(find.text('连接状态'), findsOneWidget);
      expect(find.byType(SettingsAgentPage), findsNothing);
    });
  });
}
