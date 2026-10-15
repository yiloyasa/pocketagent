// Browser 数据层纯函数测试（B3 第 6 波 B1）：分流 / 预览类型 / 导航裁决。
//
// 【对位口径】分流顺序（pc: 优先）= BrowserScreen.kt:359-365/984-986；
// 后缀集合 = PcFilePreview.kt render 路由 + isBinary 名单；
// scheme 形状复用 browser_policy.dart（C2 已对位 WebNavPolicy.kt），这里只测编排。
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/browser_data.dart';

void main() {
  group('browserTargetOf 分流', () {
    test('空/空白 → empty（不猜落点）', () {
      expect(browserTargetOf(null).kind, BrowserTargetKind.empty);
      expect(browserTargetOf('').kind, BrowserTargetKind.empty);
      expect(browserTargetOf('   ').kind, BrowserTargetKind.empty);
    });

    test('网址 → web（归一化后地址）', () {
      final BrowserTarget t = browserTargetOf('example.com/a');
      expect(t.kind, BrowserTargetKind.web);
      expect(t.address, 'https://example.com/a');
    });

    test('http 原样 → web', () {
      final BrowserTarget t = browserTargetOf('http://10.0.0.5:7246/');
      expect(t.kind, BrowserTargetKind.web);
      expect(t.address, 'http://10.0.0.5:7246/');
    });

    test('搜索词 → web（Bing 串，Kotlin 同款）', () {
      final BrowserTarget t = browserTargetOf('hello world');
      expect(t.kind, BrowserTargetKind.web);
      expect(t.address.startsWith('https://www.bing.com/search?q='), isTrue);
      expect(t.address, contains('hello+world'));
    });

    test('pc: 前缀 → pcFile（路径取原文）', () {
      final BrowserTarget t = browserTargetOf(r'pc:D:\docs\a.txt');
      expect(t.kind, BrowserTargetKind.pcFile);
      expect(t.address, r'D:\docs\a.txt');
    });

    test('pc: 大小写不敏感、空路径拦截', () {
      expect(browserTargetOf('PC:/home/u/a.md').kind, BrowserTargetKind.pcFile);
      final BrowserTarget t = browserTargetOf('pc:   ');
      expect(t.kind, BrowserTargetKind.blocked);
      expect(t.reason.isNotEmpty, isTrue);
    });

    test('本地形态 → localPreview（不走网络）', () {
      for (final String u in <String>[
        'about:blank',
        'data:text/html,<h1>hi</h1>',
        'file:///sdcard/a.html',
        'content://media/external/downloads/1',
      ]) {
        final BrowserTarget t = browserTargetOf(u);
        expect(t.kind, BrowserTargetKind.localPreview, reason: u);
        expect(t.address, u);
      }
    });

    test('外部 scheme → blocked（点名 scheme）', () {
      final BrowserTarget t = browserTargetOf('tel:10086');
      expect(t.kind, BrowserTargetKind.blocked);
      expect(t.address, isEmpty);
      expect(t.reason, contains('tel'));
    });

    test('控制字符 → blocked', () {
      final BrowserTarget t = browserTargetOf('a\x01b');
      expect(t.kind, BrowserTargetKind.blocked);
      expect(t.reason.isNotEmpty, isTrue);
    });

    test('javascript:/blob: 当整页目标 → blocked（页内子资源另算）', () {
      expect(browserTargetOf('javascript:alert(1)').kind, BrowserTargetKind.blocked);
      expect(
        browserTargetOf('blob:https://x.com/abc').kind,
        BrowserTargetKind.blocked,
      );
    });
  });

  group('pcPreviewKindOf 后缀路由（PcFilePreview.kt）', () {
    test('文本：文档与源码走文本通道', () {
      for (final String n in <String>[
        'report.md',
        'notes.TXT',
        'data.json',
        'conf.xml',
        'app.log',
        'main.dart',
        'Screen.kt',
        'bot.py',
      ]) {
        expect(pcPreviewKindOf(n), PcPreviewKind.text, reason: n);
      }
    });

    test('表格 / 文档 / 压缩包 / 图片 / 网页', () {
      expect(pcPreviewKindOf('data.xlsx'), PcPreviewKind.table);
      expect(pcPreviewKindOf('a.csv'), PcPreviewKind.table);
      expect(pcPreviewKindOf('deck.pptx'), PcPreviewKind.document);
      expect(pcPreviewKindOf('paper.PDF'), PcPreviewKind.document);
      expect(pcPreviewKindOf('app.zip'), PcPreviewKind.archive);
      expect(pcPreviewKindOf('photo.jpg'), PcPreviewKind.image);
      expect(pcPreviewKindOf('page.html'), PcPreviewKind.web);
    });

    test('二进制与未收录后缀 → unsupported', () {
      for (final String n in <String>[
        'setup.apk',
        'movie.mp4',
        'song.mp3',
        'tool.exe',
        'font.ttf',
        'data.bin',
      ]) {
        expect(pcPreviewKindOf(n), PcPreviewKind.unsupported, reason: n);
      }
    });

    test('无后缀/点结尾/点文件 → unsupported（宁可严）', () {
      expect(pcPreviewKindOf('Makefile'), PcPreviewKind.unsupported);
      expect(pcPreviewKindOf('a.'), PcPreviewKind.unsupported);
      expect(pcPreviewKindOf('.gitignore'), PcPreviewKind.unsupported);
    });

    test('残留 ?/# 尾巴不影响判后缀', () {
      expect(pcPreviewKindOf('a.pdf?_pas=x&_pam=1'), PcPreviewKind.document);
    });
  });

  group('pcPreviewRefusal 类型门', () {
    test('可预览类型返回空串', () {
      expect(pcPreviewRefusal('a.md'), isEmpty);
      expect(pcPreviewRefusal('a.xlsx'), isEmpty);
      expect(pcPreviewRefusal('a.png'), isEmpty);
    });

    test('unsupported 点名后缀', () {
      final String r = pcPreviewRefusal('setup.apk');
      expect(r, contains('.apk'));
      expect(pcPreviewRefusal('Makefile').isNotEmpty, isTrue);
    });
  });

  group('browserNavVerdictOf 裁决', () {
    test('https/网址放行', () {
      expect(
        browserNavVerdictOf('https://x.com/a').decision,
        BrowserNavDecision.allow,
      );
      expect(
        browserNavVerdictOf('example.com').decision,
        BrowserNavDecision.allow,
      );
    });

    test('空地址拦截', () {
      final BrowserNavVerdict v = browserNavVerdictOf('  ');
      expect(v.decision, BrowserNavDecision.block);
      expect(v.reason.isNotEmpty, isTrue);
    });

    test('外部 scheme 拦截并点名', () {
      final BrowserNavVerdict v = browserNavVerdictOf('weixin://dl/chat');
      expect(v.decision, BrowserNavDecision.block);
      expect(v.reason, contains('weixin'));
    });

    test('控制字符拦截', () {
      expect(
        browserNavVerdictOf('a\x7fb').decision,
        BrowserNavDecision.block,
      );
    });
  });

  group('红线：拦截/拒绝文案禁「未知/错误」字样（AGENTS.md §4.4）', () {
    test('全部 blocked reason 扫一遍', () {
      final List<String> reasons = <String>[
        browserTargetOf('pc:').reason,
        browserTargetOf('tel:1').reason,
        browserTargetOf('a\x01b').reason,
        browserTargetOf('javascript:x').reason,
        browserNavVerdictOf('').reason,
        browserNavVerdictOf('mailto:a@b.c').reason,
        pcPreviewRefusal('x.apk'),
        pcPreviewRefusal('Makefile'),
      ];
      for (final String r in reasons) {
        expect(r.isNotEmpty, isTrue);
        expect(r, isNot(contains('未知')), reason: r);
        expect(r, isNot(contains('错误')), reason: r);
      }
    });
  });
}
