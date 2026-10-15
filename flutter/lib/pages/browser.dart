// 内置浏览器页（B3 第 6 波）：目标确认占位页 + 出口回调形状。
//
// 【页面职责】page-contracts §1.9 Browser：`browser?url={url}`（url 默认空串），
// 入口来自 Home 抽屉、Files 预览、Chat 链接、TaskCenter 下载、Settings 下载页、
// Git；出仅 `onBack`（Kotlin 另有「页内有历史先网页后退」的分级回退，
// BrowserScreen.kt:154-157，随 WebView 真渲染波次实现，见 B5 清单）。
//
// 【本波只落页壳 + 安全策略】WebView 需要 `webview_flutter` 依赖（pubspec 冻结，
// 本波不加），且电脑文件渲染（`FileSystemService.readFile` 流式 + PcFilePreview）
// 与 dev server 面板（`HostService.listDevServers`）都要真通道 ——
// 本页**不加载任何网络内容**，只做目标确认（显示归一化后的目标 url +
// [trustedBrowserBridgeSource] 的桥策略预告），文案如实说明，不假装能渲染网页。
//
// 【信任预告不是挂载】页内展示的「脚本桥策略」行只是纯函数判定结果的诚实公示；
// 真正的 add/removeJavascriptInterface 发生在 B5 的 WebView 装配里。

import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import 'browser_policy.dart';
import 'pa_page_kit.dart';

/// 「在内置浏览器打开」出口形状（`browser?url=` 的导航回调抽象）。
///
/// 【为什么只定形状不改调用方】Files / TaskCenter / Settings / Chat 的
/// 「打开链接」出口分属别人的页面领地（本波冻结）；B4/B5 回填时，那些页面把
/// 「打开」动作收敛成这个签名的可空回调：**不注入就不渲染对应入口**
/// （无位不显示假入口，与 TaskCenterPage 的可空重试回调同纪律）。
/// 回填清单见 docs/reviews/2026-10-15/c2-terminal-browser.md §出口回填清单。
typedef OpenBrowserUrl = void Function(String url);

/// 内置浏览器页：目标确认占位（无 WebView 真渲染）。
///
/// [initialUrl] 即 `browser?url=` 的路由参数（默认空串 = 空起始页，
/// 与 Kotlin `BrowserScreen(initialUrl = "")` 同签名）。
class BrowserPage extends StatelessWidget {
  const BrowserPage({super.key, this.initialUrl = '', required this.onBack});

  /// 目标地址原文；页内经 [normalizeBrowserUrl] 归一化后展示。
  final String initialUrl;

  /// 返回（Kotlin `onBack` → popBackStack；页内历史后退随 B5 真渲染实现）。
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String target = normalizeBrowserUrl(initialUrl);
    // 桥策略预告：空起始页是 App 预览形态（可信），与 Kotlin 初值
    // `!(http||https)`（BrowserScreen.kt:126-128）同口径。
    final bool trusted = trustedBrowserBridgeSource(target);
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: <Widget>[
        PaSubPageHeader(
          title: '浏览器',
          backKey: 'browser-back',
          onBack: onBack,
        ),
        PACard(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                target.isEmpty ? '起始页（无目标地址）' : target,
                key: const ValueKey<String>('browser-url'),
                style: PATypography.body.copyWith(
                  color: pa.fgPrimary,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                '网页渲染随 B5 依赖解冻接入：WebView（webview_flutter）未引入前，本页只做目标确认，不加载任何网络内容。',
                key: const ValueKey<String>('browser-body'),
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
              const SizedBox(height: 8),
              Text(
                trusted
                    ? '脚本桥策略：可信来源，B5 接线时挂载下载桥。'
                    : '脚本桥策略：外部站点，B5 接线时不挂载下载桥。',
                key: const ValueKey<String>('browser-trust-note'),
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
