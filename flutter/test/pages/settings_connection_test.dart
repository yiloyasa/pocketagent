// Settings ①连接子步测试 —— mock 的是「宿主桥的 wire 行为」，PaBridge 全程真实。
//
// 【为什么这样测】UI 层真正要验收的是三件事：① 快照/事件两路 conn_state 落到
// 状态条的三态文案；② pairCode/connect 的参数编码与返回语义；③ 错误码 → 用户
// 可读文案的分支（channel-contract.md §5）。三件都要真走 MethodChannel，用
// setMockMethodCallHandler 伪造宿主应答；直接 stub PaBridge 只能测到「我自己
// 写的假实现」。
//
// 【宿主未就绪的诚实路径】Kotlin 宿主桥 B4 波次才注册 handler，真实 connect 会
// 收到 MissingPluginException → HOST_UNAVAILABLE。测试按同路径伪造，断言 UI 显示
// 「桌面端桥接未就绪（B4 波次）」且绝不出现「已连接」——不许把未就绪粉饰成成功。

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/settings.dart';
import 'package:pocketagent_ui/pages/settings_connection.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  const EventChannel eventChannel = EventChannel(kPaBridgeEventChannel);

  late TestDefaultBinaryMessenger messenger;
  MockStreamHandlerEventSink? eventSink;
  late List<MethodCall> calls;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    calls = <MethodCall>[];
    eventSink = null;
    // 事件面默认放行 listen：状态条只在方法面探针成功后才订阅事件
    //（宿主缺席时根本不 listen，也就没有框架层握手错噪音）。
    // 【为什么必须用块体闭包】onListen 的返回值会流进 setMockStreamHandler 的
    // switch 表达式，成为 'listen' 方法的应答载荷：箭头体 `=> eventSink = sink`
    // 会把 sink 对象当应答返回，StandardMethodCodec 编码不了它 →
    // ArgumentError → listen 收到错误信封 → 服务层报错把整个用例判红。
    // 块体无 return，运行时返回 null，应答才是合法的空成功信封。
    messenger.setMockStreamHandler(
      eventChannel,
      MockStreamHandler.inline(
        onListen: (Object? args, MockStreamHandlerEventSink sink) {
          eventSink = sink;
        },
      ),
    );
  });

  /// 连接态五键 wire Map（键名 = Kotlin 五条 StateFlow 字段名，契约 §3.4）。
  Map<Object?, Object?> connMap({
    String state = 'ok',
    String transport = 'adb-reverse',
    String address = '127.0.0.1:7246',
    int epoch = 3,
  }) =>
      <Object?, Object?>{
        'connState': state,
        'connTransport': transport,
        'connAddress': address,
        'connectionEpoch': epoch,
        'hostCapabilities': <Object?>['terminal', 'fs', 'agent:codex'],
      };

  /// 记录每次方法调用；[onMethod] 可按方法名返回应答（或抛宿主错）。
  void mockMethods(FutureOr<Object?> Function(MethodCall call) onMethod) {
    messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
      calls.add(call);
      return onMethod(call);
    });
  }

  Future<void> pumpSettings(WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: SettingsPage(
            brightness: PABrightness.dark,
            onBrightnessChanged: (PABrightness b) {},
            bridge: PaBridge(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('状态条：PaConnState → 三态文案', () {
    testWidgets('ok → 已连接，并显示 transport 与 address', (tester) async {
      mockMethods((MethodCall c) => connMap());

      await pumpSettings(tester);

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-conn-state-label')))
            .data,
        '已连接',
      );
      expect(find.text('adb-reverse'), findsOneWidget);
      expect(find.text('127.0.0.1:7246'), findsOneWidget);
    });

    testWidgets('checking → 检查中', (tester) async {
      mockMethods((MethodCall c) => connMap(state: 'checking'));

      await pumpSettings(tester);

      expect(find.byKey(const ValueKey('pa-conn-state-label')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-conn-state-label')))
            .data,
        '检查中',
      );
    });

    testWidgets('down → 已断开', (tester) async {
      mockMethods((MethodCall c) => connMap(state: 'down', transport: '', address: ''));

      await pumpSettings(tester);

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-conn-state-label')))
            .data,
        '已断开',
      );
    });

    testWidgets('未知档位值原样透传（不显「未知/错误」，不进任何分支）',
        (tester) async {
      mockMethods((MethodCall c) => connMap(state: 'reconnecting'));

      await pumpSettings(tester);

      expect(find.text('reconnecting'), findsOneWidget);
      expect(find.text('未知'), findsNothing);
      expect(find.text('错误'), findsNothing);
    });
  });

  group('宿主缺席：诚实占位，不假装连接', () {
    testWidgets('快照 HOST_UNAVAILABLE → 「桌面端桥接未就绪（B4 波次）」+ 不宣称已连接',
        (tester) async {
      mockMethods((MethodCall c) async {
        if (c.method == 'getConnectionState') {
          // 复刻生产：宿主未注册 handler → 引擎回 null 应答 → MissingPluginException。
          throw MissingPluginException(
              'No implementation found for method getConnectionState');
        }
        return null;
      });

      await pumpSettings(tester);

      expect(
        find.textContaining('桌面端桥接未就绪（B4 波次）'),
        findsOneWidget,
        reason: '占位文案必须点名是哪一层没就绪（B4 波次），不能含糊成「连接失败」',
      );
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-conn-state-label')))
            .data,
        '状态不可读',
        reason: '读不到状态 ≠ 已连接，也 ≠ 已断开（断开是"连过又掉了"，我们并不知道）',
      );
      expect(find.text('已连接'), findsNothing);
      expect(find.text('已断开'), findsNothing);
      // 探针失败就不订阅事件面（契约：宿主在不在以方法面为准）
      expect(eventSink, isNull);
    });

    testWidgets('connect HOST_UNAVAILABLE → 表单下方诚实占位，不出现受理提示',
        (tester) async {
      mockMethods((MethodCall c) async {
        if (c.method == 'connect') {
          throw MissingPluginException('no host');
        }
        return connMap(state: 'down');
      });
      await pumpSettings(tester);

      await tester.enterText(
          find.byKey(const ValueKey('pa-connect-addr')), '192.168.1.20:7246');
      await tester.tap(find.byKey(const ValueKey('pa-connect-submit')));
      await tester.pumpAndSettle();

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-connect-error')))
            .data,
        contains('桌面端桥接未就绪（B4 波次）'),
      );
      expect(find.byKey(const ValueKey('pa-connect-notice')), findsNothing,
          reason: '没受理就不能显示「请求已受理」');
      expect(find.text('已连接'), findsNothing);
    });
  });

  group('配对码', () {
    testWidgets('点「读取配对码」→ pairCode 无参调用，配对码原样显示', (tester) async {
      mockMethods((MethodCall c) =>
          c.method == 'pairCode' ? '482913' : connMap());
      await pumpSettings(tester);

      expect(find.text('482913'), findsNothing,
          reason: '不自动读取：进页面不应主动向宿主要敏感值');
      await tester.tap(find.byKey(const ValueKey('pa-pair-read')));
      await tester.pumpAndSettle();

      expect(find.text('482913'), findsOneWidget);
      final MethodCall pair =
          calls.firstWhere((MethodCall c) => c.method == 'pairCode');
      expect(pair.arguments, isNull, reason: 'pairCode 无参（契约 §3）');
    });

    testWidgets('PAIR_DISABLED → 可读文案，且不显示任何配对码', (tester) async {
      mockMethods((MethodCall c) {
        if (c.method == 'pairCode') {
          throw PlatformException(code: 'PAIR_DISABLED', message: '非回环来源');
        }
        return connMap();
      });
      await pumpSettings(tester);

      await tester.tap(find.byKey(const ValueKey('pa-pair-read')));
      await tester.pumpAndSettle();

      expect(find.textContaining('配对码不可读'), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-pair-code-value')))
            .data,
        isNot(contains('482913')),
        reason: '读失败就清空：不保留旧码冒充本次结果',
      );
    });
  });

  group('手动连接', () {
    testWidgets('addr/token 键值原样过桥，受理后如实提示「请求已受理」且不宣告已连接',
        (tester) async {
      mockMethods((MethodCall c) =>
          c.method == 'connect' ? null : connMap(state: 'down'));
      await pumpSettings(tester);
      expect(find.text('已断开'), findsOneWidget);

      await tester.enterText(
          find.byKey(const ValueKey('pa-connect-addr')), '192.168.1.20:7246');
      await tester.enterText(
          find.byKey(const ValueKey('pa-connect-token')), 'tok-secret');
      await tester.tap(find.byKey(const ValueKey('pa-connect-submit')));
      await tester.pumpAndSettle();

      final List<MethodCall> connects =
          calls.where((MethodCall c) => c.method == 'connect').toList();
      expect(connects, hasLength(1));
      final Map<Object?, Object?> args =
          connects.single.arguments as Map<Object?, Object?>;
      expect(args.keys.toSet(), <String>{'addr', 'token'});
      expect(args['addr'], '192.168.1.20:7246');
      expect(args['token'], 'tok-secret',
          reason: 'token 只在进程内过桥（不进日志），原样透传');

      expect(find.byKey(const ValueKey('pa-connect-notice')), findsOneWidget);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-connect-notice')))
            .data,
        contains('请求已受理'),
      );
      expect(find.text('已连接'), findsNothing,
          reason: 'null 返回只是"受理"，Kotlin 连接是异步的——真值看状态条');
      expect(find.text('已断开'), findsOneWidget);
    });

    testWidgets('空地址本地即拒 INVALID_ADDR，不产生跨端调用', (tester) async {
      mockMethods((MethodCall c) => connMap());
      await pumpSettings(tester);

      await tester.tap(find.byKey(const ValueKey('pa-connect-submit')));
      await tester.pumpAndSettle();

      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('pa-connect-error')))
            .data,
        contains('连接地址无效'),
      );
      expect(calls.where((MethodCall c) => c.method == 'connect'), isEmpty,
          reason: '本地校验不过桥：过桥只会拿回同一个错');
    });

    testWidgets('宿主 AUTH_REQUIRED → 「需要配对」文案', (tester) async {
      mockMethods((MethodCall c) {
        if (c.method == 'connect') {
          throw PlatformException(code: 'AUTH_REQUIRED', message: 'token 过期');
        }
        return connMap();
      });
      await pumpSettings(tester);

      await tester.enterText(
          find.byKey(const ValueKey('pa-connect-addr')), '127.0.0.1:7246');
      await tester.enterText(
          find.byKey(const ValueKey('pa-connect-token')), 'stale-token');
      await tester.tap(find.byKey(const ValueKey('pa-connect-submit')));
      await tester.pumpAndSettle();

      expect(find.textContaining('需要配对'), findsOneWidget);
      expect(find.textContaining('AUTH_REQUIRED'), findsNothing,
          reason: '表内码要转成用户能行动的中文，不裸抛 code');
    });

    testWidgets('宿主 NOT_CONNECTED → 「尚未连上 daemon」文案', (tester) async {
      mockMethods((MethodCall c) {
        if (c.method == 'connect') {
          throw PlatformException(code: 'NOT_CONNECTED', message: 'no daemon');
        }
        return connMap();
      });
      await pumpSettings(tester);

      await tester.enterText(
          find.byKey(const ValueKey('pa-connect-addr')), '127.0.0.1:7246');
      await tester.tap(find.byKey(const ValueKey('pa-connect-submit')));
      await tester.pumpAndSettle();

      expect(find.textContaining('尚未连上 daemon'), findsOneWidget);
    });
  });

  group('事件面：conn_state 增量', () {
    testWidgets('探针成功后订阅事件，推送的 conn_state 全量快照实时改写状态条',
        (tester) async {
      // 【为什么在测试体内重新注册 handler】setMockStreamHandler 会新建一个内部
      // StreamController：在 setUp 里注册时它落在 FakeAsync zone 之外，sink.success
      // 排的微任务只有真异步（runAsync）才冲得出来，pumpAndSettle 收不到；
      // 在测试体内注册则 controller 创建在 fake zone 内，pump 一帧即达。
      // （探针实测：setUp 注册 got=[]，测试体内注册 got=[PaConnEvent(connState)]。）
      MockStreamHandlerEventSink? liveSink;
      messenger.setMockStreamHandler(
        eventChannel,
        MockStreamHandler.inline(
          onListen: (Object? args, MockStreamHandlerEventSink sink) {
            liveSink = sink;
          },
        ),
      );

      mockMethods((MethodCall c) => connMap(state: 'down'));
      await pumpSettings(tester);
      expect(find.text('已断开'), findsOneWidget);
      expect(liveSink, isNotNull, reason: '方法面探针成功 → 才订阅事件面');

      liveSink!.success(<Object?, Object?>{
        'type': 'conn_state',
        'connState': 'ok',
        'connTransport': 'grpc+tls',
        'connAddress': '100.64.0.7:7246',
        'connectionEpoch': 12,
        'hostCapabilities': <Object?>['terminal'],
      });
      await tester.pumpAndSettle();

      expect(find.text('已连接'), findsOneWidget);
      expect(find.text('已断开'), findsNothing);
      expect(find.text('grpc+tls'), findsOneWidget);
      expect(find.text('100.64.0.7:7246'), findsOneWidget);
    });
  });

  group('错误码 → 文案（纯函数）', () {
    test('七个契约码各有用户可读文案，且点名行动方向', () {
      String textOf(String code) => paBridgeErrorText(
          PaBridgeException(code, '宿主原因', details: null));

      expect(textOf(PaBridgeErrorCodes.hostUnavailable),
          contains('桌面端桥接未就绪（B4 波次）'));
      expect(textOf(PaBridgeErrorCodes.invalidAddr), contains('host:port'));
      expect(textOf(PaBridgeErrorCodes.authRequired), contains('需要配对'));
      expect(textOf(PaBridgeErrorCodes.notConnected),
          contains('尚未连上 daemon'));
      expect(textOf(PaBridgeErrorCodes.pairDisabled),
          contains('仅本机回环来源可读'));
      expect(textOf(PaBridgeErrorCodes.decodeFailed),
          contains('DECODE_FAILED'));
      expect(textOf(PaBridgeErrorCodes.internal), contains('宿主原因'),
          reason: '兜底码必须把宿主人话原因带出来');
    });

    test('表外 code 原样透传（code + message 都显示，不吞、不改写）', () {
      final String text =
          paBridgeErrorText(const PaBridgeException('WEIRD_HOST_CODE', '新错'));
      expect(text, contains('WEIRD_HOST_CODE'));
      expect(text, contains('新错'));
    });

    test('连接态未知档位透传（不进三态白名单，不显示「未知」）', () {
      expect(connStateLabelOf('ok'), '已连接');
      expect(connStateLabelOf('checking'), '检查中');
      expect(connStateLabelOf('down'), '已断开');
      expect(connStateLabelOf('syncing_v2'), 'syncing_v2');
    });
  });
}
