// Browser 占位页渲染测试：只有「就绪说明态」，断言诚实文案逐字。
//
// 【三态不适用】本波无 WebView 真渲染（webview_flutter 冻结），页面没有
// 加载中/读失败分支 —— 唯一状态就是「目标确认 + 诚实说明」。
// 文案逐字断言：改一个字就红，防止占位文案被悄悄改成「看起来能用了」。
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/browser.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';

Widget _host(BrowserPage page) => MaterialApp(
      theme: PocketTheme.fromBrightness(PABrightness.dark),
      home: page,
    );

void main() {
  testWidgets('有目标地址_显示归一化目标 + 诚实说明 + 外部站点桥策略', (tester) async {
    await tester.pumpWidget(
      _host(BrowserPage(initialUrl: 'example.com/a', onBack: () {})),
    );

    expect(
      find.byKey(const ValueKey<String>('browser-url')),
      findsOneWidget,
    );
    expect(find.text('https://example.com/a'), findsOneWidget);
    expect(
      tester.widget<Text>(find.byKey(const ValueKey<String>('browser-body'))).data,
      '网页渲染随 B5 依赖解冻接入：WebView（webview_flutter）未引入前，本页只做目标确认，不加载任何网络内容。',
    );
    expect(
      tester.widget<Text>(find.byKey(const ValueKey<String>('browser-trust-note'))).data,
      '脚本桥策略：外部站点，B5 接线时不挂载下载桥。',
    );
  });

  testWidgets('空地址_显示起始页 + 可信来源桥策略（App 预览形态）', (tester) async {
    await tester.pumpWidget(_host(BrowserPage(onBack: () {})));

    expect(find.text('起始页（无目标地址）'), findsOneWidget);
    expect(
      tester.widget<Text>(find.byKey(const ValueKey<String>('browser-trust-note'))).data,
      '脚本桥策略：可信来源，B5 接线时挂载下载桥。',
    );
  });

  testWidgets('私网目标_桥策略为可信来源', (tester) async {
    await tester.pumpWidget(
      _host(
        BrowserPage(initialUrl: 'http://192.168.1.7:7246/x.html', onBack: () {}),
      ),
    );

    expect(find.text('http://192.168.1.7:7246/x.html'), findsOneWidget);
    expect(
      tester.widget<Text>(find.byKey(const ValueKey<String>('browser-trust-note'))).data,
      '脚本桥策略：可信来源，B5 接线时挂载下载桥。',
    );
  });

  testWidgets('返回键触发 onBack', (tester) async {
    int backs = 0;
    await tester.pumpWidget(
      _host(BrowserPage(initialUrl: 'example.com', onBack: () => backs++)),
    );

    await tester.tap(find.byKey(const ValueKey<String>('browser-back')));
    expect(backs, 1);
  });
}
