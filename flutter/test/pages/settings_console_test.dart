// Settings ③console 子步测试 —— mock 的仍是「宿主桥的 wire 行为」，
// PaBridge 全程真实。
//
// 【验收三件事（本文件的三组 group）】
//   ① 三门控：`runtime.v1` 位不在 → 三块内容整块不渲染、只给一句明话，且
//      **不产生任何 runtime 域调用**（不把 UNIMPLEMENTED 摆给用户看）；位在读
//      不到（HOST_UNAVAILABLE）→ 能力位块错误态 + 重试；位在 → 三块各自三态。
//   ② 位过滤后三块各自三态分家：一块失败不拖垮其余两块（Kotlin
//      RuntimeControlState 逐块记错同款）；空态 ≠ 错误态（台账空列表 vs
//      HOST_UNAVAILABLE）。
//   ③ 只读纪律：全页**没有任何写调用**（setRuntimePolicy / setCollabPolicy /
//      acknowledgeUnknownEvent / executeUnknownEventAction 一个都不发），也
//      不摆任何开关/动作按钮；未知事件行走 getUnknownEvent 详情（只读）。
//
// 【纯函数】标签词表与额度行映射照 Kotlin RuntimeStateLogic 同款口径逐条钉：
// 未知取值原样返回（透传红线），UNKNOWN 安全分级说「尚未分类」而非「未知」。

import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/runtime.pb.dart';
import 'package:pocketagent_ui/pages/settings_console.dart';
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
    messenger.setMockStreamHandler(
      eventChannel,
      MockStreamHandler.inline(
        onListen: (Object? args, MockStreamHandlerEventSink sink) {},
      ),
    );
  });

  void mockMethods(FutureOr<Object?> Function(MethodCall call) onMethod) {
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      calls.add(call);
      return onMethod(call);
    });
  }

  /// 宿主能力位应答（`getHostInfo` 的 wire 字节）。
  Uint8List hostInfoBytes(List<String> capabilities) => HostInfo(
        hostId: 'host-1',
        displayName: 'YANZI-PC',
        daemonVersion: '0.4.0',
        capabilities: capabilities,
      ).writeToBuffer();

  Uint8List runtimeStateBytes({
    String runtimeState = 'ready',
    List<RuntimeContractReport> contracts = const <RuntimeContractReport>[],
  }) =>
      RuntimeStateResponse(
        daemonVersion: '0.4.0',
        runtimeState: runtimeState,
        hostId: 'host-1',
        dataDir: r'C:\data',
        contracts: contracts,
      ).writeToBuffer();

  Uint8List runtimePolicyBytes({
    RuntimeLimits? effectiveGlobal,
    int revision = 3,
    String policyPath = r'C:\policy.json',
  }) =>
      RuntimePolicyResponse(
        revision: Int64(revision),
        policyPath: policyPath,
        effectiveGlobal: effectiveGlobal,
      ).writeToBuffer();

  Uint8List listUnknownEventsBytes({
    List<UnknownEventSummary> items = const <UnknownEventSummary>[],
    int total = 0,
    int dropped = 0,
  }) =>
      ListUnknownEventsResponse(
        items: items,
        total: Int64(total),
        dropped: Int64(dropped),
      ).writeToBuffer();

  UnknownEventSummary summary({
    String eventId = 'ue-1',
    String sourceMethod = 'tools/call',
    String sourceAgent = 'codex',
    String normalizedKind = 'UNKNOWN_REQUEST',
    String processingStatus = 'NEW',
    String safetyClass = 'UNKNOWN',
    bool requiresUser = false,
    String plainTitle = '',
    String summaryText = '',
  }) =>
      UnknownEventSummary(
        eventId: eventId,
        sourceMethod: sourceMethod,
        sourceAgent: sourceAgent,
        normalizedKind: normalizedKind,
        processingStatus: processingStatus,
        safetyClass: safetyClass,
        requiresUser: requiresUser,
        plainTitle: plainTitle,
        summary: summaryText,
      );

  Uint8List unknownEventBytes({
    String rawJson = '{"method":"tools/call","params":{"x":1}}',
    String translation = 'agent 发来了一个我们不认识的操作。',
    String recommendedNextStep = '可在电脑端查看原文。',
    String plainTitle = '一个不认识的工具调用',
    List<UnknownEventField> digest = const <UnknownEventField>[],
  }) =>
      UnknownEventResponse(
        summary: UnknownEventSummary(plainTitle: plainTitle),
        rawJson: rawJson,
        translation: translation,
        recommendedNextStep: recommendedNextStep,
        digest: digest,
      ).writeToBuffer();

  /// 位在 + 三块都正常应答的宿主（多数用例的默认装配）。
  void mockCapable() {
    mockMethods((MethodCall c) => switch (c.method) {
          'getHostInfo' => hostInfoBytes(<String>['terminal', 'runtime.v1']),
          'getRuntimeState' => runtimeStateBytes(contracts: <RuntimeContractReport>[
              RuntimeContractReport(
                agentId: 'codex',
                displayName: 'Codex',
                version: '0.160.0',
                declared: true,
                sessionModel: 'NATIVE',
                resumeModel: 'reattach',
              ),
            ]),
          'getRuntimePolicy' => runtimePolicyBytes(
            effectiveGlobal: RuntimeLimits(
              ramEventBufferBytes: Int64(64 * 1024 * 1024),
              diskCacheBytes: Int64(2 * 1024 * 1024 * 1024),
              maxParallelTurns: 3,
            ),
          ),
          'listUnknownEvents' => listUnknownEventsBytes(
            items: <UnknownEventSummary>[summary()],
            total: 5,
            dropped: 2,
          ),
          _ => null,
        });
  }

  Future<void> pumpConsolePage(WidgetTester tester, {VoidCallback? onBack}) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: SettingsConsolePage(
            bridge: PaBridge(),
            onBack: onBack ?? () {},
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('纯函数：标签词表与额度映射', () {
    test('runtimeControlCapable：大小写不敏感地找 runtime.v1', () {
      expect(runtimeControlCapable(<String>['terminal', 'runtime.v1']), isTrue);
      expect(runtimeControlCapable(<String>['RUNTIME.V1']), isTrue);
      expect(runtimeControlCapable(<String>['terminal', 'runtime.v2']), isFalse);
      expect(runtimeControlCapable(const <String>[]), isFalse);
    });

    test('标签词表：已知值译人话，未知值原样（不透「未知/错误」）', () {
      expect(runtimeStateLabel('ready'), '就绪');
      expect(runtimeStateLabel('degraded'), '降级');
      expect(runtimeStateLabel('future_state'), 'future_state',
          reason: '词表外原样透传');
      expect(unknownStatusLabel('NEW'), '待处理');
      expect(unknownStatusLabel('NEEDS_NATIVE_SUPPORT'), '待上游支持');
      expect(unknownStatusLabel('FUTURE_STATUS'), 'FUTURE_STATUS');
      expect(unknownKindLabel('UNKNOWN_REQUEST'), '新请求');
      expect(unknownKindLabel('UNKNOWN_NOTIFICATION'), '新通知');
      expect(unknownKindLabel('FUTURE_KIND'), 'FUTURE_KIND');
      expect(safetyClassLabel('UNKNOWN'), '尚未分类',
          reason: '上游分类规则未定义：说「尚未分类」，不说「未知」');
      expect(safetyClassLabel('RISKY'), 'RISKY');
    });

    test('unknownEventTitle：方法名原文优先，缺席退归类，再退 event_id', () {
      expect(unknownEventTitle(summary()), 'tools/call',
          reason: '原生方法名原文不翻译');
      expect(unknownEventTitle(summary(sourceMethod: '')), '新请求');
      expect(unknownEventTitle(summary(sourceMethod: '', normalizedKind: '')),
          'ue-1');
    });

    test('unknownLedgerNote：空台账/没丢 → null；有丢 → 明面告知', () {
      expect(unknownLedgerNote(0, 0), isNull);
      expect(unknownLedgerNote(5, 0), '累计 5 条');
      expect(unknownLedgerNote(5, 2), '累计 5 条，超出上限已丢弃 2 条',
          reason: '只回最近 N 条而不说，用户会以为「就这么几条」');
    });

    test('limitRowsOf：只渲染 daemon 明确设过的项（optional 语义）', () {
      final List<LimitRow> rows = limitRowsOf(RuntimeLimits(
        ramEventBufferBytes: Int64(64 * 1024 * 1024),
        maxParallelTurns: 3,
      ));
      expect(rows, hasLength(2));
      expect(rows[0].label, '内存 · 事件缓冲');
      expect(rows[0].value, 64 * 1024 * 1024);
      expect(rows[0].isCount, isFalse);
      expect(rows[1].label, '并发轮次上限');
      expect(rows[1].isCount, isTrue,
          reason: '并发轮次是计数量，不是字节量，不带单位换算');
      expect(limitRowsOf(null), isEmpty);
      expect(limitRowsOf(RuntimeLimits()), isEmpty,
          reason: '全未设 = 一行都不渲染，不把 0 显示成「封禁」');
    });

    test('consolePageErrorText：HOST_UNAVAILABLE 点名 B4，其余委托通用映射', () {
      expect(
        consolePageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.hostUnavailable, 'x'),
        ),
        contains('桌面端桥接未就绪（B4 波次）'),
      );
      expect(
        consolePageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.notConnected, 'x'),
        ),
        contains('尚未连上 daemon'),
      );
    });
  });

  group('门控①：runtime.v1 位', () {
    testWidgets('位不在 → 三块内容不渲染，只给一句明话；零 runtime 域调用',
        (tester) async {
      mockMethods((MethodCall c) => switch (c.method) {
            'getHostInfo' => hostInfoBytes(<String>['terminal', 'fs']),
            _ => null,
          });

      await pumpConsolePage(tester);

      expect(find.byKey(const ValueKey('pa-console-no-capability')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-no-capability')))
            .data,
        contains('runtime.v1'),
        reason: '明说缺的是哪一位，用户/开发者都能定位',
      );
      expect(find.text('运行时状态'), findsNothing);
      expect(find.text('预算与资源'), findsNothing);
      expect(find.text('未知事件'), findsNothing);
      // 无位不产生任何 runtime 域调用（旧 daemon 上必然 UNIMPLEMENTED）。
      expect(calls.map((MethodCall c) => c.method).toSet(), <String>{'getHostInfo'},
          reason: '门控未通过时一个 runtime 方法都不发');
    });

    testWidgets('位在读不到（宿主缺席）→ 能力位块错误态 + 重试恢复', (tester) async {
      bool fail = true;
      mockMethods((MethodCall c) async {
        if (fail) {
          throw MissingPluginException(
              'No implementation found for method getHostInfo');
        }
        return hostInfoBytes(<String>['runtime.v1']);
      });

      await pumpConsolePage(tester);

      expect(
        find.textContaining('桌面端桥接未就绪（B4 波次）'),
        findsOneWidget,
        reason: '门控无从判断时如实报错，不猜「有位」也不猜「无位」',
      );
      expect(find.byKey(const ValueKey('pa-console-no-capability')), findsNothing,
          reason: '读不到位 ≠ 没有位：不渲染成「本机不支持」');

      fail = false;
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('pa-console-caps-retry')),
        120,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pa-console-caps-retry')));
      await tester.pumpAndSettle();

      // 重试后位读到：三块按各自三态渲染（这里宿主对 runtime 方法回 null →
      // DECODE_FAILED 错误态，逐块细节见下一组）。
      expect(find.text('运行时状态'), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-console-caps-error')), findsNothing);
    });

    testWidgets('位在 → 三块各自就绪，主机身份与形态卡呈现', (tester) async {
      mockCapable();

      await pumpConsolePage(tester);

      // 块一：运行时状态（daemon 版本 + 运行时人话 + 形态卡）。
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-state-line')))
            .data,
        contains('daemon 0.4.0'),
      );
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-state-line')))
            .data,
        contains('运行时 就绪'),
      );
      expect(find.byKey(const ValueKey('pa-console-contract-codex')),
          findsOneWidget);
      expect(find.text('会话 NATIVE · 恢复 reattach'), findsOneWidget,
          reason: '六个 model 字段是 daemon 给的机器可读答案，照原文排版');
      // 块二：额度行（只渲染设过的项）。
      expect(find.text('内存 · 事件缓冲'), findsOneWidget);
      expect(find.text('64 MB'), findsOneWidget);
      expect(find.text('并发轮次上限'), findsOneWidget);
      expect(find.text('3'), findsOneWidget);
      expect(find.text('内存 · 工作集'), findsNothing,
          reason: 'daemon 没设的项不占位');
      // 块三：台账脚注 + 列表项。
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-events-ledger')))
            .data,
        '累计 5 条，超出上限已丢弃 2 条',
      );
      expect(find.byKey(const ValueKey('pa-console-event-ue-1')), findsOneWidget);
      // 只读说明常驻。
      expect(find.byKey(const ValueKey('pa-console-readonly-note')), findsOneWidget);
    });
  });

  group('门控②：三块各自三态，一块失败不拖垮其余', () {
    testWidgets('状态块读失败 → 策略与事件块照常就绪', (tester) async {
      mockMethods((MethodCall c) => switch (c.method) {
            'getHostInfo' => hostInfoBytes(<String>['runtime.v1']),
            'getRuntimeState' =>
              throw PlatformException(code: 'INTERNAL', message: '炸了'),
            'getRuntimePolicy' => runtimePolicyBytes(),
            'listUnknownEvents' => listUnknownEventsBytes(),
            _ => null,
          });

      await pumpConsolePage(tester);

      // 失败块：错误态 + 重试键。
      expect(find.byKey(const ValueKey('pa-console-state-error')), findsOneWidget);
      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey('pa-console-state-error')))
            .data,
        contains('桌面端内部错误'),
      );
      expect(find.byKey(const ValueKey('pa-console-state-retry')), findsOneWidget);
      // 其余两块不空、不报错（各拉各的，Kotlin 逐块记错同款）。
      expect(find.byKey(const ValueKey('pa-console-policy-error')), findsNothing);
      expect(find.byKey(const ValueKey('pa-console-events-empty')), findsOneWidget);
      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey('pa-console-events-empty')))
            .data,
        contains('还没有未知事件'),
      );
    });

    testWidgets('策略块 effective 全缺席 → 如实「未设置」，不渲染假额度',
        (tester) async {
      mockMethods((MethodCall c) => switch (c.method) {
            'getHostInfo' => hostInfoBytes(<String>['runtime.v1']),
            'getRuntimeState' => runtimeStateBytes(),
            'getRuntimePolicy' => runtimePolicyBytes(),
            'listUnknownEvents' => listUnknownEventsBytes(),
            _ => null,
          });

      await pumpConsolePage(tester);

      expect(find.byKey(const ValueKey('pa-console-policy-empty')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-policy-empty')))
            .data,
        contains('未设置全局额度'),
      );
      expect(find.byKey(const ValueKey('pa-console-policy-line')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-policy-line')))
            .data,
        contains('策略版本 3'),
      );
      // 没有任何额度行。
      expect(find.text('磁盘 · 缓存'), findsNothing);
    });

    testWidgets('状态块读成功但零形态申报 → 「未申报」，不造形态卡', (tester) async {
      mockMethods((MethodCall c) => switch (c.method) {
            'getHostInfo' => hostInfoBytes(<String>['runtime.v1']),
            'getRuntimeState' => runtimeStateBytes(),
            'getRuntimePolicy' => runtimePolicyBytes(),
            'listUnknownEvents' => listUnknownEventsBytes(),
            _ => null,
          });

      await pumpConsolePage(tester);

      expect(find.byKey(const ValueKey('pa-console-state-empty')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-console-state-empty')))
            .data,
        contains('未申报任何 agent'),
      );
    });
  });

  group('门控③：空态 ≠ 错误态 + 只读纪律', () {
    testWidgets('台账空列表 = 真空态；HOST_UNAVAILABLE = 错误态', (tester) async {
      mockMethods((MethodCall c) {
        // 复刻生产形态：宿主侧把「通道无 handler」作为 PlatformException 抛回来
        // （flutter_test 的 mock handler 抛任意异常都会被包成 code='error' 的
        // PlatformException，Dart 侧按契约只透传不改写 —— 这里直接按宿主形态抛）。
        if (c.method == 'listUnknownEvents') {
          throw PlatformException(
            code: PaBridgeErrorCodes.hostUnavailable,
            message: '宿主桥未注册：dev.pocketagent/bridge',
          );
        }
        return switch (c.method) {
          'getHostInfo' => hostInfoBytes(<String>['runtime.v1']),
          'getRuntimeState' => runtimeStateBytes(),
          'getRuntimePolicy' => runtimePolicyBytes(),
          _ => null,
        };
      });

      await pumpConsolePage(tester);

      expect(find.byKey(const ValueKey('pa-console-events-error')), findsOneWidget);
      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey('pa-console-events-error')))
            .data,
        contains('桌面端桥接未就绪（B4 波次）'),
      );
      expect(find.byKey(const ValueKey('pa-console-events-empty')), findsNothing,
          reason: '读失败不许渲染成「还没有未知事件」——那是真空态的文案');
      // 台账脚注也不出现（没有 total/dropped 可呈现）。
      expect(find.byKey(const ValueKey('pa-console-events-ledger')), findsNothing);
    });

    testWidgets('未知事件详情：走 getUnknownEvent（只读），原文可查',
        (tester) async {
      mockMethods((MethodCall c) => switch (c.method) {
            'getHostInfo' => hostInfoBytes(<String>['runtime.v1']),
            'getRuntimeState' => runtimeStateBytes(),
            'getRuntimePolicy' => runtimePolicyBytes(),
            'listUnknownEvents' => listUnknownEventsBytes(
              items: <UnknownEventSummary>[
                summary(
                  eventId: 'ue-9',
                  plainTitle: '一个不认识的工具调用',
                  summaryText: 'agent 调用了未注册的工具。',
                ),
              ],
            ),
            'getUnknownEvent' => unknownEventBytes(
              digest: <UnknownEventField>[
                UnknownEventField(
                  key: 'method',
                  kind: 'text',
                  preview: 'tools/call',
                  meaning: '方法名',
                ),
              ],
            ),
            _ => null,
          });

      await pumpConsolePage(tester);

      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('pa-console-event-ue-9')),
        120,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pa-console-event-ue-9')));
      await tester.pumpAndSettle();

      // 详情三句人话 + 透析表 + 脱敏原文。
      // 【为什么遍历 Text 而不是按 key 取】这些文案的键有两种落点：PaCardCaption
      // 把 key 传给内部 Text（自身即命中），透析表的键挂在外层 Padding 上（命中的
      // 是容器）。按「容器键下的第一个 Text」统一取，测试不为布局细节写两份取法。
      String textUnder(String key) {
        final Iterable<Text> hits = tester
            .widgetList<Text>(find.byType(Text))
            .where((Text t) =>
                t.key == ValueKey<String>(key) ||
                find
                    .ancestor(
                      of: find.byWidget(t),
                      matching: find.byKey(ValueKey<String>(key)),
                    )
                    .evaluate()
                    .isNotEmpty);
        return hits.first.data!;
      }

      expect(find.byKey(const ValueKey('pa-console-detail-text-0')), findsOneWidget);
      expect(textUnder('pa-console-detail-text-0'), '一个不认识的工具调用');
      expect(find.byKey(const ValueKey('pa-console-digest-method')), findsOneWidget);
      expect(textUnder('pa-console-digest-method'), 'method（text）：tools/call');
      expect(
        textUnder('pa-console-detail-raw'),
        contains('tools/call'),
        reason: '翻译是摘要不是裁决，冲突以原文为准 —— 原文必须可查',
      );
      // getUnknownEvent 的请求字节里带着点中的 event_id。
      final MethodCall detailCall =
          calls.firstWhere((MethodCall c) => c.method == 'getUnknownEvent');
      final GetUnknownEventRequest wire =
          GetUnknownEventRequest.fromBuffer(detailCall.arguments as Uint8List);
      expect(wire.eventId, 'ue-9');

      // 收起详情：卡片消失，不发任何调用。
      final int before = calls.length;
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('pa-console-detail-close')),
        120,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pa-console-detail-close')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('pa-console-detail-raw')), findsNothing);
      expect(calls.length, before, reason: '收起是纯本地 UI 动作，不过桥');
    });

    testWidgets('summary 全缺席 → 固定降级说明，不造翻译', (tester) async {
      mockMethods((MethodCall c) => switch (c.method) {
            'getHostInfo' => hostInfoBytes(<String>['runtime.v1']),
            'getRuntimeState' => runtimeStateBytes(),
            'getRuntimePolicy' => runtimePolicyBytes(),
            'listUnknownEvents' =>
              listUnknownEventsBytes(items: <UnknownEventSummary>[summary()]),
            _ => null,
          });

      await pumpConsolePage(tester);

      expect(find.byKey(const ValueKey('pa-console-event-fallback')), findsOneWidget);
      expect(
        tester
            .widget<Text>(
                find.byKey(const ValueKey('pa-console-event-fallback')))
            .data,
        '此 daemon 版本未提供翻译',
      );
    });

    testWidgets('只读纪律：全页零写调用、零开关、零动作按钮', (tester) async {
      mockCapable();

      await pumpConsolePage(tester);

      // 展开一条详情（本页唯一的第二个调用面），确认它也只是读。
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('pa-console-event-ue-1')),
        120,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pa-console-event-ue-1')));
      await tester.pumpAndSettle();

      final Set<String> methods = calls.map((MethodCall c) => c.method).toSet();
      expect(
        methods.difference(<String>{
          'getHostInfo',
          'getRuntimeState',
          'getRuntimePolicy',
          'listUnknownEvents',
          'getUnknownEvent',
        }),
        isEmpty,
        reason: '写操作四件套（setRuntimePolicy/setCollabPolicy/'
            'acknowledgeUnknownEvent/executeUnknownEventAction）一个都不发',
      );
      // 没有任何开关/可选动作按钮（灰按钮也不摆）。
      expect(find.byType(Switch), findsNothing);
      expect(find.byType(Checkbox), findsNothing);
      expect(find.byType(Radio), findsNothing);
      // 「收起详情」之外没有别的按钮（重试键此时不在错误态，不出现）。
      expect(find.byType(TextButton), findsOneWidget);
    });
  });
}
