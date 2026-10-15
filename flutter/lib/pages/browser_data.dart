// Browser 页数据层（B3 第 6 波 B1）：`browser?url=` 目标分流 + 电脑文件预览类型 +
// 页内导航裁决（顶栏输入/初始目标级别的纯判定）。
//
// 【与 C2 的 browser_policy.dart 是什么关系】只读复用，不复制判定：
// 归一化用 [normalizeBrowserUrl]、scheme 形状用 [isBrowserWebScheme] /
// [isBrowserExternalScheme]（均来自 browser_policy.dart，C2 已逐函数对位
// WebNavPolicy.kt）。本文件只做「编排」——把三个 B5 才需要的结论钉成纯函数：
// ① 地址到底走哪条渲染链（分流）；② 电脑文件按后缀走哪种预览；③ 初始目标允不允许进。
//
// 【出处】
//  · 分流顺序（先分发 `pc:` 再归一化）= BrowserScreen.kt:359-365/984-986 的
//    `submit`/`Body` 分发口径；`pc:<path>[?_pas=&_pam=]` 元数据剥离是 B5 的
//    `stripPcMeta` 对位工作，本文件只管分流，不管剥离。
//  · 预览后缀集合 = PcFilePreview.kt 的 `render` 路由表 + `isBinary` 名单
//    （表格/文档/压缩包/图片/网页五类渲染口径，注释头 32-44 行）；
//    "md/txt/json/xml/源码 → 排版页/文本页"见同一文件头注释。
//  · 信任门（挂不挂下载桥）仍是 C2 的 [trustedBrowserBridgeSource]，本文件不包
//    第二层——安全红线只有一个真值。
//
// 【B5 接线说明】
//  · kind == web → WebView（`webview_flutter` 4.14.1，探针已验证镜像可解，
//    本波 repo 未加，见 b1-browser.md §1；无消费者时加依赖 = 死依赖）。
//  · kind == pcFile 且后缀 kind == text → 复用 B3-4 的 `BridgeFilePreviewSource`
//   （files_bridge.dart：`PaBridge.readFile` 首块 + UTF-8 + 大小门
//    [filePreviewable]），**不另起预览源**。
//  · kind == pcFile 且后缀 kind ∈ {table, document, archive} → B5 用 readFile
//    字节喂 `PcFilePreview.render` 移植（本波不移植 1434 行 Kotlin）。
//  · kind == localPreview（about:/data:/file:/content:）→ B5 本地渲染，
//    不走网络、不挂下载桥判定之外的任何东西。
//  · kind == blocked → 拦截页只展示 [BrowserTarget.reason]，别自己编第二套文案。

import 'browser_policy.dart';

// ── 目标分流 ─────────────────────────────────────────────────────────────

/// `browser?url=` 路由参数分流结论：地址到底走哪条渲染链。
enum BrowserTargetKind {
  /// 空起始页（Kotlin `BrowserScreen(initialUrl = "")`）：渲染起始页，不猜落点。
  empty,

  /// http(s)（含搜索词转出的 Bing 串）：B5 WebView 真渲染对象。
  web,

  /// App 预览/本地形态（about:/data:/file:/content:）：B5 本地渲染，不走网络。
  localPreview,

  /// `pc:` 前缀：电脑文件，B5 走 readFile 真通道 + 按后缀选预览器。
  pcFile,

  /// 打不开：拦截页只展示 reason（宁可严，未知形状一律落这里）。
  blocked,
}

/// 分流结果：[kind] 定渲染链；[address] 是各链的输入
/// （web = 归一化后 URL；pcFile = 剥掉 `pc:` 的电脑路径原文；其余为空串）；
/// [reason] 仅 blocked 非空（人话拦截说明，禁「未知/错误」字样，见 AGENTS.md §4.4）。
class BrowserTarget {
  const BrowserTarget({
    required this.kind,
    required this.address,
    this.reason = '',
  });

  final BrowserTargetKind kind;
  final String address;
  final String reason;
}

/// `browser?url=` 分流（纯函数）：先分发 `pc:`，再分发本地形态，最后裁决。
BrowserTarget browserTargetOf(String? raw) {
  final String t = (raw ?? '').trim();
  if (t.isEmpty) {
    return const BrowserTarget(kind: BrowserTargetKind.empty, address: '');
  }
  final String lower = t.toLowerCase();
  if (lower.startsWith('pc:')) {
    // Windows 盘符路径大小写敏感显示：只判前缀小写，路径取原文。
    final String path = t.substring(3).trim();
    if (path.isEmpty) {
      return const BrowserTarget(
        kind: BrowserTargetKind.blocked,
        address: '',
        reason: '电脑文件地址是空的：请回到文件页，重新点一次要预览的文件。',
      );
    }
    return BrowserTarget(kind: BrowserTargetKind.pcFile, address: path);
  }
  if (lower.startsWith('about:') ||
      lower.startsWith('data:') ||
      lower.startsWith('file:') ||
      lower.startsWith('content://')) {
    return BrowserTarget(kind: BrowserTargetKind.localPreview, address: t);
  }
  final BrowserNavVerdict verdict = browserNavVerdictOf(t);
  if (verdict.decision == BrowserNavDecision.allow) {
    // allow 时 normalize 必出 http(s) 或本地形态（见 verdict 注释），这里复用
    // 同一归一化，保证「裁决看到的 URL」与「渲染用的 URL」是同一个。
    return BrowserTarget(
      kind: BrowserTargetKind.web,
      address: normalizeBrowserUrl(t),
    );
  }
  return BrowserTarget(
    kind: BrowserTargetKind.blocked,
    address: '',
    reason: verdict.reason,
  );
}

// ── 电脑文件预览类型 ─────────────────────────────────────────────────────

/// 电脑文件按后缀的预览路由（口径 = PcFilePreview.kt 的 render 路由 + isBinary）。
enum PcPreviewKind {
  /// 纯文本/排版页：B5 走 `BridgeFilePreviewSource.readText`（readFile 首块 UTF-8）。
  text,

  /// 表格：xlsx/xlsm/xls/ods/csv/tsv，B5 用 readFile 字节喂表格渲染。
  table,

  /// 文档/演示/页位图：docx/doc/pptx/pdf…，B5 用 readFile 字节喂文档渲染。
  document,

  /// 图片：B5 图片直显，不走 readFile 文本通道。
  image,

  /// 网页：html/htm，B5 WebView 原生渲染。
  web,

  /// 压缩包目录：zip/jar，B5 读条目列表渲染。
  archive,

  /// 二进制或应用专属格式：诚实拒绝（见 [pcPreviewRefusal]）。
  unsupported,
}

/// 文件名 → 预览类型（纯函数，只看后缀，不读字节）。
///
/// [fileName] 传剥离 `?_pas=&_pam=` 元数据之后的名字（剥离是 B5 `stripPcMeta`
/// 的对位工作）；残留的 `?`/`#` 尾巴这里顺手剥掉，判后缀不受影响。
/// 无后缀/点结尾/点文件（如 `.gitignore` 判出 `gitignore`）一律 unsupported——
/// 宁可严，不猜。
PcPreviewKind pcPreviewKindOf(String fileName) {
  final String ext = _pcExtensionOf(fileName);
  if (ext.isEmpty) return PcPreviewKind.unsupported;
  switch (ext) {
    case 'md':
    case 'markdown':
    case 'txt':
    case 'json':
    case 'xml':
    case 'log':
    case 'yaml':
    case 'yml':
    case 'toml':
    case 'ini':
    case 'conf':
      return PcPreviewKind.text;
    case 'dart':
    case 'kotlin':
    case 'kt':
    case 'kts':
    case 'java':
    case 'py':
    case 'js':
    case 'jsx':
    case 'ts':
    case 'tsx':
    case 'go':
    case 'rs':
    case 'c':
    case 'h':
    case 'cpp':
    case 'hpp':
    case 'cs':
    case 'sh':
    case 'swift':
      // 源码 = PcFilePreview 头注释的"源码 → 排版页"分支，走文本通道。
      return PcPreviewKind.text;
    case 'xlsx':
    case 'xlsm':
    case 'xls':
    case 'ods':
    case 'fods':
    case 'csv':
    case 'tsv':
      return PcPreviewKind.table;
    case 'docx':
    case 'docm':
    case 'doc':
    case 'dot':
    case 'rtf':
    case 'odt':
    case 'fodt':
    case 'pptx':
    case 'pptm':
    case 'ppt':
    case 'odp':
    case 'fodp':
    case 'pdf':
      return PcPreviewKind.document;
    case 'png':
    case 'jpg':
    case 'jpeg':
    case 'gif':
    case 'webp':
    case 'bmp':
    case 'ico':
    case 'svg':
      return PcPreviewKind.image;
    case 'html':
    case 'htm':
      return PcPreviewKind.web;
    case 'zip':
    case 'jar':
      return PcPreviewKind.archive;
    default:
      // PcFilePreview.isBinary 名单（apk/exe/mp4/ttf…）与一切未收录后缀都落这里。
      return PcPreviewKind.unsupported;
  }
}

/// 预览被拒的人话理由（纯函数）：可预览的类型返回空串；unsupported 返回一句
/// 带后缀的诚实说明（禁「未知/错误」字样；大小门走 B3-4 [filePreviewRejection]，
/// 这里只管类型门）。
String pcPreviewRefusal(String fileName) {
  if (pcPreviewKindOf(fileName) != PcPreviewKind.unsupported) return '';
  final String ext = _pcExtensionOf(fileName);
  if (ext.isEmpty) {
    return '这个文件没有可识别的后缀，不能按文本预览：请换电脑上的对应程序打开。';
  }
  return '“.$ext”文件不能按文本预览：它是二进制或应用专属格式，'
      '请换电脑上的对应程序打开。';
}

/// 后缀提取：小写 + 剥 `?`/`#` 尾巴 + 取最后一段；无点/点结尾返回空串。
String _pcExtensionOf(String fileName) {
  String n = fileName.trim().toLowerCase();
  final int query = n.indexOf('?');
  if (query >= 0) n = n.substring(0, query);
  final int frag = n.indexOf('#');
  if (frag >= 0) n = n.substring(0, frag);
  final int dot = n.lastIndexOf('.');
  if (dot < 0 || dot == n.length - 1) return '';
  return n.substring(dot + 1);
}

// ── 页内导航裁决 ─────────────────────────────────────────────────────────

/// 初始目标/顶栏输入的导航裁决：放行或拦截（二值，无第三态）。
enum BrowserNavDecision {
  allow,
  block,
}

/// 裁决结果：[decision] 定放行/拦截；[reason] 仅 block 非空
/// （拦截页人话文案，禁「未知/错误」字样）。
class BrowserNavVerdict {
  const BrowserNavVerdict.allow() : decision = BrowserNavDecision.allow, reason = '';

  const BrowserNavVerdict.block(this.reason)
      : decision = BrowserNavDecision.block;

  final BrowserNavDecision decision;
  final String reason;
}

/// 初始目标裁决（纯函数）：外部专属 scheme → 拦；`javascript:`/`blob:`
/// 当作页面目标 → 拦（它们只是页内子资源形态，WebView 能处理子资源不等于能
/// 当作整页打开；**必须在归一化之前判**，否则无点号的 `javascript:…` 会被
/// [normalizeBrowserUrl] 转成 Bing 搜索串而漏网）；控制字符 → 拦；
/// 其余归一化形状（http(s)/about:/data:/file:/content:，[normalizeBrowserUrl]
/// 已钉死输出集合，无第七种）→ 放行。
BrowserNavVerdict browserNavVerdictOf(String raw) {
  final String t = raw.trim();
  if (t.isEmpty) {
    return const BrowserNavVerdict.block('没有可打开的地址：请返回重试。');
  }
  if (isBrowserExternalScheme(t)) {
    final int colon = t.indexOf(':');
    final String scheme = colon > 0 ? t.substring(0, colon) : '该';
    return BrowserNavVerdict.block(
      '这是 $scheme 打头的应用专属链接，内置浏览器打不开：请复制到对应应用里打开。',
    );
  }
  final String tl = t.toLowerCase();
  if (tl.startsWith('javascript:') || tl.startsWith('blob:')) {
    return const BrowserNavVerdict.block('这种地址不能当作整页打开：请回到上一页，点页面里的正常链接进入。');
  }
  final String n = normalizeBrowserUrl(t);
  if (n.isEmpty) {
    return const BrowserNavVerdict.block('这条链接里有多余的控制字符，打不开：请核对分享来源后重试。');
  }
  return const BrowserNavVerdict.allow();
}
