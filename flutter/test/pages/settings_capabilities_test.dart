// Settings ②capabilities 子步测试 —— 模型列表消费形状（映射纯函数 + 三态渲染）。
//
// 【为什么数据源用 fake 而不是 mock 宿主 wire】pa_bridge.dart 当前没有
// listModels（transport 是 B3-3 领土，本波只写消费形状），真桥接的 wire 行为
// 在 B3-3/B4 接入后有它自己的通道测试（同 settings_connection_test 的先例）。
// 本文件钉的是：① `modelOptionsOf`/`modelOptionsOfResponse` 映射（displayName
// 兜底 id、current 标记、未知上下文不造数字、坏字节→DECODE_FAILED）；②
// [BridgeModelsDataSource] 缺桥接时如实 HOST_UNAVAILABLE / 有 invoke 时
// arguments 形状；③ 页面三态（加载 / 错误 / 就绪 + 空态）与「本页只读」——
// 行里没有写入口，数据源之外没有任何跨端调用。
//
// 【诚实红线】错误态、空态、成功态三者互不冒充：state_error 非空且无模型 =
// 「读不到元数据」（带 daemon 原文）；读成功但空 = 「未报告任何模型」。

import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/model.pb.dart';
import 'package:pocketagent_ui/pages/settings_capabilities.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 可控数据源：值或错都在构造时定死，load 真异步返回。
class _FakeSource implements ModelsDataSource {
  _FakeSource({this.models, this.failure});

  final List<ModelOption>? models;
  final Object? failure;
  int loads = 0;

  @override
  Future<List<ModelOption>> listModels() async {
    loads += 1;
    final Object? f = failure;
    if (f != null) throw f;
    return models ?? const <ModelOption>[];
  }
}

/// 手动放行数据源：测加载态用（Future 挂在 Completer 上，pump 不收敛它）。
class _HangingSource implements ModelsDataSource {
  @override
  Future<List<ModelOption>> listModels() => Completer<List<ModelOption>>().future;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  ModelOption option({
    String id = 'codex/gpt-5',
    String provider = 'codex',
    String displayName = 'GPT-5',
    int contextWindow = 128000,
    bool supportsThinking = true,
    bool supportsImages = true,
    bool available = true,
    bool isCurrent = false,
  }) =>
      ModelOption(
        id: id,
        provider: provider,
        displayName: displayName,
        contextWindow: contextWindow,
        supportsThinking: supportsThinking,
        supportsImages: supportsImages,
        available: available,
        isCurrent: isCurrent,
      );

  Future<void> pumpPage(WidgetTester tester, ModelsDataSource source,
      {VoidCallback? onBack}) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: SettingsCapabilitiesPage(source: source, onBack: onBack ?? () {}),
        ),
      ),
    );
  }

  group('映射纯函数：wire → 消费面', () {
    test('modelOptionsOfResponse：displayName 兜底 id、current 标记、保序', () {
      final List<ModelOption> options = modelOptionsOfResponse(
        ListModelsResponse(
          currentModelId: 'pi/glm-5.3',
          models: <ModelInfo>[
            ModelInfo(id: 'codex/gpt-5', provider: 'codex', displayName: 'GPT-5'),
            // 空 displayName = 原生没报：展示层退回 id，绝不空白。
            ModelInfo(id: 'pi/glm-5.3', provider: 'pi'),
            ModelInfo(id: 'hermes/default', provider: 'hermes', displayName: 'Hermes · 默认'),
          ],
        ),
      );

      expect(options.map((ModelOption o) => o.id).toList(), <String>[
        'codex/gpt-5',
        'pi/glm-5.3',
        'hermes/default',
      ], reason: '顺序保持 daemon 原序');
      expect(options[0].title, 'GPT-5');
      expect(options[1].title, 'pi/glm-5.3',
          reason: 'displayName 空串不算翻译成功：退回 id（Android modelDisplayLabel 同款兜底）');
      expect(options[1].isCurrent, isTrue);
      expect(options[0].isCurrent, isFalse);
      // current_model_id 为空（旧 daemon / 无会话上下文）= 谁都不标「当前使用」。
      expect(
        modelOptionsOfResponse(
          ListModelsResponse(
            models: <ModelInfo>[ModelInfo(id: 'a')],
          ),
        ).single.isCurrent,
        isFalse,
      );
    });

    test('detail：未知上下文不造数字、可用性不进副文案、字段顺序稳定', () {
      expect(
        option(contextWindow: 200000, supportsThinking: true, supportsImages: false, isCurrent: true).detail,
        '上下文 200k · 支持思考 · 当前使用',
      );
      expect(
        option(contextWindow: 0, supportsThinking: false, supportsImages: false)
            .detail,
        '',
        reason: '什么都没报 = 空副文案，不占位',
      );
      expect(
        option(contextWindow: 32000, supportsThinking: false, supportsImages: true).detail,
        '上下文 32k · 支持图片',
      );
    });

    test('坏字节 → PaBridgeException(DECODE_FAILED)，不裸抛 protobuf 异常', () {
      expect(
        () => modelOptionsOf(<int>[0xFF, 0xFF, 0x00, 0x01, 0x7F, 0xEE]),
        throwsA(isA<PaBridgeException>().having(
          (PaBridgeException e) => e.code,
          'code',
          PaBridgeErrorCodes.decodeFailed,
        )),
      );
    });

    test('modelOptionsOf：真字节 → 选项（与 decode 两步同源）', () {
      final Uint8List bytes = ListModelsResponse(
        currentModelId: 'm1',
        models: <ModelInfo>[ModelInfo(id: 'm1', provider: 'echo', displayName: 'Echo')],
      ).writeToBuffer();
      final List<ModelOption> options = modelOptionsOf(bytes);
      expect(options.single.id, 'm1');
      expect(options.single.isCurrent, isTrue);
    });
  });

  group('BridgeModelsDataSource：真桥接未就位时如实上报', () {
    test('无 invoke 闭包 → HOST_UNAVAILABLE（不编造空列表冒充「没有模型」）', () async {
      await expectLater(
        const BridgeModelsDataSource().listModels(),
        throwsA(isA<PaBridgeException>().having(
          (PaBridgeException e) => e.code,
          'code',
          PaBridgeErrorCodes.hostUnavailable,
        )),
      );
    });

    test('有 invoke：arguments = ListModelsRequest 空消息字节；回 wire 字节则解码',
        () async {
      String? method;
      Object? arguments;
      final BridgeModelsDataSource source = BridgeModelsDataSource(
        invoke: (String m, Object? args) async {
          method = m;
          arguments = args;
          return ListModelsResponse(
            currentModelId: 'pi/glm-5.3',
            models: <ModelInfo>[
              ModelInfo(id: 'pi/glm-5.3', provider: 'pi', available: true),
            ],
          ).writeToBuffer();
        },
      );

      final List<ModelOption> options = await source.listModels();
      expect(method, 'listModels');
      expect(arguments, isA<Uint8List>());
      expect((arguments! as Uint8List), isEmpty,
          reason: '本波空请求：无会话上下文，ListModelsRequest 全缺省 = 空消息字节');
      expect(options.single.id, 'pi/glm-5.3');
    });

    test('有 invoke：宿主回非字节 → DECODE_FAILED；state_error 非空且无模型 → ModelsUnavailableException(原文)',
        () async {
      await expectLater(
        BridgeModelsDataSource(invoke: (String m, Object? a) async => 'oops')
            .listModels(),
        throwsA(isA<PaBridgeException>().having(
          (PaBridgeException e) => e.code,
          'code',
          PaBridgeErrorCodes.decodeFailed,
        )),
      );

      await expectLater(
        BridgeModelsDataSource(
          invoke: (String m, Object? a) async => ListModelsResponse(
            stateError: 'metadata unavailable',
          ).writeToBuffer(),
        ).listModels(),
        throwsA(isA<ModelsUnavailableException>().having(
          (ModelsUnavailableException e) => e.reason,
          'reason',
          'metadata unavailable',
        )),
      );
    });

    test('有 invoke：state_error 非空但带模型 → 模型照常返回（元数据可用就不拦）',
        () async {
      final List<ModelOption> options = await BridgeModelsDataSource(
        invoke: (String m, Object? a) async => ListModelsResponse(
          stateError: 'partial',
          models: <ModelInfo>[ModelInfo(id: 'm1')],
        ).writeToBuffer(),
      ).listModels();
      expect(options.single.id, 'm1');
    });
  });

  group('页面三态', () {
    testWidgets('就绪：名称/来源/可用态三列渲染（含「当前使用」副文案与暂不可用徽标）',
        (tester) async {
      await pumpPage(
        tester,
        _FakeSource(models: <ModelOption>[
          option(isCurrent: true),
          option(
            id: 'dsh/deepseek',
            provider: 'dsh',
            displayName: '',
            contextWindow: 0,
            supportsThinking: false,
            supportsImages: false,
            available: false,
          ),
        ]),
      );
      await tester.pumpAndSettle();

      expect(find.text('GPT-5'), findsOneWidget);
      expect(find.text('codex'), findsWidgets, reason: '右列来源');
      // 不可用行：displayName 空 → 退回 id 当主文案，徽标如实「暂不可用」。
      expect(find.text('dsh/deepseek'), findsOneWidget);
      expect(find.text('暂不可用'), findsOneWidget);
      // 可用行徽标 + 当前标记。
      expect(find.text('可用'), findsOneWidget);
      expect(find.textContaining('当前使用'), findsOneWidget);
      // 三列键都在行上（行 key 以模型 id 定位）。
      expect(find.byKey(const ValueKey('pa-model-row-codex/gpt-5')), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-model-row-dsh/deepseek')), findsOneWidget);
    });

    testWidgets('加载中：先挂起态文案；数据到达后切列表', (tester) async {
      await pumpPage(tester, _HangingSource());
      await tester.pump();

      expect(find.byKey(const ValueKey('pa-models-loading')), findsOneWidget);
      expect(find.textContaining('正在读取模型列表'), findsOneWidget);
    });

    testWidgets('错误态：桥接码 → 映射文案 + 重试恢复；不见任何模型行',
        (tester) async {
      int calls = 0;
      final ModelsDataSource flaky = _FlakySource(
        onLoad: () {
          calls += 1;
          if (calls == 1) {
            throw const PaBridgeException(
              PaBridgeErrorCodes.hostUnavailable,
              'no host',
            );
          }
          return <ModelOption>[option()];
        },
      );

      await pumpPage(tester, flaky);
      await tester.pumpAndSettle();

      expect(
        find.textContaining('桌面端桥接未就绪（B4 波次）'),
        findsOneWidget,
      );
      expect(find.text('GPT-5'), findsNothing,
          reason: '读失败就不许出现模型行');

      await tester.tap(find.byKey(const ValueKey('pa-models-retry')));
      await tester.pumpAndSettle();
      expect(find.text('GPT-5'), findsOneWidget);
      expect(find.byKey(const ValueKey('pa-models-error')), findsNothing);
    });

    testWidgets('读失败（state_error）→ 带 daemon 原文的错误文案，不冒充「没有模型」',
        (tester) async {
      await pumpPage(
        tester,
        _FakeSource(failure: const ModelsUnavailableException('metadata unavailable')),
      );
      await tester.pumpAndSettle();

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-models-error')))
            .data,
        contains('metadata unavailable'),
      );
      expect(find.byKey(const ValueKey('pa-models-empty')), findsNothing);
    });

    testWidgets('空态：读成功但零模型 → 如实说「未报告」，不冒充读失败', (tester) async {
      await pumpPage(tester, _FakeSource(models: const <ModelOption>[]));
      await tester.pumpAndSettle();

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-models-empty')))
            .data,
        contains('未报告任何模型'),
      );
      expect(find.byKey(const ValueKey('pa-models-error')), findsNothing);
    });

    testWidgets('生产默认数据源（无注入）→ HOST_UNAVAILABLE 诚实占位', (tester) async {
      await pumpPage(tester, const BridgeModelsDataSource());
      await tester.pumpAndSettle();

      expect(
        find.textContaining('桌面端桥接未就绪（B4 波次）'),
        findsOneWidget,
      );
    });

    testWidgets('刷新键：重拉一次数据源（本页唯一动作 = 读）', (tester) async {
      final _FakeSource source = _FakeSource(models: <ModelOption>[option()]);
      await pumpPage(tester, source);
      await tester.pumpAndSettle();
      expect(source.loads, 1);

      await tester.tap(find.byKey(const ValueKey('pa-models-refresh')));
      await tester.pumpAndSettle();
      expect(source.loads, 2);
    });

    testWidgets('返回键回调到 onBack', (tester) async {
      bool back = false;
      await pumpPage(tester, _FakeSource(), onBack: () => back = true);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('pa-models-back')));
      expect(back, isTrue);
    });
  });

  group('纯函数：错误文案', () {
    test('HOST_UNAVAILABLE 点名 B4；其余委托通用映射；表外码透传', () {
      expect(
        capabilitiesPageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.hostUnavailable, 'x'),
        ),
        contains('桌面端桥接未就绪（B4 波次）'),
      );
      expect(
        capabilitiesPageErrorText(
          const PaBridgeException(PaBridgeErrorCodes.authRequired, 'x'),
        ),
        contains('需要配对'),
      );
      final String weird = capabilitiesPageErrorText(
          const PaBridgeException('WEIRD_HOST_CODE', '新错'));
      expect(weird, contains('WEIRD_HOST_CODE'));
      expect(weird, contains('新错'));
    });
  });
}

/// 按调用次数决定结果的源（错误→重试→成功链路用）。
class _FlakySource implements ModelsDataSource {
  _FlakySource({required this.onLoad});

  final List<ModelOption> Function() onLoad;

  @override
  Future<List<ModelOption>> listModels() async => onLoad();
}
