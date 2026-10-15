// Browser 安全策略 + 路由参数解析纯函数测试（Kotlin 对位）。
//
// 【对位口径】断言全部来自 Android 真值：
//  · `WebNavPolicyWorkflowC.kt`（6 个 @Test 逐条移植，见各组注释行号）；
//  · `normalizeUrl`（BrowserScreen.kt:1251-1258）+ `browser?url=` 路由语义
//    （MainActivity 路由表：url 默认空串；page-contracts §0）。
//
// 【唯一 deliberate 偏差】控制字符：Kotlin 会把它送进 Bing 查询串，本移植
// 拒绝回空起始页（路由参数来自外部深链，拒绝比搜索更安全；见 browser_policy.dart）。
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/browser_policy.dart';

void main() {
  group('isBrowserWebScheme（WebNavPolicyWorkflowC.webSchemesAreNotExternal）', () {
    test('http/https 放行', () {
      expect(isBrowserWebScheme('https://www.baidu.com'), isTrue);
      expect(isBrowserWebScheme('http://x.com'), isTrue);
    });

    test('内置页形态放行', () {
      expect(isBrowserWebScheme('about:blank'), isTrue);
      expect(isBrowserWebScheme('data:text/plain,hi'), isTrue);
      expect(isBrowserWebScheme('blob:https://x.com/abc'), isTrue);
      expect(isBrowserWebScheme('file:///sdcard/a.html'), isTrue);
      expect(isBrowserWebScheme('content://media/external/downloads/1'), isTrue);
      expect(isBrowserWebScheme('javascript:void(0)'), isTrue);
    });

    test('外部 scheme 不算 web scheme', () {
      expect(isBrowserWebScheme('baiduboxapp://xx'), isFalse);
      expect(isBrowserWebScheme('weixin://dl/chat'), isFalse);
    });

    test('大小写不敏感（Kotlin 先 lowercase）', () {
      expect(isBrowserWebScheme('HTTPS://X.COM/A'), isTrue);
      expect(isBrowserWebScheme('  https://x.com  '), isTrue);
    });
  });

  group('isBrowserExternalScheme（externalSchemesAreDetected + webAndRelativeUrlsAreNotExternal）', () {
    test('外部 App 专属 scheme 命中', () {
      expect(
        isBrowserExternalScheme('intent://scan/#Intent;scheme=zxing;end'),
        isTrue,
      );
      expect(isBrowserExternalScheme('baiduboxapp://xx'), isTrue);
      expect(isBrowserExternalScheme('snssdk1128://feed'), isTrue);
      expect(isBrowserExternalScheme('weixin://dl/chat'), isTrue);
      expect(isBrowserExternalScheme('tel:10086'), isTrue);
      expect(isBrowserExternalScheme('mailto:a@b.com'), isTrue);
    });

    test('web 与相对链接不算外部', () {
      expect(isBrowserExternalScheme('https://x.com/a'), isFalse);
      expect(isBrowserExternalScheme('//x.com/a'), isFalse);
      expect(isBrowserExternalScheme('/a/b'), isFalse);
      expect(isBrowserExternalScheme(''), isFalse);
      expect(isBrowserExternalScheme(':novalidscheme'), isFalse);
    });
  });

  group('browserTrustedLocalHost（trustedLocalHostCoversPrivateRangesOnly）', () {
    test('回环与私网段可信', () {
      expect(browserTrustedLocalHost('localhost'), isTrue);
      expect(browserTrustedLocalHost('127.0.0.1'), isTrue);
      expect(browserTrustedLocalHost('::1'), isTrue);
      expect(browserTrustedLocalHost('10.0.0.5'), isTrue);
      expect(browserTrustedLocalHost('192.168.1.7'), isTrue);
      expect(browserTrustedLocalHost('172.16.3.4'), isTrue);
      expect(browserTrustedLocalHost('172.31.255.254'), isTrue);
      expect(browserTrustedLocalHost('100.64.0.9'), isTrue);
      expect(browserTrustedLocalHost('100.127.255.254'), isTrue);
      expect(browserTrustedLocalHost('10.0.0.5:7246'), isTrue);
    });

    test('边界外一律不可信', () {
      expect(browserTrustedLocalHost('100.63.0.1'), isFalse);
      expect(browserTrustedLocalHost('100.128.0.1'), isFalse);
      expect(browserTrustedLocalHost('172.32.0.1'), isFalse);
      expect(browserTrustedLocalHost('8.8.8.8'), isFalse);
      expect(browserTrustedLocalHost('www.baidu.com'), isFalse);
      expect(browserTrustedLocalHost(''), isFalse);
    });

    test('大小写不敏感', () {
      expect(browserTrustedLocalHost('LOCALHOST'), isTrue);
    });
  });

  group('trustedBrowserBridgeSource（bridgeSourceTrustsAppPreviewAndLocalOnly + bridgeSourceRejectsPublicSites）', () {
    test('App 预览形态可信', () {
      expect(trustedBrowserBridgeSource(''), isTrue);
      expect(trustedBrowserBridgeSource('about:blank'), isTrue);
      expect(
        trustedBrowserBridgeSource(
          'data:text/html;charset=utf-8;base64,PGgxPkhpPC9oMT4=',
        ),
        isTrue,
      );
    });

    test('本地文件形态可信', () {
      expect(trustedBrowserBridgeSource('file:///sdcard/a.html'), isTrue);
      expect(
        trustedBrowserBridgeSource('content://media/external/downloads/1'),
        isTrue,
      );
    });

    test('私网 daemon 可信', () {
      expect(
        trustedBrowserBridgeSource('http://127.0.0.1:7246/preview'),
        isTrue,
      );
      expect(
        trustedBrowserBridgeSource('http://192.168.1.7:7246/x.html'),
        isTrue,
      );
      expect(trustedBrowserBridgeSource('http://100.64.0.9:7246/x'), isTrue);
    });

    test('公网站点一律不可信', () {
      expect(trustedBrowserBridgeSource('https://www.baidu.com/'), isFalse);
      expect(trustedBrowserBridgeSource('https://www.douyin.com/'), isFalse);
      expect(trustedBrowserBridgeSource('http://8.8.8.8/x'), isFalse);
      expect(
        trustedBrowserBridgeSource('https://example.com/a/b?q=1'),
        isFalse,
      );
    });

    test('非 http(s) 的其它 scheme 不可信', () {
      expect(
        trustedBrowserBridgeSource('blob:https://example.com/xyz'),
        isFalse,
      );
      expect(trustedBrowserBridgeSource('baiduboxapp://xx'), isFalse);
    });
  });

  group('browserFileNameFromDisposition（fileNameFromDispositionHandlesRfc5987AndPlainAndUrl）', () {
    test('RFC 5987 编码文件名', () {
      expect(
        browserFileNameFromDisposition(
          "attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.pdf",
          'https://x/a',
        ),
        '报告.pdf',
      );
    });

    test('RFC 5987 里 + 是字面量（不许变空格）', () {
      expect(
        browserFileNameFromDisposition(
          "attachment; filename*=UTF-8''a+b.txt",
          'https://x/a',
        ),
        'a+b.txt',
      );
    });

    test('普通 filename', () {
      expect(
        browserFileNameFromDisposition(
          'attachment; filename="report.pdf"',
          'https://x/a',
        ),
        'report.pdf',
      );
    });

    test('头里没有就从 URL 末段兜（含 query 剥离）', () {
      expect(
        browserFileNameFromDisposition('', 'https://x/a/b.zip?t=1'),
        'b.zip',
      );
    });

    test('都拿不到回兜底名', () {
      expect(browserFileNameFromDisposition('', 'https://x/'), '下载文件');
    });
  });

  group('normalizeBrowserUrl（browser?url= 路由参数解析）', () {
    test('空回起始页（null/空串/纯空白）', () {
      expect(normalizeBrowserUrl(null), '');
      expect(normalizeBrowserUrl(''), '');
      expect(normalizeBrowserUrl('   '), '');
    });

    test('缺 scheme 补 https', () {
      expect(normalizeBrowserUrl('example.com/a'), 'https://example.com/a');
      expect(normalizeBrowserUrl('  example.com  '), 'https://example.com');
    });

    test('已有可用 scheme 原样返回', () {
      expect(
        normalizeBrowserUrl('https://x.com/a?q=1'),
        'https://x.com/a?q=1',
      );
      expect(normalizeBrowserUrl('http://x.com'), 'http://x.com');
      expect(normalizeBrowserUrl('about:blank'), 'about:blank');
      expect(normalizeBrowserUrl('data:text/plain,hi'), 'data:text/plain,hi');
    });

    test('B5 分发前缀原样保留（pc:/content:/file: 不吞不改）', () {
      expect(
        normalizeBrowserUrl(r'pc:D:\docs\index.html'),
        r'pc:D:\docs\index.html',
      );
      expect(
        normalizeBrowserUrl('content://media/external/downloads/1'),
        'content://media/external/downloads/1',
      );
    });

    test('非法字符拒绝（控制字符回起始页，不送搜索）', () {
      expect(normalizeBrowserUrl('a\u0000b.com'), '');
      expect(normalizeBrowserUrl('https://x.com/a\u000Ab'), '');
    });

    test('搜索词走 Bing（Kotlin normalizeUrl 同口径）', () {
      expect(
        normalizeBrowserUrl('hello world'),
        'https://www.bing.com/search?q=hello+world',
      );
    });
  });
}
