// 内置浏览器的导航策略 + `browser?url=` 路由参数解析（B3 第 6 波）。
//
// 【出处】Android `ui/component/WebNavPolicy.kt`（110 行，object 纯函数）与
// `ui/BrowserScreen.kt` 的顶层纯函数 `normalizeUrl`（:1251）、`stripPcMeta` 是
// B5 的事（见下）。本文件是逐函数移植，判定口径与 Kotlin **逐条等价**，
// 唯一 deliberate 偏差是 [normalizeBrowserUrl] 对控制字符的拒绝（见该函数注释）。
//
// 【为什么先落纯函数】安全红线：`trustedBridgeSource` 决定 B5 是否给页面挂
// `WebDownloadBridge`（任意页面一行 JS 就能落盘任意文件，BrowserScreen.kt:142）。
// WebView（webview_flutter）依赖本波冻结，但信任判定不依赖任何插件——
// 先把策略钉死，B5 只做挂载动作，不再碰判定逻辑。
//
// 【本波不做的】`stripPcMeta`（`pc:<path>[?_pas=&_pam=]` 元数据剥离）、
// `toPcPath`（盘符判定）、`devServerPreviewUrl` / `isPcPreviewUrl`
// （dev server 面板，需 `HostService.listDevServers` 真通道）——
// 全部随 B5 真通道波次接，清单见 docs/reviews/2026-10-15/c2-terminal-browser.md。

/// WebView 能自己处理的 scheme（必须放行，否则页面内链接/子资源会被拦成
/// 「点了没反应」）。`file:` 也放行 —— 本地快照/内置页用它。
/// 出处：`WebNavPolicy.isWebScheme`（逐字对位，含前导空白容忍 + 全小写比较）。
bool isBrowserWebScheme(String url) {
  final String u = url.trim().toLowerCase();
  return u.startsWith('http://') ||
      u.startsWith('https://') ||
      u.startsWith('about:') ||
      u.startsWith('data:') ||
      u.startsWith('blob:') ||
      u.startsWith('javascript:') ||
      u.startsWith('file:') ||
      u.startsWith('content:');
}

/// 是否是「外部 App 专属」的自定义 scheme（`intent://` / `weixin://` /
/// `tel:` / `mailto:` …）。这些 WebView 一律加载不了，B5 必须拦截后交外部
/// App，绝不能直接 loadUrl（白屏）或抛 ActivityNotFoundException。
/// 相对链接（无 scheme）与 WebView 能处理的 scheme 返回 false。
/// 出处：`WebNavPolicy.isExternalScheme`（RFC 3986 scheme 形状检查同款）。
bool isBrowserExternalScheme(String url) {
  final String u = url.trim();
  if (u.isEmpty) return false;
  if (isBrowserWebScheme(u)) return false;
  final int colon = u.indexOf(':');
  if (colon <= 0) return false;
  final String scheme = u.substring(0, colon);
  return RegExp(r'^[A-Za-z][A-Za-z0-9+\-.]*$').hasMatch(scheme);
}

/// 该宿主是否属于「本地/自建、可信任」范围 —— B5 **仅**对本地自签证书放行。
/// 覆盖：回环（localhost / 127.0.0.1 / ::1）、私网（10/8、192.168/16、
/// 172.16/12）、Tailscale CGNAT（100.64/10）。公网宿主一律 false
/// （证书错误必须拒绝）。
/// 出处：`WebNavPolicy.trustedLocalHost`（含「带端口 `10.0.0.5:7246`」同款处理）。
bool browserTrustedLocalHost(String host) {
  final String h = host.trim().toLowerCase();
  if (h.isEmpty) return false;
  if (h == 'localhost' ||
      h == '127.0.0.1' ||
      h == '::1' ||
      h == '[::1]') {
    return true;
  }
  // Kotlin `substringBefore(':')`：把 `10.0.0.5:7246` 的端口先剥掉再看段。
  final String bare = h.split(':').first;
  final List<String> parts = bare.split('.');
  if (parts.length < 4) return false;
  final int? a = int.tryParse(parts[0]);
  final int? b = int.tryParse(parts[1]);
  if (a == null || b == null) return false;
  if (a == 10) return true;
  if (a == 192 && b == 168) return true;
  if (a == 172 && b >= 16 && b <= 31) return true;
  // Tailscale 100.64.0.0/10：上界 100.127.x.x（Kotlin 测试钉死 100.127.255.254）。
  if (a == 100 && b >= 64 && b <= 127) return true;
  return false;
}

/// 是否为「可信的脚本桥注入来源」—— B5 决定要不要给当前页面挂下载桥。
/// **绝不能把桥暴露给公网站点**。
///
/// 可信集合（覆盖 App 自身的全部预览/下载主路径）：
///  · `about:blank` / 空串 / `data:`（App 预览走无 host 形态）；
///  · `file:` / `content:`（本地文件形态）；
///  · 私网/回环/Tailscale 的 http(s)（复用 [browserTrustedLocalHost]）。
/// 其余（公网 http(s) 与其它 scheme）一律不可信。
/// 出处：`WebNavPolicy.trustedBridgeSource`。
bool trustedBrowserBridgeSource(String url) {
  final String u = url.trim().toLowerCase();
  // 空串/空白页：App 预览的典型回报形态，且不含任何外部页面。
  if (u.isEmpty || u == 'about:blank') return true;
  if (u.startsWith('data:') ||
      u.startsWith('file:') ||
      u.startsWith('content:')) {
    return true;
  }
  if (!u.startsWith('http://') && !u.startsWith('https://')) return false;
  final String host = Uri.tryParse(u)?.host ?? '';
  return browserTrustedLocalHost(host);
}

/// 从 Content-Disposition 头解析文件名，解析不到再从 URL 末段兜。
/// 支持 RFC 5987 的 `filename*=UTF-8''xxx`（百度/网盘常用）与普通
/// `filename="xxx"`。
/// 出处：`WebNavPolicy.fileNameFromDisposition`（含 `+` 字面量保护同款技巧）。
String browserFileNameFromDisposition(String disposition, String url) {
  String? star;
  final RegExpMatch? starMatch = RegExp(
    r"filename\*\s*=\s*[^']*'[^']*'([^;]+)",
    caseSensitive: false,
  ).firstMatch(disposition);
  if (starMatch != null) {
    final String raw = (starMatch.group(1) ?? '').trim();
    try {
      // RFC 5987 里 `+` 是字面量，而 percent-decode 会把它当空格 —— 先转义回来
      // （Kotlin `URLDecoder.decode(raw.replace("+", "%2B"))` 同款）。
      star = Uri.decodeComponent(raw.replaceAll('+', '%2B'));
    } catch (_) {
      star = raw;
    }
  }
  String? plain;
  final RegExpMatch? plainMatch = RegExp(
    r'filename\s*=\s*"?([^";]+)"?',
    caseSensitive: false,
  ).firstMatch(disposition);
  if (plainMatch != null) {
    // Kotlin `.trim().trim('"')`：去两端空白后再去两端引号。
    plain = (plainMatch.group(1) ?? '')
        .trim()
        .replaceAll(RegExp(r'^"+|"+$'), '');
  }
  String fromUrl = url;
  final int slash = fromUrl.lastIndexOf('/');
  if (slash >= 0) fromUrl = fromUrl.substring(slash + 1);
  final int query = fromUrl.indexOf('?');
  if (query >= 0) fromUrl = fromUrl.substring(0, query);
  final int frag = fromUrl.indexOf('#');
  if (frag >= 0) fromUrl = fromUrl.substring(0, frag);
  final String name = (star ?? plain ?? fromUrl).trim();
  return name.isEmpty ? '下载文件' : name;
}

/// `browser?url=` 路由参数归一化（纯函数，与 Android `normalizeUrl`
/// BrowserScreen.kt:1251 同语义 + 路由层适配）：
///
///  · 空串（含 null）→ `''` = 空起始页（调用方渲染起始页，**不猜落点**；
///    与 `normalizeSettingsSub` 未知回一级页同理，深链参数来自历史入口）；
///  · 已带可用 scheme（http/https/about:/data:）→ 原样返回；
///  · `pc:` / `content://` / `file:` → 原样返回：它们是 B5 的分发前缀
///    （Kotlin 在 `submit`/`Body` 里先分发 pc/content 再调 `normalizeUrl`，
///    见 BrowserScreen.kt:359-365/984-986；这里把「分发保留」折进归一化，
///    B5 按前缀分发，`?_pas=` 元数据剥离仍是 B5 的 `stripPcMeta` 对位工作）；
///  · 含控制字符（`[\x00-\x1F\x7F]`，破损深链）→ `''` **拒绝**
///    （【唯一 deliberate 偏差】Kotlin 会把它送进 Bing 查询串；路由参数来自
///    外部深链，拒绝比搜索更安全，测试钉死）；
///  · 像网址（含 `.` 且无空格）→ 补 `https://`；
///  · 其余搜索词 → 交 Bing（Kotlin 同款 `URLEncoder.encode` 口径，
///    Dart `Uri.encodeQueryComponent` 同为 form-urlencoded，空格编 `+`）。
String normalizeBrowserUrl(String? raw) {
  final String t = (raw ?? '').trim();
  if (t.isEmpty) return '';
  if (t.contains(RegExp(r'[\x00-\x1F\x7F]'))) return '';
  if (t.startsWith('http://') ||
      t.startsWith('https://') ||
      t.startsWith('about:') ||
      t.startsWith('data:') ||
      t.startsWith('pc:') ||
      t.startsWith('content://') ||
      t.startsWith('file:')) {
    return t;
  }
  if (t.contains('.') && !t.contains(' ')) return 'https://$t';
  return 'https://www.bing.com/search?q=${Uri.encodeQueryComponent(t)}';
}
